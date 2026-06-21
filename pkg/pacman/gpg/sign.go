package gpg

import (
	"os"
	"os/exec"
	"path"
)

func SignFile(key string, gpgDir string, file string) error {
	homeDir := gpgDir
	if gpgDir == "" {
		homeDir = os.Getenv("GNUPGHOME")
		if homeDir == "" {
			homeDir = path.Join(os.Getenv("HOME"), ".gnupg")
		}
	}

	cmd := exec.Command("gpg", "--detach-sign", "--use-agent", "-u", key, "--no-armor", "--homedir", homeDir, file)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
