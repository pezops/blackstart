package cloud

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

type fakeMetadataReader struct {
	values map[string]string
}

var errMetadataNotFound = errors.New("metadata key not found")

func (f fakeMetadataReader) raw(path string) (string, error) {
	value, ok := f.values[path]
	if !ok {
		return "", errMetadataNotFound
	}
	return value, nil
}

func (f fakeMetadataReader) projectID(_ context.Context) (string, error) {
	return f.raw("project/project-id")
}

func (f fakeMetadataReader) projectNumber(_ context.Context) (string, error) {
	return f.raw("project/numeric-project-id")
}

func (f fakeMetadataReader) instanceID(_ context.Context) (string, error) {
	return f.raw("instance/id")
}

func (f fakeMetadataReader) instanceName(_ context.Context) (string, error) {
	return f.raw("instance/name")
}

func (f fakeMetadataReader) hostname(_ context.Context) (string, error) {
	return f.raw("instance/hostname")
}

func (f fakeMetadataReader) zone(_ context.Context) (string, error) {
	return f.raw("instance/zone")
}

func (f fakeMetadataReader) instanceTags(_ context.Context) ([]string, error) {
	raw, err := f.raw("instance/tags")
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	return []string{"tag-a", "tag-b"}, nil
}

func (f fakeMetadataReader) get(_ context.Context, path string) (string, error) {
	return f.raw(path)
}

func fullReader() fakeMetadataReader {
	return fakeMetadataReader{
		values: map[string]string{
			"project/project-id":         "test-project",
			"project/numeric-project-id": "1234567890",
			"instance/id":                "987654321",
			"instance/name":              "gke-node-1",
			"instance/hostname":          "gke-node-1.c.test-project.internal",
			"instance/cpu-platform":      "Intel Ice Lake",
			"instance/image":             "projects/debian-cloud/global/images/debian-12-bookworm-v20250311",
			"instance/machine-type":      "projects/1234567890/machineTypes/e2-standard-4",
			"instance/preempted":         "FALSE",
			"instance/tags":              "tag-a\ntag-b\n",
			"instance/maintenance-event": "NONE",
			"instance/zone":              "projects/1234567890/zones/us-central1-a",
		},
	}
}

func TestCollectMetadata(t *testing.T) {
	ctx := blackstart.InputsToContext(context.Background(), map[string]blackstart.Input{})

	out, err := collectMetadataWithReader(ctx, fullReader(), nil)
	require.NoError(t, err)
	require.Equal(t, "test-project", out.projectID)
	require.Equal(t, "1234567890", out.projectNumber)
	require.Equal(t, "987654321", out.instanceID)
	require.Equal(t, "gke-node-1", out.instanceName)
	require.Equal(t, "gke-node-1.c.test-project.internal", out.hostname)
	require.Equal(t, "Intel Ice Lake", out.cpuPlatform)
	require.Equal(t, "projects/debian-cloud/global/images/debian-12-bookworm-v20250311", out.image)
	require.Equal(t, "e2-standard-4", out.machineType)
	require.False(t, out.preempted)
	require.Equal(t, []string{"tag-a", "tag-b"}, out.tags)
	require.Equal(t, "NONE", out.maintenance)
	require.Equal(t, "us-central1-a", out.zone)
	require.Equal(t, "us-central1", out.region)
}

func TestCollectMetadata_SelectedOutputsOnly(t *testing.T) {
	ctx := blackstart.InputsToContext(context.Background(), map[string]blackstart.Input{})
	reader := fakeMetadataReader{
		values: map[string]string{
			"project/project-id": "test-project",
			"instance/zone":      "projects/1234567890/zones/us-central1-a",
		},
	}
	requested := map[string]struct{}{
		outputProjectID: {},
		outputRegion:    {},
	}

	out, err := collectMetadataWithReader(ctx, reader, requested)
	require.NoError(t, err)
	require.Equal(t, "test-project", out.projectID)
	require.Equal(t, "us-central1", out.region)
	require.Equal(t, "us-central1-a", out.zone)
	require.Empty(t, out.instanceName)
}

func TestNormalizeRequestedOutputs_Default(t *testing.T) {
	requested, err := normalizeRequestedOutputs(nil)
	require.NoError(t, err)
	_, hasProject := requested[outputProjectID]
	_, hasRegion := requested[outputRegion]
	require.True(t, hasProject)
	require.True(t, hasRegion)
}

func TestCollectMetadata_ErrorWhenMissingRequiredRequestedKey(t *testing.T) {
	ctx := blackstart.InputsToContext(context.Background(), map[string]blackstart.Input{})
	reader := fakeMetadataReader{
		values: map[string]string{
			"project/project-id": "test-project",
		},
	}
	requested := map[string]struct{}{
		outputInstanceID: {},
	}

	_, err := collectMetadataWithReader(ctx, reader, requested)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed reading instance ID")
}

func TestRequestedOutputsValidation(t *testing.T) {
	_, err := normalizeRequestedOutputs([]string{"project_id", "region"})
	require.NoError(t, err)

	_, err = normalizeRequestedOutputs([]string{"unknown_field"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown request")
}

func TestMetadataModule_Validate(t *testing.T) {
	module := NewCloudMetadata()

	validErr := module.Validate(
		blackstart.Operation{
			Id:     "metadata-valid",
			Module: "google_cloud_metadata",
			Inputs: map[string]blackstart.Input{
				inputRequests: blackstart.NewInputFromValue([]string{"project_id", "zone"}),
			},
		},
	)
	require.NoError(t, validErr)

	invalidErr := module.Validate(
		blackstart.Operation{
			Id:     "metadata-invalid",
			Module: "google_cloud_metadata",
			Inputs: map[string]blackstart.Input{
				inputRequests: blackstart.NewInputFromValue("nope"),
			},
		},
	)
	require.Error(t, invalidErr)
	require.Contains(t, invalidErr.Error(), "invalid requests")
}

func TestMetadataModule_CheckDoesNotExistUnsupported(t *testing.T) {
	module := NewCloudMetadata()
	ctx := blackstart.InputsToContext(
		context.Background(),
		map[string]blackstart.Input{},
		blackstart.DoesNotExistFlag,
	)

	ok, err := module.Check(ctx)
	require.Error(t, err)
	require.False(t, ok)
	require.Contains(t, err.Error(), "doesNotExist")
}

func TestZoneAndRegionHelpers(t *testing.T) {
	require.Equal(t, "us-central1-b", zoneFromPath("projects/123/zones/us-central1-b"))
	require.Equal(t, "us-central1", regionFromZone("us-central1-b"))
	require.Equal(t, "unknown", regionFromZone("unknown"))
	require.Equal(
		t,
		"e2-standard-4",
		machineTypeFromPath("projects/1234567890/machineTypes/e2-standard-4"),
	)
}
