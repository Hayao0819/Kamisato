package repository

import (
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// transientAliasStore returns a transient (non-NotFound) error for one object name
// (the .db.tar.gz archive) while every other name behaves normally, modelling a
// backend read hiccup on a populated repo.
type transientAliasStore struct {
	*memStore
	failName string
	err      error
}

func (t *transientAliasStore) FetchFile(repo, arch, name string) (stream.File, error) {
	if name == t.failName {
		return nil, t.err
	}
	return t.memStore.FetchFile(repo, arch, name)
}

// A transient error reading <repo>.db.tar.gz must be surfaced by FetchFile(<repo>.db),
// not masked as the bare-name ErrNotFound — otherwise the upload version gate would
// read a backend hiccup as "no prior version" and fail open.
func TestFetchFileSurfacesTransientAliasError(t *testing.T) {
	transient := errors.New("s3: transient 503")
	store := &transientAliasStore{memStore: newMemStore(), failName: "r.db.tar.gz", err: transient}
	r := NewBinaryRepository(store)

	_, err := r.FetchFile("r", "x86_64", "r.db")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("transient archive error was masked as ErrNotFound: %v", err)
	}
	if !strings.Contains(err.Error(), "transient") {
		t.Fatalf("expected the transient error surfaced, got %v", err)
	}
}

// A genuinely absent db (fresh repo: neither bare name nor archive stored) still
// reads as ErrNotFound so the first upload is accepted.
func TestFetchFileFreshRepoIsNotFound(t *testing.T) {
	r := NewBinaryRepository(newMemStore())
	if _, err := r.FetchFile("r", "x86_64", "r.db"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("fresh repo db read = %v, want ErrNotFound", err)
	}
}
