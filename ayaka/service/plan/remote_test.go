package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/source"
)

func TestResolveDiffServerLegacyArchitecture(t *testing.T) {
	dir := t.TempDir()
	data := []byte(`{"name":"legacy","server":"https://repo.example/legacy/i686"}`)
	if err := os.WriteFile(filepath.Join(dir, "repo.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := source.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveDiffServer("", "", cfg.URL, "i686"); got != "https://repo.example/legacy/i686" {
		t.Fatalf("diff server = %q", got)
	}
}
