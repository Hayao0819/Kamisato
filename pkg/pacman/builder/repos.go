package builder

import (
	"fmt"
	"strings"
)

// RepoSpec is an extra pacman repository exposed inside the build environment so
// dependencies already published there — e.g. the ayato repo holding built AUR or
// first-party packages — resolve via `pacman -S` during a build.
type RepoSpec struct {
	Name   string
	Server string
	// SigLevel is the pacman SigLevel for the repo. Empty defaults to
	// "Optional TrustAll", which needs no keyring setup in the build environment;
	// set "Required" only when the environment trusts the repo's signing key.
	SigLevel string
}

// pacmanRepoStanzas renders pacman.conf [repo] sections for repos. Entries missing
// a name or server are skipped. The server may contain pacman's $repo/$arch
// variables; they are emitted verbatim for pacman (not the shell) to expand.
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

// extraReposScript renders a shell snippet that appends the repo stanzas to
// /etc/pacman.conf inside the build environment. It returns "" when there is
// nothing to add so the script placeholder collapses to nothing. The heredoc
// marker is single-quoted so the shell leaves $repo/$arch for pacman.
func extraReposScript(repos []RepoSpec) string {
	stanzas := pacmanRepoStanzas(repos)
	if strings.TrimSpace(stanzas) == "" {
		return ""
	}
	return "cat >> /etc/pacman.conf <<'KAMISATO_EXTRA_REPO_EOF'\n" + stanzas + "KAMISATO_EXTRA_REPO_EOF"
}
