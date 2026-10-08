package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

type injectedBackend struct {
	spec builder.Spec
}

func (*injectedBackend) Name() string { return "test" }
func (b *injectedBackend) Build(_ context.Context, spec builder.Spec) (*builder.Result, error) {
	b.spec = spec
	return &builder.Result{Packages: []string{filepath.Join(spec.OutDir, "test.pkg.tar.zst")}}, nil
}

func TestRunBuildUsesInjectedBackendAfterResolvingTrustedSettings(t *testing.T) {
	backend := &injectedBackend{}
	var resolved builder.ResolvedConfig
	service := New(Settings{Builder: builder.HostConfig{Timeout: 3 * time.Minute}}, WithBuildBackend(func(config builder.ResolvedConfig) (builder.Backend, error) {
		resolved = config
		return backend, nil
	}))
	job := &domain.BuildJob{Request: &domain.BuildRequest{Arch: "x86_64", Pkgbuild: "pkgname=test\npkgver=1\npkgrel=1\n", Timeout: 2}}
	result, dir, err := service.runBuild(context.Background(), job)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if resolved.Timeout != 2*time.Minute || resolved.Backend != builder.KindContainer {
		t.Fatalf("resolved config = %+v", resolved)
	}
	if backend.spec.Arch != "x86_64" || backend.spec.OutDir != dir || len(result.Packages) != 1 {
		t.Fatalf("backend spec = %+v; result = %+v", backend.spec, result)
	}
	if _, err := os.Stat(backend.spec.SrcDir); !os.IsNotExist(err) {
		t.Fatalf("temporary source was not cleaned: %v", err)
	}
}

func TestNewDoesNotInitializeSonameStorage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	service := New(Settings{DataDir: dir, SonameRebuild: true})
	if service.sonames != nil {
		t.Fatal("constructor opened its own soname store")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("constructor modified filesystem: %v", err)
	}
	store, err := NewFileSonameStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	service = New(Settings{DataDir: dir}, WithSonameStore(store))
	if service.sonames != store {
		t.Fatal("constructor did not retain injected soname store")
	}
}
