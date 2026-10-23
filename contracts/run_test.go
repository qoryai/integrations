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
		"documents-without-step.json":                                {"documents", "/ required step"},
		"documents-without-domain.json":                              {"documents", "/ required domain"},
		"documents-without-inputs.json":                              {"documents", "/ required inputs"},
		"documents-undefined-member.json":                            {"documents", "/ additionalProperties"},
		"documents-inputs-no-version.json":                           {"documents", "/inputs required version"},
		"documents-prebuild-inputs-target.json":                      {"documents", "/inputs additionalProperties"},
		"documents-prebuild-with-work-item.json":                     {"documents", "/ "},
		"documents-prebuild-with-stack.json":                         {"documents", "/ dependentRequired/stack; /stack ; / required stack_reference"},
		"documents-prebuild-with-workflow.json":                      {"documents", "/ dependentRequired/workflow; /workflow "},
		"documents-explore-without-work-item.json":                   {"documents", "/ required work_item"},
		"documents-prebuild-inputs-version-2.json":                   {"documents", "/inputs/version const"},
		"documents-prebuild-inputs-without-environment-ref.json":     {"documents", "/inputs required environment_ref"},
		"documents-prebuild-inputs-environment-ref-empty.json":       {"documents", "/inputs/environment_ref minLength"},
		"documents-prebuild-inputs-environment-ref-201.json":         {"documents", "/inputs/environment_ref maxLength"},
		"documents-run-settings-prebuild.json":                       {"documents", "/step const"},
		"documents-run-settings-effort-without-model.json":           {"documents", "/run_settings dependentRequired/effort"},
		"documents-run-settings-tiers-without-model.json":            {"documents", "/run_settings dependentRequired/tiers"},
		"documents-run-settings-model-without-provider-models.json":  {"documents", "/run_settings dependentRequired/model"},
		"documents-run-settings-provider-models-without-model.json":  {"documents", "/run_settings dependentRequired/provider_models"},
		"documents-run-settings-unknown-tier.json":                   {"documents", "/run_settings/tiers additionalProperties"},
		"documents-run-settings-undefined-member.json":               {"documents", "/run_settings additionalProperties"},
		"documents-run-settings-tier-without-model.json":             {"documents", "/run_settings/tiers/thinking required model"},
		"documents-run-settings-effort-empty.json":                   {"documents", "/run_settings/provider_models/0/effort minItems"},
		"documents-run-settings-time-limit-minutes-14.json":          {"documents", "/run_settings/time_limit_minutes minimum"},
		"documents-run-settings-time-limit-minutes-1441.json":        {"documents", "/run_settings/time_limit_minutes maximum"},
		"documents-access-without-target.json":                       {"documents", "/ dependentRequired/access"},
		"documents-target-without-access.json":                       {"documents", "/ dependentRequired/target"},
		"documents-prebuild-without-target.json":                     {"documents", "/ required access,target"},
		"documents-domain-upper-case.json":                           {"documents", "/domain pattern"},
		"documents-stack-without-workflow.json":                      {"documents", "/ dependentRequired/stack"},
		"documents-workflow-without-stack.json":                      {"documents", "/ dependentRequired/workflow"},
		"documents-stack-upper-case.json":                            {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-starts-with-digit.json":                     {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-65-characters.json":                         {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-object.json":                                {"documents", "/stack type"},
		"documents-stack-reference-without-path.json":                {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-query.json":                       {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-user-information.json":            {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-http.json":                        {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-ref-dot-dot.json":                 {"documents", "/stack pattern; /stack "},
		"documents-stack-reference-ref-starts-with-hyphen.json":      {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-ref-colon.json":                   {"documents", "/stack pattern; /stack pattern"},
		"documents-stack-reference-path-dot-dot.json":                {"documents", "/stack pattern; /stack "},
		"documents-stack-reference-ref-256-characters.json":          {"documents", "/stack pattern; /stack "},
		"documents-stack-reference-path-256-characters.json":         {"documents", "/stack pattern; /stack "},
		"documents-stack-without-harness-hidden.json":                {"documents", "/ dependentRequired/stack"},
		"documents-harness-hidden-without-stack.json":                {"documents", "/ dependentRequired/harness_hidden"},
		"documents-stack-name-without-stack-reference-member.json":   {"documents", "/ required stack_reference"},
		"documents-stack-reference-with-stack-reference-member.json": {"documents", "/stack_reference "},
		"documents-stack-reference-member-without-stack.json":        {"documents", "/stack_reference "},
		"documents-prebuild-with-stack-reference-member.json":        {"documents", "/stack_reference "},
		"documents-stack-reference-member-http.json":                 {"documents", "/stack_reference pattern"},
		"documents-stack-reference-member-user-information.json":     {"documents", "/stack_reference pattern"},
		"documents-prebuild-with-harness-hidden.json":                {"documents", "/ dependentRequired/harness_hidden; /harness_hidden "},
		"documents-harness-hidden-string.json":                       {"documents", "/harness_hidden type"},
		"documents-access-admin.json":                                {"documents", "/access enum"},
		"documents-prebuild-without-image-destination.json":          {"documents", "/ required image_destination"},
		"documents-explore-with-image-destination.json":              {"documents", "/image_destination ; /step const"},
		"documents-image-destination-tag.json":                       {"documents", "/image_destination pattern"},
		"documents-image-destination-digest.json":                    {"documents", "/image_destination pattern"},
		"documents-image-destination-path-upper-case.json":           {"documents", "/image_destination pattern"},
		"documents-image-destination-scheme.json":                    {"documents", "/image_destination pattern"},
		"documents-image-destination-256-characters.json":            {"documents", "/image_destination maxLength"},
		"documents-image-destination-host-without-dot.json":          {"documents", "/image_destination pattern"},
		"documents-image-destination-without-path.json":              {"documents", "/image_destination pattern"},
		"documents-prebuild-with-both-environment-members.json":      {"documents", "/ oneOf"},
		"documents-prebuild-without-environment-members.json":        {"documents", "/ required environment_destination; / required environment_image"},
		"documents-explore-with-environment-destination.json":        {"documents", "/environment_destination "},
		"documents-explore-with-environment-image.json":              {"documents", "/environment_image "},
		"documents-environment-image-tag.json":                       {"documents", "/environment_image pattern"},
		"documents-environment-image-without-host.json":              {"documents", "/environment_image pattern"},
		"documents-environment-image-upper-case-hex.json":            {"documents", "/environment_image pattern"},
		"documents-environment-destination-digest.json":              {"documents", "/environment_destination pattern"},
		"documents-environment-destination-docker-hub.json":          {"documents", "/environment_destination "},
		"documents-work-item-url-http.json":                          {"documents", "/work_item/url pattern"},
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

// TestAPrebuildIsRefusedForItsStackOrWorkflowAlone pins that the invalid run documents
// of the step prebuild that have stack, stack_reference, workflow or harness_hidden are
// refused for that member alone: the same documents without it are accepted.
func TestAPrebuildIsRefusedForItsStackOrWorkflowAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct{ fixture, member string }{
		{"documents-prebuild-with-stack.json", "stack"},
		{"documents-prebuild-with-workflow.json", "workflow"},
		{"documents-prebuild-with-harness-hidden.json", "harness_hidden"},
		{"documents-prebuild-with-stack-reference-member.json", "stack_reference"},
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

// TestAStackReferenceHasItsEdges pins the edges of the stack reference of
// documents.schema.json, url#ref:path: each row pairs a refused value with an
// accepted value close to it, a value refused by a rule of allOf is refused by that
// rule alone, and any other refused value is refused by no rule of allOf.
func TestAStackReferenceHasItsEdges(t *testing.T) {
	schema := compileRun(t, runSchema("documents/stack"))
	const url = "https://code.example.com/team/stacks"
	long := "https://code.example.com/" + strings.Repeat("s", 2048-len("https://code.example.com/"))
	for _, c := range []struct {
		accepted, refused string
		// rule is the index in the allOf of stack_reference of the one rule that
		// refuses the value, or -1 when the pattern refuses it.
		rule int
	}{
		{url + "#main:core", url + "#main", -1},
		{url + "#main:core", url + "#main:", -1},
		{url + "#main:.", url + "#main:./", -1},
		{url + "#main:core", "http://code.example.com/team/stacks#main:core", -1},
		{url + "#main:core", url + "?at=main#main:core", -1},
		{url + "#main:core", "https://alex@code.example.com/team/stacks#main:core", -1},
		{url + "#main:core", "https://#main:core", -1},
		{"https://code.example.com#main:core", "https://code.example.com#main#next:core", -1},
		{"https://code.example.com:8443/team/stacks#main:core", "https://code.example.com:65536/team/stacks#main:core", -1},
		{"https://[::1]:8443/stacks#main:core", "https://[::1]ab/stacks#main:core", -1},
		{"https://ex%c3%a9mple.com/s#main:core", "https://ex%41mple.com/s#main:core", -1},
		{"https://ex%25mple.com/s#main:core", "https://ex%7Fmple.com/s#main:core", -1},
		{"https://ex%c3%a9mple.com/s#main:core", "https://ex%00mple.com/s#main:core", -1},
		{"https://ex%25mple.com/s#main:core", "https://ex%2fmple.com/s#main:core", -1},
		{url + "#0123456789abcdef0123456789abcdef01234567:.", url + "#:core", -1},
		{url + "#_main:core", url + "#-main:core", -1},
		{url + "#.main:core", url + "#/main:core", -1},
		{url + "#team/main:core", url + "#team//main:core", -1},
		{url + "#team/main:core", url + "#team/main/:core", -1},
		{url + "#team-main:core", url + "#team:main:core", -1},
		{url + "#main.next:core", url + "#main..next:core", 1},
		{url + "#" + strings.Repeat("r", 255) + ":core", url + "#" + strings.Repeat("r", 256) + ":core", 0},
		{url + "#main:stacks/core", url + "#main:stacks//core", -1},
		{url + "#main:stacks/core", url + "#main:/stacks/core", -1},
		{url + "#main:stacks/core", url + "#main:stacks/core/", -1},
		{url + "#main:stacks/...", url + "#main:stacks/..", 3},
		{url + "#main:stacks..core", url + "#main:../core", 3},
		{url + "#main:" + strings.Repeat("p", 255), url + "#main:" + strings.Repeat("p", 256), 2},
		{long + "#" + strings.Repeat("r", 255) + ":" + strings.Repeat("p", 255),
			long + "s#" + strings.Repeat("r", 255) + ":" + strings.Repeat("p", 255), -2},
	} {
		if err := schema.Validate(c.accepted); err != nil {
			t.Errorf("%s is refused: %v", c.accepted, err)
		}
		v, ok := schema.Validate(c.refused).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s is accepted", c.refused)
			continue
		}
		var rules []string
		for _, l := range leafErrors(v) {
			if l.ErrorKind.KeywordPath() == nil {
				rules = append(rules, l.SchemaURL)
			}
		}
		want := map[int]string{-1: "", -2: ""}[c.rule]
		if c.rule >= 0 {
			want = fmt.Sprintf("%s/documents.schema.json#/$defs/stack_reference/allOf/%d", contracts.RunBase, c.rule)
		}
		if strings.Join(rules, "; ") != want {
			t.Errorf("%s is refused by the rules %q, want %q", c.refused, rules, want)
		}
	}
	if n := len(long + "#" + strings.Repeat("r", 255) + ":" + strings.Repeat("p", 255)); n != 2560 {
		t.Errorf("the longest accepted reference has %d characters, want 2560", n)
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
		{"documents", contracts.Documents{Step: "explore", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), WorkItem: &item, Stack: "core",
			StackReference: "https://code.example.com/team/stacks#main:core", Workflow: "hello",
			HarnessHidden: &hidden}},
		{"documents", contracts.Documents{Step: "explore", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), WorkItem: &item,
			Stack: "https://code.example.com/team/stacks#main:core", Workflow: "hello", HarnessHidden: &shown}},
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

// TestTheRunDocumentsReadIntoTheirGoTypes pins that the target, work_item and
// run_settings of each run documents fixture read into their Go types and
// are written back as the fixture's bytes, compacted, that a fixture without
// target has no access either, and that StackReference is set exactly when Stack is
// a stack name.
func TestTheRunDocumentsReadIntoTheirGoTypes(t *testing.T) {
	n, named := 0, 0
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
			if _, ok := members["access"]; ok {
				t.Errorf("%s has access and no target", f)
			}
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		name := d.Stack != "" && !strings.HasPrefix(d.Stack, "https://")
		if name != (d.StackReference != "") {
			t.Errorf("%s has the stack %q and the stack_reference %q", f, d.Stack, d.StackReference)
		}
		if name {
			named++
		}
		for name, v := range map[string]any{
			"target":       &contracts.Target{},
			"work_item":    &contracts.WorkItem{},
			"run_settings": &contracts.RunSettings{},
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
		t.Error("fixtures/documents has no target, work_item or run_settings")
	}
	if named == 0 {
		t.Error("fixtures/documents has no stack name with stack_reference")
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
