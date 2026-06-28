package aurweb

import "testing"

// TestMatchByCoMaintainers guards the by=comaintainers search field, which was
// advertised as valid (exposedBys) but had no case in Match and so always
// returned empty.
func TestMatchByCoMaintainers(t *testing.T) {
	p := Pkg{Name: "foo", Maintainer: "alice", CoMaintainers: []string{"bob", "carol"}}

	cases := []struct {
		by   By
		arg  string
		want bool
	}{
		{ByCoMaintainers, "bob", true},
		{ByCoMaintainers, "CAROL", true},  // case-insensitive
		{ByCoMaintainers, "alice", false}, // the maintainer is not a co-maintainer
		{ByCoMaintainers, "dave", false},
		{ByMaintainer, "alice", true},
		{ByMaintainer, "bob", false},
	}
	for _, tc := range cases {
		if got := Match(p, tc.by, tc.arg); got != tc.want {
			t.Errorf("Match(by=%q, arg=%q) = %v, want %v", tc.by, tc.arg, got, tc.want)
		}
	}
}
