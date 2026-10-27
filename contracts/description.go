package contracts

import (
	"fmt"
	"net/netip"
	"net/url"
	"regexp"
	"slices"
	"strconv"

	"github.com/qoryai/integrations/internal/rawjson"
)

// The limits of a description that description.schema.json states in words, since a
// schema cannot express them.
const (
	// DescriptionMaxBytes is the most bytes a description's raw document takes, 256 KiB.
	DescriptionMaxBytes = 262144
	// DescriptionMaxStringBytes is the most bytes of UTF-8 a string of a description,
	// a member name included, holds.
	DescriptionMaxStringBytes = 16384
	// DescriptionMaxValues is the most JSON values a description holds in all: the
	// document itself, each array entry and each member's value count one each, and
	// member names none.
	DescriptionMaxValues = 4096
	// DescriptionMaxDepth is the most levels arrays and objects nest in a description,
	// the top-level object being level 1.
	DescriptionMaxDepth = 64
)

// Guide is the guide of a description's source role, roles.source.guide in
// description.schema.json: short step-by-step help for the set-up step, Setup, and for
// the connect step, Connect, each nil when absent. The control plane shows each value
// verbatim as plain text, never as Markdown or HTML. For each part, it shows Before,
// then a first step that the control plane writes, then the adapter's Steps, numbered
// after that first step, then After. The adapter's steps describe only the adapter's
// system.
type Guide struct {
	Setup   *GuidePart `json:"setup,omitempty"`
	Connect *GuidePart `json:"connect,omitempty"`
}

// GuidePart is the help for one step of a [Guide]: Before, a line shown above the
// steps, and After, a line shown below them, each empty when absent; and Steps, 1 to 10
// lines in the order a person takes them. Each value is a prose line of
// description.schema.json, $defs.line: 1 to 300 characters on one line.
type GuidePart struct {
	Before string   `json:"before,omitempty"`
	Steps  []string `json:"steps"`
	After  string   `json:"after,omitempty"`
}

// CheckDescription reports the first rule of description.schema.json that a
// description's raw bytes break and that no schema can express: the bytes are at most
// [DescriptionMaxBytes] and are UTF-8; no string holds a \u escape of a lone surrogate;
// no object repeats a member, at any depth; no string, a member name included, holds
// more than [DescriptionMaxStringBytes] bytes of UTF-8; the document holds at most
// [DescriptionMaxValues] JSON values; and it nests arrays and objects at most
// [DescriptionMaxDepth] levels deep. It checks b before any decoder reads it leniently,
// and leaves the schema's rules to the schema.
func CheckDescription(b []byte) error {
	if len(b) > DescriptionMaxBytes {
		return fmt.Errorf("the document is %d bytes, more than %d (256 KiB)", len(b), DescriptionMaxBytes)
	}
	if err := rawjson.Text(b); err != nil {
		return err
	}
	return rawjson.Check(b, rawjson.Rules{
		MaxStringBytes: DescriptionMaxStringBytes,
		MaxValues:      DescriptionMaxValues,
		MaxDepth:       DescriptionMaxDepth,
	})
}

// CheckModelProvider reports the first rule of the model_provider role of
// description.schema.json that a description breaks and that the schema does not
// express: no two entries of apis have the same format; an API endpoint that is a URL
// has a host and port as [endpointHost] reads them; an API endpoint that is a setting
// is the name of a property of settings whose type is string and that is not marked
// writeOnly; and no two entries of models have the same id. doc is the
// description as a schema validates it, after the schema accepts it. A description
// with no model_provider role keeps every rule.
func CheckModelProvider(doc any) error {
	d, _ := doc.(map[string]any)
	roles, _ := d["roles"].(map[string]any)
	role, ok := roles["model_provider"].(map[string]any)
	if !ok {
		return nil
	}
	apis, _ := role["apis"].([]any)
	formats := map[string]int{}
	for i, a := range apis {
		api, _ := a.(map[string]any)
		format, _ := api["format"].(string)
		if j, ok := formats[format]; ok {
			return fmt.Errorf("the model_provider apis %d and %d have the same format %q; no two APIs of a provider have the same format", j, i, format)
		}
		formats[format] = i
	}
	for i, a := range apis {
		api, _ := a.(map[string]any)
		if endpoint, ok := api["endpoint"].(string); ok {
			if _, err := endpointHost(endpoint, i); err != nil {
				return err
			}
		}
	}
	settings, _ := d["settings"].(map[string]any)
	props, _ := settings["properties"].(map[string]any)
	for i, a := range apis {
		api, _ := a.(map[string]any)
		endpoint, ok := api["endpoint"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := endpoint["setting"].(string)
		p, ok := props[name].(map[string]any)
		switch {
		case !ok:
			return fmt.Errorf("the endpoint of model_provider apis[%d] is the setting %q, and settings has no property %q", i, name, name)
		case p["type"] != "string":
			return fmt.Errorf("the endpoint of model_provider apis[%d] is the setting %q, whose type is not string", i, name)
		case p["writeOnly"] == true:
			return fmt.Errorf("the endpoint of model_provider apis[%d] is the setting %q, which is marked writeOnly; an endpoint setting is never a secret", i, name)
		}
	}
	models, _ := role["models"].([]any)
	first := map[string]int{}
	for i, m := range models {
		entry, _ := m.(map[string]any)
		id, _ := entry["id"].(string)
		if j, ok := first[id]; ok {
			return fmt.Errorf("the model_provider models %d and %d have the same id %q; no two models have the same id", j, i, id)
		}
		first[id] = i
	}
	return nil
}

// CheckEgressHosts reports the first rule of egress_hosts of description.schema.json
// that a description breaks and that the schema does not express: the setting of each
// setting entry is a property of settings whose type is string and that is not marked
// writeOnly; each host in roles.credential.hosts, each host of the items of
// roles.source.apply, and the host of each roles.model_provider.apis endpoint that is a
// URL, has an equal entry; and an API endpoint that is a setting has an entry with the
// same setting. doc is the description as a schema validates it, after the schema
// accepts it. A description with no credential hosts, no source apply items and no
// model_provider APIs keeps every rule without egress_hosts.
func CheckEgressHosts(doc any) error {
	d, _ := doc.(map[string]any)
	entries, _ := d["egress_hosts"].([]any)
	settings, _ := d["settings"].(map[string]any)
	props, _ := settings["properties"].(map[string]any)
	var hosts []string
	fromSetting := map[string]bool{}
	for _, e := range entries {
		switch e := e.(type) {
		case string:
			hosts = append(hosts, e)
		case map[string]any:
			name, _ := e["setting"].(string)
			p, ok := props[name].(map[string]any)
			switch {
			case !ok:
				return fmt.Errorf("the setting %q of an egress_hosts entry is not a property of settings", name)
			case p["type"] != "string":
				return fmt.Errorf("the setting %q of an egress_hosts entry is not of type string", name)
			case p["writeOnly"] == true:
				return fmt.Errorf("the setting %q of an egress_hosts entry is marked writeOnly; the setting of an egress_hosts entry is never a secret", name)
			}
			fromSetting[name] = true
		}
	}
	roles, _ := d["roles"].(map[string]any)
	credential, _ := roles["credential"].(map[string]any)
	listed, _ := credential["hosts"].([]any)
	for _, h := range listed {
		host, _ := h.(string)
		if !slices.Contains(hosts, host) {
			return fmt.Errorf("add %q to egress_hosts: the credential host %q has no equal entry there, and each host in a role's hosts needs one", host, host)
		}
	}
	source, _ := roles["source"].(map[string]any)
	apply, _ := source["apply"].([]any)
	for i, a := range apply {
		item, _ := a.(map[string]any)
		itemHosts, _ := item["hosts"].([]any)
		for _, h := range itemHosts {
			host, _ := h.(string)
			if !slices.Contains(hosts, host) {
				return fmt.Errorf("add %q to egress_hosts: the host %q of source apply[%d] has no equal entry there, and each host of an apply item needs one", host, host, i)
			}
		}
	}
	provider, _ := roles["model_provider"].(map[string]any)
	apis, _ := provider["apis"].([]any)
	for i, a := range apis {
		api, _ := a.(map[string]any)
		switch endpoint := api["endpoint"].(type) {
		case string:
			host, err := endpointHost(endpoint, i)
			if err != nil {
				return err
			}
			if !slices.Contains(hosts, host) {
				return fmt.Errorf("add %q to egress_hosts: the host %q of the endpoint of model_provider apis[%d] has no equal entry there, and each API endpoint host needs one", host, host, i)
			}
		case map[string]any:
			name, _ := endpoint["setting"].(string)
			if !fromSetting[name] {
				return fmt.Errorf(`the endpoint of model_provider apis[%d] is the setting %q, and egress_hosts has no entry {"setting": %q}; an API endpoint that is a setting has an egress_hosts entry with the same setting`, i, name, name)
			}
		}
	}
	return nil
}

// exactHost matches $defs/host of description.schema.json: lower-case labels joined by
// dots, each 1 to 63 letters, digits and hyphens that starts and ends with a letter or a
// digit.
var exactHost = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// numberLabel is a last label that makes a host a number, as the WHATWG URL Standard
// tests it: all digits, or 0x followed by hex digits or none.
var numberLabel = regexp.MustCompile(`(^|\.)([0-9]+|0x[0-9a-f]*)$`)

// endpointHost returns the host of endpoint, the URL endpoint of model_provider
// apis[i], or an error that says why it has none. The host is in exactHost's form, and
// a host whose last label is a number is a canonical IPv4 address, four decimal parts
// from 0 to 255 with no leading zero; a port, when one is written, is from 1 to 65535.
// These hosts are a subset of the hosts Forager's policy.EndpointHost reads, so a
// gateway reads the host of every endpoint this accepts.
func endpointHost(endpoint string, i int) (string, error) {
	fail := func(why string) error {
		return fmt.Errorf("the endpoint %q of model_provider apis[%d] has no host a gateway can read: %s", endpoint, i, why)
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", fail(err.Error())
	}
	host := u.Hostname()
	if !exactHost.MatchString(host) {
		return "", fail(fmt.Sprintf("the host %q is not labels joined by dots, each 1 to 63 lower-case letters, digits and hyphens that starts and ends with a letter or a digit", host))
	}
	if numberLabel.MatchString(host) {
		if a, err := netip.ParseAddr(host); err != nil || !a.Is4() {
			return "", fail(fmt.Sprintf("the host %q ends in a number and is not an IPv4 address of four decimal parts from 0 to 255, each with no leading zero", host))
		}
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return "", fail(fmt.Sprintf("the port %q is not a number from 1 to 65535", port))
		}
	}
	return host, nil
}
