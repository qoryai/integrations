package contracts

import (
	"bytes"
	"encoding/json"
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
// the machine composes that file at TargetVersion, and Harness is nil. A harness section
// extends a stack when it has extends, a base, and defines one when it has no extends
// and lists at least one module. The run documents contain Harness in three cases:
// TargetVersion is set and the read returned a file whose harness section is absent, or
// has no extends and lists no module; TargetVersion is set and the read returned found:
// false; or TargetVersion is empty, because the run has no target or the control plane
// read no harness file of the target. When Harness is set, the machine composes Harness
// instead of the harness section of the harness file of the target. Of that file, the
// machine applies the qory version that the file requires, the worktree section, and the
// tiers of the harness section, as the doc of [RunSettings.Tiers] states. If the
// machine's qory version does not meet that requirement, the run stops. It reads none of
// the other machine keys of that file and none of its git and env sections, so the
// harness file of the target does not set how the run starts. The machine works at
// TargetVersion when it is set. A run of the step explore has WorkItem. A run of the
// step prebuild has Access, Target, ImageDestination and exactly one of
// EnvironmentDestination and EnvironmentImage, and no TargetVersion, WorkItem,
// RunSettings, Workflow, HarnessHidden or Harness. RunSettings is set only for the step
// explore. Inputs is the step's input as the control plane wrote it, whose shape belongs
// to the step's schema.
type Documents struct {
	Step   string          `json:"step"`
	Domain string          `json:"domain"`
	Inputs json.RawMessage `json:"inputs"`
	Access string          `json:"access,omitempty"`
	Target *Target         `json:"target,omitempty"`
	// TargetVersion is set only with Target, and never for the step prebuild: the
	// version of the target at which the control plane checked for a harness file, the
	// resolved_at of the read at the path of the harness file, whether the read returned
	// the file or found: false. The machine works at this version, and composes the
	// harness file at it when the read returned a file whose harness section has extends
	// or lists at least one module. It is the exact version that the read returned,
	// never a name that can point to another version later, because the machine would
	// then compose a file that the control plane did not read: 1 to 255 letters, digits
	// and the characters ._/-, a generic grammar, because a target in another system has
	// another kind of version.
	TargetVersion string       `json:"target_version,omitempty"`
	WorkItem      *WorkItem    `json:"work_item,omitempty"`
	RunSettings   *RunSettings `json:"run_settings,omitempty"`
	// Workflow is set exactly when HarnessHidden is: the workflow of the harness that
	// the run starts, invoked as its command, such as hello, which the job invokes as
	// /hello.
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
	// module, the machine composes that file at TargetVersion, and Harness is nil. When
	// Harness is set, the machine composes Harness instead of the harness section of the
	// harness file of the target. Of that file, the machine applies the qory version
	// that the file requires, the worktree section, and the tiers of the harness
	// section, as the doc of [RunSettings.Tiers] states. If the machine's qory version
	// does not meet that requirement, the run stops. It reads none of the other machine
	// keys of that file and none of its git and env sections, so the harness file of the
	// target does not set how the run starts.
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
	// Bind maps <kind>/<role> to the name of the entry of that kind that fills the
	// role.
	Bind map[string]string `json:"bind,omitempty"`
	// Extensions are values that qory copies into the report and does not read, by
	// key, each a JSON value of any shape. Empty extensions are not written.
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
	// Workflows are the settings of each workflow of the stack the section defines, by
	// the workflow's name, as workflows in qory-stack.yaml has them: a lower-case
	// letter, then up to 63 lower-case letters, digits and hyphens. Only without
	// Extends: a section with Extends takes the workflows of its base.
	Workflows map[string]HarnessWorkflow `json:"workflows,omitempty"`
}

// HarnessWorkflow is the settings of one workflow of a stack that a [HarnessSection]
// defines.
type HarnessWorkflow struct {
	// Access is the access level the workflow declares for what a run of it works on,
	// "read" or "write", and empty for read.
	Access string `json:"access,omitempty"`
	// TimeLimit is how long a run of the workflow may run, counted from the run's open,
	// as a Go duration above zero, such as 7h or 45m, and empty when the workflow sets
	// none. The run's run_settings.time_limit_minutes, when the control plane sets it,
	// applies instead.
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
// model settings the job composes the agent's harness with. It contains no key or
// secret. With no provider chosen, Model, Effort, ProviderModels and Tiers are empty:
// the run uses the gateway's configured key and the runtime's default model. Effort
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
	// TimeLimitMinutes is the time limit that the client set for the run, in minutes,
	// from 15 to 1440, and 0 when the client set none. The machine sets it as the
	// session's time limit.
	TimeLimitMinutes int `json:"time_limit_minutes,omitempty"`
}

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
