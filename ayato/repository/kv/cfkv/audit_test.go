package cfkv

import (
	"encoding/base64"
	"testing"
)

func TestIsAppKey(t *testing.T) {
	appKey := base64.RawURLEncoding.EncodeToString([]byte("pkgfile")) + sep +
		base64.RawURLEncoding.EncodeToString([]byte("x86_64/foo"))
	cases := map[string]bool{
		appKey:        true,
		"test":        false, // hand-typed via the dashboard: no separator
		"foo.bar.baz": false, // too many separators
		"":            false,
		"a.b":         false, // segments are not valid base64url lengths
		appKey + ".x": false,
	}
	for in, want := range cases {
		if got := isAppKey(in); got != want {
			t.Errorf("isAppKey(%q) = %v, want %v", in, got, want)
		}
	}
}
