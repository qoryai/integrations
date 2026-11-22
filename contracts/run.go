package contracts

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
)

// MaxInputs is the most bytes the run's inputs has, serialized,
// documents.schema.json.
const MaxInputs = 768 << 10

// Documents is the run's documents, run/v1 documents.schema.json: one object, the
// documents member of Forager's run answer, which the machine receives, signed, in the
// 201 of the run code's redemption. Forager passes it on unread. Step, Domain and Inputs
// are set for every run. Access is set exactly when Target is, and TargetVersion only
// when Target is. Workflow is set exactly when HarnessHidden is, and for every run of the
// step workflow. A run with Workflow has Harness, TargetVersion, or both. When the run
// composes a harness, the harness comes from one of two places. If TargetVersion is set
// and the read of the harness file of the target returned a file whose harness section
// extends a stack or defines one, Harness is nil. For a target that is a repository, the
// machine composes that file at TargetVersion, as described under
// [Documents.TargetVersion]. A harness section extends a stack when it has extends, a
// base, and defines one when it has no extends and lists at least one module. The run
// documents contain Harness in three cases: TargetVersion is set and the read returned a
// file whose harness section is absent, or has no extends and lists no module;
// TargetVersion is set and the read returned found: false; or TargetVersion is empty,
// because the run has no target or the control plane read no harness file of the target.
// When Harness is set, the machine composes Harness instead of the harness section of the
// harness file of the target. Of that file, the machine applies the qory version that the
// file requires, the worktree section, and the tiers of the harness section, as described
// under [RunSettings.Tiers]. If the machine's qory version does not meet that
// requirement, the run stops. It reads none of the other machine harness settings of that
// file and none of its git and env sections, so the harness file of the target does not
// set how the run starts. A run of the step workflow has Subject, RunSettings and
// Workflow, and runs the workflow named in Workflow. A run of the step prebuild has
// Access, Target, ImageDestination and exactly one of EnvironmentDestination and
// EnvironmentImage, and no TargetVersion, Subject, RunSettings, TimeLimitMinutes,
// SharedRun, TypeSettings, Workflow, HarnessHidden or Harness. TimeLimitMinutes,
// SharedRun and TypeSettings are set only for the step workflow. The documents are the
// documents of session 1: its Workflow, RunSettings, TimeLimitMinutes, TypeSettings and
// Inputs. With SharedRun, each later session comes from the answer to the next-session
// request, as [NextSessionAnswer] describes. Inputs is the step's input as the control
// plane wrote it, whose shape belongs to the step's schema. A reader ignores a member of
// the documents that Documents does not have, unless MustUnderstand lists it.
type Documents struct {
	Step   string          `json:"step,case:strict"`
	Domain string          `json:"domain,case:strict"`
	Inputs json.RawMessage `json:"inputs,case:strict"`
	// Access is set exactly when Target is: what the run may do on its target,
	// "read", "write-no-push" or "write". With "read", the run gets a read credential
	// and works on a copy of the target that the agent cannot change. With
	// "write-no-push", the run gets a read credential and a copy that the agent may
	// change, and the run sends nothing back to the target. With "write", the run gets
	// a write credential, the run sends the agent's changes to the target, and only a
	// run with "write" may have Deliver. On a target that is a repository, the copy is a
	// checkout and a change is a commit. The machine's credential integration narrows
	// the run's access to it.
	Access string  `json:"access,omitempty,case:strict"`
	Target *Target `json:"target,omitempty,case:strict"`
	// ReadTargets are the places that the run only reads, 0 to [MaxReadTargets] of them.
	// It is never set for the step prebuild, and it is set with or without Target. The
	// run only reads a place in it: it changes, pushes and delivers nothing to the place.
	// Target stays the one place where the run's change would go, and Access applies to
	// Target only. No two entries have the same Collection, and none has the Collection
	// of Target. The schema cannot check that: [Documents.Check] does. An entry has no
	// version: TargetVersion is the version of Target only. A nil list is left out, and
	// an empty list is written as [].
	ReadTargets []Target `json:"read_targets,omitzero,case:strict"`
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
	TargetVersion string   `json:"target_version,omitempty,case:strict"`
	Subject       *Subject `json:"subject,omitempty,case:strict"`
	// Trigger is what started the run, in snake_case, such as take_in. It is optional.
	Trigger     string       `json:"trigger,omitempty,case:strict"`
	RunSettings *RunSettings `json:"run_settings,omitempty,case:strict"`
	// TimeLimitMinutes is the time limit of session 1, in minutes, from 1 to 1440,
	// and 0 when the control plane sets none. The machine sets it as that session's
	// time limit. The limit of each later session is the TimeLimitMinutes of the
	// [NextSessionAnswer]. It is set only for the step workflow.
	TimeLimitMinutes int `json:"time_limit_minutes,omitempty,case:strict"`
	// SharedRun is true when qory sends the next-session request after each workflow
	// session that did not pause and runs the session that the answer sets. False and
	// absent in the JSON are the same: qory sends no request, and the run has session 1
	// only. It is set only for the step workflow.
	SharedRun bool `json:"shared_run,omitempty,case:strict"`
	// TypeSettings is the type_settings object of session 1, the settings that belong to
	// the type of the workspace. It is a JSON object whose members this contract does not
	// list: the process of the workspace type registers its schema, and the schema of
	// the software factory requires full_suite in it. The type_settings of each later
	// session is the TypeSettings of the [NextSessionAnswer]. It is set only for the
	// step workflow. The schema accepts a pointer below it in MustUnderstand, because
	// the members below it belong to the workspace type's schema, not to this contract.
	TypeSettings json.RawMessage `json:"type_settings,omitempty,case:strict"`
	// Workflow is set exactly when HarnessHidden is, and for every run of the step
	// workflow: the workflow of the harness that
	// the run starts, such as hello. qory starts the first of these that the harness
	// has: the skill workflow-hello, the command workflow-hello, the skill hello, the
	// command hello. At most 55 characters, so that workflow-<name> stays within 64.
	Workflow string `json:"workflow,omitempty,case:strict"`
	// HarnessHidden is set exactly when Workflow is: when true, the text that comes
	// from the harness is hidden in the run, and when false, that text is shown. A
	// pointer, so that false is written.
	HarnessHidden *bool `json:"harness_hidden,omitempty,case:strict"`
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
	// that requirement, the run stops. It reads none of the other machine harness
	// settings of that file and none of its git and env sections, so the harness file of
	// the target does not set how the run starts.
	Harness *HarnessDocument `json:"harness,omitempty,case:strict"`
	// ImageDestination is set exactly for the step prebuild: the place at the registry
	// where the build pushes the target image, the image that the runs of the target
	// start from, such as registry.example.com/targets/0123456789abcdef. The target
	// image contains the environment, the agent, the target's files and the results of
	// the setup commands. The place is a host, an optional port and a path, with no tag
	// and no digest; the host contains a dot, has a port, or is localhost, and a
	// Docker Hub host, docker.io, index.docker.io, registry-1.docker.io,
	// registry.hub.docker.com or registry.docker.com, in any case and with or without a
	// port, is refused. The run reports the image by its digest.
	ImageDestination string `json:"image_destination,omitempty,case:strict"`
	// EnvironmentDestination is set only for the step prebuild: the place at the
	// registry where the build pushes the environment image, in the form of
	// ImageDestination. The environment image is built from the environment definition
	// alone, and is one image for all of the organisation's targets with the same
	// environment definition. The build pushes nothing there when the environment
	// definition alone does not determine the environment image.
	EnvironmentDestination string `json:"environment_destination,omitempty,case:strict"`
	// EnvironmentImage is set only for the step prebuild: the organisation's existing
	// environment image for this environment definition, by digest: a place at the
	// registry in the form of ImageDestination, then @sha256: and 64 lower-case
	// hexadecimal digits, at most 327 characters. A name with no host is refused, and a
	// Docker Hub host, docker.io, index.docker.io, registry-1.docker.io,
	// registry.hub.docker.com or registry.docker.com, in any case and with or without a
	// port, is refused. The build starts the target image from it and builds no
	// environment image. A run of the step prebuild has exactly one of
	// EnvironmentDestination and EnvironmentImage.
	EnvironmentImage string `json:"environment_image,omitempty,case:strict"`
	// LargestPrebuiltImageBytes is set only for the step prebuild: the largest size, in
	// decimal bytes, that the target image of the build may have, at least 1 and within
	// int64. When it is nil, the image has no size limit.
	LargestPrebuiltImageBytes *int64 `json:"largest_prebuilt_image_bytes,omitempty,case:strict"`
	// Deliver is set only with Workflow and with Access "write": the artifacts that the
	// run delivers, 1 to [MaxDeliver] entries with different types, which the control
	// plane writes whenever the process version of the workspace has a type to deliver.
	// When it is nil, the run delivers nothing. On a run with a run code, qory takes the
	// settings of each entry only from Deliver, never from process.yaml. For what qory
	// does when process.yaml has different settings, see deliver in
	// documents.schema.json. The schema cannot check that the types differ:
	// [Documents.Check] does.
	Deliver []Deliver `json:"deliver,omitempty,case:strict"`
	// MustUnderstand lists members of the documents that the reader must understand, as
	// JSON Pointers counted from the documents, such as "/run_settings/runtime". A
	// reader ignores a member that this type does not have, unless MustUnderstand lists
	// it. A reader refuses documents whose MustUnderstand lists a pointer to a member that
	// this type does not have: [CheckMustUnderstand] returns an error for them.
	MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
}

// HarnessDocument is the run's harness, documents.schema.json. It is the harness document
// of the workspace in the form of qory.yaml,
// https://qory.dev/contracts/harness/v1/config.schema.json. It has the member harness,
// the harness section, which extends a stack or defines one. Every source is a remote
// source. A source of the form {path} and a module given by name alone are refused. A
// reader ignores link, the machine harness settings and every other member of qory.yaml.
type HarnessDocument struct {
	Harness HarnessSection `json:"harness,case:strict"`
}

// HarnessSection is the harness section of a [HarnessDocument]. A section extends a
// stack or defines one. A section with Extends appends Modules to that stack, the base,
// and takes the workflows of the base, so it has no Workflows. A section without Extends
// defines a stack: it requires Modules, at least 1, and may have Workflows, in which the
// stack declares the settings of each of its workflows. A section that defines a
// stack, without Extends, takes no target: the machine composes it for the runtime of
// the session: for session 1, RunSettings.Runtime; for a later session, the Runtime of
// its [ModelChoice], or RunSettings.Runtime when that is empty. Either kind may
// have Bind, Extensions, Name and Description.
type HarnessSection struct {
	// Name is the name of the harness.
	Name string `json:"name,omitempty,case:strict"`
	// Description is a description of the harness. An empty description is not
	// written.
	Description string `json:"description,omitempty,case:strict"`
	// Extends is the stack the harness extends, the base, and nil when the section
	// defines a stack.
	Extends *HarnessBase `json:"extends,omitempty,case:strict"`
	// Modules are the modules, in order: appended to the base when Extends is set, and
	// the modules of the stack the section defines when it is nil, where at least 1 is
	// required.
	Modules []HarnessModule `json:"modules,omitempty,case:strict"`
	// Bind maps a kind, agents, commands or skills, to the roles of that kind, each
	// role to the name of the entry of that kind that fills it, such as agents:
	// {developer: web-developer}, or, for a group role of the kind agents, to the list
	// of the agents that fill it, such as agents: {reviewers: [copy-reviewer,
	// legal-reviewer]}. See [BindValue].
	Bind map[string]map[string]BindValue `json:"bind,omitempty,case:strict"`
	// Extensions are values that qory copies into the report and does not read, by
	// key, each a JSON value of any shape. Empty extensions are not written.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty,case:strict"`
	// Workflows are the settings of each workflow of the stack the section defines, by
	// the workflow's name, as workflows in qory-stack.yaml has them: a lower-case
	// letter, then up to 54 lower-case letters, digits and hyphens. Only without
	// Extends: a section with Extends takes the workflows of its base.
	Workflows map[string]HarnessWorkflow `json:"workflows,omitempty,case:strict"`
}

// definesPointer reports whether a pointer below a HarnessSection leads to one of its
// members. Below bind, the schema defines the kinds agents, commands and skills alone.
func (HarnessSection) definesPointer(segments []string) bool {
	if segments[0] == "bind" && len(segments) > 1 && !slices.Contains([]string{"agents", "commands", "skills"}, segments[1]) {
		return false
	}
	return resolvesBelow(reflect.TypeFor[HarnessSection](), segments)
}

// UnmarshalJSON reads a harness section. It ignores a member of Bind that is not a kind,
// agents, commands or skills, as documents.schema.json does. An example is a role in the
// flat form agents/writer. It returns an error in these cases, and each fails
// documents.schema.json:
//   - a kind whose value is null
//   - a role whose value is null
//   - a list on a role of the kind commands or skills
//   - an empty list
//   - a null item of a list
func (s *HarnessSection) UnmarshalJSON(b []byte) error {
	b = withoutUndefinedBindKinds(b)
	// A section that does not read as raw roles fails the decode below, which returns
	// that error.
	var raw struct {
		Bind map[string]map[string]json.RawMessage `json:"bind,case:strict"`
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

// withoutUndefinedBindKinds returns the section b without the members of its bind that
// are not a kind of bind. It returns b when b has no bind, or when it does not read as a
// section with an object bind. The decode of b returns the error for that.
func withoutUndefinedBindKinds(b []byte) []byte {
	var section map[string]json.RawMessage
	if json.Unmarshal(b, &section) != nil {
		return b
	}
	var bind map[string]json.RawMessage
	if json.Unmarshal(section["bind"], &bind) != nil || bind == nil {
		return b
	}
	for k := range bind {
		if k != "agents" && k != "commands" && k != "skills" {
			delete(bind, k)
		}
	}
	section["bind"], _ = json.Marshal(bind)
	out, err := json.Marshal(section)
	if err != nil {
		return b
	}
	return out
}

// checkBind returns the error of [HarnessSection.UnmarshalJSON] for the first kind, in
// order, and then the first role of it, in order, that fails documents.schema.json for
// one of the reasons that method lists, and nil for none. It returns nil at the first
// role whose value is neither a string, a list nor null, and at the first list item
// that is not a string: the decode then returns the error for that value.
func checkBind(bind map[string]map[string]json.RawMessage) error {
	for _, k := range slices.Sorted(maps.Keys(bind)) {
		if bind[k] == nil {
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

// definesPointer reports whether a pointer below a BindValue leads to an item of a list.
func (BindValue) definesPointer(segments []string) bool {
	return len(segments) == 1 && isIndex(segments[0])
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
	Access string `json:"access,omitempty,case:strict"`
	// Plan is the artifact type whose launch version sets what a run of the workflow
	// may push, such as "build-plan", and empty for none. Only a workflow whose Access
	// is "write" has a Plan.
	Plan string `json:"plan,omitempty,case:strict"`
	// Tier is the tier of the model and effort of the main session of a run of the
	// workflow, "light", "balanced" or "thinking", and empty for none. The control plane
	// resolves the model and effort from the tier.
	Tier string `json:"tier,omitempty,case:strict"`
	// ReviewGroup is the name of the group role whose members' reports a review session
	// of the workflow needs, such as "reviewers", and empty for none.
	ReviewGroup string `json:"review_group,omitempty,case:strict"`
	// RunsLockedChecks is true for a Verify workflow: before the agent of a session of
	// the workflow starts, qory runs the locked checks that a module declares. It is
	// false when left out, and a false value is left out of the document.
	RunsLockedChecks bool `json:"runs_locked_checks,omitempty,case:strict"`
}

// HarnessBase is a remote source of a stack, the extends of a [HarnessSection]: Git and
// Ref, with at most one of Path, the folder of the stack inside the remote source, and
// Stack, the name of a stack the remote source exports. With neither, the source is
// the root of the remote source.
type HarnessBase struct {
	// Git is the address of the remote source.
	Git string `json:"git,case:strict"`
	// Ref is the version of the remote source.
	Ref   string `json:"ref,case:strict"`
	Path  string `json:"path,omitempty,case:strict"`
	Stack string `json:"stack,omitempty,case:strict"`
}

// HarnessModule is a module appended to the stack of a [HarnessSection]: a remote
// source, with an optional name, exclude or only, and variant.
type HarnessModule struct {
	// Name is the name of the module, which the manifest of the module must declare.
	Name   string        `json:"name,omitempty,case:strict"`
	Source HarnessSource `json:"source,case:strict"`
	// Exclude is what of the module the compose leaves out. Not with Only.
	Exclude *HarnessSelection `json:"exclude,omitempty,case:strict"`
	// Only is the only things of the module the compose takes. Not with Exclude.
	Only    *HarnessSelection `json:"only,omitempty,case:strict"`
	Variant string            `json:"variant,omitempty,case:strict"`
}

// HarnessSource is a remote source of a module, the source of a [HarnessModule]: Git
// and Ref, with at most one of Path, the folder of the module inside the remote
// source, and Module, the name of a module the remote source exports. With neither,
// the source is the root of the remote source.
type HarnessSource struct {
	// Git is the address of the remote source.
	Git string `json:"git,case:strict"`
	// Ref is the version of the remote source.
	Ref    string `json:"ref,case:strict"`
	Path   string `json:"path,omitempty,case:strict"`
	Module string `json:"module,omitempty,case:strict"`
}

// MaxDeliver is the most entries that the deliver of the run documents has,
// documents.schema.json.
const MaxDeliver = 16

// MaxReadTargets is the most entries that the read_targets of the run documents has,
// documents.schema.json.
const MaxReadTargets = 20

// Deliver is one artifact that a run delivers, an entry of the deliver of the run
// documents, documents.schema.json: Type and Send, both required.
type Deliver struct {
	// Type is the artifact type that the run delivers: the name of a structured entry of
	// artifact_types of the process. The name is lower-case letters and digits in groups
	// joined by single hyphens, 1 to 64 bytes. The schema cannot check that
	// artifact_types has that entry.
	Type string `json:"type,case:strict"`
	// Send is when the control plane writes the change_request entry to the change feed. With
	// "end_of_run", it writes the entry at the end of the run. With "after_gate", it stores
	// the artifact and writes the entry after the gate of the stage passes.
	Send string `json:"send,case:strict"`
}

// Check reports the first rule of documents.schema.json that the documents
// break and that the schema does not express: no two entries of Deliver have
// the same Type, no two entries of ReadTargets have the same Collection,
// none has the Collection of Target, and no two entries of the destinations of
// the explore inputs have the same id. Each message starts with the fix. A
// reader of run documents calls it after it decodes them and after it
// validates them against the schema.
func (d Documents) Check() error {
	types := make(map[string]bool, len(d.Deliver))
	for _, e := range d.Deliver {
		if types[e.Type] {
			return fmt.Errorf("List each deliver type once: %q appears twice.", e.Type)
		}
		types[e.Type] = true
	}
	// shortcut: repeats are keyed on the collection, not on an id, because run documents
	// have no id; replace when run documents carry the id of a target.
	collections := make(map[string]bool, len(d.ReadTargets)+1)
	if d.Target != nil {
		collections[d.Target.Collection] = true
	}
	for i, e := range d.ReadTargets {
		if collections[e.Collection] {
			if d.Target != nil && e.Collection == d.Target.Collection {
				return fmt.Errorf("Remove the collection %q from read_targets (index %d): target has it.", e.Collection, i)
			}
			return fmt.Errorf("List each collection of read_targets once: %q appears twice.", e.Collection)
		}
		collections[e.Collection] = true
	}
	var inputs struct {
		Destinations []struct {
			ID string `json:"id,case:strict"`
		} `json:"destinations,case:strict"`
	}
	// An Inputs that is not an object, or has no destinations, has no id to check: the
	// schema refuses it, or the step has no destinations.
	if json.Unmarshal(d.Inputs, &inputs) == nil {
		ids := make(map[string]bool, len(inputs.Destinations))
		for _, e := range inputs.Destinations {
			if ids[e.ID] {
				return fmt.Errorf("List each destination once: %q appears twice.", e.ID)
			}
			ids[e.ID] = true
		}
	}
	return nil
}

// HarnessSelection is parts of a module, by kind, the exclude or only of a
// [HarnessModule]: the names of some entries of each kind listed, Instructions true for
// the instructions, and Settings and Env for the settings and the variables.
type HarnessSelection struct {
	Skills       []string     `json:"skills,omitempty,case:strict"`
	Agents       []string     `json:"agents,omitempty,case:strict"`
	Commands     []string     `json:"commands,omitempty,case:strict"`
	OutputStyles []string     `json:"output-styles,omitempty,case:strict"`
	Hooks        []string     `json:"hooks,omitempty,case:strict"`
	MCP          []string     `json:"mcp,omitempty,case:strict"`
	Files        []string     `json:"files,omitempty,case:strict"`
	Instructions bool         `json:"instructions,omitempty,case:strict"`
	Settings     *HarnessPart `json:"settings,omitempty,case:strict"`
	Env          *HarnessPart `json:"env,omitempty,case:strict"`
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

// definesPointer reports whether a pointer below a HarnessPart leads to an item of a list.
func (HarnessPart) definesPointer(segments []string) bool {
	return len(segments) == 1 && isIndex(segments[0])
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

// Subject is what a run is for, the run's subject, documents.schema.json. Kind is
// snake_case, and work_item is its first value, a work item of the control plane, whose
// ID starts with wi_. URL and Title are empty when the control plane has none.
type Subject struct {
	Kind  string `json:"kind,case:strict"`
	ID    string `json:"id,case:strict"`
	URL   string `json:"url,omitempty,case:strict"`
	Title string `json:"title,omitempty,case:strict"`
}

// RunSettings is the run's run_settings, documents.schema.json: the runtime and
// model settings the job composes the agent's harness with, the settings of session 1.
// Runtime is the runtime of session 1 and of each later session whose [ModelChoice] has
// no Runtime. Each run of the step workflow has RunSettings, and a run of the step
// prebuild has none. RunSettings contains no key or secret.
// With no provider chosen, Model, Effort, ProviderModels and Tiers are empty: the run
// uses the run's default model provider credential, a local model provider credential
// when the run configuration has none, and the runtime's default model. Effort and
// Tiers are set only with Model, and ProviderModels is set exactly when Model is.
// The control plane guarantees four rules that the schema cannot check: the ids in
// ProviderModels are unique; Effort is listed for Model in ProviderModels; each tier's
// model is in ProviderModels; and a tier's effort is listed for that tier's model in
// ProviderModels. The job writes the values as given, and maps no model or effort
// value to another.
type RunSettings struct {
	// Runtime is the runtime's name in Forager's runtime descriptor, such as claude.
	Runtime string `json:"runtime,case:strict"`
	// Model is the exact id of the model the run uses.
	Model string `json:"model,omitempty,case:strict"`
	// Effort is the effort value the run uses with Model.
	Effort string `json:"effort,omitempty,case:strict"`
	// ProviderModels are the models of the run's provider, 1 to 256. When it is empty,
	// the agents run on the provider of the run's default model provider credential, and
	// for a run with a target, the job applies the tier map set at the run's target as
	// written, as on a local run. A run with no target has no such tier map: with
	// ProviderModels empty, no tier map applies, and the run uses the runtime's default
	// model. When it is set and the run has a target, an entry of the tier map set at
	// the run's target applies only when its model id is listed in ProviderModels, and
	// its effort only when that effort is listed for that model.
	ProviderModels []ProviderModel `json:"provider_models,omitempty,case:strict"`
	// Tiers is the tier map the control plane resolved for the run.
	Tiers *Tiers `json:"tiers,omitempty,case:strict"`
}

// ModelChoice is a model_choice of the next-session answer, run/v1
// documents.schema.json: the runtime, model, effort, provider models and tiers of one
// session after session 1. Its members are those of [RunSettings] with the same
// meaning, except Runtime. Runtime is the runtime the session runs on; empty, the
// session runs on the runtime of RunSettings, and a session after session 1 may run on
// a runtime that differs from it. Runtime can be the only member: the session
// runs on that runtime with no provider chosen. With no provider chosen, Model, Effort,
// ProviderModels and Tiers are empty: the session uses the default model of its
// runtime, with the key of the credential whose name is in
// ModelProviderCredentialName or, without one, the key of the run's default model
// provider credential (the active key of its group when it has a group). The four rules that [RunSettings] lists for the control plane
// apply to those four members that are set. For a session whose ModelChoice has Runtime, the control plane sets a
// ModelProviderCredentialName only for a credential of an API whose runtimes, when
// present, list that runtime and whose format that runtime sends. A session with
// Runtime and no ModelProviderCredentialName joins with the run's default model
// provider credential (the active key of its group when it has a group), which that
// runtime must be able to use. The model members suit that runtime.
type ModelChoice struct {
	// Runtime is the runtime's name in Forager's runtime descriptor, such as claude. It
	// is the runtime the session runs on; empty, the runtime of [RunSettings].
	Runtime string `json:"runtime,omitempty,case:strict"`
	// Model is the exact id of the model the session uses.
	Model string `json:"model,omitempty,case:strict"`
	// Effort is the effort value the session uses with Model.
	Effort string `json:"effort,omitempty,case:strict"`
	// ProviderModels are the models of the session's provider, as [RunSettings] has
	// them. It is set exactly when Model is.
	ProviderModels []ProviderModel `json:"provider_models,omitempty,case:strict"`
	// Tiers is the tier map the control plane resolved for the session. It is set only
	// with Model.
	Tiers *Tiers `json:"tiers,omitempty,case:strict"`
}

// NextSessionRequest is the body of the next-session request, run/v1
// next-session-request.schema.json: the request that qory sends to the control plane
// after a session of the run ended, to ask for the next session. It has no verdict: the
// control plane reads the newest verdict that the run stored after the previous step.
// When the session's type_settings turn on a check that runs after the session, and it
// fails, qory stores that check's result before it sends the request. So the control
// plane reads the newest verdict after that result. qory sends it after each workflow
// session that did not pause, for a run whose documents have SharedRun. State and Reason
// follow the rules of the session's dev.qory.session.exited that do not depend on its
// exit_code: a cancelled session has a reason; the reasons timeout and interrupted are
// for a cancelled session only; and the reason gateway_lost is for a failed session
// only. qory sends the request through Forager's link, and the gateway forwards it to
// the control plane. A container cannot send it.
type NextSessionRequest struct {
	// Session is Forager's number of the session that ended, from 1, as in its
	// dev.qory.session.exited. qory can run an agent hook as a session between two
	// workflow sessions. The hook session takes the next Forager session number, and qory
	// sends no next-session request for it, so the `session` numbers in the requests can
	// have gaps.
	Session int `json:"session,case:strict"`
	// State is how the session ended: "succeeded", "failed" or "cancelled". A session
	// that paused has no request.
	State string `json:"state,case:strict"`
	// Reason is the reason of the session's dev.qory.session.exited, and nil, written
	// as null, when that event has none.
	Reason *string `json:"reason,case:strict"`
	// MustUnderstand lists the members of the request that the control plane must
	// understand, as JSON Pointers counted from the request: see [CheckMustUnderstand].
	MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
}

// NextSessionAnswer is the answer to the next-session request, run/v1
// next-session-answer.schema.json. It is the next session of the run, with Next
// [NextSessionKindSession], or the end of the run, with Next [NextSessionKindEnd] and no
// member of the session shape. The session shape has Session, Workflow, TimeLimitMinutes
// and Inputs, and optionally TypeSettings, ModelChoice and ModelProviderCredentialName.
// MarshalJSON writes the members of the shape that Next selects, and no member of the
// other shape. Reading takes the members of either shape and leaves the others at their
// zero values.
type NextSessionAnswer struct {
	// Next is the kind of the answer: "session" or "end".
	Next string `json:"next,case:strict"`
	// Session is the control plane's sequence number of the session in the run, from 1.
	// It is not Forager's number, which Forager assigns at the join. Forager's numbers of
	// the sessions can have gaps; see Session of [NextSessionRequest].
	Session int `json:"session,omitempty,case:strict"`
	// Workflow is the workflow that the session starts, as [Documents.Workflow].
	Workflow string `json:"workflow,omitempty,case:strict"`
	// ModelChoice is the model choice of the session, nil when the session runs on the
	// runtime of [RunSettings] and no provider is chosen: the session uses the default
	// model of that runtime, with the key of the credential whose name is in
	// ModelProviderCredentialName or, without one, the key of the run's default model
	// provider credential (the active key of its group when it has a group).
	ModelChoice *ModelChoice `json:"model_choice,omitempty,case:strict"`
	// ModelProviderCredentialName is the name of the credential of the run that the
	// session joins with, such as <the provider's public id>-<its place from 1>. The
	// schema checks only the grammar of a credential's name in Forager's run
	// configuration. When empty, the session joins with the run's default model provider
	// credential, with the active key of its group when it has a group. The control plane
	// sets it for every later session whose key is a credential of the run configuration
	// other than the run's default model provider credential and the active key of its
	// group.
	ModelProviderCredentialName string `json:"model_provider_credential_name,omitempty,case:strict"`
	// TimeLimitMinutes is the time limit of the session, in minutes, from 1 to 1440.
	TimeLimitMinutes int `json:"time_limit_minutes,omitempty,case:strict"`
	// TypeSettings is the type_settings object of the session, as
	// [Documents.TypeSettings]. It is written only when it is set.
	TypeSettings json.RawMessage `json:"type_settings,omitempty,case:strict"`
	// Inputs is the input of the session, as [Documents.Inputs].
	Inputs json.RawMessage `json:"inputs,omitempty,case:strict"`
	// MustUnderstand lists members of the answer that the reader must understand, as
	// [Documents.MustUnderstand] does for the documents. An answer of either shape may
	// have it.
	MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
}

// The values of [NextSessionAnswer.Next].
const (
	NextSessionKindSession = "session"
	NextSessionKindEnd     = "end"
)

// MarshalJSON writes a. For Next "end" it writes {"next":"end"}, and MustUnderstand when
// it is not empty, and nothing else. For Next "session" it writes the members of the
// session shape, and MustUnderstand when it is not empty, and writes type_settings when
// it is set. It returns an error for any other Next, the empty Next included,
// and for a session answer whose Inputs is empty, which would be written as null.
func (a NextSessionAnswer) MarshalJSON() ([]byte, error) {
	switch a.Next {
	case NextSessionKindEnd:
		return json.Marshal(struct {
			Next           string   `json:"next,case:strict"`
			MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
		}{a.Next, a.MustUnderstand})
	case NextSessionKindSession:
		if len(a.Inputs) == 0 {
			return nil, errors.New("set Inputs of a session answer to the input of the session, an object with a version")
		}
	default:
		return nil, fmt.Errorf("set Next of an answer to %q or %q, not %q", NextSessionKindSession, NextSessionKindEnd, a.Next)
	}
	return json.Marshal(struct {
		Next                        string          `json:"next,case:strict"`
		Session                     int             `json:"session,case:strict"`
		Workflow                    string          `json:"workflow,case:strict"`
		ModelChoice                 *ModelChoice    `json:"model_choice,omitempty,case:strict"`
		ModelProviderCredentialName string          `json:"model_provider_credential_name,omitempty,case:strict"`
		TimeLimitMinutes            int             `json:"time_limit_minutes,case:strict"`
		TypeSettings                json.RawMessage `json:"type_settings,omitempty,case:strict"`
		Inputs                      json.RawMessage `json:"inputs,case:strict"`
		MustUnderstand              []string        `json:"must_understand,omitempty,case:strict"`
	}{a.Next, a.Session, a.Workflow, a.ModelChoice, a.ModelProviderCredentialName,
		a.TimeLimitMinutes, a.TypeSettings, a.Inputs, a.MustUnderstand})
}

// RunOutcome is a run's end, run/v1 outcome.schema.json, titled Run end: how the run
// ended. State, Reason and ExitCode are the members of Forager's dev.qory.run.exited.
// It is not the body of Forager's run-outcome.schema.json.
type RunOutcome struct {
	// State is how the run ended: "succeeded", "failed" or "cancelled".
	State string `json:"state,case:strict"`
	// Reason is the reason of the run's dev.qory.run.exited, empty for none. qory sets one
	// of the [EndCode] values, with State "failed", to say why a run failed.
	Reason string `json:"reason,omitempty,case:strict"`
	// ExitCode is the run's exit code, from -1, and nil for none. A nil ExitCode is left
	// out, and a pointer to 0 is written as 0.
	ExitCode *int `json:"exit_code,omitempty,case:strict"`
	// MustUnderstand lists the members of the outcome that the control plane must
	// understand, as JSON Pointers counted from the outcome: see [CheckMustUnderstand].
	MustUnderstand []string `json:"must_understand,omitempty,case:strict"`
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
	// EndCodeEnvironmentImageMissing: the prebuild's environment image
	// ([Documents.EnvironmentImage]) is not in the registry.
	EndCodeEnvironmentImageMissing EndCode = "environment_image_missing"
	// EndCodePushHeadChanged: the branch head at the forge is not the commit that the run
	// expected (its start commit, or its last push), or the branch is missing.
	EndCodePushHeadChanged EndCode = "push_head_changed"
	// EndCodePushScopeRefused: a commit of the session changes a file that the session
	// may not change: a file outside its plan's scope at the session's start, or any
	// file outside .qory/ when the session has no plan.
	EndCodePushScopeRefused EndCode = "push_scope_refused"
	// EndCodePushKeyFound: a commit of the session adds a key or a value of the run's
	// secrets, or the commits are too large to verify, so qory pushed nothing.
	EndCodePushKeyFound EndCode = "push_key_found"
	// EndCodeRunDocumentsInvalid: the run's documents, or the response to the run code, fail
	// qory's validation.
	EndCodeRunDocumentsInvalid EndCode = "run_documents_invalid"
	// EndCodeHarnessInvalid: the harness that the run composes, from the run documents or
	// the target's harness file, cannot be used: its source, fetch or composition fails,
	// neither has one, its qory version does not fit, or the runtime has no headless start
	// for the workflow.
	EndCodeHarnessInvalid EndCode = "harness_invalid"
	// EndCodeMachineSetupInvalid: the machine cannot run the step: its forager.yaml,
	// folders, home, proxies, image fit, runtime install, push verification or setup
	// container fail, or this machine has no registry.
	EndCodeMachineSetupInvalid EndCode = "machine_setup_invalid"
	// EndCodeTargetFetchFailed: qory could not fetch the target of the run.
	EndCodeTargetFetchFailed EndCode = "target_fetch_failed"
	// EndCodeRepositoryCheckFailed: a git or folder step on the run's repository failed.
	EndCodeRepositoryCheckFailed EndCode = "repository_check_failed"
	// EndCodeForgeUnreachable: qory could not reach the forge of the run's repository.
	EndCodeForgeUnreachable EndCode = "forge_unreachable"
	// EndCodeHistoryRewritten: the HEAD of the run's working folder does not descend from
	// the commit that the run started from, so a push would not be a fast-forward.
	EndCodeHistoryRewritten EndCode = "history_rewritten"
	// EndCodeRunAPIFailed: a call from qory to the run API failed.
	EndCodeRunAPIFailed EndCode = "run_api_failed"
	// EndCodePrebuildFailed: a prebuild failed: its build, the pull of its base image, or
	// the push or verification of the image it built.
	EndCodePrebuildFailed EndCode = "prebuild_failed"
	// EndCodeNextSessionFailed: the next-session request after a session failed: Qory
	// Apiary refused it or did not answer.
	EndCodeNextSessionFailed EndCode = "next_session_failed"
	// EndCodeOutputsInvalid: qory's output check stopped the run before the push: a
	// declared output is missing or does not match its output type.
	EndCodeOutputsInvalid EndCode = "outputs_invalid"
)

// ProviderModel is one model of the run's provider, an entry of
// run_settings.provider_models. Effort lists the effort values the model accepts, 1 to
// 16, each listed once. An entry with no Effort means the model takes no effort value.
type ProviderModel struct {
	ID     string   `json:"id,case:strict"`
	Effort []string `json:"effort,omitempty,case:strict"`
}

// Tiers is run_settings.tiers: a model, and optionally an effort value, for each tier
// the control plane maps. A tier the control plane does not map is nil. For a run with
// a target, a tier the control plane maps applies instead of the entry for that tier in
// the tier map set at the run's target, and the tier map set at the run's target fills
// only the tiers left nil. A run with no target has no such tier map: only the tiers the
// control plane maps apply.
type Tiers struct {
	Light    *Tier `json:"light,omitempty,case:strict"`
	Balanced *Tier `json:"balanced,omitempty,case:strict"`
	Thinking *Tier `json:"thinking,omitempty,case:strict"`
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

// definesPointer reports whether a pointer below a Tier leads to model or to effort, the
// members of the object form.
func (Tier) definesPointer(segments []string) bool {
	return len(segments) == 1 && (segments[0] == "model" || segments[0] == "effort")
}

// tierObject is the object form of a [Tier].
type tierObject struct {
	Model  string `json:"model,case:strict"`
	Effort string `json:"effort,omitempty,case:strict"`
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
