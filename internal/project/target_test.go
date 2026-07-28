package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestClaudeTargetPaths unit-checks the claude adapter's path formulas. ADR-0016's
// target-output-paths invariant is retired by ADR-0037 (retires_invariants); the
// per-target rendering property is now backed by inv: multi-target-render.
func TestClaudeTargetPaths(t *testing.T) {
	if got := claudeTarget.SkillPath("awf", "tdd"); got != ".claude/skills/awf-tdd/SKILL.md" {
		t.Fatalf("SkillPath = %q", got)
	}
	if got := claudeTarget.AgentPath("code-reviewer"); got != ".claude/agents/code-reviewer.md" {
		t.Fatalf("AgentPath = %q", got)
	}
	if claudeTarget.BridgeFile != "CLAUDE.md" {
		t.Fatalf("BridgeFile = %q", claudeTarget.BridgeFile)
	}
}

// invariant: rendering/catalog-and-targets:claude-md-bridge
// invariant: rendering/catalog-and-targets:target-dialect-render
func TestCodexTargetRendersTOMLAgents(t *testing.T) {
	if got := codexTarget.AgentPath("code-reviewer"); got != ".codex/agents/code-reviewer.toml" {
		t.Fatalf("Codex AgentPath = %q", got)
	}
	root := scaffold(t, sampleYAML+"targets:\n  - codex\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var got *RenderedFile
	for i := range files {
		if files[i].Path == ".codex/agents/code-reviewer.toml" {
			got = &files[i]
		}
	}
	if got == nil {
		t.Fatal("Codex agent not rendered")
	}
	if err := validateArtifact([]byte(got.Content), TOMLAgentDialect); err != nil {
		t.Fatalf("validate Codex profile: %v\n%s", err, got.Content)
	}
	if !strings.HasPrefix(got.Content, "# "+bannerText+"\n") {
		t.Fatalf("Codex profile missing TOML banner:\n%s", got.Content)
	}
	if !strings.Contains(got.Content, "developer_instructions") {
		t.Fatalf("Codex profile missing instructions:\n%s", got.Content)
	}
	for _, f := range files {
		if f.TemplateID == "skills/tdd/SKILL.md.tmpl" {
			if f.Path != ".agents/skills/example-tdd/SKILL.md" {
				t.Fatalf("Codex skill path = %q", f.Path)
			}
			if !strings.Contains(f.Content, "<!-- "+bannerText+" -->") {
				t.Fatalf("Codex markdown skill lost HTML provenance:\n%s", f.Content)
			}
		}
	}
}

// invariant: rendering/pi-runtime:pi-extension-target-render

/* func TestTelemetryProtocolDescriptorAttribution(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	op, err := p.OutputPlan()
	if err != nil {
		t.Fatal(err)
	}
	const output = ".pi/extensions/awf-telemetry/protocol.ts"
	want := OutputInput{Path: "internal/telemetry/protocol.json", Role: ArtifactProtocolDescriptor}
	found := false
	for _, node := range op.Nodes {
		if node.Path == output {
			found = slices.Contains(node.ConsumedInputs, want)
		}
	}
	if !found {
		t.Fatal("protocol output does not consume the descriptor")
	}
	corpus, err := p.Corpus()
	if err != nil {
		t.Fatal(err)
	}
	decls, err := BuildOutputDeclarations(p.Cfg, p.Cat, p.Targets, filesystemProjectReader{root: root}, corpus)
	if err != nil {
		t.Fatal(err)
	}
	records := artifactRecords(output, decls, artifactAuthorities{Layout: p.layout(), ADRs: corpus})
	var labeled bool
	for _, record := range records {
		for _, source := range record.Sources {
			if source.Path == want.Path && source.Label == "protocol descriptor" {
				labeled = true
			}
		}
	}
	if !labeled {
		t.Fatalf("protocol descriptor attribution missing: %#v", records)
	}
} */

func TestTargetOutputRenderError(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	original := p.Targets[0].Outputs[0].TemplateID
	defer func() { p.Targets[0].Outputs[0].TemplateID = original }()
	p.Targets[0].Outputs[0].TemplateID = "missing-target-output.tmpl"
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), "missing-target-output") {
		t.Fatalf("RenderAll error = %v, want missing target-output template", err)
	}
}

// invariant: rendering/pi-workflows:pi-structured-exploration-contract

// invariant: rendering/pi-workflows:pi-subagent-model-routing

// invariant: rendering/pi-workflows:pi-subagent-model-preferences

// invariant: rendering/pi-workflows:pi-subagent-model-wizard

func explorationFixtureConfig(target string) string {
	return "prefix: example\nskills: [adr-lifecycle, brainstorming, debugging, executing-direct, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [" + target + "]\n"
}

func explorationRenderedByPath(t *testing.T, config string) map[string]string {
	t.Helper()
	p, err := Open(scaffold(t, config))
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, file := range files {
		got[file.Path] = file.Content
	}
	return got
}

// invariant: rendering/workflow-skill-templates:cross-runtime-exploration-dispatch
func TestCrossRuntimeExplorationDispatch(t *testing.T) {
	if !catalog.Standard.Skills["exploring"].Core {
		t.Fatal("exploring is not a core skill")
	}
	dirs := map[string]string{
		"claude": ".claude/skills", "codex": ".agents/skills", "copilot": ".github/skills",
		"cursor": ".cursor/skills", "gemini": ".gemini/skills", "pi": ".pi/skills",
	}
	for _, target := range KnownTargets() {
		t.Run(target, func(t *testing.T) {
			files := explorationRenderedByPath(t, explorationFixtureConfig(target))
			base := dirs[target] + "/example-"
			skillBody := func(name string) string {
				if target == "pi" {
					return files[".pi/skills/example-"+name+"/SKILL.md"]
				}
				return files[base+name+"/SKILL.md"]
			}
			exploring := skillBody("exploring")
			if exploring == "" {
				t.Fatalf("missing rendered exploring skill for %s", target)
			}
			shared := []string{
				"targeted < bounded < broad", "targeted` locates one declaration", "bounded` investigates within a named symbol", "broad` searches across the project search universe",
				"paths < summary < analysis", "file:line", "file:start-end", "minimal labels needed to distinguish", "concise explanations", "evidence-grounded synthesis",
				"adaptive maximum", "cheapest targeted lookup", "widen only when evidence requires", "never widen beyond the selected maximum", "boundary is exhausted, report that explicitly",
				"tracked files plus non-ignored untracked working-tree files", "tracked generated and vendor files", "ignored files", ".git", "nested repositories", "external dependencies unless explicitly scoped",
				"not-found", "inconclusive", "unverified", "Not found within <breadth> boundary:", "successful execution", "one concise next refinement", "project search universe and searched surfaces", "Ground every material claim with file/line evidence",
				"new fresh-context call", "correct the task", "change report detail", "widen breadth", "one information need", "relevant final findings",
			}
			for _, want := range shared {
				if !strings.Contains(exploring, want) {
					t.Errorf("%s exploring skill missing shared contract %q", target, want)
				}
			}
			if target == "pi" {
				for _, want := range []string{"subagent_explore", "required task, breadth, and detail", "at most ten exploration children", "queues the rest FIFO", "provider/model-id", "omission inherits the parent"} {
					if !strings.Contains(exploring, want) {
						t.Errorf("Pi exploring skill missing %q", want)
					}
				}
			} else {
				for _, want := range []string{"target-native fresh-context exploration subagent", "task", "breadth", "detail"} {
					if !strings.Contains(exploring, want) {
						t.Errorf("%s exploring skill missing %q", target, want)
					}
				}
				for _, absent := range []string{"subagent_explore", "provider/model-id", "ten exploration children", "queues the rest FIFO"} {
					if strings.Contains(exploring, absent) {
						t.Errorf("%s exploring skill leaks Pi guidance %q", target, absent)
					}
				}
			}
			modelGuidanceSkills := []string{"brainstorming", "exploring", "reviewing-adr", "reviewing-impl", "reviewing-plan-resync", "reviewing-plan", "subagent-driven-development"}
			for _, skill := range modelGuidanceSkills {
				body := skillBody(skill)
				if target == "pi" {
					if !strings.Contains(body, "provider/model-id") || !strings.Contains(body, "inherits the parent") {
						t.Errorf("Pi/%s missing optional model guidance", skill)
					}
				} else if strings.Contains(body, "provider/model-id") || strings.Contains(body, "subagent_") {
					t.Errorf("%s/%s leaks Pi model or tool syntax", target, skill)
				}
			}
			for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
				body := skillBody(consumer)
				for _, want := range []string{"location is unknown", "and inline search would pollute the parent context", "exact-known-file", "genuinely trivial"} {
					if !strings.Contains(body, want) {
						t.Errorf("%s/%s missing dispatch condition %q", target, consumer, want)
					}
				}
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:bounded-exploration-reporting
func TestBoundedExplorationReporting(t *testing.T) {
	files := explorationRenderedByPath(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n")
	guidance := files[".pi/skills/example-exploring/SKILL.md"]
	prompt := renderPiExtensionFile(t, "awf-subagents/index.ts")
	contracts := map[string]struct {
		body  string
		wants []string
	}{
		"rendered exploring guidance": {guidance, []string{
			"Independent information needs may be sibling-dispatched", "at most ten exploration children", "queues the rest FIFO", "Refinement stays sequential",
			"targeted < bounded < broad", "`targeted` locates one declaration, implementation, file, or exact fact", "`bounded` investigates within a named symbol, package, component, or subsystem", "`broad` searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts",
			"adaptive maximum", "cheapest targeted lookup", "widen only when evidence requires it", "never widen beyond the selected maximum", "If the boundary is exhausted, report that explicitly",
			"tracked files plus non-ignored untracked working-tree files under the repository root", "tracked generated and vendor files", "ignored files", ".git", "nested repositories", "external dependencies unless explicitly scoped",
			"paths < summary < analysis", "`paths` returns only relevant `file:line` or `file:start-end` locations with minimal labels needed to distinguish them", "`summary` returns grounded locations plus concise explanations of what each contains and why it matters", "`analysis` directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty",
			"Ground every material claim with file/line evidence", "Not found within <breadth> boundary: <what was searched>", "successful execution", "one concise next refinement", "broad absence report must name the project search universe and searched surfaces", "Distinguish inconclusive and unverified outcomes from absence",
			"new fresh-context call to correct the task, change report detail, or widen breadth",
		}},
		"Pi fixed prompt": {prompt, []string{
			"independent information needs concurrently", "refinement of an earlier result stays sequential", "at most ten active exploration children", "queues the rest FIFO with abort-aware removal",
			"Breadth is ordered targeted < bounded < broad", "targeted locates one declaration, implementation, file, or exact fact", "bounded investigates within a named symbol, package, component, or subsystem", "broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts",
			"adaptive maximum: start with the cheapest targeted lookup, widen only when evidence requires it, and never widen beyond the selected maximum", "If the boundary is exhausted, report that explicitly",
			"tracked files plus non-ignored untracked working-tree files under the current repository root", "tracked generated and vendored files", "ignored files", ".git", "nested repositories", "external dependencies unless the task explicitly brings one of those surfaces into scope",
			"paths < summary < analysis", "paths returns only relevant file:line or file:start-end locations with minimal labels and no search narrative", "summary returns grounded locations plus concise explanations of what each contains and why it matters", "analysis directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty",
			"Ground every material claim with file:line evidence", "Not-found is successful execution and begins exactly: Not found within <breadth> boundary: <what was searched>", "broad absence report must name the project search universe and searched surfaces", "not-found result may suggest one concise next refinement", "inconclusive or unverified result is not an absence claim",
			"new fresh-context call that corrects the task, changes report detail, or widens breadth",
		}},
	}
	for label, contract := range contracts {
		for _, want := range contract.wants {
			if !strings.Contains(contract.body, want) {
				t.Errorf("%s missing bounded-reporting clause %q", label, want)
			}
		}
	}
	fallback := renderSkillGolden(t, "exploring", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{},
	})
	if strings.Contains(fallback, "subagent_explore") || !strings.Contains(fallback, "target-native fresh-context exploration subagent") {
		t.Errorf("empty-capability exploring render has incoherent dispatch:\n%s", fallback)
	}
}

// invariant: rendering/pi-workflows:pi-subagent-progress-context-isolation

// invariant: rendering/pi-workflows:pi-subagent-progress-rendering

// invariant: rendering/pi-workflows:pi-subagent-failure-details

// invariant: rendering/pi-workflows:pi-subagent-progress-bounds

// invariant: rendering/pi-runtime:pi-child-tool-boundaries

// invariant: rendering/pi-workflows:pi-implementation-batch-exclusivity

// invariant: rendering/pi-runtime:pi-child-process-safety

// invariant: rendering/pi-runtime:pi-implementation-state-boundary

// invariant: rendering/pi-runtime:pi-minimum-runtime

// invariant: rendering/pi-workflows:pi-session-handoff-public-contract

// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle

// invariant: rendering/pi-workflows:pi-session-handoff-workflow

// TestNeutralSingletonSessionHandoffSignal pins the ADR-0157 Decision 6
// contract: the neutral (once-rendered) guide and workflow doc receive a
// project-level targetSessionHandoff signal, true iff any enabled target
// supports session handoff, so their Pi-gated prose renders for a
// handoff-capable target set and stays absent otherwise. These both-branch
// assertions belong to guide-entry-point-routing's proof set.
// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing

func renderPiExtensionFile(t *testing.T, name string) string {
	t.Helper()
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	path := ".pi/extensions/" + name
	for _, file := range files {
		if file.Path == path {
			return file.Content
		}
	}
	t.Fatalf("missing %s", path)
	return ""
}

// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch

func TestAllTargetPathsAndBridges(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ntargets:\n  - claude\n  - codex\n  - copilot\n  - cursor\n  - gemini\n  - pi\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	for _, want := range []string{"CLAUDE.md", "GEMINI.md"} {
		if !paths[want] {
			t.Errorf("missing bridge %q", want)
		}
	}
	for _, absent := range []string{"CODEX.md", "COPILOT.md", "CURSOR.md", "PI.md"} {
		if paths[absent] {
			t.Errorf("unexpected bridge %q", absent)
		}
	}
	if got := KnownTargets(); strings.Join(got, ",") != "claude,codex,copilot,cursor,gemini,pi" {
		t.Fatalf("KnownTargets = %v", got)
	}
}

func TestClaudeMdBridgeRendered(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var got *RenderedFile
	for i := range files {
		if files[i].Path == "CLAUDE.md" {
			got = &files[i]
		}
	}
	if got == nil {
		t.Fatal("CLAUDE.md not rendered")
	}
	if !strings.Contains(got.Content, "@AGENTS.md") {
		t.Fatalf("CLAUDE.md missing @AGENTS.md import:\n%s", got.Content)
	}
	if !strings.HasPrefix(got.Content, "<!-- ") {
		t.Fatalf("CLAUDE.md missing provenance banner:\n%s", got.Content)
	}
}

// TestMultiTargetRender backs inv: multi-target-render and inv: cursor-no-bridge
// (both declared in render.go): adapter artifacts render once per enabled target
// with byte-identical bodies, neutral artifacts render once, and cursor emits no
// bridge.
func TestMultiTargetRender(t *testing.T) {
	root := scaffold(t, sampleYAML+"targets:\n  - claude\n  - cursor\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	agentsMd, bridges := 0, 0
	for _, f := range files {
		byPath[f.Path] = f.Content
		if f.Path == "AGENTS.md" {
			agentsMd++
		}
		if f.TemplateID == "claude/CLAUDE.md.tmpl" {
			bridges++
		}
	}
	// invariant: rendering/project-output-plan:multi-target-render
	for _, pair := range [][2]string{
		{".claude/skills/example-tdd/SKILL.md", ".cursor/skills/example-tdd/SKILL.md"},
		{".claude/agents/code-reviewer.md", ".cursor/agents/code-reviewer.md"},
	} {
		a, b := byPath[pair[0]], byPath[pair[1]]
		if a == "" || b == "" {
			t.Fatalf("missing render: %q=%dB, %q=%dB", pair[0], len(a), pair[1], len(b))
		}
		if a != b {
			t.Errorf("content differs between %q and %q", pair[0], pair[1])
		}
	}
	if agentsMd != 1 {
		t.Errorf("AGENTS.md rendered %d times, want 1 (neutral)", agentsMd)
	}
	// invariant: rendering/project-output-plan:cursor-no-bridge
	if bridges != 1 {
		t.Errorf("bridge files = %d, want 1 (claude only; cursor has none)", bridges)
	}
	if _, ok := byPath["CLAUDE.md"]; !ok {
		t.Error("CLAUDE.md (claude bridge) not rendered")
	}
}

// invariant: config/configuration:targets-default-claude
func TestResolveTargetsRejectsUnknown(t *testing.T) {
	root := scaffold(t, "prefix: awf\nskills: []\nagents: []\ntargets:\n  - nope\n")
	if _, err := Open(root); err == nil {
		t.Fatal("expected Open to reject an unknown target name")
	}
}

func TestPlannedOutputsIncludesGeneratedDocs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nskills: []\nagents: []\ndocs: []\ndomains: [rendering]\n", nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := p.PlannedOutputs()
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, rel := range planned {
		set[rel] = true
	}
	for _, want := range []string{"CLAUDE.md", "AGENTS.md", "docs/decisions/INDEX.md", "docs/domains/rendering.md"} {
		if !set[want] {
			t.Errorf("PlannedOutputs missing %q; got %v", want, planned)
		}
	}
}

func TestPlannedOutputsSurfacesRenderError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a sidecar so the RenderAll inside PlannedOutputs fails.
	corruptSidecar(t, root, "skills/tdd.yaml")
	if _, err := p.PlannedOutputs(); err == nil {
		t.Fatal("expected PlannedOutputs to surface the RenderAll error")
	}
}
