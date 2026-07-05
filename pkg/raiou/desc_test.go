package raiou_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
)

func TestSyncParseAllDescFiles(t *testing.T) {
	const syncDir = "testdata/sync"

	entries, err := flist.Get(syncDir, flist.WithFileOnly(), flist.WithExtOnly(".db"), flist.WithExactDepth(1))
	if err != nil {
		t.Fatalf("failed to read %s: %v", syncDir, err)
	}

	for _, entry := range *entries {
		name := futils.BaseWithoutExt(entry)
		rr, err := repo.RepoFromDBFile(name, entry)
		if err != nil {
			t.Errorf("failed to parse %s: %v", entry, err)
			continue
		}

		for _, pkg := range rr.Pkgs {
			if pkg.PKGINFO() == nil {
				t.Errorf("failed to get PKGINFO from %s", entry)
			}
		}
	}
}

func TestLocalParseAllDescFiles(t *testing.T) {
	const localDir = "testdata/local"

	entries, err := os.ReadDir(localDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", localDir, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		descPath := filepath.Join(localDir, entry.Name(), "desc")
		data, err := os.ReadFile(descPath)
		if err != nil {
			t.Errorf("failed to read %s: %v", descPath, err)
			continue
		}

		desc, err := raiou.ParseDescString(string(data))
		if err != nil {
			t.Errorf("failed to parse %s: %v", descPath, err)
			continue
		}

		if len(desc.ExtraFields) > 0 {
			t.Errorf("unknown keys found in %s: %v", descPath, keysOf(desc.ExtraFields))
		}
	}
}

func keysOf[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
