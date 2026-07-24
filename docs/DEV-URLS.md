# Dev URLs

Dev URLs give supported web applications a memorable local HTTPS address:

```text
https://my-app.kmc.localhost
```

KMC uses Caddy as a local reverse proxy and stores the selected project, host,
and port in `.kmc/dev.json`.

## Supported projects

- Next.js
- Vite
- NestJS
- Express

Project detection reads `package.json`. Node.js is therefore required for the
detected application's dev server, but not for the KMC binary.

## Interactive setup

Run:

```sh
kmc
```

Choose **Dev URLs**. The screen can:

- start the selected project on its stored port;
- reload Caddy;
- trust Caddy's local certificate authority;
- change the local name and host;
- select a detected monorepo project;
- use a project path manually.

## Direct commands

```sh
kmc dev detect
kmc dev configure
kmc dev configure apps/web
kmc dev reload
kmc dev trust
kmc dev start
```

## Monorepos

KMC searches upward for a workspace root and scans nested packages. Recognized
markers include:

- `package.json` workspaces;
- `pnpm-workspace.yaml` or `pnpm-workspace.yml`;
- `lerna.json`;
- `turbo.json`;
- `nx.json`.

Run KMC from the repository root or a nested application and select the desired
detected project.

## Caddy setup

Install Caddy with the package manager appropriate for the system:

```sh
sudo apt install caddy
```

```sh
sudo dnf install caddy
sudo pacman -S caddy
brew install caddy
```

KMC generates the shared Caddyfile in the user KMC configuration directory. On
Linux this is normally:

```text
~/.config/kmc/Caddyfile
```

If reload fails because Caddy is not running:

```sh
caddy start --config ~/.config/kmc/Caddyfile
kmc dev reload
```

If the browser reports `net::ERR_CERT_AUTHORITY_INVALID`:

```sh
caddy trust
```

Restart the browser if it still uses the previous certificate state.
