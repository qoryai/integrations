// Package contracts embeds the integration contract, contracts/integration/v1, and
// compiles its schema.
//
// The contract is what a program of an integration reports about itself: `<program>
// describe` prints its description, description.schema.json, and a reader, `qory` or a
// control plane, expands a declared integration from it. The schema's $id is [Base]
// followed by the file's path under integration/v1, and [Compile] compiles it by that
// path. The tests in this package validate every fixture under integration/v1/fixtures,
// so the schema and the fixtures cannot drift apart.
package contracts

import (
	"bytes"
	"embed"
	"io/fs"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Base is the $id every schema of the contract has, followed by the schema's path
// under integration/v1. It identifies the contract; nothing is fetched from it.
const Base = "https://qory.dev/contracts/integration/v1"

//go:embed all:integration
var embedded embed.FS

// FS is the contract directory, rooted at integration/v1: the README, the schema and
// the fixtures.
var FS = must(fs.Sub(embedded, "integration/v1"))

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Compile compiles one schema of the contract by its path under integration/v1,
// "description.schema.json". The JSON Schema meta-schema a description's settings are
// checked against is the compiler's own copy, so nothing is fetched.
func Compile(name string) (*jsonschema.Schema, error) {
	doc, err := Document(name)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(Base+"/"+name, doc); err != nil {
		return nil, err
	}
	return c.Compile(Base + "/" + name)
}

// Document reads one JSON file of the contract, by its path under integration/v1, into
// the JSON types a schema validates: maps with string keys, slices, json.Number for
// numbers.
func Document(name string) (any, error) {
	b, err := fs.ReadFile(FS, name)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}
