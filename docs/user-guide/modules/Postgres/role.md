---
title: postgres_role
---

# postgres_role

Module to manage PostgreSQL roles.

## Inputs

| Id          | Description                                                | Type   | Required |
| ----------- | ---------------------------------------------------------- | ------ | -------- |
| create_db   | If true, the Role can create databases.                    | bool   | false    |
| create_role | If true, the Role can create other roles.                  | bool   | false    |
| inherit     | If true, the Role can Inherit privileges from other roles. | bool   | false    |
| login       | If true, the Role can log in to the database.              | bool   | false    |
| name        | Id of the Role to manage.                                  | string | true     |
| replication | If true, the Role can initiate streaming Replication.      | bool   | false    |

## Outputs

No outputs are supported for this module

## Examples

### Create a new Role

```yaml
id: create-Role
module: postgres_role
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  Name: my-new-Role
  Login: true
```
