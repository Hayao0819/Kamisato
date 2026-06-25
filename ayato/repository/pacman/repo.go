// Package pacman is the blinky-backed implementation of the repository's
// repo-DB tool port: it shells out to the repo-add/repo-remove CLI. It is the
// single boundary where ayato execs the pacman tooling, so a Go-native
// implementation can replace it behind that port without touching the
// repository layer.
package pacman

import "github.com/BrenekH/blinky/pacman"

// CLI runs repo-add/repo-remove by shelling out (via blinky). It is the default
// repoDBTool implementation; it requires the repo-add/repo-remove binaries on
// PATH (and GNUPGHOME when signing).
type CLI struct{}

func (CLI) RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	return pacman.RepoAdd(dbPath, pkgFilePath, useSignedDB, gnupgDir)
}

func (CLI) RepoRemove(dbPath, pkg string, useSignedDB bool, gnupgDir *string) error {
	return pacman.RepoRemove(dbPath, pkg, useSignedDB, gnupgDir)
}
