package raiou_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/remote"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/Hayao0819/nahi/flist"
	"github.com/Hayao0819/nahi/futils"
)

func TestSyncParseAllDescFiles(t *testing.T) {
	const localDBPath = "/var/lib/pacman/sync"

	entries, err := flist.Get(localDBPath, flist.WithFileOnly(), flist.WithExtOnly(".db"), flist.WithExactDepth(1))
	if err != nil {
		t.Fatalf("failed to read %s: %v", localDBPath, err)
	}

	for _, entry := range *entries {
		name := futils.BaseWithoutExt(entry)
		rr, err := remote.RepoFromDBFile(name, entry)
		if err != nil {
			t.Errorf("failed to parse %s: %v", entry, err)
			continue
		}

		// 未知のキーがある場合はテスト失敗
		// if len(rr.Pkgs) > 0 {
		// 	t.Errorf("unknown keys found in %s: %v", descPath, keysOf(desc.ExtraFields))
		// }
		for _, pkg := range rr.Pkgs {
			pi := pkg.MustPKGINFO()
			if pi == nil {
				t.Errorf("failed to get PKGINFO")
				continue
			}
		}
	}
}

func TestLocalParseAllDescFiles(t *testing.T) {
	const localDBPath = "/var/lib/pacman/local"

	entries, err := os.ReadDir(localDBPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", localDBPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		descPath := filepath.Join(localDBPath, entry.Name(), "desc")
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

		// 未知のキーがある場合はテスト失敗
		if len(desc.ExtraFields) > 0 {
			t.Errorf("unknown keys found in %s: %v", descPath, keysOf(desc.ExtraFields))
		}
	}
}

// keysOf extracts keys from a map[string]string into a []string
func keysOf[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
