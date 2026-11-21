package contracts_test

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	foragercontracts "github.com/qoryai/forager/contracts"
	"github.com/qoryai/forager/policy"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// foragerCredentialPath is the pattern of $defs.credential_paths in Forager's
// run-configuration.schema.json, read from the Forager module: the grammar of the paths
// of a credential in a run configuration, where { and } are allowed only as the whole
// segment {}. A filled path must match it.
var foragerCredentialPath = func() *regexp.Regexp {
	doc, err := foragercontracts.Document("run-configuration.schema.json")
	if err != nil {
		panic(err)
	}
	pattern := doc.(map[string]any)["$defs"].(map[string]any)["credential_paths"].(map[string]any)["items"].(map[string]any)["pattern"].(string)
	return regexp.MustCompile(pattern)
}()

// TestASourceApplyItemIsRefusedForTheRuleInItsName pins the reason of each invalid
// description whose source apply item breaks a rule, since a refusal by pattern or by
// an if/then rule does not state the rule: the same document is accepted once the one
// part its name points to is fixed.
func TestASourceApplyItemIsRefusedForTheRuleInItsName(t *testing.T) {
	schema := compile(t, "description.schema.json")
	for name, fix := range map[string][2]string{
		"fixtures/invalid/description-source-apply-host-wildcard.json":                {`"*.example.com"`, `"api.example.com"`},
		"fixtures/invalid/description-source-apply-header-without-header.json":        {`"scheme": "header"`, `"scheme": "header", "header": "X-Api-Key"`},
		"fixtures/invalid/description-source-apply-username-with-bearer.json":         {`"scheme": "bearer"`, `"scheme": "basic"`},
		"fixtures/invalid/description-source-apply-path-two-placeholders.json":        {`/{target.path}/{target.path}"`, `/{target.path}/files"`},
		"fixtures/invalid/description-source-apply-path-star-before-placeholder.json": {`/*/{target.path}"`, `/{target.path}/*"`},
	} {
		b := read(t, name)
		fixed := bytes.Replace(b, []byte(fix[0]), []byte(fix[1]), 1)
		if bytes.Equal(fixed, b) {
			t.Errorf("%s contains no %s", name, fix[0])
		}
		if err := validate(schema, b); err == nil {
			t.Errorf("%s is accepted", name)
		}
		if err := validate(schema, fixed); err != nil {
			t.Errorf("%s, fixed, is refused: %v", name, err)
		}
	}
}

// TestAPathTemplateAndItsFilledFormFollowForagersPathGrammar pins the pattern of a
// source apply path template: it accepts a path in Forager's path grammar with no
// space character, {target.path} at most once and no other { or }, a / right before
// {target.path}, and after it a fixed suffix with no *, {, } or /, if any, then the
// end or a /; and each accepted template, filled with a target path that has no .., //,
// *, ?, # or space character, is in that grammar too.
func TestAPathTemplateAndItsFilledFormFollowForagersPathGrammar(t *testing.T) {
	schema := compile(t, "description.schema.json#/$defs/apply/properties/paths/items")
	for _, tc := range []struct {
		template string
		accept   bool
	}{
		{"/", true},
		{"/*", true},
		{"/user", true},
		{"/projects/{target.path}", true},
		{"/projects/{target.path}/*", true},
		{"/projects/{target.path}/", true},
		{"/{target.path}", true},
		{"/{target.path}/*", true},
		{"/v1/{target.path}/files/*", true},
		{"/repos/{target.path}", true},
		{"/files/{target.path}.v2/*", true},
		{"/{target.path}.v2", true},
		{"/{target.path}.v2/", true},
		{"/projects/{target.path}.json", true},
		{"/projects/{target.path}-v2/files/*", true},
		{"/a{b", false},
		{"/a}b", false},
		{"/projects/{target.path}*", false},
		{"/projects/{target.path}-*", false},
		{"/{target.path}.v2*", false},
		{"/{target.path}.v2{x}", false},
		{"/projects{target.path}", false},
		{"/projects/x{target.path}/*", false},
		{"/a\tb", false},
		{"/a\x7fb", false},
		{"/a\u0085b", false},
		{"/a\u00a0b", false},
		{"/a\u1680b", false},
		{"/a\u2009b", false},
		{"/a\u2028b", false},
		{"/a\u202fb", false},
		{"/a\u3000b", false},
		{"/a\ufeffb", false},
		{"", false},
		{"{target.path}", false},
		{"projects/{target.path}", false},
		{"/projects/{target.path}/{target.path}", false},
		{"/projects/{target.path}{target.path}", false},
		{"/projects/*/{target.path}", false},
		{"/projects/{target.path}/**", false},
		{"/projects/{target.path}?ref=1", false},
		{"/projects/{target.path}#top", false},
		{"/projects/{target.path} x", false},
		{"/projects/{target}", false},
		{"/projects/{path}", false},
		{"/projects/{target.path", false},
		{"/projects/target.path}", false},
		{"/projects/{}", false},
	} {
		err := schema.Validate(tc.template)
		if (err == nil) != tc.accept {
			t.Errorf("%q: accepted %v, want %v: %v", tc.template, err == nil, tc.accept, err)
			continue
		}
		if !tc.accept {
			if v, ok := err.(*jsonschema.ValidationError); !ok || strings.Join(leaves(v), "; ") != "/ pattern" {
				t.Errorf("%q is refused for %v, want / pattern", tc.template, err)
			}
			continue
		}
		for _, target := range []string{"acme", "acme/widgets", "acme/widgets.v2", "acme/reports", "a-b_c~d", "acme/.hidden", "acme/a%20b"} {
			filled := strings.Replace(tc.template, "{target.path}", target, 1)
			if !foragerCredentialPath.MatchString(filled) {
				t.Errorf("the template %q filled with %q is %q, which is not in the grammar of credential_paths of Forager", tc.template, target, filled)
			}
		}
	}
}

// TestFilledPathsLeaveOutAPathTheyAlreadyHave pins why the control plane leaves out a
// filled path that equals an earlier filled path of the same item: two templates that
// the schema accepts, none twice, can fill to the same path, and the filled paths with
// that path left out are valid as $defs.credential_paths of the Forager module, in its grammar and none twice.
func TestFilledPathsLeaveOutAPathTheyAlreadyHave(t *testing.T) {
	templates := compile(t, "description.schema.json#/$defs/apply/properties/paths")
	c, err := foragercontracts.Compiler()
	if err != nil {
		t.Fatal(err)
	}
	forager, err := c.Compile(foragercontracts.Base + "/run-configuration.schema.json#/$defs/credential_paths")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		paths  []any
		target string
		want   []any
	}{
		{[]any{"/projects/{target.path}", "/projects/acme"}, "acme", []any{"/projects/acme"}},
		{[]any{"/{target.path}/a", "/a/{target.path}"}, "a", []any{"/a/a"}},
		{[]any{"/projects/{target.path}", "/projects/{target.path}/*", "/user"}, "acme/widgets", []any{"/projects/acme/widgets", "/projects/acme/widgets/*", "/user"}},
	} {
		if err := templates.Validate(tc.paths); err != nil {
			t.Errorf("%v: the schema refuses the templates: %v", tc.paths, err)
		}
		var filled, kept []any
		for _, p := range tc.paths {
			f := strings.Replace(p.(string), "{target.path}", tc.target, 1)
			filled = append(filled, f)
			if !slices.Contains(kept, any(f)) {
				kept = append(kept, f)
			}
		}
		if !slices.Equal(kept, tc.want) {
			t.Errorf("%v filled with %q: %v, want %v", tc.paths, tc.target, kept, tc.want)
		}
		if err := forager.Validate(kept); err != nil {
			t.Errorf("%v filled with %q, with each repeated path left out, is %v, which Forager refuses: %v", tc.paths, tc.target, kept, err)
		}
		if len(kept) < len(filled) && forager.Validate(filled) == nil {
			t.Errorf("%v filled with %q is %v, and Forager accepts it with a path twice", tc.paths, tc.target, filled)
		}
	}
}

// applyItem is one description item of the source apply for the helper below: the host,
// scheme, username and header, and the path templates. shortcut: the helper models one
// host for each item, since the text does not say whether a path that is covered on one
// host of an item is dropped for its other hosts; replace it when the text says.
type applyItem struct {
	host, scheme, username, header string
	templates                      []string
}

// exploreItem is one item of the run configuration that the helper makes: the paths of
// one chunk of the filled paths of a description item.
type exploreItem struct {
	from                           int
	host, scheme, username, header string
	paths                          []string
}

// fillExploreCall makes the items of the run configuration for a run with no target, as
// $defs.apply of description.schema.json says. Each template of an item is filled for
// every resource in the order of the call's resources, one template after another. Before
// the filled paths are split into items of at most 32 paths, a group, the paths of the
// items that share host, scheme, username and header, keeps the first of two equal paths
// and drops each path that another path of the group covers. A call is refused when an
// equal, covered or overlapping path stays on one host between two groups. Forager's
// policy package decides what covers and what overlaps. An item whose paths are all
// dropped has no chunk.
func fillExploreCall(items []applyItem, resources []string) (out []exploreItem, refused bool) {
	const chunk = 32
	type entry struct {
		item int
		path string
	}
	type group struct {
		host    string
		entries []entry
		kept    []string
	}
	var groups []*group
	byKey := map[string]*group{}
	for i, item := range items {
		key := strings.Join([]string{item.host, item.scheme, item.username, item.header}, "\x00")
		g := byKey[key]
		if g == nil {
			g = &group{host: item.host}
			byKey[key] = g
			groups = append(groups, g)
		}
		for _, template := range item.templates {
			for _, resource := range resources {
				g.entries = append(g.entries, entry{i, strings.Replace(template, "{target.path}", resource, 1)})
			}
		}
	}
	kept := make([][]string, len(items))
	for _, g := range groups {
		var seen []string
		for _, e := range g.entries {
			if slices.Contains(seen, e.path) {
				continue
			}
			seen = append(seen, e.path)
			if !slices.ContainsFunc(g.entries, func(other entry) bool { return other.path != e.path && policy.CoversPath(other.path, e.path) }) {
				kept[e.item] = append(kept[e.item], e.path)
				g.kept = append(g.kept, e.path)
			}
		}
	}
	for i, g := range groups {
		for _, other := range groups[:i] {
			if g.host != other.host {
				continue
			}
			for _, a := range g.kept {
				for _, b := range other.kept {
					if policy.OverlapPaths(a, b) {
						return nil, true
					}
				}
			}
		}
	}
	for i, paths := range kept {
		for len(paths) > 0 {
			n := min(chunk, len(paths))
			out = append(out, exploreItem{i, items[i].host, items[i].scheme, items[i].username, items[i].header, paths[:n]})
			paths = paths[n:]
		}
	}
	return out, false
}

// fillers returns count resources, numbered from 1, with the resources of at in the places
// that it gives and distinct filler resources elsewhere.
func fillers(count int, at map[int]string) []string {
	var out []string
	for i := 1; i <= count; i++ {
		if r, ok := at[i]; ok {
			out = append(out, r)
		} else {
			out = append(out, fmt.Sprintf("example/project-%02d", i))
		}
	}
	return out
}

// TestAnExploreCallDropsCoveredPathsBeforeItSplitsThem pins the rule of $defs.apply about
// the filled paths of a call: a path that another path of its group covers is dropped
// before the split into items of at most 32 paths, of two equal paths the first stays, and
// an equal, covered or overlapping path between two groups of one host is refused. A group
// is the items that share host, scheme, username and header.
func TestAnExploreCallDropsCoveredPathsBeforeItSplitsThem(t *testing.T) {
	bearer := func(templates ...string) applyItem {
		return applyItem{host: "code.example.com", scheme: "bearer", templates: templates}
	}
	header := func(templates ...string) applyItem {
		return applyItem{host: "code.example.com", scheme: "header", header: "X-Api-Key", templates: templates}
	}
	basicAs := func(username string, templates ...string) applyItem {
		return applyItem{host: "code.example.com", scheme: "basic", username: username, templates: templates}
	}
	headerAs := func(name string, templates ...string) applyItem {
		return applyItem{host: "code.example.com", scheme: "header", header: name, templates: templates}
	}
	other := func(templates ...string) applyItem {
		return applyItem{host: "other.example.com", scheme: "bearer", templates: templates}
	}
	// The fortieth resource is covered by the thirty-second, which is in the first item.
	forty := fillers(40, map[int]string{32: "group", 40: "group/sub/repo"})
	var kept, others []string
	for _, r := range forty {
		if r != "group/sub/repo" {
			kept = append(kept, "/"+r+"/*")
		}
		others = append(others, "/other/"+r)
	}
	firstChunk, secondChunk, otherFirst, otherSecond := kept[:32], kept[32:], others[:32], others[32:]
	for _, tc := range []struct {
		name      string
		items     []applyItem
		resources []string
		want      []exploreItem
		refused   bool
	}{
		{name: "a path under a path with a star is dropped", items: []applyItem{bearer("/{target.path}/*")}, resources: []string{"group", "group/sub/repo"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/group/*"}}}},
		{name: "the order of the resources does not matter", items: []applyItem{bearer("/{target.path}/*")}, resources: []string{"group/sub/repo", "group"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/group/*"}}}},
		{name: "a path without a star does not cover a longer path", items: []applyItem{bearer("/{target.path}")}, resources: []string{"group", "group/sub"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/group", "/group/sub"}}}},
		{name: "of two equal paths of one item the first stays", items: []applyItem{bearer("/projects/{target.path}", "/projects/acme")}, resources: []string{"acme"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/projects/acme"}}}},
		{name: "of two equal paths of two items of one group the first stays, and the second item has no chunk", items: []applyItem{bearer("/user"), bearer("/user")}, resources: []string{"a", "b"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/user"}}}},
		{name: "an item whose paths another item of its group covers has no chunk", items: []applyItem{bearer("/{target.path}/*"), bearer("/{target.path}/issues")}, resources: []string{"group"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/group/*"}}}},
		{name: "a template without the target path gives one path", items: []applyItem{bearer("/user")}, resources: []string{"a", "b"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/user"}}}},
		{name: "a covered path that falls in the second chunk is dropped before the split", items: []applyItem{bearer("/{target.path}/*"), header("/other/{target.path}")}, resources: forty,
			want: []exploreItem{
				{0, "code.example.com", "bearer", "", "", firstChunk},
				{0, "code.example.com", "bearer", "", "", secondChunk},
				{1, "code.example.com", "header", "", "X-Api-Key", otherFirst},
				{1, "code.example.com", "header", "", "X-Api-Key", otherSecond},
			}},
		{name: "an equal path in two groups of one host is refused", items: []applyItem{bearer("/user"), basicAs("robot", "/user")}, resources: []string{"a"}, refused: true},
		{name: "an equal path in two groups that differ in the username is refused", items: []applyItem{basicAs("robot", "/user"), basicAs("clerk", "/user")}, resources: []string{"a"}, refused: true},
		{name: "a covered path in two groups that differ in the username is refused", items: []applyItem{basicAs("robot", "/{target.path}/*"), basicAs("clerk", "/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "an equal path in two groups that differ in the header is refused", items: []applyItem{headerAs("X-Api-Key", "/user"), headerAs("X-Token", "/user")}, resources: []string{"a"}, refused: true},
		{name: "a covered path in two groups that differ in the header is refused", items: []applyItem{headerAs("X-Api-Key", "/{target.path}/*"), headerAs("X-Token", "/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "a covered path in two groups of one host is refused", items: []applyItem{bearer("/{target.path}/*"), header("/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "a path in two groups of one host that do not overlap is kept", items: []applyItem{bearer("/a/{target.path}"), header("/b/{target.path}")}, resources: []string{"x"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/a/x"}}, {1, "code.example.com", "header", "", "X-Api-Key", []string{"/b/x"}}}},
		{name: "an equal path in two groups of two hosts is kept", items: []applyItem{bearer("/user"), other("/user")}, resources: []string{"a"},
			want: []exploreItem{{0, "code.example.com", "bearer", "", "", []string{"/user"}}, {1, "other.example.com", "bearer", "", "", []string{"/user"}}}},
	} {
		got, refused := fillExploreCall(tc.items, tc.resources)
		if refused != tc.refused || !slices.EqualFunc(got, tc.want, func(a, b exploreItem) bool {
			return a.from == b.from && a.host == b.host && a.scheme == b.scheme && a.username == b.username && a.header == b.header && slices.Equal(a.paths, b.paths)
		}) {
			t.Errorf("%s: got %d items, refused %v; want %d items, refused %v\n got %v\nwant %v", tc.name, len(got), refused, len(tc.want), tc.refused, got, tc.want)
		}
		for _, item := range got {
			if len(item.paths) == 0 || len(item.paths) > 32 {
				t.Errorf("%s: an item has %d paths, want 1 to 32", tc.name, len(item.paths))
			}
			for _, p := range item.paths {
				if !foragerCredentialPath.MatchString(p) {
					t.Errorf("%s: the path %q is not in the grammar of credential_paths of Forager", tc.name, p)
				}
			}
		}
	}
}

// TestPathsWithNoSegmentBraceOverlapOnlyWhenOneCoversTheOther pins why a call is never
// refused for paths that overlap and neither covers the other: no target path and no
// template has a { or }, so no filled path has a {} segment, and for paths with none
// Forager's OverlapPaths agrees with CoversPath in either direction.
func TestPathsWithNoSegmentBraceOverlapOnlyWhenOneCoversTheOther(t *testing.T) {
	paths := []string{"/", "/*", "/a", "/a/*", "/a/b", "/a/b/*", "/a/bc", "/a/b*", "/b/*", "/a/b/c", "/ab/*"}
	for _, a := range paths {
		for _, b := range paths {
			if got, want := policy.OverlapPaths(a, b), policy.CoversPath(a, b) || policy.CoversPath(b, a); got != want {
				t.Errorf("OverlapPaths(%q, %q) is %v, and CoversPath in either direction is %v", a, b, got, want)
			}
		}
	}
}

// TestAnAddressPathHasNoBrace pins $defs.address_path of definitions.schema.json: a { or
// } is refused anywhere in the path, so a filled path template has no segment {} that
// Forager reads as a wildcard, and no { or } that Forager refuses.
func TestAnAddressPathHasNoBrace(t *testing.T) {
	schema := compile(t, "definitions.schema.json#/$defs/address_path")
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"acme/shop", true},
		{"acme/shop.v2~x", true},
		{"acme/%7Bshop", true},
		{"acme/{}/x", false},
		{"{}", false},
		{"acme/{shop", false},
		{"acme/shop}", false},
		{"acme/{target.path}", false},
	} {
		if err := schema.Validate(tc.path); (err == nil) != tc.ok {
			t.Errorf("the address path %q: accepted %v, want %v", tc.path, err == nil, tc.ok)
		}
	}
}
