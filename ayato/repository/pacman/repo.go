package pacman

import "github.com/BrenekH/blinky/pacman"

func RepoAdd(dbPath string, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	return pacman.RepoAdd(dbPath, pkgFilePath, useSignedDB, gnupgDir)
}

func RepoRemove(dbPath string, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	return pacman.RepoRemove(dbPath, pkgFilePath, useSignedDB, gnupgDir)
}
