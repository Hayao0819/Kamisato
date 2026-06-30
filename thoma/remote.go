package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/utils"
)

// maxInlineSource caps the size of a build-dir file shipped inline to miko.
// Larger files are assumed to be sources makepkg downloaded locally, which miko
// re-fetches from the PKGBUILD itself, so shipping them would only bloat the
// request.
const maxInlineSource = 1 << 20 // 1 MiB

type config struct {
	serverURL string
	token     string
	repo      string
	arch      string
	timeout   int
}

func loadConfig() (*config, error) {
	repo := os.Getenv("THOMA_REPO")
	if repo == "" {
		return nil, utils.NewErr("THOMA_REPO is not set; thoma needs the ayato repo to build into")
	}
	url, token, err := resolveServer(os.Getenv("THOMA_SERVER"))
	if err != nil {
		return nil, err
	}
	arch := os.Getenv("THOMA_ARCH")
	if arch == "" {
		arch = detectArch()
	}
	timeout, _ := strconv.Atoi(os.Getenv("THOMA_TIMEOUT"))
	return &config{
		serverURL: url,
		token:     token,
		repo:      repo,
		arch:      arch,
		timeout:   timeout,
	}, nil
}

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
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return utils.WrapErr(err, "failed to get working directory")
	}
	pkgbuild, files, err := readSource(cwd)
	if err != nil {
		return err
	}

	req := &ayatoclient.BuildRequest{
		Repo:     cfg.repo,
		Arch:     cfg.arch,
		Pkgbuild: pkgbuild,
		Files:    files,
		Timeout:  cfg.timeout,
	}

	fmt.Fprintf(os.Stderr, "thoma: delegating build to %s (repo %s, arch %s)\n", cfg.serverURL, cfg.repo, cfg.arch)
	jobID, err := ayatoclient.SubmitBuild(cfg.serverURL, cfg.token, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}
	fmt.Fprintf(os.Stderr, "thoma: build job %s\n", jobID)

	// Cancel the wait on Ctrl-C/SIGTERM so a job stuck in queued does not hang
	// the makepkg invocation forever.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	job, err := ayatoclient.WaitJob(ctx, cfg.serverURL, jobID, os.Stdout)
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
	return placePackages(cfg, dests, job.Packages)
}

// readSource reads the PKGBUILD and small sidecar files from the build dir to
// ship to miko. Directories, build outputs, the regenerated .SRCINFO, and large
// files (assumed downloaded sources) are skipped.
func readSource(dir string) (pkgbuild string, files map[string]string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, utils.WrapErr(err, "failed to read build directory")
	}

	files = map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == ".SRCINFO" || strings.HasSuffix(name, ".log") || strings.Contains(name, ".pkg.tar") {
			continue
		}
		if info, ierr := e.Info(); ierr == nil && name != "PKGBUILD" && info.Size() > maxInlineSource {
			fmt.Fprintf(os.Stderr, "thoma: skipping large file %q (%d bytes); miko fetches sources itself\n", name, info.Size())
			continue
		}
		b, rerr := os.ReadFile(filepath.Join(dir, name))
		if rerr != nil {
			return "", nil, utils.WrapErr(rerr, "failed to read "+name)
		}
		if name == "PKGBUILD" {
			pkgbuild = string(b)
			continue
		}
		files[name] = string(b)
	}

	if pkgbuild == "" {
		return "", nil, utils.NewErr("no PKGBUILD found in " + dir)
	}
	return pkgbuild, files, nil
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
func placePackages(cfg *config, dests, built []string) error {
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
		if err := ayatoclient.DownloadPackage(cfg.serverURL, cfg.repo, cfg.arch, match, dest); err != nil {
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
