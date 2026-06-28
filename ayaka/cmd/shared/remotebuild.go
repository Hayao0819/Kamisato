package shared

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/Hayao0819/Kamisato/internal/ayatoclient"
	"github.com/Hayao0819/Kamisato/internal/utils"
	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// RemoteBuildOpts collects the inputs for a server-side build submission.
type RemoteBuildOpts struct {
	Repo      string
	Server    string
	GPGKey    string
	GitURL    string
	GitRef    string
	GitSubdir string
	Arch      string
	Timeout   int
	Pkgs      []string
}

// RunRemoteBuild submits a build to ayato and prints the resulting job id. The
// source is either a git repository (--git) or, by default, the local PKGBUILD
// of the named source package in the repo.
func RunRemoteBuild(o RemoteBuildOpts) error {
	srv, err := ResolveAyatoServer(o.Server)
	if err != nil {
		return err
	}

	arch := o.Arch
	if arch == "" {
		arch = "x86_64"
	}

	req := &ayatoclient.BuildRequest{
		Repo:        o.Repo,
		Arch:        arch,
		InstallPkgs: o.Pkgs,
		GPGKey:      o.GPGKey,
		Timeout:     o.Timeout,
	}

	if o.GitURL != "" {
		req.Git = &ayatoclient.GitSource{
			URL:    o.GitURL,
			Ref:    o.GitRef,
			Subdir: o.GitSubdir,
		}
	} else {
		pkgbuild, files, err := readLocalSource(o.Repo, o.Pkgs)
		if err != nil {
			return err
		}
		req.Pkgbuild = pkgbuild
		req.Files = files
		// install_pkgs targets local package files on the builder, not source
		// package names, so don't pass the selected build packages through.
		req.InstallPkgs = nil
	}

	slog.Info("submitting remote build", "server", srv.URL, "repo", o.Repo)
	jobID, err := ayatoclient.SubmitBuild(srv.URL, srv.Password, req)
	if err != nil {
		return utils.WrapErr(err, "failed to submit build")
	}

	fmt.Println(jobID)
	return nil
}

// readLocalSource reads the PKGBUILD and accompanying files of a source package
// in the named repo. When pkgs names a single package that one is used;
// otherwise the repo must hold exactly one source package.
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

// selectSourcePkg picks the source package to submit. With no package named it
// requires the repo to hold exactly one; otherwise it matches a single named
// package by pkgbase or package name.
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
