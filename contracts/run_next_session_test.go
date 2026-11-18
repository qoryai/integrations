package contracts_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// fixture reads a JSON fixture of the run contract as a map, or fails the test.
func fixture(t *testing.T, name string) map[string]any {
	t.Helper()
	doc, err := contracts.RunDocument(path.Join("fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object", name)
	}
	return m
}

// TestAnAnswerIsOneOfTwoShapes pins the check of the next-session answer: an answer
// for a next session and an end pass, and an answer with the members of both shapes,
// or with the members of one shape and the next of the other, fails.
func TestAnAnswerIsOneOfTwoShapes(t *testing.T) {
	schema := compileRun(t, runSchema("next-session-answer"))
	session := fixture(t, "next-session-answer/session.json")
	end := fixture(t, "next-session-answer/end.json")
	for name, doc := range map[string]map[string]any{"session": session, "end": end} {
		if err := schema.Validate(doc); err != nil {
			t.Errorf("the answer %s is refused: %v", name, err)
		}
	}
	both := maps.Clone(session)
	both["next"] = "end"
	other := maps.Clone(end)
	other["next"] = "session"
	merged := maps.Clone(session)
	maps.Copy(merged, end)
	for name, doc := range map[string]map[string]any{
		"the session members with next end":     both,
		"the end with next session":             other,
		"the session members with next as end":  merged,
		"the end with one session member":       {"next": "end", "full_suite": true},
		"the session shape with no next member": withoutKey(session, "next"),
	} {
		if schema.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
	}
}

// withoutKey returns a copy of m without key.
func withoutKey(m map[string]any, key string) map[string]any {
	c := maps.Clone(m)
	delete(c, key)
	return c
}

// TestAnInvalidNextSessionFixtureIsRefusedForOneDefectAlone pins that each invalid
// next-session fixture is refused for its defect alone: the same document with the
// defect repaired is accepted.
func TestAnInvalidNextSessionFixtureIsRefusedForOneDefectAlone(t *testing.T) {
	session := fixture(t, "next-session-answer/session.json")
	repairs := map[string]func(m map[string]any){
		"next-session-answer-end-with-workflow.json": func(m map[string]any) { delete(m, "workflow") },
		"next-session-answer-both-shapes.json": func(m map[string]any) {
			for k := range m {
				if k != "next" {
					delete(m, k)
				}
			}
		},
		"next-session-answer-session-without-time-limit-minutes.json": func(m map[string]any) { m["time_limit_minutes"] = json.Number("60") },
		"next-session-answer-session-without-full-suite.json":         func(m map[string]any) { m["full_suite"] = false },
		"next-session-answer-session-without-workflow.json":           func(m map[string]any) { m["workflow"] = "review-changes" },
		"next-session-answer-session-without-inputs.json":             func(m map[string]any) { m["inputs"] = session["inputs"] },
		"next-session-answer-session-0.json":                          func(m map[string]any) { m["session"] = json.Number("2") },
		"next-session-answer-time-limit-minutes-14.json":              func(m map[string]any) { m["time_limit_minutes"] = json.Number("15") },
		"next-session-answer-time-limit-minutes-1441.json":            func(m map[string]any) { m["time_limit_minutes"] = json.Number("1440") },
		"next-session-answer-model-credential-upper-case.json":        func(m map[string]any) { m["model_credential"] = "example-provider-1" },
		"next-session-answer-inputs-without-version.json":             func(m map[string]any) { m["inputs"] = session["inputs"] },
		"next-session-answer-undefined-member.json":                   func(m map[string]any) { delete(m, "verdict") },
		"next-session-answer-next-unknown.json":                       func(m map[string]any) { m["next"] = "session" },
		"next-session-answer-without-next.json":                       func(m map[string]any) { m["next"] = "session" },
		"next-session-answer-model-choice-runtime-number.json":        func(m map[string]any) { m["model_choice"].(map[string]any)["runtime"] = "claude" },
		"next-session-answer-model-choice-runtime-upper-case.json":    func(m map[string]any) { m["model_choice"].(map[string]any)["runtime"] = "claude" },
		"next-session-answer-model-choice-runtime-effort-only.json": func(m map[string]any) {
			m["model_choice"].(map[string]any)["model"] = "example-large"
			m["model_choice"].(map[string]any)["provider_models"] = session["model_choice"].(map[string]any)["provider_models"]
		},
		"next-session-answer-model-choice-effort-without-model.json": func(m map[string]any) { m["model_choice"] = session["model_choice"] },
		"next-session-answer-model-choice-undefined-member.json": func(m map[string]any) {
			delete(m["model_choice"].(map[string]any), "time_limit_minutes")
		},
		"next-session-request-verdict.json":                             func(m map[string]any) { delete(m, "verdict") },
		"next-session-request-session-0.json":                           func(m map[string]any) { m["session"] = json.Number("1") },
		"next-session-request-session-string.json":                      func(m map[string]any) { m["session"] = json.Number("1") },
		"next-session-request-state-paused.json":                        func(m map[string]any) { m["state"] = "failed" },
		"next-session-request-state-unknown.json":                       func(m map[string]any) { m["state"] = "succeeded" },
		"next-session-request-without-reason.json":                      func(m map[string]any) { m["reason"] = nil },
		"next-session-request-reason-upper-case.json":                   func(m map[string]any) { m["reason"] = nil },
		"next-session-request-undefined-member.json":                    func(m map[string]any) { delete(m, "attempt") },
		"next-session-request-cancelled-without-reason.json":            func(m map[string]any) { m["reason"] = "stopped" },
		"next-session-request-reason-timeout-state-failed.json":         func(m map[string]any) { m["state"] = "cancelled" },
		"next-session-request-reason-interrupted-state-succeeded.json":  func(m map[string]any) { m["state"] = "cancelled" },
		"next-session-request-reason-gateway-lost-state-cancelled.json": func(m map[string]any) { m["state"] = "failed" },
		"next-session-request-without-session.json":                     func(m map[string]any) { m["session"] = json.Number("1") },
	}
	n := 0
	for _, f := range runFiles(t, "fixtures/invalid") {
		name := path.Base(f)
		var schema string
		switch {
		case strings.HasPrefix(name, "next-session-answer-"):
			schema = "next-session-answer"
		case strings.HasPrefix(name, "next-session-request-"):
			schema = "next-session-request"
		default:
			continue
		}
		n++
		repair, ok := repairs[name]
		if !ok {
			t.Errorf("%s has no repair", name)
			continue
		}
		doc := fixture(t, "invalid/"+name)
		compiled := compileRun(t, runSchema(schema))
		if compiled.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
		repair(doc)
		if err := compiled.Validate(doc); err != nil {
			t.Errorf("%s with its defect repaired is refused: %v", name, err)
		}
	}
	if n != len(repairs) {
		t.Errorf("fixtures/invalid has %d next-session documents, and the test repairs %d", n, len(repairs))
	}
}

// TestTheNextSessionTypesMarshalToTheirSchemas pins that [contracts.NextSessionRequest],
// [contracts.NextSessionAnswer] and [contracts.ModelChoice] marshal to documents that
// their schemas accept, that an answer with next end is written as {"next":"end"}
// alone, that an answer for a session with full_suite false writes it, and that each
// valid fixture reads into its Go type and is written back as the same JSON value.
func TestTheNextSessionTypesMarshalToTheirSchemas(t *testing.T) {
	reason := "timeout"
	inputs := json.RawMessage(`{"version":1}`)
	values := []struct {
		schema string
		v      any
		want   string
	}{
		{"next-session-request", contracts.NextSessionRequest{Session: 1, State: "succeeded"},
			`{"session":1,"state":"succeeded","reason":null}`},
		{"next-session-request", contracts.NextSessionRequest{Session: 2, State: "cancelled", Reason: &reason},
			`{"session":2,"state":"cancelled","reason":"timeout"}`},
		{"next-session-answer", contracts.NextSessionAnswer{Next: contracts.NextSessionKindEnd}, `{"next":"end"}`},
		{"next-session-answer", contracts.NextSessionAnswer{Next: contracts.NextSessionKindEnd, Workflow: "build", Session: 3},
			`{"next":"end"}`},
		{"next-session-answer", contracts.NextSessionAnswer{Next: contracts.NextSessionKindSession, Session: 2,
			Workflow: "build", TimeLimitMinutes: 15, Inputs: inputs},
			`{"next":"session","session":2,"workflow":"build","time_limit_minutes":15,"full_suite":false,"inputs":{"version":1}}`},
		{"next-session-answer", contracts.NextSessionAnswer{Next: contracts.NextSessionKindSession, Session: 3,
			Workflow: "review", ModelCredential: "example-provider-2", TimeLimitMinutes: 1440, FullSuite: true, Inputs: inputs,
			ModelChoice: &contracts.ModelChoice{Model: "example-large", Effort: "high",
				ProviderModels: []contracts.ProviderModel{{ID: "example-large", Effort: []string{"low", "high"}}},
				Tiers:          &contracts.Tiers{Thinking: &contracts.Tier{Model: "example-large"}}}},
			`{"next":"session","session":3,"workflow":"review","model_choice":{"model":"example-large","effort":"high",` +
				`"provider_models":[{"id":"example-large","effort":["low","high"]}],"tiers":{"thinking":"example-large"}},` +
				`"model_credential":"example-provider-2","time_limit_minutes":1440,"full_suite":true,"inputs":{"version":1}}`},
		{"documents/model_choice", contracts.ModelChoice{}, `{}`},
		{"documents/model_choice", contracts.ModelChoice{Runtime: "codex"}, `{"runtime":"codex"}`},
		{"documents/model_choice", contracts.ModelChoice{Runtime: "codex", Model: "example-large",
			ProviderModels: []contracts.ProviderModel{{ID: "example-large"}}},
			`{"runtime":"codex","model":"example-large","provider_models":[{"id":"example-large"}]}`},
		{"documents/model_choice", contracts.ModelChoice{Model: "example-large",
			ProviderModels: []contracts.ProviderModel{{ID: "example-large"}}}, `{"model":"example-large","provider_models":[{"id":"example-large"}]}`},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if string(b) != c.want {
			t.Errorf("%T marshals to %s, want %s", c.v, b, c.want)
		}
		if err := validate(compileRun(t, runSchema(c.schema)), b); err != nil {
			t.Errorf("%T marshals to %s, which %s refuses: %v", c.v, b, runSchema(c.schema), err)
		}
	}
	for name, v := range map[string]func() any{
		"next-session-request": func() any { return &contracts.NextSessionRequest{} },
		"next-session-answer":  func() any { return &contracts.NextSessionAnswer{} },
	} {
		for _, f := range runFiles(t, "fixtures/"+name) {
			b, err := fs.ReadFile(contracts.RunFS, f)
			if err != nil {
				t.Fatal(err)
			}
			x := v()
			if err := json.Unmarshal(b, x); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			out, err := json.Marshal(x)
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
				t.Errorf("%s is written back as %s", f, out)
			}
		}
	}
}

// TestAnAnswerWithNoShapeIsNotWritten pins that [contracts.NextSessionAnswer] returns an
// error, and writes nothing, for a Next that is neither "session" nor "end", the empty
// Next included, and for a session answer with no Inputs, which the schema refuses
// as null.
func TestAnAnswerWithNoShapeIsNotWritten(t *testing.T) {
	inputs := json.RawMessage(`{"version":1}`)
	for name, a := range map[string]contracts.NextSessionAnswer{
		"no Next":                  {Session: 2, Workflow: "build", TimeLimitMinutes: 60, Inputs: inputs},
		"an unknown Next":          {Next: "pause"},
		"a session without Inputs": {Next: contracts.NextSessionKindSession, Session: 2, Workflow: "build", TimeLimitMinutes: 60},
	} {
		if b, err := json.Marshal(a); err == nil {
			t.Errorf("%s is written as %s", name, b)
		}
	}
	if _, err := json.Marshal(contracts.NextSessionAnswer{Next: contracts.NextSessionKindEnd}); err != nil {
		t.Errorf("an end is refused: %v", err)
	}
}

// TestTheNextSessionStateAndReasonRules pins the rules of the request that
// dev.qory.session.exited sets and that do not depend on its exit_code: a cancelled
// session has a reason, timeout and interrupted are for a cancelled session only, and
// gateway_lost is for a failed session only. The rules that depend on the exit_code, a
// succeeded or failed session with no reason, are not in the schema, because the request
// has no exit_code: a failed session with the reason of a refused join is accepted.
func TestTheNextSessionStateAndReasonRules(t *testing.T) {
	schema := compileRun(t, runSchema("next-session-request"))
	str := func(s string) *string { return &s }
	for _, c := range []struct {
		state  string
		reason *string
		ok     bool
	}{
		{"cancelled", str("timeout"), true}, {"cancelled", str("interrupted"), true},
		{"cancelled", str("stopped"), true}, {"cancelled", nil, false},
		{"failed", str("timeout"), false}, {"succeeded", str("interrupted"), false},
		{"failed", str("gateway_lost"), true}, {"succeeded", str("gateway_lost"), false},
		{"cancelled", str("gateway_lost"), false},
		{"failed", str("image_user_root"), true}, {"failed", nil, true}, {"succeeded", nil, true},
	} {
		b, err := json.Marshal(contracts.NextSessionRequest{Session: 1, State: c.state, Reason: c.reason})
		if err != nil {
			t.Fatal(err)
		}
		if err := validate(schema, b); (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", b, err, c.ok)
		}
	}
}

// TestTheDocumentsMembersOfTheSessions pins that the run documents of the step explore
// contain time_limit_minutes, shared_run and full_suite and read them into
// [contracts.Documents], that false for shared_run and full_suite is accepted and
// reads as the member left out, and that time_limit_minutes accepts 15 and 1440.
func TestTheDocumentsMembersOfTheSessions(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	var d contracts.Documents
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore-shared-run.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if d.TimeLimitMinutes != 15 || !d.SharedRun || !d.FullSuite {
		t.Errorf("explore-shared-run.json reads as time_limit_minutes %d, shared_run %v, full_suite %v", d.TimeLimitMinutes, d.SharedRun, d.FullSuite)
	}
	for _, limit := range []json.Number{"15", "1440"} {
		doc := fixture(t, "documents/explore-shared-run.json")
		doc["time_limit_minutes"] = limit
		if err := schema.Validate(doc); err != nil {
			t.Errorf("time_limit_minutes %s is refused: %v", limit, err)
		}
	}
	doc := fixture(t, "documents/explore-shared-run.json")
	doc["shared_run"], doc["full_suite"] = false, false
	if err := schema.Validate(doc); err != nil {
		t.Errorf("shared_run and full_suite false are refused: %v", err)
	}
	out, err := json.Marshal(contracts.Documents{})
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []string{"time_limit_minutes", "shared_run", "full_suite"} {
		if strings.Contains(string(out), m) {
			t.Errorf("an empty Documents is written with %s: %s", m, out)
		}
	}
	for _, name := range runFiles(t, "fixtures/documents") {
		if !strings.HasPrefix(path.Base(name), "prebuild") {
			continue
		}
		prebuild := fixture(t, strings.TrimPrefix(name, "fixtures/"))
		for _, m := range []string{"time_limit_minutes", "shared_run", "full_suite"} {
			if _, ok := prebuild[m]; ok {
				t.Errorf("%s has %s", name, m)
			}
		}
	}
}

// TestTheModelChoiceIsTheModelMembersOfRunSettings pins that model_choice has the
// members runtime, model, effort, provider_models and tiers of run_settings by
// reference, the same dependentRequired, and no required member, and that the
// next-session answer takes workflow, time_limit_minutes and inputs from
// documents.schema.json by reference.
func TestTheModelChoiceIsTheModelMembersOfRunSettings(t *testing.T) {
	doc, err := contracts.RunDocument("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defs := doc.(map[string]any)["$defs"].(map[string]any)
	settings := defs["run_settings"].(map[string]any)
	choice := defs["model_choice"].(map[string]any)
	props := choice["properties"].(map[string]any)
	if got := slices.Sorted(maps.Keys(props)); !slices.Equal(got, []string{"effort", "model", "provider_models", "runtime", "tiers"}) {
		t.Errorf("model_choice has the members %v", got)
	}
	for k, v := range props {
		if want := "#/$defs/run_settings/properties/" + k; v.(map[string]any)["$ref"] != want {
			t.Errorf("model_choice.%s refers to %v, want %s", k, v.(map[string]any)["$ref"], want)
		}
		if _, ok := settings["properties"].(map[string]any)[k]; !ok {
			t.Errorf("run_settings has no %s", k)
		}
	}
	if !reflect.DeepEqual(choice["dependentRequired"], settings["dependentRequired"]) {
		t.Errorf("model_choice has the dependentRequired %v, run_settings has %v", choice["dependentRequired"], settings["dependentRequired"])
	}
	if _, ok := choice["required"]; ok {
		t.Error("model_choice requires a member")
	}
	if choice["additionalProperties"] != false {
		t.Error("model_choice accepts members it does not define")
	}
	if _, ok := settings["properties"].(map[string]any)["time_limit_minutes"]; ok {
		t.Error("run_settings has time_limit_minutes")
	}
	answer, err := contracts.RunDocument("next-session-answer.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	branch := answer.(map[string]any)["oneOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)
	for member, def := range map[string]string{"workflow": "workflow", "model_choice": "model_choice",
		"time_limit_minutes": "time_limit_minutes", "inputs": "inputs"} {
		if got, want := branch[member].(map[string]any)["$ref"], contracts.RunBase+"/documents.schema.json#/$defs/"+def; got != want {
			t.Errorf("%s of the next-session answer refers to %v, want %s", member, got, want)
		}
	}
}

// TestTheNextSessionSchemasNameTheLink pins that the description of each next-session
// schema says once that the request goes through Forager's link, and contains no path,
// method or status.
func TestTheNextSessionSchemasNameTheLink(t *testing.T) {
	sentences := map[string]string{
		"next-session-request": "qory sends the request through Forager's link, and the gateway forwards it to the control plane. A container cannot send it.",
		"next-session-answer":  "It answers a request that qory sends through Forager's link, which the gateway forwards to the control plane.",
	}
	for name, sentence := range sentences {
		doc, err := contracts.RunDocument(name + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		d := doc.(map[string]any)["description"].(string)
		if strings.Count(d, sentence) != 1 {
			t.Errorf("%s has a description that states %q %d times", name, sentence, strings.Count(d, sentence))
		}
		for _, banned := range []string{"/v1/", "POST", "GET", "HTTP", "status", "200"} {
			if strings.Contains(d, banned) {
				t.Errorf("the description of %s contains %q", name, banned)
			}
		}
	}
}

// TestTheRuntimeOfAModelChoiceNamesTheCredentialRule pins that the descriptions of
// model_choice and of the model_credential of the next-session answer say that, for a
// session with a runtime, the credential's API lists that runtime in runtimes when it
// has any, and that model_choice says what an absent runtime means with no run_settings.
func TestTheRuntimeOfAModelChoiceNamesTheCredentialRule(t *testing.T) {
	rule := "only for a credential of an API whose runtimes, when present, list that runtime and whose format that runtime sends"
	documents, err := contracts.RunDocument("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	choice := documents.(map[string]any)["$defs"].(map[string]any)["model_choice"].(map[string]any)["description"].(string)
	answer, err := contracts.RunDocument("next-session-answer.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	credential := answer.(map[string]any)["oneOf"].([]any)[0].(map[string]any)["properties"].(map[string]any)["model_credential"].(map[string]any)["description"].(string)
	for name, d := range map[string]string{"model_choice": choice, "model_credential": credential} {
		if !strings.Contains(d, rule) {
			t.Errorf("the description of %s does not contain %q", name, rule)
		}
	}
	if want := "or, when the run documents have no run_settings, on the default runtime of the machine"; !strings.Contains(choice, want) {
		t.Errorf("the description of model_choice does not contain %q", want)
	}
}
