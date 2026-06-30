package repository

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// presignRecorder is a blob.Store that records the name handed to
// StoreFileWithSignedURL, so a test can assert the DB-alias resolution.
type presignRecorder struct{ gotName string }

func (p *presignRecorder) StoreFileWithSignedURL(_, _, name string) (string, error) {
	p.gotName = name
	return "https://example.com/" + name, nil
}

func (p *presignRecorder) StoreFile(string, string, stream.SeekFile) error       { return nil }
func (p *presignRecorder) DeleteFile(string, string, string) error               { return nil }
func (p *presignRecorder) FetchFile(string, string, string) (stream.File, error) { return nil, nil }
func (p *presignRecorder) RepoNames() ([]string, error)                          { return nil, nil }
func (p *presignRecorder) Files(string, string) ([]string, error)               { return nil, nil }
func (p *presignRecorder) Arches(string) ([]string, error)                       { return nil, nil }

func (p *presignRecorder) FetchFileWithETag(string, string, string) (stream.File, string, error) {
	return nil, "", nil
}

func (p *presignRecorder) StoreFileIfMatch(string, string, stream.SeekFile, string) error {
	return nil
}

func TestBinaryRepoSignedURLResolvesDBAlias(t *testing.T) {
	rec := &presignRecorder{}
	r := NewBinaryRepository(rec)

	cases := []struct{ name, want string }{
		{"core.db", "core.db.tar.gz"},
		{"core.files", "core.files.tar.gz"},
		{"pkg-1-1-x86_64.pkg.tar.zst", "pkg-1-1-x86_64.pkg.tar.zst"},
	}
	for _, tc := range cases {
		if _, err := r.StoreFileWithSignedURL("core", "x86_64", tc.name); err != nil {
			t.Fatalf("StoreFileWithSignedURL(%q): %v", tc.name, err)
		}
		if rec.gotName != tc.want {
			t.Errorf("presigned object for %q = %q, want %q", tc.name, rec.gotName, tc.want)
		}
	}
}
