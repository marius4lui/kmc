# Changelog

## 2.0.0-beta.1 - 2026-07-23

- Reimplemented KMC as a native Go binary.
- Preserved grouped and legacy `kmc.json` compatibility.
- Ported command importers, validated YAML workflows, trust fingerprints,
  timeouts, retries, dry runs, and command execution.
- Added native stable and experimental GitHub Release channels.
- Added checksummed atomic installers for Linux, macOS, and Windows.
- Added self-update checks, channel persistence, diagnostics, and legacy npm
  detection.
- Added GoReleaser archives, SBOMs, build provenance, cross-build CI, and
  installer validation.
