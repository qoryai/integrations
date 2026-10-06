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
| `cmd/integration-conformance/` | Command: checks a description on standard input with `conformance.Description`, for `release.yml` |
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

### Sources

An integration's source is where its releases are, in one of two forms.

- **A repository on a forge**, `<host>/<path>`, with no scheme. Examples: GitHub
  `github.com/<owner>/<repo>`, GitLab `gitlab.com/<group>[/<subgroup>…]/<project>`,
  Forgejo or Gitea `<host>/<owner>/<repo>`. It has two or more path segments, does not
  end in `.git`, and matches:

  ```
  ^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?(?:/[A-Za-z0-9_-][A-Za-z0-9_.-]{0,99}){2,}$
  ```

- **An HTTPS URL of a `description.json`**, with the release's other files in the same
  directory. It matches:

  ```
  ^https://(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?(?:/[A-Za-z0-9_-][A-Za-z0-9_.-]{0,99})*/description\.json$
  ```

Both forms follow the same rules for the host and the path:

- The host is a lower-case DNS name with at least one dot, and its last label starts with
  a letter. So no IP address is admitted in any spelling, such as `127.0.0.1`, `127.1` or
  `10.0.0.0x7f`. There is no IPv6 address, port, userinfo, query or fragment.
- A path segment starts with a letter, a digit, `_` or `-`. So a segment is never `.` or
  `..`, and `%` never occurs.
- A host that is `localhost` or ends in `.localhost`, `.local`, `.internal` or
  `.home.arpa` is refused.
- A reader that fetches refuses a host whose address is loopback, private, link-local or
  unspecified, checked on the address it connects to.

The forge kind is `github`, `gitlab` or `forgejo`; Gitea is `forgejo`. It is implied on
github.com (`github`), gitlab.com (`gitlab`) and codeberg.org (`forgejo`). On any other
host it is named beside the source, which stays `<host>/<path>`. A URL source has none.
A run's connection carries them as its `source` and `forge_kind`
([runner contract](https://github.com/qoryai/runner/tree/main/contracts/runner/v1)).

### Releases

A release is these files:

- `description.json`, what `<program> describe` prints, as it prints it;
- `<program>_X.Y.Z_<os>_<arch>.tar.gz` for `linux` and `darwin`, `amd64` and `arm64`,
  with the program at the archive's root;
- `checksums.txt`, the SHA-256 of each archive and of `description.json`.

The version is `X.Y.Z`, the description's `program_version`. On a forge the release is
tagged `vX.Y.Z`. A control plane fetches the release's `description.json` to learn an
integration without running it.

### Finding a release

Where each file of a release is, by the source's kind:

- **github**, on github.com or GitHub Enterprise Server:
  - version X.Y.Z: `https://<host>/<owner>/<repo>/releases/download/vX.Y.Z/<file>`;
  - latest release: `https://<host>/<owner>/<repo>/releases/latest/download/<file>`, or
    the API `GET https://api.github.com/repos/<owner>/<repo>/releases/latest` on
    github.com and `GET https://<host>/api/v3/repos/<owner>/<repo>/releases/latest` on
    GitHub Enterprise Server, whose `tag_name` is `vX.Y.Z`.
- **forgejo**, Forgejo and Gitea:
  - version X.Y.Z: `https://<host>/<owner>/<repo>/releases/download/vX.Y.Z/<file>`;
  - latest release: `https://<host>/<owner>/<repo>/releases/download/latest/<file>`, or
    the API `GET https://<host>/api/v1/repos/<owner>/<repo>/releases/latest`, whose
    `tag_name` is `vX.Y.Z`.
- **gitlab**, GitLab 15.4 or later, through the API. `<project>` is the path,
  URL-encoded, such as `group%2Fsub%2Fproj`:
  - version X.Y.Z: `GET https://<host>/api/v4/projects/<project>/releases/vX.Y.Z/downloads/<file>`,
    where each file is a release link whose direct asset path is `/<file>`. It answers
    302 to the link's URL;
  - latest release: `GET https://<host>/api/v4/projects/<project>/releases/permalink/latest/downloads/<file>`,
    or `GET https://<host>/api/v4/projects/<project>/releases/permalink/latest`, which
    gives the latest release, whose `tag_name` is `vX.Y.Z`.

  A reader does not use the web route `https://<host>/<path>/-/releases/…/downloads/<file>`.
  Since GitLab 17.3.2, 17.2.5 and 17.1.7 it redirects only to a link on the GitLab host
  itself. For a link on another host it answers 200 with an HTML warning page, which a
  reader would take for the file.
- **URL source**: each file is `<dir>/<file>`, where `<dir>` is the URL without
  `/description.json`. A URL source is one release. Its version is the description's
  `program_version`. A newer release may replace the files at the URL.

On github and forgejo the latest release is the newest that is neither a draft nor a
prerelease. On gitlab it is the release with the latest `released_at`, an upcoming release
included.

Every kind is read the same way. A reader:

1. fetches `description.json` and `checksums.txt`;
2. checks `description.json` and each archive it fetches against `checksums.txt`;
3. checks that `program_version` equals the version it asked for, the tag without its
   `v`.

For the latest release, the version it asked for is the API's `tag_name` without its
`v`. A reader that uses a latest download form instead takes the version from the
`description.json` that form serves, and fetches `checksums.txt` and the archives by that
version. For a URL source the reader asks for no version: it reads `program_version`
and checks every file against `checksums.txt` each time it fetches, since the files may
have been replaced. A reader that requires a version refuses any other.

A private release needs an access token, which the reader sends in a header:

| Kind | Header | What the access token needs |
|---|---|---|
| github | `Authorization: Bearer <access token>` | read access to the repository's contents |
| forgejo | `Authorization: token <access token>` | the scope `read:repository` |
| gitlab | `PRIVATE-TOKEN: <access token>` | the scope `read_api`, as `read_repository` is not enough, and at least the Reporter role |

- **github**: the reader fetches the files through the API.
  `GET https://api.github.com/repos/<owner>/<repo>/releases/tags/vX.Y.Z`, with
  `Accept: application/vnd.github+json`, gives the release. The reader picks the asset
  by `name`. `GET https://api.github.com/repos/<owner>/<repo>/releases/assets/<id>`, with
  `Accept: application/octet-stream`, answers 200 with the file, or 302 to a signed
  storage URL. The reader follows the 302 at once and without the header, since the
  signed URL expires within minutes. On GitHub Enterprise Server the API is
  `https://<host>/api/v3`. A public release keeps the download URLs, so it never meets
  the API's anonymous rate limit of 60 requests an hour.
- **forgejo**: the reader fetches the files from the download URLs above, since the API
  has no route that returns a file's bytes. They answer 200, or 303 to storage.
- **gitlab**: the reader fetches the files from the API routes above.

For the latest release the reader asks the API forms above for it, with the same header,
and fetches the files by its `tag_name`.

The reader sends the access token only to the forge's API host for the source:
`api.github.com` for github.com, and the source's own host for GitHub Enterprise Server,
GitLab, Forgejo and Gitea. It never sends it on a redirect to another host, so it
follows a redirect to storage without it. It never puts it in a URL query, such as
`?private_token=`, since a query reaches logs. On GitLab a redirect that stays on the
source's host under `/api/v4/projects/` may carry the header, since a release link to a
generic package needs it. A URL source is public: the reader fetches it with no
credentials.

`release.yml` publishes a release for a Go integration on GitHub. It fails the release
when `describe` reports another version than the tag's, or prints a description that
fails `conformance.Description`. On another forge, publish the same files with the
forge's own CI. goreleaser publishes them where the forms above find them: to GitLab
with `release.gitlab` and `gitlab_urls`, each file a release link whose direct asset
path is `/<file>`, and to Gitea with `release.gitea` and `gitea_urls`.

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

The release also fails unless `<program> describe` prints one JSON object with `version`
1 and the tag's version, without its `v`, as `program_version`, and that description
passes `conformance.Description`. The release builds
`github.com/qoryai/integrations/cmd/integration-conformance` in the integration's module
and runs it on the description, so its `go.mod` requires `github.com/qoryai/integrations`,
at a version that has `cmd/integration-conformance`. The check uses that version, the one
the integration's tests use.

## Development

```sh
mise install        # Go 1.27.1
go test ./...
```

[CONTRIBUTING.md](CONTRIBUTING.md) has the full checks and the release process.

## Licence

Apache License 2.0; see [LICENSE](LICENSE) and [NOTICE](NOTICE). *Qory* is a trademark of
8wonders GmbH.
