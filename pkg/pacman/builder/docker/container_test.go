package docker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
	"github.com/Hayao0819/Kamisato/pkg/pacman/builder/internal/buildenv"
)

func TestNewDefaults(t *testing.T) {
	backend := New(builder.ResolvedConfig{})
	if backend.image != defaultContainerImage || backend.timeout != 30*time.Minute {
		t.Errorf("defaults = image %q, timeout %s", backend.image, backend.timeout)
	}
}

// The embedded script mentions placeholders in documentation. Substitution
// must replace only standalone lines, otherwise a second heredoc opener leaks
// repository stanza contents into the shell program.
func TestBuildScriptPlaceholderSubstitution(t *testing.T) {
	repos, err := buildenv.ExtraReposScript([]builder.PacmanRepository{{Name: "myrepo", Server: "https://x/$repo/$arch"}})
	if err != nil {
		t.Fatal(err)
	}
	got := buildenv.SubstituteBuildPlaceholders(buildScript, repos, "pacman -U /p")
	if n := strings.Count(got, "<<'KAMISATO_EXTRA_REPO_EOF'"); n != 1 {
		t.Errorf("heredoc opener count = %d, want 1", n)
	}
	if strings.Contains(got, "\n__EXTRA_REPOS__\n") || strings.Contains(got, "\n__INSTALL__\n") {
		t.Error("standalone placeholder was left unsubstituted")
	}
}

func TestCollectStagedPackagesReplacesSameVersion(t *testing.T) {
	staging := t.TempDir()
	out := t.TempDir()
	const filename = "pkg-1-1-x86_64.pkg.tar.zst"
	if err := os.WriteFile(filepath.Join(out, filename), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, filename), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	packages, err := collectStagedPackages(staging, out)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0] != filepath.Join(out, filename) {
		t.Fatalf("packages = %v", packages)
	}
	data, err := os.ReadFile(filepath.Join(out, filename))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Fatalf("same-version output was not replaced: %q", data)
	}

	if _, err := collectStagedPackages(t.TempDir(), out); !errors.Is(err, builder.ErrBuildFailed) {
		t.Fatalf("empty staging dir error = %v, want ErrBuildFailed", err)
	}
}
