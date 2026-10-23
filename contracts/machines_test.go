package contracts_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

// machinesSchemas are the schemas of the machines contract: a request and an answer for
// each command, and the error.
var machinesSchemas = []string{
	"check.schema.json",
	"check-answer.schema.json",
	"make.schema.json",
	"make-answer.schema.json",
	"list.schema.json",
	"list-answer.schema.json",
	"log.schema.json",
	"log-answer.schema.json",
	"delete.schema.json",
	"delete-answer.schema.json",
	"error.schema.json",
}

// compileMachines compiles a schema of the machines contract, or fails the test.
func compileMachines(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	s, err := contracts.CompileMachines(name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return s
}

// TestMachinesSchemasAreTheContract pins that machines/v1 holds exactly the eleven
// schemas, that each has the $id MachinesBase/<file>, and that each compiles.
func TestMachinesSchemasAreTheContract(t *testing.T) {
	entries, err := fs.ReadDir(contracts.MachinesFS, ".")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		if !e.IsDir() {
			got = append(got, e.Name())
		}
	}
	want := slices.Clone(machinesSchemas)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("machines/v1 holds %v, want %v", got, want)
	}
	for _, name := range machinesSchemas {
		doc, err := contracts.MachinesDocument(name)
		if err != nil {
			t.Fatal(err)
		}
		if id := doc.(map[string]any)["$id"]; id != contracts.MachinesBase+"/"+name {
			t.Errorf("%s has the $id %v", name, id)
		}
		compileMachines(t, name)
	}
}

// TestMachinesFixturesValidate pins that each document under fixtures/<name>/ of the
// machines contract is accepted by <name>.schema.json, and that every schema has a
// fixture.
func TestMachinesFixturesValidate(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range machinesSchemas {
		dir := strings.TrimSuffix(name, ".schema.json")
		schema := compileMachines(t, name)
		for _, f := range treeFiles(t, contracts.MachinesFS, path.Join("fixtures", dir)) {
			seen[dir] = true
			doc, err := contracts.MachinesDocument(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(doc); err != nil {
				t.Errorf("%s: %v", f, err)
			}
		}
	}
	dirs, err := fs.ReadDir(contracts.MachinesFS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if d.IsDir() && d.Name() != "invalid" && !seen[d.Name()] {
			t.Errorf("fixtures/%s is the fixture folder of no schema", d.Name())
		}
	}
}

// TestInvalidMachinesFixturesAreRefused pins that each document under fixtures/invalid
// of the machines contract fails its schema for the reason in its name, and for no
// other.
func TestInvalidMachinesFixturesAreRefused(t *testing.T) {
	want := map[string]struct{ schema, reason string }{
		"check-version-2.json":                      {"check", "/version const"},
		"check-without-version.json":                {"check", "/ required version"},
		"check-undefined-member.json":               {"check", "/ additionalProperties"},
		"check-answer-version.json":                 {"check-answer", "/ additionalProperties"},
		"check-answer-undefined-member.json":        {"check-answer", "/ additionalProperties"},
		"make-version-2.json":                       {"make", "/version const"},
		"make-key-no-prefix.json":                   {"make", "/key pattern"},
		"make-key-upper-case.json":                  {"make", "/key pattern"},
		"make-pool-dispatch.json":                   {"make", "/pool pattern"},
		"make-size-upper-case.json":                 {"make", "/size pattern"},
		"make-size-65.json":                         {"make", "/size pattern"},
		"make-size-digit-first.json":                {"make", "/size pattern"},
		"make-start-data-url-safe.json":             {"make", "/start_data pattern"},
		"make-start-data-no-padding.json":           {"make", "/start_data pattern"},
		"make-start-data-space.json":                {"make", "/start_data pattern"},
		"make-without-start-data.json":              {"make", "/ required start_data"},
		"make-without-key.json":                     {"make", "/ required key"},
		"make-undefined-member.json":                {"make", "/ additionalProperties"},
		"make-answer-id-dash.json":                  {"make-answer", "/id "},
		"make-answer-id-empty.json":                 {"make-answer", "/id pattern"},
		"make-answer-id-129.json":                   {"make-answer", "/id pattern"},
		"make-answer-id-space.json":                 {"make-answer", "/id pattern"},
		"make-answer-id-non-ascii.json":             {"make-answer", "/id pattern"},
		"make-answer-version.json":                  {"make-answer", "/ additionalProperties"},
		"make-answer-undefined-member.json":         {"make-answer", "/ additionalProperties"},
		"list-version-2.json":                       {"list", "/version const"},
		"list-pool-no-prefix.json":                  {"list", "/pool pattern"},
		"list-without-pool.json":                    {"list", "/ required pool"},
		"list-undefined-member.json":                {"list", "/ additionalProperties"},
		"list-answer-without-machines.json":         {"list-answer", "/ required machines"},
		"list-answer-version.json":                  {"list-answer", "/ additionalProperties"},
		"list-answer-undefined-member.json":         {"list-answer", "/ additionalProperties"},
		"list-answer-id-dash.json":                  {"list-answer", "/machines/0/id "},
		"list-answer-key-no-prefix.json":            {"list-answer", "/machines/0/key pattern"},
		"list-answer-state-gone.json":               {"list-answer", "/machines/0/state enum"},
		"list-answer-started-offset.json":           {"list-answer", "/machines/0/started pattern"},
		"list-answer-started-fraction.json":         {"list-answer", "/machines/0/started pattern"},
		"list-answer-machine-without-started.json":  {"list-answer", "/machines/0 required started"},
		"list-answer-machine-undefined-member.json": {"list-answer", "/machines/0 additionalProperties"},
		"log-version-2.json":                        {"log", "/version const"},
		"log-id-dash.json":                          {"log", "/id "},
		"log-id-space.json":                         {"log", "/id pattern"},
		"log-without-id.json":                       {"log", "/ required id"},
		"log-undefined-member.json":                 {"log", "/ additionalProperties"},
		"log-answer-nul.json":                       {"log-answer", "/log pattern"},
		"log-answer-version.json":                   {"log-answer", "/ additionalProperties"},
		"log-answer-undefined-member.json":          {"log-answer", "/ additionalProperties"},
		"delete-version-2.json":                     {"delete", "/version const"},
		"delete-id-dash.json":                       {"delete", "/id "},
		"delete-id-129.json":                        {"delete", "/id pattern"},
		"delete-without-id.json":                    {"delete", "/ required id"},
		"delete-undefined-member.json":              {"delete", "/ additionalProperties"},
		"delete-answer-state-running.json":          {"delete-answer", "/state enum"},
		"delete-answer-version.json":                {"delete-answer", "/ additionalProperties"},
		"delete-answer-undefined-member.json":       {"delete-answer", "/ additionalProperties"},
		"error-kind-fatal.json":                     {"error", "/error/kind enum"},
		"error-without-kind.json":                   {"error", "/error required kind"},
		"error-without-message.json":                {"error", "/error required message"},
		"error-message-empty.json":                  {"error", "/error/message minLength"},
		"error-message-201.json":                    {"error", "/error/message maxLength"},
		"error-message-nul.json":                    {"error", "/error/message pattern"},
		"error-version.json":                        {"error", "/ additionalProperties"},
		"error-undefined-member.json":               {"error", "/ additionalProperties"},
		"error-failure-undefined-member.json":       {"error", "/error additionalProperties"},
	}
	got := treeFiles(t, contracts.MachinesFS, "fixtures/invalid")
	if len(got) != len(want) {
		t.Errorf("fixtures/invalid holds %d documents, and the test expects %d", len(got), len(want))
	}
	for _, f := range got {
		w, ok := want[path.Base(f)]
		if !ok {
			t.Errorf("%s: no expected reason", f)
			continue
		}
		doc, err := contracts.MachinesDocument(f)
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileMachines(t, w.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", f)
			continue
		}
		if r := strings.Join(leaves(v), "; "); r != w.reason {
			t.Errorf("%s is refused for %q, want %q", f, r, w.reason)
		}
	}
}

// TestAnIDOfADashIsRefusedForTheDashAlone pins that the invalid fixtures whose id is
// exactly -, whose refusal names no keyword, are refused by the not of the id alone.
func TestAnIDOfADashIsRefusedForTheDashAlone(t *testing.T) {
	for _, c := range []struct{ fixture, schema string }{
		{"make-answer-id-dash.json", "make-answer"},
		{"list-answer-id-dash.json", "list-answer"},
		{"log-id-dash.json", "log"},
		{"delete-id-dash.json", "delete"},
	} {
		doc, err := contracts.MachinesDocument(path.Join("fixtures/invalid", c.fixture))
		if err != nil {
			t.Fatal(err)
		}
		v, ok := compileMachines(t, c.schema+".schema.json").Validate(doc).(*jsonschema.ValidationError)
		if !ok {
			t.Errorf("%s was accepted", c.fixture)
			continue
		}
		for len(v.Causes) == 1 {
			v = v.Causes[0]
		}
		if _, ok := v.ErrorKind.(*kind.Not); !ok || len(v.Causes) != 0 {
			t.Errorf("%s is refused for %v, want the not of the id alone", c.fixture, v)
		}
	}
}

// refusedFor validates doc against schema and returns the reasons it is refused for,
// joined by "; ", or "" when it is accepted. A reason of the subschema under then ends
// in " in then".
func refusedFor(t *testing.T, schema *jsonschema.Schema, doc any) string {
	t.Helper()
	err := schema.Validate(doc)
	if err == nil {
		return ""
	}
	v, ok := err.(*jsonschema.ValidationError)
	if !ok {
		t.Fatal(err)
	}
	var out []string
	for _, l := range leafErrors(v) {
		r := strings.Join(leaves(l), "")
		if strings.HasSuffix(l.SchemaURL, "/then") {
			r += " in then"
		}
		out = append(out, r)
	}
	return strings.Join(out, "; ")
}

// leafErrors are the causes of v that have no cause.
func leafErrors(v *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(v.Causes) == 0 {
		return []*jsonschema.ValidationError{v}
	}
	var out []*jsonschema.ValidationError
	for _, c := range v.Causes {
		out = append(out, leafErrors(c)...)
	}
	return out
}

// TestAStartDataHasAtMost16384Bytes pins the bound of a make request's start_data at
// its edge: data of up to 16384 bytes, in standard base64 with padding, is accepted,
// and data of 16385 bytes or more is refused, also when its base64 has no more than
// 21848 characters: then by the pattern under then, which asks a value of 21845
// characters or more to end in ==.
func TestAStartDataHasAtMost16384Bytes(t *testing.T) {
	schema := compileMachines(t, "make.schema.json")
	for _, c := range []struct {
		bytes   int
		padding bool
		reason  string
	}{
		{contracts.MaxStartData - 2, true, ""},
		{contracts.MaxStartData - 1, false, ""},
		{contracts.MaxStartData, true, ""},
		{contracts.MaxStartData + 1, true, "/start_data pattern in then"},
		{contracts.MaxStartData + 2, false, "/start_data pattern in then"},
		{contracts.MaxStartData + 3, true, "/start_data maxLength"},
	} {
		data := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0xa5}, c.bytes))
		if strings.HasSuffix(data, "=") != c.padding {
			t.Fatalf("%d bytes encode to base64 that ends in %q", c.bytes, data[len(data)-2:])
		}
		doc := map[string]any{"version": json.Number("1"), "key": "dp_8w3k0q9t2v7m4r6x",
			"pool": "np_f1xt0re000000000", "size": "small", "start_data": data}
		if r := refusedFor(t, schema, doc); r != c.reason {
			t.Errorf("start data of %d bytes, %d characters, is refused for %q, want %q", c.bytes, len(data), r, c.reason)
		}
	}
}

// TestAConsoleLogHasAtMost786432Characters pins the bound of a log answer's log at its
// edge: 786432 characters are accepted, and 786433 are refused for their length alone.
func TestAConsoleLogHasAtMost786432Characters(t *testing.T) {
	schema := compileMachines(t, "log-answer.schema.json")
	for _, c := range []struct {
		chars  int
		reason string
	}{
		{768 << 10, ""},
		{768<<10 + 1, "/log maxLength"},
	} {
		doc := map[string]any{"log": strings.Repeat("x", c.chars)}
		if r := refusedFor(t, schema, doc); r != c.reason {
			t.Errorf("a log of %d characters is refused for %q, want %q", c.chars, r, c.reason)
		}
	}
}

// TestAConsoleLogWithinItsBoundCanExceedTheAnswerLimit pins why the adapter bounds the
// answer as written as well as the log: a log of 786432 line feeds, which the schema
// accepts, marshals to an answer over the 1 MiB limit.
func TestAConsoleLogWithinItsBoundCanExceedTheAnswerLimit(t *testing.T) {
	out, err := json.Marshal(contracts.MachinesLogAnswer{Log: strings.Repeat("\n", 768<<10)})
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileMachines(t, "log-answer.schema.json"), out); err != nil {
		t.Fatalf("the schema refuses a log of 786432 line feeds: %v", err)
	}
	if len(out) <= contracts.MaxMachinesAnswer {
		t.Errorf("the answer is %d bytes, want more than %d", len(out), contracts.MaxMachinesAnswer)
	}
}

// machinesTypes are the Go type of each schema of the machines contract, by the name
// of its fixture folder.
var machinesTypes = map[string]func() any{
	"check":         func() any { return &contracts.MachinesCheck{} },
	"check-answer":  func() any { return &contracts.MachinesCheckAnswer{} },
	"make":          func() any { return &contracts.MachinesMake{} },
	"make-answer":   func() any { return &contracts.MachinesMakeAnswer{} },
	"list":          func() any { return &contracts.MachinesList{} },
	"list-answer":   func() any { return &contracts.MachinesListAnswer{} },
	"log":           func() any { return &contracts.MachinesLog{} },
	"log-answer":    func() any { return &contracts.MachinesLogAnswer{} },
	"delete":        func() any { return &contracts.MachinesDelete{} },
	"delete-answer": func() any { return &contracts.MachinesDeleteAnswer{} },
	"error":         func() any { return &contracts.MachinesError{} },
}

// TestMachinesTypesRoundTripTheirFixtures pins that each valid fixture of the machines
// contract reads into the Go type of its schema and is written back as the fixture's
// bytes, compacted.
func TestMachinesTypesRoundTripTheirFixtures(t *testing.T) {
	for _, name := range machinesSchemas {
		dir := strings.TrimSuffix(name, ".schema.json")
		newValue, ok := machinesTypes[dir]
		if !ok {
			t.Errorf("%s has no Go type", name)
			continue
		}
		for _, f := range treeFiles(t, contracts.MachinesFS, path.Join("fixtures", dir)) {
			b, err := fs.ReadFile(contracts.MachinesFS, f)
			if err != nil {
				t.Fatal(err)
			}
			v := newValue()
			if err := json.Unmarshal(b, v); err != nil {
				t.Errorf("%s: %v", f, err)
				continue
			}
			var want bytes.Buffer
			if err := json.Compact(&want, b); err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, want.Bytes()) {
				t.Errorf("%s is written back as %s, want %s", f, out, want.Bytes())
			}
		}
	}
}

// TestMachinesTypesMarshalToTheirSchemas marshals values of the machines contract's
// types built in Go, a list answer with a nil Machines among them, and validates each
// against its schema.
func TestMachinesTypesMarshalToTheirSchemas(t *testing.T) {
	values := []struct {
		schema string
		v      any
		want   string
	}{
		{"check", contracts.MachinesCheck{Version: contracts.Version}, `{"version":1}`},
		{"check-answer", contracts.MachinesCheckAnswer{}, `{}`},
		{"make", contracts.MachinesMake{Version: contracts.Version, Key: "dp_8w3k0q9t2v7m4r6x", Pool: "np_f1xt0re000000000",
			Size: "small", StartData: base64.StdEncoding.EncodeToString([]byte("start"))},
			`{"version":1,"key":"dp_8w3k0q9t2v7m4r6x","pool":"np_f1xt0re000000000","size":"small","start_data":"c3RhcnQ="}`},
		{"make-answer", contracts.MachinesMakeAnswer{ID: "machine-1"}, `{"id":"machine-1"}`},
		{"list", contracts.MachinesList{Version: contracts.Version, Pool: "np_f1xt0re000000000"}, `{"version":1,"pool":"np_f1xt0re000000000"}`},
		{"list-answer", contracts.MachinesListAnswer{}, `{"machines":[]}`},
		{"list-answer", contracts.MachinesListAnswer{Machines: []contracts.Machine{{ID: "machine-1", Key: "dp_8w3k0q9t2v7m4r6x",
			State: contracts.MachineStopped, Started: "2026-10-15T12:01:05Z"}}},
			`{"machines":[{"id":"machine-1","key":"dp_8w3k0q9t2v7m4r6x","state":"stopped","started":"2026-10-15T12:01:05Z"}]}`},
		{"log", contracts.MachinesLog{Version: contracts.Version, ID: "machine-1"}, `{"version":1,"id":"machine-1"}`},
		{"log-answer", contracts.MachinesLogAnswer{}, `{"log":""}`},
		{"delete", contracts.MachinesDelete{Version: contracts.Version, ID: "machine-1"}, `{"version":1,"id":"machine-1"}`},
		{"delete-answer", contracts.MachinesDeleteAnswer{State: contracts.MachineGone}, `{"state":"gone"}`},
		{"delete-answer", contracts.MachinesDeleteAnswer{State: contracts.MachineDeleting}, `{"state":"deleting"}`},
		{"error", contracts.MachinesError{Error: contracts.MachinesFailure{Kind: contracts.FailureDefect, Message: "Unknown command: start."}},
			`{"error":{"kind":"defect","message":"Unknown command: start."}}`},
	}
	for _, c := range values {
		b, err := json.Marshal(c.v)
		if err != nil {
			t.Errorf("%T: %v", c.v, err)
			continue
		}
		if string(b) != c.want {
			t.Errorf("%T is written as %s, want %s", c.v, b, c.want)
		}
		if err := validate(compileMachines(t, c.schema+".schema.json"), b); err != nil {
			t.Errorf("%T marshals to %s, which %s.schema.json refuses: %v", c.v, b, c.schema, err)
		}
	}
	for _, s := range []string{contracts.MachineStarting, contracts.MachineRunning, contracts.MachineStopped, contracts.MachineDeleting} {
		b := fmt.Appendf(nil, `{"machines":[{"id":"machine-1","key":"dp_8w3k0q9t2v7m4r6x","state":%q,"started":"2026-10-15T12:01:05Z"}]}`, s)
		if err := validate(compileMachines(t, "list-answer.schema.json"), b); err != nil {
			t.Errorf("the state %s is refused: %v", s, err)
		}
	}
	for _, k := range []string{contracts.FailureLimit, contracts.FailureRetry, contracts.FailureDefect, contracts.FailureOther} {
		b := fmt.Appendf(nil, `{"error":{"kind":%q,"message":"failed"}}`, k)
		if err := validate(compileMachines(t, "error.schema.json"), b); err != nil {
			t.Errorf("the kind %s is refused: %v", k, err)
		}
	}
}
