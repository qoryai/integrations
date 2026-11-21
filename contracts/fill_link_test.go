package contracts_test

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// TestTheLinkTemplatePatternIsTheSchemas pins that the pattern that FillLink reads is
// the pattern of $defs/link_template in description.schema.json.
func TestTheLinkTemplatePatternIsTheSchemas(t *testing.T) {
	pattern, ok := node(t, "description.schema.json", "/$defs/link_template")["pattern"].(string)
	if !ok {
		t.Fatal("description.schema.json: $defs/link_template has no pattern")
	}
	if pattern != contracts.LinkTemplatePattern {
		t.Errorf("the schema's pattern is %q, FillLink reads %q", pattern, contracts.LinkTemplatePattern)
	}
	if _, err := regexp.Compile(pattern); err != nil {
		t.Fatal(err)
	}
	if max := node(t, "description.schema.json", "/$defs/link_template")["maxLength"]; fmt.Sprint(max) != strconv.Itoa(contracts.LinkTemplateMaxLength) {
		t.Errorf("the schema's maxLength is %v, want %d", max, contracts.LinkTemplateMaxLength)
	}
}

// TestFillLinkWritesEachValueAsAPath pins the table of the README, section Links, with
// the commit template, and that a value reaches the link once, whatever it contains.
func TestFillLinkWritesEachValueAsAPath(t *testing.T) {
	const commit = "https://{system}/{collection_path}/commit/{path}"
	for _, tc := range []struct{ path, want string }{
		{"a b", "a%20b"},
		{"a/b", "a/b"},
		{"a?b", "a%3Fb"},
		{"a#b", "a%23b"},
		{"a%b", "a%25b"},
		{"ä", "%C3%A4"},
		{"..", ".."},
		{"AZaz09-._~/", "AZaz09-._~/"},
		{"a&b=c+d:e@f;g,h$i", "a%26b%3Dc%2Bd%3Ae%40f%3Bg%2Ch%24i"},
		{"a{path}b", "a%7Bpath%7Db"},
		{"a\nb\x00c", "a%0Ab%00c"},
		{"\xff", "%FF"},
	} {
		got, ok := contracts.FillLink(commit, "code.example.com", "acme/shop", tc.path)
		want := "https://code.example.com/acme/shop/commit/" + tc.want
		if !ok || got != want {
			t.Errorf("path %q: %q, %v, want %q", tc.path, got, ok, want)
		}
	}
	got, ok := contracts.FillLink(commit, "code.example.com", "acme shop/{system}", "3e2d")
	if want := "https://code.example.com/acme%20shop/%7Bsystem%7D/commit/3e2d"; !ok || got != want {
		t.Errorf("collection path with a brace: %q, %v, want %q", got, ok, want)
	}
}

// TestFillLinkMakesNoLinkWhenItCannotFillEveryPlaceholder pins the steps of the README:
// an empty value of a placeholder that the template uses, and a template that the
// schema refuses, give no link; an empty value of a placeholder that the template does
// not use does not.
func TestFillLinkMakesNoLinkWhenItCannotFillEveryPlaceholder(t *testing.T) {
	for name, tc := range map[string]struct{ template, system, collection, path string }{
		"an empty system":             {"https://{system}/{path}", "", "acme/shop", "1"},
		"an empty collection path":    {"https://x.example.com/{collection_path}/{path}", "code.example.com", "", "1"},
		"an empty path":               {"https://x.example.com/{path}", "code.example.com", "acme/shop", ""},
		"http":                        {"http://x.example.com/{path}", "code.example.com", "acme/shop", "1"},
		"an unknown placeholder":      {"https://x.example.com/{id}", "code.example.com", "acme/shop", "1"},
		"a line feed in the template": {"https://x.example.com/\n{path}", "code.example.com", "acme/shop", "1"},
		"a template of 2049":          {"https://x.example.com/{path}" + strings.Repeat("a", 2049-len("https://x.example.com/{path}")), "code.example.com", "acme/shop", "1"},
		"no template":                 {"", "code.example.com", "acme/shop", "1"},
	} {
		if got, ok := contracts.FillLink(tc.template, tc.system, tc.collection, tc.path); ok || got != "" {
			t.Errorf("%s: %q, %v, want no link", name, got, ok)
		}
	}
	if got, ok := contracts.FillLink("https://x.example.com/{path}", "", "", "1"); !ok || got != "https://x.example.com/1" {
		t.Errorf("empty values of placeholders the template does not use: %q, %v", got, ok)
	}
	long := "https://x.example.com/{path}" + strings.Repeat("a", 2048-len("https://x.example.com/{path}"))
	if _, ok := contracts.FillLink(long, "", "", "1"); !ok {
		t.Error("a template of 2048 characters is refused")
	}
}

// TestFillLinkFillsTheExamplesOfTheReadme pins the three examples of the README,
// section Links.
func TestFillLinkFillsTheExamplesOfTheReadme(t *testing.T) {
	for _, tc := range []struct{ template, system, collection, path, want string }{
		{"https://{system}/{collection_path}/commit/{path}", "code.example.com", "acme/shop", "3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1",
			"https://code.example.com/acme/shop/commit/3e2d1c0b9a8f7e6d5c4b3a291807f6e5d4c3b2a1"},
		{"https://{system}/conversations/{path}", "desk.example.com", "", "48213", "https://desk.example.com/conversations/48213"},
		{"https://{system}/console/{collection_path}/keywords/{path}", "ads.example.com", "profile-5521/campaign-9", "30417",
			"https://ads.example.com/console/profile-5521/campaign-9/keywords/30417"},
	} {
		if got, ok := contracts.FillLink(tc.template, tc.system, tc.collection, tc.path); !ok || got != tc.want {
			t.Errorf("%s: %q, %v, want %q", tc.template, got, ok, tc.want)
		}
	}
}
