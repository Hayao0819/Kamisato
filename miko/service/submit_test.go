package service

import (
	"errors"
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
