# Changelog

## Unreleased

- Reorganized the GitHub documentation into focused installation, command,
  workflow, Dev URL, migration, contributing, and security guides.
- Reworked the agent skill for the native Go CLI and current beta lifecycle.
- Added an explicit beta download experience to the project website.
- Persisted the installer-selected release channel for future native updates.
- Restored JSON Schema support for legacy `group` and `groupId` command fields.
- Replaced legacy npm ignore patterns with Go build output and narrowly scoped
  user-local KMC state.

## 2.0.0-beta.2 - 2026-07-23

- Reimplemented KMC as a native Go binary.
- Preserved grouped and legacy `kmc.json` compatibility.
- Ported command importers, validated YAML workflows, trust fingerprints,
  timeouts, retries, dry runs, and command execution.
- Added native stable and beta GitHub Release channels.
- Added checksummed atomic installers for Linux, macOS, and Windows.
- Added self-update checks, channel persistence, diagnostics, and legacy npm
  detection.
- Added GoReleaser archives, SBOMs, build provenance, cross-build CI, and
  installer validation.
