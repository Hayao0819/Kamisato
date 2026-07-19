package buildenv

import (
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestPacmanRepoStanzas(t *testing.T) {
	got, err := PacmanRepoStanzas([]builder.PacmanRepository{
		{Name: "ayato", Server: "https://repo.example.com/$repo/$arch"},
		{Name: "signed", Server: "https://s.example.com/$arch", SigLevel: "Required"},
	})
	if err != nil {
		t.Fatal(err)
	}
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
}

func TestExtraReposScript(t *testing.T) {
	if s, err := ExtraReposScript(nil); err != nil || s != "" {
		t.Errorf("no repos should render an empty script, got %q", s)
	}
	if _, err := ExtraReposScript([]builder.PacmanRepository{{Name: "x"}}); err == nil {
		t.Error("incomplete repo should be rejected")
	}
	s, err := ExtraReposScript([]builder.PacmanRepository{{Name: "ayato", Server: "https://r/$repo/$arch"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(s, "cat >> /etc/pacman.conf <<'KAMISATO_EXTRA_REPO_EOF'\n") {
		t.Errorf("script should append via a quoted heredoc, got:\n%s", s)
	}
	if !strings.HasSuffix(s, "KAMISATO_EXTRA_REPO_EOF") {
		t.Errorf("heredoc not terminated:\n%s", s)
	}
	if !strings.Contains(s, "[ayato]") {
		t.Errorf("script missing repo stanza:\n%s", s)
	}
	if _, err := ExtraReposScript([]builder.PacmanRepository{{
		Name:   "ayato",
		Server: "https://r\nKAMISATO_EXTRA_REPO_EOF\ntouch /build/out/PWNED",
	}}); err == nil {
		t.Fatal("newline/heredoc injection should be rejected")
	}
}
