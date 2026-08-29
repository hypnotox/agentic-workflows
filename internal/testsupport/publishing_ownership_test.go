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
	checkFiles, err := filepath.Glob(filepath.Join(root, "internal", "checkop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	initFiles, err := filepath.Glob(filepath.Join(root, "internal", "initop", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, checkFiles...)
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
						allowed := receiverIsIdent && receiver.Name == "composed" && (fn.Name.Name == "preparePublisher" || fn.Name.Name == "runWithDependencies")
						if call, ok := target.X.(*ast.CallExpr); ok {
							constructor, constructorOK := call.Fun.(*ast.Ident)
							allowed = constructorOK && constructor.Name == "composePublisher" && fn.Name.Name == "probeCollisions"
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
	if publisherPrepareSites != 4 || publisherPlanSites != 0 {
		t.Errorf("Publisher construction sites = %d Prepare and %d direct Plan, want check, command, final-init, and temporary collision-probe semantic seams with no separate Publisher plan route", publisherPrepareSites, publisherPlanSites)
	}
	expected := map[string]calls{
		"preparePublisher":                {},
		"operationPreparation":            {preparePublisher: 1},
		"stagedDriftResult":               {preparePublisher: 1, plan: 1},
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
		"composed.Prepare()":         1,
		"prepared.InitCollisions()":  1,
		"composed.InitializeLeased(": 1,
		"composed.SyncLeased(":       1,
	} {
		if got := strings.Count(initSource, route); got != want {
			t.Errorf("initialization prepared-universe route %q count = %d, want %d", route, got, want)
		}
	}
	for _, forbidden := range []string{"composed.InitCollisions()", "prepared.Initialize(", "prepared.SyncLeased(context.Background(), nil)", "composed.Initialize(", "composed.SyncLeased(context.Background(), nil)"} {
		if strings.Contains(initSource, forbidden) {
			t.Errorf("initialization reconstructs its prepared universe through %q", forbidden)
		}
	}

	publisherFile, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", "publisher", "sync.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	preparedRoutes := map[string]bool{"InitCollisions": false}
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
			t.Errorf("Publisher prepared read-model route Preparation.%s not inspected", route)
		}
	}
	publisherSource := readProduction(t, root, "internal/publisher/sync.go")
	for _, forbidden := range []string{"func (p Preparation) Sync", "func (p Preparation) Initialize"} {
		if strings.Contains(publisherSource, forbidden) {
			t.Errorf("stale Preparation mutator remains: %s", forbidden)
		}
	}
}

// invariant: rendering/project-output-plan:check-report-single-plan (TestStagedOutputPreparationOwnershipRoutes)
func TestStagedOutputPreparationOwnershipRoutes(t *testing.T) {
	root := testsupport.RepoRoot(t)
	coordinator := readProduction(t, root, "internal/currentstatecoord/outputstate.go")
	if strings.Contains(coordinator, "internal/publisher") {
		t.Fatal("staged universe selection imports Publisher policy")
	}
	for clause, want := range map[string]int{
		"awfgit.OpenContaining(root)":        1,
		"indexTree(root, repo, ctx)":         1,
		"optionalLockFromTree(tree)":         1,
		"configFromTree(root, tree, lock)":   1,
		"Reader: snapshotReader{tree: tree}": 1,
	} {
		if got := strings.Count(coordinator, clause); got != want {
			t.Errorf("staged output coordinator clause %q count = %d, want %d", clause, got, want)
		}
	}

	projectRoute := readProduction(t, root, "internal/project/outputstate.go")
	if got := strings.Count(projectRoute, "return currentstatecoord.PrepareStagedOutput(ctx, root)"); got != 1 {
		t.Fatalf("project staged-output adapter routes = %d, want one", got)
	}
	if strings.Contains(projectRoute, "internal/publisher") {
		t.Fatal("project staged-output adapter imports Publisher policy")
	}

	consumer := readProduction(t, root, "internal/checkop/publishing.go")
	for clause, want := range map[string]int{
		"project.PrepareStagedOutputState(ctx, root)":                          1,
		"publisher.New(prep.State, prep.Config, prep.Reader, project.Version)": 1,
		"project.CheckStagedDriftResult(prep, prepared.Plan())":                1,
	} {
		if got := strings.Count(consumer, clause); got != want {
			t.Errorf("staged drift composition clause %q count = %d, want %d", clause, got, want)
		}
	}
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
		"internal/outputplan": {"internal/project", "internal/publisher", "cmd/awf", "internal/git", "internal/snapshot", "internal/filepublication"},
		"internal/publisher":  {"internal/project", "cmd/awf", "internal/git", "internal/snapshot", "internal/filepublication"},
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
	// lower check, advisory, and staged policy only consume neutral
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
		"check":           readProduction(t, root, "internal/checkop/checkrepo.go"),
		"initialization":  readProduction(t, root, "internal/initop/init.go"),
		"resident-marker": readProduction(t, root, "cmd/awf/effort.go"),
		"outer-command":   readProduction(t, root, "cmd/awf/publishing.go"),
	}
	for class, source := range cmdSources {
		if class == "initialization" {
			if !strings.Contains(source, "composed.Prepare()") {
				t.Errorf("%s consumer does not visibly prepare through its focused operation", class)
			}
			continue
		}
		if !strings.Contains(source, "operationPreparation") && class != "outer-command" {
			t.Errorf("%s consumer does not visibly prepare through outer composition", class)
		}
	}
	if !strings.Contains(cmdSources["resident-marker"], ".ResidentMarker(") || strings.Contains(cmdSources["resident-marker"], "RenderResidentMarker") {
		t.Error("resident marker is not selected from the outer prepared plan")
	}
	for _, rel := range []string{"internal/project/staged_drift.go"} {
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
