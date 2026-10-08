package raiou_test

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
	"github.com/klauspost/compress/zstd"
)

func TestSyncParseAllDescFiles(t *testing.T) {
	const syncDir = "testdata/sync"

	entries, err := os.ReadDir(syncDir)
	if err != nil {
		t.Fatalf("failed to read %s: %v", syncDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			file, err := os.Open(filepath.Join(syncDir, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			buffered := bufio.NewReader(file)
			magic, err := buffered.Peek(4)
			if err != nil {
				t.Fatal(err)
			}
			var archive io.ReadCloser
			switch {
			case magic[0] == 0x1f && magic[1] == 0x8b:
				archive, err = gzip.NewReader(buffered)
			case string(magic) == "\x28\xb5\x2f\xfd":
				var reader *zstd.Decoder
				reader, err = zstd.NewReader(buffered)
				if err == nil {
					archive = reader.IOReadCloser()
				}
			default:
				t.Fatalf("unsupported fixture compression: %x", magic)
			}
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			reader := tar.NewReader(archive)
			count := 0
			for {
				member, err := reader.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if member.Typeflag != tar.TypeReg || !strings.HasSuffix(member.Name, "/desc") {
					continue
				}
				body, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				desc, err := raiou.ParseDescString(string(body))
				if err != nil {
					t.Fatalf("parse %s: %v", member.Name, err)
				}
				info, err := desc.ToPKGINFO()
				if err != nil || info == nil || info.PkgName == "" {
					t.Fatalf("metadata for %s = %+v, error %v", member.Name, info, err)
				}
				count++
			}
			if count == 0 {
				t.Fatal("fixture contains no desc members")
			}
		})
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
