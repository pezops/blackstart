---
title: mock_module
---

# mock_module

A mock module that does nothing. This module is used to mock operations and operation results for
testing purposes.

## Inputs

| Id   | Description                                                           | Type | Required |
| ---- | --------------------------------------------------------------------- | ---- | -------- |
| pass | Determines if the operation should pass or fail.<br>Default: **true** | bool | false    |

## Outputs

No outputs are supported for this module

## Examples

### Simple Mock

```yaml
id: mock-1
module: mock_module
```
