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
		"fixtures/invalid/description-bad-name.json":            "/name pattern",
		"fixtures/invalid/description-name-with-dot.json":       "/name pattern",
		"fixtures/invalid/description-credential-no-hosts.json": "/roles/credential required hosts",
		"fixtures/invalid/description-domains-bad-name.json":    "/domains/0 pattern",
		"fixtures/invalid/description-domains-duplicate.json":   "/domains uniqueItems",
		"fixtures/invalid/description-domains-empty.json":       "/domains minItems",
		"fixtures/invalid/description-no-settings.json":         "/ required settings",
		"fixtures/invalid/description-settings-not-object.json": "/settings/type const",
		"fixtures/invalid/description-version-2.json":           "/version const",
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

// leaves are a refusal's reasons, each where, the keyword, and what a required one
// misses.
func leaves(v *jsonschema.ValidationError) []string {
	if len(v.Causes) == 0 {
		r := "/" + strings.Join(v.InstanceLocation, "/") + " " + strings.Join(v.ErrorKind.KeywordPath(), "/")
		if k, ok := v.ErrorKind.(*kind.Required); ok {
			r += " " + strings.Join(k.Missing, ",")
		}
		return []string{r}
	}
	var out []string
	for _, c := range v.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}
