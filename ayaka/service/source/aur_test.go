package source

import (
	"context"
	"os"
	"path/filepath"
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

func TestAddAURForceValidatesBeforeRemovingCheckout(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AddAUR(context.Background(), dir, []string{"."}, true); err == nil {
		t.Fatal("accepted invalid package name with --force")
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Fatalf("invalid package name removed repository contents: %v", err)
	}
}

func TestAddAURCanceledForceKeepsCheckout(t *testing.T) {
	dir := t.TempDir()
	checkout := filepath.Join(dir, "example", ".git")
	if err := os.MkdirAll(checkout, 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := AddAUR(ctx, dir, []string{"example"}, true); err == nil {
		t.Fatal("ignored canceled context")
	}
	if _, err := os.Stat(checkout); err != nil {
		t.Fatalf("canceled operation removed checkout: %v", err)
	}
}
