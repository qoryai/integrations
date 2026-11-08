package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// diagnosticsSchemas are the schemas of the diagnostics contract.
var diagnosticsSchemas = []string{
	"diagnostics.schema.json",
	"runs.schema.json",
	"runs-refusal.schema.json",
}

// compileDiagnostics compiles a schema of the diagnostics contract, or fails the test.
func compileDiagnostics(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.CompileDiagnostics(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// TestDiagnosticsFixturesValidate pins that each document under fixtures/<name>/ of the
// diagnostics contract is accepted by <name>.schema.json, and that every schema of the
// contract compiles and has a fixture.
func TestDiagnosticsFixturesValidate(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range diagnosticsSchemas {
		dir := strings.TrimSuffix(name, ".schema.json")
		schema := compileDiagnostics(t, name)
		for _, f := range treeFiles(t, contracts.DiagnosticsFS, path.Join("fixtures", dir)) {
			seen[dir] = true
			doc, err := contracts.DiagnosticsDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
		if !seen[dir] {
			t.Errorf("%s has no fixture", name)
		}
	}
}

// TestInvalidDiagnosticsFixturesAreRefused pins that each document under
// fixtures/invalid of the diagnostics contract fails its schema for the reason in its
// name, and for no other.
func TestInvalidDiagnosticsFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"unknown-cause.json":                                  {"diagnostics", "/preparation/stopped/cause enum"},
		"unknown-fixer.json":                                  {"diagnostics", "/preparation/stopped/fixer enum"},
		"run-id-prefix.json":                                  {"diagnostics", "/run/id pattern"},
		"workflow-56-characters.json":                         {"diagnostics", "/run/workflow pattern"},
		"version-2.json":                                      {"diagnostics", "/version const"},
		"events-without-events-omitted.json":                  {"diagnostics", "/ dependentRequired/events"},
		"step-without-step-failed.json":                       {"diagnostics", "/preparation/stopped "},
		"two-dispatches.json":                                 {"diagnostics", "/dispatches maxItems"},
		"without-run.json":                                    {"diagnostics", "/ required run"},
		"step-failed-without-step.json":                       {"diagnostics", "/preparation/stopped required step"},
		"without-leaked-secrets.json":                         {"diagnostics", "/ required leaked_secrets"},
		"leaked-secret-id-with-other-kind.json":               {"diagnostics", "/leaked_secrets/0 "},
		"leaked-secret-step-without-step.json":                {"diagnostics", "/leaked_secrets/0 required step"},
		"leaked-secret-event-with-step.json":                  {"diagnostics", "/leaked_secrets/0 "},
		"leaked-secret-without-message.json":                  {"diagnostics", "/leaked_secrets/0 required message"},
		"leaked-secret-machine-log-without-machine-logs.json": {"diagnostics", "/leaked_secrets/0 "},
		"leaked-secret-access-key-with-run-id.json":           {"diagnostics", "/leaked_secrets/0/id pattern"},
		"runs-run-id-prefix.json":                             {"runs", "/runs/0/id pattern"},
		"runs-without-state.json":                             {"runs", "/runs/0 required state"},
		"runs-unknown-state.json":                             {"runs", "/runs/0/state enum"},
		"runs-next-workspace-id.json":                         {"runs", "/next type; /next pattern"},
		"runs-negative-duration.json":                         {"runs", "/runs/0/duration_seconds minimum"},
		"runs-work-item-title-1025-characters.json":           {"runs", "/runs/0/work_item_title maxLength"},
		"runs-work-item-id-alone.json":                        {"runs", "/runs/0 dependentRequired/work_item"},
		"runs-101-rows.json":                                  {"runs", "/runs maxItems"},
		"runs-empty-with-next.json":                           {"runs", "/next type"},
		"runs-without-next.json":                              {"runs", "/ required next"},
		"runs-refusal-invalid-request-without-parameter.json": {"runs-refusal", "/ required parameter"},
		"runs-refusal-parameter-with-unauthorized.json":       {"runs-refusal", "/ "},
	}
	got := treeFiles(t, contracts.DiagnosticsFS, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid contains %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.DiagnosticsDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileDiagnostics(t, w.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// TestDiagnosticsAcceptMembersTheSchemaDoesNotDefine pins that version 1 of the run
// diagnostics changes only by adding members: no object of the schema sets
// additionalProperties to false, and each valid fixture with a member added to every
// object in it, the top level included, is still accepted.
func TestDiagnosticsAcceptMembersTheSchemaDoesNotDefine(t *testing.T) {
	for _, name := range diagnosticsSchemas {
		doc, err := contracts.DiagnosticsDocument(name)
		if err != nil {
			t.Fatal(err)
		}
		var walk func(where string, v any)
		walk = func(where string, v any) {
			switch v := v.(type) {
			case map[string]any:
				if a, ok := v["additionalProperties"]; ok && a == false {
					t.Errorf("%s: %s sets additionalProperties to false", name, where)
				}
				if _, ok := v["unevaluatedProperties"]; ok {
					t.Errorf("%s: %s sets unevaluatedProperties", name, where)
				}
				for k, sub := range v {
					walk(where+"/"+k, sub)
				}
			case []any:
				for _, sub := range v {
					walk(where+"/[]", sub)
				}
			}
		}
		walk("", doc)
	}
	for _, name := range diagnosticsSchemas {
		schema := compileDiagnostics(t, name)
		dir := strings.TrimSuffix(name, ".schema.json")
		for _, f := range treeFiles(t, contracts.DiagnosticsFS, path.Join("fixtures", dir)) {
			doc, err := contracts.DiagnosticsDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			var add func(v any)
			add = func(v any) {
				switch v := v.(type) {
				case map[string]any:
					for _, sub := range v {
						add(sub)
					}
					v["example_added_member"] = "example"
				case []any:
					for _, sub := range v {
						add(sub)
					}
				}
			}
			add(doc)
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s with added members: %v", f, err)
			}
		}
	}
}

// diagnosticsTypes are the Go type of each schema of the diagnostics contract that has
// one, by the name of its fixture folder.
var diagnosticsTypes = map[string]func() any{
	"runs":         func() any { return &contracts.RunList{} },
	"runs-refusal": func() any { return &contracts.RunListRefusal{} },
}

// TestRunListTypesRoundTripTheirFixtures pins that each valid fixture of the run list
// and its refusal reads into its Go type and is written back as the fixture's bytes,
// compacted.
func TestRunListTypesRoundTripTheirFixtures(t *testing.T) {
	for dir, newValue := range diagnosticsTypes {
		for _, f := range treeFiles(t, contracts.DiagnosticsFS, path.Join("fixtures", dir)) {
			b, err := fs.ReadFile(contracts.DiagnosticsFS, f)
			if err != nil {
				t.Fatal(err)
			}
			v := newValue()
			if err := json.Unmarshal(b, v); err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			var want bytes.Buffer
			if err := json.Compact(&want, b); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, want.Bytes()) {
				t.Errorf("%s is written back as %s, want %s", f, out, want.Bytes())
			}
		}
	}
}

// TestRunListTypesMarshalToTheirSchemas marshals values of the run list's types built
// in Go, a list with a nil Runs and a refusal for an empty parameter name among them,
// and validates each against its schema.
func TestRunListTypesMarshalToTheirSchemas(t *testing.T) {
	next := "rn_0123456789abcdeg"
	empty := ""
	limit := contracts.RunListQueryLimit
	var zero int64
	values := []struct {
		schema string
		v      any
		want   string
	}{
		{"runs", contracts.RunList{}, `{"runs":[],"next":null}`},
		{"runs", contracts.RunList{Runs: []contracts.RunListRow{{
			ID: "rn_0123456789abcdeh", Number: 1,
			Organisation: contracts.RunListOrganisation{Slug: "example-org"},
			Workspace:    contracts.RunListWorkspace{ID: "ws_0123456789abcdef", Slug: "example"},
			State:        "running", CreatedAt: "2026-11-01T10:00:00Z", StartedAt: "2026-11-01T10:00:05Z",
			DurationSeconds: &zero,
		}}, Next: &next},
			`{"runs":[{"id":"rn_0123456789abcdeh","number":1,"organisation":{"slug":"example-org"},` +
				`"workspace":{"id":"ws_0123456789abcdef","slug":"example"},"state":"running",` +
				`"created_at":"2026-11-01T10:00:00Z","started_at":"2026-11-01T10:00:05Z","duration_seconds":0}],` +
				`"next":"rn_0123456789abcdeg"}`},
		{"runs-refusal", contracts.RunListRefusal{Error: contracts.RunListInvalidRequest, Parameter: &limit},
			`{"error":"invalid_request","parameter":"limit"}`},
		{"runs-refusal", contracts.RunListRefusal{Error: contracts.RunListInvalidRequest, Parameter: &empty},
			`{"error":"invalid_request","parameter":""}`},
		{"runs-refusal", contracts.RunListRefusal{Error: "unauthorized"}, `{"error":"unauthorized"}`},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if string(b) != c.want {
			t.Errorf("%T is written as %s, want %s", c.v, b, c.want)
		}
		if err := validate(compileDiagnostics(t, c.schema+".schema.json"), b); err != nil {
			t.Errorf("%T marshals to %s, which %s.schema.json refuses: %v", c.v, b, c.schema, err)
		}
	}
}

// TestARunListHasAtMostRunListMaxLimitRows pins that runs.schema.json accepts
// RunListMaxLimit rows and refuses one more, and that the default limit is within it.
func TestARunListHasAtMostRunListMaxLimitRows(t *testing.T) {
	if contracts.RunListDefaultLimit < 1 || contracts.RunListDefaultLimit > contracts.RunListMaxLimit {
		t.Errorf("RunListDefaultLimit is %d, want 1 to %d", contracts.RunListDefaultLimit, contracts.RunListMaxLimit)
	}
	row := contracts.RunListRow{
		ID: "rn_0123456789abcdeh", Number: 1,
		Organisation: contracts.RunListOrganisation{Slug: "example-org"},
		Workspace:    contracts.RunListWorkspace{ID: "ws_0123456789abcdef", Slug: "example"},
		State:        "pending", CreatedAt: "2026-11-01T10:00:00Z",
	}
	schema := compileDiagnostics(t, "runs.schema.json")
	for _, n := range []int{contracts.RunListMaxLimit, contracts.RunListMaxLimit + 1} {
		b, err := json.Marshal(contracts.RunList{Runs: slices.Repeat([]contracts.RunListRow{row}, n)})
		if err != nil {
			t.Fatal(err)
		}
		err = validate(schema, b)
		if n <= contracts.RunListMaxLimit && err != nil {
			t.Errorf("%d rows are refused: %v", n, err)
		}
		if n > contracts.RunListMaxLimit && err == nil {
			t.Errorf("%d rows are accepted", n)
		}
	}
}

// TestRunListStatesAreTheRunStates pins that a row accepts each run state of the run
// diagnostics, and that the description of runs.schema.json describes each query
// parameter by the name of its constant.
func TestRunListStatesAreTheRunStates(t *testing.T) {
	schema := compileDiagnostics(t, "runs.schema.json")
	for _, s := range []string{"pending", "running", "completed", "failed", "cancelled", "lost"} {
		b := fmt.Appendf(nil, `{"runs":[{"id":"rn_0123456789abcdeh","number":1,"organisation":{"slug":"example-org"},`+
			`"workspace":{"id":"ws_0123456789abcdef","slug":"example"},"state":%q,"created_at":"2026-11-01T10:00:00Z"}],"next":null}`, s)
		if err := validate(schema, b); err != nil {
			t.Errorf("the state %s is refused: %v", s, err)
		}
	}
	doc, err := contracts.DiagnosticsDocument("runs.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	description, _ := doc.(map[string]any)["description"].(string)
	for _, p := range []string{
		contracts.RunListQueryWorkspace, contracts.RunListQueryWorkItem, contracts.RunListQueryWorkflow,
		contracts.RunListQueryState, contracts.RunListQuerySince, contracts.RunListQueryBefore,
		contracts.RunListQueryLimit,
	} {
		if !strings.Contains(description, " "+p+": ") {
			t.Errorf("runs.schema.json does not describe the parameter %s", p)
		}
	}
}
