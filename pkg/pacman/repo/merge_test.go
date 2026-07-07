package repo

import (
	"bytes"
	"sort"
	"testing"
)

// buildDB assembles a .db and .files archive for the given package metas, the way
// a written repository database looks.
func buildDB(t *testing.T, metas ...*struct {
	name, ver string
	files     []string
},
) (db, files []byte) {
	t.Helper()
	b := newDBBuilder()
	for _, m := range metas {
		if err := b.Upsert(metaFor(m.name, m.ver, m.files), nil); err != nil {
			t.Fatalf("upsert %s: %v", m.name, err)
		}
	}
	var dbBuf, filesBuf bytes.Buffer
	if err := b.WriteDB(&dbBuf); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := b.WriteFiles(&filesBuf); err != nil {
		t.Fatalf("write files: %v", err)
	}
	return dbBuf.Bytes(), filesBuf.Bytes()
}

type meta = struct {
	name, ver string
	files     []string
}

func versionsOf(t *testing.T, db []byte) map[string]string {
	t.Helper()
	rr, err := RemoteRepoFromDB("m", bytes.NewReader(db))
	if err != nil {
		t.Fatalf("parse merged db: %v", err)
	}
	out := make(map[string]string, len(rr.Pkgs))
	for _, p := range rr.Pkgs {
		out[p.Name()] = p.Version()
	}
	return out
}

// TestMergeLocalShadowsUpstream proves the merged database is the union of
// upstream and local, with a local package of the same name winning over its
// upstream namesake, and that untouched upstream files survive into the merged
// files database.
func TestMergeLocalShadowsUpstream(t *testing.T) {
	upDB, upFiles := buildDB(t,
		&meta{"foo", "1.0-1", []string{"usr/bin/foo"}},
		&meta{"bar", "1.0-1", []string{"usr/bin/bar"}},
	)
	localDB, localFiles := buildDB(t,
		&meta{"bar", "2.0-1", []string{"usr/bin/bar2"}}, // shadows upstream bar
		&meta{"baz", "1.0-1", []string{"usr/bin/baz"}},  // local-only
	)

	var mergedDB, mergedFiles bytes.Buffer
	if err := Merge(bytes.NewReader(upDB), bytes.NewReader(upFiles), bytes.NewReader(localDB), bytes.NewReader(localFiles), &mergedDB, &mergedFiles); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	got := versionsOf(t, mergedDB.Bytes())
	want := map[string]string{"foo": "1.0-1", "bar": "2.0-1", "baz": "1.0-1"}
	if len(got) != len(want) {
		t.Fatalf("merged packages = %v, want %v", got, want)
	}
	for name, ver := range want {
		if got[name] != ver {
			t.Errorf("merged %s = %q, want %q (local should shadow upstream)", name, got[name], ver)
		}
	}

	// The upstream-only package's files list survives into the merged files db.
	fooFiles, ok := readMember(t, mergedFiles.Bytes(), "foo-1.0-1/files")
	if !ok {
		t.Fatal("merged files db dropped the upstream foo files member")
	}
	if string(fooFiles) != "%FILES%\nusr/bin/foo\n" {
		t.Errorf("foo files member = %q", fooFiles)
	}
	// The shadowed bar carries the LOCAL files list, not upstream's.
	barFiles, ok := readMember(t, mergedFiles.Bytes(), "bar-2.0-1/files")
	if !ok {
		t.Fatal("merged files db missing the local bar files member")
	}
	if string(barFiles) != "%FILES%\nusr/bin/bar2\n" {
		t.Errorf("bar files member = %q (want the local list)", barFiles)
	}
}

// TestMergeNilReaders proves an absent archive is treated as empty: merging only
// a local overlay yields exactly the overlay, and only an upstream yields it.
func TestMergeNilReaders(t *testing.T) {
	localDB, localFiles := buildDB(t, &meta{"only", "1-1", nil})
	var db, files bytes.Buffer
	if err := Merge(nil, nil, bytes.NewReader(localDB), bytes.NewReader(localFiles), &db, &files); err != nil {
		t.Fatalf("Merge with nil upstream: %v", err)
	}
	if got := versionsOf(t, db.Bytes()); len(got) != 1 || got["only"] != "1-1" {
		t.Fatalf("merged (nil upstream) = %v, want [only]", got)
	}
}

// TestDiffDB proves a snapshot diff reports adds, removes and version changes,
// and that an identical snapshot is empty (the sync no-op signal).
func TestDiffDB(t *testing.T) {
	oldDB, _ := buildDB(t,
		&meta{"keep", "1-1", nil},
		&meta{"gone", "1-1", nil},
		&meta{"bump", "1-1", nil},
	)
	newDB, _ := buildDB(t,
		&meta{"keep", "1-1", nil},  // unchanged
		&meta{"bump", "2-1", nil},  // version changed
		&meta{"fresh", "1-1", nil}, // added
	)

	diff, err := DiffDB(bytes.NewReader(oldDB), bytes.NewReader(newDB))
	if err != nil {
		t.Fatalf("DiffDB: %v", err)
	}
	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Updated)
	if len(diff.Added) != 1 || diff.Added[0] != "fresh" {
		t.Errorf("Added = %v, want [fresh]", diff.Added)
	}
	if len(diff.Removed) != 1 || diff.Removed[0] != "gone" {
		t.Errorf("Removed = %v, want [gone]", diff.Removed)
	}
	if len(diff.Updated) != 1 || diff.Updated[0] != "bump" {
		t.Errorf("Updated = %v, want [bump]", diff.Updated)
	}

	same, err := DiffDB(bytes.NewReader(newDB), bytes.NewReader(newDB))
	if err != nil {
		t.Fatalf("DiffDB (identical): %v", err)
	}
	if !same.Empty() {
		t.Errorf("identical snapshots diff = %+v, want empty", same)
	}
}
