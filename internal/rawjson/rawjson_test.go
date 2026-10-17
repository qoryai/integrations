package rawjson

import "testing"

// TestText pins which bytes Text refuses: bytes that are not UTF-8, and a \u escape of
// a lone surrogate, in a value or a member name, and not a pair, an escaped backslash
// before a u, or a surrogate escape after a malformed escape, whose string still ends at
// its closing quote.
func TestText(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"a":"😀"}`, ""},
		{`{"a":"\\ud800"}`, ""},
		{`{"a":"é"}`, ""},
		{`{"a":"\ud800"}`, `a string holds the \u escape \ud800 of a lone surrogate`},
		{`{"a":"\udc00"}`, `a string holds the \u escape \udc00 of a lone surrogate`},
		{`{"a":"\ud800A"}`, `a string holds the \u escape \ud800 of a lone surrogate`},
		{`{"a":"\ud800𐀀"}`, `a string holds the \u escape \ud800 of a lone surrogate`},
		{`{"\ud800":1}`, `a string holds the \u escape \ud800 of a lone surrogate`},
		{`["\u12", "\ud800"]`, `a string holds the \u escape \ud800 of a lone surrogate`},
		{`["\u12", "\\ud800"]`, ""},
		{"{\"a\":\"\xff\"}", "the document is not UTF-8"},
	} {
		err := Text([]byte(tc.in))
		if got := errText(err); got != tc.want {
			t.Errorf("Text(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCheck pins what Check refuses: a member an object repeats, at any depth, always;
// and a number with a fraction or an exponent, only under Integers.
func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		in    string
		rules Rules
		want  string
	}{
		{`{"a":{"b":1,"b":2}}`, Rules{}, `an object repeats the member "b"`},
		{`[{"a":1},{"a":1}]`, Rules{}, ""},
		{`{"a":900.0}`, Rules{}, ""},
		{`{"a":900.0}`, Rules{Integers: true}, "the number 900.0 has a fraction or an exponent"},
		{`{"a":9e2}`, Rules{Integers: true}, "the number 9e2 has a fraction or an exponent"},
	} {
		if got := errText(Check([]byte(tc.in), tc.rules)); got != tc.want {
			t.Errorf("Check(%s, %+v) = %q, want %q", tc.in, tc.rules, got, tc.want)
		}
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
