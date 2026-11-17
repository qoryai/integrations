package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

// MaxInputs is the most bytes the run's inputs has, serialized,
// documents.schema.json.
const MaxInputs = 768 << 10

// Documents is the run's documents, run/v1 documents.schema.json: one object, the
// documents member of Forager's run answer, which the machine receives, signed, in the
// 201 of the run code's redemption. Forager passes it on unread. Step, Domain and Inputs
// are set for every run. Access is set exactly when Target is, and TargetVersion only
// when Target is. Workflow is set exactly when HarnessHidden is, and a run with Workflow
// has Harness, TargetVersion, or both. When the run composes a harness, the harness
// comes from one of two places. If TargetVersion is set and the read of the harness file
// of the target returned a file whose harness section extends a stack or defines one,
// Harness is nil. For a target that is a repository, the machine composes that file at
// TargetVersion, as described under [Documents.TargetVersion]. A harness section
// extends a stack when it has extends, a base, and defines one when it has no extends
// and lists at least one module. The run documents contain Harness in three cases:
// TargetVersion is set and the read returned a file whose harness section is absent, or
// has no extends and lists no module; TargetVersion is set and the read returned found:
// false; or TargetVersion is empty, because the run has no target or the control plane
// read no harness file of the target. When Harness is set, the machine composes
// Harness instead of the harness section of the harness file of the target. Of that
// file, the machine applies the qory version that the file requires, the worktree
// section, and the tiers of the harness section, as described under [RunSettings.Tiers].
// If the machine's qory version does not meet that requirement, the run stops. It reads
// none of the other machine keys of that file and none of its git and env sections, so
// the harness file of the target does not set how the run starts. A run of the step
// explore has WorkItem. A run of the step prebuild has Access, Target, ImageDestination
// and exactly one of EnvironmentDestination and EnvironmentImage, and no TargetVersion,
// WorkItem, RunSettings, TimeLimitMinutes, SharedRun, FullSuite, Workflow,
// HarnessHidden or Harness. RunSettings, TimeLimitMinutes, SharedRun and FullSuite are
// set only for the step explore. The documents are the documents of session 1: its
// Workflow, RunSettings, TimeLimitMinutes, FullSuite and Inputs. With SharedRun, each
// later session comes from the answer to the next-session request, as
// [NextSessionAnswer] describes. Inputs is the step's input as the control plane wrote
// it, whose shape belongs to the step's schema.
type Documents struct {
	Step   string          `json:"step"`
	Domain string          `json:"domain"`
	Inputs json.RawMessage `json:"inputs"`
	// Access is set exactly when Target is: what the run may do on its target,
	// "read", "write-no-push" or "write". With "read", the run gets a read credential
	// and works on a copy of the target that the agent cannot change. With
	// "write-no-push", the run gets a read credential and a copy that the agent may
	// change, and the run sends nothing back to the target. With "write", the run gets
	// a write credential, the run sends the agent's changes to the target, and only a
	// run with "write" may have Submit. On a target that is a repository, the copy is a
	// checkout and a change is a commit. The machine's credential integration narrows
	// the run's access to it.
	Access string  `json:"access,omitempty"`
	Target *Target `json:"target,omitempty"`
	// TargetVersion is set only with Target, and never for the step prebuild: the
	// version of the target at which the control plane checked for a harness file, the
	// resolved_at of the read at the path of the harness file, whether the read returned
	// the file or found: false.
	//
	// The rules in this paragraph apply to a target that is a repository: a target on
	// the system of the repositories that the run configuration lists. The machine reads
	// the harness file at this version, whether or not the run works on a branch of its
	// work item. At this version, the machine composes the harness section of that file
	// when the section has extends or lists at least one module. The machine applies the
	// qory version that the file requires, the worktree section and the tiers at this
	// version too. After the compose, the machine works at this version, or checks out
	// the branch of the run's work item and works on that branch. The run documents
	// contain no branch: the machine reads the branch from the control plane. An agent
	// may change, on that branch, the harness file of the target or a file that the
	// compose reads. A run composes that change only when the TargetVersion of that run
	// contains the change. Because TargetVersion is exact, the machine reads the same
	// file that the control plane read.
	//
	// The value is the exact version that the read returned, never a name that can point
	// to another version later. The grammar is generic, because a target in another
	// system has another kind of version: 1 to 255 letters, digits and the characters
	// ._/-.
	TargetVersion string       `json:"target_version,omitempty"`
	WorkItem      *WorkItem    `json:"work_item,omitempty"`
	RunSettings   *RunSettings `json:"run_settings,omitempty"`
	// TimeLimitMinutes is the time limit of session 1, in minutes, from 15 to 1440,
	// and 0 when the control plane sets none. The machine sets it as that session's
	// time limit. The limit of each later session is the TimeLimitMinutes of the
	// [NextSessionAnswer]. It is set only for the step explore.
	TimeLimitMinutes int `json:"time_limit_minutes,omitempty"`
	// SharedRun is true when qory sends the next-session request after each session
	// and runs the session that the answer sets. False and absent in the JSON are the
	// same: qory sends no request, and the run has session 1 only. It is set only for
	// the step explore.
	SharedRun bool `json:"shared_run,omitempty"`
	// FullSuite is true when session 1 runs the full test suite after the agent ends.
	// False and absent in the JSON are the same. The flag of each later session is the
	// FullSuite of the [NextSessionAnswer]. It is set only for the step explore.
	FullSuite bool `json:"full_suite,omitempty"`
	// Workflow is set exactly when HarnessHidden is: the workflow of the harness that
	// the run starts, such as hello. qory starts the first of these that the harness
	// has: the skill workflow-hello, the command workflow-hello, the skill hello, the
	// command hello. At most 55 characters, so that workflow-<name> stays within 64.
	Workflow string `json:"workflow,omitempty"`
	// HarnessHidden is set exactly when Workflow is: when true, the text that comes
	// from the harness is hidden in the run, and when false, that text is shown. A
	// pointer, so that false is written.
	HarnessHidden *bool `json:"harness_hidden,omitempty"`
	// Harness is set only when the run composes a harness, in one of three cases:
	// TargetVersion is set and the read returned a harness file of the target whose
	// harness section is absent, or has no extends and lists no module; TargetVersion is
	// set and the read returned found: false; or TargetVersion is empty, because the run
	// has no target or the control plane read no harness file of the target. It is the
	// harness document of the workspace, in the form of qory.yaml, which the control
	// plane takes from the settings of the workspace. If TargetVersion is set and the
	// read returned a file whose harness section has extends or lists at least one
	// module, Harness is nil. For a target that is a repository, the machine composes
	// that file at TargetVersion, as described under [Documents.TargetVersion]. When
	// Harness is set, the machine composes Harness instead of the harness section of the
	// harness file of the target. Of that file, the machine applies the qory version that
	// the file requires, the worktree section, and the tiers of the harness section, as
	// described under [RunSettings.Tiers]. If the machine's qory version does not meet
	// that requirement, the run stops. It reads none of the other machine keys of that
	// file and none of its git and env sections, so the harness file of the target does
	// not set how the run starts.
	Harness *HarnessDocument `json:"harness,omitempty"`
	// ImageDestination is set exactly for the step prebuild: the place at the registry
	// where the build pushes the target image, the image that the runs of the target
	// start from, such as registry.example.com/targets/0123456789abcdef. The target
	// image contains the environment, the agent, the target's files and the results of
	// the setup commands. The place is a host, an optional port and a path, with no tag
	// and no digest; the host contains a dot, has a port, or is localhost, and a
	// Docker Hub host, docker.io, index.docker.io, registry-1.docker.io,
	// registry.hub.docker.com or registry.docker.com, in any case and with or without a
	// port, is refused. The run reports the image by its digest.
	ImageDestination string `json:"image_destination,omitempty"`
	// EnvironmentDestination is set only for the step prebuild: the place at the
	// registry where the build pushes the environment image, in the form of
	// ImageDestination. The environment image is built from the environment definition
	// alone, and is one image for all of the organisation's targets with the same
	// environment definition. The build pushes nothing there when the environment
	// definition alone does not determine the environment image.
	EnvironmentDestination string `json:"environment_destination,omitempty"`
	// EnvironmentImage is set only for the step prebuild: the organisation's existing
	// environment image for this environment definition, by digest: a place at the
	// registry in the form of ImageDestination, then @sha256: and 64 lower-case
	// hexadecimal digits, at most 327 characters. A name with no host is refused, and a
	// Docker Hub host, docker.io, index.docker.io, registry-1.docker.io,
	// registry.hub.docker.com or registry.docker.com, in any case and with or without a
	// port, is refused. The build starts the target image from it and builds no
	// environment image. A run of the step prebuild has exactly one of
	// EnvironmentDestination and EnvironmentImage.
	EnvironmentImage string `json:"environment_image,omitempty"`
	// LargestPrebuiltImageBytes is set only for the step prebuild: the largest size, in
	// decimal bytes, that the target image of the build may have, at least 1 and within
	// int64. When it is nil, the image has no size limit.
	LargestPrebuiltImageBytes *int64 `json:"largest_prebuilt_image_bytes,omitempty"`
	// SubmitModule is set only with Submit: the harness module that contains the submit
	// program of the forge of the target, which the control plane takes from
	// [Adapter.SubmitModule] of the forge's adapter when the start-up of the adapter has
	// one. The machine appends it after the modules of the harness it composes, whether
	// Harness is set or nil, and the control plane never also lists it in Harness. For
	// what the machine does when a module that it composes has the same name, see
	// submit_module in documents.schema.json.
	SubmitModule *SubmitModule `json:"submit_module,omitempty"`
	// Submit is set only with Workflow and with Access "write": the settings of the
	// submit step of the run, which the control plane writes whenever the process version
	// of the workspace has one. With Submit and a nil SubmitModule, qory runs the submit
	// step and records the draft of each repository as {"none": "no-program"}.
	// On a run with a run code, qory takes the settings of the submit step only from
	// Submit, never from process.yaml. For what qory does when process.yaml has a
	// different submit, see submit in documents.schema.json.
	Submit *Submit `json:"submit,omitempty"`
}

// HarnessDocument is the run's harness, documents.schema.json: the harness document
// of the workspace in the form of qory.yaml,
// https://qory.dev/contracts/harness/v1/config.schema.json, with exactly one member,
// the harness section, which extends a stack or defines one. Every source is a remote
// source. A source of the form {path},
// a module given by name alone, link, the machine keys and every other member of
// qory.yaml are refused.
type HarnessDocument struct {
	Harness HarnessSection `json:"harness"`
}

// HarnessSection is the harness section of a [HarnessDocument]. A section extends a
// stack or defines one. A section with Extends appends Modules to that stack, the base,
// and takes the workflows of the base, so it has no Workflows. A section without Extends
// defines a stack: it requires Modules, at least 1, and may have Workflows, in which the
// stack declares the access level of each of its workflows. A section that defines a
// stack, without Extends, takes no target: the machine composes it for
// RunSettings.Runtime, or, when the run documents have no RunSettings, for the default
// runtime of the machine. Either kind may have Bind, Extensions, Name and Description.
type HarnessSection struct {
	// Name is the name of the harness.
	Name string `json:"name,omitempty"`
	// Description is a description of the harness. An empty description is not
	// written.
	Description string `json:"description,omitempty"`
	// Extends is the stack the harness extends, the base, and nil when the section
	// defines a stack.
	Extends *HarnessBase `json:"extends,omitempty"`
	// Modules are the modules, in order: appended to the base when Extends is set, and
	// the modules of the stack the section defines when it is nil, where at least 1 is
	// required.
	Modules []HarnessModule `json:"modules,omitempty"`
	// Bind maps a kind, agents, commands or skills, to the roles of that kind, each
	// role to the name of the entry of that kind that fills it, such as agents:
	// {developer: web-developer}, or, for a group role of the kind agents, to the list
	// of the agents that fill it, such as agents: {reviewers: [copy-reviewer,
	// legal-reviewer]}. See [BindValue].
	Bind map[string]map[string]BindValue `json:"bind,omitempty"`
	// Extensions are values that qory copies into the report and does not read, by
	// key, each a JSON value of any shape. Empty extensions are not written.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	// Workflows are the settings of each workflow of the stack the section defines, by
	// the workflow's name, as workflows in qory-stack.yaml has them: a lower-case
	// letter, then up to 54 lower-case letters, digits and hyphens. Only without
	// Extends: a section with Extends takes the workflows of its base.
	Workflows map[string]HarnessWorkflow `json:"workflows,omitempty"`
}

// UnmarshalJSON reads a harness section, and returns an error for a kind of Bind that
// is not agents, commands or skills, for a kind whose value is null, for a role whose
// value is null, for a list on a role of the kind commands or skills, for an empty
// list, and for a null item of a list: each fails documents.schema.json.
func (s *HarnessSection) UnmarshalJSON(b []byte) error {
	// A section that does not read as raw roles fails the decode below, which returns
	// that error.
	var raw struct {
		Bind map[string]map[string]json.RawMessage `json:"bind"`
	}
	if json.Unmarshal(b, &raw) == nil {
		if err := checkBind(raw.Bind); err != nil {
			return err
		}
	}
	type harnessSection HarnessSection
	var v harnessSection
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*s = HarnessSection(v)
	return nil
}

// checkBind returns the error of [HarnessSection.UnmarshalJSON] for the first kind, in
// order, and then the first role of it, in order, that fails documents.schema.json for
// one of the reasons that method lists, and nil for none. It returns nil at the first
// role whose value is neither a string, a list nor null, and at the first list item
// that is not a string: the decode then returns the error for that value.
func checkBind(bind map[string]map[string]json.RawMessage) error {
	for _, k := range slices.Sorted(maps.Keys(bind)) {
		switch {
		case k != "agents" && k != "commands" && k != "skills":
			return fmt.Errorf("use agents, commands or skills as a bind kind, not %q", k)
		case bind[k] == nil:
			return fmt.Errorf("set the bind kind %q to an object of roles to names, not null", k)
		}
		for _, role := range slices.Sorted(maps.Keys(bind[k])) {
			at := k + "." + role
			x := bytes.TrimSpace(bind[k][role])
			if string(x) == "null" {
				return fmt.Errorf("set the bind role %q to one name or a list of names, not null", at)
			}
			if len(x) > 0 && x[0] == '"' {
				continue
			}
			// The decode returns the error for a value that is neither a string, a list
			// nor null, so that the error is for the first bad value in key order.
			var items []json.RawMessage
			if len(x) == 0 || x[0] != '[' || json.Unmarshal(x, &items) != nil {
				return nil
			}
			switch {
			case k != "agents":
				return fmt.Errorf("set the bind role %q to one name, not a list: only an agents role can be a group", at)
			case len(items) == 0:
				return fmt.Errorf("write one name or more in the list of the bind role %q: the list is empty", at)
			}
			for i, item := range items {
				item = bytes.TrimSpace(item)
				if string(item) == "null" {
					return fmt.Errorf("set item %d of the list of the bind role %q to a name, not null", i+1, at)
				}
				if len(item) == 0 || item[0] != '"' {
					return nil
				}
			}
		}
	}
	return nil
}

// BindValue is the value of one role in the Bind of a [HarnessSection]: in JSON, the
// name of the one entry that fills the role, or, for a group role of the kind agents, a
// list of the names of the agents that fill it. Name is set for the first form, and
// Names, not nil, for the second, in the order written. A list of one name is a list:
// qory reads it as a group role with one member, and qory's compose fails for it on a
// role that is not a group in the module manifests.
type BindValue struct {
	Name  string
	Names []string
}

// MarshalJSON writes the list Names when it is not nil, and the name Name otherwise.
func (v BindValue) MarshalJSON() ([]byte, error) {
	if v.Names != nil {
		return json.Marshal(v.Names)
	}
	return json.Marshal(v.Name)
}

// UnmarshalJSON reads a name, or a list of names. An empty list reads as Names set to
// an empty list, not nil. It returns an error for null and for a null item of a list:
// both fail documents.schema.json.
func (v *BindValue) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "null" {
		return errors.New("set a bind role to one name or a list of names, not null")
	}
	if len(b) > 0 && b[0] == '[' {
		var items []*string
		if err := json.Unmarshal(b, &items); err != nil {
			return fmt.Errorf("set a bind role to one name or a list of names: %w", err)
		}
		names := make([]string, 0, len(items))
		for i, item := range items {
			if item == nil {
				return fmt.Errorf("set item %d of the list of a bind role to a name, not null", i+1)
			}
			names = append(names, *item)
		}
		*v = BindValue{Names: names}
		return nil
	}
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return fmt.Errorf("set a bind role to one name or a list of names: %w", err)
	}
	*v = BindValue{Name: name}
	return nil
}

// HarnessWorkflow is the settings of one workflow of a stack that a [HarnessSection]
// defines.
type HarnessWorkflow struct {
	// Access is the access level the workflow declares for what a run of it works on,
	// "read", "write-no-push" or "write", as described under access in the run documents
	// (see [Documents.Access]), and empty for read.
	Access string `json:"access,omitempty"`
	// TimeLimit is how long a run of the workflow may run, counted from the run's open,
	// as a Go duration above zero, such as 7h or 45m, and empty when the workflow sets
	// none. The run's time_limit_minutes, when the control plane sets it, applies
	// instead.
	TimeLimit string `json:"time_limit,omitempty"`
}

// HarnessBase is a remote source of a stack, the extends of a [HarnessSection]: Git and
// Ref, with at most one of Path, the folder of the stack inside the remote source, and
// Stack, the name of a stack the remote source exports. With neither, the source is
// the root of the remote source.
type HarnessBase struct {
	// Git is the address of the remote source.
	Git string `json:"git"`
	// Ref is the version of the remote source.
	Ref   string `json:"ref"`
	Path  string `json:"path,omitempty"`
	Stack string `json:"stack,omitempty"`
}

// HarnessModule is a module appended to the stack of a [HarnessSection]: a remote
// source, with an optional name, exclude or only, and variant.
type HarnessModule struct {
	// Name is the name of the module, which the manifest of the module must declare.
	Name   string        `json:"name,omitempty"`
	Source HarnessSource `json:"source"`
	// Exclude is what of the module the compose leaves out. Not with Only.
	Exclude *HarnessSelection `json:"exclude,omitempty"`
	// Only is the only things of the module the compose takes. Not with Exclude.
	Only    *HarnessSelection `json:"only,omitempty"`
	Variant string            `json:"variant,omitempty"`
}

// HarnessSource is a remote source of a module, the source of a [HarnessModule]: Git
// and Ref, with at most one of Path, the folder of the module inside the remote
// source, and Module, the name of a module the remote source exports. With neither,
// the source is the root of the remote source.
type HarnessSource struct {
	// Git is the address of the remote source.
	Git string `json:"git"`
	// Ref is the version of the remote source.
	Ref    string `json:"ref"`
	Path   string `json:"path,omitempty"`
	Module string `json:"module,omitempty"`
}

// SubmitModule is the harness module that contains the submit program of a forge, the
// module whose manifest declares submit_program: the submit_module of the run
// documents, documents.schema.json, and of the adapter's start-up, adapter.schema.json.
// It has the shape of a [HarnessModule] with no Exclude, Only or Variant, and its
// Source.Ref is a full commit hash, 40 or 64 lower-case hexadecimal digits: the schemas
// refuse a branch name, a tag and a short hash.
type SubmitModule struct {
	// Name is the name of the module, which the manifest of the module must declare.
	Name   string        `json:"name,omitempty"`
	Source HarnessSource `json:"source"`
}

// Submit is the settings of the submit step of a run, the submit of the run documents,
// documents.schema.json: Type and Send, both required.
type Submit struct {
	// Type is the artifact type of the submission: the name of a structured entry of
	// artifact_types of the process. The name is lower-case letters and digits in groups
	// joined by single hyphens, 1 to 64 bytes. The schema cannot check that
	// artifact_types has that entry.
	Type string `json:"type"`
	// Send is when the submit step sends the submission. With "end_of_run", the submit
	// step sends the submission at the end of the run. With "after_gate", the submit step
	// stores the submission and sends it after the gate of the stage passes.
	Send string `json:"send"`
}

// HarnessSelection is parts of a module, by kind, the exclude or only of a
// [HarnessModule]: the names of some entries of each kind listed, Instructions true for
// the instructions, and Settings and Env for the settings and the variables.
type HarnessSelection struct {
	Skills       []string     `json:"skills,omitempty"`
	Agents       []string     `json:"agents,omitempty"`
	Commands     []string     `json:"commands,omitempty"`
	OutputStyles []string     `json:"output-styles,omitempty"`
	Hooks        []string     `json:"hooks,omitempty"`
	MCP          []string     `json:"mcp,omitempty"`
	Files        []string     `json:"files,omitempty"`
	Instructions bool         `json:"instructions,omitempty"`
	Settings     *HarnessPart `json:"settings,omitempty"`
	Env          *HarnessPart `json:"env,omitempty"`
}

// HarnessPart is a merged part of a [HarnessSelection]: in JSON, true for all of it,
// or the names of some of it. All is true for the form true, and Names is set for the
// other form.
type HarnessPart struct {
	All   bool
	Names []string
}

// MarshalJSON writes p as true when All is set, and as its names otherwise.
func (p HarnessPart) MarshalJSON() ([]byte, error) {
	if p.All {
		return []byte("true"), nil
	}
	return json.Marshal(p.Names)
}

// UnmarshalJSON reads true, or an array of names.
func (p *HarnessPart) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if string(b) == "true" {
		*p = HarnessPart{All: true}
		return nil
	}
	var names []string
	if err := json.Unmarshal(b, &names); err != nil {
		return err
	}
	*p = HarnessPart{Names: names}
	return nil
}

// WorkItem is the work item a run is for, the run's work_item,
// documents.schema.json.
type WorkItem struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
}

// RunSettings is the run's run_settings, documents.schema.json: the runtime and
// model settings the job composes the agent's harness with, the settings of session 1.
// Runtime is the runtime of every session of the run. It contains no key or secret.
// With no provider chosen, Model, Effort, ProviderModels and Tiers are empty: the run
// uses the gateway's configured key and the runtime's default model. Effort
// and Tiers are set only with Model, and ProviderModels is set exactly when Model is.
// The control plane guarantees four rules that the schema cannot check: the ids in
// ProviderModels are unique; Effort is listed for Model in ProviderModels; each tier's
// model is in ProviderModels; and a tier's effort is listed for that tier's model in
// ProviderModels. The job writes the values as given, and maps no model or effort
// value to another.
type RunSettings struct {
	// Runtime is the runtime's name in Forager's runtime descriptor, such as claude.
	Runtime string `json:"runtime"`
	// Model is the exact id of the model the run uses.
	Model string `json:"model,omitempty"`
	// Effort is the effort value the run uses with Model.
	Effort string `json:"effort,omitempty"`
	// ProviderModels are the models of the run's provider, 1 to 256. When it is empty,
	// the agents run on the provider of the gateway's key, and for a run with a target,
	// the job applies the tier map set at the run's target as written, as on a local
	// run. A run with no target has no such tier map: with ProviderModels empty, no tier
	// map applies, and the run uses the runtime's default model. When it is set and the
	// run has a target, an entry of the tier map set at the run's target applies only
	// when its model id is listed in ProviderModels, and its effort only when that
	// effort is listed for that model.
	ProviderModels []ProviderModel `json:"provider_models,omitempty"`
	// Tiers is the tier map the control plane resolved for the run.
	Tiers *Tiers `json:"tiers,omitempty"`
}

// ModelChoice is a model_choice of the next-session answer, run/v1
// documents.schema.json: the model, effort, provider models and tiers of one session
// after session 1. Its
// members are those of [RunSettings] with the same meaning and no Runtime, because
// every session of the run uses the runtime of RunSettings. With no provider chosen,
// the four members are empty, and the four rules that [RunSettings] lists for the
// control plane apply to the members that are set.
type ModelChoice struct {
	// Model is the exact id of the model the session uses.
	Model string `json:"model,omitempty"`
	// Effort is the effort value the session uses with Model.
	Effort string `json:"effort,omitempty"`
	// ProviderModels are the models of the session's provider, as [RunSettings] has
	// them. It is set exactly when Model is.
	ProviderModels []ProviderModel `json:"provider_models,omitempty"`
	// Tiers is the tier map the control plane resolved for the session. It is set only
	// with Model.
	Tiers *Tiers `json:"tiers,omitempty"`
}

// NextSessionRequest is the body of the next-session request, run/v1
// next-session-request.schema.json: the request that qory sends to the control plane
// after a session of the run ended, to ask for the next session. It has no verdict: the
// control plane reads the newest verdict that the run stored after the previous step.
// What qory posts for a red full suite after the session comes before the request, so
// the newest verdict is read after that post. qory sends it after each session that
// did not pause, for a run whose documents have SharedRun. State and Reason follow the
// rules of the session's dev.qory.session.exited that do not depend on its exit_code:
// a cancelled session has a reason; the reasons timeout and interrupted are for a
// cancelled session only; and the reason gateway_lost is for a failed session only.
// qory sends the request through Forager's link, and the gateway forwards it to the
// control plane. A container cannot send it.
type NextSessionRequest struct {
	// Session is Forager's number of the session that ended, from 1, as in its
	// dev.qory.session.exited.
	Session int `json:"session"`
	// State is how the session ended: "succeeded", "failed" or "cancelled". A session
	// that paused has no request.
	State string `json:"state"`
	// Reason is the reason of the session's dev.qory.session.exited, and nil, written
	// as null, when that event has none.
	Reason *string `json:"reason"`
}

// NextSessionAnswer is the answer to the next-session request, run/v1
// next-session-answer.schema.json: the next session of the run, with Next
// [NextSessionKindSession], or the end of the run, with Next [NextSessionKindEnd] and
// no other member. The session shape has Session, Workflow, TimeLimitMinutes,
// FullSuite and Inputs, and optionally ModelChoice and ModelCredential. MarshalJSON
// writes the members of the shape that Next selects, and no member of the other shape.
// Reading takes the members of either shape and leaves the others at their zero values.
type NextSessionAnswer struct {
	// Next is the kind of the answer: "session" or "end".
	Next string `json:"next"`
	// Session is the control plane's sequence number of the session in the run, from 1.
	// It is not Forager's number, which Forager assigns at the join.
	Session int `json:"session,omitempty"`
	// Workflow is the workflow that the session starts, as [Documents.Workflow].
	Workflow string `json:"workflow,omitempty"`
	// ModelChoice is the model choice of the session, nil when no provider is chosen:
	// the session uses the gateway's configured key and the runtime's default model.
	ModelChoice *ModelChoice `json:"model_choice,omitempty"`
	// ModelCredential is the credential of the run that the session joins with, such as
	// <the provider's public id>-<its place from 1>, and empty when no provider is
	// chosen. The schema checks only the pattern of a credential's name in Forager's
	// policy.
	ModelCredential string `json:"model_credential,omitempty"`
	// TimeLimitMinutes is the time limit of the session, in minutes, from 15 to 1440.
	TimeLimitMinutes int `json:"time_limit_minutes,omitempty"`
	// FullSuite is true when the session runs the full test suite after the agent ends.
	FullSuite bool `json:"full_suite,omitempty"`
	// Inputs is the input of the session, as [Documents.Inputs].
	Inputs json.RawMessage `json:"inputs,omitempty"`
}

// The values of [NextSessionAnswer.Next].
const (
	NextSessionKindSession = "session"
	NextSessionKindEnd     = "end"
)

// MarshalJSON writes a. For Next "end" it writes {"next":"end"} and nothing else. For
// Next "session" it writes the members of the session shape, and writes full_suite
// when it is false. It returns an error for any other Next, the empty Next included,
// and for a session answer whose Inputs is empty, which would be written as null.
func (a NextSessionAnswer) MarshalJSON() ([]byte, error) {
	switch a.Next {
	case NextSessionKindEnd:
		return json.Marshal(struct {
			Next string `json:"next"`
		}{a.Next})
	case NextSessionKindSession:
		if len(a.Inputs) == 0 {
			return nil, errors.New("set Inputs of a session answer to the input of the session, an object with a version")
		}
	default:
		return nil, fmt.Errorf("set Next of an answer to %q or %q, not %q", NextSessionKindSession, NextSessionKindEnd, a.Next)
	}
	return json.Marshal(struct {
		Next             string          `json:"next"`
		Session          int             `json:"session"`
		Workflow         string          `json:"workflow"`
		ModelChoice      *ModelChoice    `json:"model_choice,omitempty"`
		ModelCredential  string          `json:"model_credential,omitempty"`
		TimeLimitMinutes int             `json:"time_limit_minutes"`
		FullSuite        bool            `json:"full_suite"`
		Inputs           json.RawMessage `json:"inputs"`
	}{a.Next, a.Session, a.Workflow, a.ModelChoice, a.ModelCredential,
		a.TimeLimitMinutes, a.FullSuite, a.Inputs})
}

// RunOutcome is a run's end, run/v1 outcome.schema.json, titled Run end: how the run
// ended. State, Reason and ExitCode are the members of Forager's dev.qory.run.exited.
// It is not the body of Forager's run-outcome.schema.json.
type RunOutcome struct {
	// State is how the run ended: "succeeded", "failed" or "cancelled".
	State string `json:"state"`
	// Reason is the reason of the run's dev.qory.run.exited, empty for none. qory sets one
	// of the [EndCode] values, with State "failed", to say why a run failed.
	Reason string `json:"reason,omitempty"`
	// ExitCode is the run's exit code, from -1, and nil for none. A nil ExitCode is left
	// out, and a pointer to 0 is written as 0.
	ExitCode *int `json:"exit_code,omitempty"`
}

// EndCode is a code that qory sets in [RunOutcome.Reason] to say why a run failed: a code
// in the list of $defs end_codes of run/v1 outcome.schema.json, which shows each code's
// cause in plain words. The schema refuses one of them with a State other than failed.
type EndCode string

// The end codes, with the cause of each: see outcome.schema.json for the words. None is
// one of Forager's reserved reasons.
const (
	// EndCodePushCredentialMissing: the run's configuration has no credential with the
	// purpose push, so qory pushed nothing.
	EndCodePushCredentialMissing EndCode = "push_credential_missing"
	// EndCodePushRefused: a push of the run's commits was refused with status 403.
	EndCodePushRefused EndCode = "push_refused"
	// EndCodePrebuiltImageTooLarge: the image of the target is larger than
	// [Documents.LargestPrebuiltImageBytes].
	EndCodePrebuiltImageTooLarge EndCode = "prebuilt_image_too_large"
	// EndCodePushNotRecorded: qory pushed the run's commits, and the control plane has
	// no record of the push.
	EndCodePushNotRecorded EndCode = "push_not_recorded"
)

// ProviderModel is one model of the run's provider, an entry of
// run_settings.provider_models. Effort lists the effort values the model accepts, 1 to
// 16, each listed once. An entry with no Effort means the model takes no effort value.
type ProviderModel struct {
	ID     string   `json:"id"`
	Effort []string `json:"effort,omitempty"`
}

// Tiers is run_settings.tiers: a model, and optionally an effort value, for each tier
// the control plane maps. A tier the control plane does not map is nil. For a run with
// a target, a tier the control plane maps applies instead of the entry for that tier in
// the tier map set at the run's target, and the tier map set at the run's target fills
// only the tiers left nil. A run with no target has no such tier map: only the tiers the
// control plane maps apply.
type Tiers struct {
	Light    *Tier `json:"light,omitempty"`
	Fast     *Tier `json:"fast,omitempty"`
	Thinking *Tier `json:"thinking,omitempty"`
}

// Tier is the model of one tier: in JSON, a model id, or an object of model and an
// optional effort. A Tier read from JSON is written back in the form it was read in. A
// Tier built in Go is written as a model id when Effort is empty, and as an object
// otherwise.
type Tier struct {
	Model  string
	Effort string
	// object is true when the Tier was read from the object form.
	object bool
}

// tierObject is the object form of a [Tier].
type tierObject struct {
	Model  string `json:"model"`
	Effort string `json:"effort,omitempty"`
}

// MarshalJSON writes t as a model id, or as an object of model and effort.
func (t Tier) MarshalJSON() ([]byte, error) {
	if t.Effort == "" && !t.object {
		return json.Marshal(t.Model)
	}
	return json.Marshal(tierObject{Model: t.Model, Effort: t.Effort})
}

// UnmarshalJSON reads a model id, or an object of model and an optional effort.
func (t *Tier) UnmarshalJSON(b []byte) error {
	b = bytes.TrimLeft(b, " \t\r\n")
	if string(b) == "null" {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var model string
		if err := json.Unmarshal(b, &model); err != nil {
			return err
		}
		*t = Tier{Model: model}
		return nil
	}
	var o tierObject
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	*t = Tier{Model: o.Model, Effort: o.Effort, object: true}
	return nil
}
