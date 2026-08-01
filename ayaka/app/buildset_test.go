package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectBuildConfigIsNeverDiscoveredImplicitly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ayakarc.json"), []byte(`{"builder":{"backend":"chroot"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	}()
	config, err := loadDirectBuildHostConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if config.Backend != "" {
		t.Fatalf("implicit config was loaded: %+v", config)
	}
}

func TestDirectBuildUsesArchdockerImageForTargetArchitecture(t *testing.T) {
	_, environment, err := NewDirectBuildApplication(DirectBuildOptions{Arch: "i486"})
	if err != nil {
		t.Fatal(err)
	}
	if environment.Image != "ghcr.io/hayao0819/archlinux:i486" {
		t.Fatalf("image = %q", environment.Image)
	}
}

func TestDirectBuildRejectsRepositoriesOutsidePacmanConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ayaka.json")
	data := []byte(`{"builder":{"backend":"container","repositories":[{"name":"custom","server":"https://repo.example"}]}}`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := NewDirectBuildApplication(DirectBuildOptions{ConfigFile: path, Arch: "x86_64"})
	if err == nil || !strings.Contains(err.Error(), "--pacman-conf") {
		t.Fatalf("error = %v", err)
	}
}
