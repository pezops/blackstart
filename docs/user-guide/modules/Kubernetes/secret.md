---
title: kubernetes_secret
---

# kubernetes_secret

Manages a Kubernetes Secret resource, but not content.

**Notes**

- This module does not manage content of the Secret. Use the `kubernetes_secret_value` module to
  manage key-value pairs in the Secret.
- Once a Secret is set to be immutable, values cannot be set or changed. Do not set a Secret to be
  immutable before setting the values. See
  [Immutable Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#secret-immutable)
  for more information.

## Inputs

| Id        | Description                                                                                                 | Type                 | Required |
| --------- | ----------------------------------------------------------------------------------------------------------- | -------------------- | -------- |
| client    | Kubernetes client interface to use for API calls                                                            | kubernetes.Interface | true     |
| immutable | Make the Secret immutable. Ignored if not set (default).                                                    | \*bool               | false    |
| name      | Name of the Secret                                                                                          | string               | true     |
| namespace | Namespace where the Secret exists<br>Default: **default**                                                   | string               | false    |
| type      | Type of the Secret (e.g., Opaque, kubernetes.io/tls, kubernetes.io/dockerconfigjson)<br>Default: **Opaque** | string               | false    |

## Outputs

| Id     | Description         | Type                |
| ------ | ------------------- | ------------------- |
| secret | The Secret resource | \*kubernetes.secret |

## Examples

### Basic Secret Usage

```yaml
id: create-secret
module: kubernetes_secret
inputs:
  client:
    fromDependency:
      id: k8s-client
      output: client
  name: my-secret
  namespace: default
```

### Configure Secret to be Immutable

```yaml
operations:
  - id: k8s_client
    module: kubernetes_client
  - id: myapp_secret
    module: kubernetes_secret
    name: MyApp Secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: myapp
      name: myapp-secret
  - id: myapp_db_host
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: myapp_secret
          output: secret
      key: db_host
      value: db.myapp.svc.cluster.local
  - id: myapp_db_port
    module: kubernetes_secret_value
    inputs:
      secret:
        fromDependency:
          id: myapp_secret
          output: secret
      key: db_port
      value: "5432"
  - id: myapp_secret_immutable
    module: kubernetes_secret
    inputs:
      client:
        fromDependency:
          id: k8s_client
          output: client
      namespace: myapp
      name: myapp-secret
      immutable: true
    dependsOn:
      - myapp_db_host
      - myapp_db_port
```
