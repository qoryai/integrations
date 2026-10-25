package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"crypto/hpke"
	"encoding/hex"

	"github.com/qoryai/forager/accesskey"
	foragercontracts "github.com/qoryai/forager/contracts"
	"github.com/qoryai/integrations/contracts"
	"github.com/qoryai/integrations/internal/rawjson"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// adapterSchemas are the schemas of the adapter's requests and answers, and the
// definitions they share.
var adapterSchemas = []string{
	"definitions.schema.json",
	"adapter.schema.json",
	"source-collections.schema.json",
	"source-items.schema.json",
	"source-item-comments.schema.json",
	"sources-disconnect.schema.json",
	"connections-disconnect.schema.json",
	"changes.schema.json",
	"connect-pass.schema.json",
	"connect-return.schema.json",
	"setup-pass.schema.json",
	"refusal.schema.json",
	"adapter-secrets.schema.json",
	"adapter-secrets-answer.schema.json",
	"adapter-secrets-plaintext.schema.json",
	"grant.schema.json",
	"grants.schema.json",
	"resolve.schema.json",
	"resolve-answer.schema.json",
	"read.schema.json",
	"read-answer.schema.json",
	"run-credential.schema.json",
	"run-credential-answer.schema.json",
}

// compile compiles a schema of the contract, or fails the test.
func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.Compile(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// read reads a file of the contract, or fails the test.
func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := fs.ReadFile(contracts.FS, name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// validate validates JSON bytes against a schema.
func validate(s *jsonschema.Schema, b []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return err
	}
	return s.Validate(doc)
}

// TestAdapterFixturesValidate pins that each document under fixtures/adapter/<name>/ is
// accepted by <name>.schema.json, and that every schema of the adapter compiles.
func TestAdapterFixturesValidate(t *testing.T) {
	for _, name := range adapterSchemas {
		compile(t, name)
	}
	dirs, err := fs.ReadDir(contracts.FS, "fixtures/adapter")
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "invalid" || d.Name() == "signed" || d.Name() == "refused-by-control-plane" {
			continue
		}
		schema := compile(t, d.Name()+".schema.json")
		for _, f := range files(t, path.Join("fixtures/adapter", d.Name())) {
			seen++
			doc, err := contracts.Document(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
	if seen == 0 {
		t.Fatal("fixtures/adapter holds no fixture")
	}
}

// TestInvalidAdapterFixturesAreRefused pins that each document under
// fixtures/adapter/invalid fails its schema for the reason in its name, and for no
// other: where, the keyword, and the property a required one misses.
func TestInvalidAdapterFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"adapter-sync-without-sends-changes.json":              {"adapter", "/sync required sends_changes"},
		"adapter-without-description.json":                     {"adapter", "/ required description"},
		"adapter-description-version-2.json":                   {"adapter", "/description/version const"},
		"adapter-description-item-noun-upper-case.json":        {"adapter", "/description/roles/source/item_noun/one pattern"},
		"adapter-description-string.json":                      {"adapter", "/description type"},
		"adapter-description-without-roles.json":               {"adapter", "/description required roles"},
		"changes-sync-interval-30.json":                        {"changes", "/changes/0/interval minimum"},
		"changes-unknown-kind.json":                            {"changes", "/changes/0/kind enum"},
		"changes-work-item-status-linked.json":                 {"changes", "/changes/0/status enum"},
		"changes-work-item-url-http.json":                      {"changes", "/changes/0/url pattern"},
		"changes-without-revision.json":                        {"changes", "/ required revision"},
		"source-collections-no-listing.json":                   {"source-collections", "/ required listing"},
		"source-item-comments-person-kind-app.json":            {"source-item-comments", "/comments/0/author/kind enum"},
		"source-items-101-items.json":                          {"source-items", "/items maxItems"},
		"source-items-body-too-long.json":                      {"source-items", "/items/0/body maxLength"},
		"source-items-empty-label.json":                        {"source-items", "/items/0/labels/1 minLength"},
		"source-items-state-reason-too-long.json":              {"source-items", "/items/0/state_reason maxLength"},
		"source-items-time-fraction.json":                      {"source-items", "/time pattern"},
		"source-items-time-offset.json":                        {"source-items", "/time pattern"},
		"source-items-undefined-member.json":                   {"source-items", "/ additionalProperties"},
		"refusal-supported-member.json":                        {"refusal", "/ additionalProperties"},
		"source-items-url-http.json":                           {"source-items", "/items/0/url pattern"},
		"source-items-key-too-long.json":                       {"source-items", "/items/0/key maxLength"},
		"source-items-event-empty.json":                        {"source-items", "/items minItems"},
		"source-item-comments-empty-without-complete.json":     {"source-item-comments", "/comments minItems"},
		"source-items-truncated-labels-twice.json":             {"source-items", "/items/0/truncated uniqueItems"},
		"source-items-truncated-state-reason.json":             {"source-items", "/items/0/truncated/0 enum"},
		"source-items-truncated-unknown-field.json":            {"source-items", "/items/0/truncated/0 enum"},
		"sources-disconnect-with-reason.json":                  {"sources-disconnect", "/ additionalProperties"},
		"sources-disconnect-without-connection-id.json":        {"sources-disconnect", "/ required connection_id"},
		"connect-return-refused-without-reason.json":           {"connect-return", "/ required reason"},
		"connect-return-linked-with-reason.json":               {"connect-return", "/reason "},
		"connect-return-unknown-outcome.json":                  {"connect-return", "/outcome enum"},
		"connect-return-undefined-member.json":                 {"connect-return", "/ additionalProperties"},
		"connect-pass-without-connection-id.json":              {"connect-pass", "/ required connection_id"},
		"connections-disconnect-undefined-member.json":         {"connections-disconnect", "/ additionalProperties"},
		"connections-disconnect-connection-id-adapter-id.json": {"connections-disconnect", "/connection_id pattern"},
		"source-collections-part-10001.json":                   {"source-collections", "/listing/part maximum"},
		"source-collections-environment-ref-empty.json":        {"source-collections", "/collections/0/environment_ref minLength"},
		"source-collections-environment-ref-201.json":          {"source-collections", "/collections/0/environment_ref maxLength"},
		"source-collections-environment-ref-nul.json":          {"source-collections", "/collections/0/environment_ref pattern"},
		"source-collections-items-true.json":                   {"source-collections", "/collections/1/items const"},
		"source-collections-items-string.json":                 {"source-collections", "/collections/1/items const"},
		"source-collections-items-null.json":                   {"source-collections", "/collections/1/items const"},
		"source-items-comment-count-above-maximum.json":        {"source-items", "/items/0/comment_count maximum"},
		"source-items-title-nul.json":                          {"source-items", "/items/0/title pattern"},
		"source-items-created-at-leap-second.json":             {"source-items", "/items/0/created_at pattern"},
		"source-items-created-at-hour-24.json":                 {"source-items", "/items/0/created_at format"},
		"source-items-created-at-minute-60.json":               {"source-items", "/items/0/created_at format"},
		"source-items-url-user-information.json":               {"source-items", "/items/0/url pattern"},
		"source-items-url-bad-percent-escape.json":             {"source-items", "/items/0/url format"},
		"source-items-url-port-0.json":                         {"source-items", "/items/0/url pattern"},
		"source-items-url-port-65536.json":                     {"source-items", "/items/0/url pattern"},
		"setup-pass-without-nonce.json":                        {"setup-pass", "/ required nonce"},
		"setup-pass-undefined-member.json":                     {"setup-pass", "/ additionalProperties"},
		"setup-pass-version-2.json":                            {"setup-pass", "/version const"},
		"setup-pass-adapter-id-connection-id.json":             {"setup-pass", "/adapter_id pattern"},
		"adapter-setup-url-fragment.json":                      {"adapter", "/setup_url "},
		"adapter-setup-url-state.json":                         {"adapter", "/setup_url "},
		"adapter-setup-url-and-state.json":                     {"adapter", "/setup_url "},
		"adapter-setup-url-http.json":                          {"adapter", "/setup_url pattern"},
		"adapter-setup-url-bare-state.json":                    {"adapter", "/setup_url "},
		"adapter-setup-url-state-first.json":                   {"adapter", "/setup_url "},
		"adapter-setup-url-state-percent-encoded.json":         {"adapter", "/setup_url "},
		"adapter-call-url-http.json":                           {"adapter", "/call_url pattern"},
		"adapter-call-url-query.json":                          {"adapter", "/call_url pattern"},
		"adapter-call-url-without-final-slash.json":            {"adapter", "/call_url pattern"},
		"adapter-description-secrets-twice.json":               {"adapter", "/description/roles/source/secrets uniqueItems"},
		"grant-without-pass-nonce.json":                        {"grant", "/ required pass_nonce"},
		"grant-pass-nonce-21-characters.json":                  {"grant", "/pass_nonce pattern"},
		"grants-without-revision.json":                         {"grants", "/ required revision"},
		"changes-grant-status-taken-in.json":                   {"changes", "/changes/0/status enum"},
		"changes-grant-without-connection-id.json":             {"changes", "/changes/0 required connection_id"},
		"adapter-secrets-value-in-clear.json":                  {"adapter-secrets", "/sealed additionalProperties"},
		"adapter-secrets-without-server-key.json":              {"adapter-secrets", "/sealed required server_key"},
		"adapter-secrets-answer-suite-chacha20.json":           {"adapter-secrets-answer", "/sealed/suite const"},
		"adapter-secrets-answer-enc-31-bytes.json":             {"adapter-secrets-answer", "/sealed/enc pattern"},
		"adapter-secrets-plaintext-17-secrets.json":            {"adapter-secrets-plaintext", "/secrets maxItems"},
		"adapter-secrets-plaintext-name-upper-case.json":       {"adapter-secrets-plaintext", "/secrets/0/name pattern"},
		"adapter-secrets-plaintext-empty-value.json":           {"adapter-secrets-plaintext", "/secrets/0/value minLength"},
		"resolve-65-addresses.json":                            {"resolve", "/addresses maxItems"},
		"resolve-without-addresses.json":                       {"resolve", "/addresses minItems"},
		"resolve-address-space.json":                           {"resolve", "/addresses/0 pattern"},
		"resolve-address-twice.json":                           {"resolve", "/addresses uniqueItems"},
		"resolve-answer-covered-true.json":                     {"resolve-answer", "/results/1/covered const"},
		"resolve-answer-not-covered-with-grant-ref.json":       {"resolve-answer", "/results/1/grant_ref "},
		"resolve-answer-without-resource-ref.json":             {"resolve-answer", "/results/0 required resource_ref"},
		"resolve-answer-without-revision.json":                 {"resolve-answer", "/ required revision"},
		"read-path-leading-slash.json":                         {"read", "/path pattern"},
		"read-path-dot-dot.json":                               {"read", "/path pattern"},
		"read-path-empty-part.json":                            {"read", "/path pattern"},
		"read-at-space.json":                                   {"read", "/at pattern"},
		"read-without-at.json":                                 {"read", "/ required at"},
		"read-answer-content-nul.json":                         {"read-answer", "/content pattern"},
		"read-answer-without-resolved-at.json":                 {"read-answer", "/ required resolved_at"},
		"read-answer-content-and-not-found.json":               {"read-answer", "/content "},
		"read-answer-without-content-or-found.json":            {"read-answer", "/ required content"},
		"run-credential-access-admin.json":                     {"run-credential", "/access enum"},
		"run-credential-purpose-other.json":                    {"run-credential", "/purpose enum"},
		"run-credential-without-resources.json":                {"run-credential", "/resources minItems; /resources minItems"},
		"run-credential-work-two-resources.json":               {"run-credential", "/resources maxItems"},
		"run-credential-without-purpose.json":                  {"run-credential", "/ required purpose"},
		"run-credential-work-with-grant-ref.json":              {"run-credential", "/ "},
		"run-credential-harness-without-grant-ref.json":        {"run-credential", "/ required grant_ref"},
		"run-credential-work-without-connection-id.json":       {"run-credential", "/ required connection_id"},
		"run-credential-harness-with-connection-id.json":       {"run-credential", "/ "},
		"run-credential-harness-write.json":                    {"run-credential", "/access const"},
		"run-credential-answer-without-token.json":             {"run-credential-answer", "/credential required token"},
		"run-credential-answer-without-nested-version.json":    {"run-credential-answer", "/credential required version"},
		"run-credential-answer-apply-token.json":               {"run-credential-answer", "/credential/apply/0/token "},
		"run-credential-answer-host-star-dot.json":             {"run-credential-answer", "/credential/apply/0/hosts/0 "},
		"run-credential-answer-placeholders.json":              {"run-credential-answer", "/credential/placeholders "},
		"run-credential-answer-token-line-feed.json":           {"run-credential-answer", "/credential/token pattern"},
		"run-credential-answer-token-carriage-return.json":     {"run-credential-answer", "/credential/token pattern"},
		"run-credential-answer-token-nul.json":                 {"run-credential-answer", "/credential/token pattern"},
		"run-credential-answer-token-leading-space.json":       {"run-credential-answer", "/credential/token pattern"},
		"run-credential-answer-token-trailing-tab.json":        {"run-credential-answer", "/credential/token pattern"},
		"run-credential-answer-token-16385.json":               {"run-credential-answer", "/credential/token maxLength"},
		"run-credential-answer-expires-at-without-zone.json":   {"run-credential-answer", "/credential/expires_at format"},
	}
	got := files(t, "fixtures/adapter/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/adapter/invalid holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.Document(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compile(t, w.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// node walks a JSON pointer, "/a/b", from a schema document of the contract, or fails
// the test.
func node(t *testing.T, name, pointer string) map[string]any {
	t.Helper()
	doc, err := contracts.Document(name)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		m, ok := doc.(map[string]any)
		if !ok {
			t.Fatalf("%s#%s: %q is not in an object", name, pointer, k)
		}
		doc = m[k]
	}
	m, ok := doc.(map[string]any)
	if !ok {
		t.Fatalf("%s#%s is not a schema", name, pointer)
	}
	return m
}

// TestTheItemPatternsRefuseTheFormatFixtures pins that the pattern of an item's
// created_at and url, as source-items.schema.json refers to it, refuses the value of
// each invalid fixture that the format keyword refuses first, and accepts that value
// with the one fault fixed. TestInvalidAdapterFixturesAreRefused checks only the first
// reason, so without this test a pattern could stop refusing these values unnoticed.
func TestTheItemPatternsRefuseTheFormatFixtures(t *testing.T) {
	for name, c := range map[string]struct{ member, bad, good string }{
		"source-items-created-at-hour-24.json":     {"created_at", "2026-10-01T24:00:00Z", "2026-10-01T23:00:00Z"},
		"source-items-created-at-minute-60.json":   {"created_at", "2026-10-01T09:60:00Z", "2026-10-01T09:59:00Z"},
		"source-items-url-bad-percent-escape.json": {"url", "https://tracker.acme.example/TRK/TRK%2G1", "https://tracker.acme.example/TRK/TRK%2D1"},
	} {
		if b := read(t, "fixtures/adapter/invalid/"+name); !bytes.Contains(b, []byte(`"`+c.member+`": "`+c.bad+`"`)) {
			t.Errorf("%s contains no %s %q", name, c.member, c.bad)
		}
		member := node(t, "source-items.schema.json", "/$defs/item/properties/"+c.member)
		ref, _ := member["$ref"].(string)
		file, pointer, ok := strings.Cut(ref, "#")
		if !ok {
			t.Fatalf("source-items.schema.json: the item's %s refers to %q, not to a definition", c.member, ref)
		}
		pattern, ok := node(t, file, pointer)["pattern"].(string)
		if !ok {
			t.Fatalf("%s: the item's %s has no pattern", ref, c.member)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Fatalf("%s: the pattern of the item's %s does not compile: %v", ref, c.member, err)
		}
		if re.MatchString(c.bad) {
			t.Errorf("%s: the pattern of the item's %s accepts %q", name, c.member, c.bad)
		}
		if !re.MatchString(c.good) {
			t.Errorf("%s: the pattern of the item's %s refuses %q", name, c.member, c.good)
		}
	}
}

// TestTypesMarshalToTheirSchemas marshals one value of each request and answer type,
// with its optional members left out where it has them, and validates it against its
// schema, so the types and the schemas cannot drift apart.
func TestTypesMarshalToTheirSchemas(t *testing.T) {
	now := contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	interval := 600
	reason := "Fixed"
	listing := contracts.Listing{ID: "items-1", Part: 1, StartedAt: now, Complete: true}
	person := contracts.Person{Name: "Ada Example", Kind: "person"}
	target := &contracts.Target{System: "code.example.com", Path: "acme/shop"}
	noItems := false
	values := []struct {
		schema string
		v      any
	}{
		{"adapter", contracts.Adapter{Version: contracts.Version, Time: now, ConnectURL: "https://adapter.example.com/connect",
			Sync:        contracts.Sync{DefaultInterval: 900, MinimumInterval: 60, SendsChanges: true},
			Description: read(t, "fixtures/example-tracker.json")}},
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef",
			Listing: listing}},
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef",
			Listing: listing,
			Collections: []contracts.Collection{
				{Ref: "TRK", URL: "https://tracker.acme.example/TRK", Title: "Tracker"},
				{Ref: "SHOP", URL: "https://code.example.com/acme/shop", Title: "acme/shop", Target: target, EnvironmentRef: "env-3f2a91c4"},
				{Ref: "DOCS", URL: "https://code.example.com/acme/docs", Title: "acme/docs", Items: &noItems},
			}}},
		{"source-items", contracts.SourceItems{Version: contracts.Version, Time: now, CollectionRef: "TRK", Listing: &listing,
			Items: []contracts.Item{
				{Ref: "TRK-1", URL: "https://tracker.acme.example/TRK/TRK-1", Title: "Checkout fails", BodyFormat: "markdown",
					FiledBy: person, State: "open", CreatedAt: now, UpdatedAt: now},
				{Ref: "TRK-2", URL: "https://tracker.acme.example/TRK/TRK-2", Key: "TRK-2", Type: "bug", Title: "Prices without tax",
					Body: "Shown **without** tax.", BodyFormat: "markdown", FiledBy: person, Labels: []string{"bug"},
					State: "closed", StateReason: &reason, CommentCount: 3, CreatedAt: now, UpdatedAt: now, Target: target,
					Truncated: []string{"title", "body", "labels"}},
			}}},
		{"source-items", contracts.SourceItems{Version: contracts.Version, Time: now, CollectionRef: "TRK",
			Items: []contracts.Item{{Ref: "TRK-1", URL: "https://tracker.acme.example/TRK/TRK-1", Title: "Checkout fails",
				BodyFormat: "text", FiledBy: person, State: "open", CreatedAt: now, UpdatedAt: now}}}},
		{"source-item-comments", contracts.SourceItemComments{Version: contracts.Version, Time: now, ItemRef: "TRK-1",
			Complete: &contracts.CommentsComplete{Since: now},
			Comments: []contracts.Comment{
				{Ref: "C-1", URL: "https://tracker.acme.example/TRK/TRK-1#C-1", Author: person, Body: "Seen too.",
					BodyFormat: "text", CreatedAt: now, UpdatedAt: now},
				{Ref: "C-2", URL: "https://tracker.acme.example/TRK/TRK-1#C-2", Author: contracts.Person{Name: "Tracker bot", Kind: "automation"},
					BodyFormat: "text", CreatedAt: now, UpdatedAt: now, Deleted: true, Truncated: []string{"author.name"}},
			}}},
		{"sources-disconnect", contracts.SourcesDisconnect{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef", CollectionRef: "TRK"}},
		{"connections-disconnect", contracts.ConnectionsDisconnect{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef"}},
		{"changes", contracts.Changes{Version: contracts.Version, Revision: contracts.Revision, Cursor: "c_1"}},
		{"changes", contracts.Changes{Version: contracts.Version, Revision: contracts.Revision, Cursor: "c_2", Changes: []contracts.Change{
			{Kind: contracts.ChangeWorkItem, WorkItemID: "wi_1", Ref: "TRK-1", Status: "taken_in", URL: "https://control.example.com/work-items/wi_1"},
			{Kind: contracts.ChangeCollection, CollectionRef: "TRK", Status: "linked"},
			{Kind: contracts.ChangeSync},
			{Kind: contracts.ChangeSync, Interval: &interval},
		}}},
		{"connect-pass", contracts.ConnectPass{Version: contracts.Version, AdapterID: "na_0123456789abcdef",
			ConnectionID: "ac_0123456789abcdef", Nonce: "AAECAwQFBgcICQoLDA0ODw", ExpiresAt: now}},
		{"connect-return", contracts.ConnectReturn{Version: contracts.Version, ConnectionID: "ac_0123456789abcdef",
			Nonce: "AAECAwQFBgcICQoLDA0ODw", Outcome: "linked", ExpiresAt: now}},
		{"connect-return", contracts.ConnectReturn{Version: contracts.Version, ConnectionID: "ac_0123456789abcdef",
			Nonce: "AAECAwQFBgcICQoLDA0ODw", Outcome: "refused", Reason: "permission_missing", ExpiresAt: now}},
		{"setup-pass", contracts.SetupPass{Version: contracts.Version, AdapterID: "na_0123456789abcdef",
			Nonce: "EBESExQVFhcYGRobHB0eHw", ExpiresAt: now}},
		{"adapter", contracts.Adapter{Version: contracts.Version, Time: now, ConnectURL: "https://adapter.example.com/connect",
			SetupURL: "https://adapter.example.com/setup", Sync: contracts.Sync{DefaultInterval: 900, MinimumInterval: 60},
			Description: read(t, "fixtures/example-tracker.json")}},
		{"adapter", contracts.Adapter{Version: contracts.Version, Time: now, ConnectURL: "https://adapter.example.com/connect",
			CallURL: "https://adapter.example.com/calls/", Sync: contracts.Sync{DefaultInterval: 900, MinimumInterval: 60},
			Description: read(t, "fixtures/example-tracker.json")}},
		{"refusal", contracts.Refusal{Error: "collection_not_linked"}},
		{"refusal", contracts.Refusal{Error: "grant_taken"}},
		{"refusal", contracts.Refusal{Error: "pass_unknown"}},
		{"adapter-secrets", contracts.AdapterSecrets{Version: contracts.Version, Time: now, Sealed: contracts.SealedToServer{
			Suite: contracts.SealSuite, ServerKey: "rcFAEfgtHFbZVqpPnXPYhYNhpgYEhSXg0Ixjjcdd2Mc", Enc: strings.Repeat("A", 43), CT: strings.Repeat("A", 22)}}},
		{"adapter-secrets-answer", contracts.AdapterSecretsAnswer{Version: contracts.Version, Revision: contracts.Revision,
			Sealed: contracts.SealedToAccessKey{Suite: contracts.SealSuite, AccessKeyID: "ak_f1xt0re000000000", Enc: strings.Repeat("A", 43), CT: strings.Repeat("A", 22)}}},
		{"adapter-secrets-plaintext", contracts.AdapterSecretsPlaintext{Version: contracts.Version}},
		{"adapter-secrets-plaintext", contracts.AdapterSecretsPlaintext{Version: contracts.Version,
			Secrets: []contracts.SecretEntry{{Name: "signing-key", Value: "a value"}}}},
		{"grant", contracts.Grant{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef", GrantRef: "G_1",
			PassNonce: "AAECAwQFBgcICQoLDA0ODw"}},
		{"grants", contracts.Grants{Version: contracts.Version, Revision: contracts.Revision}},
		{"grants", contracts.Grants{Version: contracts.Version, Revision: contracts.Revision,
			Grants: []contracts.GrantEntry{{ConnectionID: "ac_0123456789abcdef", GrantRef: "G_1"}}}},
		{"resolve", contracts.Resolve{Version: contracts.Version, Time: now, Addresses: []string{"https://code.example.com/acme/harness"}}},
		{"resolve-answer", contracts.ResolveAnswer{Version: contracts.Version, Revision: contracts.Revision, Results: []contracts.Resolved{
			{Address: "https://code.example.com/acme/harness", GrantRef: "G_1", ResourceRef: "R_1001"},
			{Address: "reader@code.example.com:acme/tools", Covered: &noItems},
		}}},
		{"run-credential", contracts.RunCredential{Version: contracts.Version, Time: now, GrantRef: "G_1", Resources: []string{"R_1001"},
			Access: "read", Purpose: "harness"}},
		{"run-credential", contracts.RunCredential{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef",
			Resources: []string{"C_2001"}, Access: "write", Purpose: "work"}},
		{"run-credential-answer", contracts.RunCredentialAnswer{Version: contracts.Version, Revision: contracts.Revision,
			Credential: contracts.Credential{Version: 1, Token: "fixture-access-token",
				Apply: []contracts.CredentialApply{{Hosts: []string{"code.example.com"}, Scheme: "bearer"}}}}},
		{"run-credential-answer", contracts.RunCredentialAnswer{Version: contracts.Version, Revision: contracts.Revision,
			Credential: contracts.Credential{Version: 1, Token: "fixture-access-token", ExpiresAt: "2026-10-15T13:00:00Z",
				Apply: []contracts.CredentialApply{
					{Hosts: []string{"code.example.com"}, Scheme: "basic", Username: "x-access", Paths: []string{"/acme/*"}},
					{Hosts: []string{"api.code.example.com"}, Scheme: "header", Header: "X-Access"},
				}}}},
		{"read", contracts.Read{Version: contracts.Version, Time: now, GrantRef: "G_1", ResourceRef: "R_1001", Path: "qory.yaml", At: "main"}},
		{"read-answer", contracts.ReadAnswer{Version: contracts.Version, Revision: contracts.Revision, Content: "",
			ResolvedAt: "3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4"}},
		{"read-answer", contracts.ReadAnswer{Version: contracts.Version, Revision: contracts.Revision, Found: new(bool),
			ResolvedAt: "3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4"}},
		{"changes", contracts.Changes{Version: contracts.Version, Revision: contracts.Revision, Cursor: "c_3", Changes: []contracts.Change{
			{Kind: contracts.ChangeGrant, ConnectionID: "ac_0123456789abcdef", GrantRef: "G_1", Status: "linked"},
		}}},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if err := validate(compile(t, c.schema+".schema.json"), b); err != nil {
			t.Errorf("%T marshals to %s, which %s.schema.json refuses: %v", c.v, b, c.schema, err)
		}
	}
	if _, err := json.Marshal(contracts.Change{Kind: "settings"}); err == nil {
		t.Error("a change of a kind the feed does not define marshals")
	}
}

// TestCollectionItemsRoundTrips decodes a listing whose second collection has items:
// false into SourceCollections, and pins that Items is nil where the listing leaves
// items out, points to false where it says false, and marshals back to a document that
// source-collections.schema.json accepts.
func TestCollectionItemsRoundTrips(t *testing.T) {
	var s contracts.SourceCollections
	if err := json.Unmarshal(read(t, "fixtures/adapter/source-collections/code-host-items-false.json"), &s); err != nil {
		t.Fatal(err)
	}
	if len(s.Collections) != 2 || s.Collections[0].Items != nil || s.Collections[1].Items == nil || *s.Collections[1].Items {
		t.Fatalf("the listing decodes to %+v", s.Collections)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"items":false`)) || bytes.Count(b, []byte(`"items"`)) != 1 {
		t.Errorf("the listing marshals to %s", b)
	}
	if err := validate(compile(t, "source-collections.schema.json"), b); err != nil {
		t.Errorf("the listing marshals to %s, which source-collections.schema.json refuses: %v", b, err)
	}
}

// TestAdapterDescriptionRoundTrips pins that an Adapter whose Description is the bytes
// of a description fixture marshals to a body adapter.schema.json accepts, whose
// description member is the fixture's document, and that an Adapter without a
// Description marshals the member as null, which adapter.schema.json refuses.
func TestAdapterDescriptionRoundTrips(t *testing.T) {
	description := read(t, "fixtures/example-tracker.json")
	a := contracts.Adapter{Version: contracts.Version, Time: contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)),
		ConnectURL: "https://adapter.example.com/connect", Sync: contracts.Sync{DefaultInterval: 900, MinimumInterval: 60},
		Description: description}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	schema := compile(t, "adapter.schema.json")
	if err := validate(schema, b); err != nil {
		t.Errorf("the adapter marshals to %s, which adapter.schema.json refuses: %v", b, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, description); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(doc["description"], want.Bytes()) {
		t.Errorf("the adapter marshals its description as %s, want %s", doc["description"], want.Bytes())
	}
	if err := contracts.CheckDescription(doc["description"]); err != nil {
		t.Errorf("the marshalled description breaks a rule of description.schema.json: %v", err)
	}
	a.Description = nil
	if b, err = json.Marshal(a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["description"]) != "null" {
		t.Errorf("an adapter without a description marshals it as %q, want null", doc["description"])
	}
	if validate(schema, b) == nil {
		t.Errorf("an adapter without a description marshals to %s, which adapter.schema.json accepts", b)
	}
}

// TestAdapterSetupURLRoundTrips decodes the start-up fixture with a setup_url into
// Adapter, and pins that SetupURL holds it and that it marshals back to a document
// adapter.schema.json accepts, with the same setup_url; and that an Adapter without a
// SetupURL marshals no setup_url member.
func TestAdapterSetupURLRoundTrips(t *testing.T) {
	const name = "fixtures/adapter/adapter/acme-tracker-setup.json"
	var a contracts.Adapter
	if err := json.Unmarshal(read(t, name), &a); err != nil {
		t.Fatal(err)
	}
	if a.SetupURL != "https://tracker-adapter.acme.example/setup?site=main" {
		t.Fatalf("%s decodes setup_url as %q", name, a.SetupURL)
	}
	schema := compile(t, "adapter.schema.json")
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(schema, b); err != nil {
		t.Errorf("%s marshals back to %s, which adapter.schema.json refuses: %v", name, b, err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["setup_url"]) != `"https://tracker-adapter.acme.example/setup?site=main"` {
		t.Errorf("%s marshals back setup_url as %s", name, doc["setup_url"])
	}
	a.SetupURL = ""
	if b, err = json.Marshal(a); err != nil {
		t.Fatal(err)
	}
	doc = nil
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["setup_url"]; ok {
		t.Errorf("an adapter without a setup URL marshals to %s, with a setup_url member", b)
	}
	if err := validate(schema, b); err != nil {
		t.Errorf("an adapter without a setup URL marshals to %s, which adapter.schema.json refuses: %v", b, err)
	}
}

// TestASetupURLIsRefusedForItsFragmentOrState pins the reason of each invalid start-up
// fixture whose setup_url breaks a "not" of adapter.schema.json, which the refusal
// itself does not name: the same document is accepted once the fragment is taken out, or
// the query parameter state, written plainly or percent-encoded, bare or with a value, is
// named otherwise.
func TestASetupURLIsRefusedForItsFragmentOrState(t *testing.T) {
	schema := compile(t, "adapter.schema.json")
	for name, fix := range map[string][2]string{
		"adapter-setup-url-fragment.json":              {"/setup#start", "/setup"},
		"adapter-setup-url-state.json":                 {"?state=x", "?next=x"},
		"adapter-setup-url-and-state.json":             {"&state=x", "&next=x"},
		"adapter-setup-url-bare-state.json":            {"?state\"", "?next\""},
		"adapter-setup-url-state-first.json":           {"?state&x=1", "?next&x=1"},
		"adapter-setup-url-state-percent-encoded.json": {"?%73tate=1", "?next=1"},
	} {
		b := read(t, "fixtures/adapter/invalid/"+name)
		if validate(schema, b) == nil {
			t.Errorf("%s is accepted", name)
		}
		fixed := bytes.Replace(b, []byte(fix[0]), []byte(fix[1]), 1)
		if bytes.Equal(fixed, b) {
			t.Errorf("%s holds no %q", name, fix[0])
		}
		if err := validate(schema, fixed); err != nil {
			t.Errorf("%s with %q for %q is refused: %v", name, fix[1], fix[0], err)
		}
	}
}

// TestASetupURLsStateCheckReadsTheQueryAlone pins which setup URLs adapter.schema.json
// refuses for a query parameter named state: the name in any position of the query,
// plain or percent-encoded, bare or with a value, and no other name, no state inside a
// value, and no state= in the path.
func TestASetupURLsStateCheckReadsTheQueryAlone(t *testing.T) {
	schema := compile(t, "adapter.schema.json")
	body := read(t, "fixtures/adapter/adapter/acme-tracker-setup.json")
	const fixture = "https://tracker-adapter.acme.example/setup?site=main"
	if !bytes.Contains(body, []byte(fixture)) {
		t.Fatalf("acme-tracker-setup.json holds no %s", fixture)
	}
	for url, refused := range map[string]bool{
		"https://adapter.example.com/setup?state=1":         true,
		"https://adapter.example.com/setup?state":           true,
		"https://adapter.example.com/setup?state&x=1":       true,
		"https://adapter.example.com/setup?x=1&state":       true,
		"https://adapter.example.com/setup?x=1&state=":      true,
		"https://adapter.example.com/setup?x=1&&state=1":    true,
		"https://adapter.example.com/setup?%73tate=1":       true,
		"https://adapter.example.com/setup?st%61te=1":       true,
		"https://adapter.example.com/setup?%73%74%61%74%65": true,
		"https://adapter.example.com/setup?xstate=1":        false,
		"https://adapter.example.com/setup?State=1":         false,
		"https://adapter.example.com/setup?states=1":        false,
		"https://adapter.example.com/setup?x=a?state=1":     false,
		"https://adapter.example.com/setup?x=state":         false,
		"https://adapter.example.com/p&state=1/setup":       false,
		"https://adapter.example.com/p&state=1/setup?x=1":   false,
	} {
		err := validate(schema, bytes.Replace(body, []byte(fixture), []byte(url), 1))
		if refused && err == nil {
			t.Errorf("a start-up with the setup_url %s is accepted", url)
		}
		if !refused && err != nil {
			t.Errorf("a start-up with the setup_url %s is refused: %v", url, err)
		}
	}
}

// TestSetupPassRoundTrips decodes setup-pass/payload.json into SetupPass and pins that it
// marshals back to the same bytes, compact, which setup-pass.schema.json accepts.
func TestSetupPassRoundTrips(t *testing.T) {
	const name = "fixtures/adapter/setup-pass/payload.json"
	var p contracts.SetupPass
	if err := json.Unmarshal(read(t, name), &p); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	if err := json.Compact(&want, read(t, name)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, want.Bytes()) {
		t.Errorf("%s marshals back to %s, want %s", name, b, want.Bytes())
	}
	if err := validate(compile(t, "setup-pass.schema.json"), b); err != nil {
		t.Errorf("%s marshals back to %s, which setup-pass.schema.json refuses: %v", name, b, err)
	}
}

// TestADescriptionsNumbersFollowTheDescriptionContract pins that the numbers of an
// adapter start-up's description member follow description.schema.json, not the integer
// rule: fixtures/adapter/adapter/settings-numbers.json, whose description's settings
// schema holds 1e3 and 0.5, is accepted by adapter.schema.json and by the strict check,
// its description member is accepted by CheckDescription, and the integer rule over the
// whole body refuses it. It also pins that every start-up fixture's description member
// is accepted by CheckDescription.
func TestADescriptionsNumbersFollowTheDescriptionContract(t *testing.T) {
	const name = "fixtures/adapter/adapter/settings-numbers.json"
	b := read(t, name)
	if err := validate(compile(t, "adapter.schema.json"), b); err != nil {
		t.Errorf("%s is refused by adapter.schema.json: %v", name, err)
	}
	if err := strictJSON("adapter", b); err != nil {
		t.Errorf("%s is refused by the strict check: %v", name, err)
	}
	if rawjson.Check(b, rawjson.Rules{Integers: true}) == nil {
		t.Errorf("%s holds no number with a fraction or an exponent", name)
	}
	for _, f := range files(t, "fixtures/adapter/adapter") {
		var body map[string]json.RawMessage
		if err := json.Unmarshal(read(t, f), &body); err != nil {
			t.Fatal(err)
		}
		if err := contracts.CheckDescription(body["description"]); err != nil {
			t.Errorf("%s: CheckDescription refuses the description member: %v", f, err)
		}
	}
}

// externalWords are the words of an external system no member name, const or enum value
// of the adapter's schemas contains; app may not be a whole value.
var externalWords = []string{"github", "issue", "pull", "repositor", "installation", "webhook", "workspace"}

// TestTheAdapterSchemasNameNoExternalSystem walks every schema of the adapter, those of
// the provisioner and explore contracts and description.schema.json among them, and fails on a member name, a required name, a
// const or an enum value that holds an external system's word, or is app. The fixtures'
// data are not checked.
func TestTheAdapterSchemasNameNoExternalSystem(t *testing.T) {
	check := func(name, where, word string) {
		w := strings.ToLower(word)
		if w == "app" {
			t.Errorf("%s: %s is %q", name, where, word)
		}
		for _, x := range externalWords {
			if strings.Contains(w, x) {
				t.Errorf("%s: %s %q holds %q", name, where, word, x)
			}
		}
	}
	var walk func(name string, v any)
	walk = func(name string, v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, sub := range v {
				switch k {
				case "properties":
					if props, ok := sub.(map[string]any); ok {
						for p := range props {
							check(name, "the member", p)
						}
					}
				case "required":
					for _, r := range sub.([]any) {
						check(name, "the required member", r.(string))
					}
				case "const":
					if s, ok := sub.(string); ok {
						check(name, "the const", s)
					}
				case "enum":
					for _, e := range sub.([]any) {
						if s, ok := e.(string); ok {
							check(name, "the enum value", s)
						}
					}
				}
				walk(name, sub)
			}
		case []any:
			for _, sub := range v {
				walk(name, sub)
			}
		}
	}
	for _, name := range append(adapterSchemas, "description.schema.json") {
		doc, err := contracts.Document(name)
		if err != nil {
			t.Fatal(err)
		}
		walk(name, doc)
	}
	for _, name := range provisionerSchemas {
		doc, err := contracts.ProvisionerDocument(name)
		if err != nil {
			t.Fatal(err)
		}
		walk("provisioner/v1/"+name, doc)
	}
	for _, name := range exploreSchemas {
		doc, err := contracts.ExploreDocument(name)
		if err != nil {
			t.Fatal(err)
		}
		walk("explore/v1/"+name, doc)
	}
}

// fixtureKeys are the keys of fixtures/adapter/signed/keys.json: the access key that
// signs the requests and the answer to a call, and the server key that signs the answer,
// the connect pass, the setup pass and the call.
func fixtureKeys(t *testing.T) (access *accesskey.Key, id, instance string, server *accesskey.Key) {
	t.Helper()
	var doc struct {
		Note      string `json:"note"`
		AccessKey struct {
			Secret      string `json:"secret"`
			AccessKeyID string `json:"access_key_id"`
			InstanceID  string `json:"instance_id"`
			PublicKey   string `json:"public_key"`
		} `json:"access_key"`
		ServerKey struct {
			Seed        string `json:"seed"`
			PublicKey   string `json:"public_key"`
			Fingerprint string `json:"fingerprint"`
		} `json:"server_key"`
	}
	b := read(t, "fixtures/adapter/signed/keys.json")
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "{\n  \"note\": \"For tests only. These keys grant nothing") {
		t.Error("keys.json does not say in its first field that it is for tests only and grants nothing")
	}
	access, err := accesskey.ParseSecret(doc.AccessKey.Secret)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := base64.RawURLEncoding.Strict().DecodeString(doc.ServerKey.Seed)
	if err != nil {
		t.Fatal(err)
	}
	server, err = accesskey.NewKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	if access.PublicKey().String() != doc.AccessKey.PublicKey || server.PublicKey().String() != doc.ServerKey.PublicKey ||
		server.Fingerprint() != doc.ServerKey.Fingerprint {
		t.Fatal("keys.json's public keys are not its secrets'")
	}
	if !access.PublicKey().Fixture() || !server.PublicKey().Fixture() {
		t.Error("keys.json holds a key accesskey does not name a fixture key")
	}
	return access, doc.AccessKey.AccessKeyID, doc.AccessKey.InstanceID, server
}

// signedRequest is a signed request of fixtures/adapter/signed.
type signedRequest struct {
	Method  string            `json:"method"`
	Target  string            `json:"target"`
	Headers map[string]string `json:"headers"`
	Body    *string           `json:"body"`
}

// request reads a signed request of fixtures/adapter/signed as accesskey signs it.
func request(t *testing.T, name string) (accesskey.Request, signedRequest) {
	t.Helper()
	var s signedRequest
	if err := json.Unmarshal(read(t, name), &s); err != nil {
		t.Fatal(err)
	}
	r := accesskey.Request{
		AccessKeyID: s.Headers[accesskey.HeaderAccessKeyID],
		InstanceID:  s.Headers[accesskey.HeaderInstanceID],
		Method:      s.Method,
		Target:      s.Target,
		Timestamp:   s.Headers[accesskey.HeaderTimestamp],
	}
	if s.Body != nil {
		r.Body = []byte(*s.Body)
	}
	if s.Headers["X-Qory-Contract-Version"] != strconv.Itoa(contracts.Revision) {
		t.Errorf("%s: X-Qory-Contract-Version is %q", name, s.Headers["X-Qory-Contract-Version"])
	}
	return r, s
}

// TestTheSignedRequestsVerify pins the signed POST of a source-items body and the signed
// GET of the change feed: each verifies under the fixture access key with
// accesskey.PublicKey.VerifyRequest, as its id and instance, and fails with one byte of
// its body or its timestamp changed; the POST's body is a source-items document whose
// time is its timestamp.
func TestTheSignedRequestsVerify(t *testing.T) {
	access, id, instance, _ := fixtureKeys(t)
	when := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

	post, s := request(t, "fixtures/adapter/signed/post-source-items.json")
	if post.Method != "POST" || post.Target != "/v1/source-items" || post.AccessKeyID != id || post.InstanceID != instance ||
		s.Headers["Content-Type"] != "application/json" {
		t.Errorf("post-source-items.json is not a POST of /v1/source-items as the fixture key: %+v", s)
	}
	if !access.PublicKey().VerifyRequest(post, s.Headers[accesskey.HeaderSignature]) {
		t.Error("post-source-items.json: the signature does not verify")
	}
	if err := validate(compile(t, "source-items.schema.json"), post.Body); err != nil {
		t.Errorf("post-source-items.json: the body: %v", err)
	}
	var body contracts.SourceItems
	if err := json.Unmarshal(post.Body, &body); err != nil || body.Time != contracts.FormatTime(when) {
		t.Errorf("post-source-items.json: the body's time is %q, want %s", body.Time, contracts.FormatTime(when))
	}
	tampered := post
	tampered.Body = bytes.Replace(post.Body, []byte("without"), []byte("withouT"), 1)
	if access.PublicKey().VerifyRequest(tampered, s.Headers[accesskey.HeaderSignature]) {
		t.Error("post-source-items.json verifies with a byte of its body changed")
	}

	get, s := request(t, "fixtures/adapter/signed/get-changes.json")
	if get.Method != "GET" || !strings.HasPrefix(get.Target, "/v1/work-items/changes?after=") || get.AccessKeyID != id ||
		get.InstanceID != instance || get.Body != nil || get.Timestamp != accesskey.Timestamp(when) {
		t.Errorf("get-changes.json is not a GET of the change feed as the fixture key at %s: %+v", accesskey.Timestamp(when), s)
	}
	if !access.PublicKey().VerifyRequest(get, s.Headers[accesskey.HeaderSignature]) {
		t.Error("get-changes.json: the signature does not verify")
	}
	late := get
	late.Timestamp = accesskey.Timestamp(when.Add(time.Second))
	if access.PublicKey().VerifyRequest(late, s.Headers[accesskey.HeaderSignature]) {
		t.Error("get-changes.json verifies with another timestamp")
	}
}

// TestTheSignedAnswerVerifies pins the answer to the GET of the change feed: a 200
// bound to the request's signature, whose body changes.schema.json accepts, verifies
// under a pin of the fixture server key with accesskey.Pin.VerifyAnswer, and fails
// bound to another signature.
func TestTheSignedAnswerVerifies(t *testing.T) {
	_, _, _, server := fixtureKeys(t)
	_, get := request(t, "fixtures/adapter/signed/get-changes.json")
	var doc struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/answer-changes.json"), &doc); err != nil {
		t.Fatal(err)
	}
	pin := accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}
	a := accesskey.Answer{Status: doc.Status, RequestSignature: get.Headers[accesskey.HeaderSignature], Body: []byte(doc.Body)}
	if doc.Status != 200 || !pin.VerifyAnswer(a, doc.Headers[accesskey.HeaderSignature]) {
		t.Error("answer-changes.json: the signature does not verify as a 200 to get-changes.json")
	}
	if err := validate(compile(t, "changes.schema.json"), a.Body); err != nil {
		t.Errorf("answer-changes.json: the body: %v", err)
	}
	a.RequestSignature = strings.Repeat("A", 86)
	if pin.VerifyAnswer(a, doc.Headers[accesskey.HeaderSignature]) {
		t.Error("answer-changes.json verifies bound to another request")
	}
}

// verifyConnectPass checks a connect pass's signature and its payload's schema, as
// verifySigned does over ConnectDomain and connect-pass.schema.json.
func verifyConnectPass(t *testing.T, pass string, pin accesskey.Pin) ([]byte, bool) {
	t.Helper()
	return verifySigned(t, pass, contracts.ConnectDomain, "connect-pass.schema.json", pin)
}

// verifySigned checks a signed value's signature and its payload's schema, and nothing
// else: at most ConnectPassMaxLength characters, the payload and the signature in strict
// base64url without padding split on the one dot, the signature verified under the pin
// over the domain line, a line feed and the payload bytes, and then the payload against
// the schema. It checks no id, no expiry and no nonce. It returns the payload's bytes.
func verifySigned(t *testing.T, signed, domain, schema string, pin accesskey.Pin) ([]byte, bool) {
	t.Helper()
	encoded, sig, ok := strings.Cut(signed, ".")
	if !ok || len(signed) > contracts.ConnectPassMaxLength || strings.Contains(sig, ".") {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, false
	}
	if !pin.Verify(append([]byte(domain+"\n"), payload...), sig) {
		return nil, false
	}
	if err := validate(compile(t, schema), payload); err != nil {
		t.Errorf("the payload of %s: %v", schema, err)
		return nil, false
	}
	return payload, true
}

// TestTheConnectPassVerifies pins the connect pass of fixtures/adapter/signed: it
// verifies under a pin of the fixture server key, beside another key as after a
// rotation, its payload is the fixture's, with its adapter_id and connection_id, and
// connect-pass.schema.json accepts it, and the same pass with one byte of its
// connection_id changed is refused.
func TestTheConnectPassVerifies(t *testing.T) {
	access, _, _, server := fixtureKeys(t)
	var doc struct {
		Payload  string `json:"payload"`
		Pass     string `json:"pass"`
		Tampered string `json:"tampered"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/connect-pass.json"), &doc); err != nil {
		t.Fatal(err)
	}
	pin := accesskey.Pin{
		{Alg: "ed25519", PublicKey: access.PublicKey().String()},
		{Alg: "ed25519", PublicKey: server.PublicKey().String()},
	}
	payload, ok := verifyConnectPass(t, doc.Pass, pin)
	if !ok || string(payload) != doc.Payload {
		t.Errorf("the connect pass does not verify to its payload: %q", payload)
	}
	var fixture bytes.Buffer
	if err := json.Compact(&fixture, read(t, "fixtures/adapter/connect-pass/payload.json")); err != nil || fixture.String() != doc.Payload {
		t.Errorf("the connect pass's payload is not connect-pass/payload.json")
	}
	var p contracts.ConnectPass
	if err := json.Unmarshal(payload, &p); err != nil || p.Version != contracts.Version ||
		p.AdapterID != "na_f1xt0re000000000" || p.ConnectionID != "ac_f1xt0re000000000" {
		t.Errorf("the connect pass's payload: %+v, %v", p, err)
	}
	if _, ok := verifyConnectPass(t, doc.Tampered, pin); ok {
		t.Error("the connect pass with a payload byte changed verifies")
	}
	passPayload, passSig, _ := strings.Cut(doc.Pass, ".")
	tamperedPayload, tamperedSig, _ := strings.Cut(doc.Tampered, ".")
	a, errA := base64.RawURLEncoding.Strict().DecodeString(passPayload)
	b, errB := base64.RawURLEncoding.Strict().DecodeString(tamperedPayload)
	changed := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			changed++
		}
	}
	if errA != nil || errB != nil || len(a) != len(b) || changed != 1 || tamperedSig != passSig {
		t.Error("tampered is not the pass with one payload byte changed")
	}
	var q contracts.ConnectPass
	if err := json.Unmarshal(b, &q); err != nil || q.AdapterID != p.AdapterID || q.ConnectionID == p.ConnectionID {
		t.Errorf("tampered does not change the pass's connection_id alone: %+v, %v", q, err)
	}
	if _, ok := verifyConnectPass(t, doc.Pass, accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}); ok {
		t.Error("the connect pass verifies under a pin without the server key")
	}
}

// verifySetupPass checks a setup pass's signature and its payload's schema, as
// verifySigned does over SetupDomain and setup-pass.schema.json, and that the pass has
// at most SetupPassMaxLength characters.
func verifySetupPass(t *testing.T, pass string, pin accesskey.Pin) ([]byte, bool) {
	t.Helper()
	if len(pass) > contracts.SetupPassMaxLength {
		return nil, false
	}
	return verifySigned(t, pass, contracts.SetupDomain, "setup-pass.schema.json", pin)
}

// signedPass is a signed pass of fixtures/adapter/signed: the payload, the pass, and
// the pass with one payload byte changed after signing.
type signedPass struct {
	Payload  string `json:"payload"`
	Pass     string `json:"pass"`
	Tampered string `json:"tampered"`
	Note     string `json:"note"`
}

// readSignedPass reads a signed pass of fixtures/adapter/signed.
func readSignedPass(t *testing.T, name string) signedPass {
	t.Helper()
	var doc signedPass
	if err := json.Unmarshal(read(t, name), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestTheSetupPassVerifies pins the setup pass of fixtures/adapter/signed: it verifies
// under a pin of the fixture server key, beside another key as after a rotation, its
// payload is setup-pass/payload.json, with its adapter_id, and setup-pass.schema.json
// accepts it; the same pass with one byte of its adapter_id changed is refused, and so
// is the pass under a pin without the server key.
func TestTheSetupPassVerifies(t *testing.T) {
	access, _, _, server := fixtureKeys(t)
	doc := readSignedPass(t, "fixtures/adapter/signed/setup-pass.json")
	pin := accesskey.Pin{
		{Alg: "ed25519", PublicKey: access.PublicKey().String()},
		{Alg: "ed25519", PublicKey: server.PublicKey().String()},
	}
	payload, ok := verifySetupPass(t, doc.Pass, pin)
	if !ok || string(payload) != doc.Payload {
		t.Errorf("the setup pass does not verify to its payload: %q", payload)
	}
	var fixture bytes.Buffer
	if err := json.Compact(&fixture, read(t, "fixtures/adapter/setup-pass/payload.json")); err != nil || fixture.String() != doc.Payload {
		t.Error("the setup pass's payload is not setup-pass/payload.json")
	}
	var p contracts.SetupPass
	if err := json.Unmarshal(payload, &p); err != nil || p.Version != contracts.Version || p.AdapterID != "na_f1xt0re000000000" {
		t.Errorf("the setup pass's payload: %+v, %v", p, err)
	}
	if _, ok := verifySetupPass(t, doc.Tampered, pin); ok {
		t.Error("the setup pass with a payload byte changed verifies")
	}
	passPayload, passSig, _ := strings.Cut(doc.Pass, ".")
	tamperedPayload, tamperedSig, _ := strings.Cut(doc.Tampered, ".")
	a, errA := base64.RawURLEncoding.Strict().DecodeString(passPayload)
	b, errB := base64.RawURLEncoding.Strict().DecodeString(tamperedPayload)
	changed := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			changed++
		}
	}
	if errA != nil || errB != nil || len(a) != len(b) || changed != 1 || tamperedSig != passSig {
		t.Error("tampered is not the pass with one payload byte changed")
	}
	var q contracts.SetupPass
	if err := json.Unmarshal(b, &q); err != nil || q.AdapterID == p.AdapterID || q.Nonce != p.Nonce || q.ExpiresAt != p.ExpiresAt {
		t.Errorf("tampered does not change the pass's adapter_id alone: %+v, %v", q, err)
	}
	if _, ok := verifySetupPass(t, doc.Pass, accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}); ok {
		t.Error("the setup pass verifies under a pin without the server key")
	}
}

// TestAPassVerifiesOverItsDomainAlone pins that the signed setup pass does not verify
// over ConnectDomain, and the signed connect pass does not verify over SetupDomain, under
// the same pin of the fixture server key that verifies each over its domain.
func TestAPassVerifiesOverItsDomainAlone(t *testing.T) {
	_, _, _, server := fixtureKeys(t)
	pin := accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}
	setup := readSignedPass(t, "fixtures/adapter/signed/setup-pass.json")
	connect := readSignedPass(t, "fixtures/adapter/signed/connect-pass.json")
	if _, ok := verifySetupPass(t, setup.Pass, pin); !ok {
		t.Error("the setup pass does not verify over SetupDomain")
	}
	if _, ok := verifyConnectPass(t, connect.Pass, pin); !ok {
		t.Error("the connect pass does not verify over ConnectDomain")
	}
	if _, ok := verifySigned(t, setup.Pass, contracts.ConnectDomain, "setup-pass.schema.json", pin); ok {
		t.Error("the setup pass verifies over the connect pass's domain")
	}
	if _, ok := verifySigned(t, connect.Pass, contracts.SetupDomain, "connect-pass.schema.json", pin); ok {
		t.Error("the connect pass verifies over the setup pass's domain")
	}
}

// TestTheConnectReturnVerifies pins the connect return of fixtures/adapter/signed:
// it verifies under the fixture access key's public half over ConnectReturnDomain, its
// payload is connect-return/linked.json and connect-return.schema.json accepts it, it
// verifies neither over ConnectDomain nor under the server key, and the same return with
// one byte of its connection_id changed is refused.
func TestTheConnectReturnVerifies(t *testing.T) {
	access, _, _, server := fixtureKeys(t)
	var doc struct {
		Payload  string `json:"payload"`
		Token    string `json:"token"`
		Tampered string `json:"tampered"`
		Note     string `json:"note"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/connect-return.json"), &doc); err != nil {
		t.Fatal(err)
	}
	pin := accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}
	payload, ok := verifySigned(t, doc.Token, contracts.ConnectReturnDomain, "connect-return.schema.json", pin)
	if !ok || string(payload) != doc.Payload {
		t.Errorf("the connect return does not verify to its payload: %q", payload)
	}
	var fixture bytes.Buffer
	if err := json.Compact(&fixture, read(t, "fixtures/adapter/connect-return/linked.json")); err != nil || fixture.String() != doc.Payload {
		t.Error("the connect return's payload is not connect-return/linked.json")
	}
	var r contracts.ConnectReturn
	if err := json.Unmarshal(payload, &r); err != nil || r.ConnectionID != "ac_f1xt0re000000000" || r.Outcome != "linked" || r.Reason != "" {
		t.Errorf("the connect return's payload: %+v, %v", r, err)
	}
	if _, ok := verifySigned(t, doc.Token, contracts.ConnectDomain, "connect-return.schema.json", pin); ok {
		t.Error("the connect return verifies over the connect pass's domain")
	}
	if _, ok := verifySigned(t, doc.Token, contracts.ConnectReturnDomain, "connect-return.schema.json",
		accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}); ok {
		t.Error("the connect return verifies under the server key")
	}
	if _, ok := verifySigned(t, doc.Tampered, contracts.ConnectReturnDomain, "connect-return.schema.json", pin); ok {
		t.Error("the connect return with a payload byte changed verifies")
	}
	returnPayload, returnSig, _ := strings.Cut(doc.Token, ".")
	tamperedPayload, tamperedSig, _ := strings.Cut(doc.Tampered, ".")
	a, errA := base64.RawURLEncoding.Strict().DecodeString(returnPayload)
	b, errB := base64.RawURLEncoding.Strict().DecodeString(tamperedPayload)
	changed := 0
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			changed++
		}
	}
	var q contracts.ConnectReturn
	if errA != nil || errB != nil || len(a) != len(b) || changed != 1 || tamperedSig != returnSig ||
		json.Unmarshal(b, &q) != nil || q.ConnectionID == r.ConnectionID {
		t.Error("tampered is not the return with one byte of its connection_id changed")
	}
}

// TestEmptyListsMarshalAsEmptyLists pins that a listing part, or a comment answer with
// complete, with nil or no items or comments marshals the list as [], which its schema
// accepts, and never as null or absent.
func TestEmptyListsMarshalAsEmptyLists(t *testing.T) {
	now := contracts.FormatTime(time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC))
	listing := &contracts.Listing{ID: "items-1", Part: 1, StartedAt: now, Complete: true}
	values := []struct {
		schema, member string
		v              any
	}{
		{"source-items", "items", contracts.SourceItems{Version: contracts.Version, Time: now, CollectionRef: "OPS", Listing: listing}},
		{"source-items", "items", contracts.SourceItems{Version: contracts.Version, Time: now, CollectionRef: "OPS", Listing: listing, Items: []contracts.Item{}}},
		{"source-item-comments", "comments", contracts.SourceItemComments{Version: contracts.Version, Time: now, ItemRef: "TRK-3",
			Complete: &contracts.CommentsComplete{Since: now}}},
		{"source-item-comments", "comments", contracts.SourceItemComments{Version: contracts.Version, Time: now, ItemRef: "TRK-3",
			Complete: &contracts.CommentsComplete{Since: now}, Comments: []contracts.Comment{}}},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(b, &doc); err != nil {
			t.Fatal(err)
		}
		if string(doc[c.member]) != "[]" {
			t.Errorf("%T marshals %s as %q, want []", c.v, c.member, doc[c.member])
		}
		if err := validate(compile(t, c.schema+".schema.json"), b); err != nil {
			t.Errorf("%T marshals to %s, which %s.schema.json refuses: %v", c.v, b, c.schema, err)
		}
	}
}

// strictJSON reports the first member an object repeats, at any depth, and the first
// number written with a fraction or an exponent: the two rules of definitions.schema.json
// that the control plane checks on a request's raw bytes, and that no schema can express,
// since a parsed document holds 900.0 as the integer 900 and keeps one member of a name.
// Every number of a request of the schema is an integer, except inside the description
// member of an adapter start-up, whose numbers follow description.schema.json: for the
// schema adapter, the integer rule is checked on the body without that member.
func strictJSON(schema string, b []byte) error {
	if schema != "adapter" {
		return rawjson.Check(b, rawjson.Rules{Integers: true})
	}
	if err := rawjson.Check(b, rawjson.Rules{}); err != nil {
		return err
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(b, &body); err != nil {
		return err
	}
	delete(body, "description")
	rest, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return rawjson.Check(rest, rawjson.Rules{Integers: true})
}

// TestTheControlPlaneRefusesWhatTheSchemaCannotSee pins the rule that an integer has no
// fraction or exponent and that no object repeats a member: each document under
// fixtures/adapter/refused-by-control-plane is accepted by its schema, since the rule is
// not expressible in one, and breaks the rule on its raw bytes, so the control plane
// refuses it, 400 invalid_request; and every accepted request fixture keeps the rule,
// the integer rule outside an adapter start-up's description member.
func TestTheControlPlaneRefusesWhatTheSchemaCannotSee(t *testing.T) {
	refused := map[string]string{
		"adapter-integer-with-fraction.json": "adapter",
		"adapter-repeated-member.json":       "adapter",
	}
	got := files(t, "fixtures/adapter/refused-by-control-plane")
	if len(got) != len(refused) {
		t.Errorf("fixtures/adapter/refused-by-control-plane holds %d documents, and the test expects %d", len(got), len(refused))
	}
	for _, f := range got {
		schema, ok := refused[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected schema", f)
			continue
		}
		b := read(t, f)
		if err := validate(compile(t, schema+".schema.json"), b); err != nil {
			t.Errorf("%s is refused by %s.schema.json, and the rule is the control plane's alone: %v", f, schema, err)
		}
		if strictJSON(schema, b) == nil {
			t.Errorf("%s keeps the rule on its raw bytes", f)
		}
	}
	for _, d := range []string{"adapter", "source-collections", "source-items", "source-item-comments", "sources-disconnect",
		"connections-disconnect", "connect-pass", "connect-return", "setup-pass", "adapter-secrets", "grant", "resolve",
		"resolve-answer", "read", "read-answer", "run-credential", "run-credential-answer"} {
		for _, f := range files(t, path.Join("fixtures/adapter", d)) {
			if err := strictJSON(d, read(t, f)); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
}

// TestACallURLIsRefusedForItsOneFault pins the reason of each invalid start-up fixture
// whose call_url breaks one of its two patterns, which the refusal does not tell apart:
// the same document is accepted once the one fault is fixed. It also pins that the
// start-up fixture with a call_url decodes into Adapter with CallURL set, and that an
// Adapter without a CallURL marshals no call_url member.
func TestACallURLIsRefusedForItsOneFault(t *testing.T) {
	schema := compile(t, "adapter.schema.json")
	for name, fix := range map[string][2]string{
		"adapter-call-url-http.json":                {"http://", "https://"},
		"adapter-call-url-query.json":               {"/calls/?site=main/", "/calls/"},
		"adapter-call-url-without-final-slash.json": {"/calls\"", "/calls/\""},
	} {
		b := read(t, "fixtures/adapter/invalid/"+name)
		if validate(schema, b) == nil {
			t.Errorf("%s is accepted", name)
		}
		fixed := bytes.Replace(b, []byte(fix[0]), []byte(fix[1]), 1)
		if bytes.Equal(fixed, b) {
			t.Errorf("%s holds no %q", name, fix[0])
		}
		if err := validate(schema, fixed); err != nil {
			t.Errorf("%s with %q for %q is refused: %v", name, fix[1], fix[0], err)
		}
	}
	var a contracts.Adapter
	if err := json.Unmarshal(read(t, "fixtures/adapter/adapter/code-host-call.json"), &a); err != nil {
		t.Fatal(err)
	}
	if a.CallURL != "https://code-adapter.example.com/calls/" {
		t.Errorf("code-host-call.json decodes call_url as %q", a.CallURL)
	}
	a.CallURL = ""
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"call_url"`)) {
		t.Errorf("an adapter without a call URL marshals to %s, with a call_url member", b)
	}
}

// signedCall is the signed call of fixtures/adapter/signed/call.json.
type signedCall struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// readCall reads the signed call of fixtures/adapter/signed/call.json as Call, and its
// signature.
func readCall(t *testing.T) (contracts.Call, string, signedCall) {
	t.Helper()
	var s signedCall
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/call.json"), &s); err != nil {
		t.Fatal(err)
	}
	c := contracts.Call{AdapterID: s.Headers[contracts.HeaderAdapterID], InstanceID: s.Headers[accesskey.HeaderInstanceID],
		Method: s.Method, Path: s.Path, Body: []byte(s.Body)}
	return c, s.Headers[accesskey.HeaderSignature], s
}

// callMessage returns the message of a call, or fails the test.
func callMessage(t *testing.T, c contracts.Call) []byte {
	t.Helper()
	m, err := c.Message()
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestTheSignedCallVerifies pins the signed call of fixtures/adapter/signed: a POST to
// the call_url of code-host-call.json followed by its path, for the adapter id and the
// instance id of the fixtures, whose body's time is its moment, verifies under a pin of
// the fixture server key, beside another key as after a rotation, and fails with one
// byte of its body, its adapter id, its instance id or its path changed, under a pin
// without the server key, and as a GET.
func TestTheSignedCallVerifies(t *testing.T) {
	access, _, instance, server := fixtureKeys(t)
	c, sig, s := readCall(t)
	var startup contracts.Adapter
	if err := json.Unmarshal(read(t, "fixtures/adapter/adapter/code-host-call.json"), &startup); err != nil {
		t.Fatal(err)
	}
	if c.Method != "POST" || s.URL != startup.CallURL+c.Path || c.AdapterID != "na_f1xt0re000000000" || c.InstanceID != instance ||
		s.Headers["Content-Type"] != "application/json" || s.Headers["X-Qory-Contract-Version"] != strconv.Itoa(contracts.Revision) {
		t.Errorf("call.json is not a POST to the call_url of code-host-call.json for the fixture adapter: %+v", s)
	}
	var body struct {
		Time string `json:"time"`
	}
	if err := json.Unmarshal(c.Body, &body); err != nil || body.Time != "2026-10-15T12:00:00Z" {
		t.Errorf("call.json: the body's time is %q, %v", body.Time, err)
	}
	pin := accesskey.Pin{
		{Alg: "ed25519", PublicKey: access.PublicKey().String()},
		{Alg: "ed25519", PublicKey: server.PublicKey().String()},
	}
	if !pin.Verify(callMessage(t, c), sig) {
		t.Error("call.json: the signature does not verify under the server key")
	}
	for what, changed := range map[string]contracts.Call{
		"a byte of its body":  {AdapterID: c.AdapterID, InstanceID: c.InstanceID, Method: c.Method, Path: c.Path, Body: bytes.Replace(c.Body, []byte("acme"), []byte("acmE"), 1)},
		"its adapter id":      {AdapterID: "na_f1xt0re000000001", InstanceID: c.InstanceID, Method: c.Method, Path: c.Path, Body: c.Body},
		"its instance id":     {AdapterID: c.AdapterID, InstanceID: "i_another", Method: c.Method, Path: c.Path, Body: c.Body},
		"its path":            {AdapterID: c.AdapterID, InstanceID: c.InstanceID, Method: c.Method, Path: "read", Body: c.Body},
		"its address as path": {AdapterID: c.AdapterID, InstanceID: c.InstanceID, Method: c.Method, Path: "/calls/resolve", Body: c.Body},
	} {
		if pin.Verify(callMessage(t, changed), sig) {
			t.Errorf("call.json verifies with %s changed", what)
		}
	}
	if (accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}).Verify(callMessage(t, c), sig) {
		t.Error("call.json verifies under a pin without the server key")
	}
	get := c
	get.Method = "GET"
	if _, err := get.Message(); err != contracts.ErrCallMethod {
		t.Errorf("a GET call has a message, %v", err)
	}
}

// TestTheSignedCallAnswerVerifies pins the answer of fixtures/adapter/signed to the
// signed call: a 200 bound to the call's signature verifies under the fixture access
// key's public half over CallAnswerDomain, and fails bound to another call, with another
// status, with one byte of its body changed, and under the server key.
func TestTheSignedCallAnswerVerifies(t *testing.T) {
	access, _, _, server := fixtureKeys(t)
	_, callSig, _ := readCall(t)
	var doc struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/call-answer.json"), &doc); err != nil {
		t.Fatal(err)
	}
	sig := doc.Headers[accesskey.HeaderSignature]
	pin := accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}
	a := contracts.CallAnswer{Status: doc.Status, CallSignature: callSig, Body: []byte(doc.Body)}
	if doc.Status != 200 || !pin.Verify(a.Message(), sig) {
		t.Error("call-answer.json: the signature does not verify as a 200 to call.json")
	}
	for what, changed := range map[string]contracts.CallAnswer{
		"bound to another call": {Status: a.Status, CallSignature: strings.Repeat("A", 86), Body: a.Body},
		"with another status":   {Status: 201, CallSignature: a.CallSignature, Body: a.Body},
		"with a byte changed":   {Status: a.Status, CallSignature: a.CallSignature, Body: bytes.Replace(a.Body, []byte("G_1"), []byte("G_2"), 1)},
	} {
		if pin.Verify(changed.Message(), sig) {
			t.Errorf("call-answer.json verifies %s", what)
		}
	}
	if (accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}).Verify(a.Message(), sig) {
		t.Error("call-answer.json verifies under the server key")
	}
}

// TestACallVerifiesOverItsDomainAlone pins that a call's signature verifies only as a
// call, and an answer to a call only as such an answer: the signed call does not verify
// as a request of the same fields, as an answer bound to it, or as a pass or a connect
// return of its body; the signed answer to it does not verify as accesskey's Answer; and
// neither the signed GET of the change feed nor its signed answer verifies as a call or
// as an answer to a call.
func TestACallVerifiesOverItsDomainAlone(t *testing.T) {
	access, _, _, server := fixtureKeys(t)
	serverPin := accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}
	accessPin := accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}
	c, sig, _ := readCall(t)
	asRequest, err := accesskey.Request{AccessKeyID: c.AdapterID, InstanceID: c.InstanceID, Method: c.Method, Target: c.Path, Body: c.Body}.Message()
	if err != nil {
		t.Fatal(err)
	}
	for what, m := range map[string][]byte{
		"a request":        asRequest,
		"an answer":        accesskey.Answer{Status: 200, RequestSignature: sig, Body: c.Body}.Message(),
		"a connect pass":   append([]byte(contracts.ConnectDomain+"\n"), c.Body...),
		"a setup pass":     append([]byte(contracts.SetupDomain+"\n"), c.Body...),
		"a connect return": append([]byte(contracts.ConnectReturnDomain+"\n"), c.Body...),
	} {
		if serverPin.Verify(m, sig) {
			t.Errorf("the signed call verifies as %s", what)
		}
	}
	var answer struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/call-answer.json"), &answer); err != nil {
		t.Fatal(err)
	}
	if accessPin.Verify(accesskey.Answer{Status: answer.Status, RequestSignature: sig, Body: []byte(answer.Body)}.Message(),
		answer.Headers[accesskey.HeaderSignature]) {
		t.Error("the signed answer to the call verifies as accesskey's Answer")
	}
	get, s := request(t, "fixtures/adapter/signed/get-changes.json")
	asCall := contracts.Call{AdapterID: get.AccessKeyID, InstanceID: get.InstanceID, Method: "POST", Path: get.Target, Body: []byte(get.Timestamp)}
	if accessPin.Verify(callMessage(t, asCall), s.Headers[accesskey.HeaderSignature]) {
		t.Error("the signed GET of the change feed verifies as a call")
	}
	var changes struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/answer-changes.json"), &changes); err != nil {
		t.Fatal(err)
	}
	if serverPin.Verify(contracts.CallAnswer{Status: changes.Status, CallSignature: s.Headers[accesskey.HeaderSignature],
		Body: []byte(changes.Body)}.Message(), changes.Headers[accesskey.HeaderSignature]) {
		t.Error("the signed answer of the change feed verifies as an answer to a call")
	}
}

// openSealed opens a sealed value of the fixtures with the X25519 key of an access key or
// a server key, under info and aad, as RFC 9180 base mode with the suite SealSuite.
func openSealed(t *testing.T, key *accesskey.Key, enc, ct string, info, aad []byte) ([]byte, error) {
	t.Helper()
	priv, err := hpke.NewDHKEMPrivateKey(key.X25519())
	if err != nil {
		t.Fatal(err)
	}
	e, errE := base64.RawURLEncoding.Strict().DecodeString(enc)
	c, errC := base64.RawURLEncoding.Strict().DecodeString(ct)
	if errE != nil || errC != nil {
		t.Fatalf("enc or ct is not strict base64url: %v, %v", errE, errC)
	}
	r, err := hpke.NewRecipient(e, priv, hpke.HKDFSHA256(), hpke.AES256GCM(), info)
	if err != nil {
		return nil, err
	}
	return r.Open(aad, c)
}

// TestTheSealedSecretsOpen pins the known answer of fixtures/adapter/signed/adapter-secrets.json:
// SecretsUploadInfo, SecretsInfo and SecretsAAD build its info and aad; the upload of
// adapter-secrets/upload.json opens with the fixture server key to its plaintext, and the
// delivery of adapter-secrets-answer/delivery.json with the fixture access key; the
// plaintext is adapter-secrets-plaintext/two-secrets.json and keeps every rule; and
// neither opens with the info of the other direction, with the aad of another adapter,
// or with the other recipient's key.
func TestTheSealedSecretsOpen(t *testing.T) {
	access, id, _, server := fixtureKeys(t)
	var kat struct {
		AdapterID    string `json:"adapter_id"`
		AccessKeyID  string `json:"access_key_id"`
		ServerKey    string `json:"server_key"`
		Plaintext    string `json:"plaintext"`
		UploadInfo   string `json:"upload_info"`
		DeliveryInfo string `json:"delivery_info"`
		AAD          string `json:"aad"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/adapter-secrets.json"), &kat); err != nil {
		t.Fatal(err)
	}
	if kat.AccessKeyID != id || kat.ServerKey != server.PublicKey().String() || kat.AdapterID != "na_f1xt0re000000000" {
		t.Fatalf("adapter-secrets.json is not for the keys of keys.json: %+v", kat)
	}
	uploadInfo := contracts.SecretsUploadInfo(kat.ServerKey)
	deliveryInfo := contracts.SecretsInfo(kat.AccessKeyID)
	aad := contracts.SecretsAAD(kat.AdapterID, kat.AccessKeyID)
	for name, c := range map[string]struct {
		got  []byte
		want string
	}{"upload_info": {uploadInfo, kat.UploadInfo}, "delivery_info": {deliveryInfo, kat.DeliveryInfo}, "aad": {aad, kat.AAD}} {
		if hex.EncodeToString(c.got) != c.want {
			t.Errorf("adapter-secrets.json: %s is %s, and the Go builder writes %x", name, c.want, c.got)
		}
	}
	var upload contracts.AdapterSecrets
	if err := json.Unmarshal(read(t, "fixtures/adapter/adapter-secrets/upload.json"), &upload); err != nil {
		t.Fatal(err)
	}
	var delivery contracts.AdapterSecretsAnswer
	if err := json.Unmarshal(read(t, "fixtures/adapter/adapter-secrets-answer/delivery.json"), &delivery); err != nil {
		t.Fatal(err)
	}
	if upload.Sealed.ServerKey != kat.ServerKey || delivery.Sealed.AccessKeyID != kat.AccessKeyID {
		t.Errorf("the envelopes are not sealed to the keys of keys.json: %+v, %+v", upload.Sealed, delivery.Sealed)
	}
	u, err := openSealed(t, server, upload.Sealed.Enc, upload.Sealed.CT, uploadInfo, aad)
	if err != nil || string(u) != kat.Plaintext {
		t.Errorf("the upload does not open with the server key to the plaintext: %q, %v", u, err)
	}
	d, err := openSealed(t, access, delivery.Sealed.Enc, delivery.Sealed.CT, deliveryInfo, aad)
	if err != nil || string(d) != kat.Plaintext {
		t.Errorf("the delivery does not open with the access key to the plaintext: %q, %v", d, err)
	}
	var fixture bytes.Buffer
	if err := json.Compact(&fixture, read(t, "fixtures/adapter/adapter-secrets-plaintext/two-secrets.json")); err != nil || fixture.String() != kat.Plaintext {
		t.Error("the plaintext is not adapter-secrets-plaintext/two-secrets.json")
	}
	if err := validate(compile(t, "adapter-secrets-plaintext.schema.json"), []byte(kat.Plaintext)); err != nil {
		t.Errorf("the plaintext: %v", err)
	}
	var p contracts.AdapterSecretsPlaintext
	if err := json.Unmarshal([]byte(kat.Plaintext), &p); err != nil || p.Check() != nil || len(p.Secrets) != 2 {
		t.Errorf("the plaintext breaks a rule: %v, %v", err, p.Check())
	}
	another := contracts.SecretsAAD("na_f1xt0re000000001", kat.AccessKeyID)
	for what, err := range map[string]error{
		"the upload with the delivery's info":   second(openSealed(t, server, upload.Sealed.Enc, upload.Sealed.CT, deliveryInfo, aad)),
		"the delivery with the upload's info":   second(openSealed(t, access, delivery.Sealed.Enc, delivery.Sealed.CT, uploadInfo, aad)),
		"the upload with another adapter id":    second(openSealed(t, server, upload.Sealed.Enc, upload.Sealed.CT, uploadInfo, another)),
		"the delivery with another adapter id":  second(openSealed(t, access, delivery.Sealed.Enc, delivery.Sealed.CT, deliveryInfo, another)),
		"the upload with the access key":        second(openSealed(t, access, upload.Sealed.Enc, upload.Sealed.CT, uploadInfo, aad)),
		"the delivery with the server key":      second(openSealed(t, server, delivery.Sealed.Enc, delivery.Sealed.CT, deliveryInfo, aad)),
		"the delivery's ct with the upload enc": second(openSealed(t, access, upload.Sealed.Enc, delivery.Sealed.CT, deliveryInfo, aad)),
	} {
		if err == nil {
			t.Errorf("%s opens", what)
		}
	}
}

// second returns the second of two values.
func second[A, B any](_ A, b B) B { return b }

// TestTheSignedSecretsAnswerVerifies pins the signed read of the adapter's secrets: the
// GET verifies under the fixture access key, and its 200 verifies under a pin of the
// fixture server key, bound to the GET's signature, with adapter-secrets-answer/delivery.json,
// compact, as its body.
func TestTheSignedSecretsAnswerVerifies(t *testing.T) {
	access, id, instance, server := fixtureKeys(t)
	get, s := request(t, "fixtures/adapter/signed/get-adapter-secrets.json")
	if get.Method != "GET" || get.Target != "/v1/adapter/secrets" || get.AccessKeyID != id || get.InstanceID != instance || get.Body != nil {
		t.Errorf("get-adapter-secrets.json is not a GET of /v1/adapter/secrets as the fixture key: %+v", s)
	}
	if !access.PublicKey().VerifyRequest(get, s.Headers[accesskey.HeaderSignature]) {
		t.Error("get-adapter-secrets.json: the signature does not verify")
	}
	var doc struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    string            `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/answer-adapter-secrets.json"), &doc); err != nil {
		t.Fatal(err)
	}
	pin := accesskey.Pin{{Alg: "ed25519", PublicKey: server.PublicKey().String()}}
	a := accesskey.Answer{Status: doc.Status, RequestSignature: s.Headers[accesskey.HeaderSignature], Body: []byte(doc.Body)}
	if doc.Status != 200 || !pin.VerifyAnswer(a, doc.Headers[accesskey.HeaderSignature]) {
		t.Error("answer-adapter-secrets.json: the signature does not verify as a 200 to get-adapter-secrets.json")
	}
	var want bytes.Buffer
	if err := json.Compact(&want, read(t, "fixtures/adapter/adapter-secrets-answer/delivery.json")); err != nil || want.String() != doc.Body {
		t.Error("answer-adapter-secrets.json: the body is not adapter-secrets-answer/delivery.json")
	}
}

// TestASecretsPlaintextHoldsTheRulesTheSchemaCannotSee pins AdapterSecretsPlaintext.Check:
// it refuses a name twice, an empty value, a value of more than 16384 bytes of UTF-8,
// which the schema accepts when it has at most 16384 characters, and a value with a NUL,
// and accepts a value of exactly 16384 bytes; and no message contains a value.
func TestASecretsPlaintextHoldsTheRulesTheSchemaCannotSee(t *testing.T) {
	long := strings.Repeat("\u00e9", 8193)
	schema := compile(t, "adapter-secrets-plaintext.schema.json")
	b, err := json.Marshal(contracts.AdapterSecretsPlaintext{Version: contracts.Version, Secrets: []contracts.SecretEntry{{Name: "signing-key", Value: long}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(schema, b); err != nil {
		t.Errorf("a value of 8193 characters and 16386 bytes is refused by the schema: %v", err)
	}
	for what, c := range map[string]struct {
		secrets []contracts.SecretEntry
		refused bool
	}{
		"a name twice":       {[]contracts.SecretEntry{{Name: "a", Value: "value-one"}, {Name: "a", Value: "value-two"}}, true},
		"an empty value":     {[]contracts.SecretEntry{{Name: "a", Value: ""}}, true},
		"16386 bytes":        {[]contracts.SecretEntry{{Name: "a", Value: long}}, true},
		"a NUL":              {[]contracts.SecretEntry{{Name: "a", Value: "value-one\x00"}}, true},
		"16384 bytes":        {[]contracts.SecretEntry{{Name: "a", Value: strings.Repeat("x", contracts.SecretValueMaxBytes)}}, false},
		"two names, no NULs": {[]contracts.SecretEntry{{Name: "a", Value: "value-one"}, {Name: "b", Value: "value-two"}}, false},
	} {
		err := contracts.AdapterSecretsPlaintext{Version: contracts.Version, Secrets: c.secrets}.Check()
		if c.refused && err == nil {
			t.Errorf("a plaintext with %s is accepted", what)
		}
		if !c.refused && err != nil {
			t.Errorf("a plaintext with %s is refused: %v", what, err)
		}
		if err != nil && (strings.Contains(err.Error(), "value-") || strings.Contains(err.Error(), "\u00e9")) {
			t.Errorf("the refusal of %s quotes a value: %v", what, err)
		}
	}
}

// TestTheSignedCallIsAResolveCall pins that the signed call of fixtures/adapter/signed has
// the path ResolvePath and a body resolve.schema.json accepts, resolve/two-addresses.json
// compact, and that its signed answer has a body resolve-answer.schema.json accepts,
// resolve-answer/one-covered.json compact, with one result for each address of the
// call, in its order, which decodes into ResolveAnswer.
func TestTheSignedCallIsAResolveCall(t *testing.T) {
	c, _, _ := readCall(t)
	if c.Path != contracts.ResolvePath {
		t.Errorf("call.json has the path %q, want %q", c.Path, contracts.ResolvePath)
	}
	if err := validate(compile(t, "resolve.schema.json"), c.Body); err != nil {
		t.Errorf("call.json: the body: %v", err)
	}
	var doc struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/signed/call-answer.json"), &doc); err != nil {
		t.Fatal(err)
	}
	if err := validate(compile(t, "resolve-answer.schema.json"), []byte(doc.Body)); err != nil {
		t.Errorf("call-answer.json: the body: %v", err)
	}
	for name, body := range map[string]string{"resolve/two-addresses.json": string(c.Body), "resolve-answer/one-covered.json": doc.Body} {
		var want bytes.Buffer
		if err := json.Compact(&want, read(t, "fixtures/adapter/"+name)); err != nil || want.String() != body {
			t.Errorf("the signed body is not %s", name)
		}
	}
	var call contracts.Resolve
	var answer contracts.ResolveAnswer
	if err := json.Unmarshal(c.Body, &call); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(doc.Body), &answer); err != nil {
		t.Fatal(err)
	}
	if len(answer.Results) != len(call.Addresses) {
		t.Fatalf("the answer has %d results for %d addresses", len(answer.Results), len(call.Addresses))
	}
	for i, r := range answer.Results {
		if r.Address != call.Addresses[i] {
			t.Errorf("result %d is for %q, want %q", i, r.Address, call.Addresses[i])
		}
	}
	if answer.Results[0].Covered != nil || answer.Results[0].ResourceRef == "" || answer.Results[1].Covered == nil || *answer.Results[1].Covered ||
		answer.Results[1].GrantRef != "" {
		t.Errorf("the answer decodes to %+v", answer.Results)
	}
}

// TestAReadPathIsRelativeAndHasNoDotParts pins which paths read.schema.json accepts: one
// or more parts joined by /, with no / at the start or the end, no empty part, and no
// part that is . or .., while a part that starts with a dot, or has three dots, is a name.
func TestAReadPathIsRelativeAndHasNoDotParts(t *testing.T) {
	schema := compile(t, "read.schema.json")
	body := read(t, "fixtures/adapter/read/at-a-name.json")
	if !bytes.Contains(body, []byte(`"path": "qory.yaml"`)) {
		t.Fatal("read/at-a-name.json has no path qory.yaml")
	}
	for p, refused := range map[string]bool{
		"qory.yaml":          false,
		"docs/setup.md":      false,
		".config/qory.yaml":  false,
		"docs/.../notes.txt": false,
		"docs/..a/b":         false,
		"/qory.yaml":         true,
		"docs/":              true,
		"docs//setup.md":     true,
		".":                  true,
		"..":                 true,
		"docs/./setup.md":    true,
		"docs/../qory.yaml":  true,
		"docs/..":            true,
	} {
		err := validate(schema, bytes.Replace(body, []byte(`"path": "qory.yaml"`), []byte(`"path": "`+p+`"`), 1))
		if refused && err == nil {
			t.Errorf("a read of the path %q is accepted", p)
		}
		if !refused && err != nil {
			t.Errorf("a read of the path %q is refused: %v", p, err)
		}
	}
}

// TestAReadAnswerHoldsItsByteLimit pins ReadAnswer.Check: content of more than 262144
// bytes of UTF-8 is refused, though the schema accepts it when it has at most 262144
// characters; content of exactly 262144 bytes is accepted; and so is the fixture answer,
// which read-answer.schema.json accepts.
func TestAReadAnswerHoldsItsByteLimit(t *testing.T) {
	schema := compile(t, "read-answer.schema.json")
	long := contracts.ReadAnswer{Version: contracts.Version, Revision: contracts.Revision, Content: strings.Repeat("\u00e9", 131073),
		ResolvedAt: "3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4"}
	b, err := json.Marshal(long)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(schema, b); err != nil {
		t.Errorf("content of 131073 characters and 262146 bytes is refused by the schema: %v", err)
	}
	if long.Check() == nil {
		t.Error("content of 262146 bytes is accepted")
	}
	exact := long
	exact.Content = strings.Repeat("x", contracts.ReadContentMaxBytes)
	if err := exact.Check(); err != nil {
		t.Errorf("content of 262144 bytes is refused: %v", err)
	}
	var fixture contracts.ReadAnswer
	if err := json.Unmarshal(read(t, "fixtures/adapter/read-answer/at-a-fixed-version.json"), &fixture); err != nil || fixture.Check() != nil {
		t.Errorf("read-answer/at-a-fixed-version.json breaks a rule: %v, %v", err, fixture.Check())
	}
}

// TestAReadAnswerWithoutADocumentHasNoContent pins the answer with found: Check accepts
// found false without content, and the fixture read-answer/no-document.json, and refuses
// found true and content beside found; and the answer with found false marshals without
// the member content, to a body read-answer.schema.json accepts.
func TestAReadAnswerWithoutADocumentHasNoContent(t *testing.T) {
	absent := contracts.ReadAnswer{Version: contracts.Version, Revision: contracts.Revision, Found: new(bool),
		ResolvedAt: "3f2a91c4d5e6f708192a3b4c5d6e7f8091a2b3c4"}
	if err := absent.Check(); err != nil {
		t.Errorf("found false without content is refused: %v", err)
	}
	b, err := json.Marshal(absent)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte(`"content"`)) {
		t.Errorf("found false marshals to %s, with content", b)
	}
	if err := validate(compile(t, "read-answer.schema.json"), b); err != nil {
		t.Errorf("found false marshals to %s, which the schema refuses: %v", b, err)
	}
	withContent := absent
	withContent.Content = "harness:\n"
	if withContent.Check() == nil {
		t.Error("content beside found is accepted")
	}
	found := true
	foundTrue := absent
	foundTrue.Found = &found
	if foundTrue.Check() == nil {
		t.Error("found true is accepted")
	}
	var fixture contracts.ReadAnswer
	if err := json.Unmarshal(read(t, "fixtures/adapter/read-answer/no-document.json"), &fixture); err != nil ||
		fixture.Found == nil || fixture.Check() != nil {
		t.Errorf("read-answer/no-document.json does not decode to found false: %v, %v", err, fixture.Check())
	}
}

// TestARunCredentialAnswerIsForagersCredentialNarrowed pins that each accepted run
// credential answer's credential, version 1 included, is a document Forager's
// credential.schema.json accepts, so the narrowing only takes away; that the answer
// without the credential's version is refused, as Forager refuses it; and that the call
// fixtures have the path's members as RunCredential decodes them.
func TestARunCredentialAnswerIsForagersCredentialNarrowed(t *testing.T) {
	forager, err := foragercontracts.Compile("credential.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files(t, "fixtures/adapter/run-credential-answer") {
		var doc struct {
			Credential json.RawMessage `json:"credential"`
		}
		if err := json.Unmarshal(read(t, f), &doc); err != nil {
			t.Fatal(err)
		}
		if err := validate(forager, doc.Credential); err != nil {
			t.Errorf("%s: Forager's credential.schema.json refuses the credential: %v", f, err)
		}
		var a contracts.RunCredentialAnswer
		if err := json.Unmarshal(read(t, f), &a); err != nil || a.Credential.Version != 1 {
			t.Errorf("%s: the credential's version is %d, %v", f, a.Credential.Version, err)
		}
	}
	if validate(forager, []byte(`{"token":"x","apply":[{"hosts":["code.example.com"],"scheme":"bearer"}]}`)) == nil {
		t.Error("Forager's credential.schema.json accepts a credential without its version")
	}
	if contracts.RunCredentialPath != "run-credential" {
		t.Errorf("RunCredentialPath is %q", contracts.RunCredentialPath)
	}
}

// TestARunCredentialAnswerHoldsTheRulesTheSchemaCannotSee pins RunCredentialAnswer.Check:
// a token of more than 16384 bytes, which the schema accepts when it has at most 16384
// characters, is refused, and one of exactly 16384 bytes accepted; a token with a NUL, a
// carriage return or a line feed, or a space or a tab at an edge, is refused; expires_at
// is refused when it is not later than the call's time, and accepted when later or
// absent; and no message contains the token.
func TestARunCredentialAnswerHoldsTheRulesTheSchemaCannotSee(t *testing.T) {
	call := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	answer := func(token, expires string) contracts.RunCredentialAnswer {
		return contracts.RunCredentialAnswer{Version: contracts.Version, Revision: contracts.Revision,
			Credential: contracts.Credential{Version: 1, Token: token, ExpiresAt: expires,
				Apply: []contracts.CredentialApply{{Hosts: []string{"code.example.com"}, Scheme: "bearer"}}}}
	}
	long := strings.Repeat("\u00e9", 8193)
	b, err := json.Marshal(answer(long, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compile(t, "run-credential-answer.schema.json"), b); err != nil {
		t.Errorf("a token of 8193 characters and 16386 bytes is refused by the schema: %v", err)
	}
	for what, c := range map[string]struct {
		a       contracts.RunCredentialAnswer
		refused bool
	}{
		"a token of 16386 bytes":            {answer(long, ""), true},
		"a token of 16384 bytes":            {answer(strings.Repeat("x", contracts.RunCredentialTokenMaxBytes), ""), false},
		"an empty token":                    {answer("", ""), true},
		"a token with a NUL":                {answer("token-value\x00", ""), true},
		"a token with a carriage return":    {answer("token\rvalue", ""), true},
		"a token with a line feed":          {answer("token\nvalue", ""), true},
		"a token with a space at its start": {answer(" token-value", ""), true},
		"a token with a tab at its end":     {answer("token-value\t", ""), true},
		"a token with a space inside":       {answer("token value", ""), false},
		"expires_at at the call's time":     {answer("token-value", "2026-10-15T12:00:00Z"), true},
		"expires_at before the call's time": {answer("token-value", "2026-10-15T13:00:00+02:00"), true},
		"expires_at after the call's time":  {answer("token-value", "2026-10-15T12:00:01Z"), false},
		"expires_at in another zone, later": {answer("token-value", "2026-10-15T14:30:00+02:00"), false},
		"no expires_at":                     {answer("token-value", ""), false},
	} {
		err := c.a.Check(call)
		if c.refused && err == nil {
			t.Errorf("an answer with %s is accepted", what)
		}
		if !c.refused && err != nil {
			t.Errorf("an answer with %s is refused: %v", what, err)
		}
		if err != nil && strings.Contains(err.Error(), "token-value") {
			t.Errorf("the refusal of %s quotes the token: %v", what, err)
		}
	}
	for _, f := range files(t, "fixtures/adapter/run-credential-answer") {
		var a contracts.RunCredentialAnswer
		if err := json.Unmarshal(read(t, f), &a); err != nil || a.Check(call) != nil {
			t.Errorf("%s breaks a rule: %v, %v", f, err, a.Check(call))
		}
	}
}

// TestAConnectFlowOnALiveConnectionHasTheSameShape pins that a connect pass and a linked
// connect return for a live connection, after which a grant report adds or replaces the
// grant of one external account, are accepted by their schemas with the members of a
// pending connection's: connect-pass/payload-live-connection.json and
// connect-return/linked-live-connection.json, with the same connection_id and nonce.
// Whether the connection is pending or live, and whether the grant is already linked to
// another live connection, are the control plane's and the adapter's checks, which
// connect-return.schema.json states and no schema expresses.
func TestAConnectFlowOnALiveConnectionHasTheSameShape(t *testing.T) {
	var p contracts.ConnectPass
	var r contracts.ConnectReturn
	if err := json.Unmarshal(read(t, "fixtures/adapter/connect-pass/payload-live-connection.json"), &p); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/connect-return/linked-live-connection.json"), &r); err != nil {
		t.Fatal(err)
	}
	if r.ConnectionID != p.ConnectionID || r.Nonce != p.Nonce || r.Outcome != "linked" {
		t.Errorf("the live connection's return does not answer its pass: %+v, %+v", p, r)
	}
	for schema, f := range map[string]string{
		"connect-pass.schema.json":   "fixtures/adapter/connect-pass/payload-live-connection.json",
		"connect-return.schema.json": "fixtures/adapter/connect-return/linked-live-connection.json",
	} {
		if err := validate(compile(t, schema), read(t, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	whole, err := contracts.Document("connect-return.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	description, _ := whole.(map[string]any)["description"].(string)
	for _, want := range []string{"pending or live", "grant_taken"} {
		if !strings.Contains(description, want) {
			t.Errorf("connect-return.schema.json's description does not say %q", want)
		}
	}
}
