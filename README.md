# kmc

Interactive command launcher for project scripts.

`kmc` keeps useful project commands in a local `kmc.json` file and lets you run them from a clean terminal menu. It is useful for deploy scripts, local dev commands, migrations, build steps, maintenance tasks, or any other command you do not want to remember and retype.

## Install

```sh
npm install -g @marius4lui/kmc
```

The installed command is:

```sh
kmc
```

## Quick Start

Open any project folder and run:

```sh
kmc
```

If the project does not have a `kmc.json` yet, choose **Add command** in the menu. `kmc` creates the file automatically when the first command is saved.

Example entries:

- `deploy` -> `python deploy.py`
- `dev` -> `npm run dev`
- `migrate` -> `python manage.py migrate`
- `release` -> `npm run build && npm publish`

## How It Works

`kmc` is intentionally interactive. You do not need setup subcommands.

Run:

```sh
kmc
```

Then use the menu to:

- run a saved command
- add a new command
- edit an existing command
- delete a command
- import detected project commands
- adjust local settings
- quit

Use the arrow keys to move through the menu and press Enter to select. Press Esc to go back from nested screens.

## Direct Commands

The interactive menu is the default, but `kmc` can also be controlled directly:

```sh
kmc
kmc run deploy
kmc run manual.deploy
kmc run npm.dev
kmc add
kmc edit deploy
kmc delete deploy
kmc import
kmc validate
kmc settings
```

Direct commands are useful for power users, scripts, and CI.

## Import

`kmc import` opens an import screen. Detected sources are preselected, missing sources are shown as unavailable, and checkbox selection uses Space to toggle plus Enter to confirm.

| Source | Group |
| --- | --- |
| `package.json` | NPM Scripts |
| `Makefile` | Make Commands |
| `pubspec.yaml` | Flutter Commands |
| `docker-compose.yml` / `compose.yml` | Docker Commands |

Imported commands are not mixed into one flat menu. The run flow is grouped:

```text
Run command
├─ Manual Commands
├─ NPM Scripts
├─ Make Commands
├─ Flutter Commands
└─ Docker Commands
```

Manual commands are kept when importing. Previously imported commands are refreshed from their source files.

## Validate

`kmc validate` prints a readable report. It shows each check, what was checked, whether it passed, and a final summary. In the interactive menu the report waits for one Enter confirmation before returning.

## `kmc.json`

Commands are stored in `kmc.json` in the current working directory.

You normally manage this file through the CLI, but it is plain JSON and can be edited manually:

```json
{
  "$schema": "https://github.com/marius4lui/kmc/blob/main/schema.json",
  "commands": [
    {
      "name": "deploy",
      "id": "manual.deploy",
      "command": "python deploy.py",
      "description": "Deploy this project",
      "cwd": ".",
      "group": "manual",
      "source": "manual",
      "imported": false
    },
    {
      "name": "dev",
      "id": "npm.dev",
      "command": "npm run dev",
      "description": "Start the development server",
      "cwd": ".",
      "group": "npm",
      "source": "package.json",
      "imported": true
    }
  ]
}
```

### Fields

| Field | Required | Description |
| --- | --- | --- |
| `name` | yes | Short name shown in the menu. |
| `command` | yes | Shell command to execute. |
| `description` | no | Text shown next to the command in the menu. |
| `cwd` | no | Working directory relative to the folder where `kmc` was started. Defaults to `.`. |
| `id` | no | Stable id. Defaults to `<group>.<name>`. |
| `group` | no | Command group. Defaults to `manual`. |
| `source` | no | Source file or `manual`. |
| `imported` | no | Whether the command was generated from a project file. |

## Settings

Local user settings are stored in:

```text
.kmc/settings.json
```

`kmc` automatically adds `.kmc/` to `.gitignore` when settings are written, so each user can keep their own behavior without changing the team setup.

Example:

```json
{
  "defaultGroup": "npm",
  "lastSelectedGroup": "manual",
  "hiddenGroups": ["flutter"],
  "favoriteCommands": ["npm.dev", "manual.deploy"],
  "maxFavoriteCommands": 3
}
```

Favorites are selected in settings with Space and confirmed with Enter. The default maximum is `3`, and you can change it in settings.

## Examples

### Python deploy script

```json
{
  "name": "deploy",
  "command": "python deploy.py",
  "description": "Deploy production",
  "cwd": "."
}
```

### Node development server

```json
{
  "name": "dev",
  "command": "npm run dev",
  "description": "Start local app",
  "cwd": "."
}
```

### Backend command from a subfolder

```json
{
  "name": "migrate",
  "command": "python manage.py migrate",
  "description": "Run database migrations",
  "cwd": "backend"
}
```

## Local Development

Clone the repo:

```sh
git clone https://github.com/marius4lui/kmc.git
cd kmc
npm install
```

Run locally:

```sh
node ./bin/kmc.js
```

Link globally while developing:

```sh
npm link
kmc
```

If your shell says the command exists but is not executable, make sure the bin file has execute permissions:

```sh
chmod 755 bin/kmc.js
npm link
```

## Requirements

- Node.js 18 or newer
- A terminal with interactive prompt support

## Notes

- `kmc` runs commands through your system shell.
- Commands inherit your current environment variables.
- A command's `cwd` is resolved relative to the directory where you started `kmc`.
- `kmc.json` is project-local by design. Put it in the repo if the commands should be shared with the team.

## License

MIT
