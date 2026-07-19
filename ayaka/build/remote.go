package build

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/Hayao0819/Kamisato/ayaka/cmd/shared"
	"github.com/Hayao0819/Kamisato/internal/client"
	"github.com/Hayao0819/Kamisato/internal/errors"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/pkgfile"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
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
func RunRemoteBuild(ctx context.Context, o RemoteBuildOpts) error {
	srv, err := shared.ResolveAyatoServer(o.Server)
	if err != nil {
		return err
	}
	req, err := buildRequest(ctx, o)
	if err != nil {
		return err
	}

	slog.Info("submitting remote build", "server", srv.URL, "repo", o.Repo)
	api, err := shared.AyatoClient(srv)
	if err != nil {
		return err
	}
	jobID, err := api.SubmitBuild(ctx, req)
	if err != nil {
		return errors.WrapErr(err, "failed to submit build")
	}

	fmt.Println(jobID)
	return nil
}

// buildRequest assembles the build request from opts: a git source, else the
// local PKGBUILD of the named source package.
func buildRequest(ctx context.Context, o RemoteBuildOpts) (*client.BuildRequest, error) {
	arch := o.Arch
	if arch == "" {
		arch = "x86_64"
	}
	req := &client.BuildRequest{
		Repo:        o.Repo,
		Arch:        arch,
		InstallPkgs: o.Pkgs,
		Timeout:     o.Timeout,
	}
	if o.GitURL != "" {
		req.Git = &client.GitSource{URL: o.GitURL, Ref: o.GitRef, Subdir: o.GitSubdir}
		return req, nil
	}
	pkgbuild, files, err := readLocalSource(ctx, o.Repo, o.Pkgs)
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
func RunRemoteBuildLocalSign(ctx context.Context, o RemoteBuildOpts, keyPath, passphrase string) error {
	srv, err := shared.ResolveAyatoServer(o.Server)
	if err != nil {
		return err
	}
	signer, err := sign.NewLocalSigner(keyPath, passphrase)
	if err != nil {
		return errors.WrapErr(err, "failed to load local signing key")
	}
	req, err := buildRequest(ctx, o)
	if err != nil {
		return err
	}
	req.SignMode = "client"

	api, err := shared.AyatoClient(srv)
	if err != nil {
		return err
	}
	jobID, err := api.SubmitBuild(ctx, req)
	if err != nil {
		return errors.WrapErr(err, "failed to submit build")
	}
	slog.Info("submitted client-signed build", "job", jobID, "server", srv.URL)

	// Bound the wait so a stuck queued/running job cannot hang the CLI forever.
	waitCtx, cancel := context.WithTimeout(ctx, clientBuildTimeout)
	defer cancel()
	job, err := api.WaitJob(waitCtx, jobID, nil)
	if err != nil {
		return err
	}
	switch job.Status {
	case "failed":
		return errors.NewErrf("build failed: %s", job.Err)
	case "cancelled":
		return errors.NewErr("build cancelled")
	}

	names, err := api.ListArtifacts(ctx, jobID)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return errors.NewErr("build produced no artifacts")
	}

	tmp, err := os.MkdirTemp("", "ayaka-dl-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	for _, name := range names {
		artifact, parseErr := pkgfile.Parse(name)
		if parseErr != nil {
			return errors.WrapErr(parseErr, "build returned invalid package artifact "+name)
		}
		// Sign locally, so skip any signature the server may have produced.
		if artifact.IsSignature() {
			continue
		}
		pkgPath := filepath.Join(tmp, name)
		f, err := os.Create(pkgPath)
		if err != nil {
			return err
		}
		if derr := api.DownloadArtifact(ctx, jobID, name, f); derr != nil {
			_ = f.Close()
			return derr
		}
		if cerr := f.Close(); cerr != nil {
			return cerr
		}
		sigPath, serr := signer.Sign(context.Background(), pkgPath)
		if serr != nil {
			return errors.WrapErr(serr, "failed to sign "+name)
		}
		if uerr := api.UploadPackageFiles(ctx, o.Repo, pkgPath, sigPath); uerr != nil {
			return errors.WrapErr(uerr, "failed to upload "+name)
		}
		slog.Info("signed and uploaded", "pkg", name)
	}
	return nil
}

// clientBuildTimeout bounds how long the local-sign flow waits for a remote
// build, so a stuck queued/running job does not hang the CLI forever.
const clientBuildTimeout = 2 * time.Hour

// readLocalSource reads the PKGBUILD and files of a source package in the repo.
// With one named package that one is used, else the repo must hold exactly one.
func readLocalSource(ctx context.Context, repoName string, pkgs []string) (string, map[string]string, error) {
	srcrepo := shared.AppFromContext(ctx).GetSrcRepo(repoName)
	if srcrepo == nil {
		return "", nil, errors.WrapErr(shared.ErrSourceRepoNotFound, repoName)
	}

	sp, err := selectSourcePkg(srcrepo, pkgs)
	if err != nil {
		return "", nil, err
	}

	return pkg.ReadInline(sp.Dir(), func(name string, size int64) {
		slog.Warn("skipping large source file", "name", name, "size", size)
	})
}

// selectSourcePkg picks the source package to submit: with none named the repo
// must hold exactly one, else it matches one by pkgbase or package name.
func selectSourcePkg(srcrepo *repo.SourceRepo, pkgs []string) (*pkg.SourcePackage, error) {
	if len(pkgs) == 0 {
		switch len(srcrepo.Pkgs) {
		case 0:
			return nil, errors.NewErr("no source packages found in repository")
		case 1:
			return srcrepo.Pkgs[0], nil
		default:
			return nil, errors.NewErr("repository holds multiple packages; specify one to build remotely")
		}
	}
	if len(pkgs) > 1 {
		return nil, errors.NewErr("remote build accepts only one package at a time")
	}

	name := pkgs[0]
	for _, p := range srcrepo.Pkgs {
		if p.Base() == name || slices.Contains(p.Names(), name) {
			return p, nil
		}
	}
	return nil, errors.NewErr("package not found: " + name)
}
