package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto"
	"encoding/base64"
	"errors"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/klauspost/compress/zstd"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type member struct {
	name    string
	dir     bool
	content string
}

var repoToolBackends = []struct {
	name         string
	tool         Tool
	needsRepoAdd bool
}{
	{"native", NativeTool{}, false},
	{"cli", CLITool{}, true},
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
	defer func() { _ = gz.Close() }()
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

// pkginfoBuild renders a .PKGINFO with the common baseline fields, an explicit
// arch, and any extra "key = value" lines inserted before the pkgtype xdata.
func pkginfoBuild(name, base, ver, arch string, size int, extra ...string) string {
	lines := []string{
		"pkgname = " + name,
		"pkgbase = " + base,
		"pkgver = " + ver,
		"pkgdesc = a sample package",
		"url = https://example.com",
		"builddate = 1700000000",
		"packager = Dev <dev@example.com>",
		"size = " + strconv.Itoa(size),
		"arch = " + arch,
		"license = MIT",
		"license = GPL2",
		"depend = glibc",
		"depend = curl>=7.0",
		"optdepend = bash: scripts",
		"provides = libsample",
		"conflict = oldsample",
	}
	lines = append(lines, extra...)
	lines = append(lines, "xdata = pkgtype=pkg", "")
	return strings.Join(lines, "\n")
}

func pkginfoFull(name, base, ver string, size int) string {
	return pkginfoBuild(name, base, ver, "x86_64", size)
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

func samplePackage(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "sample-1.0-1-x86_64.pkg.tar.zst")
	buildPkg(t, path, pkginfoSample("sample", "sample", "1.0-1"), sampleMembers())
	return path
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
	for _, be := range repoToolBackends {
		t.Run(be.name, func(t *testing.T) {
			if be.needsRepoAdd {
				requireRepoAdd(t)
			}

			dir := t.TempDir()
			pkgPath := samplePackage(t, dir)

			dbPath := filepath.Join(dir, "r.db.tar.gz")
			if err := be.tool.RepoAdd(dbPath, pkgPath, false, nil); err != nil {
				t.Fatalf("RepoAdd: %v", err)
			}

			assertArtifactQuartet(t, dir)
			if !be.needsRepoAdd {
				assertNativeByteCopies(t, dir)
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

func TestWriteToolArchiveFailurePreservesExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repo.db.tar.gz")
	if err := os.WriteFile(path, []byte("published"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeErr := errors.New("archive generation failed")
	err := writeToolArchive(path, func(w io.Writer) error {
		if _, err := io.WriteString(w, "partial"); err != nil {
			return err
		}
		return writeErr
	})
	if !errors.Is(err, writeErr) {
		t.Fatalf("writeToolArchive error = %v, want generation failure", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "published" {
		t.Fatalf("failed archive generation replaced database with %q", got)
	}
}

// TestRepoAddBatch adds several packages in a single RepoAddBatch call and
// asserts every one lands in the database, for both backends.
func TestRepoAddBatch(t *testing.T) {
	names := []string{"alpha", "bravo", "charlie"}
	for _, be := range repoToolBackends {
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

// descPGPSIG parses a raw desc member and returns its %PGPSIG% value.
func descPGPSIG(t *testing.T, desc []byte) string {
	t.Helper()
	d, err := raiou.ParseDescString(string(desc))
	if err != nil {
		t.Fatalf("parse desc: %v", err)
	}
	return d.PGPSIG
}

// TestNativeEmbedsAdjacentPGPSIG checks that a detached signature written beside a
// package as "<pkg>.sig" is embedded into the desc as base64 %PGPSIG%, and — when
// repo-add is available — that the value matches repo-add --include-sigs exactly.
func TestNativeEmbedsAdjacentPGPSIG(t *testing.T) {
	// A non-armored (binary) signature; both repo-add --include-sigs and the
	// native tool base64-encode the raw bytes verbatim.
	sig := []byte{0x89, 0x01, 0x0d, 0x03, 0x00, 0xde, 0xad, 0xbe, 0xef, 0x00, 0x7f}
	want := base64.StdEncoding.EncodeToString(sig)

	build := func(dir string) string {
		pkgPath := samplePackage(t, dir)
		if err := os.WriteFile(pkgPath+".sig", sig, 0o644); err != nil {
			t.Fatal(err)
		}
		return pkgPath
	}

	dir := t.TempDir()
	pkgPath := build(dir)
	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAddBatch(dbPath, []string{pkgPath}, false, nil); err != nil {
		t.Fatalf("RepoAddBatch: %v", err)
	}
	desc, ok := readMemberSuffix(t, dbPath, "/desc")
	if !ok {
		t.Fatal("desc member missing")
	}
	if got := descPGPSIG(t, desc); got != want {
		t.Errorf("native %%PGPSIG%% = %q, want %q", got, want)
	}

	if _, err := exec.LookPath("repo-add"); err != nil {
		t.Skip("repo-add not installed; skipping --include-sigs parity")
	}
	refDir := t.TempDir()
	refPkg := build(refDir)
	refDB := filepath.Join(refDir, "r.db.tar.gz")
	if out, err := exec.Command("repo-add", "-q", "-R", "--nocolor", "--include-sigs", refDB, refPkg).CombinedOutput(); err != nil {
		t.Fatalf("repo-add --include-sigs: %v: %s", err, out)
	}
	refDesc, ok := readMemberSuffix(t, refDB, "/desc")
	if !ok {
		t.Fatal("repo-add desc member missing")
	}
	if got := descPGPSIG(t, refDesc); got != want {
		t.Errorf("repo-add %%PGPSIG%% = %q, want %q", got, want)
	}
}

// TestNativeNoAdjacentSig checks that a package with no "<pkg>.sig" beside it
// yields a desc with no %PGPSIG% field.
func TestNativeNoAdjacentSig(t *testing.T) {
	dir := t.TempDir()
	pkgPath := samplePackage(t, dir)

	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAddBatch(dbPath, []string{pkgPath}, false, nil); err != nil {
		t.Fatalf("RepoAddBatch: %v", err)
	}
	desc, ok := readMemberSuffix(t, dbPath, "/desc")
	if !ok {
		t.Fatal("desc member missing")
	}
	if got := descPGPSIG(t, desc); got != "" {
		t.Errorf("unsigned package has %%PGPSIG%% = %q, want empty", got)
	}
	if bytes.Contains(desc, []byte("%PGPSIG%")) {
		t.Error("desc contains a %PGPSIG% field for an unsigned package")
	}
}

// TestNativeRejectsArmoredSig checks that an ASCII-armored signature next to a
// package is rejected rather than embedded (pacman cannot use armored sigs).
func TestNativeRejectsArmoredSig(t *testing.T) {
	dir := t.TempDir()
	pkgPath := samplePackage(t, dir)
	armored := "-----BEGIN PGP SIGNATURE-----\n\nc2ln\n-----END PGP SIGNATURE-----\n"
	if err := os.WriteFile(pkgPath+".sig", []byte(armored), 0o644); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "r.db.tar.gz")
	if err := (NativeTool{}).RepoAddBatch(dbPath, []string{pkgPath}, false, nil); err == nil {
		t.Fatal("expected an error for an armored package signature")
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
	pkgPath := samplePackage(t, dir)

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

// readArchiveMembers reads every member of a gzipped tar into a name->content
// map, so two archives can be compared as a set regardless of member order:
// repo-add writes each .files package as <dir>/, <dir>/files, <dir>/desc while
// the native writer emits desc before files, and package dirs come out in
// different orders — only the set of names and their contents must match.
func readArchiveMembers(t *testing.T, archivePath string) map[string]string {
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
	defer func() { _ = gz.Close() }()
	members := map[string]string{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar %s: %v", archivePath, err)
		}
		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s in %s: %v", hdr.Name, archivePath, err)
		}
		members[hdr.Name] = string(b)
	}
	return members
}

func assertSameMembers(t *testing.T, kind string, ref, ours map[string]string) {
	t.Helper()
	for name, refContent := range ref {
		ourContent, ok := ours[name]
		if !ok {
			t.Errorf("%s: native missing member %q", kind, name)
			continue
		}
		if refContent != ourContent {
			t.Errorf("%s member %q differs:\n--- repo-add ---\n%s\n--- native ---\n%s", kind, name, refContent, ourContent)
		}
	}
	for name := range ours {
		if _, ok := ref[name]; !ok {
			t.Errorf("%s: native has extra member %q not produced by repo-add", kind, name)
		}
	}
}

func assertArtifactQuartet(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"r.db", "r.db.tar.gz", "r.files", "r.files.tar.gz"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("artifact %s missing in %s: %v", name, dir, err)
		}
	}
}

// assertNativeByteCopies checks the bare <repo>.db / <repo>.files are byte copies
// of their archives. repo-add makes them symlinks instead, so the reference dir is
// intentionally not byte-compared this way.
func assertNativeByteCopies(t *testing.T, dir string) {
	t.Helper()
	for _, pair := range [][2]string{{"r.db.tar.gz", "r.db"}, {"r.files.tar.gz", "r.files"}} {
		archive, err := os.ReadFile(filepath.Join(dir, pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		copyBytes, err := os.ReadFile(filepath.Join(dir, pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(archive, copyBytes) {
			t.Errorf("%s is not a byte copy of %s", pair[1], pair[0])
		}
	}
}

// TestNativeMatchesRepoAdd is the differential check: given the same package set,
// the native writer's .db and .files archives must be structurally identical to
// what real repo-add produces — the same set of tar members with byte-identical
// contents (order normalized). It skips where repo-add is absent. The table
// exercises the field shapes repo-add's desc writer has to reproduce exactly.
func TestNativeMatchesRepoAdd(t *testing.T) {
	requireRepoAdd(t)

	type diffPkg struct {
		fname   string
		pkginfo string
		members []member
	}
	cases := []struct {
		label string
		pkgs  []diffPkg
	}{
		{"plain", []diffPkg{{"sample-1.0-1-x86_64.pkg.tar.zst", pkginfoSample("sample", "sample", "1.0-1"), sampleMembers()}}},
		{"epoch-and-split-base", []diffPkg{{"sample-lib-2_3.4-5-x86_64.pkg.tar.zst", pkginfoSample("sample-lib", "sample", "2:3.4-5"), sampleMembers()}}},
		{"no-files", []diffPkg{{"empty-1-1-x86_64.pkg.tar.zst", pkginfoSample("empty", "empty", "1-1"), nil}}},
		{"zero-isize", []diffPkg{{"meta-1-1-x86_64.pkg.tar.zst", pkginfoFull("meta", "meta", "1-1", 0), nil}}},
		{"arch-any", []diffPkg{{"anypkg-1.0-1-any.pkg.tar.zst", pkginfoBuild("anypkg", "anypkg", "1.0-1", "any", 8192), sampleMembers()}}},
		{"groups", []diffPkg{{"grouped-1.0-1-x86_64.pkg.tar.zst", pkginfoBuild("grouped", "grouped", "1.0-1", "x86_64", 8192, "group = devel", "group = utils"), sampleMembers()}}},
		{"replaces-make-check", []diffPkg{{"repl-1.0-1-x86_64.pkg.tar.zst", pkginfoBuild("repl", "repl", "1.0-1", "x86_64", 8192, "replaces = oldrepl", "replaces = ancientrepl", "makedepend = cmake", "makedepend = ninja", "checkdepend = python-pytest"), sampleMembers()}}},
		{"split-package", []diffPkg{
			{"split-bin-1.0-1-x86_64.pkg.tar.zst", pkginfoBuild("split-bin", "split", "1.0-1", "x86_64", 8192), sampleMembers()},
			{"split-lib-1.0-1-x86_64.pkg.tar.zst", pkginfoBuild("split-lib", "split", "1.0-1", "x86_64", 4096), sampleMembers()},
		}},
		{"batch", []diffPkg{
			{"alpha-1.0-1-x86_64.pkg.tar.zst", pkginfoSample("alpha", "alpha", "1.0-1"), sampleMembers()},
			{"bravo-2.0-1-x86_64.pkg.tar.zst", pkginfoSample("bravo", "bravo", "2.0-1"), sampleMembers()},
			{"charlie-3.0-1-x86_64.pkg.tar.zst", pkginfoSample("charlie", "charlie", "3.0-1"), sampleMembers()},
		}},
	}

	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			src := t.TempDir()
			var pkgPaths []string
			for _, p := range tc.pkgs {
				pp := filepath.Join(src, p.fname)
				buildPkg(t, pp, p.pkginfo, p.members)
				pkgPaths = append(pkgPaths, pp)
			}

			// Reference: real repo-add (all packages in one invocation).
			refDir := t.TempDir()
			refDB := filepath.Join(refDir, "r.db.tar.gz")
			args := append([]string{"-q", "-R", "--nocolor", refDB}, pkgPaths...)
			if out, err := exec.Command("repo-add", args...).CombinedOutput(); err != nil {
				t.Fatalf("repo-add: %v: %s", err, out)
			}

			// Ours.
			ourDir := t.TempDir()
			ourDB := filepath.Join(ourDir, "r.db.tar.gz")
			if err := (NativeTool{}).RepoAddBatch(ourDB, pkgPaths, false, nil); err != nil {
				t.Fatalf("native RepoAddBatch: %v", err)
			}

			assertSameMembers(t, "db", readArchiveMembers(t, refDB), readArchiveMembers(t, ourDB))
			assertSameMembers(t, "files",
				readArchiveMembers(t, filepath.Join(refDir, "r.files.tar.gz")),
				readArchiveMembers(t, filepath.Join(ourDir, "r.files.tar.gz")))

			assertArtifactQuartet(t, refDir)
			assertArtifactQuartet(t, ourDir)
			assertNativeByteCopies(t, ourDir)
		})
	}
}

// sigSet returns the set of detached-signature file names in dir. repo-add makes
// the bare <repo>.db.sig / <repo>.files.sig as symlinks while the native tool
// writes byte copies, but both appear as directory entries, so the sets compare.
func sigSet(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	set := map[string]bool{}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sig") {
			set[e.Name()] = true
		}
	}
	return set
}

// setupGPGKey provisions a throwaway ed25519 signing key in a private GNUPGHOME so
// repo-add --sign can run, returning the homedir. It skips the caller when gpg is
// absent or the key cannot be generated (e.g. no working gpg-agent).
func setupGPGKey(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("gpg not installed; skipping repo-add --sign structural parity")
	}
	home := t.TempDir()
	if err := os.Chmod(home, 0o700); err != nil {
		t.Fatal(err)
	}
	gen := exec.Command("gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-generate-key", "Ayato Repo Test <repo@test.example>", "ed25519", "sign", "never")
	gen.Env = append(os.Environ(), "GNUPGHOME="+home)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot provision gpg key for repo-add --sign: %v: %s", err, out)
	}
	t.Cleanup(func() {
		kill := exec.Command("gpgconf", "--kill", "gpg-agent")
		kill.Env = append(os.Environ(), "GNUPGHOME="+home)
		_ = kill.Run()
	})
	return home
}

// TestNativeSignedMatchesRepoAdd proves the signed path produces the same SET of
// .sig artifacts as repo-add --sign. Signatures are not byte-compared: EdDSA is
// non-deterministic and the keys differ, so only the structure — which files gain
// a detached signature — is asserted. The native half runs unconditionally; the
// repo-add --sign parity is a subtest that skips unless a gpg key can be set up.
func TestNativeSignedMatchesRepoAdd(t *testing.T) {
	requireRepoAdd(t)

	entity, err := openpgp.NewEntity("ayato db", "test", "db@example.com",
		&packet.Config{Algorithm: packet.PubKeyAlgoEdDSA, DefaultHash: crypto.SHA256})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}

	src := t.TempDir()
	pkgPath := samplePackage(t, src)

	ourDir := t.TempDir()
	ourDB := filepath.Join(ourDir, "r.db.tar.gz")
	if err := NewSigningNativeTool(entity).RepoAddBatch(ourDB, []string{pkgPath}, true, nil); err != nil {
		t.Fatalf("signed native RepoAddBatch: %v", err)
	}
	for _, name := range []string{"r.db.tar.gz.sig", "r.db.sig", "r.files.tar.gz.sig", "r.files.sig"} {
		if _, err := os.Stat(filepath.Join(ourDir, name)); err != nil {
			t.Errorf("native signature %s missing: %v", name, err)
		}
	}
	ourSet := sigSet(t, ourDir)

	t.Run("repo-add-sign-parity", func(t *testing.T) {
		gnupg := setupGPGKey(t)
		refDir := t.TempDir()
		refDB := filepath.Join(refDir, "r.db.tar.gz")
		cmd := exec.Command("repo-add", "-q", "-R", "--nocolor", "--sign", refDB, pkgPath)
		cmd.Env = append(os.Environ(), "GNUPGHOME="+gnupg)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("repo-add --sign unavailable in this environment: %v: %s", err, out)
		}
		if refSet := sigSet(t, refDir); !maps.Equal(refSet, ourSet) {
			t.Errorf("signature set mismatch:\nrepo-add: %v\nnative:   %v", refSet, ourSet)
		}
	})
}
