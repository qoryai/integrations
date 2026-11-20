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

A reader ignores a member that the schema of a document does not define, at any depth.
The rule covers the run documents, the next-session answer, the next-session request and
the outcome. It applies at the top level, and in `subject`, `run_settings`, `submit`,
`harness`, `model_choice` and each other object inside them.

- `must_understand` lists the members that the reader must understand. Each is a JSON
  Pointer ([RFC 6901](https://www.rfc-editor.org/rfc/rfc6901)) counted from the document,
  such as `/run_settings/full_suite` or `/subject/kind`. Write a pointer once.
- A reader refuses a document whose `must_understand` lists a pointer to a member that
  it does not define. The schema accepts a pointer to a member that the schema defines.
- Another schema defines the members below `inputs` and below
  `harness.harness.extensions`. The schema accepts any pointer there.
- The control plane refuses a request whose `must_understand` lists a pointer that it does
  not define, with 400 `invalid_request`. The contract defines no answer for an outcome
  with such a list, so the control plane refuses it as it refuses any outcome that the
  schema refuses.
- A reader refuses a bad value of a member that it defines.
- `minProperties` counts members, defined or not. A harness selection with only an
  undefined member reads as empty: `exclude: {"zz": 1}` leaves nothing out, and
  `only: {"zz": 1}` takes nothing, because `only` leaves out everything it does not list.
  Go writes such a selection back as `{}`, which the schema refuses, because
  `minProperties` is 1.

Examples:

- A run document has a top-level member that an older qory does not define. The older
  qory ignores the member and runs as before.
- A run document has `/subject/language`, a member of the `subject` that an older machine
  does not define. The older machine ignores it.
- The run document lists `/subject/language` in `must_understand`. The older qory refuses
  the run documents, and the run ends with `run_documents_invalid`.

A control plane's tests check its documents against a closed copy of the schema. The
closed copy refuses a member that the schema does not define, so a misspelled member
fails the test. It covers each object of the schema and of the integration contract that
it reaches by reference. `contracts.CompileRunClosed` returns the closed copy of
`documents.schema.json`, `next-session-answer.schema.json`,
`next-session-request.schema.json` or `outcome.schema.json`.

`contracts.CheckMustUnderstand` applies the rule to a document that a reader decodes into
a Go type. It resolves each pointer through the `json` tags of the type.

## Next-session refusals

`next-session-refusal.schema.json` has nine codes. An unsigned 401 or 415 from Qory
Apiary is outside them, because the gateway always signs its requests and sends JSON.
