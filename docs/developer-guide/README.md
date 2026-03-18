# Developer Guide

Blackstart is extended through modules. This guide is for developers implementing new modules or
maintaining existing ones.

## In This Guide

- [Modules](./modules.md): implementation contract and behavior expectations for `Validate`,
  `Check`, and `Set`.
- [Core Types](./types.md): runtime/configuration types used by module implementations.
- [Building](./building.md): build, generation, lint, test, and docs workflows.
- [Release Process](./release.md): drafting and publishing releases.

## Module Authoring Guidance

If you are adding a new module, this is the minimum path to a production-ready implementation.

### Before You Code

- Define module scope clearly: one module should manage one coherent resource concern.
- Decide authoritative behavior up front: what your module manages vs intentionally leaves alone.
- Design inputs/outputs first (`Info()`), then implement runtime behavior.

### Implementation Checklist

1. Create a package under `modules/<domain>/...`.
2. Implement `Info`, `Validate`, `Check`, and `Set`.
3. Register the module in `init()` using `RegisterModule("<module_id>", factory)`.
4. Import the package in `internal/all_modules/all_modules.go` for side-effect registration.
5. Add tests for validation, check/set behavior, idempotency, and outputs.
6. Run `make docs` to regenerate module docs.
7. Run `make build` before opening a PR.

### Runtime Expectations

- Keep `Check` read-only. If state cannot be determined reliably, return an error.
- Keep `Set` idempotent so repeated runs converge safely.
- On successful operations, populate any outputs that other operations may reference.
