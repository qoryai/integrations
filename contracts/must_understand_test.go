package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/qoryai/integrations/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// understood is a schema whose reader ignores a member that it does not define: its name,
// the compile functions of the schema and of its closed copy, the files of its valid
// fixtures, the places of those fixtures that define no members, which the closed copy
// leaves open, such as the settings schema of a description and the auth of an API, which
// the Forager contract defines, and, for a schema with must_understand, a valid fixture, a pointer to a
// member of that fixture, and a change that sets a known member to a bad value.
type understood struct {
	name     string
	open     func() (*jsonschema.Schema, error)
	closed   func() (*jsonschema.Schema, error)
	read     func(name string) (any, error)
	list     func(t *testing.T) []string
	freePlac *regexp.Regexp
	only     *regexp.Regexp // when set, the places that the schema leaves open; the others are closed in both
	valid    string
	known    string
	// openPlace is a pointer below which the schema defines no members, so must_understand
	// accepts any pointer there, or "".
	openPlace string
	invalid   func(m map[string]any) // sets a known member to a bad value
}

// understoodSchemas lists them.
func understoodSchemas() []understood {
	runList := func(dir string) func(*testing.T) []string {
		return func(t *testing.T) []string { return runFiles(t, dir) }
	}
	intList := func(dir string) func(*testing.T) []string {
		return func(t *testing.T) []string { return files(t, dir) }
	}
	list := []understood{
		{
			name:      "documents.schema.json",
			open:      func() (*jsonschema.Schema, error) { return contracts.CompileRun("documents.schema.json") },
			closed:    func() (*jsonschema.Schema, error) { return contracts.CompileRunClosed("documents.schema.json") },
			read:      contracts.RunDocument,
			list:      runList("fixtures/documents"),
			freePlac:  regexp.MustCompile(`^/(inputs|type_settings|harness/harness/extensions|harness/harness/workflows|harness/harness/bind/(agents|commands|skills))$|^/(inputs|type_settings)/|^/harness/harness/extensions/`),
			valid:     "fixtures/documents/explore.json",
			known:     "/domain",
			openPlace: "/inputs",
			invalid:   func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name: "next-session-answer.schema.json",
			open: func() (*jsonschema.Schema, error) { return contracts.CompileRun("next-session-answer.schema.json") },
			closed: func() (*jsonschema.Schema, error) {
				return contracts.CompileRunClosed("next-session-answer.schema.json")
			},
			read:      contracts.RunDocument,
			list:      runList("fixtures/next-session-answer"),
			freePlac:  regexp.MustCompile(`^/(inputs|type_settings)$|^/(inputs|type_settings)/`),
			valid:     "fixtures/next-session-answer/session.json",
			known:     "/next",
			openPlace: "/inputs",
			invalid:   func(m map[string]any) { m["time_limit_minutes"] = json.Number("0") },
		},
		{
			name: "next-session-request.schema.json",
			open: func() (*jsonschema.Schema, error) { return contracts.CompileRun("next-session-request.schema.json") },
			closed: func() (*jsonschema.Schema, error) {
				return contracts.CompileRunClosed("next-session-request.schema.json")
			},
			read:    contracts.RunDocument,
			list:    runList("fixtures/next-session-request"),
			valid:   "fixtures/next-session-request/succeeded.json",
			known:   "/state",
			invalid: func(m map[string]any) { m["session"] = json.Number("0") },
		},
		{
			name:    "outcome.schema.json",
			open:    func() (*jsonschema.Schema, error) { return contracts.CompileRun("outcome.schema.json") },
			closed:  func() (*jsonschema.Schema, error) { return contracts.CompileRunClosed("outcome.schema.json") },
			read:    contracts.RunDocument,
			list:    runList("fixtures/outcome"),
			valid:   "fixtures/outcome/failed.json",
			known:   "/state",
			invalid: func(m map[string]any) { m["state"] = "paused" },
		},
		{
			name:      "description.schema.json",
			open:      func() (*jsonschema.Schema, error) { return contracts.Compile("description.schema.json") },
			closed:    func() (*jsonschema.Schema, error) { return contracts.CompileClosed("description.schema.json") },
			read:      contracts.Document,
			list:      intList("fixtures"),
			freePlac:  regexp.MustCompile(`^/settings$|^/settings/|^/roles$|^/roles/[^/]+$|^/roles/model_provider/apis/\d+/auth$`),
			valid:     "fixtures/example-tracker.json",
			known:     "/name",
			openPlace: "/settings",
			invalid:   func(m map[string]any) { m["name"] = "Upper Case" },
		},
		{
			name:      "adapter.schema.json",
			open:      func() (*jsonschema.Schema, error) { return contracts.Compile("adapter.schema.json") },
			closed:    func() (*jsonschema.Schema, error) { return contracts.CompileClosed("adapter.schema.json") },
			read:      contracts.Document,
			list:      intList("fixtures/adapter/adapter"),
			freePlac:  regexp.MustCompile(`^/description/settings$|^/description/settings/|^/description/roles$|^/description/roles/[^/]+$|^/description/roles/model_provider/apis/\d+/auth$|^/settings$|^/settings/`),
			valid:     "fixtures/adapter/adapter/acme-tracker.json",
			known:     "/connect_url",
			invalid:   func(m map[string]any) { m["version"] = json.Number("2") },
			openPlace: "/description",
		},
		{
			name:      "changes-submitted.schema.json",
			open:      func() (*jsonschema.Schema, error) { return contracts.Compile("changes-submitted.schema.json") },
			closed:    func() (*jsonschema.Schema, error) { return contracts.CompileClosed("changes-submitted.schema.json") },
			read:      contracts.Document,
			list:      intList("fixtures/adapter/changes-submitted"),
			freePlac:  regexp.MustCompile(`^/data$|^/data/`),
			valid:     "fixtures/adapter/changes-submitted/pull-request-created.json",
			known:     "/action",
			openPlace: "/data",
			invalid:   func(m map[string]any) { m["version"] = json.Number("2") },
		},
		{
			name:    "changes-failed.schema.json",
			open:    func() (*jsonschema.Schema, error) { return contracts.Compile("changes-failed.schema.json") },
			closed:  func() (*jsonschema.Schema, error) { return contracts.CompileClosed("changes-failed.schema.json") },
			read:    contracts.Document,
			list:    intList("fixtures/adapter/changes-failed"),
			valid:   "fixtures/adapter/changes-failed/change-request-refused.json",
			known:   "/reason",
			invalid: func(m map[string]any) { m["version"] = json.Number("2") },
		},
		{
			name:     "changes.schema.json",
			open:     func() (*jsonschema.Schema, error) { return contracts.Compile("changes.schema.json") },
			closed:   func() (*jsonschema.Schema, error) { return contracts.CompileClosed("changes.schema.json") },
			read:     contracts.Document,
			list:     intList("fixtures/adapter/changes"),
			freePlac: regexp.MustCompile(`^/changes/\d+/data$|^/changes/\d+/data/`),
		},
	}
	return append(list,
		integrationUnderstood("adapter-secrets.schema.json", "adapter-secrets/upload.json", "/time", version2, nil),
		integrationUnderstood("adapter-secrets-answer.schema.json", "adapter-secrets-answer/delivery.json", "/revision", version2, nil),
		integrationUnderstood("adapter-secrets-plaintext.schema.json", "adapter-secrets-plaintext/two-secrets.json", "/secrets", version2, nil),
		integrationUnderstood("connect-pass.schema.json", "connect-pass/payload.json", "/nonce", version2, nil),
		integrationUnderstood("connect-return.schema.json", "connect-return/linked.json", "/outcome", version2, nil),
		integrationUnderstood("connections-disconnect.schema.json", "connections-disconnect/acme-tracker.json", "/time", version2, nil),
		integrationUnderstood("grant.schema.json", "grant/linked.json", "/account", version2, nil),
		integrationUnderstood("grant-end.schema.json", "grant-end/ended.json", "/grant_ref", version2, nil),
		integrationUnderstood("grants.schema.json", "grants/two-grants.json", "/grants", version2, nil),
		integrationUnderstood("read.schema.json", "read/at-a-name.json", "/path", version2, nil),
		integrationUnderstood("read-answer.schema.json", "read-answer/at-a-fixed-version.json", "/resolved_at", version2, nil),
		integrationUnderstood("read-public.schema.json", "read-public/at-a-name.json", "/path", version2, nil),
		integrationUnderstood("refusal.schema.json", "refusal/bad-request.json", "/error", func(m map[string]any) { m["error"] = json.Number("7") }, nil),
		integrationUnderstood("resolve.schema.json", "resolve/two-addresses.json", "/addresses", version2, nil),
		integrationUnderstood("resolve-answer.schema.json", "resolve-answer/one-covered.json", "/results", version2, nil),
		integrationUnderstood("run-credential.schema.json", "run-credential/read-harness.json", "/purpose", version2, nil),
		integrationUnderstood("run-credential-answer.schema.json", "run-credential-answer/read.json", "/credential", version2, regexp.MustCompile(`^/credential$|^/credential/apply/\d+$`)),
		integrationUnderstood("setup-pass.schema.json", "setup-pass/payload.json", "/nonce", version2, nil),
		integrationUnderstood("source-collections.schema.json", "source-collections/acme-tracker-part-1.json", "/collections", version2, nil),
		integrationUnderstood("source-item-comments.schema.json", "source-item-comments/acme-tracker-complete.json", "/comments", version2, nil),
		integrationUnderstood("source-items.schema.json", "source-items/acme-tracker-listing-1-part-1.json", "/items", version2, nil),
		integrationUnderstood("sources-disconnect.schema.json", "sources-disconnect/code-host.json", "/collection_ref", version2, nil),
		provisionerUnderstood("claim.schema.json", "claim/with-offers.json", "/wait_seconds", func(m map[string]any) { m["wait_seconds"] = json.Number("26") }),
		provisionerUnderstood("claim-answer.schema.json", "claim-answer/machine.json", "/run_code", func(m map[string]any) { m["revision"] = json.Number("2") }),
		provisionerUnderstood("dispatched.schema.json", "dispatched/with-external.json", "/time", version2),
		provisionerUnderstood("machine-end.schema.json", "machine-end/gone.json", "/state", func(m map[string]any) { m["state"] = "unknown" }),
		provisionerUnderstood("machine-log.schema.json", "machine-log/whole.json", "/log", version2),
		provisionerUnderstood("refusal.schema.json", "refusal/no-gateway.json", "/error", func(m map[string]any) { m["error"] = json.Number("7") }),
		provisionerUnderstood("release.schema.json", "release/no-capacity.json", "/reason", version2),
		machinesUnderstood("check.schema.json", "check/check.json", "/version", version2),
		machinesUnderstood("check-answer.schema.json", "check-answer/check-answer.json", "/offers", func(m map[string]any) { m["offers"] = []any{} }),
		machinesUnderstood("delete.schema.json", "delete/delete.json", "/id", version2),
		machinesUnderstood("delete-answer.schema.json", "delete-answer/gone.json", "/state", func(m map[string]any) { m["state"] = "running" }),
		machinesUnderstood("error.schema.json", "error/defect.json", "/error", func(m map[string]any) { m["error"].(map[string]any)["kind"] = "fatal" }),
		machinesUnderstood("list.schema.json", "list/list.json", "/pool", version2),
		machinesUnderstood("list-answer.schema.json", "list-answer/list-answer.json", "/machines", func(m map[string]any) { m["machines"].([]any)[0].(map[string]any)["state"] = "gone" }),
		machinesUnderstood("log.schema.json", "log/log.json", "/id", version2),
		machinesUnderstood("log-answer.schema.json", "log-answer/log-answer.json", "/log", func(m map[string]any) { m["log"] = "a\x00b" }),
		machinesUnderstood("make.schema.json", "make/make.json", "/dispatch_id", version2),
		exploreUnderstood(),
		machinesUnderstood("make-answer.schema.json", "make-answer/make-answer.json", "/id", func(m map[string]any) { m["id"] = "" }),
	)
}

// integrationUnderstood is the understood entry of a schema of the integration contract
// whose fixtures are in fixtures/adapter/<dir>. free matches the object places that the
// schema leaves to another contract, or nil.
func integrationUnderstood(name, valid, known string, invalid func(m map[string]any), free *regexp.Regexp) understood {
	base := strings.TrimSuffix(name, ".schema.json")
	return understood{
		name:     name,
		open:     func() (*jsonschema.Schema, error) { return contracts.Compile(name) },
		closed:   func() (*jsonschema.Schema, error) { return contracts.CompileClosed(name) },
		read:     contracts.Document,
		list:     func(t *testing.T) []string { return files(t, "fixtures/adapter/"+base) },
		freePlac: free,
		valid:    "fixtures/adapter/" + valid,
		known:    known,
		invalid:  invalid,
	}
}

// provisionerUnderstood is the understood entry of a root schema of the provisioner
// contract.
func provisionerUnderstood(name, valid, known string, invalid func(m map[string]any)) understood {
	base := strings.TrimSuffix(name, ".schema.json")
	return understood{
		name:    name,
		open:    func() (*jsonschema.Schema, error) { return contracts.CompileProvisioner(name) },
		closed:  func() (*jsonschema.Schema, error) { return contracts.CompileProvisionerClosed(name) },
		read:    contracts.ProvisionerDocument,
		list:    func(t *testing.T) []string { return provisionerFiles(t, "fixtures/"+base) },
		valid:   "fixtures/" + valid,
		known:   known,
		invalid: invalid,
	}
}

// machinesUnderstood is the understood entry of a root schema of the machines contract.
func machinesUnderstood(name, valid, known string, invalid func(m map[string]any)) understood {
	base := strings.TrimSuffix(name, ".schema.json")
	return understood{
		name:    name,
		open:    func() (*jsonschema.Schema, error) { return contracts.CompileMachines(name) },
		closed:  func() (*jsonschema.Schema, error) { return contracts.CompileMachinesClosed(name) },
		read:    contracts.MachinesDocument,
		list:    func(t *testing.T) []string { return treeFiles(t, contracts.MachinesFS, "fixtures/"+base) },
		valid:   "fixtures/" + valid,
		known:   known,
		invalid: invalid,
	}
}

// exploreUnderstood is the understood entry of the explore inputs, which have no
// must_understand: the run documents list a member of the inputs below /inputs.
func exploreUnderstood() understood {
	return understood{
		name:   "inputs.schema.json",
		open:   func() (*jsonschema.Schema, error) { return contracts.CompileExplore("inputs.schema.json") },
		closed: func() (*jsonschema.Schema, error) { return contracts.CompileExploreClosed("inputs.schema.json") },
		read:   contracts.ExploreDocument,
		list:   func(t *testing.T) []string { return treeFiles(t, contracts.ExploreFS, "fixtures/inputs") },
	}
}

// version2 sets version to 2, which the schemas refuse.
func version2(m map[string]any) { m["version"] = json.Number("2") }

// objectPlaces returns the JSON pointer of each object in doc, "" for doc itself.
func objectPlaces(doc any, ptr string) []string {
	var out []string
	switch v := doc.(type) {
	case map[string]any:
		out = append(out, ptr)
		for k, x := range v {
			out = append(out, objectPlaces(x, ptr+"/"+strings.NewReplacer("~", "~0", "/", "~1").Replace(k))...)
		}
	case []any:
		for i, x := range v {
			out = append(out, objectPlaces(x, fmt.Sprintf("%s/%d", ptr, i))...)
		}
	}
	return out
}

// objectAtPointer returns the object of doc at the JSON pointer ptr.
func objectAtPointer(t *testing.T, doc any, ptr string) map[string]any {
	t.Helper()
	cur := doc
	for _, seg := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		if seg == "" {
			continue
		}
		seg = strings.NewReplacer("~1", "/", "~0", "~").Replace(seg)
		switch v := cur.(type) {
		case map[string]any:
			cur = v[seg]
		case []any:
			var i int
			fmt.Sscanf(seg, "%d", &i)
			cur = v[i]
		}
	}
	m, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("the document has no object at %q", ptr)
	}
	return m
}

// read reads the valid fixture of the schema.
func (u understood) fixture(t *testing.T) map[string]any {
	t.Helper()
	doc, err := u.read(u.valid)
	if err != nil {
		t.Fatal(err)
	}
	return doc.(map[string]any)
}

// closedSites are the places where CloseSchema closes each schema of the integration and
// run contracts that a closed copy refers to, by file name: the objects that the schema
// leaves open, and the objects of a shared schema, which stay open in every other
// compilation.
var closedSites = map[string][]string{
	"integration/description.schema.json": {
		"", "/$defs/api", "/$defs/api/properties/endpoint/else", "/$defs/api/properties/oauth_device",
		"/$defs/api/properties/oauth_refresh", "/$defs/apply", "/$defs/guide_part",
		"/properties/egress_hosts/items/else", "/properties/roles/properties/credential",
		"/properties/roles/properties/model_provider",
		"/properties/roles/properties/model_provider/properties/models/items",
		"/properties/roles/properties/source",
		"/properties/roles/properties/source/properties/collection_noun",
		"/properties/roles/properties/source/properties/guide",
		"/properties/roles/properties/source/properties/item_noun",
		"/properties/roles/properties/source/properties/setup",
	},
	"integration/adapter.schema.json":                   {"", "/properties/sync"},
	"integration/changes.schema.json":                   {"", "/$defs/change_request", "/$defs/change_request/properties/previous", "/$defs/collection", "/$defs/comment", "/$defs/grant", "/$defs/sync", "/$defs/watch", "/$defs/work_item"},
	"integration/changes-submitted.schema.json":         {""},
	"integration/changes-failed.schema.json":            {""},
	"integration/source-items.schema.json":              {"", "/$defs/item"},
	"integration/adapter-secrets.schema.json":           {"", "/properties/sealed"},
	"integration/adapter-secrets-answer.schema.json":    {"", "/properties/sealed"},
	"integration/adapter-secrets-plaintext.schema.json": {"", "/properties/secrets/items"},
	"integration/connect-pass.schema.json":              {""},
	"integration/connect-return.schema.json":            {""},
	"integration/connections-disconnect.schema.json":    {""},
	"integration/grant.schema.json":                     {"", "/properties/account"},
	"integration/grant-end.schema.json":                 {""},
	"integration/grants.schema.json":                    {"", "/properties/grants/items"},
	"integration/read.schema.json":                      {""},
	"integration/read-answer.schema.json":               {""},
	"integration/read-public.schema.json":               {""},
	"integration/refusal.schema.json":                   {""},
	"integration/resolve.schema.json":                   {""},
	"integration/resolve-answer.schema.json":            {"", "/$defs/result"},
	"integration/run-credential.schema.json":            {""},
	"integration/run-credential-answer.schema.json":     {""},
	"integration/setup-pass.schema.json":                {""},
	"integration/source-collections.schema.json":        {"", "/properties/collections/items"},
	"integration/source-item-comments.schema.json":      {"", "/$defs/comment", "/properties/complete"},
	"integration/sources-disconnect.schema.json":        {""},
	"integration/definitions.schema.json":               {"/$defs/change_address", "/$defs/listing", "/$defs/person", "/$defs/target", "/$defs/target/properties/address"},
	"run/documents.schema.json": {
		"", "/$defs/deliver_entry", "/$defs/harness", "/$defs/harness_base", "/$defs/harness_module", "/$defs/harness_section",
		"/$defs/harness_section/properties/bind", "/$defs/harness_selection", "/$defs/harness_source",
		"/$defs/harness_workflow", "/$defs/model_choice", "/$defs/prebuild_inputs", "/$defs/run_settings",
		"/$defs/run_settings/properties/provider_models/items", "/$defs/run_settings/properties/tiers",
		"/$defs/subject", "/$defs/tier/else",
	},
	"run/next-session-answer.schema.json":  {"/oneOf/0", "/oneOf/1"},
	"run/next-session-request.schema.json": {""},
	"run/outcome.schema.json":              {""},
	"provisioner/claim.schema.json":        {""},
	"provisioner/claim-answer.schema.json": {""},
	"provisioner/dispatched.schema.json":   {"", "/properties/external"},
	"provisioner/machine-end.schema.json":  {""},
	"provisioner/machine-log.schema.json":  {""},
	"provisioner/refusal.schema.json":      {""},
	"provisioner/release.schema.json":      {""},
	"provisioner/definitions.schema.json":  {"/$defs/machine_spec"},
	"machines/check.schema.json":           {""},
	"machines/check-answer.schema.json":    {""},
	"machines/delete.schema.json":          {""},
	"machines/delete-answer.schema.json":   {""},
	"machines/error.schema.json":           {"", "/properties/error"},
	"machines/list.schema.json":            {""},
	"machines/list-answer.schema.json":     {"", "/properties/machines/items"},
	"machines/log.schema.json":             {""},
	"machines/log-answer.schema.json":      {""},
	"machines/make.schema.json":            {""},
	"machines/make-answer.schema.json":     {""},
	"explore/inputs.schema.json":           {"", "/$defs/member", "/properties/comments/items", "/properties/instruction", "/properties/run", "/properties/ticket", "/properties/work_item"},
}

// contractTree is a contract of the repository whose schemas a closed copy can reach: the
// base of its $id, its name in the keys of closedSites, and the function that reads one of
// its files.
type contractTree struct {
	base, short string
	read        func(string) (any, error)
}

var contractTrees = []contractTree{
	{contracts.Base, "integration", contracts.Document},
	{contracts.RunBase, "run", contracts.RunDocument},
	{contracts.ProvisionerBase, "provisioner", contracts.ProvisionerDocument},
	{contracts.MachinesBase, "machines", contracts.MachinesDocument},
	{contracts.ExploreBase, "explore", contracts.ExploreDocument},
}

// siteKey is the key of closedSites for the schema file at the URL: the contract and the file name.
func siteKey(file string) string {
	for _, c := range contractTrees {
		if n, ok := strings.CutPrefix(file, c.base+"/"); ok {
			return c.short + "/" + n
		}
	}
	return file
}

// reachableFiles returns the files of the contracts of contractTrees that the schema at
// url refers to by $ref, directly or through another file, the file itself included.
func reachableFiles(t *testing.T, url string, seen map[string]bool) {
	t.Helper()
	file, _, _ := strings.Cut(url, "#")
	if seen[file] {
		return
	}
	var doc any
	var err error
	found := false
	for _, c := range contractTrees {
		if n, ok := strings.CutPrefix(file, c.base+"/"); ok {
			found = true
			doc, err = c.read(n)
			break
		}
	}
	if !found {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	seen[file] = true
	var walk func(n any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for k, x := range v {
				if ref, ok := x.(string); ok && k == "$ref" {
					target := ref
					if strings.HasPrefix(ref, "#") {
						target = file + ref
					} else if !strings.Contains(ref, "://") {
						target = file[:strings.LastIndex(file, "/")+1] + ref
					}
					reachableFiles(t, target, seen)
				} else {
					walk(x)
				}
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(doc)
}

// TestAClosedCopyClosesTheSitesOfItsSchema pins the places where CloseSchema closes each
// schema of the integration and run contracts that a closed copy compiles, and that the
// closed copies of the schemas reach no other file of those contracts.
func TestAClosedCopyClosesTheSitesOfItsSchema(t *testing.T) {
	reached := map[string]bool{}
	for _, name := range []string{"description", "adapter", "changes", "changes-submitted", "changes-failed", "source-items", "adapter-secrets", "adapter-secrets-answer", "adapter-secrets-plaintext", "connect-pass", "connect-return", "connections-disconnect", "grant", "grant-end", "grants", "read", "read-answer", "read-public", "refusal", "resolve", "resolve-answer", "run-credential", "run-credential-answer", "setup-pass", "source-collections", "source-item-comments", "sources-disconnect"} {
		reachableFiles(t, contracts.Base+"/"+name+".schema.json", reached)
	}
	for _, name := range []string{"claim", "claim-answer", "dispatched", "machine-end", "machine-log", "refusal", "release"} {
		reachableFiles(t, contracts.ProvisionerBase+"/"+name+".schema.json", reached)
	}
	reachableFiles(t, contracts.ExploreBase+"/inputs.schema.json", reached)
	for _, name := range []string{"check", "check-answer", "delete", "delete-answer", "error", "list", "list-answer", "log", "log-answer", "make", "make-answer"} {
		reachableFiles(t, contracts.MachinesBase+"/"+name+".schema.json", reached)
	}
	for _, name := range []string{"documents", "next-session-answer", "next-session-request", "outcome"} {
		reachableFiles(t, contracts.RunBase+"/"+name+".schema.json", reached)
	}
	var names []string
	for file := range reached {
		names = append(names, siteKey(file))
	}
	slices.Sort(names)
	var pinned []string
	for name := range closedSites {
		pinned = append(pinned, name)
	}
	slices.Sort(pinned)
	if !slices.Equal(names, pinned) {
		t.Errorf("the closed copies compile the files %q, and the test pins the sites of %q", names, pinned)
	}
	for file := range reached {
		var doc any
		var err error
		for _, c := range contractTrees {
			if n, ok := strings.CutPrefix(file, c.base+"/"); ok {
				doc, err = c.read(n)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		_, sites := contracts.CloseSchema(doc)
		slices.Sort(sites)
		if want := closedSites[siteKey(file)]; !slices.Equal(sites, want) && (len(sites) != 0 || len(want) != 0) {
			t.Errorf("%s: the closed copy closes %q, want %q", siteKey(file), sites, want)
		}
	}
}

// TestAReaderIgnoresAMemberItDoesNotDefine pins, for each schema, the rule of
// must_understand. In each object of each valid fixture, a member that the schema does
// not define is accepted, and the closed copy refuses it, so a misspelled member fails a
// writer's test. The places that define no members are the exception, as freePlac lists them.
func TestAReaderIgnoresAMemberItDoesNotDefine(t *testing.T) {
	for _, u := range understoodSchemas() {
		open, err := u.open()
		if err != nil {
			t.Fatal(err)
		}
		closed, err := u.closed()
		if err != nil {
			t.Fatal(err)
		}
		checked := 0
		for _, f := range u.list(t) {
			raw, err := u.read(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, place := range objectPlaces(raw, "") {
				if (u.freePlac != nil && u.freePlac.MatchString(place)) || (u.only != nil && !u.only.MatchString(place)) {
					continue
				}
				doc, _ := u.read(f)
				m := objectAtPointer(t, doc, place)
				if kind, ok := m["kind"].(string); ok && strings.HasPrefix(place, "/changes/") && !slices.Contains(feedKinds, kind) {
					continue
				}
				m["extra_member"] = "x"
				if err := open.Validate(doc); err != nil {
					t.Errorf("%s %s: a member of %q that the schema does not define is refused: %v", u.name, f, place, err)
				}
				want := place + " additionalProperties"
				if place == "" {
					want = "/ additionalProperties"
				}
				if got := runRefusal(closed.Validate(doc)); !strings.Contains(got, want) {
					t.Errorf("%s %s: the closed copy refuses a member of %q for %q, want %q", u.name, f, place, got, want)
				}
				checked++
			}
		}
		if checked == 0 {
			t.Errorf("%s: no fixture has an object", u.name)
		}
		if u.valid == "" {
			continue
		}
		doc := u.fixture(t)
		doc["extra_member"] = "x"
		doc["must_understand"] = []any{"/extra_member"}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 pattern"; !strings.Contains(got, want) {
			t.Errorf("%s: a member that must_understand lists is refused for %q, want %q", u.name, got, want)
		}
		doc = u.fixture(t)
		doc["must_understand"] = []any{u.known}
		for _, s := range []*jsonschema.Schema{open, closed} {
			if err := s.Validate(doc); err != nil {
				t.Errorf("%s: must_understand that lists a member of the schema is refused: %v", u.name, err)
			}
		}
		doc["must_understand"] = []any{u.known, u.known}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand uniqueItems"; !strings.Contains(got, want) {
			t.Errorf("%s: a member listed twice is refused for %q, want %q", u.name, got, want)
		}
		doc["must_understand"] = []any{"/" + strings.Repeat("a", 256)}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 "; !strings.Contains(got, want) {
			t.Errorf("%s: a pointer of 257 characters is refused for %q, want %q", u.name, got, want)
		}
		doc["must_understand"] = []any{u.known + "\n"}
		if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 "; !strings.Contains(got, want) {
			t.Errorf("%s: a pointer with a line feed is refused for %q, want %q", u.name, got, want)
		}
		if u.openPlace != "" {
			// An open place matches the pattern with a line feed in it, so only the not
			// of the list refuses the pointer.
			doc["must_understand"] = []any{u.openPlace + "/a\nb"}
			if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 "; !strings.Contains(got, want) || strings.Contains(got, want+"pattern") {
				t.Errorf("%s: a pointer below %s with a line feed is refused for %q, want %q alone", u.name, u.openPlace, got, want)
			}
			doc["must_understand"] = []any{u.openPlace + "//a~01b/x"}
			if err := open.Validate(doc); err != nil {
				t.Errorf("%s: a pointer below %s is refused: %v", u.name, u.openPlace, err)
			}
			doc["must_understand"] = []any{u.openPlace + "/a~2b"}
			if got, want := runRefusal(open.Validate(doc)), "/must_understand/0 pattern"; !strings.Contains(got, want) {
				t.Errorf("%s: a pointer below %s with ~2 is refused for %q, want %q", u.name, u.openPlace, got, want)
			}
		}
		delete(doc, "must_understand")
		u.invalid(doc)
		if err := open.Validate(doc); err == nil {
			t.Errorf("%s: a bad value of a member that the schema defines is accepted", u.name)
		}
		doc = u.fixture(t)
		doc["must_understand"] = "extra_member"
		if err := open.Validate(doc); err == nil {
			t.Errorf("%s: a must_understand that is not a list is accepted", u.name)
		}
	}
}

// feedKinds are the kinds of entry that changes.schema.json defines.
var feedKinds = []string{"work_item", "comment", "collection", "grant", "sync", "watch", "change_request"}

// TestMustUnderstandListsThePointersOfItsSchema pins that the pattern of each pointer that
// must_understand accepts is the one built from the schema: the members that the schema
// defines, at any depth, from the object that has the list. It pins too that the Go type
// of the document resolves every such pointer.
func TestMustUnderstandListsThePointersOfItsSchema(t *testing.T) {
	for _, c := range []struct {
		name string
		base string
		read func(string) (any, error)
		open []string
		goes any
	}{
		{"documents.schema.json", contracts.RunBase, contracts.RunDocument, []string{"/inputs", "/type_settings", "/harness/harness/extensions"}, contracts.Documents{}},
		{"next-session-answer.schema.json", contracts.RunBase, contracts.RunDocument, []string{"/inputs", "/type_settings"}, contracts.NextSessionAnswer{}},
		{"description.schema.json", contracts.Base, contracts.Document, []string{"/settings"}, nil},
		{"next-session-request.schema.json", contracts.RunBase, contracts.RunDocument, nil, contracts.NextSessionRequest{}},
		{"outcome.schema.json", contracts.RunBase, contracts.RunDocument, nil, contracts.RunOutcome{}},
		{"claim.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.Claim{}},
		{"claim-answer.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.ClaimAnswer{}},
		{"dispatched.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.Dispatched{}},
		{"machine-end.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.MachineEnd{}},
		{"machine-log.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.MachineLog{}},
		{"refusal.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, nil},
		{"release.schema.json", contracts.ProvisionerBase, contracts.ProvisionerDocument, nil, contracts.Release{}},
		{"check.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesCheck{}},
		{"check-answer.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesCheckAnswer{}},
		{"delete.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesDelete{}},
		{"delete-answer.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesDeleteAnswer{}},
		{"error.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesError{}},
		{"list.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesList{}},
		{"list-answer.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesListAnswer{}},
		{"log.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesLog{}},
		{"log-answer.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesLogAnswer{}},
		{"make.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesMake{}},
		{"make-answer.schema.json", contracts.MachinesBase, contracts.MachinesDocument, nil, contracts.MachinesMakeAnswer{}},
		{"changes-submitted.schema.json", contracts.Base, contracts.Document, []string{"/data"}, contracts.ChangesSubmitted{}},
		{"changes-failed.schema.json", contracts.Base, contracts.Document, nil, contracts.ChangesFailed{}},
		{"adapter.schema.json", contracts.Base, contracts.Document, []string{"/description"}, contracts.Adapter{}},
		{"adapter-secrets.schema.json", contracts.Base, contracts.Document, nil, contracts.AdapterSecrets{}},
		{"adapter-secrets-answer.schema.json", contracts.Base, contracts.Document, nil, contracts.AdapterSecretsAnswer{}},
		{"adapter-secrets-plaintext.schema.json", contracts.Base, contracts.Document, nil, contracts.AdapterSecretsPlaintext{}},
		{"connect-pass.schema.json", contracts.Base, contracts.Document, nil, contracts.ConnectPass{}},
		{"connect-return.schema.json", contracts.Base, contracts.Document, nil, contracts.ConnectReturn{}},
		{"connections-disconnect.schema.json", contracts.Base, contracts.Document, nil, contracts.ConnectionsDisconnect{}},
		{"grant.schema.json", contracts.Base, contracts.Document, nil, contracts.Grant{}},
		{"grant-end.schema.json", contracts.Base, contracts.Document, nil, contracts.GrantEnd{}},
		{"grants.schema.json", contracts.Base, contracts.Document, nil, contracts.Grants{}},
		{"read.schema.json", contracts.Base, contracts.Document, nil, contracts.Read{}},
		{"read-answer.schema.json", contracts.Base, contracts.Document, nil, contracts.ReadAnswer{}},
		{"read-public.schema.json", contracts.Base, contracts.Document, nil, contracts.ReadPublic{}},
		{"refusal.schema.json", contracts.Base, contracts.Document, nil, contracts.Refusal{}},
		{"resolve.schema.json", contracts.Base, contracts.Document, nil, contracts.Resolve{}},
		{"resolve-answer.schema.json", contracts.Base, contracts.Document, nil, contracts.ResolveAnswer{}},
		{"run-credential.schema.json", contracts.Base, contracts.Document, nil, contracts.RunCredential{}},
		{"run-credential-answer.schema.json", contracts.Base, contracts.Document, nil, contracts.RunCredentialAnswer{}},
		{"setup-pass.schema.json", contracts.Base, contracts.Document, nil, contracts.SetupPass{}},
		{"source-collections.schema.json", contracts.Base, contracts.Document, nil, contracts.SourceCollections{}},
		{"source-item-comments.schema.json", contracts.Base, contracts.Document, nil, contracts.SourceItemComments{}},
		{"source-items.schema.json", contracts.Base, contracts.Document, nil, contracts.SourceItems{}},
		{"sources-disconnect.schema.json", contracts.Base, contracts.Document, nil, contracts.SourcesDisconnect{}},
	} {
		raw, err := c.read(c.name)
		if err != nil {
			t.Fatal(err)
		}
		defs, _ := raw.(map[string]any)["$defs"].(map[string]any)
		def, ok := defs["must_understand"].(map[string]any)
		if !ok {
			def = raw.(map[string]any)["properties"].(map[string]any)["must_understand"].(map[string]any)
		}
		templates := definedPointers(c.base+"/"+c.name, raw, c.open...)
		if got, want := def["items"].(map[string]any)["pattern"], pointerPattern(templates); got != want {
			t.Errorf("%s: the pattern of must_understand is not the one built from the schema:\n got %v\nwant %v", c.name, got, want)
		}
		if c.goes == nil {
			continue
		}
		if !hasMustUnderstandField(reflect.TypeOf(c.goes)) {
			t.Errorf("%s: the Go type %T has no field with the json name must_understand", c.name, c.goes)
		}
		for _, tpl := range templates {
			p := strings.NewReplacer("/**", "/0/kind", "/#", "/0", "/*", "/k").Replace(tpl)
			doc := fmt.Sprintf(`{"must_understand": [%q]}`, p)
			if err := contracts.CheckMustUnderstand([]byte(doc), c.goes); err != nil {
				t.Errorf("%s: the Go type %T does not resolve %s: %v", c.name, c.goes, p, err)
			}
		}
	}
}

// hasMustUnderstandField reports whether the struct type t has a field with the json name
// must_understand.
func hasMustUnderstandField(t reflect.Type) bool {
	for i := range t.NumField() {
		if name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ","); name == contracts.MustUnderstand {
			return true
		}
	}
	return false
}

// TestAnEntryListsThePointersOfItsKind pins, for each kind of entry, that the pattern of
// the must_understand of the entry is the one built from the kind's schema, that the Go
// type of the kind resolves each pointer of it, and that the pattern refuses a pointer to a
// member of another kind.
func TestAnEntryListsThePointersOfItsKind(t *testing.T) {
	raw, err := contracts.Document("changes.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	defs := raw.(map[string]any)["$defs"].(map[string]any)
	for _, kind := range feedKinds {
		def := defs[kind].(map[string]any)
		mu := def["properties"].(map[string]any)["must_understand"].(map[string]any)
		pattern := mu["items"].(map[string]any)["pattern"].(string)
		templates := definedPointers(contracts.Base+"/changes.schema.json", def, "/data")
		if want := pointerPattern(templates); pattern != want {
			t.Errorf("%s: the pattern of must_understand is not the one built from the schema:\n got %v\nwant %v", kind, pattern, want)
		}
		for _, tpl := range templates {
			p := strings.NewReplacer("/**", "/x/y", "/#", "/0", "/*", "/k").Replace(tpl)
			entry := fmt.Sprintf(`{"kind": %q, "must_understand": [%q]}`, kind, p)
			if err := contracts.CheckChangeMustUnderstand([]byte(entry)); err != nil {
				t.Errorf("%s: the Go type does not resolve %s: %v", kind, p, err)
			}
		}
		other := "/work_item_id"
		if kind == "work_item" || kind == "watch" {
			other = "/comment_id"
		}
		if regexp.MustCompile(pattern).MatchString(other) {
			t.Errorf("%s: the pattern of must_understand accepts %s", kind, other)
		}
		entry := fmt.Sprintf(`{"kind": %q, "must_understand": [%q]}`, kind, other)
		if err := contracts.CheckChangeMustUnderstand([]byte(entry)); err == nil {
			t.Errorf("%s: CheckChangeMustUnderstand accepts %s", kind, other)
		}
	}
}

// TestEveryValidFixtureValidatesAgainstTheClosedCopy pins that each valid fixture of the
// schemas that a reader reads leniently has no member that the schema does not define:
// the closed copy accepts it, and refuses it with one added member, which a writer's test
// finds when it misspells a member.
func TestEveryValidFixtureValidatesAgainstTheClosedCopy(t *testing.T) {
	for _, u := range understoodSchemas() {
		closed, err := u.closed()
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range u.list(t) {
			doc, err := u.read(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := closed.Validate(doc); err != nil {
				t.Errorf("%s: the closed copy of %s refuses it: %v", f, u.name, err)
			}
		}
	}
}

// TestCheckMustUnderstand pins how a Go reader applies the rule: a document whose
// must_understand lists only members of the type passes, whatever other members it has,
// and a document that lists a member the type does not have, or lists it as a non-string,
// fails.
func TestCheckMustUnderstand(t *testing.T) {
	for _, c := range []struct {
		doc  string
		v    any
		want string
	}{
		{`{"step": "workflow"}`, contracts.Documents{}, ""},
		{`{"extra_member": 1}`, &contracts.Documents{}, ""},
		{`{"must_understand": ["/step", "/subject/kind"], "extra_member": 1}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/run_settings/runtime", "/harness/harness/modules/0/name"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/inputs/run/attempt", "/harness/harness/extensions/a~1b"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/step", "/extra_member"], "extra_member": 1}`, contracts.Documents{}, `use a reader that defines "/extra_member"`},
		{`{"must_understand": ["/subject/language"]}`, contracts.Documents{}, `use a reader that defines "/subject/language"`},
		{`{"must_understand": ["/harness/harness/modules/01/name"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/step/kind"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/must_understand"]}`, contracts.Documents{}, `use a reader that defines "/must_understand"`},
		{`{"must_understand": ["step"]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": [""]}`, contracts.Documents{}, "starts with /"},
		{`{"must_understand": ["/next", "/inputs"]}`, contracts.NextSessionAnswer{}, ""},
		{`{"must_understand": ["/step"]}`, contracts.NextSessionAnswer{}, `use a reader that defines "/step"`},
		{`{"must_understand": "step"}`, contracts.Documents{}, "must_understand to a list"},
		{`{"must_understand": null}`, contracts.Documents{}, "list of JSON Pointers"},
		{`{"must_understand": ["/step", "/step"]}`, contracts.Documents{}, `list "/step" once`},
		{`{"must_understand": ["/run_settings/tiers/light/model", "/run_settings/tiers/light/effort"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/run_settings/tiers/light/bogus"]}`, contracts.Documents{}, `use a reader that defines "/run_settings/tiers/light/bogus"`},
		{`{"must_understand": ["/harness/harness/bind/agents/writer/0"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/harness/harness/bind/agents/writer/name"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/harness/harness/bind/hooks/writer"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/harness/harness/modules/0/only/skills/0"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/harness/harness/modules/0/only/skills/all"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": [` + strings.TrimSuffix(strings.Repeat(`"/step",`, 33), ",") + `]}`, contracts.Documents{}, "at most 32 pointers"},
		{`{"must_understand": ["/` + strings.Repeat("a", 256) + `"]}`, contracts.Documents{}, "at most 256 characters"},
		{`{"must_understand": []}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/harness/harness/workflows//access"]}`, contracts.Documents{}, "use a reader that defines"},
		{`{"must_understand": ["/harness/harness/workflows/a~2b/access"]}`, contracts.Documents{}, "with ~ only as ~0 or ~1"},
		{`{"must_understand": ["/inputs/a~2b"]}`, contracts.Documents{}, "with ~ only as ~0 or ~1"},
		{`{"must_understand": ["/inputs/a~"]}`, contracts.Documents{}, "with ~ only as ~0 or ~1"},
		{`{"must_understand": ["/inputs/a~01b", "/inputs//x"]}`, contracts.Documents{}, ""},
		{`{"must_understand": ["/domain\n"]}`, contracts.Documents{}, "without a line break"},
		{`{"must_understand": ["/inputs/a\nb"]}`, contracts.Documents{}, "without a line break"},
		{`[]`, contracts.Documents{}, "JSON object"},
	} {
		err := contracts.CheckMustUnderstand([]byte(c.doc), c.v)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: %v", c.doc, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%s: got %v, want an error with %q", c.doc, err, c.want)
		}
	}
}

// TestMustUnderstandIsWrittenBack pins that a NextSessionAnswer and Documents write their
// MustUnderstand, and that the document they write passes the schema.
func TestMustUnderstandIsWrittenBack(t *testing.T) {
	answerSchema := compileRun(t, "next-session-answer.schema.json")
	for _, name := range []string{"end.json", "session.json"} {
		b, err := fs.ReadFile(contracts.RunFS, "fixtures/next-session-answer/"+name)
		if err != nil {
			t.Fatal(err)
		}
		var a contracts.NextSessionAnswer
		if err := json.Unmarshal(b, &a); err != nil {
			t.Fatal(err)
		}
		a.MustUnderstand = []string{"/next"}
		out, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(out), `"must_understand":["/next"]`) {
			t.Errorf("%s is written as %s, without must_understand", name, out)
		}
		if err := validate(answerSchema, out); err != nil {
			t.Errorf("%s with must_understand: %v", name, err)
		}
	}
	b, err := fs.ReadFile(contracts.RunFS, "fixtures/documents/explore.json")
	if err != nil {
		t.Fatal(err)
	}
	var d contracts.Documents
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	d.MustUnderstand = []string{"/subject/kind"}
	out, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(compileRun(t, runSchema("documents")), out); err != nil {
		t.Errorf("the documents with must_understand: %v", err)
	}
	feed := contracts.Changes{Version: 1, Revision: 1, Cursor: "c_1", Changes: []contracts.Change{
		{Kind: contracts.ChangeSync, MustUnderstand: []string{"/interval"}},
		{Kind: contracts.ChangeWatch, WorkItemID: "wi_1", URL: "https://x.example"},
	}}
	out, err = json.Marshal(feed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"must_understand":["/interval"]`) || strings.Count(string(out), "must_understand") != 1 {
		t.Errorf("the feed is written as %s, want must_understand in the first entry only", out)
	}
	if err := validate(compile(t, "changes.schema.json"), out); err != nil {
		t.Errorf("the feed with must_understand: %v", err)
	}
}

// TestAnAdapterSkipsAnEntryOfAnUnknownKind pins the feed rule: changes.schema.json accepts
// an entry of a kind that it does not define, with any members, so an adapter reads its
// kind, skips it and moves on; it refuses a kind that is not snake_case; and an entry of a
// defined kind ignores a member that it does not define, as the rule of must_understand says,
// and keeps its rules for the members that it defines.
func TestAnAdapterSkipsAnEntryOfAnUnknownKind(t *testing.T) {
	schema := compile(t, "changes.schema.json")
	feed := func(entries string) []byte {
		return []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": ` + entries + `}`)
	}
	sync := `{"kind": "sync", "interval": 600}`
	for _, c := range []struct {
		name, entries string
		accepted      bool
	}{
		{"an unknown kind with members", `[{"kind": "settings", "interval": 600, "extra": {"a": 1}}, ` + sync + `]`, true},
		{"an unknown kind alone", `[{"kind": "settings"}]`, true},
		{"an unknown kind with must_understand", `[{"kind": "settings", "must_understand": ["/extra"]}]`, true},
		{"a kind in upper case", `[{"kind": "Settings"}]`, false},
		{"a kind of 64 characters", `[{"kind": "` + strings.Repeat("a", 64) + `"}]`, true},
		{"a kind of 65 characters", `[{"kind": "` + strings.Repeat("a", 65) + `"}]`, false},
		{"a kind that is not a string", `[{"kind": 7}]`, false},
		{"no kind", `[{"interval": 600}]`, false},
		{"a defined kind with a member it does not define", `[{"kind": "sync", "interval": 600, "extra": 1}]`, true},
		{"a defined kind with a bad value", `[{"kind": "sync", "interval": 30}]`, false},
		{"a defined kind without a member it requires", `[{"kind": "sync"}]`, false},
	} {
		if err := validate(schema, feed(c.entries)); (err == nil) != c.accepted {
			t.Errorf("%s: accepted %v, want %v: %v", c.name, err == nil, c.accepted, err)
		}
	}
}

// TestAnEntryListsWhatTheAdapterMustUnderstand pins the rule of must_understand for the
// feed: an entry, and the answer, ignore a member that they list nowhere; an entry whose
// must_understand lists a pointer that its kind defines passes; a pointer that its kind does
// not define fails the schema; below data of a change_request entry, any pointer passes the
// schema; and CheckChangeMustUnderstand returns an error for an entry that the adapter cannot
// meet, with a message that starts with the fix.
func TestAnEntryListsWhatTheAdapterMustUnderstand(t *testing.T) {
	schema := compile(t, "changes.schema.json")
	feed := func(entry string) []byte {
		return []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [` + entry + `]}`)
	}
	changeRequest := func(mu string) string {
		return `{"kind": "change_request", "change_request_id": "av_0123456789abcdef", "change_kind": "pull-request", ` +
			`"target": {"collection": "R_1001"}, "title": "t", "text": "x", "data": {"a": 1}, "must_understand": ` + mu + `}`
	}
	for _, c := range []struct {
		name, entry string
		schemaOK    bool
		checkErr    string
	}{
		{"a listed member of the kind", `{"kind": "sync", "interval": 600, "must_understand": ["/interval"]}`, true, ""},
		{"a listed member of another kind", `{"kind": "sync", "interval": 600, "must_understand": ["/url"]}`, false, `Use an adapter that defines "/url"`},
		{"a member that the kind does not define, not listed", `{"kind": "sync", "interval": 600, "extra": 1}`, true, ""},
		{"an empty list", `{"kind": "sync", "interval": 600, "must_understand": []}`, true, ""},
		{"a list that is null", `{"kind": "sync", "interval": 600, "must_understand": null}`, false, "not null"},
		{"a member listed twice", `{"kind": "sync", "interval": 600, "must_understand": ["/kind", "/kind"]}`, false, `list "/kind" once`},
		{"a name, not a pointer", `{"kind": "sync", "interval": 600, "must_understand": ["interval"]}`, false, "starts with /"},
		{"must_understand itself", `{"kind": "sync", "interval": 600, "must_understand": ["/must_understand"]}`, false, `Use an adapter that defines "/must_understand"`},
		{"a nested member of a change request", changeRequest(`["/target/address/path", "/previous/title"]`), true, ""},
		{"a member of data", changeRequest(`["/data/a"]`), true, ""},
		{"a member of data that the entry lacks", changeRequest(`["/data/extra/deep"]`), true, ""},
		{"a member of the target that it does not define", changeRequest(`["/target/extra"]`), false, `Use an adapter that defines "/target/extra"`},
		{"a member of a work item in data", `{"kind": "work_item", "work_item_id": "wi_1", "ref": "r", "status": "taken_in", "url": "https://x.example", "must_understand": ["/data/a"]}`, false, `Use an adapter that defines "/data/a"`},
	} {
		if err := validate(schema, feed(c.entry)); (err == nil) != c.schemaOK {
			t.Errorf("%s: the schema accepts it: %v, want %v: %v", c.name, err == nil, c.schemaOK, err)
		}
		err := contracts.CheckChangeMustUnderstand([]byte(c.entry))
		switch {
		case c.checkErr == "" && err != nil:
			t.Errorf("%s: CheckChangeMustUnderstand: %v", c.name, err)
		case c.checkErr != "" && (err == nil || !strings.Contains(err.Error(), c.checkErr)):
			t.Errorf("%s: CheckChangeMustUnderstand: got %v, want an error with %q", c.name, err, c.checkErr)
		}
	}
	if err := contracts.CheckChangeMustUnderstand([]byte(`{"kind": "settings", "must_understand": ["/extra"]}`)); err == nil || !strings.HasPrefix(err.Error(), "skip the entry and acknowledge it") {
		t.Errorf("an entry of an unknown kind: %v, want an error that starts with skip the entry and acknowledge it", err)
	}
	// The failure report of an entry that the adapter cannot meet has the text of the error as
	// its detail, and the schema accepts it.
	entry := changeRequest(`["/target/extra"]`)
	err := contracts.CheckChangeMustUnderstand([]byte(entry))
	report, rerr := contracts.Document("fixtures/adapter/changes-failed/change-request-must-understand.json")
	if rerr != nil || err == nil {
		t.Fatalf("the fixture: %v, the check: %v", rerr, err)
	}
	if got := report.(map[string]any)["detail"]; got != err.Error() {
		t.Errorf("the detail of the fixture is %q, and CheckChangeMustUnderstand returns %q", got, err)
	}
	if got := report.(map[string]any)["reason"]; got != "refused" {
		t.Errorf("the reason of the fixture is %v, want refused", got)
	}
	// The answer has no must_understand: it is a member that the schema does not define, so
	// a reader ignores it, whatever it lists, and the closed copy refuses it.
	answer := []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [], "must_understand": ["/extra"]}`)
	if err := validate(schema, answer); err != nil {
		t.Errorf("an answer with a must_understand: %v", err)
	}
	closedFeed, err := contracts.CompileClosed("changes.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if got := runRefusal(validate(closedFeed, answer)); got != "/ additionalProperties" {
		t.Errorf("the closed copy refuses an answer with a must_understand for %q, want %q", got, "/ additionalProperties")
	}
	if err := validate(schema, []byte(`{"version": 1, "revision": 1, "cursor": "c_2", "changes": [], "extra": 1}`)); err != nil {
		t.Errorf("an answer with a member that the schema does not define: %v", err)
	}
}

// TestOnlyTheClosableSchemasHaveAClosedCopy pins that CompileClosed and CompileRunClosed
// return an error for a schema other than the ones that a reader reads leniently: the
// closed copy of a schema that lists only some members of an object, such as the
// next-session refusal, refuses members that it defines.
func TestOnlyTheClosableSchemasHaveAClosedCopy(t *testing.T) {
	for _, name := range []string{"description.schema.json", "adapter.schema.json", "changes.schema.json",
		"changes-submitted.schema.json", "changes-failed.schema.json", "source-items.schema.json", "definitions.schema.json",
		"read.schema.json", "source-collections.schema.json", "description.schema.json#/$defs/api"} {
		_, err := contracts.CompileClosed(name)
		want := !strings.HasPrefix(name, "definitions") && !strings.Contains(name, "#")
		if (err == nil) != want {
			t.Errorf("CompileClosed(%q): %v", name, err)
		}
	}
	for _, name := range []string{"documents.schema.json", "next-session-answer.schema.json",
		"next-session-request.schema.json", "outcome.schema.json",
		"next-session-refusal.schema.json", "documents.schema.json#/$defs/subject"} {
		_, err := contracts.CompileRunClosed(name)
		want := name != "next-session-refusal.schema.json" && !strings.Contains(name, "#")
		if (err == nil) != want {
			t.Errorf("CompileRunClosed(%q): %v", name, err)
		}
	}
}

// TestAReaderDecodesEveryClosedFixture pins that the Go types read each document that the
// schema accepts and the closed copy refuses: a reader ignores the undefined member, and
// does not fail on it.
func TestAReaderDecodesEveryClosedFixture(t *testing.T) {
	for _, f := range runFiles(t, "fixtures/closed") {
		b, err := fs.ReadFile(contracts.RunFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var v any = &contracts.NextSessionAnswer{}
		switch name := path.Base(f); {
		case strings.HasPrefix(name, "documents-"):
			v = &contracts.Documents{}
		case strings.HasPrefix(name, "next-session-request-"):
			v = &contracts.NextSessionRequest{}
		case strings.HasPrefix(name, "outcome-"):
			v = &contracts.RunOutcome{}
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range treeFiles(t, contracts.ExploreFS, "fixtures/closed") {
		b, err := fs.ReadFile(contracts.ExploreFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		if err := json.Unmarshal(b, &v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range treeFiles(t, contracts.MachinesFS, "fixtures/closed") {
		b, err := fs.ReadFile(contracts.MachinesFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		switch name := path.Base(f); {
		case strings.HasPrefix(name, "check-answer-"):
			v = &contracts.MachinesCheckAnswer{}
		case strings.HasPrefix(name, "check-"):
			v = &contracts.MachinesCheck{}
		case strings.HasPrefix(name, "delete-answer-"):
			v = &contracts.MachinesDeleteAnswer{}
		case strings.HasPrefix(name, "delete-"):
			v = &contracts.MachinesDelete{}
		case strings.HasPrefix(name, "error-"):
			v = &contracts.MachinesError{}
		case strings.HasPrefix(name, "list-answer-"):
			v = &contracts.MachinesListAnswer{}
		case strings.HasPrefix(name, "list-"):
			v = &contracts.MachinesList{}
		case strings.HasPrefix(name, "log-answer-"):
			v = &contracts.MachinesLogAnswer{}
		case strings.HasPrefix(name, "log-"):
			v = &contracts.MachinesLog{}
		case strings.HasPrefix(name, "make-answer-"):
			v = &contracts.MachinesMakeAnswer{}
		case strings.HasPrefix(name, "make-"):
			v = &contracts.MachinesMake{}
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range treeFiles(t, contracts.ProvisionerFS, "fixtures/closed") {
		b, err := fs.ReadFile(contracts.ProvisionerFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var v any
		switch name := path.Base(f); {
		case strings.HasPrefix(name, "claim-answer-"):
			v = &contracts.ClaimAnswer{}
		case strings.HasPrefix(name, "claim-"):
			v = &contracts.Claim{}
		case strings.HasPrefix(name, "dispatched-"):
			v = &contracts.Dispatched{}
		case strings.HasPrefix(name, "machine-end-"):
			v = &contracts.MachineEnd{}
		case strings.HasPrefix(name, "machine-log-"):
			v = &contracts.MachineLog{}
		case strings.HasPrefix(name, "release-"):
			v = &contracts.Release{}
		default:
			continue
		}
		if err := json.Unmarshal(b, v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
	for _, f := range treeFiles(t, contracts.FS, "fixtures/adapter/closed") {
		b, err := fs.ReadFile(contracts.FS, f)
		if err != nil {
			t.Fatal(err)
		}
		v := adapterClosedType(path.Base(f))
		if err := json.Unmarshal(b, v); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// TestEveryMustUnderstandListRefusesALineFeed pins, for each must_understand list of each
// schema of the contracts, that an item has at most 256 characters, matches a pattern that
// starts with ^ and ends with $, and has a not that refuses a line feed. A regular
// expression engine that lets $ match before a final line feed, such as PCRE, would
// accept "/domain" followed by a line feed without the not.
func TestEveryMustUnderstandListRefusesALineFeed(t *testing.T) {
	trees := map[string]fs.FS{"integration": contracts.FS, "run": contracts.RunFS, "provisioner": contracts.ProvisionerFS,
		"machines": contracts.MachinesFS, "explore": contracts.ExploreFS, "diagnostics": contracts.DiagnosticsFS}
	checked := 0
	for name, fsys := range trees {
		entries, err := fs.ReadDir(fsys, ".")
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".schema.json") {
				continue
			}
			doc, err := document(fsys, e.Name())
			if err != nil {
				t.Fatal(err)
			}
			var walk func(n any)
			walk = func(n any) {
				switch v := n.(type) {
				case map[string]any:
					for k, x := range v {
						list, ok := x.(map[string]any)
						if k != "must_understand" || !ok {
							walk(x)
							continue
						}
						if _, ok := list["$ref"]; ok {
							continue
						}
						checked++
						item, _ := list["items"].(map[string]any)
						pattern, _ := item["pattern"].(string)
						not, _ := item["not"].(map[string]any)
						if item["maxLength"] != json.Number("256") || !strings.HasPrefix(pattern, "^") || !strings.HasSuffix(pattern, "$") || not["pattern"] != `\n` {
							t.Errorf("%s/%s: an item of must_understand is %v, want maxLength 256, a pattern from ^ to $ and not pattern \\n", name, e.Name(), item)
						}
					}
				case []any:
					for _, x := range v {
						walk(x)
					}
				}
			}
			walk(doc)
		}
	}
	if checked < 10 {
		t.Errorf("the test found %d must_understand lists, want at least 10", checked)
	}
}

func document(fsys fs.FS, name string) (any, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

// TestTheDetailOfAnUnmetPointerStaysWithinItsLimit pins that CheckChangeMustUnderstand
// returns, for a pointer of 256 characters that need escapes, an error of at most 1000
// characters that starts with the fix and that changes-failed.schema.json accepts as the
// detail of a failure report.
func TestTheDetailOfAnUnmetPointerStaysWithinItsLimit(t *testing.T) {
	schema := compile(t, "changes-failed.schema.json")
	base, err := contracts.Document("fixtures/adapter/changes-failed/change-request-must-understand.json")
	if err != nil {
		t.Fatal(err)
	}
	entry := func(p string) []byte {
		b, err := json.Marshal(map[string]any{"kind": "change_request", "change_request_id": "av_0123456789abcdef",
			"change_kind": "pull-request", "target": map[string]any{"collection": "R_1001"}, "title": "t", "text": "x",
			"must_understand": []string{p}})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	long := func(prefix, unit string, n int) string { return prefix + strings.Repeat(unit, n) }
	for _, c := range []struct{ name, pointer, start string }{
		{"control characters", long("/", "\x01", 255), "Use an adapter that defines "},
		{"zero-width spaces", long("/", "​", 255), "Use an adapter that defines "},
		{"tag characters", long("/", "\U000e0001", 255), "Use an adapter that defines "},
		{"a long ordinary pointer", long("/", "a", 255), "Use an adapter that defines "},
		{"no leading slash", long("", "\x01", 256), "write "},
		{"a line feed", long("/", "\n", 255), "write "},
		{"a bad escape", long("/", "~2", 127), "write "},
	} {
		err := contracts.CheckChangeMustUnderstand(entry(c.pointer))
		if err == nil {
			t.Errorf("%s: CheckChangeMustUnderstand returns no error", c.name)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.start) {
			t.Errorf("%s: the text starts with %.40q, want %q", c.name, err.Error(), c.start)
		}
		if n := utf8.RuneCountInString(err.Error()); n > 1000 {
			t.Errorf("%s: the text has %d characters, want at most 1000", c.name, n)
		}
		report := map[string]any{}
		for k, v := range base.(map[string]any) {
			report[k] = v
		}
		report["detail"] = err.Error()
		b, _ := json.Marshal(report)
		if err := validate(schema, b); err != nil {
			t.Errorf("%s: changes-failed.schema.json refuses the report: %v", c.name, err)
		}
	}
}

// adapterClosedType is a value of the Go type that reads the document of the closed
// fixture with the file name name, chosen by the longest schema name that starts the name.
func adapterClosedType(name string) any {
	types := map[string]any{
		"adapter": &contracts.Adapter{}, "adapter-secrets": &contracts.AdapterSecrets{},
		"adapter-secrets-answer":    &contracts.AdapterSecretsAnswer{},
		"adapter-secrets-plaintext": &contracts.AdapterSecretsPlaintext{},
		"changes":                   &contracts.Changes{}, "changes-failed": &contracts.ChangesFailed{},
		"changes-submitted": &contracts.ChangesSubmitted{}, "connect-pass": &contracts.ConnectPass{},
		"connect-return": &contracts.ConnectReturn{}, "connections-disconnect": &contracts.ConnectionsDisconnect{},
		"grant": &contracts.Grant{}, "grant-end": &contracts.GrantEnd{}, "grants": &contracts.Grants{},
		"read": &contracts.Read{}, "read-answer": &contracts.ReadAnswer{}, "read-public": &contracts.ReadPublic{},
		"refusal": &contracts.Refusal{}, "resolve": &contracts.Resolve{}, "resolve-answer": &contracts.ResolveAnswer{},
		"run-credential": &contracts.RunCredential{}, "run-credential-answer": &contracts.RunCredentialAnswer{},
		"setup-pass": &contracts.SetupPass{}, "source-collections": &contracts.SourceCollections{},
		"source-item-comments": &contracts.SourceItemComments{}, "source-items": &contracts.SourceItems{},
		"sources-disconnect": &contracts.SourcesDisconnect{},
	}
	best := ""
	for k := range types {
		if strings.HasPrefix(name, k+"-") && len(k) > len(best) {
			best = k
		}
	}
	if best == "" {
		return &contracts.Changes{}
	}
	return types[best]
}

// TestAMarshalledDocumentKeepsItsMustUnderstand pins that each Go type of a document that
// has a must_understand list writes the list back, also a type that writes its document by
// a method.
func TestAMarshalledDocumentKeepsItsMustUnderstand(t *testing.T) {
	found := false
	for _, v := range []any{
		contracts.Adapter{}, contracts.AdapterSecrets{}, contracts.AdapterSecretsAnswer{}, contracts.AdapterSecretsPlaintext{},
		contracts.ChangesFailed{}, contracts.ChangesSubmitted{}, contracts.ConnectPass{}, contracts.ConnectReturn{},
		contracts.ConnectionsDisconnect{}, contracts.Grant{}, contracts.GrantEnd{}, contracts.Grants{},
		contracts.Read{}, contracts.ReadAnswer{}, contracts.ReadAnswer{Found: &found}, contracts.ReadPublic{},
		contracts.Refusal{}, contracts.Resolve{}, contracts.ResolveAnswer{}, contracts.RunCredential{},
		contracts.RunCredentialAnswer{}, contracts.SetupPass{}, contracts.SourceCollections{},
		contracts.SourceItemComments{}, contracts.SourceItems{}, contracts.SourcesDisconnect{},
	} {
		ptr := reflect.New(reflect.TypeOf(v))
		ptr.Elem().Set(reflect.ValueOf(v))
		ptr.Elem().FieldByName("MustUnderstand").Set(reflect.ValueOf([]string{"/version"}))
		b, err := json.Marshal(ptr.Interface())
		if err != nil {
			t.Fatalf("%T: %v", v, err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatalf("%T: %v", v, err)
		}
		if got, ok := m["must_understand"].([]any); !ok || len(got) != 1 || got[0] != "/version" {
			t.Errorf("%T writes %s, want must_understand [\"/version\"]", v, b)
		}
	}
}
