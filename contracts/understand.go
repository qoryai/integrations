package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MustUnderstand is the member that lists, in a document that a writer sends to an
// older reader, the members of the document that the reader must understand.
const MustUnderstand = "must_understand"

// The limits of a must_understand list in the schemas of this repository.
const (
	// MustUnderstandPointers is the most entries a must_understand list has.
	MustUnderstandPointers = 32
	// MustUnderstandPointerLength is the most characters an entry has.
	MustUnderstandPointerLength = 256
)

// CheckMustUnderstand reports the first pointer in the must_understand member of the JSON
// object b that is not a member of the document type v. It returns nil when b has no
// must_understand, or lists only members of v. Each entry is a JSON Pointer (RFC 6901)
// counted from b, such as /run_settings/runtime. v is a struct, or a pointer to one, such
// as [Documents], [NextSessionAnswer] or [Changes].
//
// A pointer resolves through the json tags of structs, the index of a list, and any key
// of a map, a key that is not empty. A type that decodes itself, such as [Tier], implements
// definesPointer for the members it defines. Below a json.RawMessage or an interface, any
// pointer resolves, since the type does not define what is there.
//
// A reader that decodes b into v ignores a member that v does not have. It calls
// CheckMustUnderstand first, and refuses the document when it returns an error. It
// returns an error too when b is not a JSON object, when must_understand is null or not a
// list of strings, when an entry is not a pointer, has a line feed, has a ~ that is not
// ~0 or ~1, or has more than [MustUnderstandPointerLength] characters, when an entry
// repeats, and when the list has more than [MustUnderstandPointers] entries, the limits
// of the schemas.
func CheckMustUnderstand(b []byte, v any) error {
	p, err := unmetPointer(b, v)
	if err != nil {
		return err
	}
	if p != "" {
		return fmt.Errorf("use a reader that defines %s: the document lists it in must_understand, and this reader has no such member", quotePointer(p))
	}
	return nil
}

// unmetPointer returns the first pointer of the must_understand member of b that v does
// not define, or "" when there is none.
func unmetPointer(b []byte, v any) (string, error) {
	var doc struct {
		MustUnderstand json.RawMessage `json:"must_understand"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return "", fmt.Errorf("read the document as a JSON object: %w", err)
	}
	if len(doc.MustUnderstand) == 0 {
		return "", nil
	}
	var pointers []string
	if err := json.Unmarshal(doc.MustUnderstand, &pointers); err != nil {
		return "", fmt.Errorf("set must_understand to a list of JSON Pointers, or leave it out: %w", err)
	}
	if pointers == nil {
		return "", errors.New("set must_understand to a list of JSON Pointers, or leave it out, not null")
	}
	if len(pointers) > MustUnderstandPointers {
		return "", fmt.Errorf("list at most %d pointers in must_understand, not %d", MustUnderstandPointers, len(pointers))
	}
	seen := make(map[string]bool, len(pointers))
	for _, p := range pointers {
		if utf8.RuneCountInString(p) > MustUnderstandPointerLength {
			return "", fmt.Errorf("write each pointer in must_understand with at most %d characters, not %d", MustUnderstandPointerLength, utf8.RuneCountInString(p))
		}
		if seen[p] {
			return "", fmt.Errorf("list %s once in must_understand", quotePointer(p))
		}
		seen[p] = true
		if !strings.HasPrefix(p, "/") {
			return "", fmt.Errorf("write %s in must_understand as a JSON Pointer that starts with /", quotePointer(p))
		}
		if strings.Contains(p, "\n") {
			return "", fmt.Errorf("write %s in must_understand without a line break", quotePointer(p))
		}
		segments := strings.Split(p[1:], "/")
		for i, s := range segments {
			if !validEscapes(s) {
				return "", fmt.Errorf("write %s in must_understand with ~ only as ~0 or ~1", quotePointer(p))
			}
			segments[i] = unescape(s)
		}
		if !resolves(reflect.TypeOf(v), segments) {
			return p, nil
		}
	}
	return "", nil
}

// quotedPointerLimit is the most bytes that [quotePointer] writes between the quotes.
const quotedPointerLimit = 300

// quotePointer writes the pointer p in double quotes, as %q does, so the text has no
// control character and no line break. It cuts a pointer whose quoted form has more than
// 300 bytes, such as 256 control characters, and ends the cut text with "...", so a message
// that has it stays within the 1000 characters of the detail of [ChangesFailed].
func quotePointer(p string) string {
	q := strconv.Quote(p)
	if len(q)-2 <= quotedPointerLimit {
		return q
	}
	var sb strings.Builder
	for _, r := range p {
		piece := strconv.Quote(string(r))
		piece = piece[1 : len(piece)-1]
		if sb.Len()+len(piece) > quotedPointerLimit-3 {
			break
		}
		sb.WriteString(piece)
	}
	return `"` + sb.String() + `..."`
}

// unescape returns the member name that the pointer segment s writes: ~1 is /, and ~0 is
// ~, in that order, so ~01 is the two characters ~1.
func unescape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

// validEscapes reports whether each ~ in the pointer segment s is ~0 or ~1.
func validEscapes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '~' && (i+1 == len(s) || (s[i+1] != '0' && s[i+1] != '1')) {
			return false
		}
	}
	return true
}

// resolves reports whether the pointer with the segments, unescaped, leads to a member
// of the type t.
func resolves(t reflect.Type, segments []string) bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if len(segments) == 0 {
		return true
	}
	if r, ok := reflect.Zero(t).Interface().(pointerResolver); ok {
		return r.definesPointer(segments)
	}
	if readsItself(t) {
		return true
	}
	return resolvesBelow(t, segments)
}

// resolvesBelow reports whether the segments, one or more, lead to a member of the type t
// by its fields, its map keys or its list items.
func resolvesBelow(t reflect.Type, segments []string) bool {
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
		return seg != "" && resolves(t.Elem(), segments[1:])
	case reflect.Slice, reflect.Array:
		if isIndex(seg) {
			return resolves(t.Elem(), segments[1:])
		}
	}
	return false
}

// pointerResolver is implemented by a type that decodes itself from JSON, and defines the
// members below it in a way that json tags cannot show.
type pointerResolver interface {
	// definesPointer reports whether the type defines the member that the segments, one or
	// more, lead to below a value of the type.
	definesPointer(segments []string) bool
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
