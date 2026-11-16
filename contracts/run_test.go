package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// runSchema is the path of the schema that the fixtures under fixtures/<name> of the
// run contract, and the invalid fixtures whose expected reason names name, validate
// against: documents.schema.json for documents, and for documents/<definition>, that
// definition of documents.schema.json.
func runSchema(name string) string {
	if def, ok := strings.CutPrefix(name, "documents/"); ok {
		return "documents.schema.json#/$defs/" + def
	}
	return name + ".schema.json"
}

// compileRun compiles a schema of the run contract, or fails the test.
func compileRun(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.CompileRun(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// runFiles are the fixtures of the run contract directly under dir.
func runFiles(t *testing.T, dir string) []string {
	t.Helper()
	return treeFiles(t, contracts.RunFS, dir)
}

// TestRunSchemaIsTheContract pins that run/v1 holds exactly documents.schema.json, with
// the $id RunBase/documents.schema.json, and that it compiles.
func TestRunSchemaIsTheContract(t *testing.T) {
	entries, err := fs.ReadDir(contracts.RunFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if !e.IsDir() {
			got = append(got, e.Name())
		}
	}
	if strings.Join(got, ",") != "documents.schema.json" {
		t.Errorf("run/v1 holds %v, want documents.schema.json", got)
	}
	doc, err := contracts.RunDocument("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if id := doc.(map[string]any)["$id"]; id != contracts.RunBase+"/documents.schema.json" {
		t.Errorf("documents.schema.json has the $id %v", id)
	}
	compileRun(t, "documents.schema.json")
}

// TestRunFixturesValidate pins that each document under fixtures/documents/ of the run
// contract is accepted by documents.schema.json, and that fixtures/ holds no other
// folder of valid documents.
func TestRunFixturesValidate(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, f := range runFiles(t, "fixtures/documents") {
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	dirs, err := fs.ReadDir(contracts.RunFS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if d.IsDir() && d.Name() != "invalid" && d.Name() != "documents" {
			t.Errorf("fixtures/%s is the fixture folder of no schema", d.Name())
		}
	}
}

// TestInvalidRunFixturesAreRefused pins that each document under fixtures/invalid of
// the run contract fails its schema for the reason in its name, and for no other.
func TestInvalidRunFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"documents-without-step.json":                               {"documents", "/ required step"},
		"documents-without-domain.json":                             {"documents", "/ required domain"},
		"documents-without-inputs.json":                             {"documents", "/ required inputs"},
		"documents-undefined-member.json":                           {"documents", "/ additionalProperties"},
		"documents-inputs-no-version.json":                          {"documents", "/inputs required version"},
		"documents-prebuild-inputs-target.json":                     {"documents", "/inputs additionalProperties"},
		"documents-prebuild-with-work-item.json":                    {"documents", "/ "},
		"documents-prebuild-with-workflow.json":                     {"documents", "/ dependentRequired/workflow; /workflow ; / required harness; / required target_version"},
		"documents-explore-without-work-item.json":                  {"documents", "/ required work_item"},
		"documents-prebuild-inputs-version-2.json":                  {"documents", "/inputs/version const"},
		"documents-prebuild-inputs-without-environment-ref.json":    {"documents", "/inputs required environment_ref"},
		"documents-prebuild-inputs-environment-ref-empty.json":      {"documents", "/inputs/environment_ref minLength"},
		"documents-prebuild-inputs-environment-ref-201.json":        {"documents", "/inputs/environment_ref maxLength"},
		"documents-run-settings-prebuild.json":                      {"documents", "/step const"},
		"documents-run-settings-effort-without-model.json":          {"documents", "/run_settings dependentRequired/effort"},
		"documents-run-settings-tiers-without-model.json":           {"documents", "/run_settings dependentRequired/tiers"},
		"documents-run-settings-model-without-provider-models.json": {"documents", "/run_settings dependentRequired/model"},
		"documents-run-settings-provider-models-without-model.json": {"documents", "/run_settings dependentRequired/provider_models"},
		"documents-run-settings-unknown-tier.json":                  {"documents", "/run_settings/tiers additionalProperties"},
		"documents-run-settings-undefined-member.json":              {"documents", "/run_settings additionalProperties"},
		"documents-run-settings-tier-without-model.json":            {"documents", "/run_settings/tiers/thinking required model"},
		"documents-run-settings-effort-empty.json":                  {"documents", "/run_settings/provider_models/0/effort minItems"},
		"documents-run-settings-time-limit-minutes-14.json":         {"documents", "/run_settings/time_limit_minutes minimum"},
		"documents-run-settings-time-limit-minutes-1441.json":       {"documents", "/run_settings/time_limit_minutes maximum"},
		"documents-access-without-target.json":                      {"documents", "/ dependentRequired/access"},
		"documents-target-without-access.json":                      {"documents", "/ dependentRequired/target"},
		"documents-prebuild-without-target.json":                    {"documents", "/ required access,target"},
		"documents-domain-upper-case.json":                          {"documents", "/domain pattern"},
		"documents-workflow-without-harness-hidden.json":            {"documents", "/ dependentRequired/workflow"},
		"documents-harness-hidden-without-workflow.json":            {"documents", "/ dependentRequired/harness_hidden"},
		"documents-prebuild-with-harness-hidden.json":               {"documents", "/ dependentRequired/harness_hidden; /harness_hidden "},
		"documents-harness-hidden-string.json":                      {"documents", "/harness_hidden type"},
		"documents-access-admin.json":                               {"documents", "/access enum"},
		"documents-prebuild-without-image-destination.json":         {"documents", "/ required image_destination"},
		"documents-explore-with-image-destination.json":             {"documents", "/image_destination ; /step const"},
		"documents-image-destination-tag.json":                      {"documents", "/image_destination pattern"},
		"documents-image-destination-digest.json":                   {"documents", "/image_destination pattern"},
		"documents-image-destination-path-upper-case.json":          {"documents", "/image_destination pattern"},
		"documents-image-destination-scheme.json":                   {"documents", "/image_destination pattern"},
		"documents-image-destination-256-characters.json":           {"documents", "/image_destination maxLength"},
		"documents-image-destination-host-without-dot.json":         {"documents", "/image_destination pattern"},
		"documents-image-destination-without-path.json":             {"documents", "/image_destination pattern"},
		"documents-prebuild-with-both-environment-members.json":     {"documents", "/ oneOf"},
		"documents-prebuild-without-environment-members.json":       {"documents", "/ required environment_destination; / required environment_image"},
		"documents-explore-with-environment-destination.json":       {"documents", "/environment_destination "},
		"documents-explore-with-environment-image.json":             {"documents", "/environment_image "},
		"documents-environment-image-tag.json":                      {"documents", "/environment_image pattern"},
		"documents-environment-image-without-host.json":             {"documents", "/environment_image pattern"},
		"documents-environment-image-upper-case-hex.json":           {"documents", "/environment_image pattern"},
		"documents-environment-destination-digest.json":             {"documents", "/environment_destination pattern"},
		"documents-environment-destination-docker-hub.json":         {"documents", "/environment_destination "},
		"documents-harness-without-extends-or-modules.json":         {"documents", "/harness/harness required modules"},
		"documents-harness-extends-and-workflows.json":              {"documents", "/harness/harness/workflows "},
		"documents-harness-workflow-access-admin.json":              {"documents", "/harness/harness/workflows/hello/access enum"},
		"documents-harness-workflow-time-limit-zero.json":           {"documents", "/harness/harness/workflows/hello/time_limit "},
		"documents-harness-workflow-name-56-characters.json":        {"documents", "/ pattern"},
		"documents-workflow-56-characters.json":                     {"documents", "/workflow pattern"},
		"documents-harness-path-source.json":                        {"documents", "/harness/harness/modules/0/source required git,ref"},
		"documents-harness-extends-path-stack.json":                 {"documents", "/harness/harness/extends required git,ref; /harness/harness/extends "},
		"documents-harness-module-name-only.json":                   {"documents", "/harness/harness/modules/0 required source"},
		"documents-harness-extends-path-and-stack.json":             {"documents", "/harness/harness/extends "},
		"documents-harness-module-path-and-module.json":             {"documents", "/harness/harness/modules/0/source "},
		"documents-harness-module-link.json":                        {"documents", "/harness/harness/modules/0 additionalProperties"},
		"documents-harness-machine-key-runtime.json":                {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-model.json":                  {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-effort.json":                 {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-tiers.json":                  {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-force.json":                  {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-update.json":                 {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-home.json":                   {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-links.json":                  {"documents", "/harness/harness additionalProperties"},
		"documents-harness-machine-key-launch.json":                 {"documents", "/harness/harness additionalProperties"},
		"documents-harness-member-git.json":                         {"documents", "/harness additionalProperties"},
		"documents-harness-member-env.json":                         {"documents", "/harness additionalProperties"},
		"documents-harness-member-worktree.json":                    {"documents", "/harness additionalProperties"},
		"documents-harness-member-exports.json":                     {"documents", "/harness additionalProperties"},
		"documents-harness-member-target.json":                      {"documents", "/harness additionalProperties"},
		"documents-harness-section-target.json":                     {"documents", "/harness/harness additionalProperties"},
		"documents-harness-bind-flat.json":                          {"documents", "/harness/harness/bind additionalProperties"},
		"documents-harness-bind-unknown-kind.json":                  {"documents", "/harness/harness/bind additionalProperties"},
		"documents-harness-bind-list-empty.json":                    {"documents", "/harness/harness/bind/agents/reviewers type; /harness/harness/bind/agents/reviewers minItems"},
		"documents-harness-bind-list-commands.json":                 {"documents", "/harness/harness/bind/commands/publish type"},
		"documents-harness-bind-list-skills.json":                   {"documents", "/harness/harness/bind/skills/review type"},
		"documents-harness-bind-list-name-slash.json":               {"documents", "/harness/harness/bind/agents/reviewers type; /harness/harness/bind/agents/reviewers/1 pattern"},
		"documents-harness-bind-name-slash.json":                    {"documents", "/harness/harness/bind/agents/writer pattern; /harness/harness/bind/agents/writer type"},
		"documents-harness-bind-role-dot.json":                      {"documents", "/ pattern"},
		"documents-target-version-without-target.json":              {"documents", "/ dependentRequired/target_version"},
		"documents-target-version-space.json":                       {"documents", "/target_version pattern"},
		"documents-prebuild-with-harness.json":                      {"documents", "/harness "},
		"documents-prebuild-with-target-version.json":               {"documents", "/target_version "},
		"documents-workflow-without-harness-or-target-version.json": {"documents", "/ required harness; / required target_version"},
		"documents-work-item-url-http.json":                         {"documents", "/work_item/url pattern"},
		"documents-submit-module-ref-branch.json":                   {"documents", "/submit_module/source/ref pattern"},
		"documents-submit-module-ref-short.json":                    {"documents", "/submit_module/source/ref pattern"},
		"documents-submit-module-ref-tag.json":                      {"documents", "/submit_module/source/ref pattern"},
		"documents-submit-module-ref-upper-case.json":               {"documents", "/submit_module/source/ref pattern"},
		"documents-submit-module-exclude.json":                      {"documents", "/submit_module/exclude "},
		"documents-submit-module-only.json":                         {"documents", "/submit_module/only "},
		"documents-submit-module-variant.json":                      {"documents", "/submit_module/variant "},
		"documents-submit-module-path-and-module.json":              {"documents", "/submit_module/source ; /submit_module/source "},
		"documents-submit-module-without-source.json":               {"documents", "/submit_module required source"},
		"documents-submit-module-access-read.json":                  {"documents", "/access const"},
		"documents-submit-module-without-workflow.json":             {"documents", "/ dependentRequired/submit_module"},
		"documents-submit-module-without-target.json":               {"documents", "/ dependentRequired/submit_module"},
		"documents-prebuild-with-submit-module.json":                {"documents", "/ dependentRequired/submit_module; /submit_module ; /access const"},
	}
	got := runFiles(t, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileRun(t, runSchema(w.schema)).Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// TestAPrebuildIsRefusedForEachRunMemberAlone pins that the invalid run documents of
// the step prebuild that have workflow, harness_hidden, harness or submit_module are
// refused for that member alone: the same documents without it are accepted.
func TestAPrebuildIsRefusedForEachRunMemberAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct{ fixture, member string }{
		{"documents-prebuild-with-workflow.json", "workflow"},
		{"documents-prebuild-with-harness-hidden.json", "harness_hidden"},
		{"documents-prebuild-with-harness.json", "harness"},
		{"documents-prebuild-with-submit-module.json", "submit_module"},
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		documents, ok := doc.(map[string]any)
		if !ok || documents["step"] != "prebuild" || documents[c.member] == nil {
			t.Errorf("%s are not run documents of the step prebuild with %s", c.fixture, c.member)
			continue
		}
		delete(documents, c.member)
		if err := schema.Validate(documents); err != nil {
			t.Errorf("%s without %s is refused: %v", c.fixture, c.member, err)
		}
	}
}

// TestAHarnessIsRefusedForItsHarnessAlone pins that each invalid fixture whose name
// starts with documents-harness-, other than documents-harness-hidden-, is refused for
// its harness and for nothing else: the same documents with the harness of
// documents/explore-harness.json are accepted.
func TestAHarnessIsRefusedForItsHarnessAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	valid, err := contracts.RunDocument("fixtures/documents/explore-harness.json")
	if err != nil {
		t.Fatal(err)
	}
	harness := valid.(map[string]any)["harness"]
	n := 0
	for _, f := range runFiles(t, "fixtures/invalid") {
		name := path.Base(f)
		if !strings.HasPrefix(name, "documents-harness-") || strings.HasPrefix(name, "documents-harness-hidden-") {
			continue
		}
		n++
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		documents, ok := doc.(map[string]any)
		if !ok || documents["harness"] == nil {
			t.Errorf("%s are not run documents with harness", f)
			continue
		}
		if schema.Validate(documents) == nil {
			t.Errorf("%s is accepted", f)
		}
		documents["harness"] = harness
		if err := schema.Validate(documents); err != nil {
			t.Errorf("%s with the harness of documents/explore-harness.json is refused: %v", f, err)
		}
	}
	if n == 0 {
		t.Error("fixtures/invalid contains no run documents with a refused harness")
	}
}

// TestASubmitModuleIsRefusedForItsSubmitModuleAlone pins that each invalid fixture of
// the run contract, and of the adapter contract, whose name starts with
// documents-submit-module- or adapter-submit-module- and whose refusal is at
// /submit_module, is refused for its submit_module and for nothing else: the same
// document with the submit_module of a valid fixture is accepted. The refused values
// are a branch name, a tag, a short hash and an upper-case hash as the ref, exclude,
// only, variant, both path and module, and no source.
func TestASubmitModuleIsRefusedForItsSubmitModuleAlone(t *testing.T) {
	for _, c := range []struct {
		prefix, valid, schema string
		files                 []string
		read                  func(string) (any, error)
		compile               func(string) (*jsonschema.Schema, error)
	}{
		{"documents-submit-module-", "fixtures/documents/explore-submit-module-without-harness.json", "documents.schema.json",
			runFiles(t, "fixtures/invalid"), contracts.RunDocument, contracts.CompileRun},
		{"adapter-submit-module-", "fixtures/adapter/adapter/code-host-submit-module.json", "adapter.schema.json",
			files(t, "fixtures/adapter/invalid"), contracts.Document, contracts.Compile},
	} {
		schema, err := c.compile(c.schema)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := c.read(c.valid)
		if err != nil {
			t.Fatal(err)
		}
		module := valid.(map[string]any)["submit_module"]
		n := 0
		for _, f := range c.files {
			if !strings.HasPrefix(path.Base(f), c.prefix) {
				continue
			}
			doc, err := c.read(f)
			if err != nil {
				t.Fatal(err)
			}
			v, ok := schema.Validate(doc).(*jsonschema.ValidationError)
			if !ok {
				t.Errorf("%s is accepted", f)
				continue
			}
			if !strings.HasPrefix(strings.Join(leaves(v), "; "), "/submit_module") {
				continue
			}
			n++
			doc.(map[string]any)["submit_module"] = module
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s with the submit_module of %s is refused: %v", f, c.valid, err)
			}
		}
		if n == 0 {
			t.Errorf("no invalid fixture starts with %s and is refused at /submit_module", c.prefix)
		}
	}
}

// TestTheTwoSubmitModulesAreOneDefinition pins that submit_module of adapter.schema.json
// refers to $defs/submit_module of the run documents, so the adapter's start-up and the
// run documents accept and refuse the same modules, and that the general
// $defs/harness_source of the run documents takes any ref, a branch name included.
func TestTheTwoSubmitModulesAreOneDefinition(t *testing.T) {
	doc, err := contracts.Document("adapter.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	property := doc.(map[string]any)["properties"].(map[string]any)["submit_module"].(map[string]any)
	if want := contracts.RunBase + "/documents.schema.json#/$defs/submit_module"; property["$ref"] != want {
		t.Errorf("submit_module of adapter.schema.json refers to %v, want %s", property["$ref"], want)
	}
	source := compileRun(t, runSchema("documents/harness_source"))
	pinned := compileRun(t, runSchema("documents/harness_pinned_source"))
	for ref, isPinned := range map[string]bool{
		"main":    false,
		"v1.2.0":  false,
		"3f2a91c": false,
		"3F2A91C4D5E6F708192A3B4C5D6E7F8091A2B3C4": false,
		"3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4": true,
		strings.Repeat("9c", 32):                   true,
		strings.Repeat("9c", 21):                   false,
	} {
		v := map[string]any{"git": "https://code.example.com/example-org/forge-module", "ref": ref}
		if err := source.Validate(v); err != nil {
			t.Errorf("harness_source refuses the ref %q: %v", ref, err)
		}
		if err := pinned.Validate(v); (err == nil) != isPinned {
			t.Errorf("harness_pinned_source with the ref %q: %v, want accepted %v", ref, err, isPinned)
		}
	}
}

// TestATargetVersionHasItsGrammar pins the target_version of documents.schema.json: 1
// to 255 letters, digits and the characters ._/-, and nothing else.
func TestATargetVersionHasItsGrammar(t *testing.T) {
	schema := compileRun(t, runSchema("documents/target_version"))
	for _, v := range []string{
		"0123456789abcdef0123456789abcdef01234567",
		strings.Repeat("0a", 32),
		"v1.2.0",
		"release/2026-10_a",
		"7",
		strings.Repeat("r", 255),
	} {
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s is refused: %v", v, err)
		}
	}
	for _, v := range []string{"", strings.Repeat("r", 256), "release 1", "a:b", "a@b", "a~1", "a#b"} {
		if err := schema.Validate(v); err == nil {
			t.Errorf("%q is accepted", v)
		}
	}
}

// TestAnImageDestinationIsAnOCIName pins the image_destination of
// documents.schema.json: a lower-case host of DNS labels, an optional port of 1 to 5
// digits, and one or more path components of lower-case letters and digits joined by
// a dot, one or two underscores, or hyphens, with no tag and no digest; that the host
// contains a dot, has a port, or is localhost; that 255 characters are accepted and
// 256 refused; that the documents of the step explore with image_destination are
// refused for that member alone; and that the documents of a step other than explore
// and prebuild are refused for it by step.
func TestAnImageDestinationIsAnOCIName(t *testing.T) {
	schema := compileRun(t, runSchema("documents/image_destination"))
	for _, v := range []string{
		"registry.example.com/targets/0123456789abcdef",
		"registry.example.com:5000/env",
		"localhost/env",
		"registry.example.com/a.b/c_d/e__f/g-h/i---j",
		"registry:5000/env",
		"localhost:5000/env",
		"registry.example.com/" + strings.Repeat("a", 234),
	} {
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s is refused: %v", v, err)
		}
	}
	for _, v := range []string{
		"registry.example.com",
		"registry.example.com/",
		"registry.example.com/env/",
		"registry.example.com/a___b",
		"registry.example.com/.env",
		"registry.example.com/env-",
		"registry.example.com:123456/env",
		"registry.example.com:/env",
		"-registry.example.com/env",
		"registry..example.com/env",
		"registry.example.com/env:latest",
		"registry.example.com/env@sha256:" + strings.Repeat("0", 64),
		"https://registry.example.com/env",
		"registry/env",
		"registry",
		"Localhost/env",
		"Registry.Example.com/env",
		"Registry.example.com/env",
		"Registry:5000/env",
		"LOCALHOST/env",
		"localhost.:5000/env",
		"registry.example.com/" + strings.Repeat("a", 235),
	} {
		if err := schema.Validate(v); err == nil {
			t.Errorf("%s is accepted", v)
		}
	}

	doc, err := contracts.RunDocument("fixtures/invalid/documents-explore-with-image-destination.json")
	if err != nil {
		t.Fatal(err)
	}
	documents, ok := doc.(map[string]any)
	if !ok || documents["step"] != "explore" || documents["image_destination"] == nil {
		t.Fatal("documents-explore-with-image-destination.json are not run documents of the step explore with image_destination")
	}
	delete(documents, "image_destination")
	if err := compileRun(t, runSchema("documents")).Validate(documents); err != nil {
		t.Errorf("documents-explore-with-image-destination.json without image_destination is refused: %v", err)
	}

	other := `{"step": "review", "domain": "software", "inputs": {"version": 1},
 "image_destination": "registry.example.com/env"}`
	v, ok := compileRun(t, runSchema("documents")).Validate(mustDocument(t, other)).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("the documents of the step review with image_destination are accepted")
	}
	if r := strings.Join(leaves(v), "; "); r != "/step const" {
		t.Errorf("the documents of the step review with image_destination are refused for %q, want %q", r, "/step const")
	}
}

// TestAPrebuildHasOneEnvironmentMember pins environment_destination and
// environment_image of documents.schema.json: the documents of the step explore with
// either are refused for that member alone; and the documents of the step prebuild
// with both are accepted with either one left out.
func TestAPrebuildHasOneEnvironmentMember(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct{ fixture, member string }{
		{"documents-explore-with-environment-destination.json", "environment_destination"},
		{"documents-explore-with-environment-image.json", "environment_image"},
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		documents, ok := doc.(map[string]any)
		if !ok || documents["step"] != "explore" || documents[c.member] == nil {
			t.Errorf("%s are not run documents of the step explore with %s", c.fixture, c.member)
			continue
		}
		delete(documents, c.member)
		if err := schema.Validate(documents); err != nil {
			t.Errorf("%s without %s is refused: %v", c.fixture, c.member, err)
		}
	}
	for _, member := range []string{"environment_destination", "environment_image"} {
		doc, err := contracts.RunDocument("fixtures/invalid/documents-prebuild-with-both-environment-members.json")
		if err != nil {
			t.Fatal(err)
		}
		documents := doc.(map[string]any)
		delete(documents, member)
		if err := schema.Validate(documents); err != nil {
			t.Errorf("the prebuild with both environment members, without %s, is refused: %v", member, err)
		}
	}
}

// TestAnEnvironmentImageHasAHost pins the environment_image of documents.schema.json:
// a place at the registry as image_place defines it, then @sha256: and 64 lower-case
// hexadecimal digits, at most 327 characters; a name with no host, an upper-case host,
// a Docker Hub host, upper-case hexadecimal digits and a tag are refused.
func TestAnEnvironmentImageHasAHost(t *testing.T) {
	schema := compileRun(t, runSchema("documents/environment_image"))
	digest := "@sha256:" + strings.Repeat("0", 64)
	longest := "registry.example.com/" + strings.Repeat("e", 255-len("registry.example.com/")) + digest
	if len(longest) != 327 {
		t.Fatalf("the longest image has %d characters, want 327", len(longest))
	}
	for _, v := range []string{
		"registry.example.com/environments/0123456789abcdef" + digest,
		"localhost/env" + digest,
		longest,
	} {
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s is refused: %v", v, err)
		}
	}
	for _, v := range []string{
		"env" + digest,
		"team/env" + digest,
		"Registry.example.com/env" + digest,
		"Registry:5000/env" + digest,
		"LOCALHOST/env" + digest,
		"docker.io/env" + digest,
		"registry.example.com/env@sha256:" + strings.Repeat("0A", 32),
		"registry.example.com/env:latest",
		"registry.example.com/e" + longest[len("registry.example.com/"):],
	} {
		if err := schema.Validate(v); err == nil {
			t.Errorf("%s is accepted", v)
		}
	}
}

// TestADockerHubHostIsRefused pins the rules of image_place and image_by_digest of
// documents.schema.json that refuse a Docker Hub host: docker.io, index.docker.io,
// registry-1.docker.io, registry.hub.docker.com and registry.docker.com, in any case,
// with or without a port. Each refused value is refused by one rule of allOf, and a
// value with an upper-case host by the pattern too. docker.example.com/env,
// notdocker.io/env and hub.docker.example.com/env are accepted, each with the digest
// for image_by_digest.
func TestADockerHubHostIsRefused(t *testing.T) {
	digest := "@sha256:" + strings.Repeat("0", 64)
	for _, def := range []struct{ name, suffix string }{
		{"image_place", ""},
		{"image_by_digest", digest},
	} {
		schema := compileRun(t, runSchema("documents/"+def.name))
		for _, c := range []struct {
			refused string
			rule    int
			// upper is true when the host has upper case, which the pattern
			// refuses too.
			upper bool
		}{
			{"docker.io/env", 0, false},
			{"Docker.IO/env", 0, true},
			{"index.docker.io/env", 1, false},
			{"registry-1.docker.io:443/env", 2, false},
			{"registry.hub.docker.com/env", 3, false},
			{"registry.docker.com:443/env", 4, false},
		} {
			v, ok := schema.Validate(c.refused + def.suffix).(*jsonschema.ValidationError)
			if !ok {
				t.Errorf("%s: %s is accepted", def.name, c.refused+def.suffix)
				continue
			}
			var rules []string
			for _, l := range leafErrors(v) {
				rules = append(rules, l.SchemaURL)
			}
			want := fmt.Sprintf("%s/documents.schema.json#/$defs/%s/allOf/%d", contracts.RunBase, def.name, c.rule)
			if c.upper {
				want = fmt.Sprintf("%s/documents.schema.json#/$defs/%s; %s", contracts.RunBase, def.name, want)
			}
			if strings.Join(rules, "; ") != want {
				t.Errorf("%s: %s is refused by %q, want %q", def.name, c.refused+def.suffix, rules, want)
			}
		}
		for _, v := range []string{"docker.example.com/env", "notdocker.io/env", "hub.docker.example.com/env"} {
			if err := schema.Validate(v + def.suffix); err != nil {
				t.Errorf("%s: %s is refused: %v", def.name, v+def.suffix, err)
			}
		}
	}
}

// mustDocument reads a JSON document, or fails the test.
func mustDocument(t *testing.T, s string) any {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestRunTypesMarshalToTheirSchemas marshals one value of each run document type, with
// its optional members left out where it has them, and validates it against its
// definition, and one value of [contracts.Documents] for each step against
// documents.schema.json.
func TestRunTypesMarshalToTheirSchemas(t *testing.T) {
	target := &contracts.Target{System: "code.example.com", Path: "acme/shop"}
	ads := &contracts.Target{System: "ads.example.com", Path: "accounts/1234"}
	item := contracts.WorkItem{ID: "wi_1", URL: "https://control.example.com/work-items/wi_1", Title: "Checkout fails"}
	hidden, shown := true, false
	const source = "https://code.example.com/team/harness"
	harness := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Extends: &contracts.HarnessBase{Git: source, Ref: "v2.4.0", Stack: "marketing"},
	}}
	bare := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Extends: &contracts.HarnessBase{Git: source, Ref: "v2.4.0"},
		Modules: []contracts.HarnessModule{{Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0"}}},
	}}
	full := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Name: "campaigns", Description: "The harness of the campaign work.",
		Extends: &contracts.HarnessBase{Git: source, Ref: "v2.4.0", Path: "stacks/marketing"},
		Modules: []contracts.HarnessModule{
			{Name: "budgets", Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0", Module: "budgets"},
				Exclude: &contracts.HarnessSelection{Skills: []string{"spend"}, Instructions: true,
					Settings: &contracts.HarnessPart{All: true}}},
			{Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0", Path: "modules/reports"},
				Only:    &contracts.HarnessSelection{OutputStyles: []string{"brief"}, Env: &contracts.HarnessPart{Names: []string{"REPORT_DIR"}}},
				Variant: "weekly"},
		},
		Bind: map[string]map[string]contracts.BindValue{
			"agents": {"writer": {Name: "copywriter"}, "reviewers": {Names: []string{"copy-reviewer", "legal-reviewer"}}},
			"skills": {"review": {Name: "brand-review"}},
		},
		Extensions: map[string]json.RawMessage{"team": json.RawMessage(`{"size":3}`)},
	}}
	defined := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Modules: []contracts.HarnessModule{{Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0", Module: "budgets"}}},
		Workflows: map[string]contracts.HarnessWorkflow{
			"hello":  {Access: "write", TimeLimit: "45m"},
			"review": {},
		},
	}}
	definedBare := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Modules: []contracts.HarnessModule{{Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0"}}},
	}}
	const forge = "https://code.example.com/example-org/forge-module"
	submit := contracts.SubmitModule{Name: "forge-submit",
		Source: contracts.HarnessSource{Git: forge, Ref: "3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4", Module: "forge-submit"}}
	values := []struct {
		schema string
		v      any
	}{
		{"documents/target", target},
		{"documents/target", ads},
		{"documents/work_item", item},
		{"documents/run_settings", contracts.RunSettings{Runtime: "claude"}},
		{"documents/run_settings", contracts.RunSettings{Runtime: "claude", TimeLimitMinutes: 1440}},
		{"documents/run_settings", contracts.RunSettings{Runtime: "claude", Model: "example-large", Effort: "high",
			ProviderModels: []contracts.ProviderModel{{ID: "example-large", Effort: []string{"low", "high"}}, {ID: "example-small"}},
			Tiers: &contracts.Tiers{Light: &contracts.Tier{Model: "example-small"},
				Thinking: &contracts.Tier{Model: "example-large", Effort: "high"}}}},
		{"documents", contracts.Documents{Step: "prebuild", Domain: "software",
			Inputs: json.RawMessage(`{"version":1,"environment_ref":"env-3f2a91c4"}`), Access: "read", Target: target,
			ImageDestination:       "registry.example.com/targets/0123456789abcdef",
			EnvironmentDestination: "registry.example.com/environments/0123456789abcdef"}},
		{"documents", contracts.Documents{Step: "prebuild", Domain: "software",
			Inputs: json.RawMessage(`{"version":1,"environment_ref":"env-3f2a91c4"}`), Access: "read", Target: target,
			ImageDestination: "registry.example.com/targets/0123456789abcdef",
			EnvironmentImage: "registry.example.com/environments/0123456789abcdef@sha256:" + strings.Repeat("3f", 32)}},
		{"documents", contracts.Documents{Step: "explore", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), WorkItem: &item}},
		{"documents/harness", harness},
		{"documents/harness", full},
		{"documents/harness", bare},
		{"documents/harness", defined},
		{"documents/harness", definedBare},
		{"documents", contracts.Documents{Step: "explore", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "read", Target: target,
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", WorkItem: &item, Workflow: "hello",
			HarnessHidden: &hidden}},
		{"documents", contracts.Documents{Step: "explore", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), Access: "read", Target: ads, WorkItem: &item,
			Workflow: "hello", HarnessHidden: &shown, Harness: harness}},
		{"documents", contracts.Documents{Step: "explore", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), WorkItem: &item, Harness: full}},
		{"documents/submit_module", submit},
		{"documents/submit_module", contracts.SubmitModule{Source: contracts.HarnessSource{Git: forge,
			Ref: strings.Repeat("9c", 32), Path: "modules/forge-submit"}}},
		{"documents", contracts.Documents{Step: "explore", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target,
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", WorkItem: &item, Workflow: "build",
			HarnessHidden: &shown, SubmitModule: &submit}},
		{"documents", contracts.Documents{Step: "explore", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target, WorkItem: &item,
			Workflow: "build", HarnessHidden: &shown, Harness: harness, SubmitModule: &submit}},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if err := validate(compileRun(t, runSchema(c.schema)), b); err != nil {
			t.Errorf("%T marshals to %s, which %s refuses: %v", c.v, b, runSchema(c.schema), err)
		}
	}
}

// TestTheDocumentsReadIntoDocuments pins that each valid run documents fixture reads
// into [contracts.Documents] and is written back as the same JSON value.
func TestTheDocumentsReadIntoDocuments(t *testing.T) {
	for _, f := range runFiles(t, "fixtures/documents") {
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var want, back any
		if err := json.Unmarshal(b, &want); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(out, &back); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, want) {
			t.Errorf("%s is written back as %s", f, out)
		}
	}
}

// TestTheInputsPassThrough pins that the run's inputs pass through a
// [contracts.Documents] unchanged: read from the fixture, the bytes are the fixture's,
// and marshalled and read again, the same bytes, with members a map would reorder and
// numbers a float would round kept as they were.
func TestTheInputsPassThrough(t *testing.T) {
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore.json")
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		Inputs json.RawMessage `json:"inputs"`
	}
	var d contracts.Documents
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d.Inputs, raw.Inputs) {
		t.Errorf("the fixture's inputs read as %s, want %s", d.Inputs, raw.Inputs)
	}

	inputs := `{"version":1,"zeta":"last first","alpha":12345678901234567890,"ratio":1.50,"nested":{"b":[],"a":null}}`
	d.Inputs = json.RawMessage(inputs)
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileRun(t, runSchema("documents")), out); err != nil {
		t.Errorf("the documents marshal to %s, which documents.schema.json refuses: %v", out, err)
	}
	var back contracts.Documents
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if string(back.Inputs) != inputs {
		t.Errorf("inputs pass through as %s, want %s", back.Inputs, inputs)
	}
}

// TestAPrebuildIsRefusedForItsWorkItemAlone pins that the invalid run documents of the
// step prebuild that have work_item, whose refusal names no keyword, are refused for
// work_item and for nothing else: the same documents without work_item are accepted.
func TestAPrebuildIsRefusedForItsWorkItemAlone(t *testing.T) {
	for _, c := range []struct{ fixture, schema string }{
		{"documents-prebuild-with-work-item.json", "documents"},
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "prebuild" || answer["work_item"] == nil {
			t.Errorf("%s are not run documents of the step prebuild with work_item", c.fixture)
			continue
		}
		delete(answer, "work_item")
		if err := compileRun(t, runSchema(c.schema)).Validate(answer); err != nil {
			t.Errorf("%s without work_item is refused: %v", c.fixture, err)
		}
	}
}

// TestAPrebuildsInputsAreRefusedForTheirEnvironmentRefAlone pins that the invalid run
// documents of the step prebuild whose inputs miss environment_ref, or have one outside
// its bounds, are refused for environment_ref and for nothing else: the same documents
// with the inputs of documents/prebuild.json are accepted.
func TestAPrebuildsInputsAreRefusedForTheirEnvironmentRefAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	valid, err := contracts.RunDocument("fixtures/documents/prebuild.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("documents/prebuild.json is refused: %v", err)
	}
	inputs := valid.(map[string]any)["inputs"]
	for _, name := range []string{
		"documents-prebuild-inputs-without-environment-ref.json",
		"documents-prebuild-inputs-environment-ref-empty.json",
		"documents-prebuild-inputs-environment-ref-201.json",
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", name))
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "prebuild" {
			t.Errorf("%s are not run documents of the step prebuild", name)
			continue
		}
		if schema.Validate(answer) == nil {
			t.Errorf("%s is accepted", name)
		}
		answer["inputs"] = inputs
		if err := schema.Validate(answer); err != nil {
			t.Errorf("%s with the inputs of documents/prebuild.json is refused: %v", name, err)
		}
	}
}

// TestRunSettingsAreRefusedForRunSettingsAlone pins that each invalid fixture whose
// name starts with documents-run-settings- is refused for run_settings and for
// nothing else: the same documents without run_settings are accepted.
func TestRunSettingsAreRefusedForRunSettingsAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	n := 0
	for _, f := range runFiles(t, "fixtures/invalid") {
		if !strings.HasPrefix(path.Base(f), "documents-run-settings-") {
			continue
		}
		n++
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["run_settings"] == nil {
			t.Errorf("%s are not run documents with run_settings", f)
			continue
		}
		delete(answer, "run_settings")
		if err := schema.Validate(answer); err != nil {
			t.Errorf("%s without run_settings is refused: %v", f, err)
		}
	}
	if n == 0 {
		t.Error("fixtures/invalid contains no run documents with run_settings")
	}
}

// TestProviderModelsHaveAtMost256Entries pins the bound of run_settings.provider_models
// at its edge: 256 entries are accepted, and 257 are refused for their count alone.
func TestProviderModelsHaveAtMost256Entries(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct {
		count  int
		reason string
	}{
		{256, ""},
		{257, "/run_settings/provider_models maxItems"},
	} {
		doc, err := contracts.RunDocument("fixtures/documents/explore-run-settings.json")
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

// TestTheRunDocumentsReadIntoTheirGoTypes pins that the target, work_item,
// run_settings, harness and submit_module of each run documents fixture read into their
// Go types and are written back as the fixture's bytes, compacted, that a fixture without
// target has no access and no target_version either, that a fixture with workflow has
// harness or target_version, that at least one fixture has a harness with bind, and that
// at least one fixture has submit_module with harness and one without.
func TestTheRunDocumentsReadIntoTheirGoTypes(t *testing.T) {
	n, harnesses, binds := 0, 0, 0
	submits := map[bool]int{}
	for _, f := range runFiles(t, "fixtures/documents") {
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var members map[string]json.RawMessage
		if err := json.Unmarshal(b, &members); err != nil {
			t.Fatal(err)
		}
		if _, ok := members["target"]; !ok {
			for _, m := range []string{"access", "target_version"} {
				if _, ok := members[m]; ok {
					t.Errorf("%s has %s and no target", f, m)
				}
			}
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if d.Workflow != "" && d.Harness == nil && d.TargetVersion == "" {
			t.Errorf("%s has workflow, and neither harness nor target_version", f)
		}
		if d.Harness != nil {
			harnesses++
			if len(d.Harness.Harness.Bind) > 0 {
				binds++
			}
		}
		if d.SubmitModule != nil {
			submits[d.Harness != nil]++
		}
		for name, v := range map[string]any{
			"target":        &contracts.Target{},
			"work_item":     &contracts.WorkItem{},
			"run_settings":  &contracts.RunSettings{},
			"harness":       &contracts.HarnessDocument{},
			"submit_module": &contracts.SubmitModule{},
		} {
			raw, ok := members[name]
			if !ok {
				continue
			}
			n++
			if err := json.Unmarshal(raw, v); err != nil {
				t.Fatalf("%s: %s: %v", f, name, err)
			}
			var want bytes.Buffer
			if err := json.Compact(&want, raw); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, want.Bytes()) {
				t.Errorf("%s: %s is written back as %s, want %s", f, name, out, want.Bytes())
			}
		}
	}
	if n == 0 {
		t.Error("fixtures/documents has no target, work_item, run_settings or harness")
	}
	if harnesses == 0 {
		t.Error("fixtures/documents has no harness")
	}
	if binds == 0 {
		t.Error("fixtures/documents has no harness with bind")
	}
	if submits[true] == 0 || submits[false] == 0 {
		t.Errorf("fixtures/documents has %d documents with submit_module and harness, and %d with submit_module and no harness; want 1 or more of each",
			submits[true], submits[false])
	}
}

// TestABindTheSchemaRefusesFailsToRead pins that a bind in the flat form, <kind>/<role>
// to a name, a bind with a kind other than agents, commands and skills, a bind with a
// kind whose value is null, a role whose value is null, a list on a commands or skills
// role, an empty list, a null item of a list, and a role set to neither a name nor a
// list fail to read into a HarnessSection with an
// error, and that a bind with each kind, with a list on an agents role, or with empty
// objects, reads.
func TestABindTheSchemaRefusesFailsToRead(t *testing.T) {
	for _, c := range []struct{ in, err string }{
		{`{"bind":{"agents/writer":"copywriter"}}`, "bind"},
		{`{"bind":{"hooks":{"x":"y"}}}`, `use agents, commands or skills as a bind kind, not "hooks"`},
		{`{"bind":{"agents":null}}`, `set the bind kind "agents" to an object of roles to names, not null`},
		{`{"bind":{"skills":{"review":"brand-review"},"agents":null}}`, `set the bind kind "agents" to an object of roles to names, not null`},
		{`{"bind":{"commands":{"publish":["post","schedule"]}}}`, `set the bind role "commands.publish" to one name, not a list: only an agents role can be a group`},
		{`{"bind":{"skills":{"review":["brand-review"]}}}`, `set the bind role "skills.review" to one name, not a list: only an agents role can be a group`},
		{`{"bind":{"agents":{"reviewers":[]}}}`, `write one name or more in the list of the bind role "agents.reviewers": the list is empty`},
		{`{"bind":{"agents":{"writer":null}}}`, `set the bind role "agents.writer" to one name or a list of names, not null`},
		{`{"bind":{"hooks":{"x":null}}}`, `use agents, commands or skills as a bind kind, not "hooks"`},
		{`{"bind":{"agents":{"reviewers":["copy-reviewer",null]}}}`, `set item 2 of the list of the bind role "agents.reviewers" to a name, not null`},
		{`{"bind":{"agents":{"reviewers":{"one":"copy-reviewer"}}}}`, "set a bind role to one name or a list of names"},
		{`{"bind":{"agents":{"reviewers":["copy-reviewer",3]}}}`, "set a bind role to one name or a list of names"},
	} {
		var s contracts.HarnessSection
		err := json.Unmarshal([]byte(c.in), &s)
		if err == nil {
			t.Errorf("%s reads as %v, want an error", c.in, s.Bind)
			continue
		}
		if !strings.Contains(err.Error(), c.err) {
			t.Errorf("%s fails with %q, want an error with %q", c.in, err, c.err)
		}
	}
	for _, in := range []string{
		`{"bind":{"agents":{"writer":"copywriter"},"commands":{"publish":"post"},"skills":{"review":"brand-review"}}}`,
		`{"bind":{"agents":{"reviewers":["copy-reviewer","legal-reviewer"],"writer":"copywriter"}}}`,
		`{"bind":{"agents":{}}}`,
		`{"bind":{}}`,
		`{}`,
	} {
		var s contracts.HarnessSection
		if err := json.Unmarshal([]byte(in), &s); err != nil {
			t.Errorf("%s: %v", in, err)
		}
	}
}

// TestABindErrorIsForTheFirstBadValue pins that decoding a HarnessSection returns the
// error for the first bad value of its bind, in the order of roles and then of list
// items: a value of the wrong type before a later null, and a list item of the wrong
// type before a later null item, as BindValue alone returns it.
func TestABindErrorIsForTheFirstBadValue(t *testing.T) {
	for _, c := range []struct{ in, err string }{
		{`{"bind":{"agents":{"reviewers":["copy-reviewer",3,null]}}}`, "set a bind role to one name or a list of names: json: cannot unmarshal number into .1 of type string"},
		{`{"bind":{"agents":{"a":{"o":1},"b":null}}}`, "set a bind role to one name or a list of names: json: cannot unmarshal object into Go value of type string"},
	} {
		var s contracts.HarnessSection
		if err := json.Unmarshal([]byte(c.in), &s); err == nil || err.Error() != c.err {
			t.Errorf("%s fails with %v, want %q", c.in, err, c.err)
		}
	}
	var v contracts.BindValue
	const alone = "set a bind role to one name or a list of names: json: cannot unmarshal number into .1 of type string"
	if err := json.Unmarshal([]byte(`["copy-reviewer",3,null]`), &v); err == nil || err.Error() != alone {
		t.Errorf("a BindValue alone fails with %v, want %q", err, alone)
	}
}

// TestABindValueKeepsItsForm pins that a bind role read from JSON is written back as
// the same bytes: a name as a name, and a list as a list, a list of one name and a list
// with a name twice included, that null and a null item of a list fail to read, and
// that a BindValue built in Go is written as a name when Names is nil, and as a list
// otherwise.
func TestABindValueKeepsItsForm(t *testing.T) {
	for _, c := range []struct {
		in   string
		want contracts.BindValue
	}{
		{`"copywriter"`, contracts.BindValue{Name: "copywriter"}},
		{`["copy-reviewer","legal-reviewer"]`, contracts.BindValue{Names: []string{"copy-reviewer", "legal-reviewer"}}},
		{`["copy-reviewer"]`, contracts.BindValue{Names: []string{"copy-reviewer"}}},
		{`["copy-reviewer","copy-reviewer"]`, contracts.BindValue{Names: []string{"copy-reviewer", "copy-reviewer"}}},
		{`[]`, contracts.BindValue{Names: []string{}}},
	} {
		var v contracts.BindValue
		if err := json.Unmarshal([]byte(c.in), &v); err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(v, c.want) {
			t.Errorf("%s reads as %+v, want %+v", c.in, v, c.want)
		}
		out, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.in {
			t.Errorf("%s is written back as %s", c.in, out)
		}
	}
	for _, c := range []struct{ in, err string }{
		{`null`, "set a bind role to one name or a list of names, not null"},
		{`["copy-reviewer",null]`, "set item 2 of the list of a bind role to a name, not null"},
	} {
		var v contracts.BindValue
		if err := json.Unmarshal([]byte(c.in), &v); err == nil || err.Error() != c.err {
			t.Errorf("%s fails with %v, want %q", c.in, err, c.err)
		}
	}
	for _, c := range []struct {
		v    contracts.BindValue
		want string
	}{
		{contracts.BindValue{Name: "copywriter"}, `"copywriter"`},
		{contracts.BindValue{Names: []string{"copy-reviewer"}}, `["copy-reviewer"]`},
	} {
		out, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.want {
			t.Errorf("%+v is written as %s, want %s", c.v, out, c.want)
		}
	}
}

// TestAHarnessPartKeepsItsForm pins that a merged part of a harness selection is read
// from true or from an array of names, and written back in the same form.
func TestAHarnessPartKeepsItsForm(t *testing.T) {
	for _, c := range []struct {
		in   string
		want contracts.HarnessPart
	}{
		{`true`, contracts.HarnessPart{All: true}},
		{`["permissions","model"]`, contracts.HarnessPart{Names: []string{"permissions", "model"}}},
	} {
		var p contracts.HarnessPart
		if err := json.Unmarshal([]byte(c.in), &p); err != nil {
			t.Errorf("%s: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(p, c.want) {
			t.Errorf("%s reads as %+v, want %+v", c.in, p, c.want)
		}
		out, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != c.in {
			t.Errorf("%s is written back as %s", c.in, out)
		}
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
// documents/explore-run-settings.json read into [contracts.RunSettings] with every
// member in place, and are written back as the fixture's bytes, compacted.
func TestRunSettingsReadFromTheFixture(t *testing.T) {
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore-run-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var a struct {
		RunSettings *contracts.RunSettings `json:"run_settings"`
	}
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
	if err := validate(compileRun(t, runSchema("documents/run_settings")), out); err != nil {
		t.Errorf("run_settings marshal to a document its definition refuses: %v", err)
	}
}
