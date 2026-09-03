package publisher

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/artifactregistry"
	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// TestClaudeTargetPaths unit-checks the claude adapter's path formulas. ADR-0016's
// target-output-paths invariant is retired by ADR-0037 (retires_invariants); the
// per-target rendering property is now backed by inv: multi-target-render.
func TestClaudeTargetPaths(t *testing.T) {
	if got := claudeTarget.SkillPath("awf-effort"); got != ".claude/skills/awf-effort/SKILL.md" {
		t.Fatalf("SkillPath = %q", got)
	}
	if claudeTarget.BridgeFile != "CLAUDE.md" {
		t.Fatalf("BridgeFile = %q", claudeTarget.BridgeFile)
	}
}

// invariant: rendering/pi-runtime:pi-session-handoff-workflow (TestPiTargetRetainsOnlySessionHandoffCapability)
// invariant: rendering/pi-workflows:pi-native-awf-skills (TestPiTargetRetainsOnlySessionHandoffCapability)
// invariant: rendering/adapter-outputs:no-awf-adapter-outputs (TestPiTargetRetainsOnlySessionHandoffCapability)
func TestPiTargetRetainsOnlySessionHandoffCapability(t *testing.T) {
	if !slices.Equal(piTarget.Capabilities, []artifactregistry.Capability{artifactregistry.CapabilitySessionHandoff}) {
		t.Fatalf("Pi capabilities = %v", piTarget.Capabilities)
	}
	if len(piTarget.Outputs) != 0 {
		t.Fatalf("Pi target retained adapter outputs: %#v", piTarget.Outputs)
	}
	data := targetTemplateData(piTarget)
	if data["targetSessionHandoff"] != true || len(data) != 1 {
		t.Fatalf("Pi template data = %#v", data)
	}
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, file := range files {
		paths[file.Path] = true
	}
	if !paths[".pi/skills/awf-effort/SKILL.md"] {
		t.Fatal("awf-effort skill was not rendered for Pi")
	}
	for path := range paths {
		if strings.HasPrefix(path, ".pi/extensions/awf-subagents/") || strings.HasPrefix(path, ".pi/agents/") {
			t.Errorf("retired Pi adapter output rendered: %s", path)
		}
	}
}

// invariant: rendering/catalog-and-targets:built-in-runtime-targets (TestTargetDescriptorCustomization)
// invariant: rendering/project-output-plan:multi-target-render (TestTargetDescriptorCustomization)
func TestTargetDescriptorCustomization(t *testing.T) {
	custom := artifactregistry.Target{
		Name:           "custom",
		SkillDir:       ".custom/workflows",
		AgentDialect:   artifactregistry.MarkdownAgentDialect,
		BridgeFile:     "CUSTOM.md",
		BridgeTemplate: bridgeTID,
		Capabilities:   []artifactregistry.Capability{artifactregistry.CapabilitySessionHandoff},
	}
	if err := artifactregistry.ValidateTarget(custom); err != nil {
		t.Fatal(err)
	}
	if custom.SkillPath("awf-effort") != ".custom/workflows/awf-effort/SKILL.md" {
		t.Fatal("custom descriptor skill path was not preserved")
	}
	if data := targetTemplateData(custom); data["targetSessionHandoff"] != true || len(data) != 1 {
		t.Fatalf("custom descriptor capabilities = %#v", data)
	}
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	p = setTestTargets(p, []artifactregistry.Target{custom})
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]artifactregistry.AgentDialect{
		".custom/workflows/awf-effort/SKILL.md": artifactregistry.MarkdownAgentDialect,
		"CUSTOM.md":                             artifactregistry.MarkdownAgentDialect,
	}
	counts := map[string]int{}
	for _, file := range files {
		if encoder, ok := want[file.Path]; ok {
			counts[file.Path]++
			if file.Encoder != encoder {
				t.Errorf("%s encoder = %q, want %q", file.Path, file.Encoder, encoder)
			}
		}
	}
	for path := range want {
		if counts[path] != 1 {
			t.Errorf("%s rendered %d times, want 1", path, counts[path])
		}
	}
}

func TestAllTargetPathsAndBridges(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{}
	for _, f := range files {
		paths[f.Path] = true
	}
	if !paths["CLAUDE.md"] {
		t.Error("missing Claude bridge")
	}
	if paths["PI.md"] {
		t.Error("unexpected Pi bridge")
	}
}

// invariant: rendering/catalog-and-targets:claude-md-bridge (TestClaudeMdBridgeRendered)
func TestClaudeMdBridgeRendered(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n")
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
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

// TestMultiTargetRender proves adapter artifacts render once per enabled target
// at descriptor-owned paths while neutral artifacts render once.
// invariant: rendering/catalog-and-targets:target-dialect-render (TestMultiTargetRender)
// invariant: rendering/workflow-skill-templates:fixed-awf-skill-surface (TestMultiTargetRender)
func TestMultiTargetRender(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := renderAll(p)
	if err != nil {
		t.Fatal(err)
	}
	seenTargets := map[string]bool{}
	for _, target := range p.Targets() {
		seenTargets[target.Name] = true
	}
	if !slices.Equal(slices.Sorted(maps.Keys(seenTargets)), []string{"claude", "pi"}) {
		t.Fatalf("render targets = %v, want fixed Claude and Pi targets", seenTargets)
	}
	byPath := map[string]string{}
	pathCounts := map[string]int{}
	agentsMd, bridges := 0, 0
	for _, f := range files {
		byPath[f.Path] = f.Content
		pathCounts[f.Path]++
		if f.Path == "AGENTS.md" {
			agentsMd++
		}
		if f.TemplateID == "claude/CLAUDE.md.tmpl" {
			bridges++
		}
	}
	// invariant: rendering/project-output-plan:multi-target-render (TestMultiTargetRender)
	for _, target := range p.Targets() {
		for name := range catalog.Standard.Skills {
			path := target.SkillPath(name)
			content := byPath[path]
			if content == "" || pathCounts[path] != 1 {
				t.Errorf("skill %q for %s rendered %d times with %d bytes", name, target.Name, pathCounts[path], len(content))
			}
		}
	}
	if agentsMd != 1 {
		t.Errorf("AGENTS.md rendered %d times, want 1 (neutral)", agentsMd)
	}
	if bridges != 1 {
		t.Errorf("bridge files = %d, want 1 (claude only)", bridges)
	}
	if _, ok := byPath["CLAUDE.md"]; !ok {
		t.Error("CLAUDE.md (claude bridge) not rendered")
	}
}

const (
	semanticPlanningInstruction    = "- **Semantic rendering review:** plans name concrete examples and expected readings only when load-bearing. The implementation phase owner performs focused generated-prose meaning review, records inspected output boundaries and result in completion evidence, and checks contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders without a universal language validator. Plan review inspects the requirement and any evidence already available; code review inspects the completed implementation evidence."
	semanticPlanReviewInstruction  = "1. **semantic-rendering-review**: inspect the change-specific requirement for focused generated-prose meaning review at affected output boundaries, including contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders such as `<literal-placeholder>`. During precommit plan review, do not require future implementation completion evidence; during a later review, inspect that evidence when it exists. Concrete examples and expected readings are required only when load-bearing; this is not a general output validator."
	semanticCodeReviewInstruction  = "1. **semantic-rendering-review**: for generated prose changes, inspect the requirement and phase completion evidence naming produced-output boundaries and result, including contradictory fragments, concept-preserving paraphrase, and literal-placeholder intent. Keep this as human meaning review, not a general output validator or new deterministic inference."
	semanticImplementerInstruction = "For generated-prose changes, perform the focused meaning review required by the phase at the produced-output boundaries. Check contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders without inventing a universal language validator; retain the inspected boundaries and result as completion evidence for your report."
	semanticInlineInstruction      = "For generated-prose changes, perform the focused meaning review at the produced-output boundaries and retain the inspected boundaries and result as completion evidence."
)

func TestResolveTargetsRejectsUnknown(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n  - nope\n")
	if _, err := loadTestSession(testContext(t), root); err == nil {
		t.Fatal("expected Open to reject an unknown target name")
	}
}

func TestPlannedOutputsIncludesGeneratedDocs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: [rendering]\n", nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := plannedOutputsProject(p)
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, rel := range planned {
		set[rel] = true
	}
	for _, want := range []string{"CLAUDE.md", "AGENTS.md", "docs/domains/rendering.md"} {
		if !set[want] {
			t.Errorf("PlannedOutputs missing %q; got %v", want, planned)
		}
	}
	for _, historical := range []string{"docs/decisions/0001-engine.md", "docs/decisions/INDEX.md", "docs/decisions/README.md", "docs/decisions/template.md"} {
		if set[historical] {
			t.Errorf("historical decision path %q remains publisher-managed", historical)
		}
	}
}

func TestPlannedOutputsSurfacesRenderError(t *testing.T) {
	root := scaffold(t, sampleYAML)
	p, err := loadTestSession(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a sidecar so the RenderAll inside PlannedOutputs fails.
	corruptSidecar(t, root, "skills/awf-effort.yaml")
	if _, err := plannedOutputsProject(p); err == nil {
		t.Fatal("expected PlannedOutputs to surface the RenderAll error")
	}
}
