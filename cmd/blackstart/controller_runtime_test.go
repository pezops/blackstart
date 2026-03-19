package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
)

func TestComputeNextRunFromStatus_RestoreFromLastRan(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)

	t.Run("overdue runs immediately", func(t *testing.T) {
		next := computeNextRunFromStatus(now, now.Add(-2*time.Hour), true, time.Hour)
		require.Equal(t, now, next)
	})

	t.Run("not overdue waits until next interval", func(t *testing.T) {
		next := computeNextRunFromStatus(now, now.Add(-20*time.Minute), true, time.Hour)
		require.Equal(t, now.Add(40*time.Minute), next)
	})
}

func TestRunWorkflowsControllerInK8s_RestoresOverdueScheduleFromStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	workflow := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "overdue-workflow",
			Namespace: "default",
		},
		Spec: v1alpha1.WorkflowSpec{
			ReconcileInterval: "1h",
			Operations:        []v1alpha1.Operation{},
		},
		Status: v1alpha1.WorkflowStatus{
			LastRan: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
	}

	fakeClient := newFakeClientWithStatus(scheme, workflow)
	statusClient, ok := fakeClient.(*fakeClientWithStatus)
	require.True(t, ok)

	cfg := &blackstart.RuntimeConfig{
		RuntimeMode:                "controller",
		KubeNamespace:              "default",
		MaxParallelReconciliations: 1,
		ControllerResyncInterval:   "100ms",
		QueueWaitWarningThreshold:  "1ms",
		LogFormat:                  "text",
		LogLevel:                   "info",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(cfg))

	require.NoError(t, runWorkflowsControllerInK8s(ctx, fakeClient))

	updatedRaw, found := statusClient.statusStore.Load("default/overdue-workflow")
	require.True(t, found, "expected overdue workflow to run and update status")

	updated, ok := updatedRaw.(*v1alpha1.Workflow)
	require.True(t, ok)
	require.True(t, updated.Status.LastRan.After(workflow.Status.LastRan.Time))
	require.True(t, updated.Status.NextRun.After(updated.Status.LastRan.Time))
}

func TestRunWorkflowsControllerInK8s_ResyncFindsNewWorkflow(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	fakeClient := newFakeClientWithStatus(scheme)
	statusClient, ok := fakeClient.(*fakeClientWithStatus)
	require.True(t, ok)

	cfg := &blackstart.RuntimeConfig{
		RuntimeMode:                "controller",
		KubeNamespace:              "default",
		MaxParallelReconciliations: 1,
		ControllerResyncInterval:   "100ms",
		QueueWaitWarningThreshold:  "1ms",
		LogFormat:                  "text",
		LogLevel:                   "info",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1600*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(cfg))

	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = fakeClient.Create(
			context.Background(),
			&v1alpha1.Workflow{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "blackstart.pezops.github.io/v1alpha1",
					Kind:       "Workflow",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "created-later",
					Namespace: "default",
				},
				Spec: v1alpha1.WorkflowSpec{
					ReconcileInterval: "5m",
					Operations:        []v1alpha1.Operation{},
				},
			},
		)
	}()

	require.NoError(t, runWorkflowsControllerInK8s(ctx, fakeClient))

	_, found := statusClient.statusStore.Load("default/created-later")
	require.True(t, found, "expected controller resync loop to find and run new workflow")
}

func TestControllerScheduler_NoOverlapForQueuedOrRunning(t *testing.T) {
	scheduler := newControllerScheduler()
	now := time.Now()
	wf := &blackstart.Workflow{
		Name:              "overlap-test",
		ReconcileInterval: time.Minute,
		Source: &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "overlap-test", Namespace: "default"},
			Spec:       v1alpha1.WorkflowSpec{Operations: []v1alpha1.Operation{}},
		},
	}

	scheduler.replaceFromWorkflows(now, []*blackstart.Workflow{wf})
	due := scheduler.dueWorkflows(now)
	require.Len(t, due, 1)

	// Still queued -> must not be selected again.
	due = scheduler.dueWorkflows(now)
	require.Len(t, due, 0)

	scheduler.markRunning(scheduler.entries[scheduleKey(types.NamespacedName{Name: "overlap-test", Namespace: "default"})])
	due = scheduler.dueWorkflows(now)
	require.Len(t, due, 0)
}

func TestControllerScheduler_QueueHealthDetectsMissedIntervals(t *testing.T) {
	scheduler := newControllerScheduler()
	now := time.Now()

	wf := &blackstart.Workflow{
		Name:              "missed-interval",
		ReconcileInterval: 100 * time.Millisecond,
		Source: &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "missed-interval", Namespace: "default"},
			Spec:       v1alpha1.WorkflowSpec{Operations: []v1alpha1.Operation{}},
		},
	}

	scheduler.replaceFromWorkflows(now, []*blackstart.Workflow{wf})
	due := scheduler.dueWorkflows(now)
	require.Len(t, due, 1)

	health := scheduler.queueHealth(now.Add(150 * time.Millisecond))
	require.Equal(t, 1, health.queuedCount)
	require.Equal(t, []string{"default/missed-interval"}, health.missedIntervalKeys)
}

func TestControllerScheduler_QueueFullBackoffPreventsImmediateReselect(t *testing.T) {
	scheduler := newControllerScheduler()
	now := time.Now()
	wf := &blackstart.Workflow{
		Name:              "queue-backoff",
		ReconcileInterval: time.Minute,
		Source: &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "queue-backoff", Namespace: "default"},
			Spec:       v1alpha1.WorkflowSpec{Operations: []v1alpha1.Operation{}},
		},
	}

	scheduler.replaceFromWorkflows(now, []*blackstart.Workflow{wf})
	due := scheduler.dueWorkflows(now)
	require.Len(t, due, 1)

	scheduler.markQueueFull(due[0].entry, now, time.Second)

	due = scheduler.dueWorkflows(now.Add(500 * time.Millisecond))
	require.Len(t, due, 0)

	due = scheduler.dueWorkflows(now.Add(1100 * time.Millisecond))
	require.Len(t, due, 1)
}

func TestControllerScheduler_ReplaceDoesNotDropRunningEntry(t *testing.T) {
	scheduler := newControllerScheduler()
	now := time.Now()

	wf := &blackstart.Workflow{
		Name:              "keep-running",
		ReconcileInterval: time.Minute,
		Source: &v1alpha1.Workflow{
			ObjectMeta: metav1.ObjectMeta{Name: "keep-running", Namespace: "default"},
			Spec:       v1alpha1.WorkflowSpec{Operations: []v1alpha1.Operation{}},
		},
	}

	scheduler.replaceFromWorkflows(now, []*blackstart.Workflow{wf})
	due := scheduler.dueWorkflows(now)
	require.Len(t, due, 1)
	scheduler.markRunning(due[0].entry)

	// Simulate a transient refresh where this workflow is missing from the loaded list.
	scheduler.replaceFromWorkflows(now, []*blackstart.Workflow{})

	key := scheduleKey(types.NamespacedName{Name: "keep-running", Namespace: "default"})
	_, exists := scheduler.entries[key]
	require.True(t, exists, "running entry should not be dropped during refresh")
}

func TestParseControllerRuntimeOptions(t *testing.T) {
	_, err := parseControllerRuntimeOptions(
		&blackstart.RuntimeConfig{
			MaxParallelReconciliations: 0,
			ControllerResyncInterval:   "15s",
			QueueWaitWarningThreshold:  "30s",
		},
	)
	require.Error(t, err)

	opts, err := parseControllerRuntimeOptions(
		&blackstart.RuntimeConfig{
			MaxParallelReconciliations: 2,
			ControllerResyncInterval:   "15s",
			QueueWaitWarningThreshold:  "30s",
		},
	)
	require.NoError(t, err)
	require.Equal(t, 2, opts.MaxParallel)
	require.Equal(t, 15*time.Second, opts.ResyncInterval)
	require.Equal(t, 30*time.Second, opts.QueueWarnThreshold)
}

func TestParseNamespaces(t *testing.T) {
	t.Run("empty means all namespaces", func(t *testing.T) {
		got := parseNamespaces(&blackstart.RuntimeConfig{KubeNamespace: ""})
		require.Equal(t, []string{""}, got)
	})

	t.Run("single namespace", func(t *testing.T) {
		got := parseNamespaces(&blackstart.RuntimeConfig{KubeNamespace: " default "})
		require.Equal(t, []string{"default"}, got)
	})

	t.Run("dedupe and ignore empty segments", func(t *testing.T) {
		got := parseNamespaces(&blackstart.RuntimeConfig{KubeNamespace: "default, ,default,team-a,"})
		require.Equal(t, []string{"default", "team-a"}, got)
	})

	t.Run("all empty segments falls back to all namespaces", func(t *testing.T) {
		got := parseNamespaces(&blackstart.RuntimeConfig{KubeNamespace: " , , "})
		require.Equal(t, []string{""}, got)
	})
}
