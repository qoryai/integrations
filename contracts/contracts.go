// Package contracts embeds the integration contract, contracts/integration/v1, the
// provisioner contract, contracts/provisioner/v1, the run contract, contracts/run/v1,
// the explore contract, contracts/explore/v1, and the machines contract,
// contracts/machines/v1, and compiles their schemas.
//
// The contract is what a program of an integration reports about itself: `<program>
// describe` prints its description, description.schema.json, and a reader, `qory` or a
// control plane, expands a declared integration from it.
//
// The contract also holds the requests an adapter, the program that reads a source of
// work such as a tracker, sends the control plane, and their answers: [Adapter] at
// start-up, [SourceCollections], [SourceItems], [SourceItemComments], [SourcesDisconnect]
// and [ConnectionsDisconnect], the change feed [Changes] it reads back, the secrets it
// stores in the control plane, [AdapterSecrets], sealed to the server key, and reads
// back sealed to its access key, [AdapterSecretsAnswer], the [Grant] it reports for a
// connection, the [GrantEnd] it reports when the source no longer covers a grant, and
// the control plane's copy of grants, [Grants], the [Refusal] of a request, the
// [ConnectPass] a person brings when they connect the source, the [ConnectReturn] the adapter sends the person back with, and the [SetupPass] an
// instance admin brings when they set the adapter up at its source. Each type marshals
// to the document its schema accepts.
//
// Every request is signed with the adapter's access key exactly as
// github.com/qoryai/forager/accesskey signs a request (Request.Message,
// Key.SignRequest): the headers X-Qory-Access-Key-Id, X-Qory-Instance-Id,
// X-Qory-Contract-Version and X-Qory-Signature-Ed25519; a GET also sends and signs
// X-Qory-Timestamp, and a POST signs its raw body, whose time gives its freshness.
//
// X-Qory-Contract-Version is the revision of the contract the adapter implements,
// [Revision] here, and the control plane refuses one that is missing, sent twice, not a
// decimal integer, or a revision it does not serve, 400 unsupported_contract_version.
// The control plane's answer holds only the members, and the change feed only the entry
// kinds, that the named revision defines: an entry of a kind newer than that revision is
// skipped for that adapter, not sent, and the feed's cursor still moves past it, so a
// replay from the start stays in step. The header alone never selects what an adapter
// accepts: every answer whose content the revision bounds names that revision in its
// signed body, as [Changes] does in Revision, and the adapter refuses one that names
// another revision than the one it implements and sent, and advances no cursor from it.
// An answer's signature covers its body and is bound to the request's signature, so the
// revision the answer was built for is signed. So the answers' schemas stay strict,
// refusing a member they do not define and an entry of a kind they do not know, and an
// addition to the contract never breaks an adapter already deployed. The Forager
// contract makes additions safe the other way, with lenient answers whose unknown
// members are ignored; both conventions are safe, and they apply to different contracts.
//
// The control plane refuses a time or a timestamp more than 300 seconds from its clock,
// either way, and a request whose signature it has already seen, with an unsigned 401;
// definitions.schema.json has the replay rule. That 401 is one uniform answer for
// everything about who is asking, whose causes refusal.schema.json lists, and it tells
// the adapter nothing about which cause applied. An X-Qory-Access-Key-Id,
// X-Qory-Instance-Id, X-Qory-Signature-Ed25519 or X-Qory-Timestamp sent twice, or an
// X-Qory-Instance-Id that is absent or outside its pattern, is refused 400 bad_request,
// and only those are. A retry is a new request: the adapter builds it again, a POST with
// a time and a GET with an X-Qory-Timestamp later than those of the request it retries,
// and signs it again. Each GET an instance sends has an X-Qory-Timestamp later than
// that of the previous GET it sent, so no two GETs of one instance have the same
// signature, and an answer bound to one GET verifies for no other. A request sent again
// unchanged has a signature the control plane has seen, and is refused, 401; an adapter
// never takes that 401 as its access key being revoked. Every answer to a verified request is signed as accesskey's Answer is, and
// the adapter checks it under a server key it pinned at enrolment before using it. A 401
// is always unsigned. A 413, 415 or 400 bad_request may be unsigned, because the control
// plane may decide them before it verifies the request's signature. Every other answer
// is signed, and an adapter acts on no other unsigned answer. On an unsigned 401, 413,
// 415 or 400 bad_request, the adapter changes no state: no cursor, no run, no key. It
// treats the request as failed and sends it again, built anew, with backoff. It never
// takes a 401 as revocation, and never splits or drops items because of an unsigned 413.
// An unsigned 429 is not a 429: the adapter ignores its Retry-After and backs off as it
// does for any failed request. A POST whose Content-Type is not application/json is
// refused 415 unsupported_media_type; a GET carries no body and no Content-Type, and is
// never refused 415. A POST body is application/json. An integer is written without a
// fraction or an exponent, and no object repeats a member, at any depth; the control
// plane refuses either, 400 invalid_request. The numbers inside the description member
// of POST /v1/adapter follow description.schema.json instead, and may have a fraction or
// an exponent; no object inside it repeats a member either. A POST whose schema names no
// answer body is answered, on success, with a signed 204 and no body. The adapter's
// access key is the integration's, enrolled once for the instance. An organisation
// reaches the adapter only through a connection, and no request names an organisation or
// a workspace. forager/accesskey is the reference signer; an adapter may sign to this
// text itself.
//
// The control plane also calls an adapter, at the call_url the adapter declares in its
// start-up body: each call is a POST, a [Call], signed with the control plane's server
// key over [CallDomain], and the adapter signs its answer, a [CallAnswer], with its
// access key over [CallAnswerDomain], bound to the call's signature. The calls are
// [RunCredential], with the path [RunCredentialPath], which the adapter answers with
// [RunCredentialAnswer]; [Resolve], with the path [ResolvePath], which it answers with
// [ResolveAnswer]; and [Read], with the path [ReadPath], which it answers with
// [ReadAnswer]. The adapter has one access key for every organisation, so the control
// plane uses a resolve result, and sends a [Read] or a [RunCredential] whose Purpose is
// "harness", only under a grant that the run may use: a grant of a live connection of
// the run's organisation, or, only to read a harness source, a grant whose
// organisation assigned that source to the run's organisation. It sends a
// [RunCredential] whose Purpose is "work" with no grant and with the ConnectionID of
// the run's target collection, a live connection of the run's organisation when the
// call is sent, and the adapter uses only a grant that the control plane's copy of
// grants, [Grants], has under that connection. Under an assignment, it sends a [RunCredential] only when
// the adapter's credential_scope is resource, with Resources set to the assigned
// source's resource alone; when credential_scope is grant, it reads the assigned
// source only with a [Read]. resolve-answer.schema.json states the rule.
//
// The provisioner contract holds the requests with which a node pool's starter, the
// program that serves a node pool, claims a request for one machine, [Claim] and its
// [ClaimAnswer], whose run code and gateway URL it passes to the machine it creates,
// says it created the machine, [Dispatched], gives the dispatch back before
// dispatched, [Release], or uploads the log of the machine it created for a dispatch,
// [MachineLog]. The starter signs them with the pool's access key, a key of the pool
// kind enrolled for the node pool. They are signed, retried and refused with a
// [Refusal] exactly as the integration contract's requests above, and the claim answer
// names the revision it was built for, [ProvisionerRevision] here, in Revision, as
// [Changes] does. The machine redeems the run code for the run key and the run's
// documents.
//
// The run contract holds the run's documents, documents.schema.json: one object, whose
// Go type is [Documents], with [RunSettings] and [WorkItem] among its members' types. The object is the documents member of Forager's run answer: the machine
// receives it, signed, in the 201 of the run code's redemption, and Forager passes it
// on unread. It is a document, not a request.
//
// The explore contract holds the documents of the step explore: inputs.schema.json, the
// shape of the run's inputs for that step, run/v1 documents.schema.json, and
// result.schema.json, the run's final response, which the control plane reads from the
// result text of the Forager contract's dev.qory.runtime.result event. They are
// documents of a step, not requests, and are not signed.
//
// The machines contract holds the protocol of a machine adapter, a program that makes
// and deletes machines at one vendor, which the provisioner runs as `<adapter>
// <command> --settings FILE`. Each command, check, make, list, log and delete, reads one
// request on standard input, [MachinesCheck] say, and writes one answer on standard
// output, [MachinesCheckAnswer], or writes a [MachinesError] and exits with status 1.
// Its documents travel between two programs on one host, and are not signed.
//
// Each schema's $id is [Base] followed by the file's path under integration/v1,
// [ProvisionerBase] followed by its path under provisioner/v1, [RunBase] followed by
// its path under run/v1, [ExploreBase] followed by its path under explore/v1, or
// [MachinesBase] followed by its path under machines/v1, and [Compile],
// [CompileProvisioner], [CompileRun], [CompileExplore] and [CompileMachines] compile it
// by that path, with every embedded schema it refers to. A schema of the Forager
// contract that one of them refers to, whose $id starts with
// https://qory.dev/contracts/forager/v1/, is read from the forager module's
// contracts.FS, at the version go.mod pins. The tests in this package validate every
// fixture under integration/v1/fixtures, provisioner/v1/fixtures, run/v1/fixtures,
// explore/v1/fixtures and machines/v1/fixtures, so the schemas and the fixtures cannot
// drift apart.
package contracts

import (
	"bytes"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	forager "github.com/qoryai/forager/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Base is the $id every schema of the contract has, followed by the schema's path
// under integration/v1. It identifies the contract; nothing is fetched from it.
const Base = "https://qory.dev/contracts/integration/v1"

// ProvisionerBase is the $id every schema of the provisioner contract has, followed by
// the schema's path under provisioner/v1. It identifies the contract; nothing is
// fetched from it.
const ProvisionerBase = "https://qory.dev/contracts/provisioner/v1"

// RunBase is the $id every schema of the run contract has, followed by the schema's
// path under run/v1. It identifies the contract; nothing is fetched from it.
const RunBase = "https://qory.dev/contracts/run/v1"

// ExploreBase is the $id every schema of the explore contract has, followed by the
// schema's path under explore/v1. It identifies the contract; nothing is fetched from
// it.
const ExploreBase = "https://qory.dev/contracts/explore/v1"

// MachinesBase is the $id every schema of the machines contract has, followed by the
// schema's path under machines/v1. It identifies the contract; nothing is fetched from
// it.
const MachinesBase = "https://qory.dev/contracts/machines/v1"

//go:embed all:integration all:provisioner all:run all:explore all:machines
var embedded embed.FS

// FS is the contract directory, rooted at integration/v1: the README, the schema and
// the fixtures.
var FS = must(fs.Sub(embedded, "integration/v1"))

// ProvisionerFS is the provisioner contract's directory, rooted at provisioner/v1: the
// schemas and the fixtures.
var ProvisionerFS = must(fs.Sub(embedded, "provisioner/v1"))

// RunFS is the run contract's directory, rooted at run/v1: the schema and the
// fixtures.
var RunFS = must(fs.Sub(embedded, "run/v1"))

// ExploreFS is the explore contract's directory, rooted at explore/v1: the schemas and
// the fixtures.
var ExploreFS = must(fs.Sub(embedded, "explore/v1"))

// MachinesFS is the machines contract's directory, rooted at machines/v1: the schemas
// and the fixtures.
var MachinesFS = must(fs.Sub(embedded, "machines/v1"))

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
}{
	{Base, FS}, {ProvisionerBase, ProvisionerFS}, {RunBase, RunFS},
	{ExploreBase, ExploreFS}, {MachinesBase, MachinesFS},
}

// Compile compiles one schema of the contract by its path under integration/v1,
// "description.schema.json" say. A schema of the contract it refers to, such as
// definitions.schema.json, is read from [FS] by its $id, a schema of the Forager contract
// it refers to, such as auth.schema.json, from the forager module's contracts.FS by its
// $id, and the JSON Schema meta-schema a description's settings are checked against is
// the copy built into the compiler, so nothing is fetched.
func Compile(name string) (*jsonschema.Schema, error) { return compile(Base, FS, name) }

// CompileProvisioner compiles one schema of the provisioner contract by its path under
// provisioner/v1, "claim.schema.json" say. The path may end in a fragment,
// "definitions.schema.json#/$defs/dispatch_id" say, which compiles that definition. A
// schema it refers to, of this contract or of the integration contract, is read from
// [ProvisionerFS] or [FS] by its $id, so nothing is fetched.
func CompileProvisioner(name string) (*jsonschema.Schema, error) {
	return compile(ProvisionerBase, ProvisionerFS, name)
}

// CompileRun compiles one schema of the run contract by its path under run/v1,
// "documents.schema.json" say. The path may end in a fragment,
// "documents.schema.json#/$defs/target" say, which compiles that definition. A schema
// it refers to, of the integration contract or of the Forager contract, is read from
// [FS] or the forager module's contracts.FS by its $id, so nothing is fetched.
func CompileRun(name string) (*jsonschema.Schema, error) {
	return compile(RunBase, RunFS, name)
}

// CompileExplore compiles one schema of the explore contract by its path under
// explore/v1, "result.schema.json" say. A schema it refers to, of this contract or of
// the integration contract, is read from [ExploreFS] or [FS] by its $id, so nothing is
// fetched.
func CompileExplore(name string) (*jsonschema.Schema, error) {
	return compile(ExploreBase, ExploreFS, name)
}

// CompileMachines compiles one schema of the machines contract by its path under
// machines/v1, "make.schema.json" say. A schema it refers to, of this contract, of the
// provisioner contract or of the integration contract, is read from [MachinesFS],
// [ProvisionerFS] or [FS] by its $id, so nothing is fetched.
func CompileMachines(name string) (*jsonschema.Schema, error) {
	return compile(MachinesBase, MachinesFS, name)
}

// compile compiles the schema at name in fsys, whose $id is base followed by name. A
// fragment after # in name selects a part of that schema, such as a definition.
func compile(base string, fsys fs.FS, name string) (*jsonschema.Schema, error) {
	file, _, _ := strings.Cut(name, "#")
	doc, err := document(fsys, file)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(loader{})
	if err := c.AddResource(base+"/"+file, doc); err != nil {
		return nil, err
	}
	return c.Compile(base + "/" + name)
}

// loader reads a schema of an embedded contract by its $id, from [FS],
// [ProvisionerFS], [RunFS], [ExploreFS] or [MachinesFS], or a schema of the Forager contract by
// its $id, from the forager module's contracts.FS, and refuses any other URL.
type loader struct{}

// Load returns the document of a schema of an embedded contract, or of the Forager
// contract, by its URL. It reads forager.FS when it is called, not when the package
// starts.
func (loader) Load(url string) (any, error) {
	for _, t := range trees {
		if name, ok := strings.CutPrefix(url, t.base+"/"); ok {
			return document(t.fsys, name)
		}
	}
	if name, ok := strings.CutPrefix(url, forager.Base+"/"); ok {
		return document(forager.FS, name)
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

// RunDocument reads one JSON file of the run contract, by its path under run/v1, as
// [Document] reads one of the integration contract.
func RunDocument(name string) (any, error) { return document(RunFS, name) }

// ExploreDocument reads one JSON file of the explore contract, by its path under
// explore/v1, as [Document] reads one of the integration contract.
func ExploreDocument(name string) (any, error) { return document(ExploreFS, name) }

// MachinesDocument reads one JSON file of the machines contract, by its path under
// machines/v1, as [Document] reads one of the integration contract.
func MachinesDocument(name string) (any, error) { return document(MachinesFS, name) }

// document reads the JSON file at name in fsys.
func document(fsys fs.FS, name string) (any, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}
