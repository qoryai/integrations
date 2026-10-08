package conformance_test

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/qoryai/integrations/conformance"
	"github.com/qoryai/integrations/contracts"
)

// fixtures are the contents of the contract's fixtures directly under dir, by path.
func fixtures(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	entries, err := fs.ReadDir(contracts.FS, dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := fs.ReadFile(contracts.FS, path.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[path.Join(dir, e.Name())] = b
	}
	if len(out) == 0 {
		t.Fatalf("%s contains no fixture", dir)
	}
	return out
}

// TestEveryAcceptedFixtureConforms pins that each description the contract accepts
// passes Description, the secret rule the schema cannot express among its checks.
func TestEveryAcceptedFixtureConforms(t *testing.T) {
	for name, b := range fixtures(t, "fixtures") {
		if err := conformance.Description(b); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

// TestEveryRefusedFixtureDoesNot pins that each description the contract's schema
// refuses fails Description too.
func TestEveryRefusedFixtureDoesNot(t *testing.T) {
	for name, b := range fixtures(t, "fixtures/invalid") {
		if err := conformance.Description(b); err == nil {
			t.Errorf("%s conforms", name)
		}
	}
}

const tracker = `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": %s,
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"]}}}
`

func describe(settings string) []byte {
	return []byte(strings.Replace(tracker, "%s", settings, 1))
}

// TestADescriptionIsOneDocumentWithItsSecretsOnTop pins what Description refuses beyond
// the schema: output that is not one document, and a secret the contract does not allow.
func TestADescriptionIsOneDocumentWithItsSecretsOnTop(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stdout []byte
		want   string
	}{
		{"nothing", nil, "not a JSON document"},
		{"two documents", append(describe(`{"type": "object"}`), "{}\n"...), "more than one JSON document"},
		{"a secret without its file", describe(`{"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}`), "the secret token has no setting token_file"},
		{"a secret in a setting", describe(`{"type": "object", "properties": {"auth": {"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}}}`), "/settings/properties/auth/properties/token is a secret nested in a setting"},
		{"a secret outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "writeOnly": true}}}`), "/settings/$defs/token is a secret outside the settings' properties"},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	ok := describe(`{"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}, "token_file": {"type": "string"}}}`)
	if err := conformance.Description(ok); err != nil {
		t.Errorf("a secret with its file: %v", err)
	}
}

// TestCredentialIsTheRunnersDocument pins Credential to the runner's schema: an answer
// it accepts conforms, and one it refuses, or two, do not.
func TestCredentialIsTheRunnersDocument(t *testing.T) {
	answer := `{"version":1,"token":"synthetic","expires_at":"2026-09-25T21:00:00Z","apply":[{"hosts":["tracker.acme.example"],"scheme":"bearer","paths":["/api/*"]}],"placeholders":["TRACKER_TOKEN"]}` + "\n"
	if err := conformance.Credential([]byte(answer)); err != nil {
		t.Errorf("a valid answer: %v", err)
	}
	for _, tc := range []struct{ name, stdout, want string }{
		{"no apply", `{"version":1,"token":"synthetic"}`, "the runner's schema refuses"},
		{"an unknown scheme", `{"version":1,"token":"synthetic","apply":[{"hosts":["a.example"],"scheme":"cookie"}]}`, "the runner's schema refuses"},
		{"two documents", answer + answer, "more than one JSON document"},
	} {
		err := conformance.Credential([]byte(tc.stdout))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}

// TestFailureIsOneLineAndNothingElse pins the exit status the contract's commands share.
func TestFailureIsOneLineAndNothingElse(t *testing.T) {
	if err := conformance.Failure(1, nil, []byte("acme-tracker credential: the token file is missing\n")); err != nil {
		t.Errorf("a failure as the contract says: %v", err)
	}
	for _, tc := range []struct {
		name           string
		code           int
		stdout, stderr string
		want           string
	}{
		{"exit 0", 0, "", "failed\n", "exits 0"},
		{"standard output", 1, "{}\n", "failed\n", "on standard output"},
		{"two lines", 1, "", "failed\nbecause\n", "one line"},
		{"no newline", 1, "", "failed", "one line"},
		{"an empty line", 1, "", " \n", "say what failed"},
	} {
		err := conformance.Failure(tc.code, []byte(tc.stdout), []byte(tc.stderr))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
}
