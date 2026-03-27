---
title: postgres_default_privileges
---

# postgres_default_privileges

Ensures PostgreSQL default privilege definitions in
[`pg_default_acl`](https://www.postgresql.org/docs/current/catalog-pg-default-acl.html) are present
or absent. This module does not reconcile grants on existing resources. Default privileges apply
only to new objects created under the matching `FOR ROLE` context.

When operation `doesNotExist=false`, this module applies default privilege grants. When
`doesNotExist=true`, it removes matching default privilege entries.

## Requirements

- A valid Postgres `connection` input must be provided.

- The database user in `connection` must have permission to execute `ALTER DEFAULT PRIVILEGES` for
  the configured owner role context (`FOR ROLE`).

- Target roles/users in `role` should exist before applying grants or revokes.

## Inputs

| Id                | Description                                                                                                           | Type             | Required |
| ----------------- | --------------------------------------------------------------------------------------------------------------------- | ---------------- | -------- |
| connection        | Database connection.                                                                                                  | \*sql.DB         | true     |
| for_role          | Owner role(s) used in `FOR ROLE`. If omitted, current database role is used.                                          | string, []string | false    |
| permission        | Permission(s) to grant or revoke in the default-privilege definition.                                                 | string, []string | true     |
| revoke_mode       | Revoke behavior when operation `doesNotExist=true`. Supported values: `RESTRICT`, `CASCADE`.<br>Default: **RESTRICT** | string           | false    |
| role              | Role(s) receiving the default privileges.                                                                             | string, []string | true     |
| schema            | Optional schema(s) used in `IN SCHEMA`.                                                                               | string, []string | false    |
| scope             | Object class for default privileges. Supported values: `TABLES`.                                                      | string           | true     |
| with_grant_option | Apply `WITH GRANT OPTION`. Not valid when operation `doesNotExist=true` (revoke mode).<br>Default: **false**          | bool             | false    |

## Outputs

No outputs are supported for this module

## Examples

### Grant SELECT default privilege for future tables

```yaml
id: default-privs-grant-tables
module: postgres_default_privileges
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role:
    - app_reader
    - analytics_team
  permission:
    - SELECT
    - UPDATE
  scope: TABLES
  for_role: app_owner
  schema: public
  with_grant_option: false
```

### Revoke SELECT default privilege for future tables

```yaml
id: default-privs-revoke-tables
module: postgres_default_privileges
doesNotExist: true
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: analytics_team
  permission: UPDATE
  scope: TABLES
  for_role: app_owner
  schema: public
  revoke_mode: RESTRICT
```
