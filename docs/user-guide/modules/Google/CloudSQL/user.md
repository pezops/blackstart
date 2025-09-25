---
title: google_cloudsql_user
---

# google_cloudsql_user

Ensures that a CloudSQL user exists with the specified parameters.

## Inputs

| Id        | Description                                                                                                       | Type   | Required |
| --------- | ----------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| instance  | CloudSQL instance ID.                                                                                             | string | true     |
| project   | Google Cloud project ID. If not provided, the current project will be used.                                       | string | false    |
| region    | Google Cloud region for the CloudSQL instance. If not provided, the region will be inferred from the instance ID. | string | false    |
| user      | username for the CloudSQL user.                                                                                   | string | true     |
| user_type | Type of the user to create. Must be one of: `CLOUD_IAM_USER`, `CLOUD_IAM_SERVICE_ACCOUNT`.                        | string | true     |

## Outputs

| Id   | Description                                                | Type   |
| ---- | ---------------------------------------------------------- | ------ |
| user | The name of the CloudSQL user that was created or managed. | string |

## Examples

### Create a Cloud IAM user

```yaml
id: create-iam-user
module: google_cloudsql_user
inputs:
  instance: my-cloudsql-instance
  user: my-iam-user@example.com
  user_type: CLOUD_IAM_USER
```
