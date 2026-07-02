# natsmith

Unofficial CLI toolkit for [NATS](https://nats.io) and JetStream. Not affiliated with Synadia — this is not a replacement for the official [`nats` CLI](https://github.com/nats-io/natscli).

## Tools

| Command | Description |
|---------|-------------|
| `migrate-nats-kv` | Copy KV buckets between clusters and verify |
| `migrate-nats-objects` | Copy object store buckets between clusters |

Both tools are read-only on source. They copy matching records to destination; they do **not** delete destination keys or objects absent from source.

## Prerequisites

- Go 1.25+ (to install from source)
- Network access to both NATS clusters
- `.creds` files with **read** access on source and **write** access on destination
- **Destination buckets must already exist** (e.g. via Terraform) — neither migration tool creates or updates bucket config

## Install

```bash
go install github.com/sabinadams/natsmith/cmd/migrate-nats-kv@latest
go install github.com/sabinadams/natsmith/cmd/migrate-nats-objects@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`.

Pin a release:

```bash
go install github.com/sabinadams/natsmith/cmd/migrate-nats-kv@v0.1.0
go install github.com/sabinadams/natsmith/cmd/migrate-nats-objects@v0.1.0
```

Verify:

```bash
migrate-nats-kv -h
migrate-nats-objects -h
```

### Development

```bash
git clone git@github.com:sabinadams/natsmith.git
cd natsmith
make install   # or: make build && export PATH="$PWD/bin:$PATH"
```

## Connection flags

Pass these four flags on **every** command:

| Flag | Example |
|------|---------|
| `-source-url` | `nats://source.example.com:4222` |
| `-source-creds` | `/path/to/source.creds` |
| `-dest-url` | `nats://dest.example.com:4222` |
| `-dest-creds` | `/path/to/dest.creds` |

## migrate-nats-kv

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
migrate-nats-kv -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

**Migrate all buckets:**

```bash
migrate-nats-kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -failures-file kv-failures.log
```

**Migrate one bucket:**

```bash
migrate-nats-kv \
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
migrate-nats-kv -verify-only \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16
```

**Resume an interrupted run:**

```bash
migrate-nats-kv \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -skip-existing \
  -workers 16
```

## migrate-nats-objects

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
migrate-nats-objects -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16
```

**Migrate all buckets:**

```bash
migrate-nats-objects \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -workers 16 \
  -timeout 5m
```

**Migrate one bucket:**

```bash
migrate-nats-objects \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds \
  -bucket my-bucket \
  -workers 16 \
  -timeout 5m
```

## Recommended workflow

1. **Dry-run KV** — `migrate-nats-kv -dry-run …`
2. **Dry-run objects** — `migrate-nats-objects -dry-run …`
3. **Migrate one KV bucket** — `-bucket my-bucket`, then `-verify-only` on the same bucket
4. **Full KV migration** — all buckets with `-failures-file`
5. **Full object migration** — all buckets with `-timeout 5m`

## Troubleshooting

| Problem | Likely cause |
|---------|--------------|
| `command not found` | Ensure `$(go env GOPATH)/bin` is on your `PATH` |
| `connect to …` | Wrong URL, network, or credentials |
| `destination bucket not found` | Create the bucket on destination first (Terraform) |
| KV scan / object scan appears stuck | Large stream; progress updates every 250 messages — wait, or filter with `-bucket` |
| Object store reports omitted | Meta exists on source but object data is not retrievable (deleted/tombstone) |
| Object copy failures / `context canceled` | Increase `-timeout` (e.g. `-timeout 5m`) and/or lower `-workers` |
| Verify reports missing/mismatch | Re-run `migrate-nats-kv -verify-only -failures-file kv-failures.log …` |
| Verify reports dest-only | Extra keys on destination not in source migratable set |
| Link migration fails | Linked bucket/object not migrated yet |

## License

MIT — see [LICENSE](LICENSE).
