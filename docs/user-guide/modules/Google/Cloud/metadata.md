---
title: google_cloud_metadata
---

# google_cloud_metadata

Retrieves runtime metadata from the Google Cloud metadata service and exposes it as outputs for
downstream operations.

This module is intended for workloads running on Google Cloud platforms that provide the metadata
service, such as GKE, GCE, and Cloud Run.

## Inputs

| Id       | Description                                                                                                                                                                                                                                                                                                 | Type             | Required |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| requests | Requested metadata fields to fetch. Valid values: `project_id`, `project_number`, `instance_id`, `instance_name`, `hostname`, `cpu_platform`, `image`, `machine_type`, `preempted`, `tags`, `maintenance_event`, `zone`, `region`. Accepts a string or list of strings.<br>Default: **[project_id region]** | string, []string | false    |

## Outputs

| Id                | Description                                                   | Type     |
| ----------------- | ------------------------------------------------------------- | -------- |
| cpu_platform      | CPU platform of the instance.                                 | string   |
| hostname          | Instance hostname.                                            | string   |
| image             | Image path used by the instance.                              | string   |
| instance_id       | Compute instance ID from metadata.                            | string   |
| instance_name     | Compute instance name from metadata.                          | string   |
| machine_type      | Machine type (for example `e2-standard-4`).                   | string   |
| maintenance_event | Current maintenance event state.                              | string   |
| preempted         | Whether the instance is preempted.                            | bool     |
| project_id        | Google Cloud project ID.                                      | string   |
| project_number    | Google Cloud numeric project ID.                              | string   |
| region            | Compute region derived from zone (for example `us-central1`). | string   |
| tags              | Network tags attached to the instance.                        | []string |
| zone              | Compute zone (for example `us-central1-a`).                   | string   |

## Examples

### Read default metadata

```yaml
id: cloud_metadata
module: google_cloud_metadata
```

### Read only project and region

```yaml
id: cloud_metadata
module: google_cloud_metadata
inputs:
  requests:
    - project_id
    - region
```
