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
- Every role is started the same way:

  ```sh
  # <json>: the settings, one word of the command line
  <program> <role> --settings <json> -- [the role's own arguments]
  ```

- For the credential role, that command line is exactly the adapter of a runner
  definition.
- Every integration speaks the contract, Qory's and yours alike.

The roles the runner calls follow the
[runner's contracts](https://github.com/qoryai/runner/tree/main/contracts/runner/v1):
§Credentials and §Tools.

### Secrets

- The settings go on a command line. The machine's other processes can see it. So a
  secret is refused there.
- The description marks a secret `writeOnly`. The mark is on a property of the settings
  themselves, never on one nested in another.
- A secret reaches the program as a file whose path the settings define: `<name>_file`.
  For example, `private_key_file` for the secret `private_key`. Only the program's user
  may read that file.
- A program refuses a `writeOnly` value on its command line. It reports which setting,
  never the value.

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
  the secret rule.
- `conformance.Credential`: a credential role's answer, against the runner's schema;
  it also refuses an `apply` entry of the scheme `header` whose `header` is one it
  reserves, as `conformance/headers.json` lists them.
- `conformance.Failure`: how a failed command ended.

## Release it

Follow the [release rule](../README.md#release-rule), so `qory` installs your integration
the way it installs any other. A Go integration calls
[`release.yml`](../.github/workflows/release.yml) on its tags.

## Declaring it

A machine's `runner.yaml` declares each integration under `integrations:`, by name:

- its settings,
- for a program kept elsewhere, `program:`: the program's path, or a name on the `PATH`.

`qory` then:

1. runs the program's `describe`,
2. checks the settings against the description,
3. turns each role it knows into the runner's definition.

The credential role becomes a credential of the same name. A run's policy selects it:
`{name: github, argument: acme/shop}`.

- [qory-github's README](https://github.com/qoryai/qory-github#4-declare-the-integration)
  shows a declaration, and the policy that selects its credential.
- The integration contract shows
  [how a reader expands one](../contracts/integration/v1/README.md#declaring-an-integration)
  into the runner's definition, an integration of your own among them.
