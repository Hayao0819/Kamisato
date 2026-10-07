package docker

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallNamesBeforeSourceRetrieval(t *testing.T) {
	command, err := installNamesCommand([]string{"alterlinux-nostalgia/curl"})
	if err != nil {
		t.Fatal(err)
	}
	script := strings.ReplaceAll(buildScript, "__PACMAN_CONFIG__", "")
	script = strings.ReplaceAll(script, "__EXTRA_REPOS__", "echo repos")
	script = strings.ReplaceAll(script, "__INSTALL__", command)
	start := strings.Index(script, command)
	if start < strings.Index(script, "pacman -Sy") || start > strings.Index(script, "makepkg --config") {
		t.Fatal("wrong installation order")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	stub := "#!/bin/sh\nprintf '%s\\n' \"$@\" >\"$TEST_LOG\"\nexit \"${TEST_STATUS:-0}\"\n"
	if err := os.WriteFile(filepath.Join(dir, "pacman"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"0", "1"} {
		cmd := exec.Command("bash", "-c", "set -euo pipefail\n"+command+"touch \"$TEST_MARKER\"")
		marker := filepath.Join(dir, "marker"+status)
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"), "TEST_LOG="+log, "TEST_STATUS="+status, "TEST_MARKER="+marker)
		err := cmd.Run()
		if (err == nil) != (status == "0") {
			t.Fatalf("status %s: %v", status, err)
		}
		_, markerErr := os.Stat(marker)
		if (markerErr == nil) != (status == "0") {
			t.Fatal("continued after installation failure")
		}
	}
	args, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(args) != "-S\n--needed\n--noconfirm\n--\nalterlinux-nostalgia/curl\n" {
		t.Fatalf("arguments: %s", args)
	}
}

func TestInstallNamesValidation(t *testing.T) {
	for _, name := range []string{"", "--help", "curl git", "curl\n"} {
		if _, err := installNamesCommand([]string{name}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	if got, err := installNamesCommand(nil); got != "" || err != nil {
		t.Fatalf("empty names: %q %v", got, err)
	}
}
