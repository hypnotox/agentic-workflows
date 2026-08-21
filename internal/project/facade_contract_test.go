package project

import (
	"go/ast"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"testing"

	"golang.org/x/tools/go/packages"
)

const projectImportPath = "github.com/hypnotox/agentic-workflows/internal/project"

func TestProjectStateProductionBoundary(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: projectPackageMode}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	allowedMethods := []string{"Config", "Root", "Targets", "catalog", "completeCatalog", "resolvedTargets"}
	var methods []string
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
		if pkg.PkgPath == projectImportPath && pkg.Types.Scope().Lookup("Project") != nil {
			t.Fatal("production Project facade still exists")
		}
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if pkg.PkgPath == projectImportPath && projectStateReceiver(pkg.TypesInfo, fn.Recv) {
					methods = append(methods, fn.Name.Name)
				}
				if pkg.PkgPath != projectImportPath && callsProjectOpen(pkg.TypesInfo, fn) {
					t.Errorf("production caller %s.%s still invokes project.Open", pkg.PkgPath, fn.Name.Name)
				}
			}
		}
	}
	sort.Strings(methods)
	if !slices.Equal(methods, allowedMethods) {
		t.Fatalf("ProjectState methods = %v, want fact accessors only %v", methods, allowedMethods)
	}
}

func projectStateReceiver(info *types.Info, recv *ast.FieldList) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}
	typ := info.TypeOf(recv.List[0].Type)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == projectImportPath && named.Obj().Name() == "ProjectState"
}

func callsProjectOpen(info *types.Info, node ast.Node) bool {
	found := false
	ast.Inspect(node, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Open" {
			return true
		}
		object := info.Uses[selector.Sel]
		if object != nil && object.Pkg() != nil && object.Pkg().Path() == projectImportPath {
			found = true
		}
		return true
	})
	return found
}
