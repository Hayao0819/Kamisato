package conf

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAyakaValidate(t *testing.T) {
	if err := (&AyakaConfig{}).Validate(); err != nil {
		t.Errorf("empty config (legacy/fresh) rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{Dir: "myrepo"}}}).Validate(); err != nil {
		t.Errorf("valid repo entry rejected: %v", err)
	}
	if err := (&AyakaConfig{Repos: []RepoEntry{{DestDir: "out"}}}).Validate(); err == nil {
		t.Error("expected an error for a repo entry with no dir")
	}
}

func TestSrcRepoValidate(t *testing.T) {
	if err := (&SrcRepoConfig{Name: "myrepo"}).Validate(); err != nil {
		t.Errorf("valid src repo rejected: %v", err)
	}
	if err := (&SrcRepoConfig{}).Validate(); err == nil {
		t.Error("expected an error for a src repo with no name")
	}
}

func TestSrcRepoMigrateLegacy(t *testing.T) {
	c := &SrcRepoConfig{
		Name:            "alterlinux",
		LegacyServer:    "https://host/repo/alterlinux/x86_64",
		LegacyArchBuild: "extra-x86_64-build",
	}
	c.migrateLegacy()
	if c.URL != "https://host/repo/alterlinux" {
		t.Errorf("URL = %q, want the arch-stripped https://host/repo/alterlinux", c.URL)
	}
	if c.Build.ArchBuild != "extra-x86_64-build" {
		t.Errorf("Build.ArchBuild = %q, want extra-x86_64-build", c.Build.ArchBuild)
	}
	if c.LegacyServer != "" || c.LegacyArchBuild != "" {
		t.Errorf("legacy aliases not cleared: server=%q archbuild=%q", c.LegacyServer, c.LegacyArchBuild)
	}

	// Explicit new-shape fields take precedence over the legacy aliases.
	c2 := &SrcRepoConfig{
		URL:          "https://new/url",
		Build:        SrcBuildConfig{ArchBuild: "custom-build"},
		LegacyServer: "https://host/repo/alterlinux/aarch64",
	}
	c2.LegacyArchBuild = "extra-x86_64-build"
	c2.migrateLegacy()
	if c2.URL != "https://new/url" || c2.Build.ArchBuild != "custom-build" {
		t.Errorf("legacy overrode explicit fields: url=%q archbuild=%q", c2.URL, c2.Build.ArchBuild)
	}
}

func TestStripArchSuffix(t *testing.T) {
	cases := map[string]string{
		"https://host/repo/x/x86_64":    "https://host/repo/x",
		"https://host/repo/x/x86_64_v3": "https://host/repo/x",
		"https://host/repo/x/aarch64/":  "https://host/repo/x",
		"https://host/repo/x/any":       "https://host/repo/x",
		"https://host/repo/x":           "https://host/repo/x",
	}
	for in, want := range cases {
		if got := stripArchSuffix(in); got != want {
			t.Errorf("stripArchSuffix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSrcRepoConfigRoundTrip(t *testing.T) {
	c := &SrcRepoConfig{
		Name:       "alterlinux",
		Maintainer: "Hayao",
		URL:        "https://host/repo/alterlinux",
		Build: SrcBuildConfig{
			Repos:     []BuildRepo{{Name: "ayato", Server: "https://host/repo/$repo/$arch", SigLevel: "Optional TrustAll"}},
			Makepkg:   MakepkgConfig{Packager: "Hayao", Microarch: "x86_64_v3", CFlagsAppend: "-O3", Options: []string{"!strip"}},
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
			t.Errorf("marshalled repo.json missing lowercase key %s:\n%s", want, data)
		}
	}
	var back SrcRepoConfig
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*c, back) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", back, *c)
	}
}
