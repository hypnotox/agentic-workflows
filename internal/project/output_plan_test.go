package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// invariant: rendering/project-output-plan:output-plan-complete
func TestOutputPlanContainsWritesGeneratedNodesAndReservations(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [mine]\nagents: []\ndomains: [rendering]\ntargets: [pi]\n", map[string]string{"skills/mine.yaml": "local: true\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan()
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
	for _, path := range []string{".pi/extensions/awf-subagents/index.ts", "AGENTS.md", ".awf/memory/.gitignore", "docs/decisions/INDEX.md", "docs/domains/rendering.md", "docs/config-reference.md"} {
		if !seen[path] {
			t.Errorf("plan missing producer class path %q", path)
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
func TestTargetDescriptorValidation(t *testing.T) {
	for _, target := range []Target{
		{Name: "bad", BridgeFile: "X"},
		{Name: "bad", Capabilities: []Capability{"unknown"}},
		{Name: "bad", AgentDialect: "unknown"},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "../bad", TemplateID: "x"}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Encoder: "unknown", Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Encoder: MarkdownAgentDialect, Provenance: 99, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Encoder: MarkdownAgentDialect, Provenance: render.HTMLComment}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Encoder: PlainAgentDialect, Provenance: render.HTMLComment, PolicyDeclared: true}}},
		{Name: "bad", AgentDialect: MarkdownAgentDialect, Outputs: []TargetOutput{{Path: "x", TemplateID: "x", Encoder: PlainAgentDialect, Provenance: render.SlashComment, Policy: OutputPolicy{ScanReferences: true}, PolicyDeclared: true}}},
	} {
		if err := target.validate(); err == nil {
			t.Fatalf("invalid target %#v was accepted", target)
		}
		root := scaffold(t, "prefix: example\nskills: []\nagents: []\n")
		p, err := Open(root)
		if err != nil {
			t.Fatal(err)
		}
		p.Targets = []Target{target}
		if _, err := p.OutputPlan(); err == nil {
			t.Fatalf("planner accepted invalid target %#v", target)
		}
	}
	if got := piTarget.targetTemplateData()["targetSubagentTools"]; got != true {
		t.Fatalf("Pi capability projection = %#v", got)
	}
	if _, err := resolveTargets([]string{"nope"}); err == nil {
		t.Fatal("unknown target resolved")
	}
}

// invariant: rendering/project-output-plan:output-policy-explicit
// invariant: rendering/project-output-plan:shared-output-coalesced
func TestOutputPlanCoalescesAndRejectsSharedTargetOutputsBeforeRendering(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	shared := piTarget
	shared.Name = "second-pi"
	shared.Outputs = append([]TargetOutput(nil), piTarget.Outputs...)
	p.Targets = append(p.Targets, shared)
	op, err := p.OutputPlan()
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
	op, err = p.OutputPlan()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range op.Nodes {
		if n.Path == ".pi/extensions/awf-subagents/index.ts" && n.file.ConfigHash == sharedHash {
			t.Fatal("declarer descriptor identity did not change drift hash")
		}
	}
	p.Targets[1].Outputs[0].Policy = OutputPolicy{Regenerate: true}
	if _, err := p.OutputPlan(); err == nil || !strings.Contains(err.Error(), "conflicting output recipes") {
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
	if got := declaredPolicy("memory", false); got.ScanReferences {
		t.Fatalf("memory policy = %#v", got)
	}
	if (OutputPolicy{}).ScanReferences {
		t.Fatal("zero policy must not scan")
	}
}

func TestOutputPlanTopicNodesHaveLiteralPathsAndInputs(t *testing.T) {
	root := topicProject(t)
	writeProjectTopic(t, root, "contracts", "Contracts", "paths: [\"internal/**\"]\n")
	p, _ := Open(root)
	op, err := p.OutputPlan()
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
