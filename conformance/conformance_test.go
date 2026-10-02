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
		{"a secret without its file", describe(`{"type": "object", "properties": {"token": {"title": "Access token", "type": "string", "writeOnly": true}}}`), "the secret token has no setting token_file"},
		{"a secret in a setting", describe(`{"type": "object", "properties": {"auth": {"type": "object", "properties": {"token": {"type": "string", "writeOnly": true}}}}}`), "/settings/properties/auth/properties/token is a secret nested in a setting"},
		{"a secret outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "writeOnly": true}}}`), "/settings/$defs/token is a secret outside the settings' properties"},
		{"the settings marked a secret", describe(`{"type": "object", "writeOnly": true}`), "/settings is a secret outside the settings' properties"},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	ok := describe(`{"type": "object", "properties": {"token": {"title": "Access token", "type": "string", "writeOnly": true}, "token_file": {"type": "string"}}}`)
	if err := conformance.Description(ok); err != nil {
		t.Errorf("a secret with its file: %v", err)
	}
}

// TestASecretHasATitleAndAtMostOneName pins the rules on a secret's annotations that the
// schema cannot express: a title, a string that is not only white space, on every
// secret, and an x-secret-name on a secret alone, of its grammar, and on no two secrets.
// A key spelled x-secret-name or writeOnly in data, or as a setting's name, marks
// nothing.
func TestASecretHasATitleAndAtMostOneName(t *testing.T) {
	// secret is a secret <name> with the annotations given and its <name>_file beside it.
	secret := func(name, annotations string) string {
		return `"` + name + `": {"type": "string", "writeOnly": true` + annotations + `}, "` + name + `_file": {"type": "string"}`
	}
	settings := func(props ...string) []byte {
		return describe(`{"type": "object", "properties": {` + strings.Join(props, ", ") + `}}`)
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
		want   string
	}{
		{"no title", settings(secret("token", ``)), "the secret token has no title"},
		{"an empty title", settings(secret("token", `, "title": ""`)), "the secret token has no title"},
		{"a title of white space", settings(secret("token", `, "title": " \t "`)), "the secret token has no title"},
		{"a title that is not a string", settings(secret("token", `, "title": 1`)), "/settings/properties/token/title"},
		{"a name on a setting that is not a secret", settings(`"url": {"type": "string", "x-secret-name": "TRACKER_URL"}`), "the setting url has an x-secret-name and is not a secret"},
		{"a name that is not a string", settings(secret("token", `, "title": "Access token", "x-secret-name": 1`)), "the secret token has an x-secret-name that is not a string"},
		{"a name in lower case", settings(secret("token", `, "title": "Access token", "x-secret-name": "tracker_token"`)), `the secret token has the x-secret-name "tracker_token", which does not match ^[A-Z][A-Z0-9_]{0,127}$`},
		{"a name that starts with a digit", settings(secret("token", `, "title": "Access token", "x-secret-name": "1TOKEN"`)), `the secret token has the x-secret-name "1TOKEN"`},
		{"an empty name", settings(secret("token", `, "title": "Access token", "x-secret-name": ""`)), `the secret token has the x-secret-name ""`},
		{"a name of 129 characters", settings(secret("token", `, "title": "Access token", "x-secret-name": "T`+strings.Repeat("O", 128)+`"`)), "which does not match"},
		{"a name twice", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret", "x-secret-name": "TRACKER_TOKEN"`)), "the secrets token and webhook_secret have the same x-secret-name TRACKER_TOKEN"},
		{"a name in a setting", settings(`"auth": {"type": "object", "properties": {"user": {"type": "string", "x-secret-name": "TRACKER_USER"}}}`), "/settings/properties/auth/properties/user has an x-secret-name nested in a setting"},
		{"a name outside the properties", describe(`{"type": "object", "$defs": {"token": {"type": "string", "x-secret-name": "TRACKER_TOKEN"}}}`), "/settings/$defs/token has an x-secret-name outside the settings' properties"},
		{"a name on the settings", describe(`{"type": "object", "x-secret-name": "FOO"}`), "/settings has an x-secret-name outside the settings' properties"},
	} {
		err := conformance.Description(tc.stdout)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", tc.name, err, tc.want)
		}
	}
	for _, tc := range []struct {
		name   string
		stdout []byte
	}{
		{"a title and no name", settings(secret("token", `, "title": "Access token"`))},
		{"a title and a name", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`))},
		{"a name of one character", settings(secret("token", `, "title": "Access token", "x-secret-name": "T"`))},
		{"a name of 128 characters", settings(secret("token", `, "title": "Access token", "x-secret-name": "T`+strings.Repeat("O", 127)+`"`))},
		{"two secrets, two names", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret", "x-secret-name": "TRACKER_WEBHOOK_SECRET"`))},
		{"two secrets, one named", settings(secret("token", `, "title": "Access token", "x-secret-name": "TRACKER_TOKEN"`), secret("webhook_secret", `, "title": "Webhook secret"`))},
		{"the keys of a default", settings(`"auth": {"type": "object", "default": {"x-secret-name": "a", "writeOnly": true}}`)},
		{"the keys of a const, an enum and an example", settings(`"auth": {"type": "object", "const": {"x-secret-name": "a"}, "enum": [{"writeOnly": true}], "examples": [{"x-secret-name": "a"}]}`)},
		{"a setting named x-secret-name in another", settings(`"auth": {"type": "object", "properties": {"x-secret-name": {"type": "string"}, "writeOnly": true}}`)},
	} {
		if err := conformance.Description(tc.stdout); err != nil {
			t.Errorf("%s: %v", tc.name, err)
		}
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
