package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/adr"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
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

// TestBuildOutputDeclarationsPropagatesEnumerationFaults pins that a faulting
// tree read reaches the caller. Erasing it truncated the declaration set the
// drift oracle is computed over, so a partial enumeration reported a clean tree
// and exited 0.
func TestBuildOutputDeclarationsPropagatesEnumerationFaults(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndomains: [d]\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	// Three sites enumerate the tree, in call order: per-domain metadata for the
	// domain docs, the flat metadata list for topic docs, and per-domain metadata
	// for the topic indexes. Each must surface its own fault.
	for site := 1; site <= 3; site++ {
		t.Run("site"+strconv.Itoa(site), func(t *testing.T) {
			calls := 0
			faulting := failingPathsReader{memoryProjectReader: read, failAt: site, calls: &calls}
			if _, err := BuildOutputDeclarations(cfg, cat, nil, faulting, mustCorpus()); err == nil || !strings.Contains(err.Error(), "enumeration fault") {
				t.Fatalf("site %d: error = %v, want the enumeration fault", site, err)
			}
		})
	}
}

func TestBuildOutputDeclarationsPropagatesReadFaults(t *testing.T) {
	read := failingReadReader{memoryProjectReader: memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x")}}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndomains: [d]\n"), configReaderAdapter(read))
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	if _, err := BuildOutputDeclarations(cfg, cat, nil, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), "read fault") {
		t.Fatalf("error = %v, want the project-tree read fault", err)
	}
}

func outputDeclarationParityError(nodes []OutputNode, declarations []OutputDeclaration) error {
	if len(nodes) != len(declarations) {
		planPaths, declPaths := map[string]bool{}, map[string]bool{}
		for _, node := range nodes {
			planPaths[node.Path] = true
		}
		for _, declaration := range declarations {
			declPaths[declaration.Path] = true
		}
		var planOnly, declOnly []string
		for path := range planPaths {
			if !declPaths[path] {
				planOnly = append(planOnly, path)
			}
		}
		for path := range declPaths {
			if !planPaths[path] {
				declOnly = append(declOnly, path)
			}
		}
		slices.Sort(planOnly)
		slices.Sort(declOnly)
		return fmt.Errorf("declaration parity: plan-only %v, declarations-only %v", planOnly, declOnly)
	}
	for i := range nodes {
		node, declaration := nodes[i], declarations[i]
		if node.Path != declaration.Path ||
			!slices.Equal(node.Declarers, declaration.Declarers) ||
			!slices.Equal(node.ConsumedInputs, normalizeOutputInputs(declaration.Inputs)) ||
			!slices.Equal(node.DependsOn, declaration.Dependencies) {
			return fmt.Errorf("declaration parity at %q: plan declarers=%v consumed=%v dependencies=%v; declaration declarers=%v inputs=%v dependencies=%v",
				node.Path, node.Declarers, node.ConsumedInputs, node.DependsOn,
				declaration.Declarers, declaration.Inputs, declaration.Dependencies)
		}
	}
	return nil
}

// invariant: rendering/project-output-plan:conditional-unit-single-source (TestOutputDeclarationsMatchThePlan)
// invariant: rendering/project-output-plan:output-plan-complete (TestOutputDeclarationsMatchThePlan)
func TestOutputDeclarationsMatchThePlan(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	corpus, _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := BuildOutputDeclarations(testConfig(p), projectCatalog(renderInputsForTest(p)), p.Targets(), filesystemProjectReader{root: p.Root()}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := outputDeclarationParityError(plan.Nodes, declarations); err != nil {
		t.Fatal(err)
	}

	runnerNode := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == "awf" })
	runnerDeclaration := slices.IndexFunc(declarations, func(declaration OutputDeclaration) bool { return declaration.Path == "awf" })
	if runnerNode < 0 || runnerDeclaration < 0 {
		t.Fatal("enabled runner conditional unit missing from parity populations")
	}
	planOnly := slices.Delete(slices.Clone(declarations), runnerDeclaration, runnerDeclaration+1)
	if err := outputDeclarationParityError(plan.Nodes, planOnly); err == nil || !strings.Contains(err.Error(), "plan-only [awf]") {
		t.Fatalf("declaration-only omission escaped parity: %v", err)
	}
	declarationOnly := slices.Delete(slices.Clone(plan.Nodes), runnerNode, runnerNode+1)
	if err := outputDeclarationParityError(declarationOnly, declarations); err == nil || !strings.Contains(err.Error(), "declarations-only [awf]") {
		t.Fatalf("render-only omission escaped parity: %v", err)
	}
}

func TestEnabledDeclarationsRejectMissingBridgeTemplate(t *testing.T) {
	cfg := &config.Config{Prefix: "example", Render: &config.RenderConfig{TemplateSourceRoot: "templates"}}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	targets := []Target{{Name: "broken", BridgeFile: "BRIDGE.md", BridgeTemplate: "missing/bridge.md.tmpl"}}
	_, err := BuildOutputDeclarations(cfg, cat, targets, memoryProjectReader{}, mustCorpus())
	if err == nil || !strings.Contains(err.Error(), "read template missing/bridge.md.tmpl") {
		t.Fatalf("missing enabled bridge template error = %v", err)
	}
}

func TestEnabledMarkdownDeclarationsMatchObservedTemplateSources(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	testConfig(p).Render = &config.RenderConfig{TemplateSourceRoot: "templates"}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	corpus, _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := BuildOutputDeclarations(testConfig(p), projectCatalog(renderInputsForTest(p)), p.Targets(), filesystemProjectReader{root: p.Root()}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	path := "docs/architecture.md"
	nodeIndex := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	declarationIndex := slices.IndexFunc(declarations, func(declaration OutputDeclaration) bool { return declaration.Path == path })
	if nodeIndex < 0 || declarationIndex < 0 {
		t.Fatalf("ordinary Markdown output missing: plan=%d declaration=%d", nodeIndex, declarationIndex)
	}
	if got, want := plan.Nodes[nodeIndex].ConsumedInputs, normalizeOutputInputs(declarations[declarationIndex].Inputs); !slices.Equal(got, want) {
		t.Fatalf("enabled declaration/observation parity:\n got %#v\nwant %#v", got, want)
	}
	if !slices.Contains(plan.Nodes[nodeIndex].ConsumedInputs, OutputInput{Path: "templates/docs/architecture.md.tmpl", Role: ArtifactTemplate}) {
		t.Fatalf("configured root source was not observed: %#v", plan.Nodes[nodeIndex].ConsumedInputs)
	}
}

func TestPitfallDeclarationPlanDependencyParity(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/alpha.md": pitfallSource("Alpha", "", "alpha body\n"),
		"docs/pitfalls/beta.md":  pitfallSource("Beta", "", "beta body\n"),
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	corpus, _, _, _, err := deriveOperationStateWithPitfalls(renderInputsForTest(p))
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := BuildOutputDeclarations(testConfig(p), projectCatalog(renderInputsForTest(p)), p.Targets(), filesystemProjectReader{root: root}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := outputDeclarationParityError(plan.Nodes, declarations); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"docs/pitfalls.md":       {".awf/docs/pitfalls/alpha.md", ".awf/docs/pitfalls/beta.md"},
		"docs/pitfalls/alpha.md": {".awf/docs/pitfalls/alpha.md"},
		"docs/pitfalls/beta.md":  {".awf/docs/pitfalls/beta.md"},
	}
	for path, deps := range want {
		node := slices.IndexFunc(plan.Nodes, func(n OutputNode) bool { return n.Path == path })
		decl := slices.IndexFunc(declarations, func(d OutputDeclaration) bool { return d.Path == path })
		if node < 0 || decl < 0 || !slices.Equal(plan.Nodes[node].DependsOn, deps) || !slices.Equal(declarations[decl].Dependencies, deps) {
			t.Fatalf("%s dependency parity: node=%v declaration=%v want=%v", path, func() []string {
				if node < 0 {
					return nil
				}
				return plan.Nodes[node].DependsOn
			}(), func() []string {
				if decl < 0 {
					return nil
				}
				return declarations[decl].Dependencies
			}(), deps)
		}
	}
	mutated := slices.Clone(declarations)
	idx := slices.IndexFunc(mutated, func(d OutputDeclaration) bool { return d.Path == "docs/pitfalls/alpha.md" })
	mutated[idx].Dependencies = nil
	if err := outputDeclarationParityError(plan.Nodes, mutated); err == nil || !strings.Contains(err.Error(), "dependencies") {
		t.Fatalf("missing pitfall dependency escaped parity: %v", err)
	}
}

func TestOutputPlanObservesConsumedInputsIndependently(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nprofile: full\nintegrationBranch: main\n"+debuggingVars+"", map[string]string{
		"skills/debugging.yaml":                        "data: {}\n",
		"skills/parts/debugging/debugging-surfaces.md": "Observed part.\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	path := p.Targets()[0].SkillPath(testConfig(p).Prefix, "debugging")
	idx := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == path })
	if idx < 0 {
		t.Fatalf("missing node %s", path)
	}
	want := normalizeOutputInputs([]OutputInput{
		{Path: ".awf/config.yaml", Role: ArtifactConfig},
		{Path: ".awf/skills/debugging.yaml", Role: ArtifactAuthoredData},
		{Path: ".awf/skills/parts/debugging/debugging-surfaces.md", Role: ArtifactConventionPart},
		{Path: "templates/skills/debugging/SKILL.md.tmpl", Role: ArtifactTemplate},
	})
	if !reflect.DeepEqual(plan.Nodes[idx].ConsumedInputs, want) {
		t.Fatalf("consumed inputs = %#v, want %#v", plan.Nodes[idx].ConsumedInputs, want)
	}
}

// normalizeOutputInputs is the render seam's observed-input normalizer: two
// roles recorded at one path keep both entries and sort by role, so a
// role-misclassified input is still visible to every consumer of the plan.
func TestNormalizeOutputInputsOrdersRolesAtOnePath(t *testing.T) {
	rolesAtOnePath := normalizeOutputInputs([]OutputInput{{Path: "same", Role: ArtifactTemplate}, {Path: "same", Role: ArtifactConfig}})
	if !reflect.DeepEqual(rolesAtOnePath, []OutputInput{{Path: "same", Role: ArtifactConfig}, {Path: "same", Role: ArtifactTemplate}}) {
		t.Fatalf("same-path role ordering = %#v", rolesAtOnePath)
	}
}

// Full-catalog declaration planning must parse every standard sidecar, even
// when Phase 2's legacy selection arrays omit that artifact.
func TestBuildOutputDeclarationsRejectsMalformedFullCatalogSidecars(t *testing.T) {
	for _, tc := range []struct {
		name, kind, artifact, config, sidecar string
	}{
		{"skill", "skills", "tdd", "", ".awf/skills/tdd.yaml"},
		{"agent", "agents", "code-reviewer", "", ".awf/agents/code-reviewer.yaml"},
		{"doc", "docs", "architecture", "", ".awf/docs/architecture.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read := memoryProjectReader{tc.sidecar: []byte("local: [bad")}
			cfg, err := config.ParseTree(".awf", []byte("prefix: example\n"+tc.config), configReaderAdapter{read})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := cfg.Sidecar(tc.kind, tc.artifact); err == nil {
				t.Fatalf("test fixture did not corrupt %s sidecar", tc.name)
			}
			if _, err := BuildOutputDeclarations(cfg, catalog.Standard, []Target{{Name: "test"}}, read, mustCorpus()); err == nil || !strings.Contains(err.Error(), tc.sidecar[5:]) {
				t.Fatalf("full-catalog malformed %s sidecar error = %v", tc.name, err)
			}
		})
	}
}

func TestResolvedTargetOutputsFiltersRequiredSkills(t *testing.T) {
	target := Target{SkillDir: ".target/skills", Outputs: []TargetOutput{{Path: "always"}, {Path: "conditional", RequiresSkill: "tdd"}, {SkillName: "workflow", RequiresSkill: "effort-workflow"}}}
	outputs := resolvedTargetOutputs(target, "example", []string{"tdd"})
	if len(outputs) != 2 || outputs[0].Path != "always" || outputs[1].Path != "conditional" {
		t.Fatalf("filtered outputs = %#v", outputs)
	}
	outputs = resolvedTargetOutputs(target, "example", []string{"effort-workflow"})
	if len(outputs) != 2 || outputs[1].Path != ".target/skills/example-workflow/SKILL.md" {
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
	ProjectTreeReader
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
	return r.ProjectTreeReader.ReadFile(path)
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

// mustCorpus builds a corpus from fixture records that carry no duplicate
// identity, so the construction error the seam returns cannot occur here.
func mustCorpus() adr.Corpus {
	c, err := adr.NewCorpus(nil)
	if err != nil { // coverage-ignore: fixture records are duplicate-free by construction
		panic(err)
	}
	return c
}
