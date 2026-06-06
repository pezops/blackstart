---
title: kubernetes_secret_value
---

# kubernetes_secret_value

Manages key-value pairs in a Kubernetes Secret resource.

**Update Policies**

Update policies control how existing values are handled when setting key-value pairs in ConfigMaps
and Secrets. The following update policies are supported:

- `preserve_any` - Any existing value will be preserved. To avoid any accidental changes, this is
  the default update policy.
- `overwrite` - Existing values will be overwritten if they differ from the new value.
- `preserve` - Any non-empty, existing value will be preserved.
- `fail` - If the new value differs from the existing value, the operation will fail.

## Requirements

- The Kubernetes identity must be authorized to read and update Secrets in the target namespace.

- Required Secret verbs for this module: `get`, `update`.

## Inputs

| Id            | Description                                                                                             | Type                | Required |
| ------------- | ------------------------------------------------------------------------------------------------------- | ------------------- | -------- |
| key           | Key in the Secret to set                                                                                | string              | true     |
| secret        | Secret resource                                                                                         | \*kubernetes.secret | true     |
| update_policy | Update policy for the key-value pair<br>Default: **preserve_any**                                       | string              | false    |
| value         | Value to set for the key. Required unless `update_policy` is `preserve_any`. Empty strings are allowed. | string              | false    |

## Outputs

| Id    | Description                                            | Type   |
| ----- | ------------------------------------------------------ | ------ |
| value | Current value stored for the key after reconciliation. | string |

## Examples

### Read Secret Value

```yaml
id: read-secret-value
module: kubernetes_secret_value
inputs:
  secret:
    fromDependency:
      id: app-secret
      output: secret
  key: DATABASE_PASSWORD
```

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
  update_policy: overwrite
```
