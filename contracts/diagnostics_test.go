package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"math"
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
		"runs-state-upper-case.json":                          {"runs", "/runs/0/state pattern"},
		"runs-number-0.json":                                  {"runs", "/runs/0/number minimum"},
		"runs-number-above-int64.json":                        {"runs", "/runs/0/number maximum"},
		"runs-work-item-number-0.json":                        {"runs", "/runs/0/work_item_number minimum"},
		"runs-work-item-number-alone.json":                    {"runs", "/runs/0 dependentRequired/work_item_number"},
		"runs-work-item-title-alone.json":                     {"runs", "/runs/0 dependentRequired/work_item_title"},
		"runs-duration-without-started-at.json":               {"runs", "/runs/0 dependentRequired/duration_seconds"},
		"runs-next-workspace-id.json":                         {"runs", "/next type; /next pattern"},
		"runs-negative-duration.json":                         {"runs", "/runs/0/duration_seconds minimum"},
		"runs-work-item-title-1025-characters.json":           {"runs", "/runs/0/work_item_title maxLength"},
		"runs-101-rows.json":                                  {"runs", "/runs maxItems"},
		"runs-empty-with-next.json":                           {"runs", "/next type"},
		"runs-without-next.json":                              {"runs", "/ required next"},
		"runs-refusal-invalid-request-without-parameter.json": {"runs-refusal", "/ required parameter"},
		"runs-refusal-parameter-with-unauthorized.json":       {"runs-refusal", "/ "},
		"runs-refusal-parameter-65-characters.json":           {"runs-refusal", "/parameter pattern"},
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

// TestDiagnosticsAcceptMembersTheSchemaDoesNotDefine pins that version 1 of the
// diagnostics contract changes only by adding members: no object of its schemas sets
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

// diagnosticsTypes are the Go types of the schemas of the diagnostics contract that
// have one, each by the name of its fixture folder.
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
			t.Errorf("%T marshals to %s, which fails %s.schema.json: %v", c.v, b, c.schema, err)
		}
	}
}

// TestARunListHasAtMostRunListMaxLimitRows pins that runs.schema.json accepts
// RunListMaxLimit rows and that one more fails it, and that the default limit is
// within it.
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

// TestRunListStatesAreTheRunStates pins that each state of run_state in
// diagnostics.schema.json is listed in the state of a row and the parameter state in
// runs.schema.json, in the RunListQueryState comment and in the State paragraph of the
// RunListRow comment, and that a row accepts each of them.
func TestRunListStatesAreTheRunStates(t *testing.T) {
	doc, err := contracts.DiagnosticsDocument("diagnostics.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var states []string
	for _, s := range doc.(map[string]any)["$defs"].(map[string]any)["run_state"].(map[string]any)["enum"].([]any) {
		states = append(states, s.(string))
	}
	if len(states) == 0 {
		t.Fatal("run_state in diagnostics.schema.json lists no state")
	}
	runs, err := contracts.DiagnosticsDocument("runs.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	row := runs.(map[string]any)["$defs"].(map[string]any)["row"].(map[string]any)
	top, _ := runs.(map[string]any)["description"].(string)
	_, parameter, _ := strings.Cut(top, " state: ")
	parameter, _, _ = strings.Cut(parameter, ". ")
	_, paragraph, _ := strings.Cut(declComment(t, "diagnostics.go", "RunListRow"), "State is the run's state")
	paragraph, _, _ = strings.Cut(paragraph, "\n\n")
	lists := map[string]string{
		"the state of a row in runs.schema.json":        row["properties"].(map[string]any)["state"].(map[string]any)["description"].(string),
		"the parameter state in runs.schema.json":       parameter,
		"the RunListQueryState comment":                 declComment(t, "diagnostics.go", "RunListQueryState"),
		"the State paragraph of the RunListRow comment": paragraph,
	}
	for where, list := range lists {
		if strings.TrimSpace(list) == "" {
			t.Errorf("%s is not found", where)
		}
	}
	schema := compileDiagnostics(t, "runs.schema.json")
	for _, s := range states {
		for where, list := range lists {
			if !strings.Contains(list, s) {
				t.Errorf("%s does not list %s", where, s)
			}
		}
		b := fmt.Appendf(nil, `{"runs":[{"id":"rn_0123456789abcdeh","number":1,"organisation":{"slug":"example-org"},`+
			`"workspace":{"id":"ws_0123456789abcdef","slug":"example"},"state":%q,"created_at":"2026-11-01T10:00:00Z"}],"next":null}`, s)
		if err := validate(schema, b); err != nil {
			t.Errorf("the state %s fails runs.schema.json: %v", s, err)
		}
	}
}

// declComment returns the doc comment of the constant or type name in the file of this
// package at file, or fails the test. For a declaration outside parentheses, the doc
// comment is the comment of the declaration.
func declComment(t *testing.T, file, name string) string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			var doc *ast.CommentGroup
			var names []*ast.Ident
			switch v := spec.(type) {
			case *ast.ValueSpec:
				doc, names = v.Doc, v.Names
			case *ast.TypeSpec:
				doc, names = v.Doc, []*ast.Ident{v.Name}
			}
			if doc == nil && g.Lparen == token.NoPos {
				doc = g.Doc
			}
			for _, n := range names {
				if n.Name == name && doc != nil {
					return doc.Text()
				}
			}
		}
	}
	t.Fatalf("%s has no declaration %s with a doc comment", file, name)
	return ""
}

// TestRunListDescribesEachQueryParameter pins that the description of runs.schema.json
// describes each query parameter by the name of its constant.
func TestRunListDescribesEachQueryParameter(t *testing.T) {
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

// TestRunListEdgesAreAccepted pins the largest values that the run list and its refusal
// accept: a run number and a work item number of 9223372036854775807, the largest
// int64, a work item title of 1024 characters and a parameter name of 64.
func TestRunListEdgesAreAccepted(t *testing.T) {
	var zero int64
	list := contracts.RunList{Runs: []contracts.RunListRow{{
		ID: "rn_0123456789abcdeh", Number: math.MaxInt64,
		Organisation:   contracts.RunListOrganisation{Slug: "example-org"},
		Workspace:      contracts.RunListWorkspace{ID: "ws_0123456789abcdef", Slug: "example"},
		WorkItemNumber: math.MaxInt64, WorkItemTitle: strings.Repeat("x", 1024),
		State: "running", CreatedAt: "2026-11-01T10:00:00Z", StartedAt: "2026-11-01T10:00:00Z",
		DurationSeconds: &zero,
	}}}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileDiagnostics(t, "runs.schema.json"), b); err != nil {
		t.Errorf("a row with the largest numbers and title fails runs.schema.json: %v", err)
	}
	name := strings.Repeat("p", 64)
	b, err = json.Marshal(contracts.RunListRefusal{Error: contracts.RunListInvalidRequest, Parameter: &name})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileDiagnostics(t, "runs-refusal.schema.json"), b); err != nil {
		t.Errorf("a parameter of 64 characters fails runs-refusal.schema.json: %v", err)
	}
}
