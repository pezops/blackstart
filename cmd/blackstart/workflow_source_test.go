package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

func TestLoadWorkflowFromSource_LocalFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow.yaml")
	content := []byte("name: from-file\noperations: []\n")
	require.NoError(t, os.WriteFile(path, content, 0o600))

	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: path}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	wf, err := loadWorkflowFromSource(ctx)
	require.NoError(t, err)
	assert.Equal(t, "from-file", wf.Name)
}

func TestLoadWorkflowFromSource_Env(t *testing.T) {
	original := defaultWorkflowSourceReader
	t.Cleanup(func() { defaultWorkflowSourceReader = original })

	defaultWorkflowSourceReader = workflowSourceReader{
		readFile: original.readFile,
		getEnv: func(name string) string {
			if name == "BLACKSTART_WORKFLOW_YAML" {
				return "name: from-env\noperations: []\n"
			}
			return ""
		},
		readGCS: original.readGCS,
	}

	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: "env:BLACKSTART_WORKFLOW_YAML"}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	wf, err := loadWorkflowFromSource(ctx)
	require.NoError(t, err)
	assert.Equal(t, "from-env", wf.Name)
}

func TestLoadWorkflowFromSource_GCS(t *testing.T) {
	original := defaultWorkflowSourceReader
	t.Cleanup(func() { defaultWorkflowSourceReader = original })

	defaultWorkflowSourceReader = workflowSourceReader{
		readFile: original.readFile,
		getEnv:   original.getEnv,
		readGCS: func(_ context.Context, bucket, object string) ([]byte, error) {
			assert.Equal(t, "my-bucket", bucket)
			assert.Equal(t, "path/to/workflow.yaml", object)
			return []byte("name: from-gcs\noperations: []\n"), nil
		},
	}

	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: "gs://my-bucket/path/to/workflow.yaml"}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	wf, err := loadWorkflowFromSource(ctx)
	require.NoError(t, err)
	assert.Equal(t, "from-gcs", wf.Name)
}

func TestLoadWorkflowFromEnv_MissingVarName(t *testing.T) {
	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: "env:"}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	_, err := loadWorkflowFromEnv(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing environment variable name")
}

func TestLoadWorkflowFromEnv_EmptyValue(t *testing.T) {
	original := defaultWorkflowSourceReader
	t.Cleanup(func() { defaultWorkflowSourceReader = original })

	defaultWorkflowSourceReader = workflowSourceReader{
		readFile: original.readFile,
		getEnv: func(_ string) string {
			return ""
		},
		readGCS: original.readGCS,
	}

	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: "env:BLACKSTART_WORKFLOW_YAML"}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	_, err := loadWorkflowFromEnv(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is empty or not set")
}

func TestLoadWorkflowFromGCS_ReadError(t *testing.T) {
	original := defaultWorkflowSourceReader
	t.Cleanup(func() { defaultWorkflowSourceReader = original })

	defaultWorkflowSourceReader = workflowSourceReader{
		readFile: original.readFile,
		getEnv:   original.getEnv,
		readGCS: func(_ context.Context, _, _ string) ([]byte, error) {
			return nil, errors.New("access denied")
		},
	}

	ctx := context.Background()
	cfg := &blackstart.RuntimeConfig{WorkflowFile: "gs://my-bucket/path/to/workflow.yaml"}
	ctx = context.WithValue(ctx, blackstart.ConfigKey, cfg)
	ctx = context.WithValue(ctx, blackstart.LoggerKey, blackstart.NewLogger(nil))

	_, err := loadWorkflowFromGCS(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "error reading workflow file from GCS")
}

func TestParseGCSWorkflowSource(t *testing.T) {
	bucket, object, err := parseGCSWorkflowSource("gs://bucket/path/to/file.yaml")
	require.NoError(t, err)
	assert.Equal(t, "bucket", bucket)
	assert.Equal(t, "path/to/file.yaml", object)
}

func TestParseGCSWorkflowSource_Invalid(t *testing.T) {
	tests := []string{
		"gs://",
		"gs://bucket-only",
		"gs:///missing-bucket",
		"http://example.com/workflow.yaml",
	}
	for _, tc := range tests {
		t.Run(tc, func(t *testing.T) {
			_, _, err := parseGCSWorkflowSource(tc)
			require.Error(t, err)
		})
	}
}
