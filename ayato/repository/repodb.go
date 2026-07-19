package repository

import (
	stderrors "errors"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// repoDBTool runs pacman repository mutations against a local database path.
// Keeping the writer behind this port lets the repository use either the native
// Go implementation or repo-add-compatible tooling without leaking that choice
// into the service layer.
type repoDBTool interface {
	RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error
	RepoAddBatch(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
	RepoRemove(dbPath, pkg string, useSignedDB bool, gnupgDir *string) error
	RebuildDerived(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
}

func (r *binaryRepository) repoTool() repoDBTool {
	if r.tool != nil {
		return r.tool
	}
	return pacmanrepo.NativeTool{}
}

// A conflict means another instance committed first. Retrying from a freshly
// fetched canonical database preserves both writers' changes.
const (
	maxDBAttempts         = 6
	dbConflictBackoffBase = 10 * time.Millisecond
	dbConflictBackoffCap  = 500 * time.Millisecond
)

// dbArtifactBases are the archive names repository tools read and rewrite.
// Public .db/.files names are aliases resolved at fetch time.
func dbArtifactBases(repoName string) []string {
	return pacmanrepo.Artifacts(repoName).Archives()
}

// derivedArtifacts are published after the canonical database archive.
func derivedArtifacts(repoName string, useSignedDB bool) []string {
	artifacts := pacmanrepo.Artifacts(repoName)
	names := []string{artifacts.FilesArchive()}
	if useSignedDB {
		names = append(names, artifacts.ArchiveSignatures()...)
	}
	return names
}

// RepoAddItem is one package and its optional detached signature in a batch.
// Conditional fields protect a publish from replacing a concurrently changed
// package while still allowing an idempotent retry of the intended version.
type RepoAddItem struct {
	Pkg                    stream.SeekFile
	Sig                    stream.SeekFile
	CheckCurrent           bool
	ExpectedName           string
	ExpectedCurrentVersion string
	ExpectedCurrentFile    string
	IntendedVersion        string
	IntendedFile           string
}

// ErrPackageChanged means the package no longer matches the caller's snapshot.
var ErrPackageChanged = errors.New("repository package changed concurrently")

// CanonicalCommitError reports a failure after the canonical database may have
// been committed. Callers must reconcile rather than blindly roll back package
// objects when this marker is present.
type CanonicalCommitError struct{ Err error }

func (e *CanonicalCommitError) Error() string { return e.Err.Error() }
func (e *CanonicalCommitError) Unwrap() error { return e.Err }

func CanonicalCommitted(err error) bool {
	var committed *CanonicalCommitError
	return stderrors.As(err, &committed)
}
