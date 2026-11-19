package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// MustUnderstand is the member that lists, in a document that a writer sends to an
// older reader, the members of the document that the reader must understand.
const MustUnderstand = "must_understand"

// CheckMustUnderstand reports the first pointer in the must_understand member of the JSON
// object b that is not a member of the document type v, and nil when b has no
// must_understand, or it lists only members of v. Each entry is a JSON Pointer (RFC 6901)
// counted from b, such as /run_settings/runtime. v is a struct, or a pointer to one, such
// as [Documents], [NextSessionAnswer] or [Change]. A pointer resolves through the json
// tags of structs, the index of a list, and any key of a map. Below a value that the type
// reads itself, such as json.RawMessage, an interface, or a type with its own JSON
// decoding and no tagged fields, any pointer resolves, since the type does not define
// what is there. A reader that decodes b into v ignores a member that v does not have, so
// it calls CheckMustUnderstand first, and refuses the document when it returns an error.
// It returns an error too when b is not a JSON object, when must_understand is null or
// not a list of strings, when an entry is not a pointer, and when an entry repeats. The
// schema's limit on the count needs no check here: a list of distinct pointers to
// members of v is never longer than the schema's maxItems for the documents of this
// package.
func CheckMustUnderstand(b []byte, v any) error {
	var doc struct {
		MustUnderstand json.RawMessage `json:"must_understand"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("read the document as a JSON object: %w", err)
	}
	if len(doc.MustUnderstand) == 0 {
		return nil
	}
	var pointers []string
	if err := json.Unmarshal(doc.MustUnderstand, &pointers); err != nil {
		return fmt.Errorf("set must_understand to a list of JSON Pointers, or leave it out: %w", err)
	}
	if pointers == nil {
		return errors.New("set must_understand to a list of JSON Pointers, or leave it out, not null")
	}
	seen := make(map[string]bool, len(pointers))
	for _, p := range pointers {
		if seen[p] {
			return fmt.Errorf("list %q once in must_understand", p)
		}
		seen[p] = true
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("write %q in must_understand as a JSON Pointer that starts with /", p)
		}
		segments := strings.Split(p[1:], "/")
		for i, s := range segments {
			segments[i] = strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
		}
		if !resolves(reflect.TypeOf(v), segments) {
			return fmt.Errorf("the document lists %q in must_understand, and this reader has no such member: use a reader that has it", p)
		}
	}
	return nil
}

// resolves reports whether the pointer with the segments, unescaped, leads to a member
// of the type t.
func resolves(t reflect.Type, segments []string) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if len(segments) == 0 || readsItself(t) {
		return true
	}
	seg := segments[0]
	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
			if name == seg && name != MustUnderstand && name != "-" {
				return resolves(t.Field(i).Type, segments[1:])
			}
		}
	case reflect.Map:
		return resolves(t.Elem(), segments[1:])
	case reflect.Slice, reflect.Array:
		if isIndex(seg) {
			return resolves(t.Elem(), segments[1:])
		}
	}
	return false
}

// readsItself reports whether the type decodes its JSON by itself and defines no member
// below it: an interface, a type that decodes by a method and has no field with a json
// tag, such as json.RawMessage.
func readsItself(t reflect.Type) bool {
	if t.Kind() == reflect.Interface {
		return true
	}
	if !reflect.PointerTo(t).Implements(reflect.TypeFor[json.Unmarshaler]()) {
		return false
	}
	if t.Kind() != reflect.Struct {
		return true
	}
	for i := range t.NumField() {
		if _, ok := t.Field(i).Tag.Lookup("json"); ok {
			return false
		}
	}
	return true
}

// isIndex reports whether s is an index of a list in a JSON Pointer: digits, with no
// leading zero.
func isIndex(s string) bool {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
