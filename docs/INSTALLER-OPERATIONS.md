# Installer and release operations

The canonical installers are `scripts/install.sh` and `scripts/install.ps1`.
GitHub Pages copies those exact files to:

```text
https://kmc.kmuc.app/install.sh
https://kmc.kmuc.app/install.ps1
```

KMC is distributed as native GitHub Release binaries. Node.js and npm are not
part of the runtime or release process.

## Release channels

- `stable` resolves the newest non-draft, non-prerelease semantic version.
- `beta` resolves the newest GitHub prerelease and is opt-in.
- An exact `--version vX.Y.Z` bypasses channel resolution but never accepts a
  draft release.

The release tag is the source of truth. Final tags use `vX.Y.Z`; beta
tags use a SemVer prerelease such as `v2.1.0-rc.1`.

KMC 2 currently has beta releases only. Public installation instructions must
pass `--channel beta` or `-Channel beta` until the first final KMC 2 release is
published. Do not present the stable channel as currently available.

## Release assets

GoReleaser creates archives for Linux, macOS, and Windows on amd64 and arm64:

```text
kmc_2.0.0_linux_amd64.tar.gz
kmc_2.0.0_linux_arm64.tar.gz
kmc_2.0.0_darwin_amd64.tar.gz
kmc_2.0.0_darwin_arm64.tar.gz
kmc_2.0.0_windows_amd64.zip
kmc_2.0.0_windows_arm64.zip
kmc_checksums.txt
```

Every installer download is verified using SHA-256 before extraction. With
`--verify-signature`/`-VerifySignature`, the installer additionally verifies the
GitHub artifact attestation through `gh`. The new binary is staged and moved
into place only after verification. Release archives also receive GitHub
build-provenance attestations and SBOMs.

## Delivery flows

Pull requests and branches run:

- tests and vet on Go 1.24 plus the current stable Go release;
- gofmt verification;
- cross-compilation for every supported target;
- POSIX and PowerShell installer validation;
- a GoReleaser snapshot build.

Pushing `v*.*.*` runs tests, validates the tag, builds the release with
GoReleaser, uploads checksums and SBOMs, and attests the archives. SemVer
prerelease tags automatically create GitHub prereleases.

Installer or website changes on `main` deploy through the Pages workflow.

## Release procedure

1. Ensure CI is green and update the changelog.
2. Tag the tested commit, for example `v2.0.0` or `v2.1.0-rc.1`.
3. Push the tag.
4. Confirm that every archive, `kmc_checksums.txt`, and SBOM is attached.
5. Verify the provenance attestation.
6. Test stable or beta resolution as appropriate.
7. Run installer smoke tests on Linux/macOS and Windows.

```sh
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- doctor
curl -fsSL https://kmc.kmuc.app/install.sh | sh -s -- --channel beta
kmc update --check
```

For a beta install, confirm `kmc channel` prints `beta`; the installer is
responsible for persisting its selected channel into the native CLI settings.

## Incident and rollback

- Never replace or delete an existing GitHub Release asset in place. Publish a
  corrected patch version.
- If a release is unsafe, mark it clearly and publish a fixed version. Drafts
  are never selected; prereleases are never selected by stable.
- A checksum mismatch must be treated as a failed installation. Do not advise
  users to bypass verification.
- Exact-version installation provides the supported rollback path.
- Uninstall removes the binary and installer metadata only. Project
  configuration and workflow trust data are retained.

## One-time repository setup

- Configure GitHub Pages to use GitHub Actions.
- Point `kmc.kmuc.app` to `marius4lui.github.io` and enforce HTTPS.
- Protect `main` and require CI.
- Keep release workflow permissions minimal: contents, identity token, and
  attestations only.
