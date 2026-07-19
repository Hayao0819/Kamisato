package blinkyutils

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const upstreamModule = "github.com/BrenekH/blinky"

// Blinky remains only as an oracle for the released servers.json format. New
// production code must use Kamisato-owned clients and registry types.
func TestBlinkyImportsAreCompatibilityTestsOnly(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil || (importPath != upstreamModule && !strings.HasPrefix(importPath, upstreamModule+"/")) {
				continue
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if filepath.ToSlash(filepath.Dir(relative)) != "internal/blinkyutils" ||
				!strings.HasSuffix(relative, "_test.go") {
				t.Errorf("%s imports %s outside the compatibility tests", relative, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
