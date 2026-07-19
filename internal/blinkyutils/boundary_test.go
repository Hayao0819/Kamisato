package blinkyutils

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

const upstreamModule = "github.com/BrenekH/blinky"
const adapterImportPath = "github.com/Hayao0819/Kamisato/internal/blinkyutils"

func TestBlinkyImportsStayInsideAdapter(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	fset := token.NewFileSet()

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

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		aliases := make(map[string]bool)
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil || (importPath != upstreamModule && !strings.HasPrefix(importPath, upstreamModule+"/")) {
				continue
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(filepath.Dir(relative)) != "internal/blinkyutils" {
				t.Errorf("%s imports %s outside internal/blinkyutils", relative, importPath)
			}
			if spec.Name == nil {
				t.Errorf("%s imports %s without an explicit package name", relative, importPath)
				continue
			}
			name := spec.Name.Name
			if name == "." || name == "_" {
				t.Errorf("%s imports %s as %q", relative, importPath, name)
				continue
			}
			aliases[name] = true
		}
		checkExportedAPI(t, path, file, aliases)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBlinkyTypesDoNotEscapeAdapter(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test source")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(current), "..", ".."))
	loaded, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedSyntax |
			packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps,
		Dir: root,
	}, adapterImportPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d adapter packages", len(loaded))
	}
	for _, packageErr := range loaded[0].Errors {
		t.Error(packageErr)
	}
	if len(loaded[0].Errors) != 0 {
		return
	}
	guard := typeBoundary{upstreamBased: findUpstreamBasedTypes(loaded[0])}
	scope := loaded[0].Types.Scope()
	for _, name := range scope.Names() {
		if !token.IsExported(name) {
			continue
		}
		object := scope.Lookup(name)
		if guard.references(object.Type(), make(map[types.Type]bool)) {
			t.Errorf("%s exposes upstream type through %s", adapterImportPath, name)
		}
	}
}

func TestTypeBoundaryDetectsIndirectUpstreamTypes(t *testing.T) {
	upstreamPackage := types.NewPackage(upstreamModule+"/clientlib", "clientlib")
	upstreamName := types.NewTypeName(token.NoPos, upstreamPackage, "BlinkyClient", nil)
	upstreamClient := types.NewNamed(upstreamName, types.NewStruct(nil, nil), nil)
	result := types.NewVar(token.NoPos, nil, "", types.NewPointer(upstreamClient))
	signature := types.NewSignatureType(nil, nil, nil, types.NewTuple(), types.NewTuple(result), false)
	if !(typeBoundary{}).references(signature, make(map[types.Type]bool)) {
		t.Fatal("indirect function type was not detected")
	}

	localPackage := types.NewPackage(adapterImportPath, "blinkyutils")
	middle := types.NewTypeName(token.NoPos, localPackage, "middle", nil)
	hidden := types.NewTypeName(token.NoPos, localPackage, "hidden", nil)
	origins := map[*types.TypeName]*types.TypeName{
		middle: upstreamName,
		hidden: middle,
	}
	if !typeOriginatesUpstream(hidden, origins, make(map[*types.TypeName]bool)) {
		t.Fatal("local type chain was not detected")
	}
}

func checkExportedAPI(t *testing.T, path string, file *ast.File, aliases map[string]bool) {
	t.Helper()
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Name.IsExported() && usesUpstreamType(declaration.Type, aliases) {
				t.Errorf("%s exposes an upstream type through %s", path, declaration.Name.Name)
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					checkTypeSpec(t, path, spec, aliases)
				case *ast.ValueSpec:
					checkValueSpec(t, path, spec, aliases)
				}
			}
		}
	}
}

func checkTypeSpec(t *testing.T, path string, spec *ast.TypeSpec, aliases map[string]bool) {
	t.Helper()
	if spec.Assign.IsValid() {
		if usesUpstreamType(spec.Type, aliases) {
			t.Errorf("%s aliases an upstream type through %s", path, spec.Name.Name)
		}
		return
	}
	if !spec.Name.IsExported() {
		return
	}
	if spec.TypeParams != nil && usesUpstreamType(spec.TypeParams, aliases) {
		t.Errorf("%s exposes an upstream type through %s constraints", path, spec.Name.Name)
	}
	structType, ok := spec.Type.(*ast.StructType)
	if !ok {
		if usesUpstreamType(spec.Type, aliases) {
			t.Errorf("%s exposes an upstream type through %s", path, spec.Name.Name)
		}
		return
	}
	for _, field := range structType.Fields.List {
		if fieldIsExported(field) && usesUpstreamType(field.Type, aliases) {
			t.Errorf("%s exposes an upstream type through %s field", path, spec.Name.Name)
		}
	}
}

func checkValueSpec(t *testing.T, path string, spec *ast.ValueSpec, aliases map[string]bool) {
	t.Helper()
	exported := false
	for _, name := range spec.Names {
		exported = exported || name.IsExported()
	}
	if !exported {
		return
	}
	if usesUpstreamType(spec.Type, aliases) {
		t.Errorf("%s exposes an upstream type through an exported value", path)
		return
	}
	if spec.Type == nil {
		for _, value := range spec.Values {
			if usesUpstreamType(value, aliases) {
				t.Errorf("%s infers an exported value from an upstream declaration", path)
				return
			}
		}
	}
}

func fieldIsExported(field *ast.Field) bool {
	if len(field.Names) == 0 {
		return true
	}
	for _, name := range field.Names {
		if name.IsExported() {
			return true
		}
	}
	return false
}

func usesUpstreamType(node ast.Node, aliases map[string]bool) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if ok && aliases[pkg.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

type typeBoundary struct {
	upstreamBased map[*types.TypeName]bool
}

func (guard typeBoundary) references(typ types.Type, seen map[types.Type]bool) bool {
	if typ == nil || seen[typ] {
		return false
	}
	seen[typ] = true

	switch typ := typ.(type) {
	case *types.Alias:
		return objectFromUpstream(typ.Obj()) || guard.upstreamBased[typ.Obj()] || guard.references(typ.Rhs(), seen)
	case *types.Named:
		if objectFromUpstream(typ.Obj()) || guard.upstreamBased[typ.Obj()] {
			return true
		}
		for index := 0; index < typ.TypeArgs().Len(); index++ {
			if guard.references(typ.TypeArgs().At(index), seen) {
				return true
			}
		}
		if typ.Obj().Pkg() == nil || typ.Obj().Pkg().Path() != adapterImportPath {
			return false
		}
		for index := 0; index < typ.TypeParams().Len(); index++ {
			if guard.references(typ.TypeParams().At(index).Constraint(), seen) {
				return true
			}
		}
		if guard.referencesNamedSurface(typ.Underlying(), seen) {
			return true
		}
		for index := 0; index < typ.NumMethods(); index++ {
			method := typ.Method(index)
			if method.Exported() && guard.references(method.Type(), seen) {
				return true
			}
		}
		return false
	case *types.Array:
		return guard.references(typ.Elem(), seen)
	case *types.Slice:
		return guard.references(typ.Elem(), seen)
	case *types.Pointer:
		return guard.references(typ.Elem(), seen)
	case *types.Map:
		return guard.references(typ.Key(), seen) || guard.references(typ.Elem(), seen)
	case *types.Chan:
		return guard.references(typ.Elem(), seen)
	case *types.Signature:
		if guard.referencesTypeParameters(typ.RecvTypeParams(), seen) ||
			guard.referencesTypeParameters(typ.TypeParams(), seen) ||
			guard.references(typ.Params(), seen) ||
			guard.references(typ.Results(), seen) {
			return true
		}
		return typ.Recv() != nil && guard.references(typ.Recv().Type(), seen)
	case *types.Tuple:
		for index := 0; index < typ.Len(); index++ {
			if guard.references(typ.At(index).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Struct:
		for index := 0; index < typ.NumFields(); index++ {
			if guard.references(typ.Field(index).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Interface:
		typ.Complete()
		for index := 0; index < typ.NumMethods(); index++ {
			if guard.references(typ.Method(index).Type(), seen) {
				return true
			}
		}
		for index := 0; index < typ.NumEmbeddeds(); index++ {
			if guard.references(typ.EmbeddedType(index), seen) {
				return true
			}
		}
		return false
	case *types.TypeParam:
		return guard.references(typ.Constraint(), seen)
	case *types.Union:
		for index := 0; index < typ.Len(); index++ {
			if guard.references(typ.Term(index).Type(), seen) {
				return true
			}
		}
	}
	return false
}

func (guard typeBoundary) referencesTypeParameters(parameters *types.TypeParamList, seen map[types.Type]bool) bool {
	if parameters == nil {
		return false
	}
	for index := 0; index < parameters.Len(); index++ {
		if guard.references(parameters.At(index).Constraint(), seen) {
			return true
		}
	}
	return false
}

func (guard typeBoundary) referencesNamedSurface(underlying types.Type, seen map[types.Type]bool) bool {
	structType, ok := underlying.(*types.Struct)
	if !ok {
		return guard.references(underlying, seen)
	}
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if (field.Exported() || field.Embedded()) && guard.references(field.Type(), seen) {
			return true
		}
	}
	return false
}

func findUpstreamBasedTypes(pkg *packages.Package) map[*types.TypeName]bool {
	origins := make(map[*types.TypeName]*types.TypeName)
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			declared, ok := pkg.TypesInfo.Defs[spec.Name].(*types.TypeName)
			if !ok {
				return true
			}
			if origin := directTypeName(pkg.TypesInfo.TypeOf(spec.Type)); origin != nil {
				origins[declared] = origin
			}
			return true
		})
	}

	result := make(map[*types.TypeName]bool)
	for declared := range origins {
		if typeOriginatesUpstream(declared, origins, make(map[*types.TypeName]bool)) {
			result[declared] = true
		}
	}
	return result
}

func directTypeName(typ types.Type) *types.TypeName {
	switch typ := typ.(type) {
	case *types.Alias:
		return typ.Obj()
	case *types.Named:
		return typ.Obj()
	default:
		return nil
	}
}

func typeOriginatesUpstream(
	declared *types.TypeName,
	origins map[*types.TypeName]*types.TypeName,
	seen map[*types.TypeName]bool,
) bool {
	if declared == nil || seen[declared] {
		return false
	}
	seen[declared] = true
	origin := origins[declared]
	return objectFromUpstream(origin) || typeOriginatesUpstream(origin, origins, seen)
}

func objectFromUpstream(object *types.TypeName) bool {
	if object == nil || object.Pkg() == nil {
		return false
	}
	path := object.Pkg().Path()
	return path == upstreamModule || strings.HasPrefix(path, upstreamModule+"/")
}
