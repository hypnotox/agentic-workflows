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
			}
		}
	}
	sort.Strings(methods)
	if !slices.Equal(methods, allowedMethods) {
		t.Fatalf("ProjectState methods = %v, want fact accessors only %v", methods, allowedMethods)
	}
	wantCallers := map[projectOpenCaller]int{{pkg: projectImportPath, owner: "VerifyCommitPolicyAt"}: 1}
	assertProjectOpenCallers(t, projectOpenCallers(pkgs), wantCallers)

	fixture := filepath.Join(root, filepath.FromSlash("internal/project/compatibility_opener_mutation_fixture.go"))
	mutation, err := packages.Load(&packages.Config{Dir: root, Mode: projectPackageMode, Overlay: map[string][]byte{
		fixture: []byte(`package project

import "context"

func mutationAddsCompatibilityCaller(ctx context.Context, root string) (*ProjectState, error) {
	return Open(ctx, root)
}
`),
	}}, "./internal/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(mutation) != 1 || len(mutation[0].Errors) != 0 {
		if len(mutation) == 1 && len(mutation[0].Errors) != 0 {
			t.Fatal(mutation[0].Errors[0])
		}
		t.Fatalf("loaded mutation packages = %d, want 1", len(mutation))
	}
	if projectOpenCallers(mutation)[projectOpenCaller{pkg: projectImportPath, owner: "mutationAddsCompatibilityCaller"}] != 1 {
		t.Fatal("compatibility-opener census did not detect an added internal caller")
	}
}

type projectOpenCaller struct {
	pkg   string
	owner string
}

func projectOpenCallers(pkgs []*packages.Package) map[projectOpenCaller]int {
	callers := map[projectOpenCaller]int{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if ok && callsProjectOpen(pkg.TypesInfo, fn) {
					callers[projectOpenCaller{pkg: pkg.PkgPath, owner: fn.Name.Name}]++
				}
			}
		}
	}
	return callers
}

func assertProjectOpenCallers(t *testing.T, got, want map[projectOpenCaller]int) {
	t.Helper()
	for caller, count := range want {
		if got[caller] != count {
			t.Errorf("project.Open caller %s.%s count = %d, want %d", caller.pkg, caller.owner, got[caller], count)
		}
		delete(got, caller)
	}
	for caller, count := range got {
		t.Errorf("unexpected project.Open caller %s.%s count %d", caller.pkg, caller.owner, count)
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
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		var object types.Object
		switch function := call.Fun.(type) {
		case *ast.Ident:
			object = info.Uses[function]
		case *ast.SelectorExpr:
			object = info.Uses[function.Sel]
		}
		if object != nil && object.Name() == "Open" && object.Pkg() != nil && object.Pkg().Path() == projectImportPath && object.Parent() == object.Pkg().Scope() {
			found = true
		}
		return true
	})
	return found
}
