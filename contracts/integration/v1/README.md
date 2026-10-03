# Integration contract, v1

What a program of an integration reports about itself, and how it is handed its settings.
A reader, `qory` or a control plane, finds and sets up every integration the same way:
Qory's own `qory-<name>` programs and the programs you keep in a repository of your own
alike. The machine's configuration declares each integration and the program that
serves it; the reader runs `<program> describe` and checks the declaration against the
answer alone. This directory is the contract: the description's JSON schema and the
fixtures a program or a reader is tested against.

## Versions

The description contains `version: 1`, an integer, and its schema is addressed by URL
under `https://qory.dev/contracts/integration/v1/`, as the runner's documents are
([§Versions](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#versions)).
A reader reads the versions it knows and refuses a description of any other. An
addition keeps `version` 1; a change that breaks a reader is `version` 2.

## Describe

```sh
<program> describe
```

prints one JSON document, `description.schema.json`, and exits 0. It takes no settings
and reaches no network, so a reader runs it before it has any and a form is drawn
from it.

| Field | |
|---|---|
| `version` | `1`, the contract's version |
| `name` | the integration's name, `^[a-z0-9][a-z0-9_-]{0,63}$`: the key it is declared under, such as `github`, unless the machine chooses another. The key is the name of the runner's credential, `^[a-z0-9][a-z0-9_.-]{0,63}$`, and of a program Qory publishes, `qory-<key>`, so the name is the credential's grammar without the dot, which in a program's name reads as an extension |
| `title`, `description` | human text, for a form or a listing; `description` may be absent |
| `domains` | the domains the integration serves, each a domain name, `^[a-z][a-z0-9-]{0,63}$`, such as `software`, the domain of software work; at least one and none twice; may be absent |
| `program_version` | the program's own version, a string |
| `settings` | a JSON Schema, draft 2020-12, of type `object`: the settings document the program takes |
| `roles` | the roles the program plays, the ways it offers, an object keyed by the role's name; at least one (§Roles) |

```json
{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": {"type": "object"},
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": []}}}
```

**Domains.** A control plane offers a workspace the integrations whose `domains` include
the workspace's domain name exactly, the same string, and those whose description has no
`domains` in every domain: the field is absent, never an empty list, which the schema
refuses. A domain name is, for example, `software`, the domain of software work;
`qory-github` declares `["software"]`. A declared integration expands the same whatever
its domains: they decide what is offered, not what runs.

**Secrets.** A property of the settings marked `writeOnly: true` is a secret. A form
shows it as one, written and never read back, and a log leaves it out. A secret is a
property of the settings document itself, never one nested in another, so a reader finds
every secret among the settings' `properties`.
Every secret has a `title`, a string that is not only white space: its label, which a
form shows. Every secret `<name>` has a setting `<name>_file`, the path of a file that
contains it, for a secret kept on disk: for example the secret `private_key` and the
setting `private_key_file`. Every secret is in the `settings` of a role, the credential
role's or the tool role's, by its `<name>` (§Roles).

A secret may carry `x-secret-name`, the conventional name of the secret,
`^[A-Z][A-Z0-9_]{0,127}$`. A control plane that stores secrets by name fills it in as
the name to store the secret under. It is a suggestion, not an identity: the control
plane may store the secret under another name, and the program never sees the name. A
setting that is not a secret carries no `x-secret-name`, and no two secrets of a
description carry the same one. Like `title`, it sits on the property it describes, and
JSON Schema reads it as an annotation:

```json
"private_key": {"title": "Private key", "type": "string", "writeOnly": true,
                "x-secret-name": "GITHUB_APP_PRIVATE_KEY"},
"private_key_file": {"title": "Private key file", "type": "string"}
```

The schema checks a description's `settings` against JSON Schema's own meta-schema,
which cannot express the rules of the two paragraphs above: that a secret is a property
of the settings themselves, has a `title` and a `<name>_file`, is in a role's
`settings`, and where `x-secret-name` may be and what it may contain. Nor can it express
the rules of §Settings and §Roles that join one part of the description to another: what
a role's `settings` and `required` name, that the credential role's `hosts` and the tool
role's `serves` do not overlap, and where the tool role's `mcp` is.
The Go package `github.com/qoryai/integrations/conformance` checks them. The schema
itself refuses the top-level keywords of §Settings, and the package names each one.

## Settings

Every role is started the same way, so a reader needs no template language:

```sh
<program> <role> -- [the role's own arguments]
```

`--` is always there, even when the role has no arguments, and an argument after it is
never read as a flag. The credential role is `<program> credential -- <argument>`, and
the tool role is `<program> tool -- <argument>`: exactly one argument each. A role with
an `argument` pattern gets the run's argument, which is empty when the run gives none.
A role without `argument` ignores the run's argument and is started with an empty one
(§Roles). A program takes no flags for a role, and refuses `--settings` as it refuses any
flag it does not know, by the rules of §Exit status.

Standard input carries exactly one JSON document, the settings object of the role,
valid against the description's `settings`, a secret's value in it as any other value.
The document holds the settings the role lists and nothing else, a secret as `<name>`
or `<name>_file`. The runner leaves out every other setting. For example, the credential
role of `fixtures/github.json`:

```json
{"app_id": 123456, "private_key": "-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----\n"}
```

What a role needs is its `required`, a subset of its `settings` (§Roles). Each role's
document holds a subset of the settings and is valid against the one `settings`. So the
top level of the settings schema carries no keyword a subset can break: `required`,
`allOf`, `anyOf`, `oneOf`, `not`, `if`, `then`, `else`, `dependentRequired`,
`dependentSchemas`, `minProperties`, `maxProperties`, `$ref` or `$dynamicRef`. The
schema refuses each of them. A rule across settings belongs in a role's `required` or in
the program. The top level keeps `properties`, `patternProperties`,
`additionalProperties`, `propertyNames`, `type`, `title`, `description`, `$schema`,
`$id` and `$defs`, and a property may carry any keyword: a `required` inside a setting
of type `object` is JSON Schema's as usual.

The runner checks each started role's document, in this order:

1. It holds only the names the role lists in `settings`, a secret as `<name>` or
   `<name>_file`, and nothing in the connection falls outside every chosen role's list.
2. Every name in the role's `required` is present, a secret in either form.
3. It is valid against the description's `settings`.
4. It does not hold both `<name>` and `<name>_file`.

When the document holds no setting it is `{}`. The program reads standard input to
its end before it acts and before any network call. It refuses empty input, a second
document or anything but white space after the first, and input larger than 64 KiB,
65536 bytes. The writer refuses a larger document before it starts the program, writes
the document whole, and closes standard input. The writer always connects standard input
to the document, never to a terminal. Nothing in the document is replaced and nothing is
escaped: it is data, and a `$` in it is a `$`. `describe` reads no standard input.

A program reads its settings from standard input alone and keeps none of them: it reads
no setting and no secret from its environment.

A secret has one source: either its value inline in the document under `<name>`, or
`<name>_file`, a path the settings define for a secret kept on disk, readable by the
program's user alone. Settings with both `<name>` and `<name>_file` are refused.

## Exit status

`describe` and every role end the same way. On success a program exits 0 and prints one
document on standard output, the description or the one the role's contract specifies,
and nothing else there. On failure it exits non-zero, prints nothing on standard output,
and writes one line on standard error describing what failed, never a secret. The reader
reports that line as the reason it refuses the declaration when `describe` fails, and the
runner reports it as the reason it refuses the run when a role fails.

The tool role prints no document: it serves until the runner ends it (§Tool). A tool
that fails before it listens ends as any failure does.

## Roles

An integration offers one or more ways, and a way is a role of its description: the
credential role for an API, the tool role for an MCP server. A run uses an integration
in the ways its connection names: the connection's `ways`, at least one, names roles of
the description; the runner starts those roles alone
([runner contract](https://github.com/qoryai/runner/tree/main/contracts/runner/v1)). An
API that needs only a static key needs no program and no role: the runner's services
cover it, and the description does not declare it.

A reader expands the roles it knows and leaves the others as they are, so one
description serves readers that know different roles.

| Role | Defined | Started as |
|---|---|---|
| `credential` | here | `<program> credential -- <argument>` |
| `tool` | here | `<program> tool -- <argument>` |
| `work_source` | reserved, for a contract of its own | |
| `output` | reserved, for a contract of its own | |

Every credential and every tool role has `settings`: the names of the top-level
settings the role may receive, plain and secret alike, none twice, possibly none. Two
roles may list the same setting. A secret is listed by its `<name>`, never by
`<name>_file`. The role's standard input holds those settings alone (§Settings).

The run's argument is matched, whole, only against the roles that have an `argument`
pattern. A role without `argument` ignores the run's argument and is started with an
empty one. So a credential role with a pattern and a tool role with none both run in one
connection.

A role may have `required`: the names of its `settings` it needs, none twice. A secret
is listed by its `<name>`, and either `<name>` or `<name>_file` in the document
satisfies it. A setting the role lists and does not require is optional: the program
handles it being absent.

```json
"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"],
               "settings": ["url", "project", "api_key"], "required": ["url", "api_key"]}
```

Hosts are per role. The runner's proxy sets the credential role's access token on its
`hosts`, and sends the requests to the tool role's `serves` to the tool. A host is never
in both, the same host or one a `*.` pattern covers.

### Credential

The runner's credential adapter
([§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials)):
the program mints or fetches an access token for the run's argument and prints the
runner's credential document. It is started as `<program> credential -- <argument>`,
reads its settings on standard input, prints the document and exits.

| Field | |
|---|---|
| `argument` | a regular expression, RE2, the run's argument must match whole |
| `hosts` | the hosts the program answers for, at least one, the most an answer may claim: a lower-case host name, or `*.` followed by one for every host below it |
| `settings` | the settings the role may receive, by name |
| `required` | the settings of `settings` the role needs, by name; may be absent |

### Tool

The runner's tool
([§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools)): an MCP
server reached over HTTP behind the runner's wall, so its secrets stay outside the
agent's enclosure. A stdio MCP server the agent would start inside the enclosure is
outside this contract.

| Field | |
|---|---|
| `argument` | a regular expression, RE2, the run's argument must match whole; may be absent. A tool role without it ignores the run's argument and is started with an empty one (§Roles) |
| `serves` | the hosts whose requests the runner sends to the tool, at least one, none twice, in the grammar of the credential role's `hosts` |
| `mcp` | the `https://` URL of the MCP server the tool serves, with no userinfo, port or fragment, its host one of `serves`; may be absent. `qory` registers it with the agent's MCP client |
| `placeholders` | variables the enclosure gets with the placeholder value, as a credential's ([§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials)), each `^[A-Z_][A-Z0-9_]{0,127}$`, none twice; may be absent. A credential role has no `placeholders`: its answer lists them |
| `settings` | the settings the role may receive, by name |
| `required` | the settings of `settings` the role needs, by name; may be absent |

```json
"tool": {"serves": ["mcp.tracker.acme.example"], "mcp": "https://mcp.tracker.acme.example/mcp",
         "settings": ["url", "mcp_key"], "required": ["url", "mcp_key"]}
```

The runner starts the program as `<program> tool -- <argument>`, the argument empty when
the role has no `argument`. The program reads its settings document on standard input to
its end, by the rules of §Settings. Then it listens on the Unix socket whose path is the variable `QORY_TOOL_LISTEN`, within a
minute. The runner sends it every request it allows for `serves` as HTTP/1.1 over that
socket. A failure before it listens follows §Exit status: non-zero, one line on standard
error, nothing on standard output. Once it listens, what it writes to standard error is
reported as the runner's own lines, and its standard output is discarded. SIGTERM ends
it. `QORY_TOOL_LISTEN` and `QORY_RUN_ID` are the only variables it reads, and they are
not settings. The runner's §Tools has the rest: the socket, the headers the proxy sets,
and what the tool decides.

## Declaring an integration

A machine's configuration declares each integration under a key, with its settings. A
reader checks each declared integration:

1. The program is the one the declaration names, by a path or a name on the `PATH`.
   When it names none, the program is `qory-<key>`, found on the `PATH`, which is the
   case for the integrations Qory publishes declared under their own name.
2. It runs `<program> describe` and refuses the declaration unless the program exits 0
   with a description this schema accepts.
3. It checks the settings against the description's `settings`.
4. It expands the roles it knows, and leaves a description's other roles as they are.

How a run carries an integration's settings and secrets, and how the runner builds the
document of §Settings from them, is the runner's contract,
[contracts/runner/v1](https://github.com/qoryai/runner/tree/main/contracts/runner/v1).

## Fixtures

| Path | Contains | Validated against |
|---|---|---|
| `fixtures/*.json` | descriptions that are accepted: `github.json`, what `qory-github describe` printed at 0.1.0, built without a version, with the `x-secret-name` of its secret, the credential role's `settings` and `required` in place of the settings' top-level `required`, and no top-level `oneOf` over `private_key` and `private_key_file`; `acme-tracker.json`, the least a description of your own contains; `acme-chat.json`, one that serves two domains; `acme-tracker-mcp.json`, one with both ways, a credential role and a tool role with `mcp`, a secret each and one plain setting they share; `unknown-role.json`, one with `work_source`, a reserved role, beside `credential` | `description.schema.json` |
| `fixtures/invalid/` | descriptions the schema refuses, named `description-<reason>` | `description.schema.json`, expecting a failure |

Every fixture is synthetic. No host name of anyone's infrastructure and no real secret.

## Sources

- [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-validation),
  §9.4 for `writeOnly`.
- The runner's contract, [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials),
  for the credential document a credential role prints, and
  [§Tools](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#tools), for how
  the runner starts a tool role and reaches it.
