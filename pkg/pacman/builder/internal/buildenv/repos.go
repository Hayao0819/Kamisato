package buildenv

import (
	"fmt"
	"strings"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

// PacmanRepoStanzas preserves $repo and $arch for pacman to expand.
func PacmanRepoStanzas(repos []builder.PacmanRepository) (string, error) {
	if err := builder.ValidateRepositories(repos); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, r := range repos {
		sig := r.SigLevel
		if sig == "" {
			sig = "Optional TrustAll"
		}
		fmt.Fprintf(&b, "\n[%s]\nSigLevel = %s\nServer = %s\n", r.Name, sig, r.Server)
	}
	return b.String(), nil
}

// SubstituteBuildPlaceholders replaces only standalone tokens, not documentation.
func SubstituteBuildPlaceholders(script, reposScript, installScript string) string {
	script = strings.ReplaceAll(script, "\n__EXTRA_REPOS__\n", "\n"+reposScript+"\n")
	script = strings.ReplaceAll(script, "\n__INSTALL__\n", "\n"+installScript+"\n")
	return script
}

// ExtraReposScript quotes the heredoc marker so pacman, not the shell, expands variables.
func ExtraReposScript(repos []builder.PacmanRepository) (string, error) {
	stanzas, err := PacmanRepoStanzas(repos)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(stanzas) == "" {
		return "", nil
	}
	return "cat >> /etc/pacman.conf <<'KAMISATO_EXTRA_REPO_EOF'\n" + stanzas + "KAMISATO_EXTRA_REPO_EOF", nil
}
