---
title: kubernetes_configmap
---

# kubernetes_configmap

Manages a Kubernetes ConfigMap resource, but not content.

**Notes**

- This module does not manage content of the ConfigMap. Use the `kubernetes_configmap_value` module
  to manage key-value pairs in the ConfigMap.
- Once a ConfigMap is set to be immutable, values cannot be set or changed. Do not set a ConfigMap
  to be immutable before setting the values. See
  [Immutable ConfigMaps](https://kubernetes.io/docs/concepts/configuration/configmap/#configmap-immutable)
  for more information.

## Inputs

| Id        | Description                                                  | Type                 | Required |
| --------- | ------------------------------------------------------------ | -------------------- | -------- |
| client    | Kubernetes client interface to use for API calls             | kubernetes.Interface | true     |
| immutable | Make the ConfigMap immutable. Ignored if not set (default).  | \*bool               | false    |
| name      | Name of the ConfigMap                                        | string               | true     |
| namespace | Namespace where the ConfigMap exists<br>Default: **default** | string               | false    |

## Outputs

| Id        | Description            | Type                   |
| --------- | ---------------------- | ---------------------- |
| configmap | The ConfigMap resource | \*kubernetes.configMap |

## Examples

### Configure ConfigMap to be Immutable

```yaml
id: immutable-configmap
module: kubernetes_configmap
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-immutable-config
  namespace: default
  immutable: true
```

### Create ConfigMap

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
