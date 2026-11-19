package contracts_test

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// TestAWorkflowRunHasASubjectAndATrigger pins that run documents of the step workflow
// with a subject of kind work_item and the trigger take_in are accepted, that the step
// explore and the member work_item are refused, and that the documents read into
// [contracts.Documents] with the subject and the trigger.
func TestAWorkflowRunHasASubjectAndATrigger(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	doc := fixture(t, "documents/explore.json")
	if doc["step"] != "workflow" || doc["trigger"] != "take_in" {
		t.Fatalf("documents/explore.json has step %v and trigger %v, want workflow and take_in", doc["step"], doc["trigger"])
	}
	if subject, _ := doc["subject"].(map[string]any); subject["kind"] != "work_item" {
		t.Fatalf("documents/explore.json has subject %v, want kind work_item", doc["subject"])
	}
	if err := schema.Validate(doc); err != nil {
		t.Errorf("run documents of the step workflow with a work_item subject and take_in are refused: %v", err)
	}
	var d contracts.Documents
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if d.Subject == nil || d.Subject.Kind != "work_item" || d.Trigger != "take_in" {
		t.Errorf("documents/explore.json reads as subject %+v and trigger %q", d.Subject, d.Trigger)
	}
	doc["step"] = "explore"
	if schema.Validate(doc) == nil {
		t.Error("run documents of the step explore are accepted")
	}
	delete(doc, "run_settings")
	if schema.Validate(doc) == nil {
		t.Error("run documents of the step explore without run_settings are accepted")
	}
	for member, value := range map[string]any{
		"run_settings":       map[string]any{"runtime": "claude"},
		"time_limit_minutes": 15,
		"shared_run":         true,
		"full_suite":         true,
	} {
		for _, c := range []struct {
			fixture string
			ok      bool
		}{{"documents/explore.json", true}, {"documents/prebuild.json", false}} {
			d := fixture(t, c.fixture)
			d[member] = value
			if err := schema.Validate(d); (err == nil) != c.ok {
				t.Errorf("%s with %s: accepted %v, want %v: %v", c.fixture, member, err == nil, c.ok, err)
			}
		}
	}
}

// TestASubjectAndATriggerAreBoundedAtTheirEdges pins the edges of subject and trigger
// in documents.schema.json: the longest kind, id, title and trigger are accepted and one
// character more is refused; an id outside the pattern wi_ is accepted for a kind other
// than work_item and refused for work_item; and a subject has no url and no title.
func TestASubjectAndATriggerAreBoundedAtTheirEdges(t *testing.T) {
	schema := compileRun(t, runSchema("documents"))
	for _, c := range []struct {
		name   string
		change func(d map[string]any)
		ok     bool
	}{
		{"kind of 64 characters", func(d map[string]any) { subject(d)["kind"] = strings.Repeat("a", 64) }, true},
		{"kind of 65 characters", func(d map[string]any) { subject(d)["kind"] = strings.Repeat("a", 65) }, false},
		{"kind of words that single underscores join", func(d map[string]any) { subject(d)["kind"] = "support_ticket_2" }, true},
		{"kind with a double underscore", func(d map[string]any) { subject(d)["kind"] = "support__ticket" }, false},
		{"kind in camel case", func(d map[string]any) { subject(d)["kind"] = "workItem" }, false},
		{"kind that starts with a digit", func(d map[string]any) { subject(d)["kind"] = "2fa" }, false},
		{"id of 200 characters, kind ticket", func(d map[string]any) { subject(d)["kind"], subject(d)["id"] = "ticket", strings.Repeat("a", 200) }, true},
		{"id of 201 characters, kind ticket", func(d map[string]any) { subject(d)["kind"], subject(d)["id"] = "ticket", strings.Repeat("a", 201) }, false},
		{"id without the prefix wi_, kind ticket", func(d map[string]any) { subject(d)["kind"], subject(d)["id"] = "ticket", "TRK-1" }, true},
		{"id without the prefix wi_, kind work_item", func(d map[string]any) { subject(d)["id"] = "TRK-1" }, false},
		{"id with a line feed, kind ticket", func(d map[string]any) { subject(d)["kind"], subject(d)["id"] = "ticket", "TRK-1\nTRK-2" }, false},
		{"title of 256 characters", func(d map[string]any) { subject(d)["title"] = strings.Repeat("a", 256) }, true},
		{"title of 257 characters", func(d map[string]any) { subject(d)["title"] = strings.Repeat("a", 257) }, false},
		{"no url and no title", func(d map[string]any) { delete(subject(d), "url"); delete(subject(d), "title") }, true},
		{"no trigger", func(d map[string]any) { delete(d, "trigger") }, true},
		{"trigger of 64 characters", func(d map[string]any) { d["trigger"] = strings.Repeat("a", 64) }, true},
		{"trigger of 65 characters", func(d map[string]any) { d["trigger"] = strings.Repeat("a", 65) }, false},
		{"trigger that no contract lists", func(d map[string]any) { d["trigger"] = "scheduled_run" }, true},
		{"trigger with a hyphen", func(d map[string]any) { d["trigger"] = "take-in" }, false},
	} {
		d := fixture(t, "documents/explore.json")
		c.change(d)
		if err := schema.Validate(d); (err == nil) != c.ok {
			t.Errorf("%s: accepted %v, want %v: %v", c.name, err == nil, c.ok, err)
		}
	}
}

func subject(d map[string]any) map[string]any {
	s, _ := d["subject"].(map[string]any)
	return s
}
