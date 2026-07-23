package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/internal/limits"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
)

const (
	stagedUploadTTL = time.Hour
	// stagedUploadGCCutoff abandons an intent that was never committed. It must
	// dwarf stagedUploadTTL so gc can never reclaim an in-flight upload.
	stagedUploadGCCutoff = 24 * time.Hour
)

// stagedIntentIDPattern matches the hex ids newStagedIntentID generates and
// rejects anything else before it reaches a storage key.
var stagedIntentIDPattern = regexp.MustCompile(`^[0-9a-f]{16,64}$`)

// PresignUpload grants presigned staging PUTs for repo, one per requested
// file, and lazily GCs abandoned staged intents on the way out.
func (s *Service) PresignUpload(repo string, files []domain.StagedFileRequest) (*domain.StagedUploadGrant, error) {
	staged, ok := s.pkgBinaryRepo.StagedUploads()
	if !ok {
		return nil, domain.ErrNotImplemented
	}
	target := s.publishTarget(repo)
	if err := s.pkgBinaryRepo.VerifyPkgRepo(target); err != nil {
		return nil, fmt.Errorf("%w: repository %q not found", domain.ErrNotFound, repo)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: at least one file is required", domain.ErrInvalidUpload)
	}
	if len(files) > limits.BatchPackages(s.batchPackagesLimit()) {
		return nil, fmt.Errorf("%w: too many files in one staged upload", domain.ErrInvalidUpload)
	}

	id, err := newStagedIntentID()
	if err != nil {
		return nil, errors.WrapErr(err, "generate staged upload id")
	}

	urls := make(map[string]string, len(files))
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		if err := validateStagedFileName(file.Name); err != nil {
			return nil, err
		}
		if _, duplicate := seen[file.Name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate file %q", domain.ErrInvalidUpload, file.Name)
		}
		seen[file.Name] = struct{}{}
		// Size is mandatory: it is signed into the PUT so storage enforces it.
		if file.Size <= 0 {
			return nil, fmt.Errorf("%w: %q needs a positive size", domain.ErrInvalidUpload, file.Name)
		}
		if err := s.checkStagedSize(file.Name, file.Size); err != nil {
			return nil, err
		}
		url, err := staged.PresignStagedPut(id, file.Name, file.Size, stagedUploadTTL)
		if err != nil {
			return nil, errors.WrapErr(err, "presign staged upload for "+file.Name)
		}
		urls[file.Name] = url
	}

	s.gcStagedUploads(staged)

	return &domain.StagedUploadGrant{
		ID:         id,
		TTLSeconds: int(stagedUploadTTL.Seconds()),
		URLs:       urls,
	}, nil
}

// CommitUpload fetches every entry from staging, then runs it through the same
// pipeline as a direct upload. A validation failure leaves the staged objects
// in place for gcStagedUploads to reclaim; a successful commit deletes them.
func (s *Service) CommitUpload(repo, id string, entries []domain.StagedCommitEntry) error {
	staged, ok := s.pkgBinaryRepo.StagedUploads()
	if !ok {
		return domain.ErrNotImplemented
	}
	if err := validateStagedIntentID(id); err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("%w: commit requires at least one file", domain.ErrInvalidUpload)
	}
	if len(entries) > limits.BatchPackages(s.batchPackagesLimit()) {
		return fmt.Errorf("%w: too many files in one commit", domain.ErrInvalidUpload)
	}

	files, cleanup, err := spoolStagedEntries(staged, id, entries, s.maxPackageSize())
	defer cleanup()
	if err != nil {
		return err
	}

	if err := s.UploadFiles(repo, files); err != nil {
		return err
	}

	if err := staged.DeleteStaged(id); err != nil {
		slog.Warn("failed to delete staged upload after commit", "id", id, "error", err)
	}
	return nil
}

func spoolStagedEntries(
	staged blob.StagedUploader,
	id string,
	entries []domain.StagedCommitEntry,
	maxPackage int,
) ([]*domain.UploadFiles, func(), error) {
	files := make([]*domain.UploadFiles, 0, len(entries))
	var cleanups []func()
	cleanup := func() {
		for _, c := range cleanups {
			c()
		}
	}
	for _, entry := range entries {
		pkgFile, pkgCleanup, err := spoolStagedFile(staged, id, entry.Package, limits.PackageBytes(maxPackage))
		if err != nil {
			return nil, cleanup, err
		}
		cleanups = append(cleanups, pkgCleanup)
		upload := &domain.UploadFiles{PkgFile: pkgFile}
		if entry.Signature != "" {
			sigFile, sigCleanup, err := spoolStagedFile(staged, id, entry.Signature, limits.MaxSignatureBytes)
			if err != nil {
				return nil, cleanup, err
			}
			cleanups = append(cleanups, sigCleanup)
			upload.SigFile = sigFile
		}
		files = append(files, upload)
	}
	return files, cleanup, nil
}

// spoolStagedFile bounds the spool: the staged object was size-checked at
// presign, but the local temp copy must not trust storage state.
func spoolStagedFile(staged blob.StagedUploader, id, name string, maxBytes int64) (platform.SeekFile, func(), error) {
	if err := validateStagedFileName(name); err != nil {
		return nil, nil, err
	}
	source, err := staged.FetchStaged(id, name)
	if err != nil {
		if errors.Is(err, blob.ErrNotFound) {
			return nil, nil, fmt.Errorf("%w: staged file %q not found or expired", domain.ErrInvalidUpload, name)
		}
		return nil, nil, errors.WrapErr(err, "fetch staged file "+name)
	}
	return spoolSource(source, name, maxBytes)
}

// gcStagedUploads best-effort expires abandoned staged intents; it piggybacks
// on presign since that is the path guaranteed to run often enough.
func (s *Service) gcStagedUploads(staged blob.StagedUploader) {
	intents, err := staged.ListStagedIntents()
	if err != nil {
		slog.Warn("failed to list staged upload intents for gc", "error", err)
		return
	}
	cutoff := time.Now().Add(-stagedUploadGCCutoff)
	for _, intent := range intents {
		if intent.ModTime.After(cutoff) {
			continue
		}
		if err := staged.DeleteStaged(intent.ID); err != nil {
			slog.Warn("failed to delete expired staged upload", "id", intent.ID, "error", err)
		}
	}
}

func (s *Service) checkStagedSize(name string, size int64) error {
	limit := limits.PackageBytes(s.maxPackageSize())
	if strings.HasSuffix(name, ".sig") {
		limit = limits.MaxSignatureBytes
	}
	if size > limit {
		return fmt.Errorf(
			"%w: %q exceeds its size limit (%d > %d bytes)",
			domain.ErrInvalidUpload, name, size, limit,
		)
	}
	return nil
}

func (s *Service) batchPackagesLimit() int {
	if s.cfg != nil {
		return s.cfg.MaxBatchPackages
	}
	return 0
}

func (s *Service) maxPackageSize() int {
	if s.cfg != nil {
		return s.cfg.MaxSize
	}
	return 0
}

func validateStagedFileName(name string) error {
	if _, err := pacmanpkg.ParseArtifact(name); err != nil {
		return fmt.Errorf("%w: invalid staged file name %q", domain.ErrInvalidUpload, name)
	}
	return nil
}

func validateStagedIntentID(id string) error {
	if !stagedIntentIDPattern.MatchString(id) {
		return fmt.Errorf("%w: invalid staged upload id", domain.ErrInvalidUpload)
	}
	return nil
}

func newStagedIntentID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
