package contracts

import "encoding/json"

// The query parameters of GET /v1/runs, the run list, diagnostics/v1 runs.schema.json.
// The request is signed with a diagnostics key, as GET /v1/runs/<run id>/diagnostics is.
// Every parameter is optional and appears at most once, and each row of the answer
// matches every parameter in the request. A work_item, workflow or state that no run
// has gives an empty Runs. A parameter that is not one of these, a parameter given
// twice, an empty value or a value outside its rule is refused with 400 and a
// [RunListRefusal] whose Error is [RunListInvalidRequest] and whose Parameter is the
// parameter's name.
const (
	// RunListQueryWorkspace selects the runs of one workspace, as
	// <organisation>/<workspace>: the two slugs, such as example-org/example. A
	// workspace that does not exist is refused.
	RunListQueryWorkspace = "workspace"
	// RunListQueryWorkItem selects the runs of one work item, by its number in the
	// workspace, from 1, at most 9223372036854775807; a larger value is refused, 400
	// invalid_request. It is refused without [RunListQueryWorkspace].
	RunListQueryWorkItem = "work_item"
	// RunListQueryWorkflow selects the runs that start one workflow, such as hello: a
	// lower-case letter, then lower-case letters, digits and hyphens, 1 to 55
	// characters in all.
	RunListQueryWorkflow = "workflow"
	// RunListQueryState selects the runs in one state: pending, running, completed,
	// failed, cancelled or lost. Any other value is refused.
	RunListQueryState = "state"
	// RunListQuerySince selects the runs created at or after a time, RFC 3339 in UTC
	// ending in Z, with seconds 00 to 59, and at most 9 fraction digits.
	RunListQuerySince = "since"
	// RunListQueryBefore is the [RunList] Next of the previous page, a run id. The rows
	// are ordered by created_at, the last first, then by id, the greatest first. before
	// selects the runs whose created_at is earlier than that run's, or the same with a
	// smaller id. The id of no run is refused.
	RunListQueryBefore = "before"
	// RunListQueryLimit is the most rows in the answer, from 1 to [RunListMaxLimit],
	// [RunListDefaultLimit] when absent.
	RunListQueryLimit = "limit"
)

// RunListDefaultLimit is the most rows of a run list whose request has no
// [RunListQueryLimit].
const RunListDefaultLimit = 25

// RunListMaxLimit is the highest [RunListQueryLimit], and the most rows of a run list.
const RunListMaxLimit = 100

// RunListInvalidRequest is the Error of a [RunListRefusal] for a query parameter that
// breaks its rule, with status 400.
const RunListInvalidRequest = "invalid_request"

// RunList is the body of the 200 answer to GET /v1/runs, diagnostics/v1
// runs.schema.json: the runs of one page, in the order of [RunListQueryBefore], and
// Next, the value of [RunListQueryBefore] for the next page. Next is nil, written as
// null, when no run matches the request after the last row. A nil Runs is written as the
// empty list.
type RunList struct {
	Runs []RunListRow `json:"runs"`
	Next *string      `json:"next"`
}

// MarshalJSON writes l, with a nil Runs as the empty list, which the schema accepts, in
// place of null, which fails the schema.
func (l RunList) MarshalJSON() ([]byte, error) {
	type plain RunList
	if l.Runs == nil {
		l.Runs = []RunListRow{}
	}
	return json.Marshal(plain(l))
}

// RunListRow is one run of a [RunList]. Where run in diagnostics.schema.json has a
// member for the same value, the row uses its name and definition. State is the one
// exception, with a wider definition. Workflow is empty when the run starts no workflow.
// WorkItemNumber and WorkItemTitle are both set when the run is for a work item, and
// both zero otherwise. Reason is empty when the run has no reason. CreatedAt is when Qory
// Apiary created the run. StartedAt is empty when the run has not started. Times are RFC
// 3339 in UTC with whole seconds.
//
// State is the run's state, 1 to 32 lower-case letters and underscores. The states
// written today are pending, running, completed, failed, cancelled and lost. A reader
// accepts a state that is not in this list, and shows it as it is.
//
// DurationSeconds is how long the run ran, in whole seconds, rounded down, from
// StartedAt: to the run's exit for a run that has exited, and to the moment the answer
// was built for a run in the state pending or running that has started and has not
// exited. It is nil when the run has not started, and when the run ended without an
// exit.
type RunListRow struct {
	ID              string              `json:"id"`
	Number          int64               `json:"number"`
	Organisation    RunListOrganisation `json:"organisation"`
	Workspace       RunListWorkspace    `json:"workspace"`
	Workflow        string              `json:"workflow,omitempty"`
	WorkItemNumber  int64               `json:"work_item_number,omitempty"`
	WorkItemTitle   string              `json:"work_item_title,omitempty"`
	State           string              `json:"state"`
	Reason          string              `json:"reason,omitempty"`
	CreatedAt       string              `json:"created_at"`
	StartedAt       string              `json:"started_at,omitempty"`
	DurationSeconds *int64              `json:"duration_seconds,omitempty"`
}

// RunListOrganisation is the organisation of a [RunListRow], by its slug.
type RunListOrganisation struct {
	Slug string `json:"slug"`
}

// RunListWorkspace is the workspace of a [RunListRow]: its public id, ws_, and its slug,
// which is unique in its organisation.
type RunListWorkspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
}

// RunListRefusal is the body of a refusal of GET /v1/runs, diagnostics/v1
// runs-refusal.schema.json. Parameter is non-nil exactly when Error is
// [RunListInvalidRequest]: the name of a query parameter that breaks its rule, at most
// 64 characters, and empty for a parameter with an empty name.
type RunListRefusal struct {
	Error     string  `json:"error"`
	Parameter *string `json:"parameter,omitempty"`
}
