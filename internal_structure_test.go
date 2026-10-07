package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var internalGroups = map[string]bool{
	"api":        true,
	"auth":       true,
	"cli":        true,
	"config":     true,
	"errors":     true,
	"filesystem": true,
	"http":       true,
	"pacman":     true,
	"vcs":        true,
}

var productPackages = []string{
	"github.com/Hayao0819/Kamisato/ayaka",
	"github.com/Hayao0819/Kamisato/ayato",
	"github.com/Hayao0819/Kamisato/kayo",
	"github.com/Hayao0819/Kamisato/lumine",
	"github.com/Hayao0819/Kamisato/miko",
	"github.com/Hayao0819/Kamisato/thoma",
}

func TestInternalPackageLayout(t *testing.T) {
	entries, err := os.ReadDir("internal")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			t.Errorf("internal root contains file %s", entry.Name())
			continue
		}
		if !internalGroups[entry.Name()] {
			t.Errorf("internal/%s is not a recognized group", entry.Name())
		}
	}

	err = filepath.WalkDir("internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		leaf := filepath.Base(filepath.Dir(path))
		if file.Name.Name != leaf && file.Name.Name != leaf+"_test" {
			t.Errorf("%s declares package %s; want %s", path, file.Name.Name, leaf)
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			for _, product := range productPackages {
				if importPath == product || strings.HasPrefix(importPath, product+"/") {
					t.Errorf("%s imports product package %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
