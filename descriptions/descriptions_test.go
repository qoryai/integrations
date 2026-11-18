// Package descriptions_test checks the data descriptions in descriptions/: each
// *.json file is a data description that conformance accepts, its name is the file
// name without .json, it sets vendor, each runtime its APIs list is a runtime of
// Forager's runtimes.json that sends the API's format, and each effort value of a model
// is one that such a runtime takes.
package descriptions_test

import (
	"encoding/json"
	"fmt"
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
// each runtime an API lists sends the API's format. It checks that turboEffort, which
// conformance accepts and whose model has the effort value "turbo", fails that check.
func TestEveryEffortValueIsTakenByARuntime(t *testing.T) {
	runtimes := foragerModelRuntimes(t)
	for _, file := range descriptionFiles(t) {
		for _, problem := range effortProblems(t, file, read(t, file), runtimes) {
			t.Error(problem)
		}
	}
	if err := conformance.DataDescription([]byte(turboEffort)); err != nil {
		t.Errorf("turboEffort: conformance refuses it, and the effort rule is this test's alone: %v", err)
	}
	got := effortProblems(t, "turbo-effort.json", []byte(turboEffort), runtimes)
	if len(got) != 1 || !strings.Contains(got[0], `the model "acme-large-2" has the effort values ["turbo"]`) {
		t.Errorf("turboEffort: the problems are %q, want one that names the effort value \"turbo\"", got)
	}
}

// turboEffort is a data description whose API lists claude and whose model has the effort
// values high and turbo, which no runtime takes. It is not a file in descriptions/, so no
// reader of the directory takes it for a description.
const turboEffort = `{"version": 1, "name": "turbo-effort", "title": "Turbo effort", "vendor": "Acme",
 "settings": {"type": "object"}, "egress_hosts": ["api.acme.example"],
 "roles": {"model_provider": {"apis": [{"format": "anthropic-messages", "endpoint": "https://api.acme.example",
 "auth": {"scheme": "bearer"}, "runtimes": ["claude"]}],
 "models": [{"id": "acme-large-2", "title": "Acme Large 2", "effort": ["high", "turbo"]}]}}}`

// effortProblems returns a message for each effort value of a model in the description b
// that no runtime of its APIs takes, and for each API whose format no runtime sends or
// whose listed runtime does not send it. file names the description in the messages.
func effortProblems(t *testing.T, file string, b []byte, runtimes map[string]runtimeModel) []string {
	t.Helper()
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
	if err := json.Unmarshal(b, &d); err != nil {
		return []string{fmt.Sprintf("%s: %v", file, err)}
	}
	var problems []string
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
			problems = append(problems, fmt.Sprintf("%s: apis[%d] has the format %q, and no runtime that it lists, or no runtime at all when it lists none, sends that format in Forager's runtimes.json", file, i, api.Format))
		}
		for _, name := range api.Runtimes {
			if r, ok := runtimes[name]; ok && !slices.Contains(r.Formats, api.Format) {
				problems = append(problems, fmt.Sprintf("%s: apis[%d] lists the runtime %q, which sends %v, not the format %q", file, i, name, r.Formats, api.Format))
			}
		}
	}
	if !allFound {
		return problems
	}
	for _, m := range d.Roles.ModelProvider.Models {
		var bad []string
		for _, v := range m.Effort {
			if !taken[v] {
				bad = append(bad, v)
			}
		}
		if bad != nil {
			problems = append(problems, fmt.Sprintf("%s: the model %q has the effort values %q, which no runtime of its APIs takes; use values from %v", file, m.ID, bad, slices.Sorted(maps.Keys(taken))))
		}
	}
	return problems
}

// TestTheOpenAIDescriptionsAreAsAgreed pins the members of openai.json and
// openai-subscription.json that a run depends on: the format, endpoint, auth, key
// prefixes, paths, runtimes, refresh url and client id of the API, the limit windows,
// the egress hosts, and each model's id, title and effort values, in order. A change to
// one of them fails here and is made on purpose.
func TestTheOpenAIDescriptionsAreAsAgreed(t *testing.T) {
	const claim = "/https:~1~1api.openai.com~1auth/chatgpt_account_id"
	type api struct {
		Format    string `json:"format"`
		Endpoint  string `json:"endpoint"`
		Auth      any    `json:"auth"`
		Prefixes  any    `json:"key_prefixes"`
		Paths     any    `json:"paths"`
		Runtimes  any    `json:"runtimes"`
		OAuthRefr any    `json:"oauth_refresh"`
		OAuthDev  any    `json:"oauth_device"`
	}
	type model struct {
		ID     string   `json:"id"`
		Title  string   `json:"title"`
		Effort []string `json:"effort"`
	}
	type description struct {
		EgressHosts any `json:"egress_hosts"`
		Roles       struct {
			ModelProvider struct {
				APIs         []api   `json:"apis"`
				LimitWindows any     `json:"limit_windows"`
				Models       []model `json:"models"`
			} `json:"model_provider"`
		} `json:"roles"`
	}
	asJSON := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	five := []string{"low", "medium", "high", "xhigh", "max"}
	six := []string{"low", "medium", "high", "xhigh", "max", "ultra"}
	none := append([]string{"none"}, five...)
	for _, tc := range []struct {
		file    string
		hosts   string
		api     api
		windows string
		models  []model
	}{
		{"openai.json", `["api.openai.com"]`, api{
			Format: "openai-responses", Endpoint: "https://api.openai.com/v1",
			Auth: map[string]any{"scheme": "bearer"}, Prefixes: []any{"sk-"},
		}, "null", []model{
			{"gpt-6-astra", "GPT-6 Astra", five},
			{"gpt-6.2-astra", "GPT-6.2 Astra", five},
			{"gpt-6.2-sol", "GPT-6.2 Sol", five},
			{"gpt-6.1-sol", "GPT-6.1 Sol", five},
			{"gpt-6.1-terra", "GPT-6.1 Terra", none},
			{"gpt-6-luna", "GPT-6 Luna", none},
		}},
		{"openai-subscription.json", `["chatgpt.com"]`, api{
			Format: "openai-responses", Endpoint: "https://chatgpt.com/backend-api/codex",
			Auth:  map[string]any{"scheme": "bearer", "oauth": true, "account_header": "chatgpt-account-id", "account_claim": claim},
			Paths: []any{"/backend-api/codex/*"}, Runtimes: []any{"codex"},
			OAuthRefr: map[string]any{"url": "https://auth.openai.com/oauth/token", "client_id": "app_EMoamEEZ73f0CkXaXp7hrann"},
			OAuthDev: map[string]any{
				"usercode_url":     "https://auth.openai.com/api/accounts/deviceauth/usercode",
				"poll_url":         "https://auth.openai.com/api/accounts/deviceauth/token",
				"verification_url": "https://auth.openai.com/codex/device",
				"redirect_uri":     "https://auth.openai.com/deviceauth/callback",
			},
		}, `["five_hour","seven_day"]`, []model{
			{"gpt-6.1-sol", "GPT-6.1 Sol", six},
			{"gpt-6.2-sol", "GPT-6.2 Sol", six},
			{"gpt-6.2-astra", "GPT-6.2 Astra", six},
			{"gpt-6-astra", "GPT-6 Astra", six},
			{"gpt-6.1-terra", "GPT-6.1 Terra", six},
			{"gpt-6-sol", "GPT-6 Sol", six},
			{"gpt-6-luna", "GPT-6 Luna", five},
			{"gpt-5.6-sol", "GPT-5.6 Sol", six},
			{"gpt-5.6-terra", "GPT-5.6 Terra", six},
			{"gpt-5.6-luna", "GPT-5.6 Luna", five},
		}},
	} {
		var d description
		if err := json.Unmarshal(read(t, tc.file), &d); err != nil {
			t.Errorf("%s: %v", tc.file, err)
			continue
		}
		mp := d.Roles.ModelProvider
		if got := asJSON(d.EgressHosts); got != tc.hosts {
			t.Errorf("%s: egress_hosts is %s, want %s", tc.file, got, tc.hosts)
		}
		if got := asJSON(mp.LimitWindows); got != tc.windows {
			t.Errorf("%s: limit_windows is %s, want %s", tc.file, got, tc.windows)
		}
		if len(mp.APIs) != 1 {
			t.Errorf("%s: %d APIs, want 1", tc.file, len(mp.APIs))
		} else if got, want := asJSON(mp.APIs[0]), asJSON(tc.api); got != want {
			t.Errorf("%s: the API is %s, want %s", tc.file, got, want)
		}
		if got, want := asJSON(mp.Models), asJSON(tc.models); got != want {
			t.Errorf("%s: the models are %s, want %s", tc.file, got, want)
		}
	}
}

// TestTheFirstModelIsTheDefault pins the first model of each OpenAI description. The
// first entry of models is the default model of a new provider, so a model put in front
// of it changes the default, and that change is made on purpose.
func TestTheFirstModelIsTheDefault(t *testing.T) {
	for file, want := range map[string]string{
		"openai.json":              "gpt-6-astra",
		"openai-subscription.json": "gpt-6.1-sol",
	} {
		var d struct {
			Roles struct {
				ModelProvider struct {
					Models []struct {
						ID string `json:"id"`
					} `json:"models"`
				} `json:"model_provider"`
			} `json:"roles"`
		}
		if err := json.Unmarshal(read(t, file), &d); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		models := d.Roles.ModelProvider.Models
		if len(models) == 0 || models[0].ID != want {
			t.Errorf("%s: the first model is %v, want %q, the default model of a new provider", file, models, want)
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

// TestTheOpenAISubscriptionDescriptionStartsWithTheDeviceSignIn pins the first sentence of
// the description text of the OpenAI subscription provider, which the control plane
// shows to the admin who adds the provider. It says how the sign-in is made, which is the
// oauth_device flow of the API.
func TestTheOpenAISubscriptionDescriptionStartsWithTheDeviceSignIn(t *testing.T) {
	var d struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(read(t, "openai-subscription.json"), &d); err != nil {
		t.Fatal(err)
	}
	const want = "Runs use your ChatGPT subscription through a sign-in that Qory Apiary makes with OpenAI's device code."
	if !strings.HasPrefix(d.Description, want+" ") {
		t.Errorf("the description %q does not start with %q", d.Description, want)
	}
}

// TestASubscriptionDescriptionWarnsOfPersonalUse checks that the description text of each
// subscription provider says the subscription is for personal use only and links the
// vendor's terms, since the provider's terms do not allow a subscription to serve requests
// on behalf of other users.
func TestASubscriptionDescriptionWarnsOfPersonalUse(t *testing.T) {
	for file, terms := range map[string]string{
		"claude-subscription.json": "Anthropic's terms: https://code.claude.com/docs/en/legal-and-compliance",
		"openai-subscription.json": "OpenAI's terms: https://openai.com/policies/terms-of-use/",
	} {
		var d struct {
			Description string `json:"description"`
		}
		if err := json.Unmarshal(read(t, file), &d); err != nil {
			t.Errorf("%s: %v", file, err)
			continue
		}
		if !strings.Contains(d.Description, "For personal use only: ") {
			t.Errorf("%s: the description %q lacks the warning that begins \"For personal use only: \"", file, d.Description)
		}
		if !strings.HasSuffix(d.Description, terms) {
			t.Errorf("%s: the description %q does not end with the terms link %q", file, d.Description, terms)
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
