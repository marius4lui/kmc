---
name: kmc
description: Use the native KMC CLI to inspect, validate, import, and run project commands from kmc.json or trusted YAML workflows in .kmc/scripts.yml.
---

# KMC

Use KMC as the repository's command center when a project contains `kmc.json`
or `.kmc/scripts.yml`, or when the user asks to create reusable project
commands.

KMC 2 is a native Go binary. It is currently beta software; use the beta
installer channel until a stable KMC 2 release is published.

## Start safely

1. Check whether KMC is available:

   ```sh
   kmc --version
   ```

2. If it is missing, install the current beta:

   ```sh
   # Linux and macOS
   curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta

   # Windows PowerShell
   & ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) -Channel beta
   ```

3. Inspect the repository before changing or running anything:

   ```sh
   test -f kmc.json && kmc validate
   test -f .kmc/scripts.yml && kmc scripts validate
   ```

   Also inspect any detected command sources that are relevant:
   `package.json`, `Makefile`, `pubspec.yaml`, `docker-compose.yml`, and
   `compose.yml`.

## Saved commands

Prefer stable, descriptive ids such as `development.test`,
`database.migrate`, or `deployment.release`. Keep each `cwd` relative to the
project directory where KMC is started.

Use direct commands for automated work:

```sh
kmc validate
kmc import
kmc run <command-id>
```

Use interactive flows only when the user wants to choose or edit manually:

```sh
kmc
kmc add
kmc edit <command-id>
kmc settings
```

KMC reads both the current grouped `kmc.json` format and the legacy flat
`commands` format. Do not rewrite user-owned configuration merely to modernize
its shape; KMC normalizes it when it next writes the file.

## KMC Scripts

KMC Scripts are registered in `.kmc/scripts.yml`. Initialize missing files with:

```sh
kmc scripts init
```

The version 1 workflow schema supports sequential steps with `name`, `run`,
`shell`, `cwd`, `env`, `timeout`, `retries`, and `continue_on_error`. Unknown
fields are rejected. Registry and workflow paths must remain inside the project
root.

Useful non-interactive commands:

```sh
kmc scripts list
kmc scripts validate
kmc run <script-id> --dry-run
kmc run <script-id> --step <name-or-index>
kmc run <script-id> --env KEY=value
```

Workflow files execute local shell commands. Read the registry and every
referenced workflow before trusting them:

```sh
kmc trust status
kmc trust
kmc untrust
```

Trust is bound to the canonical project path and a SHA-256 fingerprint of all
active workflow files. Any workflow edit invalidates it. Use `--yes` only after
reviewing the current fingerprint in this task.

Never run a saved command or workflow that can deploy, publish, delete data,
rotate secrets, purchase resources, or change external systems without explicit
user approval. Tell the user which command or script id will run before
executing it.

## Dev URLs

Dev URLs support Next.js, Vite, NestJS, and Express projects, including nested
monorepo apps:

```sh
kmc dev detect
kmc dev configure [project]
kmc dev start
```

The interactive **Dev URLs** screen also supports project selection, Caddy
reloads, and local CA trust. KMC stores project-local Dev URL state in
`.kmc/dev.json` and generates the shared Caddy configuration in the user's KMC
configuration directory.

If Caddy is missing or not running:

```sh
sudo apt install caddy
caddy start --config ~/.config/kmc/Caddyfile
```

If the browser rejects the local certificate:

```sh
caddy trust
```

## Validate changes

After changing project commands or workflows, run every applicable validator:

```sh
test -f kmc.json && kmc validate
test -f .kmc/scripts.yml && kmc scripts validate
```

Use `kmc run <script-id> --dry-run` before executing a changed workflow.

## Install this skill

Install the agent skill from this repository with:

```sh
npx skills add marius4lui/kmc
```

The external skill installer currently uses `npx`; this does not install the
legacy Node.js KMC CLI. Update the native CLI with `kmc update` and update the
skill through the agent host's skill management flow.
