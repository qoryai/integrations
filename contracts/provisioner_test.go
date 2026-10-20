package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	"prebuilt.schema.json",
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
		"claim-wait-seconds-26.json":                                      {"claim", "/wait_seconds maximum"},
		"claim-pool-id-node.json":                                         {"claim", "/pool_id pattern"},
		"claim-undefined-member.json":                                     {"claim", "/ additionalProperties"},
		"claim-answer-access-admin.json":                                  {"claim-answer", "/access enum"},
		"claim-answer-dispatch-id-no-prefix.json":                         {"claim-answer", "/dispatch_id pattern"},
		"claim-answer-dispatch-id-upper-case.json":                        {"claim-answer", "/dispatch_id pattern"},
		"claim-answer-work-item-url-http.json":                            {"claim-answer", "/work_item/url pattern"},
		"dispatched-external-type-upper-case.json":                        {"dispatched", "/external/type pattern"},
		"dispatched-external-ref-nul.json":                                {"dispatched", "/external/ref pattern"},
		"release-reason-nul.json":                                         {"release", "/reason pattern"},
		"handover-answer-run-key-no-prefix.json":                          {"handover-answer", "/run_key pattern"},
		"handover-answer-inputs-no-version.json":                          {"handover-answer", "/inputs required version"},
		"handover-answer-without-revision.json":                           {"handover-answer", "/ required revision"},
		"claim-answer-revision-2.json":                                    {"claim-answer", "/revision const"},
		"handover-answer-revision-2.json":                                 {"handover-answer", "/revision const"},
		"claim-answer-explore-without-work-item.json":                     {"claim-answer", "/ required work_item"},
		"claim-answer-prebuild-with-work-item.json":                       {"claim-answer", "/ "},
		"handover-answer-prebuild-inputs-target.json":                     {"handover-answer", "/inputs additionalProperties"},
		"prebuilt-image-tag.json":                                         {"prebuilt", "/image pattern"},
		"prebuilt-image-upper-case.json":                                  {"prebuilt", "/image pattern"},
		"prebuilt-image-digest-63.json":                                   {"prebuilt", "/image pattern"},
		"prebuilt-image-without-name.json":                                {"prebuilt", "/image pattern"},
		"prebuilt-size-0.json":                                            {"prebuilt", "/size minimum"},
		"prebuilt-size-fraction.json":                                     {"prebuilt", "/size type"},
		"prebuilt-undefined-member.json":                                  {"prebuilt", "/ additionalProperties"},
		"prebuilt-without-commit.json":                                    {"prebuilt", "/ required commit"},
		"handover-answer-prebuild-with-work-item.json":                    {"handover-answer", "/ "},
		"handover-answer-explore-without-work-item.json":                  {"handover-answer", "/ required work_item"},
		"handover-answer-prebuild-inputs-version-2.json":                  {"handover-answer", "/inputs/version const"},
		"prebuilt-size-above-maximum.json":                                {"prebuilt", "/size maximum"},
		"handover-answer-prebuild-inputs-without-environment-ref.json":    {"handover-answer", "/inputs required environment_ref"},
		"handover-answer-prebuild-inputs-environment-ref-empty.json":      {"handover-answer", "/inputs/environment_ref minLength"},
		"handover-answer-prebuild-inputs-environment-ref-201.json":        {"handover-answer", "/inputs/environment_ref maxLength"},
		"handover-answer-run-settings-prebuild.json":                      {"handover-answer", "/step const"},
		"handover-answer-run-settings-effort-without-model.json":          {"handover-answer", "/run_settings dependentRequired/effort"},
		"handover-answer-run-settings-tiers-without-model.json":           {"handover-answer", "/run_settings dependentRequired/tiers"},
		"handover-answer-run-settings-model-without-provider-models.json": {"handover-answer", "/run_settings dependentRequired/model"},
		"handover-answer-run-settings-provider-models-without-model.json": {"handover-answer", "/run_settings dependentRequired/provider_models"},
		"handover-answer-run-settings-unknown-tier.json":                  {"handover-answer", "/run_settings/tiers additionalProperties"},
		"handover-answer-run-settings-undefined-member.json":              {"handover-answer", "/run_settings additionalProperties"},
		"handover-answer-run-settings-tier-without-model.json":            {"handover-answer", "/run_settings/tiers/thinking required model"},
		"handover-answer-run-settings-effort-empty.json":                  {"handover-answer", "/run_settings/provider_models/0/effort minItems"},
		"claim-answer-access-without-target.json":                         {"claim-answer", "/ dependentRequired/access"},
		"claim-answer-target-without-access.json":                         {"claim-answer", "/ dependentRequired/target"},
		"claim-answer-prebuild-without-target.json":                       {"claim-answer", "/ required access,target"},
		"handover-answer-access-without-target.json":                      {"handover-answer", "/ dependentRequired/access"},
		"handover-answer-target-without-access.json":                      {"handover-answer", "/ dependentRequired/target"},
		"handover-answer-prebuild-without-target.json":                    {"handover-answer", "/ required access,target"},
		"handover-answer-without-domain.json":                             {"handover-answer", "/ required domain"},
		"handover-answer-domain-upper-case.json":                          {"handover-answer", "/domain pattern"},
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

// TestAPrebuildIsRefusedForItsWorkItemAlone pins that the invalid answers of the step
// prebuild that carry work_item, whose refusal names no keyword, are refused for
// work_item and for nothing else: the same document without work_item is accepted.
func TestAPrebuildIsRefusedForItsWorkItemAlone(t *testing.T) {
	for _, c := range []struct{ fixture, schema string }{
		{"claim-answer-prebuild-with-work-item.json", "claim-answer"},
		{"handover-answer-prebuild-with-work-item.json", "handover-answer"},
	} {
		doc, err := contracts.ProvisionerDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "prebuild" || answer["work_item"] == nil {
			t.Errorf("%s is not an answer of the step prebuild with work_item", c.fixture)
			continue
		}
		delete(answer, "work_item")
		if err := compileProvisioner(t, c.schema+".schema.json").Validate(answer); err != nil {
			t.Errorf("%s without work_item is refused: %v", c.fixture, err)
		}
	}
}

// TestAPrebuildsInputsAreRefusedForTheirEnvironmentRefAlone pins that the invalid
// handover answers of the step prebuild whose inputs miss environment_ref, or hold one
// outside its bounds, are refused for environment_ref and for nothing else: the same
// document with the inputs of handover-answer/prebuild.json is accepted.
func TestAPrebuildsInputsAreRefusedForTheirEnvironmentRefAlone(t *testing.T) {
	schema := compileProvisioner(t, "handover-answer.schema.json")
	valid, err := contracts.ProvisionerDocument("fixtures/handover-answer/prebuild.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("handover-answer/prebuild.json is refused: %v", err)
	}
	inputs := valid.(map[string]any)["inputs"]
	for _, name := range []string{
		"handover-answer-prebuild-inputs-without-environment-ref.json",
		"handover-answer-prebuild-inputs-environment-ref-empty.json",
		"handover-answer-prebuild-inputs-environment-ref-201.json",
	} {
		doc, err := contracts.ProvisionerDocument(path.Join("fixtures/invalid", name))
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "prebuild" {
			t.Errorf("%s is not an answer of the step prebuild", name)
			continue
		}
		if schema.Validate(answer) == nil {
			t.Errorf("%s is accepted", name)
		}
		answer["inputs"] = inputs
		if err := schema.Validate(answer); err != nil {
			t.Errorf("%s with the inputs of handover-answer/prebuild.json is refused: %v", name, err)
		}
	}
}

// TestRunSettingsAreRefusedForRunSettingsAlone pins that each invalid handover answer
// whose name starts with handover-answer-run-settings- is refused for run_settings and
// for nothing else: the same document without run_settings is accepted.
func TestRunSettingsAreRefusedForRunSettingsAlone(t *testing.T) {
	schema := compileProvisioner(t, "handover-answer.schema.json")
	n := 0
	for _, f := range provisionerFiles(t, "fixtures/invalid") {
		if !strings.HasPrefix(path.Base(f), "handover-answer-run-settings-") {
			continue
		}
		n++
		doc, err := contracts.ProvisionerDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["run_settings"] == nil {
			t.Errorf("%s is not a handover answer with run_settings", f)
			continue
		}
		delete(answer, "run_settings")
		if err := schema.Validate(answer); err != nil {
			t.Errorf("%s without run_settings is refused: %v", f, err)
		}
	}
	if n == 0 {
		t.Error("fixtures/invalid contains no handover answer with run_settings")
	}
}

// TestProviderModelsHaveAtMost256Entries pins the bound of run_settings.provider_models
// at its edge: 256 entries are accepted, and 257 are refused for their count alone.
func TestProviderModelsHaveAtMost256Entries(t *testing.T) {
	schema := compileProvisioner(t, "handover-answer.schema.json")
	for _, c := range []struct {
		count  int
		reason string
	}{
		{256, ""},
		{257, "/run_settings/provider_models maxItems"},
	} {
		doc, err := contracts.ProvisionerDocument("fixtures/handover-answer/explore-run-settings.json")
		if err != nil {
			t.Fatal(err)
		}
		settings := doc.(map[string]any)["run_settings"].(map[string]any)
		models := []any{map[string]any{"id": settings["model"]}}
		for i := 1; i < c.count; i++ {
			models = append(models, map[string]any{"id": fmt.Sprintf("example-%d", i)})
		}
		settings["provider_models"] = models
		err = schema.Validate(doc)
		if c.reason == "" {
			if err != nil {
				t.Errorf("%d provider models are refused: %v", c.count, err)
			}
			continue
		}
		v, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%d provider models are accepted", c.count)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != c.reason {
			t.Errorf("%d provider models are refused for %q, want %q", c.count, r, c.reason)
		}
	}
}

// TestProvisionerTypesMarshalToTheirSchemas marshals one value of each request and
// answer type of the provisioner contract, with its optional members left out where it
// has them, and validates it against its schema.
func TestProvisionerTypesMarshalToTheirSchemas(t *testing.T) {
	now := contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	target := &contracts.Target{System: "code.example.com", Path: "acme/shop"}
	ads := &contracts.Target{System: "ads.example.com", Path: "accounts/1234"}
	item := contracts.WorkItem{ID: "wi_1", URL: "https://control.example.com/work-items/wi_1", Title: "Checkout fails"}
	values := []struct {
		schema string
		v      any
	}{
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, WaitSeconds: 25}},
		{"claim", contracts.Claim{Version: contracts.Version, Time: now, PoolID: "np_0123456789abcdef"}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef", RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, Step: "explore", Access: "write", Target: target, WorkItem: &item}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef", RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, Step: "prebuild", Access: "read", Target: target}},
		{"claim-answer", contracts.ClaimAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, DispatchID: "dp_0123456789abcdef", RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			PoolID: "np_0123456789abcdef", LeaseUntil: now, Step: "explore", WorkItem: &item}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now}},
		{"dispatched", contracts.Dispatched{Version: contracts.Version, Time: now,
			External: &contracts.External{Type: "example-ci", Ref: "run-1", URL: "https://ci.example.com/runs/1"}}},
		{"handover", contracts.Handover{Version: contracts.Version, Time: now}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Domain: "software", Access: "read", Target: target, WorkItem: &item,
			Inputs: json.RawMessage(`{"version":1}`)}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Domain: "software", Access: "read", Target: target, WorkItem: &item,
			RunSettings: &contracts.RunSettings{Runtime: "claude"}, Inputs: json.RawMessage(`{"version":1}`)}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Domain: "software", Access: "read", Target: target, WorkItem: &item,
			RunSettings: &contracts.RunSettings{Runtime: "claude", Model: "example-large", Effort: "high",
				ProviderModels: []contracts.ProviderModel{{ID: "example-large", Effort: []string{"low", "high"}}, {ID: "example-small"}},
				Tiers: &contracts.Tiers{Light: &contracts.Tier{Model: "example-small"},
					Thinking: &contracts.Tier{Model: "example-large", Effort: "high"}}},
			Inputs: json.RawMessage(`{"version":1}`)}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Domain: "marketing", WorkItem: &item, Inputs: json.RawMessage(`{"version":1}`)}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "explore", Domain: "marketing", Access: "write", Target: ads, WorkItem: &item,
			Inputs: json.RawMessage(`{"version":1}`)}},
		{"handover-answer", contracts.HandoverAnswer{Version: contracts.Version, Revision: contracts.ProvisionerRevision, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			RunKey: "qrk_e30.AAAA", Step: "prebuild", Domain: "software", Access: "read", Target: target,
			Inputs: json.RawMessage(`{"version":1,"environment_ref":"env-3f2a91c4"}`)}},
		{"release", contracts.Release{Version: contracts.Version, Time: now, Reason: "No runner was free.", Retry: false}},
		{"prebuilt", contracts.Prebuilt{Version: contracts.Version, Time: now, RunID: "0192f0c1-7d4e-7a2b-8c3d-4e5f6a7b8c9d",
			Image: "registry.example.com/acme/shop-env@sha256:" + strings.Repeat("0a", 32), Size: 734003200,
			EnvironmentRef: "env-3f2a91c4", Commit: "9c1e4a7b2d5f8e0a3c6b9d2f5a8c1e4b7d0a3f6c"}},
		{"refusal", contracts.Refusal{Error: "lease_expired"}},
		{"refusal", contracts.Refusal{Error: "unavailable"}},
		{"refusal", contracts.Refusal{Error: "not_prebuild"}},
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

// TestARunWithNoTargetReadsAsNoTarget pins that the answers of the step explore with no
// target read into Go with a nil Target and an empty Access, and are written back with
// neither member.
func TestARunWithNoTargetReadsAsNoTarget(t *testing.T) {
	for _, c := range []struct {
		fixture string
		v       any
	}{
		{"fixtures/claim-answer/explore-without-target.json", &contracts.ClaimAnswer{}},
		{"fixtures/handover-answer/explore-without-target.json", &contracts.HandoverAnswer{}},
	} {
		b, err := fs.ReadFile(contracts.ProvisionerFS, c.fixture)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, c.v); err != nil {
			t.Fatal(err)
		}
		switch a := c.v.(type) {
		case *contracts.ClaimAnswer:
			if a.Target != nil || a.Access != "" {
				t.Errorf("%s reads with target %v and access %q", c.fixture, a.Target, a.Access)
			}
		case *contracts.HandoverAnswer:
			if a.Target != nil || a.Access != "" || a.Domain != "marketing" {
				t.Errorf("%s reads with target %v, access %q and domain %q", c.fixture, a.Target, a.Access, a.Domain)
			}
		}
		out, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		var back map[string]any
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatal(err)
		}
		if _, ok := back["target"]; ok {
			t.Errorf("%s is written back with target: %s", c.fixture, out)
		}
		if _, ok := back["access"]; ok {
			t.Errorf("%s is written back with access: %s", c.fixture, out)
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

// TestATierKeepsItsForm pins that a tier read from JSON is written back in the form it
// was read in, a model id or an object, with or without effort, and that a tier built
// in Go is written as a model id without effort and as an object with it.
func TestATierKeepsItsForm(t *testing.T) {
	for _, in := range []string{
		`"example-small"`,
		`{"model":"example-large"}`,
		`{"model":"example-large","effort":"max"}`,
	} {
		var tier contracts.Tier
		if err := json.Unmarshal([]byte(in), &tier); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		out, err := json.Marshal(tier)
		if err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if string(out) != in {
			t.Errorf("%s is written back as %s", in, out)
		}
	}
	for _, c := range []struct {
		tier contracts.Tier
		want string
	}{
		{contracts.Tier{Model: "example-small"}, `"example-small"`},
		{contracts.Tier{Model: "example-large", Effort: "max"}, `{"model":"example-large","effort":"max"}`},
	} {
		out, err := json.Marshal(c.tier)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.want {
			t.Errorf("%+v is written as %s, want %s", c.tier, out, c.want)
		}
	}
}

// TestRunSettingsReadFromTheFixture pins that the run_settings of
// handover-answer/explore-run-settings.json read into [contracts.RunSettings] with every
// member in place, and are written back as the fixture's bytes, compacted.
func TestRunSettingsReadFromTheFixture(t *testing.T) {
	b, err := fs.ReadFile(contracts.ProvisionerFS, "fixtures/handover-answer/explore-run-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var a contracts.HandoverAnswer
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	s := a.RunSettings
	if s == nil {
		t.Fatal("the fixture's run_settings read as nil")
	}
	if s.Runtime != "claude" || s.Model != "example-large" || s.Effort != "high" {
		t.Errorf("runtime, model and effort read as %q, %q and %q", s.Runtime, s.Model, s.Effort)
	}
	if len(s.ProviderModels) != 2 || s.ProviderModels[0].ID != "example-large" ||
		strings.Join(s.ProviderModels[0].Effort, ",") != "low,medium,high,max" ||
		s.ProviderModels[1].ID != "example-small" || s.ProviderModels[1].Effort != nil {
		t.Errorf("provider_models read as %+v", s.ProviderModels)
	}
	if s.Tiers == nil || s.Tiers.Light == nil || s.Tiers.Fast != nil || s.Tiers.Thinking == nil ||
		s.Tiers.Light.Model != "example-small" || s.Tiers.Light.Effort != "" ||
		s.Tiers.Thinking.Model != "example-large" || s.Tiers.Thinking.Effort != "max" {
		t.Errorf("tiers read as %+v", s.Tiers)
	}

	var raw struct {
		RunSettings json.RawMessage `json:"run_settings"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, raw.RunSettings); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, want.Bytes()) {
		t.Errorf("run_settings are written back as %s, want %s", out, want.Bytes())
	}
	whole, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileProvisioner(t, "handover-answer.schema.json"), whole); err != nil {
		t.Errorf("the handover answer marshals to a document its schema refuses: %v", err)
	}
}
