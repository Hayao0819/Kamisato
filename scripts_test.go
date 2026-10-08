package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func scriptCommand(t *testing.T, shell, script string, args ...string) *exec.Cmd {
	t.Helper()
	if _, err := exec.LookPath(shell); err != nil {
		t.Skipf("%s is not available: %v", shell, err)
	}
	return exec.Command(shell, append([]string{script}, args...)...)
}

func scriptTools(t *testing.T, tools map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestInstallHelpDoesNotInvokeTools(t *testing.T) {
	tools := scriptTools(t, map[string]string{
		"go":   "echo unexpected-go-invocation; exit 1\n",
		"pnpm": "echo unexpected-pnpm-invocation; exit 1\n",
	})
	cmd := scriptCommand(t, "sh", "install.sh", "--help")
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(output), "Usage:") || strings.Contains(string(output), "unexpected-") {
		t.Fatalf("--help invoked an installation step: output=%s error=%v", output, err)
	}
}

func TestInstallBuildsWebBeforeCombinedBinary(t *testing.T) {
	tools := scriptTools(t, map[string]string{
		"go":    "printf 'go %s\\n' \"$*\" >> \"$RUN_LOG\"\n",
		"pnpm":  "printf 'pnpm %s\\n' \"$*\" >> \"$RUN_LOG\"\n",
		"strip": "exit 0\n",
	})
	log := filepath.Join(t.TempDir(), "tools.log")
	cmd := scriptCommand(t, "sh", "install.sh", "--disable-all", "--lumine-web", "--kamisato", "--no-upx", "--bin", t.TempDir())
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "RUN_LOG="+log)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install fixture: %v: %s", err, output)
	}
	output, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	web, binary := strings.Index(string(output), "pnpm run build"), strings.Index(string(output), "go build ")
	if web < 0 || binary <= web || !strings.Contains(string(output), "pnpm install --frozen-lockfile") {
		t.Fatalf("unexpected build order or dependency contract: %s", output)
	}
}

func TestDaemonLaunchersForwardFlags(t *testing.T) {
	tools := scriptTools(t, map[string]string{"go": "printf '%s\\n' \"$@\"\n"})
	for _, script := range []string{"run_ayato.sh", "run_miko.sh"} {
		cmd := scriptCommand(t, "sh", script, "--debug", "-c", "settings.json")
		cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()
		if err != nil || string(output) != "run\n.\n--debug\n-c\nsettings.json\n" {
			t.Fatalf("%s changed program arguments: output=%s error=%v", script, output, err)
		}
	}
}

func TestPruneActionPropagatesPlannerFailure(t *testing.T) {
	action, err := os.ReadFile("actions/prune/action.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, block, ok := strings.Cut(string(action), "      run: |\n")
	if !ok {
		t.Fatal("prune action has no run block")
	}
	var script strings.Builder
	for line := range strings.SplitSeq(block, "\n") {
		script.WriteString(strings.TrimPrefix(line, "        ") + "\n")
	}
	path := filepath.Join(t.TempDir(), "prune.sh")
	if err := os.WriteFile(path, []byte(script.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	tools := scriptTools(t, map[string]string{"ayaka": "exit 17\n"})
	cmd := scriptCommand(t, "bash", path)
	cmd.Env = append(os.Environ(), "PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"),
		"IN_SERVER=https://example.invalid", "IN_REPO=repo", "IN_ARCH=x86_64", "IN_DRY_RUN=true")
	output, err := cmd.CombinedOutput()
	if cmd.ProcessState == nil || cmd.ProcessState.ExitCode() != 17 || strings.Contains(string(output), "nothing to prune") {
		t.Fatalf("failed planner was ignored: output=%s error=%v", output, err)
	}
}
