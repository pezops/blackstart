# Developer Guide

In order to provide extensibility, Blackstart uses a module system. Modules provide a framework for
developers to add support for new resources. This guide will walk through the concepts and
requirements for modules.

<span class="mkdocs-hidden">
Table of Contents
</span>

## Operation

The `Operation` struct is a configuration for a module to implement and is a single step the overall
workflow. Each operation carries metadata about the module it uses, the inputs it requires, and its
dependencies.

<!-- prettier-ignore-start -->
??? abstract "Operation"
    ```go
    --8<-- "operation.go:Operation"
    ```
<!-- prettier-ignore-end -->

## Input

All inputs are passed to module implementations as a `Input` type. There are several methods that
provide scalar values of the input. The `Auto` method attempts to auto-detect a scalar value from
the input. For complex types, the `Any` method returns an `interface{}` that can be type-asserted to
the desired type. The module must implement the assertion and type validation when using `Any` to
retrieve the input.

When validating inputs, modules must check if the input is static using the `IsStatic` method. Only
static values are able to be checked for compatability with the module in the `Validate` method.
Values that are not static are only available at runtime.

<!-- prettier-ignore-start -->
??? abstract "Input"
    ```go
    --8<-- "module.go:Input"
    ```
<!-- prettier-ignore-end -->

## ModuleContext

The `ModuleContext` is a runtime context passed to `Check` and `Set` methods.
