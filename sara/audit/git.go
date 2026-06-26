package audit

import (
	"context"
	"os"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// HeadCommit returns the HEAD commit hash of the git repo in dir. Commit author
// emails are deliberately NOT read: aurweb does not validate them, so they are
// not a trust anchor (the maintainer account is).
func HeadCommit(ctx context.Context, dir string) (string, error) {
	out, err := gitcmd.Output(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Clone shallow-clones url (optionally at ref) into a temp dir for auditing. The
// caller must call cleanup.
func Clone(ctx context.Context, url, ref string) (dir string, cleanup func(), err error) {
	dir, err = os.MkdirTemp("", "sara-audit-*")
	if err != nil {
		return "", func() {}, utils.WrapErr(err, "failed to create temp dir")
	}
	cleanup = func() { _ = os.RemoveAll(dir) }

	// Full clone: Inspect harvests author emails across history.
	if err := gitcmd.Clone(ctx, gitcmd.CloneOptions{URL: url, Dir: dir, Ref: ref}); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return dir, cleanup, nil
}
