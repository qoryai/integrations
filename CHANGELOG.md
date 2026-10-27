# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

### Upgrading

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
- The module requires Go 1.27.3 or later (`go 1.27.3` in `go.mod`), because Forager does.
  An integration that requires this module builds with Go 1.27.3 or later. `go.yml` sets
  `GOTOOLCHAIN` to `go1.27.3+auto`, because its image has Go 1.27.2.
- Run documents have no top-level `stack` and no `stack_reference`, members that this
  module had after 0.3.0 and before this release. A reader takes the harness from the
  harness file of the target at `target_version`, or from `harness`, as Added describes.
  A run with `workflow` has `harness`, `target_version` or both. In Go, `Documents.Stack`
  and `Documents.StackReference` are removed, and `Documents.TargetVersion` and
  `Documents.Harness` are added.
- If an adapter follows the adapter contract as this module had it after 0.3.0, change
  the adapter in three ways. Accept change feed entries of the kind `grant`, because
  `changes.schema.json` refuses an entry of a kind it does not list. After the adapter
  mints a linked connect return, report the grant with `POST /v1/grants`, read
  `GET /v1/grants` when the adapter starts, follow the `grant` entries, and match the
  adapter's state to that copy. Accept a connect pass for a live connection, which adds
  or replaces the grant of one external account.
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

### Added

- A description can declare the role `model_provider`. The role lists the provider's APIs
  in `apis`, each with its `format`, `endpoint` and `auth`, where `oauth` marks a
  subscription access token, and optional `key_prefixes`; and the `models` a new provider
  of this kind starts with.
- A description in the data form has no `program_version`: it is a file that no program
  prints, its one role is `model_provider`, and its settings have no member `writeOnly`.
  `descriptions/` contains the data descriptions of five model providers, and the `name`
  of each is its file name without `.json`.
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
  `POST /v1/connections/disconnect` and `POST /v1/grants/end`, and the change feed
  `GET /v1/work-items/changes`, with the entry kinds `work_item`, `collection`, `grant`
  and `sync`. The adapter signs each request with its access key as
  `github.com/qoryai/forager/accesskey` signs one, sends the revision in
  `X-Qory-Contract-Version`, and checks each signed answer under a server key it pinned at
  enrolment. `refusal.schema.json` lists the refusals.
- A connect pass and a setup pass, signed by the control plane, send a person to the
  adapter's `connect_url` or `setup_url`, and a connect return, signed by the adapter,
  sends the person back. A connect pass may be for a live connection, to add or replace
  the grant of one external account.
- A setup pass may contain `organisation_slug`, the slug of the instance's organisation
  in the control plane: 1 to 40 lower-case letters, digits and hyphens, with no hyphen at
  either end. An adapter may use it to suggest a name at its source, and accepts a pass
  without it. In Go, `SetupPass.OrganisationSlug` is added.
- Grants: a connection has one grant per external account at the source. The adapter
  reports one grant with `POST /v1/grants`, with `connection_id`, `grant_ref`, `account`
  and `pass_nonce`, and the grant is added only when the control plane accepts the report.
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
- The provisioner contract, `contracts/provisioner/v1`, revision 1: the requests with
  which a node pool's starter claims a request for one machine, `POST /v1/pools/claim`;
  reports that it created the machine, `POST /v1/dispatches/<dispatch_id>/dispatched`;
  releases the dispatch before that, `POST /v1/dispatches/<dispatch_id>/release`; and
  uploads the log of the machine, `POST /v1/dispatches/<dispatch_id>/machine-log`. The
  starter signs them with the pool's access key. The claim answer has the dispatch, its
  `lease_until`, the run code that the machine redeems and the gateway URL that the run
  code is bound to.
- The machines contract, `contracts/machines/v1`: a machine adapter makes and deletes
  machines at one vendor, and the provisioner runs it as
  `<adapter> <command> --settings FILE` for the commands `check`, `make`, `list`, `log`
  and `delete`. Each command reads one JSON request on standard input and writes one
  answer of at most 1 MiB on standard output, or writes an error and exits with status 1.
  A second `make` with the same `key` makes no machine.
- The explore contract, `contracts/explore/v1`: `inputs.schema.json`, the run's inputs for
  the step `explore`, and `result.schema.json`, the run's final response, which the
  control plane reads from the result text of the last `dev.qory.runtime.result` event.
  In the inputs, `ticket` is absent when the work item was not taken in from an item at
  a source.
- The run documents, `contracts/run/v1/documents.schema.json`: the object that a machine
  receives, signed, when it redeems its run code. `step`, `domain` and `inputs` are
  present for every run, and `access`, `target`, `target_version`, `work_item`,
  `run_settings`, `workflow`, `harness_hidden`, `harness`, `image_destination`,
  `environment_destination` and `environment_image` by step. `target_version` is the exact
  version at which the control plane checked for the harness file of the target.
  `harness` is present when that file is absent, when its `harness` section has no
  `extends` and lists no module, or when the file was not read, and the machine then
  composes `harness`. The registry host of `image_destination`,
  `environment_destination` and `environment_image` is lower case only, and a Docker Hub
  host is refused.
- The `harness` section of the run documents extends a stack or defines one. A section
  without `extends` defines a stack: it requires `modules`, at least one, and may contain
  `workflows`, each `<name>: {access, time_limit}` with the names, keys and values of
  `workflows` in `qory-stack.yaml`, so the stack declares the access level of each of its
  workflows. A section that defines a stack, without `extends`, takes no target: the
  machine composes it for `run_settings.runtime`, or, when the run documents have no
  `run_settings`, for the default runtime of the machine. A section with `extends` takes
  the workflows of its base, and the schema rejects `workflows` in it. In Go,
  `HarnessSection.Extends` is a pointer, nil for a section that defines a stack, and
  `HarnessSection.Workflows` and `HarnessWorkflow` are added.
- Package `contracts` embeds every contract, with `ProvisionerFS`, `RunFS`, `ExploreFS`
  and `MachinesFS` beside `FS`, and compiles their schemas with `CompileProvisioner`,
  `CompileRun`, `CompileExplore` and `CompileMachines` beside `Compile`. A schema that
  refers to another embedded schema, or to a schema of the Forager contract, compiles with
  nothing fetched. Go types such as `Adapter`, `Grant`, `RunCredential`, `Claim`,
  `Documents` and `MachinesMake` marshal to the documents that their schemas accept.
  `CheckDescription`, `CheckModelProvider` and `CheckEgressHosts` check the rules of a
  description that its schema states only in words.
- `conformance.Description` also checks a description's `model_provider` role and
  `egress_hosts`, and refuses an API `auth` of the scheme `header` whose header
  conformance reserves, as `conformance.Credential` refuses one in an `apply` entry.
  `conformance.DataDescription` checks a data description by the same rules.
- `release.yml` attaches `description.json` to each release: the description that
  `<program> describe` prints and that passes the `conformance.Description` check.
  `checksums.txt` lists its SHA-256 beside the archives.

### Changed

- The module requires Forager at its main commit b6ac9f0
  (`v0.7.1-0.20261023001330-b6ac9f060456`), in place of v0.7.0.
- The contract states that `domains` applies only to where work comes from: a credential
  of the `credential` role reaches a run in a workspace of any domain.

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
