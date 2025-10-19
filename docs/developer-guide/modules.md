<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Modules

A module is an implementation of the `Module` interface. This interface defines the methods that a
module must implement in order to be used in a workflow. Additionally, there are a few requirements
for how a module must behave when implementing these methods.

<!-- prettier-ignore-start -->
???+ abstract "Module"
    ```go
    --8<-- "module.go:Module"
    ```
<!-- prettier-ignore-end -->

Each implementation should be specific to a task, and within a package hierarchy that makes sense
for the module. For example, modules for Postgres-related are in the `postgres` package.
Additionally, modules for Google Cloud are under multiple sub-packages under the `google` package.

<!-- prettier-ignore-start -->
???+ warning "Module Interface is Not Finalized"
    The `Module` interface is not finalized and may change significantly in the future.
<!-- prettier-ignore-end -->

## Importing a Module

All modules are currently developed in-tree. A more complex import, communication, and distribution
pattern would be needed to provide external modules. To add new modules to Blackstart, the module
must be imported for side-effects in the `internal/all_modules/all_modules.go` file.

## Validate

```go
Validate(op Operation) error
```

The `Validate` method is called before the `Check` and `Set` methods. This method validates the
operation provided. All modules are encouraged to check that all required inputs are present. All
inputs are passed to the module as a [`Input`](types.md#input) type. The `IsStatic` method can be
used to check if the input is static. Non-static values are only available at runtime and cannot be
checked in the `Validate` method.

Any [Operation](types.md#operation) that is not valid must return an error. This error will be
propagated up to the user.

During validation, more complex logic may be used to ensure the configuration is valid. For example,
modules may check that specific combinations of optional inputs are specified. However, since only
static inputs are available during validation, modules must also check the actual inputs at runtime
as well in the `Check` and `Set` methods.

## Check

```go
Check(ctx ModuleContext) (bool, error)
```

The `Check` method is called to determine if the operation needs to be run. This method must check
the current state of the system and return `true` if the system is in the desired state. If the
system is not in the desired state, the method must return `false`.

If the desired state is met, then the `Check` method must also set all outputs in the provided
[`ModuleContext`](types.md#modulecontext) that are expected to be returned by the module.

<!-- prettier-ignore-start -->
???+ warning "Check May Affect Idempotency"
    The `Check` method may need to inspect if an error encountered is a transient error or a permanent error. In other 
    words, there may be errors that are safe to encounter during the `Check` method that would be resolved when running 
    `Set`. If `Check` returns false with a transient error, there can be a situation where idempotency of the target 
    resource is broken. Be careful when implementing the `Check` method to ensure that the method is idempotent.

    Additionally, this pattern is still under review and may change in the future.
<!-- prettier-ignore-end -->

## Set

```go
Set(ctx ModuleContext) error
```

The `Set` method is called when the resource is not in the desired state. When run, the `Set` method
may need to not simply create a resource, but inspect it and change it to the desired state. After
configuring the resource, the `Set` method must set all outputs in the provided
[`ModuleContext`](types.md#modulecontext) that are expected to be returned by the module.
