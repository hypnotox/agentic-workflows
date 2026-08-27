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

func publisherImportName(file *ast.File) string {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || path != "github.com/hypnotox/agentic-workflows/internal/publisher" {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "publisher"
	}
	return ""
}

func publisherNewPrepareCall(construction *ast.CallExpr, publisherName string) bool {
	constructor, ok := construction.Fun.(*ast.SelectorExpr)
	if !ok || constructor.Sel.Name != "New" {
		return false
	}
	return isIdent(constructor.X, publisherName)
}

func completePublisherPreparationValid(t *testing.T, expression, publisherName string) bool {
	t.Helper()
	expr, err := parser.ParseExpr(expression)
	if err != nil {
		t.Fatal(err)
	}
	prepare, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := prepare.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Prepare" {
		return false
	}
	construction, ok := selector.X.(*ast.CallExpr)
	return ok && publisherNewPrepareCall(construction, publisherName)
}

// invariant: rendering/project-output-plan:check-report-single-plan (TestPublishingConsumerPlanIdentity)
func TestPublishingConsumerPlanIdentity(t *testing.T) {
	root := testsupport.RepoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "cmd", "awf", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	checkFiles, err := filepath.Glob(filepath.Join(root, "internal", "checkop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	initFiles, err := filepath.Glob(filepath.Join(root, "internal", "initop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	contextFiles, err := filepath.Glob(filepath.Join(root, "internal", "contextop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, checkFiles...)
	files = append(files, initFiles...)
	files = append(files, contextFiles...)
	type calls struct {
		preparePublisher, operationPreparation, operationPlan int
		injectedPrepare, plan, residentMarker                 int
	}
	byFunction := map[string]calls{}
	publisherPrepareSites, publisherNewPrepareSites, publisherPlanSites := 0, 0, 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		publisherName := publisherImportName(file)
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
						allowed := receiverIsIdent && receiver.Name == "composed" && (fn.Name.Name == "preparePublisher" || fn.Name.Name == "Run")
						if call, ok := target.X.(*ast.CallExpr); ok {
							constructor, constructorOK := call.Fun.(*ast.Ident)
							allowed = constructorOK && constructor.Name == "composePublisher" && fn.Name.Name == "probeCollisions"
							if constructor, ok := call.Fun.(*ast.SelectorExpr); ok && constructor.Sel.Name == "New" {
								isPublisherNew := publisherNewPrepareCall(call, publisherName)
								allowed = isPublisherNew && fn.Name.Name == "complete"
								if isPublisherNew {
									publisherNewPrepareSites++
								}
							}
						}
						if allowed {
							publisherPrepareSites++
						} else {
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
	if publisherPrepareSites != 5 || publisherPlanSites != 0 {
		t.Errorf("Publisher construction sites = %d Prepare and %d direct Plan, want check, context, command, final-init, and temporary collision-probe semantic seams with no separate Publisher plan route", publisherPrepareSites, publisherPlanSites)
	}
	if publisherNewPrepareSites != 1 {
		t.Errorf("package-qualified Publisher New(...).Prepare() sites = %d, want the one complete route", publisherNewPrepareSites)
	}
	if !completePublisherPreparationValid(t, "pub.New(state, config, reader, version).Prepare()", "pub") {
		t.Fatal("package-qualified Publisher New(...).Prepare() fixture failed")
	}
	if completePublisherPreparationValid(t, "other.New(state, config, reader, version).Prepare()", "pub") {
		t.Fatal("another package's New(...).Prepare() was accepted as Publisher preparation")
	}
	expected := map[string]calls{
		"preparePublisher":                {},
		"operationPreparation":            {preparePublisher: 1},
		"stagedDriftResult":               {preparePublisher: 1, plan: 1},
		"complete":                        {plan: 1},
		"productionRepoCheckDependencies": {operationPreparation: 1, plan: 1},
		"Run":                             {plan: 1},
		"probeCollisions":                 {plan: 1},
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
	for _, rel := range []string{"internal/checkop/checkrepo.go", "internal/initop/init.go"} {
		source := readProduction(t, root, rel)
		if strings.Count(source, "prepared.Plan(), projectSemantics(prepared)") != 1 {
			t.Errorf("%s does not reuse one preparation for plan and semantics", rel)
		}
	}
	initSource := readProduction(t, root, "internal/initop/init.go")
	for route, want := range map[string]int{
		"composed.Prepare()":        1,
		"prepared.InitCollisions()": 1,
		"prepared.Initialize(":      1,
		"prepared.Sync()":           1,
	} {
		if got := strings.Count(initSource, route); got != want {
			t.Errorf("initialization prepared-universe route %q count = %d, want %d", route, got, want)
		}
	}
	for _, forbidden := range []string{"composed.InitCollisions()", "composed.Initialize(", "composed.Sync()"} {
		if strings.Contains(initSource, forbidden) {
			t.Errorf("initialization reconstructs its prepared universe through %q", forbidden)
		}
	}

	publisherFile, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", "publisher", "sync.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	preparedRoutes := map[string]bool{"Sync": false, "Initialize": false, "InitCollisions": false}
	for _, declaration := range publisherFile.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || len(fn.Recv.List) != 1 || fn.Body == nil {
			continue
		}
		receiver := fn.Recv.List[0].Type
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = pointer.X
		}
		name, ok := receiver.(*ast.Ident)
		_, tracked := preparedRoutes[fn.Name.Name]
		if !ok || name.Name != "Preparation" || !tracked {
			continue
		}
		planUses, reconstructed := 0, 0
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if ident, ok := selector.X.(*ast.Ident); ok && ident.Name == "p" && selector.Sel.Name == "plan" {
				planUses++
			}
			if selector.Sel.Name == "Prepare" || selector.Sel.Name == "Plan" {
				reconstructed++
			}
			return true
		})
		if planUses != 1 || reconstructed != 0 {
			t.Errorf("Preparation.%s plan uses = %d and reconstruction calls = %d, want one bound p.plan use and no Prepare/Plan call", fn.Name.Name, planUses, reconstructed)
		}
		preparedRoutes[fn.Name.Name] = true
	}
	for route, found := range preparedRoutes {
		if !found {
			t.Errorf("Publisher prepared-universe route Preparation.%s not inspected", route)
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

	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal/contextop/context.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]int{
		"workingState":         {"PrepareFocusedWorkingContext": 1, "PrepareWorkingContext": 1},
		"workingCompleteState": {"PrepareWorkingContext": 1},
		"stagedState":          {"PrepareStagedContext": 1},
		"focused":              {"CompleteContext": 1, "PrepareContext": 1},
		"complete":             {"CompleteContext": 1, "Prepare": 1},
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
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				if receiver, ok := selector.X.(*ast.Ident); ok && receiver.Name == "currentstatecoord" {
					got[selector.Sel.Name]++
					if selector.Sel.Name == "CompleteContext" && !completeContextArgumentsValid(call, fn.Name.Name == "focused") {
						t.Errorf("%s does not complete context from its selected universe and one Publisher preparation", fn.Name.Name)
					}
				}
				if selector.Sel.Name == "Prepare" || selector.Sel.Name == "PrepareContext" {
					got[selector.Sel.Name]++
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
		"Prepare":                {"deriveOperationStateWithPitfalls": 1, "derivePlans": 1},
		"PrepareContext":         {"deriveContextSemantics": 1, "derivePlans": 1, "buildOutputDeclarations": 1},
		"deriveContextSemantics": {"LoadCorpusFromTree": 1, "LoadCorpusFromReader": 1},
		"derivePlans":            {"ParseSources": 1},
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

	completeValid := `currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`
	focusedValid := `currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Declarations())`
	if !completeContextExpressionValid(t, completeValid, false) || !completeContextExpressionValid(t, focusedValid, true) {
		t.Fatal("valid context completion fixture failed")
	}
	for _, mutation := range []struct {
		expression string
		focused    bool
	}{
		{`currentstatecoord.CompleteContext(other, prepared.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`, false},
		{`currentstatecoord.CompleteContext(prep, other.ADRs(), prepared.Topics(), prepared.Plans(), prepared.Plan().Declarations())`, false},
		{`currentstatecoord.CompleteContext(prep, prepared.ADRs(), other.Topics(), prepared.Plans(), prepared.Plan().Declarations())`, false},
		{`currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), other.Plans(), prepared.Plan().Declarations())`, false},
		{`currentstatecoord.CompleteContext(prep, prepared.ADRs(), prepared.Topics(), prepared.Plans(), other.Plan().Declarations())`, false},
		{focusedValid, false},
		{completeValid, true},
	} {
		if completeContextExpressionValid(t, mutation.expression, mutation.focused) {
			t.Errorf("mutated context completion passed: %s", mutation.expression)
		}
	}
}

func forbiddenContextQueryImport(rel, source string) (string, error) {
	if !strings.HasPrefix(rel, "internal/contextq/") {
		return "", nil
	}
	return forbiddenImport(source, "internal/project", "internal/currentstatecoord")
}

func completeContextExpressionValid(t *testing.T, expression string, focused bool) bool {
	t.Helper()
	expr, err := parser.ParseExpr(expression)
	if err != nil {
		t.Fatal(err)
	}
	call, ok := expr.(*ast.CallExpr)
	return ok && completeContextArgumentsValid(call, focused)
}

func completeContextArgumentsValid(call *ast.CallExpr, focused bool) bool {
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
	if focused {
		return isIdent(selector.X, "prepared")
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
		"check":                readProduction(t, root, "internal/checkop/checkrepo.go"),
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
