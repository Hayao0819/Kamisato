package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/ayaka/app"
	"github.com/Hayao0819/Kamisato/internal/pacman/source"
)

func TestCompleteSrcRepoNamesLoadsRuntime(t *testing.T) {
	loads := 0
	runtime := app.NewRuntime(func() (*app.App, error) {
		loads++
		return &app.App{SrcRepos: []*source.SourceRepo{
			{Config: &source.SrcConfig{Name: "core"}},
			{Config: &source.SrcConfig{Name: "extra"}},
		}}, nil
	})
	candidates, _ := CompleteSrcRepoNames(runtime)(nil, nil, "")
	if !reflect.DeepEqual(candidates, []string{"core", "extra"}) {
		t.Fatalf("candidates = %v", candidates)
	}
	if loads != 1 {
		t.Fatalf("loader calls = %d, want 1", loads)
	}
}

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
