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
	"handover.schema.json",
	"handover-answer.schema.json",
	"release.schema.json",
	"refusal.schema.json",
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
// provisioner contract is accepted by <name>.schema.json, and that every schema of the
// contract compiles.
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
		schema := compileProvisioner(t, d.Name()+".schema.json")
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
		"claim-wait-seconds-26.json":               {"claim", "/wait_seconds maximum"},
		"claim-pool-id-node.json":                  {"claim", "/pool_id pattern"},
		"claim-undefined-member.json":              {"claim", "/ additionalProperties"},
		"claim-answer-access-admin.json":           {"claim-answer", "/access enum"},
		"claim-answer-dispatch-id-no-prefix.json":  {"claim-answer", "/dispatch_id pattern"},
		"claim-answer-dispatch-id-upper-case.json": {"claim-answer", "/dispatch_id pattern"},
		"handover-answer-run-key-no-prefix.json":   {"handover-answer", "/run_key pattern"},
		"handover-answer-inputs-no-version.json":   {"handover-answer", "/inputs required version"},
		"handover-answer-without-revision.json":    {"handover-answer", "/ required revision"},
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
		v, ok := compileProvisioner(t, w.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
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
	target := contracts.Target{System: "code.example.com", Path: "acme/shop"}
	item := contracts.WorkItem{ID: "wi_1", ExternalRef: "TRK-1", URL: "https://control.example.com/work-items/wi_1", Title: "Checkout fails"}
	values := []struct {
		schema string
		v      any
	}{
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, WaitSeconds: 25}},
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, PoolID: "np_0123456789abcdef"}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef", RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, Step: "explore", Access: "write", Target: target, WorkItem: item}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now,
			External: &contracts.External{Type: "job", Ref: "run-1", URL: "https://ci.example.com/runs/1"}}},
		{"handover", contracts.Handover{Version: contracts.Version, Time: now}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Access: "read", Target: target, WorkItem: item,
			Inputs: json.RawMessage(`{"version":1}`)}},
		{"release", contracts.Release{Version: contracts.Version, Time: now, Reason: "No runner was free.", Retry: false}},
		{"refusal", contracts.Refusal{Error: "lease_expired"}},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if err := validate(compileProvisioner(t, c.schema+".schema.json"), b); err != nil {
			t.Errorf("%T marshals to %s, which %s.schema.json refuses: %v", c.v, b, c.schema, err)
		}
	}
}

// TestTheHandoverInputsPassThrough pins that a handover answer's inputs pass through a
// Go value unchanged: read from the fixture, the bytes are the fixture's, and marshalled
// and read again, the same bytes, with members a map would reorder and numbers a float
// would round kept as they were.
func TestTheHandoverInputsPassThrough(t *testing.T) {
	b, err := fs.ReadFile(contracts.ProvisionerFS, "fixtures/handover-answer/explore.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Inputs json.RawMessage `json:"inputs"`
	}
	var a contracts.HandoverAnswer
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Inputs, raw.Inputs) {
		t.Errorf("the fixture's inputs read as %s, want %s", a.Inputs, raw.Inputs)
	}

	inputs := `{"version":1,"zeta":"last first","alpha":12345678901234567890,"ratio":1.50,"nested":{"b":[],"a":null}}`
	a.Inputs = json.RawMessage(inputs)
	out, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileProvisioner(t, "handover-answer.schema.json"), out); err != nil {
		t.Errorf("the handover answer marshals to %s, which its schema refuses: %v", out, err)
	}
	var back contracts.HandoverAnswer
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if string(back.Inputs) != inputs {
		t.Errorf("inputs pass through as %s, want %s", back.Inputs, inputs)
	}
}
