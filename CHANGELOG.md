# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

### Upgrading

- Read the `text` of a `change_request` entry as Markdown with no rule on its lines. The
  entry refused a line that starts with `<!-- qory:start -->` or `<!-- qory:end -->`.
  A `reply` whose `text` has such a line passes `changes.schema.json`. A kind that needs
  marker lines defines the rule: the artifact type of the kind and the adapter that
  applies it. The submit program contract keeps its marker lines.
- Take the draft names of the Go package from the submit program, not from the change
  request reply. `DraftOpen`, `DraftStateDraft`, `DraftStateOpen`, `DraftStateClosed`,
  `DraftStateMerged`, `DraftOpened`, `DraftUpdated`, `DraftKept` and `SubmittedDraft`
  are in `submit_program.go`, with the same values. The submit program uses all of them,
  and `SubmitResult.Draft` is a `*SubmittedDraft`. No Go name of the change request
  reply is a draft name.
- Take the rules of the `pull-request` kind from the artifact type of the kind and from
  the adapter that applies it, not from the README of the integration contract. The
  README had a section on the kind. It covered the members of `data`, how an adapter
  finds a draft by its branch, the block between the marker lines, the title rule and
  the members of the reply. The section is gone. The README keeps the rules of every
  change request, and the submit program section. No release had the section.
- Rename `$defs/draft_title` in `definitions.schema.json` to `$defs/change_title`. It is
  the title of a change request of any kind. It keeps the rule that `draft_title` had: 1
  to 256 characters on one line, the character rule of a prose line, and at least one
  character that is not a space. A schema with a `$ref` to `draft_title` no longer
  compiles, so change each such `$ref`. No release had `draft_title`. The key
  `draft_title` of the submit program result stays, and has the rule of `change_title`.
- Change each reader and writer of the run documents from `submit` to `deliver`. No
  release had `submit`. `deliver` is a list of `{type, send}` entries, both members
  required, in place of the one object `{type, send}`. It has at most 16 entries, and at
  least 1 when it is present. `type` has the grammar of `submit.type`, and `send` has
  the values of `submit.send`. A run without `deliver` delivers nothing. `deliver` is
  present only on a run with `workflow` and with `access` `write`, and never on a run of
  the step `prebuild`. No two entries have the same `type`. The schema cannot check that,
  so a reader calls `Documents.Check` after it decodes the documents. A reader ignores
  a `submit` member, because the schema does not define it. The closed copy that
  `CompileRunClosed` builds for a writer's test refuses it, and a reader refuses a
  `must_understand` that lists `/submit`. In Go, `Submit` and `Documents.Submit` are
  `Deliver` and `Documents.Deliver`, a `[]Deliver` with `Type` and `Send`, and
  `MaxDeliver` is 16. The submit program contract and its Go names stay.
- Read `read_targets` from the run documents when a run reads places beside its `target`.
  `read_targets` is a list of `{collection, address}` targets, in the form of `target`,
  with 0 to 20 entries, and `[]` is valid. The run only reads a place in `read_targets`:
  it changes, pushes and delivers nothing to it. `target` stays the one place where a run
  changes, and `access` applies to `target` only. `read_targets` is present with or
  without `target`, and never on a run of the step `prebuild`. No two entries have the
  same `collection`, and no entry has the `collection` of `target`. The schema cannot
  check that, so a reader calls `Documents.Check` after it decodes the documents. An entry
  has no version: `target_version` applies to `target` only. In Go,
  `Documents.ReadTargets` is a `[]Target`, and `MaxReadTargets` is 20. The folder
  `fixtures/check` of the run contract has run documents that the schema accepts and
  `Documents.Check` refuses.
- Write and read `destinations` in the explore inputs. It is required, and it has 1 to 250
  entries. Each entry is a destination: `id`, the control plane's id of a place, and
  `target`, the place as the adapter wrote it. An `id` is `tg_` and 16 characters, each a
  digit or a lower-case letter other than `i`, `l`, `o` and `u`. `target` of the inputs
  stays optional. No two entries are equal, and the control plane drops the oldest
  comments, never a destination, to keep the inputs within their bound. Before this
  release the inputs had no list of destinations, and a writer without `destinations`
  fails the schema. `$defs/destination` in `definitions.schema.json` defines an entry. A
  document that an adapter writes keeps `target` with no `id`. In Go, `Destination` has
  `ID` and `Target`.
- Accept 1 to 250 `resources` in a `run-credential` call with `purpose` `work` and
  `access` `read`. Before this release a work call had exactly 1, and an adapter refused
  more with `400`. A work call with `access` `write` still has exactly 1, and a `harness`
  call has 1 to 64. All resources of a call are collections of one connection, the
  `connection_id` of the call. The answer is one credential. In an `apply` item of the
  answer, `paths` lists each template filled with the path of each resource. The answer
  has at most 16 `apply` items of 1 to 32 paths, 512 paths together, as the Forager
  contract limits it. An adapter covers N resources only when N × p + c ≤ 512. Here p is
  the number of path entries that one resource adds across all `apply` items. c is the
  number of paths that belong to no single resource, such as a template without
  `{target.path}`. It covers fewer when paths of different items cannot share one list of
  32. It refuses a call over its limit with `403 resource_not_covered`.
- Fill the source `apply` templates of a run with no target, such as Explore, for each
  resource of the `run-credential` call. The control plane fills the templates of an
  item one after another, each for every resource. It splits the filled paths into items
  of at most 32 paths. Before that, it drops a covered path among paths that share host,
  scheme, username and header. A path of an item is dropped only when, on every host of
  the item, a path of the group of that host covers it. If it is covered on some hosts
  of the item and not on others, the adapter refuses the call with
  `403 resource_not_covered`. Of two equal paths, the control plane keeps the first: the
  one of the earlier item in the description, then the earlier path in one item. Two
  items on one host that differ in scheme, username or header may not have equal,
  covered or overlapping paths. The adapter refuses such a call with
  `403 resource_not_covered`. The read access of the run covers every destination of the
  workspace from the start and does not grow during the session.
- The `address_path` definition of `definitions.schema.json` refuses `{` and `}` in the
  `address.path` of every target, and in `run.target` of `diagnostics.schema.json`.
  Before this release it accepted them. The control plane fills a path template with
  that path, and Forager's `credential_paths` allows `{` and `}` only as the whole
  segment `{}`.
- The control plane refuses a target path with a segment `.`, a backslash or a `%`
  before it fills a template. It refuses a first `/`, a final `/` and a segment that
  starts with `;` or `.;` too. Before this release it refused only `..`, `//`, `*`, `?`,
  `#` and a space character. Forager's `UnclearPath` refuses a filled path with a `.`
  segment, a backslash, `//`, or `%2f`, `%5c`, `%2e` or `%25` in any case. It also
  refuses a segment that is not empty when its part before the first `;` or `%3b`, in
  any case, is empty. It refuses the segment too when that part is a single dot or two
  dots. The refusals of `..` and `%` cover `..;x` and `%3b`.
- Handle the refusal `409 connection_held` of a grant report, and the reason
  `connection_held` of a connect return. The control plane refuses a report with it when
  the connection has a grant that has not ended and the report has another `grant_ref`. A
  report with the same `grant_ref` replaces that grant. The text for the person is
  "Disconnect the account that is connected first, then connect again. A connection has
  one account." Before this release a connection had a list of grants, one for each
  external account. A connection has at most one grant that has not ended. The adapter
  treats `connection_held` as final, as `grant_taken`. The control plane checks a report
  for `pass_unknown`, then `grant_taken`, then `connection_held`. `grant_taken` stays for
  a grant that another connection has.
- Read `full_suite` from `type_settings`, in the run documents and in the answer to the
  next-session call. `full_suite` at the top level is no member of either, so a reader
  ignores it, and the closed copy that `CompileRunClosed` builds for a writer's test
  refuses it. In Go, `Documents.FullSuite` and
  `NextSessionAnswer.FullSuite` are `Documents.TypeSettings` and
  `NextSessionAnswer.TypeSettings`, a `json.RawMessage`. `NextSessionAnswer` writes
  `type_settings` when it is set. An answer for a session no longer requires
  `full_suite`, and `type_settings` is optional in it: a session in a workspace that is
  not a software factory has no `full_suite`.
- Change each reader and writer of the `submission` entry of the change feed to the
  `change_request` entry. No release had the members that change here.
  - The `submission` entry kind is gone. So are `submissions: true` in the `source` role
    of a description and the reasons `branch_gone` and `forge_refused`. So is every member
    that was specific to a pull request.
  - `submission_id` is `change_request_id`, with the same `av_` pattern.
  - One entry has one `target`, a `{collection, address}`. A submission had `targets`, 1
    to 20, each with `system` and `path`.
  - `text.title` is `title`, and `text.description` is `text`. `previous_title` is
    `previous.title`, and `previous.address` is added.
  - A pull request has the members `target`, `title`, `text`, `commits`, `head`, `base`,
    `branch`, `base_branch`, `steps`, `draft` and `earlier_commits` in `data`. The
    artifact type of the entry's `change_kind` defines them. This contract does not list
    them.
  - `submissions: true` is `change_kinds`, a list of 1 to 16 kinds.
  - The reply `changes-submitted` has `change_request_id` and `target`. It has `address`
    (`url`, `ref`) and `action` (`created`, `updated` or `kept`) together, or both left
    out. It has `data`. Before, it had `targets` with `base`, `commits` and `draft`, and a
    draft's `opened` is `created`.
  - The failure `changes-failed` has `change_request_id` and `target`, where it had
    `submission_id`, `system` and `path`. Its reasons are `not_permitted`, `unreachable`,
    `target_gone`, `too_long` and `refused`.
  - In Go, `ChangeSubmission`, `SubmissionText`, `SubmissionTarget`, `SubmissionBase`,
    `SubmissionCommit`, `DraftRequest`, `DraftUpdate`, `SubmittedTarget` and
    `CommitAddress` are removed.
  - These members are removed too: `SubmissionID`, `PreviousTitle`, `Text` (a pointer)
    and `Targets` of `Change`; `SubmissionID`, `System` and `Path` of `ChangesFailed`;
    `SubmissionID` and `Targets` of `ChangesSubmitted`.
  - `ChangeRequest`, `ChangeAddress`, `ChangePrevious`, `ChangeCreated`, `ChangeUpdated`
    and `ChangeKept` are added. `Change` has `ChangeRequestID`, `ChangeKind`, `Target`,
    `Title`, `Text` (a string), `Previous` and `Data`. `ChangesFailed` has
    `ChangeRequestID` and `Target`. `ChangesSubmitted` has `ChangeRequestID`, `Target`,
    `Action`, `Address` and `Data`.
  - `DraftOpen` stays as the value of `QORY_SUBMIT_DRAFT`.
- Stop relying on this contract for the numbers inside the `data` of a change request.
  The integer rule (a number has no fraction or exponent) does not apply inside `data`,
  so a bid of `1.25` is valid. The artifact type of the kind checks the numbers.
  JSON Schema counts `34.0` as an integer, so a schema with `type: integer` accepts
  `34.0`. The rule that the control plane enforced for `draft.number` and
  `earlier_commits` no longer reaches them. No object inside `data` repeats a member, at
  any depth, and the control plane refuses a repeat, 400 `invalid_request`.
- Change each reader and writer of the run documents from step `explore` to step
  `workflow`, and from `work_item` to `subject`. `step` was a pattern. It is the closed
  enum `workflow` or `prebuild`, so a document with step `explore` fails
  `documents.schema.json`. A step `workflow` runs the workflow named in `workflow`. A step
  `workflow` requires `workflow`. `workflow` still requires `harness_hidden`, and
  `harness` or `target_version`. `run_settings`, `time_limit_minutes`, `shared_run` and
  `type_settings` apply to every step `workflow`.
  `subject` is `{kind, id, url, title}`, and a document with `work_item` and no
  `subject` fails the schema. `kind` is a snake_case string of at most 64 characters,
  with `work_item` as its first value. `id` is 1 to 200 characters on one line, and
  starts with `wi_` when `kind` is `work_item`. `url` and `title` are optional. `title`
  narrows from 1 to 1024 characters, line breaks allowed, to 1 to 256 characters on one
  line. The optional `trigger` is a snake_case string of at most 64 characters. Its
  first value is `take_in`: the control plane took in the subject. In Go,
  `contracts.WorkItem` is removed. `contracts.Subject` has `Kind`, `ID`, and the
  optional `URL` and `Title`. `contracts.Documents` has `Subject` and `Trigger` in place
  of `WorkItem`. `explore/v1/inputs.schema.json` keeps its `work_item`, and
  `inputs.work_item.id` is `subject.id` when `subject.kind` is `work_item`.
- Update a reader of the run documents to this `documents.schema.json` before the
  control plane writes `deliver`, `write-no-push` in `access`, or `write-no-push` in the
  `access` of a workflow in `harness`, to the run documents: a reader that checks the
  run documents against an earlier copy refuses each of them.
- Rename three members, which this module had after 0.3.0 and before this release, and
  drop one. In the run documents, a workflow of a `harness` section has no `time_limit`,
  and has `plan` and `tier`; a workflow with `time_limit` fails `documents.schema.json`.
  `HarnessWorkflow` no longer has `TimeLimit`, and has `Plan` and `Tier`.
  `time_limit_minutes` stays the time limit of a session. In the machines contract,
  `key` of `make` and of each machine of the `list` answer is `dispatch_id`; a request
  or answer with `key` fails its schema. `MachinesMake.Key` and `Machine.Key` are
  `DispatchID`. In the diagnostics, `run.machine.key` is `run.machine.type`; a document
  with `key` passes the schema, which accepts a member it does not define, so a reader
  that reads `key` finds none.
- Change each reader and writer of a target to `{collection, address}`. `$defs/target` in
  `definitions.schema.json` has `collection` and `address`. `contracts.Target` has
  `Collection` and `Address`, a `*TargetAddress` with `System` and `Path`. A target with
  `system` and `path` and no `collection` fails `source-collections.schema.json`,
  `source-items.schema.json`, `explore/v1/inputs.schema.json`, `documents.schema.json`
  and the `run.target` of `diagnostics.schema.json`. A `change_request` entry, its reply
  and its failure report use this `target`. `$defs/system` is the host name that
  `$defs/target/properties/system` was.
- Write the `run.target` of a diagnostics document by the new rules. `address.system` is a
  lower-case host name, `^[a-z0-9.-]{1,253}$`. The schema refuses an upper-case host, a
  port and a non-ASCII host. `address.path` has 1 to 200 characters and no control character,
  U+0000 to U+001F or U+007F. The schema refuses a path of 201 to 256 characters, which
  the earlier limit of 256 allowed.
- `contracts.Compile` asserts the `format` keyword, so `conformance.Description`,
  `integration-conformance` and a check against an integration contract schema refuse
  values they accepted before: a malformed URI or date in an integration document, such
  as the `created_at` `2026-02-30T00:00:00Z`, and a `pattern` in a description's
  `settings` that is not a valid regular expression.
- `conformance.Description` and `integration-conformance` refuse a description that goes
  past a limit of `description.schema.json`, and `release.yml` publishes nothing for one:
  more than 262144 bytes, bytes that are not UTF-8, a `\u` escape of a lone surrogate, an
  object that repeats a member, a string or member name of more than 16384 bytes, more
  than 4096 JSON values, more than 64 levels of nesting, or an array or object with more
  than 1024 entries.
- Add each host in the `credential` role's `hosts` to `egress_hosts`.
  `conformance.Description` refuses a credential host without an equal entry there, so a
  description of 0.3.0 with a `credential` role and no `egress_hosts` fails the check.
  `conformance.Description` also refuses a credential host with `*.` in front, and a
  credential host that an `egress_hosts` entry cannot be, such as `localhost` or a single
  label.
- The module requires Go 1.27.4 or later (`go 1.27.4` in `go.mod`), because Forager does.
  An integration that requires this module builds with Go 1.27.4 or later. `go.yml` runs
  its job in `ghcr.io/qoryai/ci-images:golang-1.27.4`, by digest. That image has Go
  1.27.4, and `go.yml` sets `GOTOOLCHAIN` to `go1.27.4+auto`, so a job downloads a newer
  Go when the `go.mod` of the calling repository requires one.
- Run documents have no top-level `stack` and no `stack_reference`, members that this
  module had after 0.3.0 and before this release. A reader takes the harness from the
  harness file of the target at `target_version`, or from `harness`, as Added describes.
  A run with `workflow` has `harness`, `target_version` or both. In Go, `Documents.Stack`
  and `Documents.StackReference` are removed, and `Documents.TargetVersion` and
  `Documents.Harness` are added.
- If an adapter follows the adapter contract as this module had it after 0.3.0, change
  the adapter in three ways. Accept change feed entries of the kind `grant`, which the
  feed has. After the adapter mints a linked connect return, report the grant with `POST
  /v1/grants`, read `GET /v1/grants` when the adapter starts, follow the `grant`
  entries, and match the adapter's state to that copy. Accept a connect pass for a live
  connection, which connects one external account to a connection, or replaces the grant
  of that account.
- If an adapter follows the adapter contract as this module had it after 0.3.0, add two
  members, because the schemas require them. In each grant report, `POST /v1/grants`,
  send `account`, `{ref, title}`: `ref` is the external account's id at the source, and
  `title` is its display name, one line. In each collection of
  `POST /v1/source-collections`, send `grant_ref`, the grant under the same connection
  that covers the collection. List a collection only after the control plane accepts the
  report of its grant: the control plane refuses a part that contains a `grant_ref` its
  copy of grants does not have under that connection, `400 invalid_request`. In Go,
  `Grant.Account`, `Account` and `Collection.GrantRef` are added.
- If an adapter ends one grant of a connection, as when one external account withdraws
  the access it gave at the source, send `POST /v1/grants/end` with `connection_id` and
  `grant_ref`, instead of `POST /v1/connections/disconnect`, which ends the whole
  connection. The control plane ends that grant and the collections whose `grant_ref` in
  the latest listing that contains them is that grant's, and the connection stays. It
  refuses a `grant_ref` its copy of grants does not have under that connection,
  `409 grant_unknown`, which the adapter takes as the grant ended. In Go, `GrantEnd` is
  added.
- If a run or a reader follows the explore contract as this module had it after 0.3.0,
  change it, because `contracts/explore/v1` stays at revision 1 with a new shape.
  `result.schema.json` is removed, because the run pushes its outputs, the proposal and
  the questions, as artifacts with `qory artifact push`, and the next run reads their
  newest versions with `qory artifact pull`. An artifact is a named output of a work
  item, such as the proposal. The control plane stores each version of it, and each
  version is one or more files. In `inputs.schema.json`, `answers` and `run.of` are
  removed; members' answers are in the newest version of the `questions` artifact.
  `run.attempt` has no maximum. Without `ticket`, `comments` must be empty and
  `comments_omitted` must be 0.
- If an adapter follows the adapter contract as this module had it after 0.3.0, change it
  to post the comments that a person writes in the control plane on a work item taken in
  from an item at the source. `changes.schema.json` has the entry kind `comment`, with
  `comment_id`, `ref`, `author_name` and `body`. Set `comments: true` on the `source` role
  to receive `comment` entries; an adapter that does not set `comments: true` never
  receives a `comment` entry. Without `comments: true`, the source takes no comment from
  the control plane, so the adapter cannot post comments there, and the comment stays in
  the control plane only. An adapter that sets `comments: true` and checks the feed
  against the earlier `changes.schema.json` fails on the first `comment` entry and stops
  advancing its cursor. Post each `comment` entry on its item as the integration's
  account: the first line `<author_name> wrote in Qory Apiary:`, then an empty line, then
  `body` unchanged. The control plane sends `author_name` as it is, with no escaping.
  Write it in that line as plain text: escape each character or sequence that the
  format of the source's comments would read as markup, as a mention, or as a link,
  such as a code fence or a reference to another item for a source whose comments
  are Markdown. Keep the escaped name, the other 22 characters of the first line,
  the two line breaks after it, and the mark with any line break before it at most
  536 characters together: the room that `body`, at most 65000 characters, leaves of
  the 65536 characters of a comment. An escape of at most 3 characters for each
  character of the name, such as a backslash before a character and a zero-width
  space after it, keeps a name of 100 characters at most 300 characters. A numeric
  character reference for every character makes the name 500 characters or more
  and leaves too little room for the mark. Put a mark that contains the `comment_id`
  in the posted comment, in a form that the source keeps and that, where the source
  allows, people who read the comment do not see. Before you post, check the item
  for a comment with that mark, so the comment of a post whose result is unknown,
  such as one that timed out, is not posted a second time. Send the posted comment
  back with `POST /v1/source-items/comments`, and set `comment_id` on that
  comment in every request that contains it. After a failure that trying again cannot
  fix, or when your retries end, send `POST /v1/work-items/changes/failed` with
  `comment_id` and a `reason`: `item_gone`, `not_permitted`, `too_long` or `unreachable`.
  A feed answer ends before its body passes 4194304 bytes (4 MiB), and has at least one
  entry whenever the feed has an entry of a kind that the revision in the request's
  `X-Qory-Contract-Version` defines after the cursor; read the next entries from its
  `cursor`. In Go, `ChangeComment`, `Change.CommentID`, `Change.AuthorName`,
  `Change.Body`, `Comment.CommentID` and `ChangesFailed` are added.
- If a program builds or reads the `bind` of the `harness` of the run documents in Go,
  use `BindValue` for each role. `HarnessSection.Bind` is a
  `map[string]map[string]BindValue`; after 0.3.0 and before this release it was a
  `map[string]map[string]string`. Set `BindValue.Name` for a role that is not a group,
  and `BindValue.Names` for a group role, which a list of agents fills.
- Update a reader of the run documents or of the next-session answer to this
  `documents.schema.json` and `next-session-answer.schema.json` before the control plane
  sends a `time_limit_minutes` below 15. `time_limit_minutes` is the time limit of a
  session, from 1 to 1440 minutes. Until this release the limit was 15 to 1440, and a
  reader built from an older copy of the schemas refuses 1 to 14.
- Change each reader of the documents of the integration, run, provisioner, machines and
  explore contracts. A reader ignores a member that its schema does not define, in any
  object. Before, these schemas and the objects inside them refused such a member. The
  objects that the Forager contract defines are the exception. They are the `auth` and the
  `format` of an API, and the `credential` of a run credential answer with its `apply`
  items.
  - The optional `must_understand` of a document lists the members that the reader must
    understand. Each is a JSON Pointer (RFC 6901) counted from the document, such as
    `/run_settings/runtime`. Before, it was a list of top-level member names.
  - A reader refuses a document whose `must_understand` lists a pointer to a member that
    it does not define. A reader still refuses a bad value of a member that it defines.
  - The schema accepts a pointer to a member that the schema defines. It accepts any
    pointer below `inputs`, below `type_settings`, below `harness.harness.extensions` and
    below the description's `settings`.
  - The description's `settings` stays the integration's settings schema. The `auth` and
    `format` of an API are defined by the Forager contract, and they refuse an undefined
    member.
  - A reader refuses a pointer that has a line break, a `~` that is not `~0` or `~1`, or
    an empty name of a map key. `minProperties` counts members, defined or not. An object
    with only an undefined member, such as `{"zz": 1}`, reads as empty in two places.
    `previous` of a change then has no title and no address. A harness selection,
    `exclude` or `only`, then lists no part: `exclude` leaves nothing out, and `only`
    takes nothing. Go writes such a selection back as `{}`, which the schema refuses.
  - An answer with `next` `end` and a member of the session shape fails
    `next-session-answer.schema.json`, as before.
  - Each entry of the change feed has an optional `must_understand`. The answer has none.
    An adapter ignores an unknown member of the answer and of an entry of a defined kind.
    The detail of a refused entry has at most 1000 characters, and a pointer of more than
    300 bytes is cut.
  - An adapter skips and acknowledges an entry in two cases. The `kind` of the entry is
    one that its revision does not define, or its `must_understand` lists a pointer that
    the adapter does not define.
  - A `change_request` entry is the exception for a `must_understand` that the adapter
    cannot meet. The adapter reports it failed with `reason` `refused`
    (`changes-failed.schema.json`), and the `detail` starts with the fix.
  - The artifact type of the `change_kind` defines the members below the `data` of a
    `change_request` entry. The adapter checks those pointers.
  - `changes.schema.json` accepts any snake_case `kind` and checks the members of the
    seven kinds it defines.
  - The shared schemas `target`, `change_address`, `person` and `listing` in
    `definitions.schema.json` ignore an unknown member too, in every schema that refers
    to them, among them the explore inputs, `source-collections.schema.json` and
    `source-items.schema.json`.
  - Every other schema of the integration contract ignores an unknown member too:
    `adapter-secrets`, `adapter-secrets-answer`, `adapter-secrets-plaintext`,
    `connect-pass`, `connect-return`, `connections-disconnect`, `grant`, `grant-end`,
    `grants`, `read`, `read-answer`, `read-public`, `refusal`, `resolve`,
    `resolve-answer`, `run-credential`, `run-credential-answer`, `setup-pass`,
    `source-collections`, `source-item-comments`, `source-items` and
    `sources-disconnect`. Each has an optional `must_understand` at its root.
    A writer sends no key or secret in a member that the contract does not define. The schema refuses three members by name: `values` in the `sealed` of
    `adapter-secrets` and `adapter-secrets-answer`, `token` in an `apply` item, and
    `placeholders` in a credential. In this release `adapter-secrets` accepts and ignores
    a root member `values`, `secrets` or `plaintext`, which it refused before. It still
    refuses `values` inside `sealed`.
  - The replies `changes-submitted.schema.json` and `changes-failed.schema.json`, the
    body of `adapter.schema.json` and its `sync`, the request of the run contract, the
    seven schemas of the provisioner contract, the eleven schemas of the machines contract
    and the seven objects of `explore/v1/inputs.schema.json` ignore an unknown member. Each root of these, except the explore inputs, has an optional
    `must_understand`. The explore inputs use the list of the run documents.
  - The outcome of the run contract has the members `state`, `reason` and `exit_code` of
    Forager's `dev.qory.run.exited`. A reader of the data of that event ignores a member that
    Forager's schema does not define, unless the data's `must_understand` lists it. The
    outcome schema accepts a member that it does not define, and the control plane reads
    `state`, `reason` and `exit_code` from the event.
  - A writer marks a member that the reader must read by listing its pointer in
    `must_understand`. The reader refuses a document that lists a pointer it does not
    define. A machines answer that lists such a pointer is a failure of kind `defect`. The
    control plane refuses a release with such a pointer, 400 `invalid_request`, as it
    refuses any body that breaks its schema. A batch's `dev.qory.run.exited` can have an
    outcome whose `must_understand` lists a pointer that the outcome schema does not
    define. The control plane answers that batch with `400 invalid_request`, as Forager
    defines for a body its contract refuses. This contract defines no body for that
    refusal. The contract describes no further handling for the provisioner claim answer
    and the provisioner refusal.
  - A writer's test validates against a closed copy of the schema, so a misspelled
    member fails it. The copy covers the schema and each schema of this repository that
    it refers to.
  - `conformance.Description` and `integration-conformance` validate a description
    against the closed copy.
  - `fixtures/closed` contains the documents that the schema accepts and the closed copy
    refuses: 142 documents in all contracts, of which 95 moved there from
    `fixtures/invalid`. These fixtures moved there because a retired or secret member
    was a refusal until this release: `claim-answer-run-id`, `release-with-retry`,
    `grant-end-with-pass-nonce`, `read-public-with-grant-ref`, `refusal-supported-member`,
    `sources-disconnect-with-reason`, the outcome `end_code`, `action` and `message`, the
    request `verdict` and `attempt`, and the machines `*-version.json` answers. The tests
    of the adapter `submit_module` and of the reply `targets` are inline in `run_test.go`
    and `change_request_limits_test.go`.
    They expect the reader to ignore the member. A reader ignores each of these members.
    Five removed fixtures came back:
    `documents-undefined-member.json`, `documents-subject-undefined-member.json`,
    `next-session-answer-undefined-member.json`,
    `next-session-answer-model-credential.json` and
    `description-source-undefined-member.json`.
  - `changes-unknown-kind.json` moved to `fixtures/adapter/changes/unknown-kind.json`.

### Added

- Read and write the findings of a work item with `findings.schema.json`,
  `findings-post.schema.json`, `findings-post-answer.schema.json` and
  `findings-refusal.schema.json` of the run contract. A finding is one problem that a
  reviewer, a check, a criterion, a scenario or a hook raised about a target. Qory Apiary
  mints its id, `f` and a number. A finding has a target, an owning stage, a source, a
  severity, a place, a point, a problem, a fix, a proof, a state (`open`, `fixed`,
  `dismissed` or `accepted`), `disputed` and its events. An event has the `round`, the
  `stage_round` of the work item, and the `stage`. A run that repeats a finding that a
  person closed adds `repeated_after_person_close`, and the finding stays closed. The post
  body is a batch of events (`raise`, `repeat`, `reply_fixed`, `reply_disputed`,
  `close_fixed` and `close_dismissed`) and `bases`, which Qory Apiary stores once for each
  take in (one start of the work item in Qory Apiary) and each target. The answer has the
  id of each raise and the open blocking count of each stage. Qory Apiary refuses a verdict
  `passed` with `open_blocking_findings` (409, with `stage` and up to 50 `findings`) while
  the stage owns an open blocking finding. In Go: `Findings`, `Finding`, `FindingEvent`,
  `FindingsPost`, `FindingsPostAnswer` and `FindingsRefusal`, with `Check` methods.
  `findings`, `findings-post` and `findings-post-answer` have a closed copy.
- `$defs/harness_workflow` of `documents.schema.json` has `review_group` and
  `runs_locked_checks`, both optional, as a workflow has them in `qory-stack.yaml`.
  `review_group` is the name of the group role whose members' reports a review session
  of the workflow needs: one path segment with no slash, backslash, `@` or space, and no
  leading dot. `runs_locked_checks` is a boolean: `true` marks a Verify workflow, whose
  locked checks qory runs before the agent of a session starts, and `false` or left out
  means qory runs none. Without the two members, a run that Qory Apiary starts loses
  both keys of a workflow, because the schema and the Go type define neither. In Go,
  `HarnessWorkflow` has `ReviewGroup` and `RunsLockedChecks`. A false `RunsLockedChecks`
  is left out of the document.
- `description.schema.json` has `links`, optional: 1 to 16 entries `{system, template}`, one for
  each system. A `template` is at most 2048 characters. It starts with `https://`, then
  the host, then a path. The host is the `system` of the entry, with no port. The path
  has no `?` or `#`, and only it has placeholders, `{collection_path}` and `{path}`. The
  section Links of the integration README describes how a reader fills a template.
  `contracts.CheckLinks` refuses a system listed twice and a host that is not the
  `system`. Before this release a description had no member for a link.
- `contracts.CompileClosed`, `contracts.CompileRunClosed`,
  `contracts.CompileProvisionerClosed`, `contracts.CompileMachinesClosed` and
  `contracts.CompileExploreClosed` compile the closed copy of a schema, and return an
  error for a schema without one. Fifty-three schemas have a closed copy:
  - 27 of the integration contract, all but `definitions.schema.json`;
  - seven of the run contract: `documents`, `findings`, `findings-post`,
    `findings-post-answer`, `next-session-answer`, `next-session-request` and
    `outcome`;
  - the seven of the provisioner contract and the eleven of the machines contract;
  - `inputs` of the explore contract.

  `contracts.CloseSchema` makes the copy of one file.
- `contracts.CheckMustUnderstand` returns an error for a document whose `must_understand`
  lists a pointer that a Go type does not define. It also returns an error for a repeated
  pointer, for null, for a string that is no JSON Pointer, for a pointer with a line
  break, with a `~` that is not `~0` or `~1` or with an empty map key, for more than
  `contracts.MustUnderstandPointers` (32) pointers, and for a pointer of more than
  `contracts.MustUnderstandPointerLength` (256) characters.
  `contracts.CheckChangeMustUnderstand` does the same for a feed entry, by its `kind`.
- `contracts.Documents`, `contracts.NextSessionAnswer`, `contracts.NextSessionRequest`,
  `contracts.RunOutcome`, `contracts.Adapter`, `contracts.ChangesSubmitted`,
  `contracts.ChangesFailed`, `contracts.Change`, the provisioner types `Claim`,
  `ClaimAnswer`, `Dispatched`, `Release`, `MachineLog` and `MachineEnd`, and the eleven
  `contracts.Machines*` types have `MustUnderstand`. `contracts.Changes` has none. The
  rule is in `contracts/run/v1/README.md` and `contracts/integration/v1/README.md`, with a
  note that an unsigned 401 or 415 is not one of the nine next-session refusal codes.
- An API of `model_provider` can set `oauth_device`, next to `oauth_refresh`, in
  `description.schema.json`: the four URLs with which the control plane signs a person
  in with the OAuth device code flow. `usercode_url` is where the control plane sends
  the request for a user code, `poll_url` is where it polls until the person approves,
  `verification_url` is the page where the person enters the code, and `redirect_uri`
  is the redirect URI of the code exchange. Each is an https URL, as for `url` of
  `oauth_refresh`, and all four are required. `oauth_device` is allowed only with
  `oauth_refresh`, and it has no client id: the sign-in uses the `client_id` of
  `oauth_refresh`, and sends the code exchange to the `url` of `oauth_refresh`.
  `contracts.CheckModelProvider` refuses an `oauth_device` URL whose host or port the
  control plane cannot read. `contracts.CheckDescription` refuses an `oauth_device`
  without `oauth_refresh`, and an `oauth_device` URL that is not an https address, with
  a message that starts with the fix. A description without `oauth_device` is unchanged.
  `descriptions/openai-subscription.json` sets it for OpenAI's device code sign-in, and
  its `description` now starts with "Runs use your ChatGPT subscription through a
  sign-in that Qory Apiary makes with OpenAI's device code."
- The change feed has the entry kind `change_request` (`changes.schema.json`). The
  control plane writes one for each target of each change request version that a run
  stores. It writes it in the feed of the adapter of the target's collection, and the
  adapter applies the change there. The entry is the same for every type of workspace and
  every kind of change. Examples are a reply to a customer, a bid change for a campaign,
  and a pull request for a repository. It has these members:
  - `change_request_id`, `change_kind` and `target`.
  - `ref`: the ref of the item that the work item was taken from. It is absent when the
    work item came from no item.
  - `title`: 1 to 256 characters on one line.
  - `text`: at most 20000 characters, possibly empty, in Markdown. The envelope has no
    rule on its lines.
  - `previous`: absent on the first version. Otherwise it has the `title` and the
    `address` of the previous version.
  - `data`: the part.

  This contract lists no kind and no member of `data`. The artifact type whose name is
  `change_kind`, in the process of the workspace's type, defines them. The control plane
  validates the `data` of an entry against it. `data` has at most 786432 bytes
  serialized, and the control plane enforces that limit. The schema limits each value
  inside `data`. An array has at most 1024 items, an object at most 256 members, and a
  string at most 20000 characters. A member name has 1 to 128 characters. No string or
  name has U+0000. An adapter that updates the item at `previous.address` changes its title
  only while that title equals `previous.title`. It updates what is at `previous.address` when there is one. It applies the entries with one
  `target.collection` in cursor order. `change_kind` has the pattern of the `type` of an
  entry of `deliver` of the run documents: lower-case letters and digits in groups joined by single hyphens, 1
  to 64 characters. In Go, `ChangeRequest`, `ChangeAddress` and `ChangePrevious` are
  added.
- The `source` role may contain `change_kinds`: the kinds of change request that the
  adapter applies, 1 to 16, each once, such as `pull-request`. The control plane writes a
  `change_request` entry only for an adapter whose `change_kinds` lists the change's kind.
  This applies whatever source the work item came from, and also when the work item came
  from no item at a source. An adapter without `change_kinds`, or without that kind in
  it, never receives the entry. The control plane keeps the change in the control plane
  only. A source with `change_kinds` also has `targets: true`, because the run's target
  collection is one that `targets: true` offers.
- The adapter reports a `change_request` entry with `POST
  /v1/work-items/changes/submitted` (`changes-submitted.schema.json`). The reply has
  these members:
  - `change_request_id` and `target`.
  - `address` (`url` and `ref`, the reference that the target gives the change) and
    `action` (`created`, `updated`, or `kept` when the adapter changed nothing). Both are
    present or both are absent. A reply for an entry that asks for no change at the target
    has neither.
  - `data`: the result that the kind defines, such as the address of each commit of a
    pull request.

  Where the entry's target has an `address`, `address.url` is on its `system`. This
  contract checks no address inside `data`.
- An adapter that stopped trying reports `POST /v1/work-items/changes/failed`. The report
  has `comment_id`, or `change_request_id` with `target`. For a change request the reason
  is one of these:
  - `not_permitted`.
  - `unreachable`: the target timed out, could not be reached, or returned a busy or 5xx
    status until the retries ended.
  - `target_gone`: the place the change goes to does not exist.
  - `too_long`.
  - `refused`: the target refused a request with a status that no other reason covers.
    It also covers an entry that the adapter cannot apply, as for a `change_kind` that it
    did not list.

  A failure of a change request may have `detail`, one line of 1 to 1000 characters.
  It has what to do first, then the target's status and message. The adapter never puts
  an access token, a secret or a header in it. A failure of a comment has no `detail`.
  An adapter that gets a `change_kind` that it did not list sends `refused` with a
  `detail`.
- In Go, `ChangesSubmitted` is added. `ChangesFailed` has `ChangeRequestID`, `Target` and
  `Detail`. The actions of a reply are `ChangeCreated`, `ChangeUpdated` and `ChangeKept`.
  The reply has no draft type: its `data` is the part of the kind.
- The README of the integration contract states that the artifact type of a kind defines
  the kind. It states that the adapter that applies the kind defines its rules. The
  `pull-request` kind is an example. The README keeps the section on the submit program.
- The submit program contract, `contracts/run/v1/submit-program/README.md`, is for the
  program that `qory run` runs on a run with no run code, with the login of the forge's
  command line that the person made. A run's draft is opened by the adapter from the
  change_request entry, and qory runs the module's submit program only on a run with no run
  code. qory finds the program in the composed harness: the `submit_program` key,
  `{command: [<path>, <argument>, ...]}`, of the manifest of the forge's module, with
  the path relative to the module. A run with no run code has no work item and no
  earlier submission. Only a write run gets a draft. qory pushes the run's branch before
  it runs the program: `qory/run-<run id>`, or `qory/run-<run id>-2`, `-3` and so on
  when that name is taken, or `qory-run-<run id>` when a branch named `qory` exists, so
  the program reads the name from `QORY_SUBMIT_BRANCH`, with the check for keys and for
  files outside the session's scope that a run from a run code has (`push_key_found`,
  `push_scope_refused`); if a check refuses, qory pushes nothing and runs no program.
  The program pushes nothing, and opens a draft or edits the draft that it finds from
  the branch. qory sets the environment variables `QORY_SUBMIT_SYSTEM`,
  `QORY_SUBMIT_PATH`, `QORY_SUBMIT_BRANCH`, `QORY_SUBMIT_BASE_BRANCH`,
  `QORY_SUBMIT_BASE_COMMIT`, `QORY_SUBMIT_COMMITS`, `QORY_SUBMIT_DRAFT` (`open`, or
  `none` when the run has no hand-off text), `QORY_SUBMIT_TITLE` and
  `QORY_SUBMIT_DESCRIPTION_FILE`, and sets standard input to an empty file such as
  `/dev/null`. The program prints lines of the form `draft_number 34`, with the keys
  `draft_number`, `draft_url`, `draft_title`, `draft_state` and `draft_action`, and
  `commit_url <commit> <address>`, so it needs no JSON tool. qory records the valid
  lines after exit 0, 1 or 2. With `QORY_SUBMIT_DRAFT=none`, qory records no draft and
  no failure. Exit 3 means the person has no login of the forge's command line: the
  program prints as its first line on standard error, for a forge whose command line is
  `gh`, "Run `gh auth login`, then `qory run` again.", and qory records `login-needed`
  with exit 3. A program that runs more than 120 seconds is stopped and recorded as
  `program-timeout`, with no exit status and no line read. A program that did not start,
  or ended on a signal or another status, is recorded as `program-failed` with exit 1
  and no line read. For a draft that it asked for and the program did not make, qory
  records `program-failed` with the exit status. qory prints at most 20 lines of
  standard error, each prefixed `qory run: submit program: `. The contract has the rule
  of the marked block. In Go, these are added: `ParseSubmitLines`, `ReadSubmitRun`,
  `MarkedDescription` and `NextTitle`; the types `SubmitResult`, `SubmitLineError`,
  `SubmitRecord`, `SubmitFailure` and `SubmitEnd`; the input names `SubmitEnv*` and
  `SubmitDraftNone`; the key names `SubmitKey*`; `SubmitMarkerStart` and
  `SubmitMarkerEnd`; `SubmitPrintedLines` and `SubmitLinePrefix`; the exit statuses
  `SubmitExitDone`, `SubmitExitForge`, `SubmitExitInput` and `SubmitExitLogin`;
  `SubmitTimeLimit` and `SubmitLoginLine`; and the reasons `SubmitProgramFailed`,
  `SubmitProgramTimeout` and `SubmitLoginNeeded`.
- The run contract has a run's end, `outcome.schema.json`, titled Run end: `state`
  (`succeeded`, `failed` or `cancelled`), and optionally `reason` and `exit_code`, as
  Forager's `dev.qory.run.exited` has them. qory sets `reason` to one of the end codes
  to say why a run failed, and the schema lists each with its cause in plain words:
  `push_credential_missing` (the run's configuration has no credential with the purpose
  `push`), `push_refused` (a push of the run's commits was refused with status 403),
  `prebuilt_image_too_large` (the image of the target is larger than
  `largest_prebuilt_image_bytes`), `push_not_recorded` (qory pushed the run's commits
  and the control plane has no record of the push), `environment_image_missing` (the
  prebuild's environment image, `environment_image` in the run documents, is not in the
  registry), `push_head_changed` (the branch head at the forge is not the commit that
  the run expected, its start commit or its last push, or the branch is missing),
  `push_scope_refused` (a commit of the session changes a file that the session may not
  change: a file outside its plan's scope at the session's start, or any file outside
  `.qory/` when the session has no plan, unless the scope check that the harness module
  declares in `scope_checks` allows it), `push_key_found` (a commit of the session adds
  a key or a value of the run's secrets, or the commits are too large to verify, so qory
  pushed nothing),
  `run_documents_invalid` (the run's documents, or the response to the run code, fail
  qory's validation), `harness_invalid` (the harness that the run composes cannot be
  used), `machine_setup_invalid` (the machine cannot run the step, or has no registry),
  `target_fetch_failed` (qory could not fetch the target of the run),
  `repository_check_failed` (a git or folder step on the run's repository failed),
  `forge_unreachable` (qory could not reach the forge), `history_rewritten` (the HEAD of
  the run's working folder does not descend from the commit that the run started from,
  so a push would not be a fast-forward), `run_api_failed` (a call from qory to the run
  API failed), `prebuild_failed` (a prebuild failed: its build, the pull of its base
  image, or the push or verification of the image it built), `next_session_failed`
  (the next-session request after a session failed: Qory Apiary refused it or did not
  answer) and `outputs_invalid` (qory's output check stopped the run before the push: a
  declared output is missing or does not match its output type). A `reason` that is not in
  the list passes the schema, as a newer qory may send a code that an older reader does
  not know. One of the end codes with a `state` other than `failed` fails, and so does a
  member the schema does not define, such as `message` or `action`. None of them is one
  of Forager's reserved reasons. In Go, `RunOutcome`, `EndCode` and a constant for each
  code are added, named `EndCode` and the code in camel case, such as
  `EndCodePushHeadChanged`.
- The run contract has the next-session call, for a run of several sessions on one
  machine. After each session that did not pause, qory sends a request,
  `next-session-request.schema.json`: `session`, Forager's number of the session that
  ended, from 1; `state`, `succeeded`, `failed` or `cancelled`; and `reason`, the reason
  of the session's `dev.qory.session.exited`, or `null`. The request has no verdict: the
  control plane reads the newest verdict that the run stored after the previous step. The
  answer, `next-session-answer.schema.json`, is one of two shapes, and an answer with the
  members of both fails. `{"next": "session", ...}` has `session`, the control plane's
  number of the session in the run; `workflow`; `time_limit_minutes`; `inputs`; and
  optionally `type_settings`, `model_choice` and `model_provider_credential_name`.
  `{"next": "end"}` has no other member. qory sends the request through Forager's link,
  and the gateway forwards it to the control plane; a container cannot send it. In Go,
  `NextSessionRequest`, `NextSessionAnswer`, `NextSessionKindSession` and
  `NextSessionKindEnd` are added.
- The run contract has the control plane's coded refusals of the next-session request,
  `next-session-refusal.schema.json`: Forager's link refusal with `from` `apiary`, as
  the gateway passes them on. `error` is one of nine codes. Three come from the checks
  of every signed request: `429` `rate_limited`, the rate limit of the gateway's access
  key for the run endpoint is reached; `400` `bad_request`, an instance id that is
  absent or outside its pattern; and `400` `unsupported_contract_version`, a contract
  version other than 1. Two come from the checks of the run: `404` `not_found`, an
  unknown run or one that is not a shared run; and `409` `run_ended`, a run that ended
  or that a member cancelled. Four come from the checks of the request: `413`
  `payload_too_large`, a body over 4 KiB, which reaches qory only when the control
  plane's limit is lower than the gateway's 4 KiB; `400` `invalid_request`, a body
  outside the request's schema, with `names` and `message`; `409` `session_mismatch`, a
  session lower than the newest one the run reported; and `503` `unavailable`, a request
  that the control plane could not complete and that stored nothing, so the same
  request can be sent again. Another code, or `from` `gateway`, fails the schema. A
  signing header sent twice gets a `400` `bad_request` that is not signed: the gateway
  returns `502` `control_plane_unavailable` with `from` `gateway` for it, and this
  schema does not describe it.
- The gateway sets `retry_after` in a refusal of the next-session request when the
  `Retry-After` header of the control plane's response is whole seconds of at least 1, sent
  once. A wait over 2678400 seconds (31 days) becomes 2678400, the most the link refusal
  allows. `next-session-refusal.schema.json` accepts the member through the link refusal,
  and its description mentions it.
- `model_choice` of the next-session answer is `ModelChoice` in Go: the `model`,
  `effort`, `provider_models` and `tiers` of `run_settings`, with the same meaning.
  `model_provider_credential_name` is the name of a credential of the run, such as the
  provider's public id, a hyphen and the key's place from 1. The schema checks only the
  grammar of a credential's name in Forager's `run-configuration.schema.json`: 1 to 64
  of `a-z`, `0-9`, underscore, dot and dash, starting with a letter or a digit. Without
  `model_provider_credential_name`, the session joins with the run's default model
  provider credential, with the active key of its group when it has a group. The control
  plane sets `model_provider_credential_name` for every later session whose key is a
  credential of the run configuration other than the run's default model provider
  credential and the active key of its group.
- A session after session 1 may choose its runtime: `model_choice` gains `runtime`
  (`ModelChoice.Runtime` in Go), a runtime name in Forager's runtime descriptor, as
  `run_settings.runtime`. A `model_choice` without `runtime` runs the session on the
  runtime of `run_settings`, and `runtime` can be its only member: the session runs on
  that runtime with the run's default model provider credential (the active key of its
  group when it has a group) and that runtime's default model.
  The `runtime` of `run_settings` is the runtime of session 1 and of each later session
  whose `model_choice` has none. For a session with `runtime`, the control plane sets a
  `model_provider_credential_name` only for a credential of an API whose `runtimes`,
  when present, list that runtime and whose format that runtime sends.
- The next-session request follows the rules of `dev.qory.session.exited` for `state`
  and `reason` that do not depend on its `exit_code`: a `cancelled` session has a
  `reason`, the reasons `timeout` and `interrupted` are for a `cancelled` session only,
  and the reason `gateway_lost` is for a `failed` session only.
- The run documents of the step `explore` can contain `time_limit_minutes`,
  `shared_run` and `type_settings`, and `contracts.Documents` has them as
  `TimeLimitMinutes`, `SharedRun` and `TypeSettings`. The run documents are the documents
  of session 1.
  `time_limit_minutes` is the time limit of session 1, in minutes, from 1 to 1440.
  `shared_run` is `true` when qory sends the next-session request after each session.
  `type_settings` is an object whose members this contract does not list. `shared_run`
  left out is the same as `false`. A run of the step `prebuild` has none of the three.
- The run documents of every run of the step `explore` contain `run_settings`, and
  `contracts.Documents.RunSettings` is set for each of them. A run of the step
  `prebuild` has no `run_settings`. The runtime of session 1 is `run_settings.runtime`,
  and the runtime of a later session is the `runtime` of its `model_choice`, or
  `run_settings.runtime` when it has none.
- The run documents of the step `prebuild` can contain `largest_prebuilt_image_bytes`,
  and `contracts.Documents` has it as `LargestPrebuiltImageBytes`, an `*int64`: the
  largest size, in decimal bytes, that the target image of the build may have. It is an
  integer of at least 1 and at most 9223372036854775807. A run of any other step has no
  `largest_prebuilt_image_bytes`. When it is absent, the image has no size limit.
- The run documents can contain `deliver`: the artifacts that the run delivers, a list of
  1 to 16 entries with `type` and `send`, both required. `type` is the
  name of the artifact type: lower-case letters and digits in groups joined by single
  hyphens, 1 to 64 bytes. `send` is `end_of_run` or `after_gate`: the control
  plane writes the change_request entry at the end of the run or after the gate of the stage
  passes. The control plane writes `deliver` whenever the process version of the
  workspace has a type to deliver. `deliver` is present only on a run with `workflow` and
  with `access` `write`, and never on a run of the step `prebuild`. `deliver` sets the
  `type` and the `send` of each artifact that the run stores. On a run with a run code,
  qory takes these settings only from `deliver`, never from `process.yaml`. If `process.yaml`
  has different settings, qory prints one line with both values and uses `deliver` of
  the run documents. On that run, the adapter of the forge opens the draft from the
  change_request entry, and no submit program runs. A run that a person starts with no run
  code has no run documents, so it has no `deliver`; it runs the submit program of the
  composed harness. In Go, `Deliver` and `Documents.Deliver` are added.
- `access` in the run documents, and `access` of a workflow in `harness`, can be
  `write-no-push`: the run gets a read credential and a copy of the target that the
  agent may change, and the run sends nothing back to the target and has no `deliver`.
  A call with the path `run-credential` whose purpose is `work` has the access `read`
  for such a run.
- A description can declare the role `model_provider`. The role lists the provider's APIs
  in `apis`, each with its `format`, `endpoint` and `auth`, where `oauth` marks a
  subscription access token, and optional `key_prefixes`; and the `models` a new provider
  of this kind starts with.
- The `model_provider` role can list `limit_windows`: 1 to 11 windows of the provider's
  limits, such as `five_hour` and `seven_day`, that a key's `use_up_to_percent` applies
  to. A window is a period whose use the provider limits and reports as a percentage.
  The control plane sends the list as `use_up_to_windows` in the run configuration, with
  a key's `use_up_to_percent`. Without `limit_windows`, the provider's keys take no
  `use_up_to_percent`. In `descriptions/claude-subscription.json`, `limit_windows` is
  `["five_hour", "seven_day"]`.
- Five optional members describe model keys further. An API of `model_provider` can
  list `runtimes`, the runtimes that may use its key, each a name in Forager's
  `runtimes.json`; `paths`, the paths of the endpoint's host that the gateway sets the
  key on, which the control plane copies unchanged into `paths` of `api_credential`;
  and, only with `oauth: true`, `oauth_refresh`, with the `url`, `client_id` and
  optional `scope` with which the control plane refreshes an OAuth login. The
  `model_provider` role can set `key_lifetime_days`, from 1 to 3650: the days a key
  stays valid, counted from the time the key was added; the control plane warns 14 days
  before the end. A description can set `vendor`, the vendor's name in lines the control
  plane shows, such as `Anthropic rejected it on <date time>.`.
  `contracts.CheckModelProvider` refuses an `oauth_refresh` `url` whose host or port
  the control plane cannot read. It refuses an entry of `paths` that Forager's
  `policy.ValidPath` refuses, such as a path with a space character or with `{` or `}`
  anywhere but in a segment written exactly `{}`, and an entry that Forager's
  `policy.UnclearPath` reports: a path with an empty, `.` or `..` segment, a backslash,
  an encoded slash, backslash, dot or percent sign, or a control character, which the
  gateway denies in a request path. The schema's pattern for `paths` is Forager's
  `credential_paths` pattern. Every description in `descriptions/` sets `vendor`; in
  `descriptions/claude-subscription.json`, `runtimes` is `["claude"]` and
  `key_lifetime_days` is `365`.
- `descriptions/openai.json` describes the OpenAI API key provider: the Responses API at
  `https://api.openai.com/v1`, a bearer key that starts with `sk-`, and six GPT models.
  The endpoint and host follow OpenAI's API reference.
- `descriptions/openai-subscription.json` describes the OpenAI subscription provider: a
  ChatGPT plan's sign-in, for the `codex` runtime, on the Responses API at
  `https://chatgpt.com/backend-api/codex`, with `paths` `/backend-api/codex/*`,
  `limit_windows` `five_hour` and `seven_day`, the refresh at
  `https://auth.openai.com/oauth/token`, and ten GPT models. Its `auth` sets
  `account_header` `chatgpt-account-id` and `account_claim`, the pointer to the account
  id in the access token, as Forager's `auth.schema.json` defines them. Its
  `description` says that the subscription is for personal use only and links OpenAI's
  terms, as `descriptions/claude-subscription.json` does for Anthropic's.
- A description in the data form has no `program_version`: it is a file that no program
  prints, its one role is `model_provider`, and its settings have no member `writeOnly`.
  `descriptions/` contains the data descriptions of seven model providers, and the
  `name` of each is its file name without `.json`.
- A description can list `egress_hosts`, every host that runs of the integration reach:
  each entry is an exact lower-case host, or `{"setting": "<name>"}` for the host of the
  URL in a string setting that is not a secret. Each host of the `credential` role and of
  the `source` role's `apply` items has an equal entry, and so does each `model_provider`
  API endpoint: its host, or its setting. A single label, a localhost name, a loopback,
  link-local or `0.0.0.0/8` IPv4 address, and a number other than a canonical IPv4 address
  are refused.
- A description can declare the role `source`, the adapter that reads a source of work,
  such as a tracker: its catalogue line in `description`; its `program`, which takes the
  enrol command `<program> access-key enrol --secret-file FILE <server> <code>`; `setup`,
  the command that prepares the source side; `start`, the command that runs the program;
  the lines `connect`, `access` and `disconnect`; and the optional `item_noun`,
  `collection_noun`, `items: false`, `secrets`, `credential_scope` and `apply`. `secrets`
  lists the names of 1 to 16 secrets that the adapter stores in the control plane.
  `credential_scope` is `resource` or `grant`, how far one run credential reaches, and
  absent means `resource`. `apply` has 1 to 16 apply items in the shape of Forager's
  apply, for the run's work credential; a path may contain `{target.path}` once, right
  after a `/`, which the control plane fills with the path of the run's target.
- The adapter contract in `contracts/integration/v1`, revision 1: the signed requests that
  a source adapter sends the control plane, `POST /v1/adapter` at start-up,
  `POST /v1/source-collections`, `POST /v1/source-items`,
  `POST /v1/source-items/comments`, `POST /v1/sources/disconnect`,
  `POST /v1/connections/disconnect`, `POST /v1/grants/end` and
  `POST /v1/work-items/changes/failed`, and the change feed
  `GET /v1/work-items/changes`, with the entry kinds `work_item`, `comment`,
  `collection`, `grant`, `sync` and `watch`. The adapter signs each request with its
  access key as `github.com/qoryai/forager/accesskey` signs one, sends the revision in
  `X-Qory-Contract-Version`, and checks each signed answer under a server key it pinned at
  enrolment. `refusal.schema.json` lists the refusals.
- The change feed entry kind `watch`, with `work_item_id` and `url`, the work item's
  page in the control plane. The control plane writes no `watch` entry. In Go,
  `ChangeWatch` is added.
- In the change feed, entries for one item, the `work_item` and `comment` entries with
  the same `ref`, apply in cursor order. While the adapter retries an entry that failed,
  it may apply a later entry first when neither entry changes what the other depends on,
  such as entries for different items, but not a `grant` entry and an entry for an item
  that the grant covers. The adapter stores a cursor only when every entry before it is
  done. An entry is done once the adapter has applied it, or has passed it: after a
  failure that trying again cannot fix, or when the adapter's retries for it end. A
  `comment` entry is passed only once the adapter has also stopped sending its failure
  report (`changes-failed.schema.json`): on the signed 204, a `403 forbidden` or a
  `400 invalid_request`, or once its retries of the report end. The adapter applies no
  entry of a refused answer, and neither sends that answer's cursor as `after` nor stores
  it.
- A connect pass and a setup pass, signed by the control plane, send a person to the
  adapter's `connect_url` or `setup_url`, and a connect return, signed by the adapter,
  sends the person back. A connect pass may be for a live connection, which connects
  one external account to a connection, or replaces the grant of that account.
- A setup pass may contain `organisation_slug`, the slug of the instance's organisation
  in the control plane: 1 to 40 lower-case letters, digits and hyphens, with no hyphen at
  either end. An adapter may use it to suggest a name at its source, and accepts a pass
  without it. In Go, `SetupPass.OrganisationSlug` is added.
- Grants: a connection has at most one grant that has not ended, for one external
  account at the source. The adapter reports one grant with `POST /v1/grants`, with
  `connection_id`, `grant_ref`, `account` and `pass_nonce`, and the grant is added only
  when the control plane accepts the report.
  `account` is `{ref, title}`, both required: `ref` is the external account's id at the
  source, and `title` is its display name, one line. `GET /v1/grants` returns the control
  plane's copy of grants, and the adapter matches its state to that copy.
- Each collection in a part of `POST /v1/source-collections` requires `grant_ref`, the
  grant under the same connection that covers the collection. The control plane refuses a
  part that contains a `grant_ref` that its copy of grants does not have under
  `connection_id`, `400 invalid_request`, after the checks for `connection_unknown`,
  `connection_ended` and `connection_suspended`.
- Grant end: the adapter reports that the source no longer covers one grant with
  `POST /v1/grants/end`, with `version`, `time`, `connection_id` and `grant_ref`. The
  control plane ends that grant and the collections whose `grant_ref` in the latest
  listing that contains them is that grant's, and the connection stays. In this order,
  the control plane refuses a request that the access key may not make, `403 forbidden`;
  a `connection_id` that it does not hold for this adapter, or a pending connection,
  `403 connection_unknown`; a connection that ended, `409 connection_ended`; and a
  `grant_ref` that its copy of grants does not have under `connection_id`,
  `409 grant_unknown`. On success the control plane returns a signed 204 with no body. For
  a suspended connection, the control plane returns the signed 204 and ends the grant.
- Comments from the control plane: an adapter whose source role has `comments: true`
  receives a `comment` entry in the change feed for a comment that a person writes in the
  control plane on a work item taken in from one of the source's items. The entry has
  `comment_id`, `sc_` and 16 lower-case Crockford base32 characters, `ref`, `author_name`,
  1 to 100 characters on one line with at least one character that is not a space, and
  `body`, 1 to 65000 characters. The adapter posts the comment on the item as the
  integration's account, with the first line `<author_name> wrote in Qory Apiary:`, then
  an empty line, then `body` unchanged, and with a mark that contains the `comment_id`, in
  a form that the source keeps and that, where the source allows, people who read the
  comment do not see. The control plane sends `author_name` as it is, with no escaping.
  The adapter writes it in the first line as plain text: it escapes each character or
  sequence that the format of the source's comments would read as markup or as a
  mention. The escaped name, the other 22 characters of the first line, the two line
  breaks after it, and the mark with any line break before it are at most 536
  characters together: the room that `body` leaves of the 65536 characters of a
  comment. Before it posts, it checks the item for a comment with that mark, so it posts
  at most one comment per `comment_id`, and the comment of a post whose result is
  unknown, such as one that timed out, is not posted a second time. It sends the posted
  comment back with `POST /v1/source-items/comments` and its `comment_id`, and the
  control plane matches it to the stored comment and stores no second comment. After a
  failure that trying again cannot fix, or when its retries end, the adapter sends
  `POST /v1/work-items/changes/failed` with `version`, `time`, `comment_id` and `reason`:
  `item_gone`, `not_permitted`, `too_long` or `unreachable`. The control plane marks the
  comment not posted and stores `reason`. It returns `403 forbidden` for a request the
  access key has no permission for. On success it returns a signed 204 with no body, and
  for an unknown or already matched `comment_id` it returns the signed 204 and changes
  nothing. Without `comments: true`, the control plane writes no `comment` entry for the
  source.
- Adapter secrets: the adapter uploads up to 16 values with `POST /v1/adapter/secrets`,
  each at most 16384 bytes, sealed with HPKE `x25519-sha256-aes256gcm` to a pinned server
  key, and `GET /v1/adapter/secrets` returns them sealed to the adapter's access key.
- Calls: when the adapter declares `call_url` in its start-up body, the control plane
  calls it with a `POST` signed with its server key over `qory-call-ed25519-v1`, and the
  adapter signs its answer with its access key over `qory-call-answer-ed25519-v1`, bound
  to the call's signature. `resolve` maps 1 to 64 addresses each to a `grant_ref` and a
  `resource_ref`, or to `covered: false`. `read` returns the text of one document of a
  resource at one version, at most 262144 bytes, or `found: false`, with the exact version
  read in `resolved_at`. `run-credential` returns an access credential for one run: for
  the purpose `harness`, with read access under one grant; for the purpose `work`, for the
  one resource of the run's target collection, with read or write access, inside the work
  credential entry built from the source role's `apply`.
- A `resolve` result may contain `public: true`: the source shows the resource to anyone
  without a credential. An adapter answers it only if it serves `read-public`. A public
  result contains `resource_ref` and `public_ref`, and also `grant_ref` when a grant that
  the adapter holds covers the resource too. `public_ref` is the ref of the resource that
  a request without a credential can use, and may be equal to `resource_ref` in a result
  without `grant_ref`. The adapter decides `public` from the visibility of the resource as
  a request without a credential sees it, never from a grant it holds, and a resource that
  the source shows only to signed-in users is not public. In Go, `Resolved.Public` and
  `Resolved.PublicRef` are added.
- The call `read-public` returns the text of one document of a public resource at one
  version, under no grant, with the answer of `read`. Its body is the body of `read`
  without `grant_ref`. In this order, the adapter refuses a resource that the source does
  not show without a credential, a resource that no longer exists included,
  `403 resource_not_public`; then `404 version_not_found`, `409 document_not_text` and
  `503 source_unavailable`, which includes the source's rate limit. The control plane
  sends `read-public` only with `resource_ref` set to the `public_ref` of a result with
  `public: true`, never with the `resource_ref` of a covered result, and never sends
  `run-credential` for a resource that it reads with `read-public`, so reading a public
  resource gives the run no credential. In Go, `ReadPublic` and `ReadPublicPath` are
  added.
- The `source` role may contain `guide`, short step-by-step help for the set-up step,
  `setup`, and the connect step, `connect`, each optional. Each part has 1 to 10 `steps`
  and an optional `before` and `after`, each a prose line. The control plane shows the
  guide as plain text. It shows a first step that the control plane writes, then the
  adapter's steps, numbered after that first step, so the adapter's steps describe only
  the adapter's system. In Go, `Guide` and `GuidePart` are added.
- The `source` role may contain `targets: true`: the adapter's collections can have a
  target, a place where a run's changes go. An adapter with `targets` handles a
  `run-credential` call with purpose `work` for any of its collections that has a target,
  under the rules of `run-credential.schema.json`. This applies whatever source the run's
  work item came from, and also when the work item came from no item at a source. The
  control plane offers the collections of an adapter with `targets` as places for a run's
  changes, and does not offer them when `targets` is absent. In both cases, the control
  plane accepts the `target` of each collection in a listing
  (`source-collections.schema.json`).
- The provisioner contract, `contracts/provisioner/v1`, revision 1: the requests with
  which a node pool's starter claims a request for one machine, `POST /v1/pools/claim`;
  reports that it created the machine, `POST /v1/dispatches/<dispatch_id>/dispatched`;
  releases the dispatch before that, `POST /v1/dispatches/<dispatch_id>/release`;
  uploads the log of the machine, `POST /v1/dispatches/<dispatch_id>/machine-log`; and
  reports the result of deleting the machine,
  `POST /v1/dispatches/<dispatch_id>/machine-end`. The starter signs them with the pool's
  access key. The claim answer has the dispatch, its `lease_until`, the run code that the
  machine redeems and the gateway URL that the run code is bound to. A machine end has
  the `state` `deleting`, `gone` or `delete_failed`. A starter sends `gone` for every
  dispatch for which the machine adapter's `make` returned a machine, every dispatch
  whose release was refused `409 machine_exists`, and every dispatch that is the
  `dispatch_id` of a machine in an answer of `list`. It sends `gone` again until the
  control plane accepts it, or until a signed 400, 403 or 404 refusal ends the attempts.
  A starter sends `gone` when the machine adapter's delete call returned that no machine
  with the id exists at the vendor, or when it has no record of a machine for the
  dispatch and a successful answer of the machine adapter's `list`, received at or after
  `max_lifetime_minutes` after the claim, contained no machine for the dispatch; a
  failed `list` is no proof. A starter that has no machine command (`make`, `delete` and
  `list` of a machine adapter) runs the work of a dispatch as a process on its host, and
  the end of that process is the end of the machine. Such a starter needs no `list` and
  no no-record case. For a dispatch for which it started the process, it sends `gone`
  after the process exits. When the starter could not start the process after an
  accepted `dispatched`, it sends `gone` at once. For a dispatch whose `dispatched` was
  refused `409 lease_expired` or `410 gone`, it sends `gone` once no process for the
  dispatch runs, because an earlier `dispatched` may have been accepted with its answer
  lost. When a `dispatched` sent again is refused while the process runs, the starter
  stops the process and sends `gone` after the process exits. For a dispatch whose
  release was refused `409 machine_exists`, it sends `gone` at once when no process for
  the dispatch runs. A released dispatch may still get `gone` with no machine. When it
  had no earlier machine event and no refusal took its slot again, its slot stays free;
  otherwise `gone` frees it. `deleting` and `delete_failed` are optional, and a starter
  that has no machine command does not send them. Each dispatch takes a slot of the node
  pool's instance limit at the claim. A machine event is an accepted `dispatched`, the
  machine's redemption of the run code, or an accepted machine end in any state; a
  machine log is not one. Until the first machine event, the release, the lapse of the
  lease or the end of the dispatch frees the slot. From the first machine event on, the
  slot is kept at least until the control plane accepts `gone`. An accepted `gone` frees
  the slot once the dispatch has ended or is stale, and otherwise when the dispatch ends
  or goes stale. A later machine end other than `gone`, a `dispatched` refused
  `409 lease_expired` or `410 gone`, or a refused redemption of the run code takes a
  freed slot again, even over the instance limit. A refused redemption counts only when
  all of these are true. It comes through the gateway that the run code is bound to. The
  control plane still has the run code; the control plane deletes the run code when the
  work is closed or undone. The timestamp of the redemption is within 300 seconds of the
  clock of the control plane, as the enrolment contract requires. The redemption is
  refused because the run code expired, the run ended, or the dispatch is stale or ended.
  Its proof verifies. It is within the limit of attempts with one run code, the limit of
  the signed 429 `rate_limited` of the enrolment contract; past that limit, the control
  plane records nothing for the refusal. Neither `dispatched` nor a redemption of the run
  code was accepted before for the dispatch. Such a refused redemption gets the same
  answer as an unknown code, the unsigned 401 of the enrolment contract, past that limit
  too. An accepted `gone` never takes a slot, and after it no call takes the slot again.
  An accepted `dispatched` for a live dispatch takes no slot. The control plane frees a
  kept slot `max_lifetime_minutes` plus 60 minutes after the latest of the claim, the
  first accepted `dispatched`, the first accepted redemption when no `dispatched` was
  accepted before it, the last accepted machine end up to the first accepted `gone`, and
  the last refusal of a kind that takes a slot again, whether or not that refusal took
  the slot again, even when no `gone` arrives. An accepted `dispatched` or redemption
  sent again does not move that time. A claim may contain `offers`: the machines the pool
  can create, each a `machine_spec` of vCPUs, memory, disk, architecture and
  virtualization. The control plane gives a claim with offers only work for a run whose
  machine type one offer fits. An answer to a claim with offers has the run's machine
  type in `machine` and its spec in `machine_spec`, and an answer to a claim without
  offers has neither. A dispatched may contain `machine_type`, the vendor's type of the
  machine. In Go, `MachineEnd`, `MachineEndDeleting`, `MachineEndGone`,
  `MachineEndDeleteFailed`, `MachineSpec` and `MachineSpec.Fits` are added.
- The machines contract, `contracts/machines/v1`: a machine adapter makes and deletes
  machines at one vendor, and the provisioner runs it as
  `<adapter> <command> --settings FILE` for the commands `check`, `make`, `list`, `log`
  and `delete`. Each command reads one JSON request on standard input and writes one
  answer of at most 1 MiB on standard output, or writes an error and exits with status 1.
  A second `make` with the same `dispatch_id` makes no machine. The `check` answer has
  `offers`, the machines the adapter can make with its settings, which the provisioner
  sends in its claims. `make` has `machine_spec`, and the adapter makes the smallest
  machine in its settings that fits it. The `make` answer may contain `machine_type`,
  the vendor's type of the machine.
- The explore contract, `contracts/explore/v1`: `inputs.schema.json`, the run's inputs for
  the step `explore`. In the inputs, `ticket` is absent when the work item was not taken
  in from an item at a source.
- The diagnostics contract, `contracts/diagnostics/v1/diagnostics.schema.json`: the run
  diagnostics, the document that the control plane returns about one run in the 200
  answer to `GET /v1/runs/<run id>/diagnostics`, and that **Copy diagnostics** on the
  run page copies. It contains the run, its queue entry, its dispatch, the steps of its
  preparation and why the preparation stopped, the run's last events and terminal
  output, and, for an instance admin and a diagnostics key only, the machine logs.
  Text members contain the text as the control plane stored it, unfiltered, so a secret
  that the run printed appears as printed. `leaked_secrets` lists each secret that the
  run printed as compromised, with its kind, where it appeared and a message to revoke
  or rotate it, and `leaked_secrets_omitted` counts those beyond the first 1000. Entries
  found in a machine log are present only when the document has the machine logs. The
  redacted fields of the run never appear as fields, and variables appear by name only.
  `run.machine` contains the `type` and the display `name` of the machine type that the
  run used. `run.machine` is absent when the control plane has no record of the machine
  type, and `type` or `name` is absent when the control plane has no value for it.
  The run list, `contracts/diagnostics/v1/runs.schema.json`, is one page of the runs of
  the instance in the 200 answer to `GET /v1/runs`. The request is signed with a
  diagnostics key. The rows are ordered by `created_at`, the last first, then by id,
  the greatest first. The query parameters `workspace` (`<organisation>/<workspace>`),
  `work_item` (a work item number, only with `workspace`), `workflow`, `state` and
  `since` select the runs. `before` is the `next` of the previous page. `limit` is 1 to
  100, 25 by default. Each row contains the run's id, number, organisation, workspace,
  workflow, work item number and title, state, reason, `created_at`, `started_at` and
  `duration_seconds`. A reader accepts a `state` other than the six that the run
  diagnostics list, and shows it as it is. A bad parameter is refused with 400
  `invalid_request`. The refusal, `runs-refusal.schema.json`, contains the parameter's
  name in `parameter`. The query is read as `application/x-www-form-urlencoded`, so `+`
  is a space. A request whose query is not valid UTF-8 after percent-decoding is refused
  with 400 and a body that is not defined, which can come before the request is
  verified. Version 1 changes only by adding members and row states, so the
  schemas of the diagnostics contract accept a member they do not define. In Go,
  `DiagnosticsBase`, `DiagnosticsFS`, `CompileDiagnostics` and `DiagnosticsDocument`
  are added. For the run list, Go adds `RunList`, `RunListRow`, `RunListOrganisation`,
  `RunListWorkspace`, `RunListRefusal`, `RunListInvalidRequest`, the parameter names
  `RunListQueryWorkspace` to `RunListQueryLimit`, `RunListDefaultLimit` and
  `RunListMaxLimit`.
- The run documents, `contracts/run/v1/documents.schema.json`: the object that a machine
  receives, signed, when it redeems its run code. `step`, `domain` and `inputs` are
  present for every run, and `access`, `target`, `target_version`, `work_item`,
  `run_settings`, `time_limit_minutes`, `shared_run`, `type_settings`, `workflow`,
  `harness_hidden`, `harness`, `image_destination`, `environment_destination` and
  `environment_image` by step. `target_version` is the exact
  version at which the control plane checked for the harness file of the target.
  `harness` is present in three cases: that file is absent, its `harness` section has no
  `extends` and lists no module, or the file was not read. In each of these cases, the
  machine composes `harness`. The registry host of `image_destination`,
  `environment_destination` and `environment_image` is lower case only, and a Docker Hub
  host is refused. `workflow` and each name in `harness.workflows` are a lower-case
  letter, then up to 54 lower-case letters, digits and hyphens, so that
  `workflow-<name>` stays within 64 characters. The rest of this entry applies to a
  target that is a repository, as described under `target_version`. The machine
  composes the harness file of the target at `target_version` when its `harness`
  section has `extends` or lists a module. A branch of the run's work item does not
  change that version. If the run works on such a branch, the machine checks it out after
  the compose. An agent may change, on that branch, the harness file of the target or a
  file that the compose reads. A run composes that change only when the run's
  `target_version` contains the change.
- The `harness` section of the run documents extends a stack or defines one. A section
  without `extends` defines a stack: it requires `modules`, at least one, and may contain
  `workflows`, each `<name>: {access, plan, tier}` with the names, keys and values of
  `workflows` in `qory-stack.yaml`, so the stack declares the access level, the plan and
  the tier of each of its workflows. `plan` is the artifact type whose launch version
  sets what a run of the workflow may push, such as `build-plan`, and only a workflow
  with `access` `write` has one. `tier` is `light`, `balanced` or `thinking`, the tier of
  the model and effort of the run's main session. A section that defines a stack,
  without `extends`, takes no target: the machine composes it for the runtime of the
  session: `run_settings.runtime` for session 1; for a later session, the `runtime` of
  its `model_choice`, or `run_settings.runtime` when it has none. A section with
  `extends` takes the workflows of its base, and the schema rejects `workflows` in it.
  In Go, `HarnessSection.Extends` is a pointer, nil for a section that defines a stack,
  and `HarnessSection.Workflows` and `HarnessWorkflow` are added. `bind` maps a kind,
  `agents`, `commands` or `skills`, to its roles, each to the name of one entry of that
  kind: `bind: {agents: {developer: web-developer}}`. A group role of the kind `agents`,
  a role that several agents fill, takes a list of 1 or more agents, as in qory's
  `qory.yaml`: `bind: {agents: {reviewers: [copy-reviewer, legal-reviewer]}}`. A name may
  appear twice in a list. A list on a `commands` or `skills` role and an empty list fail
  the schema. A list on any `agents` role passes it; qory's compose fails for a list on
  a role that is not a group in the module manifests. In Go,
  `HarnessSection.Bind` is a `map[string]map[string]BindValue`: `BindValue.Name` is set
  for a name, and `BindValue.Names` for a list, and each is written back in the form it
  was read in. Decoding a `HarnessSection` ignores a bind kind other than `agents`,
  `commands` or `skills`. A module role that no bind fills stays unfilled, and the reader
  must not compose that module role. Decoding returns an error for a kind whose value is
  null, for a role whose value is null, for a list on a `commands` or `skills` role, for
  an empty list, and for a null item of a list.
- Package `contracts` embeds every contract, with `ProvisionerFS`, `RunFS`, `ExploreFS`
  and `MachinesFS` beside `FS`, and compiles their schemas with `CompileProvisioner`,
  `CompileRun`, `CompileExplore` and `CompileMachines` beside `Compile`. A schema that
  refers to another embedded schema, or to a schema of the Forager contract, compiles with
  nothing fetched. Go types such as `Adapter`, `Grant`, `RunCredential`, `Claim`,
  `Documents` and `MachinesMake` marshal to the documents that their schemas accept.
  `CheckDescription`, `CheckModelProvider` and `CheckEgressHosts` check the rules of a
  description that its schema describes only in words.
- `conformance.Description` also checks a description's `model_provider` role and
  `egress_hosts`, and refuses an API `auth` of the scheme `header` whose header
  conformance reserves, as `conformance.Credential` refuses one in an `apply` entry.
  `conformance.DataDescription` checks a data description by the same rules.
- `release.yml` attaches `description.json` to each release: the description that
  `<program> describe` prints and that passes the `conformance.Description` check.
  `checksums.txt` lists its SHA-256 beside the archives.

### Changed

- `full_suite` moves into `type_settings`, a new member of the run documents and of the
  next-session answer for a session. `type_settings` is an object whose members this
  contract does not list. The process of the workspace type registers its schema, and
  the schema of the software factory requires `full_suite` in it. The run documents have
  it only for the step `workflow`. The answer for a session had `full_suite` as a required
  member and has `type_settings` as an optional one.
- A target has the ref of one of the adapter's collections, `collection`. It may have an
  `address` with `system` and `path`. Before, `system` and `path` were required. The place
  of a run in another system, such as an advertising profile, has a `collection` and no
  `address`. `ParseSubmitLines` and `ReadSubmitRun` check that an address is on the
  repository's system only where `system` is not empty. The other rules of an address
  stay. `run.target` of `diagnostics.schema.json` has the same two members, and still
  accepts a member it does not define. Its `address.system` and `address.path` have the
  rules of `$defs/target`. `$defs/address_path` is the path of an address: 1 to 200
  characters and no control character.
- The `pattern` of `$defs/url` in `definitions.schema.json` now refuses URLs that the
  description excludes and that only the `format` keyword refused. A host takes only the
  percent-encodings `%25` and `%80` to `%FF`, so `https://a%41.example/` fails the
  pattern. After the closing bracket of an IPv6 host, the URL may only end or continue
  with `:`, `/`, `?` or `#`, so `https://[::1]ab/x` and `https://[::1].5/x` fail it. The
  description is unchanged.
- `run_settings` has no `time_limit_minutes`. The time limit of session 1 is
  `time_limit_minutes` in the run documents, with the same limits, 1 to 1440 minutes.
  `contracts.RunSettings` has no `TimeLimitMinutes`.
- The module requires Forager at its main commit b6ac9f0
  (`v0.7.1-0.20261023001330-b6ac9f060456`), in place of v0.7.0.
- The contract describes `domains` as applying only to where work comes from: a
  credential of the `credential` role reaches a run in a workspace of any domain.
- The tier map of the run documents, `run_settings.tiers`, has the tiers `light`,
  `balanced` and `thinking`. `contracts.Tiers` has the member `Balanced`.
- When a node pool's starter created no machine for a dispatch, the machine log,
  `POST /v1/dispatches/<dispatch_id>/machine-log`, contains the error text of its attempt
  to create one, such as the error that the machine adapter wrote. The members of the
  machine log and revision 1 of the provisioner contract stay as they are.
- The control plane refuses a `changes-submitted` reply, 400 `invalid_request`, when
  `address.url` breaks one of these rules, as in the submit program contract:
  - The host is the `system` of the entry's `target.address`, in lower case.
  - The address has no port, query or fragment.
  - The path is not empty or `/` alone.

  Where the target has no `address`, only the form of `$defs/url` applies. The schema
  accepts such a reply. The fixture
  `refused-by-control-plane/changes-submitted-address-other-host.json` shows it.
- A reader that cannot read a refusal, or cannot meet its `must_understand`, acts on the
  status alone. On 403 or 409 it changes nothing and sends the request again. The
  integration refusal says to send it with backoff, and the provisioner refusal at the
  next check. In the integration refusal, such a 403 never counts as the connection ended.
  A machine-end is the exception in the provisioner refusal: it stops on a signed 400, 403
  or 404, read or not.
- A Go reader of a contract document matches a member name by exact case. Before this
  release, a member such as `MUST_UNDERSTAND` or `STATE` set the field of `must_understand`
  or `state`, because `encoding/json` ignores letter case. The schemas treat such a member
  as undefined, so the reader and the schema disagreed. Each `json` tag in the `contracts`
  package has the option `case:strict`. In the default build of Go 1.27, where the
  `jsonv2` experiment is on, `encoding/json` and `encoding/json/v2` both honour it.
  A program built with `GOEXPERIMENT=nojsonv2` reads these members regardless of case.
  `CheckMustUnderstand`, `Tier` and `HarnessSection` read the same way.
- A prose line (`$defs/line` of `description.schema.json`) refuses every control character
  (category Cc), U+2028 and U+2029. It also refuses every format character (category Cf)
  except U+200C and U+200D. Before this release, it refused a fixed list of characters.
  That list missed format characters such as the tag characters U+E0001 and U+E0020 to
  U+E007F. Emoji sequences and some scripts need U+200C and U+200D, so both stay allowed.
  - These members refer to `$defs/line`, so they have the rule: the lines of a description,
    the `vendor`, the `author_name` of a comment, and the `title` of a change request.
    The `title` of a subject and, in the explore inputs, the member `name` have it too.
  - The subject `id` of the run documents refers to `$defs/line` too. Before this release,
    it refused only control characters, U+2028 and U+2029.
  - These members have a copy of the pattern, because their length limit is above 300
    or they allow tab, line feed and carriage return: the failure `detail`, the account
    `title` of a grant, and the explore inputs `text`. `text` allows those three controls.
  - A change request title and the `author_name` of a comment also need a character that is
    not a space, U+200C or U+200D. Before this release, a value of U+200D alone was valid. A guide line of U+200D alone
    stays valid.
  - The rule follows the Unicode version of the Go toolchain that builds the program
    (Unicode 17.0.0 with Go 1.27.4). A later Unicode version can add characters to Cf.
    A validator on another Unicode version can accept them.
  - A validator that does not read `\p{Cc}` and `\p{Cf}`, such as an ECMA-262 engine
    without the `u` flag, cannot use these patterns as written.
  - `ParseSubmitLines` reads the title patterns from the schemas, so it has the same rule.
- The adapter sends a grant report again after every answer that is not final, until the
  `expires_at` of the linked connect return. These are `409 pass_unknown`,
  `403 connection_unknown`, `409 connection_suspended`, and a `403` or `409` that the
  adapter cannot read. It uses the backoff of `grant.schema.json`, also while the connection
  is suspended. After `expires_at`, the adapter stops sending the report and drops
  the grant, unless the control plane's copy has it.
- Three texts called Forager's `auth`, `credential` and `dev.qory.run.exited` closed: the
  READMEs of the integration and run contracts, and `outcome.schema.json`. Forager's
  `auth`, `credential` and the data of `dev.qory.run.exited` are open. A reader ignores a
  member that their schema does not define, unless `must_understand` lists it. A reader
  refuses an unknown `scheme` or `format`. No member changed.
- The module requires Forager at the commit 3a8876d (`v0.7.1-0.20261120100355-3a8876d85c0d`),
  in place of 5e9ca86. From that commit, a reader of Forager's `auth`, `credential` and
  the data of `dev.qory.run.exited` ignores a member that their schema does not define,
  unless `must_understand` lists it.
- The control plane writes no `watch` entry in the change feed. Four texts described a
  `watch` entry that it writes whenever a run stores a change request version:
  `changes.schema.json`, the `targets` text of `description.schema.json`, the README of
  the integration contract, and the comment of `contracts.Change`. The `targets` text has
  no sentence on the `watch` entry, and the other three state that the control plane
  writes none. The kind `watch`, its members `work_item_id` and `url`, and its rules stay
  defined, and `ChangeWatch` stays. An adapter that reads a `watch` entry checks it
  against `$defs/watch` and applies the `must_understand` rules, as for any entry of a
  defined kind. No member changed.
- The run contract describes the numbers of workflow sessions. A workflow session after
  session 1 starts only after a next-session answer. qory can run an agent hook as a
  session between two workflow sessions. The hook session takes the next Forager session
  number, and qory sends no next-session request for it, so the `session` numbers in the
  requests can have gaps. The `session` numbers in the answers stay the control plane's
  sequence numbers, with no gaps. The text is in the run contract README, `session` of
  `next-session-request.schema.json` and `next-session-answer.schema.json`, the request
  and `shared_run` descriptions of the run contract schemas, and the comments of
  `NextSessionRequest`, `NextSessionAnswer` and `Documents.SharedRun`. A request follows
  each workflow session that did not pause, not each session. No schema keyword changed.

### Removed

- `submit_module` is no member of the adapter's start-up (`adapter.schema.json`) or of
  the run documents (`documents.schema.json`), and `$defs/submit_module` and
  `$defs/harness_pinned_source` are gone. A run with a run code has no submit program;
  the forge's adapter opens the draft from the change_request entry. In Go, `SubmitModule`,
  `Adapter.SubmitModule` and `Documents.SubmitModule` are removed.

## [0.3.0] - 2026-10-12

### Upgrading

- Require `github.com/qoryai/integrations` v0.3.0, and call the workflows at `@v0.3.0`.
- `release.yml` publishes nothing unless `<program> describe` prints a description that
  `conformance.Description` accepts, and the integration's `go.mod` requires this module
  at v0.3.0 or later.
- `conformance.Credential` refuses an `apply` entry of the scheme `header` whose header it
  reserves, and its refusal of an answer that Forager's schema refuses starts
  `credential: the gateway's schema refuses the answer:`. A test that matches the old
  text changes with it.

### Added

- Command `integration-conformance`, `cmd/integration-conformance`: reads one description
  on standard input and checks it with `conformance.Description`. It exits 0 when the
  description passes, 1 with the refusal on standard error when it does not, and 2 when
  it is given an argument or cannot read standard input.
- `release.yml` runs `<program> describe` and checks the description with
  `conformance.Description`, through `integration-conformance`, built within the
  integration's Go module, at the version of this module that the integration's `go.mod`
  requires. A describe that fails, a description the contract refuses, or a `go.mod` that
  does not require `github.com/qoryai/integrations` publishes nothing.
- `conformance.Credential` refuses an `apply` entry of the scheme `header` whose `header`
  is one it reserves: a name in `conformance/headers.json`'s `refused`, or one that starts
  with a prefix in its `refused_prefixes`, compared in lower case, such as `Cookie` or
  `X-Forwarded-Host`. Each such entry is refused separately, and the refusal names the
  entry's index and the header. The error has an `Unwrap() []error` method that returns
  the refusals. The text of every other refusal is unchanged, but for the one under
  Changed.

### Changed

- `go.yml` runs its job in `ghcr.io/qoryai/ci-images:golang-1.27`, by digest, in place of
  `docker.io/golang:1.27`: a copy of Docker Hub's official `golang:1.27` image, under the
  same digest, in a public package of qoryai's. It is pulled without a login, so a
  repository that calls `go.yml` grants no permission for it, and it is out of reach of
  the pull limits that Docker Hub and Amazon's copy set for pulls made without a login.
- The module requires `github.com/qoryai/forager` v0.7.0 in place of
  `github.com/qoryai/runner`, and `conformance.Credential` checks an answer against
  Forager's `credential.schema.json`, in `contracts/forager/v1`. Its refusal of an answer
  that schema refuses reads
  `credential: the gateway's schema refuses the answer: …`, where it read
  `credential: the runner's schema refuses the answer: …`.
- The contract, the README and the guide declare an integration under `gateway.integrations`
  in a machine's `~/.config/qory/forager.yaml`, which `qory` expands under
  `gateway.credentials`, where they said `integrations:` in `runner.yaml`. The program is
  installed on the machine where the gateway runs.
- The contract no longer reserves the role names `tool`, `work_source` and `output`, and no
  longer says how a control plane offers integrations by `domains`. What validates is
  unchanged.
- `go.yml` keeps Go's build cache between runs, next to the module cache: it restores the
  newest one saved for the same Go version, preferring the same `go.sum`, and each passing
  run saves the cache for its `go.sum`.

## [0.2.0] - 2026-09-30

### Upgrading

- Require `github.com/qoryai/integrations` v0.2.0, and call the workflows at `@v0.2.0`.
  The Go module proxy and the checksum database hold v0.1.0 as an earlier tree of this
  repository: `qory-github` under `github/`, and no `conformance` package. They keep that
  tree for good. So `go.mod` retracts v0.1.0, and v0.2.0 publishes the tree that the tag
  v0.1.0 on GitHub points at, the one described under 0.1.0 below. Nothing else changes.

## [0.1.0] - 2026-09-29

### Added

- The integration contract, `contracts/integration/v1`: `<program> describe` prints one
  JSON document, `description.schema.json`, with the integration's name, the domains it
  serves, its settings as a JSON Schema, a secret marked `writeOnly`, and the roles it
  plays, `credential` with the argument and the hosts of a runner definition. Every role
  is started as `<program> <role> --settings <json> -- [arguments]`, so a reader expands
  any declared integration into its definitions from its description alone; a `$` of the
  settings is written `$`. A secret is a top-level setting with a `<name>_file` in
  its place on a command line. A program exits 0 with one document on standard output, or
  non-zero with one line on standard error.
- Package `contracts`, which embeds the contract, its schema and its fixtures, and
  compiles the schema.
- Package `conformance`, which checks what a program prints: `Description` what
  `describe` printed, against the schema and the secret rule the schema cannot express;
  `Credential` a credential role's answer, against the runner's credential schema;
  `Failure` how a failed command ended, a status other than 0, nothing on standard output
  and one line on standard error.
- The workflows every Go integration calls: `go.yml`, its formatting, vet, tests, build
  and doc comments, and `release.yml`, which builds the program for Linux and macOS, amd64
  and arm64, and publishes the release by one rule, README §Release rule.
- The catalog of integrations in the README, `qory-github` first.
- `docs/writing-an-integration.md`, the guide: what an integration is, its roles, the
  contract in short, secrets, names, layout, testing, releasing and declaring it.

### Moved

- Each integration lives in a repository of its own: `qory-github` is in
  [qoryai/qory-github](https://github.com/qoryai/qory-github), and a new integration
  starts from [qoryai/integration-template](https://github.com/qoryai/integration-template).
  This repository keeps the contract, the module every integration imports, and the
  catalog.

[Unreleased]: https://github.com/qoryai/integrations/compare/v0.3.0...HEAD
[0.3.0]: https://github.com/qoryai/integrations/releases/tag/v0.3.0
[0.2.0]: https://github.com/qoryai/integrations/releases/tag/v0.2.0
[0.1.0]: https://github.com/qoryai/integrations/releases/tag/v0.1.0
