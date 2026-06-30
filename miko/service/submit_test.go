package service

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func TestSubmitRejectsBadArch(t *testing.T) {
	s := New(&conf.MikoConfig{}, nil)

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

func TestSubmitRejectsInstallPkgsEscape(t *testing.T) {
	// No staging dir: any install_pkgs entry is rejected.
	s := New(&conf.MikoConfig{}, nil)
	req := &domain.BuildRequest{Arch: "x86_64", InstallPkgs: []string{"/etc/passwd"}}
	if _, err := s.Submit(req); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("no staging dir: want ErrInvalidRequest, got %v", err)
	}

	dataDir := t.TempDir()
	staging := filepath.Join(dataDir, "staging")
	s = New(&conf.MikoConfig{DataDir: dataDir}, nil)

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
