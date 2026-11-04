package contracts_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

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
// a test can check which member a document adds.
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

// entryNames are the names of the entries of the folder dir in fsys, sorted: its
// folders when folders is true, and its files otherwise.
func entryNames(t *testing.T, fsys fs.FS, dir string, folders bool) []string {
	t.Helper()
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() == folders {
			out = append(out, e.Name())
		}
	}
	return out
}

// TestExploreSchemasAreTheContract pins that explore/v1 contains exactly the files of
// exploreSchemas, each with the $id ExploreBase/<file>, and that fixtures/ contains a
// folder for each schema, the folder invalid, and no other folder.
func TestExploreSchemasAreTheContract(t *testing.T) {
	schemas := slices.Clone(exploreSchemas)
	slices.Sort(schemas)
	if files := entryNames(t, contracts.ExploreFS, ".", false); !slices.Equal(files, schemas) {
		t.Errorf("explore/v1 contains %v, want %v", files, schemas)
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
	if dirs := entryNames(t, contracts.ExploreFS, "fixtures", true); !slices.Equal(dirs, want) {
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

// invalidExploreFixtures are the documents under fixtures/invalid of the explore
// contract, each with its schema and the reason it is refused for. A reason of the
// keyword additionalProperties ends with the member that the document adds.
var invalidExploreFixtures = map[string]struct{ schema, reason string }{
	"inputs-version-2.json":                         {"inputs", "/version const"},
	"inputs-work-item-number-0.json":                {"inputs", "/work_item/number minimum"},
	"inputs-work-item-url-http.json":                {"inputs", "/work_item/url pattern"},
	"inputs-attempt-0.json":                         {"inputs", "/run/attempt minimum"},
	"inputs-run-of.json":                            {"inputs", "/run additionalProperties of"},
	"inputs-filer-kind-bot.json":                    {"inputs", "/ticket/filed_by/kind enum"},
	"inputs-collection-ref.json":                    {"inputs", "/ticket additionalProperties collection_ref"},
	"inputs-ticket-ref.json":                        {"inputs", "/ticket required external_ref; /ticket additionalProperties ref"},
	"inputs-time-not-utc.json":                      {"inputs", "/ticket/filed_at pattern"},
	"inputs-instruction-escape.json":                {"inputs", "/instruction/text pattern"},
	"inputs-instruction-c1-csi.json":                {"inputs", "/instruction/text pattern"},
	"inputs-instruction-bidi-override.json":         {"inputs", "/instruction/text pattern"},
	"inputs-instruction-zero-width-space.json":      {"inputs", "/instruction/text pattern"},
	"inputs-instruction-4001-characters.json":       {"inputs", "/instruction/text maxLength"},
	"inputs-member-name-line-feed.json":             {"inputs", "/instruction/by/name pattern"},
	"inputs-member-name-tab.json":                   {"inputs", "/instruction/by/name pattern"},
	"inputs-member-name-bidi-override.json":         {"inputs", "/instruction/by/name pattern"},
	"inputs-member-name-empty.json":                 {"inputs", "/instruction/by/name minLength"},
	"inputs-member-name-161-characters.json":        {"inputs", "/instruction/by/name maxLength"},
	"inputs-without-comments-omitted.json":          {"inputs", "/ required comments_omitted"},
	"inputs-comments-omitted-negative.json":         {"inputs", "/comments_omitted minimum"},
	"inputs-without-domain.json":                    {"inputs", "/ required domain"},
	"inputs-without-ticket-with-comments.json":      {"inputs", "/comments maxItems"},
	"inputs-without-ticket-comments-omitted-1.json": {"inputs", "/comments_omitted const"},
	"inputs-answers.json":                           {"inputs", "/ additionalProperties answers"},
	"inputs-changes.json":                           {"inputs", "/ additionalProperties changes"},
	"inputs-facts.json":                             {"inputs", "/ additionalProperties facts"},
	"inputs-proposal.json":                          {"inputs", "/ additionalProperties proposal"},
}

// TestInvalidExploreFixturesAreRefused pins that each document under fixtures/invalid
// of the explore contract fails its schema for the reason in its name, and for no
// other, as invalidExploreFixtures lists them.
func TestInvalidExploreFixturesAreRefused(t *testing.T) {
	want := invalidExploreFixtures
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

// objectAt is the object at the instance location where in doc, each step a member
// name or an array index. It returns nil when where leads to no object.
func objectAt(doc any, where []string) map[string]any {
	for _, k := range where {
		switch v := doc.(type) {
		case map[string]any:
			doc = v[k]
		case []any:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(v) {
				return nil
			}
			doc = v[i]
		default:
			return nil
		}
	}
	m, _ := doc.(map[string]any)
	return m
}

// TestExploreLengthFixturesAreOneCharacterPastTheBound pins that each invalid fixture
// of invalidExploreFixtures refused for maxLength alone is one character past its
// bound: with the first character of the string at the reason's location removed,
// the document is accepted.
func TestExploreLengthFixturesAreOneCharacterPastTheBound(t *testing.T) {
	checked := 0
	for name, w := range invalidExploreFixtures {
		location, ok := strings.CutSuffix(w.reason, " maxLength")
		if !ok || strings.Contains(location, ";") {
			continue
		}
		checked++
		doc, err := contracts.ExploreDocument(path.Join("fixtures/invalid", name))
		if err != nil {
			t.Fatal(err)
		}
		where := strings.Split(strings.TrimPrefix(location, "/"), "/")
		object := objectAt(doc, where[:len(where)-1])
		member := where[len(where)-1]
		s, _ := object[member].(string)
		if s == "" {
			t.Errorf("%s has no string at %s", name, location)
			continue
		}
		_, size := utf8.DecodeRuneInString(s)
		object[member] = s[size:]
		if err := compileExplore(t, w.schema+".schema.json").Validate(doc); err != nil {
			t.Errorf("%s with one character fewer at %s is refused: %v", name, location, err)
		}
	}
	if checked == 0 {
		t.Error("invalidExploreFixtures has no fixture refused for maxLength")
	}
}

// TestExploreMembersAreRequiredAndClosed pins two rules that inputs.schema.json states
// for every object at every level: every member is required, except target and
// ticket, and a member the schema does not define is refused. It first checks that
// each object of fixtures/inputs/first-attempt.json has every member that the schema
// defines at that location. It then removes each member in turn, then adds the member
// undefined to each object in turn, and checks the refusal of each document. Without
// ticket, the schema requires comments to be empty and comments_omitted to be 0, so
// the document without ticket also has those values.
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
		got := slices.Sorted(maps.Keys(objectAt(read(), where)))
		if want := definedAt(schema, where); !slices.Equal(got, want) {
			t.Errorf("%s has the members %v in %s, and the schema defines %v", fixture, got, location, want)
		}
		for member := range objectAt(read(), where) {
			members++
			doc := read()
			delete(objectAt(doc, where), member)
			if len(where) == 0 && member == "ticket" {
				objectAt(doc, where)["comments"] = []any{}
				objectAt(doc, where)["comments_omitted"] = json.Number("0")
			}
			want := location + " required " + member
			if len(where) == 0 && (member == "target" || member == "ticket") {
				want = "accepted"
			}
			if got := refusal(doc); got != want {
				t.Errorf("%s without %s in %s: %s, want %s", fixture, member, location, got, want)
			}
		}
		doc := read()
		objectAt(doc, where)["undefined"] = "x"
		if got, want := refusal(doc), location+" additionalProperties undefined"; got != want {
			t.Errorf("%s with undefined in %s: %s, want %s", fixture, location, got, want)
		}
	}
	if len(objects) < 2 || members == 0 {
		t.Errorf("%s has %d objects and %d members", fixture, len(objects), members)
	}
}

// definedAt are the members, sorted, that schema defines for the object at the
// instance location where, each step a member name or an array index. It follows
// $ref to the schema that defines properties or items.
func definedAt(schema *jsonschema.Schema, where []string) []string {
	resolve := func(s *jsonschema.Schema) *jsonschema.Schema {
		for s != nil && s.Ref != nil && s.Properties == nil && s.Items2020 == nil {
			s = s.Ref
		}
		return s
	}
	s := resolve(schema)
	for _, k := range where {
		if s == nil {
			return nil
		}
		if _, err := strconv.Atoi(k); err == nil && s.Items2020 != nil {
			s = resolve(s.Items2020)
		} else {
			s = resolve(s.Properties[k])
		}
	}
	if s == nil {
		return nil
	}
	return slices.Sorted(maps.Keys(s.Properties))
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
			run, ok := doc.(map[string]any)
			if !ok || run["step"] != "explore" {
				continue
			}
			if dir == "fixtures/invalid" && (run["inputs"] == nil || refusedAtInputs(documents.Validate(doc))) {
				continue
			}
			checked++
			if err := schema.Validate(run["inputs"]); err != nil {
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
		run, ok := doc.(map[string]any)
		if !ok || run["step"] != "explore" {
			continue
		}
		explore++
		inputs, _ := run["inputs"].(map[string]any)
		if !reflect.DeepEqual(run["target"], inputs["target"]) {
			t.Errorf("%s: target is %v, and inputs.target is %v", f, run["target"], inputs["target"])
		}
		if run["domain"] == nil || run["domain"] != inputs["domain"] {
			t.Errorf("%s: domain is %v, and inputs.domain is %v", f, run["domain"], inputs["domain"])
		}
		item, _ := run["work_item"].(map[string]any)
		inputsItem, _ := inputs["work_item"].(map[string]any)
		if item["id"] == nil || item["id"] != inputsItem["id"] {
			t.Errorf("%s: work_item.id is %v, and inputs.work_item.id is %v", f, item["id"], inputsItem["id"])
		}
	}
	if explore == 0 {
		t.Error("fixtures/documents has no documents for the step explore")
	}
}
