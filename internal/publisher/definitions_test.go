package publisher

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/snapshot"
)

func TestProjectTreeReaders(t *testing.T) {
	tree, err := snapshot.NewTree([]snapshot.File{{Path: "a.txt", Mode: snapshot.Regular, Bytes: []byte("a")}, {Path: "link", Mode: snapshot.Symlink, Bytes: []byte("a.txt")}})
	if err != nil {
		t.Fatal(err)
	}
	r := snapshotTreeReader{tree: tree}
	b, ok, err := r.ReadFile("a.txt")
	if err != nil || !ok || string(b) != "a" {
		t.Fatal("snapshot read")
	}
	b[0] = 'X'
	again, _, _ := r.ReadFile("a.txt")
	if string(again) != "a" {
		t.Fatal("snapshot alias")
	}
	if _, ok, err := r.ReadFile("link"); err != nil || ok {
		t.Fatal("scanned symlink")
	}
	if got, err := r.Paths(""); err != nil || !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("paths=%v err=%v", got, err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := filesystemProjectReader{root: root}
	if b, ok, err := fr.ReadFile("a.txt"); err != nil || !ok || string(b) != "a" {
		t.Fatal("filesystem read")
	}
	if _, ok, err := fr.ReadFile("missing"); err != nil || ok {
		t.Fatal("missing read")
	}
	if got, err := fr.Paths(""); err != nil || !reflect.DeepEqual(got, []string{"a.txt"}) {
		t.Fatalf("filesystem paths=%v err=%v", got, err)
	}
	// An absent prefix stays an empty enumeration rather than a fault.
	if got, err := fr.Paths("missing"); err != nil || len(got) != 0 {
		t.Fatalf("missing paths=%v err=%v", got, err)
	}
	// A directory that cannot be read is a fault, not a short list: erasing it
	// would compute the drift oracle over a silently truncated tree.
	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(denied, "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := fr.ReadFile("denied"); err == nil || ok {
		t.Fatalf("directory read = ok %t, error %v; want a propagated non-file error", ok, err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })
	if got, err := fr.Paths(""); err == nil {
		t.Fatalf("unreadable directory erased: paths=%v", got)
	} else if !strings.Contains(err.Error(), "enumerate project tree") {
		t.Fatalf("whole-tree fault names an empty subject: %v", err)
	}
	// A named prefix keeps its own subject rather than the whole-tree wording.
	if got, err := fr.Paths("denied"); err == nil {
		t.Fatalf("unreadable prefix erased: paths=%v", got)
	} else if !strings.Contains(err.Error(), "enumerate denied") {
		t.Fatalf("prefixed fault lost its subject: %v", err)
	}
}

// TestBuildOutputDefinitionsPropagatesEnumerationFaults pins that a faulting
// tree read reaches the caller. Erasing it truncated the declaration set the
// drift oracle is computed over, so a partial enumeration reported a clean tree
// and exited 0.
func TestBuildOutputDefinitionsPropagatesEnumerationFaults(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndomains: [d]\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Docs: map[string]catalog.DocEntry{}}
	// Definitions enumerate the pitfall sources and the flat topic metadata set.
	// Each path-only population read must surface its own fault.
	for site := 1; site <= 2; site++ {
		t.Run("site"+strconv.Itoa(site), func(t *testing.T) {
			calls := 0
			faulting := failingPathsReader{memoryProjectReader: read, failAt: site, calls: &calls}
			if _, err := buildOutputDefinitions(cfg, cat, nil, faulting); err == nil || !strings.Contains(err.Error(), "enumeration fault") {
				t.Fatalf("site %d: error = %v, want the enumeration fault", site, err)
			}
		})
	}
}

// invariant: rendering/project-output-plan:conditional-unit-single-source (TestDefinitionsCoverRenderedPlan)
// invariant: rendering/project-output-plan:output-plan-complete (TestDefinitionsCoverRenderedPlan)
// invariant: rendering/sync-and-drift:managed-output-attribution (TestDefinitionsCoverRenderedPlan)
func TestDefinitionsCoverRenderedPlan(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(state)
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := buildOutputDefinitions(testConfig(state), projectCatalog(renderInputsForTest(state)), state.Targets(), filesystemProjectReader{root: state.Root()})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Nodes) != len(definitions) {
		t.Fatalf("rendered nodes=%d definitions=%d", len(plan.Nodes), len(definitions))
	}
	for i := range plan.Nodes {
		node, definition := plan.Nodes[i], definitions[i]
		if node.Path != definition.Path || !slices.Equal(node.Declarers, definition.Declarers) || !slices.Equal(node.DependsOn, definition.Dependencies) {
			t.Fatalf("definition %q != rendered node: definition=%#v node=%#v", definition.Path, definition, node)
		}
	}
}

func TestEnabledDefinitionMissingBridgeTemplateRefusesBeforeOutput(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	state = setTestTargets(state, []artifactregistry.Target{{Name: "broken", BridgeFile: "BRIDGE.md", BridgeTemplate: "missing/bridge.md.tmpl", AgentDialect: artifactregistry.MarkdownAgentDialect}})
	if _, err := outputPlanProject(state); err == nil || !strings.Contains(err.Error(), "read template missing/bridge.md.tmpl") {
		t.Fatalf("missing enabled bridge template error = %v", err)
	}
}

func TestMarkdownRenderObservesTemplateSources(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(state).Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	plan, err := outputPlanProject(state)
	if err != nil {
		t.Fatal(err)
	}
	path := "docs/architecture.md"
	i := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	if i < 0 || !slices.Contains(plan.Nodes[i].ConsumedInputs, OutputInput{Path: "templates/docs/architecture.md.tmpl", Role: outputplan.ArtifactTemplate}) {
		t.Fatalf("configured root source was not observed: %#v", plan.Nodes[i].ConsumedInputs)
	}
}

func TestPitfallDefinitionsPreserveDependencies(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/alpha.md": pitfallSource("Alpha", "", "alpha body\n"),
		"docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
	})
	state, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(state)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"docs/pitfalls.md":       {".awf/docs/pitfalls/alpha.md", ".awf/docs/pitfalls/beta.md"},
		"docs/pitfalls/alpha.md": {".awf/docs/pitfalls/alpha.md"},
		"docs/pitfalls/beta.md":  {".awf/docs/pitfalls/beta.md"},
	}
	for path, dependencies := range want {
		i := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
		if i < 0 || !slices.Equal(plan.Nodes[i].DependsOn, dependencies) {
			t.Fatalf("%s dependencies=%v want=%v", path, plan.Nodes[i].DependsOn, dependencies)
		}
	}
}

func TestOutputPlanObservesConsumedInputsIndependently(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n"+debuggingVars+"", map[string]string{
		"skills/awf-maintenance.yaml":                         "data: {}\n",
		"skills/parts/awf-maintenance/generated-documents.md": "Observed part.\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	path := p.Targets()[0].SkillPath("awf-maintenance")
	idx := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	if idx < 0 {
		t.Fatalf("missing node %s", path)
	}
	want := normalizeOutputInputs([]OutputInput{
		{Path: ".awf/config.yaml", Role: outputplan.ArtifactConfig},
		{Path: ".awf/skills/awf-maintenance.yaml", Role: outputplan.ArtifactAuthoredData},
		{Path: ".awf/skills/parts/awf-maintenance/generated-documents.md", Role: outputplan.ArtifactConventionPart},
		{Path: "templates/skills/awf-maintenance/SKILL.md.tmpl", Role: outputplan.ArtifactTemplate},
	})
	if !reflect.DeepEqual(plan.Nodes[idx].ConsumedInputs, want) {
		t.Fatalf("consumed inputs = %#v, want %#v", plan.Nodes[idx].ConsumedInputs, want)
	}
}

// normalizeOutputInputs is the render seam's observed-input normalizer: two
// roles recorded at one path keep both entries and sort by role, so a
// role-misclassified input is still visible to every consumer of the plan.
func TestNormalizeOutputInputsOrdersRolesAtOnePath(t *testing.T) {
	rolesAtOnePath := normalizeOutputInputs([]OutputInput{{Path: "same", Role: outputplan.ArtifactTemplate}, {Path: "same", Role: outputplan.ArtifactConfig}})
	if !reflect.DeepEqual(rolesAtOnePath, []OutputInput{{Path: "same", Role: outputplan.ArtifactConfig}, {Path: "same", Role: outputplan.ArtifactTemplate}}) {
		t.Fatalf("same-path role ordering = %#v", rolesAtOnePath)
	}
}

func TestResolvedTargetOutputsFiltersRequiredSkills(t *testing.T) {
	target := artifactregistry.Target{SkillDir: ".target/skills", Outputs: []artifactregistry.TargetOutput{{Path: "always"}, {Path: "conditional", RequiresSkill: "implementing"}, {SkillName: "workflow", RequiresSkill: "effort-workflow"}}}
	outputs := resolvedTargetOutputs(target, "example", []string{"implementing"})
	if len(outputs) != 2 || outputs[0].Path != "always" || outputs[1].Path != "conditional" {
		t.Fatalf("filtered outputs = %#v", outputs)
	}
	outputs = resolvedTargetOutputs(target, "example", []string{"effort-workflow"})
	if len(outputs) != 2 || outputs[1].Path != ".target/skills/workflow/SKILL.md" {
		t.Fatalf("skill-path outputs = %#v", outputs)
	}
}

type memoryProjectReader map[string][]byte

func (r memoryProjectReader) ReadFile(p string) ([]byte, bool, error) {
	b, ok := r[p]
	return append([]byte(nil), b...), ok, nil
}
func (r memoryProjectReader) Paths(prefix string) ([]string, error) {
	out := []string{}
	for p := range r {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out, nil
}

type failingReadReader struct{ memoryProjectReader }

func (r failingReadReader) ReadFile(string) ([]byte, bool, error) {
	return nil, false, errors.New("project-tree read fault")
}

type faultingProjectReader struct {
	outputplan.TreeReader
	path      string
	err       error
	failAfter int
	calls     *int
}

func (r faultingProjectReader) ReadFile(path string) ([]byte, bool, error) {
	if path == r.path {
		if r.calls != nil {
			*r.calls++
		}
		if r.failAfter == 0 || r.calls != nil && *r.calls > r.failAfter {
			return nil, false, r.err
		}
	}
	return r.TreeReader.ReadFile(path)
}

// failingPathsReader faults the failAt'th Paths call so each propagation site
// can be exercised on its own.
type failingPathsReader struct {
	memoryProjectReader
	failAt int
	calls  *int
}

func (r failingPathsReader) Paths(prefix string) ([]string, error) {
	*r.calls++
	if *r.calls == r.failAt {
		return nil, errors.New("enumeration fault at " + prefix)
	}
	return r.memoryProjectReader.Paths(prefix)
}

type configReaderAdapter struct{ memoryProjectReader }

func (r configReaderAdapter) ReadFile(p string) ([]byte, bool) {
	b, ok, _ := r.memoryProjectReader.ReadFile(".awf/" + p)
	return b, ok
}
func (r configReaderAdapter) Paths(prefix string) []string { return nil }
