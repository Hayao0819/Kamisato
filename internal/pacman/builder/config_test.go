package builder

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestResolve(t *testing.T) {
	hostRepos := []PacmanRepository{{Name: "host", Server: "https://host/$arch"}}
	host := HostConfig{
		Backend:      KindContainer,
		Timeout:      20 * time.Minute,
		Repositories: hostRepos,
		Docker: DockerConfig{
			Image:          "host/$arch:latest",
			Host:           "unix:///run/docker.sock",
			PacmanCacheDir: "/cache/pacman",
			CcacheDir:      "/cache/ccache",
		},
		Bwrap:    BwrapConfig{Rootfs: "/rootfs"},
		Devtools: DevtoolsConfig{ArchBuildTemplate: "extra-%s-build"},
	}
	overrides := BuildOverrides{
		Timeout:      time.Hour,
		Repositories: []PacmanRepository{{Name: "project", Server: "https://project/$arch"}},
		Makepkg:      MakepkgConfig{Microarch: "x86_64_v3", Options: []string{"!strip"}},
		DockerImage:  "project/$arch:latest",
	}

	got, err := Resolve(host, overrides, "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != KindContainer || got.Timeout != time.Hour {
		t.Errorf("resolved backend/timeout = %q/%s", got.Backend, got.Timeout)
	}
	if got.Docker.Image != "project/x86_64:latest" {
		t.Errorf("Docker.Image = %q", got.Docker.Image)
	}
	if got.Docker.Host != host.Docker.Host || got.Docker.PacmanCacheDir != host.Docker.PacmanCacheDir || got.Docker.CcacheDir != host.Docker.CcacheDir {
		t.Errorf("host-only Docker settings changed: %+v", got.Docker)
	}
	if got.Devtools.ArchBuild != "extra-x86_64-build" {
		t.Errorf("Devtools.ArchBuild = %q", got.Devtools.ArchBuild)
	}
	wantRepos := []PacmanRepository{hostRepos[0], overrides.Repositories[0]}
	if !reflect.DeepEqual(got.Repositories, wantRepos) {
		t.Errorf("Repositories = %+v, want %+v", got.Repositories, wantRepos)
	}

	// Resolve must not let later caller mutation alter the effective settings.
	hostRepos[0].Name = "mutated"
	overrides.Makepkg.Options[0] = "mutated"
	if got.Repositories[0].Name != "host" || got.Makepkg.Options[0] != "!strip" {
		t.Errorf("resolved config aliases inputs: %+v", got)
	}
}

func TestProjectConfigOverrides(t *testing.T) {
	project := ProjectConfig{
		Arches:    map[string]ArchConfig{"x86_64": {}},
		Timeout:   "45m",
		Image:     "archlinux:$arch",
		ArchBuild: "./repository-payload",
	}
	overrides, err := project.Overrides("x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if overrides.Timeout != 45*time.Minute || overrides.DockerImage != "archlinux:$arch" {
		t.Errorf("Overrides = %+v", overrides)
	}
	resolved, err := Resolve(HostConfig{
		Devtools: DevtoolsConfig{ArchBuildTemplate: "extra-%s-build"},
	}, overrides, "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Devtools.ArchBuild != "extra-x86_64-build" {
		t.Errorf("repository-owned ArchBuild escaped into host config: %q", resolved.Devtools.ArchBuild)
	}
	if _, err := project.Overrides("aarch64"); err == nil {
		t.Fatal("unsupported project arch: want error")
	}
	project.Arches = nil
	project.Timeout = "not-a-duration"
	if _, err := project.Overrides("x86_64"); err == nil {
		t.Fatal("invalid duration: want error")
	}
}

func TestResolveValidationAndDefaults(t *testing.T) {
	got, err := Resolve(HostConfig{
		Devtools: DevtoolsConfig{ArchBuildTemplate: "extra-%s-build"},
	}, BuildOverrides{}, "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if got.Backend != KindChroot {
		t.Errorf("empty backend resolved to %q, want chroot", got.Backend)
	}
	if got.Devtools.ArchBuild != "extra-x86_64-build" {
		t.Errorf("ArchBuild = %q", got.Devtools.ArchBuild)
	}
	if _, err := Resolve(HostConfig{}, BuildOverrides{}, "x86_64"); err == nil {
		t.Fatal("unconfigured chroot: want error")
	}
	if _, err := Resolve(HostConfig{Backend: "unknown"}, BuildOverrides{}, "x86_64"); err == nil {
		t.Fatal("unknown backend: want error")
	}
	if _, err := Resolve(HostConfig{Backend: KindBwrap}, BuildOverrides{}, "x86_64"); err == nil {
		t.Fatal("bwrap without rootfs: want error")
	}
	if _, err := Resolve(HostConfig{}, BuildOverrides{Makepkg: MakepkgConfig{Microarch: "x86_64_v9"}}, "x86_64"); err == nil {
		t.Fatal("unknown microarchitecture: want error")
	}
	if _, err := Resolve(HostConfig{}, BuildOverrides{Makepkg: MakepkgConfig{Microarch: "x86_64_v3"}}, "aarch64"); err == nil {
		t.Fatal("x86-64 microarchitecture with aarch64: want error")
	}

	container, err := Resolve(HostConfig{Backend: KindContainer}, BuildOverrides{}, "x86_64")
	if err != nil {
		t.Fatal(err)
	}
	if container.Timeout != DefaultDockerTimeout || container.Docker.Image != DefaultDockerImage {
		t.Errorf("container defaults = timeout %s image %q", container.Timeout, container.Docker.Image)
	}
}

func TestHostConfigJSONDurationIsStringOnly(t *testing.T) {
	host := HostConfig{
		Backend: KindContainer,
		Timeout: 45 * time.Minute,
		Docker:  DockerConfig{Image: "archlinux:test"},
	}
	data, err := json.Marshal(host)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"timeout":"45m0s"`) {
		t.Fatalf("duration was not marshalled as a string: %s", data)
	}
	var roundTrip HostConfig
	if err := json.Unmarshal(data, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if roundTrip.Timeout != host.Timeout {
		t.Errorf("round-trip timeout = %s, want %s", roundTrip.Timeout, host.Timeout)
	}
	if err := json.Unmarshal([]byte(`{"timeout":30}`), &roundTrip); err == nil {
		t.Fatal("unitless numeric timeout: want error")
	}
}

func TestValidateRepositories(t *testing.T) {
	valid := []PacmanRepository{{Name: "custom-repo", Server: "https://repo/$repo/$arch", SigLevel: "Optional TrustAll"}}
	if err := ValidateRepositories(valid); err != nil {
		t.Fatal(err)
	}
	for name, repos := range map[string][]PacmanRepository{
		"missing server": {{Name: "repo"}},
		"newline":        {{Name: "repo", Server: "https://repo\nKAMISATO_EXTRA_REPO_EOF"}},
		"section break":  {{Name: "repo]", Server: "https://repo"}},
		"duplicate": {
			{Name: "repo", Server: "https://one"},
			{Name: "repo", Server: "https://two"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateRepositories(repos); err == nil {
				t.Fatal("want validation error")
			}
		})
	}
}

func TestProjectConfigOverridesAppliesPerArchTimeout(t *testing.T) {
	project := ProjectConfig{
		Timeout: "2h",
		Arches: map[string]ArchConfig{
			"pentium4": {},
			"i486":     {Timeout: "5h45m"},
		},
	}
	for _, tt := range []struct {
		arch string
		want time.Duration
	}{
		{"pentium4", 2 * time.Hour},
		{"i486", 5*time.Hour + 45*time.Minute},
	} {
		overrides, err := project.Overrides(tt.arch)
		if err != nil {
			t.Fatalf("%s: %v", tt.arch, err)
		}
		if overrides.Timeout != tt.want {
			t.Errorf("%s timeout = %s, want %s", tt.arch, overrides.Timeout, tt.want)
		}
	}
}

func TestProjectConfigOverridesFiltersReposByArch(t *testing.T) {
	project := ProjectConfig{
		Arches: map[string]ArchConfig{"x86_64": {}, "i486": {}},
		Repos: []PacmanRepository{
			{Name: "everywhere", Server: "https://r/$repo/$arch"},
			{Name: "thirtytwo", Server: "https://r/$repo/$arch", Arches: []string{"i486"}},
		},
	}
	// A repo absent for an arch must not reach pacman.conf: a missing database
	// aborts the entire sync, not just that repository.
	for _, tt := range []struct {
		arch string
		want []string
	}{
		{"x86_64", []string{"everywhere"}},
		{"i486", []string{"everywhere", "thirtytwo"}},
	} {
		overrides, err := project.Overrides(tt.arch)
		if err != nil {
			t.Fatalf("%s: %v", tt.arch, err)
		}
		var got []string
		for _, repo := range overrides.Repositories {
			got = append(got, repo.Name)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("%s repos = %v, want %v", tt.arch, got, tt.want)
		}
	}
}

func TestProjectConfigOverridesRejectsUnknownArch(t *testing.T) {
	project := ProjectConfig{Arches: map[string]ArchConfig{"x86_64": {}}}
	if _, err := project.Overrides("i486"); err == nil {
		t.Error("an arch outside build.arches should be rejected")
	}
}

func TestProjectConfigArchNamesAreSorted(t *testing.T) {
	project := ProjectConfig{
		Arches: map[string]ArchConfig{"pentium4": {}, "i486": {}, "i686": {}},
	}
	want := []string{"i486", "i686", "pentium4"}
	if got := project.ArchNames(); !slices.Equal(got, want) {
		t.Errorf("ArchNames() = %v, want %v", got, want)
	}
}
