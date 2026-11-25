package contracts

import "testing"

// TestUnescapeReadsTilde1BeforeTilde0 pins the order of the two escapes of a JSON Pointer:
// ~01 is the two characters ~1, not /.
func TestUnescapeReadsTilde1BeforeTilde0(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"a~01b", "a~1b"},
		{"a~10b", "a/0b"},
		{"~0~1", "~/"},
		{"plain", "plain"},
	} {
		if got := unescape(c.in); got != c.want {
			t.Errorf("unescape(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
