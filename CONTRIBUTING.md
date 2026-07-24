# Contributing to KMC

Thanks for helping improve KMC. Focused bug fixes, documentation corrections,
tests, and well-scoped feature proposals are welcome.

KMC 2 is currently beta software. Discuss large behavior or configuration
changes in an issue before investing in an implementation.

## Development setup

Requirements:

- Go 1.24 or newer;
- Git;
- POSIX shell for `scripts/install.sh` checks;
- PowerShell for `scripts/install.ps1` checks when available.

Clone and verify:

```sh
git clone https://github.com/marius4lui/kmc.git
cd kmc
go mod download
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
```

Run the CLI from source:

```sh
go run ./cmd/kmc
```

Build a development binary:

```sh
go build -o ./dist/kmc ./cmd/kmc
./dist/kmc --version
```

## Project layout

| Path | Responsibility |
| --- | --- |
| `cmd/kmc` | Native CLI entry point |
| `internal/cli` | Direct commands and interactive terminal UI |
| `internal/config` | `kmc.json` and settings |
| `internal/importers` | Project command discovery |
| `internal/workflows` | Workflow loading and validation |
| `internal/runner` | Workflow process execution |
| `internal/trust` | Repository fingerprint trust |
| `internal/devurls` | Local HTTPS project discovery and Caddy config |
| `internal/update` | Release resolution, verification, and self-update |
| `scripts` | Native installers |
| `site` | Static project website |
| `docs` | User and maintainer guides |

## Pull requests

1. Keep each pull request focused on one change.
2. Add or update tests for behavior changes.
3. Run the test, vet, and formatting checks.
4. Update user documentation when commands, configuration, or output changes.
5. Add a changelog entry for user-visible changes.
6. Do not commit generated binaries, release archives, or secrets.

Pull requests run tests on Go 1.24 and the current stable Go release, validate
both installers, cross-compile all supported targets, and create a GoReleaser
snapshot.

## Compatibility

Existing `kmc.json`, settings, Dev URL state, workflow definitions, and trust
records are user-owned data. Changes to these formats need an explicit migration
path and compatibility tests.

## Security

Do not open public issues for suspected vulnerabilities. Follow
[SECURITY.md](SECURITY.md).
