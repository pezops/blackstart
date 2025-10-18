---
title: kubernetes_configmap_value
---

# kubernetes_configmap_value

Manages key-value pairs in a Kubernetes ConfigMap resource.

**Update Policies**

Update policies control how existing values are handled when setting key-value pairs in ConfigMaps
and Secrets. The following update policies are supported:

- `overwrite` - Existing values will be overwritten if they differ from the new value.
- `preserve` - Any non-empty, existing value will be preserved.
- `preserve_any` - Any existing value will be preserved.
- `fail` - If the new value differs from the existing value, the operation will fail.

## Inputs

| Id            | Description                                                    | Type                   | Required |
| ------------- | -------------------------------------------------------------- | ---------------------- | -------- |
| configmap     | ConfigMap resource                                             | \*kubernetes.configMap | true     |
| key           | Key in the ConfigMap to set                                    | string                 | true     |
| update_policy | Update policy for the key-value pair<br>Default: **overwrite** | string                 | false    |
| value         | Value to set for the key                                       | string                 | true     |

## Outputs

No outputs are supported for this module

## Examples

### Set ConfigMap Value

```yaml
id: set-configmap-example
module: kubernetes_configmap_value
inputs:
  configmap:
    fromDependency:
      id: app-configmap
      output: configmap
  key: DATABASE_URL
  value: postgres://user:password@localhost:5432/db
```
