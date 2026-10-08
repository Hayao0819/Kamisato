package build

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	miko "github.com/Hayao0819/Kamisato/miko/client"
	thomaconfig "github.com/Hayao0819/Kamisato/thoma/config"
)

type submitClient func(context.Context, *miko.BuildRequest) (string, error)

func (f submitClient) SubmitBuild(ctx context.Context, request *miko.BuildRequest) (string, error) {
	return f(ctx, request)
}
func (submitClient) WaitJob(context.Context, string, io.Writer) (*miko.Job, error) {
	panic("job must not be awaited after failed submit")
}
func (submitClient) DownloadPackageFile(context.Context, string, string, string, string) error {
	panic("failed job must not download")
}
func (submitClient) DownloadArtifact(context.Context, string, string, io.Writer) error {
	panic("failed job must not download")
}

func TestRunUsesExplicitClientConfigAndContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "PKGBUILD.custom"), []byte("pkgname=example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Neither environment nor credentials may select a different client inside
	// the workflow. The adapter has already selected it.
	t.Setenv("THOMA_MODE", "invalid-mode")
	t.Setenv("THOMA_SERVER", "https://must-not-be-contacted.invalid")
	cfg := &thomaconfig.ThomaConfig{Mode: thomaconfig.ThomaModeDirect, Repo: "aur", Arch: "x86_64", Server: "injected", Timeout: 12}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	client := submitClient(func(got context.Context, req *miko.BuildRequest) (string, error) {
		called = true
		if got != ctx {
			t.Fatal("workflow replaced caller context")
		}
		if req.Repo != "aur" || req.Arch != "x86_64" || req.Timeout != 12 || req.SignMode != "client" || req.Pkgbuild != "pkgname=example\n" || !req.IgnoreArch {
			t.Fatalf("request = %+v", req)
		}
		return "", got.Err()
	})
	var stdout, stderr bytes.Buffer
	err := Run(ctx, cfg, client, &stdout, &stderr, Options{Dir: dir, Buildscript: "PKGBUILD.custom", IgnoreArch: true})
	if !called || !errors.Is(err, context.Canceled) {
		t.Fatalf("called=%v, error=%v", called, err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("diagnostic leaked to stdout: %q", stdout.String())
	}
}
