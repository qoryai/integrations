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
	"documents-harness-module-exclude-undefined-member.json": {"documents", "/harness/harness/modules/0/exclude additionalProperties"},
	"documents-harness-module-link.json":                     {"documents", "/harness/harness/modules/0 additionalProperties"},
	"documents-harness-module-only-undefined-member.json":    {"documents", "/harness/harness/modules/0/only additionalProperties"},
	"documents-harness-section-target.json":                  {"documents", "/harness/harness additionalProperties"},
	"documents-harness-workflow-time-limit.json":             {"documents", "/harness/harness/workflows/hello additionalProperties"},
	"documents-prebuild-inputs-target.json":                  {"documents", "/inputs additionalProperties"},
	"documents-run-settings-tier-fast.json":                  {"documents", "/run_settings/tiers additionalProperties"},
	"documents-run-settings-time-limit-minutes.json":         {"documents", "/run_settings additionalProperties"},
	"documents-run-settings-undefined-member.json":           {"documents", "/run_settings additionalProperties"},
	"documents-run-settings-unknown-tier.json":               {"documents", "/run_settings/tiers additionalProperties"},
	"documents-subject-undefined-member.json":                {"documents", "/subject additionalProperties"},
	"documents-submit-undefined-member.json":                 {"documents", "/ additionalProperties"},
	"documents-deliver-undefined-member.json":                {"documents", "/deliver/0 additionalProperties"},
	"documents-full-suite-top-level.json":                    {"documents", "/ additionalProperties"},
	"documents-undefined-member.json":                        {"documents", "/ additionalProperties"},
	"next-session-answer-full-suite-top-level.json":          {"next-session-answer", "/ additionalProperties"},
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
	"description-links-undefined-member.json":                        {"description", "/links/0 additionalProperties"},
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
	"changes-change-request-previous-undefined-member-only.json": {"changes", "/changes/0/previous additionalProperties"},
	"changes-change-request-previous-unknown-member.json":        {"changes", "/changes/0/previous additionalProperties"},
	"changes-change-request-with-url.json":                       {"changes", "/changes/0 additionalProperties"},
	"changes-watch-with-ref.json":                                {"changes", "/changes/0 additionalProperties"},
	"changes-submitted-undefined-member.json":                    {"changes-submitted", "/ additionalProperties"},
	"changes-failed-undefined-member.json":                       {"changes-failed", "/ additionalProperties"},
	"adapter-undefined-member.json":                              {"adapter", "/ additionalProperties"},
	"adapter-sync-undefined-member.json":                         {"adapter", "/sync additionalProperties"},
	"source-items-listing-undefined-member.json":                 {"source-items", "/listing additionalProperties"},
	"source-items-filed-by-undefined-member.json":                {"source-items", "/items/0/filed_by additionalProperties"},
	"adapter-secrets-undefined-member.json":                      {"adapter-secrets", "/ additionalProperties"},
	"adapter-secrets-answer-undefined-member.json":               {"adapter-secrets-answer", "/ additionalProperties"},
	"adapter-secrets-plaintext-undefined-member.json":            {"adapter-secrets-plaintext", "/ additionalProperties"},
	"connect-pass-undefined-member.json":                         {"connect-pass", "/ additionalProperties"},
	"grant-undefined-member.json":                                {"grant", "/ additionalProperties"},
	"grants-undefined-member.json":                               {"grants", "/ additionalProperties"},
	"read-undefined-member.json":                                 {"read", "/ additionalProperties"},
	"read-answer-undefined-member.json":                          {"read-answer", "/ additionalProperties"},
	"resolve-undefined-member.json":                              {"resolve", "/ additionalProperties"},
	"resolve-answer-undefined-member.json":                       {"resolve-answer", "/ additionalProperties"},
	"run-credential-undefined-member.json":                       {"run-credential", "/ additionalProperties"},
	"run-credential-answer-undefined-member.json":                {"run-credential-answer", "/ additionalProperties"},
	"source-collections-undefined-member.json":                   {"source-collections", "/ additionalProperties"},
	"source-item-comments-undefined-member.json":                 {"source-item-comments", "/ additionalProperties"},
	"adapter-secrets-sealed-undefined-member.json":               {"adapter-secrets", "/sealed additionalProperties"},
	"adapter-secrets-answer-sealed-undefined-member.json":        {"adapter-secrets-answer", "/sealed additionalProperties"},
	"adapter-secrets-plaintext-secret-undefined-member.json":     {"adapter-secrets-plaintext", "/secrets/0 additionalProperties"},
	"grant-account-undefined-member.json":                        {"grant", "/account additionalProperties"},
	"grants-grant-undefined-member.json":                         {"grants", "/grants/0 additionalProperties"},
	"source-collections-collection-undefined-member.json":        {"source-collections", "/collections/0 additionalProperties"},
	"resolve-answer-result-undefined-member.json":                {"resolve-answer", "/results/0 additionalProperties"},
	"source-item-comments-comment-undefined-member.json":         {"source-item-comments", "/comments/0 additionalProperties"},
	"source-item-comments-complete-undefined-member.json":        {"source-item-comments", "/complete additionalProperties"},
	"source-items-item-undefined-member.json":                    {"source-items", "/items/0 additionalProperties"},
	"connect-return-undefined-member.json":                       {"connect-return", "/ additionalProperties"},
	"connections-disconnect-undefined-member.json":               {"connections-disconnect", "/ additionalProperties"},
	"grant-end-with-pass-nonce.json":                             {"grant-end", "/ additionalProperties"},
	"read-public-with-grant-ref.json":                            {"read-public", "/ additionalProperties"},
	"refusal-supported-member.json":                              {"refusal", "/ additionalProperties"},
	"setup-pass-undefined-member.json":                           {"setup-pass", "/ additionalProperties"},
	"source-items-undefined-member.json":                         {"source-items", "/ additionalProperties"},
	"sources-disconnect-with-reason.json":                        {"sources-disconnect", "/ additionalProperties"},
}

// closedProvisionerFixtures are the fixtures of provisioner/v1/fixtures/closed, by file
// name.
var closedProvisionerFixtures = map[string]closedFixture{
	"claim-answer-machine-spec-undefined-member.json": {"claim-answer", "/machine_spec additionalProperties"},
	"claim-answer-run-id.json":                        {"claim-answer", "/ additionalProperties"},
	"claim-offers-undefined-member.json":              {"claim", "/offers/0 additionalProperties"},
	"claim-undefined-member.json":                     {"claim", "/ additionalProperties"},
	"dispatched-external-undefined-member.json":       {"dispatched", "/external additionalProperties"},
	"dispatched-undefined-member.json":                {"dispatched", "/ additionalProperties"},
	"machine-end-undefined-member.json":               {"machine-end", "/ additionalProperties"},
	"machine-log-undefined-member.json":               {"machine-log", "/ additionalProperties"},
	"refusal-undefined-member.json":                   {"refusal", "/ additionalProperties"},
	"release-with-retry.json":                         {"release", "/ additionalProperties"},
}

// closedMachinesFixtures are the fixtures of machines/v1/fixtures/closed, by file name.
var closedMachinesFixtures = map[string]closedFixture{
	"check-undefined-member.json":               {"check", "/ additionalProperties"},
	"check-answer-version.json":                 {"check-answer", "/ additionalProperties"},
	"check-answer-undefined-member.json":        {"check-answer", "/ additionalProperties"},
	"make-with-size.json":                       {"make", "/ additionalProperties"},
	"make-key.json":                             {"make", "/ additionalProperties"},
	"list-answer-machine-key.json":              {"list-answer", "/machines/0 additionalProperties"},
	"make-undefined-member.json":                {"make", "/ additionalProperties"},
	"make-answer-version.json":                  {"make-answer", "/ additionalProperties"},
	"make-answer-undefined-member.json":         {"make-answer", "/ additionalProperties"},
	"list-undefined-member.json":                {"list", "/ additionalProperties"},
	"list-answer-version.json":                  {"list-answer", "/ additionalProperties"},
	"list-answer-undefined-member.json":         {"list-answer", "/ additionalProperties"},
	"list-answer-machine-undefined-member.json": {"list-answer", "/machines/0 additionalProperties"},
	"log-undefined-member.json":                 {"log", "/ additionalProperties"},
	"log-answer-version.json":                   {"log-answer", "/ additionalProperties"},
	"log-answer-undefined-member.json":          {"log-answer", "/ additionalProperties"},
	"delete-undefined-member.json":              {"delete", "/ additionalProperties"},
	"delete-answer-version.json":                {"delete-answer", "/ additionalProperties"},
	"delete-answer-undefined-member.json":       {"delete-answer", "/ additionalProperties"},
	"error-version.json":                        {"error", "/ additionalProperties"},
	"error-undefined-member.json":               {"error", "/ additionalProperties"},
	"error-failure-undefined-member.json":       {"error", "/error additionalProperties"},
	"make-machine-spec-undefined-member.json":   {"make", "/machine_spec additionalProperties"},
	"check-answer-offers-undefined-member.json": {"check-answer", "/offers/0 additionalProperties"},
}

// closedExploreFixtures are the fixtures of explore/v1/fixtures/closed, by file name.
var closedExploreFixtures = map[string]closedFixture{
	"inputs-answers.json":                             {"inputs", "/ additionalProperties"},
	"inputs-changes.json":                             {"inputs", "/ additionalProperties"},
	"inputs-collection-ref.json":                      {"inputs", "/ticket additionalProperties"},
	"inputs-comment-by-undefined-member.json":         {"inputs", "/comments/0/by additionalProperties"},
	"inputs-comment-undefined-member.json":            {"inputs", "/comments/0 additionalProperties"},
	"inputs-facts.json":                               {"inputs", "/ additionalProperties"},
	"inputs-instruction-by-undefined-member.json":     {"inputs", "/instruction/by additionalProperties"},
	"inputs-instruction-undefined-member.json":        {"inputs", "/instruction additionalProperties"},
	"inputs-proposal.json":                            {"inputs", "/ additionalProperties"},
	"inputs-run-of.json":                              {"inputs", "/run additionalProperties"},
	"inputs-work-item-undefined-member.json":          {"inputs", "/work_item additionalProperties"},
	"inputs-destination-undefined-member.json":        {"inputs", "/destinations/0 additionalProperties"},
	"inputs-destination-target-undefined-member.json": {"inputs", "/destinations/0/target additionalProperties"},
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

// TestTheClosedProvisionerFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// provisioner/v1/fixtures/closed.
func TestTheClosedProvisionerFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, provisionerFiles(t, "fixtures/closed"), closedProvisionerFixtures,
		contracts.ProvisionerDocument, contracts.CompileProvisioner, contracts.CompileProvisionerClosed)
}

// TestTheClosedMachinesFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// machines/v1/fixtures/closed.
func TestTheClosedMachinesFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, treeFiles(t, contracts.MachinesFS, "fixtures/closed"), closedMachinesFixtures,
		contracts.MachinesDocument, contracts.CompileMachines, contracts.CompileMachinesClosed)
}

// TestTheClosedExploreFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// explore/v1/fixtures/closed.
func TestTheClosedExploreFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, treeFiles(t, contracts.ExploreFS, "fixtures/closed"), closedExploreFixtures,
		contracts.ExploreDocument, contracts.CompileExplore, contracts.CompileExploreClosed)
}

// TestTheClosedAdapterFixturesAreAcceptedAndRefusedByTheClosedCopy checks
// integration/v1/fixtures/adapter/closed.
func TestTheClosedAdapterFixturesAreAcceptedAndRefusedByTheClosedCopy(t *testing.T) {
	checkClosedFixtures(t, treeFiles(t, contracts.FS, "fixtures/adapter/closed"), closedAdapterFixtures,
		contracts.Document, contracts.Compile, contracts.CompileClosed)
}
