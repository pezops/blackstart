---
title: google_cloudsql_user
---

# google_cloudsql_user

Ensures that a Cloud SQL user exists with the specified parameters. In alignment with Blackstart's
security best practices, this module only supports managing IAM users and service accounts, and does
not support built-in users.

**Notes**

- Cloud SQL for SQL Server does not support IAM authentication for database operations and is not
  supported by this module. Use Active Directory authentication instead for SQL Server instances.

## Requirements

- The Cloud SQL instance must exist.

- The IAM user or service account specified must exist.

- The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled
  on the project.

- The instance must have
  [IAM authentication](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth)
  enabled with the `cloudsql.iam_authentication` / `cloudsql_iam_authentication` flag set to `on`.

- The Blackstart service account must have permission to manage the database instance. The suggested
  pre-defined role is
  [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin).

## Inputs

| Id        | Description                                                                                                        | Type   | Required |
| --------- | ------------------------------------------------------------------------------------------------------------------ | ------ | -------- |
| instance  | Cloud SQL instance ID.                                                                                             | string | true     |
| project   | Google Cloud project ID. If not provided, the current project will be used.                                        | string | false    |
| region    | Google Cloud region for the Cloud SQL instance. If not provided, the region will be inferred from the instance ID. | string | false    |
| user      | Username for the Cloud SQL user.                                                                                   | string | true     |
| user_type | Type of the user to create. Must be one of: `CLOUD_IAM_USER`, `CLOUD_IAM_SERVICE_ACCOUNT`.                         | string | true     |

## Outputs

| Id   | Description                                                 | Type   |
| ---- | ----------------------------------------------------------- | ------ |
| user | The name of the Cloud SQL user that was created or managed. | string |

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
