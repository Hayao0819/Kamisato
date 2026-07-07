package builder

import (
	"strings"
	"testing"
)

func TestSubstituteBuildPlaceholdersLeavesCommentsIntact(t *testing.T) {
	// A doc comment mentioning the tokens must survive verbatim; only the
	// standalone placeholder lines are filled in.
	script := "# uses __EXTRA_REPOS__ and __INSTALL__\nset -e\n__EXTRA_REPOS__\necho ok\n__INSTALL__\n"
	got := substituteBuildPlaceholders(script, "cat <<'EOF'\n[repo]\nEOF", "pacman -U x")

	if !strings.Contains(got, "# uses __EXTRA_REPOS__ and __INSTALL__") {
		t.Errorf("comment token was expanded:\n%s", got)
	}
	if strings.Contains(got, "\n__EXTRA_REPOS__\n") || strings.Contains(got, "\n__INSTALL__\n") {
		t.Errorf("standalone placeholder not replaced:\n%s", got)
	}
	if !strings.Contains(got, "[repo]") || !strings.Contains(got, "pacman -U x") {
		t.Errorf("substituted content missing:\n%s", got)
	}
}

// The embedded backend scripts document the placeholders in a comment; the
// heredoc that appends extra repos must open exactly once, or a comment leak
// spills the stanza body as bare commands (sh: [repo]: command not found).
func TestEmbeddedScriptsNoPlaceholderCommentLeak(t *testing.T) {
	repos := extraReposScript([]RepoSpec{{Name: "myrepo", Server: "https://x/$repo/$arch"}})
	for _, s := range []struct {
		name, script string
	}{
		{"container", buildScript},
		{"bwrap", bwrapDepsScript},
	} {
		got := substituteBuildPlaceholders(s.script, repos, "pacman -U /p")
		if n := strings.Count(got, "<<'KAMISATO_EXTRA_REPO_EOF'"); n != 1 {
			t.Errorf("%s: heredoc opener count = %d, want 1 (comment leak?)", s.name, n)
		}
		if strings.Contains(got, "\n__EXTRA_REPOS__\n") || strings.Contains(got, "\n__INSTALL__\n") {
			t.Errorf("%s: a standalone placeholder was left unsubstituted", s.name)
		}
	}
}
