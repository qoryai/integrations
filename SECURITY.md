# Security

## Reporting a vulnerability

Write to **info@8wonders.de**. Do not open a public issue or pull request for it.

Say what you found, the version, and how to see it happen: the document, the command or
the test, with every secret taken out, and what it did that it should not.

You get an answer within three working days. We tell you what we found, fix what is a
vulnerability in a new release, and publish an advisory that credits you unless you
would rather it did not.

## Supported versions

The latest release. Below 1.0 a fix is a new release and is not carried back to an
earlier one.

## What is a vulnerability here

- The contract's schema accepts a description that puts a secret on a command line: a
  `writeOnly` property a reader cannot find among the settings' `properties`.
- `conformance` passes a program's output that breaks a rule it checks, such as a
  failure that prints on standard output.
- `release.yml` publishes an archive other than the one it built, or a `checksums.txt`
  that does not match the archives.

## What is not

- A vulnerability in an integration: report it to that integration's repository, such as
  [qoryai/qory-github](https://github.com/qoryai/qory-github/blob/main/SECURITY.md).
- The runner, and what it does with a credential role's answer, are the
  [runner's](https://github.com/qoryai/runner/blob/main/SECURITY.md).

If you are not sure which side something falls on, write anyway.
