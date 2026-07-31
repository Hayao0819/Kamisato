package service

import "testing"

func TestSettingsDefaults(t *testing.T) {
	service := New(nil, nil, nil, nil, Settings{})

	if service.catalog == nil {
		t.Fatal("catalog is nil")
	}
	if service.settings.ExpectedBuildDir != "/build" {
		t.Fatalf("expected build dir = %q, want /build", service.settings.ExpectedBuildDir)
	}
}
