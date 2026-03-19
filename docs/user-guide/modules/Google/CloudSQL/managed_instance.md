---
title: google_cloudsql_managed_instance
---

# google_cloudsql_managed_instance

Manages a Google CloudSQL instance. When managed, the module will ensure that the current workload
identity is a member of the `cloudsqlsuperuser` role on the instance. The instance is then usable
for further operations.

**Requirements**

- The CloudSQL instance must exist.
- The instance must have IAM authentication enabled.
- The current workload identity must have the `roles/cloudsql.admin` role on the instance.

**Notes**

- This module does not create or delete the CloudSQL instance, it only manages the IAM user access.
- The module uses a temporary built-in user to perform the role management operations. This user is
  created and deleted as needed.
- When the module is set to not exist, the current workload identity is removed from the
  `cloudsqlsuperuser` role, but the user itself is not deleted.

## Inputs

| Id              | Description                                                                                                                   | Type   | Required |
| --------------- | ----------------------------------------------------------------------------------------------------------------------------- | ------ | -------- |
| connection_type | Type of connection to use. Must be one of: `PUBLIC_IP`, or `PRIVATE_IP`. Defaults to `PRIVATE_IP`.<br>Default: **PRIVATE_IP** | string | false    |
| instance        | CloudSQL instance ID to manage.                                                                                               | string | true     |
| project         | Google Cloud project ID. If not provided, the current project will be used.                                                   | string | false    |
| user            | The user to manage. If not provided, the current user will be used.                                                           | string | false    |

## Outputs

| Id         | Description                                                                              | Type     |
| ---------- | ---------------------------------------------------------------------------------------- | -------- |
| connection | Database connection to the managed CloudSQL instance authenticated as the managing user. | \*sql.DB |

## Examples

### Manage a CloudSQL instance

```yaml
id: manage-instance
module: google_cloudsql_managed_instance
inputs:
  instance: my-cloudsql-instance
```
