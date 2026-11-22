package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// runSchema is the path of the schema that the fixtures under fixtures/<name> of the
// run contract validate against: <name>.schema.json, and for documents/<definition>,
// that definition of documents.schema.json.
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

// runSchemas are the schemas of the run contract: the files of run/v1.
var runSchemas = []string{"documents", "next-session-answer", "next-session-refusal", "next-session-request", "outcome"}

// TestRunSchemaIsTheContract pins that run/v1 contains exactly README.md,
// documents.schema.json, next-session-answer.schema.json, next-session-refusal.schema.json,
// next-session-request.schema.json and outcome.schema.json, each with the $id
// RunBase/<file name>, and that each compiles.
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
	want := "README.md,documents.schema.json,next-session-answer.schema.json,next-session-refusal.schema.json,next-session-request.schema.json,outcome.schema.json"
	if strings.Join(got, ",") != want {
		t.Errorf("run/v1 contains %v, want %s", got, want)
	}
	for _, name := range runSchemas {
		file := name + ".schema.json"
		doc, err := contracts.RunDocument(file)
		if err != nil {
			t.Fatal(err)
		}
		if id := doc.(map[string]any)["$id"]; id != contracts.RunBase+"/"+file {
			t.Errorf("%s has the $id %v", file, id)
		}
		compileRun(t, file)
	}
}

// TestRunFixturesValidate pins that each document under fixtures/<name>/ of the run
// contract is accepted by <name>.schema.json, for the five schemas of the contract,
// and that fixtures/ contains no other folder of valid documents.
func TestRunFixturesValidate(t *testing.T) {
	for _, name := range runSchemas {
		schema := compileRun(t, runSchema(name))
		files := runFiles(t, "fixtures/"+name)
		if len(files) == 0 {
			t.Errorf("fixtures/%s contains no document", name)
		}
		for _, f := range files {
			doc, err := contracts.RunDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
	dirs, err := fs.ReadDir(contracts.RunFS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if d.IsDir() && d.Name() != "invalid" && d.Name() != "closed" && d.Name() != "check" && !slices.Contains(runSchemas, d.Name()) {
			t.Errorf("fixtures/%s is the fixture folder of no schema", d.Name())
		}
	}
}

// TestInvalidRunFixturesAreRefused pins that each document under fixtures/invalid of
// the run contract fails its schema for the reason in its name, and for no other. The
// test compares the reasons in sorted order, because the validator reports the members
// of one dependentRequired that fail in no fixed order.
func TestInvalidRunFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"documents-without-step.json":                                        {"documents", "/ required step"},
		"documents-without-domain.json":                                      {"documents", "/ required domain"},
		"documents-without-inputs.json":                                      {"documents", "/ required inputs"},
		"documents-inputs-no-version.json":                                   {"documents", "/inputs required version"},
		"documents-prebuild-with-subject.json":                               {"documents", "/ "},
		"documents-prebuild-with-workflow.json":                              {"documents", "/ dependentRequired/workflow; /workflow ; / required harness; / required target_version"},
		"documents-workflow-without-subject.json":                            {"documents", "/ required subject"},
		"documents-explore-without-run-settings.json":                        {"documents", "/ required run_settings"},
		"documents-prebuild-inputs-version-2.json":                           {"documents", "/inputs/version const"},
		"documents-prebuild-inputs-without-environment-ref.json":             {"documents", "/inputs required environment_ref"},
		"documents-prebuild-inputs-environment-ref-empty.json":               {"documents", "/inputs/environment_ref minLength"},
		"documents-prebuild-inputs-environment-ref-201.json":                 {"documents", "/inputs/environment_ref maxLength"},
		"documents-run-settings-prebuild.json":                               {"documents", "/step const"},
		"documents-run-settings-effort-without-model.json":                   {"documents", "/run_settings dependentRequired/effort"},
		"documents-run-settings-tiers-without-model.json":                    {"documents", "/run_settings dependentRequired/tiers"},
		"documents-run-settings-model-without-provider-models.json":          {"documents", "/run_settings dependentRequired/model"},
		"documents-run-settings-provider-models-without-model.json":          {"documents", "/run_settings dependentRequired/provider_models"},
		"documents-run-settings-tier-without-model.json":                     {"documents", "/run_settings/tiers/thinking required model"},
		"documents-run-settings-effort-empty.json":                           {"documents", "/run_settings/provider_models/0/effort minItems"},
		"documents-time-limit-minutes-0.json":                                {"documents", "/time_limit_minutes minimum"},
		"documents-time-limit-minutes-1441.json":                             {"documents", "/time_limit_minutes maximum"},
		"documents-time-limit-minutes-string.json":                           {"documents", "/time_limit_minutes type"},
		"documents-shared-run-string.json":                                   {"documents", "/shared_run type"},
		"documents-type-settings-string.json":                                {"documents", "/type_settings type"},
		"documents-prebuild-with-time-limit-minutes.json":                    {"documents", "/step const"},
		"documents-prebuild-with-shared-run.json":                            {"documents", "/step const"},
		"documents-prebuild-with-type-settings.json":                         {"documents", "/step const"},
		"next-session-answer-both-shapes.json":                               {"next-session-answer", "/next const; / "},
		"next-session-answer-end-with-workflow.json":                         {"next-session-answer", "/ required session,time_limit_minutes,inputs; /next const; / "},
		"next-session-answer-inputs-without-version.json":                    {"next-session-answer", "/inputs required version; /next const; / "},
		"next-session-answer-model-choice-effort-without-model.json":         {"next-session-answer", "/model_choice dependentRequired/provider_models; /model_choice dependentRequired/effort; /model_choice dependentRequired/tiers; /next const; / "},
		"next-session-answer-model-choice-runtime-number.json":               {"next-session-answer", "/model_choice/runtime type; /next const; / "},
		"next-session-answer-model-choice-runtime-upper-case.json":           {"next-session-answer", "/model_choice/runtime pattern; /next const; / "},
		"next-session-answer-model-choice-runtime-effort-only.json":          {"next-session-answer", "/model_choice dependentRequired/effort; /next const; / "},
		"next-session-answer-model-provider-credential-name-upper-case.json": {"next-session-answer", "/model_provider_credential_name pattern; /next const; / "},
		"next-session-answer-next-unknown.json":                              {"next-session-answer", "/next const; /next const; / "},
		"next-session-answer-session-0.json":                                 {"next-session-answer", "/session minimum; /next const; / "},
		"next-session-answer-session-without-inputs.json":                    {"next-session-answer", "/ required inputs; /next const; / "},
		"next-session-answer-session-without-time-limit-minutes.json":        {"next-session-answer", "/ required time_limit_minutes; /next const; / "},
		"next-session-answer-session-without-workflow.json":                  {"next-session-answer", "/ required workflow; /next const; / "},
		"next-session-answer-time-limit-minutes-0.json":                      {"next-session-answer", "/time_limit_minutes minimum; /next const; / "},
		"next-session-answer-time-limit-minutes-string.json":                 {"next-session-answer", "/time_limit_minutes type; /next const; / "},
		"next-session-answer-time-limit-minutes-1441.json":                   {"next-session-answer", "/time_limit_minutes maximum; /next const; / "},
		"next-session-answer-without-next.json":                              {"next-session-answer", "/ required next; / required next; / "},
		"next-session-request-reason-upper-case.json":                        {"next-session-request", "/reason type; /reason pattern"},
		"next-session-request-session-0.json":                                {"next-session-request", "/session minimum"},
		"next-session-request-session-string.json":                           {"next-session-request", "/session type"},
		"next-session-request-state-paused.json":                             {"next-session-request", "/state enum"},
		"next-session-request-state-unknown.json":                            {"next-session-request", "/state enum"},
		"next-session-request-without-reason.json":                           {"next-session-request", "/ required reason"},
		"next-session-request-cancelled-without-reason.json":                 {"next-session-request", "/reason type"},
		"next-session-request-reason-timeout-state-failed.json":              {"next-session-request", "/state const"},
		"next-session-request-reason-interrupted-state-succeeded.json":       {"next-session-request", "/state const"},
		"next-session-request-reason-gateway-lost-state-cancelled.json":      {"next-session-request", "/state const"},
		"next-session-request-without-session.json":                          {"next-session-request", "/ required session"},
		"next-session-refusal-unknown-code.json":                             {"next-session-refusal", "/error enum"},
		"next-session-refusal-from-gateway.json":                             {"next-session-refusal", "/from const"},
		"outcome-end-code-succeeded.json":                                    {"outcome", "/state const"},
		"outcome-environment-image-missing-succeeded.json":                   {"outcome", "/state const"},
		"outcome-environment-image-missing-cancelled.json":                   {"outcome", "/state const"},
		"outcome-push-head-changed-succeeded.json":                           {"outcome", "/state const"},
		"outcome-push-head-changed-cancelled.json":                           {"outcome", "/state const"},
		"outcome-push-scope-refused-succeeded.json":                          {"outcome", "/state const"},
		"outcome-push-scope-refused-cancelled.json":                          {"outcome", "/state const"},
		"outcome-push-key-found-succeeded.json":                              {"outcome", "/state const"},
		"outcome-push-key-found-cancelled.json":                              {"outcome", "/state const"},
		"outcome-run-documents-invalid-succeeded.json":                       {"outcome", "/state const"},
		"outcome-run-documents-invalid-cancelled.json":                       {"outcome", "/state const"},
		"outcome-harness-invalid-succeeded.json":                             {"outcome", "/state const"},
		"outcome-harness-invalid-cancelled.json":                             {"outcome", "/state const"},
		"outcome-machine-setup-invalid-succeeded.json":                       {"outcome", "/state const"},
		"outcome-machine-setup-invalid-cancelled.json":                       {"outcome", "/state const"},
		"outcome-target-fetch-failed-succeeded.json":                         {"outcome", "/state const"},
		"outcome-target-fetch-failed-cancelled.json":                         {"outcome", "/state const"},
		"outcome-repository-check-failed-succeeded.json":                     {"outcome", "/state const"},
		"outcome-repository-check-failed-cancelled.json":                     {"outcome", "/state const"},
		"outcome-forge-unreachable-succeeded.json":                           {"outcome", "/state const"},
		"outcome-forge-unreachable-cancelled.json":                           {"outcome", "/state const"},
		"outcome-history-rewritten-succeeded.json":                           {"outcome", "/state const"},
		"outcome-history-rewritten-cancelled.json":                           {"outcome", "/state const"},
		"outcome-run-api-failed-succeeded.json":                              {"outcome", "/state const"},
		"outcome-run-api-failed-cancelled.json":                              {"outcome", "/state const"},
		"outcome-prebuild-failed-succeeded.json":                             {"outcome", "/state const"},
		"outcome-prebuild-failed-cancelled.json":                             {"outcome", "/state const"},
		"outcome-next-session-failed-succeeded.json":                         {"outcome", "/state const"},
		"outcome-next-session-failed-cancelled.json":                         {"outcome", "/state const"},
		"outcome-outputs-invalid-succeeded.json":                             {"outcome", "/state const"},
		"outcome-outputs-invalid-cancelled.json":                             {"outcome", "/state const"},
		"outcome-end-code-cancelled.json":                                    {"outcome", "/state const"},
		"outcome-reason-empty.json":                                          {"outcome", "/reason pattern"},
		"outcome-reason-upper-case.json":                                     {"outcome", "/reason pattern"},
		"outcome-without-state.json":                                         {"outcome", "/ required state"},
		"outcome-state-unknown.json":                                         {"outcome", "/state enum"},
		"outcome-exit-code-below-minus-1.json":                               {"outcome", "/exit_code minimum"},
		"documents-access-without-target.json":                               {"documents", "/ dependentRequired/access"},
		"documents-target-without-access.json":                               {"documents", "/ dependentRequired/target"},
		"documents-prebuild-without-target.json":                             {"documents", "/ required access,target"},
		"documents-domain-upper-case.json":                                   {"documents", "/domain pattern"},
		"documents-workflow-without-harness-hidden.json":                     {"documents", "/ dependentRequired/workflow"},
		"documents-harness-hidden-without-workflow.json":                     {"documents", "/ dependentRequired/harness_hidden; / required workflow"},
		"documents-workflow-step-without-workflow.json":                      {"documents", "/ required workflow"},
		"documents-prebuild-with-harness-hidden.json":                        {"documents", "/ dependentRequired/harness_hidden; /harness_hidden "},
		"documents-harness-hidden-string.json":                               {"documents", "/harness_hidden type"},
		"documents-access-admin.json":                                        {"documents", "/access enum"},
		"documents-prebuild-without-image-destination.json":                  {"documents", "/ required image_destination"},
		"documents-explore-with-image-destination.json":                      {"documents", "/image_destination ; /step const"},
		"documents-explore-with-largest-image.json":                          {"documents", "/largest_prebuilt_image_bytes "},
		"documents-largest-image-zero.json":                                  {"documents", "/largest_prebuilt_image_bytes minimum"},
		"documents-largest-image-negative.json":                              {"documents", "/largest_prebuilt_image_bytes minimum"},
		"documents-largest-image-fraction.json":                              {"documents", "/largest_prebuilt_image_bytes type"},
		"documents-largest-image-string.json":                                {"documents", "/largest_prebuilt_image_bytes type"},
		"documents-largest-image-above-int64.json":                           {"documents", "/largest_prebuilt_image_bytes maximum"},
		"documents-image-destination-tag.json":                               {"documents", "/image_destination pattern"},
		"documents-image-destination-digest.json":                            {"documents", "/image_destination pattern"},
		"documents-image-destination-path-upper-case.json":                   {"documents", "/image_destination pattern"},
		"documents-image-destination-scheme.json":                            {"documents", "/image_destination pattern"},
		"documents-image-destination-256-characters.json":                    {"documents", "/image_destination maxLength"},
		"documents-image-destination-host-without-dot.json":                  {"documents", "/image_destination pattern"},
		"documents-image-destination-without-path.json":                      {"documents", "/image_destination pattern"},
		"documents-prebuild-with-both-environment-members.json":              {"documents", "/ oneOf"},
		"documents-prebuild-without-environment-members.json":                {"documents", "/ required environment_destination; / required environment_image"},
		"documents-explore-with-environment-destination.json":                {"documents", "/environment_destination "},
		"documents-explore-with-environment-image.json":                      {"documents", "/environment_image "},
		"documents-environment-image-tag.json":                               {"documents", "/environment_image pattern"},
		"documents-environment-image-without-host.json":                      {"documents", "/environment_image pattern"},
		"documents-environment-image-upper-case-hex.json":                    {"documents", "/environment_image pattern"},
		"documents-environment-destination-digest.json":                      {"documents", "/environment_destination pattern"},
		"documents-environment-destination-docker-hub.json":                  {"documents", "/environment_destination "},
		"documents-harness-without-extends-or-modules.json":                  {"documents", "/harness/harness required modules"},
		"documents-harness-extends-and-workflows.json":                       {"documents", "/harness/harness/workflows "},
		"documents-harness-workflow-access-admin.json":                       {"documents", "/harness/harness/workflows/hello/access enum"},
		"documents-harness-workflow-tier-unknown.json":                       {"documents", "/harness/harness/workflows/hello/tier enum"},
		"documents-harness-workflow-review-group-slash.json":                 {"documents", "/harness/harness/workflows/verify-a-ticket/review_group pattern"},
		"documents-harness-workflow-runs-locked-checks-string.json":          {"documents", "/harness/harness/workflows/verify-a-ticket/runs_locked_checks type"},
		"documents-harness-workflow-plan-without-write.json":                 {"documents", "/harness/harness/workflows/hello/access const"},
		"documents-harness-workflow-plan-without-access.json":                {"documents", "/harness/harness/workflows/hello required access"},
		"documents-harness-workflow-name-56-characters.json":                 {"documents", "/ pattern"},
		"documents-workflow-56-characters.json":                              {"documents", "/workflow pattern"},
		"documents-harness-path-source.json":                                 {"documents", "/harness/harness/modules/0/source required git,ref"},
		"documents-harness-extends-path-stack.json":                          {"documents", "/harness/harness/extends required git,ref; /harness/harness/extends "},
		"documents-harness-module-name-only.json":                            {"documents", "/harness/harness/modules/0 required source"},
		"documents-harness-extends-path-and-stack.json":                      {"documents", "/harness/harness/extends "},
		"documents-harness-module-path-and-module.json":                      {"documents", "/harness/harness/modules/0/source "},
		"documents-harness-bind-list-empty.json":                             {"documents", "/harness/harness/bind/agents/reviewers type; /harness/harness/bind/agents/reviewers minItems"},
		"documents-harness-bind-list-commands.json":                          {"documents", "/harness/harness/bind/commands/publish type"},
		"documents-harness-bind-list-skills.json":                            {"documents", "/harness/harness/bind/skills/review type"},
		"documents-harness-bind-list-name-slash.json":                        {"documents", "/harness/harness/bind/agents/reviewers type; /harness/harness/bind/agents/reviewers/1 pattern"},
		"documents-harness-bind-name-slash.json":                             {"documents", "/harness/harness/bind/agents/writer pattern; /harness/harness/bind/agents/writer type"},
		"documents-harness-bind-role-dot.json":                               {"documents", "/ pattern"},
		"documents-target-version-without-target.json":                       {"documents", "/ dependentRequired/target_version"},
		"documents-read-targets-21-entries.json":                             {"documents", "/read_targets maxItems"},
		"documents-read-targets-without-collection.json":                     {"documents", "/read_targets/0 required collection"},
		"documents-prebuild-with-read-targets.json":                          {"documents", "/read_targets "},
		"documents-target-version-space.json":                                {"documents", "/target_version pattern"},
		"documents-prebuild-with-harness.json":                               {"documents", "/harness "},
		"documents-prebuild-with-target-version.json":                        {"documents", "/target_version "},
		"documents-workflow-without-harness-or-target-version.json":          {"documents", "/ required harness; / required target_version"},
		"documents-subject-url-http.json":                                    {"documents", "/subject/url pattern"},
		"documents-deliver-type-upper-case.json":                             {"documents", "/deliver/0/type pattern"},
		"documents-deliver-type-underscore.json":                             {"documents", "/deliver/0/type pattern"},
		"documents-deliver-type-65-characters.json":                          {"documents", "/deliver/0/type maxLength"},
		"documents-deliver-send-unknown.json":                                {"documents", "/deliver/0/send enum"},
		"documents-deliver-without-type.json":                                {"documents", "/deliver/0 required type"},
		"documents-deliver-without-send.json":                                {"documents", "/deliver/0 required send"},
		"documents-deliver-empty.json":                                       {"documents", "/deliver minItems"},
		"documents-deliver-17-entries.json":                                  {"documents", "/deliver maxItems"},
		"documents-deliver-access-read.json":                                 {"documents", "/access const"},
		"documents-deliver-access-write-no-push.json":                        {"documents", "/access const"},
		"documents-deliver-without-workflow.json":                            {"documents", "/ dependentRequired/deliver; / required workflow"},
		"documents-prebuild-with-deliver.json":                               {"documents", "/ dependentRequired/deliver; /deliver ; /access const"},
		"documents-step-explore.json":                                        {"documents", "/step const; /step enum"},
		"documents-subject-without-kind.json":                                {"documents", "/subject required kind"},
		"documents-subject-without-id.json":                                  {"documents", "/subject required id"},
		"documents-subject-work-item-id-prefix.json":                         {"documents", "/subject/id pattern"},
		"documents-subject-kind-camel-case.json":                             {"documents", "/subject/kind pattern"},
		"documents-subject-kind-hyphen.json":                                 {"documents", "/subject/kind pattern"},
		"documents-subject-kind-65-characters.json":                          {"documents", "/subject/kind maxLength"},
		"documents-subject-id-empty.json":                                    {"documents", "/subject/id minLength; /subject/id pattern"},
		"documents-subject-id-line-feed.json":                                {"documents", "/subject/id pattern"},
		"documents-subject-id-201-characters.json":                           {"documents", "/subject/id maxLength"},
		"documents-subject-title-line-feed.json":                             {"documents", "/subject/title pattern"},
		"documents-trigger-hyphen.json":                                      {"documents", "/trigger pattern"},
		"documents-trigger-upper-case.json":                                  {"documents", "/trigger pattern"},
		"documents-trigger-65-characters.json":                               {"documents", "/trigger maxLength"},
		"documents-work-item-member.json":                                    {"documents", "/ required subject"},
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
		r, reasons := leaves(v), strings.Split(w.reason, "; ")
		slices.Sort(r)
		slices.Sort(reasons)
		if !slices.Equal(r, reasons) {
			t.Errorf("%s is refused for %q, want %q", f, strings.Join(r, "; "), w.reason)
		}
	}
}

// TestAPrebuildIsRefusedForEachRunMemberAlone pins that the invalid run documents of
// the step prebuild that have workflow, harness_hidden, harness, deliver,
// time_limit_minutes, shared_run or type_settings are refused for that member alone: the
// same documents without it are accepted. TestReadTargetsAreRefusedForOneRuleAlone
// covers read_targets.
func TestAPrebuildIsRefusedForEachRunMemberAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct{ fixture, member string }{
		{"documents-prebuild-with-workflow.json", "workflow"},
		{"documents-prebuild-with-harness-hidden.json", "harness_hidden"},
		{"documents-prebuild-with-harness.json", "harness"},
		{"documents-prebuild-with-deliver.json", "deliver"},
		{"documents-prebuild-with-time-limit-minutes.json", "time_limit_minutes"},
		{"documents-prebuild-with-shared-run.json", "shared_run"},
		{"documents-prebuild-with-type-settings.json", "type_settings"},
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

// TestADeliverIsRefusedForItsDeliverAlone pins that each invalid fixture refused at
// /deliver is refused for its deliver and for nothing else: the same documents with the
// deliver of documents/explore-deliver-with-harness.json are accepted. The refused values
// are a type in upper case, with an underscore or of 65 characters, a send other than
// end_of_run and after_gate, no type or no send, an empty list and a list of 17 entries.
func TestADeliverIsRefusedForItsDeliverAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	valid, err := contracts.RunDocument("fixtures/documents/explore-deliver-with-harness.json")
	if err != nil {
		t.Fatal(err)
	}
	deliver := valid.(map[string]any)["deliver"]
	for _, fixture := range []string{
		"documents-deliver-type-upper-case.json",
		"documents-deliver-type-underscore.json",
		"documents-deliver-type-65-characters.json",
		"documents-deliver-send-unknown.json",
		"documents-deliver-without-type.json",
		"documents-deliver-without-send.json",
		"documents-deliver-empty.json",
		"documents-deliver-17-entries.json",
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", fixture))
		if err != nil {
			t.Fatal(err)
		}
		v, ok := schema.Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s is accepted", fixture)
			continue
		}
		if r := strings.Join(leaves(v), "; "); !strings.HasPrefix(r, "/deliver ") && !strings.HasPrefix(r, "/deliver/") {
			t.Errorf("%s is refused for %q, not at /deliver", fixture, r)
		}
		doc.(map[string]any)["deliver"] = deliver
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s with the deliver of documents/explore-deliver-with-harness.json is refused: %v", fixture, err)
		}
	}
}

// TestADeliverIsRefusedForOneRuleAlone pins that run documents with deliver are refused on
// a run with access read, on a run with access write-no-push and on a run with no
// workflow, and for that alone: the same documents with access write, or with workflow
// and harness_hidden, are accepted.
func TestADeliverIsRefusedForOneRuleAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct {
		fixture string
		fix     func(map[string]any)
	}{
		{"documents-deliver-access-read.json", func(doc map[string]any) { doc["access"] = "write" }},
		{"documents-deliver-access-write-no-push.json", func(doc map[string]any) { doc["access"] = "write" }},
		{"documents-deliver-without-workflow.json", func(doc map[string]any) {
			doc["workflow"] = "build"
			doc["harness_hidden"] = false
		}},
	} {
		v, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		doc := v.(map[string]any)
		if doc["deliver"] == nil {
			t.Errorf("%s does not have deliver", c.fixture)
			continue
		}
		if schema.Validate(doc) == nil {
			t.Errorf("%s is accepted", c.fixture)
		}
		c.fix(doc)
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s, corrected, is refused: %v", c.fixture, err)
		}
	}
}

// TestRunDocumentsWithSubmitAreRefused pins that the run documents have no submit member.
// The open schema ignores an undefined member, so the closed copy refuses a submit at
// the root, and a document that lists /submit in must_understand is refused, because
// the schema and the Go type define no such pointer.
func TestRunDocumentsWithSubmitAreRefused(t *testing.T) {
	open := compileRun(t, runSchema("documents"))
	read := func() map[string]any {
		t.Helper()
		v, err := contracts.RunDocument("fixtures/closed/documents-submit-undefined-member.json")
		if err != nil {
			t.Fatal(err)
		}
		return v.(map[string]any)
	}
	doc := read()
	if doc["submit"] == nil {
		t.Fatal("fixtures/closed/documents-submit-undefined-member.json has no submit")
	}
	if err := open.Validate(doc); err != nil {
		t.Errorf("the open schema refuses a submit member, which it ignores: %v", err)
	}
	closed, err := contracts.CompileRunClosed("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := closed.Validate(doc).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("the closed copy accepts run documents with submit")
	}
	if r := strings.Join(leaves(v), "; "); r != "/ additionalProperties" {
		t.Errorf("the closed copy refuses run documents with submit for %q, want %q", r, "/ additionalProperties")
	}
	for _, p := range []string{"/submit", "/submit/type", "/submit/send"} {
		doc := read()
		doc["must_understand"] = []any{p}
		if err := open.Validate(doc); err == nil {
			t.Errorf("the schema accepts must_understand with %s", p)
		}
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckMustUnderstand(b, contracts.Documents{}); err == nil {
			t.Errorf("CheckMustUnderstand accepts %s for the Go type", p)
		}
	}
}

// TestADeliverHasAtMostSixteenEntries pins MaxDeliver and maxItems of deliver: a list of
// 1 or 16 entries is accepted, and a list of 17 is refused for maxItems.
func TestADeliverHasAtMostSixteenEntries(t *testing.T) {
	if contracts.MaxDeliver != 16 {
		t.Errorf("MaxDeliver is %d, want 16", contracts.MaxDeliver)
	}
	schema := compileRun(t, runSchema("documents/deliver"))
	for _, c := range []struct {
		n    int
		want string
	}{{1, ""}, {16, ""}, {17, "/ maxItems"}} {
		var list []any
		for i := 1; i <= c.n; i++ {
			list = append(list, map[string]any{"type": fmt.Sprintf("artifact-%d", i), "send": "after_gate"})
		}
		got := ""
		if v, ok := schema.Validate(list).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != c.want {
			t.Errorf("a deliver of %d entries is refused for %q, want %q", c.n, got, c.want)
		}
	}
}

// TestCheckRefusesWhatTheSchemaCannotSay pins that each document under fixtures/check
// is accepted by documents.schema.json and breaks a rule that Documents.Check reports, and
// that each valid fixture of run documents passes Documents.Check.
func TestCheckRefusesWhatTheSchemaCannotSay(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	want := map[string]string{
		"documents-deliver-type-twice.json":                 `List each deliver type once: "code-change" appears twice.`,
		"documents-read-targets-collection-twice.json":      `List each collection of read_targets once: "R_1002" appears twice.`,
		"documents-read-targets-collection-of-target.json":  `Remove the collection "R_1001" from read_targets (index 1): target has it.`,
		"documents-destination-id-twice.json":               `List each destination once: "tg_0123456789abcdef" appears twice.`,
		"documents-destination-id-twice-after-another.json": `List each destination once: "tg_0123456789abcdef" appears twice.`,
	}
	got := runFiles(t, "fixtures/check")
	if len(got) != len(want) {
		t.Errorf("fixtures/check holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s is refused by the schema: %v", f, err)
		}
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := d.Check(); err == nil || err.Error() != want[path.Base(f)] {
			t.Errorf("%s: Check returns %v, want %q", f, err, want[path.Base(f)])
		}
	}
	for _, f := range runFiles(t, "fixtures/documents") {
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := d.Check(); err != nil {
			t.Errorf("%s: Check returns %v", f, err)
		}
	}
}

// TestReadTargetsAreRefusedForOneRuleAlone pins that run documents are refused for
// read_targets with 21 entries, with an entry that has no collection, and on the step
// prebuild, and for that alone: the same documents with 20 entries, with an entry that
// has a collection, or without read_targets, are accepted.
func TestReadTargetsAreRefusedForOneRuleAlone(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	valid, err := contracts.RunDocument("fixtures/documents/explore-read-targets.json")
	if err != nil {
		t.Fatal(err)
	}
	one := valid.(map[string]any)["read_targets"].([]any)[0]
	for _, c := range []struct {
		fixture string
		fix     func(map[string]any)
	}{
		{"documents-read-targets-21-entries.json", func(doc map[string]any) {
			doc["read_targets"] = doc["read_targets"].([]any)[:20]
		}},
		{"documents-read-targets-without-collection.json", func(doc map[string]any) {
			doc["read_targets"] = []any{one}
		}},
		{"documents-prebuild-with-read-targets.json", func(doc map[string]any) {
			delete(doc, "read_targets")
		}},
	} {
		v, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		doc := v.(map[string]any)
		if doc["read_targets"] == nil {
			t.Errorf("%s does not have read_targets", c.fixture)
			continue
		}
		if schema.Validate(doc) == nil {
			t.Errorf("%s is accepted", c.fixture)
		}
		c.fix(doc)
		if err := schema.Validate(doc); err != nil {
			t.Errorf("%s, corrected, is refused: %v", c.fixture, err)
		}
	}
}

// TestReadTargetsHaveAtMostTwentyEntries pins MaxReadTargets and maxItems of
// read_targets: a list of 0, 1 or 20 targets is accepted, and a list of 21 is refused
// for maxItems.
func TestReadTargetsHaveAtMostTwentyEntries(t *testing.T) {
	if contracts.MaxReadTargets != 20 {
		t.Errorf("MaxReadTargets is %d, want 20", contracts.MaxReadTargets)
	}
	schema := compileRun(t, runSchema("documents/read_targets"))
	for _, c := range []struct {
		n    int
		want string
	}{{0, ""}, {1, ""}, {20, ""}, {21, "/ maxItems"}} {
		list := []any{}
		for i := 1; i <= c.n; i++ {
			list = append(list, map[string]any{"collection": fmt.Sprintf("R_%d", i)})
		}
		got := ""
		if v, ok := schema.Validate(list).(*jsonschema.ValidationError); ok {
			got = strings.Join(leaves(v), "; ")
		}
		if got != c.want {
			t.Errorf("read_targets of %d entries is refused for %q, want %q", c.n, got, c.want)
		}
	}
}

// TestCheckReportsARepeatedDestinationID pins the id rule of Documents.Check: an id twice
// among the destinations of the inputs is reported, different ids are not, and inputs
// with no destinations, or that are not an object, are not checked.
func TestCheckReportsARepeatedDestinationID(t *testing.T) {
	for _, c := range []struct{ name, inputs, want string }{
		{"different ids", `{"destinations":[{"id":"tg_a"},{"id":"tg_b"}]}`, ""},
		{"an id twice", `{"destinations":[{"id":"tg_a"},{"id":"tg_b"},{"id":"tg_a"}]}`, `List each destination once: "tg_a" appears twice.`},
		{"no destinations", `{"version":1}`, ""},
		{"inputs that are not an object", `[]`, ""},
		{"no inputs", ``, ""},
	} {
		got := ""
		if err := (contracts.Documents{Inputs: json.RawMessage(c.inputs)}).Check(); err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("%s: Check returns %q, want %q", c.name, got, c.want)
		}
	}
}

// TestCheckReportsARepeatedCollection pins the collection rules of Documents.Check:
// a collection twice among the read targets, and the collection of the target among
// them, are reported, and different collections are not.
func TestCheckReportsARepeatedCollection(t *testing.T) {
	main := &contracts.Target{Collection: "R_1"}
	for _, c := range []struct {
		name   string
		target *contracts.Target
		read   []contracts.Target
		want   string
	}{
		{"different collections", main, []contracts.Target{{Collection: "R_2"}, {Collection: "R_3"}}, ""},
		{"no target", nil, []contracts.Target{{Collection: "R_2"}, {Collection: "R_3"}}, ""},
		{"none", main, nil, ""},
		{"an empty list", main, []contracts.Target{}, ""},
		{"twice, with no target", nil, []contracts.Target{{Collection: "R_2"}, {Collection: "R_2"}}, `List each collection of read_targets once: "R_2" appears twice.`},
		{"twice", main, []contracts.Target{{Collection: "R_2"}, {Collection: "R_2"}}, `List each collection of read_targets once: "R_2" appears twice.`},
		{"the collection of the target", main, []contracts.Target{{Collection: "R_2"}, {Collection: "R_1"}}, `Remove the collection "R_1" from read_targets (index 1): target has it.`},
		{"the same path under another collection", main, []contracts.Target{{Collection: "R_2", Address: &contracts.TargetAddress{System: "code.example.com", Path: "acme/shop"}}}, ""},
	} {
		got := ""
		if err := (contracts.Documents{Target: c.target, ReadTargets: c.read}).Check(); err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("%s: Check returns %q, want %q", c.name, got, c.want)
		}
	}
}

// TestAnAccessIsOneOfThreeLevels pins the access of documents.schema.json and the
// access of a workflow in harness: read, write-no-push and write are accepted, and
// every other value is refused.
func TestAnAccessIsOneOfThreeLevels(t *testing.T) {
	access := compileRun(t, runSchema("documents/access"))
	workflow := compileRun(t, runSchema("documents/harness_workflow"))
	check := func(v string) (error, error) {
		return access.Validate(v), workflow.Validate(map[string]any{"access": v})
	}
	for _, v := range []string{"read", "write-no-push", "write"} {
		if a, w := check(v); a != nil || w != nil {
			t.Errorf("%q is refused: access %v, workflow %v", v, a, w)
		}
	}
	for _, v := range []string{"", "admin", "Write", "WRITE-NO-PUSH", "write_no_push", "write-no-push ",
		"writenopush", "no-push", "push", "read-write"} {
		if a, w := check(v); a == nil || w == nil {
			t.Errorf("%q is accepted: access %v, workflow %v", v, a, w)
		}
	}
}

// TestADeliverTypeHasItsGrammar pins the type of an entry of deliver of documents.schema.json:
// lower-case letters and digits in groups joined by single hyphens, 1 to 64 bytes,
// and nothing else.
func TestADeliverTypeHasItsGrammar(t *testing.T) {
	schema := compileRun(t, runSchema("documents/deliver_entry"))
	check := func(v string) error {
		return schema.Validate(map[string]any{"type": v, "send": "end_of_run"})
	}
	for _, v := range []string{"a", "7", "code-change", "release-notes-2", "a1-b2-c3", strings.Repeat("a", 64)} {
		if err := check(v); err != nil {
			t.Errorf("%q is refused: %v", v, err)
		}
	}
	for _, v := range []string{"", strings.Repeat("a", 65), "Code", "code_change", "-code", "code-", "code--change",
		"code change", "code.change", "code/change"} {
		if check(v) == nil {
			t.Errorf("%q is accepted", v)
		}
	}
	for _, send := range []string{"end_of_run", "after_gate"} {
		if err := schema.Validate(map[string]any{"type": "code-change", "send": send}); err != nil {
			t.Errorf("the send %q is refused: %v", send, err)
		}
	}
	for _, send := range []string{"", "End_of_run", "on_merge", "after-gate"} {
		if schema.Validate(map[string]any{"type": "code-change", "send": send}) == nil {
			t.Errorf("the send %q is accepted", send)
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
// 256 refused; that the documents of the step workflow with image_destination are
// refused for that member alone; and that the documents of a step other than workflow
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
	if !ok || documents["step"] != "workflow" || documents["image_destination"] == nil {
		t.Fatal("documents-explore-with-image-destination.json are not run documents of the step workflow with image_destination")
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
	if r := strings.Join(leaves(v), "; "); r != "/step enum; /step const" {
		t.Errorf("the documents of the step review with image_destination are refused for %q, want %q", r, "/step enum; /step const")
	}
}

// TestAPrebuildHasOneEnvironmentMember pins environment_destination and
// environment_image of documents.schema.json: the documents of the step workflow with
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
		if !ok || documents["step"] != "workflow" || documents[c.member] == nil {
			t.Errorf("%s are not run documents of the step workflow with %s", c.fixture, c.member)
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
	target := &contracts.Target{Collection: "R_1001", Address: &contracts.TargetAddress{System: "code.example.com", Path: "acme/shop"}}
	largestImage := int64(5368709120)
	ads := &contracts.Target{Collection: "AP_1234"}
	item := contracts.Subject{Kind: "work_item", ID: "wi_1", URL: "https://control.example.com/work-items/wi_1", Title: "Checkout fails"}
	settings := &contracts.RunSettings{Runtime: "claude"}
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
			"hello":  {Access: "write", Plan: "build-plan", Tier: "thinking"},
			"review": {},
			"verify": {Access: "write-no-push", Tier: "thinking", ReviewGroup: "verify-reviewers", RunsLockedChecks: true},
		},
	}}
	definedBare := &contracts.HarnessDocument{Harness: contracts.HarnessSection{
		Modules: []contracts.HarnessModule{{Source: contracts.HarnessSource{Git: source, Ref: "v2.4.0"}}},
	}}
	step := contracts.Deliver{Type: "code-change", Send: "end_of_run"}
	values := []struct {
		schema string
		v      any
	}{
		{"documents/target", target},
		{"documents/target", ads},
		{"documents/subject", item},
		{"documents/subject", contracts.Subject{Kind: "scheduled_job", ID: "nightly-report"}},
		{"documents/run_settings", contracts.RunSettings{Runtime: "claude"}},
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
		{"documents", contracts.Documents{Step: "prebuild", Domain: "software",
			Inputs: json.RawMessage(`{"version":1,"environment_ref":"env-3f2a91c4"}`), Access: "read", Target: target,
			ImageDestination:          "registry.example.com/targets/0123456789abcdef",
			EnvironmentDestination:    "registry.example.com/environments/0123456789abcdef",
			LargestPrebuiltImageBytes: &largestImage}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), Subject: &item, RunSettings: settings,
			Workflow: "hello", HarnessHidden: &hidden, Harness: harness}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), Subject: &item, RunSettings: settings,
			Workflow: "hello", HarnessHidden: &hidden, Harness: harness,
			TimeLimitMinutes: 1440, SharedRun: true,
			TypeSettings: json.RawMessage(`{"full_suite":true}`)}},
		{"documents/harness", harness},
		{"documents/harness", full},
		{"documents/harness", bare},
		{"documents/harness", defined},
		{"documents/harness", definedBare},
		{"documents", contracts.Documents{Step: "workflow", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "read", Target: target,
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", Subject: &item, RunSettings: settings, Workflow: "hello",
			HarnessHidden: &hidden}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), Access: "read", Target: ads, Subject: &item, RunSettings: settings,
			Workflow: "hello", HarnessHidden: &shown, Harness: harness}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "marketing",
			Inputs: json.RawMessage(`{"version":1}`), Subject: &item, RunSettings: settings,
			Workflow: "hello", HarnessHidden: &hidden, Harness: full}},
		{"documents/read_targets", []contracts.Target{*ads, {Collection: "R_1002", Address: &contracts.TargetAddress{System: "code.example.com", Path: "acme/api"}}}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target,
			ReadTargets:   []contracts.Target{{Collection: "R_1002", Address: &contracts.TargetAddress{System: "code.example.com", Path: "acme/api"}}, *ads},
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", Subject: &item, RunSettings: settings, Workflow: "build",
			HarnessHidden: &shown, Deliver: []contracts.Deliver{step}}},
		{"documents/deliver_entry", step},
		{"documents/deliver_entry", contracts.Deliver{Type: "customer-facts", Send: "after_gate"}},
		{"documents/deliver", []contracts.Deliver{step, {Type: "customer-facts", Send: "after_gate"}}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target,
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", Subject: &item, RunSettings: settings, Workflow: "build",
			HarnessHidden: &shown, Deliver: []contracts.Deliver{step}}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target, Subject: &item, RunSettings: settings,
			Workflow: "build", HarnessHidden: &shown, Harness: harness, Deliver: []contracts.Deliver{step}}},
		{"documents", contracts.Documents{Step: "workflow", Domain: "software",
			Inputs: json.RawMessage(`{"version":1}`), Access: "write", Target: target,
			TargetVersion: "0123456789abcdef0123456789abcdef01234567", Subject: &item, RunSettings: settings, Workflow: "build",
			HarnessHidden: &shown, Deliver: []contracts.Deliver{step}}},
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

// TestAPrebuildIsRefusedForItsSubjectAlone pins that the invalid run documents of the
// step prebuild that have subject, whose refusal contains no keyword, are refused for
// subject and for nothing else: the same documents without subject are accepted.
func TestAPrebuildIsRefusedForItsSubjectAlone(t *testing.T) {
	for _, c := range []struct{ fixture, schema string }{
		{"documents-prebuild-with-subject.json", "documents"},
	} {
		doc, err := contracts.RunDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		answer, ok := doc.(map[string]any)
		if !ok || answer["step"] != "prebuild" || answer["subject"] == nil {
			t.Errorf("%s are not run documents of the step prebuild with subject", c.fixture)
			continue
		}
		delete(answer, "subject")
		if err := compileRun(t, runSchema(c.schema)).Validate(answer); err != nil {
			t.Errorf("%s without subject is refused: %v", c.fixture, err)
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
// nothing else: a workflow run with run_settings replaced by only a runtime, and a
// prebuild run without run_settings, are accepted.
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
		if answer["step"] == "workflow" {
			answer["run_settings"] = map[string]any{"runtime": "claude"}
		} else {
			delete(answer, "run_settings")
		}
		if err := schema.Validate(answer); err != nil {
			t.Errorf("%s with the least run_settings is refused: %v", f, err)
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

// TestTheRunDocumentsReadIntoTheirGoTypes pins that the target, subject,
// run_settings, harness and deliver of each run documents fixture read into
// their Go types and are written back as the fixture's bytes, compacted, that a fixture
// without target has no access and no target_version either, that a fixture with workflow
// has harness or target_version, that at least one fixture has a harness with bind, that
// at least one fixture has deliver with harness and one without, and that the fixtures
// with deliver have each value of send.
func TestTheRunDocumentsReadIntoTheirGoTypes(t *testing.T) {
	n, harnesses, binds, reads := 0, 0, 0, 0
	delivers := map[bool]int{}
	sends := map[string]int{}
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
		if len(d.ReadTargets) > 0 {
			reads++
		}
		if d.Deliver != nil {
			delivers[d.Harness != nil]++
			for _, e := range d.Deliver {
				sends[e.Send]++
			}
		}
		for name, v := range map[string]any{
			"target":       &contracts.Target{},
			"read_targets": &[]contracts.Target{},
			"subject":      &contracts.Subject{},
			"run_settings": &contracts.RunSettings{},
			"harness":      &contracts.HarnessDocument{},
			"deliver":      &[]contracts.Deliver{},
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
		t.Error("fixtures/documents has no target, subject, run_settings or harness")
	}
	if harnesses == 0 {
		t.Error("fixtures/documents has no harness")
	}
	if binds == 0 {
		t.Error("fixtures/documents has no harness with bind")
	}
	if reads == 0 {
		t.Error("fixtures/documents has no document with read_targets")
	}
	if delivers[true] == 0 || delivers[false] == 0 {
		t.Errorf("fixtures/documents has %d documents with deliver and harness, and %d with deliver and no harness; want 1 or more of each",
			delivers[true], delivers[false])
	}
	if sends["end_of_run"] == 0 || sends["after_gate"] == 0 {
		t.Errorf("fixtures/documents has %d documents with the send end_of_run, and %d with after_gate; want 1 or more of each",
			sends["end_of_run"], sends["after_gate"])
	}
}

// TestABindTheSchemaRefusesFailsToRead pins that a bind with a kind whose value is null, a
// role whose value is null, a list on a commands or skills role, an empty list, a null
// item of a list, and a role set to neither a name nor a list fail to read into a
// HarnessSection with an error, and that a bind with each kind, with a list on an agents
// role, with empty objects, with a flat role <kind>/<role> or with a kind other than
// agents, commands and skills, which the reader ignores, reads without those members.
func TestABindTheSchemaRefusesFailsToRead(t *testing.T) {
	for _, c := range []struct{ in, err string }{
		{`{"bind":{"agents":null}}`, `set the bind kind "agents" to an object of roles to names, not null`},
		{`{"bind":{"skills":{"review":"brand-review"},"agents":null}}`, `set the bind kind "agents" to an object of roles to names, not null`},
		{`{"bind":{"commands":{"publish":["post","schedule"]}}}`, `set the bind role "commands.publish" to one name, not a list: only an agents role can be a group`},
		{`{"bind":{"skills":{"review":["brand-review"]}}}`, `set the bind role "skills.review" to one name, not a list: only an agents role can be a group`},
		{`{"bind":{"agents":{"reviewers":[]}}}`, `write one name or more in the list of the bind role "agents.reviewers": the list is empty`},
		{`{"bind":{"agents":{"writer":null}}}`, `set the bind role "agents.writer" to one name or a list of names, not null`},
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
		`{"bind":{"agents/writer":"copywriter"}}`,
		`{"bind":{"hooks":{"x":"y"},"agents":{"writer":"copywriter"}}}`,
		`{"bind":{"hooks":{"x":null}}}`,
	} {
		var s contracts.HarnessSection
		if err := json.Unmarshal([]byte(in), &s); err != nil {
			t.Errorf("%s: %v", in, err)
		}
		for kind := range s.Bind {
			if kind != "agents" && kind != "commands" && kind != "skills" {
				t.Errorf("%s reads with the bind kind %q", in, kind)
			}
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
	if s.Tiers == nil || s.Tiers.Light == nil || s.Tiers.Balanced == nil || s.Tiers.Thinking == nil ||
		s.Tiers.Light.Model != "example-small" || s.Tiers.Light.Effort != "" ||
		s.Tiers.Balanced.Model != "example-large" || s.Tiers.Balanced.Effort != "" ||
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

// TestASubmitModuleIsNoMemberOfAnyDocument pins that the run documents define no
// submit_module member, so a reader ignores it and the closed copy of the schema refuses
// it, and that the adapter's start-up defines none either: the submit program of a run with
// no run code is found in the composed harness, so neither document has a module for it.
func TestASubmitModuleIsNoMemberOfAnyDocument(t *testing.T) {
	module := map[string]any{"source": map[string]any{
		"git": "https://code.example.com/example-org/forge-module",
		"ref": strings.Repeat("9c", 20)}}
	valid, err := contracts.RunDocument("fixtures/documents/explore-deliver-with-harness.json")
	if err != nil {
		t.Fatal(err)
	}
	schema := compileRun(t, runSchema("documents"))
	if err := schema.Validate(valid); err != nil {
		t.Fatalf("explore-deliver-with-harness.json is refused: %v", err)
	}
	valid.(map[string]any)["submit_module"] = module
	if err := schema.Validate(valid); err != nil {
		t.Errorf("run documents with a submit_module are refused: %v", err)
	}
	closed, err := contracts.CompileRunClosed("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	v, ok := closed.Validate(valid).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("the closed copy accepts run documents with a submit_module")
	}
	if r := strings.Join(leaves(v), "; "); r != "/ additionalProperties" {
		t.Errorf("the closed copy refuses run documents with a submit_module for %q, want %q", r, "/ additionalProperties")
	}
	adapter, err := contracts.Document("fixtures/adapter/adapter/code-host.json")
	if err != nil {
		t.Fatal(err)
	}
	start, err := contracts.Compile("adapter.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := start.Validate(adapter); err != nil {
		t.Fatalf("code-host.json is refused: %v", err)
	}
	adapter.(map[string]any)["submit_module"] = module
	if err := start.Validate(adapter); err != nil {
		t.Errorf("an adapter start-up with a submit_module is refused: %v", err)
	}
	startClosed, err := contracts.CompileClosed("adapter.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	v, ok = startClosed.Validate(adapter).(*jsonschema.ValidationError)
	if !ok {
		t.Fatal("the closed copy accepts an adapter start-up with a submit_module")
	}
	if r := strings.Join(leaves(v), "; "); r != "/ additionalProperties" {
		t.Errorf("the closed copy refuses an adapter start-up with a submit_module for %q, want %q", r, "/ additionalProperties")
	}
}

// TestAWorkflowPlanAndTierFollowQorysHarnessFile pins plan and tier of a workflow in
// harness, with the grammar of qory's harness file: plan is lower-case letters and
// digits in groups joined by single hyphens, 1 to 64 characters, on a workflow with
// access write only; tier is light, balanced or thinking.
func TestAWorkflowPlanAndTierFollowQorysHarnessFile(t *testing.T) {
	workflow := compileRun(t, runSchema("documents/harness_workflow"))
	longest := strings.Repeat("a", 31) + "-" + strings.Repeat("b", 32)
	for name, doc := range map[string]map[string]any{
		"plan build-plan":            {"access": "write", "plan": "build-plan"},
		"plan of 64 with a hyphen":   {"access": "write", "plan": longest},
		"tier light":                 {"tier": "light"},
		"tier balanced":              {"tier": "balanced"},
		"tier thinking":              {"tier": "thinking"},
		"plan and tier on one write": {"access": "write", "plan": "build-plan", "tier": "thinking"},
	} {
		if err := workflow.Validate(doc); err != nil {
			t.Errorf("%s is refused: %v", name, err)
		}
	}
	for name, doc := range map[string]map[string]any{
		"plan Build_Plan":                {"access": "write", "plan": "Build_Plan"},
		"plan empty":                     {"access": "write", "plan": ""},
		"plan of 65":                     {"access": "write", "plan": longest + "c"},
		"plan with a double hyphen":      {"access": "write", "plan": "build--plan"},
		"plan with access read":          {"access": "read", "plan": "build-plan"},
		"plan with access write-no-push": {"access": "write-no-push", "plan": "build-plan"},
		"plan with no access":            {"plan": "build-plan"},
		"tier fast":                      {"tier": "fast"},
		"tier Thinking":                  {"tier": "Thinking"},
	} {
		if workflow.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
	}
}

// TestAWorkflowReviewGroupAndLockedChecksFollowQorysHarnessFile pins review_group and
// runs_locked_checks of a workflow in harness, with the grammar of qory's harness file:
// review_group is one path segment with no slash, backslash, @ or space and no leading
// dot, and runs_locked_checks is a boolean.
func TestAWorkflowReviewGroupAndLockedChecksFollowQorysHarnessFile(t *testing.T) {
	workflow := compileRun(t, runSchema("documents/harness_workflow"))
	for name, doc := range map[string]map[string]any{
		"group reviewers":             {"review_group": "reviewers"},
		"group with a hyphen":         {"review_group": "verify-reviewers"},
		"group with a dot inside":     {"review_group": "team.reviewers"},
		"group of one character":      {"review_group": "r"},
		"locked checks true":          {"runs_locked_checks": true},
		"locked checks false":         {"runs_locked_checks": false},
		"group and locked checks":     {"access": "write-no-push", "tier": "thinking", "review_group": "verify-reviewers", "runs_locked_checks": true},
		"group with a leading hyphen": {"review_group": "-reviewers"},
		"group with a plus":           {"review_group": "reviewers+"},
	} {
		if err := workflow.Validate(doc); err != nil {
			t.Errorf("%s is refused: %v", name, err)
		}
	}
	for name, doc := range map[string]map[string]any{
		"group empty":              {"review_group": ""},
		"group with a slash":       {"review_group": "a/b"},
		"group with a backslash":   {"review_group": `a\b`},
		"group with an at sign":    {"review_group": "a@b"},
		"group with a space":       {"review_group": "a b"},
		"group with a leading dot": {"review_group": ".reviewers"},
		"group a number":           {"review_group": json.Number("1")},
		"group a list":             {"review_group": []any{"reviewers"}},
		"locked checks a string":   {"runs_locked_checks": "true"},
		"locked checks a number":   {"runs_locked_checks": json.Number("1")},
		"locked checks null":       {"runs_locked_checks": nil},
	} {
		if workflow.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
	}
}

// TestAHarnessSelectionWithOnlyAnUndefinedMemberIsWrittenBackEmpty pins that a selection
// whose only member is undefined reads as empty, and that Go writes it back as {}, which
// the schema refuses because its minProperties is 1.
func TestAHarnessSelectionWithOnlyAnUndefinedMemberIsWrittenBackEmpty(t *testing.T) {
	var m contracts.HarnessModule
	in := `{"source":{"git":"https://example.com/m.git","ref":"v1"},"only":{"zz":1},"exclude":{"zz":1}}`
	if err := json.Unmarshal([]byte(in), &m); err != nil {
		t.Fatal(err)
	}
	if m.Only == nil || m.Exclude == nil {
		t.Fatalf("only and exclude are %v and %v, want both set and empty", m.Only, m.Exclude)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"only":{}`, `"exclude":{}`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("%s does not contain %s", out, want)
		}
	}
}

// TestASelectionOfOnlyAnUndefinedMemberIsAcceptedAndReadAsEmpty pins the ruling for
// `exclude: {"zz": 1}` and `only: {"zz": 1}` of a harness module in the run documents.
// The schema accepts each, Go reads each as a set, empty selection, and the closed copy
// refuses each at the selection.
func TestASelectionOfOnlyAnUndefinedMemberIsAcceptedAndReadAsEmpty(t *testing.T) {
	closed, err := contracts.CompileRunClosed("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"exclude", "only"} {
		b, err := fs.ReadFile(contracts.RunFS, "fixtures/closed/documents-harness-module-"+member+"-undefined-member.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := validate(compileRun(t, "documents.schema.json"), b); err != nil {
			t.Errorf("%s: the schema refuses the document: %v", member, err)
		}
		if err := validate(closed, b); err == nil {
			t.Errorf("%s: the closed copy accepts the document", member)
		}
		var d contracts.Documents
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		m := d.Harness.Harness.Modules[0]
		sel := m.Only
		if member == "exclude" {
			sel = m.Exclude
		}
		if sel == nil || !reflect.DeepEqual(*sel, contracts.HarnessSelection{}) {
			t.Errorf("%s: Go reads %+v, want a set, empty selection", member, sel)
		}
	}
}
