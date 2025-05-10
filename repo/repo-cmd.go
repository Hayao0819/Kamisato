package repo

import (
	"fmt"
	"os/exec"
)

// RepoRemove uses `repo-remove` to remove the specified package from the database located at
// dbPath.
//
// If the database should be signed, set useSignedDB to true and set gnupgDir to the directory
// to store the keyring in.
func RepoRemove(dbPath, packageName string, useSignedDB bool, gnupgDir *string) error {
	// Build and run repo-add command, including the --sign arg if requested
	args := []string{"-q", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath, packageName)

	cmd := exec.Command("repo-remove", args...)
	if gnupgDir != nil {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GNUPGHOME=%s", *gnupgDir))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("RepoRemove: error running %s (output: %s): %w", cmd.String(), string(out), err)
	}

	return nil
}

// RepoAdd uses `repo-add` to add the *.pkg.tar.zst located at pkgFilePath to the Pacman repository
// database located at dbPath.
//
// If pkgFilePath is an empty string (""), the argument will not be passed to `repo-add`. This is
// useful for creating an empty database
//
// If the database should be signed, set useSignedDB to true and set gnupgDir to the directory
// to store the keyring in.
func RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	// Build and run repo-add command, including the --sign arg if requested
	args := []string{"-q", "-R", "--nocolor"}
	if useSignedDB {
		args = append(args, "--sign")
	}
	args = append(args, dbPath)

	if pkgFilePath != "" {
		args = append(args, pkgFilePath)
	}

	cmd := exec.Command("repo-add", args...)
	if gnupgDir != nil {
		cmd.Env = append(cmd.Env, fmt.Sprintf("GNUPGHOME=%s", *gnupgDir))
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("RepoAdd: error running %s (output: %s): %w", cmd.String(), string(out), err)
	}

	return nil
}

func RepoInit(dbPath string, useSignedDB bool, gnupgDir *string) error {
	return RepoAdd(dbPath, "", useSignedDB, gnupgDir)
}
