package contracts

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Version is the document format of integration/v1 and provisioner/v1: every request
// body of both contracts carries it, and so do the change feed's answer, the claim
// answer, the connect pass's payload and the setup pass's payload.
// It is 1 for as long as those contracts are v1. For integration/v1, a change that
// breaks a reader is a new contract, integration/v2, not a new Version inside v1.
const Version = 1

// Revision is the integration contract's revision, which X-Qory-Contract-Version
// carries as a decimal integer. It counts additions alone: each addition to
// integration/v1 raises it. Additions made before revision 1 is released are part of
// revision 1. A change that breaks a reader is a new contract, integration/v2, not a
// new revision.
const Revision = 1

// ConnectDomain is the first line of the message a connect pass's signature covers: the
// domain line, a line feed, then the payload bytes.
const ConnectDomain = "qory-connect-ed25519-v1"

// SetupDomain is the first line of the message a setup pass's signature covers: the
// domain line, a line feed, then the payload bytes. It keeps a connect pass from
// verifying as a setup pass, and a setup pass from verifying as a connect pass.
const SetupDomain = "qory-setup-ed25519-v1"

// ConnectReturnDomain is the first line of the message a connect return's signature
// covers: the domain line, a line feed, then the payload bytes. It keeps the
// signature from verifying as a request's signature, a connect pass's or a setup
// pass's.
const ConnectReturnDomain = "qory-connect-return-ed25519-v1"

// CallDomain is the first line of the message a call's signature covers, [Call.Message]:
// the control plane signs a call to the adapter with its server key over it. It keeps a
// call's signature from verifying as a request's, an answer's, a pass's or a connect
// return's.
const CallDomain = "qory-call-ed25519-v1"

// CallAnswerDomain is the first line of the message the signature of an answer to a call
// covers, [CallAnswer.Message]: the adapter signs it with its access key. It keeps that
// signature from verifying as a request's, an answer's, a call's or a connect return's.
const CallAnswerDomain = "qory-call-answer-ed25519-v1"

// HeaderAdapterID is the header of a call that contains the adapter id the control plane
// enrolled the adapter as, na_ and 16 characters.
const HeaderAdapterID = "X-Qory-Adapter-Id"

// ErrCallMethod is the error of a call whose method is not POST, the one method a call
// has.
var ErrCallMethod = errors.New("a call is a POST, and no other method")

// Call is one call of the control plane to the adapter, as its signature covers it. The
// message is six lines joined by a line feed, with none after the last: [CallDomain], the
// adapter id, the instance id, the method in upper case, the call's path as its schema
// states it, such as resolve, and the raw body.
type Call struct {
	AdapterID  string
	InstanceID string
	Method     string
	Path       string
	Body       []byte
}

// Message returns the bytes the call's signature covers, or [ErrCallMethod] for a method
// other than POST.
func (c Call) Message() ([]byte, error) {
	method := strings.ToUpper(c.Method)
	if method != http.MethodPost {
		return nil, ErrCallMethod
	}
	var b bytes.Buffer
	b.WriteString(CallDomain + "\n" + c.AdapterID + "\n" + c.InstanceID + "\n" + method + "\n" + c.Path + "\n")
	b.Write(c.Body)
	return b.Bytes(), nil
}

// CallAnswer is the adapter's answer to a call, as its signature covers it: four lines
// joined by a line feed, with none after the last: [CallAnswerDomain], the status as three
// decimal digits, the call's X-Qory-Signature-Ed25519 exactly as sent, and the lower-case
// hex SHA-256 of the body. The third line binds the answer to the call it answers.
type CallAnswer struct {
	Status        int
	CallSignature string
	Body          []byte
}

// Message returns the four lines the answer's signature covers.
func (a CallAnswer) Message() []byte {
	sum := sha256.Sum256(a.Body)
	return []byte(CallAnswerDomain + "\n" + fmt.Sprintf("%03d", a.Status) + "\n" + a.CallSignature + "\n" +
		hex.EncodeToString(sum[:]))
}

// ConnectPassMaxLength is the most characters a connect pass has, whole.
const ConnectPassMaxLength = 1024

// SetupPassMaxLength is the most characters a setup pass has, whole, as many as a
// connect pass has.
const SetupPassMaxLength = 1024

// MaxRequest is the most bytes a request's body has; the control plane refuses a larger
// one, 413.
const MaxRequest = 1 << 20

// TimeLayout is how every time of the adapter's requests and answers is written: RFC
// 3339 in UTC with whole seconds.
const TimeLayout = "2006-01-02T15:04:05Z"

// FormatTime writes a moment as the documents hold it, [TimeLayout], in UTC.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// Adapter is the body of POST /v1/adapter, adapter.schema.json, which an adapter sends
// each time it starts, and again once it is set up at its source. SetupURL is set only
// while the adapter is not set up at its source; empty, the start-up has no setup_url,
// and the control plane shows no setup step. CallURL is the base address at which the
// control plane calls the adapter, whose path ends with /; empty, the start-up has no
// call_url, and the control plane makes no call to the adapter. Description is the
// document `<program> describe` prints for the adapter's program, description.schema.json,
// sent whole: the same JSON document, which json.Marshal writes compacted and with <, >
// and & escaped. A nil Description marshals as null, which adapter.schema.json refuses.
type Adapter struct {
	Version     int             `json:"version"`
	Time        string          `json:"time"`
	ConnectURL  string          `json:"connect_url"`
	SetupURL    string          `json:"setup_url,omitempty"`
	CallURL     string          `json:"call_url,omitempty"`
	Sync        Sync            `json:"sync"`
	Description json.RawMessage `json:"description"`
}

// Sync is how often an adapter pulls changes from the source, in seconds, and whether
// it sends the source's changes as they happen.
type Sync struct {
	DefaultInterval int  `json:"default_interval"`
	MinimumInterval int  `json:"minimum_interval"`
	SendsChanges    bool `json:"sends_changes"`
}

// SourceCollections is the body of POST /v1/source-collections,
// source-collections.schema.json: one part of the listing of the collections the
// adapter reaches through the connection ConnectionID. A nil Collections marshals as an
// empty list.
type SourceCollections struct {
	Version      int          `json:"version"`
	Time         string       `json:"time"`
	ConnectionID string       `json:"connection_id"`
	Collections  []Collection `json:"collections"`
	Listing      Listing      `json:"listing"`
}

// MarshalJSON writes the document, a nil Collections as an empty list.
func (s SourceCollections) MarshalJSON() ([]byte, error) {
	type plain SourceCollections
	if s.Collections == nil {
		s.Collections = []Collection{}
	}
	return json.Marshal(plain(s))
}

// Collection is a place at the source that items are listed from. EnvironmentRef is an
// id of the collection's environment definition, as the adapter reads it: it changes
// whenever that definition changes; empty, the collection has none. Items points to false
// while the collection has no items; nil, the collection has items whenever its source
// does.
type Collection struct {
	Ref            string  `json:"ref"`
	URL            string  `json:"url"`
	Title          string  `json:"title"`
	Target         *Target `json:"target,omitempty"`
	EnvironmentRef string  `json:"environment_ref,omitempty"`
	Items          *bool   `json:"items,omitempty"`
}

// Target is where a change would go: a host name and a path on it.
type Target struct {
	System string `json:"system"`
	Path   string `json:"path"`
}

// Listing is one part of a listing of the source.
type Listing struct {
	ID        string `json:"id"`
	Part      int    `json:"part"`
	StartedAt string `json:"started_at"`
	Complete  bool   `json:"complete"`
}

// SourceItems is the body of POST /v1/source-items, source-items.schema.json: items of
// one collection, an event of 1 to 100 items when Listing is nil, and a listing's part
// of 0 to 100 otherwise. A nil Items marshals as an empty list.
type SourceItems struct {
	Version       int      `json:"version"`
	Time          string   `json:"time"`
	CollectionRef string   `json:"collection_ref"`
	Items         []Item   `json:"items"`
	Listing       *Listing `json:"listing,omitempty"`
}

// MarshalJSON writes the document, a nil Items as an empty list.
func (s SourceItems) MarshalJSON() ([]byte, error) {
	type plain SourceItems
	if s.Items == nil {
		s.Items = []Item{}
	}
	return json.Marshal(plain(s))
}

// Item is one item of work at the source. Key is the item's name as people say it at
// its source, such as #1231, short within its collection, 1 to 64 characters, shown as
// text and never compared or parsed; empty, the adapter gives none. A nil Labels
// marshals as an empty list, and a nil StateReason as null. Truncated names the fields
// the adapter cut to fit the contract's limits: "title", "body" or "filed_by.name", cut
// at a code-point boundary, or "labels", some cut to 100 characters or those past the
// 100th left out; nil, the adapter cut none.
type Item struct {
	Ref          string   `json:"ref"`
	URL          string   `json:"url"`
	Key          string   `json:"key,omitempty"`
	Type         string   `json:"type,omitempty"`
	Title        string   `json:"title"`
	Body         string   `json:"body"`
	BodyFormat   string   `json:"body_format"`
	FiledBy      Person   `json:"filed_by"`
	Labels       []string `json:"labels"`
	State        string   `json:"state"`
	StateReason  *string  `json:"state_reason"`
	CommentCount int      `json:"comment_count"`
	CreatedAt    string   `json:"created_at"`
	UpdatedAt    string   `json:"updated_at"`
	Target       *Target  `json:"target,omitempty"`
	Truncated    []string `json:"truncated,omitempty"`
}

// MarshalJSON writes the item, a nil Labels as an empty list.
func (i Item) MarshalJSON() ([]byte, error) {
	type plain Item
	if i.Labels == nil {
		i.Labels = []string{}
	}
	return json.Marshal(plain(i))
}

// Person is who wrote something at the source: a display name, and Kind "person" or
// "automation".
type Person struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// SourceItemComments is the body of POST /v1/source-items/comments,
// source-item-comments.schema.json: comments of one item, 1 to 100, or 0 to 100 with
// Complete. A nil Comments marshals as an empty list.
type SourceItemComments struct {
	Version  int               `json:"version"`
	Time     string            `json:"time"`
	ItemRef  string            `json:"item_ref"`
	Comments []Comment         `json:"comments"`
	Complete *CommentsComplete `json:"complete,omitempty"`
}

// MarshalJSON writes the document, a nil Comments as an empty list.
func (s SourceItemComments) MarshalJSON() ([]byte, error) {
	type plain SourceItemComments
	if s.Comments == nil {
		s.Comments = []Comment{}
	}
	return json.Marshal(plain(s))
}

// Comment is one comment on an item; Deleted marks one deleted at the source.
// Truncated names the text fields the adapter cut at a code-point boundary to fit the
// contract's limit, "body" or "author.name"; nil, the adapter cut none.
type Comment struct {
	Ref        string   `json:"ref"`
	URL        string   `json:"url"`
	Author     Person   `json:"author"`
	Body       string   `json:"body"`
	BodyFormat string   `json:"body_format"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
	Deleted    bool     `json:"deleted,omitempty"`
	Truncated  []string `json:"truncated,omitempty"`
}

// CommentsComplete says a request's comments are all of the item's comments, read from
// Since.
type CommentsComplete struct {
	Since string `json:"since"`
}

// SourcesDisconnect is the body of POST /v1/sources/disconnect,
// sources-disconnect.schema.json: the adapter no longer reaches the collection
// CollectionRef through the connection ConnectionID.
type SourcesDisconnect struct {
	Version       int    `json:"version"`
	Time          string `json:"time"`
	ConnectionID  string `json:"connection_id"`
	CollectionRef string `json:"collection_ref"`
}

// ConnectionsDisconnect is the body of POST /v1/connections/disconnect,
// connections-disconnect.schema.json: the source no longer grants the connection
// ConnectionID access.
type ConnectionsDisconnect struct {
	Version      int    `json:"version"`
	Time         string `json:"time"`
	ConnectionID string `json:"connection_id"`
}

// Changes is the body of the answer to GET /v1/work-items/changes, changes.schema.json:
// the entries of the adapter's change feed after a cursor. A nil Changes marshals as an
// empty list.
type Changes struct {
	Version  int      `json:"version"`
	Revision int      `json:"revision"`
	Cursor   string   `json:"cursor"`
	Changes  []Change `json:"changes"`
}

// MarshalJSON writes the document, a nil Changes as an empty list.
func (c Changes) MarshalJSON() ([]byte, error) {
	type plain Changes
	if c.Changes == nil {
		c.Changes = []Change{}
	}
	return json.Marshal(plain(c))
}

// The kinds of a change feed's entries.
const (
	ChangeWorkItem   = "work_item"
	ChangeCollection = "collection"
	ChangeGrant      = "grant"
	ChangeSync       = "sync"
)

// Change is one entry of the change feed, of the kind Kind names. A work_item entry has
// WorkItemID, Ref, Status "taken_in" or "undone", and URL; a collection entry has
// CollectionRef and Status "linked" or "unlinked"; a grant entry has ConnectionID,
// GrantRef and Status "linked" or "unlinked"; a sync entry has Interval, nil for the
// adapter's declared default. It marshals with the members of its kind alone.
type Change struct {
	Kind          string `json:"kind"`
	WorkItemID    string `json:"work_item_id,omitempty"`
	Ref           string `json:"ref,omitempty"`
	CollectionRef string `json:"collection_ref,omitempty"`
	ConnectionID  string `json:"connection_id,omitempty"`
	GrantRef      string `json:"grant_ref,omitempty"`
	Status        string `json:"status,omitempty"`
	URL           string `json:"url,omitempty"`
	Interval      *int   `json:"interval,omitempty"`
}

// MarshalJSON writes the entry with the members of its kind alone, and refuses a kind
// the feed does not define.
func (c Change) MarshalJSON() ([]byte, error) {
	switch c.Kind {
	case ChangeWorkItem:
		return json.Marshal(struct {
			Kind       string `json:"kind"`
			WorkItemID string `json:"work_item_id"`
			Ref        string `json:"ref"`
			Status     string `json:"status"`
			URL        string `json:"url"`
		}{c.Kind, c.WorkItemID, c.Ref, c.Status, c.URL})
	case ChangeCollection:
		return json.Marshal(struct {
			Kind          string `json:"kind"`
			CollectionRef string `json:"collection_ref"`
			Status        string `json:"status"`
		}{c.Kind, c.CollectionRef, c.Status})
	case ChangeGrant:
		return json.Marshal(struct {
			Kind         string `json:"kind"`
			ConnectionID string `json:"connection_id"`
			GrantRef     string `json:"grant_ref"`
			Status       string `json:"status"`
		}{c.Kind, c.ConnectionID, c.GrantRef, c.Status})
	case ChangeSync:
		return json.Marshal(struct {
			Kind     string `json:"kind"`
			Interval *int   `json:"interval"`
		}{c.Kind, c.Interval})
	}
	return nil, fmt.Errorf("the change feed defines no kind %q", c.Kind)
}

// ConnectPass is the payload of a connect pass, connect-pass.schema.json.
type ConnectPass struct {
	Version      int    `json:"version"`
	AdapterID    string `json:"adapter_id"`
	ConnectionID string `json:"connection_id"`
	Nonce        string `json:"nonce"`
	ExpiresAt    string `json:"expires_at"`
}

// SetupPass is the payload of a setup pass, setup-pass.schema.json, which the control
// plane signs with its server key over [SetupDomain].
type SetupPass struct {
	Version   int    `json:"version"`
	AdapterID string `json:"adapter_id"`
	Nonce     string `json:"nonce"`
	ExpiresAt string `json:"expires_at"`
}

// ConnectReturn is the payload of a connect return, connect-return.schema.json, which
// the adapter signs with its access key over [ConnectReturnDomain]. A connect return is
// at most 1024 characters, as a connect pass is. Outcome is "linked", "refused" or
// "cancelled"; Reason is set only when Outcome is "refused".
type ConnectReturn struct {
	Version      int    `json:"version"`
	ConnectionID string `json:"connection_id"`
	Nonce        string `json:"nonce"`
	Outcome      string `json:"outcome"`
	Reason       string `json:"reason,omitempty"`
	ExpiresAt    string `json:"expires_at"`
}

// SealSuite is the suite of a sealed value on the wire: RFC 9180 base mode with
// DHKEM(X25519, HKDF-SHA256), HKDF-SHA256 and AES-256-GCM, the suite of the Forager
// contract's secrets answer.
const SealSuite = "x25519-sha256-aes256gcm"

// The ids of the suite's three parts, as RFC 9180 numbers them and as info contains
// them.
const (
	SealKEMID  = 0x0020
	SealKDFID  = 0x0001
	SealAEADID = 0x0002
)

// SecretsInfoDomain starts the HPKE info of the secrets the control plane delivers to
// an adapter, sealed to its access key, [SecretsInfo].
const SecretsInfoDomain = "qory adapter secrets v1"

// SecretsUploadInfoDomain starts the HPKE info of the secrets an adapter uploads, sealed
// to the control plane's server key, [SecretsUploadInfo]. It is not
// [SecretsInfoDomain], so an upload never opens as a delivery, nor a delivery as an
// upload.
const SecretsUploadInfoDomain = "qory adapter secrets upload v1"

// SecretValueMaxBytes is the most bytes of UTF-8 a secret's value has.
const SecretValueMaxBytes = 16384

// SecretsMax is the most secrets an adapter stores in the control plane.
const SecretsMax = 16

// SecretsInfo returns the HPKE info of a delivery of secrets to the access key
// accessKeyID: [SecretsInfoDomain], a zero byte, the suite's three ids as u16
// big-endian, and lp(access key id), lp being a u16 big-endian length, then the bytes.
func SecretsInfo(accessKeyID string) []byte { return info(SecretsInfoDomain, accessKeyID) }

// SecretsUploadInfo returns the HPKE info of an upload of secrets sealed to the server
// key serverKey, as adapter-secrets.schema.json writes it in server_key:
// [SecretsUploadInfoDomain], a zero byte, the suite's three ids as u16 big-endian, and
// lp(server key).
func SecretsUploadInfo(serverKey string) []byte {
	return info(SecretsUploadInfoDomain, serverKey)
}

// SecretsAAD returns the associated data of a sealing of secrets, in both directions:
// lp(adapter id) and lp(access key id). It has no run id and no run configuration
// digest: the secrets belong to the adapter, not to a run.
func SecretsAAD(adapterID, accessKeyID string) []byte {
	return lp(lp(nil, adapterID), accessKeyID)
}

// info is a domain, a zero byte, the suite's three ids as u16 big-endian, and lp(id).
func info(domain, id string) []byte {
	b := append([]byte(domain), 0)
	for _, x := range []uint16{SealKEMID, SealKDFID, SealAEADID} {
		b = binary.BigEndian.AppendUint16(b, x)
	}
	return lp(b, id)
}

// lp appends a u16 big-endian length, then the bytes. Every value it is given is bounded
// by a pattern of the contract far below 65536 bytes.
func lp(b []byte, s string) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(len(s)))
	return append(b, s...)
}

// AdapterSecrets is the body of POST /v1/adapter/secrets, adapter-secrets.schema.json:
// the adapter's secrets, [AdapterSecretsPlaintext], sealed to the control plane's server
// key.
type AdapterSecrets struct {
	Version int            `json:"version"`
	Time    string         `json:"time"`
	Sealed  SealedToServer `json:"sealed"`
}

// SealedToServer is a value sealed to the control plane's server key ServerKey, an
// Ed25519 public key in base64url: Enc, the ephemeral key, and CT, the ciphertext and
// its tag, each in base64url without padding.
type SealedToServer struct {
	Suite     string `json:"suite"`
	ServerKey string `json:"server_key"`
	Enc       string `json:"enc"`
	CT        string `json:"ct"`
}

// AdapterSecretsAnswer is the body of the answer to GET /v1/adapter/secrets,
// adapter-secrets-answer.schema.json: the adapter's secrets, [AdapterSecretsPlaintext],
// sealed to the access key the request verified under.
type AdapterSecretsAnswer struct {
	Version  int               `json:"version"`
	Revision int               `json:"revision"`
	Sealed   SealedToAccessKey `json:"sealed"`
}

// SealedToAccessKey is a value sealed to the access key AccessKeyID: Enc, the ephemeral
// key, and CT, the ciphertext and its tag, each in base64url without padding.
type SealedToAccessKey struct {
	Suite       string `json:"suite"`
	AccessKeyID string `json:"access_key_id"`
	Enc         string `json:"enc"`
	CT          string `json:"ct"`
}

// AdapterSecretsPlaintext is what an adapter's secrets are sealed as,
// adapter-secrets-plaintext.schema.json. A nil Secrets marshals as an empty list.
type AdapterSecretsPlaintext struct {
	Version int           `json:"version"`
	Secrets []SecretEntry `json:"secrets"`
}

// SecretEntry is one secret: its name, as the source role's secrets list it, and its
// value.
type SecretEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// MarshalJSON writes the plaintext, a nil Secrets as an empty list.
func (p AdapterSecretsPlaintext) MarshalJSON() ([]byte, error) {
	type plain AdapterSecretsPlaintext
	if p.Secrets == nil {
		p.Secrets = []SecretEntry{}
	}
	return json.Marshal(plain(p))
}

// Check reports the first rule of adapter-secrets-plaintext.schema.json that the
// plaintext breaks and that the schema does not express: each name appears once, and
// each value is 1 to [SecretValueMaxBytes] bytes of UTF-8 without a NUL. The message
// names the secret and never contains a value.
func (p AdapterSecretsPlaintext) Check() error {
	seen := map[string]bool{}
	for _, e := range p.Secrets {
		if seen[e.Name] {
			return fmt.Errorf("the secret %q appears twice", e.Name)
		}
		seen[e.Name] = true
		if len(e.Value) < 1 || len(e.Value) > SecretValueMaxBytes || !utf8.ValidString(e.Value) || strings.ContainsRune(e.Value, 0) {
			return fmt.Errorf("the value of the secret %q is not 1 to %d bytes of UTF-8 without a NUL", e.Name, SecretValueMaxBytes)
		}
	}
	return nil
}

// Grant is the body of POST /v1/grants, grant.schema.json: one grant of the connection
// ConnectionID, the access the source grants for one external account, by the adapter's
// ref GrantRef, linked under the connect pass whose nonce is PassNonce.
type Grant struct {
	Version      int    `json:"version"`
	Time         string `json:"time"`
	ConnectionID string `json:"connection_id"`
	GrantRef     string `json:"grant_ref"`
	PassNonce    string `json:"pass_nonce"`
}

// Grants is the body of the answer to GET /v1/grants, grants.schema.json: the control
// plane's copy of the grants of the adapter's connections. A nil Grants marshals as an
// empty list.
type Grants struct {
	Version  int          `json:"version"`
	Revision int          `json:"revision"`
	Grants   []GrantEntry `json:"grants"`
}

// MarshalJSON writes the document, a nil Grants as an empty list.
func (g Grants) MarshalJSON() ([]byte, error) {
	type plain Grants
	if g.Grants == nil {
		g.Grants = []GrantEntry{}
	}
	return json.Marshal(plain(g))
}

// GrantEntry is one grant of the control plane's copy: the connection that holds it and
// the grant's ref.
type GrantEntry struct {
	ConnectionID string `json:"connection_id"`
	GrantRef     string `json:"grant_ref"`
}

// Refusal is the body of the control plane's refusal of an adapter's request,
// refusal.schema.json.
type Refusal struct {
	Error string `json:"error"`
}
