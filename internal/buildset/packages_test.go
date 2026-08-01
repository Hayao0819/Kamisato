package buildset

import (
	"slices"
	"testing"
)

func TestNormalizePackagesValidatesDeduplicatesAndSorts(t *testing.T) {
	got, err := normalizePackages([]string{"zlib-ng", "foo", "zlib-ng"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"foo", "zlib-ng"}) {
		t.Fatalf("packages = %v", got)
	}
}

func TestNormalizePackagesRejectsListSyntax(t *testing.T) {
	for _, name := range []string{"!foo", "foo bar", "#comment"} {
		if _, err := normalizePackages([]string{name}); err == nil {
			t.Errorf("package %q was accepted", name)
		}
	}
}
