package testsupport_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

func readProduction(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// invariant: rendering/project-output-plan:check-report-single-plan (TestPublishingConsumerPlanIdentity)
func TestPublishingConsumerPlanIdentity(t *testing.T) {
	root := testsupport.RepoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "cmd", "awf", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	initFiles, err := filepath.Glob(filepath.Join(root, "internal", "initop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, initFiles...)
	type calls struct {
		preparePublisher, operationPreparation, operationPlan int
		injectedPrepare, plan, residentMarker                 int
	}
	byFunction := map[string]calls{}
	publisherPrepareSites, publisherPlanSites := 0, 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			got := calls{}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch target := call.Fun.(type) {
				case *ast.Ident:
					switch target.Name {
					case "preparePublisher":
						got.preparePublisher++
					case "operationPreparation":
						got.operationPreparation++
					case "operationPlan":
						got.operationPlan++
					case "prepare":
						if fn.Name.Name == "initAdvisoryNotes" {
							got.injectedPrepare++
						}
					}
				case *ast.SelectorExpr:
					receiver, receiverIsIdent := target.X.(*ast.Ident)
					switch target.Sel.Name {
					case "Prepare":
						if receiverIsIdent && receiver.Name == "execution" {
							return true
						}
						publisherPrepareSites++
						allowed := receiverIsIdent && receiver.Name == "composed" && (fn.Name.Name == "preparePublisher" || fn.Name.Name == "Run")
						if !allowed {
							t.Errorf("unexpected Publisher preparation site %s in %s", target.Sel.Name, fn.Name.Name)
						}
					case "Plan":
						if receiverIsIdent && receiver.Name == "prepared" {
							got.plan++
							break
						}
						composed, ok := target.X.(*ast.CallExpr)
						constructor, constructorOK := composed.Fun.(*ast.Ident)
						allowed := ok && constructorOK && constructor.Name == "composePublisher" && fn.Name.Name == "probeCollisions"
						if !allowed {
							t.Errorf("%s calls Publisher Plan outside an owned plan-only seam", fn.Name.Name)
							break
						}
						publisherPlanSites++
					case "ResidentMarker":
						got.residentMarker++
						if !receiverIsIdent || receiver.Name != "prepared" {
							t.Errorf("%s selects a resident marker from another identity", fn.Name.Name)
						}
					}
				}
				return true
			})
			if got != (calls{}) {
				byFunction[fn.Name.Name] = got
			}
		}
	}
	if publisherPrepareSites != 2 || publisherPlanSites != 1 {
		t.Errorf("Publisher construction sites = %d Prepare and %d Plan, want command and init-operation semantic seams plus the init collision plan-only seam", publisherPrepareSites, publisherPlanSites)
	}
	expected := map[string]calls{
		"preparePublisher":                {},
		"operationPreparation":            {preparePublisher: 1},
		"workingContextState":             {preparePublisher: 1, plan: 1},
		"stagedDriftResult":               {preparePublisher: 1, plan: 1},
		"stagedContextState":              {preparePublisher: 1, plan: 1},
		"productionRepoCheckDependencies": {operationPreparation: 1, plan: 1},
		"Run":                             {plan: 1},
		"openEffortComposition":           {operationPreparation: 1, residentMarker: 1},
	}
	// preparePublisher's one direct .Prepare call is separately counted above;
	// its zero helper-call record is not retained in byFunction.
	delete(expected, "preparePublisher")
	if !reflect.DeepEqual(byFunction, expected) {
		t.Errorf("publishing consumer call graph changed:\n got %#v\nwant %#v", byFunction, expected)
	}

	// The check and advisory consumers must hand both projections the same local
	// preparation variable. The exact AST argument census makes replacing either
	// use with another preparation (or adding a second call) fail this proof.
	for _, rel := range []string{"cmd/awf/checkrepo.go", "internal/initop/init.go"} {
		source := readProduction(t, root, rel)
		if strings.Count(source, "prepared.Plan(), projectSemantics(prepared)") != 1 {
			t.Errorf("%s does not reuse one preparation for plan and semantics", rel)
		}
	}
}

// invariant: rendering/project-output-plan:check-report-single-plan (TestContextCompositionOwnershipRoutes)
// invariant: tooling/context-and-topic:context-query-boundary (TestContextCompositionOwnershipRoutes)
func TestContextCompositionOwnershipRoutes(t *testing.T) {
	root := testsupport.RepoRoot(t)
	contextqFiles := 0
	testsupport.WalkRepoSources(t, root, func(rel string, body []byte) {
		path, err := forbiddenContextQueryImport(rel, string(body))
		if err != nil {
			t.Errorf("parse %s imports: %v", rel, err)
			return
		}
		if !strings.HasPrefix(rel, "internal/contextq/") {
			return
		}
		contextqFiles++
		if path != "" {
			t.Errorf("%s imports forbidden package %s", rel, path)
		}
	})
	if contextqFiles == 0 {
		t.Fatal("contextq production-source scan matched no files")
	}
	assertNoImports(t, readProduction(t, root, "internal/contextinput/input.go"), "internal/currentstatecoord")

	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "cmd/awf/publishing.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]int{
		"workingContextState": {"PrepareWorkingContext": 1, "CompleteContext": 1, "preparePublisher": 1},
		"stagedContextState":  {"PrepareStagedContext": 1, "CompleteContext": 1, "preparePublisher": 1},
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || want[fn.Name.Name] == nil {
			continue
		}
		got := map[string]int{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "preparePublisher" {
				got[ident.Name]++
			}
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "currentstatecoord" {
					got[selector.Sel.Name]++
					if selector.Sel.Name == "CompleteContext" && !completeContextArgumentsValid(call) {
						t.Errorf("%s does not complete context from its selected universe and one Publisher preparation", fn.Name.Name)
					}
				}
			}
			return true
		})
		if !reflect.DeepEqual(got, want[fn.Name.Name]) {
			t.Errorf("%s context route = %#v, want %#v", fn.Name.Name, got, want[fn.Name.Name])
		}
	}
	for _, rel := range []string{"cmd/awf/context.go", "cmd/awf/publishing.go"} {
		source := readProduction(t, root, rel)
		for _, forbidden := range []string{"project.PrepareContextState", "project.CompleteContextState", "planContextFromTree", "currentstate.LoadFromTree"} {
			if strings.Contains(source, forbidden) {
				t.Errorf("command context path retains project same-tree parsing through %q in %s", forbidden, rel)
			}
		}
	}

	publisherFile, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal/publisher/inputs.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	parserCalls := map[string]map[string]int{
		"Prepare":                          {"deriveOperationStateWithPitfalls": 1, "derivePlans": 1},
		"deriveOperationStateWithPitfalls": {"LoadCorpusFromTree": 1, "LoadCorpusFromReader": 1},
		"derivePlans":                      {"ParseSources": 1},
	}
	for _, declaration := range publisherFile.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || parserCalls[fn.Name.Name] == nil {
			continue
		}
		got := map[string]int{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch target := call.Fun.(type) {
			case *ast.Ident:
				name = target.Name
			case *ast.SelectorExpr:
				name = target.Sel.Name
			}
			if _, tracked := parserCalls[fn.Name.Name][name]; tracked {
				got[name]++
			}
			return true
		})
		if !reflect.DeepEqual(got, parserCalls[fn.Name.Name]) {
			t.Errorf("Publisher %s semantic parser calls = %#v, want %#v", fn.Name.Name, got, parserCalls[fn.Name.Name])
		}
	}

	// Negative fixtures prove the complete-package import scan catches a future
	// contextq file while ignoring the same import outside that protected package.
	violating := `package contextq; import p "github.com/hypnotox/agentic-workflows/internal/project"; var _ = p.Version`
	if path, err := forbiddenContextQueryImport("internal/contextq/future.go", violating); err != nil || path == "" {
		t.Fatalf("future contextq reverse import = %q, %v; want a violation", path, err)
	}
	if path, err := forbiddenContextQueryImport("internal/project/future.go", violating); err != nil || path != "" {
		t.Fatalf("out-of-scope import = %q, %v; want ignored", path, err)
	}

	valid := `currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`
	if !completeContextExpressionValid(t, valid) {
		t.Fatal("valid context completion fixture failed")
	}
	for _, mutation := range []string{
		`currentstatecoord.CompleteContext(other, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`,
		`currentstatecoord.CompleteContext(prep, other.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`,
		`currentstatecoord.CompleteContext(prep, prepared.ADRs(), other.Topics(), prepared.Plans(), prepared.Plan().Declarations())`,
		`currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), other.Plans(), prepared.Plan().Declarations())`,
		`currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), other.Plan().Declarations())`,
	} {
		if completeContextExpressionValid(t, mutation) {
			t.Errorf("mutated context completion passed: %s", mutation)
		}
	}
}

func forbiddenContextQueryImport(rel, source string) (string, error) {
	if !strings.HasPrefix(rel, "internal/contextq/") {
		return "", nil
	}
	return forbiddenImport(source, "internal/project", "internal/currentstatecoord")
}

func completeContextExpressionValid(t *testing.T, expression string) bool {
	t.Helper()
	expr, err := parser.ParseExpr(expression)
	if err != nil {
		t.Fatal(err)
	}
	call, ok := expr.(*ast.CallExpr)
	return ok && completeContextArgumentsValid(call)
}

func completeContextArgumentsValid(call *ast.CallExpr) bool {
	if len(call.Args) != 5 || !isIdent(call.Args[0], "prep") {
		return false
	}
	if !isPreparedCall(call.Args[1], "ADRs") ||
		!isPreparedCall(call.Args[2], "Topics") ||
		!isPreparedCall(call.Args[3], "Plans") {
		return false
	}
	declarations, ok := call.Args[4].(*ast.CallExpr)
	if !ok || len(declarations.Args) != 0 {
		return false
	}
	selector, ok := declarations.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Declarations" {
		return false
	}
	plan, ok := selector.X.(*ast.CallExpr)
	return ok && len(plan.Args) == 0 && isPreparedCall(plan, "Plan")
}

func isPreparedCall(expr ast.Expr, method string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == method && isIdent(selector.X, "prepared")
}

func isIdent(expr ast.Expr, name string) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == name
}

func assertNoImports(t *testing.T, source string, denied ...string) {
	t.Helper()
	path, err := forbiddenImport(source, denied...)
	if err != nil {
		t.Fatal(err)
	}
	if path != "" {
		t.Fatalf("forbidden import %s", path)
	}
}

func forbiddenImport(source string, denied ...string) (string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, parser.ImportsOnly)
	if err != nil {
		return "", err
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", err
		}
		for _, suffix := range denied {
			if strings.HasSuffix(path, "/"+suffix) {
				return path, nil
			}
		}
	}
	return "", nil
}

func TestPublishingPlanningOwnership(t *testing.T) {
	root := testsupport.RepoRoot(t)
	for _, dir := range []string{"internal/outputplan", "internal/publisher"} {
		info, err := os.Stat(filepath.Join(root, dir))
		if err != nil || !info.IsDir() {
			t.Errorf("missing planning owner %s", dir)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "internal/project/output_plan.go")); !os.IsNotExist(err) {
		t.Error("obsolete internal/project/output_plan.go remains")
	}

	forbidden := map[string][]string{
		"internal/outputplan": {"internal/project", "internal/publisher", "cmd/awf", "internal/git", "internal/snapshot", "internal/contextq", "internal/filepublication"},
		"internal/publisher":  {"internal/project", "cmd/awf", "internal/git", "internal/snapshot", "internal/contextq", "internal/filepublication"},
	}
	for dir, denied := range forbidden {
		base := filepath.Join(root, dir)
		_ = filepath.WalkDir(base, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				t.Error(err)
				return nil
			}
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Error(err)
					continue
				}
				for _, suffix := range denied {
					if strings.HasSuffix(importPath, "/"+suffix) || strings.Contains(importPath, "/"+suffix+"/") {
						t.Errorf("%s imports forbidden package %s", filepath.ToSlash(path), importPath)
					}
				}
			}
			return nil
		})
	}

	publisherFiles, _ := filepath.Glob(filepath.Join(root, "internal/publisher", "*.go"))
	planConstructors := 0
	for _, path := range publisherFiles {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Plan" || fn.Recv == nil || len(fn.Recv.List) != 1 {
				continue
			}
			receiver := fn.Recv.List[0].Type
			if pointer, ok := receiver.(*ast.StarExpr); ok {
				receiver = pointer.X
			}
			if name, ok := receiver.(*ast.Ident); ok && name.Name == "Publisher" {
				planConstructors++
			}
		}
	}
	if planConstructors != 1 {
		t.Errorf("Publisher Plan constructors = %d, want exactly one", planConstructors)
	}

	// Exhaust the changed consumer classes. Outer composition owns Publisher;
	// lower check, advisory, staged, and context policy only consume neutral
	// plans or direct semantic values and cannot reconstruct planning policy.
	checkSource := readProduction(t, root, "internal/project/check.go")
	for _, forbidden := range []string{"adr.LoadCorpus(", "topic.LoadCorpus(", "plan.ParseDir(", "publisher."} {
		if strings.Contains(checkSource, forbidden) {
			t.Errorf("project check/advisory re-derives operation authority through %q", forbidden)
		}
	}
	if !strings.Contains(checkSource, "semantics OperationSemantics") {
		t.Error("project check no longer receives direct prepared semantics")
	}
	publisherSource := readProduction(t, root, "internal/publisher/inputs.go")
	for _, required := range []string{"adr.LoadCorpusFromTree(p.read", "topic.LoadCorpusFromReader(p.read", "outputPlanWithPitfalls(p.inputs", "func (p Preparation) ResidentMarker"} {
		if !strings.Contains(publisherSource, required) {
			t.Errorf("Publisher preparation lost selected-tree/single-plan clause %q", required)
		}
	}
	for _, forbidden := range []string{"RenderResidentMarker", "adr.LoadCorpus(decisionsDir", "topic.LoadCorpus(p.root"} {
		if strings.Contains(publisherSource, forbidden) {
			t.Errorf("Publisher retains parallel or working-tree planning path %q", forbidden)
		}
	}
	cmdSources := map[string]string{
		"check":                readProduction(t, root, "cmd/awf/checkrepo.go"),
		"initialization":       readProduction(t, root, "internal/initop/init.go"),
		"resident-marker":      readProduction(t, root, "cmd/awf/effort.go"),
		"outer/staged/context": readProduction(t, root, "cmd/awf/publishing.go"),
	}
	for class, source := range cmdSources {
		if class == "initialization" {
			if !strings.Contains(source, "composed.Prepare()") {
				t.Errorf("%s consumer does not visibly prepare through its focused operation", class)
			}
			continue
		}
		if !strings.Contains(source, "operationPreparation") && class != "outer/staged/context" {
			t.Errorf("%s consumer does not visibly prepare through outer composition", class)
		}
	}
	if !strings.Contains(cmdSources["resident-marker"], ".ResidentMarker(") || strings.Contains(cmdSources["resident-marker"], "RenderResidentMarker") {
		t.Error("resident marker is not selected from the outer prepared plan")
	}
	for _, rel := range []string{"internal/project/staged_drift.go", "internal/project/contextstate.go"} {
		source := readProduction(t, root, rel)
		if strings.Contains(source, "outputPlan(") || strings.Contains(source, "publisher.") {
			t.Errorf("%s reconstructs or imports Publisher planning", rel)
		}
	}

	// Test-only planning surfaces must not escape Publisher. Public methods with
	// real command consumers remain; declaration inventory and static inversion
	// helpers stay package-private.
	for _, forbidden := range []string{"func BuildOutputDeclarations(", "func PotentialVarConsumers("} {
		for _, path := range publisherFiles {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), forbidden) {
				t.Errorf("test-only Publisher API remains exported: %s", forbidden)
			}
		}
	}
}
