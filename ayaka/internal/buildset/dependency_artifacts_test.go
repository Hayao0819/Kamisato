package buildset

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type recordingBackend struct {
	specs *[]builder.Spec
}

func (recordingBackend) Name() string { return "fake-container" }

func (backend recordingBackend) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	*backend.specs = append(*backend.specs, spec)
	return (fakeBackend{}).Build(ctx, spec)
}

func TestBuildPassesBuiltDependenciesToLaterBuilds(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"consumer": srcinfo("consumer", []string{"provider"}, nil, "consumer"),
		"provider": srcinfo("provider", nil, nil, "provider"),
	})
	application := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	var specs []builder.Spec
	application.backend = recordingBackend{specs: &specs}
	application.repoTool = repo.NativeTool{}

	_, err := application.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local",
		OutputDir:   filepath.Join(t.TempDir(), "repository"),
		Manifest:    filepath.Join(t.TempDir(), "manifest.json"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("backend builds = %d, want 2", len(specs))
	}
	if len(specs[0].InstallPkgs) != 0 {
		t.Errorf("first build dependencies = %v, want none", specs[0].InstallPkgs)
	}
	if len(specs[1].InstallPkgs) != 1 || filepath.Base(specs[1].InstallPkgs[0]) != "provider-1-1-x86_64.pkg.tar" {
		t.Errorf("second build dependencies = %v, want provider artifact", specs[1].InstallPkgs)
	}
}
