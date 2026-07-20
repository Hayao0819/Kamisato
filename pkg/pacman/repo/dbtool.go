package repo

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/safefile"
)

// Tool mutates a pacman repository database on disk: it adds or removes a
// package's entry in the <repo>.db / <repo>.files archives at the given path.
// NativeTool writes the archives in pure Go; CLITool shells out to
// repo-add/repo-remove. The two are interchangeable behind this interface.
type Tool interface {
	RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error
	// RepoAddBatch adds every package in one load-mutate-write cycle, so N packages
	// enter the database atomically (it never appears with a partial set) and the
	// archive is rewritten once. RepoAdd is the single-package shorthand.
	RepoAddBatch(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
	RepoRemove(dbPath, pkgName string, useSignedDB bool, gnupgDir *string) error
	// RebuildDerived rebuilds .files from canonical DB state.
	RebuildDerived(dbPath string, pkgFilePaths []string, useSignedDB bool, gnupgDir *string) error
}

var (
	_ Tool = NativeTool{}
	_ Tool = CLITool{}
)

// ErrPackageNotFound is returned by RepoRemove when the named package has no
// entry in the database. A caller that wants idempotent removal (retry safety)
// treats it as a no-op success.
var ErrPackageNotFound = errors.New("package not found in database")

// NativeTool reads, mutates, and writes the pacman repo-DB archives directly,
// with no repo-add/repo-remove binary, so a server can produce pacman databases
// on any distribution. It operates on the .db archive at dbPath and its siblings
// in the same directory. A nil signer cannot produce a signed database; build a
// signing tool with NewSigningNativeTool.
type NativeTool struct {
	signer *openpgp.Entity
}

// NewSigningNativeTool returns a NativeTool that detach-signs the .db/.files
// archives with entity when a signed database is requested.
func NewSigningNativeTool(entity *openpgp.Entity) NativeTool {
	return NativeTool{signer: entity}
}

// ErrSignedDBUnsupported is returned when a signed database is requested but the
// tool has no signing key (a plain NativeTool{}). Build a NewSigningNativeTool.
var ErrSignedDBUnsupported = errors.New("native repo-db tool: signed databases require a signing key")

// MissingPackageFilesError lists package objects needed to rebuild .files.
type MissingPackageFilesError struct {
	Filenames []string
}

func (e *MissingPackageFilesError) Error() string {
	return fmt.Sprintf("package objects required to rebuild files database: %s", strings.Join(e.Filenames, ", "))
}

func (t NativeTool) RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error {
	var paths []string
	if pkgFilePath != "" {
		paths = []string{pkgFilePath}
	}
	return t.RepoAddBatch(dbPath, paths, useSignedDB, gnupgDir)
}

func (t NativeTool) RepoAddBatch(dbPath string, pkgFilePaths []string, useSignedDB bool, _ *string) error {
	if useSignedDB && t.signer == nil {
		return ErrSignedDBUnsupported
	}
	paths, err := toolPathsFor(dbPath)
	if err != nil {
		return err
	}
	b, err := loadToolBuilder(paths)
	if err != nil {
		return err
	}
	for _, pkgFilePath := range pkgFilePaths {
		if pkgFilePath == "" {
			continue
		}
		meta, err := pkg.ReadBinaryPackageMeta(pkgFilePath)
		if err != nil {
			return fmt.Errorf("failed to read package: %w", err)
		}
		sig, err := readPackageSig(pkgFilePath)
		if err != nil {
			return err
		}
		if err := b.Upsert(meta, sig); err != nil {
			return err
		}
	}
	if err := writeToolBuilder(b, paths); err != nil {
		return err
	}
	return t.maybeSign(paths, useSignedDB)
}

func (t NativeTool) RebuildDerived(dbPath string, pkgFilePaths []string, useSignedDB bool, _ *string) error {
	if useSignedDB && t.signer == nil {
		return ErrSignedDBUnsupported
	}
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
	return t.maybeSign(paths, useSignedDB)
}

func attachPackageFiles(b *dbBuilder, pkgFilePaths []string) error {
	for _, pkgFilePath := range pkgFilePaths {
		if pkgFilePath == "" {
			continue
		}
		meta, err := pkg.ReadBinaryPackageMeta(pkgFilePath)
		if err != nil {
			return fmt.Errorf("failed to read package for files rebuild: %w", err)
		}
		if err := b.AttachFiles(meta); err != nil {
			return err
		}
	}
	return nil
}

// maxPackageSigSize caps an embedded detached signature at repo-add's 16 KiB
// limit; a larger .sig is rejected as invalid.
const maxPackageSigSize = 16384

// readPackageSig returns the detached signature bytes from "<pkgFilePath>.sig",
// or nil when none exists. It mirrors repo-add's --include-sigs constraints —
// binary (non-armored) and at most 16 KiB — so the %PGPSIG% it feeds the desc is
// one pacman can verify.
func readPackageSig(pkgFilePath string) ([]byte, error) {
	sig, err := os.ReadFile(pkgFilePath + ".sig")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read package signature: %w", err)
	}
	if bytes.Contains(sig, []byte("BEGIN PGP SIGNATURE")) {
		return nil, fmt.Errorf("armored package signature is not supported: %s.sig", pkgFilePath)
	}
	if len(sig) > maxPackageSigSize {
		return nil, fmt.Errorf("package signature exceeds %d bytes: %s.sig", maxPackageSigSize, pkgFilePath)
	}
	return sig, nil
}

func (t NativeTool) RepoRemove(dbPath, pkgName string, useSignedDB bool, _ *string) error {
	if useSignedDB && t.signer == nil {
		return ErrSignedDBUnsupported
	}
	paths, err := toolPathsFor(dbPath)
	if err != nil {
		return err
	}
	b, err := loadToolBuilder(paths)
	if err != nil {
		return err
	}
	if !b.Remove(pkgName) {
		return fmt.Errorf("package matching %q: %w", pkgName, ErrPackageNotFound)
	}
	if err := writeToolBuilder(b, paths); err != nil {
		return err
	}
	return t.maybeSign(paths, useSignedDB)
}

// toolPaths holds the four artifact paths a repo-DB mutation touches, all in the
// same directory as the .db archive.
type toolPaths struct {
	db        string // <repo>.db.tar.gz
	files     string // <repo>.files.tar.gz
	dbLink    string // <repo>.db
	filesLink string // <repo>.files
}

// toolPathsFor derives the artifact paths from the .db archive path, mirroring
// repo-add's REPO_DB_PREFIX/REPO_DB_SUFFIX split: everything before the LAST
// ".db." is the prefix, the rest is the compression suffix.
func toolPathsFor(dbPath string) (toolPaths, error) {
	dir := filepath.Dir(dbPath)
	base := filepath.Base(dbPath)
	i := strings.LastIndex(base, ".db.")
	if i < 0 {
		return toolPaths{}, fmt.Errorf("not a valid db archive name: %s", base)
	}
	prefix := base[:i]
	suffix := base[i+len(".db."):]
	return toolPaths{
		db:        filepath.Join(dir, prefix+".db."+suffix),
		files:     filepath.Join(dir, prefix+".files."+suffix),
		dbLink:    filepath.Join(dir, prefix+".db"),
		filesLink: filepath.Join(dir, prefix+".files"),
	}, nil
}

func loadToolBuilder(paths toolPaths) (*dbBuilder, error) {
	b := newDBBuilder()
	if err := loadToolArchive(paths.db, b.LoadDB); err != nil {
		return nil, err
	}
	if err := loadToolArchive(paths.files, b.LoadFiles); err != nil {
		return nil, err
	}
	return b, nil
}

func loadToolArchive(path string, load func(io.Reader) error) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil // a fresh repository has no archive yet
	}
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", path, err)
	}
	defer f.Close()
	return load(f)
}

func writeToolBuilder(b *dbBuilder, paths toolPaths) error {
	if err := writeToolArchive(paths.db, b.WriteDB); err != nil {
		return err
	}
	if err := writeToolArchive(paths.files, b.WriteFiles); err != nil {
		return err
	}
	// A blob store has no symlinks, so the <repo>.db / <repo>.files names that
	// repo-add makes as symlinks are written as byte copies instead.
	if err := copyToolFile(paths.db, paths.dbLink); err != nil {
		return err
	}
	return copyToolFile(paths.files, paths.filesLink)
}

// writeDerivedBuilder rewrites only the .files archive and alias.
func writeDerivedBuilder(b *dbBuilder, paths toolPaths) error {
	if err := writeToolArchive(paths.files, b.WriteFiles); err != nil {
		return err
	}
	return copyToolFile(paths.files, paths.filesLink)
}

func writeToolArchive(path string, write func(io.Writer) error) error {
	return safefile.Replace(path, 0o644, func(out io.Writer) error { //nolint:gosec // pacman repository databases are public
		return write(out)
	})
}

func copyToolFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer in.Close()
	return safefile.Replace(dst, 0o644, func(out io.Writer) error { //nolint:gosec // pacman repository databases are public
		if _, err := io.Copy(out, in); err != nil {
			return fmt.Errorf("failed to copy to %s: %w", dst, err)
		}
		return nil
	})
}
