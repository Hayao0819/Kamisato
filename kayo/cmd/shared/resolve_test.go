package shared

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequirePinnedCommit(t *testing.T) {
	if err := (Resolved{Commit: "abc123"}).RequirePinnedCommit(); err != nil {
		t.Errorf("RequirePinnedCommit with a commit = %v, want nil", err)
	}
	if err := (Resolved{Commit: ""}).RequirePinnedCommit(); err == nil {
		t.Error("RequirePinnedCommit with no commit = nil, want error")
	}
}

const validSrcinfo = `pkgbase = realbase
	pkgver = 1.0.0
	pkgrel = 1
	arch = x86_64

pkgname = realbase
`

// readPkgbase must fall back to the target's basename whenever the .SRCINFO is
// absent, unparseable, or missing pkgbase — never panic on the raiou parser.
func TestReadPkgbase(t *testing.T) {
	tests := []struct {
		name     string
		srcinfo  *string // nil: write no .SRCINFO at all
		fallback string
		want     string
	}{
		{"valid pkgbase wins over fallback", ptr(validSrcinfo), "/tmp/aur/foo", "realbase"},
		{"missing .SRCINFO falls back to basename", nil, "/tmp/aur/foo", "foo"},
		{"malformed .SRCINFO falls back", ptr("not a valid srcinfo at all\n"), "somepkg", "somepkg"},
		{"empty .SRCINFO falls back", ptr(""), "/a/b/barpkg", "barpkg"},
		{"srcinfo without pkgbase falls back", ptr("pkgname = x\n\tpkgver = 1\n"), "zzz", "zzz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.srcinfo != nil {
				if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(*tt.srcinfo), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := readPkgbase(dir, tt.fallback); got != tt.want {
				t.Errorf("readPkgbase(%q) = %q, want %q", tt.fallback, got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }
