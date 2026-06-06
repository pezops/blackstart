---
title: kubernetes_configmap_value
---

# kubernetes_configmap_value

Manages key-value pairs in a Kubernetes ConfigMap resource.

**Update Policies**

Update policies control how existing values are handled when setting key-value pairs in ConfigMaps
and Secrets. The following update policies are supported:

- `preserve_any` - Any existing value will be preserved. To avoid any accidental changes, this is
  the default update policy.
- `overwrite` - Existing values will be overwritten if they differ from the new value.
- `preserve` - Any non-empty, existing value will be preserved.
- `fail` - If the new value differs from the existing value, the operation will fail.

## Requirements

- The Kubernetes identity must be authorized to read and update ConfigMaps in the target namespace.

- Required ConfigMap verbs for this module: `get`, `update`.

## Inputs

| Id            | Description                                                                                             | Type                   | Required |
| ------------- | ------------------------------------------------------------------------------------------------------- | ---------------------- | -------- |
| configmap     | ConfigMap resource                                                                                      | \*kubernetes.configMap | true     |
| key           | Key in the ConfigMap to set                                                                             | string                 | true     |
| update_policy | Update policy for the key-value pair<br>Default: **preserve_any**                                       | string                 | false    |
| value         | Value to set for the key. Required unless `update_policy` is `preserve_any`. Empty strings are allowed. | string                 | false    |

## Outputs

| Id    | Description                                            | Type   |
| ----- | ------------------------------------------------------ | ------ |
| value | Current value stored for the key after reconciliation. | string |

## Examples

### Read ConfigMap Value

```yaml
id: read-configmap-value
module: kubernetes_configmap_value
inputs:
  configmap:
    fromDependency:
      id: app-configmap
      output: configmap
  key: DATABASE_URL
  update_policy: preserve_any
```

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
  update_policy: overwrite
```
