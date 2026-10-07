package build

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/api/ayato"
	"github.com/Hayao0819/Kamisato/internal/api/miko"
	thomaconfig "github.com/Hayao0819/Kamisato/thoma/config"
)

func testBuildClient(t *testing.T, cfg *thomaconfig.ThomaConfig, base string) *miko.Client {
	t.Helper()
	if cfg.Direct() {
		miko, err := miko.New(base, cfg.ApiKey)
		if err != nil {
			t.Fatal(err)
		}
		return miko
	}
	ayato, err := ayato.New(base, ayato.StaticBearer(cfg.ApiKey))
	if err != nil {
		t.Fatal(err)
	}
	return ayato.Client
}

// TestPlacePackages drives the pkgname-match + download loop against a stand-in
// server for both modes, covering the drift and no-match edge cases.
func TestPlacePackages(t *testing.T) {
	cases := []struct {
		name     string
		mode     string
		destName string   // the local package filename yay expects
		built    []string // what the builder reports it produced
		wantPath string   // request path the server should receive
		wantKey  string
		wantErr  string // substring of the expected error, or "" for success
	}{
		{
			name:     "ayato exact match hits the repo route",
			mode:     thomaconfig.ThomaModeAyato,
			destName: "foo-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantPath: "/repo/aur/x86_64/foo-1.0-1-x86_64.pkg.tar.zst",
		},
		{
			// A VCS package's pkgver bumped between the local packagelist and the
			// build; matching by pkgname must still place the freshly built file.
			name:     "ayato matches by pkgname across pkgver drift",
			mode:     thomaconfig.ThomaModeAyato,
			destName: "foo-git-r100.aaaaaa-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-git-r105.bbbbbb-1-x86_64.pkg.tar.zst"},
			wantPath: "/repo/aur/x86_64/foo-git-r105.bbbbbb-1-x86_64.pkg.tar.zst",
		},
		{
			name:     "direct pulls the artifact off the job",
			mode:     thomaconfig.ThomaModeDirect,
			destName: "foo-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantPath: "/api/unstable/jobs/job-123/artifacts/foo-1.0-1-x86_64.pkg.tar.zst",
			wantKey:  "k",
		},
		{
			name:     "no built package matches the wanted dest",
			mode:     thomaconfig.ThomaModeAyato,
			destName: "bar-1.0-1-x86_64.pkg.tar.zst",
			built:    []string{"foo-1.0-1-x86_64.pkg.tar.zst"},
			wantErr:  "no built package matches",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotAPIKey, gotBearer string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAPIKey = r.Header.Get("X-API-Key")
				gotBearer = r.Header.Get("Authorization")
				_, _ = w.Write([]byte("PKGDATA:" + filepath.Base(r.URL.Path)))
			}))
			defer srv.Close()

			cfg := &thomaconfig.ThomaConfig{Repo: "aur", Arch: "x86_64", Mode: tc.mode}
			if tc.mode == thomaconfig.ThomaModeDirect {
				cfg.ApiKey = "k"
			}
			dest := filepath.Join(t.TempDir(), tc.destName)

			buildAPI := testBuildClient(t, cfg, srv.URL)
			err := placePackages(context.Background(), os.Stderr, cfg, buildAPI, "job-123", []string{dest}, tc.built)
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
			if gotAPIKey != tc.wantKey || gotBearer != "" {
				t.Errorf("credentials = X-API-Key %q, Authorization %q", gotAPIKey, gotBearer)
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

func TestPkgName(t *testing.T) {
	cases := map[string]string{
		"foo-1.0-1-x86_64.pkg.tar.zst":             "foo",
		"foo-bar-1.0-1-x86_64.pkg.tar.zst":         "foo-bar",
		"python-foo-1.2.3-4-any.pkg.tar.xz":        "python-foo",
		"foo-2:1.0-1-any.pkg.tar.zst":              "foo",
		"foo-git-r123.abcdef-1-x86_64.pkg.tar.zst": "foo-git",
	}
	for in, want := range cases {
		if got := pkgName(in); got != want {
			t.Errorf("pkgName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPackageDestsForwardsBuildscript(t *testing.T) {
	dir := t.TempDir()
	makepkg := filepath.Join(dir, "makepkg")
	argsFile := filepath.Join(dir, "args")
	dirFile := filepath.Join(dir, "dir")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_FILE\"\npwd > \"$DIR_FILE\"\nprintf '/tmp/foo-1.0-1-x86_64.pkg.tar.zst\\n'\n"
	if err := os.WriteFile(makepkg, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ARGS_FILE", argsFile)
	t.Setenv("DIR_FILE", dirFile)
	dests, err := packageDests(context.Background(), makepkg, dir, "/etc/makepkg.conf", "PKGBUILD.custom")
	if err != nil {
		t.Fatal(err)
	}
	if len(dests) != 1 || dests[0] != "/tmp/foo-1.0-1-x86_64.pkg.tar.zst" {
		t.Fatalf("destinations = %#v", dests)
	}
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	want := "--packagelist\n--ignorearch\n--config\n/etc/makepkg.conf\n-p\nPKGBUILD.custom\n"
	if string(args) != want {
		t.Fatalf("makepkg args = %q, want %q", args, want)
	}
	workingDirectory, err := os.ReadFile(dirFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(workingDirectory)) != dir {
		t.Fatalf("makepkg directory = %q, want %q", strings.TrimSpace(string(workingDirectory)), dir)
	}
}
