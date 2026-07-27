package nvcheck

import (
	"strings"
	"testing"
)

func TestParseFile(t *testing.T) {
	tests := []struct {
		name    string
		toml    string
		pkgbase string
		want    Spec
	}{
		{
			"github max tag with transform",
			"[imagemagick]\nsource = \"github\"\ngithub = \"ImageMagick/ImageMagick\"\nuse_max_tag = true\nfrom_pattern = '^(\\d+\\.\\d+\\.\\d+)-(\\d+)$'\nto_pattern = '\\1.\\2'\n",
			"imagemagick",
			Spec{Kind: "github", Repo: "ImageMagick/ImageMagick", UseMaxTag: true, FromPattern: `^(\d+\.\d+\.\d+)-(\d+)$`, ToPattern: `\1.\2`},
		},
		{
			"git",
			"[ckbcomp]\nsource = \"git\"\ngit = \"https://salsa.debian.org/installer-team/console-setup.git\"\nprefix = \"v\"\n",
			"ckbcomp",
			Spec{Kind: "git", URL: "https://salsa.debian.org/installer-team/console-setup.git", Prefix: "v"},
		},
		{
			"pypi defaults to table name",
			"[requests]\nsource = \"pypi\"\n",
			"requests",
			Spec{Kind: "pypi", Package: "requests"},
		},
		{
			"archpkg strip_release",
			"[linux-nost]\nsource = \"archpkg\"\narchpkg = \"linux\"\nstrip_release = true\n",
			"linux-nost",
			Spec{Kind: "archpkg", Package: "linux", StripRelease: true},
		},
		{
			"regex",
			"[rust]\nsource = \"regex\"\nurl = \"https://www.archlinux32.org/packages/i686/extra/rust/\"\nregex = 'content=\"(?:\\d+:)?([^-\"]+)'\n",
			"rust",
			Spec{Kind: "regex", URL: "https://www.archlinux32.org/packages/i686/extra/rust/", Regex: `content="(?:\d+:)?([^-"]+)`},
		},
		{
			"unknown source passes through for NewSource to reject",
			"[google-chrome]\nsource = \"apt\"\nmirror = \"https://dl.google.com/linux/chrome/deb/\"\n",
			"google-chrome",
			Spec{Kind: "apt"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFile("test.toml", []byte(tt.toml))
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if f.Pkgbase != tt.pkgbase {
				t.Errorf("pkgbase = %q, want %q", f.Pkgbase, tt.pkgbase)
			}
			if f.Spec != tt.want {
				t.Errorf("spec = %+v, want %+v", f.Spec, tt.want)
			}
		})
	}
}

func TestParseFileRejectsMultipleTables(t *testing.T) {
	_, err := ParseFile("test.toml", []byte("[a]\nsource = \"pypi\"\n[b]\nsource = \"pypi\"\n"))
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("two tables should error, got %v", err)
	}
}

func TestParseFileIgnoresConfigTable(t *testing.T) {
	f, err := ParseFile("test.toml", []byte("[__config__]\noldver = \"old.json\"\n[pkg]\nsource = \"pypi\"\n"))
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if f.Pkgbase != "pkg" {
		t.Errorf("pkgbase = %q, want pkg", f.Pkgbase)
	}
}

func TestParseFileRejectsBrokenToml(t *testing.T) {
	if _, err := ParseFile("test.toml", []byte("[unclosed\n")); err == nil {
		t.Error("broken toml should error")
	}
}
