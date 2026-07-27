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

// ExtraReposScript quotes the heredoc marker so pacman, not the shell, expands
// variables. The stanzas go in front of the first repo section, not appended:
// alpm resolves names by repo order without backtracking, so an appended repo
// can never override a same-named package in the distro's own core/extra.
func ExtraReposScript(repos []builder.PacmanRepository) (string, error) {
	stanzas, err := PacmanRepoStanzas(repos)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(stanzas) == "" {
		return "", nil
	}
	return "cat > /run/kamisato-extra-repos.conf <<'KAMISATO_EXTRA_REPO_EOF'\n" + stanzas + "KAMISATO_EXTRA_REPO_EOF\n" +
		`awk 'FNR == NR { extra[++n] = $0; next }
  !done && /^\[/ && $0 != "[options]" { for (i = 1; i <= n; i++) print extra[i]; print ""; done = 1 }
  { print }
  END { if (!done) for (i = 1; i <= n; i++) print extra[i] }' \
  /run/kamisato-extra-repos.conf /etc/pacman.conf > /run/kamisato-pacman.conf
mv /run/kamisato-pacman.conf /etc/pacman.conf`, nil
}
