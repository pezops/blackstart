---
title: kubernetes_configmap
---

# kubernetes_configmap

Manages a Kubernetes ConfigMap resource, but not content.

## Inputs

| Id        | Description                                                  | Type                 | Required |
| --------- | ------------------------------------------------------------ | -------------------- | -------- |
| client    | Kubernetes client interface to use for API calls             | kubernetes.Interface | true     |
| name      | Name of the ConfigMap                                        | string               | true     |
| namespace | Namespace where the ConfigMap exists<br>Default: **default** | string               | false    |

## Outputs

| Id        | Description            | Type                   |
| --------- | ---------------------- | ---------------------- |
| configmap | The ConfigMap resource | \*kubernetes.configMap |

## Examples

### Set ConfigMap Value

```yaml
id: create-configmap
module: kubernetes_configmap
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-config
  namespace: default
```
