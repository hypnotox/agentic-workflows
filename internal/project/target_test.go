package project

import (
	"fmt"
	"strings"
	"testing"

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

// invariant: claude-md-bridge
// invariant: target-dialect-render
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

// invariant: pi-extension-target-render
func TestPiTargetRendersExtension(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
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
	for _, path := range []string{".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/runner.ts"} {
		if !strings.HasPrefix(got[path], "// "+bannerText+"\n") {
			t.Errorf("Pi extension %s missing TypeScript banner", path)
		}
	}
	for _, target := range []string{"claude", "codex", "copilot", "cursor", "gemini"} {
		other := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: ["+target+"]\n")
		op, err := Open(other)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := op.RenderAll()
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range rendered {
			if strings.HasPrefix(file.Path, ".pi/extensions/") {
				t.Errorf("target %s unexpectedly rendered %s", target, file.Path)
			}
		}
	}
}

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

// invariant: pi-subagent-four-tool-contract
func TestPiSubagentFourToolContract(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, name := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"} {
		if strings.Count(content, `name: "`+name+`"`) != 1 {
			t.Errorf("tool %s registration count differs from one", name)
		}
	}
	if got := strings.Count(content, `name: "subagent_`); got != 4 {
		t.Errorf("public subagent registration count = %d, want 4", got)
	}
	for _, want := range []string{
		`name: "subagent_grounding"`, `rolePrompt("grounding")`,
		`name: "subagent_explore"`, `rolePrompt("explore")`,
		`name: "subagent_review"`, `StringEnum(["adr", "plan", "code"]`,
		`name: "subagent_implement"`, `allowCommits: Type.Boolean()`,
		`task: Type.String({ minLength: 1 })`, `{ additionalProperties: false }`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing four-tool schema/role contract %q", want)
		}
	}
}

// invariant: pi-subagent-progress-context-isolation
func TestPiSubagentProgressContextIsolation(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{`text: "(running...)"`, `events: update.events`, `result.failureMessage`, `result.output`} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing context-isolation contract %q", want)
		}
	}
	for _, absent := range []string{"appendEntry", "appendMessage", "sendMessage"} {
		if strings.Contains(content, absent) {
			t.Errorf("extension contains forbidden progress channel %q", absent)
		}
	}
}

// invariant: pi-subagent-progress-rendering
func TestPiSubagentProgressRendering(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, role := range []string{"grounding", "explore", "review", "implement"} {
		if strings.Count(content, `...renderers("`+role+`")`) != 1 {
			t.Errorf("role %s does not delegate both render hooks", role)
		}
	}
	for _, want := range []string{`renderCall(args:`, `renderResult(result:`, `keyHint("app.tools.expand"`, "COLLAPSED_EVENT_COUNT"} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing rendering contract %q", want)
		}
	}
}

// invariant: pi-subagent-failure-details
func TestPiSubagentFailureDetails(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{`awfFailure: true`, `pi.on("tool_result"`, `SUBAGENT_TOOL_NAMES.has(event.toolName)`, `return { isError: true }`} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing failure-details contract %q", want)
		}
	}
}

// invariant: pi-subagent-progress-bounds
func TestPiSubagentProgressBounds(t *testing.T) {
	content := renderPiExtensionFile(t, "runner.ts") + renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{"MAX_DISPLAY_EVENTS = 20", "MAX_DISPLAY_EVENT_BYTES = 2 * 1024", "omittedEvents", `Buffer.byteLength(JSON.stringify(fitted), "utf8")`, "MAX_TASK_PREVIEW_BYTES", "MAX_FALLBACK_BYTES"} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing progress bound %q", want)
		}
	}
	if strings.Contains(content, "rawTranscript") {
		t.Fatal("extension retains a raw transcript")
	}
}

// invariant: pi-child-tool-boundaries
func TestPiSubagentToolBoundaries(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{
		`EXPLORE_TOOLS = ["read", "grep", "find", "ls", "bash"]`,
		`IMPLEMENT_TOOLS = ["read", "bash", "edit", "write", "grep", "find", "ls"]`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing boundary %q", want)
		}
	}
}

// invariant: pi-child-process-safety
func TestPiSubagentProcessSafetyContract(t *testing.T) {
	content := renderPiExtensionFile(t, "runner.ts")
	for _, want := range []string{"SIGTERM", "SIGKILL", "removeEventListener", "await deps.rm"} {
		if !strings.Contains(content, want) {
			t.Errorf("runner missing process-safety contract %q", want)
		}
	}
}

// invariant: pi-implementation-state-boundary
func TestPiImplementationStateBoundary(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{"implementationTail", "allowCommits=false", "changes were not reverted", "commitVerification"} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing implementation-state contract %q", want)
		}
	}
}

// invariant: pi-minimum-runtime
func TestPiMinimumRuntimeContract(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	if !strings.Contains(content, `MIN_PI_VERSION = "0.80.9"`) || !strings.Contains(content, `pi.on("session_start"`) {
		t.Fatal("extension missing minimum-version startup contract")
	}
}

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
	path := ".pi/extensions/awf-subagents/" + name
	for _, file := range files {
		if file.Path == path {
			return file.Content
		}
	}
	t.Fatalf("missing %s", path)
	return ""
}

func TestPiTargetDescriptorChangesSkillConfigHash(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: [tdd]\nagents: []\nvars: {testCmd: go test ./..., gateCmd: make gate}\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	var before string
	for _, file := range files {
		if file.Path == ".pi/skills/example-tdd/SKILL.md" {
			before = file.ConfigHash
		}
	}
	p.Targets[0].Capabilities = nil
	files, err = p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path == ".pi/skills/example-tdd/SKILL.md" && file.ConfigHash == before {
			t.Fatal("Pi target descriptor change did not change skill config hash")
		}
	}
}

// invariant: pi-dedicated-grounding-dispatch
func TestPiDedicatedGroundingDispatch(t *testing.T) {
	config := "prefix: example\nskills: [adr-lifecycle, brainstorming, bugfix, debugging, executing-plans, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, tdd, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [%s]\n"
	dirs := map[string]string{
		"claude": ".claude/skills", "codex": ".agents/skills", "copilot": ".github/skills",
		"cursor": ".cursor/skills", "gemini": ".gemini/skills", "pi": ".pi/skills",
	}
	for _, target := range []string{"claude", "codex", "copilot", "cursor", "gemini", "pi"} {
		root := scaffold(t, fmt.Sprintf(config, target))
		p, err := Open(root)
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
		brainstorm := got[dirs[target]+"/example-brainstorming/SKILL.md"]
		audit := got[dirs[target]+"/example-refactor-coupling-audit/SKILL.md"]
		if target == "pi" {
			if !strings.Contains(brainstorm, "`subagent_grounding`") {
				t.Error("Pi brainstorming does not name subagent_grounding")
			}
			if !strings.Contains(audit, "`subagent_explore`") {
				t.Error("Pi coupling audit does not name subagent_explore")
			}
			continue
		}
		if strings.Contains(brainstorm, "subagent_") || strings.Contains(audit, "subagent_") {
			t.Errorf("non-Pi target %s names a Pi subagent tool", target)
		}
	}
}

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
	// invariant: multi-target-render
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
	// invariant: cursor-no-bridge
	if bridges != 1 {
		t.Errorf("bridge files = %d, want 1 (claude only; cursor has none)", bridges)
	}
	if _, ok := byPath["CLAUDE.md"]; !ok {
		t.Error("CLAUDE.md (claude bridge) not rendered")
	}
}

// invariant: targets-default-claude
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
	for _, want := range []string{"CLAUDE.md", "AGENTS.md", "docs/decisions/ACTIVE.md", "docs/domains/rendering.md"} {
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
