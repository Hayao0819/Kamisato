package builder

import (
	"os"
	"os/exec"
)

var Extra_x86_64 = Builder{
	Name: "extra-x86_64",
	Build: func(dir string, target *Target) error {
		build := cmd(
			dir,
			"extra-x86_64-build",
			// archbuild arguments
			"--",
			// makechrootpkg arguments
			"-c",
			"--",
			// makepkg arguments
			"--syncdeps", "--noconfirm", "--log", "--holdver","OPTIONS=-debug",
		)

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
