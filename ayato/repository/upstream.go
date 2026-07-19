package repository

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/stream"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// The upstream snapshot and merged database live as ordinary blob objects beside
// the overlay: the merge can be recomputed from the last-synced snapshot with no
// refetch. The overlay stays the canonical <repo>.db.tar.gz that RepoAddBatch's
// compare-and-swap owns; the served <repo>.db is aliased to the merged archive.
func upstreamArtifacts(name string) pacmanrepo.DatabaseArtifacts {
	return pacmanrepo.Artifacts(name).WithArchivePrefix(name + ".upstream")
}

func mergedArtifacts(name string) pacmanrepo.DatabaseArtifacts {
	return pacmanrepo.Artifacts(name).WithArchivePrefix(name + ".merged")
}

func upstreamMetaName(name string) string { return name + ".upstream.meta.json" }

// upstreamMeta records the conditional-GET validators of the last synced snapshot.
type upstreamMeta struct {
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

// UpstreamValidators returns the stored ETag/Last-Modified of the last synced
// upstream snapshot; empty (and no error) when nothing has been synced yet.
func (r *binaryRepository) UpstreamValidators(name, arch string) (string, string, error) {
	f, err := r.Store.FetchFile(name, arch, upstreamMetaName(name))
	if errors.Is(err, blob.ErrNotFound) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	var m upstreamMeta
	if derr := json.NewDecoder(f).Decode(&m); derr != nil {
		return "", "", nil // a corrupt marker just forces a full refetch
	}
	return m.ETag, m.LastModified, nil
}

// ApplyUpstreamSnapshot records a freshly fetched upstream db/files snapshot and
// its validators, then rebuilds the merged database — all under one per-(repo,
// arch) lock so a concurrent overlay publish never reads a half-written snapshot.
// The returned diff is the change relative to the previous snapshot.
func (r *binaryRepository) ApplyUpstreamSnapshot(name, arch string, dbGz, filesGz []byte, etag, lastModified string, useSignedDB bool) (pacmanrepo.DBDiff, error) {
	defer r.dbMu.lock(name + "/" + arch)()
	upstream := upstreamArtifacts(name)

	// Diff against the previous snapshot before it is overwritten (best-effort
	// observability; a diff failure does not fail the sync).
	prev, _ := r.fetchBytes(name, arch, upstream.DatabaseArchive())
	diff, derr := pacmanrepo.DiffDB(bytesReaderOrNil(prev), bytes.NewReader(dbGz))
	if derr != nil {
		diff = pacmanrepo.DBDiff{}
	}

	if err := r.storeBytes(name, arch, upstream.DatabaseArchive(), dbGz); err != nil {
		return diff, errors.WrapErr(err, "store upstream db snapshot")
	}
	if filesGz != nil {
		if err := r.storeBytes(name, arch, upstream.FilesArchive(), filesGz); err != nil {
			return diff, errors.WrapErr(err, "store upstream files snapshot")
		}
	}
	meta, _ := json.Marshal(upstreamMeta{ETag: etag, LastModified: lastModified})
	if err := r.storeBytes(name, arch, upstreamMetaName(name), meta); err != nil {
		return diff, errors.WrapErr(err, "store upstream validators")
	}
	if err := r.rebuildMergedLocked(name, arch, useSignedDB); err != nil {
		return diff, err
	}
	return diff, nil
}

// RebuildMerged recomputes the served database for an upstream repo by merging the
// stored upstream snapshot with the local overlay. It is how a local publish or
// removal reaches an upstream repo's served view.
func (r *binaryRepository) RebuildMerged(name, arch string, useSignedDB bool) error {
	defer r.dbMu.lock(name + "/" + arch)()
	return r.rebuildMergedLocked(name, arch, useSignedDB)
}

// rebuildMergedLocked merges the overlay on top of the upstream snapshot (local
// shadows upstream) and stores the merged archives, signing them when a signed
// database is requested. The caller holds dbMu. Local overlay entries are always
// re-applied, so a sync never drops a locally published package.
func (r *binaryRepository) rebuildMergedLocked(name, arch string, useSignedDB bool) error {
	overlay := pacmanrepo.Artifacts(name)
	upstream := upstreamArtifacts(name)
	merged := mergedArtifacts(name)
	overlayDB, err := r.fetchBytes(name, arch, overlay.DatabaseArchive())
	if err != nil {
		return errors.WrapErr(err, "read overlay db")
	}
	overlayFiles, err := r.fetchBytes(name, arch, overlay.FilesArchive())
	if err != nil {
		return errors.WrapErr(err, "read overlay files db")
	}
	upDB, err := r.fetchBytes(name, arch, upstream.DatabaseArchive())
	if err != nil {
		return errors.WrapErr(err, "read upstream snapshot db")
	}
	upFiles, err := r.fetchBytes(name, arch, upstream.FilesArchive())
	if err != nil {
		return errors.WrapErr(err, "read upstream snapshot files")
	}

	var mergedDB, mergedFiles bytes.Buffer
	if err := pacmanrepo.Merge(bytesReaderOrNil(upDB), bytesReaderOrNil(upFiles), bytesReaderOrNil(overlayDB), bytesReaderOrNil(overlayFiles), &mergedDB, &mergedFiles); err != nil {
		return errors.WrapErr(err, "merge upstream and overlay databases")
	}
	if err := r.storeBytes(name, arch, merged.DatabaseArchive(), mergedDB.Bytes()); err != nil {
		return errors.WrapErr(err, "store merged db")
	}
	if err := r.storeBytes(name, arch, merged.FilesArchive(), mergedFiles.Bytes()); err != nil {
		return errors.WrapErr(err, "store merged files db")
	}
	if useSignedDB && r.dbSigner != nil {
		if err := r.signMerged(name, arch, merged.DatabaseArchive(), mergedDB.Bytes()); err != nil {
			return err
		}
		if err := r.signMerged(name, arch, merged.FilesArchive(), mergedFiles.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (r *binaryRepository) signMerged(name, arch, archiveName string, data []byte) error {
	var sig bytes.Buffer
	if err := pacmanrepo.SignDetached(r.dbSigner, bytes.NewReader(data), &sig); err != nil {
		return errors.WrapErr(err, "sign merged db")
	}
	return r.storeBytes(name, arch, archiveName+".sig", sig.Bytes())
}

// fetchBytes reads a stored object fully; a missing object is (nil, nil) so the
// merge treats it as empty.
func (r *binaryRepository) fetchBytes(name, arch, file string) ([]byte, error) {
	f, err := r.Store.FetchFile(name, arch, file)
	if errors.Is(err, blob.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func (r *binaryRepository) storeBytes(name, arch, file string, data []byte) error {
	fs := stream.NewFileStream(file, "application/gzip", byteSeeker{bytes.NewReader(data)})
	return r.Store.StoreFile(name, arch, fs)
}

func bytesReaderOrNil(b []byte) io.Reader {
	if b == nil {
		return nil
	}
	return bytes.NewReader(b)
}

type byteSeeker struct{ *bytes.Reader }

func (byteSeeker) Close() error { return nil }
