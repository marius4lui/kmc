# Security policy

## Supported versions

KMC 2 is currently in beta. Security fixes are applied to the newest published
beta release. Older beta builds may be replaced quickly and should not be
treated as long-term support releases.

| Version | Supported |
| --- | --- |
| Latest KMC 2 beta | Yes |
| Older KMC 2 betas | No |
| Legacy npm releases | No |

## Report a vulnerability

Use
[GitHub private vulnerability reporting](https://github.com/marius4lui/kmc/security/advisories/new)
to report a suspected vulnerability. Include:

- the affected KMC version and platform;
- the relevant command or workflow;
- reproducible steps or a minimal project;
- the expected and observed behavior;
- the potential impact;
- any suggested mitigation.

Do not include secrets, access tokens, or private repository content. Please do
not open a public issue until the report has been assessed and a coordinated
disclosure plan exists.

## Security boundaries

KMC saved commands and KMC Scripts execute local shell commands with the current
user's permissions. Repository trust confirms a reviewed project path and
workflow fingerprint; it is not a sandbox. Review command and workflow changes
before running them.

Release installers verify SHA-256 checksums. Optional GitHub artifact
attestation verification is available through `--verify-signature` on Unix and
`-VerifySignature` on Windows.
