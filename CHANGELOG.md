# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

### Added

- Settings on standard input: `<program> <role> --settings - -- [arguments]` reads the
  settings from standard input, exactly one JSON document, the whole settings object, a
  secret's value in it as any other. The program reads standard input to its end before
  it acts and before any network call, and refuses empty input, anything after the first
  document but white space, and input larger than 64 KiB, 65536 bytes; the writer refuses
  a larger document before it starts the program, writes the document whole, and closes
  standard input. Nothing in the document is replaced or escaped. `--settings <json>` on
  the command line stays as it is, and refuses a `writeOnly` value as before. A runner
  that supports standard input starts the program with
  `[<program>, credential, --settings, -, --, "${argument}"]` and writes the settings
  document to its standard input; how a definition gives it that document is the
  runner's contract,
  [§Credentials](https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials).
- `x-secret-name` on a secret, `^[A-Z][A-Z0-9_]{0,127}$`, such as
  `GITHUB_APP_PRIVATE_KEY`: the conventional name of the secret, which a control plane
  fills in as the name to store the secret under. It is a suggestion, not an identity.
  `fixtures/github.json`'s `private_key` carries it.
- Every release attaches `description.json`, what `<program> describe` prints, and
  `checksums.txt` includes its SHA-256. A control plane fetches it from
  `https://github.com/<owner>/<repo>/releases/download/vX.Y.Z/description.json` to learn
  an integration without running it.
- README §Release rule defines an integration's source, the repository that holds its
  releases, written `github.com/<owner>/<repo>`, and its version, `X.Y.Z`, the tag
  without its `v`, which `describe` reports as `program_version` and a reader compares
  exactly.

### Changed

- Every secret has a `title`, a string that is not only white space: its label in a form.
- A program reads no setting and no secret from its environment, and refuses settings
  that contain both `<name>` and `<name>_file` for one secret, on the command line and on
  standard input.
- `conformance.Description` refuses a secret without a `title`, an `x-secret-name` on a
  setting that is not a secret or outside the settings' own properties, one that does not
  match its pattern, and two secrets with the same one.
- `release.yml` builds the program for the runner's own platform with the release's
  flags before it publishes, runs `<program> describe`, and fails unless that prints one
  JSON object with `version` 1 and the tag's version as `program_version`.

### Upgrading

- A program accepts `--settings -` and reads its settings from standard input by the
  rules above.
- A program reads no setting and no secret from its environment, and refuses settings
  that contain both `<name>` and `<name>_file` for one secret, in either form.
- A description gives every secret a `title`, and may give it an `x-secret-name`:
  `conformance.Description` now refuses a secret without a title, so an integration's
  tests fail until each secret has one.
- A program released with `release.yml` reports the version the build sets with
  `-X main.version=X.Y.Z` as `program_version`. One that reports another version, or
  whose `describe` fails, publishes no release.

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
  plays, `credential` with the argument and the hosts of a runner definition. A control
  plane offers a workspace the integrations whose `domains` include the workspace's domain
  name exactly, and those whose description has no `domains` in every domain. Every role
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
  and arm64, and publishes the release every reader installs by one rule, README
  §Release rule.
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
