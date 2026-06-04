---
title: google_cloudsql_managed_instance
---

# google_cloudsql_managed_instance

Manages a Google Cloud SQL for PostgreSQL or MySQL instance. When managed, the module ensures the
current workload identity is a member of the `cloudsqlsuperuser` role on the instance. The instance
is then usable for further operations.

**Notes**

- This module does not create or delete the Cloud SQL instance, it only manages the IAM user access.
- The module uses a temporary built-in user to perform the role management operations. This user is
  created and deleted as needed.
- When the module is set to not exist, the current workload identity is removed from the
  `cloudsqlsuperuser` role, but the user itself is not deleted.
- In Cloud SQL for PostgreSQL, `cloudsqlsuperuser` is not a true PostgreSQL `superuser` role. For
  grants on database objects (for example tables), the managing role may still need
  `WITH GRANT OPTION`. A simple approach is to grant the Blackstart service account role membership
  in the owner role of the target object. Otherwise, the Blackstart service account will need to be
  granted the same permission `WITH GRANT OPTION` on the target object to be able to manage
  permissions for other users.
- Cloud SQL for SQL Server does not support IAM authentication for database operations and is not
  supported by this module.
- Cloud SQL for MySQL 5.6 is not supported because
  [IAM database authentication is not supported for MySQL 5.6](https://docs.cloud.google.com/sql/docs/mysql/iam-authentication#restrictions).
- Cloud SQL for MySQL 5.7 IAM users are supported by `google_cloudsql_user`, but managed-instance
  administration requires the
  [role support available in MySQL 8+](https://docs.cloud.google.com/sql/docs/mysql/users#mysql-8.0-user-privileges).

## Requirements

- The Cloud SQL instance must exist.

- The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled
  on the project.

- The instance must have IAM authentication enabled for
  [PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth)
  or [MySQL](https://docs.cloud.google.com/sql/docs/mysql/iam-authentication#configure-iam-db-auth)
  with the engine-specific authentication flag set to `on`.

- The Blackstart service account must have permission to manage, connect, and login to the database
  instance. Suggested pre-defined roles are
  [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin),
  [`roles/cloudsql.client`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.client),
  and
  [`roles/cloudsql.instanceUser`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.instanceUser).

## Inputs

| Id              | Description                                                                                                                        | Type   | Required |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| connection_type | Type of connection to use. Must be one of: `PUBLIC_IP`, or `PRIVATE_IP`.<br>Default: **PRIVATE_IP**                                | string | false    |
| database        | Database name to connect to and return in the managed connection. Defaults to `postgres` for PostgreSQL and no database for MySQL. | string | false    |
| instance        | Cloud SQL instance ID to manage.                                                                                                   | string | true     |
| project         | Google Cloud project ID. If not provided, the current project will be used.                                                        | string | false    |
| user            | The user to manage. If not provided, the current user will be used.                                                                | string | false    |

## Outputs

| Id         | Description                                                                               | Type     |
| ---------- | ----------------------------------------------------------------------------------------- | -------- |
| connection | Database connection to the managed Cloud SQL instance authenticated as the managing user. | \*sql.DB |

## Examples

### Manage a Cloud SQL instance

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
