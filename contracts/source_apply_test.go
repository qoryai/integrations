package contracts_test

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode"

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
		for _, target := range []string{"acme", "acme/widgets", "acme/widgets.v2", "acme/reports", "a-b_c~d", "acme/.hidden", "acme/a-b"} {
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

// applyItem is one description item of the source apply for the helper below: its hosts,
// scheme, username and header, and its path templates.
type applyItem struct {
	hosts                    []string
	scheme, username, header string
	templates                []string
}

// exploreItem is one item of the run configuration that the helper makes: one chunk of
// the filled paths of the description item with the index from.
type exploreItem struct {
	from                     int
	hosts                    []string
	scheme, username, header string
	paths                    []string
}

// fillExploreCall makes the items of the run configuration for a run with no target. See
// $defs.apply in description.schema.json. It fills each template of an item for every
// resource in the order of the call's resources, one template after another. The paths of
// the items that share host, scheme, username and header are a group, one for each host of
// an item. Before the split into items of at most 32 paths, a path is dropped when another
// path of its group covers it. A path is dropped too when an equal path comes earlier: the
// earlier item first, then the earlier path of one item. A path is dropped only when that
// holds on every host of its item. The adapter refuses the call when a path is covered on
// some hosts and not on others. It refuses the call too for an equal, covered or
// overlapping path between two groups of one host. Forager's policy package decides what
// covers and what overlaps. An item whose paths are all dropped has no chunk.
func fillExploreCall(items []applyItem, resources []string) (out []exploreItem, refused bool) {
	const chunk = 32
	type entry struct {
		item, index int
		path        string
	}
	type group struct {
		host    string
		entries []entry
		kept    []string
	}
	var groups []*group
	byKey := map[string]*group{}
	groupOf := func(item applyItem, host string) *group {
		key := strings.Join([]string{host, item.scheme, item.username, item.header}, "\x00")
		g := byKey[key]
		if g == nil {
			g = &group{host: host}
			byKey[key] = g
			groups = append(groups, g)
		}
		return g
	}
	filled := make([][]string, len(items))
	for i, item := range items {
		for _, template := range item.templates {
			for _, resource := range resources {
				filled[i] = append(filled[i], strings.Replace(template, "{target.path}", resource, 1))
			}
		}
		for _, host := range item.hosts {
			g := groupOf(item, host)
			for k, p := range filled[i] {
				g.entries = append(g.entries, entry{i, k, p})
			}
		}
	}
	// coveredOn reports whether the path at index k of item i is covered in its group on host.
	coveredOn := func(i, k int, host string) bool {
		g := groupOf(items[i], host)
		p := filled[i][k]
		for _, e := range g.entries {
			if e.path != p && policy.CoversPath(e.path, p) {
				return true
			}
		}
		for _, e := range g.entries {
			if e.item == i && e.index == k {
				return false // the first entry of the path in the group is this one
			}
			if e.path == p {
				return true
			}
		}
		return false
	}
	kept := make([][]string, len(items))
	for i, item := range items {
		for k, p := range filled[i] {
			covered := 0
			for _, host := range item.hosts {
				if coveredOn(i, k, host) {
					covered++
				}
			}
			switch {
			case covered == 0:
				kept[i] = append(kept[i], p)
				for _, host := range item.hosts {
					g := groupOf(item, host)
					g.kept = append(g.kept, p)
				}
			case covered < len(item.hosts):
				return nil, true
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
			out = append(out, exploreItem{i, items[i].hosts, items[i].scheme, items[i].username, items[i].header, paths[:n]})
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
// the filled paths of a call. A path is dropped before the split into items of at most 32
// paths when a path of its group covers it on every host of its item. The adapter refuses
// the call when a path is covered on some hosts only. Of two equal paths the first of the
// description stays. The adapter also refuses an equal, covered or overlapping path
// between two groups of one host. A group is the items that share host, scheme, username
// and header on a host.
func TestAnExploreCallDropsCoveredPathsBeforeItSplitsThem(t *testing.T) {
	const a, b = "code.example.com", "other.example.com"
	bearer := func(templates ...string) applyItem {
		return applyItem{hosts: []string{a}, scheme: "bearer", templates: templates}
	}
	bearerOn := func(hosts []string, templates ...string) applyItem {
		return applyItem{hosts: hosts, scheme: "bearer", templates: templates}
	}
	header := func(templates ...string) applyItem {
		return applyItem{hosts: []string{a}, scheme: "header", header: "X-Api-Key", templates: templates}
	}
	basicAs := func(username string, templates ...string) applyItem {
		return applyItem{hosts: []string{a}, scheme: "basic", username: username, templates: templates}
	}
	headerAs := func(name string, templates ...string) applyItem {
		return applyItem{hosts: []string{a}, scheme: "header", header: name, templates: templates}
	}
	other := func(templates ...string) applyItem {
		return applyItem{hosts: []string{b}, scheme: "bearer", templates: templates}
	}
	one := func(from int, scheme string, paths ...string) exploreItem {
		return exploreItem{from: from, hosts: []string{a}, scheme: scheme, paths: paths}
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
			want: []exploreItem{one(0, "bearer", "/group/*")}},
		{name: "the order of the resources does not matter", items: []applyItem{bearer("/{target.path}/*")}, resources: []string{"group/sub/repo", "group"},
			want: []exploreItem{one(0, "bearer", "/group/*")}},
		{name: "a path without a star does not cover a longer path", items: []applyItem{bearer("/{target.path}")}, resources: []string{"group", "group/sub"},
			want: []exploreItem{one(0, "bearer", "/group", "/group/sub")}},
		{name: "of two equal paths of one item the first stays", items: []applyItem{bearer("/projects/{target.path}", "/projects/acme")}, resources: []string{"acme"},
			want: []exploreItem{one(0, "bearer", "/projects/acme")}},
		{name: "of two equal paths of two items of one group the earlier item keeps it, and the later item has no chunk", items: []applyItem{bearer("/user"), bearer("/user")}, resources: []string{"a", "b"},
			want: []exploreItem{one(0, "bearer", "/user")}},
		{name: "an item whose paths another item of its group covers has no chunk", items: []applyItem{bearer("/{target.path}/*"), bearer("/{target.path}/issues")}, resources: []string{"group"},
			want: []exploreItem{one(0, "bearer", "/group/*")}},
		{name: "a template without the target path gives one path", items: []applyItem{bearer("/user")}, resources: []string{"a", "b"},
			want: []exploreItem{one(0, "bearer", "/user")}},
		{name: "a covered path that falls in the second chunk is dropped before the split", items: []applyItem{bearer("/{target.path}/*"), header("/other/{target.path}")}, resources: forty,
			want: []exploreItem{
				one(0, "bearer", firstChunk...),
				one(0, "bearer", secondChunk...),
				{from: 1, hosts: []string{a}, scheme: "header", header: "X-Api-Key", paths: otherFirst},
				{from: 1, hosts: []string{a}, scheme: "header", header: "X-Api-Key", paths: otherSecond},
			}},
		{name: "an item covered on one of its two hosts is refused", items: []applyItem{bearerOn([]string{a, b}, "/{target.path}/issues"), bearer("/{target.path}/*")}, resources: []string{"group"}, refused: true},
		{name: "an item covered on both of its hosts is dropped", items: []applyItem{bearerOn([]string{a, b}, "/{target.path}/issues"), bearerOn([]string{a, b}, "/{target.path}/*")}, resources: []string{"group"},
			want: []exploreItem{{from: 1, hosts: []string{a, b}, scheme: "bearer", paths: []string{"/group/*"}}}},
		{name: "an item on hosts a and b with /x/issues, an item on host a with /x/*, an item on host b with /x/*: the first item is dropped", items: []applyItem{bearerOn([]string{a, b}, "/x/issues"), bearer("/x/*"), other("/x/*")}, resources: []string{"r"},
			want: []exploreItem{one(1, "bearer", "/x/*"), {from: 2, hosts: []string{b}, scheme: "bearer", paths: []string{"/x/*"}}}},
		{name: "an item on host a with /x, then an item on hosts a and b with /x: the call is refused", items: []applyItem{bearer("/x"), bearerOn([]string{a, b}, "/x")}, resources: []string{"r"}, refused: true},
		{name: "an equal path on one of two hosts is refused", items: []applyItem{bearer("/user"), bearerOn([]string{a, b}, "/user")}, resources: []string{"x"}, refused: true},
		{name: "of an equal path in two items the earlier item keeps it on two hosts", items: []applyItem{bearerOn([]string{a, b}, "/user"), bearerOn([]string{a, b}, "/user")}, resources: []string{"x"},
			want: []exploreItem{{from: 0, hosts: []string{a, b}, scheme: "bearer", paths: []string{"/user"}}}},
		{name: "an equal path in two groups of one host is refused", items: []applyItem{bearer("/user"), basicAs("robot", "/user")}, resources: []string{"a"}, refused: true},
		{name: "an equal path in two groups that differ in the username is refused", items: []applyItem{basicAs("robot", "/user"), basicAs("clerk", "/user")}, resources: []string{"a"}, refused: true},
		{name: "a covered path in two groups that differ in the username is refused", items: []applyItem{basicAs("robot", "/{target.path}/*"), basicAs("clerk", "/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "an equal path in two groups that differ in the header is refused", items: []applyItem{headerAs("X-Api-Key", "/user"), headerAs("X-Token", "/user")}, resources: []string{"a"}, refused: true},
		{name: "a covered path in two groups that differ in the header is refused", items: []applyItem{headerAs("X-Api-Key", "/{target.path}/*"), headerAs("X-Token", "/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "a covered path in two groups of one host is refused", items: []applyItem{bearer("/{target.path}/*"), header("/{target.path}/issues")}, resources: []string{"group"}, refused: true},
		{name: "a path in two groups of one host that do not overlap is kept", items: []applyItem{bearer("/a/{target.path}"), header("/b/{target.path}")}, resources: []string{"x"},
			want: []exploreItem{one(0, "bearer", "/a/x"), {from: 1, hosts: []string{a}, scheme: "header", header: "X-Api-Key", paths: []string{"/b/x"}}}},
		{name: "an equal path in two groups of two hosts is kept", items: []applyItem{bearer("/user"), other("/user")}, resources: []string{"a"},
			want: []exploreItem{one(0, "bearer", "/user"), {from: 1, hosts: []string{b}, scheme: "bearer", paths: []string{"/user"}}}},
	} {
		got, refused := fillExploreCall(tc.items, tc.resources)
		same := func(x, y exploreItem) bool {
			return x.from == y.from && slices.Equal(x.hosts, y.hosts) && x.scheme == y.scheme && x.username == y.username && x.header == y.header && slices.Equal(x.paths, y.paths)
		}
		if refused != tc.refused || !slices.EqualFunc(got, tc.want, same) {
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
// refused for paths that overlap and neither covers the other. No target path and no
// template has a { or }, so no filled path has a {} segment. For paths with none,
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
// } is refused anywhere in the path. So a filled path template has no segment {} that
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

// refusedTargetPath models the refusal list of $defs/apply.paths in description.schema.json.
// The control plane refuses a target path with a space character, {, }, .., //, *, ? or #,
// a segment ., a backslash or a %. It refuses a target path that starts or ends in /, and
// one with a segment that starts with ; or .;. A space character is a character below
// U+0021, U+007F, U+FEFF, or a Unicode White_Space character.
func refusedTargetPath(p string) bool {
	if strings.ContainsAny(p, "{}*?#\\%") || strings.Contains(p, "..") || strings.Contains(p, "//") || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return true
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "." || strings.HasPrefix(segment, ";") || strings.HasPrefix(segment, ".;") {
			return true
		}
	}
	return strings.ContainsFunc(p, func(r rune) bool { return r < 0x21 || r == 0x7f || r == 0xfeff || unicode.IsSpace(r) })
}

// TestATargetPathWithAnUnclearPartIsRefused pins the refusal list of $defs/apply.paths.
// Each of acme/./x, acme\x, acme/%2f, a/b/, /acme, ;x and .;x is refused, and a;b is not.
// A set of target paths is built of segments with the parts that Forager's UnclearPath
// refuses. For each of them that the list passes, no template of the set fills it into a
// path that UnclearPath refuses. So the list is not narrower than UnclearPath for that
// set. The empty target path is left out, since address_path refuses it.
func TestATargetPathWithAnUnclearPartIsRefused(t *testing.T) {
	for _, tc := range []struct {
		path    string
		refused bool
	}{
		{"acme/./x", true},
		{"acme\\x", true},
		{"acme/%2f", true},
		{"a/b/", true},
		{"/acme", true},
		{";x", true},
		{".;x", true},
		{"..;x", true},
		{"a/..;x", true},
		{"a/;x", true},
		{"a/.;x/b", true},
		{".", true},
		{"a//b", true},
		{"a/../b", true},
		{"a b", true},
		{"acme", false},
		{"acme/widgets", false},
		{"a.b/c-d", false},
		{"acme/.hidden", false},
		{"a;b", false},
		{"a;", false},
		{"a/b;c/d", false},
		{"acme/widgets.v2", false},
	} {
		if got := refusedTargetPath(tc.path); got != tc.refused {
			t.Errorf("the list refuses %q: %v, want %v", tc.path, got, tc.refused)
		}
		if !tc.refused && policy.UnclearPath("/projects/"+tc.path+"/*") {
			t.Errorf("the target path %q passes the list, and Forager's UnclearPath refuses /projects/%s/*", tc.path, tc.path)
		}
	}
	segments := []string{"a", "acme", ".", "..", "%2e", "%2F", "%25", "%3b", "%5C", "\\", "", "a b", "x%41", "a.", ".a", "..a", "a;b", "a;", "a%3Bb", ";x", ".;x", "..;x", ";", "\t", "x\x7f"}
	templates := []string{"/projects/{target.path}/*", "/projects/{target.path}", "/{target.path}", "/projects/{target.path}.json", "/files/{target.path}.v2/*", "/projects/{target.path}/"}
	passed := 0
	var paths []string
	for _, a := range segments {
		paths = append(paths, a, a+"/")
		for _, b := range segments {
			paths = append(paths, a+"/"+b, a+"/"+b+"/")
			for _, c := range segments {
				paths = append(paths, a+"/"+b+"/"+c)
			}
		}
	}
	for _, p := range paths {
		if p == "" || refusedTargetPath(p) {
			continue
		}
		passed++
		for _, template := range templates {
			if filled := strings.Replace(template, "{target.path}", p, 1); policy.UnclearPath(filled) {
				t.Errorf("the target path %q passes the list, and the template %q fills it into %q, which Forager's UnclearPath refuses", p, template, filled)
			}
		}
	}
	if passed == 0 {
		t.Error("no target path of the set passes the list")
	}
}
