---
title: google_cloudsql_managed_instance
---

# google_cloudsql_managed_instance

Manages a Google CloudSQL instance. When managed, the module will ensure that the current workload
identity is a member of the `cloudsqlsuperuser` role on the instance. The instance is then usable
for further operations.

**Notes**

- This module does not create or delete the CloudSQL instance, it only manages the IAM user access.
- The module uses a temporary built-in user to perform the role management operations. This user is
  created and deleted as needed.
- When the module is set to not exist, the current workload identity is removed from the
  `cloudsqlsuperuser` role, but the user itself is not deleted.
- In Cloud SQL for PostgreSQL, `cloudsqlsuperuser` is not a true PostgreSQL superuser. For grants on
  database objects (for example tables), the managing role may still need `WITH GRANT OPTION`. A
  simple approach is to grant the Blackstart service account role membership in the owner role of
  the target object. Otherwise, the Blackstart service account will need to be granted the same
  permission `WITH GRANT OPTION` on the target object to be able to manage permissions for other
  users.
- Cloud SQL for SQL Server does not support IAM authentication for database operations and is not
  supported by this module.

## Requirements

- The CloudSQL instance must exist.

- The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled
  on the project.

- The instance must have
  [IAM authentication](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth)
  enabled with the `cloudsql.iam_authentication` / `cloudsql_iam_authentication` flag set to `on`.

- The Blackstart service account must have permission to manage, connect, and login to the database
  instance. Suggested pre-defined roles are
  [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin),
  [`roles/cloudsql.client`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.client),
  and
  [`roles/cloudsql.instanceUser`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.instanceUser).

## Inputs

| Id              | Description                                                                                         | Type   | Required |
| --------------- | --------------------------------------------------------------------------------------------------- | ------ | -------- |
| connection_type | Type of connection to use. Must be one of: `PUBLIC_IP`, or `PRIVATE_IP`.<br>Default: **PRIVATE_IP** | string | false    |
| database        | Database name to connect to and return in the managed connection.<br>Default: **postgres**          | string | false    |
| instance        | CloudSQL instance ID to manage.                                                                     | string | true     |
| project         | Google Cloud project ID. If not provided, the current project will be used.                         | string | false    |
| user            | The user to manage. If not provided, the current user will be used.                                 | string | false    |

## Outputs

| Id         | Description                                                                              | Type     |
| ---------- | ---------------------------------------------------------------------------------------- | -------- |
| connection | Database connection to the managed CloudSQL instance authenticated as the managing user. | \*sql.DB |

## Examples

### Manage a CloudSQL instance

```yaml
id: manage-instance
module: google_cloudsql_managed_instance
inputs:
  instance: my-cloudsql-instance
```

### Manage instance and grant table privileges

```yaml
operations:
  - id: manage-instance
    module: google_cloudsql_managed_instance
    inputs:
      instance: my-cloudsql-instance
      project: my-gcp-project
      connection_type: PRIVATE_IP

  - id: grant-app-user-orders-select
    module: postgres_grant
    inputs:
      connection:
        fromDependency:
          id: manage-instance
          output: connection
      role: app_user
      permission: SELECT
      scope: TABLE
      schema: public
      resource: orders

  - id: grant-app-user-orders-update
    module: postgres_grant
    inputs:
      connection:
        fromDependency:
          id: manage-instance
          output: connection
      role: app_user
      permission: UPDATE
      scope: TABLE
      schema: public
      resource: orders
```
