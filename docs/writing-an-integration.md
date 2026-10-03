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

| Role                | In a description | Called by                                   |
| ------------------- | ---------------- | ------------------------------------------- |
| credential adapter  | `credential`     | the runner, outside the wall, per run       |
| tool                | `tool`           | the runner's proxy, for the hosts it serves |
| work-source adapter | `work_source`    | the control plane                           |
| output adapter      | `output`         | the control plane                           |

What each role does:

- **credential adapter**: mints an access token for what a run requests. It lists the
  hosts and paths the access token is used on. The runner keeps the access token outside
  the session.
  Its contract is the runner's
  [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials):
  one JSON document on standard output.
- **tool**: serves the session's requests to its hosts: a protocol, a
  signature, a service of the machine's. Its contract is the runner's
  [§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools): HTTP
  over a Unix socket.
- **work-source adapter**: brings work items and their events in. It has no contract
  yet.
- **output adapter**: applies a change request to the system it is for. It has no
  contract yet.

The integration contract defines `credential`. It reserves `tool`, `work_source` and
`output`, each for a contract of its own. A reader expands the roles it knows, and leaves
the others as they are.

## The contract in short

- `<program> describe` prints the integration's description: one JSON document. It
  contains:
  - the integration's name,
  - the domains it serves,
  - its settings, as a JSON Schema,
  - the roles it plays.
- Every role is started the same way, the settings one JSON document on standard input:

  ```sh
  <program> <role> -- [the role's own arguments]
  ```

- `--` is always there, even when the role has no arguments. The credential role gets
  exactly one argument, the run's, empty when the run gives none:
  `<program> credential -- <argument>`.
- A program takes no flags for a role. It refuses `--settings` as it refuses any flag it
  does not know.
- A program reads its settings from standard input alone: no setting and no secret from
  its environment. When the settings are empty the document is `{}`. `describe` reads no
  standard input.
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

On standard input, the program:

1. reads standard input to its end, before it acts and before any network call;
2. refuses empty input, a second document or anything but white space after the first,
   and input larger than 64 KiB (65536 bytes);
3. reads the document as it is: nothing in it is replaced or escaped, and a `$` is a
   `$`.

The writer, whatever starts the program, refuses a document larger than 65536 bytes
before it starts the program, writes the document whole, and closes standard input. It
always connects standard input to the document, never to a terminal. The runner always
starts the program with `[<program>, credential, --, <argument>]` and writes the document
to its standard input. How a run carries the settings is the runner's contract,
[contracts/runner/v1](https://github.com/qoryai/runner/tree/main/contracts/runner/v1).

The contract has the rest, in [§Describe](../contracts/integration/v1/README.md#describe)
and [§Settings](../contracts/integration/v1/README.md#settings).

## Names

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
- its CI and release workflows, which call the shared ones in this repository.

The [integration template](https://github.com/qoryai/integration-template) has this
layout.

## Test it

The Go package `github.com/qoryai/integrations/conformance` checks what a program prints:

- `conformance.Description`: what `describe` printed, against the contract's schema and
  the secret rules: where a secret is, its `title`, its `<name>_file` and its
  `x-secret-name`.
- `conformance.Credential`: a credential role's answer, against the runner's schema.
- `conformance.Failure`: how a failed command ended.

Your tests also check that the program refuses `--settings`: run
`<program> credential --settings '{}' -- a/b`, and hand its exit code, standard output
and standard error to `conformance.Failure`. Go's `flag` package prints a usage text of
several lines when it meets a flag it does not know. A program that uses it sets the
flag set's output to `io.Discard` and writes its own one-line error on standard error.

## Release it

Follow the [release rule](../README.md#release-rule), so `qory` installs your integration
the way it installs any other. A Go integration calls
[`release.yml`](../.github/workflows/release.yml) on its tags.

- The release attaches `description.json`, what `<program> describe` prints. A control
  plane reads it to learn the integration without running it.
- `release.yml` builds the program with `-X main.version=X.Y.Z`, the tag without its
  `v`, and fails when `describe` reports another `program_version`. Have `describe`
  report `main.version` as `program_version`.

## Declaring it

A machine's configuration declares each integration by name:

- its settings,
- for a program kept elsewhere, the program's path, or a name on the `PATH`.

`qory` then:

1. runs the program's `describe`,
2. checks the settings against the description,
3. expands each role it knows.

How a run carries the integration's settings and secrets, and how the runner builds the
document on standard input from them, is the runner's contract,
[contracts/runner/v1](https://github.com/qoryai/runner/tree/main/contracts/runner/v1).
The integration contract lists
[what a reader checks](../contracts/integration/v1/README.md#declaring-an-integration).
