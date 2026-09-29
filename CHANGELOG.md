# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

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

[Unreleased]: https://github.com/qoryai/integrations/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/qoryai/integrations/releases/tag/v0.1.0
