package factory

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func TestNew(t *testing.T) {
	for _, tt := range []struct {
		config builder.ResolvedConfig
		name   string
	}{
		{builder.ResolvedConfig{
			Backend:  builder.KindChroot,
			Devtools: builder.DevtoolsConfig{ArchBuild: "extra-x86_64-build"},
		}, "chroot"},
		{builder.ResolvedConfig{
			Backend: builder.KindContainer,
			Timeout: 30 * time.Minute,
			Docker:  builder.DockerConfig{Image: "archlinux:latest"},
		}, "container"},
		{builder.ResolvedConfig{
			Backend: builder.KindBwrap,
			Bwrap:   builder.BwrapConfig{Rootfs: "/rootfs"},
		}, "bwrap"},
	} {
		backend, err := New(tt.config)
		if err != nil {
			t.Fatalf("New(%q): %v", tt.config.Backend, err)
		}
		if got := backend.Name(); got != tt.name {
			t.Errorf("New(%q).Name() = %q, want %q", tt.config.Backend, got, tt.name)
		}
	}

	if _, err := New(builder.ResolvedConfig{Backend: "unknown"}); err == nil {
		t.Fatal("New(unknown): want error")
	}
	if _, err := New(builder.ResolvedConfig{Backend: builder.KindBwrap}); err == nil {
		t.Fatal("New(unvalidated bwrap): want error")
	}
}
