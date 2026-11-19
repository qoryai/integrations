package contracts_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// understood are the three schemas whose reader ignores a member it does not define:
// the schema, the compile functions of its schema and of its closed copy, a valid
// fixture, and the places in that fixture, as keys of nested objects, where a member
// may be added. A place of "" is the root.
type understood struct {
	name    string
	open    func() (*jsonschema.Schema, error)
	closed  func() (*jsonschema.Schema, error)
	read    func(name string) (any, error)
	dir     string
	valid   string
	places  []string
	sites   []string
	invalid func(m map[string]any) // gives a known member a bad value
}

// understoodSchemas lists them. The sites of each are the places of its closed copy.
func understoodSchemas() []understood {
	return []understood{
		{
			name:    "documents.schema.json",
			open:    func() (*jsonschema.Schema, error) { return contracts.CompileRun("documents.schema.json") },
			closed:  func() (*jsonschema.Schema, error) { return contracts.CompileRunClosed("documents.schema.json") },
			read:    contracts.RunDocument,
			dir:     "fixtures/documents",
			valid:   "fixtures/documents/explore.json",
			places:  []string{"", "subject"},
			sites:   []string{"", "/$defs/subject"},
			invalid: func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name: "next-session-answer.schema.json",
			open: func() (*jsonschema.Schema, error) { return contracts.CompileRun("next-session-answer.schema.json") },
			closed: func() (*jsonschema.Schema, error) {
				return contracts.CompileRunClosed("next-session-answer.schema.json")
			},
			read:    contracts.RunDocument,
			dir:     "fixtures/next-session-answer",
			valid:   "fixtures/next-session-answer/session.json",
			places:  []string{""},
			sites:   []string{"/oneOf/0", "/oneOf/1"},
			invalid: func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name:    "description.schema.json",
			open:    func() (*jsonschema.Schema, error) { return contracts.Compile("description.schema.json") },
			closed:  func() (*jsonschema.Schema, error) { return contracts.CompileClosed("description.schema.json") },
			read:    contracts.Document,
			dir:     "fixtures",
			valid:   "fixtures/example-tracker.json",
			places:  []string{"", "roles/source"},
			sites:   []string{"", "/properties/roles/properties/source"},
			invalid: func(m map[string]any) { m["name"] = "Upper Case" },
		},
	}
}

// at returns the object of doc at the place, "roles/source" say, "" for doc itself.
func at(t *testing.T, doc any, place string) map[string]any {
	t.Helper()
	m := doc.(map[string]any)
	if place == "" {
		return m
	}
	for _, k := range strings.Split(place, "/") {
		next, ok := m[k].(map[string]any)
		if !ok {
			t.Fatalf("the document has no object at %q", place)
		}
		m = next
	}
	return m
}

// read reads the valid fixture of the schema.
func (u understood) fixture(t *testing.T) map[string]any {
	t.Helper()
	doc, err := u.read(u.valid)
	if err != nil {
		t.Fatal(err)
	}
	return doc.(map[string]any)
}

// TestAClosedCopyClosesTheSitesOfItsSchema pins the places where CloseSchema closes each
// schema that a reader reads leniently: the places that the schema left open, which
// are the root, the subject of the run documents, the two shapes of the next-session
// answer, and the source role of a description.
func TestAClosedCopyClosesTheSitesOfItsSchema(t *testing.T) {
	for _, u := range understoodSchemas() {
		doc, err := u.read(u.name)
		if err != nil {
			t.Fatal(err)
		}
		_, sites := contracts.CloseSchema(doc)
		slices.Sort(sites)
		if !slices.Equal(sites, u.sites) {
			t.Errorf("%s: the closed copy closes %q, want %q", u.name, sites, u.sites)
		}
	}
}

// TestAReaderIgnoresAMemberItDoesNotDefine pins, for each schema, the rule of
// must_understand: a document with a member that the schema does not define is accepted;
// the closed copy refuses it, so a misspelled member fails a writer's test; the document is
// refused when must_understand lists that member, and accepted when must_understand lists
// a member that the schema defines; a member listed twice is refused; and a member that
// the schema defines with a bad value is refused.
func TestAReaderIgnoresAMemberItDoesNotDefine(t *testing.T) {
	for _, u := range understoodSchemas() {
		open, err := u.open()
		if err != nil {
			t.Fatal(err)
		}
		closed, err := u.closed()
		if err != nil {
			t.Fatal(err)
		}
		if err := open.Validate(u.fixture(t)); err != nil {
			t.Fatalf("%s: %v", u.valid, err)
		}
		for _, place := range u.places {
			doc := u.fixture(t)
			at(t, doc, place)["later_member"] = "x"
			if err := open.Validate(doc); err != nil {
				t.Errorf("%s: a member of %q that the schema does not define is refused: %v", u.name, place, err)
			}
			err := closed.Validate(doc)
			if got, want := runRefusal(err), "/"+place+" additionalProperties"; !strings.Contains(got, want) {
				t.Errorf("%s: the closed copy refuses a member of %q for %q, want %q", u.name, place, got, want)
			}
		}
		doc := u.fixture(t)
		doc["later_member"] = "x"
		doc["must_understand"] = []any{"/later_member"}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 pattern"; !strings.Contains(got, want) {
			t.Errorf("%s: a member that must_understand lists is refused for %q, want %q", u.name, got, want)
		}
		doc = u.fixture(t)
		doc["must_understand"] = []any{firstKnown(t, u)}
		for _, s := range []*jsonschema.Schema{open, closed} {
			if err := s.Validate(doc); err != nil {
				t.Errorf("%s: must_understand that lists a member of the schema is refused: %v", u.name, err)
			}
		}
		doc["must_understand"] = []any{firstKnown(t, u), firstKnown(t, u)}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand uniqueItems"; !strings.Contains(got, want) {
			t.Errorf("%s: a member listed twice is refused for %q, want %q", u.name, got, want)
		}
		delete(doc, "must_understand")
		u.invalid(doc)
		if err := open.Validate(doc); err == nil {
			t.Errorf("%s: a bad value of a member that the schema defines is accepted", u.name)
		}
		doc = u.fixture(t)
		doc["must_understand"] = "later_member"
		if err := open.Validate(doc); err == nil {
			t.Errorf("%s: a must_understand that is not a list is accepted", u.name)
		}
	}
}

// firstKnown is a member of the schema that its valid fixture has.
func firstKnown(t *testing.T, u understood) string {
	t.Helper()
	for _, name := range []string{"domain", "next", "name"} {
		if _, ok := u.fixture(t)[name]; ok {
			return "/" + name
		}
	}
	t.Fatalf("%s: the fixture %s has none of the known members", u.name, u.valid)
	return ""
}

// TestMustUnderstandListsThePointersOfItsSchema pins that the pattern of each pointer that
// must_understand accepts is the one built from the schema: the members that the schema
// defines, at any depth, from the object that holds the list. It pins too that the Go type
// of the document resolves every such pointer.
func TestMustUnderstandListsThePointersOfItsSchema(t *testing.T) {
	for _, c := range []struct {
		u    understood
		base string
		open []string
		goes any
	}{
		{understoodSchemas()[0], contracts.RunBase, []string{"/inputs", "/harness/harness/extensions"}, contracts.Documents{}},
		{understoodSchemas()[1], contracts.RunBase, []string{"/inputs"}, contracts.NextSessionAnswer{}},
		{understoodSchemas()[2], contracts.Base, []string{"/settings"}, nil},
	} {
		raw, err := c.u.read(c.u.name)
		if err != nil {
			t.Fatal(err)
		}
		templates := definedPointers(c.base+"/"+c.u.name, raw, c.open...)
		def := raw.(map[string]any)["$defs"].(map[string]any)["must_understand"].(map[string]any)
		if got, want := def["items"].(map[string]any)["pattern"], pointerPattern(templates); got != want {
			t.Errorf("%s: the pattern of must_understand is not the one built from the schema:\n got %v\nwant %v", c.u.name, got, want)
		}
		if c.goes == nil {
			continue
		}
		for _, tpl := range templates {
			p := strings.NewReplacer("/#", "/0", "/*", "/k", "/**", "/x/y").Replace(tpl)
			doc := fmt.Sprintf(`{"must_understand": [%q]}`, p)
			if err := contracts.CheckMustUnderstand([]byte(doc), c.goes); err != nil {
				t.Errorf("%s: the Go type %T does not resolve %s: %v", c.u.name, c.goes, p, err)
			}
		}
	}
}

// TestEveryValidFixtureValidatesAgainstTheClosedCopy pins that each valid fixture of the
// schemas that a reader reads leniently has no member that the schema does not define:
// the closed copy accepts it, and refuses it with one added member, which a writer's test
// finds when it misspells a member.
func TestEveryValidFixtureValidatesAgainstTheClosedCopy(t *testing.T) {
	for _, u := range understoodSchemas() {
		closed, err := u.closed()
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		if u.name == "description.schema.json" {
			names = files(t, u.dir)
		} else {
			names = runFiles(t, u.dir)
		}
		for _, f := range names {
			doc, err := u.read(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := closed.Validate(doc); err != nil {
				t.Errorf("%s: the closed copy of %s refuses it: %v", f, u.name, err)
			}
			doc.(map[string]any)["misspelled"] = "x"
			if got, want := runRefusal(closed.Validate(doc)), "/ additionalProperties"; !strings.Contains(got, want) {
				t.Errorf("%s: the closed copy of %s refuses a misspelled member for %q, want %q", f, u.name, got, want)
			}
		}
	}
}

// TestCheckMustUnderstand pins how a Go reader applies the rule: a document whose
// must_understand lists only members of the type passes, whatever other members it has,
// and a document that lists a member the type does not have, or lists it as a non-string,
// fails.
func TestCheckMustUnderstand(t *testing.T) {
	for _, c := range []struct {
		doc  string
		v    any
		want string
	}{
		{`{"step": "workflow"}`, contracts.Documents{}, ""},
		{`{"later_member": 1}`, &contracts.Documents{}, ""},
		{`{"must_understand": ["/step", "/subject/kind"], "later_member": 1}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/run_settings/runtime", "/harness/harness/modules/0/name"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/inputs/run/attempt", "/harness/harness/extensions/a~1b"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/step", "/later_member"], "later_member": 1}`, contracts.Documents{}, `"/later_member" in must_understand`},
		{`{"must_understand": ["/subject/language"]}`, contracts.Documents{}, `"/subject/language" in must_understand`},
		{`{"must_understand": ["/harness/harness/modules/01/name"]}`, contracts.Documents{}, "in must_understand"},
		{`{"must_understand": ["/step/kind"]}`, contracts.Documents{}, "in must_understand"},
		{`{"must_understand": ["/must_understand"]}`, contracts.Documents{}, `"/must_understand" in must_understand`},
		{`{"must_understand": ["step"]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": [""]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": ["/next", "/inputs"]}`, contracts.NextSessionAnswer{}, ""},
		{`{"must_understand": ["/step"]}`, contracts.NextSessionAnswer{}, `"/step" in must_understand`},
		{`{"must_understand": "step"}`, contracts.Documents{}, "must_understand to a list"},
		{`{"must_understand": null}`, contracts.Documents{}, "list of JSON Pointers"},
		{`{"must_understand": ["/step", "/step"]}`, contracts.Documents{}, `list "/step" once`},
		{`{"must_understand": []}`, contracts.Documents{}, ""},
		{`[]`, contracts.Documents{}, "JSON object"},
	} {
		err := contracts.CheckMustUnderstand([]byte(c.doc), c.v)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", c.doc, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: got %v, want an error with %q", c.doc, err, c.want)
		}
	}
}

// TestMustUnderstandIsWrittenBack pins that a NextSessionAnswer and Documents write their
// MustUnderstand, and that the document they write passes the schema.
func TestMustUnderstandIsWrittenBack(t *testing.T) {
	answerSchema := compileRun(t, "next-session-answer.schema.json")
	for _, name := range []string{"end.json", "session.json"} {
		b, err := fs.ReadFile(contracts.RunFS, "fixtures/next-session-answer/"+name)
		if err != nil {
			t.Fatal(err)
		}
		var a contracts.NextSessionAnswer
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}
		a.MustUnderstand = []string{"/next"}
		out, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"must_understand":["/next"]`) {
			t.Errorf("%s is written as %s, without must_understand", name, out)
		}
		if err := validate(answerSchema, out); err != nil {
			t.Errorf("%s with must_understand: %v", name, err)
		}
	}
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore.json")
	if err != nil {
		t.Fatal(err)
	}
	var d contracts.Documents
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	d.MustUnderstand = []string{"/subject/kind"}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileRun(t, runSchema("documents")), out); err != nil {
		t.Errorf("the documents with must_understand: %v", err)
	}
}

// TestAnAdapterSkipsAnEntryOfAnUnknownKind pins the feed rule: changes.schema.json accepts
// an entry of a kind that it does not define, with any members, so an adapter reads its
// kind, skips it and moves on; it refuses a kind that is not snake_case; and it still
// refuses a member that an entry of a defined kind does not have, since a revision adds
// members.
func TestAnAdapterSkipsAnEntryOfAnUnknownKind(t *testing.T) {
	schema := compile(t, "changes.schema.json")
	feed := func(entries string) []byte {
		return []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": ` + entries + `}`)
	}
	sync := `{"kind": "sync", "interval": 600}`
	for _, c := range []struct {
		name, entries string
		accepted      bool
	}{
		{"an unknown kind with members", `[{"kind": "settings", "interval": 600, "later": {"a": 1}}, ` + sync + `]`, true},
		{"an unknown kind alone", `[{"kind": "settings"}]`, true},
		{"a kind in upper case", `[{"kind": "Settings"}]`, false},
		{"a kind of 64 characters", `[{"kind": "` + strings.Repeat("a", 64) + `"}]`, true},
		{"a kind of 65 characters", `[{"kind": "` + strings.Repeat("a", 65) + `"}]`, false},
		{"a kind that is not a string", `[{"kind": 7}]`, false},
		{"no kind", `[{"interval": 600}]`, false},
		{"a defined kind with a member it lacks", `[{"kind": "sync", "interval": 600, "later": 1}]`, false},
		{"a defined kind with a bad value", `[{"kind": "sync", "interval": 30}]`, false},
	} {
		if err := validate(schema, feed(c.entries)); (err == nil) != c.accepted {
			t.Errorf("%s: accepted %v, want %v: %v", c.name, err == nil, c.accepted, err)
		}
	}
}

// TestOnlyTheOpenedSchemasHaveAClosedCopy pins that CompileClosed and CompileRunClosed
// return an error for a schema other than the three that a reader reads leniently: the
// closed copy of a schema that lists only some members of an object, such as the change
// feed or the next-session refusal, refuses members that it defines.
func TestOnlyTheOpenedSchemasHaveAClosedCopy(t *testing.T) {
	for _, name := range []string{"description.schema.json", "changes.schema.json", "adapter.schema.json"} {
		_, err := contracts.CompileClosed(name)
		if (err == nil) != (name == "description.schema.json") {
			t.Errorf("CompileClosed(%q): %v", name, err)
		}
	}
	for _, name := range []string{"documents.schema.json", "next-session-answer.schema.json",
		"next-session-refusal.schema.json", "outcome.schema.json", "documents.schema.json#/$defs/subject"} {
		_, err := contracts.CompileRunClosed(name)
		want := name == "documents.schema.json" || name == "next-session-answer.schema.json"
		if (err == nil) != want {
			t.Errorf("CompileRunClosed(%q): %v", name, err)
		}
	}
}
