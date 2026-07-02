# Contributing to natsmith

Thanks for helping improve natsmith. This document covers local development, CI expectations, and how maintainers publish releases.

## Getting started

**Requirements:** Go 1.25+ (see `go.mod`).

```bash
git clone git@github.com:sabinadams/natsmith.git
cd natsmith
make install   # installs natsmith to $(go env GOPATH)/bin
```

Or build into `./bin` without installing globally:

```bash
make build
export PATH="$PWD/bin:$PATH"
```

Run commands locally:

```bash
natsmith migrate kv -h
natsmith migrate objects -h
```

**Credentials:** Never commit `.creds` files or other secrets. Keep cluster credentials outside the repo.

## Architecture

This section is the source of truth for how code is organized. Read it before adding commands or moving logic between packages.

### CLI → code mapping

Only paths under `cmd/` mirror what users type:

| User command | Code |
|--------------|------|
| `natsmith` | `cmd/root.go` |
| `natsmith migrate` | `cmd/migrate/migrate.go` (group + shared flags) |
| `natsmith migrate kv` | `cmd/migrate/kv.go` |
| `natsmith migrate objects` | `cmd/migrate/objects.go` |

Packages under `internal/` describe **JetStream features or shared libraries**, not CLI subcommands. For example, `internal/kv/` is “KV bucket operations,” not “the kv subcommand.”

### Orchestration vs real work

```
cmd/migrate/kv.go          orchestration — flags, workflow, progress wiring, exit codes
    ↓ calls
internal/kv/               real work — scan streams, copy keys, verify values
internal/migration/        shared — config, cluster connect, summary, exit codes
internal/nats/, progress/, workpool/   generic libraries
```

**Belongs in `cmd/` (orchestration):**

- Cobra command definitions and flags
- Building config from flags (`sharedBaseConfig`, `migration.NewKVConfig`, …)
- The order of steps: connect → list buckets → for each bucket: scan → copy → verify
- When to start/finish progress bars and which phase label to show
- Aggregating per-bucket results into `migration.Summary` and choosing the exit code

**Belongs in `internal/` (real work):**

- Parsing JetStream streams and deriving bucket state (`kv.SnapshotFromStream`)
- Parallel copy and skip-existing logic (`kv.CopyBucket`, `objects.CopyBucket`)
- Verification comparisons (`kv.VerifyMigratable`)
- Listing/filtering buckets (`kv.ListBuckets`, `objects.ListBuckets`)
- Reusable connection, progress, and worker-pool primitives

**Rule of thumb:** if you’re parsing `$KV.*` subjects, calling `dest.Put`, or reading stream messages, it belongs in `internal/<feature>/`. If you’re deciding *when* to call those functions and *what to do* when one fails, it stays in `cmd/`.

### Internal package layout

```
internal/migration/     config.go, summary.go, errors.go, cluster.go, run.go
internal/kv/            buckets.go, snapshot.go, copy.go, verify.go, report.go
internal/objects/       buckets.go, snapshot.go, filter.go, copy.go, report.go
internal/nats/          NATS/JetStream connection
internal/progress/      stderr progress UI
internal/workpool/      parallel worker pool
internal/testutil/      test helpers
```

| File | Responsibility |
|------|----------------|
| `buckets.go` | List/filter buckets for migration |
| `snapshot.go` | Derive bucket state from backing streams |
| `copy.go` | Write records to destination |
| `verify.go` | Compare source vs destination (KV only) |
| `filter.go` | Object-specific filtering helpers |
| `report.go` | Formatted status lines consumed by `cmd/` (not business logic) |

`report.go` holds user-visible message strings so orchestration files stay focused on control flow. It does not perform JetStream I/O.

## Project layout

```
cmd/natsmith/main.go          ← binary entrypoint (calls cmd.Execute())
cmd/root.go                   ← root cobra command
cmd/migrate/                  ← "natsmith migrate …"
  migrate.go                  ← group + shared flags
  kv.go
  objects.go

internal/nats/                ← NATS/JetStream connection (reusable)
internal/workpool/            ← parallel worker pool (reusable)
internal/progress/            ← stderr progress UI (reusable)
internal/migration/           ← shared config, cluster connect, summary, exit codes
  config.go, summary.go, errors.go, cluster.go, run.go
internal/kv/                  ← KV buckets.go, snapshot.go, copy.go, verify.go, report.go
internal/objects/             ← objects buckets.go, snapshot.go, filter.go, copy.go, report.go

internal/testutil/            ← test helpers
```

Only paths under `cmd/` mirror CLI commands (`natsmith migrate kv`, etc.). Packages under `internal/` are implementation — folder names describe domain or libraries, not user-facing subcommands.

| Path | Purpose |
|------|---------|
| `cmd/natsmith/` | Binary entrypoint |
| `cmd/` | Root command and top-level group registration |
| `cmd/<group>/` | One folder per command group (e.g. `migrate`, future `inspect`) |
| `internal/nats/`, `internal/workpool/`, `internal/progress/` | Reusable libraries — any future command can import these |
| `internal/migration/` | Shared migration config, cluster connection, summary output, exit codes |
| `internal/kv/`, `internal/objects/` | JetStream feature libraries — see [Architecture](#architecture) |
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

Cross-cluster migration tests use [Testcontainers](https://golang.testcontainers.org/) to run separate NATS Docker instances. They require Docker and are gated behind the `integration` build tag:

```bash
go test -tags=integration -count=1 -timeout=10m ./internal/integration/ ./cmd/migrate/
```

CI runs integration tests in a dedicated step after unit tests.

## Adding a command

### New subcommand under an existing group (e.g. `migrate`)

1. Add reusable logic under `internal/<feature>/` following the file layout in [Architecture](#architecture).
2. Add `cmd/migrate/<name>.go` with flags, orchestration (`run<Name>`), and Cobra wiring. Call into `internal/<feature>/` for all JetStream work; use `migration.ConnectClusters` and `migration.CompleteRun` for shared workflow steps.

   ```go
   func init() {
       migrateCmd.AddCommand(<name>Cmd)
   }
   ```

No edits to `cmd/root.go` are needed for migrate subcommands.

### New top-level command group (e.g. `inspect`)

1. Create `cmd/inspect/` with `inspect.go` (group + shared flags) and subcommand files.
2. Put domain logic in `internal/inspect/` (or another package — not necessarily under `migrate`).
3. Register the group in `cmd/root.go`:

   ```go
   rootCmd.AddCommand(inspectcmd.Command())
   ```

Do not add an `internal/cli/` layer. Cobra wiring and command orchestration belong in `cmd/`; reusable JetStream logic belongs in `internal/<feature>/`; cross-command helpers belong in `internal/migration/`, `internal/nats/`, `internal/progress/`, etc.

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
2. Cross-compile the `natsmith` binary for linux, darwin, and windows (amd64 and arm64)
3. Attach platform archives and `checksums.txt` to the GitHub release you just created

Write release notes in the UI before publishing. The workflow adds binaries afterward — it does not replace your notes.

### Release artifacts

Each platform archive contains the `natsmith` binary.

| Platform | Archive format | Example filename |
|----------|----------------|------------------|
| macOS / Linux | `.tar.gz` | `natsmith_0.1.1_darwin_arm64.tar.gz` |
| Windows | `.zip` | `natsmith_0.1.1_windows_amd64.zip` |

Users install from releases as documented in the [README](README.md#option-3-pre-built-binaries). Go users can also `go install github.com/sabinadams/natsmith/cmd/natsmith@latest` once the tag exists.

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
