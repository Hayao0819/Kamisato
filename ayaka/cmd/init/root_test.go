package initcmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitWritesFiles(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "newrepo")

	cmd := Cmd()
	cmd.SetArgs([]string{"--name", "mypkgs", "--maintainer", "Test User <test@example.com>", targetDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	wantFiles := []string{
		".ayakarc.json",
		filepath.Join("mypkgs", "repo.json"),
		"out",
	}
	for _, rel := range wantFiles {
		if _, err := os.Stat(filepath.Join(targetDir, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}
}

func TestInitDefaultName(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "newrepo")

	cmd := Cmd()
	cmd.SetArgs([]string{targetDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// default --name is "myrepo"
	if _, err := os.Stat(filepath.Join(targetDir, "myrepo", "repo.json")); err != nil {
		t.Errorf("expected myrepo/repo.json with default --name: %v", err)
	}
}

func TestInitRejectsExistingNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	// write a file so the dir is non-empty
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := Cmd()
	cmd.SetArgs([]string{dir})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for non-empty target dir, got nil")
	}
}

func TestInitDestDir(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "newrepo")
	customDest := filepath.Join(dir, "custom-dest")

	cmd := Cmd()
	cmd.SetArgs([]string{"--dest-dir", customDest, targetDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if _, err := os.Stat(customDest); err != nil {
		t.Errorf("expected custom dest dir %s to exist: %v", customDest, err)
	}
}
