package service

import (
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/miko/domain"
)

func TestSubmitRejectsBadArch(t *testing.T) {
	s := New(Settings{})

	if _, err := s.Submit(&domain.BuildRequest{Arch: "evil; rm -rf"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("bad arch: want ErrInvalidRequest, got %v", err)
	}
	if _, err := s.Submit(&domain.BuildRequest{Arch: ""}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("empty arch: want ErrInvalidRequest, got %v", err)
	}
	if _, err := s.Submit(nil); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("nil request: want ErrInvalidRequest, got %v", err)
	}
}

func TestSubmittedRequestAndSnapshotsDoNotShareServiceState(t *testing.T) {
	s := New(Settings{})
	request := &domain.BuildRequest{Arch: "x86_64", Pkgbuild: "pkgname=original", Files: map[string]string{"source": "original"}}
	id, err := s.Submit(request)
	if err != nil {
		t.Fatal(err)
	}
	request.Pkgbuild = "pkgname=changed"
	request.Files["source"] = "changed"
	job, err := s.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.Request.Pkgbuild != "pkgname=original" || job.Request.Files["source"] != "original" {
		t.Fatal("service retained the caller's mutable request")
	}
	job.Request.Files["source"] = "status-changed"
	list := s.List()
	list[0].Request.Files["source"] = "list-changed"
	latest, err := s.Status(id)
	if err != nil {
		t.Fatal(err)
	}
	if latest.Request.Files["source"] != "original" {
		t.Fatal("status or list snapshot mutated service state")
	}
}

func TestSubmitRejectsUnsafeRepoName(t *testing.T) {
	s := New(Settings{})
	for _, repo := range []string{".", "..", "../repo", "repo/testing", "repo\n[evil]"} {
		req := &domain.BuildRequest{Arch: "x86_64", Repo: repo}
		if _, err := s.Submit(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("repo %q: want ErrInvalidRequest, got %v", repo, err)
		}
	}
}

func TestSubmitRejectsInstallPkgsEscape(t *testing.T) {
	// No staging dir: any install_pkgs entry is rejected.
	s := New(Settings{})
	req := &domain.BuildRequest{Arch: "x86_64", InstallPkgs: []string{"/etc/passwd"}}
	if _, err := s.Submit(req); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("no staging dir: want ErrInvalidRequest, got %v", err)
	}

	dataDir := t.TempDir()
	staging := filepath.Join(dataDir, "staging")
	s = New(Settings{DataDir: dataDir})

	for _, p := range []string{"/etc/passwd", filepath.Join(staging, "..", "keys", "secret.gpg")} {
		req := &domain.BuildRequest{Arch: "x86_64", InstallPkgs: []string{p}}
		if _, err := s.Submit(req); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("escaping path %q: want ErrInvalidRequest, got %v", p, err)
		}
	}

	// A path inside the staging dir is accepted.
	req = &domain.BuildRequest{Arch: "x86_64", InstallPkgs: []string{filepath.Join(staging, "dep.pkg.tar.zst")}}
	if _, err := s.Submit(req); err != nil {
		t.Errorf("staged path: want nil, got %v", err)
	}
}
