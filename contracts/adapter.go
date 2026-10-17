package contracts

import (
	"encoding/json"
	"fmt"
	"time"
)

// Version is the document format of integration/v1 and provisioner/v1: every request
// body of both contracts carries it, and so do the change feed's answer, the claim
// answer, the handover answer and the connect pass's payload. It is 1 for as long as
// those contracts are v1: a change that breaks a reader is a new contract, v2, not a new
// Version inside v1.
const Version = 1

// Revision is the integration contract's revision, which X-Qory-Contract-Version
// carries as a decimal integer. It counts additions alone: each addition to
// integration/v1 raises it. A change that breaks a reader is a new contract,
// integration/v2, not a new revision.
const Revision = 1

// ConnectDomain is the first line of the message a connect pass's signature covers: the
// domain line, a line feed, then the payload bytes.
const ConnectDomain = "qory-connect-ed25519-v1"

// ConnectReturnDomain is the first line of the message a connect return's signature
// covers: the domain line, a line feed, then the payload bytes. It keeps the
// signature from verifying as a request's signature or as a connect pass's.
const ConnectReturnDomain = "qory-connect-return-ed25519-v1"

// ConnectPassMaxLength is the most characters a connect pass has, whole.
const ConnectPassMaxLength = 1024

// MaxRequest is the most bytes a request's body has; the control plane refuses a larger
// one, 413.
const MaxRequest = 1 << 20

// TimeLayout is how every time of the adapter's requests and answers is written: RFC
// 3339 in UTC with whole seconds.
const TimeLayout = "2006-01-02T15:04:05Z"

// FormatTime writes a moment as the documents hold it, [TimeLayout], in UTC.
func FormatTime(t time.Time) string { return t.UTC().Format(TimeLayout) }

// Adapter is the body of POST /v1/adapter, adapter.schema.json, which an adapter sends
// each time it starts.
type Adapter struct {
	Version    int    `json:"version"`
	Time       string `json:"time"`
	ConnectURL string `json:"connect_url"`
	Sync       Sync   `json:"sync"`
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

// Collection is a place at the source that items are listed from.
type Collection struct {
	Ref    string  `json:"ref"`
	URL    string  `json:"url"`
	Title  string  `json:"title"`
	Target *Target `json:"target,omitempty"`
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
// text and never compared or parsed; empty, the adapter gives none. A nil Labels marshals as an empty list, and a
// nil StateReason as null. Truncated names the fields the adapter cut to fit the
// contract's limits: "title", "body" or "filed_by.name", cut at a code-point boundary,
// or "labels", some cut to 100 characters or those past the 100th left out; nil, the
// adapter cut none.
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
// sources-disconnect.schema.json: the adapter no longer reaches one collection.
type SourcesDisconnect struct {
	Version       int    `json:"version"`
	Time          string `json:"time"`
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
	ChangeSync       = "sync"
)

// Change is one entry of the change feed, of the kind Kind names. A work_item entry has
// WorkItemID, Ref, Status "taken_in" or "undone", and URL; a collection entry has
// CollectionRef and Status "linked" or "unlinked"; a sync entry has Interval, nil for
// the adapter's declared default. It marshals with the members of its kind alone.
type Change struct {
	Kind          string `json:"kind"`
	WorkItemID    string `json:"work_item_id,omitempty"`
	Ref           string `json:"ref,omitempty"`
	CollectionRef string `json:"collection_ref,omitempty"`
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

// Refusal is the body of the control plane's refusal of an adapter's request,
// refusal.schema.json.
type Refusal struct {
	Error string `json:"error"`
}
