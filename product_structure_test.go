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
)

func TestProductPackageBoundaries(t *testing.T) {
	for _, product := range productPackages {
		root := strings.TrimPrefix(product, "github.com/Hayao0819/Kamisato/")
		t.Run(root, func(t *testing.T) {
			err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
						return filepath.SkipDir
					}
					return nil
				}
				if filepath.Ext(path) != ".go" {
					return nil
				}
				file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					return err
				}
				relative, err := filepath.Rel(root, filepath.Dir(path))
				if err != nil {
					return err
				}
				parts := strings.Split(filepath.ToSlash(relative), "/")
				layer := parts[0]
				commandHelper := strings.Contains("/"+filepath.ToSlash(relative)+"/", "/internal/")
				wantPackage := filepath.Base(filepath.Dir(path))
				if layer == "." {
					wantPackage = "main"
				} else if layer == "cmd" && len(parts) > 1 && !commandHelper {
					wantPackage = strings.ReplaceAll(wantPackage, "-", "") + "cmd"
				}
				if file.Name.Name != wantPackage && file.Name.Name != wantPackage+"_test" {
					t.Errorf("%s declares package %s; want %s", path, file.Name.Name, wantPackage)
				}
				if layer == "app" || layer == "cli" {
					t.Errorf("%s: command adapters belong in cmd; startup belongs in server", path)
				}
				// Integration tests may construct another component's server; production
				// callers must use its client or protocol surface.
				if strings.HasSuffix(path, "_test.go") {
					return nil
				}
				if layer == "cmd" && !commandHelper {
					for _, declaration := range file.Decls {
						if function, ok := declaration.(*ast.FuncDecl); ok && (function.Name.Name == "Cmd" || function.Name.Name == "RootCmd") {
							if function.Type.Params.NumFields() != 0 {
								t.Errorf("%s: public command constructor must not expose dependency wiring", path)
							}
						}
					}
				}
				for _, spec := range file.Imports {
					importPath, err := strconv.Unquote(spec.Path.Value)
					if err != nil {
						return err
					}
					if layer != "cmd" && importPath == "github.com/spf13/cobra" {
						t.Errorf("%s imports Cobra outside cmd", path)
					}
					if layer == "cmd" && len(parts) > 1 && importPath == product+"/server" {
						if root != "miko" || relative != "cmd/signer" {
							t.Errorf("%s: maintenance commands must not initialize a server", path)
						}
					}
					for _, other := range productPackages {
						if importPath != other && !strings.HasPrefix(importPath, other+"/") {
							continue
						}
						target := strings.Split(strings.TrimPrefix(importPath, other+"/"), "/")[0]
						if other != product {
							if target != "client" && target != "protocol" {
								t.Errorf("%s imports another component's implementation %s", path, importPath)
							}
							continue
						}
						if forbiddenProductDependency(layer, target) {
							t.Errorf("%s: %s must not depend on %s", path, layer, importPath)
						}
						if layer == "cmd" && target == "cmd" && !strings.Contains(importPath, "/internal/") {
							ownPackage := product + "/" + filepath.ToSlash(relative)
							if commandHelper || !strings.HasPrefix(importPath, ownPackage+"/") {
								t.Errorf("%s: commands may register descendants, not import parents or siblings: %s", path, importPath)
							}
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func forbiddenProductDependency(layer, target string) bool {
	switch layer {
	case "server":
		return target == "cmd"
	case "config", "service":
		return target == "server" || target == "cmd"
	case "client", "protocol":
		return target != "client" && target != "protocol"
	case "domain":
		return target != "domain"
	default:
		return false
	}
}
