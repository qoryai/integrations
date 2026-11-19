# Run contract, v1

What the control plane tells qory about a run, and what qory reports back. The files in
this directory are the schemas and the fixtures that a writer and a reader are tested
against.

| Schema | Document |
|---|---|
| `documents.schema.json` | the run's documents, which the machine receives with the run key |
| `next-session-request.schema.json` | the request that qory sends after a session ends |
| `next-session-answer.schema.json` | the next session of the run, or the end of the run |
| `next-session-refusal.schema.json` | the control plane's coded refusal of the request |
| `outcome.schema.json` | how the run ended |

## Members a reader does not define

A reader ignores a member of the run documents, or of the next-session answer, that the
schema does not define. That holds for the members at the top level of each document and
for the members of the `subject`.

- `must_understand` lists top-level members that the reader must understand, each name
  once. A reader refuses a document whose `must_understand` lists a member that the
  reader does not define.
- A reader refuses a bad value of a member that it defines.
- The other objects stay closed, such as `run_settings`, `submit` and `harness`. A reader
  refuses a member that the schema does not define there.

Examples:

- A run document has a top-level member that an older qory does not define. The older
  qory ignores the member and runs as before.
- A run document has a member on the `subject` that an older machine does not define.
  The older machine ignores it.
- The same top-level member is listed in `must_understand`. The older qory refuses the
  run documents, and the run ends with `run_documents_invalid`.

A control plane's tests check its documents against a closed copy of the schema, which
refuses a member that the schema does not define, so a misspelled member fails the test.
`contracts.CompileRunClosed` returns the closed copy. `contracts.CheckMustUnderstand`
applies the rule to a document that a reader decodes into a Go type.

## Next-session refusals

`next-session-refusal.schema.json` has nine codes. An unsigned 401 or 415 from Qory
Apiary is outside them. The gateway always signs its requests and sends JSON, so Qory
Apiary does not answer the next-session request with either.
