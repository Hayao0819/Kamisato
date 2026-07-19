package cfkv

import (
	"encoding/base64"
	"errors"
	"reflect"
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

func TestForChunks(t *testing.T) {
	items := make([]int, bulkChunk*2+3)
	var sizes []int
	if err := forChunks(items, func(chunk []int) error {
		sizes = append(sizes, len(chunk))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if want := []int{bulkChunk, bulkChunk, 3}; !reflect.DeepEqual(sizes, want) {
		t.Errorf("chunk sizes = %v, want %v", sizes, want)
	}
}

func TestForChunksStopsOnError(t *testing.T) {
	sentinel := errors.New("stop")
	calls := 0
	err := forChunks(make([]int, bulkChunk+1), func([]int) error {
		calls++
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want sentinel", err)
	}
	if calls != 1 {
		t.Errorf("callback calls = %d, want 1", calls)
	}
}
