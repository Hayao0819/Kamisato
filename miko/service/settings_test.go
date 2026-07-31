package service

import (
	"testing"
	"time"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestSettingsDefaults(t *testing.T) {
	service := New(Settings{MaxLogBytes: -1})

	if service.settings.Builder.Backend != builder.KindContainer {
		t.Fatalf("builder backend = %q, want %q", service.settings.Builder.Backend, builder.KindContainer)
	}
	if service.settings.Workers != 1 {
		t.Fatalf("workers = %d, want 1", service.settings.Workers)
	}
	if service.settings.RetryBackoff != 5*time.Second {
		t.Fatalf("retry backoff = %s, want 5s", service.settings.RetryBackoff)
	}
	if service.settings.MaxLogBytes != 16<<20 {
		t.Fatalf("max log bytes = %d, want %d", service.settings.MaxLogBytes, 16<<20)
	}
}
