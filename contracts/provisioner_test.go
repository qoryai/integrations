package contracts_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// provisionerSchemas are the schemas of the provisioner contract.
var provisionerSchemas = []string{
	"definitions.schema.json",
	"claim.schema.json",
	"claim-answer.schema.json",
	"dispatched.schema.json",
	"release.schema.json",
	"machine-log.schema.json",
	"refusal.schema.json",
}

// provisionerSchema is the path of the schema that the fixtures under fixtures/<name>,
// and the invalid fixtures whose expected reason names name, validate against:
// <name>.schema.json.
func provisionerSchema(name string) string {
	return name + ".schema.json"
}

// compileProvisioner compiles a schema of the provisioner contract, or fails the test.
func compileProvisioner(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.CompileProvisioner(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// provisionerFiles are the fixtures of the provisioner contract directly under dir.
func provisionerFiles(t *testing.T, dir string) []string {
	t.Helper()
	return treeFiles(t, contracts.ProvisionerFS, dir)
}

// treeFiles are the fixtures of the contract directory fsys directly under dir.
func treeFiles(t *testing.T, fsys fs.FS, dir string) []string {
	t.Helper()
	entries, err := fs.ReadDir(fsys, dir)
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

// TestProvisionerFixturesValidate pins that each document under fixtures/<name>/ of the
// provisioner contract is accepted by the schema [provisionerSchema] gives for name, and
// that every schema of the contract compiles.
func TestProvisionerFixturesValidate(t *testing.T) {
	for _, name := range provisionerSchemas {
		compileProvisioner(t, name)
	}
	dirs, err := fs.ReadDir(contracts.ProvisionerFS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "invalid" {
			continue
		}
		schema := compileProvisioner(t, provisionerSchema(d.Name()))
		for _, f := range provisionerFiles(t, path.Join("fixtures", d.Name())) {
			seen[d.Name()] = true
			doc, err := contracts.ProvisionerDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
	for _, name := range provisionerSchemas[1:] {
		if !seen[strings.TrimSuffix(name, ".schema.json")] {
			t.Errorf("%s has no fixture", name)
		}
	}
}

// TestInvalidProvisionerFixturesAreRefused pins that each document under
// fixtures/invalid of the provisioner contract fails its schema for the reason in its
// name, and for no other.
func TestInvalidProvisionerFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"claim-wait-seconds-26.json":                     {"claim", "/wait_seconds maximum"},
		"claim-pool-id-node.json":                        {"claim", "/pool_id pattern"},
		"claim-undefined-member.json":                    {"claim", "/ additionalProperties"},
		"claim-answer-dispatch-id-no-prefix.json":        {"claim-answer", "/dispatch_id pattern"},
		"claim-answer-dispatch-id-upper-case.json":       {"claim-answer", "/dispatch_id pattern"},
		"dispatched-external-type-upper-case.json":       {"dispatched", "/external/type pattern"},
		"dispatched-external-ref-nul.json":               {"dispatched", "/external/ref pattern"},
		"release-with-retry.json":                        {"release", "/ additionalProperties"},
		"release-reason-nul.json":                        {"release", "/reason pattern"},
		"claim-answer-revision-2.json":                   {"claim-answer", "/revision const"},
		"claim-answer-run-code-qecn-prefix.json":         {"claim-answer", "/run_code pattern"},
		"claim-answer-gateway-url-path.json":             {"claim-answer", "/gateway_url pattern"},
		"claim-answer-gateway-url-http.json":             {"claim-answer", "/gateway_url pattern"},
		"claim-answer-without-gateway-url.json":          {"claim-answer", "/ required gateway_url"},
		"claim-answer-without-run-code.json":             {"claim-answer", "/ required run_code"},
		"claim-answer-run-id.json":                       {"claim-answer", "/ additionalProperties"},
		"claim-answer-max-lifetime-minutes-14.json":      {"claim-answer", "/max_lifetime_minutes minimum"},
		"claim-answer-max-lifetime-minutes-1471.json":    {"claim-answer", "/max_lifetime_minutes maximum"},
		"claim-answer-without-max-lifetime-minutes.json": {"claim-answer", "/ required max_lifetime_minutes"},
		"claim-answer-machine-upper-case.json":           {"claim-answer", "/machine pattern"},
		"machine-log-without-log.json":                   {"machine-log", "/ required log"},
		"machine-log-undefined-member.json":              {"machine-log", "/ additionalProperties"},
		"machine-log-log-nul.json":                       {"machine-log", "/log pattern"},
		"machine-log-version-2.json":                     {"machine-log", "/version const"},
	}
	got := provisionerFiles(t, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.ProvisionerDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileProvisioner(t, provisionerSchema(w.schema)).Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// TestProvisionerTypesMarshalToTheirSchemas marshals one value of each request and
// answer type of the provisioner contract, with its optional members left out where it
// has them, and validates it against its schema.
func TestProvisionerTypesMarshalToTheirSchemas(t *testing.T) {
	now := contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	values := []struct {
		schema string
		v      any
	}{
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, WaitSeconds: 25}},
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, PoolID: "np_0123456789abcdef"}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, RunCode: runCode, GatewayURL: gatewayURL,
			MaxLifetimeMinutes: 1470}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now,
			External: &contracts.External{Type: "example-ci", Ref: "run-1", URL: "https://ci.example.com/runs/1"}}},
		{"release", contracts.Release{Version: contracts.Version, Time: now, Reason: "No machine of this type was free."}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, Machine: "small", RunCode: runCode, GatewayURL: gatewayURL,
			MaxLifetimeMinutes: 1470}},
		{"machine-log", contracts.MachineLog{Version: contracts.Version, Time: now, Log: "machine created\n"}},
		{"machine-log", contracts.MachineLog{Version: contracts.Version, Time: now, Log: "machine stopped\n", Truncated: true}},
		{"refusal", contracts.Refusal{Error: "machine_log_exists"}},
		{"refusal", contracts.Refusal{Error: "machine_exists"}},
		{"refusal", contracts.Refusal{Error: "lease_expired"}},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if err := validate(compileProvisioner(t, provisionerSchema(c.schema)), b); err != nil {
			t.Errorf("%T marshals to %s, which %s refuses: %v", c.v, b, provisionerSchema(c.schema), err)
		}
	}
}

// runCode is a run code in the form claim-answer.schema.json gives.
const runCode = "qecr_01JB8Z3K5Q7R9T2V4W6X8Y0A1C.uoES-kuj1vk0sq0qoGlmAg"

// gatewayURL is a gateway URL in the form claim-answer.schema.json gives.
const gatewayURL = "https://gateway.example:8443"

// TestAGatewayURLIsHostAndPort pins the gateway_url of claim-answer.schema.json:
// https://, a host of DNS labels or an IPv6 address in brackets, an optional port
// from 1 to 65535 with no leading zero, nothing after them, and at most 300
// characters; a value of 301 characters that the pattern accepts is refused by
// maxLength alone.
func TestAGatewayURLIsHostAndPort(t *testing.T) {
	schema := compileProvisioner(t, "claim-answer.schema.json#/properties/gateway_url")
	label := strings.Repeat("g", 63)
	longest := "https://" + label + "." + label + "." + label + "." + label + "." + strings.Repeat("g", 36)
	if len(longest) != 300 {
		t.Fatalf("the longest gateway URL has %d characters, want 300", len(longest))
	}
	for _, v := range []string{
		"https://gateway:8443",
		"https://gateway.example",
		"https://gateway.example:65535",
		"https://[::1]:8443",
		"https://10.0.0.5:8443",
		longest,
	} {
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s is refused: %v", v, err)
		}
	}
	for _, v := range []string{
		"https://:443",
		"https://host:",
		"https://host:0",
		"https://host:65536",
		"https://host:080",
		"https://<>",
		"https://ho_st",
		"https://host.",
		"https://user@host",
		"https://ex%41mple.com",
		"https://host/path",
		"http://host",
		"https://" + strings.Repeat("g", 64) + ".example",
	} {
		if err := schema.Validate(v); err == nil {
			t.Errorf("%s is accepted", v)
		}
	}
	v, ok := schema.Validate(longest + "g").(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("a gateway URL of 301 characters is accepted")
	}
	if r := strings.Join(leaves(v), "; "); r != "/ maxLength" {
		t.Errorf("a gateway URL of 301 characters is refused for %q, want %q", r, "/ maxLength")
	}
}

// TestTheNewMembersRoundTripTheirFixtures pins that each claim answer fixture, and each
// other fixture with run_code or truncated, reads into its Go type and is written back
// as the fixture's bytes, compacted.
func TestTheNewMembersRoundTripTheirFixtures(t *testing.T) {
	type fixture struct {
		fixture string
		v       any
	}
	var answers []fixture
	for _, f := range provisionerFiles(t, "fixtures/claim-answer") {
		answers = append(answers, fixture{f, &contracts.ClaimAnswer{}})
	}
	for _, c := range append(answers, []fixture{
		{"fixtures/machine-log/whole.json", &contracts.MachineLog{}},
		{"fixtures/machine-log/truncated.json", &contracts.MachineLog{}},
	}...) {
		b, err := fs.ReadFile(contracts.ProvisionerFS, c.fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, c.v); err != nil {
			t.Fatal(err)
		}
		var want bytes.Buffer
		if err := json.Compact(&want, b); err != nil {
			t.Fatal(err)
		}
		out, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out, want.Bytes()) {
			t.Errorf("%s is written back as %s, want %s", c.fixture, out, want.Bytes())
		}
	}
}

// TestAMachineLogWithinItsBoundCanExceedTheRequestLimit pins why the starter bounds the
// serialized body as well as the log: a log of 786432 line feeds, which the schema
// accepts, marshals to a body over the 1 MiB request limit.
func TestAMachineLogWithinItsBoundCanExceedTheRequestLimit(t *testing.T) {
	now := contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	out, err := json.Marshal(contracts.MachineLog{Version: contracts.Version, Time: now,
		Log: strings.Repeat("\n", 768<<10)})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileProvisioner(t, "machine-log.schema.json"), out); err != nil {
		t.Fatalf("the schema refuses a log of 786432 line feeds: %v", err)
	}
	if len(out) <= 1<<20 {
		t.Errorf("the body is %d bytes, want more than %d", len(out), 1<<20)
	}
}

// TestAClaimAnswerHasExactlyItsMembers pins that a claim answer built in Go marshals
// to version, revision, dispatch_id, pool_id, lease_until, machine when it is set,
// run_code, gateway_url and max_lifetime_minutes, with no member about the work it is
// for, that its schema accepts it, and that it reads back into the same value.
func TestAClaimAnswerHasExactlyItsMembers(t *testing.T) {
	for _, c := range []struct {
		a    contracts.ClaimAnswer
		want string
	}{
		{contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision,
			DispatchID: "dp_8w3k0q9t2v7m4r6x", PoolID: "np_f1xt0re000000000", LeaseUntil: "2026-10-15T12:05:00Z",
			RunCode: runCode, GatewayURL: gatewayURL, MaxLifetimeMinutes: 1470},
			`{"version":1,"revision":1,"dispatch_id":"dp_8w3k0q9t2v7m4r6x","pool_id":"np_f1xt0re000000000",` +
				`"lease_until":"2026-10-15T12:05:00Z","run_code":"` + runCode + `","gateway_url":"` + gatewayURL + `",` +
				`"max_lifetime_minutes":1470}`},
		{contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision,
			DispatchID: "dp_8w3k0q9t2v7m4r6x", PoolID: "np_f1xt0re000000000", LeaseUntil: "2026-10-15T12:05:00Z",
			Machine: "small", RunCode: runCode, GatewayURL: gatewayURL, MaxLifetimeMinutes: 1470},
			`{"version":1,"revision":1,"dispatch_id":"dp_8w3k0q9t2v7m4r6x","pool_id":"np_f1xt0re000000000",` +
				`"lease_until":"2026-10-15T12:05:00Z","machine":"small","run_code":"` + runCode + `","gateway_url":"` + gatewayURL + `",` +
				`"max_lifetime_minutes":1470}`},
	} {
		out, err := json.Marshal(c.a)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.want {
			t.Errorf("the answer is written as %s, want %s", out, c.want)
		}
		if err := validate(compileProvisioner(t, "claim-answer.schema.json"), out); err != nil {
			t.Errorf("the answer marshals to %s, which its schema refuses: %v", out, err)
		}
		var back contracts.ClaimAnswer
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatal(err)
		}
		if back != c.a {
			t.Errorf("%s reads back as %+v, want %+v", out, back, c.a)
		}
	}
}
