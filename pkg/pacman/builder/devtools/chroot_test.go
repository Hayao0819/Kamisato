package devtools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/pacman/builder"
)

func writeFixture(t *testing.T, files map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	orig := devtoolsDataDir
	devtoolsDataDir = dir
	t.Cleanup(func() { devtoolsDataDir = orig })
}

func TestRenderChrootPacmanConf(t *testing.T) {
	writeFixture(t, map[string]string{
		"pacman.conf.d/extra.conf":      "# extra base\n[options]\n",
		"pacman.conf.d/alterlinux.conf": "# alterlinux base\n[options]\n",
	})

	got, err := renderChrootPacmanConf("alterlinux", []builder.PacmanRepository{{Name: "ayato", Server: "https://r/$repo/$arch"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# alterlinux base") {
		t.Errorf("missing alterlinux base:\n%s", got)
	}
	if !strings.Contains(got, "[ayato]") || !strings.Contains(got, "Server = https://r/$repo/$arch") {
		t.Errorf("repo stanza not appended:\n%s", got)
	}

	fb, err := renderChrootPacmanConf("nosuchrepo", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fb, "# extra base") {
		t.Errorf("fallback to extra.conf failed:\n%s", fb)
	}
}

func TestRenderChrootPacmanConfMissing(t *testing.T) {
	writeFixture(t, map[string]string{}) // no pacman.conf.d at all
	if _, err := renderChrootPacmanConf("extra", nil); err == nil {
		t.Fatal("want an error when neither the repo base nor extra.conf exists")
	} else if !strings.Contains(err.Error(), "devtools") {
		t.Errorf("error should mention the devtools package: %v", err)
	}
}

func TestRenderChrootMakepkgConf(t *testing.T) {
	writeFixture(t, map[string]string{
		"makepkg.conf.d/x86_64.conf": "# x86_64 base\nCARCH=x86_64\n",
	})

	got, err := renderChrootMakepkgConf("x86_64", builder.MakepkgConfig{Microarch: "x86_64_v3", Packager: "Foo <f@e>"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "# x86_64 base") {
		t.Errorf("missing makepkg base:\n%s", got)
	}
	if !strings.Contains(got, "-march=x86-64-v3") || !strings.Contains(got, "PACKAGER='Foo <f@e>'") {
		t.Errorf("override lines not appended:\n%s", got)
	}
	// The base is complete, so the file must not source /etc/makepkg.conf (that
	// would self-recurse once arch-nspawn copies it into the chroot).
	if strings.Contains(got, "source /etc/makepkg.conf") {
		t.Errorf("generated makepkg.conf must not source /etc/makepkg.conf:\n%s", got)
	}

	if _, err := renderChrootMakepkgConf("aarch64", builder.MakepkgConfig{}); err == nil {
		t.Error("want an error for a missing makepkg base")
	}
	if _, err := renderChrootMakepkgConf("x86_64", builder.MakepkgConfig{Microarch: "x86_64_v9"}); err == nil {
		t.Error("want an error for an unknown microarch tier")
	}
}

func TestRepoFromArchBuild(t *testing.T) {
	for in, want := range map[string]string{
		"extra-x86_64-build":      "extra",
		"alterlinux-x86_64-build": "alterlinux",
		"multilib-x86_64-build":   "multilib",
		"nodashes":                "nodashes",
	} {
		if got := repoFromArchBuild(in); got != want {
			t.Errorf("repoFromArchBuild(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMkarchrootArgs(t *testing.T) {
	got := mkarchrootArgs("x86_64", "/tmp/pac.conf", "/tmp/mk.conf", "/tmp/chroot/root")
	want := []string{
		"setarch", "x86_64",
		"mkarchroot",
		"-C", "/tmp/pac.conf",
		"-M", "/tmp/mk.conf",
		"/tmp/chroot/root", "base-devel",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("mkarchrootArgs = %v, want %v", got, want)
	}
}

func TestMakechrootpkgArgs(t *testing.T) {
	got := makechrootpkgArgs("/tmp/chroot", []string{"/pkgs/dep.pkg.tar.zst"})
	want := []string{
		"makechrootpkg", "-c", "-r", "/tmp/chroot",
		"-I", "/pkgs/dep.pkg.tar.zst",
		"--", "--syncdeps", "--noconfirm", "--log", "--holdver",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("makechrootpkgArgs = %v, want %v", got, want)
	}

	noInstall := makechrootpkgArgs("/tmp/chroot", nil)
	wantNoInstall := "makechrootpkg -c -r /tmp/chroot -- --syncdeps --noconfirm --log --holdver"
	if strings.Join(noInstall, " ") != wantNoInstall {
		t.Errorf("makechrootpkgArgs(no install) = %q, want %q", strings.Join(noInstall, " "), wantNoInstall)
	}
}

func TestChrootBuildGeneratedBranch(t *testing.T) {
	writeFixture(t, map[string]string{}) // empty: renders will fail fast
	b := New(builder.ResolvedConfig{Makepkg: builder.MakepkgConfig{Microarch: "x86_64_v3"}})
	_, err := b.Build(t.Context(), builder.Spec{SrcDir: t.TempDir(), Arch: "x86_64"})
	if err == nil {
		t.Fatal("want an error from the generated path with no devtools config")
	}
	if !strings.Contains(err.Error(), "devtools") {
		t.Errorf("error should surface the missing devtools config: %v", err)
	}
}

func TestChrootBuildWrapperRequiresArchBuild(t *testing.T) {
	b := New(builder.ResolvedConfig{})
	if _, err := b.Build(t.Context(), builder.Spec{SrcDir: t.TempDir()}); err == nil {
		t.Fatal("want an error when neither build config nor ArchBuild is set")
	}
}
