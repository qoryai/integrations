package contracts_test

import (
	"encoding/json"
	"io/fs"
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
// fixtures, which carry no other commit.
const (
	commitBase = "3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1"
	commitMade = "9f8e7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c"
)

// TestTheSubmitProgramNamesAreInTheReadme pins that the README of the contract lists
// each input variable and each key of a result line that the Go constants name, once in
// its table, and no other QORY_SUBMIT_ variable or key, so a rename changes the
// constants and the README together.
func TestTheSubmitProgramNamesAreInTheReadme(t *testing.T) {
	readme := submitFile(t, "README.md")
	inputs := []string{
		contracts.SubmitEnvSystem, contracts.SubmitEnvPath, contracts.SubmitEnvBranch, contracts.SubmitEnvBaseBranch,
		contracts.SubmitEnvBaseCommit, contracts.SubmitEnvCommits, contracts.SubmitEnvDraft, contracts.SubmitEnvDraftNumber,
		contracts.SubmitEnvTitle, contracts.SubmitEnvPreviousTitle, contracts.SubmitEnvDescriptionFile,
	}
	var rows []string
	for _, l := range strings.Split(readme, "\n") {
		if strings.HasPrefix(l, "| `QORY_SUBMIT_") {
			rows = append(rows, strings.Trim(strings.SplitN(l, "|", 3)[1], " `"))
		}
	}
	if !slices.Equal(rows, inputs) {
		t.Errorf("the table of inputs lists %v, and the constants name %v", rows, inputs)
	}
	for _, name := range regexp.MustCompile("QORY_[A-Z_]+").FindAllString(readme, -1) {
		if !slices.Contains(inputs, name) {
			t.Errorf("the README names %s, which no constant has", name)
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
		t.Errorf("the table of keys lists %v, and the constants name %v", rows, keys)
	}
	for _, want := range []string{
		contracts.SubmitMarkerStart, contracts.SubmitMarkerEnd, contracts.SubmitLinePrefix,
		"at most 20 lines", "`" + contracts.SubmitDraftOpen + "`", "`" + contracts.SubmitDraftUpdate + "`", "`" + contracts.SubmitDraftNone + "`",
	} {
		if !strings.Contains(readme, want) {
			t.Errorf("the README does not contain %q", want)
		}
	}
	if contracts.SubmitPrintedLines != 20 {
		t.Errorf("SubmitPrintedLines is %d, and the README says 20", contracts.SubmitPrintedLines)
	}
	// The contract has no relay, no JSON file and no wall.
	for _, bad := range []string{"QORY_PROXY", "QORY_SUBMISSION_FILE", "SSL_CERT_FILE", "proxy.sock", "program-timeout", "jq "} {
		if strings.Contains(readme, bad) {
			t.Errorf("the README contains %q", bad)
		}
	}
}

// TestTheReadmeExamplesAreTheFixtures pins that the output shown in the README is the
// content of fixtures/output/github-like.txt, and that the exit table lists the exit
// constants.
func TestTheReadmeExamplesAreTheFixtures(t *testing.T) {
	readme := submitFile(t, "README.md")
	if !strings.Contains(readme, "```text\n"+submitFile(t, "fixtures/output/github-like.txt")+"```") {
		t.Error("the README's example is not the content of fixtures/output/github-like.txt")
	}
	for _, row := range []string{"| 0 |", "| 1 |", "| 2 |", "| another status, or the program did not start |"} {
		if !strings.Contains(readme, row) {
			t.Errorf("the table of exit statuses has no row %q", row)
		}
	}
	if contracts.SubmitExitDone != 0 || contracts.SubmitExitForge != 1 || contracts.SubmitExitInput != 2 {
		t.Error("the exit constants are not 0, 1 and 2")
	}
}

// TestTheOutputFixturesAreRead pins what qory records from each output fixture whose
// lines are all valid: a draft with its five members, or none, and an address for each
// commit it lists. One fixture is for a forge whose draft is a pull request, one for a
// forge whose draft is a merge request.
func TestTheOutputFixturesAreRead(t *testing.T) {
	for _, c := range []struct {
		file, system, url, action string
		number                    int
		commits                   int
	}{
		{"github-like.txt", "code.example.com", "https://code.example.com/acme/shop/pull/34", "opened", 34, 2},
		{"gitlab-like.txt", "git.example.net", "https://git.example.net/acme/shop/-/merge_requests/7", "updated", 7, 2},
		{"kept.txt", "code.example.com", "https://code.example.com/acme/shop/pull/34", "kept", 34, 2},
	} {
		r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/"+c.file), c.system, []string{commitBase, commitMade})
		if len(r.Invalid) != 0 {
			t.Errorf("%s: invalid lines %v", c.file, r.Invalid)
		}
		d := r.Draft
		if d == nil {
			t.Errorf("%s: no draft", c.file)
			continue
		}
		if d.Number != c.number || d.URL != c.url || d.Title != "Show delivery dates at checkout" || d.State != "draft" || d.Action != c.action {
			t.Errorf("%s: draft %+v", c.file, *d)
		}
		if len(r.CommitURLs) != c.commits {
			t.Errorf("%s: %d addresses, want %d", c.file, len(r.CommitURLs), c.commits)
		}
	}
	r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/commits-only.txt"), "code.example.com", []string{commitBase, commitMade})
	if r.Draft != nil || len(r.Invalid) != 0 || len(r.CommitURLs) != 1 {
		t.Errorf("commits-only.txt: %+v", r)
	}
	if got := r.CommitURLs[commitBase]; got != "https://code.example.com/acme/shop/commit/"+commitBase {
		t.Errorf("the base commit has the address %q", got)
	}
}

// TestAnInvalidOutputLineIsLeftOut pins, for each invalid output fixture, the line that
// qory leaves out and why, that the lines around it are recorded, and that a draft that
// loses one of its five lines is not recorded.
func TestAnInvalidOutputLineIsLeftOut(t *testing.T) {
	const system = "code.example.com"
	given := []string{commitBase, commitMade}
	for file, want := range map[string]struct {
		line   int
		reason string
		draft  bool // the draft is recorded whole
	}{
		"invalid-key-unknown.txt":          {8, "the key is not one of the contract's", true},
		"invalid-no-value.txt":             {8, "not a key and a value", true},
		"invalid-control-character.txt":    {8, "control character", true},
		"invalid-no-line-feed.txt":         {8, "no line feed", true},
		"invalid-key-twice.txt":            {8, "the key is there already", true},
		"invalid-commit-twice.txt":         {8, "has an address already", true},
		"invalid-commit-not-given.txt":     {8, "not one that qory gave", true},
		"invalid-number-zero.txt":          {1, "number is not a whole number", false},
		"invalid-number-decimal.txt":       {1, "number is not a whole number", false},
		"invalid-url-http.txt":             {2, "not https", false},
		"invalid-url-other-host.txt":       {2, "not on the repository's system", false},
		"invalid-url-user.txt":             {2, "user information", false},
		"invalid-url-query.txt":            {2, "query or a fragment", false},
		"invalid-url-javascript.txt":       {2, "not https", false},
		"invalid-state-merged.txt":         {4, "state is neither", false},
		"invalid-action-closed.txt":        {5, "action is none of", false},
		"invalid-title-257-characters.txt": {3, "257 characters, over the limit of 256", false},
		"invalid-draft-without-action.txt": {0, "", false},
	} {
		r := contracts.ParseSubmitLines(submitFile(t, "fixtures/output/"+file), system, given)
		if want.line == 0 {
			if len(r.Invalid) != 0 {
				t.Errorf("%s: invalid lines %v", file, r.Invalid)
			}
		} else if len(r.Invalid) != 1 || r.Invalid[0].Line != want.line || !strings.Contains(r.Invalid[0].Reason, want.reason) {
			t.Errorf("%s: invalid lines %v, want line %d with %q", file, r.Invalid, want.line, want.reason)
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
	if invalid != 18 {
		t.Errorf("fixtures/output holds %d invalid- files, and the test expects 18", invalid)
	}
}

// TestADescriptionIsMarked pins MarkedDescription on each case of
// fixtures/description: a draft being opened, text outside the markers kept on an update,
// the markers removed, and a block added at the end.
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
	for _, name := range []string{"open", "update-keeps-text-around", "markers-removed", "start-marker-removed"} {
		if !seen[name] {
			t.Errorf("fixtures/description has no case %q", name)
		}
	}
	// Writing the result again with the same text changes nothing, where the result has one block.
	for name := range seen {
		dir := "fixtures/description/" + name + "/"
		result, text := submitFile(t, dir+"result.md"), submitFile(t, dir+"text.md")
		if strings.Count(result, contracts.SubmitMarkerStart) != 1 {
			continue // a description with a marker left over from a person's edit
		}
		if again := contracts.MarkedDescription(result, text); again != result {
			t.Errorf("%s: a second write gives %q, want %q", name, again, result)
		}
	}
}

// TestATitleChangesOnlyWhileItIsThePreviousOne pins NextTitle: an update with an
// unchanged title changes it, an update with a title that a person edited keeps it, and
// a first version, with no previous title, changes nothing.
func TestATitleChangesOnlyWhileItIsThePreviousOne(t *testing.T) {
	for _, c := range []struct{ name, current, previous, next, want string }{
		{"unchanged title", "Show dates", "Show dates", "Show delivery dates", "Show delivery dates"},
		{"edited title", "Show dates (urgent)", "Show dates", "Show delivery dates", "Show dates (urgent)"},
		{"no previous title", "Show dates", "", "Show delivery dates", "Show dates"},
		{"same title again", "Show dates", "Show dates", "Show dates", "Show dates"},
	} {
		if got := contracts.NextTitle(c.current, c.previous, c.next); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestTheSubmissionExamplesAreOneForEachTarget pins that the feed has an entry for a forge
// whose draft is a pull request and one for a forge whose draft is a merge request, on
// two hosts, that the first version has no previous_title, and that the entry of the
// second version has the text.title of the first.
func TestTheSubmissionExamplesAreOneForEachTarget(t *testing.T) {
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
	for _, reason := range []string{"not_permitted", "unreachable", "branch_gone", "too_long"} {
		b := read(t, "fixtures/adapter/changes-failed/submission-"+strings.ReplaceAll(reason, "_", "-")+".json")
		if err := validate(compile(t, "changes-failed.schema.json"), b); err != nil {
			t.Errorf("%s: %v", reason, err)
		}
	}
}

// TestTheFeedDescribesTheMarkers pins that the submission entry's description names the
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
