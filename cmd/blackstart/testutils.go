package main

import (
	"context"
	"os"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
)

// Define a variable to hold the updateWorkflowStatusInK8s function. In tests, we will mock the status update process.
var updateWorkflowStatusFunc = updateWorkflowStatusInK8s

// fakeClientWithStatus extend the fake client to support the status subresource. This allows the controller-runtime
// fake client to be used in tests that involve status updates.
type fakeClientWithStatus struct {
	client.Client
	statusStore sync.Map
}

// Status returns a fake SubResourceWriter for the status subresource.
func (f *fakeClientWithStatus) Status() client.SubResourceWriter {
	return &fakeStatusWriter{parent: f}
}

// fakeStatusWriter is a mock that implements client.SubResourceWriter to handle status updates.
type fakeStatusWriter struct {
	parent *fakeClientWithStatus
}

// Update simulates updating the status subresource by storing the status in a map.
func (f *fakeStatusWriter) Update(
	ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption,
) error {
	// Store the status in the statusStore map
	f.parent.statusStore.Store(obj.GetName(), obj.DeepCopyObject())
	return nil
}

// Patch is not implemented in this fake resource for simplicity.
func (f *fakeStatusWriter) Patch(
	ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption,
) error {
	return nil
}

// Create is not implemented in this fake resource for simplicity.
func (f *fakeStatusWriter) Create(
	ctx context.Context, obj client.Object, subResource client.Object, opts ...client.SubResourceCreateOption,
) error {
	return nil
}

// newFakeClientWithStatus returns a fake runtime-controller client that is used for mocking and supports updating
// the status subresource. This is the primary way to create a fake client for tests in Blackstart.
func newFakeClientWithStatus(scheme *runtime.Scheme, objs ...client.Object) client.Client {
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &fakeClientWithStatus{
		Client:      baseClient,
		statusStore: sync.Map{},
	}
}

// mockUpdateWorkflowStatus is a mock of the updateWorkflowStatusInK8s function to bypass the fake client, which can't
// handle status updates.
func mockUpdateWorkflowStatus(
	ctx context.Context, _ client.Client, wf *blackstart.Workflow, status v1alpha1.WorkflowStatus,
) error {
	// Log the status update for verification
	logger := loggerFromCtx(ctx)
	logger.Info("Mock updateWorkflowStatusInK8s called", "workflow", wf.Name, "status", status)
	return nil
}

// patchEnv sets an environment variable to a new value for the duration of a test. It returns a deferrable function
// that restores the original value.
func patchEnv(t *testing.T, key string, value string) (restore func()) {
	original, exists := os.LookupEnv(key)
	err := os.Setenv(key, value)
	if err != nil {
		t.Fatalf("failed to set env var: %v", err)
	}

	restore = func() {
		if !exists {
			// If the original variable didn't exist, remove it.
			err = os.Unsetenv(key)
			if err != nil {
				t.Fatalf("failed to unset env var: %v", err)
			}
			return
		}
		err = os.Setenv(key, original)
		if err != nil {
			t.Fatalf("failed to restore env var: %v", err)
		}
	}

	return
}
