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
// conforming (Loader.Open plus, since ADR-0194 carved the context query out,
// the two ContextState constructors Project.ContextState and
// StagedContextState), each of which writes fields of a value whose literal
// appears in the same function, and rejects a method mutating its receiver.
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
				report := func(sel *ast.SelectorExpr, how string) {
					if !isProjectValue(pkg.TypesInfo, sel.X) {
						return
					}
					root := selectorRoot(sel.X)
					if root == nil {
						return
					}
					if obj := pkg.TypesInfo.ObjectOf(root); obj != nil && built[obj] {
						return
					}
					pos := pkg.Fset.Position(sel.Pos())
					findings = append(findings, funcDecl.Name.Name+" "+how+" "+
						root.Name+"."+sel.Sel.Name+" at "+
						filepath.ToSlash(filepath.Base(pos.Filename))+":"+
						strconv.Itoa(pos.Line))
				}
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.AssignStmt:
						for _, lhs := range node.Lhs {
							switch target := lhs.(type) {
							case *ast.SelectorExpr:
								// x.field = v, and x.slice[i].field = v once the
								// index is unwrapped by selectorRoot.
								report(target, "writes")
							case *ast.StarExpr:
								// *x = Project{...} replaces every field at once.
								if isProjectValue(pkg.TypesInfo, target.X) {
									if root := selectorRoot(target.X); root != nil {
										if obj := pkg.TypesInfo.ObjectOf(root); obj == nil || !built[obj] {
											pos := pkg.Fset.Position(target.Pos())
											findings = append(findings, funcDecl.Name.Name+" replaces the whole value "+
												root.Name+" at "+filepath.ToSlash(filepath.Base(pos.Filename))+":"+
												strconv.Itoa(pos.Line))
										}
									}
								}
							case *ast.IndexExpr:
								// x.slice[i] = v writes through a field.
								if sel, ok := target.X.(*ast.SelectorExpr); ok {
									report(sel, "writes through an index into")
								}
							}
						}
					case *ast.UnaryExpr:
						// &x.field hands the field out to be written elsewhere.
						if node.Op == token.AND {
							if sel, ok := node.X.(*ast.SelectorExpr); ok {
								report(sel, "takes the address of")
							}
						}
					}
					return true
				})
			}
		}
	}
	sort.Strings(findings)
	return findings
}

// derivingEntries collects, per enclosing function name, the production call
// sites of deriveOperationState. Clause 2 of the claim says the operation that
// needs the state derives it and threads it, so exactly the deriving entries
// may call it and nothing nested may re-derive.
func derivingEntries(pkgs []*packages.Package) map[string]int {
	callers := map[string]int{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok || funcDecl.Body == nil {
					continue
				}
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "deriveOperationState" {
						callers[funcDecl.Name.Name]++
					}
					return true
				})
			}
		}
	}
	return callers
}

// declaredFuncNames lists every production function and method name.
func declaredFuncNames(pkgs []*packages.Package) map[string]bool {
	names := map[string]bool{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					names[funcDecl.Name.Name] = true
				}
			}
		}
	}
	return names
}

// producerCallSites collects, per enclosing function name, the production call
// sites of the derivation producers named in the claim. Clause 2 is about those
// values, so counting deriveOperationState alone would miss a nested consumer
// that calls a producer directly and bypasses the aggregate.
func producerCallSites(pkgs []*packages.Package) map[string][]string {
	producers := map[string]bool{"LoadCorpus": true, "effectiveSkills": true}
	sites := map[string][]string{}
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if !ok || funcDecl.Body == nil {
					continue
				}
				ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					sel, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || !producers[sel.Sel.Name] {
						return true
					}
					name := sel.Sel.Name
					if root := selectorRoot(sel.X); root != nil {
						name = root.Name + "." + name
					}
					sites[name] = append(sites[name], funcDecl.Name.Name)
					return true
				})
			}
		}
	}
	return sites
}

// TestProjectDerivedStateOwnership proves that no production function in
// internal/project writes a *Project field outside the function that constructs
// that value: the ADR corpus, topic corpus, and effective skill set are derived
// by the operation that needs them and threaded to their consumers, and
// beginInvocation no longer exists.
//
// The scan covers package functions as well as methods, because
// StagedContextState is a function rather than a method.
// invariant: code-design/state-ownership:project-derived-state-ownership
func TestProjectDerivedStateOwnership(t *testing.T) {
	production := loadProjectPackage(t, nil)
	if findings := projectFieldWriteFindings(production); len(findings) != 0 {
		t.Errorf("*Project fields are written outside the function that constructs the value:\n\t%s",
			strings.Join(findings, "\n\t"))
	}

	// Clause 3: beginInvocation no longer exists. A field-write scan alone
	// cannot see a per-invocation reset that clears state held anywhere else,
	// so the claim's own words are asserted directly.
	if declaredFuncNames(production)["beginInvocation"] {
		t.Error("beginInvocation is declared again; the claim says it no longer exists")
	}

	// Clause 2: the operation that needs the state derives it and threads it.
	// Exactly the deriving entries call deriveOperationState, each once, so a
	// nested re-derivation is a failure rather than an invisible regression.
	wantEntries := map[string]bool{
		"Check": true, "syncReport": true, "AdvisoryNotes": true,
		"ConfigReferenceModel": true, "OutputPlan": true,
	}
	entries := derivingEntries(production)
	for name, count := range entries {
		if !wantEntries[name] {
			t.Errorf("%s derives operation state; only a deriving entry may, everything nested receives", name)
		} else if count != 1 {
			t.Errorf("%s derives operation state %d times; a deriving entry derives exactly once", name, count)
		}
	}
	for name := range wantEntries {
		if entries[name] == 0 {
			t.Errorf("%s no longer derives operation state at its own entry", name)
		}
	}

	// Clause 2, at the level of the values themselves: each producer is called
	// from exactly one production function, deriveOperationState. Counting the
	// aggregate alone would miss a consumer calling a producer directly.
	for producer, owners := range producerCallSites(production) {
		for _, owner := range owners {
			if owner != "deriveOperationState" {
				t.Errorf("%s calls %s directly; only deriveOperationState produces the threaded values", owner, producer)
			}
		}
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

	// The detector must also see a field handed out by address, which is how a
	// post-construction write escapes an assignment-only scan.
	byAddress := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

func writeThrough(target *string, value string) { *target = value }

func (p *Project) mutationWritesViaPointer() {
	writeThrough(&p.Root, "mutated")
}
`)})
	var addressFlagged bool
	for _, f := range projectFieldWriteFindings(byAddress) {
		if strings.HasPrefix(f, "mutationWritesViaPointer takes the address of p.Root") {
			addressFlagged = true
		}
	}
	if !addressFlagged {
		t.Error("a *Project field handed out by address escaped the detector")
	}

	// A nested re-derivation is the regression the whole conversion prevents.
	nested := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

func (p *Project) mutationRederivesNested() {
	_, _, _, _ = p.deriveOperationState()
}
`)})
	if derivingEntries(nested)["mutationRederivesNested"] != 1 {
		t.Error("a nested deriveOperationState call escaped the deriving-entry scan")
	}

	// A consumer calling a producer directly bypasses the aggregate entirely
	// and writes no field, so only the producer scan can see it.
	direct := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

import "github.com/hypnotox/agentic-workflows/internal/adr"

func (p *Project) mutationRederivesCorpusDirectly() (adr.Corpus, error) {
	return adr.LoadCorpus(p.decisionsDir())
}
`)})
	var directFlagged bool
	for _, owners := range producerCallSites(direct) {
		for _, owner := range owners {
			if owner == "mutationRederivesCorpusDirectly" {
				directFlagged = true
			}
		}
	}
	if !directFlagged {
		t.Error("a direct producer call bypassing deriveOperationState escaped the producer scan")
	}

	// Replacing the whole value writes every field at once.
	wholesale := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

func (p *Project) mutationOverwritesWholeValue() {
	*p = Project{Root: "mutated"}
}
`)})
	var wholesaleFlagged bool
	for _, f := range projectFieldWriteFindings(wholesale) {
		if strings.HasPrefix(f, "mutationOverwritesWholeValue replaces the whole value p") {
			wholesaleFlagged = true
		}
	}
	if !wholesaleFlagged {
		t.Error("a wholesale *p = Project{...} overwrite escaped the detector")
	}
}
