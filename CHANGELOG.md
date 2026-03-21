# Changelog

## [0.1.2] - 2026-03-21

### 🚀 Features

- Add support for GKE and AWS workload identity in Helm chart (#37) by @mbrancato

### 📚 Documentation

- Add improved CloudSQL module example (#36) by @mbrancato
- Minor documentation cleanup (#38) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Prune missing helm chart versions on release (#35) by @mbrancato
- Publish latest tag with container releases (#39) by @mbrancato

## [0.1.1] - 2026-03-19

### 🚀 Features

- Add support for custom service account annotations (#34) by @mbrancato

## [0.1.0] - 2026-03-19

### 🚀 Features

- Add kubernetes controller runtime implementation (#31) by @mbrancato

### 🐛 Bug Fixes

- Release version flags, adjust workflow trigger (#15) by @mbrancato
- Move docs deploy to a workflow_run trigger (#16) by @mbrancato
- Remove duplicate release trigger (#17) by @mbrancato
- Pull helm index from release (#18) by @mbrancato
- Use common chart filename in release assets (#19) by @mbrancato
- Fix test waiting for database (#26) by @mbrancato
- Fix kubernetes configmap defaults, correct missing resource detection (#30) by @mbrancato

### 📚 Documentation

- Improve documentation, add draft workflow and changelog management (#22) by @mbrancato
- Minor docs updates for helm and before release (#33) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Reuse postgres test container (#20) by @mbrancato
- Remove helm chart (#21) by @mbrancato
- Update release/changelog workflows and action versions (#24) by @mbrancato
- Move helm chart back into repo pages (#25) by @mbrancato
- Fix args for git-cliff action (#27) by @mbrancato
- Add specific git ref for cosign action mising floating tag (#28) by @mbrancato
- Remove crane release workflow tool as a dependency (#29) by @mbrancato
- Dependency updates and prep for release (#32) by @mbrancato

### New Contributors

* @github-actions[bot] made their first contribution

## [0.0.0] - 2025-10-19

### 🚀 Features

- Add Kubernetes Secret Module (#7) by @mbrancato
- Add support for Immutable ConfigMaps (#9) by @mbrancato

### 🐛 Bug Fixes

- Remove YAML / JSON case differences to fix CRD deserialization (#6) by @mbrancato
- Use of pointer inputs for modules (#10) by @mbrancato
- Move coverage upload to its own job with permissions (#13) by @mbrancato

### 💼 Other

- Initial public code (#1) by @mbrancato
- Add Kubernetes ConfigMap module (#5) by @mbrancato
- Add support for value update policies in ConfigMaps and Secrets (#11) by @mbrancato
- Add Lint and Test workflow (#12) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Cleanup tests for Kubernetes modules (#8) by @mbrancato
- Add release workflow (#14) by @mbrancato

### New Contributors

* @mbrancato made their first contribution in (#1)
