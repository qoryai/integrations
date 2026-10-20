package contracts

import (
	"bytes"
	"encoding/json"
)

// ProvisionerRevision is the provisioner contract's revision, which
// X-Qory-Contract-Version carries on its requests as a decimal integer, and which
// [ClaimAnswer] and [HandoverAnswer] name in Revision. The control plane serves this
// revision alone and refuses a request that names another, 400
// unsupported_contract_version. The step prebuild, [Prebuilt] and the handover answer's
// run_settings are part of it.
const ProvisionerRevision = 3

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
// Target. Target is nil, and Access empty, when the run has no place where a change
// would go, which the step explore allows; Access is set exactly when Target is.
type ClaimAnswer struct {
	Version    int       `json:"version"`
	Revision   int       `json:"revision"`
	DispatchID string    `json:"dispatch_id"`
	RunID      string    `json:"run_id"`
	PoolID     string    `json:"pool_id"`
	LeaseUntil string    `json:"lease_until"`
	Step       string    `json:"step"`
	Access     string    `json:"access,omitempty"`
	Target     *Target   `json:"target,omitempty"`
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
// step prebuild, whose Inputs are exactly version and environment_ref, the
// environment_ref the prebuild was queued for. RunSettings is set only for the step
// explore, and nil otherwise. Domain is the domain of the workspace the run is for, such
// as software. Target is nil, and Access empty, when the run has no place where a change
// would go, which the step explore allows; Access is set exactly when Target is.
type HandoverAnswer struct {
	Version     int             `json:"version"`
	Revision    int             `json:"revision"`
	RunID       string          `json:"run_id"`
	RunKey      string          `json:"run_key"`
	Step        string          `json:"step"`
	Domain      string          `json:"domain"`
	Access      string          `json:"access,omitempty"`
	Target      *Target         `json:"target,omitempty"`
	WorkItem    *WorkItem       `json:"work_item,omitempty"`
	RunSettings *RunSettings    `json:"run_settings,omitempty"`
	Inputs      json.RawMessage `json:"inputs"`
}

// RunSettings is a handover answer's run_settings, handover-answer.schema.json: the
// runtime and model settings the job composes the agent's harness with. It contains no
// key or secret. With no provider chosen, only Runtime is set: the run uses the
// gateway's configured key and the runtime's default model. Effort and Tiers are set
// only with Model, and ProviderModels is set exactly when Model is. The control
// plane guarantees four rules that the schema cannot check: the ids in ProviderModels
// are unique; Effort is listed for Model in ProviderModels; each tier's model is in
// ProviderModels; and a tier's effort is listed for that tier's model in
// ProviderModels. The job writes the values as given, and maps no model or effort value
// to another.
type RunSettings struct {
	// Runtime is the runtime's name in Forager's runtime descriptor, such as claude.
	Runtime string `json:"runtime"`
	// Model is the exact id of the model the run uses.
	Model string `json:"model,omitempty"`
	// Effort is the effort value the run uses with Model.
	Effort string `json:"effort,omitempty"`
	// ProviderModels are the models of the run's provider, 1 to 256. When it is empty,
	// the agents run on the provider of the gateway's key, and for a run with a target,
	// the job applies the tier map set at the run's target as written, as on a local
	// run. A run with no target has no such tier map: with ProviderModels empty, no tier
	// map applies, and the run uses the runtime's default model. When it is set and the
	// run has a target, an entry of the tier map set at the run's target applies only
	// when its model id is listed in ProviderModels, and its effort only when that
	// effort is listed for that model.
	ProviderModels []ProviderModel `json:"provider_models,omitempty"`
	// Tiers is the tier map the control plane resolved for the run.
	Tiers *Tiers `json:"tiers,omitempty"`
}

// ProviderModel is one model of the run's provider, an entry of
// run_settings.provider_models. Effort lists the effort values the model accepts, 1 to
// 16, each listed once. An entry with no Effort means the model takes no effort value.
type ProviderModel struct {
	ID     string   `json:"id"`
	Effort []string `json:"effort,omitempty"`
}

// Tiers is run_settings.tiers: a model, and optionally an effort value, for each tier
// the control plane maps. A tier the control plane does not map is nil. For a run with
// a target, a tier the control plane maps applies instead of the entry for that tier in
// the tier map set at the run's target, and the tier map set at the run's target fills
// only the tiers left nil. A run with no target has no such tier map: only the tiers the
// control plane maps apply.
type Tiers struct {
	Light    *Tier `json:"light,omitempty"`
	Fast     *Tier `json:"fast,omitempty"`
	Thinking *Tier `json:"thinking,omitempty"`
}

// Tier is the model of one tier: in JSON, a model id, or an object of model and an
// optional effort. A Tier read from JSON is written back in the form it was read in. A
// Tier built in Go is written as a model id when Effort is empty, and as an object
// otherwise.
type Tier struct {
	Model  string
	Effort string
	// object is true when the Tier was read from the object form.
	object bool
}

// tierObject is the object form of a [Tier].
type tierObject struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

// MarshalJSON writes t as a model id, or as an object of model and effort.
func (t Tier) MarshalJSON() ([]byte, error) {
	if t.Effort == "" && !t.object {
		return json.Marshal(t.Model)
	}
	return json.Marshal(tierObject{Model: t.Model, Effort: t.Effort})
}

// UnmarshalJSON reads a model id, or an object of model and an optional effort.
func (t *Tier) UnmarshalJSON(b []byte) error {
	b = bytes.TrimLeft(b, " \t\r\n")
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var model string
		if err := json.Unmarshal(b, &model); err != nil {
			return err
		}
		*t = Tier{Model: model}
		return nil
	}
	var o tierObject
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	*t = Tier{Model: o.Model, Effort: o.Effort, object: true}
	return nil
}

// Release is the body of POST /v1/dispatches/<dispatch_id>/release,
// release.schema.json: the starter gives the run back. A release before its handover,
// of a dispatch that is not stale, ends the dispatch, and Retry true returns the same
// run to the queue, to be claimed under a new dispatch, except a run of the step
// prebuild whose collection and node pool have another prebuild run waiting, which ends
// instead. After its handover, while the run goes on, the release ends
// the run and is answered as a success, the signed 204, not 409; the gateway refuses the
// run's key with 410 when the job attaches, and Retry true queues a new run, with a new
// run id and a new run key. For a dispatch of the step prebuild, a release after its
// handover never queues a new run: Retry true is read as false, and the run ends. A
// failed build is reported this way, and the control plane records Reason as the last
// build's failure.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
	Retry   bool   `json:"retry"`
}

// Prebuilt is the body of POST /v1/dispatches/<dispatch_id>/prebuilt,
// prebuilt.schema.json, which gives its refusals and their order: the job for a dispatch
// of the step prebuild built the environment of its target, and the starter reports
// Image, pinned by its digest, Size in bytes, the Commit the job built, and the
// EnvironmentRef of the handover answer's inputs, unchanged. It is answered, on success,
// with a signed 204 and no body: the control plane records the image for the node pool
// and the target, and the dispatch ends and its slot is freed, so any later request
// naming the dispatch answers 410 gone, and the run's outcome then decides how the run
// ended. A RunID that is not the dispatch's run is refused 409 run_mismatch. A failed
// build is reported with a [Release] instead.
type Prebuilt struct {
	Version        int    `json:"version"`
	Time           string `json:"time"`
	RunID          string `json:"run_id"`
	Image          string `json:"image"`
	Size           int64  `json:"size"`
	EnvironmentRef string `json:"environment_ref"`
	Commit         string `json:"commit"`
}
