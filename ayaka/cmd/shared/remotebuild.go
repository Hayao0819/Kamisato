package shared

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/blinkyutils"
	"github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/pacman/sign"
)

type RemoteBuildOpts struct {
	Repo      string
	Server    string
	GitURL    string
	GitRef    string
	GitSubdir string
	Arch      string
	Timeout   int
	Pkgs      []string
}

// RunRemoteBuild submits a build to ayato and prints the job id. The source is
// --git, else the local PKGBUILD of the named source package.
func RunRemoteBuild(o RemoteBuildOpts) error {
	srv, err := ResolveAyatoServer(o.Server)
	if err != nil {
		return err
	}
	req, err := buildRequest(o)
	if err != nil {
		return err
	}

	slog.Info("submitting remote build", "server", srv.URL, "repo", o.Repo)
	jobID, err := ayatoclient.SubmitBuild(srv.URL, srv.Password, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}

	fmt.Println(jobID)
	return nil
}

// buildRequest assembles the build request from opts: a git source, else the
// local PKGBUILD of the named source package.
func buildRequest(o RemoteBuildOpts) (*ayatoclient.BuildRequest, error) {
	arch := o.Arch
	if arch == "" {
		arch = "x86_64"
	}
	req := &ayatoclient.BuildRequest{
		Repo:        o.Repo,
		Arch:        arch,
		InstallPkgs: o.Pkgs,
		Timeout:     o.Timeout,
	}
	if o.GitURL != "" {
		req.Git = &ayatoclient.GitSource{URL: o.GitURL, Ref: o.GitRef, Subdir: o.GitSubdir}
		return req, nil
	}
	pkgbuild, files, err := readLocalSource(o.Repo, o.Pkgs)
	if err != nil {
		return nil, err
	}
	req.Pkgbuild = pkgbuild
	req.Files = files
	// install_pkgs targets local package files on the builder, not source
	// package names, so don't pass the selected build packages through.
	req.InstallPkgs = nil
	return req, nil
}

// RunRemoteBuildLocalSign builds on miko without server-side signing, downloads
// the artifacts, signs them locally with keyPath, and uploads them to ayato.
func RunRemoteBuildLocalSign(o RemoteBuildOpts, keyPath, passphrase string) error {
	srv, err := ResolveAyatoServer(o.Server)
	if err != nil {
		return err
	}
	signer, err := sign.NewLocalSigner(keyPath, passphrase)
	if err != nil {
		return utils.WrapErr(err, "failed to load local signing key")
	}
	req, err := buildRequest(o)
	if err != nil {
		return err
	}
	req.SignMode = "client"

	jobID, err := ayatoclient.SubmitBuild(srv.URL, srv.Password, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}
	slog.Info("submitted client-signed build", "job", jobID, "server", srv.URL)

	if err := waitForJob(srv.URL, jobID); err != nil {
		return err
	}

	names, err := ayatoclient.ListArtifacts(srv.URL, jobID)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return utils.NewErr("build produced no artifacts")
	}

	tmp, err := os.MkdirTemp("", "ayaka-dl-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	client, err := srv.Client()
	if err != nil {
		return utils.WrapErr(err, "failed to create upload client")
	}

	for _, name := range names {
		// Sign locally, so skip any signature the server may have produced.
		if strings.HasSuffix(name, ".sig") {
			continue
		}
		pkgPath := filepath.Join(tmp, name)
		f, err := os.Create(pkgPath)
		if err != nil {
			return err
		}
		if derr := ayatoclient.DownloadArtifact(srv.URL, jobID, name, f); derr != nil {
			_ = f.Close()
			return derr
		}
		if cerr := f.Close(); cerr != nil {
			return cerr
		}
		sigPath, serr := signer.Sign(context.Background(), pkgPath)
		if serr != nil {
			return utils.WrapErr(serr, "failed to sign "+name)
		}
		if uerr := blinkyutils.Upload(client, o.Repo, pkgPath, sigPath); uerr != nil {
			return utils.WrapErr(uerr, "failed to upload "+name)
		}
		slog.Info("signed and uploaded", "pkg", name)
	}
	return nil
}

// clientBuildTimeout bounds how long the local-sign flow waits for a remote
// build, so a stuck queued/running job does not hang the CLI forever.
const clientBuildTimeout = 2 * time.Hour

// waitForJob polls until the job reaches a terminal state or the timeout elapses.
func waitForJob(base, id string) error {
	deadline := time.Now().Add(clientBuildTimeout)
	for {
		job, err := ayatoclient.JobStatus(base, id)
		if err != nil {
			return err
		}
		switch job.Status {
		case "success":
			return nil
		case "failed":
			return utils.NewErrf("build failed: %s", job.Err)
		case "cancelled":
			return utils.NewErr("build cancelled")
		}
		if time.Now().After(deadline) {
			return utils.NewErrf("timed out waiting for job %s (last status %q)", id, job.Status)
		}
		time.Sleep(2 * time.Second)
	}
}

// readLocalSource reads the PKGBUILD and files of a source package in the repo.
// With one named package that one is used, else the repo must hold exactly one.
func readLocalSource(repo string, pkgs []string) (string, map[string]string, error) {
	srcrepo := GetSrcRepo(repo)
	if srcrepo == nil {
		return "", nil, utils.WrapErr(ErrSourceRepoNotFound, repo)
	}

	srcpkg, err := selectSourcePkg(srcrepo, pkgs)
	if err != nil {
		return "", nil, err
	}

	dir := srcpkg.Dir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, utils.WrapErr(err, "failed to read source directory")
	}

	var pkgbuild string
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		content, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", nil, utils.WrapErr(err, "failed to read "+name)
		}
		if name == "PKGBUILD" {
			pkgbuild = string(content)
			continue
		}
		files[name] = string(content)
	}

	if pkgbuild == "" {
		return "", nil, utils.NewErr("PKGBUILD not found in " + dir)
	}
	return pkgbuild, files, nil
}

// selectSourcePkg picks the source package to submit: with none named the repo
// must hold exactly one, else it matches one by pkgbase or package name.
func selectSourcePkg(srcrepo *pacmanrepo.SourceRepo, pkgs []string) (*pkg.SourcePackage, error) {
	if len(pkgs) == 0 {
		switch len(srcrepo.Pkgs) {
		case 0:
			return nil, utils.NewErr("no source packages found in repository")
		case 1:
			return srcrepo.Pkgs[0], nil
		default:
			return nil, utils.NewErr("repository holds multiple packages; specify one to build remotely")
		}
	}
	if len(pkgs) > 1 {
		return nil, utils.NewErr("remote build accepts only one package at a time")
	}

	name := pkgs[0]
	for _, p := range srcrepo.Pkgs {
		if p.Base() == name || slices.Contains(p.Names(), name) {
			return p, nil
		}
	}
	return nil, utils.NewErr("package not found: " + name)
}
