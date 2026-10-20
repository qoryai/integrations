package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"gopkg.in/yaml.v3"
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
		"fixtures/invalid/description-bad-name.json":                                       "/name pattern",
		"fixtures/invalid/description-name-with-dot.json":                                  "/name pattern",
		"fixtures/invalid/description-credential-no-hosts.json":                            "/roles/credential required hosts",
		"fixtures/invalid/description-credential-hosts-wildcard.json":                      "/roles/credential/hosts/0 pattern; /egress_hosts/0 pattern",
		"fixtures/invalid/description-domains-bad-name.json":                               "/domains/0 pattern",
		"fixtures/invalid/description-domains-duplicate.json":                              "/domains uniqueItems",
		"fixtures/invalid/description-domains-empty.json":                                  "/domains minItems",
		"fixtures/invalid/description-egress-hosts-port.json":                              "/egress_hosts/0 pattern",
		"fixtures/invalid/description-egress-hosts-upper-case.json":                        "/egress_hosts/0 pattern",
		"fixtures/invalid/description-egress-hosts-bare-wildcard.json":                     "/egress_hosts/0 pattern",
		"fixtures/invalid/description-egress-hosts-repeated.json":                          "/egress_hosts uniqueItems",
		"fixtures/invalid/description-egress-hosts-single-label.json":                      "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-wildcard.json":                          "/egress_hosts/0 pattern",
		"fixtures/invalid/description-egress-hosts-wildcard-single-label.json":             "/egress_hosts/0 pattern",
		"fixtures/invalid/description-egress-hosts-localhost.json":                         "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-under-localhost.json":                   "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-loopback.json":                          "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-link-local.json":                        "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-zero-network.json":                      "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-localhost-localdomain.json":             "/egress_hosts/0 ",
		"fixtures/invalid/description-egress-hosts-number-form.json":                       "/egress_hosts/0 ",
		"fixtures/invalid/description-no-settings.json":                                    "/ required settings",
		"fixtures/invalid/description-settings-not-object.json":                            "/settings/type const",
		"fixtures/invalid/description-version-2.json":                                      "/version const",
		"fixtures/invalid/description-source-undefined-member.json":                        "/roles/source additionalProperties",
		"fixtures/invalid/description-source-line-feed.json":                               "/roles/source/connect pattern",
		"fixtures/invalid/description-source-line-too-long.json":                           "/roles/source/access maxLength",
		"fixtures/invalid/description-source-command-non-ascii.json":                       "/roles/source/start pattern",
		"fixtures/invalid/description-source-program-upper-case.json":                      "/roles/source/program pattern",
		"fixtures/invalid/description-source-setup-no-help.json":                           "/roles/source/setup required help",
		"fixtures/invalid/description-source-line-separator.json":                          "/roles/source/disconnect pattern",
		"fixtures/invalid/description-source-bidi-override.json":                           "/roles/source/connect pattern",
		"fixtures/invalid/description-source-arabic-letter-mark.json":                      "/roles/source/access pattern",
		"fixtures/invalid/description-source-zero-width-space.json":                        "/roles/source/description pattern",
		"fixtures/invalid/description-source-byte-order-mark.json":                         "/roles/source/setup/help pattern",
		"fixtures/invalid/description-source-item-noun-upper-case.json":                    "/roles/source/item_noun/one pattern",
		"fixtures/invalid/description-source-item-noun-work-item.json":                     "/roles/source/item_noun/one ",
		"fixtures/invalid/description-source-item-noun-work-items-hyphen.json":             "/roles/source/item_noun/other ",
		"fixtures/invalid/description-source-item-noun-double-space.json":                  "/roles/source/item_noun/one pattern",
		"fixtures/invalid/description-source-item-noun-leading-hyphen.json":                "/roles/source/item_noun/one pattern",
		"fixtures/invalid/description-source-item-noun-33-characters.json":                 "/roles/source/item_noun/one maxLength",
		"fixtures/invalid/description-source-item-noun-empty.json":                         "/roles/source/item_noun/other minLength; /roles/source/item_noun/other pattern",
		"fixtures/invalid/description-source-item-noun-no-other.json":                      "/roles/source/item_noun required other",
		"fixtures/invalid/description-source-item-noun-undefined-member.json":              "/roles/source/item_noun additionalProperties",
		"fixtures/invalid/description-source-item-noun-non-ascii.json":                     "/roles/source/item_noun/one pattern",
		"fixtures/invalid/description-source-collection-noun-upper-case.json":              "/roles/source/collection_noun/one pattern",
		"fixtures/invalid/description-source-collection-noun-double-space.json":            "/roles/source/collection_noun/one pattern",
		"fixtures/invalid/description-source-collection-noun-leading-hyphen.json":          "/roles/source/collection_noun/one pattern",
		"fixtures/invalid/description-source-collection-noun-33-characters.json":           "/roles/source/collection_noun/one maxLength",
		"fixtures/invalid/description-source-collection-noun-empty.json":                   "/roles/source/collection_noun/other minLength; /roles/source/collection_noun/other pattern",
		"fixtures/invalid/description-source-collection-noun-no-other.json":                "/roles/source/collection_noun required other",
		"fixtures/invalid/description-source-collection-noun-undefined-member.json":        "/roles/source/collection_noun additionalProperties",
		"fixtures/invalid/description-source-collection-noun-non-ascii.json":               "/roles/source/collection_noun/one pattern",
		"fixtures/invalid/description-source-collection-noun-work-item.json":               "/roles/source/collection_noun/one ",
		"fixtures/invalid/description-source-collection-noun-work-items-hyphen.json":       "/roles/source/collection_noun/other ",
		"fixtures/invalid/description-source-items-true.json":                              "/roles/source/items const",
		"fixtures/invalid/description-source-items-false-with-item-noun.json":              "/roles/source ",
		"fixtures/invalid/description-settings-enum-1025-entries.json":                     "/settings/properties/level/enum maxItems",
		"fixtures/invalid/description-settings-properties-1025-members.json":               "/settings/properties maxProperties",
		"fixtures/invalid/description-string-16385-characters.json":                        "/description maxLength",
		"fixtures/invalid/description-data-credential.json":                                "/ const",
		"fixtures/invalid/description-data-source.json":                                    "/ const",
		"fixtures/invalid/description-data-unknown-role.json":                              "/ const",
		"fixtures/invalid/description-data-no-role.json":                                   "/roles minProperties",
		"fixtures/invalid/description-data-secret-setting.json":                            "/settings/properties/key/writeOnly ",
		"fixtures/invalid/description-data-secret-in-all-of.json":                          "/settings/properties/url/allOf/0/writeOnly ",
		"fixtures/invalid/description-data-secret-nested.json":                             "/settings/properties/access/properties/key/writeOnly ",
		"fixtures/invalid/description-model-provider-api-undefined-member.json":            "/roles/model_provider/apis/0 additionalProperties",
		"fixtures/invalid/description-model-provider-apis-5.json":                          "/roles/model_provider/apis maxItems",
		"fixtures/invalid/description-model-provider-apis-empty.json":                      "/roles/model_provider/apis minItems",
		"fixtures/invalid/description-model-provider-auth-oauth-false.json":                "/roles/model_provider/apis/0/auth/oauth const",
		"fixtures/invalid/description-model-provider-auth-oauth-with-header.json":          "/roles/model_provider/apis/0/auth ",
		"fixtures/invalid/description-model-provider-effort-empty.json":                    "/roles/model_provider/models/0/effort minItems",
		"fixtures/invalid/description-model-provider-effort-upper-case.json":               "/roles/model_provider/models/0/effort/1 pattern",
		"fixtures/invalid/description-model-provider-endpoint-bare-host.json":              "/roles/model_provider/apis/0/endpoint pattern",
		"fixtures/invalid/description-model-provider-endpoint-fragment.json":               "/roles/model_provider/apis/0/endpoint pattern",
		"fixtures/invalid/description-model-provider-endpoint-http.json":                   "/roles/model_provider/apis/0/endpoint pattern",
		"fixtures/invalid/description-model-provider-endpoint-object-without-setting.json": "/roles/model_provider/apis/0/endpoint required setting; /roles/model_provider/apis/0/endpoint additionalProperties",
		"fixtures/invalid/description-model-provider-endpoint-query.json":                  "/roles/model_provider/apis/0/endpoint pattern",
		"fixtures/invalid/description-model-provider-endpoint-setting-and-path.json":       "/roles/model_provider/apis/0/endpoint additionalProperties",
		"fixtures/invalid/description-model-provider-endpoint-user-information.json":       "/roles/model_provider/apis/0/endpoint pattern",
		"fixtures/invalid/description-model-provider-key-prefixes-33-characters.json":      "/roles/model_provider/apis/0/key_prefixes/0 pattern",
		"fixtures/invalid/description-model-provider-key-prefixes-5-entries.json":          "/roles/model_provider/apis/0/key_prefixes maxItems",
		"fixtures/invalid/description-model-provider-key-prefixes-empty.json":              "/roles/model_provider/apis/0/key_prefixes minItems",
		"fixtures/invalid/description-model-provider-key-prefixes-repeated.json":           "/roles/model_provider/apis/0/key_prefixes uniqueItems",
		"fixtures/invalid/description-model-provider-key-prefixes-space.json":              "/roles/model_provider/apis/0/key_prefixes/0 pattern",
		"fixtures/invalid/description-model-provider-model-id-space.json":                  "/roles/model_provider/models/0/id pattern",
		"fixtures/invalid/description-model-provider-model-undefined-member.json":          "/roles/model_provider/models/0 additionalProperties",
		"fixtures/invalid/description-model-provider-no-apis.json":                         "/roles/model_provider required apis",
		"fixtures/invalid/description-model-provider-role-auth.json":                       "/roles/model_provider additionalProperties",
		"fixtures/invalid/description-model-provider-role-endpoint.json":                   "/roles/model_provider additionalProperties",
		"fixtures/invalid/description-model-provider-role-formats.json":                    "/roles/model_provider additionalProperties",
		"fixtures/invalid/description-model-provider-role-hosts.json":                      "/roles/model_provider additionalProperties",
		"fixtures/invalid/description-model-provider-role-key-variables.json":              "/roles/model_provider additionalProperties",
		"fixtures/invalid/description-model-provider-unknown-format.json":                  "/roles/model_provider/apis/1/format enum",
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
		if got := strings.Join(leaves(v), "; "); unordered(got) != unordered(want[f]) {
			t.Errorf("%s is refused for %q, want %q", f, got, want[f])
		}
	}
}

// TestACollectionNounIsRefusedForWorkItem pins the reason of each invalid description
// whose collection_noun is work item or work items, which the refusal itself does not
// name: the same document is accepted once that word is project or projects.
func TestACollectionNounIsRefusedForWorkItem(t *testing.T) {
	schema, err := contracts.Compile("description.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for name, fix := range map[string][2]string{
		"fixtures/invalid/description-source-collection-noun-work-item.json":         {`"one": "work item"`, `"one": "project"`},
		"fixtures/invalid/description-source-collection-noun-work-items-hyphen.json": {`"other": "work-items"`, `"other": "projects"`},
	} {
		b, err := fs.ReadFile(contracts.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		fixed := bytes.Replace(b, []byte(fix[0]), []byte(fix[1]), 1)
		if bytes.Equal(fixed, b) {
			t.Errorf("%s holds no %s", name, fix[0])
		}
		for _, c := range []struct {
			b      []byte
			accept bool
		}{{b, false}, {fixed, true}} {
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(c.b))
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); (err == nil) != c.accept {
				t.Errorf("%s, accepted %v, want %v: %v", name, err == nil, c.accept, err)
			}
		}
	}
}

// unordered is leaves joined by "; ", sorted, so two refusals compare equal whatever
// the order of their leaves: leaves under different members come in no fixed order.
func unordered(joined string) string {
	l := strings.Split(joined, "; ")
	slices.Sort(l)
	return strings.Join(l, "; ")
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

// TestTheContractsExampleIsWhatAReaderExpands expands the declaration the contract's
// README shows and pins the definitions the README shows for it, whole: both
// integrations, their programs, settings words, arguments and hosts.
func TestTheContractsExampleIsWhatAReaderExpands(t *testing.T) {
	b, err := fs.ReadFile(contracts.FS, "README.md")
	if err != nil {
		t.Fatal(err)
	}
	contract := string(b)
	block := func(after string) string {
		t.Helper()
		_, rest, ok := strings.Cut(contract, after+"\n\n```yaml\n")
		body, _, closed := strings.Cut(rest, "```")
		if !ok || !closed {
			t.Fatalf("the contract's README has no yaml block after %q", after)
		}
		return body
	}
	declared, expanded := block("Declared:"), block("to the gateway's definitions:")
	if got := expand(t, declared); got != expanded {
		t.Errorf("the contract's README declares\n%s\nwhich expands to\n%s\nand the README shows\n%s", declared, got, expanded)
	}
	if !strings.Contains(expanded, `'{"project":"it''s \u0024X"}'`) {
		t.Errorf("the contract's example shows no $ written \\u0024 beside a doubled quote:\n%s", expanded)
	}
}

// described are the descriptions the programs of the contract's example answer, the
// contract's fixtures.
var described = map[string]string{
	"qory-github":                "fixtures/github.json",
	"/opt/acme/bin/acme-tracker": "fixtures/acme-tracker.json",
}

// expand is a declaration as the integration contract's reader expands it (§Declaring an
// integration, step 4), in the order declared: for each key under gateway.integrations,
// the credential of the same key under gateway.credentials, with the adapter
// [<program>, credential, --settings, <json>, --, "${argument}"],
// the program qory-<key> when none is declared, <json> the settings, {} when none are
// declared, as compact JSON with every $ written \u0024, in a single-quoted scalar, and
// the argument and the hosts of the description the program answers.
func expand(t *testing.T, integrations string) string {
	t.Helper()
	var doc struct {
		Gateway struct {
			Integrations yaml.Node `yaml:"integrations"`
		} `yaml:"gateway"`
	}
	if err := yaml.Unmarshal([]byte(integrations), &doc); err != nil || doc.Gateway.Integrations.Kind != yaml.MappingNode {
		t.Fatalf("%v\n%s", err, integrations)
	}
	var b strings.Builder
	b.WriteString("gateway:\n  credentials:\n")
	nodes := doc.Gateway.Integrations.Content
	for i := 0; i+1 < len(nodes); i += 2 {
		key := nodes[i].Value
		var decl struct {
			Program  string         `yaml:"program"`
			Settings map[string]any `yaml:"settings"`
		}
		if err := nodes[i+1].Decode(&decl); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if decl.Program == "" {
			decl.Program = "qory-" + key
		}
		if decl.Settings == nil {
			decl.Settings = map[string]any{}
		}
		var settings bytes.Buffer
		enc := json.NewEncoder(&settings)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(decl.Settings); err != nil {
			t.Fatal(err)
		}
		fixture, err := fs.ReadFile(contracts.FS, described[decl.Program])
		if err != nil {
			t.Fatalf("%s: no description for %s: %v", key, decl.Program, err)
		}
		var d struct {
			Roles struct {
				Credential struct {
					Argument string   `json:"argument"`
					Hosts    []string `json:"hosts"`
				} `json:"credential"`
			} `json:"roles"`
		}
		if err := json.Unmarshal(fixture, &d); err != nil {
			t.Fatal(err)
		}
		word := strings.ReplaceAll(strings.TrimSuffix(settings.String(), "\n"), "$", `\u0024`)
		fmt.Fprintf(&b, `    %s:
      adapter: [%s, credential, --settings, %s, --, "${argument}"]
      argument: %s
      hosts: [%s]
`, key, decl.Program, quote(word), quote(d.Roles.Credential.Argument), strings.Join(d.Roles.Credential.Hosts, ", "))
	}
	return b.String()
}

// quote is YAML's single-quoted scalar, in which nothing is an escape but the quote,
// doubled.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// TestTheReaderRefusesWhatTheSchemaCannotSee pins the rules of description.schema.json
// that a reader checks on a description's raw bytes: each document under
// fixtures/refused-by-reader is accepted by the schema once parsed, since the rule is
// not expressible in one, and breaks the rule on its raw bytes; and every description
// under fixtures keeps the rules.
func TestTheReaderRefusesWhatTheSchemaCannotSee(t *testing.T) {
	want := map[string]string{
		"description-string-invalid-utf-8.json":  "the document is not UTF-8",
		"description-string-lone-surrogate.json": `a string holds the \u escape \ud800 of a lone surrogate`,
		"description-member-repeated.json":       `an object repeats the member "type"`,
	}
	schema := compile(t, "description.schema.json")
	got := files(t, "fixtures/refused-by-reader")
	if len(got) != len(want) {
		t.Errorf("fixtures/refused-by-reader holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		b := read(t, f)
		if err := validate(schema, b); err != nil {
			t.Errorf("%s is refused by description.schema.json, and the rule is the reader's alone: %v", f, err)
		}
		if err := contracts.CheckDescription(b); err == nil || err.Error() != w {
			t.Errorf("%s is refused for %v, want %q", f, err, w)
		}
	}
	for _, f := range files(t, "fixtures") {
		if err := contracts.CheckDescription(read(t, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// TestCheckDescriptionHoldsTheWordedLimits pins each limit description.schema.json
// states in words, at the limit and one past it: the raw bytes, a string's and a member
// name's bytes of UTF-8, counted in bytes and not characters, the values, and the levels
// of nesting.
func TestCheckDescriptionHoldsTheWordedLimits(t *testing.T) {
	e := "é"                       // two bytes of UTF-8
	values := func(n int) string { // the object, its member's array, and n-2 entries
		return `{"a":[` + strings.TrimSuffix(strings.Repeat("0,", n-2), ",") + `]}`
	}
	levels := func(n int) string { // the object and n-1 arrays inside it
		return `{"a":` + strings.Repeat("[", n-1) + strings.Repeat("]", n-1) + `}`
	}
	for _, tc := range []struct{ name, doc, want string }{
		{"262144 bytes", `{"a":1}` + strings.Repeat(" ", 262144-7), ""},
		{"262145 bytes", `{"a":1}` + strings.Repeat(" ", 262145-7), "the document is 262145 bytes, more than 262144 (256 KiB)"},
		{"a string of 16384 bytes", `{"a":"` + strings.Repeat(e, 8192) + `"}`, ""},
		{"a string of 16385 bytes in 8193 characters", `{"a":"` + strings.Repeat(e, 8192) + `x"}`, "a string or a member name is 16385 bytes of UTF-8, more than 16384"},
		{"a member name of 16385 bytes", `{"` + strings.Repeat(e, 8192) + `x":1}`, "a string or a member name is 16385 bytes of UTF-8, more than 16384"},
		{"4096 values", values(4096), ""},
		{"4097 values", values(4097), "the document holds more than 4096 JSON values"},
		{"64 levels", levels(64), ""},
		{"65 levels", levels(65), "the document nests arrays and objects more than 64 levels deep"},
	} {
		err := contracts.CheckDescription([]byte(tc.doc))
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}
}
