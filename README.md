# natsmith

> CLI tooling for NATS and JetStream.

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

Pre-built installs do not require Go:

**Install script** (macOS & Linux):

```bash
curl -fsSL https://sabinadams.github.io/natsmith/install.sh | sh
```

**Homebrew** (macOS & Linux):

```bash
brew install sabinadams/natsmith/natsmith
```

**Go** (optional, requires Go 1.25+):

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
go install github.com/sabinadams/natsmith/cmd/natsmith@latest
```

See the [Install](https://sabinadams.github.io/natsmith/install/) page for GitHub Release downloads and other options.

## Quick start

See the [docs](https://sabinadams.github.io/natsmith/commands/migrate/) for command reference and examples.

## Commands

| Command | Description |
|---------|-------------|
| `natsmith migrate kv` | Copy KV buckets and verify |
| `natsmith migrate objects` | Copy object store buckets |

See [Commands](https://sabinadams.github.io/natsmith/commands/) for flags and the [migrate overview](https://sabinadams.github.io/natsmith/commands/migrate/) for production workflow.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Documentation source lives in [`website/`](website/).

## License

MIT — see [LICENSE](LICENSE).
