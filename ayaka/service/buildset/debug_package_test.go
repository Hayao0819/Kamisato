package buildset

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type debugPackageBackend struct{}

func (debugPackageBackend) Name() string { return "fake-container" }

func (debugPackageBackend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	result, err := (fakeBackend{}).Build(ctx, spec)
	if err != nil {
		return nil, err
	}
	debugPath := filepath.Join(spec.OutDir, "pkg-debug-1-1-"+spec.Arch+".pkg.tar")
	if err := writeTestPackage(debugPath, "pkg-debug", "1-1", spec.Arch); err != nil {
		return nil, err
	}
	result.Packages = append(result.Packages, debugPath)
	return result, nil
}

func TestBuildAcceptsImplicitDebugPackage(t *testing.T) {
	local := writeLocalSources(t, map[string]string{"pkg": srcinfo("pkg", nil, nil, "pkg")})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = debugPackageBackend{}
	service.repoTool = repo.NativeTool{}

	manifest, err := service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local",
		OutputDir:   filepath.Join(t.TempDir(), "repository"),
		Manifest:    filepath.Join(t.TempDir(), "manifest.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Builds) != 1 || len(manifest.Builds[0].Packages) != 2 {
		t.Fatalf("manifest builds = %+v, want package and implicit debug package", manifest.Builds)
	}
	if manifest.Builds[0].Packages[1].Name != "pkg-debug" {
		t.Errorf("implicit debug package = %q, want pkg-debug", manifest.Builds[0].Packages[1].Name)
	}
	if len(manifest.Install) != 1 || manifest.Install[0] != "pkg" {
		t.Errorf("install packages = %v, want only pkg", manifest.Install)
	}
}
