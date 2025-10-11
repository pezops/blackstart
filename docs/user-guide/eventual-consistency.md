# Eventual Consistency

Blackstart is designed with the principle of **eventual consistency** at its core. This means that
workflows are not expected to complete in a single run. Instead, they are designed to be executed
periodically, making incremental progress until the entire system reaches the desired state defined
in your workflow YAML.

This approach provides immense flexibility and resilience, especially in modern, distributed
environments where different components may come online or be updated at different times.

## How it Works: Periodic Runs

When a Blackstart workflow runs, it executes each [operation](./workflows.md#operations) in the
dependency graph. For each operation, it performs a [Check then Set](./workflows.md#check-then-set)
process.

- If an operation's `Check` step passes, it means that part of the system is already in the desired
  state, and Blackstart moves on.
- If the `Check` step fails, Blackstart executes the `Set` step to bring the resource to the desired
  state.

A workflow may not fully complete in one run. An operation might fail because it's waiting on an
external dependency that isn't ready yet. This is not an error in Blackstart; it's an expected part
of the process. On the next scheduled run, the workflow will try again. Hopefully, the dependency is
now available, and the workflow can make more progress.

A workflow is considered "fully reconciled" when a run completes with every operation passing its
`Check` step, and no `Set` operations are performed. At this point, the real-world infrastructure
matches the state declared in your workflow.

## Example: Waiting for a Database Table

A common use case is managing database permissions. Imagine your application's deployment process is
responsible for creating its own database tables via a schema migration tool. Your Blackstart
workflow is responsible for creating a service account and granting it permissions on one of those
tables.

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

The first time this workflow runs, the `my_app_user` operation will likely succeed. However, if the
application hasn't deployed and created `my_table` yet, the `my_app_grant` operation will fail
during its `Check` or `Set` step because the table does not exist.

This is fine. The workflow run will be marked as failed, but it has made progress.

When the workflow runs again after the application has successfully deployed and run its migrations,
the `my_app_grant` operation will find the table, and the `Set` step will succeed in applying the
grant. The workflow will then be fully reconciled.

## Example: Waiting for an External Secret

Another common scenario involves dependencies on resources created by other systems within a
Kubernetes cluster. For instance, you might use a tool like cert-manager to automatically provision
TLS certificates and store them as Kubernetes Secrets.

A Blackstart workflow might be responsible for consuming this secret, for example, to configure a
some other service to use TLS encryption.

1.  **External System (cert-manager)**: A `Certificate` resource is created. Cert-manager sees this
    and generates a private key and certificate, storing them in a `Secret` named `my-app-tls`.
2.  **Blackstart Workflow**: An operation in your workflow needs to read `my-app-tls` to configure a
    component.

When the Blackstart workflow runs, it's a race. If cert-manager has not yet created the `my-app-tls`
secret, the Blackstart operation that needs it will fail its `Check` step.

On a subsequent run, after cert-manager has done its job, the `Check` will find the secret, and the
workflow will proceed to use it, completing successfully. This model allows different teams and
systems to work independently, relying on Blackstart's periodic, eventually consistent runs to
stitch everything together correctly over time.
