package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/render"
)

// invariant: output-plan-complete
// invariant: shared-output-coalesced
// invariant: target-capabilities-closed
func TestBridgeProjectionPreparedAndTerminal(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: [core]\ntargets: [claude]\nrunner:\n  enabled: true\n", map[string]string{"domains/core.yaml": "paths: ['internal/**']\n", "current-state-migration.yaml": "version: 1\ninvariantApprovals: []\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := p.BridgeProjection(false)
	if err != nil {
		t.Fatal(err)
	}
	terminal, err := p.BridgeProjection(true)
	if err != nil {
		t.Fatal(err)
	}
	prep, term := map[string]BridgeOutput{}, map[string]BridgeOutput{}
	for _, o := range prepared {
		prep[o.Path] = o
	}
	for _, o := range terminal {
		term[o.Path] = o
	}
	for _, path := range []string{"docs/decisions/ACTIVE.md", "docs/domains/core.md"} {
		if prep[path].Deletion || len(prep[path].Bytes) == 0 {
			t.Errorf("prepared %s: %#v", path, prep[path])
		}
		if !term[path].Deletion || len(term[path].Bytes) != 0 {
			t.Errorf("terminal %s: %#v", path, term[path])
		}
	}
	if !prep[".awf/current-state-migration.yaml"].Reservation || string(prep[".awf/current-state-migration.yaml"].Bytes) != "version: 1\ninvariantApprovals: []\n" || string(term[".awf/current-state-migration.yaml"].Bytes) != string(prep[".awf/current-state-migration.yaml"].Bytes) {
		t.Fatal("approval file not retained exactly")
	}
	for path, o := range prep {
		if path == "docs/decisions/ACTIVE.md" || path == "docs/domains/core.md" {
			continue
		}
		got, ok := term[path]
		if !ok || string(got.Bytes) != string(o.Bytes) || got.Mode != o.Mode || got.Reservation != o.Reservation {
			t.Errorf("projection drift at %s", path)
		}
	}
}

func TestBridgeProjectionPropagatesOutputPlanError(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: []\nagents: []\ndomains: [core]\ntargets: [claude]\n", map[string]string{"domains/core.yaml": "paths: ['internal/**']\n", "topics/parts/core/orphan/current-state.md": "Intro.\n\n## Claims\n"})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.BridgeProjection(false); err == nil {
		t.Fatal("invalid output plan accepted")
	}
}

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
	for _, path := range []string{".pi/extensions/awf-subagents/index.ts", "AGENTS.md", ".awf/memory/.gitignore", "docs/decisions/ACTIVE.md", "docs/domains/rendering.md", "docs/config-reference.md"} {
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

// invariant: output-policy-explicit
// invariant: shared-output-coalesced
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

// invariant: output-policy-explicit
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
