package contracts_test

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// files are the fixtures directly under dir.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := fs.ReadDir(contracts.FS, dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, path.Join(dir, e.Name()))
		}
	}
	if len(out) == 0 {
		t.Fatalf("%s contains no fixture", dir)
	}
	return out
}

// TestDescriptionFixturesValidate pins that each description under fixtures is
// accepted, one with a role the contract does not define among them.
func TestDescriptionFixturesValidate(t *testing.T) {
	schema, err := contracts.Compile("description.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files(t, "fixtures") {
		doc, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// TestInvalidFixturesAreRefused pins that each document under fixtures/invalid fails
// the schema for the reason in its name, and for no other: where, the keyword, and
// the property a required one misses.
func TestInvalidFixturesAreRefused(t *testing.T) {
	schema, err := contracts.Compile("description.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"fixtures/invalid/description-bad-name.json":                  "/name pattern",
		"fixtures/invalid/description-name-with-dot.json":             "/name pattern",
		"fixtures/invalid/description-credential-no-hosts.json":       "/roles/credential required hosts",
		"fixtures/invalid/description-credential-no-settings.json":    "/roles/credential required settings",
		"fixtures/invalid/description-role-required-not-strings.json": "/roles/credential/required/0 type",
		"fixtures/invalid/description-tool-bad-placeholder.json":      "/roles/tool/placeholders/0 pattern",
		"fixtures/invalid/description-tool-extra-member.json":         "/roles/tool additionalProperties hosts",
		"fixtures/invalid/description-tool-mcp-not-https.json":        "/roles/tool/mcp pattern",
		"fixtures/invalid/description-tool-no-serves.json":            "/roles/tool required serves",
		"fixtures/invalid/description-tool-no-settings.json":          "/roles/tool required settings",
		"fixtures/invalid/description-tool-serves-port.json":          "/roles/tool/serves/0 pattern",
		"fixtures/invalid/description-domains-bad-name.json":          "/domains/0 pattern",
		"fixtures/invalid/description-domains-duplicate.json":         "/domains uniqueItems",
		"fixtures/invalid/description-domains-empty.json":             "/domains minItems",
		"fixtures/invalid/description-no-settings.json":               "/ required settings",
		"fixtures/invalid/description-settings-not-object.json":       "/settings/type const",
		"fixtures/invalid/description-settings-required.json":         "/settings additionalProperties required",
		"fixtures/invalid/description-no-publisher.json":              "/ required publisher",
		"fixtures/invalid/description-publisher-empty-name.json":      "/publisher/name pattern",
		"fixtures/invalid/description-publisher-url-not-https.json":   "/publisher/url pattern",
		"fixtures/invalid/description-version-2.json":                 "/version const",
	}
	for _, f := range files(t, "fixtures/invalid") {
		doc, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := schema.Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if got := strings.Join(leaves(v), "; "); got != want[f] {
			t.Errorf("%s is refused for %q, want %q", f, got, want[f])
		}
	}
}

// TestSettingsTopLevelIsAnAllowlist pins that the schema refuses settings whose top
// level carries a keyword outside the list a role's subset of the settings cannot break,
// each alone, and accepts every keyword of the list, and any keyword nested in a
// property.
func TestSettingsTopLevelIsAnAllowlist(t *testing.T) {
	schema, err := contracts.Compile("description.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ keyword, member string }{
		{"required", `"required": ["url"]`}, {"required", `"required": []`}, {"allOf", `"allOf": [{}]`},
		{"anyOf", `"anyOf": [{}]`}, {"oneOf", `"oneOf": [{}]`}, {"not", `"not": false`},
		{"if", `"if": {}`}, {"then", `"then": {}`}, {"else", `"else": {}`},
		{"dependentRequired", `"dependentRequired": {}`}, {"dependentSchemas", `"dependentSchemas": {}`},
		{"minProperties", `"minProperties": 0`}, {"maxProperties", `"maxProperties": 9`},
		{"$ref", `"$ref": "#/$defs/x", "$defs": {"x": {}}`}, {"$dynamicRef", `"$dynamicRef": "#x"`},
		{"const", `"const": {}`}, {"enum", `"enum": [{}]`}, {"default", `"default": {}`},
		{"format", `"format": "uri"`}, {"writeOnly", `"writeOnly": false`}, {"x-secret-name", `"x-secret-name": "FOO"`},
	} {
		v, ok := schema.Validate(describe(t, `{"type": "object", `+tc.member+`}`)).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("settings with %s were accepted", tc.member)
			continue
		}
		want := "/settings additionalProperties " + tc.keyword
		if got := strings.Join(leaves(v), "; "); got != want {
			t.Errorf("settings with %s are refused for %q, want %q", tc.member, got, want)
		}
	}
	allowed := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://acme.example/settings",
	 "$comment": "The tracker's settings.", "type": "object", "title": "Settings", "description": "The tracker's settings.",
	 "properties": {"url": {"type": "string", "anyOf": [{"pattern": "^https://"}], "not": {"const": ""}},
	                "auth": {"type": "object", "required": ["user"], "oneOf": [{}], "minProperties": 1, "$ref": "#/$defs/auth"}},
	 "patternProperties": {"^x_": {"type": "string"}}, "additionalProperties": false, "unevaluatedProperties": false,
	 "propertyNames": {"pattern": "^[a-z_]+$"}, "$defs": {"auth": {"type": "object"}}}`
	if err := schema.Validate(describe(t, allowed)); err != nil {
		t.Errorf("settings with every keyword of the list: %v", err)
	}
}

// TestAPublisherHasANameAndAnHTTPSURL pins the publisher's grammar: a name of 1 to 100
// characters that is not only white space, and an https URL that may be absent.
func TestAPublisherHasANameAndAnHTTPSURL(t *testing.T) {
	schema, err := contracts.Compile("description.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ publisher, want string }{
		{`{"url": "https://acme.example"}`, "/publisher required name"},
		{`{"name": " \t "}`, "/publisher/name pattern"},
		{`{"name": "` + strings.Repeat("A", 101) + `"}`, "/publisher/name maxLength"},
		{`{"name": "Acme", "url": "https:///acme"}`, "/publisher/url pattern"},
		{`{"name": "Acme", "url": "https://acme example"}`, "/publisher/url pattern"},
		{`{"name": "Acme", "email": "dev@acme.example"}`, "/publisher additionalProperties email"},
	} {
		v, ok := schema.Validate(publish(t, tc.publisher)).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("the publisher %s was accepted", tc.publisher)
			continue
		}
		if got := strings.Join(leaves(v), "; "); got != tc.want {
			t.Errorf("the publisher %s is refused for %q, want %q", tc.publisher, got, tc.want)
		}
	}
	for _, publisher := range []string{
		`{"name": "Acme"}`, `{"name": "A"}`, `{"name": "` + strings.Repeat("A", 100) + `"}`,
		`{"name": "Acme", "url": "https://acme.example"}`, `{"name": "Acme", "url": "https://acme.example/tracker?lang=en"}`,
	} {
		if err := schema.Validate(publish(t, publisher)); err != nil {
			t.Errorf("the publisher %s: %v", publisher, err)
		}
	}
}

// describe is a description with the settings given.
func describe(t *testing.T, settings string) any {
	t.Helper()
	return document(t, `{"name": "Acme"}`, settings)
}

// publish is a description with the publisher given.
func publish(t *testing.T, publisher string) any {
	t.Helper()
	return document(t, publisher, `{"type": "object"}`)
}

// document is a description with the publisher and the settings given.
func document(t *testing.T, publisher, settings string) any {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(`{"version": 1, "name": "acme-tracker", "title": "Acme tracker",
	 "publisher": ` + publisher + `, "program_version": "0.1.0", "settings": ` + settings + `,
	 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": []}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// leaves are a refusal's reasons, each where, the keyword, and what a required one
// misses or what an additionalProperties refuses.
func leaves(v *jsonschema.ValidationError) []string {
	if len(v.Causes) == 0 {
		r := "/" + strings.Join(v.InstanceLocation, "/") + " " + strings.Join(v.ErrorKind.KeywordPath(), "/")
		switch k := v.ErrorKind.(type) {
		case *kind.Required:
			r += " " + strings.Join(k.Missing, ",")
		case *kind.AdditionalProperties:
			r += " " + strings.Join(k.Properties, ",")
		}
		return []string{r}
	}
	var out []string
	for _, c := range v.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}
