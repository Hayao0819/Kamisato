package buildenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/config"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestDirectBuildAppliesMakepkgOptions(t *testing.T) {
	_, _, err := New(builder.HostConfig{}, Options{
		Arch:    "x86_64",
		Makepkg: builder.MakepkgConfig{Options: []string{"!debug"}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDirectBuildUsesArchdockerImageForTargetArchitecture(t *testing.T) {
	_, environment, err := New(builder.HostConfig{}, Options{Arch: "i486"})
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
	host, err := config.LoadDirectBuildHostConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = New(host, Options{Arch: "x86_64"})
	if err == nil || !strings.Contains(err.Error(), "--pacman-conf") {
		t.Fatalf("error = %v", err)
	}
}
