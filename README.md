# natsmith

> Migrate JetStream KV buckets and object stores between NATS clusters — with verification built in.

Unofficial CLI toolkit for [NATS](https://nats.io) and JetStream. Not affiliated with Synadia; this is a focused migration tool, not a replacement for the official [`nats` CLI](https://github.com/nats-io/natscli).

[![Go](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/sabinadams/natsmith?sort=semver)](https://github.com/sabinadams/natsmith/releases)

---

## At a glance

| | |
|---|---|
| **Commands** | `natsmith migrate kv` · `natsmith migrate objects` |
| **Source** | Read-only — scans streams, never deletes source data |
| **Destination** | Copies matching records; does **not** remove extra keys or objects on dest |
| **Buckets** | Must already exist on destination (e.g. Terraform) — natsmith does not create or reconfigure them |
| **Credentials** | Source: read · Destination: write (`.creds` files) |

---

## Quick start

**Install** (requires Go 1.25+):

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
natsmith migrate kv -h
```

**Dry-run** a migration (no writes):

```bash
natsmith migrate kv -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds
```

Use the same connection flags for `migrate objects`. See [Install](#install) for pre-built binaries and private-repo setup.

---

## Commands

| Command | What it does |
|---------|--------------|
| [`natsmith migrate kv`](#migrate-kv) | Scan KV backing streams, copy migratable keys, verify byte-for-byte |
| [`natsmith migrate objects`](#migrate-objects) | Scan object meta streams, copy blobs and links |

Shared flags for both commands: [connection flags](#connection-flags) · `-bucket` · `-omit` · `-dry-run` · `-skip-existing` · `-workers` · `-timeout` · `-no-progress`

---

## How migration works

```text
  source cluster                         destination cluster
 ┌─────────────────┐                   ┌─────────────────┐
 │  KV / OBJ data  │  ── natsmith ──▶  │  existing       │
 │  (read-only)    │     copy +        │  buckets        │
 └─────────────────┘     verify          └─────────────────┘
```

1. Connect to source and destination
2. List buckets (optionally filter with `-bucket` / `-omit`)
3. Scan each bucket's JetStream stream to determine migratable records
4. Copy to destination (unless `-dry-run` or `-verify-only`)
5. Print a per-bucket summary and final totals

**Recommended order:** dry-run KV → dry-run objects → migrate one bucket → full migration. Details in [recommended workflow](#recommended-workflow).

---

## Install

natsmith is a **global CLI** — install once and run from any directory. It is not a project dependency.

### Option 1 — `go install` (recommended)

Add Go's bin directory to your `PATH` (once per shell profile):

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Install the latest release:

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
```

Pin a specific version:

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@v0.1.0
```

**Private repository** — fetch the module directly:

```bash
go env -w GOPRIVATE=github.com/sabinadams/natsmith
```

You also need GitHub credentials that can read the repo. Public clones can skip `GOPRIVATE`.

### Option 2 — Run without installing

Like `npx` — no global install; pass flags after `--`:

```bash
go run github.com/sabinadams/natsmith/cmd/natsmith@latest -- migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -dry-run
```

Use `@latest` or pin `@v0.1.0`. The same `GOPRIVATE` setup applies for private repos.

### Option 3 — Pre-built binaries

No Go required. Download an archive for your platform from [GitHub Releases](https://github.com/sabinadams/natsmith/releases). Each archive contains the `natsmith` binary. Verify downloads with `checksums.txt` on the release page.

**macOS (arm64) example:**

```bash
VERSION=v0.1.0
curl -LO "https://github.com/sabinadams/natsmith/releases/download/${VERSION}/natsmith_${VERSION#v}_darwin_arm64.tar.gz"
tar xzf "natsmith_${VERSION#v}_darwin_arm64.tar.gz"
chmod +x natsmith
sudo mv natsmith /usr/local/bin/
```

Windows releases are `.zip` archives — put `natsmith.exe` on your `PATH`.

### Updating

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
go version -m "$(command -v natsmith)"   # see which version is installed
```

For pre-built binaries, download the newer release and replace the file on your `PATH`.

---

## Connection flags

Required on **every** command:

| Flag | Example |
|------|---------|
| `-source-url` | `nats://source.example.com:4222` |
| `-source-creds` | `/path/to/source.creds` |
| `-dest-url` | `nats://dest.example.com:4222` |
| `-dest-creds` | `/path/to/dest.creds` |

**Tip:** export them once per shell session to shorten commands:

```bash
export NATS_SRC_URL=nats://source.example.com:4222
export NATS_SRC_CREDS=/path/to/source.creds
export NATS_DST_URL=nats://dest.example.com:4222
export NATS_DST_CREDS=/path/to/dest.creds

# then use -source-url "$NATS_SRC_URL" etc., or wrap in a small shell alias
```

---

## migrate kv

Scans each source KV bucket via its backing JetStream stream (`KV_<bucket>`) and classifies keys:

| Classification | Meaning |
|----------------|---------|
| **migratable** | Latest op is Put — copied to destination |
| **omitted** | Tombstone / purge / delete — not copied |

Post-migration verification (on by default) compares migratable keys byte-for-byte. Exit code is non-zero when keys are missing, values mismatch, or destination has extra keys.

**Success looks like:** `N/N migratable copied` and `verify: N ok, 0 missing, 0 mismatch, 0 dest-only`.

### KV-specific flags

| Flag | Default | Description |
|------|---------|-------------|
| `-verify` | `true` | Verify destination after migration |
| `-verify-only` | `false` | Compare source and dest without writing |
| `-failures-file` | — | Append verification issues to a log file |

Plus the [shared flags](#commands): `-bucket`, `-omit`, `-dry-run`, `-skip-existing`, `-workers`, `-timeout`, `-no-progress`.

Use `-bucket` to select buckets, `-omit` to exclude them, or both (`-bucket` first, then `-omit` removes from that set).

### Examples

<details>
<summary><strong>Dry-run all buckets</strong></summary>

```bash
natsmith migrate kv -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

</details>

<details>
<summary><strong>Migrate all buckets</strong></summary>

```bash
natsmith migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -failures-file kv-failures.log
```

</details>

<details>
<summary><strong>Migrate one bucket</strong></summary>

```bash
natsmith migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16 \
  -failures-file kv-failures.log
```

</details>

<details>
<summary><strong>Verify only (no writes)</strong></summary>

```bash
natsmith migrate kv -verify-only \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16
```

</details>

<details>
<summary><strong>Resume an interrupted run</strong></summary>

```bash
natsmith migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -skip-existing \
  -workers 16
```

</details>

---

## migrate objects

Copies object blobs and recreates link objects in a second pass.

| Behavior | Detail |
|----------|--------|
| Destination buckets | Opens existing buckets only — does not create or set descriptions |
| Scan | Full `OBJ_<bucket>` stream for meta, then `GetInfo` probe per candidate |
| Omitted objects | Meta exists but data is not retrievable (common on legacy buckets) |
| Timeouts | Each copy uses at least 5 minutes; raise `-timeout` for large files |
| Concurrency | Lower `-workers` (e.g. 8) for buckets with very large objects |

**Dry-run output:** `listed`, `meta-active`, `meta-omitted` per bucket.

**Full run output:** `listed`, `migratable`, `omitted`, then `N/N copied` with optional `(N failed)`.

### Object-specific notes

Uses the [shared flags](#commands) only — no verify mode (unlike KV).

### Examples

<details>
<summary><strong>Dry-run all buckets</strong></summary>

```bash
natsmith migrate objects -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

</details>

<details>
<summary><strong>Migrate all buckets</strong></summary>

```bash
natsmith migrate objects \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -timeout 5m
```

</details>

<details>
<summary><strong>Migrate one bucket</strong></summary>

```bash
natsmith migrate objects \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16 \
  -timeout 5m
```

</details>

---

## Recommended workflow

| Step | Command |
|------|---------|
| 1 | `natsmith migrate kv -dry-run …` — inspect KV buckets and key counts |
| 2 | `natsmith migrate objects -dry-run …` — inspect object stores |
| 3 | `natsmith migrate kv -bucket my-bucket …` — pilot one KV bucket |
| 4 | `natsmith migrate kv -verify-only -bucket my-bucket …` — confirm that bucket |
| 5 | `natsmith migrate kv … -failures-file kv-failures.log` — full KV migration |
| 6 | `natsmith migrate objects … -timeout 5m` — full object migration |

---

## Troubleshooting

| Problem | Likely cause |
|---------|--------------|
| `command not found` | Add `$(go env GOPATH)/bin` to your `PATH`, or use [pre-built binaries](#option-3--pre-built-binaries) |
| `connect to …` | Wrong URL, network, or credentials |
| `destination bucket not found` | Create the bucket on destination first (Terraform) |
| KV / object scan appears stuck | Large stream — progress updates every 250 messages; wait or filter with `-bucket` |
| Object store reports omitted | Meta on source but object data not retrievable (deleted / tombstone) |
| Object copy failures / `context canceled` | Increase `-timeout` (e.g. `5m`) and/or lower `-workers` |
| Verify reports missing / mismatch | Re-run `natsmith migrate kv -verify-only -failures-file kv-failures.log …` |
| Verify reports dest-only | Extra keys on destination not in source migratable set |
| Link migration fails | Linked bucket or object not migrated yet |

---

## Contributing

Development setup, CI, release instructions, and code architecture are in [CONTRIBUTING.md](CONTRIBUTING.md#architecture).

---

## License

MIT — see [LICENSE](LICENSE).
