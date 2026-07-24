# KMC

**A native project command center for repeatable commands, trusted workflows,
and local development URLs.**

[![CI](https://github.com/marius4lui/kmc/actions/workflows/ci.yml/badge.svg)](https://github.com/marius4lui/kmc/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/marius4lui/kmc?include_prereleases&label=release)](https://github.com/marius4lui/kmc/releases)
[![Go](https://img.shields.io/github/go-mod/go-version/marius4lui/kmc)](go.mod)
[![License](https://img.shields.io/github/license/marius4lui/kmc)](LICENSE)

> [!IMPORTANT]
> KMC 2 is currently in **beta**. There is no KMC 2 stable release yet. The
> installer commands below opt in to the beta channel explicitly.

KMC turns the commands scattered across a repository into one terminal
interface. It can launch saved commands, import common project scripts, run
strict YAML workflows, and create memorable local HTTPS URLs. KMC 2 is a single
Go binary; Node.js is not required to install or run it.

## Install the beta

Linux and macOS:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta
```

Windows PowerShell:

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) -Channel beta
```

Verify the installation:

```sh
kmc --version
kmc doctor
```

The installer selects the correct Linux, macOS, or Windows archive for amd64 or
arm64, verifies its SHA-256 checksum, installs it atomically, and remembers the
selected update channel.

[Installation guide](docs/INSTALLATION.md) ·
[Latest beta release](https://github.com/marius4lui/kmc/releases) ·
[Migration from the npm version](docs/MIGRATION-TO-GO.md)

## Quick start

Open a project and launch KMC:

```sh
cd your-project
kmc
```

Use the menu to add a command, import commands from a supported project file, or
initialize KMC Scripts. A project command can also be added non-interactively:

```sh
kmc add \
  --name test \
  --command "go test ./..." \
  --group development

kmc run development.test
```

KMC stores shared commands in a plain `kmc.json` file:

```json
{
  "$schema": "https://github.com/marius4lui/kmc/blob/main/schema.json",
  "groups": [
    {
      "id": "development",
      "label": "Development",
      "type": "manual",
      "commands": [
        {
          "id": "development.test",
          "name": "test",
          "command": "go test ./...",
          "cwd": "."
        }
      ]
    }
  ]
}
```

Commit `kmc.json` when the command center should be shared with the team.
Personal preferences and Dev URL state live under `.kmc/`.

## What KMC provides

- **Interactive command launcher** — organize project commands into searchable
  groups and favorites.
- **Direct CLI commands** — run, add, edit, delete, import, and validate without
  opening the menu.
- **Project importers** — discover npm scripts, Make targets, Flutter commands,
  and Docker Compose services.
- **KMC Scripts** — run strict, sequential YAML workflows with environment
  variables, working directories, timeouts, retries, dry runs, and step
  selection.
- **Repository trust** — bind workflow approval to the project path and a
  SHA-256 fingerprint of the active workflow files.
- **Dev URLs** — place supported Next.js, Vite, NestJS, and Express apps behind
  stable local HTTPS addresses through Caddy.
- **Native lifecycle** — checksummed releases, channel-aware updates,
  diagnostics, version pinning, and uninstall support.

## Common commands

```text
kmc                                      Open the interactive command center
kmc run <id>                             Run a command or workflow
kmc add                                  Add a command interactively
kmc import                               Import detected project commands
kmc validate                             Validate kmc.json and settings
kmc scripts init                         Create a workflow starter
kmc scripts validate                     Validate the workflow registry
kmc run <workflow> --dry-run             Print a workflow plan
kmc trust status                         Inspect workflow trust
kmc update --check                       Check the selected release channel
kmc channel set beta                     Stay on beta updates
kmc doctor                               Show installation diagnostics
```

See [Commands and configuration](docs/COMMANDS.md) for the complete reference.

## Documentation

| Guide | Purpose |
| --- | --- |
| [Documentation index](docs/README.md) | Find the right guide quickly |
| [Installation](docs/INSTALLATION.md) | Beta install, updates, channels, rollback, and uninstall |
| [Commands and configuration](docs/COMMANDS.md) | CLI reference, `kmc.json`, settings, and importers |
| [KMC Scripts](docs/WORKFLOWS.md) | Workflow registry, schema, execution, and trust |
| [Dev URLs](docs/DEV-URLS.md) | Local HTTPS and monorepo setup |
| [Migration to Go](docs/MIGRATION-TO-GO.md) | Move safely from the legacy npm package |
| [Contributing](CONTRIBUTING.md) | Local development and pull requests |
| [Security policy](SECURITY.md) | Report vulnerabilities privately |

## Agent skill

This repository also contains an optional coding-agent skill in
[`SKILL.md`](SKILL.md). The skill teaches compatible agents how to inspect and
use a project's KMC command center safely.

```sh
npx skills add marius4lui/kmc
```

`npx` is used only by the external skill installer. The KMC CLI itself is a
native Go binary and has no Node.js runtime dependency.

## Beta status

The Go rewrite is available for testing on all supported platforms. During the
beta:

- releases may include breaking changes before `v2.0.0`;
- the `beta` channel is the supported installation and update path;
- existing `kmc.json`, `.kmc/settings.json`, `.kmc/dev.json`, and KMC Scripts
  files remain supported;
- the first stable release will be announced separately and will use the
  `stable` channel.

Please report reproducible problems with the
[bug report form](https://github.com/marius4lui/kmc/issues/new?template=bug_report.yml).

## License

[MIT](LICENSE)
