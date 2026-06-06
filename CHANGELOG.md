# Changelog

## [0.2.0](https://github.com/pezops/blackstart/compare/v0.1.15...v0.2.0) (2026-06-06)


### ⚠ BREAKING CHANGES

* Add support for and default to read-only Kubernetes ConfigMap and Secret values ([#86](https://github.com/pezops/blackstart/issues/86))

### 🚀 Features

* Add `crypto` modules for public and private key support ([#88](https://github.com/pezops/blackstart/issues/88)) ([1120341](https://github.com/pezops/blackstart/commit/11203417f3ee560d74ccc42f1a8228eb0d4d59e3))
* Add support for and default to read-only Kubernetes ConfigMap and Secret values ([#86](https://github.com/pezops/blackstart/issues/86)) ([7b978ad](https://github.com/pezops/blackstart/commit/7b978ad79c3c50f4a8992c284a7bfe63706b59c4))
* Add X.509 modules for certificate management ([#89](https://github.com/pezops/blackstart/issues/89)) ([372ef00](https://github.com/pezops/blackstart/commit/372ef00411dfdee476aadb698b16513e1e4c74d9))

## [0.1.15](https://github.com/pezops/blackstart/compare/v0.1.14...v0.1.15) (2026-06-05)


### 🚀 Features

* Add `mysql_connection` and `mysql_grant` modules ([#83](https://github.com/pezops/blackstart/issues/83)) ([981d2c8](https://github.com/pezops/blackstart/commit/981d2c806490b9ca7e75a373672f5e16de8410c1))
* Add CloudSQL for MySQL support ([#78](https://github.com/pezops/blackstart/issues/78)) ([77b0b7c](https://github.com/pezops/blackstart/commit/77b0b7c1f873f47bc64824edcce9834694aa1f7f))
* Add google_cloudsql_database module ([#81](https://github.com/pezops/blackstart/issues/81)) ([3e7b3b6](https://github.com/pezops/blackstart/commit/3e7b3b68105f5288800978adfb44e38539eeaeda))


### 📚 Documentation

* Update newly exported SVG diagrams, and update documentation ([#85](https://github.com/pezops/blackstart/issues/85)) ([e779bc8](https://github.com/pezops/blackstart/commit/e779bc8ae69c79f2cf5a2dbefce5355fa87dba93))


### 👷 Continuous Integration

* Switch release automation to Release Please ([#76](https://github.com/pezops/blackstart/issues/76)) ([8315de1](https://github.com/pezops/blackstart/commit/8315de1caf00a716f2d8a6f3f91c798064a9a73c))


### 🧪 Testing

* Add improved live test logging ([#82](https://github.com/pezops/blackstart/issues/82)) ([ccda840](https://github.com/pezops/blackstart/commit/ccda840779796e0d1c8ebeae1b6e0f45894a1256))
* Add live integration tests for Cloud SQL for MySQL ([#80](https://github.com/pezops/blackstart/issues/80)) ([4085fb1](https://github.com/pezops/blackstart/commit/4085fb19b56d692ac3cca3bf38c4169e0ae46c4a))
* Add mock tests and improved test coverage to Google CloudSQL module ([#79](https://github.com/pezops/blackstart/issues/79)) ([8a15a9e](https://github.com/pezops/blackstart/commit/8a15a9ed0382a81509e974b081903c435f44f57c))


### ⚙️ Miscellaneous Tasks

* Update dependencies ([#84](https://github.com/pezops/blackstart/issues/84)) ([039f1bb](https://github.com/pezops/blackstart/commit/039f1bba8f7c62cccd7b524afe54dc2e6b66786c))

## [0.1.14] - 2026-06-04

### 🐛 Bug Fixes

- Fix user from CloudSQL credential setup (#71) by @mbrancato
- Add `userinfo.email` scope to GCP access token (#72) by @mbrancato
- Version missing in rebuild (#74) by @mbrancato

### 📚 Documentation

- Add terraform deploy docs for GCP Cloud Run (#75) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Add automatic rebuild of the latest release (#73) by @mbrancato

## [0.1.13] - 2026-04-29

### 🐛 Bug Fixes

- Fix operation decode for scalars from config file (#70) by @mbrancato

## [0.1.12] - 2026-04-29

### 🚀 Features

- Add support for loading config from GCS or an environment variable (#68) by @mbrancato

### ⚙️ Miscellaneous Tasks

- Update dependencies (#69) by @mbrancato

## [0.1.11] - 2026-04-21

### 🚀 Features

- Add template module (#67) by @mbrancato

### 🐛 Bug Fixes

- Ignore unsupported Postgres privileges on older versions of Postgres (#66) by @mbrancato

## [0.1.10] - 2026-04-20

### 🐛 Bug Fixes

- Check for duplicate operation IDs in workflows (#59) by @mbrancato
- Add Closer interface for Modules to cleanup resources after workflow run (#65) by @mbrancato
- Normalize scope names for default privileges (#60) by @mbrancato

### 💼 Other

- Sanitize logs with Google API Errors when wrapped (#61) by @mbrancato

### 📚 Documentation

- Add Cloud SQL grant limitation docs for `cloudsqlsuperuser` (#62) by @mbrancato
- Normalize Cloud SQL naming to match official documentation (#63) by @mbrancato
- Normalize PostgreSQL naming to match official documentation (#64) by @mbrancato

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
