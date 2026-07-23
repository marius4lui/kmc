# Migration from Node.js to Go

KMC 2 is a native Go application distributed through GitHub Releases. It does
not require Node.js or npm.

## Compatibility contract

The Go implementation preserves these user-owned formats:

- `kmc.json`, including legacy flat `commands` input and grouped commands;
- `.kmc/settings.json`;
- `.kmc/dev.json`;
- `.kmc/scripts.yml` and its referenced YAML workflows;
- the per-user trusted-repository store and SHA-256 fingerprint model.

Commands retain their working-directory behavior. Workflow schema version 1
remains strict: unknown fields, unsafe paths, invalid shells, missing working
directories, duplicate step names, and invalid timeout/retry values are rejected.

## Moving from an npm installation

Install the native binary:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh
kmc doctor
```

The installer and `kmc doctor` report a detected legacy npm installation. After
confirming the native binary is on `PATH`, remove the old package manually:

```sh
npm uninstall -g @marius4lui/kmc
```

The installer never deletes the npm package automatically and never removes
project configuration or trust data.

## Channels and rollback

Stable is the default:

```sh
kmc channel set stable
kmc update
```

Beta releases require explicit opt-in:

```sh
kmc channel set beta
kmc update
```

Install a known release to roll back:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --version v2.0.0
```

All archives are checked with the release SHA-256 file before installation, and
Unix self-updates replace the binary atomically. Windows updates are completed
through the PowerShell installer so the running executable is never overwritten.
