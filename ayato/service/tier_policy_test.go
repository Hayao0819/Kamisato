package service_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"
	"go.uber.org/mock/gomock"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/platform"
	"github.com/Hayao0819/Kamisato/ayato/repository"
	"github.com/Hayao0819/Kamisato/ayato/service"
	"github.com/Hayao0819/Kamisato/ayato/test/mocks"
	"github.com/Hayao0819/Kamisato/internal/conf"
	pacmanpkg "github.com/Hayao0819/Kamisato/pkg/pacman/pkg"
	pacmanrepo "github.com/Hayao0819/Kamisato/pkg/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestTieredPromotionKeepInSource(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{{
		Name: "myrepo", Tiered: true, PromotionKeepInSource: true,
	}})

	uploadPkg(t, svc, "myrepo", "foo")
	err := svc.PromotePackage(
		context.Background(),
		"myrepo",
		domain.TierStaging,
		domain.TierTesting,
		"foo",
		"",
	)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if !has(pkgNames(t, svc, "myrepo-testing", "x86_64"), "foo") {
		t.Fatal("promotion did not add foo to the testing tier")
	}
	if !has(pkgNames(t, svc, "myrepo-staging", "x86_64"), "foo") {
		t.Fatal("keep-in-source policy dropped foo from the staging tier")
	}
}

func TestPromotionRejectsInvalidRequests(t *testing.T) {
	svc, _, _ := newTieredService(t, []conf.BinRepoConfig{
		{Name: "myrepo", Tiered: true},
		{Name: "single"},
	})
	ctx := context.Background()
	uploadPkg(t, svc, "myrepo", "foo")

	tests := []struct {
		name string
		repo string
		from domain.Tier
		to   domain.Tier
		pkg  string
		ver  string
		want error
	}{
		{"non-tiered repo", "single", domain.TierStaging, domain.TierTesting, "foo", "", domain.ErrInvalid},
		{"non-adjacent step", "myrepo", domain.TierStaging, domain.TierStable, "foo", "", domain.ErrInvalid},
		{"version mismatch", "myrepo", domain.TierStaging, domain.TierTesting, "foo", "9.9-9", domain.ErrInvalid},
		{"absent package", "myrepo", domain.TierStaging, domain.TierTesting, "ghost", "", domain.ErrNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := svc.PromotePackage(ctx, test.repo, test.from, test.to, test.pkg, test.ver)
			if !errors.Is(err, test.want) {
				t.Fatalf("PromotePackage = %v, want %v", err, test.want)
			}
		})
	}
	if len(pkgNames(t, svc, "myrepo-testing", "x86_64")) != 0 {
		t.Fatal("a refused promotion half-updated the testing tier")
	}
}

func TestTieredOffUnchanged(t *testing.T) {
	svc, cfg, _ := newTieredService(t, []conf.BinRepoConfig{{Name: "single"}})
	catalog, err := cfg.RepositoryCatalog()
	if err != nil {
		t.Fatalf("RepositoryCatalog: %v", err)
	}
	if names := catalog.PhysicalNames(); len(names) != 1 || names[0] != "single" {
		t.Fatalf("PhysicalNames = %v, want [single]", names)
	}
	uploadPkg(t, svc, "single", "foo")
	if !has(pkgNames(t, svc, "single", "x86_64"), "foo") {
		t.Fatal("upload to a non-tiered repo did not land in the repo")
	}
}

func TestPromotionRegistersCarriedPackageSignature(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	bin := mocks.NewMockBinaryRepository(ctrl)
	names := mocks.NewMockNameStore(ctrl)
	cfg := &conf.AyatoConfig{Repos: []conf.BinRepoConfig{{
		Name:                  "myrepo",
		Tiered:                true,
		Arches:                []string{"x86_64"},
		PromotionKeepInSource: true,
	}}}
	sourceName := "foo-1.0-1-x86_64.pkg.tar.zst"
	sourceRepo := &pacmanrepo.RemoteRepo{Pkgs: []*pacmanpkg.BinaryPackage{
		pacmanpkg.NewBinaryPackage(
			sourceName,
			&raiou.PKGINFO{PkgName: "foo", PkgVer: "1.0-1", Arch: "x86_64"},
		),
	}}
	bin.EXPECT().Arches("myrepo-staging").Return([]string{"x86_64"}, nil)
	bin.EXPECT().RemoteRepo("myrepo-staging", "x86_64").Return(sourceRepo, nil)
	bin.EXPECT().RemoteRepo("myrepo-testing", "x86_64").Return(&pacmanrepo.RemoteRepo{}, nil)
	bin.EXPECT().FetchFile("myrepo-staging", "x86_64", sourceName).
		Return(pkgStream(sourceName, []byte("package")), nil)
	bin.EXPECT().FetchFile("myrepo-staging", "x86_64", sourceName+".sig").
		Return(platform.NewFileStream(
			sourceName+".sig",
			"application/pgp-signature",
			bufferToReadSeekCloser(bytes.NewBufferString("signature")),
		), nil)
	bin.EXPECT().StoreFileImmutable("myrepo-testing", "x86_64", gomock.Any()).
		Return(true, nil).Times(2)
	bin.EXPECT().RepoAddBatch(
		"myrepo-testing", "x86_64", gomock.Any(), false, gomock.Nil(),
	).DoAndReturn(func(
		_ string,
		_ string,
		items []repository.RepoAddItem,
		_ bool,
		_ *string,
	) error {
		if len(items) != 1 || items[0].Sig == nil {
			t.Fatalf("promoted RepoAddItem = %+v, want carried signature", items)
		}
		return nil
	})
	names.EXPECT().StorePackageFile(
		"myrepo-testing", "x86_64", "foo", sourceName,
	).Return(nil)

	svc := service.New(names, bin, nil, nil, cfg)
	err := svc.PromotePackage(
		context.Background(),
		"myrepo",
		domain.TierStaging,
		domain.TierTesting,
		"foo",
		"1.0-1",
	)
	if err != nil {
		t.Fatalf("PromotePackage: %v", err)
	}
}
