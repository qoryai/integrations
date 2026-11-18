package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// The names of the submit program contract, contracts/run/v1/submit-program/README.md,
// in one place: the inputs that qory sets for the program, the keys of the lines the
// program prints, and the limits. The README table of inputs and the README table of
// keys list exactly these names, and a test pins it.
const (
	// SubmitEnvSystem is the host name of the repository's system, such as
	// code.example.com.
	SubmitEnvSystem = "QORY_SUBMIT_SYSTEM"
	// SubmitEnvPath is the repository's path on its system, such as acme/shop.
	SubmitEnvPath = "QORY_SUBMIT_PATH"
	// SubmitEnvBranch is the head of the draft: with a work item, qory/ and the work item's
	// public id, which the run pushed; with none, the branch that qory makes for the run,
	// qory/run-<run id>, which the program pushes.
	SubmitEnvBranch = "QORY_SUBMIT_BRANCH"
	// SubmitEnvBaseBranch is the branch that the draft is based on. Unset when qory did
	// not read it.
	SubmitEnvBaseBranch = "QORY_SUBMIT_BASE_BRANCH"
	// SubmitEnvBaseCommit is the commit that the run started at.
	SubmitEnvBaseCommit = "QORY_SUBMIT_BASE_COMMIT"
	// SubmitEnvCommits is the commits of the run, from the base to the head, oldest
	// first, full commit ids joined by single spaces.
	SubmitEnvCommits = "QORY_SUBMIT_COMMITS"
	// SubmitEnvDraft is what to do with the draft: DraftOpen, DraftUpdate or
	// SubmitDraftNone.
	SubmitEnvDraft = "QORY_SUBMIT_DRAFT"
	// SubmitEnvDraftNumber is the number of the draft to update. Set exactly with
	// DraftUpdate.
	SubmitEnvDraftNumber = "QORY_SUBMIT_DRAFT_NUMBER"
	// SubmitEnvTitle is the title of the draft. Set unless SubmitDraftNone.
	SubmitEnvTitle = "QORY_SUBMIT_TITLE"
	// SubmitEnvPreviousTitle is the title of the previous submission version of the work
	// item. Unset on the first version, and when that version had no text.
	SubmitEnvPreviousTitle = "QORY_SUBMIT_PREVIOUS_TITLE"
	// SubmitEnvDescriptionFile is the path of a UTF-8 file with the run's text for
	// the description: the content of the block, without the marker lines. Set unless
	// SubmitDraftNone.
	SubmitEnvDescriptionFile = "QORY_SUBMIT_DESCRIPTION_FILE"
)

// SubmitDraftNone is the value of [SubmitEnvDraft] for no draft. The other two values are
// [DraftOpen] and [DraftUpdate], the actions of a submission entry.
const SubmitDraftNone = "none"

// The keys of the result lines that the program prints on standard output, each line
// "<key> <value>", and for [SubmitKeyCommitURL] "<key> <commit> <address>".
const (
	SubmitKeyDraftNumber = "draft_number"
	SubmitKeyDraftURL    = "draft_url"
	SubmitKeyDraftTitle  = "draft_title"
	SubmitKeyDraftState  = "draft_state"
	SubmitKeyDraftAction = "draft_action"
	SubmitKeyCommitURL   = "commit_url"
)

// The marker lines of the draft's description: the run's text sits between them.
const (
	SubmitMarkerStart = "<!-- qory:start -->"
	SubmitMarkerEnd   = "<!-- qory:end -->"
)

// SubmitPrintedLines is how many lines of the program's standard error qory prints,
// SubmitLinePrefix in front of each, with secrets masked.
const (
	SubmitPrintedLines = 20
	SubmitLinePrefix   = "qory run: submit program: "
)

// The exit statuses of the submit program that qory reads the result lines after: 0 the
// program finished, [SubmitExitForge] a call to the forge failed, [SubmitExitInput] the
// program could not use its inputs. After [SubmitExitLogin], the person has no login of
// the forge's command line, and after any other status, a time-out, or when the program
// did not start, qory reads no line.
const (
	SubmitExitDone  = 0
	SubmitExitForge = 1
	SubmitExitInput = 2
	SubmitExitLogin = 3
)

// SubmitTimeLimit is how long a submit program may run. qory stops a program that runs
// longer, so a local `qory run` never waits on the program for more.
const SubmitTimeLimit = 120 * time.Second

// SubmitLoginLine is the first line that a program prints on standard error with exit
// status [SubmitExitLogin], for a forge whose command line is gh. A program for another
// forge prints the login command of that forge's command line in its place.
const SubmitLoginLine = "Run `gh auth login`, then `qory run` again."

// The reasons that qory records for a draft that it asked for and the program did not
// make. [SubmitProgramFailed]: the program did not start, ended with status 0, 1 or 2
// and printed no whole draft, or ended with a status or a signal that the contract
// does not list. [SubmitProgramTimeout]: the program ran past [SubmitTimeLimit], and
// qory read no line. [SubmitLoginNeeded]: the program ended with [SubmitExitLogin].
const (
	SubmitProgramFailed  = "program-failed"
	SubmitProgramTimeout = "program-timeout"
	SubmitLoginNeeded    = "login-needed"
)

// The most characters of an address and of a title that a result line has.
const (
	submitMaxURL   = 2048
	submitMaxTitle = 256
)

// SubmitLineError is a result line that qory does not record: Line is its number, from
// 1, and Reason is why.
type SubmitLineError struct {
	Line   int
	Reason string
}

// SubmitResult is what qory records from the result lines of a program: Draft, nil when
// the lines do not make a whole draft, CommitURLs by commit id, and Invalid, the lines
// that qory did not record, in the order of their numbers.
type SubmitResult struct {
	Draft      *SubmittedDraft
	CommitURLs map[string]string
	Invalid    []SubmitLineError
}

var submitNumber = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)

// submitRules are the patterns that the schemas give to an address and to a title,
// compiled from the schemas, so that a result line passes the rule that the adapter's
// reply has.
var submitRules = sync.OnceValue(func() struct{ url, line, title *regexp.Regexp } {
	pattern := func(file, def string, root fs.FS) *regexp.Regexp {
		b, err := fs.ReadFile(root, file)
		if err != nil {
			panic(err)
		}
		var doc struct {
			Defs map[string]struct {
				Pattern string `json:"pattern"`
			} `json:"$defs"`
		}
		if err := json.Unmarshal(b, &doc); err != nil {
			panic(err)
		}
		return regexp.MustCompile(doc.Defs[def].Pattern)
	}
	return struct{ url, line, title *regexp.Regexp }{
		url:   pattern("definitions.schema.json", "url", FS),
		line:  pattern("description.schema.json", "line", FS),
		title: pattern("definitions.schema.json", "draft_title", FS),
	}
})

// ParseSubmitLines reads the standard output of a submit program for a repository on
// system, the lines the README of the contract describes, and returns what qory
// records. draft is the value that qory gave the program in [SubmitEnvDraft], and commits
// lists the commit ids that qory gave it, the base among them.
//
// qory strips one carriage return at the end of each line first. A line that is not
// valid is left out and listed in Invalid; the lines before and after it count. A blank
// line is left out and is not listed. For the first line of each draft key, and for the
// commits, the rules are those of the README. A later line with a key that is there
// already is left out. Draft is set only when the five draft lines are there and valid,
// the action is [DraftKept] for a draft that is [DraftStateClosed] or
// [DraftStateMerged], and draft is not [SubmitDraftNone]: with [SubmitDraftNone], every
// draft line is left out.
func ParseSubmitLines(output, system, draft string, commits []string) SubmitResult {
	res := SubmitResult{CommitURLs: map[string]string{}}
	known := map[string]bool{}
	for _, c := range commits {
		known[c] = true
	}
	bad := func(n int, reason string) { res.Invalid = append(res.Invalid, SubmitLineError{n, reason}) }
	values, at := map[string]string{}, map[string]int{}
	lines := strings.Split(output, "\n")
	// Text after the last line feed is not a line: the program may have been cut off.
	last := len(lines) - 1
	if lines[last] != "" {
		bad(last+1, "the line has no line feed at its end")
	}
	for i, line := range lines[:last] {
		n := i + 1
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if submitControl(line) {
			bad(n, "the line has a control character")
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok || value == "" {
			bad(n, "the line is not a key and a value")
			continue
		}
		if key == SubmitKeyCommitURL {
			commit, addr, ok := strings.Cut(value, " ")
			switch {
			case !ok:
				bad(n, "the line has no address")
			case !known[commit]:
				bad(n, "the commit is not one that qory gave the program")
			case res.CommitURLs[commit] != "":
				bad(n, "the commit has an address already")
			default:
				if err := submitAddress(addr, system); err != nil {
					bad(n, err.Error())
					continue
				}
				res.CommitURLs[commit] = addr
			}
			continue
		}
		if draft == SubmitDraftNone && submitDraftKey(key) {
			bad(n, "QORY_SUBMIT_DRAFT is none, so qory records no draft")
			continue
		}
		if err := submitDraftValue(key, value, system); err != nil {
			bad(n, err.Error())
			continue
		}
		if _, dup := values[key]; dup {
			bad(n, "the key is there already")
			continue
		}
		values[key], at[key] = value, n
	}
	switch {
	case len(values) == 5:
		if (values[SubmitKeyDraftState] == DraftStateClosed || values[SubmitKeyDraftState] == DraftStateMerged) &&
			values[SubmitKeyDraftAction] != DraftKept {
			bad(at[SubmitKeyDraftAction], "a draft that is closed or merged has the action "+DraftKept)
			break
		}
		number, _ := strconv.Atoi(values[SubmitKeyDraftNumber])
		res.Draft = &SubmittedDraft{Number: number, URL: values[SubmitKeyDraftURL], Title: values[SubmitKeyDraftTitle],
			State: values[SubmitKeyDraftState], Action: values[SubmitKeyDraftAction]}
	case len(values) > 0:
		var missing []string
		for _, key := range submitDraftKeys {
			if _, ok := values[key]; !ok {
				missing = append(missing, key)
			}
		}
		for _, n := range at {
			bad(n, "the draft needs five lines, and "+strings.Join(missing, ", ")+" is missing; the line is left out")
		}
	}
	slices.SortFunc(res.Invalid, func(a, b SubmitLineError) int { return a.Line - b.Line })
	return res
}

var submitDraftKeys = []string{SubmitKeyDraftNumber, SubmitKeyDraftURL, SubmitKeyDraftTitle, SubmitKeyDraftState, SubmitKeyDraftAction}

func submitDraftKey(key string) bool { return slices.Contains(submitDraftKeys, key) }

// SubmitFailure is the reason qory records for a draft that the program did not make:
// Reason, and Exit, the program's exit status. Exit is nil for [SubmitProgramTimeout],
// which has no exit status, and 1 for [SubmitProgramFailed] after a program that did not
// start or ended with a status or a signal that the contract does not list.
type SubmitFailure struct {
	Reason string
	Exit   *int
}

// SubmitRecord is what qory records for one repository after the program ended: the
// [SubmitResult] of its lines, and Failed, nil unless draft was not [SubmitDraftNone]
// and the program made no draft.
type SubmitRecord struct {
	SubmitResult
	Failed *SubmitFailure
}

// SubmitEnd is how a submit program ended. Started is false when it did not start.
// TimedOut is true when it ran past [SubmitTimeLimit] and qory stopped it; Exit is then
// not read. Otherwise Exit is the exit status, or -1 when a signal ended the program.
type SubmitEnd struct {
	Started  bool
	TimedOut bool
	Exit     int
}

// ReadSubmitRun is what qory records after a program ended. A program that timed out
// is recorded as [SubmitProgramTimeout]. After [SubmitExitLogin] it is recorded as
// [SubmitLoginNeeded] with Exit 3. A program that did not start, or ended with a
// signal or with a status other than 0 to 3, is recorded as [SubmitProgramFailed] with
// Exit 1. In each of these cases qory reads no line. Otherwise (status 0, 1 or 2) it reads
// the lines with [ParseSubmitLines], and when draft is not [SubmitDraftNone] and they
// make no draft, records [SubmitProgramFailed] with the program's exit status.
func ReadSubmitRun(end SubmitEnd, output, system, draft string, commits []string) SubmitRecord {
	failed := func(reason string, exit *int) SubmitRecord {
		rec := SubmitRecord{SubmitResult: SubmitResult{CommitURLs: map[string]string{}}}
		if draft != SubmitDraftNone {
			rec.Failed = &SubmitFailure{reason, exit}
		}
		return rec
	}
	status := func(n int) *int { return &n }
	switch {
	case end.TimedOut:
		return failed(SubmitProgramTimeout, nil)
	case !end.Started || end.Exit < SubmitExitDone || end.Exit > SubmitExitLogin:
		return failed(SubmitProgramFailed, status(1))
	case end.Exit == SubmitExitLogin:
		return failed(SubmitLoginNeeded, status(SubmitExitLogin))
	}
	rec := SubmitRecord{SubmitResult: ParseSubmitLines(output, system, draft, commits)}
	if draft != SubmitDraftNone && rec.Draft == nil {
		rec.Failed = &SubmitFailure{SubmitProgramFailed, status(end.Exit)}
	}
	return rec
}

// submitControl reports whether s has a control character: U+0000 to U+001F or U+007F.
func submitControl(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

// submitDraftValue checks the value of one of the five draft keys.
func submitDraftValue(key, value, system string) error {
	switch key {
	case SubmitKeyDraftNumber:
		if !submitNumber.MatchString(value) {
			return errors.New("the number is not a whole number of 1 to 9 digits, 1 or more")
		}
	case SubmitKeyDraftURL:
		return submitAddress(value, system)
	case SubmitKeyDraftTitle:
		if !utf8.ValidString(value) {
			return errors.New("the title is not UTF-8")
		}
		if n := utf8.RuneCountInString(value); n > submitMaxTitle {
			return fmt.Errorf("the title has %d characters, over the limit of %d", n, submitMaxTitle)
		}
		if !submitRules().line.MatchString(value) || !submitRules().title.MatchString(value) {
			return errors.New("the title has a control, bidirectional or invisible character, or only spaces")
		}
	case SubmitKeyDraftState:
		if !slices.Contains([]string{DraftStateDraft, DraftStateOpen, DraftStateClosed, DraftStateMerged}, value) {
			return fmt.Errorf("the state is none of %s, %s, %s and %s", DraftStateDraft, DraftStateOpen, DraftStateClosed, DraftStateMerged)
		}
	case SubmitKeyDraftAction:
		if value != DraftOpened && value != DraftUpdated && value != DraftKept {
			return fmt.Errorf("the action is none of %s, %s and %s", DraftOpened, DraftUpdated, DraftKept)
		}
	default:
		return errors.New("the key is not one of the contract's")
	}
	return nil
}

// submitAddress checks that addr passes the pattern of definitions.schema.json,
// $defs.url, is at most 2048 characters, is on system, and has a path and no port, query
// or fragment.
func submitAddress(addr, system string) error {
	if len(addr) > submitMaxURL {
		return fmt.Errorf("the address is over %d characters", submitMaxURL)
	}
	if !submitRules().url.MatchString(addr) {
		return errors.New("the address is not an https URL as definitions.schema.json describes it")
	}
	u, err := url.Parse(addr)
	switch {
	case err != nil:
		return errors.New("the address is not a URL")
	case u.Port() != "" || strings.HasSuffix(u.Host, ":"):
		return errors.New("the address has a port")
	case u.RawQuery != "" || u.ForceQuery || strings.Contains(addr, "#"):
		return errors.New("the address has a query or a fragment")
	case !strings.EqualFold(u.Hostname(), system):
		return errors.New("the address is not on the repository's system")
	case u.Path == "" || u.Path == "/":
		return errors.New("the address has no path")
	}
	return nil
}

// MarkedDescription returns the description of a draft after its run's text is
// written: existing is the draft's description now, empty for a draft being opened, and
// text the run's text, the content of the block without the marker lines.
//
// The block is [SubmitMarkerStart], a line ending, text, a line ending and
// [SubmitMarkerEnd]. The line ending is CRLF when the first line ending of existing is
// CRLF, otherwise LF. Only the line endings that this function adds use it: the ones
// next to the marker lines, and the blank line before a block added at the end. The line
// endings inside text stay as given. A line is a marker line when it is the marker followed by nothing
// but spaces, tabs or a carriage return. The end line is the first end marker line that
// has a start marker line before it, and the start line is the last start marker line
// before the end line. The block replaces the text from the start of the start line to
// the end marker, and every other byte of existing stays, the spaces, tabs and carriage
// return after the end marker included. Without such a pair, as when a person removed a
// marker, existing stays whole and the block follows it after a blank line.
func MarkedDescription(existing, text string) string {
	eol := "\n"
	if i := strings.Index(existing, "\n"); i > 0 && existing[i-1] == '\r' {
		eol = "\r\n"
	}
	block := SubmitMarkerStart + eol + text + eol + SubmitMarkerEnd
	if existing == "" {
		return block
	}
	lines := strings.Split(existing, "\n")
	start := -1
	for i, line := range lines {
		switch {
		case isMarker(line, SubmitMarkerStart):
			start = i
		case start >= 0 && isMarker(line, SubmitMarkerEnd):
			head := strings.Join(lines[:start], "\n")
			if start > 0 {
				head += "\n"
			}
			tail := strings.Join(lines[i+1:], "\n")
			if i+1 < len(lines) {
				tail = "\n" + tail
			}
			// Spaces, tabs and a carriage return after the end marker stay.
			return head + block + lines[i][len(strings.TrimRight(lines[i], " \t\r")):] + tail
		}
	}
	rest, endings := existing, 0
	for endings < 2 {
		switch {
		case strings.HasSuffix(rest, "\r\n"):
			rest = rest[:len(rest)-2]
		case strings.HasSuffix(rest, "\n"):
			rest = rest[:len(rest)-1]
		default:
			return existing + strings.Repeat(eol, 2-endings) + block
		}
		endings++
	}
	return existing + block
}

// isMarker reports whether line is marker, alone on its line.
func isMarker(line, marker string) bool {
	return strings.TrimRight(line, " \t\r") == marker
}

// NextTitle returns the title of a draft after an update: next, the title of the new
// submission version, while current, the draft's title now, equals previous, the title
// of the previous version; otherwise current, which a person edited. forge is the text
// that the forge puts before the title of a draft that is not ready, or empty when it
// puts none: the comparison is made without it, and the new title keeps it. With an empty
// previous, as when the previous version had no text, the title stays as it is.
func NextTitle(current, previous, next, forge string) string {
	shown := current
	if forge != "" {
		shown = strings.TrimPrefix(current, forge)
	}
	if previous != "" && shown == previous {
		return current[:len(current)-len(shown)] + next
	}
	return current
}
