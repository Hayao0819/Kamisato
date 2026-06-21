package alpm

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
)

func VerCmp(v1, v2 string) (int, error) {
	vc := exec.Command("vercmp", v1, v2)
	stdout := bytes.NewBuffer(nil)
	vc.Stdout = stdout
	if err := vc.Run(); err != nil {
		return 0, err
	}
	out, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		return 0, err
	}
	return out, nil
}
