package s3

import (
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/reponame"
)

// offlineStore builds a client against a loopback endpoint with static
// credentials; presigning is a local SigV4 computation and never dials out.
func offlineStore(t *testing.T) *S3 {
	t.Helper()
	store, err := New(&Config{
		Bucket:          "test-bucket",
		Region:          "us-east-1",
		Endpoint:        "http://127.0.0.1:1",
		AccessKeyID:     "AKIAEXAMPLE",
		SecretAccessKey: "secretexample",
		UsePathStyle:    true,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store
}

func TestPresignStagedPutIsAnOfflinePUTIntoTheStagingPrefix(t *testing.T) {
	store := offlineStore(t)

	result, err := store.presignStagedPut("abc123", "pkg-1-1-x86_64.pkg.tar.zst", time.Hour)
	if err != nil {
		t.Fatalf("presignStagedPut: %v", err)
	}
	if result.Method != "PUT" {
		t.Fatalf("Method = %q, want PUT", result.Method)
	}
	// '$' is percent-encoded in the URL path.
	if !strings.Contains(result.URL, "/%24staging/abc123/pkg-1-1-x86_64.pkg.tar.zst") {
		t.Fatalf("URL = %q, want it to contain the staging key", result.URL)
	}
}

func TestStagedKeyRejectsPathEscape(t *testing.T) {
	for _, tc := range []struct{ id, name string }{
		{"..", "pkg.pkg.tar.zst"},
		{"abc123", "../pkg.pkg.tar.zst"},
		{"abc/123", "pkg.pkg.tar.zst"},
		{"abc123", "a/b"},
		{"", "pkg.pkg.tar.zst"},
		{"abc123", ""},
	} {
		if _, err := stagedKey(tc.id, tc.name); err == nil {
			t.Errorf("stagedKey(%q, %q) = nil error, want rejection", tc.id, tc.name)
		}
	}
}

func TestExcludeStagingPrefix(t *testing.T) {
	got := excludeStagingPrefix([]string{"core", stagingPrefix, "extra"})
	want := []string{"core", "extra"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("excludeStagingPrefix = %v, want %v", got, want)
	}
}

func TestStagingPrefixIsOutsideRepoNameGrammar(t *testing.T) {
	if err := reponame.Validate(stagingPrefix); err == nil {
		t.Fatalf("reponame.Validate(%q) = nil, want rejection so it can never collide with a real repo", stagingPrefix)
	}
}
