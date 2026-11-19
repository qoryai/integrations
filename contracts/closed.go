package contracts

import (
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// CompileClosed compiles the closed copy of a schema of the integration contract that has
// one: description.schema.json, adapter.schema.json, changes.schema.json,
// changes-submitted.schema.json, changes-failed.schema.json or source-items.schema.json,
// whose closed copy closes the listing and the person that the definitions share. A reader of these ignores a
// member that its schema does not define. The closed copy refuses such a member, in the
// schema and in each schema of the integration and run contracts that it refers to, so a
// writer's tests validate against the closed copy, and a misspelled member fails a test.
// See [CloseSchema]. It returns an error for any other name, because the closed copy of a
// schema that lists only some members of an object refuses members it defines.
func CompileClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(Base, FS, name)
}

// CompileRunClosed compiles the closed copy of documents.schema.json,
// next-session-answer.schema.json, next-session-request.schema.json or outcome.schema.json,
// as [CompileClosed] does for the integration contract, and returns an error for any other
// name.
func CompileRunClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(RunBase, RunFS, name)
}

// CompileProvisionerClosed compiles the closed copy of claim.schema.json,
// claim-answer.schema.json, dispatched.schema.json, machine-end.schema.json,
// machine-log.schema.json, refusal.schema.json or release.schema.json of the provisioner
// contract, as [CompileClosed] does for the integration contract, and returns an error for
// any other name.
func CompileProvisionerClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(ProvisionerBase, ProvisionerFS, name)
}

// CompileMachinesClosed compiles the closed copy of any schema of the machines contract,
// as [CompileClosed] does for the integration contract, and returns an error for any other
// name.
func CompileMachinesClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(MachinesBase, MachinesFS, name)
}

// CloseSchema returns a copy of the schema doc, as [Document] reads it, that refuses a
// member the schema does not define, and the JSON pointers of the places it closed, ""
// for the root. It sets additionalProperties to false in each schema that lists properties
// and sets no additionalProperties, at the root, under $defs, properties, patternProperties
// and items, in the schema of additionalProperties, in each branch of oneOf and anyOf, and
// in an else or then that sets type object, which is the object form of a value that has
// two forms. It closes no other schema inside allOf, if, then, else, not or
// dependentSchemas, which list only some of the members of their object. A schema that sets
// additionalProperties stays as it is, such as roles in a description, which leaves a role
// it does not know alone, or the inputs of a run, whose shape another schema defines. A
// schema that lists no properties stays as it is too, such as a map of extensions or the
// settings of a description, whose schema is the integration's. The copy keeps
// must_understand as it is. It is for a schema whose objects list all their members, and
// [CompileClosed] and [CompileRunClosed] accept only the schemas of closable.
func CloseSchema(doc any) (closed any, sites []string) {
	closed = closeNode(doc, "", true, &sites)
	return closed, sites
}

// closeNode copies the schema n and closes it, and the schemas inside it. instance is
// true when n describes a whole object, as opposed to a branch that lists only part of
// one. ptr is the JSON pointer of n.
func closeNode(n any, ptr string, instance bool, sites *[]string) any {
	m, ok := n.(map[string]any)
	if !ok {
		return n
	}
	out := make(map[string]any, len(m)+1)
	for k, v := range m {
		switch k {
		case "properties", "$defs", "patternProperties", "dependentSchemas":
			sub := make(map[string]any)
			for name, s := range v.(map[string]any) {
				sub[name] = closeNode(s, ptr+"/"+k+"/"+escapePointer(name), k != "dependentSchemas", sites)
			}
			out[k] = sub
		case "items", "additionalProperties":
			out[k] = closeNode(v, ptr+"/"+k, true, sites)
		case "then", "else":
			out[k] = closeNode(v, ptr+"/"+k, isObjectForm(v), sites)
		case "if", "not":
			out[k] = closeNode(v, ptr+"/"+k, false, sites)
		case "allOf", "anyOf", "oneOf":
			list := v.([]any)
			sub := make([]any, len(list))
			for i, s := range list {
				sub[i] = closeNode(s, ptr+"/"+k+"/"+strconv.Itoa(i), k != "allOf", sites)
			}
			out[k] = sub
		default:
			out[k] = v
		}
	}
	_, hasAdditional := m["additionalProperties"]
	_, hasProperties := m["properties"]
	if instance && hasProperties && !hasAdditional {
		out["additionalProperties"] = false
		*sites = append(*sites, ptr)
	}
	return out
}

// isObjectForm reports whether the then or else schema v is the object form of a value
// that has two forms: it sets type object, and lists properties.
func isObjectForm(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, hasProperties := m["properties"]
	return m["type"] == "object" && hasProperties
}

// closable are the schemas that have a closed copy, by $id.
var closable = map[string]bool{
	Base + "/description.schema.json":             true,
	Base + "/adapter.schema.json":                 true,
	Base + "/changes.schema.json":                 true,
	Base + "/changes-submitted.schema.json":       true,
	Base + "/changes-failed.schema.json":          true,
	Base + "/source-items.schema.json":            true,
	RunBase + "/documents.schema.json":            true,
	RunBase + "/next-session-answer.schema.json":  true,
	RunBase + "/next-session-request.schema.json": true,
	RunBase + "/outcome.schema.json":              true,

	ProvisionerBase + "/claim.schema.json":        true,
	ProvisionerBase + "/claim-answer.schema.json": true,
	ProvisionerBase + "/dispatched.schema.json":   true,
	ProvisionerBase + "/machine-end.schema.json":  true,
	ProvisionerBase + "/machine-log.schema.json":  true,
	ProvisionerBase + "/refusal.schema.json":      true,
	ProvisionerBase + "/release.schema.json":      true,

	MachinesBase + "/check.schema.json":         true,
	MachinesBase + "/check-answer.schema.json":  true,
	MachinesBase + "/delete.schema.json":        true,
	MachinesBase + "/delete-answer.schema.json": true,
	MachinesBase + "/error.schema.json":         true,
	MachinesBase + "/list.schema.json":          true,
	MachinesBase + "/list-answer.schema.json":   true,
	MachinesBase + "/log.schema.json":           true,
	MachinesBase + "/log-answer.schema.json":    true,
	MachinesBase + "/make.schema.json":          true,
	MachinesBase + "/make-answer.schema.json":   true,
}

// closingLoader reads the schemas of the contracts that this package embeds, closed as
// [CloseSchema] closes them, and any other schema as [loader] reads it. A closed copy
// compiles with it, so the schemas that the copy refers to are closed too, in this
// compilation alone: the schemas that a reader compiles stay open.
type closingLoader struct{ loader }

// Load returns the closed document of a schema of an embedded contract, and the document of
// any other schema as it is.
func (l closingLoader) Load(url string) (any, error) {
	doc, err := l.loader.Load(url)
	if err != nil {
		return nil, err
	}
	for _, t := range trees {
		if strings.HasPrefix(url, t.base+"/") {
			doc, _ = CloseSchema(doc)
			break
		}
	}
	return doc, nil
}

// escapePointer escapes a member name for a JSON pointer.
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
