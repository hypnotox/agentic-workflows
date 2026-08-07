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
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: []\nagents: []\ndomains: [d]\n"), configReaderAdapter{read})
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
			if _, err := BuildOutputDeclarations(cfg, cat, nil, faulting, mustCorpus(nil)); err == nil || !strings.Contains(err.Error(), "enumeration fault") {
				t.Fatalf("site %d: error = %v, want the enumeration fault", site, err)
			}
		})
	}
}

func TestBuildOutputDeclarationsPropagatesReadFaults(t *testing.T) {
	read := failingReadReader{memoryProjectReader: memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x")}}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: []\nagents: []\ndomains: [d]\n"), configReaderAdapter(read))
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{}, Agents: map[string]catalog.AgentSpec{}, Docs: map[string]catalog.DocEntry{}}
	if _, err := BuildOutputDeclarations(cfg, cat, nil, read, mustCorpus(nil)); err == nil || !strings.Contains(err.Error(), "read fault") {
		t.Fatalf("error = %v, want the project-tree read fault", err)
	}
}

func TestBuildOutputDeclarationsFamiliesAndReservations(t *testing.T) {
	read := memoryProjectReader{".awf/topics/metadata/d/t.yaml": []byte("x"), ".awf/topics/metadata/d/readme.txt": []byte("x"), ".awf/docs/architecture.yaml": []byte("local: true\n"), ".awf/skills/local.yaml": []byte("local: true\n"), ".awf/skills/parts/local/content.md": []byte("part"), ".awf/agents/agent.yaml": []byte("local: true\n"), ".awf/agents/parts/agent/content.md": []byte("part"), "docs/decisions/0001-real.md": []byte("parsed"), "docs/decisions/0002-malformed.md": []byte("not parsed"), "docs/decisions/INDEX.md": []byte("generated"), "docs/decisions/README.md": []byte("navigation")}
	cfg, err := config.ParseTree(".awf", []byte("prefix: p\ndocsDir: docs\nskills: [local]\nagents: [agent]\ndocs: [enabled, architecture]\ndomains: [d]\nbootstrap: {enabled: true}\nvars: {gateCmd: test-gate}\n"), configReaderAdapter{read})
	if err != nil {
		t.Fatal(err)
	}
	cat := &catalog.Catalog{Skills: map[string]catalog.SkillSpec{"local": {Base: true, Sections: []string{"content"}}}, Agents: map[string]catalog.AgentSpec{"agent": {Base: true, Sections: []string{"content"}}}, Docs: map[string]catalog.DocEntry{"agents-doc": {Mandatory: true, AgentsDoc: true, TID: "agents-doc/AGENTS.md.tmpl"}, "architecture": {Path: "architecture.md", TID: "docs/architecture.md.tmpl"}, "disabled": {Path: "disabled.md", TID: "docs/disabled.md.tmpl"}, "enabled": {Path: "enabled.md", TID: "docs/enabled.md.tmpl"}}}
	target := Target{Name: "one", SkillDir: ".one/skills", Outputs: []TargetOutput{{Path: "shared", TemplateID: "target.tmpl", Producer: TargetOutputTemplate}}}
	other := target
	other.Name = "two"
	// A target output may declare its own producer inputs; the declaration pass
	// carries them through verbatim (target validation rejects the shape later,
	// so only this pass ever sees them).
	withInputs := Target{Name: "three", SkillDir: ".three/skills", Outputs: []TargetOutput{{Path: "declared-inputs", TemplateID: "target.tmpl", Producer: TargetOutputTemplate, Inputs: []TargetOutputInput{{Path: ".awf/extension.json", Role: ArtifactProtocolDescriptor}}}}}
	parsedADRs := mustCorpus([]adr.ADR{{Number: "0001", Filename: "0001-real.md"}})
	badRequirement := target
	badRequirement.Outputs = []TargetOutput{{Path: "gated", TemplateID: "target.tmpl", Producer: TargetOutputTemplate, RequiresSkill: "missing"}}
	if _, err := BuildOutputDeclarations(cfg, cat, []Target{badRequirement}, read, parsedADRs); err == nil || !strings.Contains(err.Error(), "unknown catalog skill") {
		t.Fatalf("declaration accepted unknown target-output requirement: %v", err)
	}
	decls, err := BuildOutputDeclarations(cfg, cat, []Target{target, other, withInputs}, read, parsedADRs)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]OutputDeclaration{}
	for _, d := range decls {
		byPath[d.Path] = d
	}
	for _, p := range []string{".one/skills/p-local/SKILL.md", "shared", "AGENTS.md", "docs/domains/d.md", "docs/topics/d/t.md", "docs/decisions/INDEX.md", "awf", ".awf/bootstrap.sh", ".awf/upgrade.sh", ".awf/hooks/pre-commit.sh", ".awf/hooks/reference-transaction.sh", ".awf/efforts/.gitignore", ".awf/worktrees/.gitignore"} {
		if _, ok := byPath[p]; !ok {
			t.Errorf("missing %s", p)
		}
	}
	for _, tc := range []struct {
		read memoryProjectReader
		body string
	}{{memoryProjectReader{".awf/agents-doc.yaml": []byte("local: [bad")}, "prefix: p\n"}, {memoryProjectReader{".awf/docs/enabled.yaml": []byte("local: [bad")}, "prefix: p\ndocs: [enabled]\n"}} {
		badCfg, _ := config.ParseTree(".awf", []byte(tc.body), configReaderAdapter{tc.read})
		if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, tc.read, mustCorpus(nil)); err == nil {
			t.Fatal("malformed document declaration accepted")
		}
	}
	badSkillRead := memoryProjectReader{".awf/skills/bad.yaml": []byte("local: [bad")}
	badSkillCfg, _ := config.ParseTree(".awf", []byte("prefix: p\nskills: [bad]\n"), configReaderAdapter{badSkillRead})
	if _, err := BuildOutputDeclarations(badSkillCfg, cat, []Target{target}, badSkillRead, mustCorpus(nil)); err == nil {
		t.Fatal("malformed skill declaration accepted")
	}
	badRead := memoryProjectReader{".awf/agents/bad.yaml": []byte("local: [bad")}
	badCfg, _ := config.ParseTree(".awf", []byte("prefix: p\nagents: [bad]\n"), configReaderAdapter{badRead})
	if _, err := BuildOutputDeclarations(badCfg, cat, []Target{target}, badRead, mustCorpus(nil)); err == nil {
		t.Fatal("malformed agent declaration accepted")
	}
	badDomainRead := memoryProjectReader{".awf/domains/d.yaml": []byte("paths: [bad")}
	badDomainCfg, _ := config.ParseTree(".awf", []byte("prefix: p\ndomains: [d]\n"), configReaderAdapter{badDomainRead})
	if _, err := BuildOutputDeclarations(badDomainCfg, cat, []Target{target}, badDomainRead, mustCorpus(nil)); err == nil {
		t.Fatal("malformed domain declaration accepted")
	}
	if !byPath[".one/skills/p-local/SKILL.md"].Reservation || !reflect.DeepEqual(byPath["shared"].Declarers, []string{"one", "two"}) {
		t.Fatalf("declarations=%#v", decls)
	}
	if !slices.Contains(byPath["declared-inputs"].Inputs, OutputInput{Path: ".awf/extension.json", Role: ArtifactProtocolDescriptor}) {
		t.Errorf("target-declared output inputs dropped: %#v", byPath["declared-inputs"])
	}
	// A standard catalog doc whose sidecar declares it local is hand-maintained:
	// it must produce no declaration at all, not a managed output.
	if _, declared := byPath["docs/architecture.md"]; declared {
		t.Errorf("a local standard doc was still declared: %#v", byPath["docs/architecture.md"])
	}
	index := byPath["docs/decisions/INDEX.md"]
	decisionInputs := []string{}
	for _, input := range index.Inputs {
		if input.Role == ArtifactDecisionRecord {
			decisionInputs = append(decisionInputs, input.Path)
		}
	}
	if !reflect.DeepEqual(decisionInputs, []string{"docs/decisions/0001-real.md"}) {
		t.Fatalf("decision inputs include unparsed lookalikes: %v", decisionInputs)
	}
}

// TestOutputDeclarationsMatchThePlan reinstates the retired
// validateDeclarationPlanParity guard as a structural test over this
// repository: BuildOutputDeclarations and the output plan remain two
// independent enumerations of the same producer set, and a producer added to
// one but not the other silently corrupts contextq's generated-output
// classification, whose only feed is the declarations. Template identity is
// deliberately excluded from the comparison: both sides derive it from the
// same declaration tables (ADR-0195 item 5), so that axis would compare the
// derivation with itself. The other five axes are compared exactly as the
// runtime check did.
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
		if node.Path != declaration.Path || node.Reservation != declaration.Reservation ||
			!slices.Equal(node.Declarers, declaration.Declarers) ||
			!slices.Equal(node.ConsumedInputs, normalizeOutputInputs(declaration.Inputs)) ||
			!slices.Equal(node.DependsOn, declaration.Dependencies) {
			return fmt.Errorf("declaration parity at %q: plan declarers=%v consumed=%v dependencies=%v reservation=%t; declaration declarers=%v inputs=%v dependencies=%v reservation=%t",
				node.Path, node.Declarers, node.ConsumedInputs, node.DependsOn, node.Reservation,
				declaration.Declarers, declaration.Inputs, declaration.Dependencies, declaration.Reservation)
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
	plan, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	corpus, _, _, err := p.deriveOperationState()
	if err != nil {
		t.Fatal(err)
	}
	declarations, err := BuildOutputDeclarations(p.Cfg, p.Cat, p.Targets, filesystemProjectReader{root: p.Root}, corpus)
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

func TestOutputPlanObservesConsumedInputsIndependently(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n"+debuggingVars+"skills: [debugging, exploring]\nagents: [explorer]\n", map[string]string{
		"skills/debugging.yaml":                        "data: {}\n",
		"skills/parts/debugging/debugging-surfaces.md": "Observed part.\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	path := p.Targets[0].SkillPath(p.Cfg.Prefix, "debugging")
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
func mustCorpus(records []adr.ADR) adr.Corpus {
	c, err := adr.NewCorpus(records)
	if err != nil { // coverage-ignore: fixture records are duplicate-free by construction
		panic(err)
	}
	return c
}
