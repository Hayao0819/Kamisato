package depend_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/depend"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want depend.Constraint
	}{
		{"glibc", depend.Constraint{Name: "glibc"}},
		{"glibc>=2.38", depend.Constraint{Name: "glibc", Op: depend.OpGE, Ver: "2.38"}},
		{"foo<=1.0", depend.Constraint{Name: "foo", Op: depend.OpLE, Ver: "1.0"}},
		{"foo>1", depend.Constraint{Name: "foo", Op: depend.OpGT, Ver: "1"}},
		{"foo<1", depend.Constraint{Name: "foo", Op: depend.OpLT, Ver: "1"}},
		{"foo=1.2-3", depend.Constraint{Name: "foo", Op: depend.OpEQ, Ver: "1.2-3"}},
		{"foo=1:2.3-4", depend.Constraint{Name: "foo", Op: depend.OpEQ, Ver: "1:2.3-4"}},
		{"  spaced >= 2 ", depend.Constraint{Name: "spaced", Op: depend.OpGE, Ver: "2"}},
	}
	for _, c := range cases {
		got := depend.Parse(c.in)
		if got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestSatisfies(t *testing.T) {
	cases := []struct {
		spec    string
		version string
		want    bool
	}{
		{"glibc", "2.38", true},
		{"glibc>=2.38", "2.38", true},
		{"glibc>=2.38", "2.40", true},
		{"glibc>=2.38", "2.37", false},
		{"foo<2", "1.9", true},
		{"foo=1.0", "1.0", true},
		{"foo=1.0", "1.1", false},
	}
	for _, c := range cases {
		ok, err := depend.Parse(c.spec).Satisfies(c.version)
		if err != nil {
			t.Fatalf("Satisfies(%q, %q): %v", c.spec, c.version, err)
		}
		if ok != c.want {
			t.Errorf("Parse(%q).Satisfies(%q) = %v, want %v", c.spec, c.version, ok, c.want)
		}
	}
}
