package source

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/internal/pacman/builder"
)

func TestSrcConfigValidate(t *testing.T) {
	if err := (&SrcConfig{Name: "myrepo"}).Validate(); err != nil {
		t.Errorf("valid source repo rejected: %v", err)
	}
	if err := (&SrcConfig{}).Validate(); err == nil {
		t.Error("source repo without a name was accepted")
	}
}

func TestSrcConfigMigrateLegacy(t *testing.T) {
	c := &SrcConfig{
		Name:            "alterlinux",
		LegacyServer:    "https://host/repo/alterlinux/x86_64",
		LegacyArchBuild: "extra-x86_64-build",
	}
	c.migrateLegacy()
	if c.URL != "https://host/repo/alterlinux" || c.Build.ArchBuild != "extra-x86_64-build" {
		t.Fatalf("migrated config = %+v", c)
	}

	explicit := &SrcConfig{
		URL:             "https://new/url",
		Build:           builder.ProjectConfig{ArchBuild: "custom-build"},
		LegacyServer:    "https://host/repo/alterlinux/aarch64",
		LegacyArchBuild: "extra-x86_64-build",
	}
	explicit.migrateLegacy()
	if explicit.URL != "https://new/url" || explicit.Build.ArchBuild != "custom-build" {
		t.Errorf("legacy fields overrode explicit fields: %+v", explicit)
	}
}

func TestStripArchSuffix(t *testing.T) {
	cases := map[string]string{
		"https://host/repo/x/x86_64":    "https://host/repo/x",
		"https://host/repo/x/x86_64_v3": "https://host/repo/x",
		"https://host/repo/x/aarch64/":  "https://host/repo/x",
		"https://host/repo/x/i486":      "https://host/repo/x",
		"https://host/repo/x/i686":      "https://host/repo/x",
		"https://host/repo/x/pentium4":  "https://host/repo/x",
		"https://host/repo/x/any":       "https://host/repo/x",
		"https://host/repo/x":           "https://host/repo/x",
	}
	for input, want := range cases {
		if got := stripArchSuffix(input); got != want {
			t.Errorf("stripArchSuffix(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSrcConfigRoundTrip(t *testing.T) {
	c := &SrcConfig{
		Name:       "alterlinux",
		Maintainer: "Hayao",
		URL:        "https://host/repo/alterlinux",
		Build: builder.ProjectConfig{
			Repos:     []builder.PacmanRepository{{Name: "ayato", Server: "https://host/repo/$repo/$arch", SigLevel: "Optional TrustAll"}},
			Makepkg:   builder.MakepkgConfig{Packager: "Hayao", Microarch: "x86_64_v3", CFlagsAppend: "-O3", Options: []string{"!strip"}},
			ArchBuild: "extra-x86_64-build",
		},
		InstallPkgs: InstallPkgsConfig{Names: []string{"foo"}},
	}
	data, err := c.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"name"`, `"url"`, `"build"`, `"repos"`, `"siglevel"`, `"makepkg"`, `"microarch"`, `"cflags_append"`, `"archbuild"`, `"installpkgs"`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("marshalled repo.json missing key %s:\n%s", want, data)
		}
	}
	var decoded SrcConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*c, decoded) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", decoded, *c)
	}
}
