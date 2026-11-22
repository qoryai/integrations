package contracts

import (
	"encoding/json"
	"fmt"
)

// Findings is the 200 answer to GET /v1/runs/<id>/findings, run/v1
// findings.schema.json: the findings of the run's work item on the run's targets, open
// ones first, each with its events, and the base of each target. A finding is one
// problem that a reviewer, a check, a criterion, a scenario or a hook raised about a
// target. Qory Apiary keeps the findings and mints their ids. A reader ignores a member
// that Findings does not have, unless MustUnderstand lists it.
type Findings struct {
	// Bases has one entry for each target whose base Qory Apiary has stored, at most 20.
	Bases []FindingBase `json:"bases,case:strict"`
	// Findings are at most 500.
	Findings []Finding `json:"findings,case:strict"`
	// MustUnderstand lists members of the answer that the reader must understand, as
	// [Documents.MustUnderstand] does for the documents.
	MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
}

// FindingBase is the base of a target: the version that the first run of the work item on
// the target started from. Qory Apiary stores it once and refuses a different one later.
type FindingBase struct {
	Target Target `json:"target,case:strict"`
	// Base follows the grammar of [Documents.TargetVersion].
	Base string `json:"base,case:strict"`
}

// Finding is one finding of [Findings], run/v1 findings.schema.json, $defs.finding.
type Finding struct {
	// ID is f and a number from 1, minted by Qory Apiary for the work item, such as f12.
	ID     string `json:"id,case:strict"`
	Target Target `json:"target,case:strict"`
	// Stage is the stage that owns the finding: its re-check closes the finding, and an
	// open blocking finding of the stage keeps the stage from passing.
	Stage    string        `json:"stage,case:strict"`
	Source   FindingSource `json:"source,case:strict"`
	Severity string        `json:"severity,case:strict"`
	Place    FindingPlace  `json:"place,case:strict"`
	// Point, Problem, Fix and Proof are the texts of the report that raised the finding.
	Point   string `json:"point,case:strict"`
	Problem string `json:"problem,case:strict"`
	Fix     string `json:"fix,case:strict"`
	Proof   string `json:"proof,case:strict"`
	// State is "open", "fixed", "dismissed" or "accepted".
	State string `json:"state,case:strict"`
	// Disputed is true when a reply disputed the finding. The finding stays open.
	Disputed bool `json:"disputed,case:strict"`
	// Events are 1 to 100, oldest first.
	Events []FindingEvent `json:"events,case:strict"`
}

// FindingSource is what raised a finding. Kind is "reviewer", "check", "criterion",
// "scenario" or "hook", and Name is the role, check, criterion, scenario or hook.
type FindingSource struct {
	Kind string `json:"kind,case:strict"`
	Name string `json:"name,case:strict"`
}

// FindingPlace is where a finding is: the Quote, the File and Line when the target has
// files, and the Version of the artifact that was reviewed. Line is set only with File.
type FindingPlace struct {
	Quote   string `json:"quote,case:strict"`
	File    string `json:"file,omitempty,case:strict"`
	Line    int    `json:"line,omitempty,case:strict"`
	Version string `json:"version,omitempty,case:strict"`
}

// FindingEvent is one event of a finding. Kind is one of "raised", "repeated",
// "replied_fixed", "replied_disputed", "closed_fixed", "closed_dismissed" and
// "repeated_after_person_close", whose Actor is qory; "dismissed_by_person", "accepted" and
// "sent_back", whose Actor is a person; or "reopened", whose Actor is qory when a reviewer
// repeats a finding that a run closed, and a person when a person reopens it. A person
// closes a finding with "dismissed_by_person" or "accepted". A run cannot reopen a finding
// that a person closed: a run that repeats it adds "repeated_after_person_close", and the
// finding stays closed.
type FindingEvent struct {
	Kind string `json:"kind,case:strict"`
	// Round is the stage_round of the work item when the event was written, from 1. It
	// restarts after a return, so each event has its Stage.
	Round int          `json:"round,case:strict"`
	Stage string       `json:"stage,case:strict"`
	Actor FindingActor `json:"actor,case:strict"`
	// Version is the version of the target that the event concerns, as
	// [Documents.TargetVersion].
	Version string `json:"version,omitempty,case:strict"`
	// Reason is 1 to 2000 characters. It is set for "replied_fixed", "replied_disputed",
	// "closed_fixed", "closed_dismissed" and "dismissed_by_person", and optional for every
	// other kind.
	Reason string `json:"reason,omitempty,case:strict"`
}

// FindingActor is who wrote an event. For Kind "qory", Name is the role of the session
// whose report the event came from, such as code-reviewer or implement-changes, or the
// name of the check, such as suite, matrix or hook-1. For Kind "person", Name is empty:
// the run API does not show the user id.
type FindingActor struct {
	Kind string `json:"kind,case:strict"`
	Name string `json:"name,omitempty,case:strict"`
}

// FindingsPost is the body of POST /v1/runs/<id>/findings, run/v1
// findings-post.schema.json: a batch of events that qory writes after a round. The answer
// is [FindingsPostAnswer]. Qory Apiary refuses a request with a [FindingsRefusal].
type FindingsPost struct {
	// Bases are the bases of the targets that the events concern. Qory Apiary stores a
	// base once for the work item and the target, and refuses a different one later with
	// the code base_changed. At most 20.
	Bases []FindingBase `json:"bases,omitempty,case:strict"`
	// Events are 1 to 100.
	Events         []FindingsPostEvent `json:"events,case:strict"`
	MustUnderstand []string            `json:"must_understand,omitempty,case:strict"`
}

// FindingsPostEvent is one event of a [FindingsPost]. Kind is "raise", "repeat",
// "reply_fixed", "reply_disputed", "close_fixed" or "close_dismissed". A "raise" has Ref,
// a key that is unique in the batch, and the members from Target to Proof, which a
// finding has, and no ID. Each other kind has ID and none of those members. Reason is
// set for each kind except "raise" and "repeat".
type FindingsPostEvent struct {
	Kind     string         `json:"kind,case:strict"`
	Ref      string         `json:"ref,omitempty,case:strict"`
	ID       string         `json:"id,omitempty,case:strict"`
	Reason   string         `json:"reason,omitempty,case:strict"`
	Version  string         `json:"version,omitempty,case:strict"`
	Target   *Target        `json:"target,omitempty,case:strict"`
	Stage    string         `json:"stage,omitempty,case:strict"`
	Source   *FindingSource `json:"source,omitempty,case:strict"`
	Severity string         `json:"severity,omitempty,case:strict"`
	Place    *FindingPlace  `json:"place,omitempty,case:strict"`
	Point    string         `json:"point,omitempty,case:strict"`
	Problem  string         `json:"problem,omitempty,case:strict"`
	Fix      string         `json:"fix,omitempty,case:strict"`
	Proof    string         `json:"proof,omitempty,case:strict"`
}

// Check reports the first rule of findings-post.schema.json that the batch breaks and that
// the schema does not express: no two events have the same Ref. Each message starts with
// the fix. A writer calls it before it sends the batch, and Qory Apiary applies the same
// rule.
func (p FindingsPost) Check() error {
	refs := make(map[string]bool, len(p.Events))
	for _, e := range p.Events {
		if e.Ref == "" {
			continue
		}
		if refs[e.Ref] {
			return fmt.Errorf("Give each raise a different ref: %q appears twice.", e.Ref)
		}
		refs[e.Ref] = true
	}
	return nil
}

// FindingsPostAnswer is the 200 answer to a [FindingsPost], run/v1
// findings-post-answer.schema.json.
type FindingsPostAnswer struct {
	// Created has the id of each raise of the batch, by its ref.
	Created []FindingCreated `json:"created,case:strict"`
	// OpenBlocking is the count of open blocking findings of each stage after the batch.
	// A stage that is not listed has none.
	OpenBlocking   []FindingsOpenBlocking `json:"open_blocking,case:strict"`
	MustUnderstand []string               `json:"must_understand,omitempty,case:strict"`
}

// FindingCreated is the id that Qory Apiary minted for the raise with the Ref.
type FindingCreated struct {
	Ref string `json:"ref,case:strict"`
	ID  string `json:"id,case:strict"`
}

// FindingsOpenBlocking is the number of open blocking findings that a stage owns.
type FindingsOpenBlocking struct {
	Stage string `json:"stage,case:strict"`
	Count int    `json:"count,case:strict"`
}

// FindingsRefusal is the body of a coded refusal of Qory Apiary to a findings request or
// to a verdict, run/v1 findings-refusal.schema.json. It has the members of Forager's link
// refusal, with From "apiary". Error is "invalid_request" (400, with Names),
// "not_found" (404), "base_changed" (409) or "open_blocking_findings" (409). The code
// open_blocking_findings answers a verdict "passed" of a stage while the stage owns an open
// blocking finding: Stage is the stage and Findings are the ids of up to 50 such findings.
type FindingsRefusal struct {
	Error    string   `json:"error,case:strict"`
	Names    []string `json:"names,omitempty,case:strict"`
	Message  string   `json:"message,omitempty,case:strict"`
	From     string   `json:"from,case:strict"`
	Stage    string   `json:"stage,omitempty,case:strict"`
	Findings []string `json:"findings,omitempty,case:strict"`
}

// Check reports the first rule of findings.schema.json that the answer breaks and that the
// schema does not express: no two findings have the same ID, and no two entries of Bases
// have the same target.
func (f Findings) Check() error {
	ids := make(map[string]bool, len(f.Findings))
	for _, e := range f.Findings {
		if ids[e.ID] {
			return fmt.Errorf("List each finding once: %q appears twice.", e.ID)
		}
		ids[e.ID] = true
	}
	targets := make(map[string]bool, len(f.Bases))
	for _, b := range f.Bases {
		key, err := json.Marshal(b.Target)
		if err != nil {
			return err
		}
		if targets[string(key)] {
			return fmt.Errorf("List the base of each target once: %s appears twice.", key)
		}
		targets[string(key)] = true
	}
	return nil
}

// Check reports the first rule of findings-post-answer.schema.json that the answer breaks
// and that the schema does not express: no two entries of Created have the same Ref, and no
// two entries of OpenBlocking have the same Stage.
func (a FindingsPostAnswer) Check() error {
	refs := make(map[string]bool, len(a.Created))
	for _, c := range a.Created {
		if refs[c.Ref] {
			return fmt.Errorf("List each ref of created once: %q appears twice.", c.Ref)
		}
		refs[c.Ref] = true
	}
	stages := make(map[string]bool, len(a.OpenBlocking))
	for _, o := range a.OpenBlocking {
		if stages[o.Stage] {
			return fmt.Errorf("List each stage of open_blocking once: %q appears twice.", o.Stage)
		}
		stages[o.Stage] = true
	}
	return nil
}
