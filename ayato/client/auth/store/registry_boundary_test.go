package store

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
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
	root := filepath.Dir(current)
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("locate module root")
		}
		root = parent
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			// Hidden dirs can hold git worktree checkouts of the whole module.
			if name == "node_modules" || name == "vendor" ||
				(strings.HasPrefix(name, ".") && path != root) {
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
			if filepath.ToSlash(filepath.Dir(relative)) != "ayato/client/auth/store" ||
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
