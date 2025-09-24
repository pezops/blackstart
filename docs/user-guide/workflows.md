# Workflows

Workflows are the heart of Blackstart. They are declarative and define a desired state for the
resources being targeted. Blackstart reads these workflows and takes action to make the real world
match the desired state.

This approach is simple, powerful, and designed for modern cloud-native environments.

## The Basics

A **Workflow** is composed of a partially ordered set of **Operations**. Each Operation is a single
step that uses a specific **Module** to manage a piece of infrastructure.

Here is a simple example of a workflow that creates a CloudSQL user, configures the instance to be
managed, and then grants the user permissions on a database within that instance.

```yaml title="demo-workflow.yaml"
apiVersion: blackstart.pezops.github.io/v1alpha1
kind: Workflow
metadata:
  name: demo-workflow
spec:
  operations:
    - name: add svc account
      id: test_svc_account
      module: "google_cloudsql_user"
      inputs:
        project: "demo-j78sj4"
        instance: "instance-j38sl4"
        user: "test-iam-svc-account-cloudsql"
        user_type: CLOUD_IAM_SERVICE_ACCOUNT

    - name: test instance
      id: test_instance
      module: "google_cloudsql_managed_instance"
      inputs:
        project: "demo-j78sj4"
        region: "us-central1"
        instance: "instance-j38sl4"

    - name: grant cloudsql superuser to user
      id: test_grant
      module: "postgres_grant"
      inputs:
        connection:
          from_dependency:
            id: test_instance
            output: connection
        role:
          from_dependency:
            id: test_svc_account
            output: user
        permission: "pg_monitor"
```

## Execution Flow

Blackstart does not execute operations based on their order in the YAML file. Instead, it builds a
Directed Acyclic Graph (DAG) based on the dependencies between operations. The operations are run
serially by topologically sorting the DAG. This ensures that operations are always executed in the
correct order.

An operation can depend on another in two ways:

1. **Explicitly**: Using the `depends_on` field.
2. **Implicitly**: When an operation's `input` comes from another operation's `output`.

Blackstart analyzes these dependencies to build the execution graph. In the example above, the
`test_grant` operation implicitly depends on `test_instance` and `test_svc_account` because it uses
their outputs as inputs. Therefore, Blackstart will ensure both `test_instance` and
`test_svc_account` are successfully reconciled before attempting the `test_grant` operation.

If any circular dependencies are detected (e.g., Operation A depends on B, and B depends on A), the
workflow execution will fail with an error.

### Check then Set

For each operation in the graph, Blackstart follows an idempotent "check then set" model.

1.  **Check**: The module associated with the operation first checks if the resource is already in
    the state defined by the operation's inputs.
    - No changes are made in the check, but API calls happen or some actions which do not change the
      resource are performed. Since Blackstart is stateless, it must read the resources to return
      any defined output values from the module.
    - If the check **passes**, the resource already exists in the desired state. The module resolves
      and provides the necessary output values for other operations to use, and Blackstart moves to
      the next operation.
    - If the check **fails**, the resource does not exist or is not in the desired state.

2.  **Set**: This step is **only** executed if the `Check` step fails. The module performs an action
    to create or modify the resource to match the desired state. Once complete, it provides the
    necessary output values.

## Operations

Operations are the building blocks of a workflow. They define a single, discrete unit of work.

| Field         | Type               | Description                                                                                                      |
| ------------- | ------------------ | ---------------------------------------------------------------------------------------------------------------- |
| `id`          | `string`           | **Required.** A unique identifier for the operation within the workflow. This is used to establish dependencies. |
| `module`      | `string`           | **Required.** The id of the module to use for this operation.                                                    |
| `name`        | `string`           | A human-readable name for the operation.                                                                         |
| `description` | `string`           | An optional, more detailed description of what the operation does.                                               |
| `dependsOn`   | `[]string`         | A list of operation IDs that this operation explicitly depends on.                                               |
| `inputs`      | `map[string]Input` | A map of key-value pairs passed as inputs to the module.                                                         |

### Inputs and Outputs

Inputs provide configuration to an operation's module. They may be static values or dynamic values
derived from the outputs of other operations.

#### Static Inputs

A static input is a fixed value, like a string, number, or boolean.

```yaml
inputs:
  project: "demo-j78sj4"
  region: "us-central1"
```

#### Dynamic Inputs

An input to a module may be sourced from the output of another operation that has already run. This
is the primary way to chain operations together in a workflow. Use the `from_dependency` structure
to define this relationship.

```yaml
inputs:
  connection:
    from_dependency:
      id: test_instance # The ID of the dependency operation
      output: connection # The name of the output from that operation
```

In this case, the `connection` input will be populated with the value of the `connection` output
from the `test_instance` operation. This also creates a dependency on `test_instance` in the
generated execution graph.

## Eventual Consistency & Statelessness

Blackstart is designed around the principle of **eventual consistency** and is completely
**stateless**.

- **Stateless**: Unlike other tools, Blackstart does not require a state file (e.g.,
  `terraform.tfstate`). The "source of truth" is the workflow YAML and the actual state of the
  resources being configured.
- **Eventually Consistent**: When a workflow runs, each operation first **checks** if the resource
  is already in the desired state. If it is, Blackstart marks that operation as done and moves on to
  the next operation. If it's not in the desired state of configuration, Blackstart **sets** the
  resource to the desired state by performing whatever actions are required.

This model has a powerful implication: workflows are idempotent and safe to run repeatedly. When a
workflow has reached a fully reconciled state, any subsequent runs will simply check the current
state of the resources and find that they are already in the desired state, so no changes will be
made. This also means that the workflows enforce the desired state and help eliminate configuration
drift.

For example, a workflow needs a database table to exist so that it may configure grants on the
table. However, that table is created by a separate application which owns and controls the database
schema. The first time the workflow runs, the operation that depends on the table will fail. This is
perfectly fine. Once the other application creates the table, the _next_ run of the Blackstart
workflow will detect this, and the dependent operation will succeed, allowing the workflow to make
further progress.

This makes Blackstart incredibly resilient and well-suited for dynamic environments where different
components may come online at different times.
