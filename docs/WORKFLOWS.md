# KMC Scripts

KMC Scripts is a strict local YAML workflow runner. Workflows run sequential
shell steps while keeping definitions reviewable inside the repository.

## Initialize

```sh
kmc scripts init
```

KMC creates only missing files:

```text
.kmc/
├── scripts.yml
└── scripts/
    └── test.yml
```

## Registry

`.kmc/scripts.yml` maps stable ids to workflow files:

```yaml
version: 1

scripts:
  checks:
    file: ./scripts/checks.yml
    description: Format, vet, and test the Go codebase
```

Registry paths are relative to `.kmc/scripts.yml` and must remain inside the
project root.

## Workflow

```yaml
name: Go checks
description: Verify the native CLI

env:
  CGO_ENABLED: "0"

defaults:
  shell: sh
  cwd: .

steps:
  - name: Check formatting
    run: test -z "$(gofmt -l .)"

  - name: Vet
    run: go vet ./...
    timeout: 300

  - name: Test
    run: go test ./...
    retries: 1
    continue_on_error: false
```

Workflow environment variables apply to all steps. Step-level values and
`--env` overrides take precedence while the process environment is preserved.
Relative working directories resolve from the project root.

Version 1 supports:

- workflow `name`, `description`, `env`, and `defaults`;
- step `name`, `run`, `shell`, `cwd`, `env`, `timeout`, `retries`, and
  `continue_on_error`;
- POSIX shells on Linux and macOS;
- `pwsh`, `powershell`, and `cmd` on Windows.

Unknown fields are rejected. Constructs such as `uses`, `depends_on`, `if`,
`parallel`, and `cache` are not part of version 1.

## Validate and inspect

```sh
kmc scripts list
kmc scripts validate
kmc run checks --dry-run
```

Validation rejects unsupported schema versions, unknown fields, unsafe paths,
missing working directories, duplicate step names, invalid timeouts or retries,
and unsupported shells.

## Run

```sh
kmc scripts run checks
kmc run checks
kmc run checks --step test
kmc run checks --step 3
kmc run checks --env CGO_ENABLED=1
kmc run checks --verbose --no-color
```

`--step` accepts a one-based index, exact step name, or slugified name. A failed
step stops the workflow unless `continue_on_error` is true. Timeouts exit with
code `124`, interruption with `130`, and other failures preserve the command's
non-zero exit code.

## Trust model

Workflow files execute arbitrary local commands. Review `.kmc/scripts.yml` and
every referenced file before granting trust:

```sh
kmc trust status
kmc trust
kmc untrust
```

Trust is bound to:

- the canonical project path; and
- a SHA-256 fingerprint of the registry and all active workflow files.

Editing an active workflow invalidates trust. Non-interactive execution is
blocked until the current fingerprint is trusted. `--yes` explicitly trusts
the current fingerprint and should be used only after review.

Do not place secrets directly in workflow YAML. Pass them through the process
environment or an appropriate secret manager.
