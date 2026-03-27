---
title: postgres_grant
---

# postgres_grant

Ensures that Postgres roles have the specified permissions on resources. The scope specifies the
type of resources where the permissions will be applied.

If multiple values are provided for `role`, `permission`, `schema`, or `resource`, Blackstart
expands all possible combinations of the Operation and applies them all.

The permissions allowed vary by scope. See the
[PostgreSQL GRANT documentation](https://www.postgresql.org/docs/current/sql-grant.html) for details
on valid permissions for each scope.

## Requirements

- A valid Postgres `connection` input must be provided.

- The database user of the `connection` must have sufficient privileges to apply the requested
  grants.

- Target roles/users and target resources must exist for the selected `scope`.

- For `TABLE` and `SEQUENCE` scopes, both schema and resource must exist and be addressable by the
  user.

- For `FUNCTION`, `PROCEDURE`, and `ROUTINE` scopes, `schema` must be provided and `resource` must
  be a routine signature that includes argument types unless `all` is true.

- `LARGE_OBJECT` scope requires `resource` to be a numeric large object OID (`loid`).

- `PARAMETER` scope requires a Postgres version that supports parameter privileges
  (`GRANT ... ON PARAMETER ...`) and `has_parameter_privilege`.

## Inputs

| Id                | Description                                                                                                                                                                                                                                                                 | Type             | Required |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| all               | Apply permissions to all resources of the scope (if supported) in the schema. When set, the resource input must be empty.<br>Default: **false**                                                                                                                             | bool             | false    |
| connection        | database connection to the managed Postgres instance.                                                                                                                                                                                                                       | \*sql.DB         | true     |
| permission        | Permission(s) or role membership(s) to be assigned to the role(s). Depending on the resource scope, the valid permissions may vary.                                                                                                                                         | string, []string | true     |
| resource          | Resource(s) where the permission(s) are applied. For `FUNCTION`, `PROCEDURE`, and `ROUTINE` scopes, provide a routine signature with argument types.                                                                                                                        | string, []string | false    |
| role              | Role(s) or username(s) that will have the grant assigned.                                                                                                                                                                                                                   | string, []string | true     |
| schema            | Schema(s) where the permission is to be applied.                                                                                                                                                                                                                            | string, []string | false    |
| scope             | Scope of the resource where the permission is to be applied. Supported values: `INSTANCE`, `DATABASE`, `SCHEMA`, `TABLE`, `SEQUENCE`, `FUNCTION`, `PROCEDURE`, `ROUTINE`, `DOMAIN`, `FDW`, `FOREIGN_SERVER`, `LANGUAGE`, `LARGE_OBJECT`, `PARAMETER`, `TABLESPACE`, `TYPE`. | string           | false    |
| with_grant_option | Request `WITH GRANT OPTION` for supported scopes. Not supported for `INSTANCE` scope.<br>Default: **false**                                                                                                                                                                 | bool             | false    |

## Outputs

No outputs are supported for this module

## Examples

### Grant EXECUTE on a function

```yaml
id: grant-execute-function
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: FUNCTION
  schema: public
  resource: do_work(integer)
```

### Grant EXECUTE on all procedures in schema

```yaml
id: grant-execute-all-procedures
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: PROCEDURE
  schema: public
  all: true
```

### Grant EXECUTE on all routines in schema

```yaml
id: grant-execute-all-routines
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: ROUTINE
  schema: public
  all: true
```

### Grant SELECT on all tables in a schema

```yaml
id: grant-select-all-tables-in-public
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: reporting_user
  permission: SELECT
  scope: TABLE
  schema: public
  all: true
```

### Grant SELECT with grant option on a table

```yaml
id: grant-select-with-grant-option
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
  with_grant_option: true
```

### Grant SET on a configuration parameter

```yaml
id: grant-set-on-parameter
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: SET
  scope: PARAMETER
  resource: work_mem
```

### Grant USAGE on a type

```yaml
id: grant-usage-on-type
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: USAGE
  scope: TYPE
  resource: status_type
```

### Grant USAGE on all sequences in a schema

```yaml
id: grant-usage-all-sequences-in-public
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: USAGE
  scope: SEQUENCE
  schema: public
  all: true
```

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
