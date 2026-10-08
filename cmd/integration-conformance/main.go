// Command integration-conformance checks one description, what `<program> describe`
// printed, against the integration contract with conformance.Description. It reads the
// description on standard input and takes no arguments:
//
//	go build -o integration-conformance github.com/qoryai/integrations/cmd/integration-conformance
//	./integration-conformance <description.json
//
// It exits 0 and prints nothing when the description passes. Otherwise it exits 1 and
// prints the refusal on standard error: the error conformance.Description returns, or,
// when that error joins several, each of them on a line of its own. The schema's refusal
// takes the lines the schema's validator gives it. It exits 2 when it is given an
// argument or cannot read standard input.
//
// release.yml builds it in the calling integration's module, so a release checks its
// description with the version of this module the integration's own tests use.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/qoryai/integrations/conformance"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, "integration-conformance takes no arguments; it reads one description on standard input")
		os.Exit(2)
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "integration-conformance: reading standard input: %v\n", err)
		os.Exit(2)
	}
	if err := conformance.Description(b); err != nil {
		refusals := []error{err}
		if r, ok := err.(interface{ Unwrap() []error }); ok {
			refusals = r.Unwrap()
		}
		for _, r := range refusals {
			fmt.Fprintln(os.Stderr, r)
		}
		os.Exit(1)
	}
}
