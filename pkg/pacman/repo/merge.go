package repo

import (
	"bytes"
	"io"
)

// Merge writes a merged repository database that layers a local overlay on top of
// an upstream one: a package present in both is taken from the local overlay, so
// LOCAL SHADOWS UPSTREAM on a name collision. Any reader may be nil (an absent
// archive is treated as empty). It reuses the native db builder, so the merged
// .db/.files are byte-identical in format to a normally written database.
func Merge(upstreamDB, upstreamFiles, localDB, localFiles io.Reader, dbOut, filesOut io.Writer) error {
	// Buffer the (small) local overlay: its names are read once to shadow upstream
	// and its entries are loaded again on top of the merged builder.
	localDBBytes, err := readAllMaybe(localDB)
	if err != nil {
		return err
	}
	localFilesBytes, err := readAllMaybe(localFiles)
	if err != nil {
		return err
	}

	overlay := newDBBuilder()
	if err := overlay.LoadDB(bytes.NewReader(localDBBytes)); err != nil {
		return err
	}

	merged := newDBBuilder()
	if err := merged.LoadDB(upstreamDB); err != nil {
		return err
	}
	if err := merged.LoadFiles(upstreamFiles); err != nil {
		return err
	}
	// Drop every upstream entry a local package of the same name shadows, then
	// layer the local overlay on top.
	for _, name := range overlay.names() {
		merged.Remove(name)
	}
	if err := merged.LoadDB(bytes.NewReader(localDBBytes)); err != nil {
		return err
	}
	if err := merged.LoadFiles(bytes.NewReader(localFilesBytes)); err != nil {
		return err
	}
	if err := merged.WriteDB(dbOut); err != nil {
		return err
	}
	return merged.WriteFiles(filesOut)
}

// DBDiff is the change between two database snapshots by package name: names only
// in the new one (Added), only in the old one (Removed), and present in both at a
// different version (Updated). It makes an incremental upstream sync observable.
type DBDiff struct {
	Added   []string
	Removed []string
	Updated []string
}

// Empty reports whether the two snapshots were identical by (name, version).
func (d DBDiff) Empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Updated) == 0
}

// DiffDB computes what changed between an old and a new database snapshot. A nil
// reader is an empty snapshot, so the first sync reports every upstream package
// as Added.
func DiffDB(oldDB, newDB io.Reader) (DBDiff, error) {
	oldIdx, err := indexVersions(oldDB)
	if err != nil {
		return DBDiff{}, err
	}
	newIdx, err := indexVersions(newDB)
	if err != nil {
		return DBDiff{}, err
	}
	var diff DBDiff
	for name, nv := range newIdx {
		ov, ok := oldIdx[name]
		if !ok {
			diff.Added = append(diff.Added, name)
		} else if ov != nv {
			diff.Updated = append(diff.Updated, name)
		}
	}
	for name := range oldIdx {
		if _, ok := newIdx[name]; !ok {
			diff.Removed = append(diff.Removed, name)
		}
	}
	return diff, nil
}

// indexVersions parses a .db stream into a name->version map. A nil reader is an
// empty index.
func indexVersions(db io.Reader) (map[string]string, error) {
	if db == nil {
		return map[string]string{}, nil
	}
	rr, err := RemoteRepoFromDB("", db)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]string, len(rr.Pkgs))
	for _, p := range rr.Pkgs {
		idx[p.Name()] = p.Version()
	}
	return idx, nil
}

func readAllMaybe(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	return io.ReadAll(r)
}
