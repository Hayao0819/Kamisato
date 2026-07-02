package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/conf"
)

// TestPlacePackages drives the pkgname-match + download loop against a stand-in
// server for both modes, covering the drift and no-match edge cases.
func TestPlacePackages(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		destName string   // the local package filename yay expects
		built    []string // what the builder reports it produced
		wantPath string   // request path the server should receive
		wantErr  string   // substring of the expected error, or "" for success
	}{
		{
			name:     "ayato exact match hits the repo route",
			mode:     conf.ThomaModeAyato,
			destName: "foo-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantPath: "/repo/aur/x86_64/foo-1.0-1-x86_64.pkg.tar.zst",
		},
		{
			// A VCS package's pkgver bumped between the local packagelist and the
			// build; matching by pkgname must still place the freshly built file.
			name:     "ayato matches by pkgname across pkgver drift",
			mode:     conf.ThomaModeAyato,
			destName: "foo-git-r100.aaaaaa-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-git-r105.bbbbbb-1-x86_64.pkg.tar.zst"},
			wantPath: "/repo/aur/x86_64/foo-git-r105.bbbbbb-1-x86_64.pkg.tar.zst",
		},
		{
			name:     "direct pulls the artifact off the job",
			mode:     conf.ThomaModeDirect,
			destName: "foo-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantPath: "/api/unstable/jobs/job-123/artifacts/foo-1.0-1-x86_64.pkg.tar.zst",
		},
		{
			name:     "no built package matches the wanted dest",
			mode:     conf.ThomaModeAyato,
			destName: "bar-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantErr:  "no built package matches",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				_, _ = w.Write([]byte("PKGDATA:" + filepath.Base(r.URL.Path)))
			}))
			defer srv.Close()

			cfg := &conf.ThomaConfig{Repo: "aur", Arch: "x86_64", Mode: tc.mode}
			if tc.mode == conf.ThomaModeDirect {
				cfg.ApiKey = "k"
			}
			dest := filepath.Join(t.TempDir(), tc.destName)

			err := placePackages(context.Background(), cfg, srv.URL, "job-123", []string{dest}, tc.built)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tc.wantErr)
				}
				if gotPath != "" {
					t.Errorf("server was hit at %q for a non-matching build", gotPath)
				}
				return
			}
			if err != nil {
				t.Fatalf("placePackages: %v", err)
			}
			if gotPath != tc.wantPath {
				t.Errorf("server path = %q, want %q", gotPath, tc.wantPath)
			}
			// The bytes land at the exact dest yay expects, even when the matched
			// build filename differs from that dest (pkgver drift).
			data, rerr := os.ReadFile(dest)
			if rerr != nil {
				t.Fatalf("reading placed file: %v", rerr)
			}
			if want := "PKGDATA:" + filepath.Base(tc.wantPath); string(data) != want {
				t.Errorf("placed body = %q, want %q", data, want)
			}
		})
	}
}
