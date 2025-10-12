---
title: kubernetes_secret_value
---

# kubernetes_secret_value

Manages key-value pairs in a Kubernetes Secret resource.

## Inputs

| Id     | Description              | Type                | Required |
| ------ | ------------------------ | ------------------- | -------- |
| key    | Key in the Secret to set | string              | true     |
| secret | Secret resource          | \*kubernetes.secret | true     |
| value  | Value to set for the key | string              | true     |

## Outputs

No outputs are supported for this module

## Examples

### Set Secret Value

```yaml
id: set-secret-example
module: kubernetes_secret_value
inputs:
  secret:
    fromDependency:
      id: app-secret
      output: secret
  key: DATABASE_PASSWORD
  value: supersecretpassword
```
