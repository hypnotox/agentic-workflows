package publisher

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"github.com/hypnotox/agentic-workflows/internal/project"
	"github.com/hypnotox/agentic-workflows/internal/projectstate"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"github.com/hypnotox/agentic-workflows/internal/topic"
)

// invariant: rendering/project-output-plan:output-plan-complete (TestLocalDocsOutputPlan)
// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocsOutputPlan)
func TestLocalDocsOutputPlan(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/z\n    title: Z\n    description: Z document.\n  - name: runbooks/a\n    title: A\n    description: A document.\n")
	p, err := Open(testContext(t), root)
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
// declaration must project that self-input; otherwise declaration/plan parity
// holds on the first render (output absent) and breaks on every render after it.
//
// invariant: rendering/project-output-plan:output-plan-complete (TestLocalDocDeclarationDeclaresExistingOutputInput)
// invariant: rendering/doc-outputs:local-doc-output-complete (TestLocalDocDeclarationDeclaresExistingOutputInput)
func TestLocalDocDeclarationDeclaresExistingOutputInput(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
	const outPath = "docs/runbooks/incident.md"
	testsupport.WriteFile(t, filepath.Join(root, "docs", "runbooks", "incident.md"),
		"# Incident\n\n<!-- awf:edit-in-place body -->\nauthored body\n")

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
	declarations, err := buildOutputDeclarations(testConfig(p), projectCatalog(renderInputsForTest(p)), p.Targets(), filesystemProjectReader{root: p.Root()}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := outputDeclarationParityError(plan.Nodes, declarations); err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(declarations, func(d OutputDeclaration) bool { return d.Path == outPath })
	if i < 0 {
		t.Fatalf("local document missing from declarations")
	}
	if !slices.Contains(declarations[i].Inputs, OutputInput{Path: outPath, Role: ArtifactManagedOutput}) {
		t.Fatalf("declared inputs = %v", declarations[i].Inputs)
	}
}

func TestLocalDocCollisionWithTargetOutputPrecedesRendering(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/x\n    title: X\n    description: X document.\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	p = setTestTargets(p, append(testTargets(p), Target{Name: "collision", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "docs/runbooks/x.md", TemplateID: "docs/architecture.md.tmpl", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}}))
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "collides with managed output") {
		t.Fatalf("collision error = %v", err)
	}
}

// TestOutputPlanPropagatesPreAdoptionEnumerationFault pins that a tree the
// planner cannot fully read fails the plan rather than producing one built from
// a truncated enumeration. A pre-adoption tree (no Git worktree) enumerates the
// filesystem directly, so an unreadable directory there used to be skipped and
// the plan, and the drift oracle computed from it, silently narrowed.
func TestOutputPlanPropagatesPreAdoptionEnumerationFault(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\ndomains: [rendering]\n")
	p, err := Open(testContext(t), root)
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

// invariant: rendering/project-output-plan:target-capabilities-closed (TestTargetDescriptorValidation)
// removed-invariant (TestTargetDescriptorValidation)
// removed-invariant (TestTargetDescriptorValidation)
func TestTargetDescriptorValidation(t *testing.T) {
	for _, target := range []Target{
		{Name: "bad", BridgeFile: "X"},
		{Name: "bad", Capabilities: []Capability{"unknown"}},
		{Name: "bad", AgentDialect: "unknown"},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "../bad", TemplateID: "x"}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{SkillName: "x/y", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{SkillName: "../escape", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{SkillName: "/absolute", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", SkillName: "ambiguous", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: "unknown"}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Inputs: []TargetOutputInput{{Path: "unexpected", Role: ArtifactTemplate}}}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: "unknown", Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: 99, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{ScanReferences: true}, PolicyDeclared: true}}},
	} {
		if err := projectstate.ValidateTarget(target); err == nil {
			t.Fatalf("invalid target %#v was accepted", target)
		}
		root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		p = setTestTargets(p, []Target{target})
		if _, err := outputPlanProject(p); err == nil {
			t.Fatalf("planner accepted invalid target %#v", target)
		}
	}
	if got := targetTemplateData(piTarget)["targetSubagentTools"]; got != true {
		t.Fatalf("Pi subagent capability projection = %#v", got)
	}
	if got := targetTemplateData(piTarget)["targetSessionHandoff"]; got != true {
		t.Fatalf("Pi handoff capability projection = %#v", got)
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
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	custom := Target{
		Name:           "custom",
		SkillDir:       ".custom/workflows",
		AgentDir:       ".custom/reviewers",
		AgentSuffix:    ".agent.md",
		AgentDialect:   MarkdownAgentDialect,
		BridgeFile:     "CUSTOM.md",
		BridgeTemplate: bridgeTID,
		Capabilities:   []Capability{CapabilitySubagentTools, CapabilitySessionHandoff},
		Outputs: []TargetOutput{{
			Path: ".custom/extension.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl",
			Producer: TargetOutputTemplate, Encoder: PlainAgentDialect,
			Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true,
		}},
	}
	p = setTestTargets(p, []Target{claudeTarget, custom, piTarget})
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
			if slices.Contains(node.ConsumedInputs, OutputInput{Path: sidecar, Role: ArtifactAuthoredData}) {
				t.Errorf("%s inherited fictitious sidecar input %s", path, sidecar)
			}
		}
		wantInputs := []OutputInput{
			{Path: ".awf/config.yaml", Role: ArtifactConfig},
			{Path: "templates/" + templateID, Role: ArtifactTemplate},
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
		".custom/workflows/example-implementing/SKILL.md",
		".custom/reviewers/reviewer.agent.md",
		".custom/extension.ts",
	} {
		if _, ok := byPath[path]; !ok {
			t.Errorf("custom descriptor output %s is absent", path)
		}
	}
	if !targetTemplateData(custom)["targetSubagentTools"].(bool) || !targetTemplateData(custom)["targetSessionHandoff"].(bool) {
		t.Error("custom descriptor capabilities were not projected")
	}
	for _, output := range piTarget.Outputs {
		if output.RequiresSkill != "" {
			continue
		}
		node, ok := byPath[output.Path]
		if !ok || node.Recipe.TemplateID != output.TemplateID || node.Recipe.Encoder != output.Encoder {
			t.Errorf("Pi target output %s changed: %#v", output.Path, node)
		}
	}
	if _, ok := byPath["PI.md"]; ok {
		t.Error("Pi emitted a bridge despite its empty declaration")
	}

	custom.BridgeTemplate = "missing/custom-bridge.tmpl"
	p = setTestTargets(p, []Target{custom})
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "read template missing/custom-bridge.tmpl") {
		t.Fatalf("missing custom bridge template error = %v", err)
	}
}

// invariant: rendering/project-output-plan:output-policy-explicit (TestOutputPlanCoalescesAndRejectsSharedTargetOutputsBeforeRendering)
// invariant: rendering/project-output-plan:shared-output-coalesced (TestOutputPlanCoalescesAndRejectsSharedTargetOutputsBeforeRendering)
func TestOutputPlanCoalescesAndRejectsSharedTargetOutputsBeforeRendering(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	shared := piTarget
	shared.Name = "second-pi"
	shared.Outputs = append([]TargetOutput(nil), piTarget.Outputs...)
	p = setTestTargets(p, append(testTargets(p), shared))
	op, err := outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	var sharedHash string
	for _, n := range op.Nodes {
		if n.Path == ".pi/extensions/awf-subagents/index.ts" {
			if got := strings.Join(n.Declarers, ","); got != "pi,second-pi" {
				t.Fatalf("shared declarers = %q", got)
			}
			if n.file.ConfigHash == n.Recipe.ConfigHash || len(n.DeclarerProjections) != 2 {
				t.Fatal("shared declarer descriptors were not folded into drift hash")
			}
			sharedHash = n.file.ConfigHash
		}
	}
	targets := testTargets(p)
	targets[len(targets)-1].Name = "renamed-pi"
	p = setTestTargets(p, targets)
	op, err = outputPlanProject(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range op.Nodes {
		if n.Path == ".pi/extensions/awf-subagents/index.ts" && n.file.ConfigHash == sharedHash {
			t.Fatal("declarer descriptor identity did not change drift hash")
		}
	}
	targets = testTargets(p)
	targets[len(targets)-1].Outputs[0].Policy = OutputPolicy{Regenerate: true}
	p = setTestTargets(p, targets)
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "conflicting output recipes") {
		t.Fatalf("conflicting shared output error = %v", err)
	}
}

// invariant: rendering/project-output-plan:output-policy-explicit (TestOutputPolicyIsExplicit)
func TestOutputPolicyIsExplicit(t *testing.T) {
	if got := declaredPolicy("agents", false); !got.ValidateFrontmatter || !got.ScanReferences {
		t.Fatalf("agent policy = %#v", got)
	}
	if got := declaredPolicy("target-output", false); got.ScanReferences {
		t.Fatalf("target output policy = %#v", got)
	}
	if got := declaredPolicy("efforts", false); got.ScanReferences {
		t.Fatalf("efforts policy = %#v", got)
	}
	if (OutputPolicy{}).ScanReferences {
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
	p, err := Open(testContext(t), root)
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
	p, _ := Open(testContext(t), root)
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

// A local document's output is read twice on the way to a plan: the declaration
// pass observes whether the in-place body exists, then the render pass reads it
// back. Each read has its own propagation path, so each must surface its fault
// rather than be erased by the other's success.
func TestOutputPlanPropagatesLocalRenderReadFault(t *testing.T) {
	for _, tc := range []struct {
		name      string
		failAfter int
	}{
		{"declaration pass", 0},
		{"render pass", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := scaffold(t, "prefix: example\nintegrationBranch: main\nlocalDocs:\n  - name: runbooks/incident\n    title: Incident\n    description: Handle incidents.\n")
			p, err := Open(testContext(t), root)
			if err != nil {
				t.Fatal(err)
			}
			failure := errors.New("local output read failed")
			calls := 0
			read := faultingProjectReader{ProjectTreeReader: filesystemProjectReader{root: root},
				path: "docs/runbooks/incident.md", err: failure, failAfter: tc.failAfter, calls: &calls}
			cfg, err := config.Load(config.RootDir(root))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := outputPlan(newRenderInputs(p.OutputState(), cfg, read, project.Version)); !errors.Is(err, failure) {
				t.Fatalf("output plan error = %v, want %v", err, failure)
			}
		})
	}
}

func TestOutputPlanPropagatesConfigReferenceRenderFault(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\n", map[string]string{
		"parts/config-reference/intro.md": "<!-- awf:comment unclosed\n",
	})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := outputPlanProject(p); err == nil || !strings.Contains(err.Error(), "malformed awf:comment") {
		t.Fatalf("output plan error = %v", err)
	}
}

func TestTargetOutputDeclarationsRejectUnreadableTemplate(t *testing.T) {
	bad := piTarget
	bad.Outputs = append([]TargetOutput(nil), piTarget.Outputs...)
	bad.Outputs[0].TemplateID = "missing/target-output.tmpl"
	cfg := &config.Config{Prefix: "example"}
	p := testState()
	p = lowerWithTargets(p, []Target{bad})
	_, err := targetOutputDeclarations(newRenderInputs(p, cfg, nil, "test"), nil)
	t.Logf("target output declaration error = %v", err)
	if err == nil || !strings.Contains(err.Error(), "read template missing/target-output.tmpl") {
		t.Fatalf("unreadable target output template error = %v", err)
	}
}

func TestTargetOutputDeclarationsRejectUnknownRequiredSkill(t *testing.T) {
	bad := piTarget
	bad.Outputs = append([]TargetOutput(nil), piTarget.Outputs...)
	bad.Outputs[0].RequiresSkill = "missing"
	cfg := &config.Config{Prefix: "example"}
	p := testState()
	p = lowerWithTargets(p, []Target{bad})
	if _, err := targetOutputDeclarations(newRenderInputs(p, cfg, nil, "test"), nil); err == nil || !strings.Contains(err.Error(), "unknown catalog skill") {
		t.Fatalf("unknown target output requirement error = %v", err)
	}
}

func TestValidateLiveTemplatesRejectsMissingTargetTemplate(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	p = setTestTargets(p, append(testTargets(p), Target{Outputs: []TargetOutput{{TemplateID: "missing/live-template.tmpl"}}}))
	if err := validateLiveTemplates(renderInputsForTest(p)); err == nil || !strings.Contains(err.Error(), "read template missing/live-template.tmpl") {
		t.Fatalf("missing live template error = %v", err)
	}
}

func TestOutputPlanPropagatesTopicGenerationEnumerationFault(t *testing.T) {
	state := testStateAt(t.TempDir())
	calls := 0
	failure := errors.New("topic enumeration failed")
	reader := failingPathsReader{memoryProjectReader: memoryProjectReader{}, failAt: 3, calls: &calls}
	_, err := outputPlanWithPitfalls(newRenderInputs(state, &config.Config{}, reader, project.Version), mustCorpus(), pitfall.Corpus{}, topic.Corpus{}, map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), failure.Error()) && !strings.Contains(err.Error(), "enumeration fault") {
		t.Fatalf("output plan error = %v", err)
	}
}
