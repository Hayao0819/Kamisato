package pacman_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// These are concept boundaries, not layers every feature must pass through.
// New implementations stay with their concept instead of introducing another
// catch-all package or making the shared library depend on a product's CLI.
func TestPackageDependencies(t *testing.T) {
	allowed := map[string]map[string]bool{
		"builder": {"builder": true, "pkg": true},
		"depend":  {"depend": true},
		"hook":    {"hook": true},
		"host":    {"host": true, "pkg": true},
		"limits":  {"limits": true},
		"nvcheck": {"nvcheck": true},
		"pkg":     {"pkg": true},
		"repo":    {"repo": true, "pkg": true, "sign": true},
		"sign":    {"sign": true},
	}
	const module = "github.com/Hayao0819/Kamisato/"
	const pacman = module + "internal/pacman/"
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative := filepath.ToSlash(path)
		owner, _, _ := strings.Cut(relative, "/")
		if allowed[owner] == nil {
			t.Errorf("%s has no documented pacman concept", path)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			dependency, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			switch {
			case dependency == "github.com/spf13/cobra" || dependency == "github.com/spf13/pflag" ||
				dependency == "github.com/gin-gonic/gin":
				t.Errorf("%s imports entry-point framework %s", path, dependency)
			case strings.HasPrefix(dependency, pacman):
				concept, _, _ := strings.Cut(strings.TrimPrefix(dependency, pacman), "/")
				if !allowed[owner][concept] {
					t.Errorf("%s imports %s against the pacman dependency direction", path, dependency)
				}
				if filepath.Dir(relative) == "builder" && strings.HasPrefix(dependency, pacman+"builder/") {
					t.Errorf("builder contract %s imports its implementation %s", path, dependency)
				}
			case strings.HasPrefix(dependency, module):
				if !strings.HasPrefix(dependency, module+"pkg/") &&
					!strings.HasPrefix(dependency, module+"internal/filesystem/") &&
					!strings.HasPrefix(dependency, module+"internal/vcs/") &&
					dependency != module+"internal/errors" {
					t.Errorf("%s imports %s outside the shared technical boundary", path, dependency)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
