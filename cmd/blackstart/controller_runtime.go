package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	k8swatch "k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
)

type controllerRuntimeOptions struct {
	MaxParallel        int
	ResyncInterval     time.Duration
	QueueWarnThreshold time.Duration
}

const controllerDispatchInterval = 200 * time.Millisecond
const controllerQueueRetryDelay = 1 * time.Second

type scheduledWorkflow struct {
	key       types.NamespacedName
	workflow  *blackstart.Workflow
	interval  time.Duration
	nextRunAt time.Time
	queuedAt  time.Time
	running   bool
	queued    bool
}

type scheduledWorkflowRun struct {
	entry    *scheduledWorkflow
	key      types.NamespacedName
	workflow *blackstart.Workflow
	queuedAt time.Time
}

type controllerScheduler struct {
	mu      sync.Mutex
	entries map[string]*scheduledWorkflow
}

type queueHealth struct {
	queuedCount        int
	missedIntervalKeys []string
}

func newControllerScheduler() *controllerScheduler {
	return &controllerScheduler{entries: map[string]*scheduledWorkflow{}}
}

func scheduleKey(key types.NamespacedName) string {
	if key.Namespace == "" {
		return key.Name
	}
	return key.Namespace + "/" + key.Name
}

func computeNextRunFromStatus(now time.Time, lastRan time.Time, hasLastRan bool, interval time.Duration) time.Time {
	if interval <= 0 {
		return now
	}
	if !hasLastRan {
		return now
	}
	next := lastRan.Add(interval)
	if next.Before(now) {
		return now
	}
	return next
}

func (s *controllerScheduler) upsert(now time.Time, wf *blackstart.Workflow) {
	kwf, ok := wf.Source.(*v1alpha1.Workflow)
	if !ok {
		return
	}
	key := types.NamespacedName{Namespace: kwf.Namespace, Name: kwf.Name}
	id := scheduleKey(key)
	entry, found := s.entries[id]
	if !found {
		s.entries[id] = &scheduledWorkflow{
			key:      key,
			workflow: wf,
			interval: wf.ReconcileInterval,
			nextRunAt: computeNextRunFromStatus(
				now,
				kwf.Status.LastRan.Time,
				!kwf.Status.LastRan.IsZero(),
				wf.ReconcileInterval,
			),
		}
		return
	}

	entry.workflow = wf
	entry.interval = wf.ReconcileInterval
	if !entry.running && !entry.queued {
		entry.nextRunAt = computeNextRunFromStatus(
			now,
			kwf.Status.LastRan.Time,
			!kwf.Status.LastRan.IsZero(),
			wf.ReconcileInterval,
		)
	}
}

func (s *controllerScheduler) replaceFromWorkflows(now time.Time, workflows []*blackstart.Workflow) {
	s.mu.Lock()
	defer s.mu.Unlock()

	present := map[string]struct{}{}
	for _, wf := range workflows {
		kwf, ok := wf.Source.(*v1alpha1.Workflow)
		if !ok {
			continue
		}
		id := scheduleKey(types.NamespacedName{Namespace: kwf.Namespace, Name: kwf.Name})
		present[id] = struct{}{}
		s.upsert(now, wf)
	}

	for id := range s.entries {
		if _, ok := present[id]; !ok && !s.entries[id].running && !s.entries[id].queued {
			delete(s.entries, id)
		}
	}
}

func (s *controllerScheduler) dueWorkflows(now time.Time) []scheduledWorkflowRun {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]scheduledWorkflowRun, 0)
	for _, entry := range s.entries {
		if entry.running || entry.queued {
			continue
		}
		if !entry.nextRunAt.After(now) {
			entry.queued = true
			entry.queuedAt = now
			out = append(
				out,
				scheduledWorkflowRun{
					entry:    entry,
					key:      entry.key,
					workflow: entry.workflow,
					queuedAt: entry.queuedAt,
				},
			)
		}
	}
	return out
}

func (s *controllerScheduler) markRunning(entry *scheduledWorkflow) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.running = true
	entry.queued = false
}

func (s *controllerScheduler) markDone(entry *scheduledWorkflow, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.running = false
	entry.queued = false
	entry.nextRunAt = now.Add(entry.interval)
}

func (s *controllerScheduler) markQueueFull(entry *scheduledWorkflow, now time.Time, retryDelay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.queued = false
	entry.nextRunAt = now.Add(retryDelay)
}

func (s *controllerScheduler) queueHealth(now time.Time) queueHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	health := queueHealth{}
	for _, entry := range s.entries {
		if !entry.queued {
			continue
		}
		health.queuedCount++
		if entry.interval > 0 && now.Sub(entry.queuedAt) >= entry.interval {
			health.missedIntervalKeys = append(health.missedIntervalKeys, entry.key.String())
		}
	}
	return health
}

func parseControllerRuntimeOptions(config *blackstart.RuntimeConfig) (controllerRuntimeOptions, error) {
	if config.MaxParallelReconciliations <= 0 {
		return controllerRuntimeOptions{}, fmt.Errorf("max parallel reconciliations must be greater than 0")
	}
	resyncInterval, err := time.ParseDuration(strings.TrimSpace(config.ControllerResyncInterval))
	if err != nil {
		return controllerRuntimeOptions{}, fmt.Errorf(
			"invalid controller resync interval %q: expected duration like \"15s\" or \"1m\": %w",
			config.ControllerResyncInterval,
			err,
		)
	}
	if resyncInterval <= 0 {
		return controllerRuntimeOptions{}, fmt.Errorf(
			"invalid controller resync interval %q: must be greater than 0",
			config.ControllerResyncInterval,
		)
	}
	queueWarnThreshold, err := time.ParseDuration(strings.TrimSpace(config.QueueWaitWarningThreshold))
	if err != nil {
		return controllerRuntimeOptions{}, fmt.Errorf(
			"invalid queue wait warning threshold %q: expected duration like \"30s\" or \"1m\": %w",
			config.QueueWaitWarningThreshold,
			err,
		)
	}
	if queueWarnThreshold < 0 {
		return controllerRuntimeOptions{}, fmt.Errorf(
			"invalid queue wait warning threshold %q: must be greater than or equal to 0",
			config.QueueWaitWarningThreshold,
		)
	}
	return controllerRuntimeOptions{
		MaxParallel:        config.MaxParallelReconciliations,
		ResyncInterval:     resyncInterval,
		QueueWarnThreshold: queueWarnThreshold,
	}, nil
}

func listWorkflowsForNamespaces(ctx context.Context, c client.Client, namespaces []string) ([]*blackstart.Workflow, error) {
	workflows := make([]*blackstart.Workflow, 0)
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		loaded, err := loadWorkflowsFromK8s(ctx, c, ns)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, loaded...)
	}
	return workflows, nil
}

func parseNamespaces(config *blackstart.RuntimeConfig) []string {
	raw := strings.TrimSpace(config.KubeNamespace)
	if raw == "" {
		return []string{""}
	}
	parts := strings.Split(raw, ",")
	namespaces := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		ns := strings.TrimSpace(part)
		if ns == "" {
			// Ignore empty segments (for example, accidental trailing comma).
			continue
		}
		if _, ok := seen[ns]; ok {
			continue
		}
		seen[ns] = struct{}{}
		namespaces = append(namespaces, ns)
	}
	if len(namespaces) == 0 {
		return []string{""}
	}
	return namespaces
}

func loadWorkflowByKey(ctx context.Context, c client.Client, key types.NamespacedName) (*blackstart.Workflow, error) {
	var kwf v1alpha1.Workflow
	if err := c.Get(ctx, key, &kwf); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return workflowFromK8sResource(kwf.DeepCopy())
}

func triggerRefresh(refreshCh chan struct{}) {
	select {
	case refreshCh <- struct{}{}:
	default:
	}
}

func watchWorkflowsInNamespace(
	ctx context.Context,
	watchClient client.WithWatch,
	namespace string,
	logger *slog.Logger,
	refreshCh chan struct{},
) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		watcher, err := watchClient.Watch(ctx, &v1alpha1.WorkflowList{}, client.InNamespace(namespace))
		if err != nil {
			logger.Warn("failed to watch workflows; retrying", "namespace", namespace, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		for {
			select {
			case <-ctx.Done():
				watcher.Stop()
				return
			case evt, ok := <-watcher.ResultChan():
				if !ok {
					watcher.Stop()
					goto restartWatch
				}
				switch evt.Type {
				case k8swatch.Added, k8swatch.Modified, k8swatch.Deleted:
					triggerRefresh(refreshCh)
				}
			}
		}
	restartWatch:
		logger.Warn("workflow watch closed; restarting", "namespace", namespace)
	}
}

func startWorkflowWatches(
	ctx context.Context,
	c client.Client,
	namespaces []string,
	logger *slog.Logger,
	refreshCh chan struct{},
) {
	watchClient, ok := c.(client.WithWatch)
	if !ok {
		logger.Warn("watch-capable kubernetes client unavailable; controller will rely on periodic resync")
		return
	}
	for _, ns := range namespaces {
		ns = strings.TrimSpace(ns)
		go watchWorkflowsInNamespace(ctx, watchClient, ns, logger, refreshCh)
	}
}

func runWorkflowsControllerInK8s(ctx context.Context, kubeClient client.Client) error {
	logger := loggerFromCtx(ctx)
	config := configFromCtx(ctx)

	opts, err := parseControllerRuntimeOptions(config)
	if err != nil {
		return err
	}
	namespaces := parseNamespaces(config)
	scheduler := newControllerScheduler()
	var activeKeys sync.Map // key(namespace/name) currently queued or running

	queue := make(chan scheduledWorkflowRun, opts.MaxParallel*4)

	workflows, err := listWorkflowsForNamespaces(ctx, kubeClient, namespaces)
	if err != nil {
		return fmt.Errorf("error loading workflows from Kubernetes: %w", err)
	}
	scheduler.replaceFromWorkflows(time.Now(), workflows)
	refreshCh := make(chan struct{}, 1)
	startWorkflowWatches(ctx, kubeClient, namespaces, logger, refreshCh)

	for i := 0; i < opts.MaxParallel; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case runItem := <-queue:
					runKey := scheduleKey(runItem.key)
					releaseActive := func() {
						activeKeys.Delete(runKey)
					}
					wait := time.Since(runItem.queuedAt)
					if wait > opts.QueueWarnThreshold {
						logger.Warn(
							"workflow waited for available reconciliation slot",
							"workflow",
							runItem.key.String(),
							"wait",
							wait.String(),
						)
					}
					scheduler.markRunning(runItem.entry)
					// Ensure the active key guard is always released even on refresh/get errors.
					currentWorkflow, wfErr := loadWorkflowByKey(ctx, kubeClient, runItem.key)
					if wfErr != nil {
						logger.Warn(
							"failed to refresh workflow before reconciliation",
							"workflow",
							runItem.key.String(),
							"error",
							wfErr,
						)
						scheduler.markDone(runItem.entry, time.Now())
						releaseActive()
						continue
					}
					if currentWorkflow == nil {
						logger.Info(
							"workflow no longer exists; skipping queued reconciliation",
							"workflow",
							runItem.key.String(),
						)
						scheduler.markDone(runItem.entry, time.Now())
						releaseActive()
						continue
					}
					if runErr := runWorkflowInK8s(ctx, kubeClient, currentWorkflow); runErr != nil {
						logger.Warn("workflow reconciliation failed", "workflow", runItem.key.String(), "error", runErr)
					}
					scheduler.markDone(runItem.entry, time.Now())
					releaseActive()
				}
			}
		}()
	}

	dispatchTicker := time.NewTicker(controllerDispatchInterval)
	defer dispatchTicker.Stop()
	resyncTicker := time.NewTicker(opts.ResyncInterval)
	defer resyncTicker.Stop()
	refreshFromCluster := func() {
		loaded, loadErr := listWorkflowsForNamespaces(ctx, kubeClient, namespaces)
		if loadErr != nil {
			logger.Error("error refreshing workflows from kubernetes", "error", loadErr)
			return
		}
		scheduler.replaceFromWorkflows(time.Now(), loaded)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-dispatchTicker.C:
			due := scheduler.dueWorkflows(time.Now())
			for _, entry := range due {
				runKey := scheduleKey(entry.key)
				if _, loaded := activeKeys.LoadOrStore(runKey, struct{}{}); loaded {
					// Already queued/running for this workflow key.
					scheduler.markQueueFull(entry.entry, time.Now(), controllerQueueRetryDelay)
					continue
				}
				select {
				case queue <- entry:
					if len(queue) > opts.MaxParallel {
						logger.Warn(
							"reconciliation backlog is growing",
							"queued",
							len(queue),
							"maxParallel",
							opts.MaxParallel,
						)
					}
				default:
					logger.Warn("reconciliation queue is full; workflow will retry next cycle", "workflow", entry.key.String())
					scheduler.markQueueFull(entry.entry, time.Now(), controllerQueueRetryDelay)
					activeKeys.Delete(runKey)
				}
			}
			health := scheduler.queueHealth(time.Now())
			if health.queuedCount > opts.MaxParallel && len(health.missedIntervalKeys) > 0 {
				logger.Error(
					"reconciliation backlog exceeded max parallel and is missing scheduled intervals",
					"queued",
					health.queuedCount,
					"maxParallel",
					opts.MaxParallel,
					"missedWorkflows",
					strings.Join(health.missedIntervalKeys, ","),
				)
			}
		case <-refreshCh:
			refreshFromCluster()
		case <-resyncTicker.C:
			refreshFromCluster()
		}
	}
}
