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
| [`.github/workflows/release.yml`](.github/workflows/release.yml) | Reusable release on GitHub: builds and publishes the program and its description |

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

## How qory uses an integration

An integration offers one or more ways: the `credential` role for an API, the `tool`
role for an MCP server. A run's connection names the ways it uses. For the `credential`
role:

1. You declare the integration in `runner.yaml`.
2. `qory run` runs `<program> describe` and checks your settings against it.
3. `qory` turns the declaration into a credential for the
   [runner](https://github.com/qoryai/runner).
4. Before the agent starts, the runner runs `<program> credential` outside the agent's
   container. The program prints an access token.
5. The agent starts. It gets a placeholder, never the access token.
6. Five minutes before the access token expires, the runner runs the program again.

In the agent's container:

- The variables the integration lists hold a placeholder,
  `qory-sets-the-credential-outside-the-enclosure`. Tools that read them start and send
  the placeholder.
- The runner's proxy replaces it with the access token on each request to the
  integration's hosts and paths.
- Under `enforce`, a request to another path on those hosts fails.

For the `tool` role, the runner runs `<program> tool` outside the agent's container
before the agent starts. The runner's proxy sends it the requests to the hosts it
serves, and `qory` registers its MCP server with the agent. The tool's secrets stay
outside the container.

This holds when the agent runs in a container, behind the runner's wall. Without one, a
program that ignores the proxy is bound by nothing.

## Write an integration

1. On GitHub, create a repository from
   [qoryai/integration-template](https://github.com/qoryai/integration-template)
   (*Use this template*).
2. Follow its README: rename, implement, test, release.
3. Add a row to the table above.

Every integration is started in these ways:

```sh
<program> describe                  # its settings and roles, as JSON
<program> <role> -- [arguments]     # play a role, the settings on standard input
```

The rules: the [contract](contracts/integration/v1/README.md). The guide:
[docs/writing-an-integration.md](docs/writing-an-integration.md).

Name your program without `qory` in it, for example `acme-tracker`. The `qory-` prefix
is reserved for programs Qory publishes ([TRADEMARKS.md](TRADEMARKS.md)).

## Roles

| Role | Called by | Contract |
|---|---|---|
| `credential` | the runner, per run | [integration contract](contracts/integration/v1/README.md#credential) and runner [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials) |
| `tool` | the runner's proxy | [integration contract](contracts/integration/v1/README.md#tool) and runner [§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools) |
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

Every integration publishes releases the same way, so `qory` can install any of them.

An integration's source is where its releases are, one of:

- **A repository on a forge**, `<host>/<path>`: a lower-case host name with no scheme or
  port, then two or more path segments. Each segment is
  `[A-Za-z0-9_-][A-Za-z0-9_.-]{0,99}`, and the source does not end in `.git`. Examples:
  GitHub `github.com/<owner>/<repo>`, GitLab
  `gitlab.com/<group>[/<subgroup>…]/<project>`, Forgejo or Gitea `<host>/<owner>/<repo>`.
  - The forge is `github` on github.com, `gitlab` on gitlab.com and `forgejo` on
    codeberg.org. On any other host the forge is named beside the source: `github`,
    `gitlab` or `forgejo` (Gitea is `forgejo`). The source itself stays `<host>/<path>`.
    A run's connection carries them as its `source` and `forge`
    ([runner contract](https://github.com/qoryai/runner/tree/main/contracts/runner/v1)).
  - A release is tagged `vX.Y.Z`. Its files download from
    `https://<host>/<path>/releases/download/vX.Y.Z/<file>` on github and forgejo, and
    from `https://<host>/<path>/-/releases/vX.Y.Z/downloads/<file>` on gitlab, where
    each file is a release link with that direct path.
- **An HTTPS URL of a `description.json`**, with the release's other files in the same
  directory. A URL source names no forge.

A release is these files:

- `description.json`, what `<program> describe` prints, as it prints it;
- `<program>_X.Y.Z_<os>_<arch>.tar.gz` for `linux` and `darwin`, `amd64` and `arm64`,
  with the program at the archive's root;
- `checksums.txt`, the SHA-256 of each archive and of `description.json`.

The version is `X.Y.Z`, the description's `program_version`, which a reader that
requires a version compares exactly. On a forge the tag is `v` followed by it. A control
plane fetches the release's `description.json` to learn an integration without running
it.

`release.yml` does this for a Go integration on GitHub, and fails the release when
`describe` reports another version than the tag's. On another forge, publish the same
files with the forge's own CI; goreleaser can publish to GitLab and Gitea releases.

## Use the workflows

`.github/workflows/ci.yml` of an integration:

```yaml
on:
  push:
    branches: [main]
  pull_request:
jobs:
  go:
    uses: qoryai/integrations/.github/workflows/go.yml@v0.2.0
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
    uses: qoryai/integrations/.github/workflows/release.yml@v0.2.0
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
