---
title: postgres_grant
---

# postgres_grant

Ensures that a Postgres role has the specified Permission on a resource.

If multiple values are provided for `role`, `permission`, `schema`, or `resource`, Blackstart
expands all possible combinations of the Operation and applies them all.

## Requirements

- A valid Postgres `connection` input must be provided.

- The database user of the `connection` must be a member of a role that has `ADMIN OPTION` on the
  target roles.

- Target roles/users and target resources must exist for the selected `scope`.

- For `TABLE` scope, both schema and table must exist and be addressable by the user.

## Inputs

| Id         | Description                                                                                                                         | Type             | Required |
| ---------- | ----------------------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| connection | database connection to the managed Postgres instance.                                                                               | \*sql.DB         | true     |
| permission | Permission(s) or role membership(s) to be assigned to the role(s). Depending on the resource scope, the valid permissions may vary. | string, []string | true     |
| resource   | Resource(s) where the permission(s) are to be applied. This might be a database name, table name, or schema name.                   | string, []string | false    |
| role       | Role(s) or username(s) that will have the grant assigned.                                                                           | string, []string | true     |
| schema     | Schema(s) where the permission is to be applied.                                                                                    | string, []string | false    |
| scope      | Scope of the resource where the permission is to be applied. This might be a database, table, or schema.                            | string           | false    |

## Outputs

No outputs are supported for this module

## Examples

### Grant across multiple resources

```yaml
id: grant-multi-resource-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role:
    - app_user
    - reporting_user
  permission:
    - SELECT
    - UPDATE
  scope: TABLE
  schema:
    - public
    - analytics
  resource:
    - orders
    - invoices
```

### Grant multiple schema permissions for one user

```yaml
id: grant-user-schema-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission:
    - USAGE
    - CREATE
  scope: SCHEMA
  resource: app_data
```

### Grant role membership

```yaml
id: grant-role-membership
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: my-other-role
```

### Grant role membership at the instance level

```yaml
id: grant-instance-role-membership
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: app_readers
  scope: INSTANCE
```

### Grant schema permission to multiple roles

```yaml
id: grant-schema-usage-to-team
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role:
    - app_user
    - reporting_user
  permission: USAGE
  scope: SCHEMA
  resource: analytics
```

### Grant schema usage

```yaml
id: grant-schema-usage
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: USAGE
  scope: SCHEMA
  resource: my-schema
```

### Grant table permissions

```yaml
id: grant-orders-table-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: reporting_user
  permission:
    - SELECT
    - UPDATE
  scope: TABLE
  schema: public
  resource: orders
```
