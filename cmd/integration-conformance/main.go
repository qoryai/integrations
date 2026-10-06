// Command integration-conformance checks one description, what `<program> describe`
// printed, against the integration contract with conformance.Description. It reads the
// description on standard input and takes no arguments:
//
//	go run github.com/qoryai/integrations/cmd/integration-conformance <description.json
//
// It exits 0 and prints nothing when the description passes. Otherwise it exits 1 and
// prints each refusal on standard error, one per line; the schema's refusal takes the
// lines the schema's validator gives it. It exits 2 when it is given an argument or
// cannot read standard input.
//
// release.yml runs it in the calling integration's module, so a release checks its
// description with the version of this module the integration's own tests use.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

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
		for _, line := range refusals(err) {
			fmt.Fprintln(os.Stderr, line)
		}
		os.Exit(1)
	}
}

// schemaRefusal starts the schema's refusal in what conformance.Description returns. It
// comes last, and its own text may hold "; ", so it is never split.
const schemaRefusal = "the contract's schema refuses the description: "

// refusals are the refusals err names, each on a line of its own: conformance.Description
// joins them with "; " after "describe: ".
func refusals(err error) []string {
	msg := strings.TrimPrefix(err.Error(), "describe: ")
	schema := ""
	if i := strings.Index(msg, schemaRefusal); i >= 0 {
		msg, schema = strings.TrimSuffix(msg[:i], "; "), msg[i:]
	}
	var out []string
	if msg != "" {
		out = strings.Split(msg, "; ")
	}
	if schema != "" {
		out = append(out, schema)
	}
	return out
}
