package builder

import "testing"

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"simple", "'simple'"},
		{"with space", "'with space'"},
		{"$HOME", "'$HOME'"},
		{"`cmd`", "'`cmd`'"},
		{`"double"`, `'"double"'`},
		{"it's", `'it'\''s'`},
		{"a'b'c", `'a'\''b'\''c'`},
	}
	for _, tc := range cases {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
