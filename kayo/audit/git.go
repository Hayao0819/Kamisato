package audit

import (
	"context"
	"os"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
)

// HeadCommit returns the HEAD commit hash of the git repo in dir. Author emails are
// deliberately not read: aurweb does not validate them, so they are not a trust anchor.
func HeadCommit(ctx context.Context, dir string) (string, error) {
	return gitcmd.HeadCommit(ctx, dir)
}

// Clone clones url (optionally at ref) into a temp dir for auditing. The caller
// must call cleanup.
func Clone(ctx context.Context, url, ref string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "kayo-audit-*")
	if err != nil {
		return "", func() {}, errors.WrapErr(err, "failed to create temp dir")
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	// SSRF/loopback rejection is omitted on purpose: the URL is operator-chosen and
	// local git (file://, http://127.0.0.1) is a supported audit target; ext:: RCE is
	// blocked unconditionally by gitcmd. Full clone so Inspect can read author history.
	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: url, Dir: dir, Ref: ref}); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return dir, cleanup, nil
}
