# Changelog

## [Unreleased]

### 🐛 Bug Fixes

- Check for duplicate operation IDs in workflows (#59) by @mbrancato

## [0.1.9] - 2026-03-27

### 🐛 Bug Fixes

- Fix static input validation on Postgres default privileges (#58) by @mbrancato

## [0.1.8] - 2026-03-27

### 🚀 Features

- Add Postgres default ACL management for advanced objects (#57) by @mbrancato

## [0.1.7] - 2026-03-27

### 🚀 Features

- Add support for Postgres grant management on `ALL TABLES` in a schema (#52) by @mbrancato
- Add support for Postgres grant scope on sequences (#53) by @mbrancato
- Add support for Postgres grants on functions / procedures (#54) by @mbrancato
- Add Postgres grant support for advanced object scopes and grant options (#55) by @mbrancato
- Add Postgres default ACL management (#56) by @mbrancato

## [0.1.6] - 2026-03-26

### 🐛 Bug Fixes

- Add improved validation for quoted Postgres identifiers (#51) by @mbrancato

## [0.1.5] - 2026-03-26

### 🐛 Bug Fixes

- Detect correct Workload Identity principals and normalize IAM service-account usernames (#50) by @mbrancato

### 📚 Documentation

- Add support for module requirements section in documentation (#49) by @mbrancato

## [0.1.4] - 2026-03-25

### 🚀 Features

- Add flexible module input types (#45) by @mbrancato
- Add support for multiple inputs for postgres grants (#46) by @mbrancato
- Add `google_cloud_metadata` module (#47) by @mbrancato

### 🐛 Bug Fixes

- Add missing `database` input to `google_cloudsql_managed_instance` (#48) by @mbrancato

## [0.1.3] - 2026-03-21

### 📚 Documentation

- Cleanup Postgres module docs (#40) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Fix Helm chart empty index publishing (#41) by @mbrancato

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
