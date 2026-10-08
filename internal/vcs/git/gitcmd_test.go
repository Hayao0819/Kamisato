package git

import (
	"io"
	"testing"

	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
)

func TestSetRefValidatesTargetBeforeMutation(t *testing.T) {
	dir := t.TempDir()
	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	blob := &plumbing.MemoryObject{}
	blob.SetType(plumbing.BlobObject)
	writer, err := blob.Writer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(writer, "not a commit"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	hash, err := repo.Storer.SetEncodedObject(blob)
	if err != nil {
		t.Fatal(err)
	}
	const ref = "refs/heads/pinned"
	for _, target := range []string{"invalid", "ffffffffffffffffffffffffffffffffffffffff", hash.String()} {
		if err := SetRef(dir, ref, target); err == nil {
			t.Errorf("branch target %q should be rejected", target)
		}
		if _, err := repo.Reference(plumbing.ReferenceName(ref), false); err != plumbing.ErrReferenceNotFound {
			t.Fatalf("rejected target mutated the branch: %v", err)
		}
	}
	if err := SetRef(dir, "refs/tags/blob", hash.String()); err != nil {
		t.Fatalf("non-branch refs may name existing blobs: %v", err)
	}
}

func TestValidateRemote(t *testing.T) {
	// Allowed forms (IP literals avoid DNS in tests).
	allow := []string{
		"https://8.8.8.8/repo.git",
		"git://8.8.8.8/repo.git",
		"ssh://git@8.8.8.8/repo.git",
		"git@8.8.8.8:user/repo.git", // scp-like ssh
	}
	for _, u := range allow {
		if err := ValidateRemote(u); err != nil {
			t.Errorf("ValidateRemote(%q) = %v, want allowed", u, err)
		}
	}

	// Rejected: SSRF to internal hosts, plaintext http, local/file, and the
	// ext:: transport-helper RCE.
	reject := []string{
		"file:///etc/passwd",
		"ext::sh -c id",
		"ext::sh -c 'touch /tmp/pwned'",
		"http://8.8.8.8/x",
		"/local/path/repo",
		"https://127.0.0.1/x",
		"https://169.254.169.254/latest/meta-data", // cloud metadata
		"https://10.1.2.3/x",
		"git://192.168.0.1/x",
		"ssh://git@127.0.0.1/x", // ssh scheme to an internal host
		"git@10.0.0.5:x",        // scp-like to an internal host
	}
	for _, u := range reject {
		if err := ValidateRemote(u); err == nil {
			t.Errorf("ValidateRemote(%q) = nil, want rejected", u)
		}
	}
}
