package cfkv

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestCompositeKeysAreURLSafe(t *testing.T) {
	cases := []struct{ ns, key string }{
		{"repodb", "alterlinux/x86_64/alterlinux.db.tar.gz"},
		{"aur\x00pkg", "name with spaces and a \x00 null"},
		{"", ""},
		{"日本語", "café/π"},
	}
	safe := func(r rune) bool {
		return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
	}
	for _, c := range cases {
		k := composite(c.ns, c.key)
		for _, r := range k {
			if !safe(r) {
				t.Errorf("composite(%q,%q) = %q has a non-URL-safe rune %q", c.ns, c.key, k, r)
			}
		}
		enc := strings.TrimPrefix(k, nsPrefix(c.ns))
		got, err := base64.RawURLEncoding.DecodeString(enc)
		if err != nil {
			t.Errorf("decode %q: %v", enc, err)
			continue
		}
		if string(got) != c.key {
			t.Errorf("round-trip ns=%q key=%q: got %q", c.ns, c.key, got)
		}
	}
}
