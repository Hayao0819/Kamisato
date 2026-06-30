package dep

import (
	"os/exec"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Constraint
	}{
		{"glibc", Constraint{Name: "glibc"}},
		{"glibc>=2.38", Constraint{Name: "glibc", Op: OpGE, Ver: "2.38"}},
		{"foo<=1.0", Constraint{Name: "foo", Op: OpLE, Ver: "1.0"}},
		{"foo>1", Constraint{Name: "foo", Op: OpGT, Ver: "1"}},
		{"foo<1", Constraint{Name: "foo", Op: OpLT, Ver: "1"}},
		{"foo=1.2-3", Constraint{Name: "foo", Op: OpEQ, Ver: "1.2-3"}},
		{"foo=1:2.3-4", Constraint{Name: "foo", Op: OpEQ, Ver: "1:2.3-4"}},
		{"  spaced >= 2 ", Constraint{Name: "spaced", Op: OpGE, Ver: "2"}},
	}
	for _, c := range cases {
		got := Parse(c.in)
		if got != c.want {
			t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestSatisfies(t *testing.T) {
	if _, err := exec.LookPath("vercmp"); err != nil {
		t.Skip("vercmp not available")
	}
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
		ok, err := Parse(c.spec).Satisfies(c.version)
		if err != nil {
			t.Fatalf("Satisfies(%q, %q): %v", c.spec, c.version, err)
		}
		if ok != c.want {
			t.Errorf("Parse(%q).Satisfies(%q) = %v, want %v", c.spec, c.version, ok, c.want)
		}
	}
}
