---
title: postgres_connection
---

# postgres_connection

Connection to a PostgreSQL database.

## Inputs

| Id       | Description                                                                                                                                                | Type   | Required |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| database | Name of the PostgreSQL database to connect to.<br>Default: **postgres**                                                                                    | string | false    |
| host     | Hostname or IP address of the PostgreSQL server.<br>Default: **localhost**                                                                                 | string | false    |
| password | password to connect to the PostgreSQL database.                                                                                                            | string | false    |
| port     | port number of the PostgreSQL server.<br>Default: **5432**                                                                                                 | int    | false    |
| sslmode  | SSL mode to use when connecting to the PostgreSQL database. Options are 'disable', 'prefer', 'require', 'verify-ca', 'verify-full'.<br>Default: **prefer** | string | false    |
| username | username to connect to the PostgreSQL database.                                                                                                            | string | true     |

## Outputs

| Id         | Description                                        | Type     |
| ---------- | -------------------------------------------------- | -------- |
| connection | The connection details to the PostgreSQL database. | \*sql.DB |

## Examples

### Connect to a database

```yaml
id: connect-db
module: postgres_connection
inputs:
  host: db.example.com
  database: mydb
  username: admin
```
