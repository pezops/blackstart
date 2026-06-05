---
title: mysql_connection
---

# mysql_connection

Connection to a MySQL database.

## Requirements

- The MySQL server must be reachable from the Blackstart runtime.

- The provided credentials must be valid for the target database.

- The provided user must have permission to connect to the target database.

- TLS settings must match the server configuration.

## Inputs

| Id       | Description                                                                                                                                             | Type   | Required |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| database | Name of the MySQL database to connect to.<br>Default: **mysql**                                                                                         | string | false    |
| host     | Hostname or IP address of the MySQL server.<br>Default: **localhost**                                                                                   | string | false    |
| password | Password to connect to the MySQL database.                                                                                                              | string | false    |
| port     | Port number of the MySQL server.<br>Default: **3306**                                                                                                   | int    | false    |
| tls      | TLS mode to use when connecting to the MySQL database. Examples: `false`, `true`, `skip-verify`, or a registered TLS config name.<br>Default: **false** | string | false    |
| username | Username to connect to the MySQL database.                                                                                                              | string | true     |

## Outputs

| Id         | Description                                   | Type     |
| ---------- | --------------------------------------------- | -------- |
| connection | The connection details to the MySQL database. | \*sql.DB |

## Examples

### Connect to a database

```yaml
id: connect-db
module: mysql_connection
inputs:
  host: db.example.com
  database: app
  username: admin
```
