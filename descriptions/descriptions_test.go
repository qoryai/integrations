// Package descriptions_test checks the data descriptions in descriptions/: each
// *.json file is a data description that conformance accepts, its name is the file
// name without .json, it sets vendor, each runtime its APIs list is a runtime of
// Forager's runtimes.json that sends the API's format, and each effort value of a model
// is one that such a runtime takes.
package descriptions_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// TestEveryEffortValueIsTakenByARuntime checks that each effort value of a model in a
// descriptions/*.json file is an effort value of a runtime in runtimes.json of the
// pinned Forager module. The runtimes are the ones the API of the model lists, or,
// when it lists none, every runtime that sends the API's format. It also checks that
// each runtime an API lists sends the API's format. A mutation to the check: a model
// with the effort value "turbo" fails it.
func TestEveryEffortValueIsTakenByARuntime(t *testing.T) {
	runtimes := foragerModelRuntimes(t)
	for _, file := range descriptionFiles(t) {
		var d struct {
			Roles struct {
				ModelProvider struct {
					APIs []struct {
						Format   string   `json:"format"`
						Runtimes []string `json:"runtimes"`
					} `json:"apis"`
					Models []struct {
						ID     string   `json:"id"`
						Effort []string `json:"effort"`
					} `json:"models"`
				} `json:"model_provider"`
			} `json:"roles"`
		}
		if err := json.Unmarshal(read(t, file), &d); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		taken := map[string]bool{}
		allFound := true
		for i, api := range d.Roles.ModelProvider.APIs {
			found := false
			for _, r := range runtimes {
				if !slices.Contains(r.Formats, api.Format) || (api.Runtimes != nil && !slices.Contains(api.Runtimes, r.Name)) {
					continue
				}
				found = true
				for _, v := range r.Effort {
					taken[v] = true
				}
			}
			if !found {
				allFound = false
				t.Errorf("%s: apis[%d] has the format %q, and no runtime that it lists, or no runtime at all when it lists none, sends that format in Forager's runtimes.json", file, i, api.Format)
			}
			for _, name := range api.Runtimes {
				if r, ok := runtimes[name]; ok && !slices.Contains(r.Formats, api.Format) {
					t.Errorf("%s: apis[%d] lists the runtime %q, which sends %v, not the format %q", file, i, name, r.Formats, api.Format)
				}
			}
		}
		if !allFound {
			continue
		}
		for _, m := range d.Roles.ModelProvider.Models {
			var bad []string
			for _, v := range m.Effort {
				if !taken[v] {
					bad = append(bad, v)
				}
			}
			if bad != nil {
				t.Errorf("%s: the model %q has the effort values %q, which no runtime of its APIs takes; use values from %v", file, m.ID, bad, slices.Sorted(maps.Keys(taken)))
			}
		}
	}
}

// TestTheAgreedMembersAreSet checks the members that each provider description sets for
// the control plane: vendor, the runtimes of each API, key_lifetime_days, the account
// header pair of auth and oauth_refresh. The values are the ones agreed for these
// providers, so a change to one of them fails here and is made on purpose.
func TestTheAgreedMembersAreSet(t *testing.T) {
	const claim = "/https:~1~1api.openai.com~1auth/chatgpt_account_id"
	for _, tc := range []struct {
		file, vendor, runtimes, lifetime, account, refresh string
	}{
		{"anthropic.json", "Anthropic", "", "", "", ""},
		{"claude-subscription.json", "Anthropic", "claude", "365", "", ""},
		{"deepseek.json", "DeepSeek", "", "", "", ""},
		{"litellm.json", "your LiteLLM proxy", "", "", "", ""},
		{"openai.json", "OpenAI", "", "", "", ""},
		{"openai-subscription.json", "OpenAI", "codex", "", "chatgpt-account-id " + claim, "https://auth.openai.com/oauth/token"},
		{"openrouter.json", "OpenRouter", "", "", "", ""},
	} {
		var d struct {
			Vendor string `json:"vendor"`
			Roles  struct {
				ModelProvider struct {
					APIs []struct {
						Runtimes []string `json:"runtimes"`
						Auth     struct {
							AccountHeader string `json:"account_header"`
							AccountClaim  string `json:"account_claim"`
						} `json:"auth"`
						OAuthRefresh *struct {
							URL string `json:"url"`
						} `json:"oauth_refresh"`
					} `json:"apis"`
					KeyLifetimeDays *int `json:"key_lifetime_days"`
				} `json:"model_provider"`
			} `json:"roles"`
		}
		if err := json.Unmarshal(read(t, tc.file), &d); err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		mp := d.Roles.ModelProvider
		if len(mp.APIs) != 1 {
			t.Errorf("%s: %d APIs, want 1", tc.file, len(mp.APIs))
			continue
		}
		api := mp.APIs[0]
		if d.Vendor != tc.vendor {
			t.Errorf("%s: vendor is %q, want %q", tc.file, d.Vendor, tc.vendor)
		}
		if got := strings.Join(api.Runtimes, ","); got != tc.runtimes {
			t.Errorf("%s: runtimes is %q, want %q", tc.file, got, tc.runtimes)
		}
		lifetime := ""
		if mp.KeyLifetimeDays != nil {
			lifetime = strconv.Itoa(*mp.KeyLifetimeDays)
		}
		if lifetime != tc.lifetime {
			t.Errorf("%s: key_lifetime_days is %q, want %q", tc.file, lifetime, tc.lifetime)
		}
		account := strings.TrimSpace(api.Auth.AccountHeader + " " + api.Auth.AccountClaim)
		if account != tc.account {
			t.Errorf("%s: the account header and claim are %q, want %q", tc.file, account, tc.account)
		}
		refresh := ""
		if api.OAuthRefresh != nil {
			refresh = api.OAuthRefresh.URL
		}
		if refresh != tc.refresh {
			t.Errorf("%s: oauth_refresh url is %q, want %q", tc.file, refresh, tc.refresh)
		}
	}
}

// runtimeModel is a runtime of runtimes.json as far as the checks here read it: its
// name, the API formats it sends and the values its run effort takes.
type runtimeModel struct {
	Name    string
	Formats []string
	Effort  []string
}

// foragerModelRuntimes returns each runtime of runtimes.json of the pinned Forager
// module's contracts.FS, by name.
func foragerModelRuntimes(t *testing.T) map[string]runtimeModel {
	t.Helper()
	var doc struct {
		Runtimes []struct {
			Name  string `json:"name"`
			Model struct {
				Formats []string `json:"formats"`
				Effort  struct {
					Run struct {
						Values []string `json:"values"`
					} `json:"run"`
				} `json:"effort"`
			} `json:"model"`
		} `json:"runtimes"`
	}
	b, err := fs.ReadFile(forager.FS, "runtimes.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("Forager's runtimes.json: %v", err)
	}
	out := map[string]runtimeModel{}
	for _, r := range doc.Runtimes {
		out[r.Name] = runtimeModel{r.Name, r.Model.Formats, r.Model.Effort.Run.Values}
	}
	return out
}
