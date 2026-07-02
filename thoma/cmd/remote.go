package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/srcpkg"
)

// resolveServer reads the ayato URL and CLI token from the same server database
// `ayaka server login` writes, selecting the default server when name is empty.
func resolveServer(name string) (url, token string, err error) {
	info, err := blinkyutils.ResolveServer(name)
	if err != nil {
		if errors.Is(err, blinkyutils.ErrNoServerSpecified) {
			return "", "", utils.NewErr("no ayato server configured; set THOMA_SERVER or run 'ayaka server login'")
		}
		return "", "", err
	}
	return info.URL, info.Password, nil
}

func detectArch() string {
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" {
			return a
		}
	}
	return "x86_64"
}

func remoteBuild(args []string) error {
	cfg, err := conf.LoadThomaConfig(nil)
	if err != nil {
		return err
	}
	if cfg.Arch == "" {
		cfg.Arch = detectArch()
	}

	base, token, err := resolveServer(cfg.Server)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return utils.WrapErr(err, "failed to get working directory")
	}
	pkgbuild, files, err := srcpkg.ReadInline(cwd, func(name string, size int64) {
		fmt.Fprintf(os.Stderr, "thoma: skipping large file %q (%d bytes); miko fetches sources itself\n", name, size)
	})
	if err != nil {
		return err
	}

	req := &ayatoclient.BuildRequest{
		Repo:     cfg.Repo,
		Arch:     cfg.Arch,
		Pkgbuild: pkgbuild,
		Files:    files,
		Timeout:  cfg.Timeout,
	}

	// Cancel the submit and wait on Ctrl-C/SIGTERM so a job stuck in queued does
	// not hang the makepkg invocation forever.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Fprintf(os.Stderr, "thoma: delegating build to %s (repo %s, arch %s)\n", base, cfg.Repo, cfg.Arch)
	jobID, err := ayatoclient.SubmitBuild(ctx, base, token, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}
	fmt.Fprintf(os.Stderr, "thoma: build job %s\n", jobID)

	job, err := ayatoclient.WaitJob(ctx, base, jobID, os.Stdout)
	if err != nil {
		return utils.WrapErr(err, "failed while waiting for build")
	}
	if job.Status != "success" {
		if job.Err != "" {
			return utils.NewErrf("remote build %s: %s", job.Status, job.Err)
		}
		return utils.NewErrf("remote build %s", job.Status)
	}

	dests, err := packageDests(args)
	if err != nil {
		return err
	}
	return placePackages(ctx, cfg, base, dests, job.Packages)
}

// packageDests asks the real makepkg where the output packages belong, so the
// downloaded artifacts land exactly where yay's --packagelist told it to look.
func packageDests(incoming []string) ([]string, error) {
	args := []string{"--packagelist", "--ignorearch"}
	if conf := configArg(incoming); conf != "" {
		args = append(args, "--config", conf)
	}
	out, err := exec.Command(realMakepkg(), args...).Output()
	if err != nil {
		return nil, utils.WrapErr(err, "failed to run makepkg --packagelist")
	}
	var dests []string
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			dests = append(dests, line)
		}
	}
	if len(dests) == 0 {
		return nil, utils.NewErr("makepkg --packagelist produced no output")
	}
	return dests, nil
}

// configArg extracts the makepkg --config value yay forwarded, so the package
// list is computed with the same makepkg.conf.
func configArg(args []string) string {
	for i, a := range args {
		if a == "--config" && i+1 < len(args) {
			return args[i+1]
		}
		if v, ok := strings.CutPrefix(a, "--config="); ok {
			return v
		}
	}
	return ""
}

// placePackages downloads each built package and writes it to the matching
// expected path. Packages are matched by pkgname (stable across pkgver drift on
// VCS packages), and the file is written under the name yay expects so its
// post-build os.Stat and pacman -U succeed.
func placePackages(ctx context.Context, cfg *conf.ThomaConfig, base string, dests, built []string) error {
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
			return utils.NewErrf("no built package matches %q (built: %s)", filepath.Base(dest), strings.Join(built, ", "))
		}
		if err := ayatoclient.DownloadPackage(ctx, base, cfg.Repo, cfg.Arch, match, dest); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "thoma: placed %s -> %s\n", match, dest)
	}
	return nil
}

// pkgName strips the -<ver>-<rel>-<arch>.pkg.tar.* tail from a package filename,
// matching how yay derives the package name (the part before the last three
// dash-separated fields).
func pkgName(filename string) string {
	base := filename
	if i := strings.Index(base, ".pkg.tar"); i >= 0 {
		base = base[:i]
	}
	parts := strings.Split(base, "-")
	if len(parts) <= 3 {
		return base
	}
	return strings.Join(parts[:len(parts)-3], "-")
}
