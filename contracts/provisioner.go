package contracts

import "encoding/json"

// ProvisionerRevision is the provisioner contract's revision, which
// X-Qory-Contract-Version carries on its requests as a decimal integer, and which
// [ClaimAnswer] and [HandoverAnswer] name in Revision. It counts additions alone: each
// addition to provisioner/v1 raises it. A change that breaks a reader is a new
// contract, provisioner/v2, not a new revision.
const ProvisionerRevision = 1

// MaxInputs is the most bytes a handover answer's inputs has, serialized.
const MaxInputs = 768 << 10

// MaxHandoverAnswer is the most bytes a handover answer's body has.
const MaxHandoverAnswer = 1 << 20

// Claim is the body of POST /v1/pools/claim, provisioner/v1 claim.schema.json. An empty
// PoolID claims from the key's node pool, the one node pool the pool's access key
// belongs to.
type Claim struct {
	Version     int    `json:"version"`
	Time        string `json:"time"`
	PoolID      string `json:"pool_id,omitempty"`
	WaitSeconds int    `json:"wait_seconds"`
}

// ClaimAnswer is the body of the 200 answer to a claim, claim-answer.schema.json. The
// starter refuses one whose Revision is not the revision it sent.
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
	ID          string `json:"id"`
	ExternalRef string `json:"external_ref"`
	URL         string `json:"url"`
	Title       string `json:"title"`
}

// Dispatched is the body of POST /v1/dispatches/<dispatch_id>/dispatched,
// dispatched.schema.json: the starter started the job. It is answered, on success, with
// a signed 204 and no body. On 409 lease_expired the dispatch is stale: its run went
// back to the queue, with the same run id, when the lease lapsed, and the starter stops
// the job it started for that dispatch.
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
// handover.schema.json, which gives its refusals and their order. A run key is handed
// over at most once per run, and once a dispatch's handover is answered, its run never
// goes back to the queue. On 409 lease_expired the starter stops the job it started
// for that dispatch.
type Handover struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
}

// HandoverAnswer is the body of the 200 answer to a handover,
// handover-answer.schema.json. Inputs holds the step's input as the answer's bytes, so
// the starter passes it to the job unread: its members, their order and its numbers
// unchanged. json.Marshal writes it compacted, with <, > and & escaped, which a JSON
// reader reads as the same text. The starter refuses one whose Revision is not the
// revision it sent. The control plane sends it once per run, with Cache-Control:
// no-store, and the starter passes RunKey to the job it started for the dispatch alone
// and keeps it no longer than that.
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
// release.schema.json: the starter gives the run back. A release before the handover
// ends the dispatch, and Retry true returns the same run to the queue, to be claimed
// under a new dispatch. After the handover, while the run goes on, the release ends
// the run and is answered as a success, the signed 204, not 409; the gateway refuses the
// run's key with 410 when the job attaches, and Retry true queues a new run, with a new
// run id and a new run key.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
	Retry   bool   `json:"retry"`
}
