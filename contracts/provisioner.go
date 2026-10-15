package contracts

import "encoding/json"

// ProvisionerRevision is the provisioner contract's revision, which
// X-Qory-Contract-Version carries on its requests as a decimal integer, and which
// [ClaimAnswer] and [HandoverAnswer] name in Revision. Additions to the contract raise
// the revision; a change that breaks a reader raises [Version].
const ProvisionerRevision = 1

// MaxInputs is the most bytes a handover answer's inputs has, serialized.
const MaxInputs = 768 << 10

// MaxHandoverAnswer is the most bytes a handover answer's body has.
const MaxHandoverAnswer = 1 << 20

// Claim is the body of POST /v1/pools/claim, provisioner/v1 claim.schema.json. An empty
// PoolID claims from any node pool the adapter serves.
type Claim struct {
	Version     int    `json:"version"`
	Time        string `json:"time"`
	PoolID      string `json:"pool_id,omitempty"`
	WaitSeconds int    `json:"wait_seconds"`
}

// ClaimAnswer is the body of the 200 answer to a claim, claim-answer.schema.json. The
// adapter refuses one whose Revision is not the revision it sent.
type ClaimAnswer struct {
	Version    int      `json:"version"`
	Revision   int      `json:"revision"`
	DispatchID string   `json:"dispatch_id"`
	RunID      string   `json:"run_id"`
	PoolID     string   `json:"pool_id"`
	LeaseUntil string   `json:"lease_until"`
	Step       string   `json:"step"`
	Access     string   `json:"access"`
	Target     Target   `json:"target"`
	WorkItem   WorkItem `json:"work_item"`
}

// WorkItem is the work item a run is for.
type WorkItem struct {
	ID    string `json:"id"`
	Ref   string `json:"ref"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Dispatched is the body of POST /v1/dispatches/<dispatch_id>/dispatched,
// dispatched.schema.json: the adapter started the job.
type Dispatched struct {
	Version  int       `json:"version"`
	Time     string    `json:"time"`
	External *External `json:"external,omitempty"`
}

// External is a job at the provider that runs it.
type External struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
	URL  string `json:"url"`
}

// Handover is the body of POST /v1/dispatches/<dispatch_id>/handover,
// handover.schema.json.
type Handover struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
}

// HandoverAnswer is the body of the 200 answer to a handover,
// handover-answer.schema.json. Inputs holds the step's input as the answer's bytes, so
// the adapter passes it to the job unread: its members, their order and its numbers
// unchanged. json.Marshal writes it compacted, with <, > and & escaped, which a JSON
// reader reads as the same text. The adapter refuses one whose Revision is not the
// revision it sent.
type HandoverAnswer struct {
	Version  int             `json:"version"`
	Revision int             `json:"revision"`
	RunID    string          `json:"run_id"`
	RunKey   string          `json:"run_key"`
	Step     string          `json:"step"`
	Access   string          `json:"access"`
	Target   Target          `json:"target"`
	WorkItem WorkItem        `json:"work_item"`
	Inputs   json.RawMessage `json:"inputs"`
}

// Release is the body of POST /v1/dispatches/<dispatch_id>/release,
// release.schema.json: the adapter gives the run back.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
	Retry   bool   `json:"retry"`
}
