package main

import (
	"context"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
)

// TestRunInK8sMode sets up a fake Kubernetes client that is used to simulate loading Workflows
// from a Kubernetes cluster. It then runs the main application logic to ensure it can load,
// execute, and update a Workflow.
//
// Currently, this only loads the scheme for v1alpha1, since that is the only version of the
// Blackstart CRDs that exist. If updated CRD versions are added, this test should be updated to
// include those schemes as well.
func TestRunInK8sMode(t *testing.T) {
	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	require.NoError(t, err)

	workflow := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workflow",
			Namespace: "default",
		},
		Spec: v1alpha1.WorkflowSpec{
			Operations: []v1alpha1.Operation{}, // No steps needed for this test
		},
	}

	// setup fake client with status subresource support
	fakeClient := newFakeClientWithStatus(scheme, workflow)

	restore := patchEnv(t, blackstart.K8sNamespaceEnv, "default")
	defer restore()

	config, err := blackstart.ReadConfig()
	require.NoError(t, err, "ReadConfig() should not return an error")

	ctx := context.Background()
	logger := blackstart.NewLogger(config)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, logger)
	ctx = context.WithValue(ctx, blackstart.ConfigKey, config)

	// Replace the updateWorkflowStatusInK8s function with the mock
	originalUpdateWorkflowStatus := updateWorkflowStatusFunc
	updateWorkflowStatusFunc = mockUpdateWorkflowStatus
	defer func() { updateWorkflowStatusFunc = originalUpdateWorkflowStatus }()

	// main logic with the injected fake client
	err = run(ctx, fakeClient)
	require.NoError(t, err, "run() should execute without error in the fake k8s environment")
}

// TestLoadWorkflowsFromK8sSuite tests the loadWorkflowsFromK8s function with various scenarios,
// including loading multiple valid Workflows and handling invalid Workflows. It uses a fake
// Kubernetes client to simulate the presence of Workflows in a cluster.
func TestLoadWorkflowsFromK8sSuite(t *testing.T) {
	// Create some test workflows

	wf1 := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workflow-1",
			Namespace: "default",
		},
		Spec: v1alpha1.WorkflowSpec{
			Operations: []v1alpha1.Operation{},
		},
	}

	wf2 := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workflow-2",
			Namespace: "default",
		},
		Spec: v1alpha1.WorkflowSpec{
			Operations: []v1alpha1.Operation{},
		},
	}

	wfValid := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "valid-workflow",
			Namespace: "default",
		},
		Spec: v1alpha1.WorkflowSpec{
			Operations: []v1alpha1.Operation{},
		},
	}

	// This object is invalid because it's missing the Spec
	wfInvalid := &v1alpha1.Workflow{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "blackstart.pezops.github.io/v1alpha1",
			Kind:       "Workflow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-workflow",
			Namespace: "default",
		},
	}

	tests := []struct {
		name          string
		objects       []client.Object
		expectedLen   int
		expectedNames []string
	}{
		{
			name:          "load multiple valid workflows",
			objects:       []client.Object{wf1, wf2},
			expectedLen:   2,
			expectedNames: []string{"workflow-1", "workflow-2"},
		},
		{
			name:          "load one valid and one invalid workflow",
			objects:       []client.Object{wfValid, wfInvalid},
			expectedLen:   2, // both are loaded, but the invalid one will have no operations
			expectedNames: []string{"valid-workflow", "invalid-workflow"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				scheme := runtime.NewScheme()
				err := v1alpha1.AddToScheme(scheme)
				require.NoError(t, err)

				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.objects...).Build()

				// Capture stderr so we can log it if needed
				var oldStderr *os.File
				var r, w *os.File

				var kubeWorkflows []*blackstart.Workflow
				kubeWorkflows, err = loadWorkflowsFromK8s(context.Background(), fakeClient, "default")

				// Restore stderr, read and log the error output
				if oldStderr != nil {
					err = w.Close()
					if err != nil {
						t.Log("failed to close stderr capture:", err.Error())
					}
					os.Stderr = oldStderr
					stderr, _ := io.ReadAll(r)
					if len(stderr) > 0 {
						t.Log(string(stderr))
					}
				}

				assert.NoError(t, err)
				assert.Len(t, kubeWorkflows, tt.expectedLen)

				if len(tt.expectedNames) > 0 {
					names := make([]string, len(kubeWorkflows))
					for i, wf := range kubeWorkflows {
						names[i] = wf.Name
					}
					sort.Strings(names)
					sort.Strings(tt.expectedNames)
					assert.Equal(t, tt.expectedNames, names)
				}
			},
		)
	}
}
