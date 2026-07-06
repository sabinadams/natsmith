# Contributing to natsmith

Thanks for helping improve natsmith. This document covers local development, CI expectations, and how maintainers publish releases.

## Getting started

**Requirements:** Go 1.25+ (see `go.mod`).

```bash
git clone git@github.com:sabinadams/natsmith.git
cd natsmith
make install   # installs natsmith to $(go env GOPATH)/bin
```

Docs site ([Nextra 4](https://nextra.site/) + Next.js 16):

```bash
cd website && npm install && npm run dev   # http://localhost:3000
```

`npm install` runs `patch-package` to apply a one-line fix for a Layout validation bug in `nextra-theme-docs@4.6.1` (fixed upstream, not yet in npm).

Or build static export locally (same as GitHub Pages):

```bash
cd website && NEXT_PUBLIC_BASE_PATH=/natsmith npm run build
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
internal/kv/               real work — ListKeys migration, copy, verify
internal/migration/        shared — config, cluster connect, summary, exit codes
internal/nats/, progress/, workpool/   generic libraries
```

**Belongs in `cmd/` (orchestration):**

- Cobra command definitions and flags
- Building config from flags (`sharedBaseConfig`, `migration.NewKVConfig`, …)
- The order of steps: connect → list buckets → for each bucket: list keys → copy → verify
- When to start/finish progress bars and which phase label to show
- Aggregating per-bucket results into `migration.Summary` and choosing the exit code

**Belongs in `internal/` (real work):**

- Listing and parallel copy/verify of live KV keys (`kv.RunBucket`)
- Listing/filtering buckets (`kv.ListBuckets`, `objects.ListBuckets`)
- Parallel object copy (`objects.CopyBucket`)
- Reusable connection, progress, and worker-pool primitives

**Rule of thumb:** if you’re calling `ListKeys`, `dest.Put`, or comparing KV values, it belongs in `internal/<feature>/`. If you’re deciding *when* to call those functions and *what to do* when one fails, it stays in `cmd/`.

### Internal package layout

```
internal/migration/     config.go, cluster.go, summary.go, buckets.go
internal/kv/            buckets.go, run_bucket.go, stream.go, verify.go
internal/objects/       buckets.go, snapshot.go, filter.go, copy.go
internal/report/        shared stderr constants (KindKV, KindObjectStore)
internal/nats/          conn.go, context.go, runctx.go — connect + NATS CLI contexts
internal/progress/      stderr progress UI (session, bars, plan, output modes)
internal/workpool/      parallel worker pool
internal/testutil/      unit test helpers
internal/integration/   cross-cluster integration test helpers
```

| File | Responsibility |
|------|----------------|
| `buckets.go` | List/filter buckets for migration |
| `run_bucket.go` | ListKeys → Get → Put migration pipeline |
| `stream.go` | JetStream snapshot backup/restore |
| `verify.go` | Verify results and failure file output |

Per-bucket stderr output goes through [`internal/progress`](internal/progress/) (`Session`, progress bars). `internal/report/` holds shared kind labels only.

## Project layout

```
cmd/natsmith/main.go          ← binary entrypoint (calls cmd.Execute())
cmd/root.go                   ← root cobra command
cmd/migrate/                  ← "natsmith migrate …"
  migrate.go                  ← group + shared flags
  kv.go
  objects.go
cmd/backup/                   ← "natsmith backup …"
  backup.go, backup_test.go
  kv.go, kv_test.go
cmd/restore/                  ← "natsmith restore …"
  restore.go, restore_test.go
  kv.go, kv_test.go

internal/nats/                ← connect + NATS CLI context loading (conn.go, context.go, runctx.go)
internal/workpool/            ← parallel worker pool (reusable)
internal/progress/            ← stderr progress UI (reusable)
internal/migration/           ← shared config, cluster connect, summary, exit codes, bucket filtering
  config.go, cluster.go, summary.go, buckets.go
internal/report/              ← shared stderr kind labels (KV, object store)
internal/kv/                  ← buckets.go, run_bucket.go, stream.go, verify.go
internal/objects/             ← buckets.go, snapshot.go, filter.go, copy.go

internal/testutil/            ← unit test helpers
internal/integration/         ← integration test cluster helpers
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

Cross-cluster tests use [Testcontainers](https://golang.testcontainers.org/) (requires Docker, `-tags=integration`):

```bash
go test -tags=integration -count=1 -timeout=10m ./internal/integration/ ./cmd/migrate/
```

CI runs integration tests in a dedicated step after unit tests.

User-facing documentation is built with [Nextra](https://nextra.site/) in `website/` and published to [GitHub Pages](https://sabinadams.github.io/natsmith/) on every push to `main`. Enable **Settings → Pages → Build and deployment → GitHub Actions** if the site is not live yet.

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

### Progress UI (all commands)

Use [`internal/progress`](internal/progress/) for stderr output:

1. `session := progress.NewSession(progress.SessionConfig{Title: "...", NoProgress: cfg.NoProgress})`
2. `session.PrintPlan(...)` after listing buckets
3. `session.Status(...)` for connection phases
4. Per bucket — `session.BeginBucket()` then **one bar at a time**:
   - `StartIndeterminate` — scan/list phases (migrate)
   - `StartBucket` — counted item copy (objects)
   - `StartTransferTracked` — byte transfer (backup/restore)
5. `session.BucketInfo` / `BucketSuccessStats` / `BucketCopied` / `BucketFail` / `BucketWarning`
6. `session.Completef(exitCode, ...)` or `migration.CompleteRun(..., session)` for the footer

Global flags on the root command:

- `--quiet` — errors and final summary only
- `--json` — structured NDJSON events on stdout

Respect `NO_COLOR`, non-TTY stderr, and `--no-progress`.

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

## Releasing

Publishing a semver tag triggers the [Release workflow](.github/workflows/release.yml). GoReleaser builds binaries, attaches them to the GitHub release, and updates the [Homebrew formula](https://github.com/sabinadams/homebrew-natsmith). **No doc version bumps are required** — user-facing install docs use `@latest` or resolve the latest tag from GitHub.

### Checklist

1. Merge changes to `main` and confirm [CI](.github/workflows/ci.yml) is green.
2. Choose the next [semver](https://semver.org/) tag (`vMAJOR.MINOR.PATCH`). Tags must start with `v`.
3. Publish the release (preferred: **GitHub UI** — steps below).
4. Watch the **Release** workflow on the [Actions](https://github.com/sabinadams/natsmith/actions) tab (~3 minutes).
5. Verify the [GitHub release](https://github.com/sabinadams/natsmith/releases) has archives + `checksums.txt`, and the Homebrew tap shows the new version.
6. Upgrade locally: `brew update && brew upgrade natsmith`.

### GitHub UI (preferred)

1. Open **Releases** → **Draft a new release**.
2. **Choose a tag** → type a new tag (e.g. `v1.2.3`) targeting `main`.
3. Set the title (usually the tag) and write release notes.
4. Click **Publish release**.

Publishing creates the tag and triggers GoReleaser. Write release notes in the UI — the workflow adds binaries afterward.

### Alternative: push a tag locally

Equivalent to the UI flow; useful if you prefer the terminal:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GoReleaser creates the GitHub release when the tag lands.

### What GoReleaser does

On tag push (`v*`):

1. Runs `go mod tidy` and `go test ./...`
2. Cross-compiles for linux, darwin, and windows (amd64 and arm64)
3. Uploads platform archives and `checksums.txt` to the GitHub release
4. Updates `homebrew-natsmith/Formula/natsmith.rb`

Config: [`.goreleaser.yaml`](.goreleaser.yaml).

### Release artifacts

| Platform | Format | Example |
|----------|--------|---------|
| macOS / Linux | `.tar.gz` | `natsmith_1.2.3_darwin_arm64.tar.gz` |
| Windows | `.zip` | `natsmith_1.2.3_windows_amd64.zip` |

### Validate locally (optional)

```bash
go run github.com/goreleaser/goreleaser/v2@v2.8.1 check
go run github.com/goreleaser/goreleaser/v2@v2.8.1 build --snapshot --clean   # outputs to ./dist
```

### How users install

Documented on the [Install](https://sabinadams.github.io/natsmith/install/) page. The [install script](website/public/install.sh) (also in `scripts/install.sh`) resolves the latest GitHub release automatically.

| Method | Command |
|--------|---------|
| Install script | `curl -fsSL https://sabinadams.github.io/natsmith/install.sh \| sh` |
| Homebrew | `brew install sabinadams/natsmith/natsmith` |
| GitHub Releases | [github.com/sabinadams/natsmith/releases](https://github.com/sabinadams/natsmith/releases) |
| Go | `go install github.com/sabinadams/natsmith/cmd/natsmith@latest` |
