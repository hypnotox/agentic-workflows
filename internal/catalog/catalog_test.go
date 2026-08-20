package catalog

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/templates"
	"gopkg.in/yaml.v3"
)

// invariant: rendering/catalog-and-targets:catalog-go-single-source (TestCatalogIsCompileTimeSingleSource)
func TestCatalogIsCompileTimeSingleSource(t *testing.T) {
	if _, err := fs.Stat(templates.FS, "catalog.yaml"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("catalog.yaml must not be embedded; got stat err = %v", err)
	}
	if len(Standard.Skills) == 0 || len(Standard.Agents) == 0 || len(Standard.Docs) == 0 ||
		len(SingletonKinds()) == 0 || len(Standard.Vars) == 0 || len(Standard.DomainDoc.Sections) == 0 {
		t.Fatalf("catalog.Standard is not populated across all kinds")
	}
}

func TestNewViewRejectsNilCatalog(t *testing.T) {
	defer func() {
		if got := recover(); got != "catalog view: missing catalog" {
			t.Fatalf("panic = %v", got)
		}
	}()
	NewView(nil)
}

// TestCompleteViewPreservesStandard verifies the preparatory view selects every
// complete-catalog entry and preserves the ordered descriptor population.
func TestCompleteViewPreservesStandard(t *testing.T) {
	view := CompleteView()
	if view.Catalog() == Standard {
		t.Fatal("complete view retained a mutable alias to Standard")
	}
	if !reflect.DeepEqual(view.Catalog(), Standard) {
		t.Fatal("complete view does not preserve the complete Standard catalog")
	}
}

func TestViewOwnsDeepCatalogSnapshot(t *testing.T) {
	injected := cloneCatalog(Standard)
	injectedSkill := injected.Skills["tdd"]
	injectedSkill.Data["strings"] = []string{"original"}
	injectedSkill.Data["numbers"] = []int{1}
	injectedSkill.Data["labels"] = map[string]string{"value": "original"}
	injectedSkill.Data["records"] = []map[string]any{{"value": "original"}}
	injectedSkill.Data["array"] = [1]string{"original"}
	injectedSkill.Data["direct-nil"] = nil
	injectedSkill.Data["nil-list"] = []any{nil}
	var nilMap map[string]string
	injectedSkill.Data["nil-map"] = nilMap
	var nilSlice []int
	injectedSkill.Data["nil-slice"] = nilSlice
	pointed := []string{"original"}
	injectedSkill.Data["pointer"] = &pointed
	var nilPointer *[]string
	injectedSkill.Data["nil-pointer"] = nilPointer
	injected.Skills["tdd"] = injectedSkill
	view := NewView(injected)

	injectedSkill.Sections[0] = "changed input"
	injectedSkill.Data["strings"].([]string)[0] = "changed input"
	injectedSkill.Data["numbers"].([]int)[0] = 2
	injectedSkill.Data["labels"].(map[string]string)["value"] = "changed input"
	injectedSkill.Data["records"].([]map[string]any)[0]["value"] = "changed input"
	pointed[0] = "changed input"
	injected.Skills["tdd"] = injectedSkill

	got := view.Catalog().Skills["tdd"]
	gotNilMap, mapOK := got.Data["nil-map"].(map[string]string)
	gotNilSlice, sliceOK := got.Data["nil-slice"].([]int)
	if got.Sections[0] == "changed input" || got.Data["strings"].([]string)[0] != "original" ||
		got.Data["numbers"].([]int)[0] != 1 || got.Data["labels"].(map[string]string)["value"] != "original" ||
		got.Data["records"].([]map[string]any)[0]["value"] != "original" || got.Data["array"].([1]string)[0] != "original" ||
		got.Data["direct-nil"] != nil || got.Data["nil-list"].([]any)[0] != nil || !mapOK || gotNilMap != nil ||
		!sliceOK || gotNilSlice != nil || (*got.Data["pointer"].(*[]string))[0] != "original" || got.Data["nil-pointer"] != (*[]string)(nil) {
		t.Fatalf("view changed through injected reference alias: %#v", got)
	}

	standardSection := Standard.Skills["tdd"].Sections[0]
	_, standardHadProbe := Standard.Skills["tdd"].Data["view-probe"]
	complete := CompleteView().Catalog()
	completeSkill := complete.Skills["tdd"]
	completeSkill.Sections[0] = "changed view"
	completeSkill.Data["view-probe"] = "changed view"
	complete.Skills["tdd"] = completeSkill
	if Standard.Skills["tdd"].Sections[0] != standardSection {
		t.Fatal("Standard sections changed through complete view alias")
	}
	_, standardHasProbe := Standard.Skills["tdd"].Data["view-probe"]
	if standardHasProbe != standardHadProbe {
		t.Fatal("Standard data changed through complete view alias")
	}

	returned := view.Catalog()
	returnedSkill := returned.Skills["tdd"]
	returnedSkill.Sections[0] = "changed returned snapshot"
	returned.Skills["tdd"] = returnedSkill
	if view.Catalog().Skills["tdd"].Sections[0] == "changed returned snapshot" {
		t.Fatal("View changed through a returned catalog snapshot")
	}
}

// TestProjectProductionCatalogBypassesRejected keeps the complete view as the
// sole project catalog authority. Only exact composition functions may acquire
// it; every other production consumer must read its Project-owned view.
func TestProjectProductionCatalogBypassesRejected(t *testing.T) {
	root := testsupport.RepoRoot(t)
	projectDir := filepath.Join(root, "internal", "project")
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(projectDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		bypasses, err := projectCatalogBypasses(entry.Name(), body)
		if err != nil {
			t.Fatal(err)
		}
		for _, bypass := range bypasses {
			t.Errorf("project production bypass %s", bypass)
		}
	}

	for _, selector := range []string{"Standard", "CompleteView", "NewView", "SingletonKinds"} {
		source := "package project\nimport \"github.com/hypnotox/agentic-workflows/internal/catalog\"\nfunc forbidden() { _ = catalog." + selector
		if selector != "Standard" {
			source += "()"
		}
		source += " }\n"
		got, err := projectCatalogBypasses("project.go", []byte(source))
		if err != nil || len(got) != 1 || !strings.HasSuffix(got[0], ":"+selector) {
			t.Errorf("bypass probe %s = %v, %v", selector, got, err)
		}
	}

	allowedSource := `package project
import "github.com/hypnotox/agentic-workflows/internal/catalog"
func Open() { _ = catalog.CompleteView() }
func forbidden() { _ = catalog.CompleteView() }
`
	got, err := projectCatalogBypasses("project.go", []byte(allowedSource))
	if err != nil || len(got) != 1 || got[0] != "project.go:forbidden:CompleteView" {
		t.Errorf("function-scoped bypass probe = %v, %v", got, err)
	}
	aliasSource := `package project
import cat "github.com/hypnotox/agentic-workflows/internal/catalog"
func forbidden() { _ = cat.Standard }
`
	got, err = projectCatalogBypasses("project.go", []byte(aliasSource))
	if err != nil || len(got) != 1 || got[0] != "project.go:forbidden:Standard" {
		t.Errorf("aliased bypass probe = %v, %v", got, err)
	}
	dotSource := `package project
import . "github.com/hypnotox/agentic-workflows/internal/catalog"
func forbidden() { _ = CompleteView() }
`
	got, err = projectCatalogBypasses("project.go", []byte(dotSource))
	if err != nil || len(got) != 1 || got[0] != "project.go:<import>:dot" {
		t.Errorf("dot-import bypass probe = %v, %v", got, err)
	}
	methodSource := `package project
import "github.com/hypnotox/agentic-workflows/internal/catalog"
type Project struct{}
func (Project) Open() { _ = catalog.CompleteView() }
`
	got, err = projectCatalogBypasses("project.go", []byte(methodSource))
	if err != nil || len(got) != 1 || got[0] != "project.go:(Project).Open:CompleteView" {
		t.Errorf("method-identity bypass probe = %v, %v", got, err)
	}
}

func projectCatalogBypasses(filename string, body []byte) ([]string, error) {
	allowed := map[string]map[string]map[string]bool{
		"configreference.go": {"PotentialVarConsumers": {"CompleteView": true}},
		"project.go": {
			"newLoader":       {"NewView": true},
			"Open":            {"CompleteView": true},
			"openRootProject": {"CompleteView": true},
			"stagedProject":   {"CompleteView": true},
		},
		"scaffold.go": {
			"ScaffoldConfig":   {"CompleteView": true},
			"neededVarsFromFS": {"CompleteView": true},
		},
	}
	watched := map[string]bool{"Standard": true, "CompleteView": true, "NewView": true, "SingletonKinds": true}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, body, 0)
	if err != nil {
		return nil, err
	}
	catalogName := ""
	var out []string
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return nil, err
		}
		if path != "github.com/hypnotox/agentic-workflows/internal/catalog" {
			continue
		}
		catalogName = "catalog"
		if imported.Name != nil {
			catalogName = imported.Name.Name
			if catalogName == "." {
				out = append(out, filename+":<import>:dot")
				return out, nil
			}
		}
	}
	inspect := func(function string, node ast.Node) {
		ast.Inspect(node, func(node ast.Node) bool {
			sel, ok := node.(*ast.SelectorExpr)
			if !ok || !watched[sel.Sel.Name] {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != catalogName || allowed[filename][function][sel.Sel.Name] {
				return true
			}
			out = append(out, filename+":"+function+":"+sel.Sel.Name)
			return true
		})
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			inspect(functionIdentity(fn), fn.Body)
			continue
		}
		inspect("<package>", decl)
	}
	return out, nil
}

func functionIdentity(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + receiverIdentity(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func receiverIdentity(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return "*" + receiverIdentity(expr.X)
	case *ast.IndexExpr:
		return receiverIdentity(expr.X)
	case *ast.IndexListExpr:
		return receiverIdentity(expr.X)
	case *ast.SelectorExpr:
		return receiverIdentity(expr.X) + "." + expr.Sel.Name
	default:
		return "<receiver>"
	}
}

// Catalog default data must be generic: no default names an awf-repo path or
// command (ADR-0045). Walks every spec's Data recursively down to the strings.
// invariant: rendering/catalog-and-targets:catalog-defaults-generic-denylist (TestCatalogDefaultDataIsGeneric)
func TestCatalogDefaultDataIsGeneric(t *testing.T) {
	cat := Standard
	states, ok := cat.Skills["adr-lifecycle"].Data["adrStates"].([]any)
	if !ok || len(states) != 5 {
		t.Fatalf("representative V2 adrStates = %#v", cat.Skills["adr-lifecycle"].Data["adrStates"])
	}
	wantStates := []string{"Proposed", "Accepted", "Implementing", "Implemented", "Abandoned"}
	for i, state := range states {
		fields, ok := state.(map[string]any)
		if !ok || fields["name"] != wantStates[i] || fields["meaning"] == "" || fields["mutability"] == "" {
			t.Fatalf("V2 adrStates[%d] = %#v", i, state)
		}
	}
	implementing := states[2].(map[string]any)
	if !strings.Contains(implementing["meaning"].(string), "Remaining may be empty") || !strings.Contains(implementing["mutability"].(string), "every explicit Applied batch belongs to implementation") {
		t.Fatalf("Implementing lifecycle guidance = %#v", implementing)
	}
	if empty := (map[string]any{})["adrStates"]; empty != nil {
		t.Fatalf("empty catalog override unexpectedly supplies V2 data: %#v", empty)
	}
	denylist := []string{"./x", "hypnotox/agentic-workflows"}
	var walk func(t *testing.T, path string, v any)
	walk = func(t *testing.T, path string, v any) {
		switch val := v.(type) {
		case string:
			for _, banned := range denylist {
				if strings.Contains(val, banned) {
					t.Errorf("%s: default data contains %q: %q", path, banned, val)
				}
			}
		case []any:
			for i, item := range val {
				walk(t, fmt.Sprintf("%s[%d]", path, i), item)
			}
		case map[string]any:
			for k, item := range val {
				walk(t, path+"."+k, item)
			}
		}
	}
	for name, spec := range cat.Skills {
		walk(t, "skills."+name, spec.Data)
	}
	for name, spec := range cat.Agents {
		walk(t, "agents."+name, spec.Data)
	}
	for name, e := range cat.Docs {
		walk(t, "docs."+name, e.Data)
	}
}

func TestAgentsDocSectionsNonEmpty(t *testing.T) {
	cat := Standard
	sections := cat.Docs["agents-doc"].Sections
	if len(sections) == 0 {
		t.Error("expected agents-doc Sections to be non-empty")
	}
	expected := []string{"you-and-this-project", "identity", "invariants", "workflow", "commands", "document-map"}
	for _, s := range expected {
		found := false
		for _, sec := range sections {
			if sec == s {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected section %q in agents-doc Sections, got: %v", s, sections)
		}
	}
}

// Every reviewing skill is a thin dispatcher around one reviewer agent; the
// catalog must pair them so the ADR-0050 validation can enforce it - the
// prefix anchor keeps a future reviewing skill from reopening the blind spot.
// requiresAgent is no longer reviewer-exclusive: the two plan-execution skills
// dispatch the implementer (ADR-0177), so they are named here explicitly rather
// than admitted by a blanket exemption, which would let any future skill gain an
// unpaired agent reference silently.
var nonReviewingDispatchers = map[string]string{
	"grounding":                   "grounding-checker",
	"executing-plans":             "implementer",
	"exploring":                   "explorer",
	"subagent-driven-development": "implementer",
}

// invariant: rendering/catalog-and-targets:reviewing-skill-specs-paired (TestReviewingSkillSpecsArePaired)
func TestReviewingSkillSpecsArePaired(t *testing.T) {
	cat := Standard
	for name, spec := range cat.Skills {
		if !strings.HasPrefix(name, "reviewing-") {
			if want, ok := nonReviewingDispatchers[name]; ok {
				if spec.RequiresAgent != want {
					t.Errorf("dispatching skill %q: requiresAgent = %q, want %q", name, spec.RequiresAgent, want)
				}
				continue
			}
			if spec.RequiresAgent != "" {
				t.Errorf("skill %q: requiresAgent %q on a skill that dispatches no agent (ADR-0050 scopes the field to dispatchers)", name, spec.RequiresAgent)
			}
			continue
		}
		if spec.RequiresAgent == "" {
			t.Errorf("reviewing skill %q carries no requiresAgent", name)
			continue
		}
		if _, ok := cat.Agents[spec.RequiresAgent]; !ok {
			t.Errorf("skill %q requires agent %q, which is not in the catalog agents map", name, spec.RequiresAgent)
		}
	}
}

// TestRequiresSkillsDeclarationsValid rejects a RequiresSkills entry naming a
// non-catalog skill or the artifact itself, and any RequiresSkills on the
// domain-doc spec - today the only TargetSpec use outside the agents map; the
// field is meaningless there and a silent no-op would invite drift (ADR-0080
// Decision 1). The self-naming rejection is an exactness corollary of Decision
// 7: a self-entry could never fail as stale (the frontmatter name always marks
// self as found), so it is refused upfront.
func TestRequiresSkillsDeclarationsValid(t *testing.T) {
	cat := Standard
	for name, spec := range cat.Skills {
		for _, r := range spec.RequiresSkills {
			if _, ok := cat.Skills[r]; !ok {
				t.Errorf("skill %q: requiresSkills entry %q is not a catalog skill", name, r)
			}
			if r == name {
				t.Errorf("skill %q: requiresSkills must not name itself", name)
			}
		}
	}
	for name, spec := range cat.Agents {
		for _, r := range spec.RequiresSkills {
			if _, ok := cat.Skills[r]; !ok {
				t.Errorf("agent %q: requiresSkills entry %q is not a catalog skill", name, r)
			}
		}
	}
	if len(cat.DomainDoc.RequiresSkills) != 0 {
		t.Error("domainDoc: requiresSkills is only valid on skills and agents (ADR-0080 Decision 1)")
	}
}

// invariant: rendering/catalog-and-targets:requires-skills-exact (TestStandardSkillRequirementsAreEmpty)
func TestStandardSkillRequirementsAreEmpty(t *testing.T) {
	for name, spec := range Standard.Skills {
		if len(spec.RequiresSkills) != 0 {
			t.Errorf("standard skill %q has structural workflow requirements %v", name, spec.RequiresSkills)
		}
	}
	for name, spec := range Standard.Agents {
		if len(spec.RequiresSkills) != 0 {
			t.Errorf("standard agent %q has structural workflow requirements %v", name, spec.RequiresSkills)
		}
	}
}

// invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor (TestNoSingleMarkerInitDescriptor)
//
// The catalog exposes no single marker/globs var descriptor; qualified markers
// reach config only through currentState.sources.
func TestNoSingleMarkerInitDescriptor(t *testing.T) {
	for _, d := range Standard.Vars {
		if d.Key == "invariantsMarker" || d.Key == "invariantsGlobs" {
			t.Errorf("catalog still declares removed descriptor key %q", d.Key)
		}
		if d.Target == "invariants-marker" || d.Target == "invariants-globs" {
			t.Errorf("catalog still declares removed descriptor target %q", d.Target)
		}
	}

	var live struct {
		CurrentState struct {
			Sources []struct {
				Globs  []string `yaml:"globs"`
				Marker string   `yaml:"marker"`
			} `yaml:"sources"`
		} `yaml:"currentState"`
	}
	configPath := filepath.Join(testsupport.RepoRoot(t), ".awf", "config.yaml")
	body, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(body, &live); err != nil {
		t.Fatal(err)
	}
	const testPath = "internal/catalog/catalog_test.go"
	const qualified = "invariant: rendering/catalog-and-targets:no-single-marker-init-descriptor"
	for _, source := range live.CurrentState.Sources {
		for _, glob := range source.Globs {
			if pathglob.Match(glob, testPath) && source.Marker+" "+qualified == "// "+qualified {
				return
			}
		}
	}
	t.Fatalf("currentState.sources has no configuration route from %s to qualified marker %q", testPath, "// "+qualified)
}

func TestProfileViewRejectsInvalidProfileAndProjectsCore(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("NewProfileView accepted invalid profile")
		}
	}()
	_ = NewProfileView(Standard, Profile("other"))
}

func TestProfileViewCoreOmitsFullOnlyEntries(t *testing.T) {
	view := StandardProfileView(ProfileCore)
	projected := view.Catalog()
	for name, spec := range Standard.Skills {
		_, found := projected.Skills[name]
		if found == spec.FullOnly {
			t.Errorf("core skill membership %q = %v, FullOnly = %v", name, found, spec.FullOnly)
		}
	}
	if got := projected.Docs["workflow"].Desc; strings.Contains(got, "ADR") || strings.Contains(got, "plan") {
		t.Errorf("Core workflow description retains Full concepts: %q", got)
	}
	reviewer := projected.Agents["code-reviewer"]
	if got, _ := reviewer.Data["readStep"].(string); strings.Contains(got, "ADR") || strings.Contains(got, "plan") || strings.Contains(got, "state doc") {
		t.Errorf("Core reviewer read step retains Full concepts: %q", got)
	}
	for _, raw := range reviewer.Data["focusItems"].([]any) {
		item := raw.(map[string]any)
		text := fmt.Sprint(item["name"], " ", item["description"])
		if strings.Contains(text, "plan") || strings.Contains(text, "state") {
			t.Errorf("Core reviewer focus retains Full concepts: %q", text)
		}
	}
}
