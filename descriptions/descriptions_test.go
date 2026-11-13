// Package descriptions_test checks the data descriptions in descriptions/: each
// *.json file is a data description that conformance accepts, its name is the file
// name without .json, it sets vendor, and each runtime its APIs list is a runtime of
// Forager's runtimes.json.
package descriptions_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	forager "github.com/qoryai/forager/contracts"
	"github.com/qoryai/integrations/conformance"
)

// TestEveryDescriptionConforms reads every descriptions/*.json file, checks it with
// conformance.DataDescription and checks that its name is the file name without
// .json. It fails when the directory has no such file.
func TestEveryDescriptionConforms(t *testing.T) {
	for _, file := range descriptionFiles(t) {
		b, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if err := conformance.DataDescription(b); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		var d struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(b, &d); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if want := strings.TrimSuffix(file, ".json"); d.Name != want {
			t.Errorf("%s: the name is %q; set it to %q, the file name without .json", file, d.Name, want)
		}
	}
}

// TestEveryDescriptionSetsVendor checks that every descriptions/*.json file sets
// vendor. The schema allows a description without vendor; the descriptions here all
// set it, so the control plane can show the vendor's name for each of them.
func TestEveryDescriptionSetsVendor(t *testing.T) {
	for _, file := range descriptionFiles(t) {
		var d struct {
			Vendor *string `json:"vendor"`
		}
		if err := json.Unmarshal(read(t, file), &d); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if d.Vendor == nil {
			t.Errorf("%s: vendor is absent; set it to the vendor's name, as the control plane shows it", file)
		}
	}
}

// TestEveryRuntimeIsInForagersRuntimes checks that each runtime in the runtimes of a
// descriptions/*.json API is a name in runtimes.json of the pinned Forager module. It
// also checks that unknownRuntime, which lists acme-runtime and which conformance
// accepts, fails that check.
func TestEveryRuntimeIsInForagersRuntimes(t *testing.T) {
	known := foragerRuntimes(t)
	for _, file := range descriptionFiles(t) {
		for _, name := range unknownRuntimes(t, read(t, file), known) {
			t.Errorf("%s: the runtime %q is not in Forager's runtimes.json; use one of %v", file, name, slices.Sorted(maps.Keys(known)))
		}
	}
	if err := conformance.DataDescription([]byte(unknownRuntime)); err != nil {
		t.Errorf("unknownRuntime: conformance refuses it, and the runtime rule is this test's alone: %v", err)
	}
	got := unknownRuntimes(t, []byte(unknownRuntime), known)
	if want := []string{"acme-runtime"}; !slices.Equal(got, want) {
		t.Errorf("unknownRuntime: the unknown runtimes are %q, want %q", got, want)
	}
}

// unknownRuntime is a data description whose API lists claude and acme-runtime, a
// runtime that Forager's runtimes.json does not have. It is not a file in
// descriptions/, so no reader of the directory takes it for a description.
const unknownRuntime = `{"version": 1, "name": "unknown-runtime", "title": "Unknown runtime", "vendor": "Acme",
 "settings": {"type": "object"}, "egress_hosts": ["api.acme.example"],
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://api.acme.example",
 "auth": {"scheme": "bearer"}, "runtimes": ["claude", "acme-runtime"]}], "models": [{"id": "acme-large-2", "title": "Acme Large 2"}]}}}`

// foragerRuntimes returns the name of each runtime in runtimes.json of the pinned
// Forager module's contracts.FS.
func foragerRuntimes(t *testing.T) map[string]bool {
	t.Helper()
	b, err := fs.ReadFile(forager.FS, "runtimes.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Runtimes []struct {
			Name string `json:"name"`
		} `json:"runtimes"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Forager's runtimes.json: %v", err)
	}
	known := map[string]bool{}
	for _, r := range doc.Runtimes {
		known[r.Name] = true
	}
	if len(known) == 0 {
		t.Fatal("Forager's runtimes.json has no runtime")
	}
	return known
}

// unknownRuntimes returns each runtime in the runtimes of a model_provider API of the
// description b that known does not have, in the order the description lists them.
func unknownRuntimes(t *testing.T, b []byte, known map[string]bool) []string {
	t.Helper()
	var d struct {
		Roles struct {
			ModelProvider struct {
				APIs []struct {
					Runtimes []string `json:"runtimes"`
				} `json:"apis"`
			} `json:"model_provider"`
		} `json:"roles"`
	}
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, api := range d.Roles.ModelProvider.APIs {
		for _, name := range api.Runtimes {
			if !known[name] {
				out = append(out, name)
			}
		}
	}
	return out
}

// descriptionFiles returns the descriptions/*.json files, or fails the test when the
// directory has none.
func descriptionFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("descriptions/ has no *.json file; the test reads the descriptions from the directory it runs in")
	}
	return files
}

// read returns the bytes of file, or fails the test.
func read(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
