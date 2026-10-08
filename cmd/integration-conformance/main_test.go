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

// TestAccepted pins that every description the contract accepts exits 0 and prints
// nothing.
func TestAccepted(t *testing.T) {
	entries, err := fs.ReadDir(contracts.FS, "fixtures")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		code, stdout, stderr := run(t, fixture(t, "fixtures/"+e.Name()))
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%s: exit %d, standard output %q, standard error %q; want 0 and nothing printed", e.Name(), code, stdout, stderr)
		}
	}
}

// TestRefusedBySchema pins that a description the contract's schema refuses exits 1,
// prints nothing on standard output, and prints the refusal on standard error.
func TestRefusedBySchema(t *testing.T) {
	code, stdout, stderr := run(t, fixture(t, "fixtures/invalid/description-bad-name.json"))
	if code != 1 || stdout != "" {
		t.Fatalf("exit %d, standard output %q; want 1 and nothing on standard output", code, stdout)
	}
	want := "describe: the contract's schema refuses the description: "
	if !strings.HasPrefix(stderr, want) || !strings.HasSuffix(stderr, "\n") {
		t.Errorf("standard error %q; want it to start with %q and end with a line ending", stderr, want)
	}
}

// TestRefusedBeyondTheSchema pins that a refusal beyond the schema, a secret without its
// file, is printed as conformance.Description gives it.
func TestRefusedBeyondTheSchema(t *testing.T) {
	description := `{"version": 1, "name": "acme-tracker", "title": "Acme tracker", "program_version": "0.1.0",
 "settings": {"type": "object", "properties": {"api_key": {"type": "string", "writeOnly": true}}},
 "roles": {"credential": {"argument": "[A-Z]+", "hosts": ["tracker.acme.example"]}}}`
	code, stdout, stderr := run(t, []byte(description))
	want := "describe: the secret api_key has no setting api_key_file\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Errorf("exit %d, standard output %q, standard error %q; want 1, nothing on standard output and %q", code, stdout, stderr, want)
	}
}

// TestNothing pins that empty standard input is refused, not accepted.
func TestNothing(t *testing.T) {
	code, stdout, stderr := run(t, nil)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "not a JSON document") {
		t.Errorf("exit %d, standard output %q, standard error %q; want 1 and a refusal", code, stdout, stderr)
	}
}

// TestArguments pins that the command refuses an argument.
func TestArguments(t *testing.T) {
	code, stdout, stderr := run(t, fixture(t, "fixtures/acme-tracker.json"), "description.json")
	if code != 2 || stdout != "" || strings.Count(stderr, "\n") != 1 {
		t.Errorf("exit %d, standard output %q, standard error %q; want 2 and one line on standard error", code, stdout, stderr)
	}
}
