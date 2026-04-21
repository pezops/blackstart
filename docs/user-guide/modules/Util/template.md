---
title: util_template
---

# util_template

Renders a templated string. Supports `workflowOutput "<operation-id>" "<output-key>"` for reading
outputs from operations in the current workflow run.

## Requirements

- Each operation referenced by `workflowOutput` must be listed in the template operation
  `dependsOn`.

## Inputs

| Id       | Description                          | Type   | Required |
| -------- | ------------------------------------ | ------ | -------- |
| template | Go template format string to render. | string | true     |

## Outputs

| Id     | Description               | Type   |
| ------ | ------------------------- | ------ |
| result | Rendered template result. | string |

## Examples

### Render SQL IAM username from dependency outputs

```yaml
operations:
  - id: identity
    module: google_cloud_metadata
    inputs:
      requests:
        - project_id

  - id: sql-iam-username
    module: util_template
    dependsOn:
      - identity
    inputs:
      template: 'blackstart-sa@{{ workflowOutput "identity" "project_id" }}.iam'
```
