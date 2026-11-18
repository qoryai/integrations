package contracts

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
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
	// SubmitEnvBranch is the branch that the run pushed, the head of the draft.
	SubmitEnvBranch = "QORY_SUBMIT_BRANCH"
	// SubmitEnvBaseBranch is the branch that the draft is based on. Unset when qory did
	// not read it.
	SubmitEnvBaseBranch = "QORY_SUBMIT_BASE_BRANCH"
	// SubmitEnvBaseCommit is the commit that the run started at.
	SubmitEnvBaseCommit = "QORY_SUBMIT_BASE_COMMIT"
	// SubmitEnvCommits is the commits of the run, from the base to the head, oldest
	// first, full commit ids joined by single spaces.
	SubmitEnvCommits = "QORY_SUBMIT_COMMITS"
	// SubmitEnvDraft is what to do with the draft: SubmitDraftOpen, SubmitDraftUpdate or
	// SubmitDraftNone.
	SubmitEnvDraft = "QORY_SUBMIT_DRAFT"
	// SubmitEnvDraftNumber is the number of the draft to update. Set exactly with
	// SubmitDraftUpdate.
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

// The values of [SubmitEnvDraft].
const (
	SubmitDraftOpen   = "open"
	SubmitDraftUpdate = "update"
	SubmitDraftNone   = "none"
)

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
// SubmitLinePrefix in front of each.
const (
	SubmitPrintedLines = 20
	SubmitLinePrefix   = "qory run: submit program: "
)

// The exit statuses of the submit program that qory reads the result lines after: 0 the
// program finished, [SubmitExitForge] a call to the forge failed, [SubmitExitInput] the
// program could not use its inputs. Any other status, or a program that did not start,
// records nothing.
const (
	SubmitExitDone  = 0
	SubmitExitForge = 1
	SubmitExitInput = 2
)

// The most bytes of an address, and of a title in characters, that a result line has.
const (
	submitMaxURL   = 2048
	submitMaxTitle = 256
)

// SubmitLineError is a result line that qory does not record: Line is its number, from
// 1, and Reason says why.
type SubmitLineError struct {
	Line   int
	Reason string
}

// SubmitResult is what qory records from the result lines of a program: Draft, nil when
// the lines do not make a whole draft, CommitURLs by commit id, and Invalid, the lines
// that qory did not record.
type SubmitResult struct {
	Draft      *SubmittedDraft
	CommitURLs map[string]string
	Invalid    []SubmitLineError
}

var (
	submitNumber = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)
	submitCommit = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
)

// ParseSubmitLines reads the standard output of a submit program for a repository on
// system, the lines the README of the contract describes, and returns what qory
// records. commits lists the commit ids that qory gave the program, the base among
// them. A line that is not valid is left out and listed in Invalid; the lines before
// and after it count. A blank line is left out and is not listed. Draft is set only
// when each of the five draft lines is there once and valid.
func ParseSubmitLines(output, system string, commits []string) SubmitResult {
	res := SubmitResult{CommitURLs: map[string]string{}}
	known := map[string]bool{}
	for _, c := range commits {
		known[c] = true
	}
	draft := map[string]string{}
	lines := strings.Split(output, "\n")
	// Text after the last line feed is not a line: the program may have been cut off.
	last := len(lines) - 1
	if lines[last] != "" {
		res.Invalid = append(res.Invalid, SubmitLineError{last + 1, "the line has no line feed at its end"})
	}
	for i, line := range lines[:last] {
		n := i + 1
		if line == "" {
			continue
		}
		if bad := submitControl(line); bad {
			res.Invalid = append(res.Invalid, SubmitLineError{n, "the line has a control character"})
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok || value == "" {
			res.Invalid = append(res.Invalid, SubmitLineError{n, "the line is not a key and a value"})
			continue
		}
		if key == SubmitKeyCommitURL {
			commit, addr, ok := strings.Cut(value, " ")
			switch {
			case !ok:
				res.Invalid = append(res.Invalid, SubmitLineError{n, "the line has no address"})
			case !known[commit]:
				res.Invalid = append(res.Invalid, SubmitLineError{n, "the commit is not one that qory gave the program"})
			case res.CommitURLs[commit] != "":
				res.Invalid = append(res.Invalid, SubmitLineError{n, "the commit has an address already"})
			default:
				if err := submitAddress(addr, system); err != nil {
					res.Invalid = append(res.Invalid, SubmitLineError{n, err.Error()})
					continue
				}
				res.CommitURLs[commit] = addr
			}
			continue
		}
		if err := submitDraftValue(key, value, system); err != nil {
			res.Invalid = append(res.Invalid, SubmitLineError{n, err.Error()})
			continue
		}
		if _, dup := draft[key]; dup {
			res.Invalid = append(res.Invalid, SubmitLineError{n, "the key is there already"})
			continue
		}
		draft[key] = value
	}
	if len(draft) == 5 {
		number, _ := strconv.Atoi(draft[SubmitKeyDraftNumber])
		res.Draft = &SubmittedDraft{Number: number, URL: draft[SubmitKeyDraftURL], Title: draft[SubmitKeyDraftTitle],
			State: draft[SubmitKeyDraftState], Action: draft[SubmitKeyDraftAction]}
	}
	return res
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
		if n := utf8.RuneCountInString(value); n > submitMaxTitle {
			return fmt.Errorf("the title has %d characters, over the limit of %d", n, submitMaxTitle)
		}
	case SubmitKeyDraftState:
		if value != DraftStateDraft && value != DraftStateOpen {
			return fmt.Errorf("the state is neither %s nor %s", DraftStateDraft, DraftStateOpen)
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

// submitAddress checks that addr is an https address of at most 2048 characters on
// system, with no user information, port, query or fragment.
func submitAddress(addr, system string) error {
	if len(addr) > submitMaxURL {
		return fmt.Errorf("the address is over %d characters", submitMaxURL)
	}
	u, err := url.Parse(addr)
	switch {
	case err != nil || strings.ContainsAny(addr, " \t"):
		return errors.New("the address is not a URL")
	case u.Scheme != "https":
		return errors.New("the address is not https")
	case u.User != nil:
		return errors.New("the address has user information")
	case u.Port() != "":
		return errors.New("the address has a port")
	case u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(addr, "#"):
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
// The block is [SubmitMarkerStart], a line feed, text, a line feed and
// [SubmitMarkerEnd]. When existing has a line that is [SubmitMarkerStart] followed
// later by a line that is [SubmitMarkerEnd], the first such pair, from the start of its
// first line to the end marker, is replaced by the block, and every other byte of
// existing stays, the spaces, tabs and carriage return after the end marker included. A
// line is a marker when it is the marker followed by nothing but spaces, tabs or a
// carriage return. Otherwise, as when a
// person removed a marker, existing stays whole and the block follows it after a blank
// line.
func MarkedDescription(existing, text string) string {
	block := SubmitMarkerStart + "\n" + text + "\n" + SubmitMarkerEnd
	if existing == "" {
		return block
	}
	lines := strings.Split(existing, "\n")
	start := -1
	for i, line := range lines {
		switch {
		case start < 0 && isMarker(line, SubmitMarkerStart):
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
	gap := 2 - min(2, len(existing)-len(strings.TrimRight(existing, "\n")))
	return existing + strings.Repeat("\n", gap) + block
}

// isMarker reports whether line is marker, alone on its line.
func isMarker(line, marker string) bool {
	return strings.TrimRight(line, " \t\r") == marker
}

// NextTitle returns the title of a draft after an update: next, the title of the new
// submission version, while current, the draft's title now, equals previous, the title
// of the previous version; otherwise current, which a person
// edited. An empty previous, as on the first version, changes nothing.
func NextTitle(current, previous, next string) string {
	if previous != "" && current == previous {
		return next
	}
	return current
}
