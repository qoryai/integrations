package contracts_test

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// withLinks is a description of the source role with the links member put in.
func withLinks(links string) string {
	return `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0", "settings": {"type": "object"},
 "roles": {"source": {"description": "Reads the open items of Acme projects.", "program": "qory-acme", "setup": {"command": "qory-acme setup", "help": "Asks for the Acme address."},
 "start": "qory-acme serve", "connect": "Acme asks you to sign in.", "access": "Open Acme settings.", "disconnect": "Remove the adapter in Acme.",
 "item_noun": {"one": "issue", "other": "issues"}, "collection_noun": {"one": "project", "other": "projects"}}},
 "links": ` + links + `}`
}

// linkEntries is n entries, each with a system that is the host of its template.
func linkEntries(n int) string {
	var e []string
	for i := range n {
		e = append(e, fmt.Sprintf(`{"system": "host%d.example.com", "template": "https://host%d.example.com/{path}"}`, i, i))
	}
	return "[" + strings.Join(e, ", ") + "]"
}

// oneLink is links with one entry for the system code.example.com and the template.
func oneLink(t *testing.T, template string) string {
	t.Helper()
	b, err := json.Marshal(template)
	if err != nil {
		t.Fatal(err)
	}
	return `[{"system": "code.example.com", "template": ` + string(b) + `}]`
}

// TestTheSchemaTakesTheEdgesOfLinks pins the edges of links that the invalid fixtures
// show on one side: 16 entries and a template of 2048 characters are accepted, and
// each template that the pattern refuses is refused.
func TestTheSchemaTakesTheEdgesOfLinks(t *testing.T) {
	schema := compile(t, "description.schema.json")
	check := func(links string) error {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withLinks(links)))
		if err != nil {
			t.Fatal(err)
		}
		return schema.Validate(doc)
	}
	const host = "https://code.example.com/"
	for name, template := range map[string]string{
		"a template of 2048":              host + strings.Repeat("a", 2048-len(host)),
		"a path of one slash":             host,
		"no placeholder":                  host + "commits",
		"both placeholders":               host + "{collection_path}/commit/{path}",
		"adjacent placeholders":           host + "{collection_path}{path}",
		"a host of one label":             "https://intranet/{path}",
		"a host with a hyphen and digits": "https://a-1.b2.example.com/{path}",
		"U+200D in the path":              host + "a\u200db",
		"a character of 2 bytes":          host + "ä",
		"a percent sign and a space":      host + "100% done",
	} {
		if err := check(oneLink(t, template)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, template := range map[string]string{
		"an upper-case scheme":           "HTTPS://code.example.com/{path}",
		"http":                           "http://code.example.com/{path}",
		"no scheme":                      "code.example.com/{path}",
		"only the scheme":                "https://",
		"no path":                        "https://code.example.com",
		"an upper-case host":             "https://Code.example.com/{path}",
		"user information":               "https://user@code.example.com/{path}",
		"a port":                         "https://code.example.com:8443/{path}",
		"a query":                        host + "x?y={path}",
		"a fragment":                     host + "x#{path}",
		"a final question mark":          host + "{path}?",
		"an empty port":                  "https://code.example.com:/{path}",
		"a port of 6 digits":             "https://code.example.com:123456/{path}",
		"{path} as the host":             "https://{path}",
		"{path} in the host":             "https://{path}.example.com/x",
		"{system} as the host":           "https://{system}/{path}",
		"{system} in the path":           host + "{system}/{path}",
		"{collection_path} as the host":  "https://{collection_path}/{path}",
		"an unknown placeholder":         host + "{id}",
		"a space in a brace":             host + "{ path }",
		"an upper-case placeholder":      host + "{Path}",
		"a lone opening brace":           host + "{path",
		"a lone closing brace":           host + "path}",
		"an empty brace":                 host + "{}",
		"a placeholder in a placeholder": host + "{{path}}",
		"a tab":                          host + "\t{path}",
		"a carriage return":              host + "\r{path}",
		"a final line feed":              host + "{path}\n",
		"U+2028":                         host + "\u2028{path}",
		"U+202E":                         host + "\u202e{path}",
		"U+200B":                         host + "\u200b{path}",
		"U+0085":                         host + "\u0085{path}",
	} {
		if err := check(oneLink(t, template)); err == nil {
			t.Errorf("%s: the schema accepts %q", name, template)
		}
	}
	if err := check(linkEntries(16)); err != nil {
		t.Errorf("16 entries: %v", err)
	}
	if err := check(linkEntries(17)); err == nil {
		t.Error("17 entries are accepted")
	}
}

// TestALinkTemplateRefusesAFinalLineFeedBesideItsPattern pins that $defs/link_template
// has the not that refuses a line feed, as each must_understand item has, besides a
// pattern that starts with ^ and ends with $. An engine that lets $ match before a final
// line feed, such as PCRE, would accept a template with one at its end without the not.
// Go's regexp does not match $ there, so a mutation that drops the not fails only this
// test.
func TestALinkTemplateRefusesAFinalLineFeedBesideItsPattern(t *testing.T) {
	n := node(t, "description.schema.json", "/$defs/link_template")
	pattern, _ := n["pattern"].(string)
	if !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") {
		t.Errorf("the pattern %q does not start with ^ and end with $", pattern)
	}
	not, _ := n["not"].(map[string]any)
	if not["pattern"] != `\n` {
		t.Errorf("the not of $defs/link_template is %v, want a pattern that matches a line feed", n["not"])
	}
}

// TestCheckLinksRefusesWhatTheSchemaCannotSay pins that CheckLinks refuses two entries
// with the same system and an entry whose template has another host than its system,
// which the schema accepts, with the message that states the fix; and that it accepts
// distinct systems with the host of each template equal to its system, no links, and
// every description under fixtures.
func TestCheckLinksRefusesWhatTheSchemaCannotSay(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, tc := range []struct{ name, links, want string }{
		{"a system twice", `[{"system": "code.example.com", "template": "https://code.example.com/{path}"},
 {"system": "desk.example.com", "template": "https://desk.example.com/{path}"},
 {"system": "code.example.com", "template": "https://code.example.com/{collection_path}/{path}"}]`,
			`List each system of links once: the entries 0 and 2 both have the system "code.example.com".`},
		{"another host", `[{"system": "code.example.com", "template": "https://evil.example.com/{path}"}]`,
			`Use the system "code.example.com" as the host of the template of link 0, not "evil.example.com".`},
		{"a longer host", `[{"system": "code.example.com", "template": "https://code.example.com.evil.example/{path}"}]`,
			`Use the system "code.example.com" as the host of the template of link 0, not "code.example.com.evil.example".`},
		{"a shorter host", `[{"system": "code.example.com", "template": "https://example.com/{path}"}]`,
			`Use the system "code.example.com" as the host of the template of link 0, not "example.com".`},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withLinks(tc.links)))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: the schema refuses it, and the rule is CheckLinks's alone: %v", tc.name, err)
		}
		if err := contracts.CheckLinks(doc); err == nil || err.Error() != tc.want {
			t.Errorf("%s: CheckLinks returns %v, want %q", tc.name, err, tc.want)
		}
	}
	for name, links := range map[string]string{
		"two systems": `[{"system": "code.example.com", "template": "https://code.example.com/{path}"}, {"system": "desk.example.com", "template": "https://desk.example.com/{path}"}]`,
		"one system":  `[{"system": "code.example.com", "template": "https://code.example.com/"}]`,
		"16 systems":  linkEntries(16),
	} {
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(withLinks(links)))
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckLinks(d); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := contracts.CheckLinks(map[string]any{"name": "acme"}); err != nil {
		t.Errorf("a description with no links: %v", err)
	}
	for _, f := range files(t, "fixtures") {
		d, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckLinks(d); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// keptCharacters are the characters that the README, section Links, keeps in a written
// segment: a range as two characters with a dash between, and a single character.
const keptCharacters = "A-Z a-z 0-9 - . _ ~"

// keptByTheReadme reports whether c is one of the [keptCharacters].
func keptByTheReadme(c byte) bool {
	for _, k := range strings.Fields(keptCharacters) {
		if len(k) == 3 && c >= k[0] && c <= k[2] || len(k) == 1 && c == k[0] {
			return true
		}
	}
	return false
}

// fillLinkByTheReadme fills template by the steps 2 to 4 of the README, section Links,
// and reports false for no link: each placeholder is replaced in one pass with its value;
// the value is split at each /; with a segment that is empty, . or .. there is no link; each
// segment is written with every byte except A-Z a-z 0-9 - . _ ~ as %XX; the segments are
// joined with /. It exists only in this test, to pin the vectors and the examples of
// the README; no reader of the contract calls it.
func fillLinkByTheReadme(template, collectionPath, path string) (string, bool) {
	ok := true
	link := regexp.MustCompile(`\{(collection_path|path)\}`).ReplaceAllStringFunc(template, func(holder string) string {
		v := path
		if holder == "{collection_path}" {
			v = collectionPath
		}
		var segments []string
		for _, segment := range strings.Split(v, "/") {
			if segment == "" || segment == "." || segment == ".." {
				ok = false
			}
			var b strings.Builder
			for _, c := range []byte(segment) {
				if keptByTheReadme(c) {
					b.WriteByte(c)
				} else {
					fmt.Fprintf(&b, "%%%02X", c)
				}
			}
			segments = append(segments, b.String())
		}
		return strings.Join(segments, "/")
	})
	return link, ok
}

// TestTheReadmeExamplesOfLinksFillAsPrinted pins the section Links of the README: each
// row of its table of written values, and each of its three examples, is what the steps
// of the section give. The inputs and the expected strings of the examples are written
// here exactly as the section prints them, and the test fails when the section does not
// print them. The rows of the table are read from the section.
func TestTheReadmeExamplesOfLinksFillAsPrinted(t *testing.T) {
	b, err := os.ReadFile("integration/v1/README.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, _ := strings.Cut(string(b), "\n## Links\n")
	section, _, _ = strings.Cut(section, "\n## ")
	kept := regexp.MustCompile("(?s)the characters (.*?) stay").FindStringSubmatch(section)
	if kept == nil {
		t.Fatal("the section Links does not list the characters that stay")
	}
	var spans []string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(kept[1], -1) {
		spans = append(spans, m[1])
	}
	if got := strings.Join(spans, " "); got != keptCharacters {
		t.Errorf("the section Links keeps the characters %q, the test helper keeps %q", got, keptCharacters)
	}
	rows := regexp.MustCompile("(?m)^\\| `([^`]*)` \\| (`[^`]*`|no link) \\|$").FindAllStringSubmatch(section, -1)
	if len(rows) != 10 {
		t.Fatalf("the table of written values has %d rows, want 10", len(rows))
	}
	for _, r := range rows {
		got, ok := fillLinkByTheReadme("{path}", "", r[1])
		switch {
		case r[2] == "no link" && ok:
			t.Errorf("the README gives no link for %q, the steps give %q", r[1], got)
		case r[2] != "no link" && (!ok || "`"+got+"`" != r[2]):
			t.Errorf("the README writes %q as %s, the steps give %q, %v", r[1], r[2], got, ok)
		}
	}
	for _, tc := range []struct{ entry, template, collection, path, want string }{
		{`{"system": "code.example.com", "template": "https://code.example.com/{collection_path}/commit/{path}"}`,
			"https://code.example.com/{collection_path}/commit/{path}", "acme/shop", "3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1",
			"https://code.example.com/acme/shop/commit/3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1"},
		{`{"system": "desk.example.com", "template": "https://desk.example.com/conversations/{path}"}`,
			"https://desk.example.com/conversations/{path}", "", "48213", "https://desk.example.com/conversations/48213"},
		{`{"system": "ads.example.com", "template": "https://ads.example.com/console/{collection_path}/keywords/{path}"}`,
			"https://ads.example.com/console/{collection_path}/keywords/{path}", "profile-5521/campaign-9", "30417",
			"https://ads.example.com/console/profile-5521/campaign-9/keywords/30417"},
	} {
		got, ok := fillLinkByTheReadme(tc.template, tc.collection, tc.path)
		if !ok || got != tc.want {
			t.Errorf("%s: the steps give %q, %v, want %q", tc.template, got, ok, tc.want)
		}
		for _, printed := range []string{"`" + tc.entry + "`", "`" + tc.want + "`", "`" + tc.path + "`"} {
			if !strings.Contains(section, printed) {
				t.Errorf("the section Links does not print %s", printed)
			}
		}
		if tc.collection != "" && !strings.Contains(section, "`"+tc.collection+"`") {
			t.Errorf("the section Links does not print the collection path %s", tc.collection)
		}
	}
	for _, tc := range []struct{ name, template, collection, path string }{
		{"an empty collection path", "https://x.example.com/{collection_path}/{path}", "", "1"},
		{"an empty path", "https://x.example.com/{path}", "acme/shop", ""},
		{"a dot segment in the collection path", "https://x.example.com/{collection_path}/{path}", "acme/./shop", "1"},
		{"a leading slash", "https://x.example.com/{path}", "acme/shop", "/1"},
		{"a trailing slash", "https://x.example.com/{path}", "acme/shop", "1/"},
	} {
		if got, ok := fillLinkByTheReadme(tc.template, tc.collection, tc.path); ok {
			t.Errorf("%s: the steps give the link %q, want none", tc.name, got)
		}
	}
	if got, ok := fillLinkByTheReadme("https://x.example.com/{path}", "", "{path}"); !ok || got != "https://x.example.com/%7Bpath%7D" {
		t.Errorf("a brace in a value: %q, %v", got, ok)
	}
	if got, ok := fillLinkByTheReadme("https://x.example.com/a", "", ""); !ok || got != "https://x.example.com/a" {
		t.Errorf("empty values of placeholders the template does not use: %q, %v", got, ok)
	}
}

// TestCheckLinksLeavesAWrongShapeToTheSchema pins that CheckLinks returns nil, and does
// not panic, for a description whose links the schema refuses by type: a document that
// is no object, links that is no array, an entry that is no object, and a system that is
// no string, which does not count as the empty system.
func TestCheckLinksLeavesAWrongShapeToTheSchema(t *testing.T) {
	for name, doc := range map[string]any{
		"no document":        nil,
		"an array":           []any{},
		"links as a string":  map[string]any{"links": "https://x.example.com"},
		"links as an object": map[string]any{"links": map[string]any{"system": "a.example.com"}},
		"entries as strings": map[string]any{"links": []any{"a", "a"}},
		"entries as null":    map[string]any{"links": []any{nil, nil}},
		"systems as numbers": map[string]any{"links": []any{map[string]any{"system": 1}, map[string]any{"system": 1}}},
		"systems missing":    map[string]any{"links": []any{map[string]any{}, map[string]any{}}},
		"systems as null":    map[string]any{"links": []any{map[string]any{"system": nil}, map[string]any{"system": nil}}},
	} {
		if err := contracts.CheckLinks(doc); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	empty := map[string]any{"links": []any{map[string]any{"system": ""}, map[string]any{"system": ""}}}
	if err := contracts.CheckLinks(empty); err == nil {
		t.Error("two empty-string systems are accepted")
	}
}
