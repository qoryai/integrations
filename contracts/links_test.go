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

func linkEntries(n int) string {
	var e []string
	for i := range n {
		e = append(e, fmt.Sprintf(`{"system": "host%d.example.com", "template": "https://{system}/{path}"}`, i))
	}
	return "[" + strings.Join(e, ", ") + "]"
}

// TestTheSchemaTakesTheEdgesOfLinks pins the edges of links that the invalid fixtures
// show on one side: 16 entries and a template of 2048 characters are accepted, and
// each template that the pattern refuses is refused with a reason at the template.
func TestTheSchemaTakesTheEdgesOfLinks(t *testing.T) {
	schema := compile(t, "description.schema.json")
	check := func(links string) error {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withLinks(links)))
		if err != nil {
			t.Fatal(err)
		}
		return schema.Validate(doc)
	}
	one := func(template string) string {
		b, _ := json.Marshal(template)
		return `[{"system": "code.example.com", "template": ` + string(b) + `}]`
	}
	for name, links := range map[string]string{
		"16 entries":                      linkEntries(16),
		"a template of 2048":              one("https://{system}/" + strings.Repeat("a", 2048-len("https://{system}/"))),
		"a template with no placeholder":  one("https://example.com/"),
		"the three placeholders":          one("https://{system}/{collection_path}/commit/{path}?x={path}#{system}"),
		"U+200D in a template":            one("https://example.com/a‍b"),
		"a path of characters of 2 bytes": one("https://example.com/ä"),
	} {
		if err := check(links); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, template := range map[string]string{
		"upper-case scheme":    "HTTPS://{system}/{path}",
		"http":                 "http://{system}/{path}",
		"no scheme":            "{system}/{path}",
		"only the scheme":      "https://",
		"a space in a brace":   "https://{system}/{ path }",
		"an upper-case holder": "https://{System}/{path}",
		"a lone opening brace": "https://{system/{path}",
		"a lone closing brace": "https://{system}/path}",
		"an empty brace":       "https://{system}/{}",
		"a holder in a holder": "https://{system}/{{path}}",
		"a tab":                "https://{system}/\t{path}",
		"a carriage return":    "https://{system}/\r{path}",
		"U+2028":               "https://{system}/ {path}",
		"U+202E":               "https://{system}/‮{path}",
		"U+200B":               "https://{system}/​{path}",
		"U+0085":               "https://{system}/\u0085{path}",
	} {
		if err := check(one(template)); err == nil {
			t.Errorf("%s: the schema accepts %q", name, template)
		}
	}
	if err := check(linkEntries(17)); err == nil {
		t.Error("17 entries are accepted")
	}
}

// TestCheckLinksRefusesASystemTwice pins that CheckLinks refuses two entries with the
// same system, which the schema accepts, and accepts distinct systems, no links, and
// every description under fixtures.
func TestCheckLinksRefusesASystemTwice(t *testing.T) {
	schema := compile(t, "description.schema.json")
	repeated := `[{"system": "code.example.com", "template": "https://{system}/{path}"},
 {"system": "desk.example.com", "template": "https://desk.example.com/{path}"},
 {"system": "code.example.com", "template": "https://{system}/{collection_path}/{path}"}]`
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withLinks(repeated)))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Errorf("the schema refuses it, and the rule is CheckLinks's alone: %v", err)
	}
	want := `List each system of links once: the entries 0 and 2 both have the system "code.example.com".`
	if err := contracts.CheckLinks(doc); err == nil || err.Error() != want {
		t.Errorf("CheckLinks returns %v, want %q", err, want)
	}
	for name, links := range map[string]string{
		"two systems":             `[{"system": "code.example.com", "template": "https://{system}/{path}"}, {"system": "desk.example.com", "template": "https://{system}/{path}"}]`,
		"one system":              `[{"system": "code.example.com", "template": "https://{system}/{path}"}]`,
		"16 systems":              linkEntries(16),
		"the same template twice": `[{"system": "a.example.com", "template": "https://x.example.com/{path}"}, {"system": "b.example.com", "template": "https://x.example.com/{path}"}]`,
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

// fillLinkByTheReadme fills template by the rule of the README, section Links, and
// reports false for no link: each placeholder is replaced in one pass with its value,
// every byte of the value except A-Z a-z 0-9 - . _ ~ and / is written as %XX, and an
// empty value of a placeholder the template uses makes no link. It exists only in this
// test, to pin the examples of the README; no reader of the contract calls it.
func fillLinkByTheReadme(template, system, collectionPath, path string) (string, bool) {
	ok := true
	link := regexp.MustCompile(`\{(system|collection_path|path)\}`).ReplaceAllStringFunc(template, func(holder string) string {
		v := map[string]string{"{system}": system, "{collection_path}": collectionPath, "{path}": path}[holder]
		if v == "" {
			ok = false
		}
		var b strings.Builder
		for _, c := range []byte(v) {
			if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("-._~/", c) >= 0 {
				b.WriteByte(c)
			} else {
				fmt.Fprintf(&b, "%%%02X", c)
			}
		}
		return b.String()
	})
	return link, ok
}

// TestTheReadmeExamplesOfLinksFillAsPrinted pins the section Links of the README: each
// row of its table of written values, and each of its three examples, is what the rule
// of the section gives, and the section prints each template and each link. The
// inputs of the examples are written here, from the same section.
func TestTheReadmeExamplesOfLinksFillAsPrinted(t *testing.T) {
	b, err := os.ReadFile("integration/v1/README.md")
	if err != nil {
		t.Fatal(err)
	}
	section := string(b)
	_, section, _ = strings.Cut(section, "\n## Links\n")
	section, _, _ = strings.Cut(section, "\n## ")
	rows := regexp.MustCompile("(?m)^\\| `([^`]*)` \\| `([^`]*)` \\|$").FindAllStringSubmatch(section, -1)
	if len(rows) != 7 {
		t.Fatalf("the table of written values has %d rows, want 7", len(rows))
	}
	for _, r := range rows {
		if got, ok := fillLinkByTheReadme("{path}", "", "", r[1]); !ok || got != r[2] {
			t.Errorf("the README writes %q as %q, the rule gives %q, %v", r[1], r[2], got, ok)
		}
	}
	for _, tc := range []struct{ template, system, collection, path, want string }{
		{"https://{system}/{collection_path}/commit/{path}", "code.example.com", "acme/shop", "3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1",
			"https://code.example.com/acme/shop/commit/3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1"},
		{"https://{system}/conversations/{path}", "desk.example.com", "", "48213", "https://desk.example.com/conversations/48213"},
		{"https://{system}/console/{collection_path}/keywords/{path}", "ads.example.com", "profile-5521/campaign-9", "30417",
			"https://ads.example.com/console/profile-5521/campaign-9/keywords/30417"},
	} {
		got, ok := fillLinkByTheReadme(tc.template, tc.system, tc.collection, tc.path)
		if !ok || got != tc.want {
			t.Errorf("%s: the rule gives %q, %v, want %q", tc.template, got, ok, tc.want)
		}
		for _, printed := range []string{"\"template\": \"" + tc.template + "\"", "`" + tc.want + "`"} {
			if !strings.Contains(section, printed) {
				t.Errorf("the section Links does not print %s", printed)
			}
		}
	}
	for _, tc := range []struct{ name, template, system, collection, path string }{
		{"an empty system", "https://{system}/{path}", "", "acme/shop", "1"},
		{"an empty collection path", "https://x.example.com/{collection_path}/{path}", "code.example.com", "", "1"},
		{"an empty path", "https://x.example.com/{path}", "code.example.com", "acme/shop", ""},
	} {
		if got, ok := fillLinkByTheReadme(tc.template, tc.system, tc.collection, tc.path); ok {
			t.Errorf("%s: the rule gives the link %q, want none", tc.name, got)
		}
	}
	if got, ok := fillLinkByTheReadme("https://x.example.com/{path}", "", "", "1"); !ok || got != "https://x.example.com/1" {
		t.Errorf("empty values of placeholders the template does not use: %q, %v", got, ok)
	}
}
