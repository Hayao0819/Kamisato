package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestRepositoryCatalogResolvesTopologyAndPolicy(t *testing.T) {
	t.Parallel()

	catalog, err := NewRepositoryCatalog([]string{"x86_64"}, []RepositorySpec{
		{
			Name:                  "core",
			Tiered:                true,
			PromotionKeepInSource: true,
			AllowNewArch:          true,
			Upstream: UpstreamSpec{
				DBURL: "https://mirror.example/core/os/$arch/core.db",
			},
		},
		{Name: "extra", Arches: []string{"aarch64"}},
	})
	if err != nil {
		t.Fatalf("NewRepositoryCatalog: %v", err)
	}

	if got, want := catalog.PhysicalNames(), []string{"core-staging", "core-testing", "core", "extra"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("PhysicalNames() = %v, want %v", got, want)
	}
	resolved, ok := catalog.Resolve("core-testing")
	if !ok {
		t.Fatal("Resolve(core-testing) did not find tier")
	}
	if resolved.Repository.Name() != "core" || resolved.Tier != TierTesting || resolved.Physical != "core-testing" {
		t.Fatalf("Resolve(core-testing) = %+v", resolved)
	}
	if !resolved.Repository.PromotionKeepsSource() || !catalog.AllowsNewArch("core-testing") {
		t.Fatal("logical policy was not preserved across a physical tier")
	}
	if got := catalog.DeclaredArches("core-staging"); !reflect.DeepEqual(got, []string{"x86_64"}) {
		t.Fatalf("DeclaredArches(core-staging) = %v", got)
	}
	if got := catalog.DeclaredArches("extra"); !reflect.DeepEqual(got, []string{"aarch64"}) {
		t.Fatalf("DeclaredArches(extra) = %v", got)
	}
	if target, ok := catalog.PublishTarget("core"); !ok || target != "core-staging" {
		t.Fatalf("PublishTarget(core) = %q, %v", target, ok)
	}
	if target, ok := catalog.PublishTarget("core-testing"); !ok || target != "core-testing" {
		t.Fatalf("PublishTarget(core-testing) = %q, %v", target, ok)
	}
	if got := catalog.UpstreamPhysicalNames(); !reflect.DeepEqual(got, []string{"core-staging", "core-testing", "core"}) {
		t.Fatalf("UpstreamPhysicalNames() = %v", got)
	}
	upstream := resolved.Repository.Upstream()
	if got := upstream.DBURLFor("aarch64"); got != "https://mirror.example/core/os/aarch64/core.db" {
		t.Fatalf("DBURLFor = %q", got)
	}
	if got := upstream.FilesURLFor("aarch64"); got != "https://mirror.example/core/os/aarch64/core.files" {
		t.Fatalf("FilesURLFor = %q", got)
	}
}

func TestRepositoryCatalogRejectsAmbiguousOrUnsafeTopology(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		defaults []string
		specs    []RepositorySpec
		want     string
	}{
		{
			name:  "duplicate logical name",
			specs: []RepositorySpec{{Name: "core"}, {Name: "core"}},
			want:  "duplicated",
		},
		{
			name:  "tier physical collision",
			specs: []RepositorySpec{{Name: "core", Tiered: true}, {Name: "core-staging"}},
			want:  "collides",
		},
		{
			name:  "traversal name",
			specs: []RepositorySpec{{Name: ".."}},
			want:  "traversal",
		},
		{
			name:     "any default arch",
			defaults: []string{"any"},
			specs:    []RepositorySpec{{Name: "core"}},
			want:     "no pacman repository database",
		},
		{
			name:  "duplicate repo arch",
			specs: []RepositorySpec{{Name: "core", Arches: []string{"x86_64", "x86_64"}}},
			want:  "duplicated",
		},
		{
			name:  "files URL without DB URL",
			specs: []RepositorySpec{{Name: "core", Upstream: UpstreamSpec{FilesURL: "https://mirror.example/core.files"}}},
			want:  "requires db_url",
		},
		{
			name:  "relative upstream URL",
			specs: []RepositorySpec{{Name: "core", Upstream: UpstreamSpec{DBURL: "/core.db"}}},
			want:  "absolute http(s)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewRepositoryCatalog(tt.defaults, tt.specs)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("NewRepositoryCatalog() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestParseTier(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"staging", "testing", "stable"} {
		if _, err := ParseTier(value); err != nil {
			t.Errorf("ParseTier(%q) = %v", value, err)
		}
	}
	if _, err := ParseTier("preview"); err == nil {
		t.Fatal("ParseTier(preview) = nil error")
	}
	if !CanPromote(TierStaging, TierTesting) || !CanPromote(TierTesting, TierStable) {
		t.Fatal("valid promotion was rejected")
	}
	if CanPromote(TierStaging, TierStable) {
		t.Fatal("tier skip was accepted")
	}
}
