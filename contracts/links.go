package contracts

import "fmt"

// CheckLinks reports the first rule of links in description.schema.json that a
// description breaks and that the schema does not express: no two entries have the same
// system. doc is the description as a schema validates it, after the schema accepts it.
// A description with no links keeps the rule.
func CheckLinks(doc any) error {
	d, _ := doc.(map[string]any)
	links, _ := d["links"].([]any)
	first := map[string]int{}
	for i, l := range links {
		entry, _ := l.(map[string]any)
		system, _ := entry["system"].(string)
		if j, ok := first[system]; ok {
			return fmt.Errorf("List each system of links once: the entries %d and %d both have the system %q.", j, i, system)
		}
		first[system] = i
	}
	return nil
}
