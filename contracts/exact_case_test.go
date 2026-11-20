package contracts_test

import (
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/qoryai/integrations/contracts"
)

// decoders are the two JSON decoders that a reader of the contracts may use. Each matches a
// member name by exact case for a type of this package.
var decoders = map[string]func([]byte, any) error{
	"encoding/json": json.Unmarshal,
	"json/v2":       func(b []byte, v any) error { return jsonv2.Unmarshal(b, v) },
}

// TestEveryMemberTagIsExactCase fails for a struct field of the package with a json tag that
// lacks case:strict. encoding/json matches the member name of a field without it in any
// letter case, and the schemas treat MUST_UNDERSTAND as a member that is not defined.
func TestEveryMemberTagIsExactCase(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		// submit_program.go reads the $defs of an embedded schema file, which is not a
		// document of a contract.
		if strings.HasSuffix(f, "_test.go") || f == "submit_program.go" {
			continue
		}
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			st, ok := n.(*ast.StructType)
			if !ok {
				return true
			}
			for _, field := range st.Fields.List {
				if field.Tag == nil || !strings.Contains(field.Tag.Value, `json:"`) {
					continue
				}
				tag := field.Tag.Value
				if !strictJSONTag(tag) && !strings.Contains(tag, `json:"-"`) {
					t.Errorf("%s: the tag %s lacks case:strict in its json key", fset.Position(field.Pos()), tag)
				}
			}
			return true
		})
	}
}

// strictJSONTag reports whether the struct tag literal tag, with its back quotes, has the
// option case:strict in its json key.
func strictJSONTag(tag string) bool {
	raw, err := strconv.Unquote(tag)
	if err != nil {
		return false
	}
	v, _ := reflect.StructTag(raw).Lookup("json")
	return slices.Contains(strings.Split(v, ",")[1:], "case:strict")
}

func TestStrictJSONTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"`json:\"a,omitempty,case:strict\"`":  true,
		"`json:\"a,case:strict\" yaml:\"a\"`": true,
		"`yaml:\"a,case:strict\"`":            false,
		"`json:\"a\" yaml:\"a,case:strict\"`": false,
		"`json:\"a,case:strictly\"`":          false,
	} {
		if got := strictJSONTag(tag); got != want {
			t.Errorf("strictJSONTag(%s) = %v, want %v", tag, got, want)
		}
	}
}

// TestCaseStrictIsHonoured probes the option on the toolchain that builds the tests, with
// a struct of its own. A build with GOEXPERIMENT=nojsonv2 has a decoder of encoding/json
// that ignores the option, and fails here.
func TestCaseStrictIsHonoured(t *testing.T) {
	type probe struct {
		State string `json:"state,case:strict"`
	}
	for name, decode := range decoders {
		for in, want := range map[string]string{
			`{"STATE":"x"}`:             "",
			`{"state":"x"}`:             "x",
			`{"STATE":"a","state":"b"}`: "b",
			`{"state":"b","STATE":"a"}`: "b",
		} {
			var p probe
			if err := decode([]byte(in), &p); err != nil {
				t.Fatal(err)
			}
			if p.State != want {
				t.Errorf("case:strict is not honoured by this build; the readers of contracts match member names regardless of case (%s read %s as %q, want %q)", name, in, p.State, want)
			}
		}
	}
}

func TestCheckMustUnderstandExactCase(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		// unmet is the pointer that the check reports, "" for none.
		unmet string
	}{
		{"variant alone", `{"MUST_UNDERSTAND":["/x"]}`, ""},
		{"variant with underscore case", `{"Must_Understand":["/x"]}`, ""},
		{"exact member lists, variant empty", `{"must_understand":["/x"],"Must_Understand":[]}`, `"/x"`},
		{"variant empty, exact member lists", `{"Must_Understand":[],"must_understand":["/x"]}`, `"/x"`},
		{"exact member empty, variant lists", `{"must_understand":[],"Must_Understand":["/x"]}`, ""},
		{"variant lists, exact member empty", `{"Must_Understand":["/x"],"must_understand":[]}`, ""},
		{"variant is not a list", `{"MUST_UNDERSTAND":7,"must_understand":["/x"]}`, `"/x"`},
		{"exact member alone", `{"must_understand":["/x"]}`, `"/x"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, v := range []any{contracts.Documents{}, contracts.NextSessionAnswer{}, contracts.Changes{}} {
				err := contracts.CheckMustUnderstand([]byte(tt.body), v)
				switch {
				case tt.unmet == "" && err != nil:
					t.Errorf("%T: got %v, want nil", v, err)
				case tt.unmet != "" && (err == nil || !strings.Contains(err.Error(), tt.unmet)):
					t.Errorf("%T: got %v, want an error with %s", v, err, tt.unmet)
				}
			}
		})
	}
}

func TestCheckChangeMustUnderstandExactCase(t *testing.T) {
	for _, tt := range []struct {
		name, entry string
		wantErr     string
	}{
		{"variant alone", `{"kind":"sync","interval":60,"MUST_UNDERSTAND":["/x"]}`, ""},
		{"exact member lists, variant empty", `{"kind":"sync","interval":60,"must_understand":["/x"],"Must_Understand":[]}`, `"/x"`},
		{"variant empty, exact member lists", `{"kind":"sync","interval":60,"Must_Understand":[],"must_understand":["/x"]}`, `"/x"`},
		{"variant kind is not the kind", `{"KIND":"sync"}`, `no kind ""`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := contracts.CheckChangeMustUnderstand([]byte(tt.entry))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("got %v, want nil", err)
			case tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)):
				t.Errorf("got %v, want an error with %s", err, tt.wantErr)
			}
		})
	}
}

// TestReadersMatchMemberNamesExactly reads a document with a variant of a defined member
// in another letter case, which a schema leaves out as undefined. The variant sets no
// field, and it does not override or merge with the member of the exact case, in either
// order.
func TestReadersMatchMemberNamesExactly(t *testing.T) {
	for name, decode := range decoders {
		t.Run(name, func(t *testing.T) {
			var d contracts.Documents
			if err := decode([]byte(`{"STEP":"a","Domain":"b","MUST_UNDERSTAND":["/x"]}`), &d); err != nil {
				t.Fatal(err)
			}
			if d.Step != "" || d.Domain != "" || d.MustUnderstand != nil {
				t.Errorf("Documents read a variant: %+v", d)
			}
			for _, in := range []string{
				`{"STEP":"a","step":"b","Step":"c","must_understand":["/y"],"Must_Understand":[]}`,
				`{"Must_Understand":[],"must_understand":["/y"],"Step":"c","step":"b","STEP":"a"}`,
			} {
				d = contracts.Documents{}
				if err := decode([]byte(in), &d); err != nil {
					t.Fatal(err)
				}
				if d.Step != "b" || len(d.MustUnderstand) != 1 || d.MustUnderstand[0] != "/y" {
					t.Errorf("Documents of %s: %+v", in, d)
				}
			}

			var a contracts.NextSessionAnswer
			if err := decode([]byte(`{"NEXT":"end","Session":2,"MUST_UNDERSTAND":["/x"]}`), &a); err != nil {
				t.Fatal(err)
			}
			if a.Next != "" || a.Session != 0 || a.MustUnderstand != nil {
				t.Errorf("NextSessionAnswer read a variant: %+v", a)
			}
			a = contracts.NextSessionAnswer{}
			if err := decode([]byte(`{"NEXT":"session","next":"end","Must_Understand":["/z"]}`), &a); err != nil {
				t.Fatal(err)
			}
			if a.Next != "end" || a.MustUnderstand != nil {
				t.Errorf("NextSessionAnswer of an exact next and a variant: %+v", a)
			}

			var c contracts.Change
			if err := decode([]byte(`{"KIND":"comment","Status":"open","MUST_UNDERSTAND":["/x"]}`), &c); err != nil {
				t.Fatal(err)
			}
			if c.Kind != "" || c.Status != "" || c.MustUnderstand != nil {
				t.Errorf("Change read a variant: %+v", c)
			}
			c = contracts.Change{}
			if err := decode([]byte(`{"STATUS":"closed","status":"open","KIND":"x","kind":"sync"}`), &c); err != nil {
				t.Fatal(err)
			}
			if c.Kind != "sync" || c.Status != "open" {
				t.Errorf("Change of exact and variant members: %+v", c)
			}

			// Tier and HarnessSection read themselves with a method.
			var tier contracts.Tier
			if err := decode([]byte(`{"MODEL":"m1","model":"m2","Effort":"high"}`), &tier); err != nil {
				t.Fatal(err)
			}
			if tier.Model != "m2" || tier.Effort != "" {
				t.Errorf("Tier read a variant: %+v", tier)
			}
			var s contracts.HarnessSection
			if err := decode([]byte(`{"NAME":"n1","Bind":{"agents":{"a":"x"}},"bind":{"agents":{"a":"y"}}}`), &s); err != nil {
				t.Fatal(err)
			}
			if s.Name != "" || s.Bind["agents"]["a"].Name != "y" {
				t.Errorf("HarnessSection read a variant: %+v", s)
			}
		})
	}
}

// TestMarshalBytesAreStable pins the bytes that three types write.
func TestMarshalBytesAreStable(t *testing.T) {
	for _, tt := range []struct {
		v    any
		want string
	}{
		{contracts.NextSessionAnswer{Next: "end"}, `{"next":"end"}`},
		{contracts.Change{Kind: "sync", Interval: ptr(60), MustUnderstand: []string{"/interval"}}, `{"kind":"sync","interval":60,"must_understand":["/interval"]}`},
		{contracts.Documents{Step: "<s>", Domain: "d", MustUnderstand: []string{"/x"}}, `{"step":"\u003cs\u003e","domain":"d","inputs":null,"must_understand":["/x"]}`},
	} {
		b, err := json.Marshal(tt.v)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tt.want {
			t.Errorf("%T is written as %s, want %s", tt.v, b, tt.want)
		}
	}
}

func ptr[T any](v T) *T { return &v }
