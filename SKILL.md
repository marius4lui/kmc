---
name: kmc
description: Use the kmc CLI to discover, import, validate, and run project commands from kmc.json and trusted YAML workflows in .kmc/scripts.yml. Use when a user wants an agent to work with a repository's command center, reusable commands, local workflows, or install the kmc skill/CLI from github.com/marius4lui/kmc.
---

# kmc

Use `kmc` as the project command center when a repository has or should have a `kmc.json` or `.kmc/scripts.yml`.

## When to use

- The user mentions `kmc`, `kmc.json`, a command center, reusable project commands, or wants agents to run known project workflows.
- A repo has a `kmc.json` and you need to inspect or run its saved commands.
- A repo has `.kmc/scripts.yml` and the user wants to run or maintain its local YAML workflows.
- A user asks to install this skill or make it available from an agent skill picker.
- A user wants common commands imported from `package.json`, `Makefile`, `pubspec.yaml`, or Docker Compose files.
- A user wants stable local HTTPS dev URLs for a Next.js, Vite, NestJS, or Express app.

## Instructions

1. Check whether the CLI is available:

   ```sh
   kmc --version
   ```

   If it is missing, prefer:

   ```sh
   npm install -g @marius4lui/kmc
   ```

   For one-off use, run:

   ```sh
   npx @marius4lui/kmc --help
   ```

2. Inspect the current repository before changing commands or workflows:

   ```sh
   test -f kmc.json && kmc validate
   test -f .kmc/scripts.yml && kmc scripts validate
   ```

   Also inspect likely command sources when present: `package.json`, `Makefile`, `pubspec.yaml`, `docker-compose.yml`, and `compose.yml`.

3. Prefer non-interactive commands in automation:

   ```sh
   kmc validate
   kmc import
   kmc run <command-id>
   kmc scripts list
   kmc scripts validate
   kmc run <script> --dry-run
   ```

   Use the interactive menu only when the user explicitly wants manual selection:

   ```sh
   kmc
   kmc add
   kmc edit <command-id>
   kmc settings
   ```

4. When adding or editing `kmc.json`, keep commands grouped. Use stable command ids like `npm.dev`, `deployment.production`, or `manual.release`. Keep `cwd` relative to the directory where `kmc` is run.

5. KMC Scripts are YAML workflows registered in `.kmc/scripts.yml`. Use `kmc scripts init` to create a safe starter layout:

   ```text
   .kmc/
   ├── scripts.yml
   └── scripts/
       └── test.yml
   ```

   A workflow runs its steps sequentially. Version 1 supports `name`, `run`, `shell`, `cwd`, `env`, `timeout`, `retries`, and `continue_on_error`. Keep all registry and workflow paths within the project root. Use `kmc scripts validate` after changes; unknown schema fields are intentionally rejected.

6. Before executing a YAML workflow, inspect it and its registry. These files can run arbitrary local commands. In non-interactive automation, KMC blocks untrusted repositories. Trust only after review:

   ```sh
   kmc trust
   kmc trust status
   kmc untrust
   ```

   Trust is invalidated when KMC workflow files change. Do not use `--yes` unless the workflow contents were reviewed in the current task. As with saved commands, request explicit user confirmation before commands that deploy, publish, delete data, rotate secrets, or make other external changes.

7. Validate after any change:

   ```sh
   kmc validate
   kmc scripts validate
   ```

8. For local web app URLs, use the interactive **Dev URLs** menu:

   ```sh
   kmc
   ```

   In monorepos, choose **Dev URLs** -> **Select detected project**. `kmc` searches upward for workspace roots such as `pnpm-workspace.yaml`, `package.json` workspaces, `turbo.json`, `nx.json`, and `lerna.json`, then scans nested app packages.

   If Caddy is missing, install it first:

   ```sh
   sudo apt install caddy
   ```

   If Caddy is installed but not running, start it:

   ```sh
   caddy start --config ~/.config/kmc/Caddyfile
   ```

   If the browser shows `net::ERR_CERT_AUTHORITY_INVALID`, trust the local Caddy CA:

   ```sh
   caddy trust
   ```

9. When running a saved command or workflow, tell the user which id you are about to run and use:

   ```sh
   kmc run <command-id>
   ```

   For a workflow step, use the script id (for example `kmc run checks`) and consider `--dry-run` first. If a command can deploy, publish, delete data, rotate secrets, or otherwise make external changes, get explicit user confirmation before running it.

## Install this skill

The skill is installed from the same GitHub repository as the CLI:

```sh
npx skills add marius4lui/kmc
```

If the host UI has an **Install skill** action, it should execute the same command for this repository. After installation, agents can invoke this skill as `$kmc`.

When the interactive KMC updater installs a newer CLI release, it also updates existing project or global installations of this skill. The skills installer reuses the prior scope, selected agents, and installation links from its lock data.
