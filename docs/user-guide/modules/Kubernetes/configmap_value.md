---
title: kubernetes_configmap_value
---

# kubernetes_configmap_value

Manages key-value pairs in a Kubernetes ConfigMap resource.

## Inputs

| Id        | Description                 | Type                   | Required |
| --------- | --------------------------- | ---------------------- | -------- |
| configmap | ConfigMap resource          | \*kubernetes.configMap | true     |
| key       | Key in the ConfigMap to set | string                 | true     |
| value     | Value to set for the key    | string                 | true     |

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
