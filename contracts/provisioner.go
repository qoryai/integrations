package contracts

import "encoding/json"

// ProvisionerRevision is the provisioner contract's revision, which
// X-Qory-Contract-Version carries on its requests as a decimal integer, and which
// [ClaimAnswer] and [HandoverAnswer] name in Revision. It counts additions alone: each
// addition to provisioner/v1 raises it. A change that breaks a reader is a new
// contract, provisioner/v2, not a new revision. Revision 2 adds the step prebuild and
// [Prebuilt]. The control plane serves revisions 1 and 2, and a starter that implements
// revision 1 gets answers of revision 1 and never meets a dispatch of the step prebuild.
const ProvisionerRevision = 2

// MaxInputs is the most bytes a handover answer's inputs has, serialized.
const MaxInputs = 768 << 10

// MaxHandoverAnswer is the most bytes a handover answer's body has.
const MaxHandoverAnswer = 1 << 20

// Claim is the body of POST /v1/pools/claim, provisioner/v1 claim.schema.json. With an
// empty PoolID the claim covers the key's node pool, the one node pool the pool's access
// key belongs to.
type Claim struct {
	Version     int    `json:"version"`
	Time        string `json:"time"`
	PoolID      string `json:"pool_id,omitempty"`
	WaitSeconds int    `json:"wait_seconds"`
}

// ClaimAnswer is the body of the 200 answer to a claim, claim-answer.schema.json. The
// starter refuses one whose Revision is not the revision it sent. WorkItem is set for
// the step explore and nil for the step prebuild, whose run builds the environment of
// Target.
type ClaimAnswer struct {
	Version    int       `json:"version"`
	Revision   int       `json:"revision"`
	DispatchID string    `json:"dispatch_id"`
	RunID      string    `json:"run_id"`
	PoolID     string    `json:"pool_id"`
	LeaseUntil string    `json:"lease_until"`
	Step       string    `json:"step"`
	Access     string    `json:"access"`
	Target     Target    `json:"target"`
	WorkItem   *WorkItem `json:"work_item,omitempty"`
}

// WorkItem is the work item a run is for.
type WorkItem struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
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
// over at most once per run, and once a dispatch's handover is answered with the signed
// 200, its run never goes back to the queue. On 409 lease_expired the starter stops the
// job it started for that dispatch. On 503 unavailable the handover took no effect.
// After a 503 unavailable, and after a transient failure, no answer, an unsigned answer
// or any other 5xx, the starter may send the handover again while the lease lasts. On
// 409 handed_over to a handover it sent again after a transient failure, it stops the
// job, releases the dispatch with retry: true and never recovers the run key.
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
// and keeps it no longer than that. WorkItem is set for the step explore and nil for the
// step prebuild, whose Inputs are exactly {"version":1}.
type HandoverAnswer struct {
	Version  int             `json:"version"`
	Revision int             `json:"revision"`
	RunID    string          `json:"run_id"`
	RunKey   string          `json:"run_key"`
	Step     string          `json:"step"`
	Access   string          `json:"access"`
	Target   Target          `json:"target"`
	WorkItem *WorkItem       `json:"work_item,omitempty"`
	Inputs   json.RawMessage `json:"inputs"`
}

// Release is the body of POST /v1/dispatches/<dispatch_id>/release,
// release.schema.json: the starter gives the run back. A release before its handover,
// of a dispatch that is not stale, ends the dispatch, and Retry true returns the same
// run to the queue, to be claimed under a new dispatch. After its handover, while the run goes on, the release ends
// the run and is answered as a success, the signed 204, not 409; the gateway refuses the
// run's key with 410 when the job attaches, and Retry true queues a new run, with a new
// run id and a new run key. For a dispatch of the step prebuild, a release after its
// handover never queues a new run: Retry true is read as false, and the run ends. A
// failed build is reported this way, and the control plane records Reason as the last
// build's failure; the next prebuild comes only from a change of the collection's
// environment_ref or from a person asking for one.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
	Retry   bool   `json:"retry"`
}

// Prebuilt is the body of POST /v1/dispatches/<dispatch_id>/prebuilt,
// prebuilt.schema.json, which gives its refusals and their order: the job for a dispatch
// of the step prebuild built the environment of its target, and the starter reports
// Image, pinned by its digest, Size in bytes, and the EnvironmentRef and Commit the job
// read. It is answered, on success, with a signed 204 and no body: the control plane
// records the image for the node pool and the target, and the run ends, so any later
// request naming the dispatch answers 410 gone. A RunID that is not the dispatch's run
// is refused 409 run_mismatch. A failed build is reported with a [Release] instead.
type Prebuilt struct {
	Version        int    `json:"version"`
	Time           string `json:"time"`
	RunID          string `json:"run_id"`
	Image          string `json:"image"`
	Size           int64  `json:"size"`
	EnvironmentRef string `json:"environment_ref"`
	Commit         string `json:"commit"`
}
