package project

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
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

// invariant: rendering/pi-workflows:pi-native-workflow-skills (TestNativePiSkillsAreDiscoverableAndPruned)
func TestNativePiSkillsAreDiscoverableAndPruned(t *testing.T) {
	root := scaffoldFiles(t, "prefix: example\nintegrationBranch: main\nskills: [tdd, local]\nagents: []\ntargets: [pi]\n", map[string]string{
		"skills/local.yaml":             "data:\n  description: Local Pi workflow guidance.\n",
		"skills/parts/local/content.md": "Use this local skill when it fits.\n",
	})
	p, err := Open(testContext(t), root)
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
	if err := os.WriteFile(configPath(root), []byte("prefix: example\nintegrationBranch: main\nskills: [local]\nagents: []\ntargets: [pi]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
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
	if err := os.WriteFile(configPath(root), []byte("prefix: example\nintegrationBranch: main\nskills: [local]\nagents: []\ntargets: [claude]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err = Open(testContext(t), root)
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

// invariant: rendering/pi-runtime:pi-extension-target-render (TestPiRuntimeTargetRender)
// invariant: rendering/pi-workflows:using-effort-skill (TestPiRuntimeTargetRender)
func TestPiRuntimeTargetRender(t *testing.T) {
	if _, independentlySelectable := catalog.Standard.Skills["using-effort"]; independentlySelectable {
		t.Fatal("using-effort companion became independently selectable")
	}
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: [effort-workflow]\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
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
	expectedExtensions := map[string]bool{}
	for _, path := range []string{".pi/extensions/awf-context-usage/index.ts", ".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/model-routing.ts", ".pi/extensions/awf-subagents/runner.ts", ".pi/extensions/awf-effort/index.ts", ".pi/extensions/awf-effort/client.ts"} {
		expectedExtensions[path] = true
		content, ok := extensions[path]
		if !ok {
			t.Errorf("missing governed Pi extension %s", path)
		} else if !strings.HasPrefix(content, "// "+bannerText+"\n") {
			t.Errorf("%s lacks provenance banner", path)
		}
	}
	for path := range extensions {
		if !expectedExtensions[path] {
			t.Errorf("unexpected Pi extension rendered: %s", path)
		}
	}
	// The optional presentation boundary is awf-owned and independent of metadata.
	effort := extensions[".pi/extensions/awf-effort/index.ts"]
	for _, required := range []string{
		"remote-pi:capabilities:request",
		"remote-pi:capabilities",
		"remote-pi:display-suffix:set",
		"remote-pi:display-suffix:request",
		"displaySuffix",
		"value: string | null",
		`suffixSupported && current ? current.slug : null`,
		`pi.on?.("session_start", () => { clear(); requestCapabilities(); })`,
		`pi.events?.on?.("remote-pi:display-suffix:request", () => publishSuffix())`,
		`pi.events?.on?.("remote-pi:capabilities", (caps: RemotePiCapabilitiesReplyPayload) => { suffixSupported = supportsDisplaySuffix(caps); publishSuffix(); })`,
		`namespace: "awf", value: snapshot ?`,
	} {
		if !strings.Contains(effort, required) {
			t.Errorf("awf effort extension lacks display-suffix behavior %q", required)
		}
	}
	if got := strings.Count(effort, "requestCapabilities()"); got != 2 {
		t.Errorf("awf effort extension capability request count = %d, want factory plus session start", got)
	}
	for _, forbidden := range []string{"name-override", "nameOverride", "NameOverride", "_displayName"} {
		if strings.Contains(effort, forbidden) {
			t.Errorf("awf effort extension retains routing-name contract %q", forbidden)
		}
	}
	for _, banned := range []string{"awf-telemetry", "awf-workflow", "awf-workflows"} {
		for path := range extensions {
			if strings.Contains(path, banned) {
				t.Errorf("retired extension rendered: %s", path)
			}
		}
	}
	contextUsage := extensions[".pi/extensions/awf-context-usage/index.ts"]
	handoff := extensions[".pi/extensions/awf-handoff/index.ts"]
	index := extensions[".pi/extensions/awf-subagents/index.ts"]
	routing := extensions[".pi/extensions/awf-subagents/model-routing.ts"]
	if !strings.Contains(contextUsage, "registerContextUsage(pi") || !strings.Contains(handoff, "registerHandoff(pi") || !strings.Contains(index, "registerSubagentTools(pi") {
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
	if _, ok := extensions[".pi/extensions/awf-effort/index.ts"]; !ok {
		t.Error("selected effort-workflow did not render awf-effort index")
	}
	if _, ok := extensions[".pi/extensions/awf-effort/client.ts"]; !ok {
		t.Error("selected effort-workflow did not render awf-effort client")
	}
	usingEffort := ""
	for _, file := range files {
		if file.Path == ".pi/skills/example-using-effort/SKILL.md" {
			usingEffort = file.Content
		}
	}
	for _, want := range []string{"Use `using_effort` explicitly", "{ effort: \"<canonical-slug>\" }", "{ detach: true }", "Pi remains at repository root", "use the supplied relative memory path `.awf/efforts/<slug>/memory.md`", "when present, managed-worktree path `.awf/worktrees/<slug>`", "Restart begins detached", "display-only suffix", "suffix is never routing input", "Activity is neither authority nor a lock", "When attached, prefer `effort_memory_read` for pathless reads", "`effort_memory_edit` only for Markdown body changes", "`effort_memory_update` for `phase` or `next`", "timestamps are automatic", "Generic file tools and direct awf commands remain available"} {
		if !strings.Contains(usingEffort, want) {
			t.Errorf("using-effort companion missing %q:\n%s", want, usingEffort)
		}
	}
	for _, config := range []string{
		"prefix: example\nintegrationBranch: main\nskills: [effort-workflow]\nagents: []\ntargets: [claude]\n",
		"prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n",
	} {
		for path, content := range explorationRenderedByPath(t, config) {
			if strings.Contains(path, "awf-effort") || strings.Contains(path, "using-effort") || strings.Contains(content, "awf-effort") || strings.Contains(content, "using-effort") || strings.Contains(content, "effort_memory_") {
				t.Errorf("unselected or non-Pi rendering leaked using-effort contract into %s", path)
			}
		}
	}
}

// invariant: rendering/pi-workflows:pi-effort-memory-tools (TestPiEffortMemoryToolContract)
func TestPiEffortMemoryToolContract(t *testing.T) {
	selected := explorationRenderedByPath(t, "prefix: example\nintegrationBranch: main\nskills: [effort-workflow]\nagents: []\ntargets: [pi]\n")
	index := selected[".pi/extensions/awf-effort/index.ts"]
	client := selected[".pi/extensions/awf-effort/client.ts"]
	guidance := selected[".pi/skills/example-using-effort/SKILL.md"]
	for _, want := range []string{
		`MEMORY_TOOL_NAMES = ["effort_memory_read", "effort_memory_edit", "effort_memory_update"]`,
		`Type.Object({ offset: Type.Optional(Type.Integer({ minimum: 1 })), limit: Type.Optional(Type.Integer({ minimum: 1 })) }, { additionalProperties: false })`,
		`Type.Array(Type.Object({ oldText: Type.String({ minLength: 1, maxLength: 1048576 }), newText: Type.String({ maxLength: 1048576 }) }, { additionalProperties: false }), { minItems: 1, maxItems: 128 })`,
		`Type.Object({ phase: Type.Optional(Type.String({ minLength: 1, maxLength: 500 })), next: Type.Optional(Type.String({ minLength: 1, maxLength: 500 })) }, { additionalProperties: false })`,
		"activate(true)", "const clear = () => { current = undefined; activate(false); publish(); }", "fileMutationQueue(join(ctx.cwd, \".awf\", \"efforts\", snapshot.slug, \"memory.md\"), run)",
		"const memoryCall = async", "serial(async () =>", `reply.condition === "not-owner" || reply.condition === "missing" || reply.condition === "unsafe-activity"`, `pi.on?.("session_start", () => { clear();`,
		"getActiveTools", "setActiveTools", "withFileMutationQueue", "promptGuidelines", "Generic file tools remain available",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("rendered effort index missing memory-tool contract %q", want)
		}
	}
	for _, want := range []string{"decodeMemory", "exact(reply", "MEMORY_STDOUT_MAX", "MEMORY_STDERR_MAX", `stdout.endsWith("\n")`, "memoryRead", "memoryEdit", "memoryUpdate", "childMemoryExecutor"} {
		if !strings.Contains(client, want) {
			t.Errorf("rendered effort client missing memory-tool contract %q", want)
		}
	}
	for _, want := range []string{"prefer `effort_memory_read`", "`effort_memory_edit` only for Markdown body changes", "`effort_memory_update` for `phase` or `next`", "timestamps are automatic", "Generic file tools and direct awf commands remain available"} {
		if !strings.Contains(guidance, want) {
			t.Errorf("rendered using-effort guidance missing %q", want)
		}
	}
	for _, config := range []string{
		"prefix: example\nintegrationBranch: main\nskills: [effort-workflow]\nagents: []\ntargets: [claude]\n",
		"prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n",
	} {
		for path, content := range explorationRenderedByPath(t, config) {
			if strings.Contains(path, "awf-effort") || strings.Contains(path, "using-effort") || strings.Contains(content, "effort_memory_") {
				t.Errorf("unselected or non-Pi rendering leaked memory tools into %s", path)
			}
		}
	}
	fallback := renderSkillGolden(t, "using-effort", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}})
	assertNoLeaks(t, fallback)
	if os.Getenv("AWF_PI_RUNTIME_SMOKE") == "1" {
		assertPiRuntimeSmoke(t)
	}
}

func TestPiMinimumRuntime(t *testing.T) {
	for _, name := range []string{"awf-context-usage/index.ts", "awf-handoff/index.ts", "awf-subagents/index.ts"} {
		out := renderPiExtensionFile(t, name)
		for _, want := range []string{"MIN_PI_VERSION", "guardMinimumRuntime", "awf.pi.minimum-runtime-notified", "Upgrade Pi and reload."} {
			if !strings.Contains(out, want) {
				t.Errorf("%s missing minimum-runtime guard %q", name, want)
			}
		}
	}
}

func TestPiContextUsageInjection(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-context-usage/index.ts")
	for _, want := range []string{"pi.on(\"context\"", "[session context]", "unknown/", "unavailable;", "getContextUsage()", "getBranch()", "entry.type===\"compaction\"", "customType:\"awf-context-usage\"", "display:false"} {
		if !strings.Contains(out, want) {
			t.Errorf("context usage output missing %q", want)
		}
	}
	for _, banned := range []string{"appendEntry(", "registerTool(", "registerCommand(", "queueCommand(", "handoff_session", "telemetry"} {
		if strings.Contains(out, banned) {
			t.Errorf("context usage output retains side effect %q", banned)
		}
	}
}

// invariant: rendering/pi-workflows:pi-subagent-model-routing (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-subagent-model-preferences (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-subagent-model-wizard (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-effort-session-association (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-native-workflow-skills (TestPiRealRuntimeSmoke)
// invariant: rendering/project-output-plan:multi-target-render (TestPiRealRuntimeSmoke)
// invariant: rendering/catalog-and-targets:target-dialect-render (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-session-handoff-lifecycle (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-session-handoff-public-contract (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-runtime:pi-context-usage-injection (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-runtime:pi-minimum-runtime (TestPiRealRuntimeSmoke)
var (
	piRuntimeSmokeOnce   sync.Once
	piRuntimeSmokeOutput []byte
	piRuntimeSmokeErr    error
)

func assertPiRuntimeSmoke(t *testing.T) {
	t.Helper()
	piRuntimeSmokeOnce.Do(func() {
		root := repoRootDir(t)
		cmd := exec.Command(filepath.Join(root, "x"), "pi-test", "run")
		cmd.Dir = root
		piRuntimeSmokeOutput, piRuntimeSmokeErr = cmd.CombinedOutput()
	})
	if piRuntimeSmokeErr != nil {
		t.Fatalf("generated Pi runtime smoke failed: %v\n%s", piRuntimeSmokeErr, piRuntimeSmokeOutput)
	}
}

func TestPiRealRuntimeSmoke(t *testing.T) {
	if os.Getenv("AWF_PI_RUNTIME_SMOKE") != "1" {
		t.Skip("Pi container skipped; run './x pi-test run' alone or './x gate' to include it")
	}
	assertPiRuntimeSmoke(t)
}

func TestTargetOutputRenderError(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
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

// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch (TestPiStructuredExplorationContractRender)
// invariant: rendering/pi-workflows:pi-structured-exploration-contract (TestPiStructuredExplorationContractRender)
func TestPiStructuredExplorationContractRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement", "MAX_EXPLORATION_CONCURRENCY = 10", "queues the rest FIFO with abort-aware removal"} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi subagent extension missing %q", want)
		}
	}
	claude := explorationRenderedByPath(t, explorationFixtureConfig("claude"))
	for path, content := range claude {
		if strings.Contains(path, "/skills/") && (strings.Contains(content, "subagent_grounding") || strings.Contains(content, "subagent_explore")) {
			t.Errorf("Claude target %s leaked Pi dispatch tools", path)
		}
	}
	brainstorming := claude[".claude/skills/example-brainstorming/SKILL.md"]
	if !strings.Contains(brainstorming, "`grounding-checker` agent") {
		t.Fatal("Claude grounding dispatch lost its target-native agent")
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
	return "prefix: example\nintegrationBranch: main\nskills: [adr-lifecycle, brainstorming, debugging, executing-direct, executing-plans, exploring, orienting, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, writing-plans]\nagents: [adr-reviewer, code-reviewer, explorer, grounding-checker, implementer, plan-reviewer]\ntargets: [" + target + "]\n"
}

func explorationRenderedByPath(t *testing.T, config string) map[string]string {
	t.Helper()
	p, err := Open(testContext(t), scaffold(t, config))
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

// invariant: rendering/workflow-skill-templates:cross-runtime-exploration-dispatch (TestCrossRuntimeExplorationDispatch)
// invariant: rendering/workflow-skill-templates:explorer-and-grounding-role-contracts (TestCrossRuntimeExplorationDispatch)
func TestCrossRuntimeExplorationDispatch(t *testing.T) {
	if !catalog.Standard.Skills["exploring"].Core {
		t.Fatal("exploring is not a core skill")
	}
	dirs := map[string]string{
		"claude": ".claude/skills",
		"pi":     ".pi/skills",
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

// invariant: rendering/workflow-skill-templates:bounded-exploration-reporting (TestBoundedExplorationReporting)
func TestBoundedExplorationReporting(t *testing.T) {
	files := explorationRenderedByPath(t, "prefix: example\nintegrationBranch: main\nskills: [exploring]\nagents: [explorer]\ntargets: [pi]\n")
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

func renderPiExtensionFile(t *testing.T, name string) string {
	t.Helper()
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: [pi]\n")
	p, err := Open(testContext(t), root)
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

// invariant: rendering/catalog-and-targets:built-in-runtime-targets (TestKnownTargets)
func TestKnownTargets(t *testing.T) {
	if got := KnownTargets(); strings.Join(got, ",") != "claude,pi" {
		t.Fatalf("KnownTargets = %v", got)
	}
	for _, removed := range []string{"codex", "copilot", "cursor", "gemini"} {
		_, err := resolveTargets([]string{removed})
		if err == nil || !strings.Contains(err.Error(), `known: claude, pi`) {
			t.Errorf("resolveTargets(%q) error = %v", removed, err)
		}
		root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: []\nagents: []\ntargets: ["+removed+"]\n")
		if _, err := Open(testContext(t), root); err == nil || !strings.Contains(err.Error(), `known: claude, pi`) {
			t.Errorf("Open target %q error = %v", removed, err)
		}
	}

	synthetic := Target{Name: "synthetic", SkillDir: ".synthetic/skills", AgentDir: ".synthetic/agents", AgentDialect: MarkdownAgentDialect}
	targetRegistry[synthetic.Name] = synthetic
	defer delete(targetRegistry, synthetic.Name)
	resolved, err := resolveTargets([]string{synthetic.Name})
	if err != nil || len(resolved) != 1 || resolved[0].Name != synthetic.Name {
		t.Fatalf("resolve synthetic target = %#v, %v", resolved, err)
	}
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills: [tdd]\nagents: []\ntargets: [synthetic]\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(files, func(file RenderedFile) bool { return file.Path == ".synthetic/skills/example-tdd/SKILL.md" }) {
		t.Fatal("registry-added synthetic target did not render through the generic target path")
	}
}

// invariant: rendering/project-output-plan:multi-target-render (TestTargetDescriptorCustomization)
func TestTargetDescriptorCustomization(t *testing.T) {
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
			Path: ".custom/extension.ts", TemplateID: "pi/awf-context-usage/index.ts.tmpl",
			Producer: TargetOutputTemplate, Encoder: PlainAgentDialect,
			Provenance: render.SlashComment, Policy: OutputPolicy{}, PolicyDeclared: true,
		}},
	}
	if err := custom.validate(); err != nil {
		t.Fatal(err)
	}
	if custom.SkillPath("example", "tdd") != ".custom/workflows/example-tdd/SKILL.md" ||
		custom.AgentPath("code-reviewer") != ".custom/reviewers/code-reviewer.agent.md" {
		t.Fatal("custom descriptor paths were not preserved")
	}
	if custom.targetTemplateData()["targetSubagentTools"] != true || custom.targetTemplateData()["targetSessionHandoff"] != true {
		t.Fatal("custom descriptor capabilities were not projected")
	}
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	p.Targets = []Target{custom}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]AgentDialect{
		".custom/workflows/example-tdd/SKILL.md":   MarkdownAgentDialect,
		".custom/reviewers/code-reviewer.agent.md": MarkdownAgentDialect,
		"CUSTOM.md":            "",
		".custom/extension.ts": PlainAgentDialect,
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
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\ntargets:\n  - claude\n  - pi\n")
	p, err := Open(testContext(t), root)
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
	if !paths["CLAUDE.md"] {
		t.Error("missing Claude bridge")
	}
	if paths["PI.md"] {
		t.Error("unexpected Pi bridge")
	}
}

// invariant: rendering/catalog-and-targets:claude-md-bridge (TestClaudeMdBridgeRendered)
func TestClaudeMdBridgeRendered(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\n")
	p, err := Open(testContext(t), root)
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

// TestMultiTargetRender proves adapter artifacts render once per enabled target
// at descriptor-owned paths while neutral artifacts render once.
// invariant: rendering/catalog-and-targets:target-dialect-render (TestMultiTargetRender)
func TestMultiTargetRender(t *testing.T) {
	root := scaffold(t, sampleYAML+"targets:\n  - claude\n  - pi\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
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
	for _, path := range []string{
		".claude/skills/example-tdd/SKILL.md",
		".pi/skills/example-tdd/SKILL.md",
		".claude/agents/code-reviewer.md",
		".pi/agents/code-reviewer.md",
	} {
		content := byPath[path]
		if content == "" || pathCounts[path] != 1 {
			t.Fatalf("render %q count = %d, content bytes = %d", path, pathCounts[path], len(content))
		}
		if strings.Contains(path, "/agents/") {
			if err := validateArtifact([]byte(content), MarkdownAgentDialect); err != nil {
				t.Fatalf("validate %q: %v", path, err)
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

// invariant: rendering/workflow-skill-templates:maintainable-code-subagent-contract (TestMaintainableCodeMultiTargetParity)
func TestMaintainableCodeMultiTargetParity(t *testing.T) {
	root := scaffold(t, "prefix: example\nintegrationBranch: main\nskills:\n  - subagent-driven-development\nagents: [implementer]\ndocs: []\ntargets:\n  - claude\n  - pi\n")
	p, err := Open(testContext(t), root)
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

// invariant: config/configuration:targets-default-claude (TestResolveTargetsRejectsUnknown)
func TestResolveTargetsRejectsUnknown(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ntargets:\n  - nope\n")
	if _, err := Open(testContext(t), root); err == nil {
		t.Fatal("expected Open to reject an unknown target name")
	}
}

func TestPlannedOutputsIncludesGeneratedDocs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\nskills: []\nagents: []\ndocs: []\ndomains: [rendering]\n", nil)
	writeADR(t, root, "0001-engine.md", testsupport.ADR("Implemented", testsupport.WithDomains("rendering"), testsupport.WithTitle("0001: Engine")))
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	planned, err := p.PlannedOutputs(testContext(t))
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
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	// Corrupt a sidecar so the RenderAll inside PlannedOutputs fails.
	corruptSidecar(t, root, "skills/tdd.yaml")
	if _, err := p.PlannedOutputs(testContext(t)); err == nil {
		t.Fatal("expected PlannedOutputs to surface the RenderAll error")
	}
}

// invariant: rendering/pi-workflows:pi-implement-role-artifact (TestPiImplementRoleArtifact)
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

// invariant: rendering/pi-workflows:pi-role-contract-loader (TestPiRoleContractLoader)
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

func TestHandoffLifecycleIndependentOfEffortState(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-handoff/index.ts")
	for _, want := range []string{"let pending", "queueCommand(\"awf-handoff-continue\"", "Fresh-session handoff", "parentSession:old", "prepared?.cleanup?.()", "if(pending!==request)", "pending=undefined"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff lifecycle output missing %q", want)
		}
	}
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), "tools/pi-extension-test/tests/handoff.test.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"handoff rejects a continuation whose pending request changes during countdown",
		"wrong continuation token preserves the valid pending request",
		"handoff preserves lineage and does not silently retry",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("TypeScript lifecycle behavior contract missing %q", want)
		}
	}
}

func TestHandoffPublicKickoffContract(t *testing.T) {
	out := renderPiExtensionFile(t, "awf-handoff/index.ts")
	for _, want := range []string{"kickoff:Type.String({maxLength:1000})", "params.kickoff.length>1000", "remote-pi:notification-disposition.v1", "additionalProperties:false"} {
		if !strings.Contains(out, want) {
			t.Errorf("handoff public contract missing %q", want)
		}
	}
	body, err := os.ReadFile(filepath.Join(repoRootDir(t), "tools/pi-extension-test/tests/handoff.test.ts"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"handoff schema exposes only required bounded kickoff prose",
		`"😀".repeat(500)`,
		`"😀".repeat(501)`,
		"handoff marks a successfully queued continuation as non-terminal",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("TypeScript public-contract behavior case missing %q", want)
		}
	}
	for _, banned := range []string{"memoryPath", "validateMemoryPath", "runAwf", "state.json", "assignment", "selected-effort", "telemetry", "adopt"} {
		if strings.Contains(out, banned) {
			t.Errorf("handoff public contract retains %q", banned)
		}
	}
}
