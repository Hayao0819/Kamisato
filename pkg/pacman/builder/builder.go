package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
)

type Target struct {
	Arch        string
	ArchBuild   string
	SignKey     string
	InstallPkgs []string
	// Output, when non-nil, receives the build command's stdout/stderr instead
	// of os.Stdout. Used to capture build logs.
	Output io.Writer
}

func (t *Target) Build(dir string) error {
	return t.BuildContext(context.Background(), dir)
}

// BuildContext runs the same clean-chroot build as Build, but ctx controls
// cancellation and timeout. Build calls this with context.Background().
func (t *Target) BuildContext(ctx context.Context, dir string) error {

	if t.ArchBuild == "" {
		return errors.New("ArchBuild is not specified")
	}

	archBuildArgs := []string{t.ArchBuild}
	makePkgArgs := []string{"--syncdeps", "--noconfirm", "--log", "--holdver"}
	makeChrootPkgArgs := []string{"-c"}
	for _, pkg := range t.InstallPkgs {
		makeChrootPkgArgs = append(makeChrootPkgArgs, "-I", pkg)
	}
	slog.Debug("install packages", "pkgs", t.InstallPkgs)

	args := append(archBuildArgs, "--")
	args = append(args, makeChrootPkgArgs...)
	args = append(args, "--")
	args = append(args, makePkgArgs...)
	build := cmdContext(ctx, dir, t.Output, args...)

	slog.Debug("build command", "cmd", build.String())
	if err := build.Run(); err != nil {
		return err
	}

	return nil

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
