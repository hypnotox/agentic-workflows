package project

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// projectPackageMode is the loader mode the state-ownership scan needs: syntax
// to walk assignments and type information to tell a *Project selector from any
// other receiver with the same field name.
const projectPackageMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo

// loadProjectPackage loads internal/project itself, optionally overlaying one
// file so a negative case can be committed rather than hand-mutated.
func loadProjectPackage(t *testing.T, overlay map[string][]byte) []*packages.Package {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Dir:     root,
		Mode:    projectPackageMode,
		Overlay: overlay,
	}, "./internal/project")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no package loaded for ./internal/project")
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
	}
	return pkgs
}

// isProjectValue reports whether expr's type is Project or *Project.
func isProjectValue(info *types.Info, expr ast.Expr) bool {
	typ := info.TypeOf(expr)
	if typ == nil {
		return false
	}
	if ptr, ok := typ.Underlying().(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok {
		return false
	}
	return named.Obj().Name() == "Project" &&
		named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == "github.com/hypnotox/agentic-workflows/internal/project"
}

// isProjectLiteral reports whether expr constructs a Project value directly,
// as `Project{...}` or `&Project{...}`.
func isProjectLiteral(expr ast.Expr) bool {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	ident, ok := lit.Type.(*ast.Ident)
	return ok && ident.Name == "Project"
}

// constructedInFunc collects the objects a function assigns from a Project
// composite literal. Those are the values that function constructs, so its
// later writes to their fields are part of one construction rather than a
// mutation of a value that outlived an operation.
func constructedInFunc(info *types.Info, fn *ast.FuncDecl) map[types.Object]bool {
	built := map[types.Object]bool{}
	record := func(lhs, rhs []ast.Expr) {
		for i, expr := range lhs {
			if i >= len(rhs) || !isProjectLiteral(rhs[i]) {
				continue
			}
			if ident, ok := expr.(*ast.Ident); ok {
				if obj := info.ObjectOf(ident); obj != nil {
					built[obj] = true
				}
			}
		}
	}
	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			record(node.Lhs, node.Rhs)
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if i < len(node.Values) && isProjectLiteral(node.Values[i]) {
					if obj := info.ObjectOf(name); obj != nil {
						built[obj] = true
					}
				}
			}
		}
		return true
	})
	return built
}

// selectorRoot returns the leftmost identifier of a selector chain.
func selectorRoot(expr ast.Expr) *ast.Ident {
	for {
		switch node := expr.(type) {
		case *ast.Ident:
			return node
		case *ast.SelectorExpr:
			expr = node.X
		case *ast.IndexExpr:
			expr = node.X
		case *ast.ParenExpr:
			expr = node.X
		case *ast.StarExpr:
			expr = node.X
		default:
			return nil
		}
	}
}

// projectFieldWriteFindings reports every assignment to a *Project field whose
// root identifier the enclosing function did not itself construct from a
// composite literal. That is exactly the shape ADR-0180 removes: a function
// writing a field of a Project value that outlives its call.
//
// The rule admits the three stepwise constructions ADR-0180 item 10 names as
// conforming (Loader.Open, ContextForOptions, StagedContextRootOptions), each
// of which writes fields of a value whose literal appears in the same function,
// and rejects a method mutating its receiver.
func projectFieldWriteFindings(pkgs []*packages.Package) []string {
	var findings []string
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok || funcDecl.Body == nil {
					continue
				}
				built := constructedInFunc(pkg.TypesInfo, funcDecl)
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					assign, ok := n.(*ast.AssignStmt)
					if !ok {
						return true
					}
					for _, lhs := range assign.Lhs {
						sel, ok := lhs.(*ast.SelectorExpr)
						if !ok || !isProjectValue(pkg.TypesInfo, sel.X) {
							continue
						}
						root := selectorRoot(sel.X)
						if root == nil {
							continue
						}
						if obj := pkg.TypesInfo.ObjectOf(root); obj != nil && built[obj] {
							continue
						}
						pos := pkg.Fset.Position(sel.Pos())
						findings = append(findings, funcDecl.Name.Name+" writes "+
							root.Name+"."+sel.Sel.Name+" at "+
							filepath.ToSlash(filepath.Base(pos.Filename))+":"+
							strconv.Itoa(pos.Line))
					}
					return true
				})
			}
		}
	}
	sort.Strings(findings)
	return findings
}

// TestProjectDerivedStateOwnership proves that no production function in
// internal/project writes a *Project field outside the function that constructs
// that value: the ADR corpus, topic corpus, and effective skill set are derived
// by the operation that needs them and threaded to their consumers, and
// beginInvocation no longer exists.
//
// The scan covers package functions as well as methods, because
// StagedContextRootOptions is a function rather than a method.
// invariant: code-design/state-ownership:project-derived-state-ownership
func TestProjectDerivedStateOwnership(t *testing.T) {
	if findings := projectFieldWriteFindings(loadProjectPackage(t, nil)); len(findings) != 0 {
		t.Errorf("*Project fields are written outside the function that constructs the value:\n\t%s",
			strings.Join(findings, "\n\t"))
	}

	// Committed negative case: a method mutating its receiver must be flagged,
	// so the detector cannot silently stop detecting.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, filepath.FromSlash("internal/project/state_ownership_mutation_fixture.go"))
	mutation := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

func (p *Project) mutationWritesAfterConstruction() {
	p.Root = "mutated"
}
`)})
	findings := projectFieldWriteFindings(mutation)
	var flagged bool
	for _, f := range findings {
		if strings.HasPrefix(f, "mutationWritesAfterConstruction writes p.Root") {
			flagged = true
		}
	}
	if !flagged {
		t.Fatalf("receiver mutation escaped the detector: %#v", findings)
	}

	// The same fixture, written through a locally constructed value, must NOT
	// be flagged: the rule turns on construction in the same function, not on
	// the mere presence of a field write.
	conforming := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

func mutationConstructsLocally(rootDir string) *Project {
	built := &Project{Root: rootDir}
	built.Cfg = nil
	return built
}
`)})
	for _, f := range projectFieldWriteFindings(conforming) {
		if strings.HasPrefix(f, "mutationConstructsLocally") {
			t.Fatalf("a write to a locally constructed value was flagged: %q", f)
		}
	}
}
