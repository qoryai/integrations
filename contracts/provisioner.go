package contracts

// ProvisionerRevision is the provisioner contract's revision, which
// X-Qory-Contract-Version carries on its requests as a decimal integer, and which
// [ClaimAnswer] names in Revision. It counts additions alone: each addition to
// provisioner/v1 raises it. Additions made before revision 1 is released are part of
// revision 1. A change that breaks a reader is a new contract, provisioner/v2, not a
// new revision. The control plane serves this revision alone and refuses a request
// that names another, 400 unsupported_contract_version. The claim answer's run code
// and gateway URL are part of it.
const ProvisionerRevision = 1

// Claim is the body of POST /v1/pools/claim, provisioner/v1 claim.schema.json. With an
// empty PoolID the claim covers the key's node pool, the one node pool the pool's access
// key belongs to.
type Claim struct {
	Version     int    `json:"version"`
	Time        string `json:"time"`
	PoolID      string `json:"pool_id,omitempty"`
	WaitSeconds int    `json:"wait_seconds"`
}

// ClaimAnswer is the body of the 200 answer to a claim, claim-answer.schema.json: a
// request for one machine. The starter refuses one whose Revision is not the revision
// it sent. Machine is the type of machine the job runs on, a name from the control
// plane's list of machine types, such as small: the starter maps it to the machine it
// creates, and releases the dispatch when it does not know the name. Machine is empty
// when the control plane names no type, and the starter then uses its default. RunCode
// is the run code the control plane made at the claim. GatewayURL is the gateway that
// the node pool's choice resolved to at the claim: https, a host and an optional port,
// and nothing after them. The starter passes both to the machine it creates for the
// dispatch alone, and does not read the code. The control plane binds the code to that
// gateway, and refuses a redemption that another gateway relays with the same answer
// as for an unknown code. The machine redeems the code at the enrolment,
// https://qory.dev/contracts/forager/v1/enrolment.schema.json, which returns the run
// key and the run's documents, run/v1 documents.schema.json, in its signed 201.
//
// The control plane sends the answer with Cache-Control: no-store and gives it once.
// The starter keeps RunCode and GatewayURL no longer than it takes to pass them to the
// machine, and writes no line that contains qecr_ to a log. The starter sends the
// first [Dispatched] before LeaseUntil. The lease ends when the first dispatched is
// accepted or when the machine redeems the code, whichever comes first. From then on
// the dispatch never goes stale, and its work is never given to another claim while
// the dispatch lives. The lease lapses only when neither dispatched nor the redemption
// came before LeaseUntil: the dispatch is then stale, and the control plane may give
// the work to another claim. A first dispatched at or after LeaseUntil is refused 409
// lease_expired when the dispatch is stale, and accepted when the code was redeemed
// before LeaseUntil. A first redemption at or after LeaseUntil, with neither
// dispatched nor an earlier redemption before LeaseUntil, is refused, with the same
// answer as for an unknown code.
//
// Of one dispatch, a [Release], a redemption, the first accepted [Dispatched], the lapse
// of the lease, and the end of the dispatch 15 minutes after the claim when the code is
// not redeemed take effect one at a time, never two of them together. A release that
// takes effect first makes the code unusable. After a redemption that takes effect
// first, the lease does not lapse, and a release is refused 409 machine_exists. After a
// lapse that takes effect first, a redemption is refused.
//
// The run code stops working the moment the lease lapses, and when the dispatch ends
// for any reason, a release among them. After dispatched, the code can be redeemed
// until 15 minutes from the claim. When the machine has not redeemed the code within 15
// minutes of the claim, the control plane ends the dispatch, so the next [Dispatched]
// is refused 410 gone, and the starter deletes the machine. After the redemption, the
// 15 minutes from the claim no longer end the dispatch. The code is used once: a
// repeated redemption by the same machine, as the run_answer of
// https://qory.dev/contracts/forager/v1/enrolment.schema.json describes, gets the same
// answer. Once the code is redeemed, a release is refused 409 machine_exists, even
// before dispatched, because a machine exists.
type ClaimAnswer struct {
	Version    int    `json:"version"`
	Revision   int    `json:"revision"`
	DispatchID string `json:"dispatch_id"`
	PoolID     string `json:"pool_id"`
	LeaseUntil string `json:"lease_until"`
	Machine    string `json:"machine,omitempty"`
	RunCode    string `json:"run_code"`
	GatewayURL string `json:"gateway_url"`
	// MaxLifetimeMinutes is the minutes after which the machine powers itself off,
	// counted from when it starts, whatever it runs: from 15 to 1470. The starter passes
	// it to the machine with the run code, and deletes a machine of its node pool that
	// has run longer than MaxLifetimeMinutes plus 15 minutes, counted from the
	// vendor's launch time.
	MaxLifetimeMinutes int `json:"max_lifetime_minutes"`
}

// Dispatched is the body of POST /v1/dispatches/<dispatch_id>/dispatched,
// dispatched.schema.json: the starter created the machine for the dispatch and passed
// it the run code and the gateway URL. The starter sends the first dispatched of a
// dispatch before the claim answer's LeaseUntil. The lease lapses only when neither the
// first dispatched nor the redemption of the run code came before LeaseUntil: the
// dispatch is then stale, the run code stops working, and the control plane may give
// the work to another claim. Once the run code is redeemed, the dispatch never goes
// stale, and its work is never given to another claim while the dispatch lives. A first
// dispatched at or after LeaseUntil on a stale dispatch is refused 409 lease_expired,
// and the starter then stops the machine it created for that dispatch. A first
// dispatched at or after LeaseUntil on a dispatch whose run code was redeemed before
// LeaseUntil is accepted, because that dispatch is not stale. After dispatched, the
// code can be redeemed until 15 minutes from the claim.
//
// The starter sends dispatched again every 60 seconds for each machine it has, with the
// same External. Each send is built anew, with a later Time and a new signature, as
// https://qory.dev/contracts/integration/v1/definitions.schema.json describes for a
// request sent again, so the replay rule does not refuse it. Only the first dispatched
// of a dispatch is recorded, and nothing of a later one is. While the dispatch lives,
// the answer is a signed 204 with no body, whether the run code was redeemed or not.
// Once the dispatch has ended, a dispatched is refused 410 gone, whether the run code
// was redeemed or not, and the starter then sends the [MachineLog] and deletes the
// machine. When the machine has not redeemed the run code within 15 minutes of the
// claim, the control plane ends the dispatch, so the next dispatched is refused 410
// gone. After dispatched, a [Release] is refused 409 machine_exists.
type Dispatched struct {
	Version  int       `json:"version"`
	Time     string    `json:"time"`
	External *External `json:"external,omitempty"`
}

// External is a job at the vendor that runs it.
type External struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
	URL  string `json:"url"`
}

// Release is the body of POST /v1/dispatches/<dispatch_id>/release,
// release.schema.json: the starter gives the dispatch back before it says
// [Dispatched], and Reason says why. A release is accepted only before dispatched, and
// only while the run code is not redeemed. It ends the dispatch and frees the slot, and
// the run code stops working at that moment. Whether the work is claimed again is the
// control plane's decision: the starter does not ask for it. After dispatched, a
// release is refused 409 machine_exists. Once the run code is redeemed, a release is
// refused 409 machine_exists even before dispatched, because a machine exists. A
// release and a redemption of the same dispatch take effect one after the other, never
// both. A release that takes effect first makes the run code unusable. After a
// redemption that takes effect first, the release is refused 409 machine_exists.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
}

// MachineLog is the body of POST /v1/dispatches/<dispatch_id>/machine-log,
// machine-log.schema.json: the starter uploads the log of the machine it created for
// the dispatch, from the machine's creation to its end. Log has no U+0000. JSON
// escaping can make the body larger than Log, so the starter keeps the last part of the
// log such that Log is at most 768 KiB as UTF-8 and the request body, as serialized and
// sent, is at most 1 MiB, and sets Truncated when it cut anything. Before the upload,
// the starter replaces each line that contains qak_, qrk_ or qecr_. The control plane
// accepts one machine log per dispatch, before or after the dispatch ends, and answers
// a signed 204 with no body. A dispatch that is unknown, or of another node pool, is
// refused 403 forbidden, and a second upload 409 machine_log_exists.
type MachineLog struct {
	Version   int    `json:"version"`
	Time      string `json:"time"`
	Log       string `json:"log"`
	Truncated bool   `json:"truncated,omitempty"`
}
