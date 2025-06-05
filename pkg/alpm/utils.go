package alpm

import (
	"bytes"
	"os/exec"
	"strconv"
)

// import goalpm "github.com/Jguer/go-alpm/v2"

// goalpmがAlpineLinuxでコンパイルできない

func VerCmp(v1, v2 string) (int, error) {
	// return goalpm.VerCmp(v1, v2)

	vc := exec.Command("vercmp", v1, v2)
	stdout := bytes.NewBuffer(nil)
	vc.Stdout = stdout
	if err := vc.Run(); err != nil {
		return 0, err
	}
	out, err := strconv.Atoi(stdout.String())
	if err != nil {
		return 0, err
	}

	/*
		vercmp (pacman) v7.0.0

		Compare package version numbers using pacman's version comparison logic.

		Usage: vercmp <ver1> <ver2>

		Output values:
		< 0 : if ver1 < ver2
			0 : if ver1 == ver2
		> 0 : if ver1 > ver2
	*/

	return out, nil

}
