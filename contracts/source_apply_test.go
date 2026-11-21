package contracts_test

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/qoryai/forager/policy"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// foragerPath is the path grammar of Forager's apply in run-configuration.schema.json,
// which a path template and its filled form both follow.
var foragerPath = regexp.MustCompile(`^/[^*?#\s]*\*?$`)

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
		if !foragerPath.MatchString(tc.template) {
			t.Errorf("the template %q is not in Forager's path grammar", tc.template)
		}
		for _, target := range []string{"acme", "acme/widgets", "acme/widgets.v2", "acme/reports", "a-b_c~d", "{x}"} {
			filled := strings.Replace(tc.template, "{target.path}", target, 1)
			if !foragerPath.MatchString(filled) {
				t.Errorf("the template %q filled with %q is %q, which is not in Forager's path grammar", tc.template, target, filled)
			}
		}
	}
}

// foragerPaths is a copy of the paths member of Forager's apply in
// run-configuration.schema.json at Forager 76d6638, which the forager module this
// module requires does not contain yet.
const foragerPaths = `{"type": "array", "minItems": 1, "maxItems": 32, "uniqueItems": true, "items": {"type": "string", "pattern": "^/[^*?#\\s]*\\*?$"}}`

// TestFilledPathsLeaveOutAPathTheyAlreadyHave pins why the control plane leaves out a
// filled path that equals an earlier filled path of the same item: two templates that
// the schema accepts, none twice, can fill to the same path, and the filled paths with
// that path left out are valid as Forager's apply paths, in its grammar and none twice.
func TestFilledPathsLeaveOutAPathTheyAlreadyHave(t *testing.T) {
	templates := compile(t, "description.schema.json#/$defs/apply/properties/paths")
	c := jsonschema.NewCompiler()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(foragerPaths))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AddResource("forager-paths.json", doc); err != nil {
		t.Fatal(err)
	}
	forager, err := c.Compile("forager-paths.json")
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

// fillExploreCall fills the path templates of one description item on one host for a run
// with no target, as $defs.apply of description.schema.json says: each template for every
// resource in the order of the call's resources, a path that equals an earlier path
// left out, and each path that another path covers dropped. It returns the kept paths
// and the first pair of kept paths that overlap, if any, which refuses the call. Forager's
// policy package decides what covers and what overlaps.
func fillExploreCall(templates, resources []string) (kept []string, first, second string, refused bool) {
	var filled []string
	for _, template := range templates {
		for _, resource := range resources {
			if p := strings.Replace(template, "{target.path}", resource, 1); !slices.Contains(filled, p) {
				filled = append(filled, p)
			}
		}
	}
	for _, p := range filled {
		if !slices.ContainsFunc(filled, func(other string) bool { return other != p && policy.CoversPath(other, p) }) {
			kept = append(kept, p)
		}
	}
	for i, p := range kept {
		for _, other := range kept[:i] {
			if policy.OverlapPaths(other, p) {
				return kept, other, p, true
			}
		}
	}
	return kept, "", "", false
}

// TestAnExploreCallDropsACoveredPathAndRefusesAnOverlap pins the rule of $defs.apply
// about the filled paths of a call: a path that another covers is dropped, and a call
// whose kept paths overlap, with neither covering the other, is refused.
func TestAnExploreCallDropsACoveredPathAndRefusesAnOverlap(t *testing.T) {
	for _, tc := range []struct {
		name          string
		templates     []string
		resources     []string
		kept          []string
		first, second string
		refused       bool
	}{
		{name: "a path under a path with a star is dropped", templates: []string{"/{target.path}/*"}, resources: []string{"group", "group/sub/repo"}, kept: []string{"/group/*"}},
		{name: "the order of the resources does not matter", templates: []string{"/{target.path}/*"}, resources: []string{"group/sub/repo", "group"}, kept: []string{"/group/*"}},
		{name: "a path equal to an earlier path is left out once", templates: []string{"/projects/{target.path}", "/projects/acme"}, resources: []string{"acme"}, kept: []string{"/projects/acme"}},
		{name: "a template without the target path gives one path", templates: []string{"/user"}, resources: []string{"a", "b"}, kept: []string{"/user"}},
		{name: "paths of different places stay", templates: []string{"/{target.path}/*"}, resources: []string{"a", "b"}, kept: []string{"/a/*", "/b/*"}},
		{name: "a path without a star does not cover a longer path", templates: []string{"/{target.path}"}, resources: []string{"group", "group/sub"}, kept: []string{"/group", "/group/sub"}},
		{
			name: "a segment of a target path written {} overlaps another path and neither covers the other", templates: []string{"/{target.path}/*"}, resources: []string{"a/{}/x", "a/y"},
			kept: []string{"/a/{}/x/*", "/a/y/*"}, first: "/a/{}/x/*", second: "/a/y/*", refused: true,
		},
	} {
		kept, first, second, refused := fillExploreCall(tc.templates, tc.resources)
		if !slices.Equal(kept, tc.kept) || first != tc.first || second != tc.second || refused != tc.refused {
			t.Errorf("%s: kept %q, overlap %q and %q, refused %v; want %q, %q and %q, %v", tc.name, kept, first, second, refused, tc.kept, tc.first, tc.second, tc.refused)
		}
		for _, p := range kept {
			if !foragerPath.MatchString(p) {
				t.Errorf("%s: the kept path %q is not in Forager's path grammar", tc.name, p)
			}
		}
		if refused && (policy.CoversPath(first, second) || policy.CoversPath(second, first)) {
			t.Errorf("%s: a refused call has the paths %q and %q, and one covers the other", tc.name, first, second)
		}
	}
}

// TestPathsWithNoSegmentBraceOverlapOnlyWhenOneCoversTheOther pins why a call
// whose paths have no {} segment is never refused for an overlap: after the covered
// paths are dropped, no two such paths overlap, since Forager's OverlapPaths agrees
// with CoversPath in either direction when no path has a {} segment.
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
