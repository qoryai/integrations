package contracts_test

import (
	"encoding/json"
	"fmt"
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
