// Package clonecache verifies an AUR helper's local clone cache against kayo's
// pinned commit. kayo approves a pkgbase at a reviewed commit, but the helper
// (yay) builds from its OWN checked-out clone under ~/.cache/yay/<pkgbase>. That
// checkout can be advanced past the approved commit or tampered with locally, so
// before a build the checked-out HEAD must be reconciled with the approved pin.
package clonecache

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Hayao0819/Kamisato/kayo/audit"
)

// Dir is the clone-cache working tree the helper keeps for a pkgbase under root.
func Dir(root, pkgbase string) string { return filepath.Join(root, pkgbase) }

// Result reports a clone-cache checkout relative to a pinned commit.
type Result struct {
	Dir     string
	Exists  bool   // the pkgbase has a clone-cache checkout
	Head    string // its checked-out HEAD (empty when !Exists)
	Pinned  string // the approved commit it is checked against
	Matches bool   // Head equals Pinned
}

// Drifted reports an existing checkout that diverged from the pin. A pkgbase the
// helper has not cloned yet is not drift: there is nothing to build from it.
func (r Result) Drifted() bool { return r.Exists && !r.Matches }

// Check reads the pkgbase's clone-cache HEAD and compares it to pinned. A missing
// root or an un-cloned pkgbase yields Exists=false rather than an error, so the
// caller can treat "not cached yet" as nothing to verify.
func Check(ctx context.Context, root, pkgbase, pinned string) (Result, error) {
	dir := Dir(root, pkgbase)
	res := Result{Dir: dir, Pinned: pinned}
	if root == "" {
		return res, nil
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return res, nil // the helper has not cloned this pkgbase
	}
	head, err := audit.HeadCommit(ctx, dir)
	if err != nil {
		return res, err
	}
	res.Exists = true
	res.Head = head
	res.Matches = head == pinned
	return res, nil
}
