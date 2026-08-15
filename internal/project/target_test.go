package project

import (
	"io/fs"
	"maps"
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
	"github.com/hypnotox/agentic-workflows/templates"
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

// invariant: rendering/pi-workflows:pi-native-workflow-skills (TestPiRuntimeTargetRender)
// invariant: rendering/pi-runtime:pi-extension-target-render (TestPiRuntimeTargetRender)
// invariant: rendering/pi-workflows:using-effort-skill (TestPiRuntimeTargetRender)
func TestPiRuntimeTargetRender(t *testing.T) {
	if _, independentlySelectable := catalog.Standard.Skills["using-effort"]; independentlySelectable {
		t.Fatal("using-effort companion became independently selectable")
	}
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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
	for _, path := range []string{".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/model-routing.ts", ".pi/extensions/awf-effort/index.ts", ".pi/extensions/awf-effort/client.ts"} {
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
	index := extensions[".pi/extensions/awf-subagents/index.ts"]
	routing := extensions[".pi/extensions/awf-subagents/model-routing.ts"]
	for _, want := range []string{"registerSubagentTools(pi", "PROTOCOL_VERSION = 2", "ProfileDefinition[]", "pi-tools:subagent-profiles:request", "pi-tools:subagent-profiles:capability", "pi-tools:subagent-profiles:registration-result", "suppressDefault: true", "awf provides no fallback"} {
		if !strings.Contains(index, want) {
			t.Errorf("Pi profile adapter missing %q", want)
		}
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
}

// invariant: rendering/pi-workflows:pi-effort-memory-tools (TestPiEffortMemoryToolContract)
func TestPiEffortMemoryToolContract(t *testing.T) {
	selected := explorationRenderedByPath(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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
	// The preview and retained-diff surface must be identical in the embedded
	// template and in what the Pi target actually writes, so a drifting or
	// partially rendered extension cannot claim the invariant (ADR-0239 as
	// revised by ADR-0244).
	for _, want := range []string{
		`import { renderDiff } from "@earendil-works/pi-coding-agent";`,
		`import { Box, Container, Spacer, Text } from "@earendil-works/pi-tui";`,
		`renderShell: "self"`,
		"const registerMutation = (spec: MutationSpec)",
		"const previewCall = (toolCallId: string, key: string, cwd: string, invoke: (snapshot: Snapshot) => Promise<MemoryReply>)",
		`if (preview.condition !== "previewed") throw new Error(renderMemoryOutcome(preview.outcome!));`,
		"existing.key === key && existing.cwd === cwd",
		"if (previews.size > PREVIEW_ENTRY_LIMIT) previews.delete(previews.keys().next().value as string);",
		`await previewCall(toolCallId, spec.key(args) ?? "", ctx.cwd, (snapshot) => spec.invoke(snapshot, ctx.cwd, args, true, signal)); return await memoryCall(ctx, signal, (snapshot) => spec.invoke(snapshot, ctx.cwd, args, false, signal), true);`,
		"(preview) => settlePreview(row, key, spec.title, theme, context, () => showMutationDiff(row, preview.diff!))",
		"(error: Error) => settlePreview(row, key, spec.title, theme, context, () => showMutationFailure(row, error.message, theme))",
		"if (row.key !== key || row.authoritative) return; try { apply(); buildMutationRow(row, title, theme); context.invalidate(); } catch",
		`if (row) { row.authoritative = true; const reply = context.isError ? undefined : result.details as MemoryReply | undefined; if (reply && reply.diff) showMutationDiff(row, reply.diff); else showMutationFailure(row, mutationResultText(result), theme);`,
		`const MUTATION_TRUNCATION_NOTICE = "Diff truncated for display.";`,
		`theme.fg("warning", MUTATION_TRUNCATION_NOTICE)`,
		`const text = diff.text.endsWith("\n") ? diff.text.slice(0, -1) : diff.text;`,
		`const body = text === "" ? undefined : renderDiff(text);`,
		"`Replaced ${reply.replacementCount} block(s) in effort memory.`",
		`reply.condition === "updated" ? "Memory metadata updated."`,
		"memoryEdit(memoryExecutor, cwd, snapshot.slug, snapshot.owner, args.edits, signal, { preview })",
		"memoryUpdate(memoryExecutor, cwd, snapshot.slug, snapshot.owner, args, signal, { preview })",
	} {
		for source, label := range map[string]string{index: "rendered effort index", templateSource(t, "pi/awf-effort/index.ts.tmpl"): "effort index template"} {
			if !strings.Contains(source, want) {
				t.Errorf("%s missing mutation-rendering contract %q", label, want)
			}
		}
	}
	for _, want := range []string{
		`"previewed"`,
		`const previewing = operation === "edit-preview" || operation === "update-preview";`,
		`if (condition === "previewed") { if (!previewing) return; const editPreview = operation === "edit-preview";`,
		`exact(reply, ["schemaVersion", "condition", ...(editPreview ? ["replacementCount"] : []), "diff"])`,
		`(editPreview && !integer(reply.replacementCount, 1, 128))`,
		`if (previewing && (condition === "read" || condition === "edited" || condition === "updated")) return;`,
		"function previewing(options: MemoryPreviewOption): boolean",
		`invokeMemory(exec, preview ? "edit-preview" : "edit", ["effort", "memory", "edit", slugValue, ...(preview ? ["--preview"] : []), "--owner", owner, "--json"]`,
		`...(args.next === undefined ? [] : ["--next", args.next]), ...(preview ? ["--preview"] : []), "--owner", owner, "--json"`,
		`invokeMemory(exec, preview ? "update-preview" : "update"`,
	} {
		for source, label := range map[string]string{client: "rendered effort client", templateSource(t, "pi/awf-effort/client.ts.tmpl"): "effort client template"} {
			if !strings.Contains(source, want) {
				t.Errorf("%s missing preview-protocol contract %q", label, want)
			}
		}
	}
	fallback := renderSkillGolden(t, "using-effort", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}})
	assertNoLeaks(t, fallback)
	for _, tid := range []string{"pi/awf-effort/index.ts.tmpl", "pi/awf-effort/client.ts.tmpl"} {
		// renderGolden already rejects unresolved tokens, so an empty-variable render is the assertion.
		renderGolden(t, tid, map[string]any{"prefix": "", "vars": map[string]any{}, "data": map[string]any{}})
	}
	if os.Getenv("AWF_PI_RUNTIME_SMOKE") == "1" {
		assertPiRuntimeSmoke(t)
	}
}

func templateSource(t *testing.T, tid string) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, tid)
	if err != nil {
		t.Fatalf("read template %s: %v", tid, err)
	}
	return string(src)
}

// invariant: rendering/pi-workflows:pi-subagent-model-routing (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-subagent-model-preferences (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-subagent-model-wizard (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-effort-session-association (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-native-workflow-skills (TestPiRealRuntimeSmoke)
// invariant: rendering/project-output-plan:multi-target-render (TestPiRealRuntimeSmoke)
// invariant: rendering/catalog-and-targets:target-dialect-render (TestPiRealRuntimeSmoke)
// removed-invariant (TestPiRealRuntimeSmoke)
// removed-invariant (TestPiRealRuntimeSmoke)
// removed-invariant (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-runtime:pi-minimum-runtime (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-runtime:pi-implementation-state-boundary (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-implement-role-artifact (TestPiRealRuntimeSmoke)
// invariant: rendering/pi-workflows:pi-structured-exploration-contract (TestPiRealRuntimeSmoke)
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
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	var pi *Target
	for i := range p.Targets {
		if p.Targets[i].Name == "pi" {
			pi = &p.Targets[i]
			break
		}
	}
	if pi == nil || len(pi.Outputs) == 0 {
		t.Fatal("full built-in targets missing Pi output declarations")
	}
	original := pi.Outputs[0].TemplateID
	defer func() { pi.Outputs[0].TemplateID = original }()
	pi.Outputs[0].TemplateID = "missing-target-output.tmpl"
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), "missing-target-output") {
		t.Fatalf("RenderAll error = %v, want missing target-output template", err)
	}
}

// invariant: rendering/pi-workflows:pi-dedicated-grounding-dispatch (TestPiStructuredExplorationContractRender)
func TestPiStructuredExplorationContractRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{"ProfileDefinition[]", "subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement", "verificationCheckout: Type.Optional(Type.String())", "Omit verificationCheckout for the project root", "MAX_EXPLORATION_CONCURRENCY = 10", "concurrency: MAX_EXPLORATION_CONCURRENCY", "exclusiveParentBatch: true", "PROTOCOL_VERSION = 2"} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi subagent extension missing %q", want)
		}
	}
	claude := explorationRenderedByPath(t, explorationFixtureConfig("claude"))
	for path, content := range claude {
		if strings.HasPrefix(path, ".claude/skills/") && (strings.Contains(content, "subagent_grounding") || strings.Contains(content, "subagent_explore")) {
			t.Errorf("Claude target %s leaked Pi dispatch tools", path)
		}
	}
	grounding := claude[".claude/skills/example-grounding/SKILL.md"]
	if !strings.Contains(grounding, "`grounding-checker` agent") {
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
		"Omit the model field to use configured or inherited routing.", "const selectModel = async", "session!.modelRegistry", "context.parent?.model ?? session!.model", "ConcreteModel",
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
	if !strings.Contains(body, "Omission is the only default form.") {
		t.Error("profile guidance does not preserve omission as the only default model form")
	}
}

func TestPiSubagentModelPreferencesRender(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts") + renderPiExtensionFile(t, "awf-subagents/model-routing.ts")
	for _, want := range []string{
		`PREFERENCE_TIERS = ["small", "standard", "large"]`,
		`PREFERENCE_FIELDS = ["default", ...PREFERENCE_ROLES, ...PREFERENCE_TIERS]`,
		`type SourceReason = "read-error" | "malformed-json" | "non-object" | "unknown-key"`,
		`type FieldReason = "malformed" | "overlong" | "unregistered" | "unauthenticated" | "unavailable"`,
		"loadPreferenceState(deps,", "session.modelRegistry", "new WeakSet<object>()", "ctx.sessionManager",
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
		"Role defaults:", "Tier mappings:", "Missing:", "Invalid:", "modified concurrently", "mode: 0o600", "loadPreferenceState(deps,", "session.modelRegistry",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("Pi model wizard render missing %q", want)
		}
	}
}

func explorationFixtureConfig(target string) string {
	return "prefix: example\nprofile: full\nintegrationBranch: main\n"
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
				if grounding := skillBody("grounding"); !strings.Contains(grounding, "`grounding-checker` agent") {
					t.Errorf("%s grounding skill does not name the grounding-checker agent", target)
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
	files := explorationRenderedByPath(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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
			"one self-contained task per child", "fan them out as sibling calls", "breadth, detail, and tier independently per child", "large analysis child with small targeted paths or summary children", "at most ten exploration children", "queues the rest FIFO", "refinements that depend on an earlier result stay sequential",
			"targeted < bounded < broad", "targeted locates one declaration, implementation, file, or exact fact", "bounded investigates within a named symbol, package, component, or subsystem", "broad searches across the project search universe, including relevant source, tests, documentation, decisions, and workflow artifacts",
			"adaptive maximum", "cheapest targeted lookup", "widen only when evidence requires it", "never widen beyond the selected maximum", "If the boundary is exhausted, report that explicitly",
			"tracked files plus non-ignored untracked working-tree files under the current repository root", "tracked generated and vendored files", "ignored files", ".git", "nested repositories", "external dependencies unless the task explicitly brings one of those surfaces into scope",
			"paths < summary < analysis", "paths returns only relevant file:line or file:start-end locations with minimal labels and no search narrative", "summary returns grounded locations plus concise explanations of what each contains and why it matters", "analysis directly answers the task with an evidence-grounded synthesis of relationships, call flow, usage patterns, assumptions, and uncertainty",
			"Ground every material claim with file/line evidence", "Not found within <breadth> boundary: <what was searched>", "successful execution", "one concise next refinement", "broad absence report must name the project search universe and searched surfaces", "Distinguish inconclusive and unverified outcomes from absence",
			"new fresh-context call to correct the task, change report detail, or widen breadth",
		}},
		"Pi profile guidance": {prompt, []string{
			"Selected breadth maximum:", "Selected report detail:", "concurrency: MAX_EXPLORATION_CONCURRENCY", "profileDataSchema",
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
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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

// invariant: rendering/catalog-and-targets:built-in-runtime-targets (TestTargetDescriptorCustomization)
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
			Path: ".custom/extension.ts", TemplateID: "pi/awf-subagents/index.ts.tmpl",
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
		"CUSTOM.md":            MarkdownAgentDialect,
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
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n")
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
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n")
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
	root := scaffold(t, sampleYAML)
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	seenTargets := map[string]bool{}
	for _, target := range p.Targets {
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
	for _, target := range p.Targets {
		for name := range catalog.Standard.Skills {
			path := target.SkillPath("example", name)
			content := byPath[path]
			if content == "" || pathCounts[path] != 1 {
				t.Errorf("skill %q for %s rendered %d times with %d bytes", name, target.Name, pathCounts[path], len(content))
			}
		}
		for name := range catalog.Standard.Agents {
			path := target.AgentPath(name)
			content := byPath[path]
			if content == "" || pathCounts[path] != 1 {
				t.Errorf("agent %q for %s rendered %d times with %d bytes", name, target.Name, pathCounts[path], len(content))
				continue
			}
			if err := validateArtifact([]byte(content), target.AgentDialect); err != nil {
				t.Errorf("validate %q as %s: %v", path, target.AgentDialect, err)
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
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
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

const (
	semanticPlanningInstruction    = "- **Semantic rendering review:** plans name concrete examples and expected readings only when load-bearing. The implementation phase owner performs focused generated-prose meaning review, records inspected output boundaries and result in completion evidence, and checks contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders without a universal language validator. Plan review inspects the requirement and any evidence already available; code review inspects the completed implementation evidence."
	semanticPlanReviewInstruction  = "1. **semantic-rendering-review**: inspect the change-specific requirement for focused generated-prose meaning review at affected output boundaries, including contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders such as `<literal-placeholder>`. During precommit plan review, do not require future implementation completion evidence; during a later review, inspect that evidence when it exists. Concrete examples and expected readings are required only when load-bearing; this is not a general output validator."
	semanticCodeReviewInstruction  = "1. **semantic-rendering-review**: for generated prose changes, inspect the requirement and phase completion evidence naming produced-output boundaries and result, including contradictory fragments, concept-preserving paraphrase, and literal-placeholder intent. Keep this as human meaning review, not a general output validator or new deterministic inference."
	semanticImplementerInstruction = "For generated-prose changes, perform the focused meaning review required by the phase at the produced-output boundaries. Check contradictory fragments, concept-preserving paraphrase, and intentional literal placeholders without inventing a universal language validator; retain the inspected boundaries and result as completion evidence for your report."
	semanticInlineInstruction      = "For generated-prose changes, perform the focused meaning review at the produced-output boundaries and retain the inspected boundaries and result as completion evidence."
)

// invariant: rendering/workflow-skill-templates:semantic-rendering-review (TestSemanticRenderingReviewMultiTargetAuthority)
func TestSemanticRenderingReviewMultiTargetAuthority(t *testing.T) {
	root := scaffold(t, "prefix: example\nprofile: full\nintegrationBranch: main\n")
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	counts := map[string]int{}
	for _, file := range files {
		byPath[file.Path] = file.Content
		counts[file.Path]++
	}
	for _, target := range []string{".claude", ".pi"} {
		for _, artifact := range []struct {
			path        string
			instruction string
		}{
			{target + "/skills/example-writing-plans/SKILL.md", semanticPlanningInstruction},
			{target + "/skills/example-executing-plans/SKILL.md", semanticInlineInstruction},
			{target + "/agents/implementer.md", semanticImplementerInstruction},
			{target + "/agents/plan-reviewer.md", semanticPlanReviewInstruction},
			{target + "/agents/code-reviewer.md", semanticCodeReviewInstruction},
		} {
			out := byPath[artifact.path]
			if counts[artifact.path] != 1 || out == "" {
				t.Fatalf("%s rendered %d times with %d bytes", artifact.path, counts[artifact.path], len(out))
			}
			if !strings.Contains(out, artifact.instruction) {
				t.Errorf("%s missing exact semantic rendering instruction %q:\n%s", artifact.path, artifact.instruction, out)
			}
			for _, residue := range []string{"<no value>", "{{ ."} {
				if strings.Contains(out, residue) {
					t.Errorf("%s contains unresolved empty-data residue %q:\n%s", artifact.path, residue, out)
				}
			}
			for _, forbidden := range []string{"synonym detection", "contradiction inference", "placeholder-intent inference", "universal output-language validator"} {
				if strings.Contains(out, forbidden) {
					t.Errorf("%s claims unsupported semantic validation %q:\n%s", artifact.path, forbidden, out)
				}
			}
		}
	}
}

func TestResolveTargetsRejectsUnknown(t *testing.T) {
	root := scaffold(t, "prefix: awf\nintegrationBranch: main\n  - nope\n")
	if _, err := Open(testContext(t), root); err == nil {
		t.Fatal("expected Open to reject an unknown target name")
	}
}

func TestPlannedOutputsIncludesGeneratedDocs(t *testing.T) {
	root := scaffoldFiles(t, "prefix: awf\nintegrationBranch: main\ndomains: [rendering]\n", nil)
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

func TestPiImplementRoleArtifact(t *testing.T) {
	src := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{
		".pi/agents/implementer.md",
		"loadAgentContract",
		"relative: IMPLEMENTER_PATH",
		"Enable the implementer agent and run ./awf render.",
		"has no instruction body; run ./awf render.",
		"parseFrontmatter",
		"const changed = state.before.available && after.available && state.before.head !== after.head",
		"commitVerification: state.before.available && after.available ? \"verified\" : \"unavailable\"",
		"commit-capable but created no commit",
		"committed despite allowCommits=false",
		"resolveVerificationCheckout",
		`requested.startsWith("@") ? requested.slice(1) : requested`,
		`["rev-parse", "--show-toplevel"]`,
		`["rev-parse", "--path-format=absolute", "--git-common-dir"]`,
		`["rev-parse", "--absolute-git-dir"]`,
		`join(canonicalGitDirectory, "gitdir")`,
		"non-symlink regular file",
		"administrative backlink",
		"same repository as the project root",
		"snapshot(pi, verificationCheckout)",
		"retry with verificationCheckout set to that checkout root",
		"cwd: root",
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
	implementer := explorationRenderedByPath(t, explorationFixtureConfig("pi"))[".pi/agents/implementer.md"]
	for _, piOnly := range []string{"verificationCheckout", "verification checkout", "Pi CWD"} {
		if strings.Contains(implementer, piOnly) {
			t.Errorf("generic implementer role gained Pi-only verification metadata %q", piOnly)
		}
	}
}

// invariant: rendering/pi-workflows:pi-role-contract-loader (TestPiRoleContractLoader)
func TestPiRoleContractLoader(t *testing.T) {
	body := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{
		"loadAgentContract", "source: ContractSource",
		"relative: EXPLORER_PATH", "relative: GROUNDING_CHECKER_PATH",
		".pi/agents/explorer.md", ".pi/agents/grounding-checker.md",
		"Enable the explorer agent and run ./awf render.",
		"Enable the grounding-checker agent and run ./awf render.",
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
