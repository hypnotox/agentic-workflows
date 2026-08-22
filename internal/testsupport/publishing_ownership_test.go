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
	type calls struct {
		preparePublisher, operationPreparation, operationPlan int
		plan, residentMarker                                  int
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
					}
				case *ast.SelectorExpr:
					receiver, receiverIsIdent := target.X.(*ast.Ident)
					switch target.Sel.Name {
					case "Prepare":
						if receiverIsIdent && receiver.Name == "execution" {
							return true
						}
						publisherPrepareSites++
						if fn.Name.Name != "preparePublisher" || !receiverIsIdent || receiver.Name != "composed" {
							t.Errorf("unexpected Publisher preparation site %s in %s", target.Sel.Name, fn.Name.Name)
						}
					case "Plan":
						if receiverIsIdent && receiver.Name == "prepared" {
							got.plan++
							break
						}
						composed, ok := target.X.(*ast.CallExpr)
						constructor, constructorOK := composed.Fun.(*ast.Ident)
						if !ok || !constructorOK || constructor.Name != "composePublisher" || fn.Name.Name != "operationPlan" {
							t.Errorf("%s calls Publisher Plan outside the single plan-only seam", fn.Name.Name)
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
	if publisherPrepareSites != 1 || publisherPlanSites != 1 {
		t.Errorf("Publisher construction sites = %d Prepare and %d Plan, want one semantic and one plan-only outer seam", publisherPrepareSites, publisherPlanSites)
	}
	expected := map[string]calls{
		"preparePublisher":                {},
		"operationPreparation":            {preparePublisher: 1},
		"workingContextState":             {preparePublisher: 1, plan: 1},
		"stagedDrift":                     {preparePublisher: 1, plan: 1},
		"stagedContextState":              {preparePublisher: 1, plan: 1},
		"productionRepoCheckDependencies": {operationPreparation: 1, plan: 1},
		"initAdvisoryNotes":               {plan: 1},
		"probeCollisions":                 {operationPlan: 1},
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
	for _, rel := range []string{"cmd/awf/checkrepo.go", "cmd/awf/init.go"} {
		source := readProduction(t, root, rel)
		if strings.Count(source, "prepared.Plan(), projectSemantics(prepared)") != 1 {
			t.Errorf("%s does not reuse one preparation for plan and semantics", rel)
		}
		if rel == "cmd/awf/init.go" && strings.Count(source, "initAdvisoryNotes(state, cfg, operationPreparation)") != 1 {
			t.Errorf("%s does not pass the one Publisher preparation seam to init advisories", rel)
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
		"initialization":       readProduction(t, root, "cmd/awf/init.go"),
		"resident-marker":      readProduction(t, root, "cmd/awf/effort.go"),
		"outer/staged/context": readProduction(t, root, "cmd/awf/publishing.go"),
	}
	for class, source := range cmdSources {
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
