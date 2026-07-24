# Commands and configuration

KMC opens an interactive command center when started without arguments:

```sh
kmc
```

Arrow keys move through menus, Enter selects, and Space toggles items on
multi-select screens. Direct commands are available for automation and users
who already know the target id.

## CLI reference

| Command | Purpose |
| --- | --- |
| `kmc run <id>` | Run a saved command or registered workflow |
| `kmc add` | Add a command interactively |
| `kmc edit <id>` | Edit a command |
| `kmc delete <id>` | Delete a command |
| `kmc import` | Import commands from detected project files |
| `kmc validate` | Validate `kmc.json` and local settings |
| `kmc settings` | Manage groups, favorites, and the optional agent skill |
| `kmc scripts …` | Initialize, list, validate, or run YAML workflows |
| `kmc trust [status]` | Add or inspect repository workflow trust |
| `kmc untrust` | Remove repository workflow trust |
| `kmc dev …` | Detect, configure, start, or reload a Dev URL project |
| `kmc update [--check]` | Check for or install a native update |
| `kmc channel [set <name>]` | Show or select the update channel |
| `kmc doctor` | Print installation diagnostics |

Run `kmc help` for the compact built-in reference.

## Non-interactive command editing

```sh
kmc add \
  --name test \
  --command "go test ./..." \
  --group development \
  --cwd .

kmc edit development.test --command "go test -race ./..."
kmc delete development.test
```

## `kmc.json`

The project command center lives in `kmc.json` at the directory where KMC is
started. The grouped format is the current format:

```json
{
  "$schema": "https://github.com/marius4lui/kmc/blob/main/schema.json",
  "groups": [
    {
      "id": "development",
      "label": "Development",
      "description": "Local development commands",
      "icon": "",
      "type": "manual",
      "commands": [
        {
          "id": "development.test",
          "name": "test",
          "command": "go test ./...",
          "description": "Run all Go tests",
          "cwd": ".",
          "source": "manual",
          "imported": false
        }
      ]
    }
  ]
}
```

### Group fields

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | Yes | Stable technical id used in command paths |
| `label` | Yes | Display label |
| `description` | No | Human-readable purpose |
| `icon` | No | Optional display symbol |
| `type` | Yes | `manual`, `imported`, or `skill` |
| `source` | No | Source file for an imported group |
| `commands` | Yes | Commands in the group |

### Command fields

| Field | Required | Meaning |
| --- | --- | --- |
| `id` | No | Defaults to `<group>.<name>` |
| `name` | Yes | Short menu name |
| `command` | Yes | Shell command to execute |
| `description` | No | Text shown beside the command |
| `cwd` | No | Project-relative working directory; defaults to `.` |
| `source` | No | Source file or `manual` |
| `imported` | No | Whether an importer generated the command |

KMC runs commands through the system shell and inherits the current environment.
Treat committed `kmc.json` commands as executable project code.

## Legacy compatibility

KMC 2 still reads the legacy flat `commands` array, including the former
`group` and `groupId` properties. When KMC next writes the file it normalizes
the configuration into groups. User commands and working directories are
preserved.

## Importers

`kmc import` detects supported project files and refreshes imported groups
without deleting manually maintained commands.

| Source | Imported group |
| --- | --- |
| `package.json` | npm scripts |
| `Makefile` | Make targets |
| `pubspec.yaml` | Flutter commands |
| `docker-compose.yml` or `compose.yml` | Docker Compose services |

Node.js is required only when an imported Node command itself needs it; the KMC
binary has no Node.js runtime dependency.

## Settings

Personal menu preferences are stored in `.kmc/settings.json`:

```json
{
  "defaultGroup": "development",
  "lastSelectedGroup": "development",
  "hiddenGroups": [],
  "favoriteGroups": ["development"],
  "favoriteCommands": ["development.test"],
  "maxFavoriteCommands": 3
}
```

KMC adds `.kmc/` to `.gitignore` when it writes local settings. Workflow files
under `.kmc/` are intended to be shared, so use a narrow ignore policy if a
repository commits KMC Scripts.
