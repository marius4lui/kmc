# Installation

KMC 2 is distributed as a native Go binary through GitHub Releases. Node.js,
npm, and a local Go toolchain are not required.

> [!IMPORTANT]
> KMC 2 is currently available only as a beta. Use the explicit `beta` channel
> commands below. The `stable` channel will become the default recommendation
> after the first final KMC 2 release.

## Linux and macOS

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta
```

The default target is `~/.local/bin/kmc`. If that directory is not already on
`PATH`, the installer prints the required shell configuration. To update the
appropriate shell profile automatically:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- \
  --channel beta \
  --modify-path
```

## Windows PowerShell

Run the remote installer with the beta parameter:

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) -Channel beta
```

The default target is `%LOCALAPPDATA%\Programs\kmc\kmc.exe`. To add it to the
user `PATH` automatically:

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) `
  -Channel beta `
  -ModifyPath
```

Open a new terminal after changing the user `PATH`.

## Verify

```sh
kmc --version
kmc doctor
```

`kmc doctor` reports the active binary, platform, update channel, configuration
directory, and any detected legacy global npm installation.

## Release channels

| Channel | Status | Selection |
| --- | --- | --- |
| `beta` | Available now | Newest non-draft prerelease |
| `stable` | Coming with KMC 2 final | Newest non-draft final release |
| `nightly` | CLI update channel only | Release tag containing `nightly` |

The installer remembers the selected channel for later self-updates. Change it
manually when needed:

```sh
kmc channel
kmc channel set beta
kmc update --check
kmc update
```

On Windows, update through the PowerShell installer because a running
executable cannot replace itself reliably:

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) `
  -Command update `
  -Channel beta
```

## Pin an exact version

Use a release tag to install or roll back to a known version:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- \
  --version v2.0.0-beta.2
```

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) `
  -Version v2.0.0-beta.2
```

Exact versions still have to exist as non-draft GitHub Releases.

## Verification model

The installer:

1. detects the operating system and architecture;
2. resolves the requested release;
3. downloads the platform archive and `kmc_checksums.txt`;
4. verifies SHA-256 before extraction;
5. verifies the downloaded binary can report its version;
6. moves the binary into place atomically;
7. saves the selected update channel.

For GitHub artifact-attestation verification, install the GitHub CLI and add
`--verify-signature` on Unix or `-VerifySignature` on Windows.

## Supported targets

- Linux: amd64 and arm64
- macOS: Intel (amd64) and Apple Silicon (arm64)
- Windows: amd64 and arm64

## Uninstall

Linux and macOS:

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- uninstall
```

Windows PowerShell:

```powershell
& ([scriptblock]::Create((irm https://kmc.kmuc.app/install.ps1))) `
  -Command uninstall
```

Uninstalling removes the installed binary and installer metadata. It does not
delete project configuration, workflow files, settings, or trust data.

## Direct downloads

Every supported archive, checksum file, SBOM, and release note is available on
the [GitHub Releases page](https://github.com/marius4lui/kmc/releases). Prefer
the installers because they select the correct asset and verify it
automatically.
