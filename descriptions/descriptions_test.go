// Package descriptions_test checks the data descriptions in descriptions/: each
// *.json file is a data description that conformance accepts, and its name is the
// file name without .json.
package descriptions_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qoryai/integrations/conformance"
)

// TestEveryDescriptionConforms reads every descriptions/*.json file, checks it with
// conformance.DataDescription and checks that its name is the file name without
// .json. It fails when the directory has no such file.
func TestEveryDescriptionConforms(t *testing.T) {
	files, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("descriptions/ has no *.json file; the test reads the descriptions from the directory it runs in")
	}
	for _, file := range files {
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
