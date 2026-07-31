package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	ayakacmd "github.com/Hayao0819/Kamisato/ayaka/cmd"
	ayatocmd "github.com/Hayao0819/Kamisato/ayato/cmd"
	kayocmd "github.com/Hayao0819/Kamisato/kayo/cmd"
	luminecmd "github.com/Hayao0819/Kamisato/lumine/cmd"
	mikocmd "github.com/Hayao0819/Kamisato/miko/cmd"
	thomacmd "github.com/Hayao0819/Kamisato/thoma/cmd"
)

func TestCommandFilesMatchCommands(t *testing.T) {
	roots := map[string]*cobra.Command{
		"ayaka":  ayakacmd.RootCmd(),
		"ayato":  ayatocmd.RootCmd(),
		"kayo":   kayocmd.RootCmd(),
		"lumine": luminecmd.RootCmd(),
		"miko":   mikocmd.RootCmd(),
		"thoma":  thomacmd.RootCmd(),
	}
	for product, root := range roots {
		reachable := reachableCommandPaths(root)
		cmdRoot := filepath.Join(product, "cmd")
		err := filepath.WalkDir(cmdRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if path == filepath.Join(cmdRoot, "root.go") {
				return nil
			}
			if entry.Name() == "root.go" {
				t.Errorf("nested command root must use its command name: %s", path)
				return nil
			}

			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			uses := commandUses(parsed)
			if len(uses) == 0 && callsInstallFactory(parsed) {
				uses = []string{"install"}
			}
			if len(uses) != 1 {
				t.Errorf("%s defines %d commands, want 1", path, len(uses))
				return nil
			}
			want := strings.ReplaceAll(strings.Fields(uses[0])[0], "-", "_") + ".go"
			if entry.Name() != want {
				t.Errorf("%s defines %q; filename must be %s", path, uses[0], want)
			}
			commandPath := sourceCommandPath(product, cmdRoot, path, strings.Fields(uses[0])[0])
			if !reachable[commandPath] {
				t.Errorf("%s defines unreachable command %q", path, commandPath)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func commandUses(file *ast.File) []string {
	var uses []string
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok || !isCobraCommandType(literal.Type) {
			return true
		}
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := field.Key.(*ast.Ident)
			value, valueOK := field.Value.(*ast.BasicLit)
			if !ok || name.Name != "Use" || !valueOK || value.Kind != token.STRING {
				continue
			}
			if unquoted, err := strconv.Unquote(value.Value); err == nil {
				uses = append(uses, unquoted)
			}
		}
		return false
	})
	return uses
}

func isCobraCommandType(expr ast.Expr) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Command" {
		return false
	}
	packageName, ok := selector.X.(*ast.Ident)
	return ok && packageName.Name == "cobra"
}

func callsInstallFactory(file *ast.File) bool {
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return !found
		}
		packageName, packageOK := selector.X.(*ast.Ident)
		if packageOK && packageName.Name == "sharedhook" && selector.Sel.Name == "NewInstallCmd" {
			found = true
		}
		return !found
	})
	return found
}

func reachableCommandPaths(root *cobra.Command) map[string]bool {
	paths := map[string]bool{root.CommandPath(): true}
	var walk func(*cobra.Command)
	walk = func(parent *cobra.Command) {
		for _, command := range parent.Commands() {
			paths[command.CommandPath()] = true
			walk(command)
		}
	}
	walk(root)
	return paths
}

func sourceCommandPath(product, cmdRoot, path, command string) string {
	if override, ok := commandPathOverrides[filepath.ToSlash(path)]; ok {
		return override
	}
	directory, _ := filepath.Rel(cmdRoot, filepath.Dir(path))
	parts := []string{product}
	if directory != "." {
		parts = append(parts, strings.Split(filepath.ToSlash(directory), "/")...)
	}
	if parts[len(parts)-1] != command {
		parts = append(parts, command)
	}
	return strings.Join(parts, " ")
}

var commandPathOverrides = map[string]string{
	"ayato/cmd/audit.go":  "ayato kv audit",
	"ayato/cmd/gc.go":     "ayato repo gc",
	"ayato/cmd/keygen.go": "ayato aur keygen",
}
