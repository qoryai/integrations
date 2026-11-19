package contracts_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// changeDoc decodes the fixture and applies change to the entry (the first entry of a
// feed fixture) or to the reply (any other fixture), and returns the JSON.
func changeDoc(t *testing.T, fixture string, feed bool, change func(doc map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(read(t, fixture), &doc); err != nil {
		t.Fatal(err)
	}
	holder := doc
	if feed {
		holder = doc["changes"].([]any)[0].(map[string]any)
	}
	change(holder)
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// set returns a change that sets one member.
func set(member string, v any) func(doc map[string]any) {
	return func(doc map[string]any) { doc[member] = v }
}

// data returns a change that sets the data member.
func data(v any) func(doc map[string]any) { return set("data", v) }

// repeat returns n copies of the value that one(i) returns.
func repeat(n int, one func(int) any) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = one(i)
	}
	return out
}

// members returns an object of n members, each with the value 1.
func members(n int) map[string]any {
	m := map[string]any{}
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("m%d", i)] = 1
	}
	return m
}

// TestTheChangeRequestLimitsAreExact pins each limit and each rule of the change_request
// entry and of its reply at the value on each side: the title, the text and its marker
// lines, the kind, the id, the previous version, and the limits of the part.
func TestTheChangeRequestLimitsAreExact(t *testing.T) {
	const entry = "fixtures/adapter/changes/change-request-pull-request.json"
	const reply = "fixtures/adapter/changes-submitted/pull-request-created.json"
	feed, replies := compile(t, "changes.schema.json"), compile(t, "changes-submitted.schema.json")
	for _, c := range []struct {
		name   string
		reply  bool
		change func(doc map[string]any)
		ok     bool
	}{
		{"title of 256 characters", false, set("title", strings.Repeat("a", 256)), true},
		{"title of 257 characters", false, set("title", strings.Repeat("a", 257)), false},
		{"title with DEL", false, set("title", "a\x7fb"), false},
		{"title with a C1 control", false, set("title", "a\u0085b"), false},
		{"title of spaces", false, set("title", "   "), false},
		{"title with a no-break space and a letter", false, set("title", "a b"), true},
		{"title of one character", false, set("title", "a"), true},
		{"empty title", false, set("title", ""), false},
		{"empty text", false, set("text", ""), true},
		{"text of 20000 characters", false, set("text", strings.Repeat("é", 20000)), true},
		{"text of 20001 characters", false, set("text", strings.Repeat("é", 20001)), false},
		{"text with the start marker on its first line", false, set("text", "<!-- qory:start -->\nx"), false},
		{"text with the end marker on its first line", false, set("text", "<!-- qory:end -->\nx"), false},
		{"text with the start marker on a later line", false, set("text", "x\n<!-- qory:start -->\ny"), false},
		{"text with the end marker on a later line", false, set("text", "x\n<!-- qory:end -->"), false},
		{"text with a marker in the middle of a line", false, set("text", "x <!-- qory:start --> y"), true},
		{"text with a similar comment", false, set("text", "<!-- qory:note -->"), true},
		{"text with U+0000", false, set("text", "a\x00b"), false},
		{"change kind of 1 character", false, set("change_kind", "a"), true},
		{"change kind of 64 characters", false, set("change_kind", strings.Repeat("a", 64)), true},
		{"change kind of 65 characters", false, set("change_kind", strings.Repeat("a", 65)), false},
		{"empty change kind", false, set("change_kind", ""), false},
		{"change kind with a hyphen", false, set("change_kind", "a-b"), true},
		{"change kind with a digit first", false, set("change_kind", "3d-print"), true},
		{"change kind with two hyphens", false, set("change_kind", "a--b"), false},
		{"change kind with a hyphen first", false, set("change_kind", "-a"), false},
		{"change kind with a hyphen last", false, set("change_kind", "a-"), false},
		{"change kind in upper case", false, set("change_kind", "A"), false},
		{"change kind with an underscore", false, set("change_kind", "a_b"), false},
		{"change kind with a space", false, set("change_kind", "a b"), false},
		{"id of 16 characters", false, set("change_request_id", "av_7k2m9q4v1x8z3c5d"), true},
		{"id of 15 characters", false, set("change_request_id", "av_7k2m9q4v1x8z3c5"), false},
		{"id of 17 characters", false, set("change_request_id", "av_7k2m9q4v1x8z3c5de"), false},
		{"id in upper case", false, set("change_request_id", "av_7K2M9Q4V1X8Z3C5D"), false},
		{"id with a letter that Crockford base32 leaves out", false, set("change_request_id", "av_7k2m9q4v1x8z3c5u"), false},
		{"previous with a title", false, set("previous", map[string]any{"title": "Show dates"}), true},
		{"previous with an address", false, set("previous", map[string]any{"address": map[string]any{"url": "https://code.example.com/acme/shop/pull/34"}}), true},
		{"previous without a member", false, set("previous", map[string]any{}), false},
		{"previous address ref of 200 characters", false, set("previous", map[string]any{"address": map[string]any{
			"url": "https://code.example.com/acme/shop/pull/34", "ref": strings.Repeat("1", 200)}}), true},
		{"previous address ref of 201 characters", false, set("previous", map[string]any{"address": map[string]any{
			"url": "https://code.example.com/acme/shop/pull/34", "ref": strings.Repeat("1", 201)}}), false},
		{"previous address with a url that is not https", false, set("previous", map[string]any{"address": map[string]any{
			"url": "http://code.example.com/acme/shop/pull/34"}}), false},
		{"empty data", false, data(map[string]any{}), true},
		{"data with 1024 items in an array", false, data(map[string]any{"a": repeat(1024, func(int) any { return 1 })}), true},
		{"data with 1025 items in an array", false, data(map[string]any{"a": repeat(1025, func(int) any { return 1 })}), false},
		{"data with 1025 items in an array inside an array", false, data(map[string]any{"a": []any{[]any{repeat(1025, func(int) any { return 1 })}}}), false},
		{"data with 256 members", false, data(members(256)), true},
		{"data with 257 members", false, data(members(257)), false},
		{"data with 257 members inside an object", false, data(map[string]any{"a": members(257)}), false},
		{"data with a string of 20000 characters", false, data(map[string]any{"a": strings.Repeat("é", 20000)}), true},
		{"data with a string of 20001 characters", false, data(map[string]any{"a": strings.Repeat("é", 20001)}), false},
		{"data with a member name of 128 characters", false, data(map[string]any{strings.Repeat("a", 128): 1}), true},
		{"data with a member name of 129 characters", false, data(map[string]any{strings.Repeat("a", 129): 1}), false},
		{"data with an empty member name", false, data(map[string]any{"": 1}), false},
		{"data with U+0000 in a string", false, data(map[string]any{"a": "x\x00y"}), false},
		{"data with U+0000 in a member name", false, data(map[string]any{"x\x00y": 1}), false},
		{"data with a number with a fraction", false, data(map[string]any{"bid": 1.25}), true},
		{"data that is an array", false, data([]any{}), false},
		{"data that is null", false, data(nil), false},
		{"entry without a title", false, func(d map[string]any) { delete(d, "title") }, false},
		{"entry without a text", false, func(d map[string]any) { delete(d, "text") }, false},
		{"entry without a target", false, func(d map[string]any) { delete(d, "target") }, false},
		{"entry without data", false, func(d map[string]any) { delete(d, "data") }, false},
		{"entry without a ref", false, func(d map[string]any) { delete(d, "ref") }, true},
		{"entry without a previous", false, func(d map[string]any) { delete(d, "previous") }, true},
		{"entry with an empty ref", false, set("ref", ""), false},
		{"target with a collection and no address", false, set("target", map[string]any{"collection": "R_1001"}), true},
		{"reply with neither address nor action", true, func(d map[string]any) { delete(d, "address"); delete(d, "action") }, true},
		{"reply with an address and no action", true, func(d map[string]any) { delete(d, "action") }, false},
		{"reply with an action and no address", true, func(d map[string]any) { delete(d, "address") }, false},
		{"reply with the action updated", true, set("action", "updated"), true},
		{"reply with the action kept", true, set("action", "kept"), true},
		{"reply with the action opened", true, set("action", "opened"), false},
		{"reply with an address url that is not https", true, set("address", map[string]any{"url": "http://code.example.com/acme/shop/pull/34"}), false},
		{"reply with an address ref of 200 characters", true, set("address", map[string]any{"url": "https://code.example.com/acme/shop/pull/34", "ref": strings.Repeat("1", 200)}), true},
		{"reply with an address ref of 201 characters", true, set("address", map[string]any{"url": "https://code.example.com/acme/shop/pull/34", "ref": strings.Repeat("1", 201)}), false},
		{"reply with an address ref that is a number", true, set("address", map[string]any{"url": "https://code.example.com/acme/shop/pull/34", "ref": 34}), false},
		{"reply without data", true, func(d map[string]any) { delete(d, "data") }, true},
		{"reply with empty data", true, data(map[string]any{}), true},
		{"reply with 257 members in data", true, data(members(257)), false},
		{"reply with 256 members in data", true, data(members(256)), true},
		{"reply with a string of 20001 characters in data", true, data(map[string]any{"a": strings.Repeat("a", 20001)}), false},
		{"reply with U+0000 in data", true, data(map[string]any{"a": "x\x00y"}), false},
		{"reply without a change request id", true, func(d map[string]any) { delete(d, "change_request_id") }, false},
		{"reply with the old members", true, set("targets", []any{}), false},
	} {
		fixture, schema := entry, feed
		if c.reply {
			fixture, schema = reply, replies
		}
		err := validate(schema, changeDoc(t, fixture, !c.reply, c.change))
		if (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", c.name, err, c.ok)
		}
	}
}

// TestAChangeRequestOfEachKindIsAccepted pins the card's fixtures: an entry of the kinds
// pull-request, reply and bid-changes is accepted, each with the members of its kind in
// data, and an entry without a target or without a title is refused.
func TestAChangeRequestOfEachKindIsAccepted(t *testing.T) {
	feed := compile(t, "changes.schema.json")
	for kind, file := range map[string]string{
		"pull-request": "change-request-pull-request.json",
		"reply":        "change-request-reply.json",
		"bid-changes":  "change-request-bid-changes.json",
	} {
		b := read(t, "fixtures/adapter/changes/"+file)
		if err := validate(feed, b); err != nil {
			t.Errorf("%s: %v", file, err)
		}
		var f struct {
			Changes []struct {
				ChangeKind string `json:"change_kind"`
			}
		}
		if err := json.Unmarshal(b, &f); err != nil {
			t.Fatal(err)
		}
		if len(f.Changes) != 1 || f.Changes[0].ChangeKind != kind {
			t.Errorf("%s has the change kinds %+v, want %s", file, f.Changes, kind)
		}
		for _, member := range []string{"target", "title"} {
			without := changeDoc(t, "fixtures/adapter/changes/"+file, true, func(d map[string]any) { delete(d, member) })
			if err := validate(feed, without); err == nil {
				t.Errorf("%s without %s is accepted", file, member)
			}
		}
	}
	var two struct {
		Changes []struct {
			ID       string `json:"change_request_id"`
			Previous *struct {
				Title   string `json:"title"`
				Address *struct{ Ref string }
			}
			Title string
		}
	}
	if err := json.Unmarshal(read(t, "fixtures/adapter/changes/change-request-pull-request-two-versions.json"), &two); err != nil {
		t.Fatal(err)
	}
	c := two.Changes
	if len(c) != 2 || c[0].Previous != nil || c[1].Previous == nil || c[0].ID == c[1].ID {
		t.Fatalf("change-request-pull-request-two-versions.json: the first version has no previous and the second has one: %+v", c)
	}
	if c[1].Previous.Title != c[0].Title {
		t.Errorf("the second version has the previous title %q, and the first has the title %q", c[1].Previous.Title, c[0].Title)
	}
	if c[1].Previous.Address == nil || c[1].Previous.Address.Ref != "34" {
		t.Errorf("the second version does not have the address of the pull request 34 in previous")
	}
	for _, name := range []string{"pull-request-created", "pull-request-kept-merged", "pull-request-another-host", "pull-request-commits-only", "reply", "bid-changes"} {
		if err := validate(compile(t, "changes-submitted.schema.json"), read(t, "fixtures/adapter/changes-submitted/"+name+".json")); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestAChangeKindHasThePatternOfASubmitType pins that change_kind has the pattern and
// the length of submit.type of the run documents, so the type that a run stores a
// change request under is always a kind that an adapter can list.
func TestAChangeKindHasThePatternOfASubmitType(t *testing.T) {
	doc, err := contracts.RunDocument("documents.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	submitType := doc.(map[string]any)["$defs"].(map[string]any)["submit"].(map[string]any)["properties"].(map[string]any)["type"].(map[string]any)
	kind := node(t, "definitions.schema.json", "/$defs/change_kind")
	for _, keyword := range []string{"pattern", "maxLength"} {
		if kind[keyword] == nil || fmt.Sprint(kind[keyword]) != fmt.Sprint(submitType[keyword]) {
			t.Errorf("change_kind has the %s %v, and submit.type has %v", keyword, kind[keyword], submitType[keyword])
		}
	}
}

// failure returns the JSON of a failure report of a change request, with the members of
// extra added.
func failure(extra map[string]any) []byte {
	m := map[string]any{"version": 1, "time": "2026-11-18T12:00:00Z", "change_request_id": "av_7k2m9q4v1x8z3c5d",
		"target": map[string]any{"collection": "R_1001"}, "reason": "refused"}
	for k, v := range extra {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return b
}

// TestAChangeRequestFailureHasTheReasonsOfItsKind pins the five reasons of a failure
// report of a change request and the four of a comment, and that the old reasons
// branch_gone and forge_refused are refused for both.
func TestAChangeRequestFailureHasTheReasonsOfItsKind(t *testing.T) {
	schema := compile(t, "changes-failed.schema.json")
	for reason, ok := range map[string]bool{
		"not_permitted": true, "unreachable": true, "target_gone": true, "too_long": true, "refused": true,
		"branch_gone": false, "forge_refused": false, "item_gone": false, "kind_unknown": false,
	} {
		if err := validate(schema, failure(map[string]any{"reason": reason})); (err == nil) != ok {
			t.Errorf("%s for a change request: %v, want accepted %v", reason, err, ok)
		}
	}
	for reason, ok := range map[string]bool{
		"item_gone": true, "not_permitted": true, "too_long": true, "unreachable": true,
		"target_gone": false, "refused": false, "branch_gone": false, "forge_refused": false,
	} {
		b, _ := json.Marshal(map[string]any{"version": 1, "time": "2026-11-18T12:00:00Z", "comment_id": "sc_7k2m9q4v1x8z3c5d", "reason": reason})
		if err := validate(schema, b); (err == nil) != ok {
			t.Errorf("%s for a comment: %v, want accepted %v", reason, err, ok)
		}
	}
	// An adapter that gets a kind it did not list reports refused, with a detail.
	const unlisted = "List pull-request in change_kinds, or leave it out of the process. The adapter does not apply pull-request."
	if err := validate(schema, failure(map[string]any{"reason": "refused", "detail": unlisted})); err != nil {
		t.Errorf("refused with the detail for an unlisted kind: %v", err)
	}
	for name, extra := range map[string]map[string]any{
		"without a target":  {"target": nil},
		"with a comment id": {"comment_id": "sc_7k2m9q4v1x8z3c5d"},
	} {
		doc := map[string]any{}
		if err := json.Unmarshal(failure(nil), &doc); err != nil {
			t.Fatal(err)
		}
		for k, v := range extra {
			if v == nil {
				delete(doc, k)
			} else {
				doc[k] = v
			}
		}
		b, _ := json.Marshal(doc)
		if err := validate(schema, b); err == nil {
			t.Errorf("a failure %s is accepted", name)
		}
	}
}

// TestAFailureDetailHasTheRuleOfALine pins the detail of a failure report of a change
// request at the value on each side: 1000 and 1001 characters, a line break, the
// next-line control, the line separator, a bidirectional override and a zero-width
// space, and that a report about a comment takes no detail.
func TestAFailureDetailHasTheRuleOfALine(t *testing.T) {
	schema := compile(t, "changes-failed.schema.json")
	for _, c := range []struct {
		name   string
		detail string
		ok     bool
	}{
		{"1 character", "a", true},
		{"1000 characters", strings.Repeat("a", 1000), true},
		{"1001 characters", strings.Repeat("a", 1001), false},
		{"empty", "", false},
		{"a status and a message", "Shorten the title, then try again. The target returned 422: title is too long.", true},
		{"a line feed", "a\nb", false},
		{"a carriage return", "a\rb", false},
		{"a next line", "a\u0085b", false},
		{"a line separator", "a b", false},
		{"a right-to-left override", "a‮b", false},
		{"a zero-width space", "a​b", false},
	} {
		if err := validate(schema, failure(map[string]any{"detail": c.detail})); (err == nil) != c.ok {
			t.Errorf("%s: %v, want accepted %v", c.name, err, c.ok)
		}
	}
	for _, reason := range []string{"not_permitted", "unreachable", "target_gone", "too_long", "refused"} {
		if err := validate(schema, failure(map[string]any{"reason": reason, "detail": "Try again. The target returned 400."})); err != nil {
			t.Errorf("%s with detail: %v", reason, err)
		}
	}
	comment, _ := json.Marshal(map[string]any{"version": 1, "time": "2026-11-18T12:00:00Z", "comment_id": "sc_7k2m9q4v1x8z3c5d",
		"reason": "too_long", "detail": "Shorten the comment."})
	if err := validate(schema, comment); err == nil {
		t.Error("a report about a comment with a detail is accepted")
	}
}
