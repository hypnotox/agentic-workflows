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
// invariant: tooling/cli:cli-runner-instance-ownership (TestThinCommandCompositionCensus)
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
		got, evasions := operationCallsForRoute(pkg, body, route.path)
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

// invariant: tooling/cli:cli-runner-instance-ownership (TestRunnerCompositionHasNoMutableProcessSeams)
func TestRunnerCompositionHasNoMutableProcessSeams(t *testing.T) {
	if violations := runnerCompositionViolations(t, loadAWFCommandPackage(t)); len(violations) != 0 {
		t.Fatal(strings.Join(violations, "; "))
	}
}

func runnerCompositionViolations(t *testing.T, pkg *packages.Package) []string {
	t.Helper()
	var violations []string
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			general, ok := decl.(*ast.GenDecl)
			if !ok || general.Tok != token.VAR {
				continue
			}
			for _, spec := range general.Specs {
				value := spec.(*ast.ValueSpec)
				for _, name := range value.Names {
					object, _ := pkg.TypesInfo.Defs[name].(*types.Var)
					if object != nil && isCommandProcessSeamType(object.Type()) {
						violations = append(violations, "mutable package-global process seam "+name.Name)
					}
				}
			}
		}
	}
	freshRunnerReturn := false
	for _, statement := range functionBody(t, pkg, "run").List {
		returned, ok := statement.(*ast.ReturnStmt)
		if !ok || len(returned.Results) != 1 {
			continue
		}
		call, ok := returned.Results[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "run" {
			continue
		}
		constructor, ok := selector.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		name, named := constructor.Fun.(*ast.Ident)
		if named && name.Name == "newRunner" {
			freshRunnerReturn = true
		}
	}
	if !freshRunnerReturn {
		violations = append(violations, "production run does not return through its freshly constructed runner")
	}
	return violations
}

func isCommandProcessSeamType(value types.Type) bool {
	switch typed := value.Underlying().(type) {
	case *types.Signature:
		if typed.Params().Len() != 0 || typed.Results().Len() != 1 && typed.Results().Len() != 2 {
			return false
		}
		return typed.Results().At(0).Type().String() == "bool" || typed.Results().At(0).Type().String() == "string"
	case *types.Interface:
		return value.String() == "io.Reader"
	case *types.Map:
		named, ok := typed.Elem().(*types.Named)
		return typed.Key().String() == "string" && ok && named.Obj().Name() == "handler"
	default:
		return false
	}
}

func TestRunnerCompositionProofRejectsRenamedGlobalAndDiscardedRunner(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "main.go")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	t.Run("renamed global", func(t *testing.T) {
		mutated := strings.Replace(source, "const gitCommandTimeout = awfgit.CommandTimeout", "var ambientDirectory = os.Getwd\n\nconst gitCommandTimeout = awfgit.CommandTimeout", 1)
		pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
		if got := strings.Join(runnerCompositionViolations(t, pkg), "; "); !strings.Contains(got, "ambientDirectory") {
			t.Fatalf("renamed global escaped proof: %q", got)
		}
	})
	t.Run("discarded runner", func(t *testing.T) {
		old := "return newRunner(os.Getwd, os.Stdin, stdinIsInteractive(os.Stdin)).run(args, stdout, stderr)"
		new := "return runner{getwd: os.Getwd, stdin: os.Stdin, handlers: newHandlers(os.Stdin, stdinIsInteractive(os.Stdin))}.run(args, stdout, stderr)"
		mutated := strings.Replace(source, old, new, 1)
		pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
		if got := strings.Join(runnerCompositionViolations(t, pkg), "; "); !strings.Contains(got, "freshly constructed runner") {
			t.Fatalf("discarded fresh runner escaped proof: %q", got)
		}
	})
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
		{"init", "init", "runInitWithProjectLoader", []string{"github.com/hypnotox/agentic-workflows/internal/initspec.Describe", "github.com/hypnotox/agentic-workflows/internal/initop.Run"}, false},
		{"render", "render", "runSyncPrinting", []string{"github.com/hypnotox/agentic-workflows/internal/publisher.SyncLeased"}, false},
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
		{"read topic", "read", "runReadTopic", []string{"github.com/hypnotox/agentic-workflows/internal/topicop.Run"}, false},
		{"read adr", "read", "runReadADR", []string{"github.com/hypnotox/agentic-workflows/internal/currentstatecoord.ReadADR"}, false},
		{"resolve topic", "resolve", "runResolveTopic", []string{"github.com/hypnotox/agentic-workflows/internal/currentstatecoord.NormalizeAuthorityPath", "github.com/hypnotox/agentic-workflows/internal/currentstatecoord.ResolveTopics", "github.com/hypnotox/agentic-workflows/internal/currentstatecoord.UncoveredPaths"}, false},
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
		{"adr number", "adr", "runADR", []string{"github.com/hypnotox/agentic-workflows/internal/currentstatecoord.NumberPendingADRsLeased"}, false},
		{"list", "list", "runList", []string{"github.com/hypnotox/agentic-workflows/internal/project.BuildListDocument"}, false},
		{"config", "config", "runConfig", []string{"github.com/hypnotox/agentic-workflows/internal/configop.Run"}, false},
		{"topic", "topic", "runTopic", []string{"github.com/hypnotox/agentic-workflows/internal/topicop.Run"}, false},
		{"new adr", "new", "newADR", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewADRLeased"}, false},
		{"new plan", "new", "newPlan", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewPlanLeased"}, false},
		{"new topic", "new", "newTopic", []string{"github.com/hypnotox/agentic-workflows/internal/topicop.CreateLeased"}, false},
		{"new domain", "new", "runNewDomain", []string{"github.com/hypnotox/agentic-workflows/internal/domainop.AddLeased"}, false},
		{"new doc", "new", "newDoc", []string{"github.com/hypnotox/agentic-workflows/internal/localdocop.RunLeased", "github.com/hypnotox/agentic-workflows/internal/project.AcquireProjectLease"}, false},
		{"new pitfall", "new", "newPitfall", []string{"github.com/hypnotox/agentic-workflows/internal/project.NewPitfall"}, false},
		{"remove domain", "remove", "runRemoveDomain", []string{"github.com/hypnotox/agentic-workflows/internal/domainop.RemoveLeased"}, false},
		{"upgrade", "upgrade", "runUpgradeFlags", []string{"github.com/hypnotox/agentic-workflows/internal/upgrade.RecoverOperation", "github.com/hypnotox/agentic-workflows/internal/upgrade.Run"}, false},
		{"uninstall", "uninstall", "runUninstall", []string{"github.com/hypnotox/agentic-workflows/internal/resident.UninstallLeased"}, false},
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
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			function, ok := decl.(*ast.FuncDecl)
			if !ok || function.Name.Name != "newHandlers" {
				continue
			}
			for _, statement := range function.Body.List {
				returned, ok := statement.(*ast.ReturnStmt)
				if !ok || len(returned.Results) != 1 {
					continue
				}
				literal, ok := returned.Results[0].(*ast.CompositeLit)
				if !ok {
					t.Fatal("newHandlers does not return a composite literal")
				}
				out := map[string]*ast.FuncLit{}
				for _, elt := range literal.Elts {
					pair, ok := elt.(*ast.KeyValueExpr)
					key, keyOK := stringLiteral(pair.Key)
					fn, fnOK := pair.Value.(*ast.FuncLit)
					if !ok || !keyOK || !fnOK {
						t.Fatalf("unrecognized newHandlers entry")
					}
					out[key] = fn
				}
				return out
			}
		}
	}
	t.Fatal("instance-owned handlers composition not found")
	return nil
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
	// Numbering and upgrade supply Publisher synchronization as a concrete
	// callback dependency while their focused operation retains lifecycle and
	// recovery ownership. The callback cannot satisfy either route operation.
	if path != "adr number" && path != "upgrade" {
		return evasions
	}
	out := evasions[:0]
	for _, evasion := range evasions {
		if evasion != "github.com/hypnotox/agentic-workflows/internal/publisher.Sync" && evasion != "github.com/hypnotox/agentic-workflows/internal/publisher.SyncLeased" {
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
	describeBody, ordinaryBody := initVariantBodies(t, pkg)
	assertBranchOperations(t, pkg, "init describe", describeBody, []string{"github.com/hypnotox/agentic-workflows/internal/initspec.Describe"})
	assertBranchOperations(t, pkg, "init ordinary", ordinaryBody, []string{"github.com/hypnotox/agentic-workflows/internal/initop.Run"})

	worktree := routedBody(t, pkg, routes["effort worktree"])
	branch := firstIf(worktree.List)
	if branch == nil || branch.Else == nil {
		t.Fatal("effort worktree does not have add/remove branches")
	}
	assertBranchOperations(t, pkg, "effort worktree add", branch.Body, []string{"github.com/hypnotox/agentic-workflows/internal/effortop.AddWorktree"})
	assertBranchOperations(t, pkg, "effort worktree remove", statementBlock(branch.Else), []string{"github.com/hypnotox/agentic-workflows/internal/effortop.RemoveWorktree"})

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

func initVariantBodies(t *testing.T, pkg *packages.Package) (*ast.BlockStmt, *ast.BlockStmt) {
	t.Helper()
	declaration := functionDeclaration(t, pkg, "runInitWithProjectLoader")
	var describeObject types.Object
	for _, field := range declaration.Type.Params.List {
		for _, name := range field.Names {
			if name.Name == "describe" {
				describeObject = pkg.TypesInfo.Defs[name]
			}
		}
	}
	if describeObject == nil {
		t.Fatal("runInitWithProjectLoader describe parameter is absent")
	}

	branchIndex := -1
	var branch *ast.IfStmt
	for i, statement := range declaration.Body.List {
		candidate, ok := statement.(*ast.IfStmt)
		if !ok || typedBooleanIdent(pkg.TypesInfo, candidate.Cond) != describeObject {
			continue
		}
		if branch != nil {
			t.Fatal("runInitWithProjectLoader has multiple typed describe branches")
		}
		branchIndex, branch = i, candidate
	}
	if branch == nil {
		t.Fatal("init describe/ordinary branch is absent")
	}

	prefix := append([]ast.Stmt(nil), declaration.Body.List[:branchIndex]...)
	suffix := append([]ast.Stmt(nil), declaration.Body.List[branchIndex+1:]...)
	describe := append(append([]ast.Stmt(nil), prefix...), branch.Body.List...)
	if !blockTerminates(branch.Body) {
		describe = append(describe, suffix...)
	}
	ordinary := append([]ast.Stmt(nil), prefix...)
	if branch.Else != nil {
		ordinary = append(ordinary, statementBlock(branch.Else).List...)
	}
	if branch.Else == nil || !blockTerminates(statementBlock(branch.Else)) {
		ordinary = append(ordinary, suffix...)
	}
	return &ast.BlockStmt{List: describe}, &ast.BlockStmt{List: ordinary}
}

func functionDeclaration(t *testing.T, pkg *packages.Package, name string) *ast.FuncDecl {
	t.Helper()
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Name.Name == name {
				return function
			}
		}
	}
	t.Fatalf("function %s not found", name)
	return nil
}

func typedBooleanIdent(info *types.Info, expression ast.Expr) types.Object {
	for {
		parenthesized, ok := expression.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parenthesized.X
	}
	ident, ok := expression.(*ast.Ident)
	if !ok {
		return nil
	}
	return info.Uses[ident]
}

func blockTerminates(block *ast.BlockStmt) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}
	_, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	return ok
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
	if strings.HasPrefix(name, "upgrade ") {
		evasions = unexpectedRouteEvasions("upgrade", evasions)
	}
	if len(evasions) != 0 || !sameOperations(got, want) {
		t.Errorf("%s operations = %v, evasions = %v, want %v", name, got, evasions, want)
	}
}

// operationCalls returns every known semantic operation reached from body,
// retaining duplicates so a second invocation cannot hide behind set equality.
// Any operation selected as a value, through a constructor chain, or inside a
// closure is reported as an evasion instead of operation evidence.
func operationCalls(pkg *packages.Package, body *ast.BlockStmt) ([]string, []string) {
	return operationCallsForRoute(pkg, body, "")
}

func operationCallsForRoute(pkg *packages.Package, body *ast.BlockStmt, path string) ([]string, []string) {
	var names []string
	for _, route := range commandRoutes() {
		names = append(names, route.operations...)
	}
	// Publisher.BuildConfigReference is legitimate dependency composition for
	// other command routes, but config must reach that model policy only through
	// configop.Run. Route-local protection overrides the global boundary allowance.
	if path == "config" {
		names = append(names, "github.com/hypnotox/agentic-workflows/internal/publisher.BuildConfigReference")
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
		"audit", "changelog", "checkop", "commitgateop", "configop", "currentstatecoord",
		"domainop", "effort", "effortop", "initop", "localdocop",
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
		module + "domainop.Document":                    true,
		module + "localdocop.Document":                  true,
		module + "topicop.Document":                     true,
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
		module + "publisher.PartialMutation":            true,
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

func TestThinCompositionConfigRouteProofRejectsDuplicateOperation(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "config.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const call = `document, err := configop.Run(ctx, cwd, key, newProjectLoader, gate)`
	mutated := strings.Replace(string(source), call, `_, _ = configop.Run(ctx, cwd, key, newProjectLoader, gate)
	`+call, 1)
	if mutated == string(source) {
		t.Fatal("config operation mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	got, evasions := operationCallsForRoute(pkg, functionBody(t, pkg, "runConfig"), "config")
	want := []string{"github.com/hypnotox/agentic-workflows/internal/configop.Run"}
	if len(evasions) == 0 && sameOperations(got, want) {
		t.Fatalf("duplicate config operation escaped route proof: calls = %v, evasions = %v", got, evasions)
	}
}

func TestThinCompositionConfigRouteProofRejectsRestoredModelPolicy(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "config.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const call = `document, err := configop.Run(ctx, cwd, key, newProjectLoader, gate)`
	mutated := strings.Replace(string(source), call, `state, cfg, _, modelErr := openProjectOperation(ctx, cwd)
	if modelErr != nil {
		return modelErr
	}
	_, modelErr = composePublisher(state, cfg).BuildConfigReference()
	if modelErr != nil {
		return modelErr
	}
	`+call, 1)
	if mutated == string(source) {
		t.Fatal("config model-policy mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	body := functionBody(t, pkg, "runConfig")
	got, evasions := operationCallsForRoute(pkg, body, "config")
	const hidden = "github.com/hypnotox/agentic-workflows/internal/publisher.BuildConfigReference"
	if !semanticOperationNames(append(got, evasions...))[hidden] {
		t.Fatalf("restored cmd config model policy escaped route proof: calls = %v, evasions = %v", got, evasions)
	}
	genericCalls, genericEvasions := operationCalls(pkg, body)
	if semanticOperationNames(append(genericCalls, genericEvasions...))[hidden] {
		t.Fatalf("config-specific protection leaked into the global boundary allowance: calls = %v, evasions = %v", genericCalls, genericEvasions)
	}
}

func TestThinCompositionInitVariantProofAcceptsSharedPrecondition(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "init.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(source), "\tif describe {", "\tif len(sets) < 0 {\n\t\treturn nil\n\t}\n\tif describe {", 1)
	if mutated == string(source) {
		t.Fatal("init shared-precondition mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	describeBody, ordinaryBody := initVariantBodies(t, pkg)
	describeCalls, describeEvasions := operationCalls(pkg, describeBody)
	describeWant := []string{"github.com/hypnotox/agentic-workflows/internal/initspec.Describe"}
	ordinaryCalls, ordinaryEvasions := operationCalls(pkg, ordinaryBody)
	ordinaryWant := []string{"github.com/hypnotox/agentic-workflows/internal/initop.Run"}
	if len(describeEvasions) != 0 || !sameOperations(describeCalls, describeWant) || len(ordinaryEvasions) != 0 || !sameOperations(ordinaryCalls, ordinaryWant) {
		t.Fatalf("harmless shared precondition changed init classification: describe calls = %v, evasions = %v; ordinary calls = %v, evasions = %v", describeCalls, describeEvasions, ordinaryCalls, ordinaryEvasions)
	}
}

func TestThinCompositionInitVariantProofRejectsDuplicateDescribeOperation(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "init.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const call = `out, err := initspec.Describe(initspec.InitDescriptors(catalog.Standard.Vars))`
	mutated := strings.Replace(string(source), call, `_ , _ = initspec.Describe(initspec.InitDescriptors(catalog.Standard.Vars))
		`+call, 1)
	if mutated == string(source) {
		t.Fatal("init describe mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	describeBody, _ := initVariantBodies(t, pkg)
	got, evasions := operationCalls(pkg, describeBody)
	want := []string{"github.com/hypnotox/agentic-workflows/internal/initspec.Describe"}
	if len(evasions) == 0 && sameOperations(got, want) {
		t.Fatalf("duplicate describe operation escaped branch proof: calls = %v, evasions = %v", got, evasions)
	}
}

func TestThinCompositionInitVariantProofRejectsLocalDescriptorPolicy(t *testing.T) {
	root := testsupport.RepoRoot(t)
	path := filepath.Join(root, "cmd", "awf", "init.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withImport := strings.Replace(string(source), `"bytes"`, `"bytes"
	"encoding/json"`, 1)
	if withImport == string(source) {
		t.Fatal("init local descriptor-policy import mutation was not applied")
	}
	mutated := strings.Replace(withImport,
		`out, err := initspec.Describe(initspec.InitDescriptors(catalog.Standard.Vars))`,
		`out, err := json.Marshal(initspec.InitDescriptors(catalog.Standard.Vars))`, 1)
	if mutated == withImport {
		t.Fatal("init local descriptor-policy call mutation was not applied")
	}
	pkg := loadAWFCommandPackageWithOverlay(t, map[string][]byte{path: []byte(mutated)})
	describeBody, _ := initVariantBodies(t, pkg)
	got, evasions := operationCalls(pkg, describeBody)
	want := []string{"github.com/hypnotox/agentic-workflows/internal/initspec.Describe"}
	if len(evasions) == 0 && sameOperations(got, want) {
		t.Fatalf("cmd-local descriptor/JSON policy escaped describe proof: calls = %v, evasions = %v", got, evasions)
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
