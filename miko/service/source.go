package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

// materialize prepares the build source in srcDir from the request. Exactly one
// source must be provided: a git/AUR clone (req.Git) or a raw PKGBUILD
// (req.Pkgbuild). It validates that a PKGBUILD exists in srcDir afterwards.
func materialize(req *domain.BuildRequest, srcDir string) error {
	switch {
	case req.Git != nil:
		if err := materializeGit(req.Git, srcDir); err != nil {
			return err
		}
	case req.Pkgbuild != "":
		if err := materializePkgbuild(req, srcDir); err != nil {
			return err
		}
	default:
		return utils.NewErr("no build source given: set git or pkgbuild")
	}

	if _, err := os.Stat(filepath.Join(srcDir, "PKGBUILD")); err != nil {
		return utils.WrapErr(err, "PKGBUILD not found in source")
	}
	return nil
}

// materializeGit clones git.URL into srcDir. When git.Subdir is set the clone
// happens in a temporary directory and the subdir is copied into srcDir.
//
// All git arguments are passed as a vector (never a shell string) and the "--"
// separator terminates option parsing, so a malicious URL or ref cannot inject
// extra options or shell syntax.
func materializeGit(git *domain.GitSource, srcDir string) error {
	if git.URL == "" {
		return utils.NewErr("git source has no URL")
	}

	dest := srcDir
	var cloneDir string
	if git.Subdir != "" {
		// Reject a subdir that escapes the clone root.
		clean := filepath.Clean(git.Subdir)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return utils.NewErrf("invalid subdir: %s", git.Subdir)
		}

		tmp, err := os.MkdirTemp("", "miko-clone-*")
		if err != nil {
			return utils.WrapErr(err, "failed to create clone dir")
		}
		defer func() { _ = os.RemoveAll(tmp) }()
		cloneDir = tmp
		dest = tmp
	}

	args := []string{"clone", "--depth", "1"}
	if git.Ref != "" {
		args = append(args, "--branch", git.Ref)
	}
	args = append(args, "--", git.URL, dest)

	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return utils.WrapErr(err, "git clone failed: "+strings.TrimSpace(string(out)))
	}

	if git.Subdir != "" {
		src := filepath.Join(cloneDir, filepath.Clean(git.Subdir))
		if err := utils.CopyDir(src, srcDir); err != nil {
			return utils.WrapErr(err, "failed to copy subdir")
		}
	}
	return nil
}

// materializePkgbuild writes the raw PKGBUILD and any extra files into srcDir.
// Extra file keys are sanitized with filepath.Base; keys that resolve to ".",
// "..", "" or that contain a path separator are skipped.
func materializePkgbuild(req *domain.BuildRequest, srcDir string) error {
	if err := os.WriteFile(filepath.Join(srcDir, "PKGBUILD"), []byte(req.Pkgbuild), 0o644); err != nil {
		return utils.WrapErr(err, "failed to write PKGBUILD")
	}

	for name, contents := range req.Files {
		if strings.ContainsRune(name, filepath.Separator) || strings.ContainsRune(name, '/') {
			continue
		}
		base := filepath.Base(name)
		if base == "." || base == ".." || base == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(srcDir, base), []byte(contents), 0o644); err != nil {
			return utils.WrapErr(err, "failed to write file: "+base)
		}
	}
	return nil
}
