package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
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
// model_provider description whose refusal does not contain it: the same document is
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
		"fixtures/invalid/description-model-provider-limit-windows-upper-case.json":    {`"Five_hour"`, `"five_hour"`},
		"fixtures/invalid/description-model-provider-limit-windows-leading-digit.json": {`"5_hour"`, `"five_hour"`},
		"fixtures/invalid/description-model-provider-limit-windows-65-characters.json": {
			`"` + strings.Repeat("a", 65) + `"`, `"` + strings.Repeat("a", 64) + `"`},
		"fixtures/invalid/description-model-provider-runtimes-upper-case.json":           {`"Claude"`, `"claude"`},
		"fixtures/invalid/description-model-provider-runtimes-leading-digit.json":        {`"1claude"`, `"claude"`},
		"fixtures/invalid/description-model-provider-paths-no-leading-slash.json":        {`"v1/messages"`, `"/v1/messages"`},
		"fixtures/invalid/description-model-provider-paths-star-inside.json":             {`"/v1/*/messages"`, `"/v1/messages/*"`},
		"fixtures/invalid/description-model-provider-paths-query.json":                   {`?beta=true`, ``},
		"fixtures/invalid/description-model-provider-oauth-refresh-without-oauth.json":   {`{"scheme": "bearer"}`, `{"scheme": "bearer", "oauth": true}`},
		"fixtures/invalid/description-model-provider-oauth-refresh-url-http.json":        {`"http://`, `"https://`},
		"fixtures/invalid/description-model-provider-oauth-refresh-client-id-space.json": {`"acme cli"`, `"acme-cli"`},
		"fixtures/invalid/description-vendor-line-feed.json":                             {`\nCloud`, ` Cloud`},
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

// TestAnAPIHoldsItsRules pins the apis of model_provider to the rules of their
// schema, with a value at each edge accepted and a value past it refused: 1 to 4 APIs;
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

// TestLimitWindowsHoldTheirRules pins limit_windows of model_provider to the rules of
// its schema, with a value at each edge accepted and a value past it refused: 1 to
// 11 names, none twice, each 1 to 64 characters of lower-case letters, digits and
// underscores that starts with a letter. A provider without limit_windows is accepted.
func TestLimitWindowsHoldTheirRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	check := func(name, windows, want string) {
		t.Helper()
		text := provider(`{"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": {"scheme": "bearer", "oauth": true}}`)
		if windows != "" {
			text = strings.Replace(text, `"models":`, `"limit_windows": `+windows+`, "models":`, 1)
		}
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(text))
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
	names := func(n int) string {
		list := make([]string, n)
		for i := range list {
			list[i] = fmt.Sprintf(`"window_%d"`, i)
		}
		return "[" + strings.Join(list, ", ") + "]"
	}
	for _, tc := range []struct{ name, windows, want string }{
		{"no limit_windows", "", ""},
		{"1 name", `["five_hour"]`, ""},
		{"2 names", `["five_hour", "seven_day"]`, ""},
		{"11 names", names(11), ""},
		{"1 character", `["a"]`, ""},
		{"64 characters", `["` + strings.Repeat("a", 64) + `"]`, ""},
		{"letters, digits and underscores", `["window_2_b"]`, ""},
		{"no name", `[]`, "/roles/model_provider/limit_windows minItems"},
		{"12 names", names(12), "/roles/model_provider/limit_windows maxItems"},
		{"a name twice", `["five_hour", "five_hour"]`, "/roles/model_provider/limit_windows uniqueItems"},
		{"an empty name", `[""]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"an upper-case letter", `["Five_hour"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"a leading digit", `["5_hour"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"a leading underscore", `["_hour"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"a hyphen", `["five-hour"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"a space", `["five hour"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"65 characters", `["` + strings.Repeat("a", 65) + `"]`, "/roles/model_provider/limit_windows/0 pattern"},
		{"a number", `[5]`, "/roles/model_provider/limit_windows/0 type"},
		{"a string, not a list", `"five_hour"`, "/roles/model_provider/limit_windows type"},
	} {
		check(tc.name, tc.windows, tc.want)
	}
}

// refusal validates with schema the description that edit makes from api, the members
// of an API of the scheme bearer with oauth true, and role, the text "models": of the
// model_provider role. It returns the refusal's reasons, or "" when schema accepts the
// description.
func refusal(t *testing.T, schema *jsonschema.Schema, name string, edit func(api, role string) string) string {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(edit(`"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": {"scheme": "bearer", "oauth": true}`, `"models":`)))
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if v, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
		return strings.Join(leaves(v), "; ")
	}
	return ""
}

// inAPI returns an edit for refusal that adds members, such as `, "paths": ["/v1"]`,
// to the API.
func inAPI(members string) func(api, role string) string {
	return func(api, _ string) string { return provider("{" + api + members + "}") }
}

// strs is a JSON array of n strings, each format with its index put in.
func strs(n int, format string) string {
	l := make([]string, n)
	for i := range l {
		l[i] = `"` + fmt.Sprintf(format, i) + `"`
	}
	return "[" + strings.Join(l, ", ") + "]"
}

// TestRuntimesHoldTheirRules pins runtimes of an API to the rules of its
// schema, with a value at each edge accepted and a value past it refused: 1 to 16 names, none
// twice, each 1 to 64 characters of lower-case letters, digits and hyphens that starts
// with a letter. An API without runtimes is accepted.
func TestRuntimesHoldTheirRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	at := "/roles/model_provider/apis/0/runtimes"
	for _, tc := range []struct{ name, runtimes, want string }{
		{"no runtimes", "", ""},
		{"1 name", `["claude"]`, ""},
		{"16 names", strs(16, "runtime-%d"), ""},
		{"1 character", `["a"]`, ""},
		{"64 characters", `["` + strings.Repeat("a", 64) + `"]`, ""},
		{"letters, digits and hyphens", `["acme-2-b"]`, ""},
		{"no name", `[]`, at + " minItems"},
		{"17 names", strs(17, "runtime-%d"), at + " maxItems"},
		{"a name twice", `["claude", "claude"]`, at + " uniqueItems"},
		{"an empty name", `[""]`, at + "/0 pattern"},
		{"an upper-case letter", `["Claude"]`, at + "/0 pattern"},
		{"a leading digit", `["1claude"]`, at + "/0 pattern"},
		{"a leading hyphen", `["-claude"]`, at + "/0 pattern"},
		{"an underscore", `["acme_cli"]`, at + "/0 pattern"},
		{"65 characters", `["` + strings.Repeat("a", 65) + `"]`, at + "/0 pattern"},
		{"a number", `[5]`, at + "/0 type"},
		{"a string, not a list", `"claude"`, at + " type"},
	} {
		members := ""
		if tc.runtimes != "" {
			members = `, "runtimes": ` + tc.runtimes
		}
		if got := refusal(t, schema, tc.name, inAPI(members)); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestPathsHoldTheirRules pins paths of an API to the rules of its schema, with a
// value at each edge accepted and a value past it refused: 1 to 32 paths, none twice,
// each in Forager's path grammar. An API without paths is accepted.
func TestPathsHoldTheirRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	at := "/roles/model_provider/apis/0/paths"
	for _, tc := range []struct{ name, paths, want string }{
		{"no paths", "", ""},
		{"1 path", `["/v1/messages"]`, ""},
		{"32 paths", strs(32, "/v1/p%d"), ""},
		{"the root", `["/"]`, ""},
		{"every path", `["/*"]`, ""},
		{"a star at the end", `["/backend-api/codex/*"]`, ""},
		{"{} as a whole segment", `["/v1/{}/messages"]`, ""},
		{"{} as the last segment", `["/v1/{}"]`, ""},
		{"{} before a star", `["/v1/{}*"]`, ""},
		{"{} as the first segment", `["/{}/messages"]`, ""},
		{"a name in braces", `["/v1/{id}"]`, at + "/0 pattern"},
		{"braces inside a segment", `["/v1/a{b}"]`, at + "/0 pattern"},
		{"an opening brace", `["/v1/a{b"]`, at + "/0 pattern"},
		{"a closing brace", `["/v1/a}b"]`, at + "/0 pattern"},
		{"{} with text after it", `["/v1/{}a"]`, at + "/0 pattern"},
		{"no path", `[]`, at + " minItems"},
		{"33 paths", strs(33, "/v1/p%d"), at + " maxItems"},
		{"a path twice", `["/v1", "/v1"]`, at + " uniqueItems"},
		{"an empty path", `[""]`, at + "/0 pattern"},
		{"no leading slash", `["v1/messages"]`, at + "/0 pattern"},
		{"a star inside", `["/v1/*/messages"]`, at + "/0 pattern"},
		{"two stars", `["/v1/**"]`, at + "/0 pattern"},
		{"a query", `["/v1/messages?beta=true"]`, at + "/0 pattern"},
		{"a fragment", `["/v1#top"]`, at + "/0 pattern"},
		{"a space", `["/v1/a b"]`, at + "/0 pattern"},
		{"a tab", `["/v1/a\tb"]`, at + "/0 pattern"},
		{"a string, not a list", `"/v1"`, at + " type"},
	} {
		members := ""
		if tc.paths != "" {
			members = `, "paths": ` + tc.paths
		}
		if got := refusal(t, schema, tc.name, inAPI(members)); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestOAuthRefreshHoldsItsRules pins oauth_refresh of an API to the rules of its
// schema: url and client_id required, scope optional, no other member; url as
// $defs/endpoint defines it; client_id 1 to 128 characters with no white space; scope 1
// to 256 characters; and oauth_refresh only with an auth whose oauth is true. An API
// without oauth_refresh is accepted.
func TestOAuthRefreshHoldsItsRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	at := "/roles/model_provider/apis/0/oauth_refresh"
	url := `"url": "https://auth.acme.example/oauth/token"`
	id := `"client_id": "acme-cli"`
	if got := refusal(t, schema, "no oauth_refresh", inAPI("")); got != "" {
		t.Errorf("no oauth_refresh: refused for %q", got)
	}
	for _, tc := range []struct{ name, refresh, want string }{
		{"url and client_id", url + ", " + id, ""},
		{"with a scope", url + ", " + id + `, "scope": "openid profile email"`, ""},
		{"a url with a port", `"url": "https://auth.acme.example:8443/oauth/token", ` + id, ""},
		{"a client_id of 128 characters", url + `, "client_id": "` + strings.Repeat("a", 128) + `"`, ""},
		{"a scope of 256 characters", url + ", " + id + `, "scope": "` + strings.Repeat("a", 256) + `"`, ""},
		{"no url", id, at + " required url"},
		{"no client_id", url, at + " required client_id"},
		{"an undefined member", url + ", " + id + `, "client_secret": "acme"`, at + " additionalProperties"},
		{"a url over http", `"url": "http://auth.acme.example/oauth/token", ` + id, at + "/url pattern"},
		{"a url with a query", `"url": "https://auth.acme.example/oauth/token?grant=1", ` + id, at + "/url pattern"},
		{"an empty client_id", url + `, "client_id": ""`, at + "/client_id minLength; " + at + "/client_id pattern"},
		{"a client_id of 129 characters", url + `, "client_id": "` + strings.Repeat("a", 129) + `"`, at + "/client_id maxLength"},
		{"a client_id with a space", url + `, "client_id": "acme cli"`, at + "/client_id pattern"},
		{"a client_id with a tab", url + `, "client_id": "acme\tcli"`, at + "/client_id pattern"},
		{"an empty scope", url + ", " + id + `, "scope": ""`, at + "/scope minLength"},
		{"a scope of 257 characters", url + ", " + id + `, "scope": "` + strings.Repeat("a", 257) + `"`, at + "/scope maxLength"},
	} {
		if got := refusal(t, schema, tc.name, inAPI(`, "oauth_refresh": {`+tc.refresh+`}`)); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
	for _, tc := range []struct{ name, auth string }{
		{"bearer without oauth", `{"scheme": "bearer"}`},
		{"header", `{"scheme": "header", "header": "x-api-key"}`},
		{"basic", `{"scheme": "basic", "username": "acme"}`},
	} {
		edit := func(_, _ string) string {
			return provider(`{"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": ` + tc.auth +
				`, "oauth_refresh": {` + url + ", " + id + `}}`)
		}
		if got, want := refusal(t, schema, tc.name, edit), "/roles/model_provider/apis/0/auth required oauth"; got != want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, want)
		}
	}
}

// TestOAuthDeviceHoldsItsRules pins oauth_device of an API to the rules of its schema:
// usercode_url, poll_url, verification_url and redirect_uri required, each an https URL
// as $defs/endpoint defines it, no other member, and no client id, since the client id is
// the client_id of oauth_refresh; and oauth_device only with oauth_refresh.
func TestOAuthDeviceHoldsItsRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	at := "/roles/model_provider/apis/0/oauth_device"
	refresh := `, "oauth_refresh": {"url": "https://auth.acme.example/oauth/token", "client_id": "acme-cli"}`
	member := map[string]string{
		"usercode_url":     "https://auth.acme.example/device/usercode",
		"poll_url":         "https://auth.acme.example/device/token",
		"verification_url": "https://auth.acme.example/device",
		"redirect_uri":     "https://auth.acme.example/device/callback",
	}
	order := []string{"usercode_url", "poll_url", "verification_url", "redirect_uri"}
	device := func(skip, key, value string) string {
		var l []string
		for _, k := range order {
			v := member[k]
			if k == key {
				v = value
			}
			if k != skip {
				l = append(l, `"`+k+`": "`+v+`"`)
			}
		}
		return strings.Join(l, ", ")
	}
	for _, tc := range []struct{ name, device, want string }{
		{"the four urls", device("", "", ""), ""},
		{"a url with a port and a path", device("", "poll_url", "https://auth.acme.example:8443/device/token"), ""},
		{"an empty object", "", at + " required usercode_url,poll_url,verification_url,redirect_uri"},
		{"no usercode_url", device("usercode_url", "", ""), at + " required usercode_url"},
		{"no poll_url", device("poll_url", "", ""), at + " required poll_url"},
		{"no verification_url", device("verification_url", "", ""), at + " required verification_url"},
		{"no redirect_uri", device("redirect_uri", "", ""), at + " required redirect_uri"},
		{"a usercode_url over http", device("", "usercode_url", "http://auth.acme.example/device/usercode"), at + "/usercode_url pattern"},
		{"a poll_url over http", device("", "poll_url", "http://auth.acme.example/device/token"), at + "/poll_url pattern"},
		{"a verification_url over http", device("", "verification_url", "http://auth.acme.example/device"), at + "/verification_url pattern"},
		{"a redirect_uri over http", device("", "redirect_uri", "http://auth.acme.example/device/callback"), at + "/redirect_uri pattern"},
		{"a poll_url with a query", device("", "poll_url", "https://auth.acme.example/device/token?x=1"), at + "/poll_url pattern"},
		{"a client_id", device("", "", "") + `, "client_id": "acme-cli"`, at + " additionalProperties"},
		{"an undefined member", device("", "", "") + `, "interval": 5`, at + " additionalProperties"},
	} {
		if got := refusal(t, schema, tc.name, inAPI(refresh+`, "oauth_device": {`+tc.device+`}`)); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
	const without = "/roles/model_provider/apis/0 dependentRequired/oauth_device"
	if got := refusal(t, schema, "oauth_device without oauth_refresh", inAPI(`, "oauth_device": {`+device("", "", "")+`}`)); got != without {
		t.Errorf("oauth_device without oauth_refresh: refused for %q, want %q", got, without)
	}
}

// TestCheckDescriptionRefusesAnUnusableOAuthDevice pins that CheckDescription, which runs
// before the schema in conformance, refuses an oauth_device without oauth_refresh and an
// oauth_device URL string that does not start with https:// in lower case (http://,
// HTTPS://, ftp://, //host/path, a bare host, a path, an empty string) with a message
// that starts with the fix, at the index of the API and with the member and the value
// of the URL, and that the schema refuses the same documents. A missing or non-string
// member and other oauth_device mistakes are left to the schema.
func TestCheckDescriptionRefusesAnUnusableOAuthDevice(t *testing.T) {
	schema := compile(t, "description.schema.json")
	good := map[string]string{
		"usercode_url":     "https://auth.acme.example/device/usercode",
		"poll_url":         "https://auth.acme.example/device/token",
		"verification_url": "https://auth.acme.example/device",
		"redirect_uri":     "https://auth.acme.example/device/callback",
	}
	order := []string{"usercode_url", "poll_url", "verification_url", "redirect_uri"}
	device := func(member, value string) string {
		var l []string
		for _, k := range order {
			v := good[k]
			if k == member {
				v = value
			}
			l = append(l, `"`+k+`": "`+v+`"`)
		}
		return `, "oauth_device": {` + strings.Join(l, ", ") + `}`
	}
	refresh := `, "oauth_refresh": {"url": "https://auth.acme.example/oauth/token", "client_id": "acme-cli"}`
	// api0 is a valid first API of the format openai-chat; the API under test is the
	// second one, at index 1, or the first one, at index 0.
	api0 := `{"format": "openai-chat", "endpoint": "https://chat.acme.example", "auth": {"scheme": "bearer"}}`
	api := func(members string) string {
		return `{"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": {"scheme": "bearer", "oauth": true}` + members + `}`
	}
	doc := func(apis ...string) string { return provider(strings.Join(apis, ", ")) }
	cases := []struct{ name, doc, want string }{
		{"without oauth_refresh, at index 0", doc(api(device("", ""))),
			"Add oauth_refresh to model_provider apis[0]: oauth_device takes its client_id and url from it."},
		{"without oauth_refresh, at index 1", doc(api0, api(device("", ""))),
			"Add oauth_refresh to model_provider apis[1]: oauth_device takes its client_id and url from it."},
	}
	for _, member := range order {
		for _, value := range []string{"http://auth.acme.example/device/x", "ftp://auth.acme.example/device/x", "//auth.acme.example/device/x", "auth.acme.example", "/device/x", "", "HTTPS://auth.acme.example/device/x"} {
			cases = append(cases,
				struct{ name, doc, want string }{member + " " + value, doc(api(refresh + device(member, value))),
					`Change the oauth_device ` + member + ` "` + value + `" of model_provider apis[0] to an https URL.`},
				struct{ name, doc, want string }{member + " " + value + ", at index 1", doc(api0, api(refresh+device(member, value))),
					`Change the oauth_device ` + member + ` "` + value + `" of model_provider apis[1] to an https URL.`})
		}
	}
	for _, tc := range cases {
		if err := contracts.CheckDescription([]byte(tc.doc)); err == nil || err.Error() != tc.want {
			t.Errorf("%s: CheckDescription returns %v, want %q", tc.name, err, tc.want)
		}
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.doc))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if schema.Validate(d) == nil {
			t.Errorf("%s: the schema accepts it", tc.name)
		}
	}
	for _, tc := range []struct{ name, doc string }{
		{"the four https URLs", doc(api(refresh + device("", "")))},
		{"no oauth_device", doc(api(refresh))},
		{"an https URL with http in the path", doc(api(refresh + device("poll_url", "https://auth.acme.example/http://x")))},
		{"a missing member, which the schema refuses", doc(api(refresh + strings.Replace(device("", ""), `"poll_url": "`+good["poll_url"]+`", `, "", 1)))},
		{"a member that is not a string, which the schema refuses", doc(api(refresh + strings.Replace(device("", ""), `"`+good["poll_url"]+`"`, "5", 1)))},
		{"an undefined member, which the schema refuses", doc(api(refresh + strings.Replace(device("", ""), "}", `, "client_id": "acme-cli"}`, 1)))},
	} {
		if err := contracts.CheckDescription([]byte(tc.doc)); err != nil {
			t.Errorf("%s: CheckDescription returns %v", tc.name, err)
		}
	}
}

// TestTheAcmeSubscriptionFixtureHasOAuthDevice pins that fixtures/acme-subscription.json
// sets oauth_device with its four members, so that a valid description with the member
// stays among the fixtures that TestDescriptionFixturesValidate accepts.
func TestTheAcmeSubscriptionFixtureHasOAuthDevice(t *testing.T) {
	doc, err := contracts.Document("fixtures/acme-subscription.json")
	if err != nil {
		t.Fatal(err)
	}
	d, _ := doc.(map[string]any)
	roles, _ := d["roles"].(map[string]any)
	role, _ := roles["model_provider"].(map[string]any)
	apis, _ := role["apis"].([]any)
	if len(apis) != 1 {
		t.Fatalf("%d APIs, want 1", len(apis))
	}
	api, _ := apis[0].(map[string]any)
	device, ok := api["oauth_device"].(map[string]any)
	if !ok {
		t.Fatal("the API has no oauth_device")
	}
	for _, member := range []string{"usercode_url", "poll_url", "verification_url", "redirect_uri"} {
		if u, ok := device[member].(string); !ok || !strings.HasPrefix(u, "https://") {
			t.Errorf("oauth_device %s is %v, want an https URL", member, device[member])
		}
	}
	if len(device) != 4 {
		t.Errorf("oauth_device has %d members, want 4", len(device))
	}
}

// TestCheckModelProviderReadsTheOAuthDeviceURLHosts pins that CheckModelProvider refuses
// each of the four urls of an oauth_device whose host or port the control plane cannot
// read, which the schema accepts, names the member, and accepts urls at the edges.
func TestCheckModelProviderReadsTheOAuthDeviceURLHosts(t *testing.T) {
	schema := compile(t, "description.schema.json")
	good := map[string]string{
		"usercode_url":     "https://auth.acme.example/device/usercode",
		"poll_url":         "https://auth.acme.example/device/token",
		"verification_url": "https://auth.acme.example/device",
		"redirect_uri":     "https://auth.acme.example/device/callback",
	}
	doc := func(member, url string) any {
		t.Helper()
		var l []string
		for _, k := range []string{"usercode_url", "poll_url", "verification_url", "redirect_uri"} {
			v := good[k]
			if k == member {
				v = url
			}
			l = append(l, `"`+k+`": "`+v+`"`)
		}
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(provider(`{"format": "anthropic-messages", "endpoint": "https://api.acme.example",
 "auth": {"scheme": "bearer", "oauth": true}, "oauth_refresh": {"url": "https://auth.acme.example/oauth/token", "client_id": "acme-cli"},
 "oauth_device": {` + strings.Join(l, ", ") + `}}`)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(d); err != nil {
			t.Fatalf("%s %s: the schema refuses it: %v", member, url, err)
		}
		return d
	}
	for _, member := range []string{"usercode_url", "poll_url", "verification_url", "redirect_uri"} {
		for _, tc := range []struct{ url, why string }{
			{"https://AUTH.acme.example/device", `the host "AUTH.acme.example" is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit`},
			{"https://10.0.0.256/device", `the host "10.0.0.256" ends in a number and is not an IPv4 address of four decimal parts from 0 to 255, each with no leading zero`},
			{"https://auth.acme.example:65536/device", `the port "65536" is not a number from 1 to 65535`},
		} {
			want := `change the oauth_device ` + member + ` "` + tc.url + `" of model_provider apis[0]: the control plane cannot read its host or port: ` + tc.why
			if err := contracts.CheckModelProvider(doc(member, tc.url)); err == nil || err.Error() != want {
				t.Errorf("%s %s: %v, want %q", member, tc.url, err, want)
			}
		}
		for _, url := range []string{good[member], "https://auth.acme.example:65535/device", "https://10.0.0.1/device"} {
			if err := contracts.CheckModelProvider(doc(member, url)); err != nil {
				t.Errorf("%s %s: %v", member, url, err)
			}
		}
	}
}

// TestKeyLifetimeDaysHoldsItsRules pins key_lifetime_days of model_provider to the rules of
// its schema, with a value at each edge accepted and a value past it refused: an
// integer from 1 to 3650. A provider without key_lifetime_days is accepted.
func TestKeyLifetimeDaysHoldsItsRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	at := "/roles/model_provider/key_lifetime_days"
	for _, tc := range []struct{ name, days, want string }{
		{"no key_lifetime_days", "", ""},
		{"1", "1", ""},
		{"365", "365", ""},
		{"3650", "3650", ""},
		{"0", "0", at + " minimum"},
		{"-1", "-1", at + " minimum"},
		{"3651", "3651", at + " maximum"},
		{"365.5", "365.5", at + " type"},
		{"a string", `"365"`, at + " type"},
		{"null", "null", at + " type"},
	} {
		edit := func(api, role string) string {
			text := provider("{" + api + "}")
			if tc.days != "" {
				text = strings.Replace(text, role, `"key_lifetime_days": `+tc.days+", "+role, 1)
			}
			return text
		}
		if got := refusal(t, schema, tc.name, edit); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestVendorHoldsItsRules pins vendor of a description to the rules of its schema,
// with a value at each edge accepted and a value past it refused: a line as $defs/line
// defines it, of 1 to 40 characters. A description without vendor is accepted.
func TestVendorHoldsItsRules(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, tc := range []struct{ name, vendor, want string }{
		{"no vendor", "", ""},
		{"1 character", `"A"`, ""},
		{"40 characters", `"` + strings.Repeat("a", 40) + `"`, ""},
		{"lower case and spaces", `"your LiteLLM proxy"`, ""},
		{"empty", `""`, "/vendor minLength; /vendor pattern"},
		{"a line feed", `"Acme\nCloud"`, "/vendor pattern"},
		{"a control character", `"Acme\u0007"`, "/vendor pattern"},
		{"a bidirectional override", `"Acme\u202eduolC"`, "/vendor pattern"},
		{"41 characters", `"` + strings.Repeat("a", 41) + `"`, "/vendor maxLength"},
		{"a number", `1`, "/vendor type"},
	} {
		edit := func(api, _ string) string {
			text := provider("{" + api + "}")
			if tc.vendor != "" {
				text = strings.Replace(text, `"settings":`, `"vendor": `+tc.vendor+`, "settings":`, 1)
			}
			return text
		}
		if got := refusal(t, schema, tc.name, edit); got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestCheckModelProviderReadsTheOAuthRefreshURLHost pins that CheckModelProvider refuses
// an oauth_refresh url whose host or port the control plane cannot read, which the
// schema accepts, and accepts one at the edges.
func TestCheckModelProviderReadsTheOAuthRefreshURLHost(t *testing.T) {
	schema := compile(t, "description.schema.json")
	doc := func(url string) any {
		t.Helper()
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(provider(`{"format": "anthropic-messages", "endpoint": "https://api.acme.example",
 "auth": {"scheme": "bearer", "oauth": true}, "oauth_refresh": {"url": "` + url + `", "client_id": "acme-cli"}}`)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(d); err != nil {
			t.Fatalf("%s: the schema refuses it: %v", url, err)
		}
		return d
	}
	for _, tc := range []struct{ url, why string }{
		{"https://API.acme.example/oauth/token", `the host "API.acme.example" is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit`},
		{"https://10.0.0.256/oauth/token", `the host "10.0.0.256" ends in a number and is not an IPv4 address of four decimal parts from 0 to 255, each with no leading zero`},
		{"https://auth.acme.example:65536/oauth/token", `the port "65536" is not a number from 1 to 65535`},
	} {
		want := `change the oauth_refresh url "` + tc.url + `" of model_provider apis[0]: the control plane cannot read its host or port: ` + tc.why
		if err := contracts.CheckModelProvider(doc(tc.url)); err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %q", tc.url, err, want)
		}
	}
	for _, url := range []string{"https://auth.acme.example/oauth/token", "https://auth.acme.example:65535/oauth/token", "https://10.0.0.1/oauth/token"} {
		if err := contracts.CheckModelProvider(doc(url)); err != nil {
			t.Errorf("%s: %v", url, err)
		}
	}
}

// TestCheckModelProviderRefusesAnUnclearPath pins that CheckModelProvider refuses an
// entry of an API's paths that Forager's policy.UnclearPath reports, which the schema
// accepts: an empty, "." or ".." segment, also before a ";" or "%3B", a backslash, an
// encoded slash, backslash, dot or percent sign in either letter case, and a control
// character. The message starts with the fix and names the entry. Paths that the gateway
// can match, with a star at the end or a segment that only starts with dots, are accepted.
func TestCheckModelProviderRefusesAnUnclearPath(t *testing.T) {
	schema := compile(t, "description.schema.json")
	doc := func(paths string) any {
		t.Helper()
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(provider(`{"format": "openai-responses", "endpoint": "https://api.acme.example/v1",
 "auth": {"scheme": "bearer"}, "paths": ` + paths + `}`)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(d); err != nil {
			t.Fatalf("%s: the schema refuses it, and the rule is CheckModelProvider's alone: %v", paths, err)
		}
		return d
	}
	const reason = `: it has an empty, "." or ".." segment, a backslash, an encoded slash, backslash, dot or percent sign, or a control character, which the gateway denies in a request path, so the path matches no request`
	for _, tc := range []struct{ name, path string }{
		{"an empty segment", "/v1//messages"},
		{"an empty segment at the start", "//v1"},
		{"a dot segment", "/v1/./messages"},
		{"a dot segment at the end", "/v1/."},
		{"a dot-dot segment", "/v1/../messages"},
		{"a dot-dot segment at the end", "/v1/.."},
		{"a dot segment before a semicolon", "/v1/.;x/messages"},
		{"a dot-dot segment before a semicolon", "/v1/..;x/messages"},
		{"a dot-dot segment before an encoded semicolon", "/v1/..%3B/messages"},
		{"a dot-dot segment before an encoded semicolon in capitals", "/v1/..%3b/messages"},
		{"an empty segment before a semicolon", "/v1/;x/messages"},
		{"a dot-dot segment before a star", "/v1/../*"},
		{"a backslash", `/v1\\messages`},
		{"an encoded slash", "/v1%2Fmessages"},
		{"an encoded slash in lower case", "/v1%2fmessages"},
		{"an encoded backslash", "/v1%5Cmessages"},
		{"an encoded dot", "/v1/%2e%2e/messages"},
		{"an encoded dot in capitals", "/v1/%2E/messages"},
		{"an encoded percent sign", "/v1/%252e/messages"},
		{"a control character", `/v1/a\u0001b`},
		{"a delete character", `/v1/a\u007fb`},
	} {
		d := doc(`["` + tc.path + `"]`)
		got := contracts.CheckModelProvider(d)
		path := tc.path
		if err := json.Unmarshal([]byte(`"`+tc.path+`"`), &path); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		want := `change the path ` + strconv.Quote(path) + ` of model_provider apis[0] paths[0]` + reason
		if got == nil || got.Error() != want {
			t.Errorf("%s: %v, want %q", tc.name, got, want)
		}
	}
	if got, want := contracts.CheckModelProvider(doc(`["/v1/messages", "/v1/a/../b"]`)), `change the path "/v1/a/../b" of model_provider apis[0] paths[1]`+reason; got == nil || got.Error() != want {
		t.Errorf("the second path: %v, want %q", got, want)
	}
	for _, paths := range []string{
		`["/"]`, `["/*"]`, `["/v1/messages"]`, `["/backend-api/codex/*"]`, `["/v1/messages/"]`,
		`["/v1/.hidden", "/v1/..hidden", "/v1/.*", "/v1/..*"]`, `["/v1/a.b/c..d"]`,
	} {
		if err := contracts.CheckModelProvider(doc(paths)); err != nil {
			t.Errorf("%s: %v", paths, err)
		}
	}
}

// TestCheckModelProviderRefusesAPathWithASpaceTheSchemaReadsAsText pins that
// CheckModelProvider refuses an entry of paths with a Unicode space character, such as
// U+00A0, U+2003 or U+0085, which the pattern of the schema accepts and Forager's
// policy.ValidPath refuses, and accepts a path with U+00FF.
func TestCheckModelProviderRefusesAPathWithASpaceTheSchemaReadsAsText(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, tc := range []struct {
		name, path string
		refused    bool
	}{
		{"a no-break space", `/v1/a\u00a0b`, true},
		{"an em space", `/v1/a\u2003b`, true},
		{"a next line", `/v1/a\u0085b`, true},
		{"a letter with a diaeresis", `/v1/a\u00ffb`, false},
	} {
		d, err := jsonschema.UnmarshalJSON(strings.NewReader(provider(`{"format": "openai-responses", "endpoint": "https://api.acme.example/v1",
 "auth": {"scheme": "bearer"}, "paths": ["` + tc.path + `"]}`)))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(d); err != nil {
			t.Errorf("%s: the schema refuses it, and the rule is CheckModelProvider's alone: %v", tc.name, err)
		}
		err = contracts.CheckModelProvider(d)
		if got := err != nil && strings.HasPrefix(err.Error(), "change the path "); got != tc.refused {
			t.Errorf("%s: %v, refused is %v, want %v", tc.name, err, got, tc.refused)
		}
	}
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
// by the schema and refused by CheckModelProvider with a message that contains the rule;
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
