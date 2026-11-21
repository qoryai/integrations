package contracts

import (
	"fmt"
	"strings"
)

// CheckLinks reports the first rule of links in description.schema.json that a
// description breaks and that the schema does not express: the host of the template of
// an entry is the system of the entry, and no two entries have the same system. The
// host is the whole text between the https:// of the template and its first /. The
// schema refuses a port, so the host has none. doc is the description as a schema
// validates it, after the schema accepts it. A description with no links keeps both
// rules. It skips an entry whose system or template is no string, which the schema
// refuses.
func CheckLinks(doc any) error {
	d, _ := doc.(map[string]any)
	links, _ := d["links"].([]any)
	first := map[string]int{}
	for i, l := range links {
		entry, _ := l.(map[string]any)
		system, ok := entry["system"].(string)
		if !ok {
			continue
		}
		if template, ok := entry["template"].(string); ok {
			if rest, ok := strings.CutPrefix(template, "https://"); ok {
				if host, _, _ := strings.Cut(rest, "/"); host != system {
					return fmt.Errorf("Use the system %q as the host of the template of link %d, not %q.", system, i, host)
				}
			}
		}
		if j, ok := first[system]; ok {
			return fmt.Errorf("List each system of links once: the entries %d and %d both have the system %q.", j, i, system)
		}
		first[system] = i
	}
	return nil
}
