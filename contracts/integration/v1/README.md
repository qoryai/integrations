# Integration contract, v1

What a program of an integration reports about itself, and how it is handed its settings.
A reader, `qory` or a control plane, finds and sets up every integration the same way:
Qory's own `qory-<name>` programs and the programs you keep in a repository of your own
alike. The machine's configuration declares each integration and the program that
serves it; the reader runs `<program> describe` and expands the definitions from the
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
| `domains` | the domains the integration works with, each a domain name, `^[a-z][a-z0-9-]{0,63}$`, such as `software`, the domain of software work; at least one and none twice; may be absent |
| `program_version` | the program's own version, a string |
| `settings` | a JSON Schema, draft 2020-12, of type `object`: the settings document the program takes |
| `roles` | the roles the program plays, an object keyed by the role's name; at least one |

```json
{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": {"type": "object"},
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"]}}}
```

**Domains.** `domains` lists the domains the integration works with, each a domain name, for example `software`, the domain of software work; `qory-github` declares `["software"]`. When the integration names no domain, the field is absent, never an empty list, which the schema refuses.

**Secrets.** A property of the settings marked `writeOnly: true` is a secret. A form
shows it as one, written and never read back, a log leaves it out, and it stays off
every command line. A secret is a property of the settings document itself, never one
nested in another, so a reader finds every secret among the settings' `properties`.
Every secret `<name>` has a setting passed on a command line in its place,
`<name>_file`, a file that contains it: for example the secret `private_key` and the
setting `private_key_file`.

## Settings

Every role is started the same way, so a reader needs no template language:

```sh
<program> <role> --settings <json> -- [the role's own arguments]
```

`<json>` is one word of the command line containing the settings document, valid against
the description's `settings`. A program reads its settings from there and from nowhere
else, and keeps none of them. `--` ends the flags, as it does for Go's `flag` package and
POSIX `getopt`, and a program takes it so: an argument a policy defines is never read as
a flag, whatever it starts with.

A command line is visible to the machine's other processes, so the settings passed on a
command line contain no `writeOnly` value. A secret reaches a program as a file whose
path the settings define, such as `private_key_file`, readable by the program's user
alone. A program refuses a `writeOnly` value it receives on its command line, and reports
which setting, never the value.

The runner replaces `${argument}` wherever it appears in an adapter's argument, so a
reader writes every `$` of the settings as `\u0024`, JSON's escape for the same
character: `${argument}` in a setting reaches the program as written, and never as the
policy's argument.

## Exit status

`describe` and every role end the same way. On success a program exits 0 and prints one
document on standard output, the description or the one the role's contract specifies,
and nothing else there. On failure it exits non-zero, prints nothing on standard output,
and writes one line on standard error describing what failed, never a secret. The reader
reports that line as the reason it refuses the declaration when `describe` fails, and the
runner reports it as the reason it refuses the run when a role fails.

## Roles

A reader expands the roles it knows and leaves the others as they are, so one
description serves readers that know different roles.

| Role | Defined | Started as |
|---|---|---|
| `credential` | here | `<program> credential --settings <json> -- ${argument}`: exactly the adapter of a runner definition |

### Credential

The runner's credential adapter
([§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials)):
the program mints or fetches a token for the argument a run's policy defines and prints
the runner's credential document.

| Field | |
|---|---|
| `argument` | a regular expression, RE2, the policy's argument must match whole: the definition's `argument` |
| `hosts` | the hosts the adapter answers for, at least one: the definition's `hosts`, the most an answer may claim |

`conformance.Credential` checks the printed document against the runner's schema, and
refuses an `apply` entry of the scheme `header` whose `header` is one it reserves: a name
in `conformance/headers.json`'s `refused`, or one that starts with a prefix in its
`refused_prefixes`, compared in lower case.

## Declaring an integration

`qory` reads the `integrations:` section of a machine's `runner.yaml`:

```yaml
integrations:
  <key>:
    program: <program>     # may be left out when the program is qory-<key> on the PATH
    settings: {...}        # the settings document; absent is {}
```

A reader expands each declared integration:

1. The program is `program`. Absent, it is `qory-<key>`, found on the `PATH`, which
   is the case for the integrations Qory publishes declared under their own name; an
   integration of your own defines its program, by a path or a name on the `PATH`, and
   one of Qory's declared under another key defines it too.
2. It runs `<program> describe` and refuses the declaration unless the program exits 0
   with a description this schema accepts.
3. It checks the settings against the description's `settings`, and refuses a
   `writeOnly` value among them, since every role it expands is started with them on a
   command line.
4. For the `credential` role it defines the credential `<key>`:
   `adapter: [<program>, credential, --settings, <json>, --, "${argument}"]`, with
   `<json>` the settings as compact JSON, every `$` in it written `\u0024`, and the
   role's `argument` and `hosts`. Written in YAML, `<json>` is a single-quoted scalar,
   in which nothing is an escape but a quote, doubled, so the word reaches the runner as
   it was written.
5. It expands the roles it knows, and leaves a description's other roles as they are.

Declared:

```yaml
integrations:
  github: {settings: {app_id: 123456, private_key_file: /etc/qory/github-app.pem}}
  tracker: {program: /opt/acme/bin/acme-tracker, settings: {project: "it's $X"}}
```

`qory-github describe` answers [`fixtures/github.json`](fixtures/github.json) and
`/opt/acme/bin/acme-tracker describe` answers
[`fixtures/acme-tracker.json`](fixtures/acme-tracker.json), and the declaration expands
to the runner's definitions:

```yaml
credentials:
  github:
    adapter: [qory-github, credential, --settings, '{"app_id":123456,"private_key_file":"/etc/qory/github-app.pem"}', --, "${argument}"]
    argument: '[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9_.-]{1,100}(,[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9_.-]{1,100})*'
    hosts: [github.com, api.github.com]
  tracker:
    adapter: [/opt/acme/bin/acme-tracker, credential, --settings, '{"project":"it''s \u0024X"}', --, "${argument}"]
    argument: '[A-Z]+'
    hosts: [tracker.acme.example]
```

The tracker's `$X` is written `\u0024X`, which JSON reads as `$X`, and the quote of
`it's` is doubled in the single-quoted scalar: the program reads `it's $X`.

A run's policy selects them by the key, as it selects any credential:
`{name: github, argument: acme/shop}`.

## Fixtures

| Path | Contains | Validated against |
|---|---|---|
| `fixtures/*.json` | descriptions that are accepted: `github.json`, what `qory-github describe` printed at 0.1.0, built without a version; `acme-tracker.json`, the least a description of your own contains; `acme-chat.json`, one that serves two domains; `unknown-role.json`, one with `acme_role`, a role the contract does not define, beside `credential` | `description.schema.json` |
| `fixtures/invalid/` | descriptions the schema refuses, named `description-<reason>` | `description.schema.json`, expecting a failure |

Every fixture is synthetic. No host name of anyone's infrastructure and no real secret.

## Sources

- [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-validation),
  §9.4 for `writeOnly`.
- The runner's contract, [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials),
  for the adapter definition a credential role expands to.
