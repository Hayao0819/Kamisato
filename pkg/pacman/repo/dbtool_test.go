package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/klauspost/compress/zstd"
)

type member struct {
	name    string
	dir     bool
	content string
}

// buildPkg writes a .pkg.tar.zst (pure Go) with the given .PKGINFO and members.
func buildPkg(t *testing.T, path, pkginfo string, members []member) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw, err := zstd.NewWriter(f)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(zw)

	write := func(name string, isDir bool, content string) {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if isDir {
			hdr.Typeflag = tar.TypeDir
			hdr.Mode = 0o755
			hdr.Size = 0
			content = ""
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if content != "" {
			if _, err := tw.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
	}

	write(".PKGINFO", false, pkginfo)
	for _, m := range members {
		write(m.name, m.dir, m.content)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func readMemberSuffix(t *testing.T, archivePath, suffix string) ([]byte, bool) {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open %s: %v", archivePath, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip %s: %v", archivePath, err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar %s: %v", archivePath, err)
		}
		if strings.HasSuffix(hdr.Name, suffix) {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return b, true
		}
	}
	return nil, false
}

func pkginfoFull(name, base, ver string, size int) string {
	return strings.Join([]string{
		"pkgname = " + name,
		"pkgbase = " + base,
		"pkgver = " + ver,
		"pkgdesc = a sample package",
		"url = https://example.com",
		"builddate = 1700000000",
		"packager = Dev <dev@example.com>",
		"size = " + strconv.Itoa(size),
		"arch = x86_64",
		"license = MIT",
		"license = GPL2",
		"depend = glibc",
		"depend = curl>=7.0",
		"optdepend = bash: scripts",
		"provides = libsample",
		"conflict = oldsample",
		"xdata = pkgtype=pkg",
		"",
	}, "\n")
}

func pkginfoSample(name, base, ver string) string {
	return pkginfoFull(name, base, ver, 8192)
}

func sampleMembers() []member {
	return []member{
		{name: "usr/", dir: true},
		{name: "usr/bin/", dir: true},
		{name: "usr/bin/sample", content: "binary"},
		{name: "usr/lib/", dir: true},
		{name: "usr/lib/libsample.so", content: "lib"},
	}
}

func requireRepoAdd(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("repo-add"); err != nil {
		t.Skip("repo-add not installed; skipping CLI backend")
	}
}

// TestBackendsRepoAddAndRemove drives BOTH the Go-native writer and the repo-add
// CLI through the same add/read/remove cycle, asserting they produce the same
// artifact quartet, the same readable package, and the same files listing. The
// CLI subtest skips where repo-add is absent.
func TestBackendsRepoAddAndRemove(t *testing.T) {
	backends := []struct {
		name         string
		tool         Tool
		needsRepoAdd bool
	}{
		{"native", NativeTool{}, false},
		{"cli", CLITool{}, true},
	}

	for _, be := range backends {
		t.Run(be.name, func(t *testing.T) {
			if be.needsRepoAdd {
				requireRepoAdd(t)
			}

			dir := t.TempDir()
			pkgPath := filepath.Join(dir, "sample-1.0-1-x86_64.pkg.tar.zst")
			buildPkg(t, pkgPath, pkginfoSample("sample", "sample", "1.0-1"), sampleMembers())

			dbPath := filepath.Join(dir, "r.db.tar.gz")
			if err := be.tool.RepoAdd(dbPath, pkgPath, false, nil); err != nil {
				t.Fatalf("RepoAdd: %v", err)
			}

			for _, name := range []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Errorf("artifact %s missing: %v", name, err)
				}
			}

			rr, err := RepoFromDBFile("r", dbPath)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if len(rr.Pkgs) != 1 || rr.Pkgs[0].Name() != "sample" {
				t.Fatalf("expected [sample], got %v", rr.Pkgs)
			}

			files, ok := readMemberSuffix(t, filepath.Join(dir, "r.files.tar.gz"), "/files")
			if !ok {
				t.Fatal("files member missing")
			}
			want := "%FILES%\nusr/\nusr/bin/\nusr/bin/sample\nusr/lib/\nusr/lib/libsample.so\n"
			if string(files) != want {
				t.Errorf("files member = %q, want %q", files, want)
			}

			if err := be.tool.RepoRemove(dbPath, "sample", false, nil); err != nil {
				t.Fatalf("RepoRemove: %v", err)
			}
			rr, err = RepoFromDBFile("r", dbPath)
			if err != nil {
				t.Fatalf("read back after remove: %v", err)
			}
			if len(rr.Pkgs) != 0 {
				t.Fatalf("expected empty db after remove, got %d", len(rr.Pkgs))
			}
		})
	}
}

func TestNativeRepoAddAndRemove(t *testing.T) {
	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "sample-1.0-1-x86_64.pkg.tar.zst")
	buildPkg(t, pkgPath, pkginfoSample("sample", "sample", "1.0-1"), sampleMembers())

	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAdd(dbPath, pkgPath, false, nil); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}

	// .db copy must be byte-identical to the .db.tar.gz archive (no symlinks in blob).
	archive, _ := os.ReadFile(dbPath)
	copyBytes, _ := os.ReadFile(filepath.Join(dir, "r.db"))
	if !bytes.Equal(archive, copyBytes) {
		t.Error("r.db is not a byte copy of r.db.tar.gz")
	}
}

// TestRepoAddBatch adds several packages in a single RepoAddBatch call and
// asserts every one lands in the database, for both backends.
func TestRepoAddBatch(t *testing.T) {
	backends := []struct {
		name         string
		tool         Tool
		needsRepoAdd bool
	}{
		{"native", NativeTool{}, false},
		{"cli", CLITool{}, true},
	}
	names := []string{"alpha", "bravo", "charlie"}
	for _, be := range backends {
		t.Run(be.name, func(t *testing.T) {
			if be.needsRepoAdd {
				requireRepoAdd(t)
			}
			dir := t.TempDir()
			var pkgPaths []string
			for _, n := range names {
				p := filepath.Join(dir, n+"-1.0-1-x86_64.pkg.tar.zst")
				buildPkg(t, p, pkginfoSample(n, n, "1.0-1"), sampleMembers())
				pkgPaths = append(pkgPaths, p)
			}
			dbPath := filepath.Join(dir, "r.db.tar.gz")
			if err := be.tool.RepoAddBatch(dbPath, pkgPaths, false, nil); err != nil {
				t.Fatalf("RepoAddBatch: %v", err)
			}
			rr, err := RepoFromDBFile("r", dbPath)
			if err != nil {
				t.Fatalf("read back: %v", err)
			}
			if len(rr.Pkgs) != len(names) {
				t.Fatalf("expected %d packages, got %d", len(names), len(rr.Pkgs))
			}
			got := map[string]bool{}
			for _, p := range rr.Pkgs {
				got[p.Name()] = true
			}
			for _, n := range names {
				if !got[n] {
					t.Errorf("package %q missing from batch db", n)
				}
			}
		})
	}
}

func TestNativeInitEmpty(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAdd(dbPath, "", false, nil); err != nil {
		t.Fatalf("init RepoAdd: %v", err)
	}
	for _, name := range []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("artifact %s missing: %v", name, err)
		}
	}
	rr, err := RepoFromDBFile("r", dbPath)
	if err != nil {
		t.Fatalf("read back empty: %v", err)
	}
	if len(rr.Pkgs) != 0 {
		t.Fatalf("expected empty db, got %d", len(rr.Pkgs))
	}
}

func TestNativeSignedUnsupported(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAdd(dbPath, "", true, nil); !errors.Is(err, ErrSignedDBUnsupported) {
		t.Fatalf("expected ErrSignedDBUnsupported, got %v", err)
	}
}

// TestNativeSignedDB checks that a signing tool produces the .db/.files signatures
// (both archive-name and bare-name aliases) and that they verify with the key.
func TestNativeSignedDB(t *testing.T) {
	entity, err := openpgp.NewEntity("ayato db", "test", "db@example.com",
		&packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}

	dir := t.TempDir()
	pkgPath := filepath.Join(dir, "sample-1.0-1-x86_64.pkg.tar.zst")
	buildPkg(t, pkgPath, pkginfoSample("sample", "sample", "1.0-1"), sampleMembers())

	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := NewSigningNativeTool(entity).RepoAddBatch(dbPath, []string{pkgPath}, true, nil); err != nil {
		t.Fatalf("signed RepoAddBatch: %v", err)
	}

	// pacman fetches <repo>.db.sig, so both the archive-name signature and the
	// bare-name alias must exist for the .db and the .files.
	for _, name := range []string{"r.db.tar.gz.sig", "r.db.sig", "r.files.tar.gz.sig", "r.files.sig"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("signature %s missing: %v", name, err)
		}
	}

	keyring := openpgp.EntityList{entity}
	verify := func(archive, sig string) {
		a, err := os.Open(filepath.Join(dir, archive))
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		s, err := os.Open(filepath.Join(dir, sig))
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		if _, err := openpgp.CheckDetachedSignature(keyring, a, s, nil); err != nil {
			t.Errorf("verify %s against %s: %v", sig, archive, err)
		}
	}
	verify("r.db.tar.gz", "r.db.tar.gz.sig")
	verify("r.files.tar.gz", "r.files.tar.gz.sig")
}

func TestNativeRepoRemoveMissing(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAdd(dbPath, "", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := (NativeTool{}).RepoRemove(dbPath, "ghost", false, nil); err == nil {
		t.Fatal("expected error removing a package that is not present")
	}
}

// TestNativeMatchesRepoAdd is the differential check: given the same package, the
// native writer's desc and files members must byte-match what the real repo-add
// produces. It is the strongest fidelity guarantee and skips where repo-add is
// absent. The zero-isize case locks the metapackage (%ISIZE%\n0) behavior.
func TestNativeMatchesRepoAdd(t *testing.T) {
	requireRepoAdd(t)

	cases := []struct {
		label, fname, pkginfo string
		members               []member
	}{
		{"plain", "sample-1.0-1-x86_64.pkg.tar.zst", pkginfoSample("sample", "sample", "1.0-1"), sampleMembers()},
		{"epoch-and-split-base", "sample-lib-2_3.4-5-x86_64.pkg.tar.zst", pkginfoSample("sample-lib", "sample", "2:3.4-5"), sampleMembers()},
		{"no-files", "empty-1-1-x86_64.pkg.tar.zst", pkginfoSample("empty", "empty", "1-1"), nil},
		{"zero-isize", "meta-1-1-x86_64.pkg.tar.zst", pkginfoFull("meta", "meta", "1-1", 0), nil},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			src := t.TempDir()
			pkgPath := filepath.Join(src, tc.fname)
			buildPkg(t, pkgPath, tc.pkginfo, tc.members)

			// Reference: real repo-add.
			refDir := t.TempDir()
			refDB := filepath.Join(refDir, "r.db.tar.gz")
			cmd := exec.Command("repo-add", "-q", "-R", "--nocolor", refDB, pkgPath)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("repo-add: %v: %s", err, out)
			}

			// Ours.
			ourDir := t.TempDir()
			ourDB := filepath.Join(ourDir, "r.db.tar.gz")
			if err := (NativeTool{}).RepoAdd(ourDB, pkgPath, false, nil); err != nil {
				t.Fatalf("native RepoAdd: %v", err)
			}

			refDesc, _ := readMemberSuffix(t, refDB, "/desc")
			ourDesc, ok := readMemberSuffix(t, ourDB, "/desc")
			if !ok {
				t.Fatal("our desc missing")
			}
			if !bytes.Equal(refDesc, ourDesc) {
				t.Errorf("desc differs from repo-add:\n--- repo-add ---\n%s\n--- native ---\n%s", refDesc, ourDesc)
			}

			refFiles, _ := readMemberSuffix(t, filepath.Join(refDir, "r.files.tar.gz"), "/files")
			ourFiles, _ := readMemberSuffix(t, filepath.Join(ourDir, "r.files.tar.gz"), "/files")
			if !bytes.Equal(refFiles, ourFiles) {
				t.Errorf("files differs from repo-add:\n--- repo-add ---\n%s\n--- native ---\n%s", refFiles, ourFiles)
			}
		})
	}
}
