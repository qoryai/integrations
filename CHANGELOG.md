# Changelog

Every release of the integration contract and its module, newest first, in the shape of
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The version numbers follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html); before 1.0 a minor release may
change what an existing reader or program relies on, and notes it under Upgrading.

## [Unreleased]

### Added

- Settings on standard input, the only way a program receives them. Every role is
  started as `<program> <role> -- [arguments]`, `--` always there, even when the role
  has no arguments; the credential role as `<program> credential -- <argument>`, exactly
  one argument, the run's, empty when the run gives none, and the tool role as
  `<program> tool -- <argument>`, the argument empty when the role has no `argument`.
  Standard input carries exactly one JSON document, the settings the role lists and
  nothing else, a secret's value in it as any other; `{}` when it holds none. The
  program reads standard input to its end before it acts and before any network call, and refuses empty input, anything after the first
  document but white space, and input larger than 64 KiB, 65536 bytes; the writer refuses
  a larger document before it starts the program, writes the document whole, closes
  standard input, and never connects it to a terminal. Nothing in the document is
  replaced or escaped. `describe` reads no standard input.
- `x-secret-name` on a secret, `^[A-Z][A-Z0-9_]{0,127}$`, such as
  `GITHUB_APP_PRIVATE_KEY`: the conventional name of the secret, which a control plane
  fills in as the name to store the secret under. It is a suggestion, not an identity.
  `fixtures/github.json`'s `private_key` carries it.
- Every release attaches `description.json`, what `<program> describe` prints, and
  `checksums.txt` includes its SHA-256. A control plane fetches the release's
  `description.json` to learn an integration without running it.
- README §Release rule defines an integration's source, where its releases are: a
  repository on a forge, `<host>/<path>`, such as `github.com/<owner>/<repo>`,
  `gitlab.com/<group>/<project>` or a Forgejo or Gitea `<host>/<owner>/<repo>`, or an
  HTTPS URL of a `description.json` with the release's other files beside it. The forge
  is `github` on github.com, `gitlab` on gitlab.com and `forgejo` on codeberg.org; on
  any other host it is named beside the source. It defines where each forge's release
  files download from, the files of a release, and the version, `X.Y.Z`, the
  description's `program_version`, which a reader compares exactly; on a forge the tag
  is `vX.Y.Z`.
- Ways. An integration offers one or more ways, each a role of its description: the
  credential role for an API, the tool role for an MCP server. A run's connection names
  in its `ways` the roles it uses, and the runner starts those alone. An API that needs
  only a static key needs no program and no role.
- `settings` on every credential and tool role, required: the top-level settings the
  role may receive, plain and secret alike, a secret by its `<name>`. `required`, which
  a role may have: the settings of its `settings` it needs, a secret satisfied by
  `<name>` or `<name>_file`.
- The `tool` role, defined: `argument`, which may be absent, `serves`, the hosts whose
  requests the runner sends to the tool, `mcp`, the `https://` URL of the MCP server the
  tool serves, which `qory` registers with the agent's MCP client, `placeholders`, and
  `settings` and `required`. It is started as `<program> tool -- <argument>`, reads its
  settings on standard input, and listens on the Unix socket `QORY_TOOL_LISTEN` names,
  where the runner sends it the requests it allows for `serves` as HTTP/1.1.
- `fixtures/acme-tracker-mcp.json`, a description with both ways, and invalid fixtures
  for the roles' new members and for a top-level `required` in the settings.

### Changed

- Every secret has a `title`, a string that is not only white space: its label in a form.
- A program reads no setting and no secret from its environment. A secret has one
  source: its value inline in the document under `<name>`, or `<name>_file`, the path of
  a file that contains it. A program refuses settings that contain both.
- A program takes no flags for a role, and refuses `--settings` as it refuses any flag it
  does not know.
- §Declaring an integration lists what a reader checks: it runs `<program> describe`,
  checks the settings against the description's `settings`, and expands the roles it
  knows. It no longer shows a declaration expanded into the runner's credential
  definitions with an `adapter` command line: how a run carries an integration's
  settings and secrets is the runner's contract.
- `conformance.Description` refuses a secret without a `title`, an `x-secret-name` on a
  setting that is not a secret or outside the settings' own properties, one that does not
  match its pattern, and two secrets with the same one.
- `release.yml` builds the program for the runner's own platform with the release's
  flags before it publishes, runs `<program> describe`, and fails unless that prints one
  JSON object with `version` 1 and the tag's version as `program_version`.
- `release.yml` and the README say it publishes a GitHub release. An integration on
  another forge publishes the same files with that forge's own CI.
- Each role's standard input holds the settings that role lists and nothing else; the
  runner leaves out every other setting. The runner checks each role's document in
  order: only the names the role lists, its `required` present, valid against the
  settings, and not both `<name>` and `<name>_file`.
- The settings schema's top level carries no keyword a role's subset of the settings can
  break: `required`, `allOf`, `anyOf`, `oneOf`, `not`, `if`, `then`, `else`,
  `dependentRequired`, `dependentSchemas`, `minProperties`, `maxProperties`, `$ref` or
  `$dynamicRef`. The description's schema refuses each of them. What a role needs is in
  its own `required`; another rule across settings is the program's.
- A role without `argument` ignores the run's argument and is started with an empty
  one. The run's argument is matched only against the roles that have an `argument`
  pattern.
- `tool` is defined here, no longer reserved. `work_source` and `output` stay reserved.
- Hosts are per role: no host is in both the credential role's `hosts` and the tool
  role's `serves`.
- `conformance.Description` refuses a role's `settings` that names a setting the
  settings do not define or a secret's `<name>_file`, a role's `required` that names a
  setting its `settings` does not list, a secret no role lists, a credential host and a
  tool host that overlap, the same or under a `*.` pattern, and a tool's `mcp` that is
  not an `https` URL, has userinfo, a port or a fragment, has a host that is not a
  lower-case host name, or is on a host the tool does not serve. It names each keyword
  of the settings' top level that a role's subset can break.

### Removed

- `--settings <json>` and `--settings -`, and with them the refusal of a `writeOnly`
  value on the command line and the `$` of the settings written `\u0024`.

### Upgrading

- A program drops `--settings`, reads its settings from standard input by the rules
  above, and refuses `--settings`.
- A program reads no setting and no secret from its environment, and refuses settings
  that contain both `<name>` and `<name>_file` for one secret.
- A description gives every secret a `title`, and may give it an `x-secret-name`:
  `conformance.Description` now refuses a secret without a title, so an integration's
  tests fail until each secret has one.
- A program released with `release.yml` reports the version the build sets with
  `-X main.version=X.Y.Z` as `program_version`. One that reports another version, or
  whose `describe` fails, publishes no release.
- A description adds `settings` to every credential role, and moves the settings
  schema's top-level `required` into each role's `required`, a secret by its `<name>`.
  `qory-github`'s credential role becomes `"settings": ["app_id", "installation_id",
  "api_url", "permissions", "private_key"], "required": ["app_id", "private_key"]`.
- A description lists each secret in some role's `settings`.
- A description's settings drop every top-level keyword a role's subset can break, such
  as a `oneOf` over a secret's `<name>` and `<name>_file`: the role's `required` and the
  rule against both forms say the same. `qory-github`'s settings drop their top-level
  `oneOf`.

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
