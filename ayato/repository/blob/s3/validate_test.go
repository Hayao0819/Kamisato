package s3

import "testing"

func TestValidatedKey(t *testing.T) {
	s := &S3{repoNames: []string{"core", "extra"}}

	k, err := s.validatedKey("core", "x86_64", "pkg-1.0-1-x86_64.pkg.tar.zst")
	if err != nil {
		t.Fatalf("validatedKey valid input: %v", err)
	}
	if want := "core/x86_64/pkg-1.0-1-x86_64.pkg.tar.zst"; k != want {
		t.Fatalf("validatedKey = %q, want %q", k, want)
	}

	if _, err := s.validatedKey("evil", "x86_64", "p.pkg.tar.zst"); err == nil {
		t.Fatal("validatedKey allowed a repo outside the allowlist")
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
		if _, err := s.validatedKey(tc.repo, tc.arch, tc.name); err == nil {
			t.Fatalf("validatedKey(%q,%q,%q) = nil, want error", tc.repo, tc.arch, tc.name)
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
