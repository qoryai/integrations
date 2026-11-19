package contracts_test

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/qoryai/integrations/contracts"
)

// pointerWalker lists the JSON pointers that a schema defines, from the object that holds
// must_understand. A member is its name; an item of a list is "#"; a member of a map with
// any key is "*". A pointer in open is open-ended: the schema does not define what is below
// it, so any pointer below it is accepted.
type pointerWalker struct {
	out   map[string]bool
	open  map[string]bool
	stack map[string]bool
}

// definedPointers returns the pointer templates that the schema at (base, node) defines,
// sorted, open-ended templates ending in "/**".
func definedPointers(base string, node any, open ...string) []string {
	w := &pointerWalker{out: map[string]bool{}, open: map[string]bool{}, stack: map[string]bool{}}
	for _, o := range open {
		w.open[o] = true
	}
	w.walk(base, node, "")
	return slices.Sorted(func(yield func(string) bool) {
		for k := range w.out {
			if !yield(k) {
				return
			}
		}
	})
}

// loadSchema reads the schema document of a URL of the integration or run contract, and
// nil for a schema of another contract, whose members the walk does not follow.
func loadSchema(url string) (any, string) {
	file, _, _ := strings.Cut(url, "#")
	var doc any
	var err error
	switch {
	case strings.HasPrefix(file, contracts.Base+"/"):
		doc, err = contracts.Document(strings.TrimPrefix(file, contracts.Base+"/"))
	case strings.HasPrefix(file, contracts.RunBase+"/"):
		doc, err = contracts.RunDocument(strings.TrimPrefix(file, contracts.RunBase+"/"))
	default:
		return nil, file
	}
	if err != nil {
		panic(err)
	}
	return doc, file
}

// follow resolves a $ref against the document at base, and returns the schema, and the
// base of the document it is in.
func follow(base string, ref string) (any, string) {
	url := ref
	if strings.HasPrefix(ref, "#") {
		url = base + ref
	} else if !strings.Contains(ref, "://") {
		url = base[:strings.LastIndex(base, "/")+1] + ref
	}
	doc, file := loadSchema(url)
	if doc == nil {
		return nil, file
	}
	_, frag, _ := strings.Cut(url, "#")
	node := doc
	for _, seg := range strings.Split(strings.TrimPrefix(frag, "/"), "/") {
		if seg == "" {
			continue
		}
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		node = node.(map[string]any)[seg]
	}
	return node, file
}

func (w *pointerWalker) walk(base string, node any, path string) {
	m, ok := node.(map[string]any)
	if !ok {
		return
	}
	if w.open[path] {
		w.out[path+"/**"] = true
		return
	}
	if ref, ok := m["$ref"].(string); ok {
		if strings.HasSuffix(ref, "#/$defs/bounds") || strings.HasSuffix(ref, "#/$defs/no_secret") {
			return
		}
		key := base + "|" + ref + "|" + path
		if w.stack[key] {
			return
		}
		target, tbase := follow(base, ref)
		if target != nil {
			w.stack[key] = true
			w.walk(tbase, target, path)
			delete(w.stack, key)
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for name, sub := range props {
			if name == "must_understand" {
				continue
			}
			child := path + "/" + strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
			w.out[child] = true
			w.walk(base, sub, child)
		}
	}
	for _, k := range []string{"allOf", "anyOf", "oneOf"} {
		if list, ok := m[k].([]any); ok {
			for _, sub := range list {
				w.walk(base, sub, path)
			}
		}
	}
	for _, k := range []string{"if", "then", "else"} {
		w.walk(base, m[k], path)
	}
	if deps, ok := m["dependentSchemas"].(map[string]any); ok {
		for _, sub := range deps {
			w.walk(base, sub, path)
		}
	}
	if items, ok := m["items"]; ok {
		w.out[path+"/#"] = true
		w.walk(base, items, path+"/#")
	}
	if add, ok := m["additionalProperties"].(map[string]any); ok {
		w.out[path+"/*"] = true
		w.walk(base, add, path+"/*")
	}
	if pp, ok := m["patternProperties"].(map[string]any); ok {
		for _, sub := range pp {
			w.out[path+"/*"] = true
			w.walk(base, sub, path+"/*")
		}
	}
}

// pointerPattern is the regular expression that accepts the pointers of templates: a
// member by its name, "#" as an index, "*" as any one key, and a trailing "/**" as any
// pointer below.
func pointerPattern(templates []string) string {
	var alts []string
	for _, t := range templates {
		open := strings.HasSuffix(t, "/**")
		t = strings.TrimSuffix(t, "/**")
		var sb strings.Builder
		for _, seg := range strings.Split(t, "/")[1:] {
			sb.WriteString("/")
			switch seg {
			case "#":
				sb.WriteString("(?:0|[1-9][0-9]*)")
			case "*":
				sb.WriteString("(?:[^/~]|~[01])+")
			default:
				sb.WriteString(regexp.QuoteMeta(seg))
			}
		}
		if open {
			sb.WriteString(`(?:/[\s\S]*)?`)
		}
		alts = append(alts, sb.String())
	}
	return fmt.Sprintf("^(?:%s)$", strings.Join(alts, "|"))
}

func printPointers(name string, templates []string) {
	fmt.Println("==", name, len(templates))
	fmt.Println(pointerPattern(templates))
}
