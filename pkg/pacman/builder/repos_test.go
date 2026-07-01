package builder

import (
	"strings"
	"testing"
)

func TestPacmanRepoStanzas(t *testing.T) {
	got := pacmanRepoStanzas([]RepoSpec{
		{Name: "ayato", Server: "https://repo.example.com/$repo/$arch"},
		{Name: "signed", Server: "https://s.example.com/$arch", SigLevel: "Required"},
		{Name: "", Server: "https://skip.example.com"}, // skipped: no name
		{Name: "noserver"},                             // skipped: no server
	})
	for _, want := range []string{
		"[ayato]",
		"SigLevel = Optional TrustAll", // default when unset
		"Server = https://repo.example.com/$repo/$arch",
		"[signed]",
		"SigLevel = Required",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("stanzas missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "skip.example.com") || strings.Contains(got, "[noserver]") {
		t.Errorf("incomplete repos were not skipped:\n%s", got)
	}
}

func TestExtraReposScript(t *testing.T) {
	if s := extraReposScript(nil); s != "" {
		t.Errorf("no repos should render an empty script, got %q", s)
	}
	// A single incomplete repo also collapses to nothing.
	if s := extraReposScript([]RepoSpec{{Name: "x"}}); s != "" {
		t.Errorf("incomplete repo should render empty, got %q", s)
	}
	s := extraReposScript([]RepoSpec{{Name: "ayato", Server: "https://r/$repo/$arch"}})
	if !strings.HasPrefix(s, "cat >> /etc/pacman.conf <<'KAMISATO_EXTRA_REPO_EOF'\n") {
		t.Errorf("script should append via a quoted heredoc, got:\n%s", s)
	}
	if !strings.HasSuffix(s, "KAMISATO_EXTRA_REPO_EOF") {
		t.Errorf("heredoc not terminated:\n%s", s)
	}
	if !strings.Contains(s, "[ayato]") {
		t.Errorf("script missing repo stanza:\n%s", s)
	}
}
