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

// CheckMustUnderstand reports the first name in the must_understand member of the JSON
// object b that is not a member of the document type v, and nil when b has no
// must_understand, or it lists only members of v. v is a struct, or a pointer to one, such
// as [Documents] or [NextSessionAnswer]; its members are the names in the json tags of its
// fields, and must_understand itself is not one of them. A reader that decodes b into v
// ignores a member that v does not have, so it calls CheckMustUnderstand first, and refuses
// the document when it returns an error. It returns an error too when b is not a JSON
// object, when must_understand is null or not a list of strings, and when a name repeats.
// The schema's limit on the count needs no check here: a list of distinct members of v is
// never longer than the schema's maxItems, which is at least the number of members.
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
	var names []string
	if err := json.Unmarshal(doc.MustUnderstand, &names); err != nil {
		return fmt.Errorf("set must_understand to a list of member names, or leave it out: %w", err)
	}
	if names == nil {
		return errors.New("set must_understand to a list of member names, or leave it out, not null")
	}
	known := members(reflect.TypeOf(v))
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return fmt.Errorf("list %q once in must_understand", name)
		}
		seen[name] = true
		if !known[name] {
			return fmt.Errorf("the document lists %q in must_understand, and this reader has no member %q: use a reader that has it", name, name)
		}
	}
	return nil
}

// members returns the JSON member names of the struct type t, or of the struct that it
// points to, without must_understand.
func members(t reflect.Type) map[string]bool {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	out := make(map[string]bool)
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" && name != MustUnderstand {
			out[name] = true
		}
	}
	return out
}
