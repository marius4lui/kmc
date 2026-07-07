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
- start stable local Dev URLs
- manage manual commands
- import detected project commands
- adjust local preferences
- quit

Use the arrow keys to move through the menu and press Enter to select. Press Esc to go back from nested screens.

On interactive starts, `kmc` checks npm for the latest published version. If a newer version is available, you can choose **Update now** to run:

```sh
npm install -g @marius4lui/kmc@latest
```

or choose **Skip** to continue with the current version for that run.

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

## Dev URLs

The **Dev URLs** screen creates stable local HTTPS URLs for supported web apps:

```text
https://my-app.kmc.localhost
```

Supported app types:

- Next.js
- Vite
- NestJS
- Express

`kmc` writes the local Dev URL settings to `.kmc/dev-url.json` and stores the generated Caddy config in:

```text
~/.config/kmc/Caddyfile
```

The menu can:

- start the selected project on its stored port
- reload Caddy
- install/trust Caddy's local HTTPS CA
- change the local name/host
- select a detected monorepo project
- set a project path manually

### Monorepos

Dev URLs work from monorepos. `kmc` searches upward for the workspace root and scans workspace packages plus common nested app folders.

Detected workspace markers include:

- `package.json` `workspaces`
- `pnpm-workspace.yaml`
- `lerna.json`
- `turbo.json`
- `nx.json`

This means you can run `kmc` from the repo root or from a nested app such as:

```text
apps/web/landing
```

Then choose **Dev URLs** -> **Select detected project**.

If your app is in an unusual location, choose **Change project path** and enter the folder that contains its `package.json`.

### Caddy

Dev URLs use Caddy as the local HTTPS reverse proxy. Install it first if `kmc` reports that `caddy` is missing:

```sh
sudo apt install caddy
```

Other common package managers:

```sh
sudo dnf install caddy
sudo pacman -S caddy
brew install caddy
```

If Caddy is installed but reload fails because no admin API is running, start it with:

```sh
caddy start --config ~/.config/kmc/Caddyfile
```

Then choose **Reload Caddy** again.

### Local HTTPS Trust

Browsers may show `net::ERR_CERT_AUTHORITY_INVALID` until Caddy's local CA is trusted.

Use:

```sh
caddy trust
```

or choose **Dev URLs** -> **Trust local HTTPS certs**.

Restart the browser if it still shows the warning after the CA is trusted.

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
Run
├─ Manual Commands
├─ NPM Scripts
├─ Make Commands
├─ Flutter
└─ Docker
```

Manual commands are kept when importing. Previously imported commands are refreshed from their source files.

## Validate

`kmc validate` prints a readable report. It shows each check, what was checked, whether it passed, and a final summary. In the interactive menu the report waits for one Enter confirmation before returning.

## `kmc.json`

Groups and commands are stored in `kmc.json` in the current working directory. Groups are the central organization layer in kmc.

You normally manage this file through the CLI, but it is plain JSON and can be edited manually:

```json
{
  "$schema": "https://github.com/marius4lui/kmc/blob/main/schema.json",
  "groups": [
    {
      "id": "deployment",
      "label": "Deployment",
      "description": "Release and deployment commands",
      "icon": "",
      "type": "manual",
      "commands": [
        {
          "id": "deployment.production",
          "name": "production",
          "command": "python deploy.py",
          "description": "Deploy production",
          "cwd": ".",
          "source": "manual",
          "imported": false
        }
      ]
    },
    {
      "id": "npm",
      "label": "NPM Scripts",
      "description": "Commands imported from package.json",
      "icon": "",
      "type": "imported",
      "source": "package.json",
      "commands": [
        {
          "id": "npm.dev",
          "name": "dev",
          "command": "npm run dev",
          "description": "Start the development server",
          "cwd": ".",
          "source": "package.json",
          "imported": true
        }
      ]
    }
  ]
}
```

Legacy flat `commands` files are still read and migrated when kmc writes the config again.

### Group Fields

| Field | Required | Description |
| --- | --- | --- |
| `id` | yes | Technical group id, used in command paths. |
| `label` | yes | Display name. |
| `description` | no | Optional group description. |
| `icon` | no | Optional display symbol. |
| `type` | yes | `manual`, `imported`, or `skill`. |
| `source` | no | Source file for imported groups. |
| `commands` | yes | Commands contained in the group. |

### Command Fields

| Field | Required | Description |
| --- | --- | --- |
| `id` | no | Stable command id. Defaults to `<group>.<name>`. |
| `name` | yes | Short name shown in the menu. |
| `command` | yes | Shell command to execute. |
| `description` | no | Text shown next to the command in the menu. |
| `cwd` | no | Working directory relative to the folder where `kmc` was started. Defaults to `.`. |
| `source` | no | Source file or `manual`. |
| `imported` | no | Whether the command was generated from a project file. |

Command paths are stable:

```sh
kmc run deployment.production
kmc run npm.build
kmc run flutter.run
```

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
  "favoriteGroups": ["development", "deployment"],
  "hiddenGroups": ["flutter"],
  "favoriteCommands": ["npm.dev", "manual.deploy"],
  "maxFavoriteCommands": 3
}
```

Favorite groups and favorite commands are selected in settings with Space and confirmed with Enter. The default maximum for favorite commands is `3`, and you can change it in settings.

### Agent Skill

`kmc` also ships an agent skill so AI coding agents can understand and use the command center in a repository.

Open:

```sh
kmc settings
```

Then choose **Install kmc skill**. The CLI runs:

```sh
npx skills add marius4lui/kmc
```

After installation, agents can invoke the skill as `$kmc`.

## Examples

### Python deploy script

```json
{
  "id": "deployment",
  "label": "Deployment",
  "type": "manual",
  "commands": [
    {
      "id": "deployment.production",
      "name": "production",
      "command": "python deploy.py",
      "description": "Deploy production",
      "cwd": "."
    }
  ]
}
```

### Node development server

```json
{
  "id": "development",
  "label": "Development",
  "type": "manual",
  "commands": [
    {
      "id": "development.dev",
      "name": "dev",
      "command": "npm run dev",
      "description": "Start local app",
      "cwd": "."
    }
  ]
}
```

### Backend command from a subfolder

```json
{
  "id": "database",
  "label": "Database",
  "type": "manual",
  "commands": [
    {
      "id": "database.migrate",
      "name": "migrate",
      "command": "python manage.py migrate",
      "description": "Run database migrations",
      "cwd": "backend"
    }
  ]
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
- Dev URL preferences are local to `.kmc/dev-url.json`; keep them out of shared project config unless your team intentionally wants to share local hosts and ports.

## License

MIT
