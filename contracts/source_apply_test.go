package contracts_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

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
// source apply path template: it accepts a path in Forager's path grammar with
// {target.path} at most once and no other { or }, and each accepted template, filled
// with a target path that has no .., //, *, ?, # or white space, is in that grammar
// too.
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
		{"/projects/{target.path}*", true},
		{"/{target.path}", true},
		{"/{target.path}.json", true},
		{"/v1/{target.path}/files/*", true},
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
		for _, target := range []string{"acme", "acme/widgets", "acme/widgets.v2", "a-b_c~d", "{x}"} {
			filled := strings.Replace(tc.template, "{target.path}", target, 1)
			if !foragerPath.MatchString(filled) {
				t.Errorf("the template %q filled with %q is %q, which is not in Forager's path grammar", tc.template, target, filled)
			}
		}
	}
}
