package repo

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"

	pkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
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
		if err := b.Upsert(meta); err != nil {
			return err
		}
	}
	if err := writeToolBuilder(b, paths); err != nil {
		return err
	}
	return t.maybeSign(paths, useSignedDB)
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

func writeToolArchive(path string, write func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", path, err)
	}
	if err := write(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func copyToolFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("failed to copy to %s: %w", dst, err)
	}
	return out.Close()
}
