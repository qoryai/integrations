// Package conformance checks what a program of an integration prints against the
// contracts it speaks: its description against the integration contract,
// contracts/integration/v1, a credential role's answer against the runner's credential
// document, and a failure against the exit status every command of the contract shares.
//
// An integration's tests run the program and hand this package what it printed, so the
// program and the contracts cannot drift apart. The integration template's tests show
// how (https://github.com/qoryai/integration-template).
package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// themselves with a setting <name>_file beside it (contracts/integration/v1 §Describe).
// It reports the first rule the output breaks.
func Description(stdout []byte) error {
	doc, err := one(stdout)
	if err != nil {
		return fmt.Errorf("describe: %w", err)
	}
	schema, err := descriptionSchema()
	if err != nil {
		return err
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("describe: the contract's schema refuses the description: %w", err)
	}
	if problems := secrets(doc); len(problems) > 0 {
		return fmt.Errorf("describe: %s", strings.Join(problems, "; "))
	}
	return nil
}

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

// secrets are the ways a description breaks the rule the schema cannot express: a secret
// is a property of the settings themselves, never one nested in another or outside the
// properties, and a secret <name> has a setting <name>_file passed on a command line in
// its place.
func secrets(doc any) []string {
	d, _ := doc.(map[string]any)
	settings, _ := d["settings"].(map[string]any)
	props, _ := settings["properties"].(map[string]any)
	var out []string
	for _, name := range sortedKeys(props) {
		p, _ := props[name].(map[string]any)
		if p["writeOnly"] == true {
			if _, ok := props[name+"_file"]; !ok {
				out = append(out, fmt.Sprintf("the secret %s has no setting %s_file", name, name))
			}
		}
		for _, at := range writeOnly(p, "/settings/properties/"+name, true) {
			out = append(out, fmt.Sprintf("%s is a secret nested in a setting", at))
		}
	}
	for _, k := range sortedKeys(settings) {
		if k != "properties" {
			for _, at := range writeOnly(settings[k], "/settings/"+k, false) {
				out = append(out, fmt.Sprintf("%s is a secret outside the settings' properties", at))
			}
		}
	}
	return out
}

// writeOnly are the places under v that mark a secret, but v's own mark when top is set.
func writeOnly(v any, at string, top bool) []string {
	var out []string
	switch v := v.(type) {
	case map[string]any:
		for _, k := range sortedKeys(v) {
			if k == "writeOnly" && v[k] == true && !top {
				out = append(out, at)
			}
			out = append(out, writeOnly(v[k], at+"/"+k, false)...)
		}
	case []any:
		for i, c := range v {
			out = append(out, writeOnly(c, at+"/"+strconv.Itoa(i), false)...)
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
