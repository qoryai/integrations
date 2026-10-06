package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// TestMain runs the command itself when the test binary is started with
// INTEGRATION_CONFORMANCE_MAIN set, so the tests run it as release.yml does.
func TestMain(m *testing.M) {
	if os.Getenv("INTEGRATION_CONFORMANCE_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// run runs the command with stdin and the arguments args, and returns its exit status,
// standard output and standard error.
func run(t *testing.T, stdin []byte, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "INTEGRATION_CONFORMANCE_MAIN=1")
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatal(err)
	}
	return cmd.ProcessState.ExitCode(), stdout.String(), stderr.String()
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := fs.ReadFile(contracts.FS, name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestAccepted pins that a description the contract accepts exits 0 and prints nothing.
func TestAccepted(t *testing.T) {
	code, stdout, stderr := run(t, fixture(t, "fixtures/acme-tracker.json"))
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("exit %d, standard output %q, standard error %q; want 0 and nothing printed", code, stdout, stderr)
	}
}

// TestRefused pins that a refused description exits 1, prints nothing on standard
// output, and prints each refusal on a line of its own on standard error.
func TestRefused(t *testing.T) {
	code, stdout, stderr := run(t, fixture(t, "fixtures/invalid/description-settings-required.json"))
	if code != 1 || stdout != "" {
		t.Fatalf("exit %d, standard output %q; want 1 and nothing on standard output", code, stdout)
	}
	lines := strings.Split(stderr, "\n")
	want := []string{
		"the settings have the top-level keyword required",
		"the top level may carry only ",
		"the contract's schema refuses the description: ",
	}
	if len(lines) < len(want) {
		t.Fatalf("standard error %q; want a line starting with each of %q", stderr, want)
	}
	for i, w := range want {
		if !strings.HasPrefix(lines[i], w) {
			t.Errorf("line %d is %q; want it to start with %q", i+1, lines[i], w)
		}
	}
}

// TestRefusalsBeyondTheSchema pins that the refusals beyond the schema each get a line.
func TestRefusalsBeyondTheSchema(t *testing.T) {
	description := `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "publisher": {"name": "Acme"}, "program_version": "0.1.0",
 "settings": {"type": "object", "properties": {"api_key": {"type": "string", "writeOnly": true}}},
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"], "settings": ["api_key"]}}}`
	code, _, stderr := run(t, []byte(description))
	want := "the secret api_key has no setting api_key_file\nthe secret api_key has no title\n"
	if code != 1 || stderr != want {
		t.Errorf("exit %d, standard error %q; want 1 and %q", code, stderr, want)
	}
}

// TestArguments pins that the command refuses an argument.
func TestArguments(t *testing.T) {
	code, stdout, stderr := run(t, fixture(t, "fixtures/acme-tracker.json"), "description.json")
	if code != 2 || stdout != "" || strings.Count(stderr, "\n") != 1 {
		t.Errorf("exit %d, standard output %q, standard error %q; want 2 and one line on standard error", code, stdout, stderr)
	}
}
