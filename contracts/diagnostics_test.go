package contracts_test

import (
	"path"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// diagnosticsSchemas are the schemas of the diagnostics contract.
var diagnosticsSchemas = []string{
	"diagnostics.schema.json",
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
	schema := compileDiagnostics(t, "diagnostics.schema.json")
	for _, f := range treeFiles(t, contracts.DiagnosticsFS, "fixtures/diagnostics") {
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
