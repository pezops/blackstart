<span class="mkdocs-hidden">&larr; [Developer Guide](README.md)</span>

# Building

This project uses `make` as the primary developer interface for build, generation, lint, test, and
docs workflows.

## Building with `go build`

You can build the `blackstart` binary directly without `make`:

```sh
go build -o blackstart ./cmd/blackstart
```

This produces a `blackstart` executable in the repository root.

## Primary Make Targets

Run these from the repository root:

| Target            | What it does                                                            |
| ----------------- | ----------------------------------------------------------------------- |
| `make blackstart` | Builds the `blackstart` binary (`./cmd/blackstart`).                    |
| `make crds`       | Regenerates CRDs and API deepcopy code for supported API versions.      |
| `make docs`       | Regenerates module docs, formats docs, and refreshes docs requirements. |
| `make lint`       | Runs generation + lint checks.                                          |
| `make test`       | Runs tests (depends on lint).                                           |
| `make build`      | Full pipeline: utils, CRDs, docs, lint, and test.                       |
| `make docs-serve` | Serves docs locally with MkDocs.                                        |

## Typical Development Flows

Quick local binary build:

```sh
make blackstart
```

Regenerate docs + CRDs after API/module changes:

```sh
make crds
make docs
```

Run full local validation before opening a PR:

```sh
make build
```

Install `pre-commit` and enable hooks:

```sh
pip install pre-commit
pre-commit install
```

This is optional but recommended. Running hooks before each commit helps catch lint failures before
they fail in CI.
