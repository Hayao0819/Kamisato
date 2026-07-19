package service_test

import (
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/internal/errors"
)

func uploadArch(t *testing.T, svc uploader, repo, name, arch string) error {
	t.Helper()
	fname := name + "-1.0-1-" + arch + ".pkg.tar.zst"
	return svc.UploadFile(repo, &domain.UploadFiles{
		PkgFile: pkgStream(fname, buildPackage(t, name, "1.0-1", arch)),
	})
}

type uploader interface {
	UploadFile(repo string, files *domain.UploadFiles) error
}

// TestUploadAnyFansOutToDeclaredArches proves an arch=any package published to a
// repo that has no stored packages yet still reaches every declared arch's db —
// the fan-out set comes from the declaration, not only from stored arches, so an
// any package is installable on the very first upload.
func TestUploadAnyFansOutToDeclaredArches(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "anyrepo", Arches: []string{"x86_64", "aarch64"}}})

	if err := uploadArch(t, svc, "anyrepo", "noarch", "any"); err != nil {
		t.Fatalf("upload arch=any to an empty declared repo: %v", err)
	}
	for _, a := range []string{"x86_64", "aarch64"} {
		if got := pkgNames(t, svc, "anyrepo", a); !has(got, "noarch") {
			t.Fatalf("arch=any package missing from %s db: %v", a, got)
		}
	}
}

// TestUploadRejectsUndeclaredArch proves a repo that declares its arches and does
// not opt into new ones rejects an upload for an arch outside the set, so a
// mislabeled package cannot silently add an arch (e.g. x86_64 into an i686 repo).
func TestUploadRejectsUndeclaredArch(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "pinned", Arches: []string{"x86_64"}}})

	err := uploadArch(t, svc, "pinned", "foo", "aarch64")
	if !errors.Is(err, domain.ErrInvalidUpload) {
		t.Fatalf("expected an undeclared arch to be rejected with ErrInvalidUpload, got %v", err)
	}
	if got := pkgNames(t, svc, "pinned", "aarch64"); len(got) != 0 {
		t.Fatalf("rejected upload must not create the arch db: %v", got)
	}
}

// TestAllowNewArchBackfillsAny proves that when a repo opts into new arches, the
// first concrete upload for a new arch both creates that arch and backfills the
// repo's already-published arch=any packages into it — an any package added before
// the arch existed stays installable there.
func TestAllowNewArchBackfillsAny(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "growable", Arches: []string{"x86_64"}, AllowNewArch: true}})

	if err := uploadArch(t, svc, "growable", "noarch", "any"); err != nil {
		t.Fatalf("upload arch=any: %v", err)
	}
	if got := pkgNames(t, svc, "growable", "x86_64"); !has(got, "noarch") {
		t.Fatalf("arch=any package missing from the only arch so far: %v", got)
	}

	if err := uploadArch(t, svc, "growable", "bar", "aarch64"); err != nil {
		t.Fatalf("upload concrete aarch64 to grow the repo: %v", err)
	}
	got := pkgNames(t, svc, "growable", "aarch64")
	if !has(got, "bar") {
		t.Fatalf("concrete aarch64 package missing from the new arch: %v", got)
	}
	if !has(got, "noarch") {
		t.Fatalf("arch=any package not backfilled into the new arch aarch64: %v", got)
	}
}

// TestAllowNewArchBatchUpgradesBackfilledAny covers the state ordering inside a
// mixed batch: the new arch is seeded first, then the any-package upgrade takes
// its conditional snapshot. Otherwise the backfilled old version looks like a
// concurrent insert and the batch rejects its own work.
func TestAllowNewArchBatchUpgradesBackfilledAny(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name:         "growable",
		Arches:       []string{"x86_64"},
		AllowNewArch: true,
	}})
	oldName := "noarch-0.9-1-any.pkg.tar.zst"
	if err := svc.UploadFile("growable", &domain.UploadFiles{
		PkgFile: pkgStream(oldName, buildPackage(t, "noarch", "0.9-1", "any")),
	}); err != nil {
		t.Fatalf("upload old any package: %v", err)
	}

	err := svc.UploadFiles("growable", []*domain.UploadFiles{
		{
			PkgFile: pkgStream(
				"bar-1.0-1-aarch64.pkg.tar.zst",
				buildPackage(t, "bar", "1.0-1", "aarch64"),
			),
		},
		{
			PkgFile: pkgStream(
				"noarch-1.0-1-any.pkg.tar.zst",
				buildPackage(t, "noarch", "1.0-1", "any"),
			),
		},
	})
	if err != nil {
		t.Fatalf("mixed new-arch/any-upgrade batch: %v", err)
	}
	detail, err := svc.PkgDetail("growable", "aarch64", "noarch")
	if err != nil || detail.PkgVer != "1.0-1" {
		t.Fatalf("aarch64 noarch = %+v, %v; want upgraded 1.0-1", detail, err)
	}
}
