package project

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestOutputPlanPropagatesPreAdoptionEnumerationFault pins that a tree the
// planner cannot fully read fails the plan rather than producing one built from
// a truncated enumeration. A pre-adoption tree (no Git worktree) enumerates the
// filesystem directly, so an unreadable directory there used to be skipped and
// the plan, and the drift oracle computed from it, silently narrowed.
func TestOutputPlanPropagatesPreAdoptionEnumerationFault(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ndomains: [rendering]\n")
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
	if _, err := p.OutputPlan(testContext(t)); err == nil {
		t.Fatal("output plan built from a truncated enumeration")
	}
}

// invariant: rendering/project-output-plan:output-plan-complete
// invariant: rendering/pi-workflows:pi-native-workflow-skills
func TestOutputPlanContainsWritesGeneratedNodesAndReservations(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [mine]\nagents: []\ndomains: [rendering]\ntargets: [pi]\n", map[string]string{"skills/mine.yaml": "local: true\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var reservation, cref bool
	for _, n := range op.Nodes {
		seen[n.Path] = true
		if n.Reservation && n.Path == ".pi/skills/example-mine/SKILL.md" {
			reservation = true
		}
		if n.Path == "docs/config-reference.md" {
			cref = true
			if len(n.DependsOn) == 0 {
				t.Error("config reference has no dependencies")
			}
			for _, dep := range n.DependsOn {
				if dep == n.Path {
					t.Error("config reference has self dependency")
				}
			}
		}
	}
	if !reservation || !cref {
		t.Fatalf("plan missing reservation=%v config-reference=%v: %#v", reservation, cref, op.Nodes)
	}
	// Catalog/local, target-owned, neutral singleton, generated index/domain,
	// and generated reference producers all appear in the one plan.
	for _, path := range []string{".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/model-routing.ts", "AGENTS.md", ".awf/efforts/.gitignore", ".awf/worktrees/.gitignore", "docs/decisions/INDEX.md", "docs/domains/rendering.md", "docs/config-reference.md"} {
		if !seen[path] {
			t.Errorf("plan missing producer class path %q", path)
		}
	}
	for _, expected := range []struct {
		path     string
		template string
	}{
		{path: ".pi/extensions/awf-handoff/index.ts", template: "templates/pi/awf-handoff/index.ts.tmpl"},
		{path: ".pi/extensions/awf-subagents/model-routing.ts", template: "templates/pi/awf-subagents/model-routing.ts.tmpl"},
	} {
		var found bool
		for _, n := range op.Nodes {
			if n.Path != expected.path {
				continue
			}
			found = true
			templateInput := slices.Contains(n.ConsumedInputs, OutputInput{Path: expected.template, Role: ArtifactTemplate})
			if n.Reservation || strings.Join(n.Declarers, ",") != "pi" || !templateInput || n.file.ConfigHash == "" {
				t.Errorf("target output-plan node %s = %#v", expected.path, n)
			}
		}
		if !found {
			t.Errorf("missing target output-plan node %s", expected.path)
		}
	}
	files := op.writeFiles()
	for _, f := range files {
		if f.Path == ".pi/skills/example-mine/SKILL.md" {
			t.Fatal("reservation was rendered")
		}
	}
}

// invariant: rendering/project-output-plan:target-capabilities-closed
// invariant: rendering/pi-workflows:pi-subagent-progress-rendering
// invariant: rendering/project-output-plan:cursor-no-bridge
// invariant: rendering/workflow-skill-templates:mandatory-approval-boundaries
// invariant: rendering/pi-workflows:pi-subagent-progress-bounds
// invariant: config/migrations-and-locks:close-enabled-set-migration
// invariant: rendering/workflow-skill-templates:plan-task-detail-modes
func TestTargetDescriptorValidation(t *testing.T) {
	for _, target := range []Target{
		{Name: "bad", BridgeFile: "X"},
		{Name: "bad", Capabilities: []Capability{"unknown"}},
		{Name: "bad", AgentDialect: "unknown"},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "../bad", TemplateID: "x"}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: "unknown"}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Inputs: []TargetOutputInput{{Path: "unexpected", Role: ArtifactTemplate}}}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: "unknown", Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: 99, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Producer: TargetOutputTemplate, Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{ScanReferences: true}, PolicyDeclared: true}}},
	} {
		if err := target.validate(); err == nil {
			t.Fatalf("invalid target %#v was accepted", target)
		}
		root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
		p, err := Open(testContext(t), root)
		if err != nil {
			t.Fatal(err)
		}
		p.Targets = []Target{target}
		if _, err := p.OutputPlan(testContext(t)); err == nil {
			t.Fatalf("planner accepted invalid target %#v", target)
		}
	}
	if got := piTarget.targetTemplateData()["targetSubagentTools"]; got != true {
		t.Fatalf("Pi subagent capability projection = %#v", got)
	}
	if got := piTarget.targetTemplateData()["targetSessionHandoff"]; got != true {
		t.Fatalf("Pi handoff capability projection = %#v", got)
	}
	if _, err := resolveTargets([]string{"nope"}); err == nil {
		t.Fatal("unknown target resolved")
	}
}

// invariant: rendering/project-output-plan:output-policy-explicit
// invariant: rendering/project-output-plan:shared-output-coalesced
func TestOutputPlanCoalescesAndRejectsSharedTargetOutputsBeforeRendering(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	shared := piTarget
	shared.Name = "second-pi"
	shared.Outputs = append([]TargetOutput(nil), piTarget.Outputs...)
	p.Targets = append(p.Targets, shared)
	op, err := p.OutputPlan(testContext(t))
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
	p.Targets[1].Name = "renamed-pi"
	op, err = p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range op.Nodes {
		if n.Path == ".pi/extensions/awf-subagents/index.ts" && n.file.ConfigHash == sharedHash {
			t.Fatal("declarer descriptor identity did not change drift hash")
		}
	}
	p.Targets[1].Outputs[0].Policy = OutputPolicy{Regenerate: true}
	if _, err := p.OutputPlan(testContext(t)); err == nil || !strings.Contains(err.Error(), "conflicting output recipes") {
		t.Fatalf("conflicting shared output error = %v", err)
	}
}

// invariant: rendering/project-output-plan:output-policy-explicit
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

// invariant: rendering/adapter-outputs:generated-adapter-runtime-ownership
// invariant: rendering/pi-runtime:pi-child-tool-boundaries
// invariant: rendering/project-output-plan:multi-target-render
// invariant: rendering/pi-workflows:pi-subagent-failure-details
// invariant: rendering/workflow-skill-templates:bounded-exploration-reporting
// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch
// invariant: rendering/workflow-skill-templates:cross-runtime-exploration-dispatch
// invariant: rendering/pi-workflows:pi-subagent-model-wizard
// invariant: tooling/init-and-enablement:add-skill-pairs-agent
// invariant: rendering/workflow-skill-templates:memory-checkpoint-chain-coverage
// invariant: rendering/pi-runtime:pi-minimum-runtime
// invariant: rendering/pi-workflows:pi-structured-exploration-contract
func TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion(t *testing.T) {
	p, err := Open(testContext(t), filepath.Clean(filepath.Join("..", "..")))
	if err != nil {
		t.Fatal(err)
	}
	const extension = ".pi/extensions/awf-subagents/index.ts"
	result, err := p.ContextForOptions(testContext(t), []string{extension}, ContextOptions{Selection: SelectionExplicit})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Requests) != 1 || result.Requests[0].Exact == nil || result.Requests[0].Exact.Context.Classification != PathGeneratedOutput {
		t.Fatalf("extension classification = %#v", result.Requests)
	}
	path := result.Requests[0].Exact.Context
	if !slices.ContainsFunc(path.Domains, func(domain DomainRef) bool { return domain.Name == "rendering" }) || !slices.ContainsFunc(path.Topics, func(topic ContextPathTopic) bool { return topic.ID == "rendering/adapter-outputs" }) {
		t.Fatalf("extension ownership = domains %#v topics %#v", path.Domains, path.Topics)
	}
	expanded, err := p.ContextForOptions(testContext(t), []string{".pi/extensions"}, ContextOptions{Selection: SelectionExplicit})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Requests) != 1 || expanded.Requests[0].Kind != RequestDirectoryEmpty || expanded.Requests[0].Directory == nil || expanded.Requests[0].Directory.Included != 0 {
		t.Fatalf("generated extension entered whole-tree expansion: %#v", expanded)
	}
}

// invariant: rendering/pi-runtime:pi-child-process-safety
// invariant: rendering/catalog-and-targets:claude-md-bridge
// invariant: rendering/sync-and-drift:uninstall-removes-lock-entries
// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle
// invariant: rendering/pi-workflows:pi-session-handoff-workflow
// invariant: rendering/pi-workflows:pi-subagent-progress-context-isolation
// invariant: rendering/pi-workflows:pi-subagent-model-routing
// invariant: rendering/pi-workflows:pi-subagent-model-preferences
// invariant: rendering/pi-workflows:pi-session-handoff-public-contract
// invariant: rendering/catalog-and-targets:target-dialect-render
// invariant: rendering/pi-runtime:pi-implementation-state-boundary
// invariant: rendering/pi-runtime:pi-extension-target-render
// invariant: rendering/pi-runtime:pi-minimum-runtime
// invariant: rendering/pi-workflows:pi-implementation-batch-exclusivity
func TestCurrentStateOutputPlanMatchesTree(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan(testContext(t))
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
		if n.Reservation || n.file == nil {
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
	if seenTopics == 0 || seenDomains != len(p.Cfg.Domains) {
		t.Fatalf("current-state output coverage: topics=%d domains=%d want-domains=%d", seenTopics, seenDomains, len(p.Cfg.Domains))
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
	op, err := p.OutputPlan(testContext(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range op.Nodes {
		if n.Path == "docs/topics/rendering/contracts.md" {
			if len(n.DependsOn) != 2 || n.Reservation {
				t.Fatalf("topic node = %#v", n)
			}
			return
		}
	}
	t.Fatal("literal topic output was absent from the plan")
}
