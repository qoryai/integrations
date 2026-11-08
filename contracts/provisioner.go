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

// MachineSpec is the size and kind of a machine, definitions.schema.json machine_spec:
// what a node pool offers in a [Claim], and what an instance admin defines for a
// machine type in the control plane. VCPUs is from 1 to 1024, MemoryMiB from 512 to
// 8388608 and DiskGiB from 1 to 65536. Arch is amd64 or arm64. Virtualization is true
// when the machine can run virtual machines inside it; on Linux, such a machine has
// /dev/kvm.
type MachineSpec struct {
	VCPUs          int    `json:"vcpus"`
	MemoryMiB      int    `json:"memory_mib"`
	DiskGiB        int    `json:"disk_gib"`
	Arch           string `json:"arch"`
	Virtualization bool   `json:"virtualization"`
}

// Fits reports whether the machine m fits spec: m's VCPUs, MemoryMiB and DiskGiB are
// each at least spec's, m's Arch is spec's, and m's Virtualization is true when spec's
// is true.
func (m MachineSpec) Fits(spec MachineSpec) bool {
	return m.VCPUs >= spec.VCPUs &&
		m.MemoryMiB >= spec.MemoryMiB &&
		m.DiskGiB >= spec.DiskGiB &&
		m.Arch == spec.Arch &&
		(m.Virtualization || !spec.Virtualization)
}

// Claim is the body of POST /v1/pools/claim, provisioner/v1 claim.schema.json. With an
// empty PoolID the claim covers the key's node pool, the one node pool the pool's access
// key belongs to. Offers are the machines the pool can create, 1 to 64 of them: for
// each, DiskGiB is the largest disk the pool gives that machine. An empty Offers is left
// out, the same as no offers. The control plane gives a claim with offers only work for
// a run whose machine type one of the offers fits. A claim without offers gets only work
// for a run of the instance's default machine type. A [ClaimAnswer] to a claim with
// offers has Machine and MachineSpec, and an answer to a claim without offers has
// neither. A provisioner that uses a machine adapter claims only after a successful
// check of the adapter, and sends the offers of the adapter's last successful check,
// [MachinesCheckAnswer], so each of its claims has offers.
type Claim struct {
	Version     int           `json:"version"`
	Time        string        `json:"time"`
	PoolID      string        `json:"pool_id,omitempty"`
	WaitSeconds int           `json:"wait_seconds"`
	Offers      []MachineSpec `json:"offers,omitempty"`
}

// ClaimAnswer is the body of the 200 answer to a claim, claim-answer.schema.json: a
// request for one machine. The starter refuses one whose Revision is not the revision it
// sent. Machine is the run's machine type, a name from the control plane's list of
// machine types, such as standard, with MachineSpec the spec of that type: the starter
// creates a machine that fits MachineSpec, with a disk of at least MachineSpec's
// DiskGiB. Machine is empty and MachineSpec nil when the claim had no offers: the run
// is of the instance's default machine type, and the starter creates its default
// machine. RunCode is the run code the control plane made at the claim. GatewayURL is
// the gateway that the node pool's choice resolved to at the claim: https, a host and
// an optional port, and nothing after them. The starter passes both to the machine it
// creates for the dispatch alone, and does not read the code. The control plane binds
// the code to that gateway, and refuses a redemption that another gateway relays with
// the same answer as for an unknown code. The machine redeems the code at the enrolment,
// https://qory.dev/contracts/forager/v1/enrolment.schema.json, which returns the run key
// and the run's documents, run/v1 documents.schema.json, in its signed 201.
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
	Version     int          `json:"version"`
	Revision    int          `json:"revision"`
	DispatchID  string       `json:"dispatch_id"`
	PoolID      string       `json:"pool_id"`
	LeaseUntil  string       `json:"lease_until"`
	Machine     string       `json:"machine,omitempty"`
	MachineSpec *MachineSpec `json:"machine_spec,omitempty"`
	RunCode     string       `json:"run_code"`
	GatewayURL  string       `json:"gateway_url"`
	// MaxLifetimeMinutes is the minutes after which the machine powers itself off,
	// counted from when it starts, whatever it runs: from 15 to 1470. The starter passes
	// it to the machine with the run code, and deletes a machine of its node pool that
	// has run longer than MaxLifetimeMinutes plus 15 minutes, counted from the
	// vendor's launch time.
	MaxLifetimeMinutes int `json:"max_lifetime_minutes"`
}

// Dispatched is the body of POST /v1/dispatches/<dispatch_id>/dispatched,
// dispatched.schema.json: the starter created the machine for the dispatch and passed it
// the run code and the gateway URL. The starter sends the first dispatched of a dispatch
// before the claim answer's LeaseUntil. The lease lapses only when neither the first
// dispatched nor the redemption of the run code came before LeaseUntil: the dispatch is
// then stale, the run code stops working, and the control plane may give the work to
// another claim. Once the run code is redeemed, the dispatch never goes stale, and its
// work is never given to another claim while the dispatch lives. A first dispatched at
// or after LeaseUntil on a stale dispatch is refused 409 lease_expired, and the starter
// then stops the machine it created for that dispatch. A first dispatched at or after
// LeaseUntil on a dispatch whose run code was redeemed before LeaseUntil is accepted,
// because that dispatch is not stale. After dispatched, the code can be redeemed until
// 15 minutes from the claim.
//
// The starter sends dispatched again every 60 seconds for each machine it has, with the
// same External and MachineType. Each send is built anew, with a later Time and a new
// signature, as https://qory.dev/contracts/integration/v1/definitions.schema.json
// describes for a request sent again, so the replay rule does not refuse it. Only the
// first dispatched of a dispatch is recorded, and nothing of a later one is. While the
// dispatch lives, the answer is a signed 204 with no body, whether the run code was
// redeemed or not. Once the dispatch has ended, a dispatched is refused 410 gone,
// whether the run code was redeemed or not, and the starter then sends the [MachineLog]
// and deletes the machine. A dispatched refused 409 lease_expired or 410 gone takes the
// slot of the dispatch again, if the slot was freed, until the control plane accepts
// [MachineEndGone] for the dispatch, or until the control plane frees it under the time
// limit for a slot, unless the control plane accepted [MachineEndGone] for it before, as
// claim.schema.json describes. When the machine has not redeemed the run code within 15
// minutes of the claim, the control plane ends the dispatch, so the next dispatched is
// refused 410 gone. After dispatched, a [Release] is refused 409 machine_exists.
//
// MachineType is the vendor's type of the machine the starter created for the job, such
// as example-type, and is empty when the starter has no type for the machine.
type Dispatched struct {
	Version     int       `json:"version"`
	Time        string    `json:"time"`
	External    *External `json:"external,omitempty"`
	MachineType string    `json:"machine_type,omitempty"`
}

// External is a job at the vendor that runs it.
type External struct {
	Type string `json:"type"`
	Ref  string `json:"ref"`
	URL  string `json:"url"`
}

// Release is the body of POST /v1/dispatches/<dispatch_id>/release,
// release.schema.json: the starter gives the dispatch back before it says [Dispatched],
// and Reason contains the cause. A release is accepted only before dispatched, and only
// while the run code is not redeemed. It ends the dispatch, and the run code stops
// working at that moment. Before the first machine event of the dispatch, as
// claim.schema.json defines it, the release frees the slot the dispatch took at the
// claim. Of the machine events, only a machine end can come before an accepted release.
// After an accepted release that follows a machine event, the dispatch keeps the slot
// until the control plane has accepted a machine end with the state [MachineEndGone] for
// the dispatch, machine-end.schema.json, or until the control plane frees it under the
// time limit for a slot, as claim.schema.json describes. The control plane may give the
// work to another claim; no member of the release changes that. After dispatched, a
// release is refused 409 machine_exists. Once the run code is redeemed, a release is
// refused 409 machine_exists even before dispatched, because a machine exists. A release
// and a redemption of the same dispatch take effect one after the other, never both. A
// release that takes effect first makes the run code unusable. After a redemption that
// takes effect first, the release is refused 409 machine_exists.
type Release struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	Reason  string `json:"reason"`
}

// MachineLog is the body of POST /v1/dispatches/<dispatch_id>/machine-log,
// machine-log.schema.json: the starter uploads the log of the machine it created for
// the dispatch, from the machine's creation to its end, or, when it created no machine
// for the dispatch, the error text of its attempt to create one. Log has no U+0000. JSON
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

// The states of a [MachineEnd], machine-end.schema.json. MachineEndDeleting: the
// starter's delete call was accepted, and the machine may still exist. MachineEndGone:
// the machine adapter's delete call returned that no machine with the id exists at the
// vendor, [MachineGone], or, in the no-record case, the starter has no record of a
// machine for the dispatch: a successful [MachinesListAnswer], received at or after the
// MaxLifetimeMinutes of the [ClaimAnswer] after the claim, contained no machine with
// the dispatch in Key. A failed list call is no proof. For a machine whose id the
// starter has, a missing entry in a list is no proof that the machine is gone, because
// a vendor's list can lag. For a starter that has no machine command, MachineEndGone
// means that no process for the dispatch runs, as [MachineEnd] describes.
// MachineEndDeleteFailed: the delete call failed, and the starter treats the machine as
// still existing.
const (
	MachineEndDeleting     = "deleting"
	MachineEndGone         = "gone"
	MachineEndDeleteFailed = "delete_failed"
)

// MachineEnd is the body of POST /v1/dispatches/<dispatch_id>/machine-end,
// machine-end.schema.json: the starter reports the result of deleting the machine it
// created for the dispatch, or that it has no record of a machine for the dispatch. The
// starter sends the call only for the machine that it created for the dispatch, or, with
// the state [MachineEndGone], for a dispatch for which it has no record of a machine.
// For a machine that the starter has, it sends the call after its delete call, repeats
// the delete call until that call returns gone, and then sends [MachineEndGone]. The
// starter sends [MachineEndGone] for every dispatch for which the command make,
// [MachinesMakeAnswer], returned a machine, every dispatch whose [Release] was refused
// 409 machine_exists, and every dispatch that is the Key of a [Machine] in a
// [MachinesListAnswer]. It sends [MachineEndGone] once the dispatch's machine is gone or
// once the no-record case of [MachineEndGone] applies. In the no-record case, a
// successful [MachinesListAnswer], received at or after the MaxLifetimeMinutes of the
// [ClaimAnswer] after the claim, contained no machine with the dispatch in Key. A failed
// list call is no proof. A starter that has no machine command (make, delete and list of
// a machine adapter) runs the work of a dispatch as a process on its host, and the end
// of that process is the end of the machine. Such a starter needs no list and no
// no-record case, and sends [MachineEndGone] in these cases. For a dispatch for which it
// started the process, it sends [MachineEndGone] after the process exits. When the
// starter could not start the process after an accepted [Dispatched], it sends
// [MachineEndGone] at once. For a dispatch whose [Dispatched] was refused 409
// lease_expired or 410 gone, it sends [MachineEndGone] once no process for the dispatch
// runs, because an earlier [Dispatched] may have been accepted with its answer lost.
// When a [Dispatched] sent again is refused while the process runs, the starter stops
// the process and sends [MachineEndGone] after the process exits. For a dispatch whose
// [Release] was refused 409 machine_exists, it sends [MachineEndGone] at once when no
// process for the dispatch runs. A released dispatch may still get [MachineEndGone] with
// no machine; the control plane accepts it with the signed 204. When the dispatch had no
// earlier machine event and no refusal took its slot again, its slot stays free;
// otherwise the accepted [MachineEndGone] frees the slot, because the release ended the
// dispatch. From the first machine event of the dispatch, as claim.schema.json defines
// it, the dispatch keeps the slot it took at the claim at least until the control plane
// accepts [MachineEndGone] for the dispatch. An accepted [MachineEndGone] frees the slot
// once the dispatch has ended or is stale, whether or not dispatched or a machine end
// with the state deleting came first. Until then, the dispatch keeps the slot, and the
// slot is freed when the dispatch ends or goes stale. Before the first machine event,
// the release, the lapse of the lease or the end of the dispatch frees the slot, as
// claim.schema.json describes. An accepted machine end other than [MachineEndGone] takes
// a freed slot again, even when the node pool is then over its instance limit; the
// dispatch then keeps it at least until the control plane accepts [MachineEndGone]. Such
// a machine end still gets the signed 204. An accepted [MachineEndGone] never takes a
// slot, and after an accepted [MachineEndGone] no call takes the slot again. Even when
// no [MachineEndGone] arrives, the control plane frees the slot under the time limit for
// a slot, claim.schema.json. That limit is MaxLifetimeMinutes of the [ClaimAnswer] plus
// 60 minutes after the latest of the claim, the first accepted dispatched, the first
// accepted redemption of the run code when no dispatched was accepted before it, the
// last accepted machine end up to the first accepted [MachineEndGone], and the last
// refusal of a kind that takes a slot again, whether or not that refusal took the slot
// again. An accepted dispatched or redemption sent again does not move the time limit.
// The starter has no limit on machines other than the node pool's instance limit.
// [MachineEndDeleting] and [MachineEndDeleteFailed] are optional, and a starter that has
// no machine command does not send them. The call may come any
// number of times, before or after the dispatch ends, and is never refused 410 gone or
// 409 lease_expired, because a machine usually ends after its dispatch. The starter
// sends one call at a time for a dispatch: it sends the next only after the answer to
// the one before, or after it gave that one up. The control plane applies the calls in
// the order it receives them and compares no time, and every accepted call replaces the
// state. MachineEndGone is final: a later call changes nothing and still gets the signed
// 204 with no body. A [MachineEndGone] after the time limit for a slot freed the slot
// also gets the signed 204. A signed 400, 403 or 404 refusal ends the starter's attempts
// to send this call for the dispatch, because the same call sent again cannot succeed;
// the delete calls go on. After any other failure to send [MachineEndGone], an unsigned
// answer among them, the starter keeps sending [MachineEndGone], each send built anew,
// until the control plane accepts it with the signed 204. After any other failure to
// send [MachineEndDeleting] or [MachineEndDeleteFailed], the starter may send that state
// again for a limited time, and then drops it.
type MachineEnd struct {
	Version int    `json:"version"`
	Time    string `json:"time"`
	State   string `json:"state"`
}
