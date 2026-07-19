package repository

import (
	"bytes"
	"encoding/json"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
)

// The upstream snapshot and merged database live as ordinary blob objects beside
// the overlay: the merge can be recomputed from the last-synced snapshot with no
// refetch. The overlay stays the canonical <repo>.db.tar.gz that RepoAddBatch's
// compare-and-swap owns; the served <repo>.db is aliased to the merged archive.
func upstreamArtifacts(name string) pacmanrepo.DatabaseArtifacts {
	return scopedArtifacts(name, "upstream")
}

func mergedArtifacts(name string) pacmanrepo.DatabaseArtifacts {
	return scopedArtifacts(name, "merged")
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

	if err := r.storeArtifactSet(name, arch, databaseArtifactSet{
		names:    upstream,
		database: dbGz,
		files:    filesGz,
	}, true, "upstream snapshot"); err != nil {
		return diff, err
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
	set := databaseArtifactSet{
		names:    merged,
		database: mergedDB.Bytes(),
		files:    mergedFiles.Bytes(),
	}
	if err := r.storeArtifactSet(name, arch, set, false, "merged"); err != nil {
		return err
	}
	if useSignedDB && r.dbSigner != nil {
		if err := r.signArtifactSet(name, arch, set); err != nil {
			return err
		}
	}
	return nil
}
