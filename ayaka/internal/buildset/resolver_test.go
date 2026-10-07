package buildset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/host"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

type fakeAUR struct {
	packages map[string]aurweb.Pkg
	provides map[string][]string
	sources  map[string]string
}

func (f *fakeAUR) Info(_ context.Context, names []string) ([]aurweb.Pkg, error) {
	var result []aurweb.Pkg
	for _, name := range names {
		if pkg, ok := f.packages[name]; ok {
			result = append(result, pkg)
		}
	}
	return result, nil
}

func (f *fakeAUR) Search(_ context.Context, by aurweb.By, name string) ([]aurweb.Pkg, error) {
	if by != aurweb.ByProvides {
		return nil, fmt.Errorf("unexpected search field %q", by)
	}
	var result []aurweb.Pkg
	for _, packageName := range f.provides[name] {
		result = append(result, f.packages[packageName])
	}
	return result, nil
}

func (f *fakeAUR) GitBase() string { return "https://aur.example" }

func testApplication(t *testing.T, aur *fakeAUR, repositories []repositorySnapshot) *Application {
	t.Helper()
	return New(Dependencies{
		AUR: aur,
		Clone: func(_ context.Context, sourceURL, dir string) (string, error) {
			base := strings.TrimSuffix(filepath.Base(sourceURL), ".git")
			data, ok := aur.sources[base]
			if !ok {
				return "", fmt.Errorf("missing source %s", base)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=x\n"), 0o644); err != nil {
				return "", err
			}
			if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(data), 0o644); err != nil {
				return "", err
			}
			return "0123456789abcdef", nil
		},
		LoadPacmanConfig: func(_, arch string) (*host.BuildConfig, error) {
			return &host.BuildConfig{Architecture: arch, PacmanConf: []byte("[options]\nArchitecture = " + arch + "\n")}, nil
		},
		LoadRepositories: func(context.Context, *host.BuildConfig) ([]repositorySnapshot, error) {
			return repositories, nil
		},
	})
}

func writeLocalSources(t *testing.T, sources map[string]string) []string {
	t.Helper()
	root := t.TempDir()
	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	slices.Sort(names)
	directories := make([]string, 0, len(names))
	for _, name := range names {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(sources[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		directories = append(directories, dir)
	}
	return directories
}

func srcinfo(base string, depends, provides []string, outputs ...string) string {
	var text strings.Builder
	fmt.Fprintf(&text, "pkgbase = %s\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n", base)
	for _, dependency := range depends {
		fmt.Fprintf(&text, "\tmakedepends = %s\n", dependency)
	}
	for _, output := range outputs {
		fmt.Fprintf(&text, "\npkgname = %s\n", output)
		for _, provided := range provides {
			fmt.Fprintf(&text, "\tprovides = %s\n", provided)
		}
	}
	return text.String()
}

func TestPlanResolvesAURAndLocalSourcesInOneGraph(t *testing.T) {
	aur := &fakeAUR{
		packages: map[string]aurweb.Pkg{
			"aur-root": {Name: "aur-root", PackageBase: "aur-root", Version: "1-1"},
			"aur-dep":  {Name: "aur-dep", PackageBase: "aur-dep", Version: "1-1"},
		},
		provides: map[string][]string{},
		sources: map[string]string{
			"aur-root": srcinfo("aur-root", []string{"local-app"}, nil, "aur-root"),
			"aur-dep":  srcinfo("aur-dep", nil, nil, "aur-dep"),
		},
	}
	local := writeLocalSources(t, map[string]string{
		"local-app": srcinfo("local-app", []string{"aur-dep"}, nil, "local-app"),
	})
	plan, err := testApplication(t, aur, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", Packages: []string{"aur-root"}, LocalSources: local, PacmanConf: "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.BuildOrder, []string{"aur-dep", "local-app", "aur-root"}) {
		t.Fatalf("build order = %v", plan.BuildOrder)
	}
	if !slices.Equal(plan.Install, []string{"aur-root", "local-app"}) {
		t.Fatalf("install = %v", plan.Install)
	}
}

func TestPlanUsesVersionedLocalProvideBeforeRepository(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"compiler": srcinfo("compiler", nil, []string{"virtual-cc=2"}, "compiler"),
		"consumer": srcinfo("consumer", []string{"virtual-cc>=2"}, nil, "consumer"),
	})
	repositoryPackage := pacmanpkg.NewBinaryPackage("repo-cc.pkg.tar.zst", &raiou.PKGINFO{
		PkgName: "repo-cc", PkgVer: "9-1", Arch: "x86_64", Provides: []string{"virtual-cc=9"},
	})
	plan, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, []repositorySnapshot{{
		name: "core", packages: []*pacmanpkg.BinaryPackage{repositoryPackage},
	}}).Plan(context.Background(), PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.BuildOrder, []string{"compiler", "consumer"}) {
		t.Fatalf("build order = %v", plan.BuildOrder)
	}
}

func TestPlanRejectsUnversionedProvideForVersionedDependency(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"provider": srcinfo("provider", nil, []string{"virtual-api"}, "provider"),
		"consumer": srcinfo("consumer", []string{"virtual-api>=2"}, nil, "consumer"),
	})
	_, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: local, PacmanConf: "ignored",
	})
	if err == nil || !strings.Contains(err.Error(), "virtual-api>=2") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanReportsDependencyCycle(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"a": srcinfo("a", []string{"b"}, nil, "a"),
		"b": srcinfo("b", []string{"a"}, nil, "b"),
	})
	_, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: local, PacmanConf: "ignored",
	})
	if err == nil || !strings.Contains(err.Error(), "a -> b -> a") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanBuildsRequestedAURSplitPackageBaseOnce(t *testing.T) {
	aur := &fakeAUR{
		packages: map[string]aurweb.Pkg{
			"suite-cli":  {Name: "suite-cli", PackageBase: "suite", Version: "1-1"},
			"suite-libs": {Name: "suite-libs", PackageBase: "suite", Version: "1-1"},
		},
		provides: map[string][]string{},
		sources: map[string]string{
			"suite": srcinfo("suite", nil, nil, "suite-cli", "suite-libs"),
		},
	}
	plan, err := testApplication(t, aur, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", Packages: []string{"suite-cli", "suite-libs"}, PacmanConf: "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.BuildOrder, []string{"suite"}) || len(plan.Builds) != 1 {
		t.Fatalf("builds = %+v", plan.Builds)
	}
	if !slices.Equal(plan.Install, []string{"suite-cli", "suite-libs"}) {
		t.Fatalf("install = %v", plan.Install)
	}
	if !slices.Equal(plan.Builds[0].Explicit, []string{"suite-cli", "suite-libs"}) {
		t.Fatalf("explicit = %v", plan.Builds[0].Explicit)
	}
}

func TestPlanRejectsAmbiguousAURProviders(t *testing.T) {
	aur := &fakeAUR{
		packages: map[string]aurweb.Pkg{
			"provider-a": {Name: "provider-a", PackageBase: "provider-a", Version: "1-1", Provides: []string{"virtual-api=1"}},
			"provider-b": {Name: "provider-b", PackageBase: "provider-b", Version: "1-1", Provides: []string{"virtual-api=1"}},
		},
		provides: map[string][]string{"virtual-api": {"provider-b", "provider-a"}},
		sources:  map[string]string{},
	}
	local := writeLocalSources(t, map[string]string{
		"consumer": srcinfo("consumer", []string{"virtual-api>=1"}, nil, "consumer"),
	})
	_, err := testApplication(t, aur, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: local, PacmanConf: "ignored",
	})
	if err == nil || !strings.Contains(err.Error(), "provider-a, provider-b") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanUsesFirstConfiguredRepository(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"consumer": srcinfo("consumer", []string{"virtual-api"}, nil, "consumer"),
	})
	first := pacmanpkg.NewBinaryPackage("preferred.pkg.tar.zst", &raiou.PKGINFO{
		PkgName: "preferred", PkgVer: "1-1", Arch: "x86_64", Provides: []string{"virtual-api"},
	})
	second := pacmanpkg.NewBinaryPackage("fallback.pkg.tar.zst", &raiou.PKGINFO{
		PkgName: "fallback", PkgVer: "1-1", Arch: "x86_64", Provides: []string{"virtual-api"},
	})
	plan, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, []repositorySnapshot{
		{name: "profile", packages: []*pacmanpkg.BinaryPackage{first}},
		{name: "core", packages: []*pacmanpkg.BinaryPackage{second}},
	}).Plan(context.Background(), PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"})
	if err != nil {
		t.Fatal(err)
	}
	dependency := plan.Builds[0].Dependencies[0]
	if dependency.Provider != "preferred" || dependency.Repository != "profile" {
		t.Fatalf("dependency = %+v", dependency)
	}
}

func TestPlanRejectsUnsupportedLocalArchitecture(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"x86-only": srcinfo("x86-only", nil, nil, "x86-only"),
	})
	_, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "i686", LocalSources: local, PacmanConf: "ignored",
	})
	if err == nil || !strings.Contains(err.Error(), "does not support architecture i686") {
		t.Fatalf("error = %v", err)
	}
}

func TestPlanAcceptsSameDirectoryNameForDifferentPkgbases(t *testing.T) {
	first := writeLocalSources(t, map[string]string{"duplicate": srcinfo("first", nil, nil, "first")})
	second := writeLocalSources(t, map[string]string{"duplicate": srcinfo("second", nil, nil, "second")})
	plan, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: append(first, second...), PacmanConf: "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(plan.BuildOrder, []string{"first", "second"}) {
		t.Fatalf("build order = %v", plan.BuildOrder)
	}
}

func TestPlanRejectsDuplicateLocalPkgbase(t *testing.T) {
	first := writeLocalSources(t, map[string]string{"one": srcinfo("duplicate", nil, nil, "duplicate")})
	second := writeLocalSources(t, map[string]string{"two": srcinfo("duplicate", nil, nil, "duplicate")})
	_, err := testApplication(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil).Plan(context.Background(), PlanRequest{
		Arch: "x86_64", LocalSources: append(first, second...), PacmanConf: "ignored",
	})
	if err == nil || !strings.Contains(err.Error(), "defined by both") {
		t.Fatalf("error = %v", err)
	}
}
