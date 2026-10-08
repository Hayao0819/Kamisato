package hook

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestReadTargetsUsesProvidedInput(t *testing.T) {
	names, err := ReadTargets(strings.NewReader(" foo \n\nbar\r\n baz\t\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"foo", "bar", "baz"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("targets = %q, want %q", names, want)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestReadTargetsReturnsReadFailure(t *testing.T) {
	want := errors.New("input failed")
	input := io.MultiReader(strings.NewReader("foo\n"), failingReader{want})
	names, err := ReadTargets(input)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want input failure", err)
	}
	if !reflect.DeepEqual(names, []string{"foo"}) {
		t.Fatalf("targets = %q, want parsed prefix", names)
	}
}

func TestReadTargetsRejectsOversizedTarget(t *testing.T) {
	if _, err := ReadTargets(strings.NewReader(strings.Repeat("a", 128<<10))); err == nil {
		t.Fatal("oversized target silently accepted")
	}
}
