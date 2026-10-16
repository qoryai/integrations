package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/qoryai/forager/accesskey"
	"github.com/qoryai/integrations/contracts"
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
	"refusal.schema.json",
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
		"changes-sync-interval-30.json":                        {"changes", "/changes/0/interval minimum"},
		"changes-unknown-kind.json":                            {"changes", "/changes/0/kind enum"},
		"changes-work-item-status-linked.json":                 {"changes", "/changes/0/status enum"},
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
		"source-items-event-empty.json":                        {"source-items", "/items minItems"},
		"source-item-comments-empty-without-complete.json":     {"source-item-comments", "/comments minItems"},
		"source-items-truncated-labels-twice.json":             {"source-items", "/items/0/truncated uniqueItems"},
		"source-items-truncated-state-reason.json":             {"source-items", "/items/0/truncated/0 enum"},
		"source-items-truncated-unknown-field.json":            {"source-items", "/items/0/truncated/0 enum"},
		"sources-disconnect-with-reason.json":                  {"sources-disconnect", "/ additionalProperties"},
		"connect-return-refused-without-reason.json":           {"connect-return", "/ required reason"},
		"connect-return-linked-with-reason.json":               {"connect-return", "/reason "},
		"connect-return-unknown-outcome.json":                  {"connect-return", "/outcome enum"},
		"connect-return-undefined-member.json":                 {"connect-return", "/ additionalProperties"},
		"connect-pass-without-connection-id.json":              {"connect-pass", "/ required connection_id"},
		"connections-disconnect-undefined-member.json":         {"connections-disconnect", "/ additionalProperties"},
		"connections-disconnect-connection-id-adapter-id.json": {"connections-disconnect", "/connection_id pattern"},
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
	values := []struct {
		schema string
		v      any
	}{
		{"adapter", contracts.Adapter{Version: contracts.Version, Time: now, ConnectURL: "https://adapter.example.com/connect",
			Sync: contracts.Sync{DefaultInterval: 900, MinimumInterval: 60, SendsChanges: true}}},
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef",
			Listing: listing}},
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, ConnectionID: "ac_0123456789abcdef",
			Listing: listing,
			Collections: []contracts.Collection{
				{Ref: "TRK", URL: "https://tracker.acme.example/TRK", Title: "Tracker"},
				{Ref: "SHOP", URL: "https://code.example.com/acme/shop", Title: "acme/shop", Target: target},
			}}},
		{"source-items", contracts.SourceItems{Version: contracts.Version, Time: now, CollectionRef: "TRK", Listing: &listing,
			Items: []contracts.Item{
				{Ref: "TRK-1", URL: "https://tracker.acme.example/TRK/TRK-1", Title: "Checkout fails", BodyFormat: "markdown",
					FiledBy: person, State: "open", CreatedAt: now, UpdatedAt: now},
				{Ref: "TRK-2", URL: "https://tracker.acme.example/TRK/TRK-2", Type: "bug", Title: "Prices without tax",
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
		{"sources-disconnect", contracts.SourcesDisconnect{Version: contracts.Version, Time: now, CollectionRef: "TRK"}},
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
		{"refusal", contracts.Refusal{Error: "collection_not_linked"}},
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
// signs the requests, and the server key that signs the answer and the connect pass.
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

// strictJSON reports the first number written with a fraction or an exponent, and the
// first member an object repeats, at any depth: the two rules of definitions.schema.json
// that the control plane checks on a request's raw bytes, and that no schema can express,
// since a parsed document holds 900.0 as the integer 900 and keeps one member of a name.
// Every number of an integration request is an integer.
func strictJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	type frame struct {
		object, key bool
		seen        map[string]bool
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if top != nil && top.object && top.key {
			if d, ok := tok.(json.Delim); ok && d == '}' {
				stack = stack[:len(stack)-1]
				continue
			}
			name := tok.(string)
			if top.seen[name] {
				return fmt.Errorf("the member %q is repeated", name)
			}
			top.seen[name] = true
			top.key = false
			continue
		}
		if top != nil && top.object {
			top.key = true
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, &frame{object: true, key: true, seen: map[string]bool{}})
			case '[':
				stack = append(stack, &frame{})
			case ']':
				stack = stack[:len(stack)-1]
			}
		case json.Number:
			if strings.ContainsAny(string(v), ".eE") {
				return fmt.Errorf("the number %s has a fraction or an exponent", v)
			}
		}
	}
}

// TestTheControlPlaneRefusesWhatTheSchemaCannotSee pins the rule that an integer has no
// fraction or exponent and that no object repeats a member: each document under
// fixtures/adapter/refused-by-control-plane is accepted by its schema, since the rule is
// not expressible in one, and breaks the rule on its raw bytes, so the control plane
// refuses it, 400 invalid_request; and every accepted request fixture keeps the rule.
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
		if strictJSON(b) == nil {
			t.Errorf("%s keeps the rule on its raw bytes", f)
		}
	}
	for _, d := range []string{"adapter", "source-collections", "source-items", "source-item-comments", "sources-disconnect",
		"connections-disconnect", "connect-pass", "connect-return"} {
		for _, f := range files(t, path.Join("fixtures/adapter", d)) {
			if err := strictJSON(read(t, f)); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
}
