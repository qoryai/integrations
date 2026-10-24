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
- The module requires Go 1.27.3 or later (`go 1.27.3` in `go.mod`), because Forager does.
  An integration that requires this module builds with Go 1.27.3 or later. `go.yml` sets
  `GOTOOLCHAIN` to `go1.27.3+auto`, because its image has Go 1.27.2.

### Added

- A description can declare the role `model_provider`. The role lists the provider's
  APIs in `apis`, each with its `format`, `endpoint` and `auth`, where `oauth` marks a
  subscription token, and optional `key_prefixes`; and the `models` a new provider of
  this kind starts with.
- `release.yml` attaches `description.json` to each release: the description that
  `<program> describe` printed and that passed the `conformance.Description` check.
  `checksums.txt` lists its SHA-256 beside the archives.

### Changed

- The module requires Forager at its main commit b6ac9f0
  (`v0.7.1-0.20261023001330-b6ac9f060456`), in place of v0.7.0.

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
