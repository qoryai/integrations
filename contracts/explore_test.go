package contracts_test

import (
	"io/fs"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// exploreSchemas are the schemas of the explore contract.
var exploreSchemas = []string{
	"inputs.schema.json",
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

// memberLeaves are a refusal's reasons as [leaves] writes them and, after a reason of
// the keyword additionalProperties, the members the schema does not define, so that
// a test sees which member a document adds.
func memberLeaves(v *jsonschema.ValidationError) []string {
	if len(v.Causes) == 0 {
		r := leaves(v)[0]
		if k, ok := v.ErrorKind.(*kind.AdditionalProperties); ok {
			r += " " + strings.Join(k.Properties, ",")
		}
		return []string{r}
	}
	var out []string
	for _, c := range v.Causes {
		out = append(out, memberLeaves(c)...)
	}
	return out
}

// TestExploreSchemasAreTheContract pins that explore/v1 contains exactly the files of
// exploreSchemas, each with the $id ExploreBase/<file>, and that fixtures/ contains a
// folder for each schema, the folder invalid, and no other folder.
func TestExploreSchemasAreTheContract(t *testing.T) {
	var files []string
	entries, err := fs.ReadDir(contracts.ExploreFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, e.Name())
		}
	}
	if !slices.Equal(files, exploreSchemas) {
		t.Errorf("explore/v1 contains %v, want %v", files, exploreSchemas)
	}
	want := []string{"invalid"}
	for _, name := range exploreSchemas {
		doc, err := contracts.ExploreDocument(name)
		if err != nil {
			t.Fatal(err)
		}
		if id := doc.(map[string]any)["$id"]; id != contracts.ExploreBase+"/"+name {
			t.Errorf("%s has the $id %v", name, id)
		}
		want = append(want, strings.TrimSuffix(name, ".schema.json"))
	}
	slices.Sort(want)
	entries, err = fs.ReadDir(contracts.ExploreFS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if !slices.Equal(dirs, want) {
		t.Errorf("explore/v1/fixtures contains the folders %v, want %v", dirs, want)
	}
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
// other. A reason of the keyword additionalProperties ends with the member that the
// document adds.
func TestInvalidExploreFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"inputs-version-2.json":                    {"inputs", "/version const"},
		"inputs-work-item-number-0.json":           {"inputs", "/work_item/number minimum"},
		"inputs-work-item-url-http.json":           {"inputs", "/work_item/url pattern"},
		"inputs-attempt-0.json":                    {"inputs", "/run/attempt minimum"},
		"inputs-run-of.json":                       {"inputs", "/run additionalProperties of"},
		"inputs-filer-kind-bot.json":               {"inputs", "/ticket/filed_by/kind enum"},
		"inputs-collection-ref.json":               {"inputs", "/ticket additionalProperties collection_ref"},
		"inputs-ticket-ref.json":                   {"inputs", "/ticket required external_ref; /ticket additionalProperties ref"},
		"inputs-time-not-utc.json":                 {"inputs", "/ticket/filed_at pattern"},
		"inputs-instruction-escape.json":           {"inputs", "/instruction/text pattern"},
		"inputs-instruction-c1-csi.json":           {"inputs", "/instruction/text pattern"},
		"inputs-instruction-bidi-override.json":    {"inputs", "/instruction/text pattern"},
		"inputs-instruction-zero-width-space.json": {"inputs", "/instruction/text pattern"},
		"inputs-instruction-4001-characters.json":  {"inputs", "/instruction/text maxLength"},
		"inputs-member-name-line-feed.json":        {"inputs", "/instruction/by/name pattern"},
		"inputs-member-name-tab.json":              {"inputs", "/instruction/by/name pattern"},
		"inputs-member-name-bidi-override.json":    {"inputs", "/instruction/by/name pattern"},
		"inputs-member-name-empty.json":            {"inputs", "/instruction/by/name minLength"},
		"inputs-member-name-161-characters.json":   {"inputs", "/instruction/by/name maxLength"},
		"inputs-without-comments-omitted.json":     {"inputs", "/ required comments_omitted"},
		"inputs-comments-omitted-negative.json":    {"inputs", "/comments_omitted minimum"},
		"inputs-without-domain.json":               {"inputs", "/ required domain"},
		"inputs-answers.json":                      {"inputs", "/ additionalProperties answers"},
		"inputs-changes.json":                      {"inputs", "/ additionalProperties changes"},
		"inputs-facts.json":                        {"inputs", "/ additionalProperties facts"},
		"inputs-proposal.json":                     {"inputs", "/ additionalProperties proposal"},
	}
	got := treeFiles(t, contracts.ExploreFS, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid contains %d documents, and the test expects %d", len(got), len(want))
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
		if r := strings.Join(memberLeaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// exploreAt is the object at the instance location where in doc, each step a member
// name or an array index.
func exploreAt(doc any, where []string) map[string]any {
	for _, k := range where {
		switch v := doc.(type) {
		case map[string]any:
			doc = v[k]
		case []any:
			i, _ := strconv.Atoi(k)
			doc = v[i]
		}
	}
	m, _ := doc.(map[string]any)
	return m
}

// TestExploreLengthsAreRefusedOneCharacterPastTheirBound pins the bounds of
// instruction.text and of a member's name: each invalid fixture one character past
// its bound is accepted with one character fewer.
func TestExploreLengthsAreRefusedOneCharacterPastTheirBound(t *testing.T) {
	schema := compileExplore(t, "inputs.schema.json")
	for _, c := range []struct {
		fixture string
		object  []string
		member  string
	}{
		{"inputs-instruction-4001-characters.json", []string{"instruction"}, "text"},
		{"inputs-member-name-161-characters.json", []string{"instruction", "by"}, "name"},
	} {
		doc, err := contracts.ExploreDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		object := exploreAt(doc, c.object)
		s, _ := object[c.member].(string)
		if s == "" {
			t.Errorf("%s has no %s", c.fixture, c.member)
			continue
		}
		object[c.member] = s[1:]
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s with one character fewer is refused: %v", c.fixture, err)
		}
	}
}

// TestExploreMembersAreRequiredAndClosed pins two rules that inputs.schema.json states
// for every object at every level: every member is required, except target and
// ticket, and a member the schema does not define is refused. In
// fixtures/inputs/first-attempt.json, which has every member, it removes each member
// in turn, then adds the member undefined to each object in turn, and checks the
// refusal of each document.
func TestExploreMembersAreRequiredAndClosed(t *testing.T) {
	const fixture = "fixtures/inputs/first-attempt.json"
	schema := compileExplore(t, "inputs.schema.json")
	read := func() any {
		doc, err := contracts.ExploreDocument(fixture)
		if err != nil {
			t.Fatal(err)
		}
		return doc
	}
	var objects [][]string
	var walk func(where []string, v any)
	walk = func(where []string, v any) {
		switch v := v.(type) {
		case map[string]any:
			objects = append(objects, where)
			for k, sub := range v {
				walk(append(slices.Clone(where), k), sub)
			}
		case []any:
			for i, sub := range v {
				walk(append(slices.Clone(where), strconv.Itoa(i)), sub)
			}
		}
	}
	walk(nil, read())
	refusal := func(doc any) string {
		v, ok := schema.Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			return "accepted"
		}
		return strings.Join(memberLeaves(v), "; ")
	}
	members := 0
	for _, where := range objects {
		location := "/" + strings.Join(where, "/")
		for member := range exploreAt(read(), where) {
			members++
			doc := read()
			delete(exploreAt(doc, where), member)
			want := location + " required " + member
			if len(where) == 0 && (member == "target" || member == "ticket") {
				want = "accepted"
			}
			if got := refusal(doc); got != want {
				t.Errorf("%s without %s in %s: %s, want %s", fixture, member, location, got, want)
			}
		}
		doc := read()
		exploreAt(doc, where)["undefined"] = "x"
		if got, want := refusal(doc), location+" additionalProperties undefined"; got != want {
			t.Errorf("%s with undefined in %s: %s, want %s", fixture, location, got, want)
		}
	}
	if len(objects) < 2 || members == 0 {
		t.Errorf("%s has %d objects and %d members", fixture, len(objects), members)
	}
}

// TestTheRunDocumentsExploreInputsValidate pins that the inputs of each run documents
// fixture of the run contract whose step is explore are accepted by explore/v1
// inputs.schema.json, the shape that run/v1 documents.schema.json refers to for the
// inputs of that step. It checks every valid fixture, and every invalid fixture that
// has inputs and whose refusal by documents.schema.json has no reason at /inputs or
// below it.
func TestTheRunDocumentsExploreInputsValidate(t *testing.T) {
	schema := compileExplore(t, "inputs.schema.json")
	documents := compileRun(t, runSchema("documents"))
	for _, dir := range []string{"fixtures/documents", "fixtures/invalid"} {
		checked := 0
		for _, f := range runFiles(t, dir) {
			doc, err := contracts.RunDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			answer, ok := doc.(map[string]any)
			if !ok || answer["step"] != "explore" {
				continue
			}
			if dir == "fixtures/invalid" && (answer["inputs"] == nil || refusedAtInputs(documents.Validate(doc))) {
				continue
			}
			checked++
			if err := schema.Validate(answer["inputs"]); err != nil {
				t.Errorf("%s: its inputs: %v", f, err)
			}
		}
		if checked == 0 {
			t.Errorf("%s has no documents for the step explore whose inputs the test checks", dir)
		}
	}
}

// refusedAtInputs reports whether err, a refusal by run/v1 documents.schema.json, has
// a reason at /inputs or below it.
func refusedAtInputs(err error) bool {
	v, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return false
	}
	for _, l := range leaves(v) {
		if strings.HasPrefix(l, "/inputs ") || strings.HasPrefix(l, "/inputs/") {
			return true
		}
	}
	return false
}

// TestTheRunTargetDomainAndWorkItemAreItsExploreInputs pins that each valid run
// documents fixture of the run contract whose step is explore has the same
// target in target and in inputs.target, or neither, the same domain in domain and in
// inputs.domain, and the same work item id in work_item.id and in inputs.work_item.id,
// as explore/v1 inputs.schema.json describes how the control plane writes them.
func TestTheRunTargetDomainAndWorkItemAreItsExploreInputs(t *testing.T) {
	explore := 0
	for _, f := range runFiles(t, "fixtures/documents") {
		doc, err := contracts.RunDocument(f)
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
		t.Error("fixtures/documents has no documents for the step explore")
	}
}
