package s3

import (
	"errors"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
)

// newTestS3 builds an S3 with a real (offline) client so presigning — a local
// signature computation, not a network call — produces a URL.
func newTestS3(t *testing.T, repoNames []string) *S3 {
	t.Helper()
	s, err := New(&Config{
		Bucket:          "test-bucket",
		Region:          "auto",
		Endpoint:        "https://example.r2.cloudflarestorage.com",
		AccessKeyID:     "AKID",
		SecretAccessKey: "secret",
		UsePathStyle:    true,
		RepoNames:       repoNames,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestStoreFileWithSignedPutURL(t *testing.T) {
	s := newTestS3(t, []string{"core", "extra"})

	url, err := s.StoreFileWithSignedPutURL("core", "x86_64", "pkg-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatalf("presign PUT for allowlisted repo: %v", err)
	}
	if url == "" {
		t.Fatal("presign PUT returned an empty URL")
	}
	if !strings.Contains(url, "core/x86_64/pkg-1.0-1-x86_64.pkg.tar.zst") {
		t.Errorf("presigned URL %q does not target the final key", url)
	}

	// An unlisted repo is refused before any presign, mapped to ErrNotFound.
	if _, err := s.StoreFileWithSignedPutURL("evil", "x86_64", "p.pkg.tar.zst"); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("presign PUT for unlisted repo = %v, want ErrNotFound", err)
	}

	// Traversal is rejected by the same validatedKey guard.
	if _, err := s.StoreFileWithSignedPutURL("core", "..", "p"); err == nil {
		t.Fatal("presign PUT accepted a traversal arch")
	}
}
