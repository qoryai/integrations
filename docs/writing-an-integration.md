# Writing an integration

An integration is a program that connects Qory to one outside system. This page is what
you need to write one. The rules themselves are in the integration contract,
[contracts/integration/v1](../contracts/integration/v1/README.md). To start, create a
repository from the [integration template](https://github.com/qoryai/integration-template).

## What an integration is

- Each integration connects one system, and plays the roles that system needs.
- A role is a contract. The part of Qory that calls the program writes it. The program
  speaks it as written.
- So an integration can be written in any language, and live in any repository. Each
  integration has a repository of its own, with its own releases.
- The domains an integration serves are data of its description.

## Roles

| Role                | In a description | Called by                                                       |
| ------------------- | ---------------- | --------------------------------------------------------------- |
| credential adapter  | `credential`     | the runner, outside the wall, per run                           |
| tool                | `tool`           | the runner's proxy, for the hosts it serves, in a later release |
| work-source adapter | `work_source`    | the control plane                                               |
| output adapter      | `output`         | the control plane                                               |

What each role does:

- **credential adapter**: mints an access token for what a run requests. It lists the
  hosts and paths the access token is used on. The runner keeps the access token outside
  the session.
  Its contract is the runner's
  [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials):
  one JSON document on standard output.
- **tool**: serves the session's requests to its hosts, such as an MCP server whose
  secrets stay outside the agent's enclosure. Its contract is the integration contract's
  [§Tool](../contracts/integration/v1/README.md#tool) and the runner's
  [§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools): HTTP
  over a Unix socket.
- **work-source adapter**: brings work items and their events in. It has no contract
  yet.
- **output adapter**: applies a change request to the system it is for. It has no
  contract yet.

The integration contract defines `credential` and `tool`. It reserves `work_source` and
`output`, each for a contract of its own. A reader expands the roles it knows, and leaves
the others as they are.

An integration offers one or more ways. A way is a role of its description:

- the API way is the `credential` role;
- the MCP way is the `tool` role.

A run's connection names the ways it uses, and the runner starts those roles alone. An
API that needs only a static key needs no program and no role: the runner's services
cover it.
A stdio MCP server the agent would start inside its enclosure is outside the contract.

## The contract in short

- `<program> describe` prints the integration's description: one JSON document. It
  contains:
  - the integration's name,
  - its `publisher`, who publishes it, as the program names it,
  - the domains it serves,
  - its settings, as a JSON Schema,
  - the roles it plays, each with the settings it needs.
- Every role is started the same way, the role's settings one JSON document on standard
  input:

  ```sh
  <program> <role> -- [the role's own arguments]
  ```

- `--` is always there, even when the role has no arguments. The credential role and the
  tool role each get exactly one argument: `<program> credential -- <argument>`,
  `<program> tool -- <argument>`.
- A role with an `argument` pattern gets the run's argument, empty when the run gives
  none. A role without `argument` ignores the run's argument and gets an empty one. The
  run's argument is matched only against the roles that have a pattern, so a credential
  role with a pattern and a tool role with none both run in one connection.
- A program takes no flags for a role. It refuses `--settings` as it refuses any flag it
  does not know.
- A program reads its settings from standard input alone: no setting and no secret from
  its environment. The document holds the settings the role lists and nothing else. When
  it holds none it is `{}`. `describe` reads no standard input.
- Every integration speaks the contract, Qory's and yours alike.

The roles the runner calls follow the
[runner's contracts](https://github.com/qoryai/runner/tree/main/contracts/runner/v1):
§Credentials and §Tools.

### Secrets

- The description marks a secret `writeOnly`. The mark is on a property of the settings
  themselves, never on one nested in another.
- A secret has a `title`, its label in a form. It may have an `x-secret-name`, such as
  `GITHUB_APP_PRIVATE_KEY`: the name a control plane suggests for storing it. No two
  secrets have the same one.

  ```json
  "private_key": {"title": "Private key", "type": "string", "writeOnly": true,
                  "x-secret-name": "GITHUB_APP_PRIVATE_KEY"}
  ```

- A secret reaches the program in one of two ways:
  - inline in the document on standard input, under `<name>`, as any other value.
  - as a file whose path the settings define: `<name>_file`. For example,
    `private_key_file` for the secret `private_key`. Only the program's user may read
    that file.
- A program refuses settings that contain both `<name>` and `<name>_file`.

### Each role's settings

Every `credential` and every `tool` role lists, in `settings`, the top-level settings it
may receive:

- plain settings and secrets alike, each by its name;
- a secret by its `<name>`, never by `<name>_file`;
- possibly none: `[]`.

Two roles may list the same setting. Every secret is in some role's list. The runner
writes a role only the settings it lists, a secret as `<name>` or `<name>_file`, and
leaves out every other setting.

A role lists in `required` the settings of its `settings` it needs. A secret is listed
by its `<name>`, and either `<name>` or `<name>_file` satisfies it. A setting the role
lists and does not require is optional: the program handles it being absent.

Each role's document holds a subset of the settings, so the settings schema's top level
carries only keywords no subset can break: `type`, `properties`, `patternProperties`,
`additionalProperties`, `unevaluatedProperties`, `propertyNames`, `title`,
`description`, `$comment`, `$schema`, `$id` and `$defs`. The contract's schema refuses
every other keyword at the top level, such as `required`, `oneOf`, `const`, `enum` or
`$ref`. A rule across settings belongs in a role's `required` or in the program. A
property may carry any keyword.

```json
"settings": {"type": "object", "properties": {
  "url": {"title": "Tracker", "type": "string"},
  "project": {"title": "Default project", "type": "string"},
  "api_key": {"title": "API key", "type": "string", "writeOnly": true},
  "api_key_file": {"title": "API key file", "type": "string"},
  "mcp_key": {"title": "MCP key", "type": "string", "writeOnly": true},
  "mcp_key_file": {"title": "MCP key file", "type": "string"}}},
"roles": {
  "credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"],
                 "settings": ["url", "api_key"], "required": ["url", "api_key"]},
  "tool": {"serves": ["mcp.tracker.acme.example"], "mcp": "https://mcp.tracker.acme.example/mcp",
           "settings": ["url", "mcp_key", "project"], "required": ["url", "mcp_key"]}}
```

The runner checks each role's document before it starts the role, in this order:

1. it holds only the settings the role lists, and nothing in the connection is outside
   every chosen role's list;
2. it holds every setting the role requires;
3. it is valid against the description's `settings`;
4. it does not hold both `<name>` and `<name>_file`.

On standard input, the program:

1. reads standard input to its end, before it acts and before any network call;
2. refuses empty input, a second document or anything but white space after the first,
   and input larger than 64 KiB (65536 bytes);
3. reads the document as it is: nothing in it is replaced or escaped, and a `$` is a
   `$`.

The writer, whatever starts the program, refuses a document larger than 65536 bytes
before it starts the program, writes the document whole, and closes standard input. It
always connects standard input to the document, never to a terminal. The runner always
starts the program with `[<program>, <role>, --, <argument>]` and writes the document
to its standard input. How a run carries the settings is the runner's contract,
[contracts/runner/v1](https://github.com/qoryai/runner/tree/main/contracts/runner/v1).

The contract has the rest, in [§Describe](../contracts/integration/v1/README.md#describe)
and [§Settings](../contracts/integration/v1/README.md#settings).

## Serve a tool

The contract defines the tool role; a runner runs it in a later release.

A tool role serves an MCP server over HTTP, behind the runner's wall. The runner:

1. starts the program as `<program> tool -- <argument>`, outside the agent's enclosure,
   with the variable `QORY_TOOL_LISTEN`, the path of a Unix socket, and `QORY_RUN_ID`.
   The argument is empty when the role has no `argument`;
2. writes the role's settings document on its standard input;
3. sends it every request it allows for the hosts in `serves`, as HTTP/1.1 over the
   socket;
4. sends it SIGTERM when the run ends.

The program:

1. reads standard input to its end, by the rules above;
2. listens on the socket at `QORY_TOOL_LISTEN` within a minute;
3. answers each request, and adds the secret it holds when it forwards one.

A failure before it listens ends as any failure does: non-zero, one line on standard
error, nothing on standard output. Once it listens, what it writes to standard error is
reported as the runner's own lines, and its standard output is discarded.
`QORY_TOOL_LISTEN` and `QORY_RUN_ID` are the only variables it reads. They are not
settings.

`mcp` is the `https://` URL of the MCP server, on a host in `serves`, with no userinfo,
port or fragment. `qory` registers it with the agent's MCP client. `placeholders` lists
variables the enclosure gets with the placeholder value, as a credential's. No host is in
both the credential role's `hosts` and the tool role's `serves`. The runner's
[§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools) has the
rest: the headers the proxy sets, and what a tool decides.

## Names

- The description's `name` is the key a machine's runner file lists the integration
  under, and the name a run's connection uses.
- A program Qory publishes is `qory-<name>`, in the repository `qoryai/qory-<name>`, built
  from its `cmd/qory-<name>/`.
- A program kept elsewhere takes a name of its own, such as `acme-tracker`
  ([TRADEMARKS.md](../TRADEMARKS.md)).
- A Go integration's package is at its repository's root, named after its system.

## Layout

Each integration is one repository. It holds:

- its README,
- its program, under `cmd/<program>/`,
- its description,
- its tests,
- its CI and release workflows. On GitHub they call the shared ones in this repository.

The [integration template](https://github.com/qoryai/integration-template) has this
layout.

## Test it

The Go package `github.com/qoryai/integrations/conformance` checks what a program prints:

- `conformance.Description`: what `describe` printed, against the contract's schema and
  the rules the schema cannot express: where a secret is, its `title`, its
  `<name>_file` and its `x-secret-name`; the settings each role lists and requires;
  hosts in both `hosts` and `serves`; and the tool role's `mcp`. It names each keyword
  the settings' top level carries outside the list above, such as `required`.
- `conformance.Credential`: a credential role's answer, against the runner's schema;
  it also refuses an `apply` entry of the scheme `header` whose `header` the runner
  reserves, as `headers.json` of the
  [runner contract](https://github.com/qoryai/runner/tree/main/contracts/runner/v1) lists
  them.
- `conformance.Failure`: how a failed command ended.

Your tests also check that the program refuses `--settings`: run
`<program> credential --settings '{}' -- a/b`, and hand its exit code, standard output
and standard error to `conformance.Failure`. Go's `flag` package prints a usage text of
several lines when it meets a flag it does not know. A program that uses it sets the
flag set's output to `io.Discard` and writes its own one-line error on standard error.

Test each role the description has:

- **credential**: write the role's settings document to the program's standard input,
  and hand what `<program> credential -- <argument>` prints to `conformance.Credential`.
- **tool**: set `QORY_TOOL_LISTEN` to a socket path under `t.TempDir`, write the role's
  settings document to standard input, wait for the socket, and send HTTP/1.1 requests
  over it. Send the program SIGTERM and check that it exits. Start it with settings it
  refuses, and hand how it ended to `conformance.Failure`.

## Release it

Follow the [release rule](../README.md#release-rule), so `qory` installs your integration
the way it installs any other. The integration's source is where its releases are: a
repository on github.com, gitlab.com or codeberg.org, or an HTTPS URL of a
`description.json`. A reader refuses a forge source on any other host. A URL source may
be on any host the release rule admits.

- A release is a set of files: `description.json`, what `<program> describe` prints,
  the program's archives and `checksums.txt`. A control plane reads `description.json`
  to learn the integration without running it.
- `describe` reports the release's version, `X.Y.Z`, as `program_version`. On a forge
  the tag is `vX.Y.Z`.
- A reader checks `description.json` and each archive against `checksums.txt`, and that
  `program_version` is the version it asked for.
- `describe` names the `publisher`. A reader shows it beside the source's owner, the
  forge namespace or the URL's host, and never instead of it.

The release rule lists where each forge serves a release's files and how a reader finds
the latest release. Private releases, which need an access token, are later.

A Go integration on GitHub calls [`release.yml`](../.github/workflows/release.yml) on
its tags. It builds the program with `-X main.version=X.Y.Z`, the tag without its `v`,
and fails when `describe` reports another `program_version`. Have `describe` report
`main.version` as `program_version`. It then builds
`github.com/qoryai/integrations/cmd/integration-conformance` in your module and runs it,
which checks the description with `conformance.Description`, and fails when it refuses
it. Your `go.mod` requires `github.com/qoryai/integrations`, as your tests already
need, at a version that has `cmd/integration-conformance`. On another forge, publish the
same files with the forge's own CI. goreleaser publishes them to gitlab.com with
`release.gitlab` and `gitlab_urls`, and to codeberg.org with `release.gitea` and
`gitea_urls`.

## Declaring it

The node owner installs an integration with `qory`, from its source. `qory` lists it in
the machine's runner file, in `integrations:`, under the description's `name`, with:

- `path`, the program's path;
- `source`, where it was installed from;
- `description_sha256`, the SHA-256 of the release's `description.json`.

A program of your own may be listed by its `path`, with no `source`. A run's connection
names the integration by the same name. At run start, a reader:

1. runs `<path> describe`, and checks that its `name` is the entry's key;
2. refuses the entry when it records `description_sha256` and the SHA-256 of what
   `describe` printed differs;
3. checks the connection's settings for each role it uses;
4. expands each role it knows.

How a run carries the integration's settings and secrets, and how the runner builds the
document on standard input from them, is the runner's contract,
[contracts/runner/v1](https://github.com/qoryai/runner/tree/main/contracts/runner/v1).
The integration contract lists
[what a reader checks](../contracts/integration/v1/README.md#declaring-an-integration).
`qory`'s [run guide](https://github.com/qoryai/qory/blob/main/docs/run.md#integrations)
shows how to install and declare one.
