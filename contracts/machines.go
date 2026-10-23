package contracts

import "encoding/json"

// MaxStartData is the most bytes a make request's start data has, decoded.
const MaxStartData = 16 << 10

// MaxMachinesAnswer is the most bytes an answer of a machine adapter has, an error
// among them.
const MaxMachinesAnswer = 1 << 20

// The states a machine has in a list answer, machines/v1 list-answer.schema.json.
const (
	MachineStarting = "starting"
	MachineRunning  = "running"
	MachineStopped  = "stopped"
	MachineDeleting = "deleting"
)

// MachineGone is a delete answer's state when no machine with the request's id exists
// at the vendor, also when none ever did. A delete answer's state is
// [MachineDeleting] or MachineGone.
const MachineGone = "gone"

// The kinds of a [MachinesFailure], machines/v1 error.schema.json. A usage error is a
// failure of kind FailureDefect.
const (
	// FailureLimit is a failure because the request exceeded a limit at the vendor,
	// such as a quota of machines.
	FailureLimit = "limit"
	// FailureRetry is a failure that a retry may clear, such as a timeout.
	FailureRetry = "retry"
	// FailureDefect is a failure of the request, the settings or the command line, which
	// a retry repeats.
	FailureDefect = "defect"
	// FailureOther is any other failure.
	FailureOther = "other"
)

// MachinesCheck is the request of `<adapter> check --settings FILE`, machines/v1
// check.schema.json, on standard input: the adapter shows that its rights at the
// vendor and its settings work, and makes no machine.
type MachinesCheck struct {
	Version int `json:"version"`
}

// MachinesCheckAnswer is the answer of check, check-answer.schema.json: the empty
// object.
type MachinesCheckAnswer struct{}

// MachinesMake is the request of `<adapter> make --settings FILE`, make.schema.json:
// the adapter makes one machine for the dispatch in Key, in the node pool in Pool, of
// the size in Size. One Key makes at most one machine. A second make with the same Key
// makes no machine, and its answer is the ID of the machine that the first make with
// that Key made. A Size that the adapter's settings do not contain fails with kind
// [FailureDefect], because a retry fails the same way. StartData is standard base64
// with padding of at most [MaxStartData] bytes, which the adapter passes to the machine
// unread. It contains a run code, so it travels only on standard input, never in an
// argument, and the adapter never writes it to a log.
type MachinesMake struct {
	Version   int    `json:"version"`
	Key       string `json:"key"`
	Pool      string `json:"pool"`
	Size      string `json:"size"`
	StartData string `json:"start_data"`
}

// MachinesMakeAnswer is the answer of make, make-answer.schema.json: the vendor's id
// of the machine the adapter made for the request's Key, by this make or by the first
// make with that Key.
type MachinesMakeAnswer struct {
	ID string `json:"id"`
}

// MachinesList is the request of `<adapter> list --settings FILE`, list.schema.json:
// the adapter lists the machines it made for the node pool in Pool.
type MachinesList struct {
	Version int    `json:"version"`
	Pool    string `json:"pool"`
}

// MachinesListAnswer is the answer of list, list-answer.schema.json. A nil Machines is
// written as the empty list.
type MachinesListAnswer struct {
	Machines []Machine `json:"machines"`
}

// MarshalJSON writes a, with a nil Machines as the empty list, which the schema
// accepts, in place of null, which it refuses.
func (a MachinesListAnswer) MarshalJSON() ([]byte, error) {
	type plain MachinesListAnswer
	if a.Machines == nil {
		a.Machines = []Machine{}
	}
	return json.Marshal(plain(a))
}

// Machine is one machine of a list answer: its id at the vendor, the dispatch it was
// made for in Key, its State, one of [MachineStarting], [MachineRunning],
// [MachineStopped] and [MachineDeleting], and when the vendor started it, RFC 3339 in
// UTC with whole seconds.
type Machine struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	State   string `json:"state"`
	Started string `json:"started"`
}

// MachinesLog is the request of `<adapter> log --settings FILE`, log.schema.json: the
// adapter reads the console text of the machine in ID.
type MachinesLog struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
}

// MachinesLogAnswer is the answer of log, log-answer.schema.json. Log is the machine's
// console text, with no U+0000. The adapter keeps the last part of the text such that
// Log is at most 768 KiB as UTF-8 and the answer, as written, is at most
// [MaxMachinesAnswer] bytes.
type MachinesLogAnswer struct {
	Log string `json:"log"`
}

// MachinesDelete is the request of `<adapter> delete --settings FILE`,
// delete.schema.json: the adapter deletes the machine in ID, or starts to delete it.
// The answer to a delete of an ID that matches no machine at the vendor has the
// state [MachineGone].
type MachinesDelete struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
}

// MachinesDeleteAnswer is the answer of delete, delete-answer.schema.json. State is
// [MachineDeleting] or [MachineGone].
type MachinesDeleteAnswer struct {
	State string `json:"state"`
}

// MachinesError is what an adapter writes on standard output, error.schema.json, before
// it exits with status 1: when check, make, list, log or delete fails, and on a usage
// error. The adapter exits with status 0 on success and 1 on failure, and with no
// other status.
type MachinesError struct {
	Error MachinesFailure `json:"error"`
}

// MachinesFailure is a [MachinesError]'s failure: its Kind, one of [FailureLimit],
// [FailureRetry], [FailureDefect] and [FailureOther], and a Message of display text, 1
// to 200 characters.
type MachinesFailure struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
