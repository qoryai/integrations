package contracts_test

import (
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// exploreSchemas are the schemas of the explore contract.
var exploreSchemas = []string{
	"inputs.schema.json",
	"result.schema.json",
}

// compileExplore compiles a schema of the explore contract, or fails the test.
func compileExplore(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.CompileExplore(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// TestExploreFixturesValidate pins that each document under fixtures/<name>/ of the
// explore contract is accepted by <name>.schema.json, and that every schema of the
// contract compiles and has a fixture.
func TestExploreFixturesValidate(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range exploreSchemas {
		dir := strings.TrimSuffix(name, ".schema.json")
		schema := compileExplore(t, name)
		for _, f := range treeFiles(t, contracts.ExploreFS, path.Join("fixtures", dir)) {
			seen[dir] = true
			doc, err := contracts.ExploreDocument(f)
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

// TestInvalidExploreFixturesAreRefused pins that each document under fixtures/invalid
// of the explore contract fails its schema for the reason in its name, and for no
// other.
func TestInvalidExploreFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"result-deliverables.json":                     {"result", "/ additionalProperties"},
		"result-understood-with-questions.json":        {"result", "/questions "},
		"result-not-understood-without-questions.json": {"result", "/ required questions"},
		"result-complexity-unknown-level.json":         {"result", "/complexity/level enum"},
		"result-source-http.json":                      {"result", "/knowledge/0/sources/0 pattern"},
		"result-understood-without-scope.json":         {"result", "/ required scope"},
		"result-question-escape.json":                  {"result", "/questions/0 pattern"},
		"result-question-c1-csi.json":                  {"result", "/questions/0 pattern"},
		"result-source-next-line.json":                 {"result", "/knowledge/0/sources/0 pattern"},
		"result-understanding-bidi-override.json":      {"result", "/understanding pattern"},
		"result-question-zero-width-space.json":        {"result", "/questions/0 pattern"},
		"result-source-space.json":                     {"result", "/knowledge/0/sources/0 pattern"},
		"result-source-userinfo.json":                  {"result", "/knowledge/0/sources/0 pattern"},
		"result-source-backslash.json":                 {"result", "/knowledge/0/sources/0 pattern"},
		"inputs-filer-kind-bot.json":                   {"inputs", "/ticket/filed_by/kind enum"},
		"inputs-attempt-4.json":                        {"inputs", "/run/attempt maximum"},
		"inputs-work-item-url-http.json":               {"inputs", "/work_item/url pattern"},
		"inputs-collection-ref.json":                   {"inputs", "/ticket additionalProperties"},
		"inputs-ticket-ref.json":                       {"inputs", "/ticket required external_ref; /ticket additionalProperties"},
		"inputs-without-comments-omitted.json":         {"inputs", "/ required comments_omitted"},
		"inputs-time-not-utc.json":                     {"inputs", "/ticket/filed_at pattern"},
		"inputs-instruction-escape.json":               {"inputs", "/instruction/text pattern"},
		"inputs-member-name-line-feed.json":            {"inputs", "/instruction/by/name pattern"},
		"inputs-without-domain.json":                   {"inputs", "/ required domain"},
	}
	got := treeFiles(t, contracts.ExploreFS, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.ExploreDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileExplore(t, w.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// TestTheHandoverAnswersExploreInputsValidate pins that the inputs of each valid
// handover answer of the provisioner contract whose step is explore are accepted by
// explore/v1 inputs.schema.json, the shape handover-answer.schema.json names for that
// step.
func TestTheHandoverAnswersExploreInputsValidate(t *testing.T) {
	schema := compileExplore(t, "inputs.schema.json")
	explore := 0
	for _, f := range provisionerFiles(t, "fixtures/handover-answer") {
		doc, err := contracts.ProvisionerDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "explore" {
			continue
		}
		explore++
		if err := schema.Validate(answer["inputs"]); err != nil {
			t.Errorf("%s: its inputs: %v", f, err)
		}
	}
	if explore == 0 {
		t.Error("fixtures/handover-answer holds no answer for the step explore")
	}
}

// TestTheHandoverAnswerTargetDomainAndWorkItemAreItsExploreInputs pins that each valid
// handover answer of the provisioner contract whose step is explore carries the same
// target in target and in inputs.target, or neither, the same domain in domain and in
// inputs.domain, and the same work item id in work_item.id and in inputs.work_item.id,
// as explore/v1 inputs.schema.json says the control plane writes them.
func TestTheHandoverAnswerTargetDomainAndWorkItemAreItsExploreInputs(t *testing.T) {
	explore := 0
	for _, f := range provisionerFiles(t, "fixtures/handover-answer") {
		doc, err := contracts.ProvisionerDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "explore" {
			continue
		}
		explore++
		inputs, _ := answer["inputs"].(map[string]any)
		if !reflect.DeepEqual(answer["target"], inputs["target"]) {
			t.Errorf("%s: target is %v, and inputs.target is %v", f, answer["target"], inputs["target"])
		}
		if answer["domain"] == nil || answer["domain"] != inputs["domain"] {
			t.Errorf("%s: domain is %v, and inputs.domain is %v", f, answer["domain"], inputs["domain"])
		}
		item, _ := answer["work_item"].(map[string]any)
		inputsItem, _ := inputs["work_item"].(map[string]any)
		if item["id"] == nil || item["id"] != inputsItem["id"] {
			t.Errorf("%s: work_item.id is %v, and inputs.work_item.id is %v", f, item["id"], inputsItem["id"])
		}
	}
	if explore == 0 {
		t.Error("fixtures/handover-answer holds no answer for the step explore")
	}
}
