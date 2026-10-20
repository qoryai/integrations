package contracts_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// TestAnEgressHostsEntryIsRefusedForTheRuleInItsName pins the reason of each invalid
// description whose egress_hosts entry breaks the host grammar or is a local or
// over-broad host, which the refusal does not name: the same document is accepted once
// the one part its name points to is fixed.
func TestAnEgressHostsEntryIsRefusedForTheRuleInItsName(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for name, fix := range map[string][2]string{
		"fixtures/invalid/description-egress-hosts-port.json":                  {`.example:443"`, `.example"`},
		"fixtures/invalid/description-egress-hosts-upper-case.json":            {`"Tracker.`, `"tracker.`},
		"fixtures/invalid/description-egress-hosts-bare-wildcard.json":         {`"*."`, `"tracker.acme.example"`},
		"fixtures/invalid/description-egress-hosts-wildcard.json":              {`"*.acme.example"`, `"tracker.acme.example"`},
		"fixtures/invalid/description-egress-hosts-single-label.json":          {`"intranet"`, `"intranet.acme.example"`},
		"fixtures/invalid/description-egress-hosts-wildcard-single-label.json": {`"*.example"`, `"tracker.acme.example"`},
		"fixtures/invalid/description-egress-hosts-localhost.json":             {`"localhost"`, `"localhost.acme.example"`},
		"fixtures/invalid/description-egress-hosts-under-localhost.json":       {`.localhost"`, `.acme.example"`},
		"fixtures/invalid/description-egress-hosts-loopback.json":              {`"127.`, `"10.`},
		"fixtures/invalid/description-egress-hosts-link-local.json":            {`"169.254.169.254"`, `"169.254.example.com"`},
		"fixtures/invalid/description-egress-hosts-zero-network.json":          {`"0.0.0.0"`, `"10.0.0.0"`},
		"fixtures/invalid/description-egress-hosts-localhost-localdomain.json": {`.localdomain"`, `.localdomain.example"`},
		"fixtures/invalid/description-egress-hosts-number-form.json":           {`"0177.`, `"177.`},
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

// TestAnEgressHostsHostIsNeitherLocalNorOverBroad pins the hosts that egress_hosts
// refuses although $defs/host accepts them, beside hosts that look alike and are
// accepted, and the *. hosts and IPv6 addresses that $defs/host already refuses. The
// hosts of the credential role keep $defs/host alone, and a model_provider API
// endpoint keeps $defs/endpoint alone.
func TestAnEgressHostsHostIsNeitherLocalNorOverBroad(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, tc := range []struct{ host, want string }{
		{"intranet", "/egress_hosts/0 "},
		{"*.example", "/egress_hosts/0 pattern"},
		{"*.acme.example", "/egress_hosts/0 pattern"},
		{"localhost", "/egress_hosts/0 "},
		{"a.localhost", "/egress_hosts/0 "},
		{"*.localhost", "/egress_hosts/0 pattern; /egress_hosts/0 "},
		{"localhost4", "/egress_hosts/0 "},
		{"localhost.localdomain", "/egress_hosts/0 "},
		{"a.localhost.localdomain", "/egress_hosts/0 "},
		{"localhost4.localdomain4", "/egress_hosts/0 "},
		{"localhost6.localdomain6", "/egress_hosts/0 "},
		{"localhost.localdomain6", "/egress_hosts/0 "},
		{"127.0.0.1", "/egress_hosts/0 "},
		{"127.255.255.255", "/egress_hosts/0 "},
		{"169.254.169.254", "/egress_hosts/0 "},
		{"0.0.0.0", "/egress_hosts/0 "},
		{"127.1", "/egress_hosts/0 "},
		{"0177.0.0.1", "/egress_hosts/0 "},
		{"0x7f.0.0.1", "/egress_hosts/0 "},
		{"0.0", "/egress_hosts/0 "},
		{"169.254.43518", "/egress_hosts/0 "},
		{"10.0.0.01", "/egress_hosts/0 "},
		{"10.0.0.256", "/egress_hosts/0 "},
		{"acme.example.0x1f", "/egress_hosts/0 "},
		{"acme.example.0x", "/egress_hosts/0 "},
		{"*.0.0.1", "/egress_hosts/0 pattern; /egress_hosts/0 "},
		{"::1", "/egress_hosts/0 pattern; /egress_hosts/0 "},
		{"[::1]", "/egress_hosts/0 pattern; /egress_hosts/0 "},
		{"fe80::1", "/egress_hosts/0 pattern; /egress_hosts/0 "},
		{"10.0.0.1", ""},
		{"1.2.3.4", ""},
		{"255.255.255.255", ""},
		{"0x7f.example", ""},
		{"localhost.localdomain.example", ""},
		{"0.example.com", ""},
		{"127.example.com", ""},
		{"169.254.example.com", ""},
		{"localhost.acme.example", ""},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(tracker(`{"type": "object"}`, `["tracker.acme.example"]`, `["`+tc.host+`"]`)))
		if err != nil {
			t.Fatalf("%s: %v", tc.host, err)
		}
		got := ""
		if v, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.host, got, tc.want)
		}
	}
	for _, host := range []string{"localhost", "127.0.0.1", "127.1"} {
		for role, doc := range map[string]string{
			"credential":     tracker(`{"type": "object"}`, `["`+host+`"]`, ""),
			"model_provider": static("https://" + host),
		} {
			v, err := jsonschema.UnmarshalJSON(strings.NewReader(doc))
			if err != nil {
				t.Fatalf("%s %s: %v", role, host, err)
			}
			if err := schema.Validate(v); err != nil {
				t.Errorf("the %s host %s is refused, and only egress_hosts refuses it: %v", role, host, err)
			}
		}
	}
}

// tracker is a description in the program form with a credential role, with the
// settings, the credential hosts and the egress_hosts member put in; an empty egress
// leaves the member out.
func tracker(settings, hosts, egress string) string {
	doc := `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0", "settings": ` + settings + `,
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ` + hosts + `}}`
	if egress != "" {
		doc += `, "egress_hosts": ` + egress
	}
	return doc + "}"
}

// withEgress is a description with the egress_hosts member put in at its end.
func withEgress(doc, egress string) string {
	return strings.TrimSuffix(doc, "}") + `, "egress_hosts": ` + egress + "}"
}

// TestCheckEgressHostsHoldsTheRulesTheSchemaDoesNotCheck pins the rules of egress_hosts
// that the schema does not check: each refused document is accepted by the schema and
// refused by CheckEgressHosts with a message that states the rule; each accepted
// document is accepted by both; and every description under fixtures keeps the rules.
func TestCheckEgressHostsHoldsTheRulesTheSchemaDoesNotCheck(t *testing.T) {
	schema := compile(t, "description.schema.json")
	object := `{"type": "object"}`
	url := `{"type": "object", "properties": {"url": {"type": "string"}}}`
	one := `[{"id": "acme-large-2", "title": "Acme Large 2"}]`
	credential := func(host string) string {
		return fmt.Sprintf(`add %q to egress_hosts: the credential host %q has no equal entry there, and each host in a role's hosts needs one`, host, host)
	}
	endpoint := func(i int, host string) string {
		return fmt.Sprintf(`add %q to egress_hosts: the host %q of the endpoint of model_provider apis[%d] has no equal entry there, and each API endpoint host needs one`, host, host, i)
	}
	setting := func(i int, name string) string {
		return fmt.Sprintf(`the endpoint of model_provider apis[%d] is the setting %q, and egress_hosts has no entry {"setting": %q}; an API endpoint that is a setting has an egress_hosts entry with the same setting`, i, name, name)
	}
	two := `{"version": 1, "name": "acme-models", "title": "Acme models", "settings": {"type": "object", "properties": {"url": {"type": "string"}}},
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://api.acme.example/anthropic", "auth": {"scheme": "bearer"}},
 {"format": "openai-chat", "endpoint": {"setting": "url"}, "auth": {"scheme": "bearer"}}], "models": []}}}`
	for _, tc := range []struct{ name, doc, want string }{
		{"a credential host and no egress_hosts", tracker(object, `["tracker.acme.example"]`, ""),
			credential("tracker.acme.example")},
		{"a credential host with no equal entry", tracker(object, `["tracker.acme.example", "files.acme.example"]`, `["tracker.acme.example"]`),
			credential("files.acme.example")},
		{"an entry and a credential host under it", tracker(object, `["api.acme.example"]`, `["acme.example"]`),
			credential("api.acme.example")},
		{"an API endpoint and no egress_hosts", static("https://api.acme.example"), endpoint(0, "api.acme.example")},
		{"an API endpoint with no equal entry", withEgress(static("https://files.acme.example/v1"), `["api.acme.example"]`), endpoint(0, "files.acme.example")},
		{"an API endpoint with a port and no equal entry", withEgress(static("https://files.acme.example:8443"), `["api.acme.example"]`), endpoint(0, "files.acme.example")},
		{"an entry and an API endpoint host under it", withEgress(static("https://api.acme.example"), `["acme.example"]`), endpoint(0, "api.acme.example")},
		{"an API endpoint host in upper case", withEgress(static("https://API.acme.example"), `["api.acme.example"]`),
			`the endpoint "https://API.acme.example" of model_provider apis[0] has no host a gateway can read: the host "API.acme.example" is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit`},
		{"a second API whose setting has no entry", withEgress(two, `["api.acme.example"]`), setting(1, "url")},
		{"a first API with no equal entry, the second's setting with one", withEgress(two, `[{"setting": "url"}]`), endpoint(0, "api.acme.example")},
		{"an endpoint setting and no egress_hosts", gateway(url, one), setting(0, "url")},
		{"an endpoint setting and an entry with another setting",
			withEgress(gateway(`{"type": "object", "properties": {"url": {"type": "string"}, "files": {"type": "string"}}}`, one), `[{"setting": "files"}]`),
			setting(0, "url")},
		{"an entry whose setting does not exist", tracker(object, `["tracker.acme.example"]`, `["tracker.acme.example", {"setting": "mirror"}]`),
			`the setting "mirror" of an egress_hosts entry is not a property of settings`},
		{"an entry whose setting is not a string", tracker(`{"type": "object", "properties": {"mirror": {"type": "integer"}}}`, `["tracker.acme.example"]`, `["tracker.acme.example", {"setting": "mirror"}]`),
			`the setting "mirror" of an egress_hosts entry is not of type string`},
		{"an entry whose setting is marked writeOnly", tracker(`{"type": "object", "properties": {"mirror": {"type": "string", "writeOnly": true}, "mirror_file": {"type": "string"}}}`, `["tracker.acme.example"]`, `["tracker.acme.example", {"setting": "mirror"}]`),
			`the setting "mirror" of an egress_hosts entry is marked writeOnly; the setting of an egress_hosts entry is never a secret`},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.doc))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: the schema refuses it, and the rule is CheckEgressHosts's alone: %v", tc.name, err)
		}
		if err := contracts.CheckEgressHosts(doc); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct{ name, doc string }{
		{"an equal entry for each credential host", tracker(object, `["api.acme.example", "eu.files.acme.example"]`, `["eu.files.acme.example", "api.acme.example"]`)},
		{"an exact entry and an equal IPv4 address", tracker(object, `["10.0.0.1"]`, `["10.0.0.1"]`)},
		{"an equal entry and an API endpoint with a port and a path", withEgress(static("https://api.acme.example:8443/v1"), `["api.acme.example"]`)},
		{"two APIs, an equal entry and a setting entry", withEgress(two, `["api.acme.example", {"setting": "url"}]`)},
		{"an endpoint setting and an entry with the same setting", withEgress(gateway(url, one), `[{"setting": "url"}]`)},
		{"no role hosts and no egress_hosts", `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0", "settings": {"type": "object"}, "roles": {"acme_role": {}}}`},
		{"no role hosts and an entry", `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0", "settings": {"type": "object"}, "roles": {"acme_role": {}}, "egress_hosts": ["files.acme.example"]}`},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.doc))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: the schema refuses it: %v", tc.name, err)
		}
		if err := contracts.CheckEgressHosts(doc); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	for _, f := range files(t, "fixtures") {
		doc, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckEgressHosts(doc); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// TestAHostWithAStarDotInFrontIsRefused pins that the schema refuses a *. entry of
// egress_hosts and a *. host of the credential role, so neither reaches
// CheckEgressHosts: each host is listed exactly.
func TestAHostWithAStarDotInFrontIsRefused(t *testing.T) {
	schema := compile(t, "description.schema.json")
	object := `{"type": "object"}`
	for _, tc := range []struct{ name, doc, want string }{
		{"a *. entry and a credential host under it", tracker(object, `["api.acme.example"]`, `["*.acme.example"]`), "/egress_hosts/0 pattern"},
		{"a *. entry and the credential host it is in front of", tracker(object, `["acme.example"]`, `["*.acme.example"]`), "/egress_hosts/0 pattern"},
		{"an exact entry and a *. credential host", tracker(object, `["*.acme.example"]`, `["api.acme.example"]`), "/roles/credential/hosts/0 pattern"},
		{"a *. entry and the same *. credential host", tracker(object, `["*.acme.example"]`, `["*.acme.example"]`),
			"/roles/credential/hosts/0 pattern; /egress_hosts/0 pattern"},
		{"a deeper *. entry and a *. credential host above it", tracker(object, `["*.acme.example"]`, `["*.api.acme.example"]`),
			"/roles/credential/hosts/0 pattern; /egress_hosts/0 pattern"},
		{"a *. entry and hosts under it, a *. host among them", tracker(object, `["api.acme.example", "eu.files.acme.example", "*.files.acme.example"]`, `["*.acme.example"]`),
			"/roles/credential/hosts/2 pattern; /egress_hosts/0 pattern"},
		{"a *. entry and an API endpoint host under it", withEgress(static("https://api.acme.example"), `["*.acme.example"]`), "/egress_hosts/0 pattern"},
		{"a *. entry and the API endpoint host it is in front of", withEgress(static("https://acme.example"), `["*.acme.example"]`), "/egress_hosts/0 pattern"},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(tc.doc))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := ""
		if v, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if unordered(got) != unordered(tc.want) {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestASettingEntryIsTheSettingAlone pins the setting entry of egress_hosts to its
// schema: an object with setting alone, a string of at least one character.
func TestASettingEntryIsTheSettingAlone(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for _, tc := range []struct{ name, egress, want string }{
		{"a setting", `[{"setting": "url"}]`, ""},
		{"an empty setting", `[{"setting": ""}]`, "/egress_hosts/0/setting minLength"},
		{"no setting", `[{}]`, "/egress_hosts/0 required setting"},
		{"a member besides setting", `[{"setting": "url", "port": 443}]`, "/egress_hosts/0 additionalProperties"},
		{"the same setting twice", `[{"setting": "url"}, {"setting": "url"}]`, "/egress_hosts uniqueItems"},
	} {
		doc, err := jsonschema.UnmarshalJSON(strings.NewReader(withEgress(gateway(`{"type": "object", "properties": {"url": {"type": "string"}}}`, `[]`), tc.egress)))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := ""
		if v, ok := schema.Validate(doc).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != tc.want {
			t.Errorf("%s: refused for %q, want %q", tc.name, got, tc.want)
		}
	}
}
