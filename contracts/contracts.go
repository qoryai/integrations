// Package contracts embeds the integration contract, contracts/integration/v1, and the
// provisioner contract, contracts/provisioner/v1, and compiles their schemas.
//
// The contract is what a program of an integration reports about itself: `<program>
// describe` prints its description, description.schema.json, and a reader, `qory` or a
// control plane, expands a declared integration from it.
//
// The contract also holds the requests an adapter, the program that reads a source of
// work such as a tracker, sends the control plane, and their answers: [Adapter] at
// start-up, [SourceCollections], [SourceItems], [SourceItemComments] and
// [SourcesDisconnect], the change feed [Changes] it reads back, the [Refusal] of a
// request, and the [ConnectPass] a person brings when they connect the source. Each type
// marshals to the document its schema accepts.
//
// Every request is signed with the adapter's access key exactly as
// github.com/qoryai/forager/accesskey signs a request (Request.Message,
// Key.SignRequest): the headers X-Qory-Access-Key-Id, X-Qory-Instance-Id,
// X-Qory-Contract-Version and X-Qory-Signature-Ed25519; a GET also sends and signs
// X-Qory-Timestamp, and a POST signs its raw body, whose time gives its freshness.
//
// X-Qory-Contract-Version is the revision of the contract the adapter implements,
// [Revision] here, and the control plane refuses a request without it, or naming a
// revision it does not serve, 400 unsupported_contract_version. The control plane's
// answer holds only the members, and the change feed only the entry kinds, that the named
// revision defines: an entry of a kind newer than that revision is skipped for that
// adapter, not sent, and the feed's cursor still moves past it, so a replay from the
// start stays in step. The header alone never selects what an adapter accepts: every
// answer whose content the revision bounds names that revision in its signed body, as
// [Changes] does in Revision, and the adapter refuses one that names another revision than
// the one it implements and sent, and advances no cursor from it. An answer's signature
// covers its body and is bound to the request's signature, so the revision the answer was
// built for is signed. So the answers' schemas stay strict, refusing a member they do not
// define and an entry of a kind they do not know, and an addition to the contract never
// breaks an adapter already deployed. The Forager contract makes additions safe the other
// way, with lenient answers whose unknown members are ignored; both conventions are
// safe, and they apply to different contracts.
//
// The control plane refuses a time or a timestamp more than 300 seconds from its clock,
// either way, and a request whose signature it has already seen, with an unsigned 401;
// definitions.schema.json has the replay rule. A retry is a new request: the adapter
// builds it again, a POST with a time and a GET with an X-Qory-Timestamp later than those
// of the request it retries, and signs it again. A request sent again unchanged has a
// signature the control plane has seen, and is refused, 401; an adapter never takes that
// 401 as its access key being revoked. Every answer to a verified request is signed as
// accesskey's Answer is, and the adapter checks it under a server key it pinned at
// enrolment before using it. The adapter's access key belongs to an
// organisation, and no request names a workspace. forager/accesskey is the reference
// signer; an adapter may sign to this text itself.
//
// The provisioner contract holds the requests with which the adapter that serves a node
// pool claims a run waiting for that pool, [Claim] and its [ClaimAnswer], says it started
// the job, [Dispatched], asks for the run key to hand the job, [Handover] and its
// [HandoverAnswer], or gives the run back, [Release]. They are signed, retried and
// refused with a [Refusal] exactly as the adapter's requests above, and the two answers
// name the revision they were built for, [ProvisionerRevision] here, in Revision, as
// [Changes] does.
//
// Each schema's $id is [Base] followed by the file's path under integration/v1, or
// [ProvisionerBase] followed by its path under provisioner/v1, and [Compile] and
// [CompileProvisioner] compile it by that path, with every embedded schema it refers to.
// The tests in this package validate every fixture under integration/v1/fixtures and
// provisioner/v1/fixtures, so the schemas and the fixtures cannot drift apart.
package contracts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Base is the $id every schema of the contract has, followed by the schema's path
// under integration/v1. It identifies the contract; nothing is fetched from it.
const Base = "https://qory.dev/contracts/integration/v1"

// ProvisionerBase is the $id every schema of the provisioner contract has, followed by
// the schema's path under provisioner/v1. It identifies the contract; nothing is
// fetched from it.
const ProvisionerBase = "https://qory.dev/contracts/provisioner/v1"

//go:embed all:integration all:provisioner
var embedded embed.FS

// FS is the contract directory, rooted at integration/v1: the README, the schema and
// the fixtures.
var FS = must(fs.Sub(embedded, "integration/v1"))

// ProvisionerFS is the provisioner contract's directory, rooted at provisioner/v1: the
// schemas and the fixtures.
var ProvisionerFS = must(fs.Sub(embedded, "provisioner/v1"))

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// trees are the contracts this package embeds, each by the base of its schemas' $id.
var trees = []struct {
	base string
	fsys fs.FS
}{{Base, FS}, {ProvisionerBase, ProvisionerFS}}

// Compile compiles one schema of the contract by its path under integration/v1,
// "description.schema.json" say. A schema of the contract it refers to, such as
// definitions.schema.json, is read from [FS] by its $id, and the JSON Schema meta-schema
// a description's settings are checked against is the copy built into the compiler, so
// nothing is fetched.
func Compile(name string) (*jsonschema.Schema, error) { return compile(Base, FS, name) }

// CompileProvisioner compiles one schema of the provisioner contract by its path under
// provisioner/v1, "claim.schema.json" say. A schema it refers to, of this contract or of
// the integration contract, is read from [ProvisionerFS] or [FS] by its $id, so nothing
// is fetched.
func CompileProvisioner(name string) (*jsonschema.Schema, error) {
	return compile(ProvisionerBase, ProvisionerFS, name)
}

// compile compiles the schema at name in fsys, whose $id is base followed by name.
func compile(base string, fsys fs.FS, name string) (*jsonschema.Schema, error) {
	doc, err := document(fsys, name)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(loader{})
	if err := c.AddResource(base+"/"+name, doc); err != nil {
		return nil, err
	}
	return c.Compile(base + "/" + name)
}

// loader reads a schema of an embedded contract by its $id, from [FS] or
// [ProvisionerFS], and refuses any other URL.
type loader struct{}

// Load returns the document of a schema of an embedded contract by its URL.
func (loader) Load(url string) (any, error) {
	for _, t := range trees {
		if name, ok := strings.CutPrefix(url, t.base+"/"); ok {
			return document(t.fsys, name)
		}
	}
	return nil, fmt.Errorf("%s is not a schema of the contracts, and nothing is fetched", url)
}

// Document reads one JSON file of the contract, by its path under integration/v1, into
// the JSON types a schema validates: maps with string keys, slices, json.Number for
// numbers.
func Document(name string) (any, error) { return document(FS, name) }

// ProvisionerDocument reads one JSON file of the provisioner contract, by its path
// under provisioner/v1, as [Document] reads one of the integration contract.
func ProvisionerDocument(name string) (any, error) { return document(ProvisionerFS, name) }

// document reads the JSON file at name in fsys.
func document(fsys fs.FS, name string) (any, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}
