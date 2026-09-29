# Qory integrations

An integration is a program that connects Qory to an outside system, for example GitHub.
Each integration has its own repository. This repository holds what they share: the
contract they implement, a Go module for testing against it, and reusable CI and release
workflows.

| Path | What it is |
|---|---|
| [`contracts/integration/v1/`](contracts/integration/v1/README.md) | The integration contract: the `describe` command, how settings and secrets are passed, exit status |
| `contracts/` | Go package: embeds the contract and compiles its schema |
| `conformance/` | Go package: checks a program's output against the contracts, for use in tests |
| [`.github/workflows/go.yml`](.github/workflows/go.yml) | Reusable CI: gofmt, vet, test, build, doc comments |
| [`.github/workflows/release.yml`](.github/workflows/release.yml) | Reusable release: builds and publishes the program |

## Integrations

| Integration | Program | Roles | Repository |
|---|---|---|---|
| GitHub | `qory-github` | credential | [qoryai/qory-github](https://github.com/qoryai/qory-github) |

To list your integration, open a pull request that adds a row.

## Use an integration

1. Install the program on the machine that runs `qory run`, on its `PATH`.
2. Declare it in `~/.config/qory/runner.yaml`:

   ```yaml
   integrations:
     github:                            # the key; the program defaults to qory-<key>
       settings: {"app_id": 123456, "private_key_file": "/home/dev/.config/qory/github-app.pem"}
     tracker:
       program: /opt/acme/bin/acme-tracker   # required when the program is not qory-<key>
       settings: {"project": "web"}
   ```

3. Select it in a run's policy by its key:

   ```yaml
   credentials:
     - {name: github, argument: acme/shop}
   ```

`qory` runs the program's `describe`, validates the settings, and turns each role into
the runner's definition.

## Write an integration

1. On GitHub, create a repository from
   [qoryai/integration-template](https://github.com/qoryai/integration-template)
   (*Use this template*).
2. Follow its README: rename, implement, test, release.
3. Add a row to the table above.

Name your program without `qory` in it, for example `acme-tracker`. The `qory-` prefix
is reserved for programs Qory publishes ([TRADEMARKS.md](TRADEMARKS.md)).

## Roles

| Role | Called by | Contract |
|---|---|---|
| `credential` | the runner, per run | [integration contract](contracts/integration/v1/README.md#credential) and runner [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials) |
| `tool` | the runner's proxy | runner [§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools) |
| `work_source` | the control plane | reserved, not defined yet |
| `output` | the control plane | reserved, not defined yet |

## Test against the contract

```go
import "github.com/qoryai/integrations/conformance"

conformance.Description(stdout)            // output of `<program> describe`
conformance.Credential(stdout)             // output of `<program> credential ...`
conformance.Failure(code, stdout, stderr)  // a command that failed
```

Each returns an error that says which rule the output breaks.

## Release rule

Every integration publishes releases the same way, so `qory` can install any of them:

- Tag `vX.Y.Z` and publish a GitHub release for it.
- Attach `<program>_X.Y.Z_<os>_<arch>.tar.gz` for `linux` and `darwin`, `amd64` and
  `arm64`, with the program at the archive's root.
- Attach `checksums.txt` with the SHA-256 of each archive.
- `<program> describe` reports `"program_version": "X.Y.Z"`.

`release.yml` does all of this for a Go integration.

## Use the workflows

`.github/workflows/ci.yml` of an integration:

```yaml
on:
  push:
    branches: [main]
  pull_request:
jobs:
  go:
    uses: qoryai/integrations/.github/workflows/go.yml@main
```

`.github/workflows/release.yml`:

```yaml
on:
  push:
    tags: ["v*"]
jobs:
  release:
    permissions:
      contents: write
    uses: qoryai/integrations/.github/workflows/release.yml@main
    with:
      program: acme-tracker     # built from ./cmd/acme-tracker
```

The release fails unless `CHANGELOG.md` has a section `## [X.Y.Z] - YYYY-MM-DD`; that
section becomes the release notes.

## Development

```sh
mise install        # Go 1.27.1
go test ./...
```

[CONTRIBUTING.md](CONTRIBUTING.md) has the full checks and the release process.

## Licence

Apache License 2.0; see [LICENSE](LICENSE) and [NOTICE](NOTICE). *Qory* is a trademark of
8wonders GmbH.
