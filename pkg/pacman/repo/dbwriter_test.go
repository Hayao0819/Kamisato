package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"testing"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func metaFor(name, ver string, files []string) *pkg.BinaryPackageMeta {
	info := raiou.NewPKGINFO()
	info.PkgName = name
	info.PkgVer = ver
	info.Arch = "x86_64"
	info.Size = 1
	return &pkg.BinaryPackageMeta{
		Filename: name + "-" + ver + "-x86_64.pkg.tar.zst",
		CSize:    2,
		SHA256:   "abc",
		Info:     info,
		Files:    files,
	}
}

// readMember returns the bytes of a named member from a gzipped db archive.
func readMember(t *testing.T, archive []byte, name string) ([]byte, bool) {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		if hdr.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return b, true
		}
	}
	return nil, false
}

func TestDBBuilderUpsertAndRead(t *testing.T) {
	b := newDBBuilder()
	if err := b.Upsert(metaFor("foo", "1.0-1", []string{"usr/", "usr/bin/foo"}), nil); err != nil {
		t.Fatal(err)
	}

	var db bytes.Buffer
	if err := b.WriteDB(&db); err != nil {
		t.Fatal(err)
	}
	rr, err := RemoteRepoFromDB("r", bytes.NewReader(db.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Pkgs) != 1 || rr.Pkgs[0].Name() != "foo" {
		t.Fatalf("expected [foo], got %v", rr.Pkgs)
	}

	// The .db archive must NOT carry a files member; the .files archive must.
	if _, ok := readMember(t, db.Bytes(), "foo-1.0-1/files"); ok {
		t.Error(".db archive unexpectedly contains a files member")
	}
	var files bytes.Buffer
	if err := b.WriteFiles(&files); err != nil {
		t.Fatal(err)
	}
	got, ok := readMember(t, files.Bytes(), "foo-1.0-1/files")
	if !ok {
		t.Fatal(".files archive missing files member")
	}
	if string(got) != "%FILES%\nusr/\nusr/bin/foo\n" {
		t.Errorf("files member = %q", got)
	}
}

func TestDBBuilderUpsertReplacesSameName(t *testing.T) {
	b := newDBBuilder()
	if err := b.Upsert(metaFor("foo", "1.0-1", nil), nil); err != nil {
		t.Fatal(err)
	}
	if err := b.Upsert(metaFor("foo", "2.0-1", nil), nil); err != nil {
		t.Fatal(err)
	}

	var db bytes.Buffer
	if err := b.WriteDB(&db); err != nil {
		t.Fatal(err)
	}
	rr, err := RemoteRepoFromDB("r", bytes.NewReader(db.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Pkgs) != 1 {
		t.Fatalf("expected exactly 1 entry after re-add, got %d", len(rr.Pkgs))
	}
	if rr.Pkgs[0].Version() != "2.0-1" {
		t.Errorf("expected version 2.0-1, got %q", rr.Pkgs[0].Version())
	}
}

func TestDBBuilderRemove(t *testing.T) {
	b := newDBBuilder()
	if err := b.Upsert(metaFor("foo", "1.0-1", nil), nil); err != nil {
		t.Fatal(err)
	}
	if !b.Remove("foo") {
		t.Error("Remove(foo) reported nothing removed")
	}
	if b.Remove("foo") {
		t.Error("second Remove(foo) reported a removal")
	}
	var db bytes.Buffer
	if err := b.WriteDB(&db); err != nil {
		t.Fatal(err)
	}
	rr, err := RemoteRepoFromDB("r", bytes.NewReader(db.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Pkgs) != 0 {
		t.Fatalf("expected empty db, got %d entries", len(rr.Pkgs))
	}
}

// TestDBBuilderPreservesUntouchedFiles proves a read-modify-write keeps an
// untouched package's files list, which the desc parser would otherwise drop.
func TestDBBuilderPreservesUntouchedFiles(t *testing.T) {
	// Build an initial .files archive holding package "a" with a files list.
	initial := newDBBuilder()
	if err := initial.Upsert(metaFor("a", "1-1", []string{"usr/", "usr/lib/a.so"}), nil); err != nil {
		t.Fatal(err)
	}
	var filesArchive bytes.Buffer
	if err := initial.WriteFiles(&filesArchive); err != nil {
		t.Fatal(err)
	}
	var dbArchive bytes.Buffer
	if err := initial.WriteDB(&dbArchive); err != nil {
		t.Fatal(err)
	}

	// Reload it, add an unrelated package "b", and re-emit the files archive.
	b := newDBBuilder()
	if err := b.LoadDB(bytes.NewReader(dbArchive.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := b.LoadFiles(bytes.NewReader(filesArchive.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := b.Upsert(metaFor("b", "1-1", []string{"usr/bin/b"}), nil); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := b.WriteFiles(&out); err != nil {
		t.Fatal(err)
	}

	aFiles, ok := readMember(t, out.Bytes(), "a-1-1/files")
	if !ok {
		t.Fatal("untouched package a lost its files member")
	}
	if string(aFiles) != "%FILES%\nusr/\nusr/lib/a.so\n" {
		t.Errorf("a files = %q", aFiles)
	}
	if _, ok := readMember(t, out.Bytes(), "b-1-1/files"); !ok {
		t.Error("added package b missing its files member")
	}
}

func TestDBBuilderRejectsStaleFilesEntries(t *testing.T) {
	canonical := newDBBuilder()
	if err := canonical.Upsert(metaFor("foo", "2.0-1", []string{"usr/bin/foo-v2"}), nil); err != nil {
		t.Fatal(err)
	}
	var canonicalDB bytes.Buffer
	if err := canonical.WriteDB(&canonicalDB); err != nil {
		t.Fatal(err)
	}

	stale := newDBBuilder()
	if err := stale.Upsert(metaFor("foo", "1.0-1", []string{"usr/bin/foo-v1"}), nil); err != nil {
		t.Fatal(err)
	}
	if err := stale.Upsert(metaFor("removed", "1.0-1", []string{"usr/bin/removed"}), nil); err != nil {
		t.Fatal(err)
	}
	var staleFiles bytes.Buffer
	if err := stale.WriteFiles(&staleFiles); err != nil {
		t.Fatal(err)
	}

	b := newDBBuilder()
	if err := b.LoadDB(bytes.NewReader(canonicalDB.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := b.LoadFiles(bytes.NewReader(staleFiles.Bytes())); err != nil {
		t.Fatal(err)
	}
	if got, err := b.missingFileObjects(); err != nil || len(got) != 1 || got[0] != "foo-2.0-1-x86_64.pkg.tar.zst" {
		t.Fatalf("missing files = %v, want canonical foo v2 only", got)
	}

	var outDB bytes.Buffer
	if err := b.WriteDB(&outDB); err != nil {
		t.Fatal(err)
	}
	rr, err := RemoteRepoFromDB("r", bytes.NewReader(outDB.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(rr.Pkgs) != 1 || rr.Pkgs[0].Name() != "foo" || rr.Pkgs[0].Version() != "2.0-1" {
		t.Fatalf("stale files changed canonical packages: %v", rr.Pkgs)
	}
}
