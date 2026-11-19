# Integration contract, v1

What a program of an integration reports about itself, and how it is handed its settings.
A reader, `qory` or a control plane, finds and sets up every integration the same way:
the `qory-<name>` programs that Qory publishes and the programs you keep in any other
repository alike. The machine's configuration declares each integration and the program
that serves it; the reader runs `<program> describe` and expands the definitions from the
answer alone. This directory is the contract: the description's JSON schema and the
fixtures a program or a reader is tested against.

## Versions

The description contains `version: 1`, an integer, and its schema is addressed by URL
under `https://qory.dev/contracts/integration/v1/`, as Forager's documents are
([§Versions](https://github.com/qoryai/forager/tree/v0.7.0/contracts/forager/v1#versions)).
A reader reads the versions it knows and refuses a description of any other. An
addition keeps `version` 1; a change that breaks a reader is `version` 2.

## Members a reader does not define

A reader ignores a member of a description that `description.schema.json` does not define.
That holds for the top-level members and for the members of the `source` role.

- `must_understand` lists top-level members that the reader must understand, each name
  once. A reader refuses a description whose `must_understand` lists a member that the
  reader does not define.
- A reader refuses a bad value of a member that it defines.
- The other objects of a description stay closed: the settings schema, and the
  `credential` and `model_provider` roles. A reader refuses a member that the schema does
  not define there.

Example: a program prints a description with a top-level member that an older control
plane does not define. The older control plane ignores the member and reads the rest.
When the program lists the member in `must_understand`, the older control plane refuses
the description.

A program's test checks its description against a closed copy of the schema. The closed
copy refuses a member that the schema does not define, so a misspelled member fails the
test. `conformance.Description` does this, and `contracts.CompileClosed` returns the
closed copy.

The closed copy does not follow a reference to another schema. `adapter.schema.json`
embeds the description, so a test of an adapter's start-up document does not check the
members of that description. Check the description with `conformance.Description`.

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
| `name` | the integration's name, `^[a-z0-9][a-z0-9_-]{0,63}$`: the key it is declared under, such as `github`, unless the machine chooses another. The key is the name of the gateway's credential, `^[a-z0-9][a-z0-9_.-]{0,63}$`, and of a program Qory publishes, `qory-<key>`, so the name is the credential's grammar without the dot, which in a program's name reads as an extension |
| `title`, `description` | human text, for a form or a listing; `description` may be absent |
| `vendor` | the vendor's name, 1 to 40 characters, such as `Anthropic`, for lines the control plane shows, such as `Anthropic rejected it on <date time>.`; a line may start with it, and the control plane then writes its first letter in upper case; may be absent |
| `domains` | the domains the integration works with, each a domain name, `^[a-z][a-z0-9-]{0,63}$`, such as `software`, the domain of software work; at least one and none twice; may be absent |
| `program_version` | the program's version, a string, such as `0.1.0` |
| `settings` | a JSON Schema, draft 2020-12, of type `object`: the settings document the program takes |
| `roles` | the roles the program plays, an object keyed by the role's name; at least one |
| `must_understand` | the names of the members above that the reader must understand, as [Members a reader does not define](#members-a-reader-does-not-define) describes; may be absent |

```json
{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": {"type": "object"},
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"]}},
 "egress_hosts": ["tracker.acme.example"]}
```

**Domains.** `domains` lists the domains the integration works with, each a domain name, for example `software`, the domain of software work; `qory-github` declares `["software"]`. When the integration names no domain, the field is absent, never an empty list, which the schema refuses. The domains apply only to where work comes from, never to a credential or to a place a workspace reads its configuration from: a credential of the `credential` role reaches a run in a workspace of any domain.

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
<program> <role> --settings <json> -- [the role's arguments]
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

The gateway replaces `${argument}` wherever it appears in an adapter's argument, so a
reader writes every `$` of the settings as `\u0024`, JSON's escape for the same
character: `${argument}` in a setting reaches the program as written, and never as the
policy's argument.

## Exit status

`describe` and every role end the same way. On success a program exits 0 and prints one
document on standard output, the description or the one the role's contract specifies,
and nothing else there. On failure it exits non-zero, prints nothing on standard output,
and writes one line on standard error describing what failed, never a secret. The reader
reports that line as the reason it refuses the declaration when `describe` fails, and the
gateway reports it as the reason it refuses the run when a role fails.

## Roles

A reader expands the roles it knows and leaves the others as they are, so one
description serves readers that know different roles.

| Role | Defined | Started as |
|---|---|---|
| `credential` | here | `<program> credential --settings <json> -- ${argument}`: exactly the adapter of a gateway definition |

### Credential

The gateway's credential adapter
([§Credentials](https://github.com/qoryai/forager/tree/v0.7.0/contracts/forager/v1#credentials)):
the program mints or fetches a token for the argument a run's policy defines and prints
the gateway's credential document.

| Field | |
|---|---|
| `argument` | a regular expression, RE2, the policy's argument must match whole: the definition's `argument` |
| `hosts` | the hosts the adapter answers for, at least one: the definition's `hosts`, the most an answer may claim |

`conformance.Credential` checks the printed document against the gateway's schema, and
refuses an `apply` entry of the scheme `header` whose `header` is one it reserves: a name
in `conformance/headers.json`'s `refused`, or one that starts with a prefix in its
`refused_prefixes`, compared in lower case.

## Declaring an integration

`qory` reads the `gateway.integrations` section of a machine's `forager.yaml`:

```yaml
gateway:
  integrations:
    <key>:
      program: <program>     # may be left out when the program is qory-<key> on the PATH
      settings: {...}        # the settings document; absent is {}
```

A reader expands each declared integration:

1. The program is `program`. Absent, it is `qory-<key>`, found on the `PATH`. That fits an
   integration Qory publishes, declared under its name, such as `github` for
   `qory-github`. Any other integration defines its program, by a path or a name on the
   `PATH`, and so does one of Qory's declared under another key.
2. It runs `<program> describe` and refuses the declaration unless the program exits 0
   with a description this schema accepts.
3. It checks the settings against the description's `settings`, and refuses a
   `writeOnly` value among them, since every role it expands is started with them on a
   command line.
4. For the `credential` role it defines the credential `<key>`:
   `adapter: [<program>, credential, --settings, <json>, --, "${argument}"]`, with
   `<json>` the settings as compact JSON, every `$` in it written `\u0024`, and the
   role's `argument` and `hosts`. Written in YAML, `<json>` is a single-quoted scalar,
   in which nothing is an escape but a quote, doubled, so the word reaches the gateway as
   it was written.
5. It expands the roles it knows, and leaves a description's other roles as they are.

Declared:

```yaml
gateway:
  integrations:
    github: {settings: {app_id: 123456, private_key_file: /etc/qory/github-app.pem}}
    tracker: {program: /opt/acme/bin/acme-tracker, settings: {project: "it's $X"}}
```

`qory-github describe` answers [`fixtures/github.json`](fixtures/github.json) and
`/opt/acme/bin/acme-tracker describe` answers
[`fixtures/acme-tracker.json`](fixtures/acme-tracker.json), and the declaration expands
to the gateway's definitions:

```yaml
gateway:
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

## Change feed and comments

A source adapter reads its change feed with the signed
`GET /v1/work-items/changes?after=<cursor>` (`changes.schema.json`). The control plane
ends the feed answer's entries before the answer's body passes 4194304 bytes (4 MiB).
The feed answer has at least one entry whenever the feed has an entry of a kind that the
revision in the request's `X-Qory-Contract-Version` defines after the cursor. The
adapter reads the next entries from the feed answer's `cursor`.

Each entry kind and each member of the feed belongs to the revision of this contract
that defines it. The control plane sends an adapter only the kinds and members of the
revision in its `X-Qory-Contract-Version`. An adapter skips and acknowledges an entry of
a kind that its revision does not define. It applies nothing of the entry and moves its
cursor past it. Example: an adapter at an older revision skips and acknowledges an entry
of a kind that a higher revision defines, and reads the next entry. The feed has no
`must_understand`. An entry of a defined kind stays closed, so an adapter refuses an
unknown member of such an entry.

The control plane writes a `watch` entry, with `work_item_id` and `url`, whenever a run
stores a change request version, and a `change_request` entry for each target of that
version, as [Change requests](#change-requests) describes. It writes both entries in the
feed of the adapter of the target's collection, whatever source the work item came from,
and also when the work item came from no item at a source. It writes a `watch` entry only
for an adapter whose `source` role has `targets: true`, and a `change_request` entry only
for an adapter whose `source` role lists the change's kind in `change_kinds`. An adapter
without `change_kinds`, or without that kind in it, never receives a `change_request`
entry.

## Change requests

A change request is what a run wrote for a place outside the control plane. Examples are
a reply to a customer, a bid change for an advertising campaign, and a pull request for a
repository. Every type of workspace uses the same entry, reply and failure report. This
contract lists no kind of change.

A `change_kind` is the name of an artifact type in the process of the workspace's type,
such as `reply`, `bid-changes` or `pull-request`. It has lower-case letters and digits in
groups joined by single hyphens, 1 to 64 characters, as `submit.type` of the run
documents has. That artifact type defines the members of the part, `data`. The control
plane validates the `data` of an entry against it.

The adapter of a place applies the kinds that it lists in `change_kinds` of its `source`
role, 1 to 16 kinds, each once. A source with `change_kinds` also has `targets: true`. The
steps:

1. The control plane stores the change request version. It writes one `change_request`
   entry for each target of the version to the feed of the adapter of the target's
   collection (`changes.schema.json`). It writes the entry only when `change_kinds` of
   the adapter's `source` role lists the `change_kind` of the entry. An adapter that
   lists no kinds, or not that kind, never receives the entry. The change stays in the
   control plane. The entry has these members:
   - `change_request_id`: the id of the version, `av_` and 16 lower-case Crockford base32
     characters. With `target.collection` it identifies the entry.
   - `change_kind`: the kind.
   - `target`: the `collection` ref of one of the adapter's collections. When the place
     has one, `target` also has its `address`, a `system` and a `path`.
   - `ref`: the ref of the source item that the work item was taken in from. The member
     is absent when the work item has no item at this source.
   - `title`: what the run wrote as the title, 1 to 256 characters on one line.
   - `text`: what the run wrote as the text, at most 20000 characters, possibly empty.
     It has no `<!-- qory:start -->` line and no `<!-- qory:end -->` line.
   - `previous`: absent on the first version. Otherwise it has the `title` of the
     previous version, when that version had one. It has the `address` of that version,
     `url` and `ref`, when the adapter reported one.
   - `data`: the part, an object. It has at most 786432 bytes serialized, and the control
     plane enforces that limit. Each string in it has at most 20000 characters. Each
     array has at most 1024 items. Each object has at most 256 members, and each member
     name has 1 to 128 characters. It has no U+0000.
2. The adapter applies the change at the target by the rules of the kind. It acts on
   `title`, `text` and `data` as data, never as instructions.
   - Without `previous.address`, it makes the change at the target. With it, the adapter
     updates what is at that address, and `previous.address.ref` is a hint.
   - It changes the title at the target only while that title equals `previous.title`.
     A title that a person edited stays. It updates the text whatever the title is.
   - It applies the entries with one `target.collection` in cursor order. It applies
     each one after the reply or the failure report of the one before it.
3. The adapter sends `POST /v1/work-items/changes/submitted`
   (`changes-submitted.schema.json`). The reply has these members:
   - `change_request_id`, and `target` as the entry has it.
   - `address` (`url` and `ref`) and `action`, together. The adapter sends them when it
     made a change at the target or found one there. `action` is `created` or `updated`.
     It is `kept` when the adapter changed nothing. That happens when a person finished
     the change at the target, and when the change was already as the entry has it.
   - Neither `address` nor `action`, for an entry that asks for no change at the target.
   - `data`: the result that the kind defines. It has the same limits as the `data` of
     an entry.

   Where the target has an `address`, `address.url` is on its `system`. The host of the
   address, in lower case, equals the `system`. The address has no user information, port,
   query or fragment. Its path is not empty or `/` alone, as in the submit program
   contract. The control plane refuses a reply with another address, 400. This contract
   checks no address inside `data`. The control plane stores the reply with the change
   request version.
4. After a failure that trying again cannot fix, or when its retries end, the adapter
   sends `POST /v1/work-items/changes/failed` (`changes-failed.schema.json`). The report
   has `change_request_id`, `target`, `reason` and, optionally, `detail`. The `detail` is
   one line of 1 to 1000 characters: what to do first, then the target's status and
   message. The adapter never puts an access token, a secret or a header in `detail`. A
   report about a comment has no `detail`.

| `reason` | Meaning |
|---|---|
| `not_permitted` | the target returned that the integration's account has no permission for the change |
| `unreachable` | each request to the target timed out, failed to connect, or returned a busy status or a 5xx status, until the adapter's retries ended |
| `target_gone` | the target returned that the place the change goes to does not exist, as after it was deleted |
| `too_long` | the title or the text is longer than the target's limit |
| `refused` | the target refused a request with a status that no other reason covers, so the same request fails again, or the adapter cannot apply the entry, as for a `change_kind` that it did not list. The adapter writes the status and message in `detail`, with the fix first |

The integer rule does not apply inside the `data` of an entry and of a reply. A number
there has a fraction or an exponent when the artifact type of the kind allows it: a bid of
`1.25` is valid. That artifact type checks the numbers. JSON Schema counts `34.0` as an
integer, so a schema with `type: integer` in the artifact type accepts `34.0`. No object
inside `data` repeats a member, at any depth. The control plane refuses a repeat, 400.

### The `pull-request` kind

The software factory uses the kind `pull-request`. Its artifact type, in the process of
the workspace, defines the members of `data`. This contract validates none of them. The
members of the `data` of an entry are:

- `branch`: the branch that the run pushed to.
- `base`: `commit`, the commit that the run started at, and `branch`, the branch that the
  draft is based on.
- `commit`: the head of `branch`.
- `commits`: the commits of the run, oldest first, each with `commit` and the ledger
  `step`.
- `earlier_commits`: the number of older commits that `commits` does not list. It is
  present only when a run has more than 100 commits in a repository. Then `commits` lists
  the newest 100.
- `draft`: `{action: open}` or `{action: update, number}`. A draft is a pull request or a
  merge request that people can still change before it is ready. `draft` is absent after
  a session that did not pass, when the run wrote no text, and when the session
  escalated. The adapter then only builds the address of each listed commit.

The entry is the same for every forge, so one adapter opens a pull request and another a
merge request from it. The rules of the kind:

- The adapter builds the forge's address of `data.base.commit` and of each commit in
  `data.commits`, whether or not the session passed. It reports them in the `data` of its
  reply, as `base` and `commits` with `commit` and `commit_url`. It builds no address for
  the commits that `earlier_commits` counts.
- For an entry with `data.draft`, the adapter opens the draft, or edits the draft with
  that `number`. The `number` is a hint. When the forge has no draft with that number,
  the adapter finds the draft from `data.branch` in any state and goes on with it. It
  opens a draft if there is none.
- With `open`, the adapter first looks for a draft from `data.branch` in any state. So a
  draft that exists is not opened a second time. The adapter finds a draft by its head
  branch, `data.branch`, and the repository only, never by `data.base.branch`.
- With several drafts on the branch, the adapter takes the entry's `data.draft.number`
  when it is among them. Otherwise it takes the not-closed, not-merged draft with the
  highest number. With none, it takes the closed or merged one with the highest number,
  reported as kept.
- The adapter edits a draft only while it is `draft` or `open`. A draft that a person
  closed or merged stays as it is.
- The draft's description has one block between the lines `<!-- qory:start -->` and
  `<!-- qory:end -->`, and `text` is the content of that block.
- On an update, the adapter takes the first end marker line that has a start marker line
  before it. It takes the last start marker line before that end line. It replaces the text
  from the start line to the end marker with the new block, and keeps the rest of the
  description.
- If there is no such pair, the adapter adds a blank line and a new block at the end. A
  person may have removed a marker.
- The new block uses the line ending of the description: CRLF when its first line ending
  is CRLF, otherwise LF. This applies to the line endings that the adapter adds. They are
  the ones next to the two marker lines and the blank line before a block added at the end. The
  line endings inside `text` stay as given.
- The draft's title is `title`. On an update, the adapter sets the title to `title` only
  while the draft's title equals `previous.title`. A title that a person edited stays.
  If the forge adds a text before the title of a draft that is not ready, the adapter
  compares the title without it. The adapter reads the draft's state before it edits.
- The reply's `address` is the draft: `url`, and `ref`, its number as a string. `action`
  is `created` or `updated`, or `kept` for a draft that a person closed or merged. The
  reply's `data.draft` has `number`, `title` and `state`: `draft`, `open`, `closed` or
  `merged`. A reply for an entry with no `draft` has no `address` and no `action`.

On a run with no run code, `qory run` runs a submit program with the login of the forge's
command line that the person made. That run has no work item and no earlier change
request. The program opens a draft from the branch that qory made for the run and pushed,
or edits the draft that it finds there. The branch is in `QORY_SUBMIT_BRANCH`, and is
`qory/run-<run id>` unless that name is taken. It builds the address of each commit as in the rules above. The
program sends nothing to the control plane: only the adapter sends `changes/submitted`.
The [submit program contract](../../run/v1/submit-program/README.md) lists its inputs and
its result lines, and has the rule of the block in the same words.

A person can write a comment in the control plane on a work item taken in from an item
at the source. The comment reaches the source in four steps:

1. The control plane stores the comment with the item's other comments and writes a
   `comment` entry to the feed, with `comment_id`, `ref`, `author_name` and `body`.
   `comment_id` is `sc_` and 16 lower-case Crockford base32 characters.
2. The adapter posts the comment on the item as the integration's account at the
   source. The first line is `<author_name> wrote in Qory Apiary:`, such as
   `Dana Example wrote in Qory Apiary:`, then an empty line, then `body` unchanged.
   `author_name` is plain text, and the control plane sends it as it is, with no
   escaping. The adapter writes it as plain text: it escapes each character or sequence
   that the format of the source's comments would read as markup, as a mention, or as a
   link. For a source whose comments are Markdown, that includes an @ at the start of a
   word, a code fence, leading spaces, the start of an HTML comment, and a sequence that
   the source turns into a link, such as a reference to another item (`ABC-123`, `#123`)
   or a commit id. The escaped name, the other 22 characters of the first line
   (`" wrote in Qory Apiary:"`), the two line breaks after it, and the mark with any line
   break before it are at most 536 characters together. 536 is the room that `body`, at
   most 65000 characters, leaves of the 65536 characters in `definitions.schema.json`,
   `$defs.body`. For example, an escape of at most 3 characters for each character of the
   name, such as a backslash before a character and a zero-width space after it, keeps a
   name of 100 characters at most 300 characters. A numeric character reference for every
   character, such as `&#64;`, makes the name 500 characters or more and leaves too
   little room for the mark. The adapter puts a mark that contains the `comment_id` in
   the posted comment. Before it posts, it checks the item for a comment with that mark.
   So it posts at most one comment per `comment_id`, however often it reads an entry with
   that `comment_id`, and the comment of a post whose result is unknown, such as one that
   timed out, is not posted a second time. The form of the mark is the adapter's choice:
   the source must keep it, and where the source allows, people who read the comment do
   not see it.
3. The adapter sends the posted comment back with `POST /v1/source-items/comments`, with
   `comment_id` set (`source-item-comments.schema.json`). The control plane matches the
   sent comment to the comment it stored, and stores no second comment.
4. After a failure that trying again cannot fix, or when its retries end, the adapter
   sends `POST /v1/work-items/changes/failed` (`changes-failed.schema.json`) with
   `comment_id` and `reason`. The control plane marks the comment not posted and stores
   `reason`.

| `reason` | Meaning |
|---|---|
| `item_gone` | the source returned that the item does not exist |
| `not_permitted` | the source returned that the integration's account has no permission to comment on the item |
| `too_long` | the comment with its first line is longer than the source's limit for a comment |
| `unreachable` | each request to the source timed out, failed to connect or returned a busy status, until the adapter's retries ended |

The control plane writes comment entries only for a source whose `source` role has
`comments: true`. An adapter that does not set `comments: true` never receives a
`comment` entry: the source takes no comment from the control plane, so the adapter
cannot post comments there, and the comment stays in the control plane only. A source
with `items: false` has no items, and its `source` role has no `comments`.

## Fixtures

| Path | Contains | Validated against |
|---|---|---|
| `fixtures/*.json` | descriptions that are accepted: `github.json`, what `qory-github describe` printed at commit c8a3ea5, built with `go build -buildvcs=false`, so its `program_version` is `dev`, with `egress_hosts` added; `acme-tracker.json`, a description with the members every description must have, plus `egress_hosts` for its credential host; `acme-chat.json`, one that serves two domains; `unknown-role.json`, one with `acme_role`, a role the contract does not define, beside `credential`; `example-source-apply-bearer.json`, a source role with an apply item of the scheme `bearer` whose paths are templates with `{target.path}` and a path without it; `example-source-apply-basic.json`, one with an apply item of the scheme `basic` and its `username`; `example-source-apply-header.json`, one with an apply item of the scheme `header` and its `header`, on a DNS name and an IPv4 address; `example-source-guide.json`, a source role with a `guide`: a `setup` part with `before`, three `steps` and `after`, and a `connect` part with two `steps` alone; `example-source-targets.json`, a source role with `targets: true`; `example-source-change-kinds.json`, a source role with `targets: true` and `change_kinds` of `pull-request` and `reply`; `example-source-comments.json`, a source role with `comments: true` | `description.schema.json` |
| `fixtures/adapter/<name>/` | the requests and answers of the adapter, named by their schema; among them `changes/change-request-pull-request.json` and `changes/change-request-pull-request-another-host.json`, an entry of the kind `pull-request` that opens a draft on one host and one that updates a draft on another, `changes/change-request-pull-request-two-versions.json`, a first version with no `previous` and a second whose `previous` has the title and the address of the first, `changes/change-request-pull-request-earlier-commits.json`, a part with 100 listed commits and `earlier_commits` 40, `changes/change-request-reply.json` and `changes/change-request-bid-changes.json`, an entry of the kinds `reply` and `bid-changes`, the second with a number that has a fraction, `changes-submitted/pull-request-created.json` and the other replies there, one for each kind, and `changes-failed/change-request-*.json`, one for each reason of a change request and one for an entry whose kind the adapter did not list | `<name>.schema.json` |
| `fixtures/invalid/` | descriptions the schema refuses, named `description-<reason>`; among them `description-source-apply-host-wildcard.json`, an apply host with `*.` in front; `description-source-apply-header-without-header.json`, the scheme `header` without `header`; `description-source-apply-username-with-bearer.json`, a `username` with the scheme `bearer`; `description-source-apply-path-two-placeholders.json`, a path template with `{target.path}` twice; `description-source-apply-path-star-before-placeholder.json`, a path template with `*` before `{target.path}`, outside Forager's path grammar; `description-source-apply-path-star-after-placeholder.json`, a path template with `*` right after `{target.path}`, in the same segment; `description-source-guide-steps-11.json` and `description-source-guide-steps-empty.json`, a guide part with 11 steps and with none; `description-source-guide-no-steps.json`, a guide part without `steps`; `description-source-guide-line-feed.json`, a guide line with a line feed; `description-source-guide-301-characters.json`, a guide line of 301 characters; `description-source-guide-undefined-member.json`, a guide part with a member the schema does not define; `description-source-comments-false.json`, `comments: false`; `description-source-change-kinds-empty.json`, `change_kinds` with no kind; `description-source-change-kinds-string.json`, `change_kinds` as the string `"pull-request"`; `description-source-change-kinds-without-targets.json`, `change_kinds` with no `targets`; `description-source-change-kinds-duplicate.json`, `description-source-change-kinds-upper-case.json`, `description-source-change-kinds-65-characters.json` and `description-source-change-kinds-17-kinds.json`, a list with a kind twice, with a kind in upper case, with a kind of 65 characters and with 17 kinds; and `description-source-items-false-with-comments.json`, `comments` beside `items: false` | `description.schema.json`, expecting a failure |

Every fixture is synthetic. No host name of anyone's infrastructure and no real secret.

## Sources

- [JSON Schema 2020-12](https://json-schema.org/draft/2020-12/json-schema-validation),
  §9.4 for `writeOnly`.
- Forager's contract, [§Credentials](https://github.com/qoryai/forager/tree/v0.7.0/contracts/forager/v1#credentials),
  for the adapter definition a credential role expands to.
