// Package conformance checks what a program of an integration prints against the
// contracts it speaks: its description against the integration contract,
// contracts/integration/v1, a credential role's answer against the runner's credential
// document, and a failure against the exit status every command of the contract shares.
//
// Beyond the description's schema, it checks the rules the schema cannot express: where
// a secret is, its title, its <name>_file and its x-secret-name; the settings each role
// lists and requires, credential and tool; that the credential role's hosts and the tool
// role's serves do not overlap; and the tool role's mcp URL. It names each keyword the
// settings' top level carries outside the few it may, such as required, which the
// schema refuses.
//
// An integration's tests run the program and hand this package what it printed, so the
// program and the contracts cannot drift apart. The integration template's tests show
// how (https://github.com/qoryai/integration-template).
//
// Every role is started as <program> <role> -- [arguments], with the settings on
// standard input, and a program refuses --settings as it refuses any flag it does not
// know. An integration's tests run <program> credential --settings '{}' -- a/b and hand
// its exit status, standard output and standard error to Failure.
package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/qoryai/integrations/contracts"
	runner "github.com/qoryai/runner/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	descriptionSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
		return contracts.Compile("description.schema.json")
	})
	credentialSchema = sync.OnceValues(func() (*jsonschema.Schema, error) {
		return runner.Compile("credential.schema.json")
	})
)

// Description checks what `<program> describe` printed on standard output: one JSON
// document and nothing after it, which the contract's description.schema.json accepts,
// and whose every secret, a property marked writeOnly, is a property of the settings
// themselves with a title and a setting <name>_file beside it (contracts/integration/v1
// §Describe). An x-secret-name is on a secret alone, matches ^[A-Z][A-Z0-9_]{0,127}$,
// and names one secret of the description.
//
// The credential and tool roles list settings the settings define, a secret as <name>,
// never <name>_file, a role requires only settings it lists, and a role lists every
// secret. No host is both one the credential role answers for and one the tool role
// serves, the same or covered by a *. pattern. The tool role's mcp is an https URL with
// no userinfo, port or fragment, on a lower-case host name the tool serves.
//
// The settings' top level carries only type, properties, patternProperties,
// additionalProperties, unevaluatedProperties, propertyNames, title, description,
// $comment, $schema, $id and $defs: each role's document holds a subset of the settings,
// and another keyword, such as required, can refuse a subset the whole settings pass.
// The schema refuses every other keyword, and Description names each one the settings
// carry, in sorted order, beside the schema's refusal. Otherwise it reports the schema's
// refusal, or else every rule beyond the schema the output breaks.
//
// The error's text is "describe: " followed by the refusals, joined by "; ". The error
// also has an Unwrap() []error method that returns each refusal as its own error, in
// the order of the text. The schema's refusal is one of them and comes last: its text
// is "the contract's schema refuses the description: " followed by the validator's
// error, which it wraps, so errors.Is and errors.As reach that error.
func Description(stdout []byte) error {
	doc, err := one(stdout)
	if err != nil {
		return refusals{err}
	}
	schema, err := descriptionSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(doc); err != nil {
		out := refusals{}
		for _, p := range topLevel(doc) {
			out = append(out, errors.New(p))
		}
		return append(out, fmt.Errorf("the contract's schema refuses the description: %w", err))
	}
	if problems := append(secrets(doc), roles(doc)...); len(problems) > 0 {
		out := refusals{}
		for _, p := range problems {
			out = append(out, errors.New(p))
		}
		return out
	}
	return nil
}

// refusals are the refusals of a description, each an error of its own.
type refusals []error

// Error is "describe: " followed by the refusals' texts, joined by "; ".
func (r refusals) Error() string {
	texts := make([]string, len(r))
	for i, err := range r {
		texts[i] = err.Error()
	}
	return "describe: " + strings.Join(texts, "; ")
}

// Unwrap returns each refusal, in order.
func (r refusals) Unwrap() []error { return r }

// Credential checks what `<program> credential` printed on standard output: one JSON
// document and nothing after it, which the runner's credential.schema.json accepts
// (https://github.com/qoryai/runner/tree/main/contracts/runner/v1#credentials).
func Credential(stdout []byte) error {
	if _, err := one(stdout); err != nil {
		return fmt.Errorf("credential: %w", err)
	}
	doc, err := runner.Decode("answer.json", stdout)
	if err != nil {
		return fmt.Errorf("credential: %w", err)
	}
	schema, err := credentialSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("credential: the runner's schema refuses the answer: %w", err)
	}
	return nil
}

// Failure checks how a program ended when it failed (contracts/integration/v1 §Exit
// status): a status other than 0, nothing on standard output, and one line on standard
// error, which says what failed.
func Failure(code int, stdout, stderr []byte) error {
	switch {
	case code == 0:
		return errors.New("a failure exits 0; it must exit with another status")
	case len(stdout) != 0:
		return fmt.Errorf("a failure prints %d bytes on standard output; it must print nothing there", len(stdout))
	case bytes.Count(stderr, []byte("\n")) != 1 || !bytes.HasSuffix(stderr, []byte("\n")):
		return fmt.Errorf("a failure writes %q on standard error; it must write one line", stderr)
	case len(bytes.TrimSpace(stderr)) == 0:
		return errors.New("a failure writes an empty line on standard error; it must say what failed")
	}
	return nil
}

// one reads b as one JSON document followed by nothing but white space, into the JSON
// types a schema validates.
func one(b []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	var first json.RawMessage
	if err := dec.Decode(&first); err != nil {
		return nil, fmt.Errorf("standard output is not a JSON document: %w", err)
	}
	var more json.RawMessage
	if err := dec.Decode(&more); !errors.Is(err, io.EOF) {
		return nil, errors.New("standard output contains more than one JSON document")
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(first))
}

// topKeywords are the only keywords the settings' top level may carry. Each role's
// document holds a subset of the settings, and none of these can refuse a subset the
// whole settings pass. A rule across settings belongs in a role's required or in the
// program.
var topKeywords = []string{
	"type", "properties", "patternProperties", "additionalProperties", "unevaluatedProperties",
	"propertyNames", "title", "description", "$comment", "$schema", "$id", "$defs",
}

// topLevel are the keywords the settings' top level carries outside topKeywords, each
// named in sorted order, followed by the list it may carry when there is one.
func topLevel(doc any) []string {
	d, _ := doc.(map[string]any)
	settings, _ := d["settings"].(map[string]any)
	var out []string
	for _, k := range sortedKeys(settings) {
		if !slices.Contains(topKeywords, k) {
			out = append(out, fmt.Sprintf("the settings have the top-level keyword %s", k))
		}
	}
	if len(out) > 0 {
		out = append(out, "the top level may carry only "+strings.Join(topKeywords, ", ")+", which no role's subset of the settings can break")
	}
	return out
}

// secretName is the grammar of an x-secret-name, the conventional name of a secret, such
// as GITHUB_APP_PRIVATE_KEY.
var secretName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,127}$`)

// secrets are the ways a description breaks the rules the schema cannot express: a secret
// is a property of the settings themselves, never one nested in another or outside the
// properties, it has a title, a string that is not only white space, and a secret
// <name> has a setting <name>_file for a secret kept on disk; an x-secret-name is on a
// secret alone, matches secretName, and no two secrets have the same one.
func secrets(doc any) []string {
	d, _ := doc.(map[string]any)
	settings, _ := d["settings"].(map[string]any)
	props, _ := settings["properties"].(map[string]any)
	var out []string
	named := map[string]string{}
	for _, name := range sortedKeys(props) {
		p, _ := props[name].(map[string]any)
		secret := p["writeOnly"] == true
		if secret {
			if _, ok := props[name+"_file"]; !ok {
				out = append(out, fmt.Sprintf("the secret %s has no setting %s_file", name, name))
			}
			if title, _ := p["title"].(string); strings.TrimSpace(title) == "" {
				out = append(out, fmt.Sprintf("the secret %s has no title", name))
			}
		}
		if v, ok := p["x-secret-name"]; ok {
			s, isString := v.(string)
			switch {
			case !secret:
				out = append(out, fmt.Sprintf("the setting %s has an x-secret-name and is not a secret", name))
			case !isString:
				out = append(out, fmt.Sprintf("the secret %s has an x-secret-name that is not a string", name))
			case !secretName.MatchString(s):
				out = append(out, fmt.Sprintf("the secret %s has the x-secret-name %q, which does not match %s", name, s, secretName))
			case named[s] != "":
				out = append(out, fmt.Sprintf("the secrets %s and %s have the same x-secret-name %s", named[s], name, s))
			default:
				named[s] = name
			}
		}
		for _, at := range marked(p, isSecret, "/settings/properties/"+name, true) {
			out = append(out, fmt.Sprintf("%s is a secret nested in a setting", at))
		}
		for _, at := range marked(p, isNamed, "/settings/properties/"+name, true) {
			out = append(out, fmt.Sprintf("%s has an x-secret-name nested in a setting", at))
		}
	}
	rest := maps.Clone(settings)
	delete(rest, "properties")
	for _, at := range marked(rest, isSecret, "/settings", false) {
		out = append(out, fmt.Sprintf("%s is a secret outside the settings' properties", at))
	}
	for _, at := range marked(rest, isNamed, "/settings", false) {
		out = append(out, fmt.Sprintf("%s has an x-secret-name outside the settings' properties", at))
	}
	return out
}

// defined are the roles the contract defines, each with the settings it lists: the
// credential role, the API way, and the tool role, the MCP way.
var defined = []string{"credential", "tool"}

// roles are the ways a description's roles break the rules the schema cannot express
// (contracts/integration/v1 §Settings and §Roles). A role lists settings the settings
// define, and a secret as <name>, never as <name>_file. A role requires only settings
// it lists. A role lists every secret. No host is both one the credential role answers
// for and one the tool role serves, the same or covered by a *. pattern. The tool's mcp
// is an https URL with no userinfo, port or fragment, on a lower-case host name the tool
// serves.
func roles(doc any) []string {
	d, _ := doc.(map[string]any)
	settings, _ := d["settings"].(map[string]any)
	props, _ := settings["properties"].(map[string]any)
	all, _ := d["roles"].(map[string]any)
	secret := func(name string) bool {
		p, _ := props[name].(map[string]any)
		return p["writeOnly"] == true
	}
	// fileOf is the secret whose <name>_file name is, when it is one.
	fileOf := func(name string) (string, bool) {
		s, ok := strings.CutSuffix(name, "_file")
		return s, ok && secret(s)
	}
	var out, present []string
	lists := map[string]map[string]bool{}
	for _, role := range defined {
		r, ok := all[role].(map[string]any)
		if !ok {
			continue
		}
		present = append(present, role)
		lists[role] = map[string]bool{}
		for _, name := range stringsOf(r["settings"]) {
			lists[role][name] = true
			if s, ok := fileOf(name); ok {
				out = append(out, fmt.Sprintf("the role %s lists %s, the file of the secret %s; it lists the secret as %s", role, name, s, s))
			} else if _, ok := props[name]; !ok {
				out = append(out, fmt.Sprintf("the role %s lists the setting %s, which the settings do not define", role, name))
			}
		}
		for _, name := range stringsOf(r["required"]) {
			if !lists[role][name] {
				out = append(out, fmt.Sprintf("the role %s requires %s, which its settings do not list", role, name))
			}
		}
	}
	for _, name := range sortedKeys(props) {
		if secret(name) && !slices.ContainsFunc(present, func(role string) bool { return lists[role][name] }) {
			out = append(out, fmt.Sprintf("the secret %s is listed by no role", name))
		}
	}
	credential, _ := all["credential"].(map[string]any)
	tool, _ := all["tool"].(map[string]any)
	serves := stringsOf(tool["serves"])
	for _, host := range stringsOf(credential["hosts"]) {
		for _, served := range serves {
			if covers(host, served) || covers(served, host) {
				out = append(out, fmt.Sprintf("the host %s of the role credential and the host %s the role tool serves overlap", host, served))
			}
		}
	}
	if m, ok := tool["mcp"].(string); ok {
		out = append(out, mcp(m, serves)...)
	}
	return out
}

// hostName is the grammar of a lower-case host name, the host grammar of the contract's
// schema without its *. pattern.
var hostName = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// written is the host of the https URL m as m spells it: what follows https:// up to
// the path, the query or the fragment, without userinfo and port. url.Parse decodes a
// host's percent escapes, and this does not.
func written(m string) string {
	host := strings.TrimPrefix(m, "https://")
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 && !strings.Contains(host[i:], "]") {
		host = host[:i]
	}
	return host
}

// mcp are the ways the tool role's mcp, m, breaks the rules the schema cannot express:
// it is an https URL with no userinfo, port or fragment, whose host is a lower-case host
// name the tool serves. An upper-case letter, a trailing dot, an IPv6 address and a
// percent escape in the host are each refused.
func mcp(m string, serves []string) []string {
	u, err := url.Parse(m)
	if err != nil || u.Scheme != "https" || u.Opaque != "" {
		return []string{fmt.Sprintf("the role tool's mcp %q is not an https URL", m)}
	}
	var out []string
	if u.User != nil {
		out = append(out, fmt.Sprintf("the role tool's mcp %q has userinfo", m))
	}
	if u.Port() != "" || strings.HasSuffix(u.Host, ":") {
		out = append(out, fmt.Sprintf("the role tool's mcp %q has a port", m))
	}
	if strings.Contains(m, "#") {
		out = append(out, fmt.Sprintf("the role tool's mcp %q has a fragment", m))
	}
	switch host := written(m); {
	case host == "":
		out = append(out, fmt.Sprintf("the role tool's mcp %q has no host", m))
	case !hostName.MatchString(host):
		out = append(out, fmt.Sprintf("the role tool's mcp %q has the host %q, which is not a lower-case host name", m, host))
	case !slices.ContainsFunc(serves, func(served string) bool { return covers(served, host) }):
		out = append(out, fmt.Sprintf("the role tool's mcp %q is on the host %q, which the role tool does not serve", m, host))
	}
	return out
}

// covers reports whether the host entry covers other, as the runner's egress grammar
// reads them: a name covers the same name, and *.x covers every host below x, at any
// depth, and every pattern *.y below it, never x itself. Two hosts overlap when either
// covers the other.
func covers(entry, other string) bool {
	if entry == other {
		return true
	}
	suffix, ok := strings.CutPrefix(entry, "*.")
	return ok && strings.HasSuffix(strings.TrimPrefix(other, "*."), "."+suffix)
}

// stringsOf are the strings of a JSON array, in their order.
func stringsOf(v any) []string {
	a, _ := v.([]any)
	var out []string
	for _, e := range a {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// isSecret is a keyword that marks a secret, writeOnly true; isNamed one that names a
// secret, x-secret-name with any value.
func isSecret(k string, v any) bool { return k == "writeOnly" && v == true }
func isNamed(k string, _ any) bool  { return k == "x-secret-name" }

// data are the keywords whose values are data, not schemas: a key spelled writeOnly or
// x-secret-name in them marks nothing.
var data = map[string]bool{"default": true, "const": true, "enum": true, "examples": true}

// byName are the keywords whose values map names to schemas: their keys are names, not
// keywords, so a property named writeOnly marks nothing.
var byName = map[string]bool{"properties": true, "patternProperties": true, "dependentSchemas": true, "$defs": true, "definitions": true}

// marked are the places under the schema v whose keyword and its value mark holds of,
// but v's own when top is set. It never looks into data: a default, a const, an enum or
// an example.
func marked(v any, mark func(k string, v any) bool, at string, top bool) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(v) {
			if data[k] {
				continue
			}
			if m, ok := v[k].(map[string]any); ok && byName[k] {
				for _, name := range sortedKeys(m) {
					out = append(out, marked(m[name], mark, at+"/"+k+"/"+name, false)...)
				}
				continue
			}
			if mark(k, v[k]) && !top {
				out = append(out, at)
			}
			out = append(out, marked(v[k], mark, at+"/"+k, false)...)
		}
	case []any:
		for i, c := range v {
			out = append(out, marked(c, mark, at+"/"+strconv.Itoa(i), false)...)
		}
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
