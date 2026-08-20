package project

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// projectPackageMode is the loader mode the state-ownership scan needs: syntax
// to walk assignments and type information to tell a watched selector from any
// other receiver with the same field name.
const projectPackageMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo

// stateOwnershipPatterns are the packages the widened claim quantifies over:
// the sync core and both ADR-0195 carves.
var stateOwnershipPatterns = []string{"./internal/project", "./internal/contextq", "./internal/resident"}

// watchedLongLivedTypes are each package's constructed long-lived values: a
// field write outside the constructing function is the shape the claim
// forbids. The conforming constructions are Loader.Open, the two ContextState
// constructors, contextq.New, and resident.NewRoots.
var watchedLongLivedTypes = map[string]map[string]bool{
	"github.com/hypnotox/agentic-workflows/internal/project":  {"Project": true, "projectState": true, "ContextState": true},
	"github.com/hypnotox/agentic-workflows/internal/contextq": {"Query": true},
	"github.com/hypnotox/agentic-workflows/internal/resident": {"Roots": true},
}

// loadProjectPackage loads the three state-owned packages, optionally
// overlaying one file so a negative case can be committed rather than
// hand-mutated.
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
	}, stateOwnershipPatterns...)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != len(stateOwnershipPatterns) {
		t.Fatalf("loaded %d packages for %v, want %d", len(pkgs), stateOwnershipPatterns, len(stateOwnershipPatterns))
	}
	for _, pkg := range pkgs {
		if len(pkg.Errors) != 0 {
			t.Fatal(pkg.Errors[0])
		}
		// Every loaded package must have a watched-type entry: a pattern added
		// without one (or a key drifting on a rename) would scan green while
		// watching nothing, which is silent non-enforcement.
		if len(watchedLongLivedTypes[pkg.PkgPath]) == 0 {
			t.Fatalf("package %s is scanned but has no watched long-lived types", pkg.PkgPath)
		}
	}
	return pkgs
}

// isProjectValue reports whether expr's type is one of the watched long-lived
// values (or a pointer to one).
func isProjectValue(info *types.Info, expr ast.Expr) bool {
	typ := info.TypeOf(expr)
	if typ == nil {
		return false
	}
	if ptr, ok := typ.Underlying().(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	return watchedLongLivedTypes[named.Obj().Pkg().Path()][named.Obj().Name()]
}

// isProjectLiteral reports whether expr constructs a watched value directly,
// as `T{...}`, `&T{...}`, or the qualified `pkg.T{...}` forms.
func isProjectLiteral(expr ast.Expr) bool {
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		expr = unary.X
	}
	lit, ok := expr.(*ast.CompositeLit)
	if !ok {
		return false
	}
	name := ""
	switch t := lit.Type.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.SelectorExpr:
		name = t.Sel.Name
	}
	for _, names := range watchedLongLivedTypes {
		if names[name] {
			return true
		}
	}
	return false
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
// conforming (Loader.Open plus, since ADR-0195 carved the context query out,
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
// sites of deriveOperationStateWithPitfalls. Clause 2 of the claim says the operation that
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
					if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "deriveOperationStateWithPitfalls" {
						callers[funcDecl.Name.Name]++
					} else if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "deriveOperationStateWithPitfalls" {
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
// internal/project, internal/contextq, or internal/resident writes a field of
// that package's constructed long-lived values (Project, ContextState, Query,
// Roots) outside the function that constructs the value: the derived state is
// threaded to its consumers, and Roots is fixed at construction. The
// conforming constructions are Loader.Open, the two ContextState constructors
// (Project.ContextState and StagedContextState), contextq.New, and
// resident.NewRoots. The beginInvocation-absence assertion below is retained
// hardening beyond the current claim body, which no longer names it.
//
// The scan covers package functions as well as methods, because
// StagedContextState is a function rather than a method.
// invariant: code-design/state-ownership:project-derived-state-ownership (TestProjectDerivedStateOwnership)
// invariant: rendering/project-output-plan:check-report-single-plan (TestProjectDerivedStateOwnership)
func TestProjectDerivedStateOwnership(t *testing.T) {
	production := loadProjectPackage(t, nil)
	if findings := projectFieldWriteFindings(production); len(findings) != 0 {
		t.Errorf("*Project fields are written outside the function that constructs the value:\n\t%s",
			strings.Join(findings, "\n\t"))
	}

	// Retained hardening: beginInvocation no longer exists. The current claim
	// body no longer names it, but a field-write scan alone cannot see a
	// per-invocation reset that clears state held anywhere else, so the
	// original clause's assertion is kept.
	if declaredFuncNames(production)["beginInvocation"] {
		t.Error("beginInvocation is declared again; the claim says it no longer exists")
	}

	// Clause 2: the operation that needs the state derives it and threads it.
	// Exactly the deriving entries call deriveOperationStateWithPitfalls, each once, so a
	// nested re-derivation is a failure rather than an invisible regression.
	wantEntries := map[string]bool{
		"CheckReportOperation": true, "syncReport": true, "AdvisoryNotesOperation": true,
		"ConfigReferenceModelOperation": true, "OutputPlanOperation": true, "ReadPlanOperation": true,
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

	// Clause 2, at the level of the values themselves: a producer is called only
	// from a function that derives on its own operation's behalf. Counting the
	// aggregate alone would miss a consumer calling a producer directly.
	//
	// numberingCorpus is the one entry beside the aggregate deriver, and only
	// for the ADR corpus. Numbering needs a duplicate-identity corpus as data
	// rather than as an abort (ADR-0202 item 12), which is the opposite of what
	// deriveOperationStateWithPitfalls owes every other consumer, so it cannot enter
	// through it. The set is pinned exactly, so a consumer re-deriving a value
	// that was threaded to it still fails here.
	producerOwners := map[string]bool{"deriveOperationStateWithPitfalls": true, "numberingCorpus": true}
	for producer, owners := range producerCallSites(production) {
		for _, owner := range owners {
			if !producerOwners[owner] {
				t.Errorf("%s calls %s directly; only an operation deriving on its own behalf produces the threaded values", owner, producer)
			}
		}
	}

	// One combined overlay proves every detector branch without paying for a
	// separate packages.Load per mutation shape.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	projectFixture := filepath.Join(root, filepath.FromSlash("internal/project/state_ownership_mutation_fixture.go"))
	contextqFixture := filepath.Join(root, filepath.FromSlash("internal/contextq/state_ownership_mutation_fixture.go"))
	mutation := loadProjectPackage(t, map[string][]byte{
		projectFixture: []byte(`package project

import "github.com/hypnotox/agentic-workflows/internal/adr"

func writeThrough(target *string, value string) { *target = value }

func (p *Project) mutationWritesAfterConstruction() {
	p.Root = "mutated"
}

func (s *projectState) mutationWritesStateAfterConstruction() {
	s.nested = true
}

func mutationConstructsLocally(rootDir string) *Project {
	built := &Project{Root: rootDir}
	built.Cfg = nil
	return built
}

func (p *Project) mutationWritesViaPointer() {
	writeThrough(&p.Root, "mutated")
}

func (p *Project) mutationRederivesNested() {
	_, _, _, _, _ = deriveOperationStateWithPitfalls(p)
}

func (p *Project) mutationRederivesCorpusDirectly() (adr.Corpus, error) {
	return adr.LoadCorpus(decisionsDir(p))
}

func (p *Project) mutationOverwritesWholeValue() {
	*p = Project{Root: "mutated"}
}
`),
		contextqFixture: []byte(`package contextq

import "github.com/hypnotox/agentic-workflows/internal/project"

func (q *Query) mutationReplacesStateAfterConstruction(state project.ContextState) {
	q.state = state
}
`),
	})
	findings := projectFieldWriteFindings(mutation)
	hasPrefix := func(prefix string) bool {
		return slices.ContainsFunc(findings, func(finding string) bool {
			return strings.HasPrefix(finding, prefix)
		})
	}
	for _, want := range []string{
		"mutationWritesAfterConstruction writes p.Root",
		"mutationWritesStateAfterConstruction writes s.nested",
		"mutationWritesViaPointer takes the address of p.Root",
		"mutationOverwritesWholeValue replaces the whole value p",
		"mutationReplacesStateAfterConstruction writes q.state",
	} {
		if !hasPrefix(want) {
			t.Errorf("mutation %q escaped the field-write detector: %#v", want, findings)
		}
	}
	for _, finding := range findings {
		if strings.HasPrefix(finding, "mutationConstructsLocally") {
			t.Errorf("a write to a locally constructed value was flagged: %q", finding)
		}
	}
	if derivingEntries(mutation)["mutationRederivesNested"] != 1 {
		t.Error("a nested deriveOperationState call escaped the deriving-entry scan")
	}
	var directFlagged bool
	for _, owners := range producerCallSites(mutation) {
		if slices.Contains(owners, "mutationRederivesCorpusDirectly") {
			directFlagged = true
		}
	}
	if !directFlagged {
		t.Error("a direct producer call bypassing deriveOperationState escaped the producer scan")
	}
}

func retainsOperationDependency(typ types.Type, seen map[types.Type]bool) bool {
	typ = types.Unalias(typ)
	if seen[typ] {
		return false
	}
	seen[typ] = true
	switch value := typ.(type) {
	case *types.Named:
		if obj := value.Obj(); obj.Pkg() != nil {
			key := obj.Pkg().Path() + "." + obj.Name()
			if key == "github.com/hypnotox/agentic-workflows/internal/config.TreeReader" ||
				key == "github.com/hypnotox/agentic-workflows/internal/config.OperationTree" ||
				key == "github.com/hypnotox/agentic-workflows/internal/git.Repo" ||
				key == "github.com/hypnotox/agentic-workflows/internal/project.ProjectTreeReader" {
				return true
			}
		}
		return retainsOperationDependency(value.Underlying(), seen)
	case *types.Pointer:
		return retainsOperationDependency(value.Elem(), seen)
	case *types.Array:
		return retainsOperationDependency(value.Elem(), seen)
	case *types.Slice:
		return retainsOperationDependency(value.Elem(), seen)
	case *types.Map:
		return retainsOperationDependency(value.Key(), seen) || retainsOperationDependency(value.Elem(), seen)
	case *types.Chan:
		return retainsOperationDependency(value.Elem(), seen)
	case *types.Signature:
		return retainsOperationDependency(value.Params(), seen) || retainsOperationDependency(value.Results(), seen)
	case *types.Tuple:
		for i := range value.Len() {
			if retainsOperationDependency(value.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Interface:
		value.Complete()
		for i := range value.NumMethods() {
			if retainsOperationDependency(value.Method(i).Type(), seen) {
				return true
			}
		}
		for i := range value.NumEmbeddeds() {
			if retainsOperationDependency(value.EmbeddedType(i), seen) {
				return true
			}
		}
	case *types.TypeParam:
		return retainsOperationDependency(value.Constraint(), seen)
	case *types.Union:
		for i := range value.Len() {
			if retainsOperationDependency(value.Term(i).Type(), seen) {
				return true
			}
		}
	case *types.Struct:
		for i := range value.NumFields() {
			if retainsOperationDependency(value.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// TestProjectStateBoundary pins the Phase 1 loaded-fact boundary: state keeps
// only private facts and cannot accidentally retain an operation mechanism.
func TestProjectStateBoundary(t *testing.T) {
	pkgs := loadProjectPackage(t, nil)
	var state *types.Named
	for _, pkg := range pkgs {
		if pkg.PkgPath != "github.com/hypnotox/agentic-workflows/internal/project" {
			continue
		}
		obj := pkg.Types.Scope().Lookup("projectState")
		if obj == nil {
			t.Fatal("projectState is missing")
		}
		var ok bool
		state, ok = obj.Type().(*types.Named)
		if !ok {
			t.Fatalf("projectState type = %T", obj.Type())
		}
	}
	if state == nil {
		t.Fatal("project package was not loaded")
	}
	fields, ok := state.Underlying().(*types.Struct)
	if !ok {
		t.Fatalf("projectState underlying type = %T", state.Underlying())
	}
	for i := range fields.NumFields() {
		field := fields.Field(i)
		if field.Exported() {
			t.Errorf("projectState exports mutable field %s", field.Name())
		}
		typeName := types.TypeString(field.Type(), func(pkg *types.Package) string { return pkg.Path() })
		if retainsOperationDependency(field.Type(), map[types.Type]bool{}) {
			t.Errorf("projectState retains operation dependency %s %s", field.Name(), typeName)
		}
	}

	projectPkg := state.Obj().Pkg()
	for _, name := range []string{"ProjectTreeReader"} {
		if typ := projectPkg.Scope().Lookup(name).Type(); !retainsOperationDependency(typ, map[types.Type]bool{}) {
			t.Errorf("boundary detector did not reject project.%s", name)
		}
	}
	for _, imported := range projectPkg.Imports() {
		if imported.Path() != "github.com/hypnotox/agentic-workflows/internal/config" && imported.Path() != "github.com/hypnotox/agentic-workflows/internal/git" {
			continue
		}
		names := []string{"TreeReader", "OperationTree"}
		if imported.Path() == "github.com/hypnotox/agentic-workflows/internal/git" {
			names = []string{"Repo"}
		}
		for _, name := range names {
			obj := imported.Scope().Lookup(name)
			if obj == nil || !retainsOperationDependency(obj.Type(), map[types.Type]bool{}) {
				t.Errorf("boundary detector did not reject %s.%s", imported.Path(), name)
			}
		}
	}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join(root, filepath.FromSlash("internal/project/state_boundary_mutation_fixture.go"))
	mutated := loadProjectPackage(t, map[string][]byte{fixture: []byte(`package project

import "github.com/hypnotox/agentic-workflows/internal/config"

type hiddenTreeAlias = config.TreeReader
type hiddenTreeInterface interface { Read() config.TreeReader }
type hiddenTreeChannel chan config.TreeReader
type boundaryMutation struct {
	Alias hiddenTreeAlias
	Interface hiddenTreeInterface
	Channel hiddenTreeChannel
}
`)})
	for _, pkg := range mutated {
		if pkg.PkgPath != projectImportPath {
			continue
		}
		probe := pkg.Types.Scope().Lookup("boundaryMutation")
		if probe == nil {
			t.Fatal("boundary mutation fixture was not loaded")
		}
		fields := types.Unalias(probe.Type()).Underlying().(*types.Struct)
		for i := range fields.NumFields() {
			if !retainsOperationDependency(fields.Field(i).Type(), map[types.Type]bool{}) {
				t.Errorf("boundary detector missed wrapped dependency field %s", fields.Field(i).Name())
			}
		}
	}
}
