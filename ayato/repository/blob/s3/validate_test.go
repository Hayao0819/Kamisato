package s3

import (
	"errors"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

func TestKeyConstruction(t *testing.T) {
	if got := key("core", "x86_64", "p.pkg.tar.zst"); got != "core/x86_64/p.pkg.tar.zst" {
		t.Errorf("key = %q, want core/x86_64/p.pkg.tar.zst", got)
	}
}

func TestS3ObjectStreamContentTypeDefault(t *testing.T) {
	s := &s3ObjectStream{}
	if got := s.ContentType(); got != "application/octet-stream" {
		t.Errorf("empty ContentType = %q, want application/octet-stream", got)
	}
	s.contentType = "text/plain"
	if got := s.ContentType(); got != "text/plain" {
		t.Errorf("ContentType = %q, want text/plain", got)
	}
}

func TestValidatedKey(t *testing.T) {
	s := &S3{repoNames: []string{"core", "extra"}}

	k, err := s.validatedKey("core", "x86_64", "pkg-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatalf("validatedKey valid input: %v", err)
	}
	if want := "core/x86_64/pkg-1.0-1-x86_64.pkg.tar.zst"; k != want {
		t.Fatalf("validatedKey = %q, want %q", k, want)
	}

	// An unlisted repo is ErrNotFound so the transport serves 404, not 500.
	if _, err := s.validatedKey("evil", "x86_64", "p.pkg.tar.zst"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("validatedKey(unlisted repo) = %v, want ErrNotFound", err)
	}

	bad := []struct{ repo, arch, name string }{
		{"core", "..", "p"},
		{"core", "x86_64", "../p"},
		{"core", "x86_64", "a/b"},
		{"../core", "x86_64", "p"},
		{"core", "", "p"},
		{"core", "x86_64", ""},
	}
	for _, tc := range bad {
		// Traversal is a validation error (400), distinct from a missing repo (404).
		if _, err := s.validatedKey(tc.repo, tc.arch, tc.name); err == nil || errors.Is(err, blob.ErrNotFound) {
			t.Fatalf("validatedKey(%q,%q,%q) = %v, want a non-ErrNotFound error", tc.repo, tc.arch, tc.name, err)
		}
	}

	// With no allowlist, any clean repo passes but traversal is still rejected.
	open := &S3{}
	if _, err := open.validatedKey("anything", "x86_64", "p.pkg.tar.zst"); err != nil {
		t.Fatalf("validatedKey without allowlist: %v", err)
	}
	if _, err := open.validatedKey("..", "x86_64", "p"); err == nil {
		t.Fatal("validatedKey without allowlist allowed a traversal repo")
	}
}

// The list entry points must reject the same bad repo/arch as the key path,
// before any prefix is built (validation precedes the S3 call).
func TestListMethodsValidate(t *testing.T) {
	s := &S3{repoNames: []string{"core", "extra"}}

	for _, repo := range []string{"..", "a/b", "/abs", "evil"} {
		if _, err := s.Arches(repo); err == nil {
			t.Fatalf("Arches(%q) = nil, want error", repo)
		}
		if _, err := s.Files(repo, "x86_64"); err == nil {
			t.Fatalf("Files(%q, x86_64) = nil, want error", repo)
		}
	}
	for _, arch := range []string{"..", "a/b", "/abs", ""} {
		if _, err := s.Files("core", arch); err == nil {
			t.Fatalf("Files(core, %q) = nil, want error", arch)
		}
	}
}
