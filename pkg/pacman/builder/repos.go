package builder

import (
	"fmt"
	"strings"
)

// RepoSpec is an extra pacman repository injected into the build environment so pre-published dependencies resolve via pacman.
type RepoSpec struct {
	Name   string
	Server string
	// SigLevel defaults to "Optional TrustAll" (no keyring setup needed); set "Required" only when the build env trusts the signing key.
	SigLevel string
}

// pacmanRepoStanzas renders pacman.conf [repo] sections; entries without a name or server are skipped.
// $repo/$arch are emitted verbatim for pacman (not the shell) to expand.
func pacmanRepoStanzas(repos []RepoSpec) string {
	var b strings.Builder
	for _, r := range repos {
		if r.Name == "" || r.Server == "" {
			continue
		}
		sig := r.SigLevel
		if sig == "" {
			sig = "Optional TrustAll"
		}
		fmt.Fprintf(&b, "\n[%s]\nSigLevel = %s\nServer = %s\n", r.Name, sig, r.Server)
	}
	return b.String()
}

// substituteBuildPlaceholders fills __EXTRA_REPOS__ and __INSTALL__ only on their own line (anchored by newlines)
// so mentions inside script comments are left intact.
func substituteBuildPlaceholders(script, reposScript, installScript string) string {
	script = strings.ReplaceAll(script, "\n__EXTRA_REPOS__\n", "\n"+reposScript+"\n")
	script = strings.ReplaceAll(script, "\n__INSTALL__\n", "\n"+installScript+"\n")
	return script
}

// extraReposScript renders the shell snippet that appends repos to /etc/pacman.conf; returns "" when empty so the placeholder collapses.
// The heredoc marker is single-quoted so $repo/$arch pass through to pacman.
func extraReposScript(repos []RepoSpec) string {
	stanzas := pacmanRepoStanzas(repos)
	if strings.TrimSpace(stanzas) == "" {
		return ""
	}
	return "cat >> /etc/pacman.conf <<'KAMISATO_EXTRA_REPO_EOF'\n" + stanzas + "KAMISATO_EXTRA_REPO_EOF"
}
