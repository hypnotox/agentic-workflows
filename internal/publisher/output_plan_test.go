package publisher

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/outputplan"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: rendering/project-output-plan:output-plan-complete (TestLocalDocsOutputPlan)
// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocsOutputPlan)
func TestLocalDocsOutputPlan(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/z\n    title: Z\n    description: Z document.\n  - name: runbooks/a\n    title: A\n    description: A document.\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, node := range plan.Nodes {
		if strings.HasPrefix(node.Path, "docs/runbooks/") {
			paths = append(paths, node.Path)
			if node.Recipe.TemplateID != localDocTID || !node.Policy.ScanReferences || !node.Policy.ScanSkillReferences || !node.Policy.Regenerate || !strings.HasPrefix(node.Declarers[0], "local-doc:") {
				t.Fatalf("local node = %#v", node)
			}
		}
	}
	if !slices.Equal(paths, []string{"docs/runbooks/a.md", "docs/runbooks/z.md"}) {
		t.Fatalf("local paths = %v", paths)
	}
	if _, ok := layout(renderInputsForTest(p)).Docs["runbooks/a"]; ok {
		t.Fatal("local document entered catalog layout")
	}
	if projectCatalog(renderInputsForTest(p)).Docs["runbooks/a"].TID != "" {
		t.Fatal("local document entered catalog")
	}
}

// A local document's only section is edit-in-place, so once its output exists the
// next render reads that output back to preserve the authored body. The
// The rendered node must observe that self-input so later comparison uses the
// exact in-place universe that produced its bytes.
//
// invariant: rendering/project-output-plan:output-plan-complete (TestLocalDocRenderObservesExistingOutputInput)
// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocRenderObservesExistingOutputInput)
func TestLocalDocRenderObservesExistingOutputInput(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	const outPath = "docs/runbooks/incident.md"
	testsupport.WriteFile(t, filepath.Join(root, "docs", "runbooks", "incident.md"),
		"# Incident\n\n<!-- awf:edit-in-place body -->\nauthored body\n")

	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(plan.Nodes, func(node OutputNode) bool { return node.Path == outPath })
	if i < 0 {
		t.Fatalf("local document missing from plan")
	}
	if !slices.Contains(plan.Nodes[i].ConsumedInputs, OutputInput{Path: outPath, Role: outputplan.ArtifactManagedOutput}) {
		t.Fatalf("observed inputs = %v", plan.Nodes[i].ConsumedInputs)
	}
}

// TestOutputPlanPropagatesPreAdoptionEnumerationFault pins that a tree the
// planner cannot fully read fails the plan rather than producing one built from
// a truncated enumeration. A pre-adoption tree (no Git worktree) enumerates the
// filesystem directly, so an unreadable directory there used to be skipped and
// the plan, and the drift oracle computed from it, silently narrowed.
func TestOutputPlanPropagatesPreAdoptionEnumerationFault(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o755) })
	if _, err := outputPlanProject(p); err == nil {
		t.Fatal("output plan built from a truncated enumeration")
	}
}

func TestTargetDescriptorValidation(t *testing.T) {
	for _, target := range []artifactregistry.Target{
		{Name: "bad", SkillDir: ".bad/skills", BridgeFile: "X"},
		{Name: "bad"},
	} {
		if err := artifactregistry.ValidateTarget(target); err == nil {
			t.Fatalf("invalid target %#v was accepted", target)
		}
	}
	if _, err := resolveTargets([]string{"nope"}); err == nil {
		t.Fatal("unknown target resolved")
	}
}

// invariant: rendering/project-output-plan:bridge-render-identity (TestBridgeRenderIdentity)
func TestBridgeRenderIdentity(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nvars: {}\n", map[string]string{
		"target-bridge/.yaml": "data: {}\n",
		"claude/.yaml":        "data: {}\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	custom := artifactregistry.Target{
		Name:           "custom",
		SkillDir:       ".custom/workflows",
		BridgeFile:     "CUSTOM.md",
		BridgeTemplate: bridgeTID,
	}
	p = setTestTargets(p, []artifactregistry.Target{claudeTarget, custom, piTarget})
	plan, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]OutputNode{}
	for _, node := range plan.Nodes {
		byPath[node.Path] = node
	}
	for path, templateID := range map[string]string{"CLAUDE.md": bridgeTID, "CUSTOM.md": bridgeTID} {
		node, ok := byPath[path]
		if !ok || node.file == nil {
			t.Fatalf("missing rendered bridge node %s", path)
		}
		if node.file.kind != "target-bridge" {
			t.Errorf("%s render kind = %q, want target-bridge", path, node.file.kind)
		}
		if node.Recipe.TemplateID != templateID || node.ObservedTemplateID != templateID {
			t.Errorf("%s template identity = recipe %q observed %q, want %q", path, node.Recipe.TemplateID, node.ObservedTemplateID, templateID)
		}
		for _, sidecar := range []string{".awf/target-bridge/.yaml", ".awf/claude/.yaml"} {
			if slices.Contains(node.ConsumedInputs, OutputInput{Path: sidecar, Role: outputplan.ArtifactAuthoredData}) {
				t.Errorf("%s inherited fictitious sidecar input %s", path, sidecar)
			}
		}
		wantInputs := []OutputInput{
			{Path: ".awf/config.yaml", Role: outputplan.ArtifactConfig},
			{Path: "templates/" + templateID, Role: outputplan.ArtifactTemplate},
		}
		if !slices.Equal(node.ConsumedInputs, wantInputs) || len(node.DependsOn) != 0 {
			t.Errorf("%s source guidance changed machine inputs: inputs=%#v dependencies=%#v", path, node.ConsumedInputs, node.DependsOn)
		}
		if strings.Contains(node.file.Content, "<no value>") {
			t.Errorf("%s rendered an unset template value: %s", path, node.file.Content)
		}
		marker := "<!-- awf:source AGENTS.md -->\n"
		at := strings.Index(node.file.Content, marker)
		prefix := node.file.Content[:max(at, 0)]
		adjacent := strings.HasSuffix(prefix, "<!-- "+bannerText+" -->\n")
		if at < 0 || strings.Count(node.file.Content, marker) != 1 || !adjacent {
			t.Errorf("%s headingless bridge marker is absent, duplicated, or misplaced:\n%s", path, node.file.Content)
		}
	}
	if got := byPath["CUSTOM.md"].file.Content; !strings.Contains(got, "@AGENTS.md") {
		t.Errorf("custom bridge did not use its descriptor-owned Markdown template with empty vars:\n%s", got)
	}
	for _, path := range []string{
		".custom/workflows/awf-effort/SKILL.md",
	} {
		if _, ok := byPath[path]; !ok {
			t.Errorf("custom descriptor output %s is absent", path)
		}
	}
	if _, ok := byPath["PI.md"]; ok {
		t.Error("Pi emitted a bridge despite its empty declaration")
	}

	custom.BridgeTemplate = "missing/custom-bridge.tmpl"
	p = setTestTargets(p, []artifactregistry.Target{custom})
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "read template missing/custom-bridge.tmpl") {
		t.Fatalf("missing custom bridge template error = %v", err)
	}
}

func TestOutputPolicyIsExplicit(t *testing.T) {
	if got := declaredPolicy("efforts", false); got.ScanReferences {
		t.Fatalf("efforts policy = %#v", got)
	}
	if (outputplan.Policy{}).ScanReferences {
		t.Fatal("zero policy must not scan")
	}
}

// invariant: rendering/project-output-plan:output-plan-complete (TestBridgeRenderIdentity)
// invariant: rendering/project-output-plan:output-plan-complete (TestCurrentStateOutputPlanMatchesTree)
// removed-invariant (TestCurrentStateOutputPlanMatchesTree)
// invariant: rendering/catalog-and-targets:claude-md-bridge (TestCurrentStateOutputPlanMatchesTree)
// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries (TestCurrentStateOutputPlanMatchesTree)
// removed-invariant (TestCurrentStateOutputPlanMatchesTree)
// removed-invariant (TestCurrentStateOutputPlanMatchesTree)
// removed-invariant (TestCurrentStateOutputPlanMatchesTree)
func TestCurrentStateOutputPlanMatchesTree(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	migrationPath := ".awf/current-state-migration.yaml"
	seenTopics, seenDomains := 0, 0
	planned := map[string]bool{}
	for _, n := range op.Nodes {
		if n.Path == migrationPath {
			t.Fatal("permanent output plan still claims the deleted migration approval file")
		}
		if n.file == nil {
			continue
		}
		switch {
		case strings.HasPrefix(n.Path, "docs/topics/"):
			seenTopics++
			planned[n.Path] = true
		case strings.HasPrefix(n.Path, "docs/domains/"):
			seenDomains++
			planned[n.Path] = true
		default:
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(n.Path)))
		if err != nil {
			t.Errorf("planned current-state output %s is absent: %v", n.Path, err)
			continue
		}
		if string(raw) != n.file.Content {
			t.Errorf("planned current-state output %s does not match the tree", n.Path)
		}
	}
	if seenTopics == 0 || seenDomains != len(testConfig(p).Domains) {
		t.Fatalf("current-state output coverage: topics=%d domains=%d want-domains=%d", seenTopics, seenDomains, len(testConfig(p).Domains))
	}
	testsupport.WalkRepoFiles(t, root, func(rel string) bool {
		return filepath.Ext(rel) == ".md" &&
			(strings.HasPrefix(rel, "docs/topics/") || strings.HasPrefix(rel, "docs/domains/"))
	}, func(rel string, _ []byte) {
		if !planned[rel] {
			t.Errorf("current-state output %s exists but is absent from the output plan", rel)
		}
	})
	lock, err := manifest.Load(filepath.Join(root, ".awf", "awf.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lock.Files[migrationPath]; ok {
		t.Fatal("permanent lock still claims the deleted migration approval file")
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(migrationPath))); !os.IsNotExist(err) {
		t.Fatalf("migration approval file survives cutover: %v", err)
	}
}

func TestOutputPlanTopicNodesHaveLiteralPathsAndInputs(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := loadTestSession(testContext(t), root)
	op, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range op.Nodes {
		if n.Path == "docs/topics/rendering/contracts.md" {
			if len(n.DependsOn) != 2 {
				t.Fatalf("topic node = %#v", n)
			}
			return
		}
	}
	t.Fatal("literal topic output was absent from the plan")
}

// A local document's output is read twice on the way to a plan: collision
// preflight observes whether the in-place body exists, then its sole render
// closure reads the body back. Each read has its own propagation path, so each
// must surface its fault rather than be erased by the other's success.
func TestOutputPlanPropagatesLocalRenderReadFault(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failAfter int
	}{
		{"collision preflight", 0},
		{"render closure", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
			p, err := loadTestSession(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("local output read failed")
			calls := 0
			read := faultingProjectReader{TreeReader: filesystemProjectReader{root: root},
				path: "docs/runbooks/incident.md", err: failure, failAfter: tc.failAfter, calls: &calls}
			cfg, err := config.Load(config.RootDir(root))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := outputPlan(newRenderInputs(p, cfg, read, project.Version)); !errors.Is(err, failure) {
				t.Fatalf("output plan error = %v, want %v", err, failure)
			}
		})
	}
}

func TestOutputPlanPropagatesConfigReferenceRenderFault(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"parts/config-reference/intro.md": "<!-- awf:comment unclosed\n",
	})
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "malformed awf:comment") {
		t.Fatalf("output plan error = %v", err)
	}
}

func TestOutputPlanPropagatesTopicGenerationEnumerationFault(t *testing.T) {
	state := testStateAt(t.TempDir())
	calls := 0
	failure := errors.New("topic enumeration failed")
	reader := failingPathsReader{memoryProjectReader: memoryProjectReader{}, failAt: 1, calls: &calls}
	_, err := outputPlanWithPitfalls(newRenderInputs(state, &config.Config{}, reader, project.Version), pitfall.Corpus{}, topic.Corpus{})
	if err == nil || !strings.Contains(err.Error(), failure.Error()) && !strings.Contains(err.Error(), "enumeration fault") {
		t.Fatalf("output plan error = %v", err)
	}
}
