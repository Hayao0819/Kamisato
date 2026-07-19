package service_test

import (
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/repository/blob"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	pkgpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

// TestReconcileOrphans_DeletesOldUnreferenced proves the reconcile deletes an
// object that is old, absent from the db, and looks like a package — while
// leaving a referenced package, a too-young orphan, and the db artifacts alone.
func TestReconcileOrphans_DeletesOldUnreferenced(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	const (
		referenced = "kept-1.0-1-x86_64.pkg.tar.zst"
		orphanOld  = "orphan-1.0-1-x86_64.pkg.tar.zst"
		orphanNew  = "fresh-1.0-1-x86_64.pkg.tar.zst"
	)

	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)

	// The db references only `referenced`; its .sig is protected too.
	rr := &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(referenced, &raiou.PKGINFO{PkgName: "kept", Arch: "x86_64"}),
	}}
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(rr, nil)

	old := time.Now().Add(-2 * time.Hour)
	fresh := time.Now().Add(-1 * time.Minute)
	bin.EXPECT().FilesWithMeta("myrepo", "x86_64").Return([]blob.FileInfo{
		{Name: referenced, LastModified: old},
		{Name: referenced + ".sig", LastModified: old},
		{Name: orphanOld, LastModified: old},
		{Name: orphanNew, LastModified: fresh},
		{Name: "myrepo.db.tar.gz", LastModified: old},
		{Name: "myrepo.db.tar.gz.sig", LastModified: old},
		{Name: "myrepo.files.tar.gz", LastModified: old},
		{Name: "notes.txt", LastModified: old}, // not a package artifact
	}, nil)

	// The shared any/ dir holds no leftover objects here.
	bin.EXPECT().FilesWithMeta("myrepo", "any").Return(nil, nil)

	// Only the old, unreferenced package object is deleted.
	bin.EXPECT().DeleteOrphanIfUnchanged("myrepo", "x86_64", gomock.Any(), gomock.Any()).Return(true, nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	orphans, err := svc.ReconcileOrphans("myrepo", time.Hour, false)
	if err != nil {
		t.Fatalf("ReconcileOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Name != orphanOld || orphans[0].Arch != "x86_64" {
		t.Fatalf("orphans = %v, want only %s", orphans, orphanOld)
	}
}

// TestReconcileOrphans_DryRunDeletesNothing proves dryRun reports the orphan but
// never calls DeleteFile.
func TestReconcileOrphans_DryRunDeletesNothing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	const orphanOld = "orphan-1.0-1-x86_64.pkg.tar.zst"

	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(&repo.RemoteRepo{}, nil)
	bin.EXPECT().FilesWithMeta("myrepo", "x86_64").Return([]blob.FileInfo{
		{Name: orphanOld, LastModified: time.Now().Add(-2 * time.Hour)},
	}, nil)
	bin.EXPECT().FilesWithMeta("myrepo", "any").Return(nil, nil)
	bin.EXPECT().DeleteOrphanIfUnchanged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	orphans, err := svc.ReconcileOrphans("myrepo", time.Hour, true)
	if err != nil {
		t.Fatalf("ReconcileOrphans dry-run: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Name != orphanOld {
		t.Fatalf("dry-run orphans = %v, want %s reported", orphans, orphanOld)
	}
}

// TestReconcileOrphans_AnyDir proves the shared any/ directory is reconciled: a
// registered arch=any package (referenced by a concrete arch's db) is kept, while
// an orphaned any object PUT but never finalized is deleted.
func TestReconcileOrphans_AnyDir(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	const (
		keptAny   = "kept-1.0-1-any.pkg.tar.zst"
		orphanAny = "orphan-1.0-1-any.pkg.tar.zst"
	)

	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)
	// The concrete db registers keptAny; it fans out to any/ under that filename.
	rr := &repo.RemoteRepo{Pkgs: []*pkgpkg.BinaryPackage{
		pkgpkg.NewBinaryPackage(keptAny, &raiou.PKGINFO{PkgName: "kept", Arch: "any"}),
	}}
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(rr, nil)
	// The concrete arch stores nothing extra of its own.
	bin.EXPECT().FilesWithMeta("myrepo", "x86_64").Return(nil, nil)

	old := time.Now().Add(-2 * time.Hour)
	bin.EXPECT().FilesWithMeta("myrepo", "any").Return([]blob.FileInfo{
		{Name: keptAny, LastModified: old},
		{Name: orphanAny, LastModified: old},
	}, nil)
	// Only the unreferenced any object is deleted; keptAny is protected by the
	// concrete db's registration.
	bin.EXPECT().DeleteOrphanIfUnchanged("myrepo", "any", gomock.Any(), gomock.Any()).Return(true, nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	orphans, err := svc.ReconcileOrphans("myrepo", time.Hour, false)
	if err != nil {
		t.Fatalf("ReconcileOrphans: %v", err)
	}
	if len(orphans) != 1 || orphans[0].Name != orphanAny || orphans[0].Arch != "any" {
		t.Fatalf("orphans = %v, want only %s under any", orphans, orphanAny)
	}
}

// TestReconcileOrphans_MissingDBSkipsArch proves an arch whose db is absent is
// skipped rather than having every object treated as an orphan.
func TestReconcileOrphans_MissingDBSkipsArch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	name := mocks.NewMockNameStore(ctrl)

	bin.EXPECT().Arches("myrepo").Return([]string{"x86_64"}, nil)
	bin.EXPECT().RemoteRepo("myrepo", "x86_64").Return(nil, blob.ErrNotFound)
	// db absent => only db artifacts are referenced; an object still present is an
	// orphan, but the db-artifact names are protected. Delete the package object.
	bin.EXPECT().FilesWithMeta("myrepo", "x86_64").Return([]blob.FileInfo{
		{Name: "stray-1.0-1-x86_64.pkg.tar.zst", LastModified: time.Now().Add(-2 * time.Hour)},
		{Name: "myrepo.db.tar.gz", LastModified: time.Now().Add(-2 * time.Hour)},
	}, nil)
	bin.EXPECT().FilesWithMeta("myrepo", "any").Return(nil, nil)
	bin.EXPECT().DeleteOrphanIfUnchanged("myrepo", "x86_64", gomock.Any(), gomock.Any()).Return(true, nil)

	svc := service.New(name, bin, nil, nil, baseConfig(false, ""))
	orphans, err := svc.ReconcileOrphans("myrepo", time.Hour, false)
	if err != nil {
		t.Fatalf("ReconcileOrphans: %v", err)
	}
	if len(orphans) != 1 {
		t.Fatalf("orphans = %v, want the stray package only (db artifact protected)", orphans)
	}
}
