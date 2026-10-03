package conformance_test

import (
	"encoding/json"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/conformance"
	"github.com/qoryai/integrations/contracts"
)

// fixtures are the contents of the contract's fixtures directly under dir, by path.
func fixtures(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := fs.ReadDir(contracts.FS, dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := fs.ReadFile(contracts.FS, path.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[path.Join(dir, e.Name())] = b
	}
	if len(out) == 0 {
		t.Fatalf("%s contains no fixture", dir)
	}
	return out
}

// TestEveryAcceptedFixtureConforms pins that each description the contract accepts
// passes Description, the secret rule the schema cannot express among its checks.
func TestEveryAcceptedFixtureConforms(t *testing.T) {
	for name, b := range fixtures(t, "fixtures") {
		if err := conformance.Description(b); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestEveryRefusedFixtureDoesNot pins that each description the contract's schema
// refuses fails Description too.
func TestEveryRefusedFixtureDoesNot(t *testing.T) {
	for name, b := range fixtures(t, "fixtures/invalid") {
		if err := conformance.Description(b); err == nil {
			t.Errorf("%s conforms", name)
		}
	}
}

const tracker = `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "publisher": {"name": "Acme"}, "program_version": "0.1.0",
 "settings": %s,
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": %l}}}
`

// describe is a description with the settings given and a credential role that lists
// each of their top-level properties but a secret's <name>_file.
func describe(settings string) []byte {
	var s struct {
		Properties map[string]struct {
			WriteOnly any `json:"writeOnly"`
		} `json:"properties"`
	}
	_ = json.Unmarshal([]byte(settings), &s)
	list := []string{}
	for name := range s.Properties {
		if secret, ok := strings.CutSuffix(name, "_file"); ok && s.Properties[secret].WriteOnly == true {
			continue
		}
		list = append(list, name)
	}
	slices.Sort(list)
	l, _ := json.Marshal(list)
	return []byte(strings.NewReplacer("%s", settings, "%l", string(l)).Replace(tracker))
}

// roles is a description with the settings and the roles given.
func roles(settings, roles string) []byte {
	return []byte(`{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "publisher": {"name": "Acme"}, "program_version": "0.1.0",
 "settings": ` + settings + `, "roles": {` + roles + `}}`)
}

// TestARoleListsTheSettingsItNeeds pins the rules on the settings a credential or a tool
// role lists and requires that the schema cannot express: a role lists settings the
// settings define, a secret by its name and never by its <name>_file, it requires only
// settings it lists, and some role lists every secret.
func TestARoleListsTheSettingsItNeeds(t *testing.T) {
	const settings = `{"type": "object", "properties": {
	 "url": {"type": "string"}, "project": {"type": "string"},
	 "api_key": {"title": "API key", "type": "string", "writeOnly": true}, "api_key_file": {"type": "string"},
	 "mcp_key": {"title": "MCP key", "type": "string", "writeOnly": true}, "mcp_key_file": {"type": "string"}}}`
	credential := func(list, required string) string {
		return `"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ` + list + `, "required": ` + required + `}`
	}
	tool := func(list, required string) string {
		return `"tool": {"serves": ["mcp.tracker.acme.example"], "settings": ` + list + `, "required": ` + required + `}`
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
		want   string
	}{
		{"a setting the settings do not define", roles(settings, credential(`["url", "api_key", "team"]`, `[]`)+", "+tool(`["mcp_key"]`, `[]`)), "the role credential lists the setting team, which the settings do not define"},
		{"a secret's file", roles(settings, credential(`["url", "api_key"]`, `[]`)+", "+tool(`["mcp_key_file"]`, `[]`)), "the role tool lists mcp_key_file, the file of the secret mcp_key; it lists the secret as mcp_key"},
		{"a secret no role lists", roles(settings, credential(`["url", "api_key"]`, `[]`)+", "+tool(`["url"]`, `[]`)), "the secret mcp_key is listed by no role"},
		{"a secret a reserved role alone lists", roles(settings, credential(`["url", "api_key"]`, `[]`)+`, "work_source": {"settings": ["mcp_key"]}`), "the secret mcp_key is listed by no role"},
		{"a required setting the role does not list", roles(settings, credential(`["url", "api_key"]`, `["url", "api_key"]`)+", "+tool(`["mcp_key"]`, `["url", "mcp_key"]`)), "the role tool requires url, which its settings do not list"},
		{"a required secret's file", roles(settings, credential(`["url", "api_key"]`, `["api_key_file"]`)+", "+tool(`["mcp_key"]`, `[]`)), "the role credential requires api_key_file, which its settings do not list"},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
	}{
		{"a secret per role and a shared setting", roles(settings, credential(`["url", "api_key"]`, `["url", "api_key"]`)+", "+tool(`["url", "mcp_key", "project"]`, `["url", "mcp_key"]`))},
		{"both secrets in one role", roles(settings, credential(`["url", "api_key", "mcp_key"]`, `["api_key"]`)+", "+tool(`[]`, `[]`))},
		{"a tool alone", roles(settings, tool(`["url", "api_key", "mcp_key"]`, `["url"]`))},
		{"no required", roles(settings, `"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ["url", "api_key", "mcp_key"]}`)},
		{"a required in a setting", roles(`{"type": "object", "properties": {"auth": {"type": "object", "required": ["user"]}}}`, credential(`["auth"]`, `["auth"]`))},
		{"no settings", roles(`{"type": "object"}`, credential(`[]`, `[]`))},
	} {
		if err := conformance.Description(tc.stdout); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// allowed is the list of the keywords the settings' top level may carry, as Description
// names it.
const allowed = "the top level may carry only type, properties, patternProperties, additionalProperties, unevaluatedProperties, propertyNames, title, description, $comment, $schema, $id, $defs, which no role's subset of the settings can break"

// TestSettingsTopLevelIsAnAllowlist pins that Description names each keyword the
// settings' top level carries outside the list it may carry, since a role's document
// holds a subset of the settings, in sorted order and beside the schema's refusal, and
// accepts every keyword of the list and any keyword nested in a property.
func TestSettingsTopLevelIsAnAllowlist(t *testing.T) {
	for _, tc := range []struct{ keyword, member string }{
		{"required", `"required": ["url"]`},
		{"required", `"required": []`},
		{"allOf", `"allOf": [{}]`},
		{"anyOf", `"anyOf": [{}]`},
		{"oneOf", `"oneOf": [{"required": ["url"]}, {"required": ["url_file"]}]`},
		{"not", `"not": false`},
		{"if", `"if": {}`},
		{"then", `"then": {}`},
		{"else", `"else": {}`},
		{"dependentRequired", `"dependentRequired": {"url": ["project"]}`},
		{"dependentSchemas", `"dependentSchemas": {"url": {}}`},
		{"minProperties", `"minProperties": 1`},
		{"maxProperties", `"maxProperties": 2`},
		{"$ref", `"$ref": "#/$defs/base", "$defs": {"base": {}}`},
		{"$dynamicRef", `"$dynamicRef": "#base"`},
		{"const", `"const": {"url": "https://tracker.acme.example"}`},
		{"enum", `"enum": [{"url": "https://tracker.acme.example"}]`},
		{"default", `"default": {"project": "WEB"}`},
		{"format", `"format": "uri"`},
	} {
		stdout := roles(`{"type": "object", `+tc.member+`, "properties": {"url": {"type": "string"}, "project": {"type": "string"}}}`,
			`"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ["url", "project"]}`)
		err := conformance.Description(stdout)
		want := "describe: the settings have the top-level keyword " + tc.keyword + "; " + allowed + "; the contract's schema refuses the description"
		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("%s: %v, want %q", tc.member, err, want)
		}
	}
	two := roles(`{"type": "object", "required": ["url"], "minProperties": 1, "properties": {"url": {"type": "string"}}}`,
		`"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ["url"]}`)
	want := "describe: the settings have the top-level keyword minProperties; the settings have the top-level keyword required; " + allowed + "; "
	if err := conformance.Description(two); err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("two keywords: %v, want %q", err, want)
	}
	every := roles(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "$id": "https://acme.example/settings",
	 "$comment": "The tracker's settings.", "type": "object", "title": "Settings", "description": "The tracker's settings.",
	 "additionalProperties": false, "unevaluatedProperties": false, "propertyNames": {"pattern": "^[a-z_]+$"},
	 "patternProperties": {"^x_": {"type": "string"}}, "$defs": {"auth": {"type": "object"}},
	 "properties": {"auth": {"type": "object", "required": ["user"], "oneOf": [{}], "minProperties": 1, "$ref": "#/$defs/auth",
	                         "const": {"user": "dev"}, "default": {"user": "dev"}, "format": "uri"}}}`,
		`"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ["auth"], "required": ["auth"]}`)
	if err := conformance.Description(every); err != nil {
		t.Errorf("every keyword of the list, and others nested in a property: %v", err)
	}
}

// TestADescriptionReportsEveryRuleInOrder pins that Description reports every rule
// beyond the schema a description breaks, in one order: the secrets by name, then the
// roles, credential before tool, each in the order it lists them, then the hosts and the
// mcp. A description with no roles is the schema's to refuse.
func TestADescriptionReportsEveryRuleInOrder(t *testing.T) {
	stdout := roles(`{"type": "object", "properties": {
	 "b_key": {"title": "B key", "type": "string", "writeOnly": true}, "b_key_file": {"type": "string"},
	 "a_key": {"type": "string", "writeOnly": true}, "a_key_file": {"type": "string"}}}`,
		`"tool": {"serves": ["*.acme.example"], "mcp": "https://MCP.acme.example/mcp", "settings": ["zeta", "a_key_file"], "required": ["b_key"]},
		 "credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example", "acme.example"], "settings": ["omega"]}`)
	want := "describe: " + strings.Join([]string{
		"the secret a_key has no title",
		"the role credential lists the setting omega, which the settings do not define",
		"the role tool lists the setting zeta, which the settings do not define",
		"the role tool lists a_key_file, the file of the secret a_key; it lists the secret as a_key",
		"the role tool requires b_key, which its settings do not list",
		"the secret a_key is listed by no role",
		"the secret b_key is listed by no role",
		"the host tracker.acme.example of the role credential and the host *.acme.example the role tool serves overlap",
		`the role tool's mcp "https://MCP.acme.example/mcp" has the host "MCP.acme.example", which is not a lower-case host name`,
	}, "; ")
	for range 10 {
		if err := conformance.Description(stdout); err == nil || err.Error() != want {
			t.Fatalf("%v, want %q", err, want)
		}
	}
	err := conformance.Description([]byte(`{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "publisher": {"name": "Acme"}, "program_version": "0.1.0", "settings": {"type": "object"}}`))
	if err == nil || !strings.Contains(err.Error(), "the contract's schema refuses the description") || !strings.Contains(err.Error(), "roles") {
		t.Errorf("no roles: %v, want the schema's refusal of a missing roles", err)
	}
}

// TestACredentialAndAToolShareNoHost pins that the credential role's hosts and the tool
// role's serves do not overlap, as the runner's egress grammar reads them: the same
// host, or a *. pattern over the other at any depth. A pattern does not cover the name
// it is below.
func TestACredentialAndAToolShareNoHost(t *testing.T) {
	both := func(hosts, serves string) []byte {
		return roles(`{"type": "object"}`, `"credential": {"argument": "[A-Z]+", "hosts": `+hosts+`, "settings": []},
		 "tool": {"serves": `+serves+`, "settings": []}`)
	}
	for _, tc := range []struct {
		name          string
		hosts, serves string
		want          string
	}{
		{"the same host", `["tracker.acme.example"]`, `["tracker.acme.example"]`, "the host tracker.acme.example of the role credential and the host tracker.acme.example the role tool serves overlap"},
		{"a pattern over a served host", `["*.acme.example"]`, `["mcp.tracker.acme.example"]`, "the host *.acme.example of the role credential and the host mcp.tracker.acme.example the role tool serves overlap"},
		{"a served pattern over a host", `["tracker.acme.example"]`, `["*.acme.example"]`, "the host tracker.acme.example of the role credential and the host *.acme.example the role tool serves overlap"},
		{"a pattern over a pattern", `["*.acme.example"]`, `["*.tracker.acme.example"]`, "the host *.acme.example of the role credential and the host *.tracker.acme.example the role tool serves overlap"},
	} {
		err := conformance.Description(both(tc.hosts, tc.serves))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct{ name, hosts, serves string }{
		{"two hosts", `["tracker.acme.example"]`, `["mcp.acme.example"]`},
		{"a pattern and the name it is below", `["*.acme.example"]`, `["acme.example"]`},
		{"a name and a pattern below it", `["acme.example"]`, `["*.acme.example"]`},
		{"a suffix that is not a label", `["*.acme.example"]`, `["notacme.example"]`},
	} {
		if err := conformance.Description(both(tc.hosts, tc.serves)); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// TestAToolsMCPIsOnAHostItServes pins the rules on the tool role's mcp that the schema
// cannot express: an https URL with no userinfo, port or fragment, on a host the tool
// serves, the same or below a *. pattern.
func TestAToolsMCPIsOnAHostItServes(t *testing.T) {
	tool := func(serves, mcp string) []byte {
		return roles(`{"type": "object"}`, `"tool": {"serves": `+serves+`, "mcp": "`+mcp+`", "settings": []}`)
	}
	for _, tc := range []struct{ name, serves, mcp, want string }{
		{"another host", `["mcp.acme.example"]`, "https://tracker.acme.example/mcp", `the role tool's mcp "https://tracker.acme.example/mcp" is on the host "tracker.acme.example", which the role tool does not serve`},
		{"the name a pattern is below", `["*.acme.example"]`, "https://acme.example/mcp", `is on the host "acme.example", which the role tool does not serve`},
		{"no host", `["mcp.acme.example"]`, "https:///mcp", `the role tool's mcp "https:///mcp" has no host`},
		{"an upper-case host", `["mcp.acme.example"]`, "https://MCP.acme.example/mcp", `the role tool's mcp "https://MCP.acme.example/mcp" has the host "MCP.acme.example", which is not a lower-case host name`},
		{"a trailing dot", `["mcp.acme.example"]`, "https://mcp.acme.example./mcp", `has the host "mcp.acme.example.", which is not a lower-case host name`},
		{"a percent escape", `["mcp.acme.example"]`, "https://mcp%c3%a9.acme.example/mcp", `has the host "mcp%c3%a9.acme.example", which is not a lower-case host name`},
		{"an escape url.Parse refuses", `["mcp.acme.example"]`, "https://mcp%2eacme.example/mcp", `is not an https URL`},
		{"an IPv6 address", `["mcp.acme.example"]`, "https://[2001:db8::1]/mcp", `has the host "[2001:db8::1]", which is not a lower-case host name`},
		{"an IPv6 address and a port", `["mcp.acme.example"]`, "https://[2001:db8::1]:8443/mcp", `has a port; the role tool's mcp "https://[2001:db8::1]:8443/mcp" has the host "[2001:db8::1]"`},
		{"a query on a host it does not serve", `["mcp.acme.example"]`, "https://tracker.acme.example?mcp", `is on the host "tracker.acme.example", which the role tool does not serve`},
		{"userinfo", `["mcp.acme.example"]`, "https://dev@mcp.acme.example/mcp", `the role tool's mcp "https://dev@mcp.acme.example/mcp" has userinfo`},
		{"a port", `["mcp.acme.example"]`, "https://mcp.acme.example:8443/mcp", `the role tool's mcp "https://mcp.acme.example:8443/mcp" has a port`},
		{"an empty port", `["mcp.acme.example"]`, "https://mcp.acme.example:/mcp", `has a port`},
		{"a fragment", `["mcp.acme.example"]`, "https://mcp.acme.example/mcp#tools", `the role tool's mcp "https://mcp.acme.example/mcp#tools" has a fragment`},
		{"an empty fragment", `["mcp.acme.example"]`, "https://mcp.acme.example/mcp#", `has a fragment`},
		{"not a URL", `["mcp.acme.example"]`, "https://mcp.acme.example/%zz", `the role tool's mcp "https://mcp.acme.example/%zz" is not an https URL`},
	} {
		err := conformance.Description(tool(tc.serves, tc.mcp))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct{ name, serves, mcp string }{
		{"a host it serves", `["mcp.acme.example"]`, "https://mcp.acme.example/mcp"},
		{"a host below a pattern", `["*.acme.example"]`, "https://mcp.tracker.acme.example/mcp"},
		{"no path", `["mcp.acme.example"]`, "https://mcp.acme.example"},
		{"a query", `["mcp.acme.example"]`, "https://mcp.acme.example/mcp?project=WEB"},
	} {
		if err := conformance.Description(tool(tc.serves, tc.mcp)); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// TestADescriptionIsOneDocumentWithItsSecretsOnTop pins what Description refuses beyond
// the schema: output that is not one document, and a secret the contract does not allow.
func TestADescriptionIsOneDocumentWithItsSecretsOnTop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stdout []byte
		want   string
	}{
		{"nothing", nil, "not a JSON document"},
		{"two documents", append(describe(`{"type": "object"}`), "{}\n"...), "more than one JSON document"},
		{"a secret without its file", describe(`{"type": "object", "properties": {"token": {"title": "Access token", "type": "string", "writeOnly": true}}}`), "the secret token has no setting token_file"},
		{"a secret in a setting", describe(`{"type": "object", "properties": {"auth": {"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}}}`), "/settings/properties/auth/properties/token is a secret nested in a setting"},
		{"a secret outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "writeOnly": true}}}`), "/settings/$defs/token is a secret outside the settings' properties"},
		{"the settings marked a secret", describe(`{"type": "object", "writeOnly": true}`), "the settings have the top-level keyword writeOnly; " + allowed},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	ok := describe(`{"type": "object", "properties": {"token": {"title": "Access token", "type": "string", "writeOnly": true}, "token_file": {"type": "string"}}}`)
	if err := conformance.Description(ok); err != nil {
		t.Errorf("a secret with its file: %v", err)
	}
}

// TestASecretHasATitleAndAtMostOneName pins the rules on a secret's annotations that the
// schema cannot express: a title, a string that is not only white space, on every
// secret, and an x-secret-name on a secret alone, of its grammar, and on no two secrets.
// A key spelled x-secret-name or writeOnly in data, or as a setting's name, marks
// nothing.
func TestASecretHasATitleAndAtMostOneName(t *testing.T) {
	// secret is a secret <name> with the annotations given and its <name>_file beside it.
	secret := func(name, annotations string) string {
		return `"` + name + `": {"type": "string", "writeOnly": true` + annotations + `}, "` + name + `_file": {"type": "string"}`
	}
	settings := func(props ...string) []byte {
		return describe(`{"type": "object", "properties": {` + strings.Join(props, ", ") + `}}`)
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
		want   string
	}{
		{"no title", settings(secret("token", ``)), "the secret token has no title"},
		{"an empty title", settings(secret("token", `, "title": ""`)), "the secret token has no title"},
		{"a title of white space", settings(secret("token", `, "title": " \t "`)), "the secret token has no title"},
		{"a title that is not a string", settings(secret("token", `, "title": 1`)), "/settings/properties/token/title"},
		{"a name on a setting that is not a secret", settings(`"url": {"type": "string", "x-secret-name": "TRACKER_URL"}`), "the setting url has an x-secret-name and is not a secret"},
		{"a name that is not a string", settings(secret("token", `, "title": "Access token", "x-secret-name": 1`)), "the secret token has an x-secret-name that is not a string"},
		{"a name in lower case", settings(secret("token", `, "title": "Access token", "x-secret-name": "tracker_token"`)), `the secret token has the x-secret-name "tracker_token", which does not match ^[A-Z][A-Z0-9_]{0,127}$`},
		{"a name that starts with a digit", settings(secret("token", `, "title": "Access token", "x-secret-name": "1TOKEN"`)), `the secret token has the x-secret-name "1TOKEN"`},
		{"an empty name", settings(secret("token", `, "title": "Access token", "x-secret-name": ""`)), `the secret token has the x-secret-name ""`},
		{"a name of 129 characters", settings(secret("token", `, "title": "Access token", "x-secret-name": "T`+strings.Repeat("O", 128)+`"`)), "which does not match"},
		{"a name twice", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret", "x-secret-name": "TRACKER_TOKEN"`)), "the secrets token and webhook_secret have the same x-secret-name TRACKER_TOKEN"},
		{"a name in a setting", settings(`"auth": {"type": "object", "properties": {"user": {"type": "string", "x-secret-name": "TRACKER_USER"}}}`), "/settings/properties/auth/properties/user has an x-secret-name nested in a setting"},
		{"a name outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "x-secret-name": "TRACKER_TOKEN"}}}`), "/settings/$defs/token has an x-secret-name outside the settings' properties"},
		{"a name on the settings", describe(`{"type": "object", "x-secret-name": "FOO"}`), "the settings have the top-level keyword x-secret-name; " + allowed},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
	}{
		{"a title and no name", settings(secret("token", `, "title": "Access token"`))},
		{"a title and a name", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`))},
		{"a name of one character", settings(secret("token", `, "title": "Access token", "x-secret-name": "T"`))},
		{"a name of 128 characters", settings(secret("token", `, "title": "Access token", "x-secret-name": "T`+strings.Repeat("O", 127)+`"`))},
		{"two secrets, two names", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret", "x-secret-name": "TRACKER_WEBHOOK_SECRET"`))},
		{"two secrets, one named", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret"`))},
		{"the keys of a default", settings(`"auth": {"type": "object", "default": {"x-secret-name": "a", "writeOnly": true}}`)},
		{"the keys of a const, an enum and an example", settings(`"auth": {"type": "object", "const": {"x-secret-name": "a"}, "enum": [{"writeOnly": true}], "examples": [{"x-secret-name": "a"}]}`)},
		{"a setting named x-secret-name in another", settings(`"auth": {"type": "object", "properties": {"x-secret-name": {"type": "string"}, "writeOnly": true}}`)},
	} {
		if err := conformance.Description(tc.stdout); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// TestCredentialIsTheRunnersDocument pins Credential to the runner's schema: an answer
// it accepts conforms, and one it refuses, or two, do not.
func TestCredentialIsTheRunnersDocument(t *testing.T) {
	answer := `{"version":1,"token":"synthetic","expires_at":"2026-09-25T21:00:00Z","apply":[{"hosts":["tracker.acme.example"],"scheme":"bearer","paths":["/api/*"]}],"placeholders":["TRACKER_TOKEN"]}` + "\n"
	if err := conformance.Credential([]byte(answer)); err != nil {
		t.Errorf("a valid answer: %v", err)
	}
	for _, tc := range []struct{ name, stdout, want string }{
		{"no apply", `{"version":1,"token":"synthetic"}`, "the runner's schema refuses"},
		{"an unknown scheme", `{"version":1,"token":"synthetic","apply":[{"hosts":["a.example"],"scheme":"cookie"}]}`, "the runner's schema refuses"},
		{"two documents", answer + answer, "more than one JSON document"},
	} {
		err := conformance.Credential([]byte(tc.stdout))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestFailureIsOneLineAndNothingElse pins the exit status the contract's commands share.
func TestFailureIsOneLineAndNothingElse(t *testing.T) {
	if err := conformance.Failure(1, nil, []byte("acme-tracker credential: the token file is missing\n")); err != nil {
		t.Errorf("a failure as the contract says: %v", err)
	}
	for _, tc := range []struct {
		name           string
		code           int
		stdout, stderr string
		want           string
	}{
		{"exit 0", 0, "", "failed\n", "exits 0"},
		{"standard output", 1, "{}\n", "failed\n", "on standard output"},
		{"two lines", 1, "", "failed\nbecause\n", "one line"},
		{"no newline", 1, "", "failed", "one line"},
		{"an empty line", 1, "", " \n", "say what failed"},
	} {
		err := conformance.Failure(tc.code, []byte(tc.stdout), []byte(tc.stderr))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}
