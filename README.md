# natsmith

Unofficial CLI toolkit for [NATS](https://nats.io) and JetStream. Not affiliated with Synadia — this is not a replacement for the official [`nats` CLI](https://github.com/nats-io/natscli).

## Tools

| Command | Description |
|---------|-------------|
| `natsmith migrate kv` | Copy KV buckets between clusters and verify |
| `natsmith migrate objects` | Copy object store buckets between clusters |

Both tools are read-only on source. They copy matching records to destination; they do **not** delete destination keys or objects absent from source.

## Prerequisites

- Network access to both NATS clusters
- `.creds` files with **read** access on source and **write** access on destination
- **Destination buckets must already exist** (e.g. via Terraform) — neither migration tool creates or updates bucket config

Go 1.25+ is required for `go install` and `go run`. Pre-built [release binaries](#option-3-pre-built-binaries) do not require Go.

## Install

These are **global CLI tools** — like `nx` or the official `nats` CLI. They are not dependencies of your project. Install once (or run ad hoc) and invoke them from any directory.

### Option 1: Global install (recommended)

Add Go's bin directory to your `PATH` (once per shell profile):

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Install `natsmith`:

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
```

Pin a specific release:

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@v0.1.0
```

**Private repository:** configure Go to fetch this module directly:

```bash
go env -w GOPRIVATE=github.com/sabinadams/natsmith
```

You also need GitHub SSH or HTTPS credentials that can read the repo. Public clones can skip `GOPRIVATE`.

Verify:

```bash
natsmith migrate kv -h
natsmith migrate objects -h
```

### Option 2: Run without installing

Like `npx` — no global install; pass the subcommand and flags after `--`:

```bash
go run github.com/sabinadams/natsmith/cmd/natsmith@v0.1.0 -- migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -dry-run
```

Use `@latest` for the newest tag, or pin `@v0.1.0` for a specific release. The same `GOPRIVATE` setup applies for private repos.

### Option 3: Pre-built binaries

If you do not have Go installed, download an archive for your platform from [GitHub Releases](https://github.com/sabinadams/natsmith/releases). Each archive contains the `natsmith` binary. Verify downloads with `checksums.txt` on the release page.

Example (macOS arm64):

```bash
VERSION=v0.1.0
curl -LO "https://github.com/sabinadams/natsmith/releases/download/${VERSION}/natsmith_${VERSION#v}_darwin_arm64.tar.gz"
tar xzf "natsmith_${VERSION#v}_darwin_arm64.tar.gz"
chmod +x natsmith
sudo mv natsmith /usr/local/bin/
```

Windows releases are `.zip` archives. Put `natsmith.exe` somewhere on your `PATH`.

## Updating

Re-run `go install` with `@latest` or a new tag:

```bash
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
```

To see which module version built your installed binary:

```bash
go version -m "$(command -v natsmith)"
```

For pre-built binaries, download the newer release archive and replace the files on your `PATH`.

## Contributing

Development setup, CI, release instructions, and the **code architecture** (how `cmd/` and `internal/` are split) are in [CONTRIBUTING.md](CONTRIBUTING.md#architecture).

## Connection flags

Pass these four flags on **every** command:

| Flag | Example |
|------|---------|
| `-source-url` | `nats://source.example.com:4222` |
| `-source-creds` | `/path/to/source.creds` |
| `-dest-url` | `nats://dest.example.com:4222` |
| `-dest-creds` | `/path/to/dest.creds` |

## migrate kv

Scans each source KV bucket via its backing JetStream stream (`KV_<bucket>`) and classifies keys:

- **migratable** — latest op is Put (copied)
- **omitted** — tombstone/purge/delete (not copied)

Post-migration verification compares migratable keys on source and destination (byte-for-byte). Exit code is non-zero when keys are missing, values mismatch, or destination has extra keys.

**Success output:** `N/N migratable copied` and `verify: N ok, 0 missing, 0 mismatch, 0 dest-only`.

### Flags

In addition to the [connection flags](#connection-flags):

| Flag | Description |
|------|-------------|
| `-bucket` | Comma-separated buckets to migrate (default: all) |
| `-omit` | Comma-separated buckets to skip |
| `-dry-run` | List buckets and keys without writing |
| `-skip-existing` | Skip keys already on destination (resume interrupted runs) |
| `-workers` | Concurrent workers (1–64, default 1) |
| `-timeout` | Per-request NATS timeout (default: 30s) |
| `-no-progress` | Plain log output (useful in CI) |
| `-verify` | Verify after migration (default: true) |
| `-verify-only` | Verify only — no writes |
| `-failures-file` | Append verification issues to a log file |

Use `-bucket` to select buckets, `-omit` to exclude them, or both (`-bucket` first, then `-omit` removes from that set).

### Usage

**Dry-run all buckets:**

```bash
natsmith migrate kv -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

**Migrate all buckets:**

```bash
natsmith migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -failures-file kv-failures.log
```

**Migrate one bucket:**

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

**Verify only:**

```bash
natsmith migrate kv -verify-only \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16
```

**Resume an interrupted run:**

```bash
natsmith migrate kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -skip-existing \
  -workers 16
```

## migrate objects

Copies object blobs and recreates link objects in a second pass.

- Opens existing destination buckets only — does **not** create buckets or set bucket descriptions
- Scans the full `OBJ_<bucket>` stream for meta messages, then probes each candidate with `GetInfo`
- Objects with meta but no retrievable data are **omitted** (common on legacy buckets)
- Each object copy uses at least a 5-minute timeout; raise `-timeout` for large files
- Lower `-workers` (e.g. 8) for buckets with very large objects

**Dry-run output:** `listed`, `meta-active`, `meta-omitted` per bucket.

**Full run output:** `listed`, `migratable`, `omitted`, then `N/N copied` with optional `(N failed)` for copy errors.

### Flags

In addition to the [connection flags](#connection-flags):

| Flag | Description |
|------|-------------|
| `-bucket` | Comma-separated buckets to migrate (default: all) |
| `-omit` | Comma-separated buckets to skip |
| `-dry-run` | List buckets and objects without writing |
| `-skip-existing` | Skip objects already on destination (resume interrupted runs) |
| `-workers` | Concurrent workers (1–64, default 1) |
| `-timeout` | Per-request NATS timeout (default: 30s; use `5m` or higher for large files) |
| `-no-progress` | Plain log output (useful in CI) |

### Usage

**Dry-run all buckets:**

```bash
natsmith migrate objects -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

**Migrate all buckets:**

```bash
natsmith migrate objects \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -timeout 5m
```

**Migrate one bucket:**

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

## Recommended workflow

1. **Dry-run KV** — `natsmith migrate kv -dry-run …`
2. **Dry-run objects** — `natsmith migrate objects -dry-run …`
3. **Migrate one KV bucket** — `-bucket my-bucket`, then `-verify-only` on the same bucket
4. **Full KV migration** — all buckets with `-failures-file`
5. **Full object migration** — all buckets with `-timeout 5m`

## Troubleshooting

| Problem | Likely cause |
|---------|--------------|
| `command not found` | Add `$(go env GOPATH)/bin` to your `PATH`, or use [pre-built binaries](#option-3-pre-built-binaries) |
| `connect to …` | Wrong URL, network, or credentials |
| `destination bucket not found` | Create the bucket on destination first (Terraform) |
| KV scan / object scan appears stuck | Large stream; progress updates every 250 messages — wait, or filter with `-bucket` |
| Object store reports omitted | Meta exists on source but object data is not retrievable (deleted/tombstone) |
| Object copy failures / `context canceled` | Increase `-timeout` (e.g. `-timeout 5m`) and/or lower `-workers` |
| Verify reports missing/mismatch | Re-run `natsmith migrate kv -verify-only -failures-file kv-failures.log …` |
| Verify reports dest-only | Extra keys on destination not in source migratable set |
| Link migration fails | Linked bucket/object not migrated yet |

## License

MIT — see [LICENSE](LICENSE).
