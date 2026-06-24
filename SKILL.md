---
name: kmc
description: Use the kmc CLI to discover, import, validate, and run project commands stored in kmc.json. Use when a user wants an agent to work with a repository's command center, add or maintain reusable project commands, or install the kmc skill/CLI from github.com/marius4lui/kmc.
---

# kmc

Use `kmc` as the project command center when a repository has or should have a `kmc.json`.

## When to use

- The user mentions `kmc`, `kmc.json`, a command center, reusable project commands, or wants agents to run known project workflows.
- A repo has a `kmc.json` and you need to inspect or run its saved commands.
- A user asks to install this skill or make it available from an agent skill picker.
- A user wants common commands imported from `package.json`, `Makefile`, `pubspec.yaml`, or Docker Compose files.

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

2. Inspect the current repository before changing commands:

   ```sh
   test -f kmc.json && kmc validate
   ```

   Also inspect likely command sources when present: `package.json`, `Makefile`, `pubspec.yaml`, `docker-compose.yml`, and `compose.yml`.

3. Prefer non-interactive commands in automation:

   ```sh
   kmc validate
   kmc import
   kmc run <command-id>
   ```

   Use the interactive menu only when the user explicitly wants manual selection:

   ```sh
   kmc
   kmc add
   kmc edit <command-id>
   kmc settings
   ```

4. When adding or editing `kmc.json`, keep commands grouped. Use stable command ids like `npm.dev`, `deployment.production`, or `manual.release`. Keep `cwd` relative to the directory where `kmc` is run.

5. Validate after any change:

   ```sh
   kmc validate
   ```

6. When running a saved command, tell the user which command id you are about to run and use:

   ```sh
   kmc run <command-id>
   ```

   If a command can deploy, publish, delete data, rotate secrets, or otherwise make external changes, get explicit user confirmation before running it.

## Install this skill

The skill is installed from the same GitHub repository as the CLI:

```sh
npx skills add marius4lui/kmc
```

If the host UI has an **Install skill** action, it should execute the same command for this repository. After installation, agents can invoke this skill as `$kmc`.
