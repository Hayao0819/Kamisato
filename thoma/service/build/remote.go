package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	pacmanhost "github.com/Hayao0819/Kamisato/internal/pacman/host"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	miko "github.com/Hayao0819/Kamisato/miko/client"
	thomaconfig "github.com/Hayao0819/Kamisato/thoma/config"
)

// detectArch resolves the build arch. makepkg.conf's CARCH is authoritative and
// can disagree with `uname -m` (armv7h userland on an aarch64 kernel, an i686
// build chroot on x86_64), so read it first and fall back to uname only when it
// is unset or unreadable. configPath is the AUR helper's --config, so the arch
// matches the makepkg.conf that computes the package list.
func detectArch(ctx context.Context, configPath string) string {
	var cfg *pacmanhost.MakepkgConfig
	var err error
	if configPath != "" {
		cfg, err = pacmanhost.ReadMakepkgConfigFile(configPath)
	} else {
		cfg, err = pacmanhost.ReadMakepkgConfig()
	}
	if err == nil && cfg.CARCH != "" {
		return cfg.CARCH
	}
	if out, err := exec.CommandContext(ctx, "uname", "-m").Output(); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" {
			return a
		}
	}
	return "x86_64"
}

type Options struct {
	Config        string
	Buildscript   string
	Dir           string
	IgnoreArch    bool
	RunCheck      *bool
	RunVerify     *bool
	SkipChecksums bool
	SkipPGPCheck  bool
}

// Client is the remote job/artifact boundary used by this workflow. Both the
// direct Miko client and Ayato's proxied client implement it.
type Client interface {
	SubmitBuild(context.Context, *miko.BuildRequest) (string, error)
	WaitJob(context.Context, string, io.Writer) (*miko.Job, error)
	DownloadPackageFile(context.Context, string, string, string, string) error
	DownloadArtifact(context.Context, string, string, io.Writer) error
}

func Run(ctx context.Context, cfg *thomaconfig.ThomaConfig, buildAPI Client, stdout, stderr io.Writer, options Options) error {
	if cfg == nil || buildAPI == nil {
		return errors.NewErr("remote build requires configuration and a client")
	}
	// Architecture fallback is per invocation, not a mutation of caller settings.
	resolvedConfig := *cfg
	cfg = &resolvedConfig
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	workDir, err := buildDirectory(options.Dir)
	if err != nil {
		return err
	}
	configPath := options.Config
	if configPath != "" && !filepath.IsAbs(configPath) {
		configPath = filepath.Join(workDir, configPath)
	}
	if cfg.Arch == "" {
		cfg.Arch = detectArch(ctx, configPath)
	}

	pkgbuild, files, err := pacmanpkg.ReadInlineBuildScript(workDir, options.Buildscript, func(name string, size int64) {
		fmt.Fprintf(stderr, "thoma: skipping large file %q (%d bytes); miko fetches sources itself\n", name, size)
	})
	if err != nil {
		return err
	}

	req := &miko.BuildRequest{
		Repo:          cfg.Repo,
		Arch:          cfg.Arch,
		Pkgbuild:      pkgbuild,
		Files:         files,
		Timeout:       cfg.Timeout,
		IgnoreArch:    options.IgnoreArch,
		RunCheck:      options.RunCheck,
		RunVerify:     options.RunVerify,
		SkipChecksums: options.SkipChecksums,
		SkipPGPCheck:  options.SkipPGPCheck,
	}
	// Direct mode leaves the build unsigned on the worker so thoma can pull the
	// artifact straight from the job: miko retains artifacts only for client-sign
	// jobs, and deliberately does not publish them to the shared repo.
	if cfg.Direct() {
		req.SignMode = "client"
	}

	mode := cfg.Mode
	if mode == "" {
		mode = thomaconfig.ThomaModeAyato
	}
	fmt.Fprintf(stderr, "thoma: delegating build to %s (mode %s, repo %s, arch %s)\n", cfg.Server, mode, cfg.Repo, cfg.Arch)
	jobID, err := buildAPI.SubmitBuild(ctx, req)
	if err != nil {
		return errors.WrapErr(err, "failed to submit build")
	}
	fmt.Fprintf(stderr, "thoma: build job %s\n", jobID)

	job, err := buildAPI.WaitJob(ctx, jobID, stdout)
	if err != nil {
		return errors.WrapErr(err, "failed while waiting for build")
	}
	if job.Status != "success" {
		if job.Err != "" {
			return errors.NewErrf("remote build %s: %s", job.Status, job.Err)
		}
		return errors.NewErrf("remote build %s", job.Status)
	}

	dests, err := packageDests(ctx, cfg.Makepkg, workDir, configPath, options.Buildscript)
	if err != nil {
		return err
	}
	return placePackages(ctx, stderr, cfg, buildAPI, jobID, dests, job.Packages)
}

func buildDirectory(dir string) (string, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", errors.WrapErr(err, "failed to get working directory")
		}
		return cwd, nil
	}
	resolved, err := filepath.Abs(dir)
	if err != nil {
		return "", errors.WrapErr(err, "failed to resolve build directory")
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", errors.WrapErr(err, "failed to open build directory")
	}
	if !info.IsDir() {
		return "", errors.NewErrf("build directory is not a directory: %s", resolved)
	}
	return resolved, nil
}

// packageDests asks the real makepkg where the output packages belong, so the
// downloaded artifacts land exactly where yay's --packagelist told it to look.
func packageDests(ctx context.Context, mkpkg, dir, config, buildscript string) ([]string, error) {
	args := []string{"--packagelist", "--ignorearch"}
	if config != "" {
		args = append(args, "--config", config)
	}
	if buildscript != "" && buildscript != "PKGBUILD" {
		args = append(args, "-p", buildscript)
	}
	command := exec.CommandContext(ctx, mkpkg, args...)
	command.Dir = dir
	out, err := command.Output()
	if err != nil {
		return nil, errors.WrapErr(err, "failed to run makepkg --packagelist")
	}
	var dests []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dests = append(dests, line)
		}
	}
	if len(dests) == 0 {
		return nil, errors.NewErr("makepkg --packagelist produced no output")
	}
	return dests, nil
}

// placePackages downloads each built package and writes it to the matching
// expected path. Packages are matched by pkgname (stable across pkgver drift on
// VCS packages), and the file is written under the name yay expects so its
// post-build os.Stat and pacman -U succeed.
func placePackages(ctx context.Context, stderr io.Writer, cfg *thomaconfig.ThomaConfig, buildAPI Client, jobID string, dests, built []string) error {
	for _, dest := range dests {
		want := pkgName(filepath.Base(dest))
		match := ""
		for _, b := range built {
			if pkgName(b) == want {
				match = b
				break
			}
		}
		if match == "" {
			return errors.NewErrf("no built package matches %q (built: %s)", filepath.Base(dest), strings.Join(built, ", "))
		}
		if err := downloadBuilt(ctx, cfg, buildAPI, jobID, match, dest); err != nil {
			return err
		}
		fmt.Fprintf(stderr, "thoma: placed %s -> %s\n", match, dest)
	}
	return nil
}

// downloadBuilt fetches one built package to dest. Ayato mode pulls the
// published, host-signed package from ayato's repo route; direct mode pulls the
// unsigned artifact retained on the miko job.
func downloadBuilt(ctx context.Context, cfg *thomaconfig.ThomaConfig, buildAPI Client, jobID, name, dest string) error {
	if !cfg.Direct() {
		return buildAPI.DownloadPackageFile(ctx, cfg.Repo, cfg.Arch, name, dest)
	}
	return safefile.Replace(dest, 0o644, func(dst io.Writer) error { //nolint:gosec // downloaded package artifacts are public
		return buildAPI.DownloadArtifact(ctx, jobID, name, dst)
	})
}

// pkgName extracts pkgname from a conventional package artifact. Invalid
// filenames are returned unchanged so the caller reports a useful non-match.
func pkgName(filename string) string {
	base := filepath.Base(filename)
	file, err := pacmanpkg.ParseArtifact(base)
	if err != nil {
		return base
	}
	coords, err := file.Coordinates()
	if err != nil {
		return base
	}
	return coords.Name
}
