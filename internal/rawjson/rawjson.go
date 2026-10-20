// Package rawjson checks rules of the contracts that hold on a JSON document's raw bytes
// and that no schema can express, since a parsed document no longer shows them: a parsed
// document holds U+FFFD for invalid UTF-8 and for a \u escape of a lone surrogate, keeps
// one member of a name an object repeats, and holds 900.0 as the integer 900.
package rawjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Rules are the rules [Check] applies beyond well-formed JSON in which no object repeats
// a member, at any depth, which it always checks. A limit of 0 is not checked.
type Rules struct {
	// Integers refuses a number written with a fraction or an exponent.
	Integers bool
	// MaxStringBytes is the most bytes of UTF-8 a string, a member name included, holds.
	MaxStringBytes int
	// MaxValues is the most JSON values the document holds in all: the document itself,
	// each array entry and each member's value count one each, and member names none.
	MaxValues int
	// MaxDepth is the most levels arrays and objects nest, the outermost being level 1.
	MaxDepth int
}

// Check reports the first rule b breaks, reading it one JSON value and member name at a
// time: a member an object repeats, and each rule of r.
func Check(b []byte, r Rules) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	type frame struct {
		object, key bool
		seen        map[string]bool
	}
	var stack []*frame
	values := 0
	for {
		item, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if d, ok := item.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if top != nil && top.object && top.key {
			name := item.(string)
			if err := r.str(name); err != nil {
				return err
			}
			if top.seen[name] {
				return fmt.Errorf("an object repeats the member %q", name)
			}
			top.seen[name] = true
			top.key = false
			continue
		}
		if top != nil && top.object {
			top.key = true
		}
		values++
		if r.MaxValues > 0 && values > r.MaxValues {
			return fmt.Errorf("the document holds more than %d JSON values", r.MaxValues)
		}
		switch v := item.(type) {
		case json.Delim:
			if r.MaxDepth > 0 && len(stack)+1 > r.MaxDepth {
				return fmt.Errorf("the document nests arrays and objects more than %d levels deep", r.MaxDepth)
			}
			stack = append(stack, &frame{object: v == '{', key: v == '{', seen: map[string]bool{}})
		case string:
			if err := r.str(v); err != nil {
				return err
			}
		case json.Number:
			if r.Integers && strings.ContainsAny(string(v), ".eE") {
				return fmt.Errorf("the number %s has a fraction or an exponent", v)
			}
		}
	}
}

// str reports a string, or a member name, longer than r allows.
func (r Rules) str(s string) error {
	if r.MaxStringBytes > 0 && len(s) > r.MaxStringBytes {
		return fmt.Errorf("a string or a member name is %d bytes of UTF-8, more than %d", len(s), r.MaxStringBytes)
	}
	return nil
}

// Text reports bytes that are not UTF-8, and the first \u escape in a string that names
// a surrogate, U+D800 to U+DFFF, outside a pair of a high surrogate escape followed at
// once by a low one. An escape that is not four hexadecimal digits is left to the JSON
// decoder.
func Text(b []byte) error {
	if !utf8.Valid(b) {
		return errors.New("the document is not UTF-8")
	}
	in := false
	for i := 0; i < len(b); i++ {
		switch {
		case !in:
			in = b[i] == '"'
		case b[i] == '"':
			in = false
		case b[i] == '\\' && i+1 < len(b) && b[i+1] == 'u':
			r, ok := escape(b, i)
			if !ok {
				i++
				continue
			}
			if !utf16.IsSurrogate(r) {
				i += 5
				continue
			}
			if low, ok := escape(b, i+6); r < 0xdc00 && ok && low >= 0xdc00 && low <= 0xdfff {
				i += 11
				continue
			}
			return fmt.Errorf(`a string holds the \u escape %s of a lone surrogate`, b[i:i+6])
		case b[i] == '\\':
			i++
		}
	}
	return nil
}

// escape is the code unit of the \u escape at b[i:i+6], and whether there is one.
func escape(b []byte, i int) (rune, bool) {
	if i+6 > len(b) || b[i] != '\\' || b[i+1] != 'u' {
		return 0, false
	}
	n, err := strconv.ParseUint(string(b[i+2:i+6]), 16, 16)
	return rune(n), err == nil
}
