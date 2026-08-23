package testsupport_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/clispec"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"golang.org/x/tools/go/packages"
)

// TestThinCommandCompositionCensus is the terminal composition oracle. It
// derives command paths from clispec, then proves the corresponding real
// handler can reach precisely the route body that calls its semantic owner.
// It deliberately works on typed syntax, rather than text or package-wide call
// census: comments, dead code, aliases, closures, and method values are not
// evidence.
func TestThinCommandCompositionCensus(t *testing.T) {
	pkg := loadAWFCommandPackage(t)
	routes := commandRoutes()
	want := executableCommands(clispec.Commands, nil)

	for path := range want {
		if _, ok := routes[path]; !ok {
			t.Errorf("clispec executable command %q has no production route", path)
		}
	}
	for path := range routes {
		if !want[path] {
			t.Errorf("production route %q has no executable clispec command", path)
		}
	}

	handlers := typedHandlers(t, pkg)
	verifyRuntimeRouteCensus(t, pkg, routes)
	for _, route := range routes {
		handler, ok := handlers[route.top]
		if !ok {
			t.Errorf("route %q has no handler for top-level %q", route.path, route.top)
			continue
		}
		if !reachesLocal(pkg, handler.Body, route.root) {
			t.Errorf("route %q root %s is not reachable from its handler", route.path, route.root)
			continue
		}
		body := routedBody(t, pkg, route)
		got, evasions := operationCalls(pkg, body)
		evasions = unexpectedRouteEvasions(route.path, evasions)
		if len(evasions) != 0 {
			t.Errorf("route %q hides semantic operations through %v", route.path, evasions)
		}
		if route.bypass {
			if len(got) != 0 {
				t.Errorf("bypass route %q unexpectedly calls semantic operations %v", route.path, got)
			}
			continue
		}
		if !sameOperations(got, route.operations) {
			t.Errorf("route %q operations = %v, want %v", route.path, got, route.operations)
		}
	}
	verifyVariantCardinality(t, pkg, routes)
	for top := range handlers {
		if _, ok := clispec.Lookup(top); !ok {
			t.Errorf("runtime handler %q has no clispec command", top)
		}
	}
}

type commandRoute struct {
	path, top, root string
	operations      []string // package path + "." + function name
	bypass          bool
}

// commandRoutes is intentionally one entry per resolved invocation. Multiple
// operations mean mutually exclusive invocation variants in that one route
// body, never an aggregate invocation. The proof below requires this exact
// set, so a new flag branch or dispatch leaf fails closed until represented.
func commandRoutes() map[string]commandRoute {
	entries := []commandRoute{
		{"init", "init", "runInitWithProjectLoader", []string{"github.com/hypnotox/agentic-workflows/internal/initop.Run"}, false},
		{"render", "render", "runSyncPrinting", []string{"github.com/hypnotox/agentic-workflows/internal/publisher.Sync"}, false},
		{"check", "check", "runCheck", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check commit-policy", "check", "runCommitPolicy", []string{"github.com/hypnotox/agentic-workflows/internal/project.VerifyCommitPolicyAt"}, false},
		{"check repo", "check", "runCheckRepo", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check repo drift", "check", "runCheckDrift", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check repo state", "check", "runCheckState", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check repo prose", "check", "runProseGate", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check repo memory", "check", "runMemoryGate", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check staged", "check", "runCheckStaged", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check staged state", "check", "runCheckStagedState", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check staged drift", "check", "runCheckStagedDrift", []string{"github.com/hypnotox/agentic-workflows/internal/checkop.Run"}, false},
		{"check staged commit", "check", "runCommitGateWithDependencies", []string{"github.com/hypnotox/agentic-workflows/internal/commitgateop.RunWithDependencies"}, false},
		{"read plan", "read", "runReadPlan", []string{"github.com/hypnotox/agentic-workflows/internal/currentstatecoord.ReadPlan"}, false},
		{"audit", "audit", "runAudit", []string{"github.com/hypnotox/agentic-workflows/internal/audit.RunConfigured"}, false},
		{"effort new", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.New"}, false},
		{"effort list", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.List"}, false},
		{"effort show", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.Show"}, false},
		{"effort finish", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.Finish"}, false},
		{"effort worktree", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.AddWorktree", "github.com/hypnotox/agentic-workflows/internal/effortop.RemoveWorktree"}, false},
		{"effort integrate", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.Integrate"}, false},
		{"effort memory read", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.ReadMemory"}, false},
		{"effort memory edit", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.EditMemory"}, false},
		{"effort memory update", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.UpdateMemory"}, false},
		{"effort activity attach", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.AttachActivity"}, false},
		{"effort activity heartbeat", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.HeartbeatActivity"}, false},
		{"effort activity detach", "effort", "runEffort", []string{"github.com/hypnotox/agentic-workflows/internal/effortop.DetachActivity"}, false},
		{"adr number", "adr", "runADR", []string{"github.com/hypnotox/agentic-workflows/internal/currentstatecoord.NumberPendingADRs"}, false},
		{"list", "list", "runList", []string{"github.com/hypnotox/agentic-workflows/internal/project.BuildListDocument"}, false},
		{"config", "config", "runConfig", []string{"github.com/hypnotox/agentic-workflows/internal/publisher.ConfigReferencePresentation", "github.com/hypnotox/agentic-workflows/internal/publisher.ConfigReferencePresentation"}, false},
		{"context", "context", "runContextWithDelivery", []string{"github.com/hypnotox/agentic-workflows/internal/contextop.Run"}, false},
		{"topic", "topic", "runTopic", []string{"github.com/hypnotox/agentic-workflows/internal/topicop.Run"}, false},
		{"new adr", "new", "newADR", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewADR"}, false},
		{"new plan", "new", "newPlan", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewPlan"}, false},
		{"new topic", "new", "newTopic", []string{"github.com/hypnotox/agentic-workflows/internal/topicop.Create"}, false},
		{"new domain", "new", "runNewDomain", []string{"github.com/hypnotox/agentic-workflows/internal/domainop.Add"}, false},
		{"new doc", "new", "newDoc", []string{"github.com/hypnotox/agentic-workflows/internal/localdocop.Run"}, false},
		{"new pitfall", "new", "newPitfall", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewPitfall"}, false},
		{"remove domain", "remove", "runRemoveDomain", []string{"github.com/hypnotox/agentic-workflows/internal/domainop.Remove"}, false},
		{"upgrade", "upgrade", "runUpgradeFlags", []string{"github.com/hypnotox/agentic-workflows/internal/upgrade.RecoverOperation", "github.com/hypnotox/agentic-workflows/internal/upgrade.Run"}, false},
		{"uninstall", "uninstall", "runUninstall", []string{"github.com/hypnotox/agentic-workflows/internal/resident.Uninstall"}, false},
		{"changelog", "changelog", "runChangelog", []string{"github.com/hypnotox/agentic-workflows/internal/changelog.Embedded", "github.com/hypnotox/agentic-workflows/internal/changelog.EmbeddedRange", "github.com/hypnotox/agentic-workflows/internal/changelog.EmbeddedSince", "github.com/hypnotox/agentic-workflows/internal/changelog.EmbeddedVersion"}, false},
		{"version", "version", "runVersion", nil, true},
	}
	out := make(map[string]commandRoute, len(entries))
	for _, entry := range entries {
		out[entry.path] = entry
	}
	return out
}

func executableCommands(commands []clispec.Command, prefix []string) map[string]bool {
	out := map[string]bool{}
	var walk func([]clispec.Command, []string)
	walk = func(nodes []clispec.Command, parent []string) {
		for _, node := range nodes {
			path := append(append([]string(nil), parent...), node.Name)
			joined := strings.Join(path, " ")
			if len(node.Children) == 0 || joined == "check" || joined == "check repo" || joined == "check staged" {
				out[joined] = true
			}
			walk(node.Children, path)
		}
	}
	walk(commands, prefix)
	return out
}

func loadAWFCommandPackage(t *testing.T) *packages.Package {
	t.Helper()
	return loadAWFCommandPackageWithOverlay(t, nil)
}

func loadAWFCommandPackageWithOverlay(t *testing.T, overlay map[string][]byte) *packages.Package {
	t.Helper()
	root := testsupport.RepoRoot(t)
	loaded, err := packages.Load(&packages.Config{
		Dir: root, Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Overlay: overlay,
	}, "./cmd/awf")
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(loaded) != 0 || len(loaded) != 1 {
		t.Fatalf("load cmd/awf packages = %d", len(loaded))
	}
	return loaded[0]
}

func typedHandlers(t *testing.T, pkg *packages.Package) map[string]*ast.FuncLit {
	t.Helper()
	out := map[string]*ast.FuncLit{}
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || value.Names[0].Name != "handlers" || len(value.Values) != 1 {
					continue
				}
				literal, ok := value.Values[0].(*ast.CompositeLit)
				if !ok {
					t.Fatalf("handlers is not a composite literal")
				}
				for _, elt := range literal.Elts {
					pair, ok := elt.(*ast.KeyValueExpr)
					key, keyOK := stringLiteral(pair.Key)
					fn, fnOK := pair.Value.(*ast.FuncLit)
					if !ok || !keyOK || !fnOK {
						t.Fatalf("unrecognized handlers entry")
					}
					out[key] = fn
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("typed handlers registry not found")
	}
	return out
}

func functionBody(t *testing.T, pkg *packages.Package, name string) *ast.BlockStmt {
	t.Helper()
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name && fn.Body != nil {
				return fn.Body
			}
		}
	}
	t.Fatalf("route root %s not found", name)
	return nil
}

func reachesLocal(pkg *packages.Package, body *ast.BlockStmt, want string) bool {
	seen := map[*types.Func]bool{}
	var visit func(*ast.BlockStmt) bool
	visit = func(block *ast.BlockStmt) bool {
		found := false
		ast.Inspect(block, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := directFunction(pkg.TypesInfo, call)
			if !ok {
				return true
			}
			if fn.Pkg() == pkg.Types && fn.Name() == want {
				found = true
				return false
			}
			if fn.Pkg() == pkg.Types && !seen[fn] {
				seen[fn] = true
				if visit(functionBodyByObject(pkg, fn)) {
					found = true
					return false
				}
			}
			return true
		})
		return found
	}
	return visit(body)
}
func functionBodyByObject(pkg *packages.Package, object *types.Func) *ast.BlockStmt {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && pkg.TypesInfo.Defs[fn.Name] == object {
				return fn.Body
			}
		}
	}
	return nil
}

func unexpectedRouteEvasions(path string, evasions []string) []string {
	// ADR numbering supplies Publisher synchronization as the declared concrete
	// callback dependency of the one currentstatecoord operation. The callback
	// is not invoked by command code and cannot satisfy the route operation.
	if path != "adr number" {
		return evasions
	}
	out := evasions[:0]
	for _, evasion := range evasions {
		if evasion != "github.com/hypnotox/agentic-workflows/internal/publisher.Sync" {
			out = append(out, evasion)
		}
	}
	return out
}

func routedBody(t *testing.T, pkg *packages.Package, route commandRoute) *ast.BlockStmt {
	t.Helper()
	body := functionBody(t, pkg, route.root)
	if route.root != "runEffort" {
		return body
	}
	name := strings.TrimPrefix(route.path, "effort ")
	clause := commandSwitchCases(t, body)[name]
	if clause == nil {
		t.Fatalf("route %q has no runEffort switch arm", route.path)
	}
	return &ast.BlockStmt{List: clause.Body}
}

// verifyRuntimeRouteCensus reads the actual nested dispatch switches. The
// clispec/table comparison catches additions on the specification side; these
// comparisons independently catch additions, removals, and rerouting in the
// production dispatcher.
func verifyRuntimeRouteCensus(t *testing.T, pkg *packages.Package, routes map[string]commandRoute) {
	t.Helper()
	for _, group := range []struct {
		root, prefix string
	}{
		{"runCheckGroup", "check "},
		{"runEffort", "effort "},
	} {
		cases := commandSwitchCases(t, functionBody(t, pkg, group.root))
		expected := map[string]commandRoute{}
		for path, route := range routes {
			if strings.HasPrefix(path, group.prefix) {
				expected[strings.TrimPrefix(path, group.prefix)] = route
			}
		}
		if group.root == "runCheckGroup" {
			delete(expected, "")
		}
		if len(cases) != len(expected) {
			t.Errorf("%s runtime leaves = %v, want %v", group.root, sortedCaseNames(cases), sortedRouteNames(expected))
		}
		for name, route := range expected {
			clause := cases[name]
			if clause == nil {
				t.Errorf("%s runtime leaf %q is absent", group.root, name)
				continue
			}
			if group.root == "runCheckGroup" && !reachesLocal(pkg, &ast.BlockStmt{List: clause.Body}, route.root) {
				t.Errorf("%s runtime leaf %q does not reach %s", group.root, name, route.root)
			}
		}
		for name := range cases {
			if _, ok := expected[name]; !ok {
				t.Errorf("%s runtime leaf %q has no route contract", group.root, name)
			}
		}
	}
	verifyNewRuntimeRoutes(t, pkg, routes)
}

func commandSwitchCases(t *testing.T, body *ast.BlockStmt) map[string]*ast.CaseClause {
	t.Helper()
	var route *ast.SwitchStmt
	ast.Inspect(body, func(node ast.Node) bool {
		statement, ok := node.(*ast.SwitchStmt)
		if !ok || route != nil {
			return true
		}
		selector, ok := statement.Tag.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "sub" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if ok && receiver.Name == "c" {
			route = statement
			return false
		}
		return true
	})
	if route == nil {
		t.Fatal("command subcommand switch is absent")
	}
	out := map[string]*ast.CaseClause{}
	for _, item := range route.Body.List {
		clause := item.(*ast.CaseClause)
		if len(clause.List) == 0 {
			continue
		}
		for _, expression := range clause.List {
			name, ok := stringLiteral(expression)
			if !ok || out[name] != nil {
				t.Fatalf("unrecognized or duplicate command switch case")
			}
			out[name] = clause
		}
	}
	return out
}

func verifyNewRuntimeRoutes(t *testing.T, pkg *packages.Package, routes map[string]commandRoute) {
	t.Helper()
	body := functionBody(t, pkg, "runNew")
	var route *ast.SwitchStmt
	for _, statement := range body.List {
		if candidate, ok := statement.(*ast.SwitchStmt); ok && candidate.Tag == nil {
			route = candidate
			break
		}
	}
	if route == nil {
		t.Fatal("runNew dispatch switch is absent")
	}
	seen := map[string]bool{"doc": true} // production handler selects doc before runNew
	for _, item := range route.Body.List {
		clause := item.(*ast.CaseClause)
		if len(clause.List) == 0 {
			continue
		}
		kind := newCaseKind(clause.List[0])
		if kind == "doc" {
			continue
		}
		routeContract, ok := routes["new "+kind]
		if kind == "" || !ok || seen[kind] {
			t.Fatalf("unrecognized or duplicate runNew runtime leaf %q", kind)
		}
		seen[kind] = true
		if !reachesLocal(pkg, &ast.BlockStmt{List: clause.Body}, routeContract.root) {
			t.Errorf("runNew runtime leaf %q does not reach %s", kind, routeContract.root)
		}
	}
	for path := range routes {
		if strings.HasPrefix(path, "new ") && !seen[strings.TrimPrefix(path, "new ")] {
			t.Errorf("new runtime leaf %q is absent", strings.TrimPrefix(path, "new "))
		}
	}
}

func newCaseKind(expression ast.Expr) string {
	switch condition := expression.(type) {
	case *ast.BinaryExpr:
		if value, ok := stringLiteral(condition.Y); ok {
			return value
		}
		if ident, ok := condition.Y.(*ast.Ident); ok && ident.Name == "localDocumentKind" {
			return "doc"
		}
	case *ast.CallExpr:
		if selector, ok := condition.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "IsFreeformDomainKind" {
			return "domain"
		}
	}
	return ""
}

func sortedCaseNames(values map[string]*ast.CaseClause) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sortedRouteNames(values map[string]commandRoute) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func verifyVariantCardinality(t *testing.T, pkg *packages.Package, routes map[string]commandRoute) {
	t.Helper()
	worktree := routedBody(t, pkg, routes["effort worktree"])
	branch := firstIf(worktree.List)
	if branch == nil || branch.Else == nil {
		t.Fatal("effort worktree does not have add/remove branches")
	}
	assertBranchOperations(t, pkg, "effort worktree add", branch.Body, []string{"github.com/hypnotox/agentic-workflows/internal/effortop.AddWorktree"})
	assertBranchOperations(t, pkg, "effort worktree remove", statementBlock(branch.Else), []string{"github.com/hypnotox/agentic-workflows/internal/effortop.RemoveWorktree"})

	config := functionBody(t, pkg, "runConfig")
	branch = firstIf(config.List)
	if branch == nil {
		t.Fatal("config static/live branch is absent")
	}
	configOperation := []string{"github.com/hypnotox/agentic-workflows/internal/publisher.ConfigReferencePresentation"}
	assertBranchOperations(t, pkg, "config static", branch.Body, configOperation)
	assertBranchOperations(t, pkg, "config live", &ast.BlockStmt{List: config.List[1:]}, configOperation)

	upgrade := functionBody(t, pkg, "runUpgradeFlags")
	branch = firstIf(upgrade.List)
	if branch == nil {
		t.Fatal("upgrade mode branch is absent")
	}
	assertBranchOperations(t, pkg, "upgrade recover", branch.Body, []string{"github.com/hypnotox/agentic-workflows/internal/upgrade.RecoverOperation"})
	assertBranchOperations(t, pkg, "upgrade ordinary", &ast.BlockStmt{List: upgrade.List[1:]}, []string{"github.com/hypnotox/agentic-workflows/internal/upgrade.Run"})

	changelog := functionBody(t, pkg, "runChangelog")
	var selection *ast.SwitchStmt
	for _, statement := range changelog.List {
		if candidate, ok := statement.(*ast.SwitchStmt); ok {
			selection = candidate
		}
	}
	if selection == nil || len(selection.Body.List) != len(routes["changelog"].operations) {
		t.Fatal("changelog variant switch does not cover the closed query set")
	}
	seen := map[string]bool{}
	for i, item := range selection.Body.List {
		clause := item.(*ast.CaseClause)
		got, evasions := operationCalls(pkg, &ast.BlockStmt{List: clause.Body})
		if len(evasions) != 0 || len(got) != 1 {
			t.Errorf("changelog variant %d operations = %v, evasions = %v, want exactly one", i, got, evasions)
			continue
		}
		seen[got[0]] = true
	}
	if len(seen) != len(routes["changelog"].operations) {
		t.Errorf("changelog variant operations = %v, want %v", seen, routes["changelog"].operations)
	}
}

func statementBlock(statement ast.Stmt) *ast.BlockStmt {
	if block, ok := statement.(*ast.BlockStmt); ok {
		return block
	}
	return &ast.BlockStmt{List: []ast.Stmt{statement}}
}

func assertBranchOperations(t *testing.T, pkg *packages.Package, name string, body *ast.BlockStmt, want []string) {
	t.Helper()
	got, evasions := operationCalls(pkg, body)
	if len(evasions) != 0 || !sameOperations(got, want) {
		t.Errorf("%s operations = %v, evasions = %v, want %v", name, got, evasions, want)
	}
}

// operationCalls returns every known semantic operation reached from body,
// retaining duplicates so a second invocation cannot hide behind set equality.
// Any operation selected as a value, through a constructor chain, or inside a
// closure is reported as an evasion instead of operation evidence.
func operationCalls(pkg *packages.Package, body *ast.BlockStmt) ([]string, []string) {
	var names []string
	for _, route := range commandRoutes() {
		names = append(names, route.operations...)
	}
	return operationCallsForOwners(pkg, body, names, semanticOwnerPackages())
}

func operationCallsFor(pkg *packages.Package, body *ast.BlockStmt, names []string) ([]string, []string) {
	return operationCallsForOwners(pkg, body, names, nil)
}

func operationCallsForOwners(pkg *packages.Package, body *ast.BlockStmt, names []string, ownerPackages map[string]bool) ([]string, []string) {
	known := semanticOperationNames(names)
	boundary := allowedOwnerBoundaryFunctions()
	var calls, evasions []string
	seenLocal := map[*types.Func]bool{}
	var visit func(*ast.BlockStmt, bool)
	visit = func(block *ast.BlockStmt, concealed bool) {
		if block == nil {
			return
		}
		var stack []ast.Node
		closureDepth := 0
		ast.Inspect(block, func(node ast.Node) bool {
			if node == nil {
				popped := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if _, ok := popped.(*ast.FuncLit); ok {
					closureDepth--
				}
				return true
			}
			var parent ast.Node
			if len(stack) > 0 {
				parent = stack[len(stack)-1]
			}
			stack = append(stack, node)
			if _, ok := node.(*ast.FuncLit); ok {
				closureDepth++
			}

			expression, isExpression := node.(ast.Expr)
			if isExpression {
				if ident, ok := expression.(*ast.Ident); ok {
					if selector, selected := parent.(*ast.SelectorExpr); selected && selector.Sel == ident {
						return true
					}
				}
				if fn := expressionFunction(pkg.TypesInfo, expression); fn != nil {
					name := qualifiedFunction(fn)
					if known[name] || ownerPackages[functionPackagePath(fn)] && !boundary[name] {
						call, isCall := parent.(*ast.CallExpr)
						var direct *types.Func
						isDirect := false
						if isCall {
							direct, isDirect = directFunction(pkg.TypesInfo, call)
						}
						if isCall && call.Fun == expression && isDirect && direct == fn && closureDepth == 0 && !concealed {
							calls = append(calls, name)
						} else {
							evasions = append(evasions, name)
						}
					}
				}
			}

			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, direct := directFunction(pkg.TypesInfo, call)
			if direct && fn.Pkg() == pkg.Types && !seenLocal[fn] {
				seenLocal[fn] = true
				visit(functionBodyByObject(pkg, fn), concealed || closureDepth > 0)
			}
			return true
		})
	}
	visit(body, false)
	sort.Strings(calls)
	sort.Strings(evasions)
	return calls, evasions
}

func semanticOperationNames(names []string) map[string]bool {
	known := map[string]bool{}
	for _, name := range names {
		known[name] = true
	}
	return known
}

// semanticOwnerPackages is independent of the route contracts it checks. It
// names every established application owner and protected lower owner whose
// semantic entry points may not be invoked as an extra command operation.
func semanticOwnerPackages() map[string]bool {
	const module = "github.com/hypnotox/agentic-workflows/internal/"
	owners := map[string]bool{}
	for _, name := range []string{
		"audit", "changelog", "checkop", "commitgateop", "contextop", "contextq",
		"currentstatecoord", "domainop", "effort", "effortop", "initop", "localdocop",
		"project", "publisher", "repositorycheck", "resident", "topicop", "upgrade", "worktree",
	} {
		owners[module+name] = true
	}
	return owners
}

// allowedOwnerBoundaryFunctions is the closed set of owner-package calls that
// compose dependencies, expose immutable operation state, or map a completed
// result for rendering. Every other function or method from an operation-owner
// package is treated as semantic even when no command route lists it.
func allowedOwnerBoundaryFunctions() map[string]bool {
	const module = "github.com/hypnotox/agentic-workflows/internal/"
	return map[string]bool{
		module + "currentstatecoord.Document":           true,
		module + "effort.MemoryDocument":                true,
		module + "project.NewLoader":                    true,
		module + "project.NewLoaderWithoutRepository":   true,
		module + "project.OpenForOperation":             true,
		module + "project.OutputState":                  true,
		module + "project.Root":                         true,
		module + "project.ValidateSchemaMinimumVersion": true,
		module + "publisher.BuildConfigReference":       true,
		module + "publisher.IsLocalDocTemplate":         true,
		module + "publisher.Mutation":                   true,
		module + "publisher.New":                        true,
		module + "publisher.NewFilesystemReader":        true,
		module + "resident.Document":                    true,
	}
}

func functionPackagePath(fn *types.Func) string {
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	return fn.Pkg().Path()
}

func qualifiedFunction(fn *types.Func) string {
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	return fn.Pkg().Path() + "." + fn.Name()
}

func expressionFunction(info *types.Info, expression ast.Expr) *types.Func {
	switch value := expression.(type) {
	case *ast.Ident:
		fn, _ := info.Uses[value].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		if selection := info.Selections[value]; selection != nil {
			fn, _ := selection.Obj().(*types.Func)
			return fn
		}
		fn, _ := info.Uses[value.Sel].(*types.Func)
		return fn
	default:
		return nil
	}
}
func directFunction(info *types.Info, call *ast.CallExpr) (*types.Func, bool) {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		fn, ok := info.Uses[fun].(*types.Func)
		return fn, ok
	case *ast.SelectorExpr:
		// A package qualifier or an already-held receiver is a direct call.
		// Calls through a constructor, type assertion, index, or selector chain
		// are deliberately not: those are alternate route representations.
		if _, ok := fun.X.(*ast.Ident); !ok {
			return nil, false
		}
		if selection := info.Selections[fun]; selection != nil {
			fn, ok := selection.Obj().(*types.Func)
			return fn, ok
		}
		fn, ok := info.Uses[fun.Sel].(*types.Func)
		return fn, ok
	}
	return nil, false
}
func sameOperations(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	a, b := append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\x00") == strings.Join(b, "\x00")
}
func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, "\""), true
}

func TestThinCompositionProofRejectsUnlistedOwnerOperation(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "list_add.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(source),
		`"github.com/hypnotox/agentic-workflows/internal/domainop"`,
		`"github.com/hypnotox/agentic-workflows/internal/domainop"
	"github.com/hypnotox/agentic-workflows/internal/repositorycheck"`, 1)
	mutated = strings.Replace(mutated,
		"\tdocument, err := project.BuildListDocument(state, cfg, kindFilter)",
		"\t_, _ = repositorycheck.Compose(repositorycheck.Inputs{})\n\tdocument, err := project.BuildListDocument(state, cfg, kindFilter)", 1)
	if mutated == string(source) {
		t.Fatal("production-route mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	calls, evasions := operationCalls(pkg, functionBody(t, pkg, "runList"))
	const hidden = "github.com/hypnotox/agentic-workflows/internal/repositorycheck.Compose"
	if !semanticOperationNames(append(calls, evasions...))[hidden] {
		t.Fatalf("unlisted owner operation escaped proof: calls = %v, evasions = %v", calls, evasions)
	}
}

func TestThinCompositionProofRejectsEvasions(t *testing.T) {
	mutatedCommands := append([]clispec.Command(nil), clispec.Commands...)
	mutatedCommands = append(mutatedCommands, clispec.Command{Name: "future-leaf"})
	if !executableCommands(mutatedCommands, nil)["future-leaf"] {
		t.Fatal("new clispec leaf escaped the executable census")
	}
	if _, admitted := commandRoutes()["future-leaf"]; admitted {
		t.Fatal("unregistered new clispec leaf unexpectedly has a route contract")
	}

	for _, source := range []string{
		`package p
func operation(){}
// operation()
func route(){}`,
		`package p; func operation(){}; func route(){}; func dead(){ operation() }`,
		`package p; func operation(){}; func route(){ alias := operation; alias() }`,
		`package p; func operation(){}; func route(){ func(){ operation() }() }`,
		`package p; type owner struct{}; func (owner) operation(){}; func route(){ call := owner{}.operation; call() }`,
		`package p; type owner struct{}; func (owner) operation(){}; func newOwner() owner{return owner{}}; func route(){ newOwner().operation() }`,
	} {
		if got := fixtureCalls(t, source); len(got) != 0 {
			t.Errorf("evasion produced operation evidence: %v", got)
		}
	}
	if got := fixtureCalls(t, `package p; func operation()(int,error){return 0,nil}; func route(){ value, _ := operation(); _=value }`); len(got) != 1 {
		t.Errorf("direct typed result call = %v, want one operation", got)
	}
}
func fixtureCalls(t *testing.T, source string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("fixture", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}
	pseudo := &packages.Package{Types: pkg, TypesInfo: info}
	var body *ast.BlockStmt
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "route" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("route absent")
	}
	calls, _ := operationCallsFor(pseudo, body, []string{"fixture.operation"})
	return calls
}
