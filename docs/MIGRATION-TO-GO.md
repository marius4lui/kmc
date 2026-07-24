# Migration from Node.js to Go

KMC 2 replaces the legacy npm-distributed CLI with a native Go binary. The
current KMC 2 release line is beta; no Node.js runtime is needed to install or
run it.

## Preserved project data

The Go implementation reads the existing user-owned formats:

- `kmc.json`, including grouped configuration and the legacy flat `commands`
  array with `group` or `groupId`;
- `.kmc/settings.json`;
- `.kmc/dev.json`;
- `.kmc/scripts.yml` and referenced workflow files;
- trusted-repository records and their SHA-256 fingerprint model.

Command working directories keep resolving from the directory where KMC is
started. Workflow schema version 1 remains strict.

## Migrate

1. Install the native beta:

   ```sh
   curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta
   ```

   Windows PowerShell:

   ```powershell
   & ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) -Channel beta
   ```

2. Open a new terminal if the installer changed `PATH`.

3. Confirm the native binary and channel:

   ```sh
   kmc --version
   kmc doctor
   kmc channel
   ```

4. Validate an existing project:

   ```sh
   kmc validate
   test -f .kmc/scripts.yml && kmc scripts validate
   ```

5. After confirming the native binary is the one on `PATH`, remove the legacy
   package:

   ```sh
   npm uninstall -g @marius4lui/kmc
   ```

The native installer and `kmc doctor` report a detected global legacy package.
They never remove it automatically and never delete project configuration,
settings, workflows, or trust data.

## Configuration normalization

Legacy flat configuration remains readable:

```json
{
  "commands": [
    {
      "name": "test",
      "command": "go test ./...",
      "group": "development"
    }
  ]
}
```

When KMC next writes the file, it converts commands to the grouped format while
preserving command ids, values, and working-directory behavior. Commit or back
up project files before any broad manual rewrite, just as for other
configuration migrations.

## Updates and rollback

Stay on the beta channel until the first KMC 2 stable release:

```sh
kmc channel set beta
kmc update --check
kmc update
```

Install a known beta to roll back:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- \
  --version v2.0.0-beta.2
```

All installer downloads are checked against the release SHA-256 file before
installation. Unix self-updates replace the binary atomically. Windows updates
run through the PowerShell installer.
