package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/otiai10/copy"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/gitcmd"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// materialize prepares the build source in srcDir from the request. Exactly one
// source must be provided: a git/AUR clone (req.Git) or a raw PKGBUILD
// (req.Pkgbuild). It validates that a PKGBUILD exists in srcDir afterwards.
func materialize(ctx context.Context, req *domain.BuildRequest, srcDir string) error {
	switch {
	case req.Git != nil:
		if err := materializeGit(ctx, req.Git, srcDir); err != nil {
			return err
		}
	case req.Pkgbuild != "":
		if err := materializePkgbuild(req, srcDir); err != nil {
			return err
		}
	default:
		return errors.NewErr("no build source given: set git or pkgbuild")
	}

	if _, err := os.Stat(filepath.Join(srcDir, "PKGBUILD")); err != nil {
		return errors.WrapErr(err, "PKGBUILD not found in source")
	}
	return nil
}

// materializeGit clones git.URL into srcDir. When git.Subdir is set the clone
// happens in a temporary directory and the subdir is copied into srcDir.
//
// git.URL comes from the build request, so the clone is Strict: it validates the
// remote and pins the https connection to a public IP, closing the SSRF into
// internal hosts (e.g. cloud metadata) that a plain clone would leave open. A
// specified ref triggers a full clone so any branch/tag/commit resolves; the
// common no-ref case stays a depth-1 shallow clone.
func materializeGit(ctx context.Context, git *domain.GitSource, srcDir string) error {
	if git.URL == "" {
		return errors.NewErr("git source has no URL")
	}

	dest := srcDir
	var cloneDir string
	if git.Subdir != "" {
		// Reject a subdir that escapes the clone root.
		clean := filepath.Clean(git.Subdir)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return errors.NewErrf("invalid subdir: %s", git.Subdir)
		}

		tmp, err := os.MkdirTemp("", "miko-clone-*")
		if err != nil {
			return errors.WrapErr(err, "failed to create clone dir")
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cloneDir = tmp
		dest = tmp
	}

	opts := gitcmd.CloneOptions{URL: git.URL, Dir: dest, Ref: git.Ref, Strict: true}
	if git.Ref == "" {
		opts.Depth = 1
	}
	if err := gitcmd.Clone(ctx, opts); err != nil {
		return errors.WrapErr(err, "git clone failed")
	}

	if git.Subdir != "" {
		src := filepath.Join(cloneDir, filepath.Clean(git.Subdir))
		if err := copy.Copy(src, srcDir); err != nil {
			return errors.WrapErr(err, "failed to copy subdir")
		}
	}
	return nil
}

// materializePkgbuild writes the raw PKGBUILD and any extra files into srcDir.
// Extra file keys are sanitized with filepath.Base; keys that resolve to ".",
// "..", "" or that contain a path separator are skipped.
func materializePkgbuild(req *domain.BuildRequest, srcDir string) error {
	if err := os.WriteFile(filepath.Join(srcDir, "PKGBUILD"), []byte(req.Pkgbuild), 0o644); err != nil { //nolint:gosec // build input read by makepkg, potentially as a different build user
		return errors.WrapErr(err, "failed to write PKGBUILD")
	}

	for name, contents := range req.Files {
		if strings.ContainsRune(name, filepath.Separator) || strings.ContainsRune(name, '/') {
			continue
		}
		base := filepath.Base(name)
		if base == "." || base == ".." || base == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(srcDir, base), []byte(contents), 0o644); err != nil { //nolint:gosec // build input read by makepkg, potentially as a different build user
			return errors.WrapErr(err, "failed to write file: "+base)
		}
	}
	return nil
}
