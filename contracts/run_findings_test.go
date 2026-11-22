package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// findingsFamilies are the findings schemas that have a folder of valid fixtures, with a
// function that reads a fixture into the Go type of the schema.
var findingsFamilies = []struct {
	schema string
	read   func(b []byte) (any, error)
}{
	{"findings", func(b []byte) (any, error) { var v contracts.Findings; err := json.Unmarshal(b, &v); return v, err }},
	{"findings-post", func(b []byte) (any, error) { var v contracts.FindingsPost; err := json.Unmarshal(b, &v); return v, err }},
	{"findings-post-answer", func(b []byte) (any, error) {
		var v contracts.FindingsPostAnswer
		err := json.Unmarshal(b, &v)
		return v, err
	}},
	{"findings-refusal", func(b []byte) (any, error) {
		var v contracts.FindingsRefusal
		err := json.Unmarshal(b, &v)
		return v, err
	}},
}

// findingsSchemaOf is the schema of an invalid or check fixture of the findings family, by
// the prefix of its name.
func findingsSchemaOf(name string) string {
	switch {
	case strings.HasPrefix(name, "findings-post-answer-"):
		return "findings-post-answer"
	case strings.HasPrefix(name, "findings-post-"):
		return "findings-post"
	case strings.HasPrefix(name, "findings-refusal-"):
		return "findings-refusal"
	case strings.HasPrefix(name, "findings-"):
		return "findings"
	}
	return ""
}

// object returns the object at the keys of path in m: a key is a member name, or the index
// of a list item.
func object(t *testing.T, m map[string]any, keys ...any) map[string]any {
	t.Helper()
	var cur any = m
	for _, k := range keys {
		switch k := k.(type) {
		case string:
			cur = cur.(map[string]any)[k]
		case int:
			cur = cur.([]any)[k]
		}
	}
	o, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("no object at %v", keys)
	}
	return o
}

// TestEachFindingsRefusalCodeHasAFixture pins that the documents under
// fixtures/findings-refusal/ cover each code of error in findings-refusal.schema.json once,
// and no other code, and that the code open_blocking_findings is a 409.
func TestEachFindingsRefusalCodeHasAFixture(t *testing.T) {
	doc, err := contracts.RunDocument("findings-refusal.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for _, c := range doc.(map[string]any)["properties"].(map[string]any)["error"].(map[string]any)["enum"].([]any) {
		codes = append(codes, c.(string))
	}
	var got []string
	for _, f := range runFiles(t, "fixtures/findings-refusal") {
		got = append(got, fixture(t, strings.TrimPrefix(f, "fixtures/"))["error"].(string))
	}
	slices.Sort(codes)
	slices.Sort(got)
	if !slices.Equal(got, codes) {
		t.Errorf("the refusal fixtures have the codes %v, want %v", got, codes)
	}
	description := doc.(map[string]any)["description"].(string)
	for _, sentence := range []string{
		"400 invalid_request", "404 not_found", "409 base_changed", "409 open_blocking_findings",
		`"Send changes-requested in place of passed: <stage> has <count> open blocking findings."`,
		`"  The open blocking findings are <ids>."`,
	} {
		if !strings.Contains(description, sentence) {
			t.Errorf("the description of findings-refusal.schema.json does not contain %q", sentence)
		}
	}
}

// TestTheOpenBlockingFindingsMessageHasTwoLines pins the message of the fixture of the code
// open_blocking_findings: two lines, the second indented by two spaces, built from the
// stage, the number of ids and the ids of the fixture.
func TestTheOpenBlockingFindingsMessageHasTwoLines(t *testing.T) {
	m := fixture(t, "findings-refusal/open-blocking-findings.json")
	var ids []string
	for _, id := range m["findings"].([]any) {
		ids = append(ids, id.(string))
	}
	want := fmt.Sprintf("Send changes-requested in place of passed: %s has %d open blocking findings.\n"+
		"  The open blocking findings are %s.", m["stage"], len(ids), strings.Join(ids, ", "))
	if m["message"] != want {
		t.Errorf("the message is %q, want %q", m["message"], want)
	}
}

// TestAnInvalidFindingsFixtureIsRefusedForOneDefectAlone pins that each invalid fixture of
// the findings schemas is refused for its defect alone: the same document with the defect
// repaired is accepted.
func TestAnInvalidFindingsFixtureIsRefusedForOneDefectAlone(t *testing.T) {
	const version = "0123456789abcdef0123456789abcdef01234567"
	repairs := map[string]func(t *testing.T, m map[string]any){
		"findings-actor-person-with-name.json": func(t *testing.T, m map[string]any) {
			delete(object(t, m, "findings", 0, "events", 2, "actor"), "name")
		},
		"findings-base-space.json": func(t *testing.T, m map[string]any) { object(t, m, "bases", 0)["base"] = version },
		"findings-event-accepted-by-qory.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 3, "events", 1)["actor"] = map[string]any{"kind": "person"}
		},
		"findings-event-closed-fixed-by-person.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 1, "events", 2)["actor"] = map[string]any{"kind": "qory", "name": "suite"}
		},
		"findings-event-round-0.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 0, "events", 0)["round"] = json.Number("1")
		},
		"findings-event-without-reason.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 1, "events", 1)["reason"] = "Changed the order of the two steps."
		},
		"findings-events-empty.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 0)["events"] = []any{map[string]any{"kind": "raised", "round": json.Number("1"), "stage": "build", "actor": map[string]any{"kind": "qory"}}}
		},
		"findings-id-upper-case.json": func(t *testing.T, m map[string]any) { object(t, m, "findings", 0)["id"] = "f1" },
		"findings-place-line-without-file.json": func(t *testing.T, m map[string]any) {
			object(t, m, "findings", 0, "place")["file"] = "src/cart.go"
		},
		"findings-severity-high.json":    func(t *testing.T, m map[string]any) { object(t, m, "findings", 0)["severity"] = "blocking" },
		"findings-stage-upper-case.json": func(t *testing.T, m map[string]any) { object(t, m, "findings", 0)["stage"] = "build" },
		"findings-state-closed.json":     func(t *testing.T, m map[string]any) { object(t, m, "findings", 0)["state"] = "fixed" },
		"findings-without-bases.json":    func(t *testing.T, m map[string]any) { m["bases"] = []any{} },

		"findings-post-answer-count-negative.json": func(t *testing.T, m map[string]any) {
			object(t, m, "open_blocking", 0)["count"] = json.Number("0")
		},
		"findings-post-answer-id-upper-case.json":         func(t *testing.T, m map[string]any) { object(t, m, "created", 0)["id"] = "f5" },
		"findings-post-answer-without-open-blocking.json": func(t *testing.T, m map[string]any) { m["open_blocking"] = []any{} },

		"findings-post-raise-without-by.json":           func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "code-reviewer" },
		"findings-post-repeat-without-by.json":          func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "code-reviewer" },
		"findings-post-reply-fixed-without-by.json":     func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "implement-changes" },
		"findings-post-reply-disputed-without-by.json":  func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "implement-changes" },
		"findings-post-close-fixed-without-by.json":     func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "suite" },
		"findings-post-close-dismissed-without-by.json": func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["by"] = "code-reviewer" },
		"findings-post-base-space.json":                 func(t *testing.T, m map[string]any) { object(t, m, "bases", 0)["base"] = version },
		"findings-post-events-101.json":                 func(t *testing.T, m map[string]any) { m["events"] = m["events"].([]any)[:100] },
		"findings-post-events-empty.json": func(t *testing.T, m map[string]any) {
			m["events"] = []any{map[string]any{"kind": "repeat", "by": "code-reviewer", "id": "f1"}}
		},
		"findings-post-kind-accepted.json":       func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["kind"] = "repeat" },
		"findings-post-raise-with-id.json":       func(t *testing.T, m map[string]any) { delete(object(t, m, "events", 0), "id") },
		"findings-post-raise-without-proof.json": func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["proof"] = "A test fails." },
		"findings-post-raise-without-ref.json":   func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["ref"] = "r1" },
		"findings-post-ref-space.json":           func(t *testing.T, m map[string]any) { object(t, m, "events", 0)["ref"] = "r1" },
		"findings-post-repeat-with-ref.json":     func(t *testing.T, m map[string]any) { delete(object(t, m, "events", 1), "ref") },
		"findings-post-repeat-with-target.json":  func(t *testing.T, m map[string]any) { delete(object(t, m, "events", 1), "target") },
		"findings-post-repeat-without-id.json":   func(t *testing.T, m map[string]any) { object(t, m, "events", 1)["id"] = "f2" },
		"findings-post-reply-without-reason.json": func(t *testing.T, m map[string]any) {
			object(t, m, "events", 2)["reason"] = "Changed the order of the two steps."
		},

		"findings-refusal-51-findings.json":  func(t *testing.T, m map[string]any) { m["findings"] = m["findings"].([]any)[:50] },
		"findings-refusal-from-gateway.json": func(t *testing.T, m map[string]any) { m["from"] = "apiary" },
		"findings-refusal-stage-with-other-code.json": func(t *testing.T, m map[string]any) {
			delete(m, "stage")
		},
		"findings-refusal-unknown-code.json":     func(t *testing.T, m map[string]any) { m["error"] = "not_found" },
		"findings-refusal-without-findings.json": func(t *testing.T, m map[string]any) { m["findings"] = []any{"f1"} },
		"findings-refusal-without-stage.json":    func(t *testing.T, m map[string]any) { m["stage"] = "build" },
	}
	n := 0
	for _, f := range runFiles(t, "fixtures/invalid") {
		name := path.Base(f)
		schema := findingsSchemaOf(name)
		if schema == "" {
			continue
		}
		n++
		repair, ok := repairs[name]
		if !ok {
			t.Errorf("%s has no repair", name)
			continue
		}
		doc := fixture(t, "invalid/"+name)
		compiled := compileRun(t, runSchema(schema))
		if compiled.Validate(doc) == nil {
			t.Errorf("%s is accepted", name)
		}
		repair(t, doc)
		if err := compiled.Validate(doc); err != nil {
			t.Errorf("%s with its defect repaired is refused: %v", name, err)
		}
	}
	if n != len(repairs) {
		t.Errorf("fixtures/invalid has %d findings documents, and the test repairs %d", n, len(repairs))
	}
}

// TestTheFindingsDocumentsReadIntoTheirGoTypes pins that each valid fixture of the findings
// schemas reads into its Go type, is written back as the bytes of the fixture, compacted,
// and passes the Check of its type.
func TestTheFindingsDocumentsReadIntoTheirGoTypes(t *testing.T) {
	for _, fam := range findingsFamilies {
		files := runFiles(t, "fixtures/"+fam.schema)
		if len(files) == 0 {
			t.Errorf("fixtures/%s has no document", fam.schema)
		}
		for _, f := range files {
			b, err := fs.ReadFile(contracts.RunFS, f)
			if err != nil {
				t.Fatal(err)
			}
			v, err := fam.read(b)
			if err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			var want bytes.Buffer
			if err := json.Compact(&want, b); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want.Bytes()) {
				t.Errorf("%s is written back as %s, want %s", f, got, want.Bytes())
			}
			if c, ok := v.(interface{ Check() error }); ok {
				if err := c.Check(); err != nil {
					t.Errorf("%s: Check returns %v", f, err)
				}
			}
		}
	}
}

// TestCheckRefusesWhatTheFindingsSchemasCannotSay pins that each findings document under
// fixtures/check is accepted by its schema and breaks a rule that the Check of its Go type
// reports.
func TestCheckRefusesWhatTheFindingsSchemasCannotSay(t *testing.T) {
	want := map[string]string{
		"findings-id-twice.json":                `List each finding once: "f1" appears twice.`,
		"findings-base-target-twice.json":       `List the base of each target once: {"collection":"R_1001","address":{"system":"code.example.com","path":"acme/shop"}} appears twice.`,
		"findings-post-bases-target-twice.json": `List the base of each target once: {"collection":"R_1001","address":{"system":"code.example.com","path":"acme/shop"}} appears twice.`,
		"findings-post-ref-twice.json":          `Give each raise a different ref: "r1" appears twice.`,
		"findings-post-answer-ref-twice.json":   `List each ref of created once: "r1" appears twice.`,
		"findings-post-answer-stage-twice.json": `List each stage of open_blocking once: "build" appears twice.`,
	}
	n := 0
	for _, f := range runFiles(t, "fixtures/check") {
		name := path.Base(f)
		schema := findingsSchemaOf(name)
		if schema == "" {
			continue
		}
		n++
		doc, err := contracts.RunDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := compileRun(t, runSchema(schema)).Validate(doc); err != nil {
			t.Errorf("%s is refused by the schema: %v", f, err)
		}
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		for _, fam := range findingsFamilies {
			if fam.schema != schema {
				continue
			}
			v, err := fam.read(b)
			if err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			if err := v.(interface{ Check() error }).Check(); err == nil || err.Error() != want[name] {
				t.Errorf("%s: Check returns %v, want %q", f, err, want[name])
			}
		}
	}
	if n != len(want) {
		t.Errorf("fixtures/check has %d findings documents, and the test expects %d", n, len(want))
	}
}

// TestTheFindingsFixturesHaveEveryKindAndState pins that the valid fixtures contain each
// event kind, source kind, severity and state of findings.schema.json, and each event kind
// of findings-post.schema.json, so each value has a fixture.
func TestTheFindingsFixturesHaveEveryKindAndState(t *testing.T) {
	enumOf := func(schema string, def string, member string) []string {
		doc, err := contracts.RunDocument(schema + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		node := doc.(map[string]any)["$defs"].(map[string]any)[def].(map[string]any)["properties"].(map[string]any)[member]
		var out []string
		for _, v := range node.(map[string]any)["enum"].([]any) {
			out = append(out, v.(string))
		}
		slices.Sort(out)
		return out
	}
	seen := map[string]map[string]bool{}
	add := func(set, v string) {
		if seen[set] == nil {
			seen[set] = map[string]bool{}
		}
		seen[set][v] = true
	}
	for _, f := range runFiles(t, "fixtures/findings") {
		b, _ := fs.ReadFile(contracts.RunFS, f)
		var v contracts.Findings
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		for _, fd := range v.Findings {
			add("source", fd.Source.Kind)
			add("severity", fd.Severity)
			add("state", fd.State)
			for _, e := range fd.Events {
				add("kind", e.Kind)
				add("actor", e.Kind+"/"+e.Actor.Kind)
			}
		}
	}
	for _, f := range runFiles(t, "fixtures/findings-post") {
		b, _ := fs.ReadFile(contracts.RunFS, f)
		var v contracts.FindingsPost
		if err := json.Unmarshal(b, &v); err != nil {
			t.Fatal(err)
		}
		for _, e := range v.Events {
			add("post", e.Kind)
		}
	}
	for set, c := range map[string][]string{
		"kind":     enumOf("findings", "event", "kind"),
		"source":   enumOf("findings", "source", "kind"),
		"severity": nil,
		"state":    enumOf("findings", "finding", "state"),
		"post":     enumOf("findings-post", "event", "kind"),
	} {
		if set == "severity" {
			c = []string{"blocking", "non-blocking"}
		}
		got := make([]string, 0, len(seen[set]))
		for k := range seen[set] {
			got = append(got, k)
		}
		slices.Sort(got)
		if !slices.Equal(got, c) {
			t.Errorf("the fixtures have the %s values %v, want %v", set, got, c)
		}
	}
	if !seen["actor"]["reopened/qory"] || !seen["actor"]["reopened/person"] {
		t.Error("the fixtures have no reopened event of qory and of a person")
	}
}

// TestAnEventHasTheActorAndTheReasonOfItsKind pins, for each event kind of
// findings.schema.json, the actor kind that it accepts and whether it needs a reason.
func TestAnEventHasTheActorAndTheReasonOfItsKind(t *testing.T) {
	schema := compileRun(t, runSchema("findings"))
	for _, c := range []struct {
		kind         string
		qory, person bool
		reason       bool
	}{
		{"raised", true, false, false},
		{"repeated", true, false, false},
		{"replied_fixed", true, false, true},
		{"replied_disputed", true, false, true},
		{"closed_fixed", true, false, true},
		{"closed_dismissed", true, false, true},
		{"repeated_after_person_close", true, false, false},
		{"dismissed_by_person", false, true, true},
		{"accepted", false, true, false},
		{"sent_back", false, true, false},
		{"reopened", true, true, false},
	} {
		for actor, ok := range map[string]bool{"qory": c.qory, "person": c.person} {
			m := fixture(t, "findings/three-targets.json")
			e := object(t, m, "findings", 0, "events", 0)
			e["kind"] = c.kind
			e["actor"] = map[string]any{"kind": actor}
			e["reason"] = "Because."
			if err := schema.Validate(m); (err == nil) != ok {
				t.Errorf("the kind %s with the actor %s: accepted is %v, want %v: %v", c.kind, actor, err == nil, ok, err)
			}
		}
		m := fixture(t, "findings/three-targets.json")
		e := object(t, m, "findings", 0, "events", 0)
		e["kind"] = c.kind
		e["actor"] = map[string]any{"kind": "qory"}
		if !c.qory {
			e["actor"] = map[string]any{"kind": "person"}
		}
		delete(e, "reason")
		if err := schema.Validate(m); (err == nil) == c.reason {
			t.Errorf("the kind %s without a reason: accepted is %v, want %v: %v", c.kind, err == nil, !c.reason, err)
		}
	}
}

// TestTheFindingsLimitsAreExact pins the lengths of the texts, and the numbers of the lists,
// that the findings schemas bound.
func TestTheFindingsLimitsAreExact(t *testing.T) {
	get := compileRun(t, runSchema("findings"))
	post := compileRun(t, runSchema("findings-post"))
	for _, c := range []struct {
		name string
		set  func(m map[string]any, n int)
		max  int
	}{
		{"quote", func(m map[string]any, n int) { object(t, m, "findings", 0, "place")["quote"] = strings.Repeat("a", n) }, 2000},
		{"point", func(m map[string]any, n int) { object(t, m, "findings", 0)["point"] = strings.Repeat("a", n) }, 500},
		{"problem", func(m map[string]any, n int) { object(t, m, "findings", 0)["problem"] = strings.Repeat("a", n) }, 4000},
		{"reason", func(m map[string]any, n int) {
			object(t, m, "findings", 1, "events", 1)["reason"] = strings.Repeat("a", n)
		}, 2000},
		{"stage", func(m map[string]any, n int) { object(t, m, "findings", 0)["stage"] = strings.Repeat("a", n) }, 64},
		{"source name", func(m map[string]any, n int) { object(t, m, "findings", 0, "source")["name"] = strings.Repeat("a", n) }, 200},
	} {
		for _, n := range []int{0, 1, c.max, c.max + 1} {
			m := fixture(t, "findings/three-targets.json")
			c.set(m, n)
			ok := n >= 1 && n <= c.max
			if err := get.Validate(m); (err == nil) != ok {
				t.Errorf("a %s of %d characters: accepted is %v, want %v", c.name, n, err == nil, ok)
			}
		}
	}
	base := fixture(t, "findings-post/raise.json")
	for _, n := range []int{20, 21} {
		bases := make([]any, n)
		for i := range bases {
			bases[i] = map[string]any{"target": map[string]any{"collection": "R_" + strings.Repeat("1", i+1)}, "base": "0123"}
		}
		base["bases"] = bases
		if err := post.Validate(base); (err == nil) != (n == 20) {
			t.Errorf("%d bases: accepted is %v", n, err == nil)
		}
	}
	events := make([]any, 100)
	for i := range events {
		events[i] = map[string]any{"kind": "repeat", "by": "code-reviewer", "id": "f1"}
	}
	if err := post.Validate(map[string]any{"events": events}); err != nil {
		t.Errorf("100 events are refused: %v", err)
	}
}

// TestTheFindingsTypesMarshalToTheirSchemas pins that the Go types write documents that
// their schemas accept, and that the closed copy of each schema accepts them.
func TestTheFindingsTypesMarshalToTheirSchemas(t *testing.T) {
	target := contracts.Target{Collection: "R_1001", Address: &contracts.TargetAddress{System: "code.example.com", Path: "acme/shop"}}
	source := contracts.FindingSource{Kind: "reviewer", Name: "code-reviewer"}
	place := contracts.FindingPlace{Quote: "total = price", File: "src/cart.go", Line: 3, Version: "7"}
	for _, c := range []struct {
		schema string
		v      any
	}{
		{"findings", contracts.Findings{Bases: []contracts.FindingBase{{Target: target, Base: "7"}}, Findings: []contracts.Finding{{
			ID: "f1", Target: target, Stage: "build", Source: source, Severity: "blocking", Place: place, Point: "p", Problem: "a", Fix: "b",
			Proof: "c", State: "open", Events: []contracts.FindingEvent{{Kind: "raised", Round: 1, Stage: "build", Actor: contracts.FindingActor{Kind: "qory", Name: "code-reviewer"}}}}},
			MustUnderstand: []string{"/findings"}}},
		{"findings", contracts.Findings{Bases: []contracts.FindingBase{}, Findings: []contracts.Finding{}}},
		{"findings-post", contracts.FindingsPost{Events: []contracts.FindingsPostEvent{
			{Kind: "raise", By: "code-reviewer", Ref: "r1", Target: &target, Stage: "build", Source: &source, Severity: "blocking", Place: &place, Point: "p", Problem: "a", Fix: "b", Proof: "c"},
			{Kind: "close_fixed", By: "suite", ID: "f1", Reason: "The suite passes."}}}},
		{"findings-post-answer", contracts.FindingsPostAnswer{Created: []contracts.FindingCreated{{Ref: "r1", ID: "f2"}}, OpenBlocking: []contracts.FindingsOpenBlocking{{Stage: "build", Count: 0}}}},
		{"findings-post-answer", contracts.FindingsPostAnswer{Created: []contracts.FindingCreated{}, OpenBlocking: []contracts.FindingsOpenBlocking{}}},
		{"findings-refusal", contracts.FindingsRefusal{Error: "open_blocking_findings", From: "apiary", Stage: "build", Findings: []string{"f1"}}},
		{"findings-refusal", contracts.FindingsRefusal{Error: "invalid_request", Names: []string{"events"}, From: "apiary"}},
	} {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		if err := dec.Decode(&doc); err != nil {
			t.Fatal(err)
		}
		if err := compileRun(t, runSchema(c.schema)).Validate(doc); err != nil {
			t.Errorf("%s: %T writes %s, which the schema refuses: %v", c.schema, c.v, b, err)
		}
		if c.schema == "findings-refusal" {
			continue
		}
		closed, err := contracts.CompileRunClosed(c.schema + ".schema.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := closed.Validate(doc); err != nil {
			t.Errorf("%s: %T writes %s, which the closed copy refuses: %v", c.schema, c.v, b, err)
		}
	}
}
