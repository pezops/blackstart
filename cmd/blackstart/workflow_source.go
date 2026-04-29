package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	gcsapi "google.golang.org/api/storage/v1"
	"gopkg.in/yaml.v3"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/api/v1alpha1"
)

type workflowSourceReader struct {
	readFile func(path string) ([]byte, error)
	getEnv   func(name string) string
	readGCS  func(ctx context.Context, bucket, object string) ([]byte, error)
}

var defaultWorkflowSourceReader = workflowSourceReader{
	readFile: os.ReadFile,
	getEnv:   os.Getenv,
	readGCS:  readWorkflowFromGCS,
}

// loadWorkflowFromSource selects a workflow source loader based on
// BLACKSTART_WORKFLOW_FILE and returns a parsed core workflow.
func loadWorkflowFromSource(ctx context.Context) (*blackstart.Workflow, error) {
	config := configFromCtx(ctx)
	spec := strings.TrimSpace(config.WorkflowFile)
	if spec == "" {
		return nil, fmt.Errorf("workflow file source is empty")
	}
	switch {
	case strings.HasPrefix(spec, "env:"):
		return loadWorkflowFromEnv(ctx)
	case strings.HasPrefix(spec, "gs://"):
		return loadWorkflowFromGCS(ctx)
	default:
		return loadWorkflowFromFile(ctx)
	}
}

// workflowConfigBytesFromEnv reads raw workflow YAML bytes from the provided
// environment variable name.
func workflowConfigBytesFromEnv(name string) ([]byte, error) {
	if name == "" {
		return nil, fmt.Errorf("missing environment variable name")
	}
	value := defaultWorkflowSourceReader.getEnv(name)
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("workflow content environment variable %q is empty or not set", name)
	}
	return []byte(value), nil
}

// workflowConfigBytesFromGCS reads raw workflow YAML bytes from a GCS object.
func workflowConfigBytesFromGCS(ctx context.Context, bucket, object string) ([]byte, error) {
	data, err := defaultWorkflowSourceReader.readGCS(ctx, bucket, object)
	if err != nil {
		return nil, fmt.Errorf(
			"error reading workflow file from GCS bucket %q object %q: %w",
			bucket,
			object,
			err,
		)
	}
	return data, nil
}

// workflowFromConfigBytes unmarshals workflow configuration YAML and converts it
// into a core blackstart.Workflow.
func workflowFromConfigBytes(workflowConfig []byte) (*blackstart.Workflow, error) {
	var apiWf v1alpha1.WorkflowConfigFile
	err := yaml.Unmarshal(workflowConfig, &apiWf)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling workflow: %w", err)
	}

	var wf blackstart.Workflow
	wf.Name = apiWf.Name
	wf.Description = apiWf.Description
	wf.ReconcileInterval, err = parseReconcileInterval(apiWf.ReconcileInterval)
	if err != nil {
		return nil, fmt.Errorf("error parsing reconcile interval for workflow %s: %w", wf.Name, err)
	}
	wf.Operations, err = loadOperations(apiWf.Operations)
	if err != nil {
		return nil, fmt.Errorf("error loading operations for workflow %s: %w", wf.Name, err)
	}
	wf.Source = apiWf
	return &wf, nil
}

// parseGCSWorkflowSource parses a gs://<bucket>/<object> workflow source value.
func parseGCSWorkflowSource(spec string) (bucket, object string, err error) {
	trimmed := strings.TrimSpace(spec)
	if !strings.HasPrefix(trimmed, "gs://") {
		return "", "", fmt.Errorf("invalid GCS workflow source %q: expected gs:// prefix", spec)
	}
	withoutScheme := strings.TrimPrefix(trimmed, "gs://")
	parts := strings.SplitN(withoutScheme, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf(
			"invalid GCS workflow source %q: expected gs://<bucket>/<object>",
			spec,
		)
	}
	return parts[0], parts[1], nil
}

// readWorkflowFromGCS fetches an object body from GCS using the Storage JSON API.
func readWorkflowFromGCS(ctx context.Context, bucket, object string) ([]byte, error) {
	svc, err := gcsapi.NewService(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := svc.Objects.Get(bucket, object).Download()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return io.ReadAll(resp.Body)
}
