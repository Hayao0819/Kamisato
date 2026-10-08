package build

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
	"github.com/Hayao0819/Kamisato/internal/pacman/pkg"
)

type backendFunc func(context.Context, builder.Spec) (*builder.Result, error)

func (f backendFunc) Name() string { return "test" }
func (f backendFunc) Build(ctx context.Context, spec builder.Spec) (*builder.Result, error) {
	return f(ctx, spec)
}

func TestPackageUsesInjectedBackendSignerAndPublisher(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD"), []byte("pkgname=test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte("pkgbase = test\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = any\n\npkgname = test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	metadata, err := pkg.OpenSourcePackage(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls []string
	var output bytes.Buffer
	target := &Target{
		Arch:   "x86_64",
		Output: &output,
		Backend: backendFunc(func(actual context.Context, spec builder.Spec) (*builder.Result, error) {
			if actual != ctx {
				t.Fatal("command context was replaced")
			}
			if spec.Arch != "x86_64" || spec.SrcDir == dir {
				t.Fatalf("spec = %+v", spec)
			}
			if spec.LogWriter != &output {
				t.Fatal("command output was replaced")
			}
			calls = append(calls, "build")
			return &builder.Result{Packages: []string{"test.pkg.tar.zst"}}, nil
		}),
		Sign:    func(file string) error { calls = append(calls, "sign:"+file); return nil },
		Publish: func(files []string) error { calls = append(calls, "publish:"+files[0]); return nil },
	}
	if err := Package(ctx, metadata, target, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"build", "sign:test.pkg.tar.zst", "publish:test.pkg.tar.zst"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	target.Sign = func(string) error { return errors.New("signing failed") }
	target.Publish = func([]string) error { t.Fatal("published an unsigned result"); return nil }
	if err := Package(ctx, metadata, target, t.TempDir()); err == nil {
		t.Fatal("signing error was ignored")
	}
	target.Backend = backendFunc(func(context.Context, builder.Spec) (*builder.Result, error) {
		cancel()
		return &builder.Result{Packages: []string{"test.pkg.tar.zst"}}, nil
	})
	target.Sign = func(string) error { t.Fatal("canceled build reached signing"); return nil }
	if err := Package(ctx, metadata, target, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled build = %v, want cancellation before signing or publishing", err)
	}
}

func TestPackageRejectsMissingBackend(t *testing.T) {
	if err := Package(context.Background(), nil, &Target{}, ""); err == nil {
		t.Fatal("missing backend was accepted")
	}
}
