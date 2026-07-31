package project

import (
	"os"
	"os/exec"
	"path/filepath"
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

// invariant: rendering/pi-workflows:pi-native-workflow-skills
func TestNativePiSkillsAreDiscoverableAndPruned(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nskills: [tdd, local]\nagents: []\ntargets: [pi]\n", map[string]string{
		"skills/local.yaml":             "data:\n  description: Local Pi workflow guidance.\n",
		"skills/parts/local/content.md": "Use this local skill when it fits.\n",
	})
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	standard := filepath.Join(root, ".pi/skills/example-tdd/SKILL.md")
	local := filepath.Join(root, ".pi/skills/example-local/SKILL.md")
	for _, path := range []string{standard, local} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing native Pi skill %s: %v", path, err)
		}
	}
	if err := os.WriteFile(configPath(root), []byte("prefix: example\nskills: [local]\nagents: []\ntargets: [pi]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(standard); !os.IsNotExist(err) {
		t.Fatalf("disabled standard skill was not pruned: %v", err)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("enabled local skill was pruned: %v", err)
	}
	if err := os.WriteFile(configPath(root), []byte("prefix: example\nskills: [local]\nagents: []\ntargets: [claude]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{local, filepath.Join(root, ".pi/skills"), filepath.Join(root, ".pi")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Pi disable did not prune %s: %v", path, err)
		}
	}
	for _, path := range []string{filepath.Join(root, ".pi/awf-workflow"), filepath.Join(root, ".pi/awf-workflows"), filepath.Join(root, ".pi/extensions/awf-telemetry")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("retired Pi output exists at %s: %v", path, err)
		}
	}
}

// invariant: rendering/pi-runtime:pi-extension-target-render
func TestPiRuntimeTargetRender(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	extensions := map[string]string{}
	for _, file := range files {
		if strings.HasPrefix(file.Path, ".pi/extensions/") {
			extensions[file.Path] = file.Content
		}
	}
	if len(extensions) != 4 {
		t.Fatalf("Pi extension count = %d, want 4: %v", len(extensions), extensions)
	}
	for _, path := range []string{".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/model-routing.ts", ".pi/extensions/awf-subagents/runner.ts"} {
		if content := extensions[path]; !strings.HasPrefix(content, "// "+bannerText+"\n") {
			t.Errorf("%s lacks provenance banner", path)
		}
	}
	for _, banned := range []string{"awf-telemetry", "awf-workflow", "awf-workflows"} {
		for path := range extensions {
			if strings.Contains(path, banned) {
				t.Errorf("retired extension rendered: %s", path)
			}
		}
	}
	handoff := extensions[".pi/extensions/awf-handoff/index.ts"]
	index := extensions[".pi/extensions/awf-subagents/index.ts"]
	routing := extensions[".pi/extensions/awf-subagents/model-routing.ts"]
	if !strings.Contains(handoff, "registerHandoff(pi") || !strings.Contains(index, "registerSubagentTools(pi") {
		t.Fatal("Pi extension entrypoints are not registered")
	}
	for _, owned := range []string{"export const PREFERENCE_FIELDS", "export function parsePreferenceSource", "export async function loadPreferenceState", "export function effectivePreferenceState", "export function resolveChildModel", "export function buildRoutingCard"} {
		if !strings.Contains(routing, owned) {
			t.Errorf("model-routing module missing owned policy %q", owned)
		}
		if strings.Contains(index, owned) {
			t.Errorf("subagent entrypoint duplicates routing policy %q", owned)
		}
	}
	if !strings.Contains(index, `from "./model-routing.ts"`) {
		t.Error("subagent entrypoint does not consume model-routing module")
	}
}

// invariant: rendering/pi-runtime:pi-minimum-runtime
func TestPiMinimumRuntime(t *testing.T) {
	for _, name := range []string{"awf-handoff/index.ts", "awf-subagents/index.ts"} {
		out := renderPiExtensionFile(t, name)
		for _, want := range []string{"MIN_PI_VERSION", "guardMinimumRuntime", "awf.pi.minimum-runtime-notified", "Upgrade Pi and reload."} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing minimum-runtime guard %q", name, want)
			}
		}
	}
}

// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle
func TestHandoffLifecycleIndependentOfEffortState(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-handoff/index.ts")
	for _, want := range []string{"let pending", "queueCommand(\"awf-handoff-continue\"", "Fresh-session handoff", "parentSession:old", "prepared?.cleanup?.()", "pending=undefined"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff lifecycle output missing %q", want)
		}
	}
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), "tools/pi-extension-test/tests/handoff.test.ts"))
	if err != nil || !strings.Contains(string(body), "handoff counts down, cancels, cleans pending, and links parent before setup kickoff") {
		t.Fatalf("TypeScript lifecycle behavior proof missing: %v", err)
	}
}

// invariant: rendering/pi-workflows:pi-session-handoff-public-contract
func TestHandoffPublicOwnedMemoryContract(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-handoff/index.ts")
	for _, want := range []string{"memoryPath:Type.Optional(Type.String())", "validateMemoryPath", ".awf/efforts/", "/memory.md", "1048576", "TextDecoder", "sameIdentity", "Effort: ${slug}", "kickoff:Type.String({maxLength:1000})", "Then continue with this immediate action"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff public contract missing %q", want)
		}
	}
	for _, banned := range []string{"runAwf", "state.json", "assignment", "selected-effort", "telemetry", "adopt"} {
		if strings.Contains(out, banned) {
			t.Errorf("handoff public contract retains %q", banned)
		}
	}
}

// invariant: rendering/pi-workflows:pi-subagent-model-routing
// invariant: rendering/pi-workflows:pi-subagent-model-preferences
// invariant: rendering/pi-workflows:pi-subagent-model-wizard
func TestPiRealRuntimeSmoke(t *testing.T) {
	root := repoRootDir(t)
	cmd := exec.Command(filepath.Join(root, "x"), "pi-test", "run")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Pi runtime smoke failed: %v\n%s", err, output)
	}
}

// invariant: rendering/pi-workflows:pi-session-handoff-workflow
func TestHandoffWorkflowUsesOwnedCheckpoint(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-handoff/index.ts")
	for _, want := range []string{"Continue a validated fresh-session handoff.", "Continue from an optional effort-owned awf checkpoint", "Read ${memoryPath} first.", "Then continue with this immediate action"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff workflow contract missing %q", want)
		}
	}
	for _, banned := range []string{"selected effort", "telemetry lifecycle", "structured resume", "adopt_effort"} {
		if strings.Contains(out, banned) {
			t.Errorf("handoff workflow contract retains %q", banned)
		}
	}

	data := map[string]any{
		"prefix": "example", "vars": map[string]any{"gateCmd": "./x gate"},
		"layout": testLayout(), "data": map[string]any{}, "targetSessionHandoff": true,
	}
	settled := map[string]string{
		"executing-plans":             "Review settles before checkpointing.",
		"subagent-driven-development": "checkpoints only after findings resolve",
	}
	for name, settledPhrase := range settled {
		body := renderSkillGolden(t, name, data)
		settledAt := strings.Index(body, settledPhrase)
		checkpointAt := strings.Index(body, "**Routine checkpoint.**")
		handoffAt := strings.Index(body, "handoff_session")
		if got := strings.Count(body, "handoff_session"); got != 1 {
			t.Errorf("%s renders %d handoff_session invocations, want one settled-phase invocation", name, got)
		}
		if settledAt < 0 || checkpointAt < settledAt || handoffAt < checkpointAt {
			t.Errorf("%s does not place its sole Pi handoff after settled phase persistence", name)
		}
		for _, banned := range []string{
			"after every checkbox task", "after each checkbox task", "after any checkbox task",
			"checkbox task triggers", "after every batch-helper return", "after each batch-helper return",
			"batch-helper return triggers", "handoff after a helper return",
		} {
			if strings.Contains(strings.ToLower(body), banned) {
				t.Errorf("%s retains task/helper handoff trigger %q", name, banned)
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

// invariant: rendering/pi-workflows:pi-structured-exploration-contract
func TestPiStructuredExplorationContractRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement", "MAX_EXPLORATION_CONCURRENCY = 10", "queues the rest FIFO with abort-aware removal"} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi subagent extension missing %q", want)
		}
	}
}

func TestPiSubagentModelRoutingRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts") + renderPiExtensionFile(t, "awf-subagents/model-routing.ts")
	for _, want := range []string{
		"maxLength: 256", `MODEL_REFERENCE_PATTERN = "^[\\x21-\\x2E\\x30-\\x7E]+/[\\x21-\\x7E]+$"`,
		"pattern: MODEL_REFERENCE_PATTERN", "MODEL_REFERENCE_FORM = new RegExp(MODEL_REFERENCE_PATTERN)",
		"!MODEL_REFERENCE_FORM.test(value)",
		"Exact provider/model-id in printable ASCII", "default, auto, and inherit parent are invalid",
		"Omit the model field to use configured or inherited routing.", "const finalSelected = await refreshAndResolve",
		"requestedModel", "resolvedModel", "thinkingLevel",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi model routing render missing %q", want)
		}
	}
	if got := strings.Count(body, "model: Type.Optional(MODEL_REFERENCE_SCHEMA)"); got != 4 {
		t.Errorf("shared model-reference schema uses = %d, want 4", got)
	}
	// ADR-0176: both validating layers derive from one pattern constant, so neither can
	// drift into a different measure or charset than the other. Scoped to the module that
	// owns the constant, so an unrelated inline JSON-Schema pattern elsewhere is not
	// misreported as a model-reference regression.
	routing := renderPiExtensionFile(t, "awf-subagents/model-routing.ts")
	if got := strings.Count(routing, `pattern: "`); got != 0 {
		t.Errorf("model-routing.ts inlines a literal JSON-Schema pattern %d times, want the shared MODEL_REFERENCE_PATTERN constant only", got)
	}
	// ADR-0176: the omitted-model display label must not be a usable argument.
	if !strings.Contains(body, `: "(configured or inherited)";`) {
		t.Error("omitted-model display label is not the non-parseable phrase")
	}
	if strings.Contains(body, `: "inherit parent";`) {
		t.Error("omitted-model display label still renders a rejected sentinel value")
	}
}

func TestPiSubagentModelPreferencesRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts") + renderPiExtensionFile(t, "awf-subagents/model-routing.ts")
	for _, want := range []string{
		`PREFERENCE_TIERS = ["small", "standard", "large"]`,
		`PREFERENCE_FIELDS = ["default", ...PREFERENCE_ROLES, ...PREFERENCE_TIERS]`,
		`type SourceReason = "read-error" | "malformed-json" | "non-object" | "unknown-key"`,
		`type FieldReason = "malformed" | "overlong" | "unregistered" | "unauthenticated" | "unavailable"`,
		"await loadPreferenceState(deps, ctx.modelRegistry)", "new WeakSet<object>()", "ctx.sessionManager",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi model preference render missing %q", want)
		}
	}
}

func TestPiSubagentModelWizardRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts") + renderPiExtensionFile(t, "awf-subagents/model-routing.ts")
	for _, want := range []string{
		`small: "openai-codex/gpt-5.6-luna"`, `standard: "openai-codex/gpt-5.6-terra"`, `large: "openai-codex/gpt-5.6-sol"`,
		"Role defaults:", "Tier mappings:", "Missing:", "Invalid:", "modified concurrently", "mode: 0o600", "await loadPreferenceState(deps, ctx.modelRegistry)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi model wizard render missing %q", want)
		}
	}
}

func explorationFixtureConfig(target string) string {
	return "prefix: example\nskills: [adr-lifecycle, brainstorming, debugging, executing-direct, executing-plans, exploring, orienting, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, writing-plans]\nagents: [adr-reviewer, code-reviewer, explorer, grounding-checker, implementer, plan-reviewer]\ntargets: [" + target + "]\n"
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
// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts
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
				"targeted < bounded < broad", "targeted locates one declaration", "bounded investigates within a named symbol", "broad searches across the project search universe",
				"paths < summary < analysis", "file:line", "file:start-end", "minimal labels and no search narrative", "concise explanations", "evidence-grounded synthesis",
				"adaptive maximum", "cheapest targeted lookup", "widen only when evidence requires", "never widen beyond the selected maximum", "boundary is exhausted, report that explicitly",
				"tracked files plus non-ignored untracked working-tree files", "tracked generated and vendored files", "ignored files", ".git", "nested repositories", "external dependencies unless the task explicitly brings",
				"not-found", "inconclusive", "unverified", "Not found within <breadth> boundary:", "successful execution", "one concise next refinement", "project search universe and searched surfaces", "Ground every material claim with file/line evidence",
				"new fresh-context call", "correct the task", "change report detail", "widen breadth", "one information need", "relevant final findings",
			}
			for _, want := range shared {
				if !strings.Contains(exploring, want) {
					t.Errorf("%s exploring skill missing shared contract %q", target, want)
				}
			}
			if target == "pi" {
				for _, want := range []string{"subagent_explore", "required task, breadth, and detail", "at most ten exploration children", "queues the rest FIFO", "omit the `model` field to use configured role routing", "tier's exact `provider/model-id`"} {
					if !strings.Contains(exploring, want) {
						t.Errorf("Pi exploring skill missing %q", want)
					}
				}
			} else {
				for _, want := range []string{"target-native fresh-context exploration subagent", "`explorer` agent", "task", "breadth", "detail"} {
					if !strings.Contains(exploring, want) {
						t.Errorf("%s exploring skill missing %q", target, want)
					}
				}
				for _, absent := range []string{"subagent_explore", "provider/model-id", "ten exploration children", "queues the rest FIFO"} {
					if strings.Contains(exploring, absent) {
						t.Errorf("%s exploring skill leaks Pi guidance %q", target, absent)
					}
				}
				// The target-native grounding branch names its agent the same way.
				if brainstorming := skillBody("brainstorming"); !strings.Contains(brainstorming, "`grounding-checker` agent") {
					t.Errorf("%s brainstorming skill does not name the grounding-checker agent", target)
				}
			}
			// Orienting owns the dispatch conditions brainstorming used to carry
			// inline; brainstorming now reaches them only by invoking orienting.
			for _, consumer := range []string{"orienting", "debugging", "refactor-coupling-audit"} {
				body := skillBody(consumer)
				for _, want := range []string{"location is unknown", "and inline search would pollute the parent context", "exact-known-file", "genuinely trivial"} {
					if !strings.Contains(body, want) {
						t.Errorf("%s/%s missing dispatch condition %q", target, consumer, want)
					}
				}
			}
			// The dispatch route names the target-prefixed exploring skill, and
			// brainstorming's shrunk step routes to orienting rather than
			// duplicating the conditions.
			if orienting := skillBody("orienting"); !strings.Contains(orienting, "`example-exploring`") {
				t.Errorf("%s orienting skill does not name the prefixed exploring skill", target)
			}
			if brainstorming := skillBody("brainstorming"); !strings.Contains(brainstorming, "`example-orienting`") {
				t.Errorf("%s brainstorming skill does not invoke the prefixed orienting skill", target)
			}
		})
	}
}

// invariant: rendering/workflow-skill-templates:bounded-exploration-reporting
func TestBoundedExplorationReporting(t *testing.T) {
	files := explorationRenderedByPath(t, "prefix: example\nskills: [exploring]\nagents: [explorer]\ntargets: [pi]\n")
	guidance := files[".pi/skills/example-exploring/SKILL.md"]
	prompt := renderPiExtensionFile(t, "awf-subagents/index.ts")
	explorer := renderAgentGolden(t, "explorer", map[string]any{
		"prefix": "example", "vars": map[string]any{}, "data": map[string]any{},
	})
	contracts := map[string]struct {
		body  string
		wants []string
	}{
		"rendered exploring guidance": {guidance, []string{
			"Independent information needs may be sibling-dispatched", "at most ten exploration children", "queues the rest FIFO", "Refinement stays sequential",
			"targeted < bounded < broad", "targeted locates one declaration, implementation, file, or exact fact", "bounded investigates within a named symbol, package, component, or subsystem", "broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts",
			"adaptive maximum", "cheapest targeted lookup", "widen only when evidence requires it", "never widen beyond the selected maximum", "If the boundary is exhausted, report that explicitly",
			"tracked files plus non-ignored untracked working-tree files under the current repository root", "tracked generated and vendored files", "ignored files", ".git", "nested repositories", "external dependencies unless the task explicitly brings one of those surfaces into scope",
			"paths < summary < analysis", "paths returns only relevant file:line or file:start-end locations with minimal labels and no search narrative", "summary returns grounded locations plus concise explanations of what each contains and why it matters", "analysis directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty",
			"Ground every material claim with file/line evidence", "Not found within <breadth> boundary: <what was searched>", "successful execution", "one concise next refinement", "broad absence report must name the project search universe and searched surfaces", "Distinguish inconclusive and unverified outcomes from absence",
			"new fresh-context call to correct the task, change report detail, or widen breadth",
		}},
		"Pi per-call suffix": {prompt, []string{
			"at most ten active exploration children", "queues the rest FIFO with abort-aware removal",
			"Selected breadth maximum:", "Selected report detail:",
		}},
		"explorer agent": {explorer, []string{
			"independent information needs concurrently", "refinement of an earlier result stays sequential",
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

// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract
func TestMaintainableCodeMultiTargetParity(t *testing.T) {
	root := scaffold(t, "prefix: example\nskills:\n  - subagent-driven-development\nagents: [implementer]\ndocs: []\ntargets:\n  - claude\n  - pi\n")
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	subagentArtifacts := map[string]int{
		".claude/skills/example-subagent-driven-development/SKILL.md": 0,
		".pi/skills/example-subagent-driven-development/SKILL.md":     0,
	}
	for _, file := range files {
		byPath[file.Path] = file.Content
		if _, ok := subagentArtifacts[file.Path]; ok && file.TemplateID == "skills/subagent-driven-development/SKILL.md.tmpl" {
			subagentArtifacts[file.Path]++
		}
	}
	categories := []string{
		"semantic boundary and ownership",
		"external/internal representations and their translation point",
		"allowed dependency direction",
		"preparatory-refactor decision",
		"prohibited bolt-on shortcuts",
		"validation expectations",
		"does not replan, broaden the task, or perform unrelated cleanup",
	}
	for _, path := range []string{
		".claude/skills/example-subagent-driven-development/SKILL.md",
		".pi/skills/example-subagent-driven-development/SKILL.md",
	} {
		out := byPath[path]
		if out == "" {
			t.Fatalf("missing rendered skill %q", path)
		}
		for _, want := range categories {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing handoff semantic %q:\n%s", path, want, out)
			}
		}
	}
	if got := len(byPath); got != len(files) {
		t.Fatalf("rendered outputs contain duplicate paths: %d files, %d paths", len(files), got)
	}
	for path, got := range subagentArtifacts {
		if got != 1 {
			t.Errorf("%s rendered %d subagent-driven-development artifacts from its template, want 1", path, got)
		}
	}
	docs := 0
	for _, file := range files {
		if file.Path == "docs/maintainable-code-design.md" {
			docs++
		}
	}
	if docs != 1 {
		t.Errorf("maintainable design guide renders %d times, want 1", docs)
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

// invariant: rendering/pi-workflows:pi-implement-role-artifact
func TestPiImplementRoleArtifact(t *testing.T) {
	src := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{
		".pi/agents/implementer.md",
		"loadImplementer",
		"Enable the implementer agent and run awf render.",
		"has no instruction body; run awf render.",
		"parseFrontmatter",
		// Both snapshot directions, the pre-existing one and the new mirror.
		"before.head !== after.head",
		"before.head === after.head",
		"commit-capable but created no commit",
		"committed despite allowCommits=false",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("rendered Pi extension missing %q", want)
		}
	}
	// The literal implement prose must be gone, so a half-applied edit that adds
	// the loader while leaving the old string in place fails here.
	if strings.Contains(src, "You are a fresh-context implementation subagent") {
		t.Error("the literal implement role prose survived the loader cutover")
	}
}

// invariant: rendering/pi-workflows:pi-role-contract-loader
func TestPiRoleContractLoader(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{
		"loadExplorer", "loadGroundingChecker",
		".pi/agents/explorer.md", ".pi/agents/grounding-checker.md",
		"Enable the explorer agent and run awf render.",
		"Enable the grounding-checker agent and run awf render.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi extension missing loader element %q", want)
		}
	}
	// The opening clause of each body as it lived in the old inline prompt, so a
	// branch left behind fails loudly rather than silently duplicating a contract.
	for _, banned := range []string{
		"You are a fresh-context exploration subagent. Read files",
		"You are a fresh-context grounding-check subagent. Read and run",
	} {
		if strings.Contains(body, banned) {
			t.Errorf("role prose survived inline in the extension: %q", banned)
		}
	}
}
