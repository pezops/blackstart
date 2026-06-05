# Eventual Consistency

Blackstart workflows are designed to converge over time. A workflow does not have to complete in a
single run to be useful. If one operation succeeds and a later operation is blocked by an external
dependency, the workflow can make more progress the next time it runs.

This is important for bootstrapping cloud systems because Blackstart often configures resources that
other tools or applications create first. For example, an application migration might create a
database table before Blackstart can grant permissions on that table.

## Stateless Reconciliation

Blackstart does not store a state file. The source of truth is the workflow YAML and the live state
of the resources being configured.

Each time a workflow runs, Blackstart rebuilds the operation dependency graph and reconciles each
operation in order. For each operation, the module follows the same
[Check then Set](./workflows.md#check-then-set) model:

- `Check` reads the current state and reports whether it already matches the workflow.
- `Set` runs only when `Check` returns false without an error.

Because the workflow is stateless, every run validates the live system again. If the system already
matches the workflow, no changes are made. If something drifted, the next run can bring it back to
the desired state.

At a high level, each workflow run follows this execution path:

![Workflow execution overview](../images/workflow.svg)

## How it Works: Periodic Runs

In controller mode, each workflow runs on its configured `spec.reconcileInterval` (default `5m`).
Controller mode also watches `Workflow` resources for add/update/delete changes and performs a
periodic full resync (`controller.resyncInterval`) as a safety net.

A workflow may not fully complete in one run. An operation might fail because it's waiting on an
external dependency that is not ready yet. This can be an expected part of bootstrapping. On the
next scheduled run, Blackstart checks the live state again and continues from the current reality,
not from stored state from the previous run.

A workflow is considered "fully reconciled" when a run completes with every operation passing its
`Check` step, and no `Set` operations are performed. At this point, the real-world infrastructure
matches the state declared in the workflow.

## Example: Waiting for a Database Table

A common use case is managing database permissions. An application's deployment process may be
responsible for creating database tables via a schema migration tool. A Blackstart workflow can be
responsible for creating a service account and granting it permissions on one of those tables.

Consider this workflow snippet:

```yaml
spec:
  operations:
    - name: create service account user
      id: my_app_user
      module: "google_cloudsql_user"
      # ... inputs ...

    - name: grant permissions on my_table
      id: my_app_grant
      module: "postgres_grant"
      inputs:
        # ... other inputs ...
        role:
          fromDependency:
            id: my_app_user
            output: user
        table: "my_table"
        permission: "SELECT"
```

The first time this workflow runs, the `my_app_user` operation will likely succeed. If the
application has not deployed and created `my_table` yet, the `my_app_grant` operation can fail
during its `Check` or `Set` step because the table does not exist.

This is fine. The workflow run will be marked as failed, but it has made progress.

When the workflow runs again after the application has successfully deployed and run its migrations,
the `my_app_grant` operation will find the table, and the `Set` step will succeed in applying the
grant. The workflow will then be fully reconciled.

## Example: Waiting for an External Secret

Another common scenario involves dependencies on resources created by other systems within a
Kubernetes cluster. For instance, a tool like cert-manager can automatically provision TLS
certificates and store them as Kubernetes Secrets.

A Blackstart workflow might be responsible for consuming this secret, for example, to configure
another service to use TLS encryption.

1.  **External System (cert-manager)**: A `Certificate` resource is created. Cert-manager sees this
    and generates a private key and certificate, storing them in a `Secret` named `my-app-tls`.
2.  **Blackstart Workflow**: An operation in a workflow needs to read `my-app-tls` to configure a
    component.

When the Blackstart workflow runs, the secret may not exist yet. If cert-manager has not created the
`my-app-tls` secret, the Blackstart operation that needs it will fail its `Check` step.

On a subsequent run, after cert-manager has done its job, the `Check` will find the secret, and the
workflow will proceed to use it, completing successfully. This model allows different teams and
systems to work independently, relying on Blackstart's periodic, eventually consistent runs to
stitch everything together correctly over time.

## Restart Behavior

Blackstart stores workflow run status on the `Workflow` object, including `status.lastRan`. On
controller restart, Blackstart recomputes schedule from status using:

`now >= lastRan + reconcileInterval`

If a workflow is overdue at startup, it is queued immediately.
