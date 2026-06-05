---
title: google_cloudsql_database
---

# google_cloudsql_database

Ensures that a database exists on a Google Cloud SQL for PostgreSQL or MySQL instance using the
Cloud SQL Admin API.

**Notes**

- This module does not create or delete the Cloud SQL instance.
- This module does not manage database ownership or privileges.
- Cloud SQL for SQL Server is not supported by this module.
- `charset` and `collation` are only supported for MySQL. When unset, Cloud SQL API defaults are
  used.

## Requirements

- The Cloud SQL instance must exist.

- The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled
  on the project.

- The Blackstart service account must have permission to manage databases on the instance. The
  suggested pre-defined role is
  [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin).

## Inputs

| Id        | Description                                                                                            | Type   | Required |
| --------- | ------------------------------------------------------------------------------------------------------ | ------ | -------- |
| charset   | Optional MySQL charset value. When omitted, the Cloud SQL API default is used.                         | string | false    |
| collation | Optional MySQL collation value. When omitted, the Cloud SQL API default is used.                       | string | false    |
| database  | Database name to manage.                                                                               | string | true     |
| instance  | Cloud SQL instance ID.                                                                                 | string | true     |
| project   | Google Cloud project ID. If not provided, the current project will be used.                            | string | false    |
| region    | Google Cloud region for the Cloud SQL instance. Accepted for consistency with other Cloud SQL modules. | string | false    |

## Outputs

| Id       | Description                                    | Type   |
| -------- | ---------------------------------------------- | ------ |
| database | The database name that was created or managed. | string |

## Examples

### Create a MySQL database with charset and collation

```yaml
id: create-app-db
module: google_cloudsql_database
inputs:
  instance: my-cloudsql-instance
  database: app
  charset: utf8mb4
  collation: utf8mb4_0900_ai_ci
```

### Create a database

```yaml
id: create-app-db
module: google_cloudsql_database
inputs:
  instance: my-cloudsql-instance
  database: app
```
