package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallRendersExecAndUninstall(t *testing.T) {
	dir := t.TempDir()
	tmpl := "[Action]\nExec = @EXEC@\nNeedsTargets\n"

	path, err := Install(dir, "x.hook", tmpl, "/usr/bin/foo verify -c /etc/foo.toml")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if path != filepath.Join(dir, "x.hook") {
		t.Errorf("path = %q", path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), ExecPlaceholder) {
		t.Error("placeholder was not substituted")
	}
	if !strings.Contains(string(got), "Exec = /usr/bin/foo verify -c /etc/foo.toml") {
		t.Errorf("rendered hook missing the exec line:\n%s", got)
	}

	if _, err := Uninstall(dir, "x.hook"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("hook file should be gone after Uninstall")
	}
	// Uninstalling a missing hook is not an error.
	if _, err := Uninstall(dir, "x.hook"); err != nil {
		t.Errorf("Uninstall of a missing hook should be a no-op, got %v", err)
	}
}
