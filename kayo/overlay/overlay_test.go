package overlay

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
)

// makeOverlayRepo builds a local git repo exporting one package `name` at
// `version`, usable directly as an overlay URL (a local-path clone).
func makeOverlayRepo(t *testing.T, name, version string) string {
	t.Helper()
	dir := t.TempDir()
	srcinfo := "pkgbase = " + name + "\n\tpkgver = " + version + "\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = " + name + "\n"
	write := func(p, body string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("PKGBUILD", "pkgname="+name+"\npkgver="+version+"\n")
	write(".SRCINFO", srcinfo)
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "--quiet", "-m", "v"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	return dir
}

func syncedRegistry(t *testing.T, overlays []kayoconfig.OverlayConfig) *Registry {
	t.Helper()
	r := New(t.TempDir(), overlays)
	if err := r.Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	return r
}

// TestOverlayPriorityShadowing asserts a lower-priority overlay can never shadow a
// higher-priority one's package — the winner is decided by priority, not the order
// overlays appear in config. A break here would let a less-trusted overlay silently
// override a trusted package under the same name.
func TestOverlayPriorityShadowing(t *testing.T) {
	low := makeOverlayRepo(t, "shared", "1")
	high := makeOverlayRepo(t, "shared", "2")
	// A lower-priority version must not leak a removed split package back into
	// the index or replace the selected recipe's URL/checkout through that name.
	srcinfo := "pkgbase = shared\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = x86_64\n\npkgname = shared\n\npkgname = removed-split\n"
	if err := os.WriteFile(filepath.Join(low, ".SRCINFO"), []byte(srcinfo), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", ".SRCINFO"}, {"commit", "--quiet", "-m", "old split package"}} {
		command := exec.Command("git", args...)
		command.Dir = low
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}

	cases := []struct {
		name     string
		overlays []kayoconfig.OverlayConfig
	}{
		{"low-then-high", []kayoconfig.OverlayConfig{
			{Name: "low", URL: low, Priority: 1},
			{Name: "high", URL: high, Priority: 10},
		}},
		{"high-then-low", []kayoconfig.OverlayConfig{
			{Name: "high", URL: high, Priority: 10},
			{Name: "low", URL: low, Priority: 1},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := syncedRegistry(t, tc.overlays)
			info, _ := r.Info(context.Background(), []string{"shared"})
			if len(info) != 1 || info[0].Version != "2-1" {
				t.Fatalf("higher-priority overlay must win regardless of order: %+v", info)
			}
			if url, ok, err := r.SourceURL(context.Background(), "shared"); err != nil || !ok || url != high {
				t.Fatalf("metadata and source differ: URL=%q, found=%v, error=%v", url, ok, err)
			}
			if dir := r.SourceDirs()["shared"]; filepath.Base(dir) != "high" {
				t.Fatalf("metadata and pin checkout differ: %q", dir)
			}
			if old, err := r.Info(context.Background(), []string{"removed-split"}); err != nil || len(old) != 0 {
				t.Fatalf("shadowed recipe leaked an old split package: %+v, error=%v", old, err)
			}
		})
	}
}

// TestSourceDirs asserts a synced overlay exposes its pkgbase -> checkout mapping,
// which the daemon needs to materialize an approved pin from the overlay tree.
func TestSourceDirs(t *testing.T) {
	repo := makeOverlayRepo(t, "mypkg", "1")
	r := syncedRegistry(t, []kayoconfig.OverlayConfig{{Name: "ov", URL: repo}})

	dirs := r.SourceDirs()
	dir, ok := dirs["mypkg"]
	if !ok {
		t.Fatalf("SourceDirs missing pkgbase mypkg: %+v", dirs)
	}
	if filepath.Base(dir) != "ov" {
		t.Errorf("SourceDirs[mypkg] = %q, want the overlay checkout dir (…/ov)", dir)
	}
	if _, err := os.Stat(filepath.Join(dir, ".SRCINFO")); err != nil {
		t.Errorf("overlay checkout not present at %q: %v", dir, err)
	}
}

// TestOverlayEqualPriorityKeepsFirst pins the tie-break: at equal priority the
// first overlay wins and a later same-priority overlay does NOT override it, so
// adding an overlay can't silently displace an existing package by matching its
// priority.
func TestOverlayEqualPriorityKeepsFirst(t *testing.T) {
	first := makeOverlayRepo(t, "shared", "1")
	second := makeOverlayRepo(t, "shared", "2")

	r := syncedRegistry(t, []kayoconfig.OverlayConfig{
		{Name: "first", URL: first, Priority: 5},
		{Name: "second", URL: second, Priority: 5},
	})
	info, _ := r.Info(context.Background(), []string{"shared"})
	if len(info) != 1 || info[0].Version != "1-1" {
		t.Fatalf("equal priority must keep the first overlay, not the later one: %+v", info)
	}
	if url, ok, err := r.SourceURL(context.Background(), "shared"); err != nil || !ok || url != first {
		t.Fatalf("equal-priority source must also keep the first: URL=%q, found=%v, error=%v", url, ok, err)
	}
	if dir := r.SourceDirs()["shared"]; filepath.Base(dir) != "first" {
		t.Fatalf("equal-priority pin checkout must keep the first: %q", dir)
	}
}

func TestSyncRejectsUnsafeOverlayBeforeFilesystemEffects(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "cache")
	r := New(cache, []kayoconfig.OverlayConfig{{Name: "../other", URL: "unused"}})
	if err := r.Sync(context.Background()); err == nil {
		t.Fatal("Sync accepted an overlay outside its cache")
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Errorf("invalid input created the cache: %v", err)
	}
}

func TestNewOwnsOverlayConfiguration(t *testing.T) {
	configs := []kayoconfig.OverlayConfig{{Name: "safe", URL: "local"}}
	r := New(t.TempDir(), configs)
	configs[0].Name = "../other"
	if r.overlays[0].Name != "safe" {
		t.Fatal("Registry retains caller-owned mutable configuration")
	}
}
