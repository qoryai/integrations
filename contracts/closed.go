package contracts

import (
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// CompileClosed compiles the closed copy of description.schema.json, the one schema of
// the integration contract whose reader ignores a member that it does not define. The
// closed copy refuses such a member, so a writer's tests validate against the closed
// copy, and a misspelled member fails a test. See [CloseSchema]. A schema that it refers
// to is read as it is. It returns an error for any other name, because the closed copy
// of a schema that lists only some members of an object refuses members it defines.
func CompileClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(Base, FS, name)
}

// CompileRunClosed compiles the closed copy of documents.schema.json or of
// next-session-answer.schema.json, as [CompileClosed] does for the integration contract,
// and returns an error for any other name.
func CompileRunClosed(name string) (*jsonschema.Schema, error) {
	return compileClosed(RunBase, RunFS, name)
}

// CloseSchema returns a copy of the schema doc, as [Document] reads it, that refuses a
// member the schema does not define, and the JSON pointers of the places it closed, ""
// for the root. It sets additionalProperties to false in each schema that lists properties
// and sets no additionalProperties, at the root, under $defs, properties, patternProperties
// and items, in the schema of additionalProperties, and in each branch of oneOf and
// anyOf. It closes no schema inside allOf, if, then, else, not or dependentSchemas, which
// list only some of the members of their object. A schema that sets additionalProperties
// stays as it is, such as roles in a description, which leaves a role it does not know
// alone, or the inputs of a run, whose shape another schema defines. A schema that lists
// no properties stays as it is too, such as a map of extensions. The copy keeps
// must_understand as it is: the list of names stays the schema's. It is for a schema whose
// objects list all their members, and [CompileClosed] and [CompileRunClosed] accept only
// three schemas.
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
		case "if", "then", "else", "not":
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

// closable are the schemas that have a closed copy, by $id.
var closable = map[string]bool{
	Base + "/description.schema.json":            true,
	RunBase + "/documents.schema.json":           true,
	RunBase + "/next-session-answer.schema.json": true,
}

// escapePointer escapes a member name for a JSON pointer.
func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
