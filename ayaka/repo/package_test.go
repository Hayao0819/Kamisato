package repo

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var yayPkg Package

func TestGetPkgFilePath(t *testing.T) {
	p, err := yayPkg.GetPkgFilePath()
	if err != nil {
		t.Errorf("GetPkgFilePath() error = %v", err)
		return
	}
	t.Log("pkg file path:", p)
	if !strings.HasSuffix(p, ".pkg.tar.zst") {
		t.Errorf("GetPkgFilePath() = %v, want .pkg.tar.zst", p)
		return
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
