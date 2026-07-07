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

// CachyOS's libalpm stamps each local db entry with %INSTALLED_DB%, the sync
// repo a package came from. The parser must treat it as a known field, not spill
// it into ExtraFields (which would warn and leak it into PKGINFO's XData).
func TestParseDescInstalledDB(t *testing.T) {
	desc := "%NAME%\nlinux-cachyos\n\n%VERSION%\n6.15.4-1\n\n" +
		"%REASON%\n0\n\n%VALIDATION%\npgp\n\n%INSTALLED_DB%\ncachyos\n"

	d, err := raiou.ParseDescString(desc)
	if err != nil {
		t.Fatalf("ParseDescString: %v", err)
	}
	if d.InstalledDB != "cachyos" {
		t.Errorf("InstalledDB = %q, want %q", d.InstalledDB, "cachyos")
	}
	if len(d.ExtraFields) > 0 {
		t.Errorf("INSTALLED_DB must be a known field, got ExtraFields %v", keysOf(d.ExtraFields))
	}
}

func keysOf[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
