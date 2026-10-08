package buildset

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type recordingMetadataBackend struct {
	called bool
	spec   builder.Spec
}

func (b *recordingMetadataBackend) GenerateSRCINFO(_ context.Context, spec builder.Spec) ([]byte, error) {
	b.called = true
	b.spec = spec
	return []byte(srcinfo("local", nil, nil, "local")), nil
}

func TestPlanGeneratesMissingSRCINFOFromCopiedSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "local")
	if err := os.Mkdir(source, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "PKGBUILD"), []byte("pkgname=local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repository := t.TempDir()
	server := (&url.URL{Scheme: "file", Path: repository}).String()
	metadata := &recordingMetadataBackend{}
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.metadata = metadata
	service.loadPacmanConfig = func(_, arch string) (*host.BuildConfig, error) {
		return &host.BuildConfig{
			Architecture: arch,
			PacmanConf:   []byte("[options]\nArchitecture = " + arch + "\n"),
			Repositories: []host.BuildRepository{{Name: "local", Servers: []string{server}}},
		}, nil
	}
	plan, err := service.Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: []string{source}, PacmanConf: "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.called || metadata.spec.SrcDir == source {
		t.Fatalf("metadata call = %+v", metadata)
	}
	if len(metadata.spec.LocalRepositoryDirs) != 1 || metadata.spec.LocalRepositoryDirs[0] != repository {
		t.Fatalf("local repository directories = %v", metadata.spec.LocalRepositoryDirs)
	}
	if len(plan.Builds) != 1 || plan.Builds[0].Pkgbase != "local" {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := os.Stat(filepath.Join(source, ".SRCINFO")); !os.IsNotExist(err) {
		t.Fatalf("input source was modified: %v", err)
	}
}

func TestPlanRejectsUnsafePackageNameFromLocalSRCINFO(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"unsafe": "pkgbase = unsafe\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = ../escape\n",
	})
	_, err := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: local, PacmanConf: "ignored",
	})
	if err == nil {
		t.Fatal("unsafe package name was accepted")
	}
}

func TestPlanRejectsUnsupportedTargetArchitectureWithoutInputs(t *testing.T) {
	_, err := New(Dependencies{}).Plan(context.Background(), PlanRequest{Arch: "riscv64"})
	if err == nil {
		t.Fatal("unsupported target architecture was accepted")
	}
}
