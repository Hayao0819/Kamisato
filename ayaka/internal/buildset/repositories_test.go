package buildset

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

func TestLoadRepositoriesReadsFileServer(t *testing.T) {
	dir := t.TempDir()
	packagePath := filepath.Join(dir, "provided-1-1-x86_64.pkg.tar")
	if err := writeTestPackage(packagePath, "provided", "1-1", "x86_64"); err != nil {
		t.Fatal(err)
	}
	if err := (repo.NativeTool{}).RepoAddBatch(filepath.Join(dir, "profile.db.tar.gz"), []string{packagePath}, false, nil); err != nil {
		t.Fatal(err)
	}
	server := (&url.URL{Scheme: "file", Path: dir}).String()
	config := &host.BuildConfig{Repositories: []host.BuildRepository{{
		Name: "profile", Servers: []string{server}, Usage: []string{"Install"},
	}}}
	snapshots, err := loadRepositories(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("snapshots = %+v", snapshots)
	}
	candidates := repositoryCandidates(snapshots[0], "provided>=1", "x86_64")
	if len(candidates) != 1 || candidates[0] != "provided" {
		t.Fatalf("candidates = %v", candidates)
	}
	directories, err := configuredLocalRepositoryDirs(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(directories) != 1 || directories[0] != dir {
		t.Fatalf("local repository directories = %v", directories)
	}
}

func TestLoadRepositoriesSkipsNonInstallUsage(t *testing.T) {
	snapshots, err := loadRepositories(context.Background(), &host.BuildConfig{Repositories: []host.BuildRepository{{
		Name: "sync-only", Usage: []string{"Sync"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("snapshots = %+v", snapshots)
	}
}
