package service

import (
	"testing"

	"github.com/Hayao0819/Kamisato/internal/errors"

	"github.com/Hayao0819/Kamisato/internal/conf"
	"github.com/Hayao0819/Kamisato/miko/domain"
)

func TestSubmitValidatesMicroarch(t *testing.T) {
	s := New(&conf.MikoConfig{}, nil, nil, nil)

	// A feature level on a non-x86_64 arch is rejected.
	if _, err := s.Submit(&domain.BuildRequest{Arch: "aarch64", Microarch: "x86_64_v3"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("microarch on aarch64: want ErrInvalidRequest, got %v", err)
	}
	// An unknown tier is rejected rather than silently built at the baseline.
	if _, err := s.Submit(&domain.BuildRequest{Arch: "x86_64", Microarch: "x86_64_v9"}); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("unknown tier: want ErrInvalidRequest, got %v", err)
	}
	// A valid tier on x86_64 is accepted.
	if _, err := s.Submit(&domain.BuildRequest{Arch: "x86_64", Microarch: "x86_64_v3", Pkgbuild: "pkgname=foo"}); err != nil {
		t.Errorf("valid microarch: want nil, got %v", err)
	}
}
