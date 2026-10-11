# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

### Added

- Command `integration-conformance`, `cmd/integration-conformance`: reads one description
  on standard input and checks it with `conformance.Description`. It exits 0 when the
  description passes, 1 with the refusal on standard error when it does not, and 2 when
  it is given an argument or cannot read standard input.
- `release.yml` runs `<program> describe` and checks the description with
  `conformance.Description`, through `integration-conformance` built in the integration's
  own module, at the version of this module its `go.mod` requires. A describe that fails,
  a description the contract refuses, or a `go.mod` that does not require
  `github.com/qoryai/integrations` publishes nothing.
- `conformance.Credential` refuses an `apply` entry of the scheme `header` whose `header`
  is one it reserves: a name in `conformance/headers.json`'s `refused`, or one that starts
  with a prefix in its `refused_prefixes`, compared in lower case, such as `Cookie` or
  `X-Forwarded-Host`. Each such entry is a refusal of its own, naming its index and the
  header, and the error has an `Unwrap() []error` method that returns them. The text of
  every other refusal is unchanged, but for the one under Changed.

### Changed

- `go.yml` runs its job in `public.ecr.aws/docker/library/golang:1.27`, Amazon's public copy
  of Docker Hub's official image, in place of `docker.io/golang:1.27`: the same image,
  with no login, and out of reach of Docker Hub's pull limit for anonymous users. A
  repository that calls `go.yml` pulls it from there.
- The module requires `github.com/qoryai/forager` in place of `github.com/qoryai/runner`,
  and `conformance.Credential` checks an answer against Forager's `credential.schema.json`,
  in `contracts/forager/v1`. Its refusal of an answer that schema refuses reads
  `credential: the gateway's schema refuses the answer: …`, where it read
  `credential: the runner's schema refuses the answer: …`.
- `go.yml` keeps Go's build cache between runs, next to the module cache: it restores the
  newest one saved for the same Go version, preferring the same `go.sum`, and each passing
  run saves its own.

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

[Unreleased]: https://github.com/qoryai/integrations/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/qoryai/integrations/releases/tag/v0.2.0
[0.1.0]: https://github.com/qoryai/integrations/releases/tag/v0.1.0
