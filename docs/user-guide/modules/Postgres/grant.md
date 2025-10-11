---
title: postgres_grant
---

# postgres_grant

Ensures that a Postgres Role has the specified Permission on a Resource.

## Inputs

| Id         | Description                                                                                                     | Type     | Required |
| ---------- | --------------------------------------------------------------------------------------------------------------- | -------- | -------- |
| connection | database connection to the managed Postgres instance.                                                           | \*sql.DB | true     |
| permission | Permission or Role to be assigned to the Role. Depending on the Resource Scope, the valid permissions may vary. | string   | true     |
| resource   | Id of the Resource for the Permission to be applied. This might be a database Name, table Name, or Schema Name. | string   | false    |
| role       | Role or username that will have the grant assigned.                                                             | string   | true     |
| schema     | Id of a Postgres Schema where the Permission is to be applied.                                                  | string   | false    |
| scope      | Scope of the Resource where the Permission is to be applied. This might be a database, table, or Schema.        | string   | false    |

## Outputs

No outputs are supported for this module

## Examples

### grant Role membership

```yaml
id: grant-Role-membership
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  Role: my-user
  Permission: my-other-Role
```

### grant Schema usage

```yaml
id: grant-Schema-usage
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  Role: my-user
  Permission: USAGE
  Scope: SCHEMA
  Resource: my-Schema
```
