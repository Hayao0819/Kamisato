package repo

import (
	"fmt"
	"os"
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
	// --include-sigs embeds each package's adjacent "<pkg>.sig" as the desc
	// %PGPSIG%, matching NativeTool; it is a no-op when no signature is present.
	args := []string{"-q", "-R", "--nocolor", "--include-sigs"}
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

// RebuildDerived rebuilds .files and optional signatures.
func (t CLITool) RebuildDerived(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error {
	paths, err := toolPathsFor(dbPath)
	if err != nil {
		return err
	}
	b, err := loadToolBuilder(paths)
	if err != nil {
		return err
	}
	if err := attachPackageFiles(b, pkgFilePaths); err != nil {
		return err
	}
	missing, err := b.missingFileObjects()
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return &MissingPackageFilesError{Filenames: missing}
	}
	if err := writeDerivedBuilder(b, paths); err != nil {
		return err
	}
	if !useSignedDB {
		return nil
	}
	for _, artifact := range []string{paths.db, paths.files} {
		args := []string{"--batch", "--yes", "--no-armor", "--detach-sign", "--output", artifact + ".sig"}
		if key := os.Getenv("GPGKEY"); key != "" {
			args = append(args, "--local-user", key)
		}
		args = append(args, artifact)
		if err := runRepoDBTool("gpg", args, gnupgDir); err != nil {
			return err
		}
	}
	return nil
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
