package repo

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var yayPkg Package

func TestGetPkgFileNames(t *testing.T) {
	pkgs, err := yayPkg.GetPkgFileNames()
	if err != nil {
		t.Errorf("GetPkgFileNames() error = %v", err)
		return
	}
	t.Log("pkg file path:", pkgs)
	if len(pkgs) == 0 {
		t.Errorf("GetPkgFileNames() = %v, want not empty", pkgs)
		return
	}
	for _, p := range pkgs {
		if !strings.HasSuffix(p, ".pkg.tar.zst") {
			t.Errorf("GetPkgFileNames() = %v, want .pkg.tar.zst", p)
			return
		}
	}
}

func init() {
	_, f, _, _ := runtime.Caller(0)
	rootDir := filepath.Clean(path.Join(path.Dir(f), "..", ".."))

	y, err := GetPackage(path.Join(rootDir, "example", "src", "myrepo", "yay"))
	if err != nil {
		panic(err)
	}
	yayPkg = *y
}
