# Installer operations

The canonical installer source is `scripts/install.sh`. The Pages workflow copies
that exact file to the website root, making it available as:

```text
https://kmc.kmuc.app/install.sh
```

## One-time owner tasks

- In the repository, open **Settings → Pages** and select **GitHub Actions** as the
  source.
- In the DNS zone for `kmuc.app`, create or change the `kmc` record to a `CNAME`
  targeting `marius4lui.github.io`. If Cloudflare is in front of the domain,
  start with **DNS only** (grey cloud) until GitHub has issued the Pages
  certificate. Remove any Worker, redirect, or Pages route currently serving
  `kmc.kmuc.app/*`; it must not replace `/install.sh` with HTML.
- After DNS has propagated, enable **Enforce HTTPS** in **Settings → Pages**.
- Add the npm automation token as the repository secret `NPM_TOKEN`. Configure it
  for package `@marius4lui/kmc`, with publish permission and 2FA bypass for CI.
- Protect `main` and require the `CI` checks before merging.

GitHub Pages reads the custom domain from `site/CNAME`. Do not configure a second
Pages deployment for this repository.

Verify that the endpoint is really the script, not merely a successful HTML page:

```sh
curl -fsS https://kmc.kmuc.app/install.sh | head -n 1
# expected: #!/bin/sh
```

## Delivery flows

### Pull request / branch

`CI` installs locked dependencies, runs the Node.js test suite on Node 18, 20, and
22, checks POSIX shell syntax, and verifies the installer help command.

### Website / installer

Every change to `scripts/install.sh`, `site/`, or the Pages workflow on `main`
deploys the site. It can also be started manually with **Run workflow**.

### npm release

1. Update `package.json` and `package-lock.json` to the same version.
2. Merge the tested change to `main`.
3. Create and push the matching tag, for example `v1.1.0`.
4. The release workflow checks that the tag equals the package version, tests the
   package, and publishes it to npm with provenance.
5. Verify the public path with:

   ```sh
   curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- doctor
   ```

The installer supports `install`, `update`, `uninstall`, and `doctor`. It falls
back to `~/.local` when the npm global prefix is not writable, avoiding `sudo`.
Use `--modify-path` only when the installer should update the user's shell profile.
