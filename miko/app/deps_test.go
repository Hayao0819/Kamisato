package app

import (
	"testing"

	mikoconfig "github.com/Hayao0819/Kamisato/miko/config"
)

func TestServiceSettingsMapsVersionChecks(t *testing.T) {
	cfg := &mikoconfig.MikoConfig{AURGitBase: "https://aur.archlinux.org"}
	cfg.NvCheck.Entries = []mikoconfig.NvCheckEntry{
		{Pkgbase: "foo", Kind: "github", Repo: "o/foo"},
		{Pkgbase: "bar", Kind: "pypi", Package: "bar", Git: "https://example.com/bar.git"},
	}

	entries := ServiceSettings(cfg).VersionCheckEntries
	if entries[0].Git != "https://aur.archlinux.org/foo.git" {
		t.Errorf("default git = %q", entries[0].Git)
	}
	if entries[1].Git != "https://example.com/bar.git" {
		t.Errorf("explicit git overridden: %q", entries[1].Git)
	}
}
