package contracts_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// definition compiles one definition of definitions.schema.json by its name under
// $defs, or fails the test.
func definition(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	doc, err := contracts.Document("definitions.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(contracts.Base+"/definitions.schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(contracts.Base + "/definitions.schema.json#/$defs/" + name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// TestTheModelProviderDefinitionsHoldTheirRules pins endpoint_url, model_id and
// effort_value of definitions.schema.json to the rules their texts state, with a value
// at each edge accepted and a value past it refused.
func TestTheModelProviderDefinitionsHoldTheirRules(t *testing.T) {
	for _, tc := range []struct {
		def      string
		accepted []string
		refused  []string
	}{
		{"endpoint_url",
			[]string{
				"https://api.acme.example",
				"https://api.acme.example/",
				"https://api.acme.example/v1/models",
				"https://models.example:1",
				"https://models.example:8443/proxy",
				"https://10.0.0.7:65535",
				"https://acme.example/a%2Fb;v=1/@x",
				"https://localhost",
				"https://" + strings.Repeat("a", 63) + ".example",
			},
			[]string{
				"http://api.acme.example",
				"https://api.acme.example?key=1",
				"https://api.acme.example/v1?key=1",
				"https://user@api.acme.example",
				"https://user:secret@api.acme.example",
				"https://api.acme.example#top",
				"https://API.acme.example",
				"https://models.example:0",
				"https://models.example:65536",
				"https://models.example:08443",
				"https://models.example:",
				"https://",
				"https://-acme.example",
				"https://acme-.example",
				"https://" + strings.Repeat("a", 64) + ".example",
				"https://acme.example/a b",
				"https://acme.example/%zz",
				"https://[::1]",
				"api.acme.example",
				"https://acme.example/" + strings.Repeat("a", 2048-len("https://acme.example/")+1),
			}},
		{"model_id",
			[]string{"acme-large-2", "acme/small-1", "Acme.Large@2026-01-01", "9", strings.Repeat("a", 200)},
			[]string{"", " acme", "acme large", "acme\tlarge", "-acme", "/acme", strings.Repeat("a", 201), "acme-é"}},
		{"effort_value",
			[]string{"low", "xhigh", "level-2", "a", strings.Repeat("a", 32)},
			[]string{"", "High", "1low", "-low", "very_high", "very high", strings.Repeat("a", 33)}},
	} {
		s := definition(t, tc.def)
		for _, v := range tc.accepted {
			if err := s.Validate(v); err != nil {
				t.Errorf("%s refuses %q: %v", tc.def, v, err)
			}
		}
		for _, v := range tc.refused {
			if err := s.Validate(v); err == nil {
				t.Errorf("%s accepts %q", tc.def, v)
			}
		}
	}
}

// TestAModelProviderIsRefusedForTheRuleInItsName pins the reason of each invalid
// model_provider description whose refusal does not name it: the same document is
// accepted once the one part its name points to is fixed.
func TestAModelProviderIsRefusedForTheRuleInItsName(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for name, fix := range map[string][2]string{
		"fixtures/invalid/description-model-provider-unknown-format.json":            {`"acme-chat"`, `"openai-chat"`},
		"fixtures/invalid/description-model-provider-endpoint-http.json":             {`"http://`, `"https://`},
		"fixtures/invalid/description-model-provider-endpoint-query.json":            {`?region=eu`, ``},
		"fixtures/invalid/description-model-provider-endpoint-fragment.json":         {`#models`, ``},
		"fixtures/invalid/description-model-provider-endpoint-user-information.json": {`admin@`, ``},
		"fixtures/invalid/description-model-provider-endpoint-bare-host.json":        {`"api.`, `"https://api.`},
		"fixtures/invalid/description-model-provider-auth-oauth-with-header.json":    {`, "oauth": true`, ``},
		"fixtures/invalid/description-model-provider-key-prefixes-space.json":        {`"acme key-"`, `"acme-key-"`},
		"fixtures/invalid/description-model-provider-key-prefixes-33-characters.json": {
			`"` + strings.Repeat("a", 33) + `"`, `"` + strings.Repeat("a", 32) + `"`},
	} {
		b := read(t, name)
		fixed := bytes.Replace(b, []byte(fix[0]), []byte(fix[1]), 1)
		if bytes.Equal(fixed, b) {
			t.Errorf("%s holds no %s", name, fix[0])
		}
		if err := validate(schema, b); err == nil {
			t.Errorf("%s is accepted", name)
		}
		if err := validate(schema, fixed); err != nil {
			t.Errorf("%s, fixed, is refused: %v", name, err)
		}
	}
}

// TestAModelProviderHoldsItsCaps pins the caps of model_provider at the cap and one
// past it: at most 256 models, and at most 16 effort values for a model.
func TestAModelProviderHoldsItsCaps(t *testing.T) {
	schema := compile(t, "description.schema.json")
	with := func(models, effort int) any {
		t.Helper()
		doc, err := contracts.Document("fixtures/acme-cloud.json")
		if err != nil {
			t.Fatal(err)
		}
		values := make([]any, effort)
		for i := range values {
			values[i] = fmt.Sprintf("level-%d", i)
		}
		list := make([]any, models)
		for i := range list {
			list[i] = map[string]any{"id": fmt.Sprintf("acme-%d", i), "title": fmt.Sprintf("Acme %d", i), "effort": values}
		}
		roles := doc.(map[string]any)["roles"].(map[string]any)
		roles["model_provider"].(map[string]any)["models"] = list
		return doc
	}
	for _, tc := range []struct {
		name           string
		models, effort int
		want           string
	}{
		{"256 models", 256, 1, ""},
		{"257 models", 257, 1, "/roles/model_provider/models maxItems"},
		{"16 effort values", 1, 16, ""},
		{"17 effort values", 1, 17, "/roles/model_provider/models/0/effort maxItems"},
	} {
		got := ""
		if v, ok := schema.Validate(with(tc.models, tc.effort)).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestAnAPIHoldsItsRules pins the apis of model_provider to the rules their schema
// states, with a value at each edge accepted and a value past it refused: 1 to 4 APIs;
// an endpoint that is https, with a host, an optional port and path, and no user
// information, query or fragment, at most 2048 characters; an auth whose oauth is true
// and beside bearer alone; and 1 to 4 key prefixes, none twice, each 1 to 32 characters
// of printable ASCII without a space.
func TestAnAPIHoldsItsRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	check := func(name, apis, want string) {
		t.Helper()
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(provider(apis)))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		got := ""
		if v, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != want {
			t.Errorf("%s: refused for %q, want %q", name, got, want)
		}
	}
	api := func(endpoint, auth, more string) string {
		return `{"format": "anthropic-messages", "endpoint": ` + endpoint + `, "auth": ` + auth + more + `}`
	}
	header := `{"scheme": "header", "header": "x-api-key"}`
	long := "https://api.acme.example/" + strings.Repeat("a", 2048-len("https://api.acme.example/"))
	for _, tc := range []struct{ endpoint, want string }{
		{`"https://api.acme.example"`, ""},
		{`"https://api.acme.example/"`, ""},
		{`"https://api.acme.example:8443/v1/models"`, ""},
		{`"https://10.0.0.7:65535"`, ""},
		{`"https://acme.example/a%2Fb;v=1/@x"`, ""},
		{`"` + long + `"`, ""},
		{`{"setting": "url"}`, ""},
		{`"` + long + `a"`, "/roles/model_provider/apis/0/endpoint maxLength"},
		{`"http://api.acme.example"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://user@api.acme.example"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://user:secret@api.acme.example"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://api.acme.example?key=1"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://api.acme.example/v1?key=1"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://api.acme.example#top"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"https://acme.example/a b"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`"api.acme.example"`, "/roles/model_provider/apis/0/endpoint pattern"},
		{`{"setting": ""}`, "/roles/model_provider/apis/0/endpoint/setting minLength"},
		{`{"setting": "url", "path": "/v1"}`, "/roles/model_provider/apis/0/endpoint additionalProperties"},
		{`{}`, "/roles/model_provider/apis/0/endpoint required setting"},
		{`443`, "/roles/model_provider/apis/0/endpoint type"},
	} {
		check(tc.endpoint, api(tc.endpoint, header, ""), tc.want)
	}
	url := `"https://api.acme.example"`
	for _, tc := range []struct{ name, auth, want string }{
		{"bearer", `{"scheme": "bearer"}`, ""},
		{"bearer with oauth", `{"scheme": "bearer", "oauth": true}`, ""},
		{"basic", `{"scheme": "basic", "username": "acme"}`, ""},
		{"basic with oauth", `{"scheme": "basic", "username": "acme", "oauth": true}`, "/roles/model_provider/apis/0/auth "},
		{"header with oauth", `{"scheme": "header", "header": "x-api-key", "oauth": true}`, "/roles/model_provider/apis/0/auth "},
		{"oauth false", `{"scheme": "bearer", "oauth": false}`, "/roles/model_provider/apis/0/auth/oauth const"},
		{"header without a header", `{"scheme": "header"}`, "/roles/model_provider/apis/0/auth required header"},
		{"an unknown scheme", `{"scheme": "cookie"}`, "/roles/model_provider/apis/0/auth/scheme enum"},
	} {
		check(tc.name, api(url, tc.auth, ""), tc.want)
	}
	for _, tc := range []struct{ name, prefixes, want string }{
		{"1 prefix", `["acme-key-"]`, ""},
		{"4 prefixes", `["a", "b", "c", "d"]`, ""},
		{"printable ASCII at both ends", `["!", "~"]`, ""},
		{"32 characters", `["` + strings.Repeat("a", 32) + `"]`, ""},
		{"no prefix", `[]`, "/roles/model_provider/apis/0/key_prefixes minItems"},
		{"5 prefixes", `["a", "b", "c", "d", "e"]`, "/roles/model_provider/apis/0/key_prefixes maxItems"},
		{"a prefix twice", `["a", "a"]`, "/roles/model_provider/apis/0/key_prefixes uniqueItems"},
		{"an empty prefix", `[""]`, "/roles/model_provider/apis/0/key_prefixes/0 pattern"},
		{"a space", `["acme key"]`, "/roles/model_provider/apis/0/key_prefixes/0 pattern"},
		{"33 characters", `["` + strings.Repeat("a", 33) + `"]`, "/roles/model_provider/apis/0/key_prefixes/0 pattern"},
		{"a non-ASCII character", `["acme-\u00e9"]`, "/roles/model_provider/apis/0/key_prefixes/0 pattern"},
	} {
		check(tc.name, api(url, header, `, "key_prefixes": `+tc.prefixes), tc.want)
	}
	formats := []string{"anthropic-messages", "openai-responses", "openai-chat", "gemini"}
	list := func(n int) string {
		entries := make([]string, n)
		for i := range entries {
			entries[i] = `{"format": "` + formats[i%len(formats)] + `", "endpoint": ` + url + `, "auth": ` + header + `}`
		}
		return strings.Join(entries, ", ")
	}
	check("4 APIs", list(4), "")
	check("5 APIs", list(5), "/roles/model_provider/apis maxItems")
	check("no API", "", "/roles/model_provider/apis minItems")
	check("an API without auth", `{"format": "gemini", "endpoint": `+url+`}`, "/roles/model_provider/apis/0 required auth")
}

// TestADataDescriptionHasTheModelProviderRoleAlone pins the reason of each invalid data
// description with a role other than model_provider, which the refusal does not name:
// the same document with a program_version, in the program form, is accepted.
func TestADataDescriptionHasTheModelProviderRoleAlone(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, name := range []string{
		"fixtures/invalid/description-data-credential.json",
		"fixtures/invalid/description-data-source.json",
		"fixtures/invalid/description-data-unknown-role.json",
	} {
		b := read(t, name)
		program := bytes.Replace(b, []byte(`"settings":`), []byte(`"program_version": "0.1.0", "settings":`), 1)
		if bytes.Equal(program, b) {
			t.Errorf("%s holds no settings", name)
		}
		if err := validate(schema, b); err == nil {
			t.Errorf("%s is accepted", name)
		}
		if err := validate(schema, program); err != nil {
			t.Errorf("%s, with a program_version, is refused: %v", name, err)
		}
	}
}

// provider is a description in the program form whose model_provider role has the
// APIs put in, the setting url, and one model.
func provider(apis string) string {
	return `{"version": 1, "name": "acme-models", "title": "Acme models", "program_version": "0.1.0",
 "settings": {"type": "object", "properties": {"url": {"type": "string"}}},
 "roles": {"model_provider": {"apis": [` + apis + `], "models": [{"id": "acme-large-2", "title": "Acme Large 2"}]}}}`
}

// gateway is a data description with one API whose endpoint is the setting url, with
// the settings and the models put in.
func gateway(settings, models string) string {
	return `{"version": 1, "name": "example-gateway", "title": "Example gateway", "settings": ` + settings + `,
 "roles": {"model_provider": {"apis": [{"format": "openai-chat", "endpoint": {"setting": "url"}, "auth": {"scheme": "bearer"}}], "models": ` + models + `}}}`
}

// static is a data description with one API whose endpoint is the URL endpoint.
func static(endpoint string) string {
	return `{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "` + endpoint + `",
 "auth": {"scheme": "header", "header": "x-api-key"}}], "models": [{"id": "acme-large-2", "title": "Acme Large 2"}]}}}`
}

// TestCheckModelProviderHoldsTheRulesTheSchemaDoesNotCheck pins the rules of the
// model_provider role that the schema does not check: each refused document is accepted
// by the schema and refused by CheckModelProvider with a message that states the rule;
// an endpoint whose host or port a gateway cannot read is refused by CheckModelProvider
// and CheckEgressHosts alike, beside endpoints at the edges that are accepted; every
// description under fixtures keeps the rules; and two APIs with two formats, the
// endpoint of one a setting, are accepted.
func TestCheckModelProviderHoldsTheRulesTheSchemaDoesNotCheck(t *testing.T) {
	schema := compile(t, "description.schema.json")
	one := `[{"id": "acme-large-2", "title": "Acme Large 2"}]`
	api := func(format, endpoint string) string {
		return `{"format": "` + format + `", "endpoint": ` + endpoint + `, "auth": {"scheme": "bearer"}}`
	}
	url := `"https://api.acme.example"`
	for _, tc := range []struct{ name, doc, want string }{
		{"two APIs with one format", provider(api("anthropic-messages", url) + ", " + api("anthropic-messages", `"https://eu.acme.example"`)),
			`the model_provider apis 0 and 1 have the same format "anthropic-messages"; no two APIs of a provider have the same format`},
		{"a third API with the format of the first", provider(api("openai-chat", url) + ", " + api("gemini", url) + ", " + api("openai-chat", `{"setting": "url"}`)),
			`the model_provider apis 0 and 2 have the same format "openai-chat"; no two APIs of a provider have the same format`},
		{"a setting that does not exist", gateway(`{"type": "object", "properties": {"address": {"type": "string"}}}`, one),
			`the endpoint of model_provider apis[0] is the setting "url", and settings has no property "url"`},
		{"a setting with no properties at all", gateway(`{"type": "object"}`, one),
			`the endpoint of model_provider apis[0] is the setting "url", and settings has no property "url"`},
		{"a setting that is not a string", gateway(`{"type": "object", "properties": {"url": {"type": "integer"}}}`, one),
			`the endpoint of model_provider apis[0] is the setting "url", whose type is not string`},
		{"a writeOnly setting, in the program form", strings.Replace(gateway(`{"type": "object", "properties": {"url": {"type": "string", "writeOnly": true}, "url_file": {"type": "string"}}}`, one), `"settings":`, `"program_version": "0.1.0", "settings":`, 1),
			`the endpoint of model_provider apis[0] is the setting "url", which is marked writeOnly; an endpoint setting is never a secret`},
		{"the second API's setting that does not exist", provider(api("openai-chat", url) + ", " + api("gemini", `{"setting": "address"}`)),
			`the endpoint of model_provider apis[1] is the setting "address", and settings has no property "address"`},
		{"two models with one id", gateway(`{"type": "object", "properties": {"url": {"type": "string"}}}`,
			`[{"id": "acme-large-2", "title": "Acme Large 2"}, {"id": "acme-small-1", "title": "Acme Small 1"}, {"id": "acme-large-2", "title": "Acme Large 2 again"}]`),
			`the model_provider models 0 and 2 have the same id "acme-large-2"; no two models have the same id`},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.doc))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: the schema refuses it, and the rule is CheckModelProvider's alone: %v", tc.name, err)
		}
		if err := contracts.CheckModelProvider(doc); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	unread := func(endpoint, why string) string {
		return `the endpoint "` + endpoint + `" of model_provider apis[0] has no host a gateway can read: ` + why
	}
	notLabels := func(host string) string {
		return `the host "` + host + `" is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit`
	}
	notIPv4 := func(host string) string {
		return `the host "` + host + `" ends in a number and is not an IPv4 address of four decimal parts from 0 to 255, each with no leading zero`
	}
	port := func(p string) string { return `the port "` + p + `" is not a number from 1 to 65535` }
	for _, tc := range []struct{ endpoint, egress, want string }{
		{"https://*.acme.example", `["api.acme.example"]`, notLabels("*.acme.example")},
		{"https://*.api.acme.example", `["api.acme.example"]`, notLabels("*.api.acme.example")},
		{"https://a!b.acme.example", `["api.acme.example"]`, notLabels("a!b.acme.example")},
		{"https://API.acme.example", `["api.acme.example"]`, notLabels("API.acme.example")},
		{"https://-api.acme.example", `["api.acme.example"]`, notLabels("-api.acme.example")},
		{"https://api-.acme.example", `["api.acme.example"]`, notLabels("api-.acme.example")},
		{"https://" + strings.Repeat("a", 64) + ".acme.example", `["api.acme.example"]`, notLabels(strings.Repeat("a", 64) + ".acme.example")},
		{"https://api.acme.example.", `["api.acme.example"]`, notLabels("api.acme.example.")},
		{"https://api..acme.example", `["api.acme.example"]`, notLabels("api..acme.example")},
		{"https://[::1]", `["api.acme.example"]`, notLabels("::1")},
		{"https://api.acme.example:0", `["api.acme.example"]`, port("0")},
		{"https://api.acme.example:99999", `["api.acme.example"]`, port("99999")},
		{"https://api.acme.example:65536/v1", `["api.acme.example"]`, port("65536")},
		{"https://127.1", `["10.0.0.1"]`, notIPv4("127.1")},
		{"https://10.0.0.01", `["10.0.0.1"]`, notIPv4("10.0.0.01")},
		{"https://10.0.0.256", `["10.0.0.1"]`, notIPv4("10.0.0.256")},
		{"https://acme.example.0x1f", `["api.acme.example"]`, notIPv4("acme.example.0x1f")},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withEgress(static(tc.endpoint), tc.egress)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: the schema refuses it, and the rule is CheckModelProvider's alone: %v", tc.endpoint, err)
		}
		want := unread(tc.endpoint, tc.want)
		if err := contracts.CheckModelProvider(doc); err == nil || err.Error() != want {
			t.Errorf("CheckModelProvider, %s: %v, want %q", tc.endpoint, err, want)
		}
		if err := contracts.CheckEgressHosts(doc); err == nil || err.Error() != want {
			t.Errorf("CheckEgressHosts, %s: %v, want %q", tc.endpoint, err, want)
		}
	}
	for _, endpoint := range []string{"https://api.acme.example:1", "https://api.acme.example:65535/v1", "https://api.acme.example:/v1", "https://10.0.0.1", "https://0x7f.acme.example", "https://localhost"} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(static(endpoint)))
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckModelProvider(doc); err != nil {
			t.Errorf("%s: %v", endpoint, err)
		}
	}
	for _, f := range files(t, "fixtures") {
		doc, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckModelProvider(doc); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	accepted := provider(api("anthropic-messages", url) + ", " + api("openai-chat", `{"setting": "url"}`))
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(accepted))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Errorf("two APIs with two formats: the schema refuses it: %v", err)
	}
	if err := contracts.CheckModelProvider(doc); err != nil {
		t.Errorf("two APIs with two formats: %v", err)
	}
}
