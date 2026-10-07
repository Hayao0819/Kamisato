package audit

import (
	"context"

	"github.com/Hayao0819/Kamisato/internal/vcs/git"
)

// HeadCommit returns the HEAD commit hash of the git repo in dir. Author emails are
// deliberately not read: aurweb does not validate them, so they are not a trust anchor.
func HeadCommit(ctx context.Context, dir string) (string, error) {
	return git.HeadCommit(ctx, dir)
}

// Clone clones url (optionally at ref) into a temp dir for auditing. The caller
// must call cleanup.
//
// SSRF/loopback rejection is omitted on purpose: the URL is operator-chosen and
// local git (file://, http://127.0.0.1) is a supported audit target; ext:: RCE is
// blocked unconditionally by git. Full clone so Inspect can read author history.
func Clone(ctx context.Context, url, ref string) (dir string, cleanup func(), err error) {
	return git.CloneTemp(ctx, "kayo-audit-*", git.CloneOptions{URL: url, Ref: ref})
}
