package repo

import (
	"reflect"
	"testing"
)

func TestDatabaseArtifacts(t *testing.T) {
	t.Parallel()
	artifacts := Artifacts("core")
	if got, want := artifacts.Archives(), []string{"core.db.tar.gz", "core.files.tar.gz"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Archives = %v, want %v", got, want)
	}
	if got, want := artifacts.AliasSignatures(), []string{"core.db.sig", "core.files.sig"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AliasSignatures = %v, want %v", got, want)
	}

	merged := artifacts.WithArchivePrefix("core.merged")
	tests := map[string]string{
		"core.db":        "core.merged.db.tar.gz",
		"core.files":     "core.merged.files.tar.gz",
		"core.db.sig":    "core.merged.db.tar.gz.sig",
		"core.files.sig": "core.merged.files.tar.gz.sig",
	}
	for alias, want := range tests {
		got, ok := merged.ArchiveForAlias(alias)
		if !ok || got != want {
			t.Errorf("ArchiveForAlias(%q) = %q, %t; want %q, true", alias, got, ok, want)
		}
	}
	if _, ok := merged.ArchiveForAlias("core.db.tar.gz"); ok {
		t.Error("archive name must not be classified as an alias")
	}
}
