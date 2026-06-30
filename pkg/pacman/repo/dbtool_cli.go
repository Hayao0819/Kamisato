package repo

import (
	"fmt"
	"os/exec"
)

// CLITool mutates the database by shelling out to repo-add/repo-remove. It needs
// those binaries on PATH (and GNUPGHOME set when signing). NativeTool is the
// dependency-free alternative used by default.
type CLITool struct{}

func (t CLITool) RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	var paths []string
	if pkgFilePath != "" {
		paths = []string{pkgFilePath}
	}
	return t.RepoAddBatch(dbPath, paths, useSignedDB, gnupgDir)
}

func (CLITool) RepoAddBatch(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error {
	args := []string{"-q", "-R", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath)
	for _, p := range pkgFilePaths {
		if p != "" {
			args = append(args, p)
		}
	}
	return runRepoDBTool("repo-add", args, gnupgDir)
}

func (CLITool) RepoRemove(dbPath, pkgName string, useSignedDB bool, gnupgDir *string) error {
	args := []string{"-q", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath, pkgName)
	return runRepoDBTool("repo-remove", args, gnupgDir)
}

func runRepoDBTool(bin string, args []string, gnupgDir *string) error {
	cmd := exec.Command(bin, args...) //nolint:gosec // bin is a fixed literal, args are paths/flags
	if gnupgDir != nil {
		cmd.Env = append(cmd.Environ(), "GNUPGHOME="+*gnupgDir)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %w (output: %s)", bin, err, out)
	}
	return nil
}
