package contracts

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// LinkTemplateMaxLength is the most characters of a template of links in
// description.schema.json, $defs/link_template.
const LinkTemplateMaxLength = 2048

// LinkTemplatePattern is the pattern of $defs/link_template in description.schema.json: the
// literal https://, then characters of one line (no Cc or Cf character but U+200C and
// U+200D, no U+2028 or U+2029) and no brace except the three placeholders.
const LinkTemplatePattern = "^https://(?:[^{}\\p{Cc}\\p{Cf}  ]|[‌‍]|\\{(?:system|collection_path|path)\\})+$"

var (
	linkTemplateRE = regexp.MustCompile(LinkTemplatePattern)
	linkHolderRE   = regexp.MustCompile(`\{(system|collection_path|path)\}`)
)

// FillLink is the reference implementation of the rule in the README of the integration
// contract, section Links. It fills template with the address.system of a reference,
// the address path of the reference's collection and the address.path of the
// reference, and reports false for no link. It reports false for a template that the
// pattern of $defs/link_template refuses or that has more than [LinkTemplateMaxLength]
// characters, and for an empty value of a placeholder that the template uses. It writes
// each value as a path: the characters A-Z, a-z, 0-9, - . _ ~ and / stay, and every
// other byte of the value's UTF-8 is written as %XX with upper-case hex digits. A value
// of .. is not refused, so the link can contain a .. segment. It substitutes each
// placeholder once, so a value that contains braces is never read as a placeholder.
func FillLink(template, system, collectionPath, path string) (string, bool) {
	if utf8.RuneCountInString(template) > LinkTemplateMaxLength || !linkTemplateRE.MatchString(template) {
		return "", false
	}
	ok := true
	link := linkHolderRE.ReplaceAllStringFunc(template, func(holder string) string {
		var v string
		switch holder {
		case "{system}":
			v = system
		case "{collection_path}":
			v = collectionPath
		default:
			v = path
		}
		if v == "" {
			ok = false
		}
		return pathEscape(v)
	})
	if !ok || !strings.HasPrefix(link, "https://") {
		return "", false
	}
	return link, true
}

// pathEscape writes s with every byte but A-Z, a-z, 0-9, - . _ ~ and / as %XX.
func pathEscape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~', c == '/':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&15])
		}
	}
	return b.String()
}
