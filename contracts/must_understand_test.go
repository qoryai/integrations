package contracts_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// understood is a schema whose reader ignores a member that it does not define: its name,
// the compile functions of the schema and of its closed copy, the files of its valid
// fixtures, the places of those fixtures that define no members, which the closed copy
// leaves open, such as the settings schema of a description and the auth of an API, which
// the Forager contract defines, and, for a schema with must_understand, a valid fixture, a pointer to a
// member of that fixture, and a change that gives a known member a bad value.
type understood struct {
	name     string
	open     func() (*jsonschema.Schema, error)
	closed   func() (*jsonschema.Schema, error)
	read     func(name string) (any, error)
	list     func(t *testing.T) []string
	freePlac *regexp.Regexp
	only     *regexp.Regexp // when set, the places that the schema leaves open; the others are closed in both
	valid    string
	known    string
	invalid  func(m map[string]any) // gives a known member a bad value
}

// understoodSchemas lists them.
func understoodSchemas() []understood {
	runList := func(dir string) func(*testing.T) []string {
		return func(t *testing.T) []string { return runFiles(t, dir) }
	}
	intList := func(dir string) func(*testing.T) []string {
		return func(t *testing.T) []string { return files(t, dir) }
	}
	return []understood{
		{
			name:     "documents.schema.json",
			open:     func() (*jsonschema.Schema, error) { return contracts.CompileRun("documents.schema.json") },
			closed:   func() (*jsonschema.Schema, error) { return contracts.CompileRunClosed("documents.schema.json") },
			read:     contracts.RunDocument,
			list:     runList("fixtures/documents"),
			freePlac: regexp.MustCompile(`^/(inputs|harness/harness/extensions|harness/harness/workflows|harness/harness/bind/(agents|commands|skills))$|^/inputs/|^/harness/harness/extensions/`),
			valid:    "fixtures/documents/explore.json",
			known:    "/domain",
			invalid:  func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name: "next-session-answer.schema.json",
			open: func() (*jsonschema.Schema, error) { return contracts.CompileRun("next-session-answer.schema.json") },
			closed: func() (*jsonschema.Schema, error) {
				return contracts.CompileRunClosed("next-session-answer.schema.json")
			},
			read:     contracts.RunDocument,
			list:     runList("fixtures/next-session-answer"),
			freePlac: regexp.MustCompile(`^/inputs$|^/inputs/`),
			valid:    "fixtures/next-session-answer/session.json",
			known:    "/next",
			invalid:  func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name:     "description.schema.json",
			open:     func() (*jsonschema.Schema, error) { return contracts.Compile("description.schema.json") },
			closed:   func() (*jsonschema.Schema, error) { return contracts.CompileClosed("description.schema.json") },
			read:     contracts.Document,
			list:     intList("fixtures"),
			freePlac: regexp.MustCompile(`^/settings$|^/settings/|^/roles$|^/roles/[^/]+$|^/roles/model_provider/apis/\d+/auth$`),
			valid:    "fixtures/example-tracker.json",
			known:    "/name",
			invalid:  func(m map[string]any) { m["name"] = "Upper Case" },
		},
		{
			name:     "adapter.schema.json",
			open:     func() (*jsonschema.Schema, error) { return contracts.Compile("adapter.schema.json") },
			closed:   func() (*jsonschema.Schema, error) { return contracts.CompileClosed("adapter.schema.json") },
			read:     contracts.Document,
			list:     intList("fixtures/adapter/adapter"),
			only:     regexp.MustCompile(`^/description`),
			freePlac: regexp.MustCompile(`^/description/settings$|^/description/settings/|^/description/roles$|^/description/roles/[^/]+$|^/description/roles/model_provider/apis/\d+/auth$|^/settings$|^/settings/`),
		},
		{
			name:     "changes.schema.json",
			open:     func() (*jsonschema.Schema, error) { return contracts.Compile("changes.schema.json") },
			closed:   func() (*jsonschema.Schema, error) { return contracts.CompileClosed("changes.schema.json") },
			read:     contracts.Document,
			list:     intList("fixtures/adapter/changes"),
			freePlac: regexp.MustCompile(`^/changes/\d+/data$|^/changes/\d+/data/`),
			valid:    "fixtures/adapter/changes/feed.json",
			known:    "/cursor",
			invalid:  func(m map[string]any) { m["cursor"] = "" },
		},
	}
}

// objectPlaces returns the JSON pointer of each object in doc, "" for doc itself.
func objectPlaces(doc any, ptr string) []string {
	var out []string
	switch v := doc.(type) {
	case map[string]any:
		out = append(out, ptr)
		for k, x := range v {
			out = append(out, objectPlaces(x, ptr+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(k))...)
		}
	case []any:
		for i, x := range v {
			out = append(out, objectPlaces(x, fmt.Sprintf("%s/%d", ptr, i))...)
		}
	}
	return out
}

// objectAtPointer returns the object of doc at the JSON pointer ptr.
func objectAtPointer(t *testing.T, doc any, ptr string) map[string]any {
	t.Helper()
	cur := doc
	for _, seg := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		if seg == "" {
			continue
		}
		seg = strings.NewReplacer("~1", "/", "~0", "~").Replace(seg)
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			var i int
			fmt.Sscanf(seg, "%d", &i)
			cur = v[i]
		}
	}
	m, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("the document has no object at %q", ptr)
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

// closedSites are the places where CloseSchema closes each schema of the integration and
// run contracts that a closed copy refers to, by file name: the objects that the schema
// leaves open, and the objects of a shared schema, which stay open in every other
// compilation.
var closedSites = map[string][]string{
	"description.schema.json": {
		"", "/$defs/api", "/$defs/api/properties/endpoint/else", "/$defs/api/properties/oauth_device",
		"/$defs/api/properties/oauth_refresh", "/$defs/apply", "/$defs/guide_part",
		"/properties/egress_hosts/items/else", "/properties/roles/properties/credential",
		"/properties/roles/properties/model_provider",
		"/properties/roles/properties/model_provider/properties/models/items",
		"/properties/roles/properties/source",
		"/properties/roles/properties/source/properties/collection_noun",
		"/properties/roles/properties/source/properties/guide",
		"/properties/roles/properties/source/properties/item_noun",
		"/properties/roles/properties/source/properties/setup",
	},
	"adapter.schema.json":           {},
	"changes.schema.json":           {"", "/$defs/change_request", "/$defs/change_request/properties/previous", "/$defs/collection", "/$defs/comment", "/$defs/grant", "/$defs/sync", "/$defs/watch", "/$defs/work_item"},
	"changes-submitted.schema.json": {},
	"changes-failed.schema.json":    {},
	"definitions.schema.json":       {"/$defs/change_address", "/$defs/listing", "/$defs/person", "/$defs/target", "/$defs/target/properties/address"},
	"documents.schema.json": {
		"", "/$defs/harness", "/$defs/harness_base", "/$defs/harness_module", "/$defs/harness_section",
		"/$defs/harness_section/properties/bind", "/$defs/harness_selection", "/$defs/harness_source",
		"/$defs/harness_workflow", "/$defs/model_choice", "/$defs/prebuild_inputs", "/$defs/run_settings",
		"/$defs/run_settings/properties/provider_models/items", "/$defs/run_settings/properties/tiers",
		"/$defs/subject", "/$defs/submit", "/$defs/tier/else",
	},
	"next-session-answer.schema.json": {"/oneOf/0", "/oneOf/1"},
}

// reachableFiles returns the files of the integration and run contracts that the schema at
// url refers to by $ref, directly or through another file, the file itself included.
func reachableFiles(t *testing.T, url string, seen map[string]bool) {
	t.Helper()
	file, _, _ := strings.Cut(url, "#")
	if seen[file] {
		return
	}
	var name string
	var doc any
	var err error
	switch {
	case strings.HasPrefix(file, contracts.Base+"/"):
		name = strings.TrimPrefix(file, contracts.Base+"/")
		doc, err = contracts.Document(name)
	case strings.HasPrefix(file, contracts.RunBase+"/"):
		name = strings.TrimPrefix(file, contracts.RunBase+"/")
		doc, err = contracts.RunDocument(name)
	default:
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	seen[file] = true
	var walk func(n any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for k, x := range v {
				if ref, ok := x.(string); ok && k == "$ref" {
					target := ref
					if strings.HasPrefix(ref, "#") {
						target = file + ref
					} else if !strings.Contains(ref, "://") {
						target = file[:strings.LastIndex(file, "/")+1] + ref
					}
					reachableFiles(t, target, seen)
				} else {
					walk(x)
				}
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(doc)
}

// TestAClosedCopyClosesTheSitesOfItsSchema pins the places where CloseSchema closes each
// schema of the integration and run contracts that a closed copy compiles, and that the
// closed copies of the schemas reach no other file of those contracts.
func TestAClosedCopyClosesTheSitesOfItsSchema(t *testing.T) {
	reached := map[string]bool{}
	for _, name := range []string{"description", "adapter", "changes", "changes-submitted", "changes-failed"} {
		reachableFiles(t, contracts.Base+"/"+name+".schema.json", reached)
	}
	for _, name := range []string{"documents", "next-session-answer"} {
		reachableFiles(t, contracts.RunBase+"/"+name+".schema.json", reached)
	}
	var names []string
	for file := range reached {
		names = append(names, file[strings.LastIndex(file, "/")+1:])
	}
	slices.Sort(names)
	var pinned []string
	for name := range closedSites {
		pinned = append(pinned, name)
	}
	slices.Sort(pinned)
	if !slices.Equal(names, pinned) {
		t.Errorf("the closed copies compile the files %q, and the test pins the sites of %q", names, pinned)
	}
	for file := range reached {
		name := file[strings.LastIndex(file, "/")+1:]
		read := contracts.Document
		if strings.HasPrefix(file, contracts.RunBase+"/") {
			read = contracts.RunDocument
		}
		doc, err := read(name)
		if err != nil {
			t.Fatal(err)
		}
		_, sites := contracts.CloseSchema(doc)
		slices.Sort(sites)
		if want := closedSites[name]; !slices.Equal(sites, want) && (len(sites) != 0 || len(want) != 0) {
			t.Errorf("%s: the closed copy closes %q, want %q", name, sites, want)
		}
	}
}

// TestAReaderIgnoresAMemberItDoesNotDefine pins, for each schema, the rule of
// must_understand. In each object of each valid fixture, a member that the schema does
// not define is accepted, and the closed copy refuses it, so a misspelled member fails a
// writer's test. The places that define no members are the exception, as freePlac says.
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
		checked := 0
		for _, f := range u.list(t) {
			raw, err := u.read(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, place := range objectPlaces(raw, "") {
				if u.freePlac.MatchString(place) || (u.only != nil && !u.only.MatchString(place)) {
					continue
				}
				doc, _ := u.read(f)
				m := objectAtPointer(t, doc, place)
				if kind, ok := m["kind"].(string); ok && strings.HasPrefix(place, "/changes/") && !slices.Contains(feedKinds, kind) {
					continue
				}
				m["later_member"] = "x"
				if err := open.Validate(doc); err != nil {
					t.Errorf("%s %s: a member of %q that the schema does not define is refused: %v", u.name, f, place, err)
				}
				want := place + " additionalProperties"
				if place == "" {
					want = "/ additionalProperties"
				}
				if got := runRefusal(closed.Validate(doc)); !strings.Contains(got, want) {
					t.Errorf("%s %s: the closed copy refuses a member of %q for %q, want %q", u.name, f, place, got, want)
				}
				checked++
			}
		}
		if checked == 0 {
			t.Errorf("%s: no fixture has an object", u.name)
		}
		if u.valid == "" {
			continue
		}
		doc := u.fixture(t)
		doc["later_member"] = "x"
		doc["must_understand"] = []any{"/later_member"}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 pattern"; !strings.Contains(got, want) {
			t.Errorf("%s: a member that must_understand lists is refused for %q, want %q", u.name, got, want)
		}
		doc = u.fixture(t)
		doc["must_understand"] = []any{u.known}
		for _, s := range []*jsonschema.Schema{open, closed} {
			if err := s.Validate(doc); err != nil {
				t.Errorf("%s: must_understand that lists a member of the schema is refused: %v", u.name, err)
			}
		}
		doc["must_understand"] = []any{u.known, u.known}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand uniqueItems"; !strings.Contains(got, want) {
			t.Errorf("%s: a member listed twice is refused for %q, want %q", u.name, got, want)
		}
		doc["must_understand"] = []any{"/" + strings.Repeat("a", 256)}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 "; !strings.Contains(got, want) {
			t.Errorf("%s: a pointer of 257 characters is refused for %q, want %q", u.name, got, want)
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

// feedKinds are the kinds of entry that changes.schema.json defines.
var feedKinds = []string{"work_item", "comment", "collection", "grant", "sync", "watch", "change_request"}

// TestMustUnderstandListsThePointersOfItsSchema pins that the pattern of each pointer that
// must_understand accepts is the one built from the schema: the members that the schema
// defines, at any depth, from the object that holds the list. It pins too that the Go type
// of the document resolves every such pointer.
func TestMustUnderstandListsThePointersOfItsSchema(t *testing.T) {
	for _, c := range []struct {
		name string
		base string
		read func(string) (any, error)
		open []string
		goes any
	}{
		{"documents.schema.json", contracts.RunBase, contracts.RunDocument, []string{"/inputs", "/harness/harness/extensions"}, contracts.Documents{}},
		{"next-session-answer.schema.json", contracts.RunBase, contracts.RunDocument, []string{"/inputs"}, contracts.NextSessionAnswer{}},
		{"description.schema.json", contracts.Base, contracts.Document, []string{"/settings"}, nil},
		{"changes.schema.json", contracts.Base, contracts.Document, []string{"/changes"}, contracts.Changes{}},
	} {
		raw, err := c.read(c.name)
		if err != nil {
			t.Fatal(err)
		}
		var def map[string]any
		if props, ok := raw.(map[string]any)["properties"].(map[string]any); ok && c.name == "changes.schema.json" {
			def = props["must_understand"].(map[string]any)
		} else {
			def = raw.(map[string]any)["$defs"].(map[string]any)["must_understand"].(map[string]any)
		}
		templates := definedPointers(c.base+"/"+c.name, raw, c.open...)
		if got, want := def["items"].(map[string]any)["pattern"], pointerPattern(templates); got != want {
			t.Errorf("%s: the pattern of must_understand is not the one built from the schema:\n got %v\nwant %v", c.name, got, want)
		}
		if c.goes == nil {
			continue
		}
		for _, tpl := range templates {
			p := strings.NewReplacer("/**", "/0/kind", "/#", "/0", "/*", "/k").Replace(tpl)
			doc := fmt.Sprintf(`{"must_understand": [%q]}`, p)
			if err := contracts.CheckMustUnderstand([]byte(doc), c.goes); err != nil {
				t.Errorf("%s: the Go type %T does not resolve %s: %v", c.name, c.goes, p, err)
			}
		}
	}
}

// TestAnEntryListsThePointersOfItsKind pins, for each kind of entry, that the pattern of
// the must_understand of the entry is the one built from the kind's schema, that the Go
// type of the kind resolves each pointer of it, and that the pattern refuses a pointer to a
// member of another kind.
func TestAnEntryListsThePointersOfItsKind(t *testing.T) {
	raw, err := contracts.Document("changes.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defs := raw.(map[string]any)["$defs"].(map[string]any)
	for _, kind := range feedKinds {
		def := defs[kind].(map[string]any)
		mu := def["properties"].(map[string]any)["must_understand"].(map[string]any)
		pattern := mu["items"].(map[string]any)["pattern"].(string)
		templates := definedPointers(contracts.Base+"/changes.schema.json", def, "/data")
		if want := pointerPattern(templates); pattern != want {
			t.Errorf("%s: the pattern of must_understand is not the one built from the schema:\n got %v\nwant %v", kind, pattern, want)
		}
		for _, tpl := range templates {
			p := strings.NewReplacer("/**", "/x/y", "/#", "/0", "/*", "/k").Replace(tpl)
			entry := fmt.Sprintf(`{"kind": %q, "must_understand": [%q]}`, kind, p)
			if err := contracts.CheckChangeMustUnderstand([]byte(entry)); err != nil {
				t.Errorf("%s: the Go type does not resolve %s: %v", kind, p, err)
			}
		}
		other := "/work_item_id"
		if kind == "work_item" || kind == "watch" {
			other = "/comment_id"
		}
		if regexp.MustCompile(pattern).MatchString(other) {
			t.Errorf("%s: the pattern of must_understand accepts %s", kind, other)
		}
		entry := fmt.Sprintf(`{"kind": %q, "must_understand": [%q]}`, kind, other)
		if err := contracts.CheckChangeMustUnderstand([]byte(entry)); err == nil {
			t.Errorf("%s: CheckChangeMustUnderstand accepts %s", kind, other)
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
		for _, f := range u.list(t) {
			doc, err := u.read(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := closed.Validate(doc); err != nil {
				t.Errorf("%s: the closed copy of %s refuses it: %v", f, u.name, err)
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
		{`{"must_understand": ["/step", "/later_member"], "later_member": 1}`, contracts.Documents{}, `use a reader that defines "/later_member"`},
		{`{"must_understand": ["/subject/language"]}`, contracts.Documents{}, `use a reader that defines "/subject/language"`},
		{`{"must_understand": ["/harness/harness/modules/01/name"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/step/kind"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/must_understand"]}`, contracts.Documents{}, `use a reader that defines "/must_understand"`},
		{`{"must_understand": ["step"]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": [""]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": ["/next", "/inputs"]}`, contracts.NextSessionAnswer{}, ""},
		{`{"must_understand": ["/step"]}`, contracts.NextSessionAnswer{}, `use a reader that defines "/step"`},
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
	feed := contracts.Changes{Version: 1, Revision: 1, Cursor: "c_1", MustUnderstand: []string{"/cursor"}, Changes: []contracts.Change{
		{Kind: contracts.ChangeSync, MustUnderstand: []string{"/interval"}},
		{Kind: contracts.ChangeWatch, WorkItemID: "wi_1", URL: "https://x.example"},
	}}
	out, err = json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"must_understand":["/interval"]`) || !strings.Contains(string(out), `"must_understand":["/cursor"]`) || strings.Count(string(out), "must_understand") != 2 {
		t.Errorf("the feed is written as %s, want must_understand in the answer and in the first entry", out)
	}
	if err := validate(compile(t, "changes.schema.json"), out); err != nil {
		t.Errorf("the feed with must_understand: %v", err)
	}
}

// TestAnAdapterSkipsAnEntryOfAnUnknownKind pins the feed rule: changes.schema.json accepts
// an entry of a kind that it does not define, with any members, so an adapter reads its
// kind, skips it and moves on; it refuses a kind that is not snake_case; and an entry of a
// defined kind ignores a member that it does not define, as the rule of must_understand says,
// and keeps its rules for the members that it defines.
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
		{"an unknown kind with must_understand", `[{"kind": "settings", "must_understand": ["/later"]}]`, true},
		{"a kind in upper case", `[{"kind": "Settings"}]`, false},
		{"a kind of 64 characters", `[{"kind": "` + strings.Repeat("a", 64) + `"}]`, true},
		{"a kind of 65 characters", `[{"kind": "` + strings.Repeat("a", 65) + `"}]`, false},
		{"a kind that is not a string", `[{"kind": 7}]`, false},
		{"no kind", `[{"interval": 600}]`, false},
		{"a defined kind with a member it does not define", `[{"kind": "sync", "interval": 600, "later": 1}]`, true},
		{"a defined kind with a bad value", `[{"kind": "sync", "interval": 30}]`, false},
		{"a defined kind without a member it requires", `[{"kind": "sync"}]`, false},
	} {
		if err := validate(schema, feed(c.entries)); (err == nil) != c.accepted {
			t.Errorf("%s: accepted %v, want %v: %v", c.name, err == nil, c.accepted, err)
		}
	}
}

// TestAnEntryListsWhatTheAdapterMustUnderstand pins the rule of must_understand for the
// feed: an entry, and the answer, ignore a member that they list nowhere; an entry whose
// must_understand lists a pointer that its kind defines passes; a pointer that its kind does
// not define fails the schema; below data of a change_request entry, any pointer passes the
// schema; and CheckChangeMustUnderstand tells an adapter to skip an entry that it cannot
// meet, with a message that starts with the fix.
func TestAnEntryListsWhatTheAdapterMustUnderstand(t *testing.T) {
	schema := compile(t, "changes.schema.json")
	feed := func(entry string) []byte {
		return []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [` + entry + `]}`)
	}
	changeRequest := func(mu string) string {
		return `{"kind": "change_request", "change_request_id": "av_0123456789abcdef", "change_kind": "pull-request", ` +
			`"target": {"collection": "R_1001"}, "title": "t", "text": "x", "data": {"a": 1}, "must_understand": ` + mu + `}`
	}
	for _, c := range []struct {
		name, entry string
		schemaOK    bool
		checkErr    string
	}{
		{"a listed member of the kind", `{"kind": "sync", "interval": 600, "must_understand": ["/interval"]}`, true, ""},
		{"a listed member of another kind", `{"kind": "sync", "interval": 600, "must_understand": ["/url"]}`, false, `Use an adapter that defines "/url"`},
		{"a member that the kind does not define, not listed", `{"kind": "sync", "interval": 600, "later": 1}`, true, ""},
		{"an empty list", `{"kind": "sync", "interval": 600, "must_understand": []}`, true, ""},
		{"a list that is null", `{"kind": "sync", "interval": 600, "must_understand": null}`, false, "not null"},
		{"a member listed twice", `{"kind": "sync", "interval": 600, "must_understand": ["/kind", "/kind"]}`, false, `list "/kind" once`},
		{"a name, not a pointer", `{"kind": "sync", "interval": 600, "must_understand": ["interval"]}`, false, "starts with /"},
		{"must_understand itself", `{"kind": "sync", "interval": 600, "must_understand": ["/must_understand"]}`, false, `Use an adapter that defines "/must_understand"`},
		{"a nested member of a change request", changeRequest(`["/target/address/path", "/previous/title"]`), true, ""},
		{"a member of data", changeRequest(`["/data/a"]`), true, ""},
		{"a member of data that the entry lacks", changeRequest(`["/data/later/deep"]`), true, ""},
		{"a member of the target that it does not define", changeRequest(`["/target/later"]`), false, `Use an adapter that defines "/target/later"`},
		{"a member of a work item in data", `{"kind": "work_item", "work_item_id": "wi_1", "ref": "r", "status": "taken_in", "url": "https://x.example", "must_understand": ["/data/a"]}`, false, `Use an adapter that defines "/data/a"`},
	} {
		if err := validate(schema, feed(c.entry)); (err == nil) != c.schemaOK {
			t.Errorf("%s: the schema accepts it: %v, want %v: %v", c.name, err == nil, c.schemaOK, err)
		}
		err := contracts.CheckChangeMustUnderstand([]byte(c.entry))
		switch {
		case c.checkErr == "" && err != nil:
			t.Errorf("%s: CheckChangeMustUnderstand: %v", c.name, err)
		case c.checkErr != "" && (err == nil || !strings.Contains(err.Error(), c.checkErr)):
			t.Errorf("%s: CheckChangeMustUnderstand: got %v, want an error with %q", c.name, err, c.checkErr)
		}
	}
	if err := contracts.CheckChangeMustUnderstand([]byte(`{"kind": "settings", "must_understand": ["/later"]}`)); err == nil || !strings.HasPrefix(err.Error(), "skip the entry and acknowledge it") {
		t.Errorf("an entry of an unknown kind: %v, want an error that starts with skip the entry and acknowledge it", err)
	}
	// The failure report of an entry that the adapter cannot meet has the text of the error as
	// its detail, and the schema accepts it.
	entry := changeRequest(`["/target/later"]`)
	err := contracts.CheckChangeMustUnderstand([]byte(entry))
	report, rerr := contracts.Document("fixtures/adapter/changes-failed/change-request-must-understand.json")
	if rerr != nil || err == nil {
		t.Fatalf("the fixture: %v, the check: %v", rerr, err)
	}
	if got := report.(map[string]any)["detail"]; got != err.Error() {
		t.Errorf("the detail of the fixture is %q, and CheckChangeMustUnderstand returns %q", got, err)
	}
	if got := report.(map[string]any)["reason"]; got != "refused" {
		t.Errorf("the reason of the fixture is %v, want refused", got)
	}
	root := func(mu string) []byte {
		return []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [], "must_understand": ` + mu + `}`)
	}
	for _, c := range []struct {
		mu string
		ok bool
	}{{`["/cursor", "/changes"]`, true}, {`["/changes/0/kind"]`, true}, {`["/later"]`, false}} {
		if err := validate(schema, root(c.mu)); (err == nil) != c.ok {
			t.Errorf("the answer lists %s: accepted %v, want %v: %v", c.mu, err == nil, c.ok, err)
		}
	}
	if err := validate(schema, []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [], "later": 1}`)); err != nil {
		t.Errorf("an answer with a member that the schema does not define: %v", err)
	}
}

// TestTheRepliesStayClosedAtTheirRoot pins that changes-submitted.schema.json and
// changes-failed.schema.json, which the adapter writes and the control plane reads from an
// adapter of a revision that it serves, refuse a member of the reply that they do not
// define, and that their closed copy follows the references into definitions.schema.json:
// the target of a reply ignores an undefined member, and the closed copy refuses it.
func TestTheRepliesStayClosedAtTheirRoot(t *testing.T) {
	for _, c := range []struct{ name, fixture string }{
		{"changes-submitted", "fixtures/adapter/changes-submitted/pull-request-created.json"},
		{"changes-failed", "fixtures/adapter/changes-failed/change-request-refused.json"},
	} {
		doc, err := contracts.Document(c.fixture)
		if err != nil {
			t.Fatal(err)
		}
		open := compile(t, c.name+".schema.json")
		closed, err := contracts.CompileClosed(c.name + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := closed.Validate(doc); err != nil {
			t.Errorf("%s: the closed copy refuses %s: %v", c.name, c.fixture, err)
		}
		doc.(map[string]any)["later"] = "x"
		if got := runRefusal(open.Validate(doc)); got != "/ additionalProperties" {
			t.Errorf("%s: the reply with a member that it does not define is refused for %q", c.name, got)
		}
		delete(doc.(map[string]any), "later")
		doc.(map[string]any)["target"].(map[string]any)["later"] = "x"
		if err := open.Validate(doc); err != nil {
			t.Errorf("%s: a target with a member that the schema does not define is refused: %v", c.name, err)
		}
		if got := runRefusal(closed.Validate(doc)); !strings.Contains(got, "/target additionalProperties") {
			t.Errorf("%s: the closed copy refuses the target for %q", c.name, got)
		}
	}
}

// TestOnlyTheClosableSchemasHaveAClosedCopy pins that CompileClosed and CompileRunClosed
// return an error for a schema other than the ones that a reader reads leniently: the
// closed copy of a schema that lists only some members of an object, such as the
// next-session refusal, refuses members that it defines.
func TestOnlyTheClosableSchemasHaveAClosedCopy(t *testing.T) {
	for _, name := range []string{"description.schema.json", "adapter.schema.json", "changes.schema.json",
		"changes-submitted.schema.json", "changes-failed.schema.json", "definitions.schema.json",
		"read.schema.json", "description.schema.json#/$defs/api"} {
		_, err := contracts.CompileClosed(name)
		want := !strings.HasPrefix(name, "definitions") && !strings.HasPrefix(name, "read") && !strings.Contains(name, "#")
		if (err == nil) != want {
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
