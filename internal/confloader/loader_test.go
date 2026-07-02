package confloader

import (
	"os"
	"path/filepath"
	"testing"
)

type testCfg struct {
	Port int    `koanf:"port"`
	Name string `koanf:"name"`
}

// TestLoadAbsoluteFile checks that an absolute config path is read as-is rather
// than joined onto each search dir (which would mangle it).
func TestLoadAbsoluteFile(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(abs, []byte(`{"port":9000,"name":"miko"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New[testCfg](".").Dirs("/some/unrelated/dir").Files(abs)
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	cfg, err := l.Get()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 9000 || cfg.Name != "miko" {
		t.Errorf("absolute config not loaded: %+v", cfg)
	}
}

// TestDirPrecedenceProjectLocalWins checks that a file in the first (project-local)
// dir overrides the same file in a later (global) dir, the standard expectation.
func TestDirPrecedenceProjectLocalWins(t *testing.T) {
	local := t.TempDir()
	global := t.TempDir()
	if err := os.WriteFile(filepath.Join(local, "cfg.json"), []byte(`{"port":1,"name":"local"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(global, "cfg.json"), []byte(`{"port":2,"name":"global"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New[testCfg](".").Dirs(local, global).Files("cfg.json")
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	cfg, err := l.Get()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != 1 || cfg.Name != "local" {
		t.Errorf("project-local dir did not win: %+v", cfg)
	}
}

// TestLoadRelativeFileInDir keeps the dir-search behaviour for relative names.
func TestLoadRelativeFileInDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cfg.json"), []byte(`{"port":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	l := New[testCfg](".").Dirs(dir).Files("cfg.json")
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	cfg, _ := l.Get()
	if cfg.Port != 1 {
		t.Errorf("relative config not loaded: %+v", cfg)
	}
}
