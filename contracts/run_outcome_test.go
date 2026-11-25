package contracts_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/forager/event"
	"github.com/qoryai/integrations/contracts"
)

// endCodes are the codes of [contracts.EndCode], each with the cause that
// outcome.schema.json shows for it, as a text that its description must contain.
var endCodes = map[contracts.EndCode]string{
	contracts.EndCodePushCredentialMissing:   "no credential with the purpose push",
	contracts.EndCodePushRefused:             "refused with status 403",
	contracts.EndCodePrebuiltImageTooLarge:   "larger than largest_prebuilt_image_bytes",
	contracts.EndCodePushNotRecorded:         "the control plane has no record of the push",
	contracts.EndCodeEnvironmentImageMissing: "is not in the registry",
	contracts.EndCodePushHeadChanged:         "is not the commit that the run expected",
	contracts.EndCodePushScopeRefused:        "changes a file that the session may not change",
	contracts.EndCodePushKeyFound:            "or the commits are too large to verify",
	contracts.EndCodeRunDocumentsInvalid:     "fail qory's validation",
	contracts.EndCodeHarnessInvalid:          "cannot be used: its source",
	contracts.EndCodeMachineSetupInvalid:     "this machine has no registry",
	contracts.EndCodeTargetFetchFailed:       "could not fetch the target",
	contracts.EndCodeRepositoryCheckFailed:   "step on the run's repository failed",
	contracts.EndCodeForgeUnreachable:        "could not reach the forge",
	contracts.EndCodeHistoryRewritten:        "HEAD of the run's working folder does not descend",
	contracts.EndCodeRunAPIFailed:            "to the run API failed",
	contracts.EndCodePrebuildFailed:          "the pull of its base image",
	contracts.EndCodeNextSessionFailed:       "Qory Apiary refused it or did not answer",
	contracts.EndCodeOutputsInvalid:          "an output does not match its output type",
}

// schemaEndCodes reads $defs end_codes of outcome.schema.json: the definition, and the
// description of each const of its list.
func schemaEndCodes(t *testing.T) (def map[string]any, entries map[string]string) {
	t.Helper()
	doc, err := contracts.RunDocument("outcome.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	def = doc.(map[string]any)["$defs"].(map[string]any)["end_codes"].(map[string]any)
	entries = map[string]string{}
	list, _ := def["oneOf"].([]any)
	for _, e := range list {
		m := e.(map[string]any)
		code, _ := m["const"].(string)
		text, _ := m["description"].(string)
		if _, dup := entries[code]; dup {
			t.Errorf("the list of end codes has %q twice", code)
		}
		entries[code] = text
	}
	return def, entries
}

// withReason returns an outcome document with the state and the reason.
func withReason(state string, reason any) map[string]any {
	return map[string]any{"state": state, "reason": reason}
}

// TestTheEndCodesAreTheSchemasList pins that the list of end codes in
// outcome.schema.json and the constants of [contracts.EndCode] are the same set, that
// each entry has a cause in plain words, that each code has the pattern of a reason of
// Forager, that no code is one of Forager's reserved reasons, which Forager drops from
// a starter's outcome, and that no cause contains advice or an action: no word of
// contact, support, asking or checking, and no instruction to the reader. The words
// "output check" and "scope check", and the key `scope_checks`, are the exceptions: each
// is a check or its name, not an instruction.
func TestTheEndCodesAreTheSchemasList(t *testing.T) {
	_, entries := schemaEndCodes(t)
	pattern := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	// namedChecks are the checks that a cause may contain, as whole words: "checks" and
	// "checking" stay banned.
	namedChecks := regexp.MustCompile(`\b(output|scope) check\b|\bscope_checks\b`)
	for code, text := range entries {
		if _, ok := endCodes[contracts.EndCode(code)]; !ok {
			t.Errorf("the schema lists %q, and no Go constant has it", code)
		}
		if !pattern.MatchString(code) {
			t.Errorf("the end code %q is not a code of the pattern %s", code, pattern)
		}
		if event.Reserved(code) || !event.StarterReason(code) {
			t.Errorf("the end code %q is a reserved reason of Forager, or not a starter's reason", code)
		}
		if strings.TrimSpace(text) == "" {
			t.Errorf("the end code %q has no cause", code)
		}
		// The output check, the scope check and the key scope_checks are not instructions.
		lower := namedChecks.ReplaceAllString(strings.ToLower(text), "")
		for _, banned := range []string{"contact", "support", "ask ", "check", "should", "please", "try ", "make ", "raise ", "retry", "again"} {
			if strings.Contains(lower, banned) {
				t.Errorf("the cause of %q contains %q: %s", code, banned, text)
			}
		}
	}
	for code, cause := range endCodes {
		text, ok := entries[string(code)]
		if !ok {
			t.Errorf("the Go constant %q is not in the schema's list", code)
			continue
		}
		if !strings.Contains(text, cause) {
			t.Errorf("the cause of %q does not contain %q: %s", code, cause, text)
		}
	}
	if len(entries) != len(endCodes) {
		t.Errorf("the schema lists %d end codes, and the Go constants are %d", len(entries), len(endCodes))
	}
}

// TestTheReasonIsOpenOnTheWire pins that the list of end codes does not close reason: a
// reason of the pattern that is not in the list passes with each state, as do Forager's
// reserved reasons, and a reason that is no code is refused.
func TestTheReasonIsOpenOnTheWire(t *testing.T) {
	schema := compileRun(t, runSchema("outcome"))
	for _, state := range []string{"succeeded", "failed", "cancelled"} {
		for _, reason := range []string{"push_exploded", "push", "push_refused_", "timeout", "gateway_lost", "a", "a" + strings.Repeat("b", 63)} {
			if err := schema.Validate(withReason(state, reason)); err != nil {
				t.Errorf("state %s with the reason %q is refused: %v", state, reason, err)
			}
		}
	}
	for _, reason := range []any{
		"PUSH_REFUSED", "Push_refused", "push-refused", "push refused", " push_refused", "push_refused ",
		"", "1push", "_push", "a" + strings.Repeat("b", 64), json.Number("1"), nil, true,
	} {
		if schema.Validate(withReason("failed", reason)) == nil {
			t.Errorf("the reason %#v is accepted", reason)
		}
	}
	if err := schema.Validate(map[string]any{"state": "failed"}); err != nil {
		t.Errorf("an outcome with no reason is refused: %v", err)
	}
}

// TestAnEndCodeIsOnlyWithStateFailed pins that an outcome whose reason is one of the end
// codes is accepted with state failed, for each code, and refused with state succeeded
// or cancelled, and that an outcome with no reason is accepted with each state.
func TestAnEndCodeIsOnlyWithStateFailed(t *testing.T) {
	schema := compileRun(t, runSchema("outcome"))
	for _, state := range []string{"succeeded", "failed", "cancelled"} {
		if err := schema.Validate(map[string]any{"state": state}); err != nil {
			t.Errorf("an outcome with state %s and no reason is refused: %v", state, err)
		}
		for code := range endCodes {
			err := schema.Validate(withReason(state, string(code)))
			if (err == nil) != (state == "failed") {
				t.Errorf("state %s with the end code %q: %v", state, code, err)
			}
		}
	}
	if schema.Validate(map[string]any{"reason": "push_refused"}) == nil {
		t.Error("a reason with no state is accepted")
	}
}

// TestTheOutcomeHasNoAdviceMember pins that the outcome has the members state, reason,
// exit_code and must_understand and no other, in the schema and in [contracts.RunOutcome],
// and that the schema ignores any other member, advice and action texts and end_code among
// them, and the closed copy refuses it.
func TestTheOutcomeHasNoAdviceMember(t *testing.T) {
	doc, err := contracts.RunDocument("outcome.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	schemaDoc := doc.(map[string]any)
	want := []string{"exit_code", "must_understand", "reason", "state"}
	if got := slices.Sorted(maps.Keys(schemaDoc["properties"].(map[string]any))); !slices.Equal(got, want) {
		t.Errorf("the schema has the members %v, want %v", got, want)
	}
	if _, ok := schemaDoc["additionalProperties"]; ok {
		t.Error("the schema sets additionalProperties, and a reader ignores a member that it does not define")
	}
	if !slices.Equal(schemaDoc["required"].([]any), []any{"state"}) {
		t.Errorf("the schema requires %v, want state alone", schemaDoc["required"])
	}
	var tags []string
	for f := range reflect.TypeFor[contracts.RunOutcome]().Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		tags = append(tags, name)
	}
	slices.Sort(tags)
	if !slices.Equal(tags, want) {
		t.Errorf("RunOutcome has the members %v, want %v", tags, want)
	}
	schema := compileRun(t, runSchema("outcome"))
	closed, err := contracts.CompileRunClosed("outcome.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"message", "action", "advice", "text", "hint", "support", "end_code"} {
		doc := map[string]any{"state": "failed", "reason": "push_refused", member: "text"}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("an outcome with the member %s is refused: %v", member, err)
		}
		if closed.Validate(doc) == nil {
			t.Errorf("the closed copy accepts an outcome with the member %s", member)
		}
	}
}

// TestTheOutcomeTakesReasonAndExitCodeFromForager pins that reason and exit_code of the
// outcome refer to the properties of the same names of Forager's dev.qory.run.exited, so
// their values follow that event, and that state is succeeded, failed or cancelled.
func TestTheOutcomeTakesReasonAndExitCodeFromForager(t *testing.T) {
	doc, err := contracts.RunDocument("outcome.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	props := doc.(map[string]any)["properties"].(map[string]any)
	for _, member := range []string{"reason", "exit_code"} {
		want := "https://qory.dev/contracts/forager/v1/events/run.exited.schema.json#/properties/" + member
		if got := props[member].(map[string]any)["$ref"]; got != want {
			t.Errorf("%s refers to %v, want %s", member, got, want)
		}
	}
	if got := props["state"].(map[string]any)["enum"]; !reflect.DeepEqual(got, []any{"succeeded", "failed", "cancelled"}) {
		t.Errorf("state is one of %v", got)
	}
	schema := compileRun(t, runSchema("outcome"))
	for name, c := range map[string]struct {
		doc map[string]any
		ok  bool
	}{
		"exit code -1":  {map[string]any{"state": "failed", "exit_code": json.Number("-1")}, true},
		"exit code 0":   {map[string]any{"state": "succeeded", "exit_code": json.Number("0")}, true},
		"exit code -2":  {map[string]any{"state": "failed", "exit_code": json.Number("-2")}, false},
		"exit code 1.5": {map[string]any{"state": "failed", "exit_code": json.Number("1.5")}, false},
		"state paused":  {map[string]any{"state": "paused"}, false},
	} {
		if err := schema.Validate(c.doc); (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", name, err, c.ok)
		}
	}
}

// TestTheOutcomeFixturesCoverEachCode pins that fixtures/outcome has a document with
// state failed for each end code, one for a reason that is not in the list, one with
// state cancelled and Forager's reason timeout, and one with no reason, and that each
// valid fixture reads into [contracts.RunOutcome] and is written back as the same JSON
// value.
func TestTheOutcomeFixturesCoverEachCode(t *testing.T) {
	seen := map[string]bool{}
	none, unknown, timeout := 0, 0, 0
	for _, f := range runFiles(t, "fixtures/outcome") {
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var o contracts.RunOutcome
		if err := json.Unmarshal(b, &o); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		_, listed := endCodes[contracts.EndCode(o.Reason)]
		switch {
		case o.Reason == "":
			none++
		case listed:
			if o.State != "failed" {
				t.Errorf("%s has an end code with state %s", f, o.State)
			}
			seen[o.Reason] = true
		case o.Reason == "timeout" && o.State == "cancelled":
			timeout++
		case o.State == "failed":
			unknown++
		}
		out, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		var want, back any
		if err := json.Unmarshal(b, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, want) {
			t.Errorf("%s is written back as %s", path.Base(f), out)
		}
		if err := validate(compileRun(t, runSchema("outcome")), out); err != nil {
			t.Errorf("%s is written as %s, which the schema refuses: %v", path.Base(f), out, err)
		}
	}
	for code := range endCodes {
		if !seen[string(code)] {
			t.Errorf("fixtures/outcome has no failed outcome with the end code %q", code)
		}
	}
	if none == 0 || unknown == 0 || timeout == 0 {
		t.Errorf("fixtures/outcome has %d outcomes with no reason, %d failed with a code not in the list and %d cancelled with timeout, want each above 0", none, unknown, timeout)
	}
}

// TestTheOutcomeTypeMarshalsToItsSchema pins that [contracts.RunOutcome] writes an exit
// code of 0 and leaves a nil one out, leaves out an empty reason, writes an [EndCode] set
// as the reason, and that each value written is accepted by the schema.
func TestTheOutcomeTypeMarshalsToItsSchema(t *testing.T) {
	zero, minusOne := 0, -1
	for _, c := range []struct {
		v    contracts.RunOutcome
		want string
	}{
		{contracts.RunOutcome{State: "succeeded", ExitCode: &zero}, `{"state":"succeeded","exit_code":0}`},
		{contracts.RunOutcome{State: "failed"}, `{"state":"failed"}`},
		{contracts.RunOutcome{State: "failed", Reason: "image_missing", ExitCode: &minusOne}, `{"state":"failed","reason":"image_missing","exit_code":-1}`},
		{contracts.RunOutcome{State: "failed", Reason: string(contracts.EndCodePushRefused)}, `{"state":"failed","reason":"push_refused"}`},
	} {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != c.want {
			t.Errorf("%+v is written as %s, want %s", c.v, b, c.want)
		}
		if err := validate(compileRun(t, runSchema("outcome")), b); err != nil {
			t.Errorf("%s is refused: %v", b, err)
		}
	}
}

// TestAnInvalidOutcomeFixtureIsRefusedForOneDefectAlone pins that each invalid outcome
// fixture is refused for its defect alone: the same document with the defect repaired is
// accepted.
func TestAnInvalidOutcomeFixtureIsRefusedForOneDefectAlone(t *testing.T) {
	failed := func(m map[string]any) { m["state"] = "failed" }
	repairs := map[string]func(m map[string]any){
		"outcome-end-code-succeeded.json":                  failed,
		"outcome-end-code-cancelled.json":                  failed,
		"outcome-environment-image-missing-succeeded.json": failed,
		"outcome-environment-image-missing-cancelled.json": failed,
		"outcome-push-head-changed-succeeded.json":         failed,
		"outcome-push-head-changed-cancelled.json":         failed,
		"outcome-push-scope-refused-succeeded.json":        failed,
		"outcome-push-scope-refused-cancelled.json":        failed,
		"outcome-push-key-found-succeeded.json":            failed,
		"outcome-push-key-found-cancelled.json":            failed,
		"outcome-run-documents-invalid-succeeded.json":     failed,
		"outcome-run-documents-invalid-cancelled.json":     failed,
		"outcome-harness-invalid-succeeded.json":           failed,
		"outcome-harness-invalid-cancelled.json":           failed,
		"outcome-machine-setup-invalid-succeeded.json":     failed,
		"outcome-machine-setup-invalid-cancelled.json":     failed,
		"outcome-target-fetch-failed-succeeded.json":       failed,
		"outcome-target-fetch-failed-cancelled.json":       failed,
		"outcome-repository-check-failed-succeeded.json":   failed,
		"outcome-repository-check-failed-cancelled.json":   failed,
		"outcome-forge-unreachable-succeeded.json":         failed,
		"outcome-forge-unreachable-cancelled.json":         failed,
		"outcome-history-rewritten-succeeded.json":         failed,
		"outcome-history-rewritten-cancelled.json":         failed,
		"outcome-run-api-failed-succeeded.json":            failed,
		"outcome-run-api-failed-cancelled.json":            failed,
		"outcome-prebuild-failed-succeeded.json":           failed,
		"outcome-prebuild-failed-cancelled.json":           failed,
		"outcome-next-session-failed-succeeded.json":       failed,
		"outcome-next-session-failed-cancelled.json":       failed,
		"outcome-outputs-invalid-succeeded.json":           failed,
		"outcome-outputs-invalid-cancelled.json":           failed,
		"outcome-reason-empty.json":                        func(m map[string]any) { delete(m, "reason") },
		"outcome-reason-upper-case.json":                   func(m map[string]any) { m["reason"] = "push_exploded" },
		"outcome-without-state.json":                       failed,
		"outcome-state-unknown.json":                       failed,
		"outcome-exit-code-below-minus-1.json":             func(m map[string]any) { m["exit_code"] = json.Number("-1") },
	}
	schema := compileRun(t, runSchema("outcome"))
	n := 0
	for _, f := range runFiles(t, "fixtures/invalid") {
		name := path.Base(f)
		if !strings.HasPrefix(name, "outcome-") {
			continue
		}
		n++
		repair, ok := repairs[name]
		if !ok {
			t.Errorf("%s has no repair", name)
			continue
		}
		doc := fixture(t, "invalid/"+name)
		if schema.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
		repair(doc)
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s with its defect repaired is refused: %v", name, err)
		}
	}
	if n != len(repairs) {
		t.Errorf("fixtures/invalid has %d outcome documents, and the test repairs %d", n, len(repairs))
	}
}
