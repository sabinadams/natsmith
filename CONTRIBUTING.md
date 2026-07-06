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
make docs-dev    # fetch latest release version + http://localhost:3000
```

Or:

```bash
cd website && npm install && npm run dev
```

`npm install` runs `patch-package` to apply a one-line fix for a Layout validation bug in `nextra-theme-docs@4.6.1` (fixed upstream, not yet in npm).

Build static export locally (same as GitHub Pages):

```bash
make docs-build
```

Or `cd website && NEXT_PUBLIC_BASE_PATH=/natsmith npm run build` (runs `scripts/fetch-release-version.sh` automatically via `prebuild`).

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
| `.github/workflows/docs.yml` | Docs site build and GitHub Pages deploy |
| `scripts/fetch-release-version.sh` | Resolves latest published release for docs builds |
| `scripts/install.sh` | End-user install script (also published at `website/public/install.sh`) |

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

User-facing documentation is built with [Nextra](https://nextra.site/) in `website/` and published to [GitHub Pages](https://sabinadams.github.io/natsmith/) by the [Deploy docs](.github/workflows/docs.yml) workflow. Enable **Settings → Pages → Build and deployment → GitHub Actions** if the site is not live yet.

The workflow runs on every push to `main`, when a [GitHub release is published](#releasing), and on manual **workflow_dispatch**.

### Docs version resolution

Pinned install examples (e.g. `go install …@v1.2.3`) are **not** edited by hand. Before each docs build, [`scripts/fetch-release-version.sh`](scripts/fetch-release-version.sh) calls the GitHub API (`releases/latest`) and writes `website/lib/version.generated.js` (gitignored). MDX components on the [Install](https://sabinadams.github.io/natsmith/install/) page read that file.

Use `make docs-dev` / `make docs-build` from the repo root, or let `npm run dev` / `npm run build` run the script via `predev` / `prebuild`.

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

Publishing a semver tag is the **only** release step. You do not edit version strings in the repo, bump doc pins, or update the install script.

### What you do

1. Merge changes to `main` and confirm [CI](.github/workflows/ci.yml) is green.
2. Open **Releases** → **Draft a new release** on GitHub.
3. **Choose a tag** → type a new [semver](https://semver.org/) tag (e.g. `v1.2.3`) targeting `main`. Tags must start with `v`.
4. Set the title (usually the tag), write release notes, and click **Publish release**.

That is the full maintainer workflow. Optional: upgrade locally with `brew update && brew upgrade natsmith`.

### What happens automatically

| Piece | When it updates | How |
|-------|-----------------|-----|
| Git tag | You publish the release | Created in GitHub UI (or `git push origin v1.2.3`) |
| Release binaries + `checksums.txt` | Tag push | [Release workflow](.github/workflows/release.yml) → [GoReleaser](.goreleaser.yaml) |
| [Homebrew formula](https://github.com/sabinadams/homebrew-natsmith) | Tag push | GoReleaser `brews` block |
| [Install script](website/public/install.sh) | Every user run | Fetches `releases/latest` from GitHub at runtime — no repo change |
| `go install …@latest` | After tag is available | Go module proxy |
| Docs pinned examples | Release published or push to `main` | [Deploy docs](.github/workflows/docs.yml) → `fetch-release-version.sh` at build time |

After you publish a release, watch the **Release** workflow on [Actions](https://github.com/sabinadams/natsmith/actions) (~3 minutes). Verify the [GitHub release](https://github.com/sabinadams/natsmith/releases) has archives and the Homebrew tap shows the new version.

### What you do **not** need to do

- Edit `install.mdx`, README, or other docs to bump a version
- Maintain a `VERSION` file
- Copy binaries by hand
- Update the Homebrew formula manually (GoReleaser opens/updates `homebrew-natsmith`)

Code-only changes on `main` redeploy docs but still show the latest **published** release until you cut a new one.

### GitHub UI (preferred)

1. Open **Releases** → **Draft a new release**.
2. **Choose a tag** → type a new tag (e.g. `v1.2.3`) targeting `main`.
3. Set the title (usually the tag) and write release notes.
4. Click **Publish release**.

Publishing creates the tag and triggers GoReleaser. Write release notes in the UI — the workflow attaches binaries afterward.

### Alternative: push a tag locally

Equivalent to the UI flow; useful if you prefer the terminal:

```bash
git tag -a v1.2.3 -m "v1.2.3"
git push origin v1.2.3
```

GoReleaser creates or updates the GitHub release when the tag lands.

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

Documented on the [Install](https://sabinadams.github.io/natsmith/install/) page.

| Method | Command | Version source |
|--------|---------|----------------|
| Install script | `curl -fsSL https://sabinadams.github.io/natsmith/install.sh \| sh` | GitHub API at runtime |
| Homebrew | `brew install sabinadams/natsmith/natsmith` | Formula updated by GoReleaser |
| GitHub Releases | [github.com/sabinadams/natsmith/releases](https://github.com/sabinadams/natsmith/releases) | Tag you published |
| Go | `go install github.com/sabinadams/natsmith/cmd/natsmith@latest` | Go module proxy |
| Docs pin examples | See [Install](https://sabinadams.github.io/natsmith/install/) | `releases/latest` at docs build time |

The install script in [`scripts/install.sh`](scripts/install.sh) is copied to [`website/public/install.sh`](website/public/install.sh) for GitHub Pages hosting. Keep both in sync when changing the script.
