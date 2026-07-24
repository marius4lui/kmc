# KMC documentation

KMC 2 is a native Go command center for project commands and trusted local
workflows. The current public release line is beta.

## Start here

- [Installation](INSTALLATION.md) — install the beta, select a release channel,
  update, diagnose, pin, or remove KMC.
- [Commands and configuration](COMMANDS.md) — use the CLI, structure
  `kmc.json`, configure preferences, and import project commands.
- [KMC Scripts](WORKFLOWS.md) — define, validate, trust, and run YAML workflows.
- [Dev URLs](DEV-URLS.md) — configure stable local HTTPS URLs through Caddy.
- [Migration from Node.js to Go](MIGRATION-TO-GO.md) — replace the legacy npm
  package without losing project data.

## Maintainer guides

- [Installer and release operations](INSTALLER-OPERATIONS.md)
- [Contributing](../CONTRIBUTING.md)
- [Security policy](../SECURITY.md)
- [Changelog](../CHANGELOG.md)

## Configuration files

| Path | Ownership | Purpose |
| --- | --- | --- |
| `kmc.json` | Project, usually committed | Shared command groups and commands |
| `.kmc/scripts.yml` | Project, usually committed | Workflow registry |
| `.kmc/scripts/*.yml` | Project, usually committed | Workflow definitions |
| `.kmc/settings.json` | User-local | Menu preferences and favorites |
| `.kmc/dev.json` | User-local | Selected Dev URL project, host, and port |
| OS KMC config directory | User-local | Update channel, trust store, Caddy sites |

Workflow files contain executable shell commands. Review them before granting
repository trust.
