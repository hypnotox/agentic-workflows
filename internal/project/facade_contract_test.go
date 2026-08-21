package project

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"golang.org/x/tools/go/packages"
)

const projectImportPath = "github.com/hypnotox/agentic-workflows/internal/project"
const projectStateImportPath = "github.com/hypnotox/agentic-workflows/internal/projectstate"

func TestProjectStateZeroValueCompatibility(t *testing.T) {
	var state ProjectState
	if state.Root() != "" {
		t.Fatalf("zero ProjectState root = %q", state.Root())
	}
	if loaded := state.Config(); loaded == nil || !reflect.DeepEqual(loaded, &config.Config{}) {
		t.Fatalf("zero ProjectState config = %#v", loaded)
	}
	if state.Targets() != nil {
		t.Fatalf("zero ProjectState targets = %#v", state.Targets())
	}
	if state.OutputState() != nil {
		t.Fatalf("zero ProjectState output state = %#v", state.OutputState())
	}
}

func TestProjectStateProductionBoundary(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{Dir: root, Mode: projectPackageMode}, "./...")
	if err != nil {
		t.Fatal(err)
	}
	allowedMethods := []string{"Catalog", "CompleteCatalog", "Config", "Facts", "Nested", "Root", "Roots", "Targets"}
	compatibilityMethods := []string{"Config", "OutputState", "Root", "Targets"}
	var methods, compatibility []string
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
				if pkg.PkgPath == projectStateImportPath && projectStateReceiver(pkg.TypesInfo, fn.Recv, projectStateImportPath) {
					methods = append(methods, fn.Name.Name)
				}
				if pkg.PkgPath == projectImportPath && fn.Name.IsExported() && projectStateReceiver(pkg.TypesInfo, fn.Recv, projectImportPath) {
					compatibility = append(compatibility, fn.Name.Name)
				}
			}
		}
	}
	sort.Strings(methods)
	if !slices.Equal(methods, allowedMethods) {
		t.Fatalf("projectstate.ProjectState methods = %v, want fact accessors only %v", methods, allowedMethods)
	}
	sort.Strings(compatibility)
	if !slices.Equal(compatibility, compatibilityMethods) {
		t.Fatalf("project.ProjectState compatibility methods = %v, want RF-002 surface %v", compatibility, compatibilityMethods)
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

var mutationAddsPackageCompatibilityCaller = func(ctx context.Context, root string) (*ProjectState, error) {
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
	mutationCallers := projectOpenCallers(mutation)
	if mutationCallers[projectOpenCaller{pkg: projectImportPath, owner: "mutationAddsCompatibilityCaller"}] != 1 {
		t.Fatal("compatibility-opener census did not detect an added internal caller")
	}
	if mutationCallers[projectOpenCaller{pkg: projectImportPath, owner: "<package>"}] != 1 {
		t.Fatal("compatibility-opener census did not detect a package-level caller")
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
			owners := map[token.Pos]string{}
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if node != nil {
						owners[node.Pos()] = fn.Name.Name
					}
					return true
				})
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || !isProjectOpenCall(pkg.TypesInfo, call) {
					return true
				}
				owner := owners[call.Pos()]
				if owner == "" {
					owner = "<package>"
				}
				callers[projectOpenCaller{pkg: pkg.PkgPath, owner: owner}]++
				return true
			})
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

func projectStateReceiver(info *types.Info, recv *ast.FieldList, packagePath string) bool {
	if recv == nil || len(recv.List) != 1 {
		return false
	}
	typ := info.TypeOf(recv.List[0].Type)
	if pointer, ok := typ.(*types.Pointer); ok {
		typ = pointer.Elem()
	}
	named, ok := typ.(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == packagePath && named.Obj().Name() == "ProjectState"
}

func isProjectOpenCall(info *types.Info, call *ast.CallExpr) bool {
	var object types.Object
	switch function := call.Fun.(type) {
	case *ast.Ident:
		object = info.Uses[function]
	case *ast.SelectorExpr:
		object = info.Uses[function.Sel]
	}
	return object != nil && object.Name() == "Open" && object.Pkg() != nil && object.Pkg().Path() == projectImportPath && object.Parent() == object.Pkg().Scope()
}
