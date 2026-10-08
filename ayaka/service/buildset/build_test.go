package buildset

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/filesystem/safefile"
	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	pacmanpkg "github.com/Hayao0819/Kamisato/internal/pacman/pkg"
	"github.com/Hayao0819/Kamisato/internal/pacman/repo"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
)

type fakeBackend struct {
	mutateVersion  bool
	symlinkPackage bool
}

func (f fakeBackend) Name() string { return "fake-container" }

func (f fakeBackend) Build(_ context.Context, spec builder.Spec) (*builder.Result, error) {
	source, err := pacmanpkg.OpenSourcePackage(spec.SrcDir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, name := range source.OutputNames(spec.Arch) {
		version := source.Version()
		if f.mutateVersion {
			version = "9-9"
		}
		path := filepath.Join(spec.OutDir, fmt.Sprintf("%s-%s-%s.pkg.tar", name, version, spec.Arch))
		writePath := path
		if f.symlinkPackage {
			writePath = filepath.Join(spec.OutDir, name+".target")
		}
		if err := writeTestPackage(writePath, name, version, spec.Arch); err != nil {
			return nil, err
		}
		if f.symlinkPackage {
			if err := os.Symlink(writePath, path); err != nil {
				return nil, err
			}
		}
		paths = append(paths, path)
	}
	return &builder.Result{Packages: paths}, nil
}

func writeTestPackage(path, name, version, arch string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	tw := tar.NewWriter(file)
	write := func(name string, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	pkginfo := []byte(fmt.Sprintf("pkgname = %s\npkgver = %s\narch = %s\nbuilddate = 1\nsize = 1\n", name, version, arch))
	if err := write(".PKGINFO", pkginfo); err != nil {
		_ = file.Close()
		return err
	}
	if err := write(".BUILDINFO", []byte("format = 2\npkgname = "+name+"\n")); err != nil {
		_ = file.Close()
		return err
	}
	if err := tw.Close(); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func TestBuildPublishesRepositoryAndManifest(t *testing.T) {
	local := writeLocalSources(t, map[string]string{
		"split": srcinfo("split", nil, nil, "split-cli", "split-libs"),
	})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = fakeBackend{}
	service.repoTool = repo.NativeTool{}
	output := filepath.Join(t.TempDir(), "repository")
	manifestPath := filepath.Join(t.TempDir(), "result.json")
	manifest, err := service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "alteriso-local",
		OutputDir:   output,
		Manifest:    manifestPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if manifest == nil || len(manifest.Builds) != 1 || len(manifest.Builds[0].Packages) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, name := range []string{"alteriso-local.db", "alteriso-local.files", "alteriso-local.db.tar.gz", "alteriso-local.files.tar.gz"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Errorf("repository artifact %s: %v", name, err)
		}
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Manifest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != SchemaVersion || decoded.Repository.Path != output {
		t.Fatalf("decoded manifest = %+v", decoded)
	}
	if !bytes.Contains(data, []byte(`"build_environment"`)) || bytes.Contains(data, []byte(`"Image"`)) {
		t.Fatalf("build environment JSON fields do not follow schema 1: %s", data)
	}
	for _, built := range decoded.Builds[0].Packages {
		if _, err := os.Stat(built.BuildInfo); err != nil {
			t.Errorf("build info %s: %v", built.BuildInfo, err)
		}
	}
}

func TestBuildRejectsPackageMetadataMismatchWithoutPublishing(t *testing.T) {
	local := writeLocalSources(t, map[string]string{"pkg": srcinfo("pkg", nil, nil, "pkg")})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = fakeBackend{mutateVersion: true}
	output := filepath.Join(t.TempDir(), "repository")
	manifestPath := filepath.Join(t.TempDir(), "result.json")
	_, err := service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local", OutputDir: output, Manifest: manifestPath,
	})
	if err == nil {
		t.Fatal("metadata mismatch succeeded")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("partial repository remains: %v", err)
	}
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Fatalf("partial manifest remains: %v", err)
	}
}

func TestBuildWithNoInputsCreatesNothing(t *testing.T) {
	output := filepath.Join(t.TempDir(), "repository")
	manifestPath := filepath.Join(t.TempDir(), "result.json")
	manifest, err := New(Dependencies{}).Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64"}, RepoName: "local", OutputDir: output, Manifest: manifestPath,
	})
	if err != nil || manifest != nil {
		t.Fatalf("manifest=%v error=%v", manifest, err)
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("output exists: %v", err)
	}
}

func TestBuildRejectsExistingManifestBeforeBuilding(t *testing.T) {
	local := writeLocalSources(t, map[string]string{"pkg": srcinfo("pkg", nil, nil, "pkg")})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = fakeBackend{}
	output := filepath.Join(t.TempDir(), "repository")
	manifestPath := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(manifestPath, []byte("old manifest"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local", OutputDir: output, Manifest: manifestPath,
	})
	if err == nil {
		t.Fatal("existing manifest was overwritten")
	}
	data, readErr := os.ReadFile(manifestPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "old manifest" {
		t.Fatalf("existing manifest changed: %q", data)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("repository output exists: %v", statErr)
	}
}

func TestBuildRejectsPackageArtifactSymlink(t *testing.T) {
	local := writeLocalSources(t, map[string]string{"pkg": srcinfo("pkg", nil, nil, "pkg")})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = fakeBackend{symlinkPackage: true}
	output := filepath.Join(t.TempDir(), "repository")
	_, err := service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local", OutputDir: output, Manifest: filepath.Join(t.TempDir(), "result.json"),
	})
	if err == nil {
		t.Fatal("package artifact symlink was accepted")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("repository output exists: %v", statErr)
	}
}

func TestBuildRejectsManifestLockContention(t *testing.T) {
	local := writeLocalSources(t, map[string]string{"pkg": srcinfo("pkg", nil, nil, "pkg")})
	service := testService(t, &fakeAUR{packages: map[string]aurweb.Pkg{}, provides: map[string][]string{}, sources: map[string]string{}}, nil)
	service.backend = fakeBackend{}
	manifestPath := filepath.Join(t.TempDir(), "result.json")
	lock, err := safefile.TryLock(manifestPath+".lock", 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			t.Errorf("unlock manifest: %v", err)
		}
	}()
	output := filepath.Join(t.TempDir(), "repository")
	_, err = service.Build(context.Background(), BuildRequest{
		PlanRequest: PlanRequest{Arch: "x86_64", LocalSources: local, PacmanConf: "ignored"},
		RepoName:    "local", OutputDir: output, Manifest: manifestPath,
	})
	if err == nil {
		t.Fatal("contended manifest lock was accepted")
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("repository output exists: %v", statErr)
	}
}
