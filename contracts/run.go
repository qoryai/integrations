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
// 201 of the run code's redemption. Forager passes it on unread. Step, Domain and
// Inputs are set for every run. Access is set exactly when Target is, and Stack
// exactly when Workflow and HarnessHidden are. StackReference is set exactly when
// Stack is a stack name. A run of the step explore has WorkItem. A run of the step
// prebuild has Access, Target, ImageDestination and exactly one of
// EnvironmentDestination and EnvironmentImage, and no WorkItem, RunSettings, Stack,
// StackReference, Workflow or HarnessHidden.
// RunSettings is set only for the step explore. Inputs is the step's input as the
// control plane wrote it, whose shape belongs to the step's schema.
type Documents struct {
	Step        string          `json:"step"`
	Domain      string          `json:"domain"`
	Inputs      json.RawMessage `json:"inputs"`
	Access      string          `json:"access,omitempty"`
	Target      *Target         `json:"target,omitempty"`
	WorkItem    *WorkItem       `json:"work_item,omitempty"`
	RunSettings *RunSettings    `json:"run_settings,omitempty"`
	// Stack is the stack that the run uses: a stack name, such as core, which is the
	// name of a stack in the operator's list of stacks, with StackReference then set to
	// the address of that stack, or a stack reference, url#ref:path, which is where the
	// stack lives, such as https://code.example.com/team/stacks#main:core.
	Stack string `json:"stack,omitempty"`
	// StackReference is set exactly when Stack is a stack name: the address of the
	// stack whose name Stack is set to, a stack reference, url#ref:path, such as
	// https://code.example.com/team/stacks#main:core. The control plane sets it from
	// the operator's list of stacks, and the machine fetches the stack from it.
	StackReference string `json:"stack_reference,omitempty"`
	Workflow       string `json:"workflow,omitempty"`
	// HarnessHidden is set exactly when Stack is: when true, the text that comes from
	// the harness of the stack is hidden in the run, and when false, that text is
	// shown. A pointer, so that false is written.
	HarnessHidden *bool `json:"harness_hidden,omitempty"`
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
