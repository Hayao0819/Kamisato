package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
	"github.com/Hayao0819/Kamisato/internal/serverstore"
	"github.com/Hayao0819/Kamisato/pkg/pacman/makepkgconf"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/pkgfile"
)

// resolveServer resolves a named or default Ayato server.
func resolveServer(name string) (*serverstore.Endpoint, error) {
	info, err := serverstore.Resolve(name)
	if err != nil {
		if errors.Is(err, serverstore.ErrNoServerSpecified) {
			return nil, errors.NewErr("no ayato server configured; set THOMA_SERVER or run 'ayaka server login'")
		}
		return nil, err
	}
	return info, nil
}

// configuredBuildClient creates the configured Ayato or Miko client.
func configuredBuildClient(cfg *conf.ThomaConfig) (string, *client.BuildClient, error) {
	if cfg.Direct() {
		if cfg.Server == "" {
			return "", nil, errors.NewErr("direct mode needs THOMA_SERVER set to the miko URL")
		}
		miko, err := client.NewMiko(cfg.Server, cfg.ApiKey)
		if err != nil {
			return "", nil, err
		}
		return cfg.Server, miko.BuildClient, nil
	}
	endpoint, err := resolveServer(cfg.Server)
	if err != nil {
		return "", nil, err
	}
	ayato, err := client.NewAyato(endpoint.URL, serverstore.NewTokenSource(endpoint))
	if err != nil {
		return "", nil, err
	}
	return endpoint.URL, ayato.BuildClient, nil
}

// detectArch resolves the build arch. makepkg.conf's CARCH is authoritative and
// can disagree with `uname -m` (armv7h userland on an aarch64 kernel, an i686
// build chroot on x86_64), so read it first and fall back to uname only when it
// is unset or unreadable. configPath is the AUR helper's --config, so the arch
// matches the makepkg.conf that computes the package list.
func detectArch(configPath string) string {
	var cfg *makepkgconf.Conf
	var err error
	if configPath != "" {
		cfg, err = makepkgconf.ReadFile(configPath)
	} else {
		cfg, err = makepkgconf.Read()
	}
	if err == nil && cfg.CARCH != "" {
		return cfg.CARCH
	}
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		if a := strings.TrimSpace(string(out)); a != "" {
			return a
		}
	}
	return "x86_64"
}

func remoteBuild(config string) error {
	cfg, err := conf.LoadThomaConfig(nil)
	if err != nil {
		return err
	}
	if cfg.Arch == "" {
		cfg.Arch = detectArch(config)
	}

	base, buildAPI, err := configuredBuildClient(cfg)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.WrapErr(err, "failed to get working directory")
	}
	pkgbuild, files, err := pacmanpkg.ReadInline(cwd, func(name string, size int64) {
		fmt.Fprintf(os.Stderr, "thoma: skipping large file %q (%d bytes); miko fetches sources itself\n", name, size)
	})
	if err != nil {
		return err
	}

	req := &client.BuildRequest{
		Repo:     cfg.Repo,
		Arch:     cfg.Arch,
		Pkgbuild: pkgbuild,
		Files:    files,
		Timeout:  cfg.Timeout,
	}
	// Direct mode leaves the build unsigned on the worker so thoma can pull the
	// artifact straight from the job: miko retains artifacts only for client-sign
	// jobs, and deliberately does not publish them to the shared repo.
	if cfg.Direct() {
		req.SignMode = "client"
	}

	// Cancel the submit and wait on Ctrl-C/SIGTERM so a job stuck in queued does
	// not hang the makepkg invocation forever.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mode := cfg.Mode
	if mode == "" {
		mode = conf.ThomaModeAyato
	}
	fmt.Fprintf(os.Stderr, "thoma: delegating build to %s (mode %s, repo %s, arch %s)\n", base, mode, cfg.Repo, cfg.Arch)
	jobID, err := buildAPI.SubmitBuild(ctx, req)
	if err != nil {
		return errors.WrapErr(err, "failed to submit build")
	}
	fmt.Fprintf(os.Stderr, "thoma: build job %s\n", jobID)

	job, err := buildAPI.WaitJob(ctx, jobID, os.Stdout)
	if err != nil {
		return errors.WrapErr(err, "failed while waiting for build")
	}
	if job.Status != "success" {
		if job.Err != "" {
			return errors.NewErrf("remote build %s: %s", job.Status, job.Err)
		}
		return errors.NewErrf("remote build %s", job.Status)
	}

	dests, err := packageDests(realMakepkg(cfg.Makepkg), config)
	if err != nil {
		return err
	}
	return placePackages(ctx, cfg, buildAPI, jobID, dests, job.Packages)
}

func newBuildClient(cfg *conf.ThomaConfig, base, credential string) (*client.BuildClient, error) {
	if cfg.Direct() {
		miko, err := client.NewMiko(base, credential)
		if err != nil {
			return nil, err
		}
		return miko.BuildClient, nil
	}
	ayato, err := client.NewAyato(base, client.StaticBearer(credential))
	if err != nil {
		return nil, err
	}
	return ayato.BuildClient, nil
}

// packageDests asks the real makepkg where the output packages belong, so the
// downloaded artifacts land exactly where yay's --packagelist told it to look.
func packageDests(mkpkg, config string) ([]string, error) {
	args := []string{"--packagelist", "--ignorearch"}
	if config != "" {
		args = append(args, "--config", config)
	}
	out, err := exec.Command(mkpkg, args...).Output()
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
func placePackages(ctx context.Context, cfg *conf.ThomaConfig, buildAPI *client.BuildClient, jobID string, dests, built []string) error {
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
		fmt.Fprintf(os.Stderr, "thoma: placed %s -> %s\n", match, dest)
	}
	return nil
}

// downloadBuilt fetches one built package to dest. Ayato mode pulls the
// published, host-signed package from ayato's repo route; direct mode pulls the
// unsigned artifact retained on the miko job.
func downloadBuilt(ctx context.Context, cfg *conf.ThomaConfig, buildAPI *client.BuildClient, jobID, name, dest string) error {
	if !cfg.Direct() {
		return buildAPI.DownloadPackageFile(ctx, cfg.Repo, cfg.Arch, name, dest)
	}
	f, err := os.Create(dest)
	if err != nil {
		return errors.WrapErr(err, "failed to create "+dest)
	}
	if err := buildAPI.DownloadArtifact(ctx, jobID, name, f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// pkgName extracts pkgname from a conventional package artifact. Invalid
// filenames are returned unchanged so the caller reports a useful non-match.
func pkgName(filename string) string {
	base := filepath.Base(filename)
	file, err := pkgfile.Parse(base)
	if err != nil {
		return base
	}
	coords, err := file.Coordinates()
	if err != nil {
		return base
	}
	return coords.Name
}
