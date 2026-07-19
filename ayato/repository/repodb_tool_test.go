package repository

import (
	"bytes"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/stream"
)

type fakeTool struct{}

func (fakeTool) RepoAdd(dbPath, _ string, _ bool, _ *string) error {
	return writeFakeArtifacts(dbPath)
}

func (fakeTool) RepoAddBatch(
	dbPath string,
	_ []string,
	_ bool,
	_ *string,
) error {
	return writeFakeArtifacts(dbPath)
}

func (fakeTool) RebuildDerived(
	dbPath string,
	_ []string,
	_ bool,
	_ *string,
) error {
	return writeFakeArtifacts(dbPath)
}

func (fakeTool) RepoRemove(dbPath, _ string, _ bool, _ *string) error {
	return writeFakeArtifacts(dbPath)
}

func writeFakeArtifacts(dbPath string) error {
	dir := path.Dir(dbPath)
	base := strings.TrimSuffix(path.Base(dbPath), ".db.tar.gz")
	for _, name := range []string{
		base + ".db",
		base + ".db.tar.gz",
		base + ".files",
		base + ".files.tar.gz",
	} {
		if err := os.WriteFile(path.Join(dir, name), []byte("db"), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func TestRepoDBToolPort(t *testing.T) {
	want := []string{"r.db.tar.gz", "r.files.tar.gz"}
	mem := newMemStore()
	repository := &binaryRepository{Store: mem, tool: fakeTool{}}

	if err := repository.InitArch("r", "x86_64", false, nil); err != nil {
		t.Fatalf("InitArch: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "InitArch")
	assertAliases(t, repository, mem, "InitArch")

	pkg := stream.NewFileStream(
		"foo-1.0-1-x86_64.pkg.tar.zst",
		"application/octet-stream",
		nopSeekCloser{bytes.NewReader([]byte("pkg"))},
	)
	if err := repository.RepoAdd(
		"r", "x86_64", pkg, nil, false, nil,
	); err != nil {
		t.Fatalf("RepoAdd: %v", err)
	}
	got := mem.names("r", "x86_64")
	assertSuperset(t, got, want, "RepoAdd")
	assertAliases(t, repository, mem, "RepoAdd")
	if contains(got, "foo-1.0-1-x86_64.pkg.tar.zst") {
		t.Error("RepoAdd stored the package through the DB path")
	}

	if err := repository.RepoRemove("r", "x86_64", "foo", false, nil); err != nil {
		t.Fatalf("RepoRemove: %v", err)
	}
	assertSuperset(t, mem.names("r", "x86_64"), want, "RepoRemove")
	assertAliases(t, repository, mem, "RepoRemove")
}
