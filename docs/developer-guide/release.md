<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Release Process

Release Please maintains a release pull request from conventional commits merged into `main`. The
release pull request updates `CHANGELOG.md` and the release manifest. Merging it creates a version
tag and a draft GitHub release.

1. Merge release changes into `main` using conventional commit titles.
2. Review the Release Please pull request, including its proposed version and changelog.
3. Merge the Release Please pull request after its required checks pass.
4. Open GitHub: `Releases` and review the draft release created by Release Please.
5. Click `Publish release`.

Publishing the draft release triggers the artifact release workflow, which publishes the container
image and Helm chart. It also triggers the documentation deployment.

## Automation Flow

The release process uses three GitHub Actions workflows:

1. `Release Please` runs after changes merge into `main`. It creates or updates one release pull
   request containing the proposed version, release notes, `CHANGELOG.md`, and
   `.release-please-manifest.json`.
2. Merging the release pull request runs `Release Please` again. It creates the version tag and a
   draft GitHub release. It does not publish release artifacts.
3. Publishing the draft GitHub release runs `Release`. That workflow builds and publishes the
   container image, uploads the Helm chart release asset, updates the `latest` container tag when
   appropriate, and starts the Pages documentation deployment.

Release Please continues updating the same open release pull request as additional changes merge
into `main`. Merge that pull request only when its proposed changelog and version are ready for a
release.

## Release Pull Request Chart Preview

The `Release PR Chart Preview` workflow runs when the Release Please pull request creates or updates
`.release-please-manifest.json`. It reads the proposed version from the manifest, packages the local
Helm chart with that version, and merges it into a preview of the published Helm repository index.

Review the workflow result before merging the release pull request:

- The `Build Helm Chart Index Preview` check must pass.
- The generated index must contain the proposed chart version.
- The `chart-index-preview-<version>` workflow artifact can be downloaded to inspect the complete
  generated `index.yaml`.

The preview validates the future chart repository entry only. Publishing the draft release creates
and uploads the actual Helm chart asset.

## Version Selection

Release Please calculates the next version from conventional commits:

| Commit type       | Before `v1.0.0` | Starting with `v1.0.0` |
| ----------------- | --------------- | ---------------------- |
| `fix:`            | Patch           | Patch                  |
| `feat:`           | Patch           | Minor                  |
| Breaking change   | Minor           | Major                  |
| Other commit type | No version bump | No version bump        |

Multiple commits are collected into one release pull request and produce only one version bump.

To override the calculated version, include a `Release-As` footer in a commit merged to `main`:

```text
chore: prepare release

Release-As: 0.2.0
```

## Notes

- Releases use semantic tags with a `v` prefix, for example `v0.4.0`.
- Release Please currently creates stable draft releases only.
- Release notes on the docs site link directly to `https://github.com/pezops/blackstart/releases`.
- Helm chart packages are uploaded as GitHub release assets. The chart `index.yaml` is generated on
  Pages from published release assets; it is not stored in the repository.
