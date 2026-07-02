# natsmith

> CLI tooling for NATS and JetStream. Currently: migrate KV buckets and object stores between clusters — with verification built in.

[![Documentation](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://sabinadams.github.io/natsmith/)
[![Go](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/sabinadams/natsmith?sort=semver)](https://github.com/sabinadams/natsmith/releases)

Unofficial CLI for [NATS](https://nats.io) and JetStream. Not affiliated with Synadia.

## Documentation

**Full docs:** [sabinadams.github.io/natsmith](https://sabinadams.github.io/natsmith/)

- [Install](https://sabinadams.github.io/natsmith/install/)
- [Commands](https://sabinadams.github.io/natsmith/commands/)
- [migrate kv](https://sabinadams.github.io/natsmith/commands/migrate/kv/)
- [migrate objects](https://sabinadams.github.io/natsmith/commands/migrate/objects/)
- [Troubleshooting](https://sabinadams.github.io/natsmith/troubleshooting/)

## Install

**Install script** (macOS & Linux, no Go required):

```bash
curl -fsSL https://sabinadams.github.io/natsmith/install.sh | sh
```

**Homebrew:**

```bash
brew install sabinadams/natsmith/natsmith
```

**Go** (requires Go 1.25+):

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
```

See the [Install](https://sabinadams.github.io/natsmith/install/) page for GitHub Release downloads and other options.

## Quick start

```bash
natsmith migrate kv -dry-run \
  -source-url nats://source.example.com:4222 \
  -source-creds /path/to/source.creds \
  -dest-url nats://dest.example.com:4222 \
  -dest-creds /path/to/dest.creds
```

## Commands

| Command | Description |
|---------|-------------|
| `natsmith migrate kv` | Copy KV buckets and verify |
| `natsmith migrate objects` | Copy object store buckets |

Destination buckets must already exist. natsmith is read-only on source and does not delete extra destination data.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Documentation source lives in [`website/`](website/).

## License

MIT — see [LICENSE](LICENSE).
