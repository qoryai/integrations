# The submit program

A submit program opens or updates the draft of a run's submission on a forge, and writes
the forge's address of each commit. A draft is a pull request or a merge request that
people can still change before it is ready. qory runs the program only on a run that has
no run code: a person starts `qory run`, and the program uses the person's login of the
forge's command line, such as `gh`. A run that qory starts from a run code runs no
program: the forge's adapter puts the submission on the forge from the change feed
(`contracts/integration/v1/changes.schema.json`, the `submission` entry).

The contract has plain inputs and plain lines back. The program needs no JSON tool.

## Inputs

qory runs the program once for each repository of the submission, with no argument, and
sets these environment variables. Each value is on one line, except where the table
shows another form.

| Variable | Value |
|---|---|
| `QORY_SUBMIT_SYSTEM` | the host name of the repository's system, such as `code.example.com` |
| `QORY_SUBMIT_PATH` | the repository's path on the system, such as `acme/shop` |
| `QORY_SUBMIT_BRANCH` | the branch that the run pushed, `qory/` and the work item's public id: the head of the draft |
| `QORY_SUBMIT_BASE_BRANCH` | the branch that the draft is based on. Unset when qory did not read it; a program that has `QORY_SUBMIT_DRAFT` `open` or `update` and no base branch exits with status 2 |
| `QORY_SUBMIT_BASE_COMMIT` | the commit that the run started at, 40 or 64 lower-case hexadecimal digits |
| `QORY_SUBMIT_COMMITS` | the run's commits from the base to the head, oldest first: full commit ids joined by single spaces, 1 to 100 |
| `QORY_SUBMIT_DRAFT` | `open`, `update` or `none` |
| `QORY_SUBMIT_DRAFT_NUMBER` | the number of the draft to update. Set exactly when `QORY_SUBMIT_DRAFT` is `update` |
| `QORY_SUBMIT_TITLE` | the title of the draft, 1 to 256 characters on one line. Unset when `QORY_SUBMIT_DRAFT` is `none` |
| `QORY_SUBMIT_PREVIOUS_TITLE` | the title of the previous submission version of the work item. Unset on the first version, and when that version had no text |
| `QORY_SUBMIT_DESCRIPTION_FILE` | the path of a UTF-8 file with the run's text for the description, at most 20000 characters, over several lines. Unset when `QORY_SUBMIT_DRAFT` is `none` |

The description is a file because it has several lines. Standard input is empty and
closed (`/dev/null`): a program that starts `gh` or `git` passes its standard input on,
and the first command that reads it would take the text. `gh pr create` and `gh pr edit`
read a body from a file with `--body-file`.

With `QORY_SUBMIT_DRAFT` `none`, as after a session that did not pass, the program makes no
draft and prints only the address of each commit.

## Credentials

The program relies on the forge command line's login of the person who runs qory, never
on a relay. qory gives the program no credential, no token and no proxy address, and no
input in the table of inputs is a secret. The program reads no credential from qory.

## The result lines

The program prints its result on standard output, one line for each fact, each ending in
a line feed. The line is a key, a space and the value; the value is the rest of the
line.

| Key | Value |
|---|---|
| `draft_number` | the draft's number: digits only, 1 to 9 of them, 1 or more |
| `draft_url` | the draft's address |
| `draft_title` | the draft's title after the call, 1 to 256 characters |
| `draft_state` | `draft` while the forge shows the draft as not ready for review, `open` once it shows it as ready |
| `draft_action` | `opened`, `updated`, or `kept` when the program changed nothing |
| `commit_url` | a commit id from `QORY_SUBMIT_BASE_COMMIT` or `QORY_SUBMIT_COMMITS`, a space, and the forge's address of that commit |

An address is an `https` address of at most 2048 characters, on the host in
`QORY_SUBMIT_SYSTEM`, with no user information, port, query or fragment, and with a path.
qory checks no more of its form, so the same program contract serves a forge whose
addresses end in `/pull/<number>` and one whose addresses end in
`/-/merge_requests/<number>`.

qory records the draft only when all five `draft_` lines are there, once each, and valid.
It records a `commit_url` line for each commit that it gave the program, once. A line
that is not valid is left out, and the lines around it count. These lines are not valid:

- a key that the table does not list, or a key with no value;
- a line with a control character, or a last line with no line feed;
- a `draft_` key that is there already, or a commit that has an address already;
- a `commit_url` for a commit that qory did not give the program;
- a value that breaks its rule in the table.

A blank line is left out and is not an error. Diagnostics go to standard error.

## Exit status

| Exit | What qory records |
|---|---|
| 0 | the program finished: the draft and the addresses from the valid lines |
| 1 | a call to the forge failed: the valid lines, and for a draft that they do not make, `program-failed` with `exit` 1 |
| 2 | the program could not use its inputs: the valid lines, and `program-failed` with `exit` 2 for a draft that they do not make |
| another status, or the program did not start | nothing from the lines; the draft is `program-failed` with `exit` 1 |

A program that finishes with exit 0 and prints no draft lines for a repository whose
`QORY_SUBMIT_DRAFT` is `open` or `update` leaves the draft as it was: `no-program`.

## What qory prints

qory prints at most 20 lines of the program's standard error, each masked of secrets and
prefixed `qory run: submit program: `. It does not print the result lines, so a
repository of 100 commits does not take the 20 lines.

## The description's block

The run's text sits in the description of the draft between two marker lines, each alone
on its line:

```text
<!-- qory:start -->
The page lists open orders.
<!-- qory:end -->
```

The program writes the draft's description as follows. It reads the draft's description
from the forge first.

1. For an `open`, the description is the block: the start marker, the text of
   `QORY_SUBMIT_DESCRIPTION_FILE`, the end marker.
2. For an `update`, if the description has a start marker line and, after it, an end
   marker line, the program replaces the first such pair, and the lines between them,
   with the new block, and keeps every other line. Spaces, tabs and a carriage return
   after a marker count as part of the marker line; the ones after the end marker stay.
3. For an `update`, if the description has no start marker followed by an end marker, as
   when a person removed one, the program keeps the description whole and adds a blank
   line and the new block at the end.

`contracts.MarkedDescription` is the same rule in Go, and `fixtures/description/` contains
plain-text cases: for each case, the draft's description before (`existing.md`), the text
(`text.md`) and the description after (`result.md`).

## The draft's title

On an `update`, the program changes the draft's title to `QORY_SUBMIT_TITLE` only while the
title that the forge shows is the same as `QORY_SUBMIT_PREVIOUS_TITLE`. A title that a
person edited stays. On an `open`, the draft has `QORY_SUBMIT_TITLE`. `contracts.NextTitle`
is the same rule in Go.

## Examples

A repository on a forge whose draft is a pull request, with the program given
`QORY_SUBMIT_DRAFT=open`, prints
[`fixtures/output/github-like.txt`](fixtures/output/github-like.txt):

```text
draft_number 34
draft_url https://code.example.com/acme/shop/pull/34
draft_title Show delivery dates at checkout
draft_state draft
draft_action opened
commit_url 3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1 https://code.example.com/acme/shop/commit/3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1
commit_url 9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c https://code.example.com/acme/shop/commit/9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c
```

A repository on a forge whose draft is a merge request prints
[`fixtures/output/gitlab-like.txt`](fixtures/output/gitlab-like.txt), with `-/` in its
addresses. The directory `fixtures/output/` also holds `invalid-` files, each with the
line that qory leaves out, and the test checks the reason for each.

## Fixtures

| Path | Contains | Checked by |
|---|---|---|
| `fixtures/output/*.txt` | result lines that qory records whole | `contracts.ParseSubmitLines` |
| `fixtures/output/invalid-*.txt` | result lines with a line that qory leaves out | `contracts.ParseSubmitLines` |
| `fixtures/description/<case>/` | a draft's description before and after an update | `contracts.MarkedDescription` |
