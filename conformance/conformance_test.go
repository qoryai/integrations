package conformance_test

import (
	"bytes"
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

const tracker = `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": %s,
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"]}},
 "egress_hosts": ["tracker.acme.example"]}
`

func describe(settings string) []byte {
	return []byte(strings.Replace(tracker, "%s", settings, 1))
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
		{"a secret without its file", describe(`{"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}`), "the secret token has no setting token_file"},
		{"a secret in a setting", describe(`{"type": "object", "properties": {"auth": {"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}}}`), "/settings/properties/auth/properties/token is a secret nested in a setting"},
		{"a secret outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "writeOnly": true}}}`), "/settings/$defs/token is a secret outside the settings' properties"},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	ok := describe(`{"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}, "token_file": {"type": "string"}}}`)
	if err := conformance.Description(ok); err != nil {
		t.Errorf("a secret with its file: %v", err)
	}
}

// TestADescriptionKeepsTheRulesOnItsRawBytes pins what Description refuses on the raw
// bytes before any decoder reads them: each document under fixtures/refused-by-reader,
// and a description past each limit description.schema.json states in words.
func TestADescriptionKeepsTheRulesOnItsRawBytes(t *testing.T) {
	refused := map[string]string{
		"fixtures/refused-by-reader/description-string-invalid-utf-8.json":  "describe: the document is not UTF-8",
		"fixtures/refused-by-reader/description-string-lone-surrogate.json": `describe: a string holds the \u escape \ud800 of a lone surrogate`,
		"fixtures/refused-by-reader/description-member-repeated.json":       `describe: an object repeats the member "type"`,
	}
	got := fixtures(t, "fixtures/refused-by-reader")
	if len(got) != len(refused) {
		t.Errorf("fixtures/refused-by-reader holds %d documents, and the test expects %d", len(got), len(refused))
	}
	for name, b := range got {
		if err := conformance.Description(b); err == nil || err.Error() != refused[name] {
			t.Errorf("%s: %v, want %q", name, err, refused[name])
		}
	}
	for _, tc := range []struct{ name, stdout, want string }{
		{"a string of 16386 bytes in 8193 characters", strings.Replace(string(describe(`{"type": "object"}`)), `"title": "Acme tracker"`, `"title": "`+strings.Repeat("\u00e9", 8193)+`"`, 1),
			"describe: a string or a member name is 16386 bytes of UTF-8, more than 16384"},
		{"over 4096 values", string(describe(`{"type": "object", "properties": {"a": {"enum": [` + strings.TrimSuffix(strings.Repeat("0,", 4097), ",") + `]}}}`)),
			"describe: the document holds more than 4096 JSON values"},
		{"65 levels", string(describe(`{"type": "object", "default": ` + strings.Repeat("[", 63) + strings.Repeat("]", 63) + `}`)),
			"describe: the document nests arrays and objects more than 64 levels deep"},
		{"262145 bytes", string(describe(`{"type": "object"}`)) + strings.Repeat(" ", 262145-len(describe(`{"type": "object"}`))),
			"describe: the document is 262145 bytes, more than 262144 (256 KiB)"},
	} {
		if err := conformance.Description([]byte(tc.stdout)); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestCredentialIsTheGatewaysDocument pins Credential to the gateway's schema: an answer
// it accepts conforms, and one it refuses, or two, do not.
func TestCredentialIsTheGatewaysDocument(t *testing.T) {
	answer := `{"version":1,"token":"synthetic","expires_at":"2026-09-25T21:00:00Z","apply":[{"hosts":["tracker.acme.example"],"scheme":"bearer","paths":["/api/*"]}],"placeholders":["TRACKER_TOKEN"]}` + "\n"
	if err := conformance.Credential([]byte(answer)); err != nil {
		t.Errorf("a valid answer: %v", err)
	}
	for _, tc := range []struct{ name, stdout, want string }{
		{"no apply", `{"version":1,"token":"synthetic"}`, "the gateway's schema refuses"},
		{"an unknown scheme", `{"version":1,"token":"synthetic","apply":[{"hosts":["a.example"],"scheme":"cookie"}]}`, "the gateway's schema refuses"},
		{"two documents", answer + answer, "more than one JSON document"},
	} {
		err := conformance.Credential([]byte(tc.stdout))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestCredentialRefusesAReservedHeader pins Credential to conformance/headers.json: an
// apply entry of the scheme header is refused when its header, in lower case, is a name
// the file lists or starts with a prefix it lists, and each such entry is a refusal of
// its own, in the order of apply.
func TestCredentialRefusesAReservedHeader(t *testing.T) {
	answer := func(headers ...string) []byte {
		var apply []string
		for _, h := range headers {
			apply = append(apply, `{"hosts":["api.acme.example"],"scheme":"header","header":"`+h+`"}`)
		}
		return []byte(`{"version":1,"token":"synthetic","apply":[` + strings.Join(apply, ",") + `]}` + "\n")
	}
	for _, tc := range []struct{ name, header string }{
		{"a refused name in mixed case", "CoOkIe"},
		{"a refused prefix", "X-Forwarded-Host"},
		{"accept-language, under the prefix accept", "accept-language"},
	} {
		err := conformance.Credential(answer(tc.header))
		want := "apply[0] sets the header " + tc.header + ", which conformance reserves"
		if err == nil || err.Error() != "credential: "+want {
			t.Errorf("%s: %v, want %q", tc.name, err, "credential: "+want)
		}
	}
	if err := conformance.Credential(answer("x-api-key")); err != nil {
		t.Errorf("x-api-key, which conformance does not reserve: %v", err)
	}

	err := conformance.Credential(answer("Authorization", "X-Api-Key", "Sec-Fetch-Mode", "x-qory-run"))
	want := []string{
		"apply[0] sets the header Authorization, which conformance reserves",
		"apply[2] sets the header Sec-Fetch-Mode, which conformance reserves",
		"apply[3] sets the header x-qory-run, which conformance reserves",
	}
	if got := unwrapped(t, err); !slices.Equal(got, want) {
		t.Errorf("Unwrap() is %q, want %q", got, want)
	}
	if err.Error() != "credential: "+strings.Join(want, "; ") {
		t.Errorf("Error() is %q, want the refusals joined by %q after %q", err, "; ", "credential: ")
	}

	bearer := []byte(`{"version":1,"token":"synthetic","apply":[{"hosts":["api.acme.example"],"scheme":"bearer","header":"Cookie"}]}`)
	if err := conformance.Credential(bearer); err != nil {
		t.Errorf("a header beside the scheme bearer, which does not read it: %v", err)
	}
}

// unwrapped are the texts of the errors err's Unwrap() []error returns.
func unwrapped(t *testing.T, err error) []string {
	t.Helper()
	r, ok := err.(interface{ Unwrap() []error })
	if !ok {
		t.Fatalf("%v has no Unwrap() []error", err)
	}
	var out []string
	for _, e := range r.Unwrap() {
		out = append(out, e.Error())
	}
	return out
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

// dataFixtures are the data descriptions among the contract's fixtures.
var dataFixtures = []string{"fixtures/acme-models.json", "fixtures/example-gateway.json"}

// gateway is a data description with one API whose endpoint is the setting url, with
// the settings and the models put in.
func gateway(settings, models string) []byte {
	return []byte(`{"version": 1, "name": "example-gateway", "title": "Example gateway", "settings": ` + settings + `,
 "roles": {"model_provider": {"apis": [{"format": "openai-chat", "endpoint": {"setting": "url"}, "auth": {"scheme": "bearer"}}], "models": ` + models + `}},
 "egress_hosts": [{"setting": "url"}]}
`)
}

// TestADataDescriptionIsTheDataForm pins what DataDescription accepts and refuses: the
// contract's data fixtures conform, and it refuses the program form, a document the
// schema refuses, a rule of the model_provider role the schema cannot express, a data
// description that only the check of its raw bytes refuses, and a file that is not one
// document.
func TestADataDescriptionIsTheDataForm(t *testing.T) {
	all := fixtures(t, "fixtures")
	for _, name := range dataFixtures {
		if err := conformance.DataDescription(all[name]); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	program := "data description: the file has a program_version, so it is in the program form; a data description has no program_version"
	for name, b := range all {
		if !slices.Contains(dataFixtures, name) {
			if err := conformance.DataDescription(b); err == nil || err.Error() != program {
				t.Errorf("%s: %v, want %q", name, err, program)
			}
		}
	}
	for name, b := range fixtures(t, "fixtures/invalid") {
		if strings.Contains(name, "/description-data-") {
			want := "data description: the contract's schema refuses the description: "
			if err := conformance.DataDescription(b); err == nil || !strings.HasPrefix(err.Error(), want) {
				t.Errorf("%s: %v, want it to start with %q", name, err, want)
			}
		}
	}
	url := `{"type": "object", "properties": {"url": {"type": "string"}}}`
	one := `[{"id": "acme-large-2", "title": "Acme Large 2"}]`
	// Each of these data descriptions is accepted by the schema once a decoder has read
	// it leniently, so only the check of the raw bytes refuses it.
	for _, tc := range []struct{ name, b, want string }{
		{"settings that repeat the member type", string(gateway(`{"type": "object", "type": "object", "properties": {"url": {"type": "string"}}}`, one)),
			`data description: an object repeats the member "type"`},
		{"a title with a lone surrogate", strings.Replace(string(gateway(url, one)), `"title": "Example gateway"`, `"title": "Example \ud800 gateway"`, 1),
			`data description: a string holds the \u escape \ud800 of a lone surrogate`},
		{"a title that is not UTF-8", strings.Replace(string(gateway(url, one)), `"title": "Example gateway"`, "\"title\": \"Example \xff gateway\"", 1),
			"data description: the document is not UTF-8"},
	} {
		if err := conformance.DataDescription([]byte(tc.b)); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name string
		b    []byte
		want string
	}{
		{"nothing", nil, "data description: the file is not a JSON document: EOF"},
		{"two documents", append(gateway(url, one), "{}\n"...), "data description: the file contains more than one JSON document"},
		{"a setting that does not exist", gateway(`{"type": "object"}`, one),
			`data description: the endpoint of model_provider apis[0] is the setting "url", and settings has no property "url"`},
		{"two models with one id", gateway(url, `[{"id": "acme-large-2", "title": "Acme Large 2"}, {"id": "acme-large-2", "title": "Acme Large 2 again"}]`),
			`data description: the model_provider models 0 and 1 have the same id "acme-large-2"; no two models have the same id`},
		{"two APIs with one format", bytes.Replace(gateway(url, one), []byte(`"auth": {"scheme": "bearer"}}]`),
			[]byte(`"auth": {"scheme": "bearer"}}, {"format": "openai-chat", "endpoint": {"setting": "url"}, "auth": {"scheme": "header", "header": "x-api-key"}}]`), 1),
			`data description: the model_provider apis 0 and 1 have the same format "openai-chat"; no two APIs of a provider have the same format`},
		{"an endpoint with a *. host", []byte(`{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://*.acme.example", "auth": {"scheme": "bearer"}}], "models": []}},
 "egress_hosts": ["api.acme.example"]}`),
			`data description: the endpoint "https://*.acme.example" of model_provider apis[0] has no host a gateway can read: the host "*.acme.example" is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit`},
		{"an endpoint with port 0", []byte(`{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://api.acme.example:0", "auth": {"scheme": "bearer"}}], "models": []}},
 "egress_hosts": ["api.acme.example"]}`),
			`data description: the endpoint "https://api.acme.example:0" of model_provider apis[0] has no host a gateway can read: the port "0" is not a number from 1 to 65535`},
		{"an endpoint whose host has no equal entry", []byte(`{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://evil.example", "auth": {"scheme": "bearer"}}], "models": []}},
 "egress_hosts": ["api.acme.example"]}`),
			`data description: add "evil.example" to egress_hosts: the host "evil.example" of the endpoint of model_provider apis[0] has no equal entry there, and each API endpoint host needs one`},
	} {
		if err := conformance.DataDescription(tc.b); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	if err := conformance.DataDescription(gateway(url, one)); err != nil {
		t.Errorf("a data description with its endpoint setting: %v", err)
	}
}

// TestADescriptionKeepsTheModelProviderRules pins that Description, which accepts both
// forms, refuses a rule of the model_provider role the schema cannot express.
func TestADescriptionKeepsTheModelProviderRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    []byte
		want string
	}{
		{"a setting that does not exist", gateway(`{"type": "object"}`, `[]`),
			`describe: the endpoint of model_provider apis[0] is the setting "url", and settings has no property "url"`},
		{"a writeOnly setting, in the program form", bytes.Replace(gateway(`{"type": "object", "properties": {"url": {"type": "string", "writeOnly": true}, "url_file": {"type": "string"}}}`, `[]`), []byte(`"settings":`), []byte(`"program_version": "0.1.0", "settings":`), 1),
			`describe: the endpoint of model_provider apis[0] is the setting "url", which is marked writeOnly; an endpoint setting is never a secret`},
		{"two models with one id", gateway(`{"type": "object", "properties": {"url": {"type": "string"}}}`, `[{"id": "acme-large-2", "title": "Acme Large 2"}, {"id": "acme-large-2", "title": "Acme Large 2 again"}]`),
			`describe: the model_provider models 0 and 1 have the same id "acme-large-2"; no two models have the same id`},
	} {
		if err := conformance.Description(tc.b); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestAModelProviderSetsNoReservedHeader pins that Description and DataDescription
// refuse the auth of a model_provider API whose header conformance reserves: a
// reserved name, compared in lower case, and a name with a reserved prefix, in the
// second of two APIs.
func TestAModelProviderSetsNoReservedHeader(t *testing.T) {
	data := func(header string) []byte {
		return []byte(`{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "openai-chat", "endpoint": "https://api.acme.example/v1", "auth": {"scheme": "bearer"}},
 {"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": {"scheme": "header", "header": "` + header + `"}}], "models": []}},
 "egress_hosts": ["api.acme.example"]}
`)
	}
	for _, header := range []string{"Cookie", "X-Forwarded-Host"} {
		b := data(header)
		want := "the auth of model_provider apis[1] sets the header " + header + ", which conformance reserves"
		if err := conformance.DataDescription(b); err == nil || err.Error() != "data description: "+want {
			t.Errorf("DataDescription, %s: %v, want %q", header, err, "data description: "+want)
		}
		if err := conformance.Description(b); err == nil || err.Error() != "describe: "+want {
			t.Errorf("Description, %s: %v, want %q", header, err, "describe: "+want)
		}
	}
	if err := conformance.DataDescription(data("x-api-key")); err != nil {
		t.Errorf("x-api-key: %v", err)
	}
}

// TestBothFormsKeepTheEgressHostsRules pins that Description and DataDescription refuse
// each rule of egress_hosts the schema cannot express, with the message that states it:
// a credential host or an API endpoint host with no equal entry, an endpoint setting
// with no entry of the same setting, and a setting entry whose setting is missing, not
// a string, or a secret.
func TestBothFormsKeepTheEgressHostsRules(t *testing.T) {
	url := `{"type": "object", "properties": {"url": {"type": "string"}}}`
	data := func(egress string) []byte {
		return []byte(`{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object"},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://api.acme.example", "auth": {"scheme": "bearer"}},
 {"format": "openai-chat", "endpoint": "https://files.acme.example/v1", "auth": {"scheme": "bearer"}}], "models": []}},
 "egress_hosts": ` + egress + `}
`)
	}
	program := func(b []byte) []byte {
		return bytes.Replace(b, []byte(`"settings":`), []byte(`"program_version": "0.1.0", "settings":`), 1)
	}
	noEntry := bytes.Replace(gateway(url, `[]`), []byte(`[{"setting": "url"}]`), []byte(`["api.acme.example"]`), 1)
	for _, tc := range []struct {
		name string
		b    []byte
		want string
	}{
		{"an API endpoint host with no equal entry", data(`["api.acme.example"]`),
			`add "files.acme.example" to egress_hosts: the host "files.acme.example" of the endpoint of model_provider apis[1] has no equal entry there, and each API endpoint host needs one`},
		{"an endpoint setting with no entry of that setting", noEntry,
			`the endpoint of model_provider apis[0] is the setting "url", and egress_hosts has no entry {"setting": "url"}; an API endpoint that is a setting has an egress_hosts entry with the same setting`},
		{"an entry whose setting does not exist", data(`["api.acme.example", "files.acme.example", {"setting": "mirror"}]`),
			`the setting "mirror" of an egress_hosts entry is not a property of settings`},
		{"an entry whose setting is not a string", bytes.Replace(data(`["api.acme.example", "files.acme.example", {"setting": "mirror"}]`), []byte(`"settings": {"type": "object"}`), []byte(`"settings": {"type": "object", "properties": {"mirror": {"type": "integer"}}}`), 1),
			`the setting "mirror" of an egress_hosts entry is not of type string`},
	} {
		if err := conformance.DataDescription(tc.b); err == nil || err.Error() != "data description: "+tc.want {
			t.Errorf("DataDescription, %s: %v, want %q", tc.name, err, "data description: "+tc.want)
		}
		if err := conformance.Description(program(tc.b)); err == nil || err.Error() != "describe: "+tc.want {
			t.Errorf("Description, %s: %v, want %q", tc.name, err, "describe: "+tc.want)
		}
	}
	credential := describe(`{"type": "object"}`)
	for _, tc := range []struct {
		name string
		b    []byte
		want string
	}{
		{"a credential host with no equal entry", bytes.Replace(credential, []byte(`"egress_hosts": ["tracker.acme.example"]`), []byte(`"egress_hosts": ["acme.example"]`), 1),
			`describe: add "tracker.acme.example" to egress_hosts: the credential host "tracker.acme.example" has no equal entry there, and each host in a role's hosts needs one`},
		{"an entry whose setting is marked writeOnly", bytes.Replace(describe(`{"type": "object", "properties": {"mirror": {"type": "string", "writeOnly": true}, "mirror_file": {"type": "string"}}}`),
			[]byte(`"egress_hosts": ["tracker.acme.example"]`), []byte(`"egress_hosts": ["tracker.acme.example", {"setting": "mirror"}]`), 1),
			`describe: the setting "mirror" of an egress_hosts entry is marked writeOnly; the setting of an egress_hosts entry is never a secret`},
	} {
		if err := conformance.Description(tc.b); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	if err := conformance.DataDescription(data(`["api.acme.example", "files.acme.example"]`)); err != nil {
		t.Errorf("API endpoint hosts with an equal entry each: %v", err)
	}
	refused := "the contract's schema refuses the description: "
	if err := conformance.DataDescription(data(`["*.acme.example"]`)); err == nil || !strings.HasPrefix(err.Error(), "data description: "+refused) {
		t.Errorf("DataDescription, API endpoint hosts under a *. entry: %v, want it to start with %q", err, "data description: "+refused)
	}
	if err := conformance.Description(program(data(`["*.acme.example"]`))); err == nil || !strings.HasPrefix(err.Error(), "describe: "+refused) {
		t.Errorf("Description, API endpoint hosts under a *. entry: %v, want it to start with %q", err, "describe: "+refused)
	}
	wildcard := bytes.ReplaceAll(credential, []byte(`["tracker.acme.example"]`), []byte(`["*.acme.example"]`))
	if err := conformance.Description(wildcard); err == nil || !strings.HasPrefix(err.Error(), "describe: "+refused) {
		t.Errorf("Description, a *. credential host and the same *. entry: %v, want it to start with %q", err, "describe: "+refused)
	}
}
