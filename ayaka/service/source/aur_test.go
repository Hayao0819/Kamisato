package source

import (
	"context"
	"strings"
	"testing"
)

func TestAurPkgNameRe(t *testing.T) {
	valid := []string{"yay", "yay-bin", "lib32-glibc", "python3.11", "g++", "foo_bar", "0ad", "a.b.c"}
	for _, name := range valid {
		if !aurPkgNameRe.MatchString(name) {
			t.Errorf("aurPkgNameRe rejected valid name %q", name)
		}
	}

	invalid := []string{
		"",
		"..",
		"../x",
		"../../etc/passwd",
		"foo/bar",
		"/abs",
		".hidden",
		"-leading-dash",
		"Upper",
		"with space",
		"semi;colon",
		"new\nline",
	}
	for _, name := range invalid {
		if aurPkgNameRe.MatchString(name) {
			t.Errorf("aurPkgNameRe accepted invalid name %q", name)
		}
	}
}

func TestAddAURRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	err := AddAUR(context.Background(), dir, []string{"../../etc/passwd"}, false)
	if err == nil || !strings.Contains(err.Error(), "invalid AUR package name") {
		t.Errorf("AddAUR with invalid name = %v, want invalid name error", err)
	}
}
