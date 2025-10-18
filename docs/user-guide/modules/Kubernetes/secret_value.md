---
title: kubernetes_secret_value
---

# kubernetes_secret_value

Manages key-value pairs in a Kubernetes Secret resource.

**Update Policies**

Update policies control how existing values are handled when setting key-value pairs in ConfigMaps
and Secrets. The following update policies are supported:

- `overwrite` - Existing values will be overwritten if they differ from the new value.
- `preserve` - Any non-empty, existing value will be preserved.
- `preserve_any` - Any existing value will be preserved.
- `fail` - If the new value differs from the existing value, the operation will fail.

## Inputs

| Id            | Description                                                    | Type                | Required |
| ------------- | -------------------------------------------------------------- | ------------------- | -------- |
| key           | Key in the Secret to set                                       | string              | true     |
| secret        | Secret resource                                                | \*kubernetes.secret | true     |
| update_policy | Update policy for the key-value pair<br>Default: **overwrite** | string              | false    |
| value         | Value to set for the key                                       | string              | true     |

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
