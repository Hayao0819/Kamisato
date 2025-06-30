// x86_64用ビルダー
package builder

import (
	"log/slog"
	"os"
	"os/exec"
)

var Extra_x86_64 = Builder{
	Name: "extra-x86_64",
	Build: func(dir string, target *Target) error {
		archBuildArgs := []string{"extra-x86_64-build"}
		makePkgArgs := []string{"--syncdeps", "--noconfirm", "--log", "--holdver", "OPTIONS=-debug"}
		makeChrootPkgArgs := []string{"-c"}
		for _, pkg := range target.InstallPkgs {
			makeChrootPkgArgs = append(makeChrootPkgArgs, "-I", pkg)
		}
		slog.Debug("install packages", "pkgs", target.InstallPkgs)

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
	},
}

func cmd(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd
}
