package config

import (
	"os"
	"path/filepath"
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
	config, err := LoadDirectBuildHostConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if config.Backend != "" {
		t.Fatalf("implicit config was loaded: %+v", config)
	}
}
