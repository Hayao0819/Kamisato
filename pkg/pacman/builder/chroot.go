package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	utils "github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/otiai10/copy"
)

// chrootBackend builds packages using the devtools clean-chroot flow
// (<ArchBuild> -- makechrootpkg -c -- makepkg ...).
// It runs only on an Arch host and requires root/nspawn.
type chrootBackend struct {
	opts Options
}

func newChrootBackend(opts Options) Backend {
	if len(opts.ExtraRepos) > 0 {
		slog.Warn("chroot backend ignores extra_repos; publish build-chain dependencies as InstallPkgs instead")
	}
	return &chrootBackend{opts: opts}
}

func (b *chrootBackend) Name() string {
	return "chroot"
}

func (b *chrootBackend) Build(ctx context.Context, spec Spec) (*Result, error) {
	if spec.ArchBuild == "" {
		return nil, errors.New("chroot backend requires a devtools wrapper (Spec.ArchBuild), e.g. extra-x86_64-build")
	}
	if spec.SrcDir == "" {
		return nil, errors.New("chroot backend requires Spec.SrcDir")
	}

	if b.opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, b.opts.Timeout)
		defer cancel()
	}

	// Record existing packages before building. makechrootpkg leaves its output
	// in SrcDir (= CWD), so we only take the diff to avoid mistaking leftovers
	// from a previous build or dependencies placed as InstallPkgs for this
	// build's output.
	baseline, err := snapshotPackages(spec.SrcDir)
	if err != nil {
		return nil, err
	}

	var out io.Writer
	if spec.LogWriter != nil {
		out = io.MultiWriter(os.Stdout, spec.LogWriter)
	}

	slog.Info("building package in clean chroot", "dir", spec.SrcDir, "archbuild", spec.ArchBuild, "arch", spec.Arch)
	if err := runChrootBuild(ctx, spec.SrcDir, spec.ArchBuild, spec.InstallPkgs, out); err != nil {
		return nil, utils.WrapErr(err, "failed to build package in chroot")
	}

	built, err := collectNewPackages(spec.SrcDir, baseline)
	if err != nil {
		return nil, err
	}
	if len(built) == 0 {
		return nil, errors.New("no package files (*.pkg.tar.*) were produced")
	}

	return moveToOutDir(built, spec.SrcDir, spec.OutDir)
}

// runChrootBuild runs the devtools clean-chroot flow in dir:
// <archBuild> -- makechrootpkg -c [-I pkg...] -- makepkg --syncdeps ...
func runChrootBuild(ctx context.Context, dir, archBuild string, installPkgs []string, out io.Writer) error {
	makePkgArgs := []string{"--syncdeps", "--noconfirm", "--log", "--holdver"}
	makeChrootPkgArgs := []string{"-c"}
	for _, pkg := range installPkgs {
		makeChrootPkgArgs = append(makeChrootPkgArgs, "-I", pkg)
	}
	slog.Debug("install packages", "pkgs", installPkgs)

	args := append([]string{archBuild}, "--")
	args = append(args, makeChrootPkgArgs...)
	args = append(args, "--")
	args = append(args, makePkgArgs...)

	build := cmdContext(ctx, dir, out, args...)
	slog.Debug("build command", "cmd", build.String())
	return build.Run()
}

func cmdContext(ctx context.Context, dir string, out io.Writer, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	if out == nil {
		out = os.Stdout
	}
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	return cmd
}

// moveToOutDir moves built (absolute paths) into outDir and returns the final
// absolute paths. If outDir equals srcDir (or is empty), it returns them as-is
// without moving.
func moveToOutDir(built []string, srcDir, outDir string) (*Result, error) {
	if outDir == "" {
		outDir = srcDir
	}
	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve src dir")
	}
	absOut, err := filepath.Abs(outDir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to resolve out dir")
	}
	if absOut == absSrc {
		return &Result{Packages: built}, nil
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		return nil, utils.WrapErr(err, "failed to create output directory")
	}
	packages := make([]string, 0, len(built))
	for _, p := range built {
		dst := filepath.Join(absOut, filepath.Base(p))
		if err := moveFile(p, dst); err != nil {
			return nil, utils.WrapErr(err, "failed to move package to output directory")
		}
		packages = append(packages, dst)
	}
	return &Result{Packages: packages}, nil
}

// moveFile renames src to dst, falling back to a mode-preserving copy+remove
// when the two live on different filesystems (os.Rename is single-device only).
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copy.Copy(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// snapshotPackages returns the set of package file names (*.pkg.tar.*)
// currently present in dir. If dir does not exist, it returns an empty set.
func snapshotPackages(dir string) (map[string]struct{}, error) {
	set := map[string]struct{}{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return set, nil
		}
		return nil, utils.WrapErr(err, "failed to snapshot package dir")
	}
	for _, entry := range entries {
		if entry.IsDir() || !isPackageFile(entry.Name()) {
			continue
		}
		set[entry.Name()] = struct{}{}
	}
	return set, nil
}

// collectNewPackages returns the absolute paths of package files in dir that
// are not in baseline (i.e. produced by this build). Signature files (*.sig)
// are excluded.
func collectNewPackages(dir string, baseline map[string]struct{}) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, utils.WrapErr(err, "failed to read package dir")
	}
	var pkgs []string
	for _, entry := range entries {
		if entry.IsDir() || !isPackageFile(entry.Name()) {
			continue
		}
		if _, ok := baseline[entry.Name()]; ok {
			continue
		}
		abs, err := filepath.Abs(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, utils.WrapErr(err, "failed to resolve package path")
		}
		pkgs = append(pkgs, abs)
	}
	return pkgs, nil
}

// pkgFileExts are the trailing extensions of Arch package files.
var pkgFileExts = []string{
	".pkg.tar.zst",
	".pkg.tar.xz",
	".pkg.tar.gz",
	".pkg.tar.bz2",
	".pkg.tar.lrz",
	".pkg.tar.lzo",
	".pkg.tar.Z",
	".pkg.tar",
}

// isPackageFile reports whether name is a build-output package (*.pkg.tar.*).
// Signature files (*.sig) are not considered output.
func isPackageFile(name string) bool {
	if strings.HasSuffix(name, ".sig") {
		return false
	}
	for _, ext := range pkgFileExts {
		if len(name) > len(ext) && strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}
