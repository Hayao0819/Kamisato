package builder

import (
	"errors"
	"log/slog"
)

type Target struct {
	Arch        string
	ArchBuild   string
	SignKey     string
	InstallPkgs []string
}

func (t *Target) Build(dir string) error {

	if t.ArchBuild == "" {
		return errors.New("ArchBuild is not specified")
	}

	archBuildArgs := []string{t.ArchBuild}
	makePkgArgs := []string{"--syncdeps", "--noconfirm", "--log", "--holdver", "OPTIONS=-debug"}
	makeChrootPkgArgs := []string{"-c"}
	for _, pkg := range t.InstallPkgs {
		makeChrootPkgArgs = append(makeChrootPkgArgs, "-I", pkg)
	}
	slog.Debug("install packages", "pkgs", t.InstallPkgs)

	args := append(archBuildArgs, "--")
	args = append(args, makeChrootPkgArgs...)
	args = append(args, "--")
	args = append(args, makePkgArgs...)
	build := cmd(dir, args...)

	slog.Debug("build command", "cmd", build.String())
	if err := build.Run(); err != nil {
		return err
	}

	return nil

}
