package contracts_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// mutate decodes the fixture, applies change to its first target (the entry's, or the
// reply's), and returns the JSON. In a feed fixture the target is changes[0].targets[0].
func mutate(t *testing.T, fixture string, feed bool, change func(doc, target map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(read(t, fixture), &doc); err != nil {
		t.Fatal(err)
	}
	holder := doc
	if feed {
		holder = doc["changes"].([]any)[0].(map[string]any)
	}
	change(holder, holder["targets"].([]any)[0].(map[string]any))
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestTheSubmissionLimitsAreExact pins each limit and each rule of the submission entry
// and of its reply at the value on each side: the number of commits and of targets, the
// repository path, the branch name, the commit id, the description and the draft title,
// and the system, which has no port.
func TestTheSubmissionLimitsAreExact(t *testing.T) {
	const entry = "fixtures/adapter/changes/submission-github-like.json"
	const reply = "fixtures/adapter/changes-submitted/github-like.json"
	feed, replies := compile(t, "changes.schema.json"), compile(t, "changes-submitted.schema.json")
	commit := func(i int) string { return fmt.Sprintf("%040x", i+1) }
	for _, c := range []struct {
		name   string
		reply  bool
		change func(doc, target map[string]any)
		ok     bool
	}{
		{"1 commit", false, func(_, tg map[string]any) { tg["commits"] = tg["commits"].([]any)[:1] }, true},
		{"0 commits", false, func(_, tg map[string]any) { tg["commits"] = []any{} }, false},
		{"100 commits", false, func(_, tg map[string]any) {
			tg["commits"] = commits(100, func(i int) any { return map[string]any{"commit": commit(i), "step": "s"} })
		}, true},
		{"101 commits", false, func(_, tg map[string]any) {
			tg["commits"] = commits(101, func(i int) any { return map[string]any{"commit": commit(i), "step": "s"} })
		}, false},
		{"100 commits and earlier_commits 40", false, func(_, tg map[string]any) {
			tg["commits"] = commits(100, func(i int) any { return map[string]any{"commit": commit(i), "step": "s"} })
			tg["earlier_commits"] = 40
		}, true},
		{"101 commits and earlier_commits 40", false, func(_, tg map[string]any) {
			tg["commits"] = commits(101, func(i int) any { return map[string]any{"commit": commit(i), "step": "s"} })
			tg["earlier_commits"] = 40
		}, false},
		{"earlier_commits 1", false, withEarlier(100, 1), true},
		{"earlier_commits 9223372036854775807", false, withEarlier(100, json.Number("9223372036854775807")), true},
		{"earlier_commits 9223372036854775808", false, withEarlier(100, json.Number("9223372036854775808")), false},
		{"earlier_commits 1.5", false, withEarlier(100, 1.5), false},
		{"earlier_commits 0", false, withEarlier(100, 0), false},
		{"earlier_commits -1", false, withEarlier(100, -1), false},
		{"earlier_commits as a string", false, withEarlier(100, "40"), false},
		{"earlier_commits null", false, withEarlier(100, nil), false},
		{"earlier_commits 40 with 99 commits", false, withEarlier(99, 40), false},
		{"earlier_commits 40 with 1 commit", false, withEarlier(1, 40), false},
		{"earlier_commits 40 with 100 commits", false, withEarlier(100, 40), true},
		{"earlier_commits in the reply", true, func(_, tg map[string]any) { tg["earlier_commits"] = 40 }, false},
		{"100 commits in the reply", true, func(_, tg map[string]any) {
			tg["commits"] = commits(100, func(i int) any {
				return map[string]any{"commit": commit(i), "commit_url": "https://code.example.com/acme/shop/commit/" + commit(i)}
			})
		}, true},
		{"101 commits in the reply", true, func(_, tg map[string]any) {
			tg["commits"] = commits(101, func(i int) any {
				return map[string]any{"commit": commit(i), "commit_url": "https://code.example.com/acme/shop/commit/" + commit(i)}
			})
		}, false},
		{"0 commits in the reply", true, func(_, tg map[string]any) { tg["commits"] = []any{} }, false},
		{"20 targets", false, func(d, tg map[string]any) { d["targets"] = targets(20, tg) }, true},
		{"21 targets", false, func(d, tg map[string]any) { d["targets"] = targets(21, tg) }, false},
		{"0 targets", false, func(d, _ map[string]any) { d["targets"] = []any{} }, false},
		{"2 equal targets", false, func(d, tg map[string]any) { d["targets"] = []any{tg, tg} }, false},
		{"20 targets in the reply", true, func(d, tg map[string]any) { d["targets"] = targets(20, tg) }, true},
		{"21 targets in the reply", true, func(d, tg map[string]any) { d["targets"] = targets(21, tg) }, false},
		{"0 targets in the reply", true, func(d, _ map[string]any) { d["targets"] = []any{} }, false},
		{"2 equal targets in the reply", true, func(d, tg map[string]any) { d["targets"] = []any{tg, tg} }, false},
		{"path with a dot segment", false, withPath("acme/./shop"), false},
		{"path with a dot-dot segment", false, withPath("acme/../shop"), false},
		{"path that is .git", false, withPath(".git"), false},
		{"path with a .git segment", false, withPath("acme/.git/shop"), false},
		{"path with .GIT segment", false, withPath("acme/.GIT"), false},
		{"path with a segment that ends in .git", false, withPath("acme/shop.git"), true},
		{"path with a dot at the start of a segment", false, withPath("acme/.shop"), true},
		{"path with a trailing slash", false, withPath("acme/shop/"), false},
		{"path with a leading slash", false, withPath("/acme/shop"), false},
		{"path with an empty segment", false, withPath("acme//shop"), false},
		{"path with a space", false, withPath("acme/my shop"), false},
		{"path of 200 characters", false, withPath(strings.Repeat("a", 200)), true},
		{"path of 201 characters", false, withPath(strings.Repeat("a", 201)), false},
		{"empty path", false, withPath(""), false},
		{"system with a port", false, func(_, tg map[string]any) { tg["system"] = "code.example.com:8443" }, false},
		{"system in upper case", false, func(_, tg map[string]any) { tg["system"] = "Code.example.com" }, false},
		{"branch with a slash", false, branch("qory/wi_8Kx2mQ4vT9"), true},
		{"branch of 255 characters", false, branch(strings.Repeat("a", 255)), true},
		{"branch of 256 characters", false, branch(strings.Repeat("a", 256)), false},
		{"branch @ alone", false, branch("@"), false},
		{"branch with @ in it", false, branch("a@b"), true},
		{"branch with ..", false, branch("a..b"), false},
		{"branch with //", false, branch("a//b"), false},
		{"branch with @{", false, branch("a@{b"), false},
		{"branch ending in .lock", false, branch("a.lock"), false},
		{"branch with a segment ending in .lock", false, branch("a/b.lock/c"), false},
		{"branch with a segment that starts with a dot", false, branch("a/.b"), false},
		{"branch that starts with a dot", false, branch(".a"), false},
		{"branch ending in a dot", false, branch("a."), false},
		{"branch ending in a slash", false, branch("a/"), false},
		{"branch starting with a slash", false, branch("/a"), false},
		{"branch starting with a hyphen", false, branch("-a"), false},
		{"branch with a hyphen inside", false, branch("a-b"), true},
		{"branch with a space", false, branch("a b"), false},
		{"branch with a tilde", false, branch("a~b"), false},
		{"branch with a colon", false, branch("a:b"), false},
		{"branch with a backslash", false, branch("a\\b"), false},
		{"branch with DEL", false, branch("a\x7fb"), false},
		{"empty branch", false, branch(""), false},
		{"base commit of 39 digits", false, baseCommit(strings.Repeat("a", 39)), false},
		{"base commit of 40 digits", false, baseCommit(strings.Repeat("a", 40)), true},
		{"base commit of 41 digits", false, baseCommit(strings.Repeat("a", 41)), false},
		{"base commit of 64 digits", false, baseCommit(strings.Repeat("a", 64)), true},
		{"base commit of 65 digits", false, baseCommit(strings.Repeat("a", 65)), false},
		{"base commit in upper case", false, baseCommit(strings.Repeat("A", 40)), false},
		{"description of 20000 characters", false, description(strings.Repeat("é", 20000)), true},
		{"description of 20001 characters", false, description(strings.Repeat("é", 20001)), false},
		{"description with the start marker on its first line", false, description("<!-- qory:start -->\nx"), false},
		{"description with the end marker on its first line", false, description("<!-- qory:end -->\nx"), false},
		{"description with the start marker on a later line", false, description("x\n<!-- qory:start -->\ny"), false},
		{"description with the end marker on a later line", false, description("x\n<!-- qory:end -->"), false},
		{"description with a marker in the middle of a line", false, description("x <!-- qory:start --> y"), true},
		{"description with a similar comment", false, description("<!-- qory:note -->"), true},
		{"title of 256 characters", false, title(strings.Repeat("a", 256)), true},
		{"title of 257 characters", false, title(strings.Repeat("a", 257)), false},
		{"title with DEL", false, title("a\x7fb"), false},
		{"title with a C1 control", false, title("a\u0085b"), false},
		{"title of spaces", false, title("   "), false},
		{"title with a no-break space and a letter", false, title("a b"), true},
		{"empty title", false, title(""), false},
		{"draft title of 256 characters in the reply", true, replyTitle(strings.Repeat("a", 256)), true},
		{"draft title of 257 characters in the reply", true, replyTitle(strings.Repeat("a", 257)), false},
		{"draft title with DEL in the reply", true, replyTitle("a\x7fb"), false},
		{"draft title of no-break spaces in the reply", true, replyTitle(" "), false},
		{"draft number 0 in the reply", true, func(_, tg map[string]any) { tg["draft"].(map[string]any)["number"] = 0 }, false},
		{"draft number 1 in the reply", true, func(_, tg map[string]any) { tg["draft"].(map[string]any)["number"] = 1 }, true},
	} {
		fixture, schema := entry, feed
		if c.reply {
			fixture, schema = reply, replies
		}
		err := validate(schema, mutate(t, fixture, !c.reply, c.change))
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", c.name, err, c.ok)
		}
	}
}

// withEarlier sets n commits and the member earlier_commits to v on a target.
func withEarlier(n int, v any) func(doc, target map[string]any) {
	return func(_, tg map[string]any) {
		tg["commits"] = commits(n, func(i int) any {
			return map[string]any{"commit": fmt.Sprintf("%040x", i+1), "step": "s"}
		})
		tg["earlier_commits"] = v
	}
}

func commits(n int, one func(int) any) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = one(i)
	}
	return out
}

// targets returns n targets that differ in their paths, each a copy of first.
func targets(n int, first map[string]any) []any {
	return commits(n, func(i int) any {
		b, _ := json.Marshal(first)
		var cp map[string]any
		_ = json.Unmarshal(b, &cp)
		cp["path"] = fmt.Sprintf("acme/shop-%d", i)
		return cp
	})
}

func withPath(p string) func(doc, target map[string]any) {
	return func(_, tg map[string]any) { tg["path"] = p }
}
func branch(b string) func(doc, target map[string]any) {
	return func(_, tg map[string]any) { tg["branch"] = b }
}
func baseCommit(c string) func(doc, target map[string]any) {
	return func(_, tg map[string]any) { tg["base"].(map[string]any)["commit"] = c }
}
func description(d string) func(doc, target map[string]any) {
	return func(doc, _ map[string]any) { doc["text"].(map[string]any)["description"] = d }
}
func title(s string) func(doc, target map[string]any) {
	return func(doc, _ map[string]any) { doc["text"].(map[string]any)["title"] = s }
}
func replyTitle(s string) func(doc, target map[string]any) {
	return func(_, tg map[string]any) { tg["draft"].(map[string]any)["title"] = s }
}

// TestAFailureDetailHasTheRuleOfALine pins the detail of a failure report of a
// submission at the value on each side: 1000 and 1001 characters, a line break, the
// next-line control, the line separator, a bidirectional override and a zero-width
// space, and that a report about a comment takes no detail.
func TestAFailureDetailHasTheRuleOfALine(t *testing.T) {
	schema := compile(t, "changes-failed.schema.json")
	submission := func(detail string) []byte {
		b, err := json.Marshal(map[string]any{"version": 1, "time": "2026-11-18T12:00:00Z",
			"submission_id": "av_7k2m9q4v1x8z3c5d", "system": "code.example.com", "path": "acme/shop",
			"reason": "forge_refused", "detail": detail})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	for _, c := range []struct {
		name   string
		detail string
		ok     bool
	}{
		{"1 character", "a", true},
		{"1000 characters", strings.Repeat("a", 1000), true},
		{"1001 characters", strings.Repeat("a", 1001), false},
		{"empty", "", false},
		{"a status and a message", "Shorten the title, then run again. The forge returned 422: title is too long.", true},
		{"a line feed", "a\nb", false},
		{"a carriage return", "a\rb", false},
		{"a next line", "a\u0085b", false},
		{"a line separator", "a\u2028b", false},
		{"a right-to-left override", "a\u202eb", false},
		{"a zero-width space", "a\u200bb", false},
	} {
		if err := validate(schema, submission(c.detail)); (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", c.name, err, c.ok)
		}
	}
	for _, reason := range []string{"not_permitted", "unreachable", "branch_gone", "too_long", "forge_refused"} {
		b, _ := json.Marshal(map[string]any{"version": 1, "time": "2026-11-18T12:00:00Z",
			"submission_id": "av_7k2m9q4v1x8z3c5d", "system": "code.example.com", "path": "acme/shop",
			"reason": reason, "detail": "Run again. The forge returned 400."})
		if err := validate(schema, b); err != nil {
			t.Errorf("%s with detail: %v", reason, err)
		}
	}
}
