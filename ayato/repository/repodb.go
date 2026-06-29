package repository

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path"

	"github.com/Hayao0819/Kamisato/ayato/stream"
	"github.com/Hayao0819/Kamisato/internal/utils"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// repoDBTool runs the pacman repo-DB mutations against a local DB path. The
// implementations live in pkg/pacman/repo: the default (repo.NativeTool) writes
// the database archives in Go, so ayato needs no repo-add/repo-remove binary and
// runs on any distribution; repo.CLITool (shelling out to repo-add) remains
// available behind the same port. Keeping it behind a port also lets
// binaryRepository be unit-tested with a fake tool.
type repoDBTool interface {
	RepoAdd(dbPath, pkgFilePath string, useSignedDB bool, gnupgDir *string) error
	RepoRemove(dbPath, pkg string, useSignedDB bool, gnupgDir *string) error
}

func (r *binaryRepository) repoTool() repoDBTool {
	if r.tool != nil {
		return r.tool
	}
	return repo.NativeTool{}
}

// dbArtifactBases are the canonical repo-DB archive names repo-add/repo-remove
// read and rewrite. The matching ".db"/".files" entries are symlinks repo-add
// regenerates, so only the archives need to be seeded into the temp working dir.
func dbArtifactBases(repo string) []string {
	return []string{
		repo + ".db.tar.gz",
		repo + ".files.tar.gz",
	}
}

// writeSeekFileTo writes a SeekFile's bytes into dir under its base name.
// A nil stream is a no-op (returns "").
func writeSeekFileTo(dir string, f stream.SeekFile) (string, error) {
	if f == nil {
		return "", nil
	}
	name := path.Base(f.FileName())
	dst := path.Join(dir, name)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", utils.WrapErr(err, "failed to seek stream")
	}
	out, err := os.Create(dst)
	if err != nil {
		return "", utils.WrapErr(err, "failed to create temp file")
	}
	if _, err := io.Copy(out, f); err != nil {
		out.Close()
		return "", utils.WrapErr(err, "failed to copy stream to temp file")
	}
	if err := out.Close(); err != nil {
		return "", utils.WrapErr(err, "failed to close temp file")
	}
	return dst, nil
}

// storeArtifacts writes every regular file in dir back through blob.StoreFile
// under its bare name, skipping any path in skip. This mirrors the full set of
// artifacts repo-add/repo-remove rewrite (the .db/.files symlinks and their
// .tar.gz archives, plus any .sig) uniformly for every blob backend.
func (r *binaryRepository) storeArtifacts(repo, arch, dir string, skip map[string]struct{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return utils.WrapErr(err, "failed to read temp dir")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, ok := skip[name]; ok {
			continue
		}
		fp := path.Join(dir, name)
		obj, err := stream.OpenFileWithType(fp)
		if err != nil {
			return utils.WrapErr(err, "failed to open artifact "+name)
		}
		// OpenFileWithType keys FileName() off the full path; re-wrap under the
		// bare name so both localfs and s3 store it as <repo>/<arch>/<name>.
		named := stream.NewFileStream(name, obj.ContentType(), obj)
		if err := r.Store.StoreFile(repo, arch, named); err != nil {
			obj.Close()
			return utils.WrapErr(err, "failed to store artifact "+name)
		}
		obj.Close()
	}
	return nil
}

// RepoAdd registers a package in the (repo, arch) database, leaving the package
// file itself to the caller's StoreFile. The per-(repo, arch) dbMu serializes
// these read-modify-writes.
func (r *binaryRepository) RepoAdd(repo, arch string, pkg, sig stream.SeekFile, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	skip := map[string]struct{}{}
	pkgPath, err := writeSeekFileTo(t, pkg)
	if err != nil {
		return err
	}
	if pkgPath != "" {
		skip[path.Base(pkgPath)] = struct{}{}
	}
	sigPath, err := writeSeekFileTo(t, sig)
	if err != nil {
		return err
	}
	if sigPath != "" {
		skip[path.Base(sigPath)] = struct{}{}
	}

	if err := r.fetchDBArtifacts(repo, arch, t, useSignedDB); err != nil {
		return err
	}

	dbPath := path.Join(t, repo+".db.tar.gz")
	if err := r.repoTool().RepoAdd(dbPath, pkgPath, useSignedDB, gnupgDir); err != nil {
		slog.Error("repo db add", "err", err)
		return utils.WrapErr(err, "repo db add failed")
	}

	return r.storeArtifacts(repo, arch, t, skip)
}

// RepoRemove removes a package from the (repo, arch) database. Serialized per
// (repo, arch) via dbMu.
func (r *binaryRepository) RepoRemove(repo, arch, pkg string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	if err := r.fetchDBArtifacts(repo, arch, t, useSignedDB); err != nil {
		return err
	}
	dbPath := path.Join(t, repo+".db.tar.gz")
	if !exists(dbPath) {
		return errors.New("repository database not found")
	}

	if err := r.repoTool().RepoRemove(dbPath, pkg, useSignedDB, gnupgDir); err != nil {
		slog.Error("repo db remove", "err", err)
		return utils.WrapErr(err, "repo db remove failed")
	}

	return r.storeArtifacts(repo, arch, t, nil)
}

// InitArch creates an empty (repo, arch) database. Serialized per (repo, arch)
// via dbMu.
func (r *binaryRepository) InitArch(repo, arch string, useSignedDB bool, gnupgDir *string) error {
	defer r.dbMu.lock(repo + "/" + arch)()
	slog.Debug("init pkg repo", "repo", repo, "arch", arch)

	t, err := os.MkdirTemp("", "ayato-repodb-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(t)

	dbPath := path.Join(t, repo+".db.tar.gz")
	if err := r.repoTool().RepoAdd(dbPath, "", useSignedDB, gnupgDir); err != nil {
		slog.Error("repo db init", "err", err)
		return utils.WrapErr(err, "repo db init failed")
	}

	return r.storeArtifacts(repo, arch, t, nil)
}

// fetchDBArtifacts seeds dir with the live DB archives (and their .sig when
// signed) for (repo, arch). Missing artifacts are tolerated.
func (r *binaryRepository) fetchDBArtifacts(repo, arch, dir string, useSignedDB bool) error {
	names := dbArtifactBases(repo)
	if useSignedDB {
		for _, b := range dbArtifactBases(repo) {
			names = append(names, b+".sig")
		}
	}
	for _, name := range names {
		f, err := r.Store.FetchFile(repo, arch, name)
		if err != nil {
			continue // not present yet
		}
		dst := path.Join(dir, name)
		out, cerr := os.Create(dst)
		if cerr != nil {
			f.Close()
			return utils.WrapErr(cerr, "failed to create temp db artifact")
		}
		if _, cerr := io.Copy(out, f); cerr != nil {
			out.Close()
			f.Close()
			return utils.WrapErr(cerr, "failed to copy db artifact")
		}
		out.Close()
		f.Close()
	}
	return nil
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
