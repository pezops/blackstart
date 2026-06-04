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

## Release Automation Setup

Release Please authenticates as a dedicated GitHub App so its pull requests trigger normal CI.
Install the App on this repository with these repository permissions:

- Contents: read and write
- Issues: read and write
- Pull requests: read and write

Configure these repository Actions values:

- Variable `RELEASE_PLEASE_APP_CLIENT_ID`: GitHub App client ID
- Secret `RELEASE_PLEASE_APP_PRIVATE_KEY`: GitHub App private key

## Notes

- Releases use semantic tags with a `v` prefix, for example `v0.4.0`.
- Release Please currently creates stable draft releases only.
- Release notes on the docs site link directly to `https://github.com/pezops/blackstart/releases`.
- Helm chart packages are uploaded as GitHub release assets. The chart `index.yaml` is generated on
  Pages from published release assets; it is not stored in the repository.
