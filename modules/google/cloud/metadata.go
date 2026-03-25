package cloud

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	computemetadata "cloud.google.com/go/compute/metadata"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

const (
	inputRequests = "requests"

	outputProjectID     = "project_id"
	outputProjectNumber = "project_number"
	outputInstanceID    = "instance_id"
	outputInstanceName  = "instance_name"
	outputHostname      = "hostname"
	outputCPUPlatform   = "cpu_platform"
	outputImage         = "image"
	outputMachineType   = "machine_type"
	outputPreempted     = "preempted"
	outputTags          = "tags"
	outputMaintenance   = "maintenance_event"
	outputZone          = "zone"
	outputRegion        = "region"
)

var validMetadataOutputs = map[string]struct{}{
	outputProjectID:     {},
	outputProjectNumber: {},
	outputInstanceID:    {},
	outputInstanceName:  {},
	outputHostname:      {},
	outputCPUPlatform:   {},
	outputImage:         {},
	outputMachineType:   {},
	outputPreempted:     {},
	outputTags:          {},
	outputMaintenance:   {},
	outputZone:          {},
	outputRegion:        {},
}

// metadataReader defines metadata fetch operations used by the metadata module.
type metadataReader interface {
	projectID(ctx context.Context) (string, error)
	projectNumber(ctx context.Context) (string, error)
	instanceID(ctx context.Context) (string, error)
	instanceName(ctx context.Context) (string, error)
	hostname(ctx context.Context) (string, error)
	zone(ctx context.Context) (string, error)
	instanceTags(ctx context.Context) ([]string, error)
	get(ctx context.Context, path string) (string, error)
}

// gcpMetadataReader reads metadata from the Google Compute metadata service client.
type gcpMetadataReader struct{}

// projectID returns the current project ID.
func (gcpMetadataReader) projectID(ctx context.Context) (string, error) {
	return computemetadata.ProjectIDWithContext(ctx)
}

// projectNumber returns the current numeric project ID.
func (gcpMetadataReader) projectNumber(ctx context.Context) (string, error) {
	return computemetadata.NumericProjectIDWithContext(ctx)
}

// instanceID returns the current instance numeric ID.
func (gcpMetadataReader) instanceID(ctx context.Context) (string, error) {
	return computemetadata.InstanceIDWithContext(ctx)
}

// instanceName returns the current instance name.
func (gcpMetadataReader) instanceName(ctx context.Context) (string, error) {
	return computemetadata.InstanceNameWithContext(ctx)
}

// hostname returns the current instance hostname.
func (gcpMetadataReader) hostname(ctx context.Context) (string, error) {
	return computemetadata.HostnameWithContext(ctx)
}

// zone returns the current instance zone.
func (gcpMetadataReader) zone(ctx context.Context) (string, error) {
	return computemetadata.ZoneWithContext(ctx)
}

// instanceTags returns the network tags set on the current instance.
func (gcpMetadataReader) instanceTags(ctx context.Context) ([]string, error) {
	return computemetadata.InstanceTagsWithContext(ctx)
}

// get reads an arbitrary metadata path from the metadata service.
func (gcpMetadataReader) get(ctx context.Context, path string) (string, error) {
	return computemetadata.GetWithContext(ctx, path)
}

var defaultMetadataReader metadataReader = gcpMetadataReader{}

// metadataOutput is the collected metadata payload written to module outputs.
type metadataOutput struct {
	projectID     string
	projectNumber string
	instanceID    string
	instanceName  string
	hostname      string
	cpuPlatform   string
	image         string
	machineType   string
	preempted     bool
	tags          []string
	maintenance   string
	zone          string
	region        string
}

func init() {
	blackstart.RegisterModule("google_cloud_metadata", NewCloudMetadata)
}

var _ blackstart.Module = &cloudMetadata{}

// NewCloudMetadata creates a module that emits runtime metadata from the Google metadata service.
func NewCloudMetadata() blackstart.Module {
	return &cloudMetadata{}
}

// cloudMetadata implements the google_cloud_metadata module.
type cloudMetadata struct{}

// Info returns module metadata and supported outputs.
func (m *cloudMetadata) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "google_cloud_metadata",
		Name: "Google Cloud metadata",
		Description: util.CleanString(
			`
Retrieves runtime metadata from the Google Cloud metadata service and exposes it as outputs for
downstream operations.

This module is intended for workloads running on Google Cloud platforms that provide the metadata
service, such as GKE, GCE, and Cloud Run.
`,
		),
		Inputs: map[string]blackstart.InputValue{
			inputRequests: {
				Description: "Requested metadata fields to fetch. Valid values: `project_id`, `project_number`, `instance_id`, `instance_name`, `hostname`, `cpu_platform`, `image`, `machine_type`, `preempted`, `tags`, `maintenance_event`, `zone`, `region`. Accepts a string or list of strings.",
				Types: []reflect.Type{
					reflect.TypeFor[string](),
					reflect.TypeFor[[]string](),
				},
				Required: false,
				Default:  []string{outputProjectID, outputRegion},
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputProjectID: {
				Description: "Google Cloud project ID.",
				Type:        reflect.TypeFor[string](),
			},
			outputProjectNumber: {
				Description: "Google Cloud numeric project ID.",
				Type:        reflect.TypeFor[string](),
			},
			outputInstanceID: {
				Description: "Compute instance ID from metadata.",
				Type:        reflect.TypeFor[string](),
			},
			outputInstanceName: {
				Description: "Compute instance name from metadata.",
				Type:        reflect.TypeFor[string](),
			},
			outputHostname: {
				Description: "Instance hostname.",
				Type:        reflect.TypeFor[string](),
			},
			outputCPUPlatform: {
				Description: "CPU platform of the instance.",
				Type:        reflect.TypeFor[string](),
			},
			outputImage: {
				Description: "Image path used by the instance.",
				Type:        reflect.TypeFor[string](),
			},
			outputMachineType: {
				Description: "Machine type (for example `e2-standard-4`).",
				Type:        reflect.TypeFor[string](),
			},
			outputPreempted: {
				Description: "Whether the instance is preempted.",
				Type:        reflect.TypeFor[bool](),
			},
			outputTags: {
				Description: "Network tags attached to the instance.",
				Type:        reflect.TypeFor[[]string](),
			},
			outputMaintenance: {
				Description: "Current maintenance event state.",
				Type:        reflect.TypeFor[string](),
			},
			outputZone: {
				Description: "Compute zone (for example `us-central1-a`).",
				Type:        reflect.TypeFor[string](),
			},
			outputRegion: {
				Description: "Compute region derived from zone (for example `us-central1`).",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Read default metadata": `id: cloud_metadata
module: google_cloud_metadata`,
			"Read only project and region": `id: cloud_metadata
module: google_cloud_metadata
inputs:
  requests:
    - project_id
    - region`,
		},
	}
}

// Validate validates static operation inputs.
func (m *cloudMetadata) Validate(op blackstart.Operation) error {
	if input, ok := op.Inputs[inputRequests]; ok && input.IsStatic() {
		raw, err := blackstart.InputAs[[]string](input, false)
		if err != nil {
			return fmt.Errorf("invalid %s: %w", inputRequests, err)
		}
		if _, err = normalizeRequestedOutputs(raw); err != nil {
			return fmt.Errorf("invalid %s: %w", inputRequests, err)
		}
	}
	return nil
}

// Check verifies metadata service availability and publishes outputs when not tainted.
func (m *cloudMetadata) Check(ctx blackstart.ModuleContext) (bool, error) {
	if ctx.DoesNotExist() {
		return false, fmt.Errorf("doesNotExist is not supported by %s", m.Info().Id)
	}
	if ctx.Tainted() {
		return false, nil
	}
	requested, err := requestedOutputsFromContext(ctx)
	if err != nil {
		return false, err
	}
	data, err := collectMetadata(ctx, requested)
	if err != nil {
		return false, err
	}
	if err = writeMetadataOutputs(ctx, data, requested); err != nil {
		return false, err
	}
	return true, nil
}

// Set reads metadata values and publishes outputs.
func (m *cloudMetadata) Set(ctx blackstart.ModuleContext) error {
	if ctx.DoesNotExist() {
		return fmt.Errorf("doesNotExist is not supported by %s", m.Info().Id)
	}
	requested, err := requestedOutputsFromContext(ctx)
	if err != nil {
		return err
	}
	data, err := collectMetadata(ctx, requested)
	if err != nil {
		return err
	}
	return writeMetadataOutputs(ctx, data, requested)
}

// collectMetadata reads the supported project and instance metadata keys.
func collectMetadata(
	ctx blackstart.ModuleContext,
	requested map[string]struct{},
) (metadataOutput, error) {
	return collectMetadataWithReader(ctx, defaultMetadataReader, requested)
}

// collectMetadataWithReader reads metadata values using the provided reader.
func collectMetadataWithReader(
	ctx blackstart.ModuleContext,
	reader metadataReader,
	requested map[string]struct{},
) (metadataOutput, error) {
	var out metadataOutput
	var err error

	if wantsOutput(requested, outputProjectID) {
		out.projectID, err = reader.projectID(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading project ID: %w", err)
		}
	}
	if wantsOutput(requested, outputProjectNumber) {
		out.projectNumber, err = reader.projectNumber(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading project number: %w", err)
		}
	}
	if wantsOutput(requested, outputInstanceID) {
		out.instanceID, err = reader.instanceID(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading instance ID: %w", err)
		}
	}
	if wantsOutput(requested, outputInstanceName) {
		out.instanceName, err = reader.instanceName(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading instance name: %w", err)
		}
	}
	if wantsOutput(requested, outputHostname) {
		out.hostname, err = reader.hostname(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading hostname: %w", err)
		}
	}
	if wantsOutput(requested, outputCPUPlatform) {
		out.cpuPlatform, err = metadataGet(ctx, reader, "instance/cpu-platform")
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading cpu platform: %w", err)
		}
	}
	if wantsOutput(requested, outputImage) {
		out.image, err = metadataGet(ctx, reader, "instance/image")
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading image: %w", err)
		}
	}
	if wantsOutput(requested, outputMachineType) {
		machineTypePath, mtErr := metadataGet(ctx, reader, "instance/machine-type")
		if mtErr != nil {
			return metadataOutput{}, fmt.Errorf("failed reading machine type: %w", mtErr)
		}
		out.machineType = machineTypeFromPath(machineTypePath)
	}
	if wantsOutput(requested, outputPreempted) {
		preemptedRaw, preemptedErr := metadataGet(ctx, reader, "instance/preempted")
		if preemptedErr != nil {
			return metadataOutput{}, fmt.Errorf("failed reading preempted: %w", preemptedErr)
		}
		out.preempted = strings.EqualFold(strings.TrimSpace(preemptedRaw), "true")
	}
	if wantsOutput(requested, outputTags) {
		out.tags, err = reader.instanceTags(ctx)
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading tags: %w", err)
		}
	}
	if wantsOutput(requested, outputMaintenance) {
		out.maintenance, err = metadataGet(ctx, reader, "instance/maintenance-event")
		if err != nil {
			return metadataOutput{}, fmt.Errorf("failed reading maintenance event: %w", err)
		}
	}
	if wantsOutput(requested, outputZone) || wantsOutput(requested, outputRegion) {
		zonePath, zoneErr := reader.zone(ctx)
		if zoneErr != nil {
			return metadataOutput{}, fmt.Errorf("failed reading zone: %w", zoneErr)
		}
		out.zone = zoneFromPath(zonePath)
		out.region = regionFromZone(out.zone)
	}

	return out, nil
}

// writeMetadataOutputs writes collected metadata values to module outputs.
func writeMetadataOutputs(
	ctx blackstart.ModuleContext,
	out metadataOutput,
	requested map[string]struct{},
) error {
	_ = requested
	if err := ctx.Output(outputProjectID, out.projectID); err != nil {
		return err
	}
	if err := ctx.Output(outputProjectNumber, out.projectNumber); err != nil {
		return err
	}
	if err := ctx.Output(outputInstanceID, out.instanceID); err != nil {
		return err
	}
	if err := ctx.Output(outputInstanceName, out.instanceName); err != nil {
		return err
	}
	if err := ctx.Output(outputHostname, out.hostname); err != nil {
		return err
	}
	if err := ctx.Output(outputCPUPlatform, out.cpuPlatform); err != nil {
		return err
	}
	if err := ctx.Output(outputImage, out.image); err != nil {
		return err
	}
	if err := ctx.Output(outputMachineType, out.machineType); err != nil {
		return err
	}
	if err := ctx.Output(outputPreempted, out.preempted); err != nil {
		return err
	}
	if err := ctx.Output(outputTags, out.tags); err != nil {
		return err
	}
	if err := ctx.Output(outputMaintenance, out.maintenance); err != nil {
		return err
	}
	if err := ctx.Output(outputZone, out.zone); err != nil {
		return err
	}
	if err := ctx.Output(outputRegion, out.region); err != nil {
		return err
	}
	return nil
}

// requestedOutputsFromContext resolves output selector input from module context.
func requestedOutputsFromContext(ctx blackstart.ModuleContext) (map[string]struct{}, error) {
	raw, err := blackstart.ContextInputAs[[]string](ctx, inputRequests, false)
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", inputRequests, err)
	}
	return normalizeRequestedOutputs(raw)
}

// normalizeRequestedOutputs validates and normalizes requested output names.
func normalizeRequestedOutputs(raw []string) (map[string]struct{}, error) {
	if len(raw) == 0 {
		return map[string]struct{}{
			outputProjectID: {},
			outputRegion:    {},
		}, nil
	}
	out := make(map[string]struct{}, len(raw))
	for i, v := range raw {
		key := strings.ToLower(strings.TrimSpace(v))
		if key == "" {
			return nil, fmt.Errorf("value[%d] cannot be empty", i)
		}
		if _, ok := validMetadataOutputs[key]; !ok {
			return nil, fmt.Errorf("unknown request %q", v)
		}
		out[key] = struct{}{}
	}
	return out, nil
}

// wantsOutput reports whether output should be fetched/emitted.
func wantsOutput(requested map[string]struct{}, output string) bool {
	if requested == nil {
		return true
	}
	_, ok := requested[output]
	return ok
}

// metadataGet performs a metadata service request for a single key path.
func metadataGet(ctx blackstart.ModuleContext, reader metadataReader, path string) (string, error) {
	value, err := reader.get(ctx, path)
	if err != nil {
		return "", fmt.Errorf("metadata request failed for %q: %w", path, err)
	}
	return strings.TrimSpace(value), nil
}

// zoneFromPath extracts the short zone name from a metadata zone path.
func zoneFromPath(zonePath string) string {
	parts := strings.Split(zonePath, "/")
	return parts[len(parts)-1]
}

// regionFromZone derives the region name from a zonal location.
func regionFromZone(zone string) string {
	if zone == "" {
		return ""
	}
	i := strings.LastIndex(zone, "-")
	if i <= 0 {
		return zone
	}
	return zone[:i]
}

// machineTypeFromPath extracts the short machine type name from a metadata machine-type path.
func machineTypeFromPath(machineTypePath string) string {
	parts := strings.Split(machineTypePath, "/")
	return parts[len(parts)-1]
}
