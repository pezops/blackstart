---
title: kubernetes_client
---

# kubernetes_client

Establishes a connection to a Kubernetes cluster and provides a client for other modules to use.

## Inputs

| Id      | Description                                                                                                                                        | Type   | Required |
| ------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| context | The Kubernetes context to use. If not provided, uses the current-context from kubeconfig, or in-cluster config if running in a Kubernetes cluster. | string | false    |

## Outputs

| Id     | Description                                          | Type                 |
| ------ | ---------------------------------------------------- | -------------------- |
| client | Kubernetes client that can be used by other modules. | kubernetes.Interface |

## Examples

### Default Client

```yaml
id: default-k8s-client
module: kubernetes_client
```

### Specific Context

```yaml
id: prod-k8s-client
module: kubernetes_client
inputs:
  context: prod-cluster
```
