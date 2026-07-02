# Contributing to natsmith

Thanks for helping improve natsmith. This document covers local development, CI expectations, and how maintainers publish releases.

## Getting started

**Requirements:** Go 1.25+ (see `go.mod`).

```bash
git clone git@github.com:sabinadams/natsmith.git
cd natsmith
make install   # installs to $(go env GOPATH)/bin
```

Or build into `./bin` without installing globally:

```bash
make build
export PATH="$PWD/bin:$PATH"
```

Run either command locally:

```bash
migrate-nats-kv -h
migrate-nats-objects -h
```

**Credentials:** Never commit `.creds` files or other secrets. Keep cluster credentials outside the repo.

## Project layout

| Path | Purpose |
|------|---------|
| `cmd/migrate-nats-kv/` | KV migration CLI |
| `cmd/migrate-nats-objects/` | Object store migration CLI |
| `internal/migrate/` | Shared connection, listing, verification, and progress logic |
| `.goreleaser.yaml` | Release build configuration |
| `.github/workflows/ci.yml` | PR and push checks |
| `.github/workflows/release.yml` | Tag-triggered release builds |

## Running tests locally

```bash
go build ./...
go vet ./...
go test -race -count=1 ./...
```

Check formatting and static analysis (same as CI):

```bash
gofmt -s -l .
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
go mod tidy && git diff --exit-code go.mod go.sum
```

Integration tests spin up an embedded NATS server; no external cluster is required for `go test`.

## Pull requests

1. Branch from `main`.
2. Keep changes focused — match existing naming, structure, and error-handling style.
3. Add or update tests for behavior changes.
4. Ensure CI passes before requesting review.

CI runs three jobs on every push and pull request to `main`:

| Job | Checks |
|-----|--------|
| **Test** | `go build`, `go vet`, race-enabled tests |
| **Lint** | `gofmt`, staticcheck |
| **Modules** | `go mod verify`, tidy check |

## Publishing releases

Releases are created through the **GitHub UI**, not by pushing tags from a local machine.

### Steps

1. Merge changes to `main` and confirm CI is green.
2. Open the repo on GitHub → **Releases** → **Draft a new release**.
3. Click **Choose a tag** → type a new semver tag (e.g. `v0.1.1`). Tags must start with `v` to match the release workflow.
4. Set the release title (often the same as the tag) and write release notes.
5. Click **Publish release**.

### What happens next

Publishing creates the tag on GitHub. That triggers the [Release workflow](.github/workflows/release.yml), which runs [GoReleaser](https://goreleaser.com/) to:

1. Run `go mod tidy` and `go test ./...`
2. Cross-compile both commands for linux, darwin, and windows (amd64 and arm64)
3. Attach platform archives and `checksums.txt` to the GitHub release you just created

Write release notes in the UI before publishing. The workflow adds binaries afterward — it does not replace your notes.

### Release artifacts

Each platform archive includes both `migrate-nats-kv` and `migrate-nats-objects`.

| Platform | Archive format | Example filename |
|----------|----------------|------------------|
| macOS / Linux | `.tar.gz` | `natsmith_0.1.1_darwin_arm64.tar.gz` |
| Windows | `.zip` | `natsmith_0.1.1_windows_amd64.zip` |

Users install from releases as documented in the [README](README.md#option-3-pre-built-binaries). Go users can also `go install` or `go run` against the new tag once it exists.

### Validate GoReleaser config locally (optional)

```bash
go run github.com/goreleaser/goreleaser/v2@v2.8.1 check
```

Build a local snapshot (outputs to `./dist`, gitignored):

```bash
go run github.com/goreleaser/goreleaser/v2@v2.8.1 build --snapshot --clean
```

## How users install and update

End-user install paths (`go install`, `go run`, pre-built binaries) are documented in the [README](README.md#install). Contributors do not need to do anything extra beyond publishing a release — `go install …@latest` and GitHub release downloads pick up new tags automatically.
