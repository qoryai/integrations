# Contributing

Thank you for considering a contribution.

## Where contributions go

- **The contract**, `contracts/integration/v1/`: a rule a reader or a program needs, or a
  role's contract once it is written. An addition keeps `version` 1; a change that breaks
  a reader is `version` 2 ([§Versions](contracts/integration/v1/README.md#versions)). Its
  schema and its fixtures change together, and the tests in `contracts/` keep them so.
- **The conformance checks**, `conformance/`: a rule of the contract a program can break
  and a test can see.
- **The workflows**, `.github/workflows/`: every integration calls them, so a change there
  reaches all of them.
- **The catalog**, the README's table: a row for an integration, yours included.

**An integration itself goes in a repository of its own**, started from the
[integration template](https://github.com/qoryai/integration-template). A fix to one of
Qory's goes to its repository, such as [qoryai/qory-github](https://github.com/qoryai/qory-github).

## Contributor Licence Agreement

Copyright in this project is held by a single owner: **8wonders GmbH, and its successors and
assigns**. To keep that true, every contribution is made under the Contributor Licence
Agreement in [CLA.md](CLA.md): a perpetual, worldwide, irrevocable licence to the
contribution, including the right to relicense it, and a patent grant on the same terms as
the Apache License, Version 2.0. You keep your copyright.

Opening a pull request against this repository is your acceptance of the agreement, for that
contribution and every later one. The pull request is the record of your acceptance. Read
[CLA.md](CLA.md) before your first pull request. A signing step on the pull request may be
added later; it will not change the terms.

The agreement names the owner with successors-and-assigns wording, so that if the
project moves into a dedicated entity, existing grants travel with it and nobody signs again.

Why a CLA at all: the licensing decisions of the project are only executable with a sole
copyright holder. Declaring it before a community exists is what makes it a kept promise
rather than a takeback.

## Licence

By contributing, you agree that your contribution is licensed under the Apache License,
Version 2.0 (see [LICENSE](LICENSE)) in addition to the CLA grant above.

## Development

The toolchain is pinned in `mise.toml`; `mise install` provides it. Go 1.27.

```sh
go build ./...
go test ./...
gofmt -l .              # must print nothing
go vet ./...
go run github.com/mgechev/revive@v1.16.0 -config revive.toml ./...
```

A test is hermetic: `t.TempDir` for files, nothing from the network, nothing from the
machine's configuration, and no fixture contains a real secret, a real account or a real
repository of anyone's.

## Doc comments

Every package and every exported name has a doc comment, and CI fails without one. The
first sentence starts with the name and is a complete sentence. Say what the code does,
including what it refuses, what it overwrites and what it leaves behind. Wrap at 90
columns.

## Releases

A release is a tag on a branch named after it, `v0.1.0`, opened as one pull request. That
branch adds the release's section to `CHANGELOG.md`, `[X.Y.Z] - YYYY-MM-DD` with the day
the tag lands; a fix that goes to `main` outside a release branch goes under
`[Unreleased]` until the next one. This repository publishes no program: its release is
the Go module's version, which every integration's `go.mod` pins.

A tag is never moved or deleted. The Go module proxy and the checksum database keep the
first tree they read for a version, for good. A mistake in a release is fixed by the next
version, and `go.mod` retracts the one that is wrong.

Commit messages state what changed and why it was needed, in the imperative.
