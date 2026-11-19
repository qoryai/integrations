package contracts_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// submitFile reads a file of the submit program contract, or fails the test.
func submitFile(t *testing.T, name string) string {
	t.Helper()
	b, err := fs.ReadFile(contracts.RunFS, path.Join("submit-program", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// the commits that the output fixtures list, and the base: the same ids as in the
// fixtures, which have no other commit.
const (
	commitBase = "3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1"
	commitMade = "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c"
)

// TestTheSubmitProgramNamesAreInTheReadme pins that the README of the contract lists
// each input variable and each key of a result line that the Go constants define, once in
// its table, and no other QORY_SUBMIT_ variable or key, so a rename changes the
// constants and the README together.
func TestTheSubmitProgramNamesAreInTheReadme(t *testing.T) {
	readme := submitFile(t, "README.md")
	inputs := []string{
		contracts.SubmitEnvSystem, contracts.SubmitEnvPath, contracts.SubmitEnvBranch, contracts.SubmitEnvBaseBranch,
		contracts.SubmitEnvBaseCommit, contracts.SubmitEnvCommits, contracts.SubmitEnvDraft,
		contracts.SubmitEnvTitle, contracts.SubmitEnvDescriptionFile,
	}
	var rows []string
	for _, l := range strings.Split(readme, "\n") {
		if strings.HasPrefix(l, "| `QORY_SUBMIT_") {
			rows = append(rows, strings.Trim(strings.SplitN(l, "|", 3)[1], " `"))
		}
	}
	if !slices.Equal(rows, inputs) {
		t.Errorf("the table of inputs lists %v, and the constants define %v", rows, inputs)
	}
	for _, name := range regexp.MustCompile("QORY_[A-Z_]+").FindAllString(readme, -1) {
		if !slices.Contains(inputs, name) {
			t.Errorf("the README mentions %s, which no constant has", name)
		}
	}
	keys := []string{
		contracts.SubmitKeyDraftNumber, contracts.SubmitKeyDraftURL, contracts.SubmitKeyDraftTitle,
		contracts.SubmitKeyDraftState, contracts.SubmitKeyDraftAction, contracts.SubmitKeyCommitURL,
	}
	rows = nil
	for _, l := range strings.Split(readme, "\n") {
		if m := regexp.MustCompile("^\\| `((?:draft_|commit_)[a-z_]*)` \\|").FindStringSubmatch(l); m != nil {
			rows = append(rows, m[1])
		}
	}
	if !slices.Equal(rows, keys) {
		t.Errorf("the table of keys lists %v, and the constants define %v", rows, keys)
	}
	for _, want := range []string{
		contracts.SubmitMarkerStart, contracts.SubmitMarkerEnd, contracts.SubmitLinePrefix,
		"`" + contracts.DraftOpen + "`", "`" + contracts.SubmitDraftNone + "`",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not contain %q", want)
		}
	}
	if want := fmt.Sprintf("at most %d lines", contracts.SubmitPrintedLines); !strings.Contains(readme, want) {
		t.Errorf("the README does not contain %q", want)
	}
	// The contract has no relay, no JSON file and no wall.
	for _, bad := range []string{"QORY_PROXY", "QORY_SUBMISSION_FILE", "SSL_CERT_FILE", "proxy.sock", "jq "} {
		if strings.Contains(readme, bad) {
			t.Errorf("the README contains %q", bad)
		}
	}
}

// TestTheReadmeExamplesAreTheFixtures pins that the output shown in the README is the
// content of fixtures/output/github-like.txt, and that the table of exit statuses has a
// row for each exit constant and for any other status.
func TestTheReadmeExamplesAreTheFixtures(t *testing.T) {
	readme := submitFile(t, "README.md")
	if !strings.Contains(readme, "```text\n"+submitFile(t, "fixtures/output/github-like.txt")+"```") {
		t.Error("the README's example is not the content of fixtures/output/github-like.txt")
	}
	for _, row := range []string{
		fmt.Sprintf("| %d |", contracts.SubmitExitDone), fmt.Sprintf("| %d |", contracts.SubmitExitForge),
		fmt.Sprintf("| %d |", contracts.SubmitExitInput), fmt.Sprintf("| %d |", contracts.SubmitExitLogin),
		"| another status, or the program did not start |",
	} {
		if !strings.Contains(readme, row) {
			t.Errorf("the table of exit statuses has no row %q", row)
		}
	}
	for _, reason := range []string{contracts.SubmitProgramFailed, contracts.SubmitProgramTimeout, contracts.SubmitLoginNeeded} {
		if !strings.Contains(readme, "`"+reason+"`") {
			t.Errorf("the README does not define %s", reason)
		}
	}
	for _, want := range []string{
		fmt.Sprintf("%d seconds", int(contracts.SubmitTimeLimit.Seconds())),
		"\n" + contracts.SubmitLoginLine + "\n",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not contain %q", want)
		}
	}
}

var (
	given = []string{commitBase, commitMade}
	// both fixture forges
	hostCode, hostGit = "code.example.com", "git.example.net"
)

// TestTheOutputFixturesAreRead pins what qory records from each output fixture whose
// lines are all valid: a draft with its five members, or none, and an address for each
// commit it lists. One fixture is for a forge whose draft is a pull request, one for a
// forge whose draft is a merge request, and two for a draft that a person closed or
// merged.
func TestTheOutputFixturesAreRead(t *testing.T) {
	for _, c := range []struct {
		file, system, url, action, state string
		number                           int64
		commits                          int
	}{
		{"github-like.txt", hostCode, "https://code.example.com/acme/shop/pull/34", "opened", "draft", 34, 2},
		{"crlf.txt", hostCode, "https://code.example.com/acme/shop/pull/34", "opened", "draft", 34, 2},
		{"gitlab-like.txt", hostGit, "https://git.example.net/acme/shop/-/merge_requests/7", "updated", "draft", 7, 2},
		{"kept.txt", hostCode, "https://code.example.com/acme/shop/pull/34", "kept", "draft", 34, 2},
		{"closed-kept.txt", hostCode, "https://code.example.com/acme/shop/pull/34", "kept", "closed", 34, 2},
		{"merged-kept.txt", hostCode, "https://code.example.com/acme/shop/pull/34", "kept", "merged", 34, 2},
	} {
		r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/"+c.file), c.system, contracts.DraftOpen, given)
		if len(r.Invalid) != 0 {
			t.Errorf("%s: invalid lines %v", c.file, r.Invalid)
		}
		d := r.Draft
		if d == nil {
			t.Errorf("%s: no draft", c.file)
			continue
		}
		if d.Number != c.number || d.URL != c.url || d.Title != "Show delivery dates at checkout" || d.State != c.state || d.Action != c.action {
			t.Errorf("%s: draft %+v", c.file, *d)
		}
		if len(r.CommitURLs) != c.commits {
			t.Errorf("%s: %d addresses, want %d", c.file, len(r.CommitURLs), c.commits)
		}
	}
	r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/commits-only.txt"), hostCode, contracts.SubmitDraftNone, given)
	if r.Draft != nil || len(r.Invalid) != 0 || len(r.CommitURLs) != 1 {
		t.Errorf("commits-only.txt: %+v", r)
	}
	if got := r.CommitURLs[commitBase]; got != "https://code.example.com/acme/shop/commit/"+commitBase {
		t.Errorf("the base commit has the address %q", got)
	}
}

// TestAnInvalidOutputLineIsLeftOut pins, for each invalid output fixture, the line that
// qory leaves out and why, that the lines around it are recorded, and that a draft that
// loses one of its five lines is not recorded and has its other lines left out too,
// each as a line of a draft that is not whole.
func TestAnInvalidOutputLineIsLeftOut(t *testing.T) {
	const whole = "the draft needs five lines"
	cases := map[string]struct {
		line   int    // the line that breaks a rule, 0 when the draft only lacks a line
		reason string // why that line is left out
		others int    // the lines left out beside it, because the draft is not whole
		draft  bool   // the draft is recorded whole
	}{
		"invalid-key-unknown.txt":           {8, "the key is not one of the contract's", 0, true},
		"invalid-no-value.txt":              {8, "not a key and a value", 0, true},
		"invalid-control-character.txt":     {8, "control character", 0, true},
		"invalid-no-line-feed.txt":          {8, "no line feed", 0, true},
		"invalid-key-twice.txt":             {8, "the key is there already", 0, true},
		"invalid-commit-twice.txt":          {8, "has an address already", 0, true},
		"invalid-commit-not-given.txt":      {8, "not one that qory gave", 0, true},
		"invalid-number-zero.txt":           {1, "number is not a whole number", 4, false},
		"invalid-number-decimal.txt":        {1, "number is not a whole number", 4, false},
		"invalid-url-http.txt":              {2, "not an https URL", 4, false},
		"invalid-url-other-host.txt":        {2, "not on the repository's system", 4, false},
		"invalid-url-user.txt":              {2, "not an https URL", 4, false},
		"invalid-url-query.txt":             {2, "query or a fragment", 4, false},
		"invalid-url-javascript.txt":        {2, "not an https URL", 4, false},
		"invalid-url-upper-case-scheme.txt": {2, "not an https URL", 4, false},
		"invalid-state-archived.txt":        {4, "state is none of", 4, false},
		"invalid-merged-not-kept.txt":       {5, "closed or merged has the action kept", 0, false},
		"invalid-action-closed.txt":         {5, "action is none of", 4, false},
		"invalid-title-257-characters.txt":  {3, "257 characters, over the limit of 256", 4, false},
		"invalid-title-all-spaces.txt":      {3, "only spaces", 4, false},
		"invalid-title-no-break-space.txt":  {3, "only spaces", 4, false},
		"invalid-title-bad-utf8.txt":        {3, "not UTF-8", 4, false},
		"invalid-draft-without-action.txt":  {0, "", 4, false},
	}
	for file, want := range cases {
		r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/"+file), hostCode, contracts.DraftOpen, given)
		broken := 0
		if want.line > 0 {
			broken = 1
		}
		if len(r.Invalid) != broken+want.others {
			t.Errorf("%s: %d lines are left out %v, want %d", file, len(r.Invalid), r.Invalid, broken+want.others)
		}
		for _, e := range r.Invalid {
			if e.Line == want.line {
				if !strings.Contains(e.Reason, want.reason) {
					t.Errorf("%s: line %d is left out for %q, want %q", file, e.Line, e.Reason, want.reason)
				}
			} else if !strings.Contains(e.Reason, whole) {
				t.Errorf("%s: line %d is left out for %q, want %q", file, e.Line, e.Reason, whole)
			}
		}
		if (r.Draft != nil) != want.draft {
			t.Errorf("%s: draft recorded %v, want %v", file, r.Draft != nil, want.draft)
		}
		if len(r.CommitURLs) != 2 {
			t.Errorf("%s: %d addresses, want the 2 of the valid lines", file, len(r.CommitURLs))
		}
	}
	files, err := fs.ReadDir(contracts.RunFS, "submit-program/fixtures/output")
	if err != nil {
		t.Fatal(err)
	}
	invalid := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "invalid-") {
			invalid++
		}
	}
	if invalid != len(cases) {
		t.Errorf("fixtures/output contains %d invalid- files, and the test lists %d", invalid, len(cases))
	}
}

// TestAnAddressHasTheRuleOfTheSchema pins that a result line's address has the rule of
// definitions.schema.json, $defs.url, and the checks of the contract: what that pattern
// refuses is left out, and so is an address with a port, a query, a fragment, no path,
// another host or more than 2048 characters.
func TestAnAddressHasTheRuleOfTheSchema(t *testing.T) {
	const good = "https://code.example.com/acme/shop/pull/34"
	long := "https://code.example.com/" + strings.Repeat("a", 2048-len("https://code.example.com/"))
	for _, c := range []struct {
		name, addr string
		ok         bool
	}{
		{"plain", good, true},
		{"host in upper case", "https://CODE.example.com/acme/shop/pull/34", true},
		{"percent-encoding", "https://code.example.com/acme/shop%20x/pull/34", true},
		{"2048 characters", long, true},
		{"2049 characters", long + "a", false},
		{"upper-case scheme", "HTTPS://code.example.com/acme/shop/pull/34", false},
		{"http", "http://code.example.com/acme/shop/pull/34", false},
		{"double quote", "https://code.example.com/acme/\"shop/pull/34", false},
		{"single quote", "https://code.example.com/acme/shop'/pull/34", true},
		{"less-than", "https://code.example.com/acme/<shop/pull/34", false},
		{"greater-than", "https://code.example.com/acme/shop>/pull/34", false},
		{"left brace", "https://code.example.com/acme/{shop/pull/34", false},
		{"right brace", "https://code.example.com/acme/shop}/pull/34", false},
		{"bar", "https://code.example.com/acme/|shop/pull/34", false},
		{"caret", "https://code.example.com/acme/^shop/pull/34", false},
		{"backtick", "https://code.example.com/acme/`shop/pull/34", false},
		{"backslash", "https://code.example.com/acme\\shop/pull/34", false},
		{"non-ASCII", "https://code.example.com/acm\u00e9/shop/pull/34", false},
		{"no-break space", "https://code.example.com/acme/shop\u00a0/pull/34", false},
		{"space", "https://code.example.com/acme/shop /pull/34", false},
		{"port", "https://code.example.com:8443/acme/shop/pull/34", false},
		{"empty port", "https://code.example.com:/acme/shop/pull/34", false},
		{"fragment", "https://code.example.com/acme/shop/pull/34#files", false},
		{"empty fragment", "https://code.example.com/acme/shop/pull/34#", false},
		{"query", "https://code.example.com/acme/shop/pull/34?tab=files", false},
		{"empty query", "https://code.example.com/acme/shop/pull/34?", false},
		{"user information", "https://user@code.example.com/acme/shop/pull/34", false},
		{"no path", "https://code.example.com", false},
		{"root path", "https://code.example.com/", false},
		{"another host", "https://other.example.com/acme/shop/pull/34", false},
		{"suffix of the host", "https://evil-code.example.com/acme/shop/pull/34", false},
		{"lone percent", "https://code.example.com/acme/shop/100%/pull/34", false},
	} {
		for _, key := range []string{contracts.SubmitKeyDraftURL, contracts.SubmitKeyCommitURL} {
			line := key + " " + c.addr + "\n"
			if key == contracts.SubmitKeyCommitURL {
				line = key + " " + commitMade + " " + c.addr + "\n"
			}
			r := contracts.ParseSubmitLines(line, hostCode, contracts.DraftOpen, given)
			recorded := len(r.CommitURLs) == 1
			if key == contracts.SubmitKeyDraftURL {
				// A lone valid address is left out because the draft is not whole.
				recorded = len(r.Invalid) == 1 && strings.Contains(r.Invalid[0].Reason, "needs five lines")
			}
			if recorded != c.ok {
				t.Errorf("%s as %s: %+v, want accepted %v", c.name, key, r, c.ok)
			}
		}
	}
	// An empty value is not a line with a value.
	for _, line := range []string{"draft_url \n", "draft_url\n", "commit_url \n", "commit_url " + commitMade + " \n"} {
		if r := contracts.ParseSubmitLines(line, hostCode, contracts.DraftOpen, given); len(r.Invalid) != 1 {
			t.Errorf("%q: %+v, want one invalid line", line, r)
		}
	}
	// A 7-bit DEL in the address is a control character.
	if r := contracts.ParseSubmitLines("draft_url https://code.example.com/a\x7f\n", hostCode, contracts.DraftOpen, given); len(r.Invalid) != 1 || !strings.Contains(r.Invalid[0].Reason, "control character") {
		t.Errorf("an address with DEL: %+v", r)
	}
}

// TestAnEmptySystemSkipsOnlyTheHostCheck pins that for a target with no system, as a
// target with a collection and no address, an address on any host is accepted, and every
// other rule of an address stays: with a system, the same address on another host is
// refused. It pins this for the address of a commit and for the address of the draft,
// the latter through ReadSubmitRun.
func TestAnEmptySystemSkipsOnlyTheHostCheck(t *testing.T) {
	for _, c := range []struct {
		name, addr string
		system     string
		ok         bool
	}{
		{"another host, no system", "https://other.example.com/acme/shop/pull/34", "", true},
		{"another host, a system", "https://other.example.com/acme/shop/pull/34", hostCode, false},
		{"the host, no system", "https://code.example.com/acme/shop/pull/34", "", true},
		{"http, no system", "http://other.example.com/acme/shop/pull/34", "", false},
		{"port, no system", "https://other.example.com:8443/acme/shop/pull/34", "", false},
		{"query, no system", "https://other.example.com/acme/shop/pull/34?tab=files", "", false},
		{"fragment, no system", "https://other.example.com/acme/shop/pull/34#files", "", false},
		{"no path, no system", "https://other.example.com", "", false},
		{"user information, no system", "https://user@other.example.com/acme/shop/pull/34", "", false},
	} {
		line := contracts.SubmitKeyCommitURL + " " + commitMade + " " + c.addr + "\n"
		r := contracts.ParseSubmitLines(line, c.system, contracts.DraftOpen, given)
		if recorded := len(r.CommitURLs) == 1; recorded != c.ok {
			t.Errorf("%s: %+v, want accepted %v", c.name, r, c.ok)
		}
		// The address of the draft goes through the same check, here with a whole
		// program output and the record that qory makes after exit 0.
		out := strings.Replace(submitFile(t, "fixtures/output/github-like.txt"),
			"https://code.example.com/acme/shop/pull/34", c.addr, 1)
		rec := contracts.ReadSubmitRun(ended(0), out, c.system, contracts.DraftOpen, given)
		if (rec.Draft != nil) != c.ok || (rec.Failed == nil) != c.ok {
			t.Errorf("%s: the draft address: draft %v, failed %v, want accepted %v", c.name, rec.Draft != nil, rec.Failed != nil, c.ok)
		}
	}
}

// TestATitleHasTheRuleOfTheSchema pins that a result line's title has the rule of
// definitions.schema.json, $defs.draft_title, the same as the title of the adapter's
// reply: 1 to 256 characters of UTF-8, with no control, bidirectional or invisible
// character, and not only spaces.
func TestATitleHasTheRuleOfTheSchema(t *testing.T) {
	for _, c := range []struct {
		name, title string
		ok          bool
	}{
		{"plain", "Show delivery dates", true},
		{"256 characters", strings.Repeat("a", 256), true},
		{"256 characters of 2 bytes", strings.Repeat("\u00e9", 256), true},
		{"257 characters", strings.Repeat("a", 257), false},
		{"DEL", "Show\x7fdates", false},
		{"C1 control", "Show\u0085dates", false},
		{"line separator", "Show\u2028dates", false},
		{"paragraph separator", "Show\u2029dates", false},
		{"bidirectional override", "Show\u202edates", false},
		{"zero-width space", "Show\u200bdates", false},
		{"byte order mark", "Show\ufeffdates", false},
		{"no-break space only", "\u00a0\u00a0", false},
		{"spaces only", "   ", false},
		{"ideographic space only", "\u3000", false},
		{"a space inside", "Show  dates", true},
		{"bytes that are not UTF-8", "Show\xffdates", false},
	} {
		r := contracts.ParseSubmitLines("draft_title "+c.title+"\n", hostCode, contracts.DraftOpen, given)
		// A lone valid title makes an incomplete draft: it is recorded as one line left out for that.
		refused := len(r.Invalid) == 1 && !strings.Contains(r.Invalid[0].Reason, "needs five lines")
		if refused == c.ok {
			t.Errorf("%s: %+v, want accepted %v", c.name, r.Invalid, c.ok)
		}
	}
}

// TestADraftLineFollowsTheInput pins what qory does with the draft lines by the value of
// QORY_SUBMIT_DRAFT: with none, each draft line is left out and no draft is recorded;
// with open, a partial draft is left out line by line, a draft that a person closed or
// merged needs the action kept, and the number of a draft that the program found from the
// branch is recorded.
func TestADraftLineFollowsTheInput(t *testing.T) {
	out := submitFile(t, "fixtures/output/github-like.txt")
	r := contracts.ParseSubmitLines(out, hostCode, contracts.SubmitDraftNone, given)
	if r.Draft != nil || len(r.Invalid) != 5 || len(r.CommitURLs) != 2 {
		t.Errorf("with none: %+v", r)
	}
	for _, e := range r.Invalid {
		if !strings.Contains(e.Reason, "QORY_SUBMIT_DRAFT is none") {
			t.Errorf("with none, line %d is left out for %q", e.Line, e.Reason)
		}
	}
	// An open that found a draft from the branch: the program printed that draft's number.
	r = contracts.ParseSubmitLines(strings.Replace(out, "draft_number 34", "draft_number 41", 1), hostCode, contracts.DraftOpen, given)
	if r.Draft == nil || r.Draft.Number != 41 {
		t.Errorf("an open that found another draft: %+v", r)
	}
	// Three of the five draft lines: none is recorded, and each is listed.
	partial := "draft_number 34\ndraft_state draft\ndraft_action opened\n"
	r = contracts.ParseSubmitLines(partial, hostCode, contracts.DraftOpen, given)
	if r.Draft != nil || len(r.Invalid) != 3 || !strings.Contains(r.Invalid[0].Reason, "draft_url, draft_title is missing") {
		t.Errorf("a partial draft: %+v", r)
	}
	// Leaving out any one of the five keys makes a partial draft.
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	for i := 0; i < 5; i++ {
		var without []string
		for j, l := range lines {
			if j != i {
				without = append(without, l)
			}
		}
		r = contracts.ParseSubmitLines(strings.Join(without, "\n")+"\n", hostCode, contracts.DraftOpen, given)
		if r.Draft != nil || len(r.Invalid) != 4 {
			t.Errorf("without line %d: %+v", i+1, r)
		}
	}
}

// TestWhatQoryRecordsAfterTheProgram pins ReadSubmitRun: the draft and the addresses
// from the lines after exit 0, 1 and 2; program-failed with the exit status when a
// draft that was asked for is not made; login-needed after exit 3; program-timeout,
// with no exit status, after a time-out; program-failed with exit 1 when the program
// did not start or ended with a signal or another status; no line read in the last four
// cases, whatever the program printed; and no failure for the draft none.
func TestWhatQoryRecordsAfterTheProgram(t *testing.T) {
	out := submitFile(t, "fixtures/output/github-like.txt")
	for _, c := range []struct {
		name, output, draft string
		end                 contracts.SubmitEnd
		wantDraft           bool
		reason              string // the reason of the failure, "" for none
		exit                int    // the exit of the failure, -1 for none
		lines               bool   // qory reads the lines
	}{
		{"finished", out, "open", ended(0), true, "", -1, true},
		{"forge failed with the lines", out, "update", ended(1), true, "", -1, true},
		{"input refused with the lines", out, "open", ended(2), true, "", -1, true},
		{"forge failed, no lines", "", "open", ended(1), false, contracts.SubmitProgramFailed, 1, true},
		{"input refused, no lines", "", "update", ended(2), false, contracts.SubmitProgramFailed, 2, true},
		{"finished, no lines", "", "open", ended(0), false, contracts.SubmitProgramFailed, 0, true},
		{"finished, partial draft", "draft_number 34\n", "open", ended(0), false, contracts.SubmitProgramFailed, 0, true},
		{"no login, with the lines", out, "open", ended(3), false, contracts.SubmitLoginNeeded, 3, false},
		{"no login, no lines", "", "update", ended(3), false, contracts.SubmitLoginNeeded, 3, false},
		{"timed out, with the lines", out, "open", contracts.SubmitEnd{Started: true, TimedOut: true, Exit: 0}, false, contracts.SubmitProgramTimeout, -1, false},
		{"timed out, no lines", "", "open", contracts.SubmitEnd{Started: true, TimedOut: true, Exit: -1}, false, contracts.SubmitProgramTimeout, -1, false},
		{"timed out with exit 0 is still a time-out", out, "update", contracts.SubmitEnd{Started: true, TimedOut: true}, false, contracts.SubmitProgramTimeout, -1, false},
		{"another status", out, "open", ended(4), false, contracts.SubmitProgramFailed, 1, false},
		{"killed by a signal", out, "open", ended(-1), false, contracts.SubmitProgramFailed, 1, false},
		{"did not start", "", "open", contracts.SubmitEnd{}, false, contracts.SubmitProgramFailed, 1, false},
		{"draft none, no lines", "", "none", ended(0), false, "", -1, true},
		{"draft none, program failed", "", "none", ended(1), false, "", -1, true},
		{"draft none, no login", "", "none", ended(3), false, "", -1, false},
		{"draft none, timed out", "", "none", contracts.SubmitEnd{Started: true, TimedOut: true}, false, "", -1, false},
		{"draft none, did not start", "", "none", contracts.SubmitEnd{}, false, "", -1, false},
	} {
		rec := contracts.ReadSubmitRun(c.end, c.output, hostCode, c.draft, given)
		if (rec.Draft != nil) != c.wantDraft {
			t.Errorf("%s: draft recorded %v", c.name, rec.Draft != nil)
		}
		switch {
		case c.reason == "" && rec.Failed != nil:
			t.Errorf("%s: failed %+v", c.name, *rec.Failed)
		case c.reason != "" && (rec.Failed == nil || rec.Failed.Reason != c.reason):
			t.Errorf("%s: failed %+v, want %s", c.name, rec.Failed, c.reason)
		case c.reason != "" && c.exit < 0 && rec.Failed.Exit != nil:
			t.Errorf("%s: failed with the exit status %d, want none", c.name, *rec.Failed.Exit)
		case c.reason != "" && c.exit >= 0 && (rec.Failed.Exit == nil || *rec.Failed.Exit != c.exit):
			t.Errorf("%s: failed %+v, want exit %d", c.name, rec.Failed, c.exit)
		}
		wantURLs := 0
		if c.output == out && c.lines {
			wantURLs = 2
		}
		if len(rec.CommitURLs) != wantURLs {
			t.Errorf("%s: %d addresses, want %d", c.name, len(rec.CommitURLs), wantURLs)
		}
		if !c.lines && len(rec.Invalid) != 0 {
			t.Errorf("%s: lines were read: %v", c.name, rec.Invalid)
		}
	}
}

// ended is a program that started and ended with the exit status, or -1 for a signal.
func ended(exit int) contracts.SubmitEnd { return contracts.SubmitEnd{Started: true, Exit: exit} }

// TestADescriptionIsMarked pins MarkedDescription on each case of
// fixtures/description: a draft being opened, text outside the markers kept on an update,
// the markers removed, a start marker that a person left alone, and a block added at the
// end.
func TestADescriptionIsMarked(t *testing.T) {
	dirs, err := fs.ReadDir(contracts.RunFS, "submit-program/fixtures/description")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range dirs {
		seen[d.Name()] = true
		dir := "fixtures/description/" + d.Name() + "/"
		got := contracts.MarkedDescription(submitFile(t, dir+"existing.md"), submitFile(t, dir+"text.md"))
		if want := submitFile(t, dir+"result.md"); got != want {
			t.Errorf("%s: got %q, want %q", d.Name(), got, want)
		}
	}
	for _, name := range []string{"open", "update-keeps-text-around", "markers-removed", "start-marker-removed",
		"stray-start-marker-first-round", "stray-start-marker-second-round", "end-marker-before-start", "crlf-body-without-markers",
		"crlf-multi-line-text", "crlf-multi-line-text-appended", "mixed-endings-first-crlf", "mixed-endings-first-lf", "update-empty-description"} {
		if !seen[name] {
			t.Errorf("fixtures/description has no case %q", name)
		}
	}
	// Writing the result again with the same text changes nothing, whatever markers the result has.
	for name := range seen {
		dir := "fixtures/description/" + name + "/"
		result, text := submitFile(t, dir+"result.md"), submitFile(t, dir+"text.md")
		if again := contracts.MarkedDescription(result, text); again != result {
			t.Errorf("%s: a second write gives %q, want %q", name, again, result)
		}
	}
	// A person's text and the stray start marker survive every round.
	person := "Intro.\n" + contracts.SubmitMarkerStart + "\nA person wrote this.\n"
	d := person
	for _, text := range []string{"One.", "Two.", "Three."} {
		d = contracts.MarkedDescription(d, text)
		if !strings.HasPrefix(d, person) || strings.Count(d, "\n"+contracts.SubmitMarkerStart+"\n") != 2 ||
			!strings.HasSuffix(d, "\n"+text+"\n"+contracts.SubmitMarkerEnd) || strings.Count(d, contracts.SubmitMarkerEnd) != 1 {
			t.Fatalf("after %q the description is %q", text, d)
		}
	}
}

// TestATitleChangesOnlyWhileItIsThePreviousOne pins NextTitle: an update with an
// unchanged title changes it, an update with a title that a person edited keeps it, a
// previous title that is absent changes nothing, and the text that a forge puts before
// the title of a draft is left out of the comparison and kept in the new title.
func TestATitleChangesOnlyWhileItIsThePreviousOne(t *testing.T) {
	for _, c := range []struct{ name, current, previous, next, forge, want string }{
		{"unchanged title", "Show dates", "Show dates", "Show delivery dates", "", "Show delivery dates"},
		{"edited title", "Show dates (urgent)", "Show dates", "Show delivery dates", "", "Show dates (urgent)"},
		{"no previous title", "Show dates", "", "Show delivery dates", "", "Show dates"},
		{"empty title, no previous title", "", "", "Show delivery dates", "", ""},
		{"empty title, a previous title", "", "Show dates", "Show delivery dates", "", ""},
		{"same title again", "Show dates", "Show dates", "Show dates", "", "Show dates"},
		{"forge text before the title", "Prefix: Show dates", "Show dates", "Show delivery dates", "Prefix: ", "Prefix: Show delivery dates"},
		{"forge text, edited title", "Prefix: Show dates (urgent)", "Show dates", "Show delivery dates", "Prefix: ", "Prefix: Show dates (urgent)"},
		{"forge text left on a title that has none", "Show dates", "Show dates", "Show delivery dates", "Prefix: ", "Show delivery dates"},
		{"forge text on a title that is only the text", "Prefix: ", "Show dates", "Show delivery dates", "Prefix: ", "Prefix: "},
		{"previous title is the forge's text", "Prefix: Show dates", "Prefix: Show dates", "Show delivery dates", "", "Show delivery dates"},
	} {
		if got := contracts.NextTitle(c.current, c.previous, c.next, c.forge); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestTheSubmissionExamplesAreOneForEachForgeKind pins that the feed has an entry for a forge
// whose draft is a pull request and one for a forge whose draft is a merge request, on
// two hosts, that the first version has no previous_title, and that the entry of the
// second version has the text.title of the first.
func TestTheSubmissionExamplesAreOneForEachForgeKind(t *testing.T) {
	type entry struct {
		Kind          string  `json:"kind"`
		SubmissionID  string  `json:"submission_id"`
		PreviousTitle *string `json:"previous_title"`
		Text          *struct {
			Title string `json:"title"`
		} `json:"text"`
		Targets []struct {
			System string `json:"system"`
			Draft  *struct {
				Action string `json:"action"`
				Number int    `json:"number"`
			} `json:"draft"`
		} `json:"targets"`
	}
	feed := func(name string) []entry {
		var f struct{ Changes []entry }
		if err := json.Unmarshal(read(t, "fixtures/adapter/changes/"+name), &f); err != nil {
			t.Fatal(err)
		}
		return f.Changes
	}
	github, gitlab := feed("submission-github-like.json")[0], feed("submission-gitlab-like.json")[0]
	if github.Targets[0].System == gitlab.Targets[0].System {
		t.Error("the two examples are on one host")
	}
	if github.Targets[0].Draft.Action != "open" || gitlab.Targets[0].Draft.Action != "update" {
		t.Error("the examples do not open and update")
	}
	two := feed("submission-two-versions.json")
	if len(two) != 2 || two[0].PreviousTitle != nil || two[1].PreviousTitle == nil || two[0].Text == nil {
		t.Fatalf("submission-two-versions.json: the first version has no previous_title and the second has one: %+v", two)
	}
	if *two[1].PreviousTitle != two[0].Text.Title {
		t.Errorf("the second version has the previous title %q, and the first has the title %q", *two[1].PreviousTitle, two[0].Text.Title)
	}
	if two[1].Targets[0].Draft.Action != "update" || two[1].Targets[0].Draft.Number != 34 {
		t.Error("the second version does not update draft 34")
	}
	for _, name := range []string{"github-like.json", "gitlab-like.json", "without-draft.json", "github-like-closed.json", "gitlab-like-merged.json"} {
		if err := validate(compile(t, "changes-submitted.schema.json"), read(t, "fixtures/adapter/changes-submitted/"+name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, reason := range []string{"not_permitted", "unreachable", "branch_gone", "too_long", "forge_refused"} {
		b := read(t, "fixtures/adapter/changes-failed/submission-"+strings.ReplaceAll(reason, "_", "-")+".json")
		if err := validate(compile(t, "changes-failed.schema.json"), b); err != nil {
			t.Errorf("%s: %v", reason, err)
		}
	}
}

// TestTheFeedDescribesTheMarkers pins that the submission entry's description mentions the
// two marker lines of the submit program contract, so an adapter and a program use the
// same ones.
func TestTheFeedDescribesTheMarkers(t *testing.T) {
	d, _ := node(t, "changes.schema.json", "/$defs/submission")["description"].(string)
	for _, want := range []string{contracts.SubmitMarkerStart, contracts.SubmitMarkerEnd, "previous_title"} {
		if !strings.Contains(d, want) {
			t.Errorf("the description of the submission entry does not contain %q", want)
		}
	}
	text := node(t, "changes.schema.json", "/$defs/submission/properties/text/properties/description")
	if text["maxLength"] != json.Number("20000") {
		t.Errorf("the description's limit is %v", text["maxLength"])
	}
}

// TestTheFeedSaysWhichDraftOfSeveralIsTaken pins the sentence that picks one draft from
// several on the branch, in the submission entry's description and in the README step that
// finds the draft, whose white space is collapsed to single spaces.
func TestTheFeedSaysWhichDraftOfSeveralIsTaken(t *testing.T) {
	d, _ := node(t, "changes.schema.json", "/$defs/submission")["description"].(string)
	readme := strings.Join(strings.Fields(string(read(t, "README.md"))), " ")
	for _, c := range []struct{ where, text, want string }{
		{"the submission entry", d, "With several drafts on the branch, the adapter takes the entry's draft.number when it is among them, otherwise the not-closed, not-merged draft with the highest number; with none, the closed or merged one with the highest number, reported as kept."},
		{"the README", readme, "With several drafts on the branch, the adapter takes the entry's `draft.number` when it is among them, otherwise the not-closed, not-merged draft with the highest number; with none, the closed or merged one with the highest number, reported as kept."},
	} {
		if !strings.Contains(c.text, c.want) {
			t.Errorf("%s does not contain %q", c.where, c.want)
		}
	}
}

// gatingTexts are the places that state that a submission entry reaches only an adapter whose
// source role has submissions: true, by where they are. A README section has its white
// space collapsed to single spaces.
func gatingTexts(t *testing.T) map[string]string {
	t.Helper()
	str := func(m map[string]any) string { d, _ := m["description"].(string); return d }
	readme := string(read(t, "README.md"))
	section := func(heading string) string {
		i := strings.Index(readme, "\n## "+heading+"\n")
		if i < 0 {
			t.Fatalf("README.md has no section %q", heading)
		}
		rest := readme[i+1:]
		if j := strings.Index(rest[3:], "\n## "); j >= 0 {
			rest = rest[:j+3]
		}
		return strings.Join(strings.Fields(rest), " ")
	}
	return map[string]string{
		"the feed's description":  str(root(t, "changes.schema.json")),
		"the submission entry":    str(node(t, "changes.schema.json", "/$defs/submission")),
		"the submissions flag":    str(node(t, "description.schema.json", "/properties/roles/properties/source/properties/submissions")),
		"the targets flag":        str(node(t, "description.schema.json", "/properties/roles/properties/source/properties/targets")),
		"the feed section":        section("Change feed and comments"),
		"the Submissions section": section("Submissions"),
	}
}

// root is the top object of a schema of the contract.
func root(t *testing.T, name string) map[string]any {
	t.Helper()
	doc, err := contracts.Document(name)
	if err != nil {
		t.Fatal(err)
	}
	return doc.(map[string]any)
}

// missingGating lists the places in texts that do not contain submissions: true.
func missingGating(texts map[string]string) []string {
	var missing []string
	for place, text := range texts {
		if !strings.Contains(text, "submissions: true") && !strings.Contains(text, "`submissions: true`") {
			missing = append(missing, place)
		}
	}
	slices.Sort(missing)
	return missing
}

// TestTheSubmissionsFlagIsInEachPlaceThatGatesTheEntry pins that the feed's description,
// the submission entry, the flag's text, the text of targets and the integration README
// each mention that the entry needs submissions: true, and that removing the phrase from a
// copy of any one place is found.
func TestTheSubmissionsFlagIsInEachPlaceThatGatesTheEntry(t *testing.T) {
	texts := gatingTexts(t)
	if missing := missingGating(texts); len(missing) != 0 {
		t.Fatalf("no mention of submissions: true in %v", missing)
	}
	for place, text := range texts {
		copied := map[string]string{}
		for k, v := range texts {
			copied[k] = v
		}
		copied[place] = strings.ReplaceAll(text, "submissions: true", "")
		missing := missingGating(copied)
		if !slices.Contains(missing, place) {
			t.Errorf("without the phrase in %s, the check finds %v", place, missing)
		}
	}
	if d := texts["the targets flag"]; !strings.Contains(d, "watch") {
		t.Error("the text of targets does not mention that it gates the watch entry")
	}
}

// TestAKeyKeepsItsFirstValidLine pins that, for a draft key and for a commit, the first
// valid line stays, a later valid line is left out, and an earlier line that is not valid
// does not count.
func TestAKeyKeepsItsFirstValidLine(t *testing.T) {
	const rest = "draft_url https://code.example.com/acme/shop/pull/34\ndraft_title Show dates\ndraft_state draft\ndraft_action opened\n"
	for _, c := range []struct {
		name, output string
		number       int64
		invalid      []int
	}{
		{"number, invalid then valid", "draft_number 0\ndraft_number 34\n" + rest, 34, []int{1}},
		{"number, valid then valid", "draft_number 34\ndraft_number 35\n" + rest, 34, []int{2}},
		{"number, valid then invalid", "draft_number 34\ndraft_number 0\n" + rest, 34, []int{2}},
		{"number of 9 digits", "draft_number 123456789\n" + rest, 123456789, nil},
		{"number of 10 digits", "draft_number 1234567890\n" + rest, 1234567890, nil},
		{"the largest number", "draft_number 9223372036854775807\n" + rest, math.MaxInt64, nil},
		{"the largest number and 1", "draft_number 9223372036854775808\n" + rest, 0, []int{1, 2, 3, 4, 5}},
		{"number of 19 digits over the largest", "draft_number 9999999999999999999\n" + rest, 0, []int{1, 2, 3, 4, 5}},
		{"number of 20 digits", "draft_number 10000000000000000000\n" + rest, 0, []int{1, 2, 3, 4, 5}},
		{"number 0", "draft_number 0\n" + rest, 0, []int{1, 2, 3, 4, 5}},
	} {
		r := contracts.ParseSubmitLines(c.output, hostCode, contracts.DraftOpen, given)
		var lines []int
		for _, e := range r.Invalid {
			lines = append(lines, e.Line)
		}
		if !slices.Equal(lines, c.invalid) {
			t.Errorf("%s: lines %v are left out, want %v", c.name, lines, c.invalid)
		}
		if (r.Draft != nil) != (c.number != 0) || (r.Draft != nil && r.Draft.Number != c.number) {
			t.Errorf("%s: draft %+v, want number %d", c.name, r.Draft, c.number)
		}
	}
	good := "https://code.example.com/acme/shop/commit/" + commitMade
	for _, c := range []struct {
		name, output string
		want         string
		invalid      []int
	}{
		{"invalid then valid", "commit_url " + commitMade + " http://code.example.com/x\ncommit_url " + commitMade + " " + good + "\n", good, []int{1}},
		{"valid then valid", "commit_url " + commitMade + " " + good + "\ncommit_url " + commitMade + " " + good + "x\n", good, []int{2}},
	} {
		r := contracts.ParseSubmitLines(c.output, hostCode, contracts.SubmitDraftNone, given)
		var lines []int
		for _, e := range r.Invalid {
			lines = append(lines, e.Line)
		}
		if !slices.Equal(lines, c.invalid) || r.CommitURLs[commitMade] != c.want {
			t.Errorf("%s: lines %v are left out and the address is %q, want %v and %q", c.name, lines, r.CommitURLs[commitMade], c.invalid, c.want)
		}
	}
}

// TestTheLeftOutLinesAreInTheOrderOfTheirNumbers pins that Invalid is sorted by line
// number when a line is left out as it is read and others as a draft that is not whole.
func TestTheLeftOutLinesAreInTheOrderOfTheirNumbers(t *testing.T) {
	out := "draft_number 34\nbogus\ndraft_state draft\n\ndraft_title\n"
	r := contracts.ParseSubmitLines(out, hostCode, contracts.DraftOpen, given)
	var lines []int
	for _, e := range r.Invalid {
		lines = append(lines, e.Line)
	}
	if want := []int{1, 2, 3, 5}; !slices.Equal(lines, want) {
		t.Errorf("lines %v are left out, want %v: %v", lines, want, r.Invalid)
	}
}

// TestTheReadmeSaysWhichDraftTheProgramTakes pins that the README of the submit program
// says where the program finds the drafts, which of several it takes, that a closed or
// merged draft is reported as kept, and that the program never changes the title of a
// draft that it finds.
func TestTheReadmeSaysWhichDraftTheProgramTakes(t *testing.T) {
	readme := strings.Join(strings.Fields(submitFile(t, "README.md")), " ")
	for _, want := range []string{
		"the program first looks for the drafts from `QORY_SUBMIT_BRANCH` and the repository, in any state",
		"With several drafts, it takes the not-closed, not-merged draft with the highest number; with none, the closed or merged one with the highest number, which it reports with `draft_action` `kept` and leaves as it is.",
		"Any other draft is edited: its description follows the rules below, and its title stays, because the program never changes the title of a draft that it finds and a run with no run code has no previous title.",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not contain %q", want)
		}
	}
}
