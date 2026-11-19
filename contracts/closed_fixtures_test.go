package contracts_test

import (
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// closedFixture is a document that its schema accepts, since a reader ignores a member
// that the schema does not define, and that the closed copy of the schema refuses at
// reason: the JSON Pointer of the object with the member, and the keyword. A writer's test
// validates against the closed copy, so each fixture is a test of the closed copy.
type closedFixture struct{ schema, reason string }

// closedRunFixtures are the fixtures of run/v1/fixtures/closed, by file name.
var closedRunFixtures = map[string]closedFixture{
	"documents-harness-bind-flat.json":                       {"documents", "/harness/harness/bind additionalProperties"},
	"documents-harness-bind-unknown-kind.json":               {"documents", "/harness/harness/bind additionalProperties"},
	"documents-harness-machine-setting-effort.json":          {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-force.json":           {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-home.json":            {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-launch.json":          {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-links.json":           {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-model.json":           {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-runtime.json":         {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-tiers.json":           {"documents", "/harness/harness additionalProperties"},
	"documents-harness-machine-setting-update.json":          {"documents", "/harness/harness additionalProperties"},
	"documents-harness-member-env.json":                      {"documents", "/harness additionalProperties"},
	"documents-harness-member-exports.json":                  {"documents", "/harness additionalProperties"},
	"documents-harness-member-git.json":                      {"documents", "/harness additionalProperties"},
	"documents-harness-member-target.json":                   {"documents", "/harness additionalProperties"},
	"documents-harness-member-worktree.json":                 {"documents", "/harness additionalProperties"},
	"documents-harness-module-link.json":                     {"documents", "/harness/harness/modules/0 additionalProperties"},
	"documents-harness-section-target.json":                  {"documents", "/harness/harness additionalProperties"},
	"documents-harness-workflow-time-limit.json":             {"documents", "/harness/harness/workflows/hello additionalProperties"},
	"documents-prebuild-inputs-target.json":                  {"documents", "/inputs additionalProperties"},
	"documents-run-settings-tier-fast.json":                  {"documents", "/run_settings/tiers additionalProperties"},
	"documents-run-settings-time-limit-minutes.json":         {"documents", "/run_settings additionalProperties"},
	"documents-run-settings-undefined-member.json":           {"documents", "/run_settings additionalProperties"},
	"documents-run-settings-unknown-tier.json":               {"documents", "/run_settings/tiers additionalProperties"},
	"documents-subject-undefined-member.json":                {"documents", "/subject additionalProperties"},
	"documents-submit-undefined-member.json":                 {"documents", "/submit additionalProperties"},
	"documents-undefined-member.json":                        {"documents", "/ additionalProperties"},
	"next-session-answer-model-choice-undefined-member.json": {"next-session-answer", "/model_choice additionalProperties"},
	"next-session-answer-model-credential.json":              {"next-session-answer", "/ additionalProperties"},
	"next-session-request-undefined-member.json":             {"next-session-request", "/ additionalProperties"},
	"next-session-request-verdict.json":                      {"next-session-request", "/ additionalProperties"},
	"outcome-action.json":                                    {"outcome", "/ additionalProperties"},
	"outcome-end-code-member.json":                           {"outcome", "/ additionalProperties"},
	"outcome-message.json":                                   {"outcome", "/ additionalProperties"},
	"next-session-answer-undefined-member.json":              {"next-session-answer", "/ additionalProperties"},
}

// closedDescriptionFixtures are the fixtures of integration/v1/fixtures/closed, by file
// name.
var closedDescriptionFixtures = map[string]closedFixture{
	"description-model-provider-api-undefined-member.json":           {"description", "/roles/model_provider/apis/0 additionalProperties"},
	"description-model-provider-endpoint-setting-and-path.json":      {"description", "/roles/model_provider/apis/0/endpoint additionalProperties"},
	"description-model-provider-model-undefined-member.json":         {"description", "/roles/model_provider/models/0 additionalProperties"},
	"description-model-provider-oauth-device-undefined-member.json":  {"description", "/roles/model_provider/apis/0/oauth_device additionalProperties"},
	"description-model-provider-oauth-refresh-undefined-member.json": {"description", "/roles/model_provider/apis/0/oauth_refresh additionalProperties"},
	"description-model-provider-role-auth.json":                      {"description", "/roles/model_provider additionalProperties"},
	"description-model-provider-role-endpoint.json":                  {"description", "/roles/model_provider additionalProperties"},
	"description-model-provider-role-formats.json":                   {"description", "/roles/model_provider additionalProperties"},
	"description-model-provider-role-hosts.json":                     {"description", "/roles/model_provider additionalProperties"},
	"description-model-provider-role-key-variables.json":             {"description", "/roles/model_provider additionalProperties"},
	"description-source-collection-noun-undefined-member.json":       {"description", "/roles/source/collection_noun additionalProperties"},
	"description-source-guide-undefined-member.json":                 {"description", "/roles/source/guide/setup additionalProperties"},
	"description-source-item-noun-undefined-member.json":             {"description", "/roles/source/item_noun additionalProperties"},
	"description-source-undefined-member.json":                       {"description", "/roles/source additionalProperties"},
}

// closedAdapterFixtures are the fixtures of integration/v1/fixtures/adapter/closed, by file
// name.
var closedAdapterFixtures = map[string]closedFixture{
	"changes-change-request-previous-unknown-member.json": {"changes", "/changes/0/previous additionalProperties"},
	"changes-change-request-with-url.json":                {"changes", "/changes/0 additionalProperties"},
	"changes-watch-with-ref.json":                         {"changes", "/changes/0 additionalProperties"},
	"changes-submitted-undefined-member.json":             {"changes-submitted", "/ additionalProperties"},
	"changes-failed-undefined-member.json":                {"changes-failed", "/ additionalProperties"},
	"adapter-undefined-member.json":                       {"adapter", "/ additionalProperties"},
	"adapter-sync-undefined-member.json":                  {"adapter", "/sync additionalProperties"},
	"source-items-listing-undefined-member.json":          {"source-items", "/listing additionalProperties"},
	"source-items-filed-by-undefined-member.json":         {"source-items", "/items/0/filed_by additionalProperties"},
}

// checkClosedFixtures pins that the files under dir are exactly the keys of want, that each
// validates against the open schema of its fixture, and that the closed copy refuses it at
// the reason of its fixture.
func checkClosedFixtures(t *testing.T, files []string, want map[string]closedFixture,
	read func(string) (any, error), open, closed func(string) (*jsonschema.Schema, error)) {
	t.Helper()
	if len(files) != len(want) {
		t.Errorf("the directory has %d fixtures, and the test expects %d", len(files), len(want))
	}
	for _, f := range files {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := read(f)
		if err != nil {
			t.Fatal(err)
		}
		o, err := open(w.schema + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := o.Validate(doc); err != nil {
			t.Errorf("%s is refused by %s: %v", f, w.schema, err)
		}
		c, err := closed(w.schema + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		v, ok := c.Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s is accepted by the closed copy of %s", f, w.schema)
			continue
		}
		if r := leaves(v); !slices.Contains(r, w.reason) {
			t.Errorf("%s is refused by the closed copy for %q, want %q among them", f, strings.Join(r, "; "), w.reason)
		}
	}
}

// TestTheClosedRunFixturesAreAcceptedAndRefusedByTheClosedCopy checks run/v1/fixtures/closed.
func TestTheClosedRunFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, runFiles(t, "fixtures/closed"), closedRunFixtures,
		contracts.RunDocument, contracts.CompileRun, contracts.CompileRunClosed)
}

// TestTheClosedDescriptionFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// integration/v1/fixtures/closed.
func TestTheClosedDescriptionFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, treeFiles(t, contracts.FS, "fixtures/closed"), closedDescriptionFixtures,
		contracts.Document, contracts.Compile, contracts.CompileClosed)
}

// TestTheClosedAdapterFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// integration/v1/fixtures/adapter/closed.
func TestTheClosedAdapterFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, treeFiles(t, contracts.FS, "fixtures/adapter/closed"), closedAdapterFixtures,
		contracts.Document, contracts.Compile, contracts.CompileClosed)
}
