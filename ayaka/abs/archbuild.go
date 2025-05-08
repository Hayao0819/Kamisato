package abs

import (
	"os"
	"os/exec"
)

var ArchBuilds = Builder{
	Name: "archbuild",
	Build: func(path string, target *Target) error {
		cmd := exec.Command("extra-x86_64-build", "--", "-c")
		cmd.Dir = path
		
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		return cmd.Run()
	},
}

func init() {
	Builders = append(Builders, &ArchBuilds)
}
