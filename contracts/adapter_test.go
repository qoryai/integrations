package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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
	"changes.schema.json",
	"connect-pass.schema.json",
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
		if !d.IsDir() || d.Name() == "invalid" || d.Name() == "signed" {
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
		"adapter-sync-without-sends-changes.json":    {"adapter", "/sync required sends_changes"},
		"changes-sync-interval-30.json":              {"changes", "/changes/0/interval minimum"},
		"changes-unknown-kind.json":                  {"changes", "/changes/0/kind enum"},
		"changes-work-item-status-linked.json":       {"changes", "/changes/0/status enum"},
		"changes-without-revision.json":              {"changes", "/ required revision"},
		"source-collections-no-listing.json":         {"source-collections", "/ required listing"},
		"source-item-comments-person-kind-app.json":  {"source-item-comments", "/comments/0/author/kind enum"},
		"source-items-101-items.json":                {"source-items", "/items maxItems"},
		"source-items-body-too-long.json":            {"source-items", "/items/0/body maxLength"},
		"source-items-empty-label.json":              {"source-items", "/items/0/labels/1 minLength"},
		"source-items-state-reason-too-long.json":    {"source-items", "/items/0/state_reason maxLength"},
		"source-items-time-fraction.json":            {"source-items", "/time pattern"},
		"source-items-time-offset.json":              {"source-items", "/time pattern"},
		"source-items-undefined-member.json":         {"source-items", "/ additionalProperties"},
		"source-items-url-http.json":                 {"source-items", "/items/0/url pattern"},
		"sources-disconnect-reason-uninstalled.json": {"sources-disconnect", "/reason enum"},
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
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, Listing: listing}},
		{"source-collections", contracts.SourceCollections{Version: contracts.Version, Time: now, Listing: listing,
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
					State: "closed", StateReason: &reason, CommentCount: 3, CreatedAt: now, UpdatedAt: now, Target: target},
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
					BodyFormat: "text", CreatedAt: now, UpdatedAt: now, Deleted: true},
			}}},
		{"sources-disconnect", contracts.SourcesDisconnect{Version: contracts.Version, Time: now, CollectionRef: "TRK", Reason: "paused"}},
		{"changes", contracts.Changes{Version: contracts.Version, Revision: contracts.Revision, Cursor: "c_1"}},
		{"changes", contracts.Changes{Version: contracts.Version, Revision: contracts.Revision, Cursor: "c_2", Changes: []contracts.Change{
			{Kind: contracts.ChangeWorkItem, WorkItemID: "wi_1", Ref: "TRK-1", Status: "taken_in", URL: "https://control.example.com/work-items/wi_1"},
			{Kind: contracts.ChangeCollection, CollectionRef: "TRK", Status: "linked"},
			{Kind: contracts.ChangeSync},
			{Kind: contracts.ChangeSync, Interval: &interval},
		}}},
		{"connect-pass", contracts.ConnectPass{Version: contracts.Version, AdapterID: "na_0123456789abcdef",
			Nonce: "AAECAwQFBgcICQoLDA0ODw", ExpiresAt: now}},
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

// TestTheAdapterSchemasNameNoExternalSystem walks every schema of the adapter and fails
// on a member name, a required name, a const or an enum value that holds an external
// system's word, or is app. The fixtures' data are not checked.
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
	for _, name := range adapterSchemas {
		doc, err := contracts.Document(name)
		if err != nil {
			t.Fatal(err)
		}
		walk(name, doc)
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

// verifyConnectPass checks a connect pass's signature and its payload's schema, and
// nothing else: at most ConnectPassMaxLength characters, the payload and the signature in
// strict base64url without padding split on the one dot, the signature verified under the
// pin over ConnectDomain, a line feed and the payload bytes, and then the payload against
// connect-pass.schema.json. It checks neither adapter_id, expires_at nor the nonce. It
// returns the payload's bytes.
func verifyConnectPass(t *testing.T, pass string, pin accesskey.Pin) ([]byte, bool) {
	t.Helper()
	encoded, sig, ok := strings.Cut(pass, ".")
	if !ok || len(pass) > contracts.ConnectPassMaxLength || strings.Contains(sig, ".") {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil {
		return nil, false
	}
	if !pin.Verify(append([]byte(contracts.ConnectDomain+"\n"), payload...), sig) {
		return nil, false
	}
	if err := validate(compile(t, "connect-pass.schema.json"), payload); err != nil {
		t.Errorf("the connect pass's payload: %v", err)
		return nil, false
	}
	return payload, true
}

// TestTheConnectPassVerifies pins the connect pass of fixtures/adapter/signed: it
// verifies under a pin of the fixture server key, beside another key as after a
// rotation, its payload is the fixture's and connect-pass.schema.json accepts it, and
// the same pass with one payload byte changed is refused.
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
	if err := json.Unmarshal(payload, &p); err != nil || p.Version != contracts.Version {
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
	if _, ok := verifyConnectPass(t, doc.Pass, accesskey.Pin{{Alg: "ed25519", PublicKey: access.PublicKey().String()}}); ok {
		t.Error("the connect pass verifies under a pin without the server key")
	}
}
