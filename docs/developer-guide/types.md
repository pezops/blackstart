<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Core Types

## Operation

The `Operation` struct is a configuration for a module to implement and is a single step in the
overall workflow. Each operation carries metadata about the module it uses, the inputs it requires,
and its dependencies.

<!-- prettier-ignore-start -->
??? abstract "Operation"
    ```go
    --8<-- "operation.go:Operation"
    ```
<!-- prettier-ignore-end -->

## Input

All inputs are passed to the operation's module implementations as an `Input` type. There are
several methods that provide scalar values of the input. The `Auto` method attempts to auto-detect a
scalar value from the input. For complex types, the `Any` method returns an `interface{}` that can
be type-asserted to the desired type. The module must implement the assertion and type validation
when using `Any` to retrieve the input.

When validating inputs, modules must check if the input is static using the `IsStatic` method. Only
static values may be checked for compatability with the module in the `Validate` method. Values that
are not static are only available at runtime.

<!-- prettier-ignore-start -->
??? abstract "Input"
    ```go
    --8<-- "module.go:Input"
    ```
<!-- prettier-ignore-end -->

## ModuleContext

The `ModuleContext` is a runtime context passed to `Check` and `Set` methods. Modules use their
context to read inputs, write outputs in the workflow. Additionally, the `ModuleContext` can be used
as a `context.Context` to propagate cancellation signals to the module and its dependencies.

In practice, `ModuleContext` is the module's handle to the current operation execution:

- Read resolved inputs with `Input(key)`. This includes static inputs and dependency-provided inputs
  that are only available at runtime.
- Write operation outputs with `Output(key, value)` so downstream operations can consume them via
  `fromDependency`.
- Inspect operation mode flags with `DoesNotExist()` and `Tainted()` to adjust behavior for delete
  and force-reconcile scenarios.
- Honor cancellation and deadlines via `Done()`, `Err()`, and `Deadline()` when making API calls.

Modules should treat `ModuleContext` as the single runtime contract for operation state and data
flow. Avoid module-global mutable state and prefer context-aware clients so retries and periodic
runs remain safe and predictable.

<!-- prettier-ignore-start -->
??? abstract "ModuleContext"
    ```go
    --8<-- "module.go:ModuleContext"
    ```
<!-- prettier-ignore-end -->
