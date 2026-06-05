---
title: mysql_grant
---

# mysql_grant

Ensures that MySQL accounts or roles have the specified permissions on databases or tables.

If multiple values are provided for `role`, `permission`, `schema`, or `resource`, Blackstart
expands all possible combinations of the Operation and applies them all.

## Requirements

- A valid MySQL `connection` input must be provided.

- The database user of the `connection` must have sufficient privileges to apply the requested
  grants.

- Target accounts/roles and resources must exist for the selected `scope`.

## Inputs

| Id                | Description                                                                                                            | Type             | Required |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| all               | Apply table permissions to all tables in the database named by `schema`.<br>Default: **false**                         | bool             | false    |
| connection        | Database connection to the managed MySQL instance.                                                                     | \*sql.DB         | true     |
| permission        | Permission(s) to assign to the role(s).                                                                                | string, []string | true     |
| resource          | Database name for `DATABASE` scope or table name for `TABLE` scope.                                                    | string, []string | false    |
| role              | Account(s) or role(s) that will have the grant assigned. Host defaults to `%` when omitted.                            | string, []string | true     |
| schema            | Database name for `TABLE` scope.                                                                                       | string, []string | false    |
| scope             | Scope of the resource where the permission is applied. Supported values: `DATABASE`, `TABLE`.<br>Default: **DATABASE** | string           | false    |
| with_grant_option | Request `WITH GRANT OPTION`.<br>Default: **false**                                                                     | bool             | false    |

## Outputs

No outputs are supported for this module

## Examples

### Grant database permissions

```yaml
id: grant-app-db-select
module: mysql_grant
inputs:
  connection:
    fromDependency:
      id: connect-db
      output: connection
  role: app_user
  permission: SELECT
  scope: DATABASE
  resource: app
```

### Grant table permissions

```yaml
id: grant-orders-select
module: mysql_grant
inputs:
  connection:
    fromDependency:
      id: connect-db
      output: connection
  role: reporting_user
  permission: SELECT
  scope: TABLE
  schema: app
  resource: orders
```
