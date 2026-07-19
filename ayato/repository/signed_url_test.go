package repository

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// presignRecorder is a blob.Store that records the arch+name handed to
// StoreFileWithSignedURL, so a test can assert the alias resolution.
type presignRecorder struct{ gotArch, gotName string }

func (p *presignRecorder) StoreFileWithSignedURL(_, arch, name string) (string, error) {
	p.gotArch, p.gotName = arch, name
	return "https://example.com/" + name, nil
}

func (p *presignRecorder) StoreFile(string, string, stream.SeekFile) error       { return nil }
func (p *presignRecorder) DeleteFile(string, string, string) error               { return nil }
func (p *presignRecorder) FetchFile(string, string, string) (stream.File, error) { return nil, nil }
func (p *presignRecorder) RepoNames() ([]string, error)                          { return nil, nil }
func (p *presignRecorder) Files(string, string) ([]string, error)                { return nil, nil }
func (p *presignRecorder) FilesWithMeta(string, string) ([]blob.FileInfo, error) { return nil, nil }
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

	cases := []struct{ arch, name, wantArch, wantName string }{
		{"x86_64", "core.db", "x86_64", "core.db.tar.gz"},
		{"x86_64", "core.files", "x86_64", "core.files.tar.gz"},
		{"x86_64", "pkg-1-1-x86_64.pkg.tar.zst", "x86_64", "pkg-1-1-x86_64.pkg.tar.zst"},
		// -any packages and their .sig live under "any/"; presign must point there.
		{"x86_64", "pkg-1-1-any.pkg.tar.zst", "any", "pkg-1-1-any.pkg.tar.zst"},
		{"x86_64", "pkg-1-1-any.pkg.tar.zst.sig", "any", "pkg-1-1-any.pkg.tar.zst.sig"},
		{"x86_64", "pkg-1-1-any.pkg.tar", "any", "pkg-1-1-any.pkg.tar"},
		// A look-alike with an unsupported suffix is an ordinary object and must
		// not be routed into the shared any directory.
		{"x86_64", "pkg-1-1-any.pkg.tar.zip", "x86_64", "pkg-1-1-any.pkg.tar.zip"},
	}
	for _, tc := range cases {
		if _, err := r.StoreFileWithSignedURL("core", tc.arch, tc.name); err != nil {
			t.Fatalf("StoreFileWithSignedURL(%q): %v", tc.name, err)
		}
		if rec.gotArch != tc.wantArch || rec.gotName != tc.wantName {
			t.Errorf("presign(%s/%s) = %s/%s, want %s/%s", tc.arch, tc.name, rec.gotArch, rec.gotName, tc.wantArch, tc.wantName)
		}
	}
}
