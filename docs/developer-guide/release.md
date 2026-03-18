<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Release Process

Use this procedure when publishing a release.

1. Make sure all release changes are merged into `main`.
2. Open GitHub: `Actions` -> `Release Drafter` -> `Run workflow`.
3. Enter the release tag (for example `v0.4.0`) and run the workflow.
4. Wait for the workflow to:
   - regenerate `CHANGELOG.md` with that release tag header
   - commit the changelog update to `main`
   - create or update the GitHub draft release for that tag
5. Open GitHub: `Releases` -> review/edit draft release notes if needed.
6. Click `Publish release`.

## Notes

- Use semantic tags with a `v` prefix (for example `v0.4.0`).
- Release notes on the docs site link directly to: `https://github.com/pezops/blackstart/releases`.
- Helm chart packages are uploaded as GitHub release assets. The chart `index.yaml` is generated on
  Pages from published release assets; it is not stored in the repository.
