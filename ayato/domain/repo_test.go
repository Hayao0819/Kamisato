package domain

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Hayao0819/Kamisato/pkg/raiou"
)

func TestPacmanPackageJSONFlattensMetadata(t *testing.T) {
	t.Parallel()
	data, err := json.Marshal(PacmanPackage{
		PKGINFO:  raiou.PKGINFO{PkgName: "demo", PkgVer: "1.0-1"},
		Filename: "demo-1.0-1-x86_64.pkg.tar.xz",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, want := range []string{
		`"pkgname":"demo"`,
		`"pkgver":"1.0-1"`,
		`"filename":"demo-1.0-1-x86_64.pkg.tar.xz"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("JSON %s does not contain %s", body, want)
		}
	}
	if strings.Contains(body, `"PKGINFO"`) {
		t.Errorf("embedded PKGINFO unexpectedly nested in JSON: %s", body)
	}
}
