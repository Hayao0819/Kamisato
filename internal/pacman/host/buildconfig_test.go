package host

import (
	"strings"
	"testing"

	pacmanconf "github.com/Morganamilo/go-pacmanconf"
)

func TestBuildConfigFromParsedPreservesRepositoryOrder(t *testing.T) {
	config := &pacmanconf.Config{
		Architecture: []string{"i486"},
		SigLevel:     []string{"PackageRequired", "DatabaseOptional"},
		Repos: []pacmanconf.Repository{
			{Name: "profile", Servers: []string{"https://repo/$repo/$arch"}, SigLevel: []string{"PackageOptional"}},
			{Name: "core", Servers: []string{"https://mirror/$repo/os/$arch"}},
		},
	}
	got, err := BuildConfigFromParsed(config, "i486")
	if err != nil {
		t.Fatal(err)
	}
	if got.Repositories[0].Servers[0] != "https://repo/profile/i486" {
		t.Fatalf("server = %q", got.Repositories[0].Servers[0])
	}
	text := string(got.PacmanConf)
	if strings.Index(text, "[profile]") > strings.Index(text, "[core]") {
		t.Fatalf("repository order changed:\n%s", text)
	}
}

func TestBuildConfigFromParsedRejectsArchitectureMismatch(t *testing.T) {
	_, err := BuildConfigFromParsed(&pacmanconf.Config{Architecture: []string{"x86_64"}}, "i686")
	if err == nil {
		t.Fatal("architecture mismatch succeeded")
	}
}
