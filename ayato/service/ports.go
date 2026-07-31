package service

import (
	"errors"
	"time"

	"github.com/Hayao0819/Kamisato/ayato/blob"
	"github.com/Hayao0819/Kamisato/ayato/domain"
	pacmanrepo "github.com/Hayao0819/Kamisato/internal/pacman/repo"
)

type PackageFileEntry struct {
	Arch, Name, FileName string
}

type RepoAddItem struct {
	Pkg                    blob.SeekFile
	Sig                    blob.SeekFile
	CheckCurrent           bool
	ExpectedName           string
	ExpectedCurrentVersion string
	ExpectedCurrentFile    string
	IntendedVersion        string
	IntendedFile           string
}

type NameStore interface {
	PackageFile(repo, arch, name string) (string, error)
	StorePackageFile(repo, arch, packageName, filePath string) error
	StorePackageFiles(repo string, entries []PackageFileEntry) error
	DeletePackageFileEntry(repo, arch, packageName string) error
}

type BinaryRepository interface {
	blob.Store
	StoreFileImmutable(name, arch string, file blob.SeekFile) (created bool, err error)
	DeleteOrphanIfUnchanged(name, arch string, expected blob.FileInfo, cutoff time.Time) (deleted bool, err error)
	RepoAdd(name, arch string, pkg, sig blob.SeekFile, useSignedDB bool, gnupgDir *string) error
	RepoAddBatch(name, arch string, items []RepoAddItem, useSignedDB bool, gnupgDir *string) error
	RepoRemove(name, arch, pkg string, useSignedDB bool, gnupgDir *string) error
	RepoRemoveIfMatch(name, arch, pkg, expectedVersion, expectedFile string, useSignedDB bool, gnupgDir *string) error
	ReconcileDB(name, arch string, useSignedDB bool, gnupgDir *string) error
	InitArch(name, arch string, useSignedDB bool, gnupgDir *string) error
	BackfillSignatures(name, arch string) error
	RebuildMerged(name, arch string, useSignedDB bool) error
	ApplyUpstreamSnapshot(name, arch string, dbGz, filesGz []byte, etag, lastModified string, useSignedDB bool) (pacmanrepo.DBDiff, error)
	UpstreamValidators(name, arch string) (etag, lastModified string, err error)
	FetchDB(repoName, archName string) (blob.File, error)
	FetchFileWithMeta(repo, arch, file string) (blob.File, blob.FileMeta, error)
	PkgNames(repoName, archName string) ([]string, error)
	RemoteRepo(name, arch string) (*pacmanrepo.RemoteRepo, error)
	PkgFiles(repoName, archName, pkgName string) ([]string, error)
	VerifyPkgRepo(name string) error
	StagedUploads() (blob.StagedUploader, bool)
}

type AuthRepository interface {
	AddAdmin(id int64, login string) error
	RemoveAdmin(id int64) error
	IsAdmin(id int64) bool
	ListAdmins() ([]domain.AllowedAdmin, error)
}

type SignerRepository interface {
	AddSigner(fingerprint string, armoredPub []byte) error
	ListSigners() ([][]byte, error)
	DeleteSigner(fingerprint string) error
}

type DenylistRepository interface {
	Revoke(jti string, ttl time.Duration) error
	Consume(jti string, ttl time.Duration) (consumed bool, err error)
	IsRevoked(jti string) (bool, error)
	RevokeSession(sessionID string, ttl time.Duration) error
	IsSessionRevoked(sessionID string) (bool, error)
}

var (
	ErrImmutableObjectConflict = errors.New("repository: immutable object content conflict")
	ErrPackageChanged          = errors.New("repository package changed concurrently")
)

type CanonicalCommitError struct{ Err error }

func (e *CanonicalCommitError) Error() string { return e.Err.Error() }
func (e *CanonicalCommitError) Unwrap() error { return e.Err }

func CanonicalCommitted(err error) bool {
	var committed *CanonicalCommitError
	return errors.As(err, &committed)
}
