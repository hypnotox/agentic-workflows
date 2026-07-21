package project

import (
	"fmt"
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

// invariant: rendering/catalog-and-targets:pi-extension-target-render
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

func registrationBlock(t *testing.T, content, name, nextMarker string) string {
	t.Helper()
	start := strings.Index(content, `name: "`+name+`"`)
	if start < 0 {
		t.Fatalf("cannot find registration %s", name)
	}
	relativeEnd := strings.Index(content[start:], nextMarker)
	if relativeEnd <= 0 {
		t.Fatalf("cannot isolate registration %s before %s", name, nextMarker)
	}
	return content[start : start+relativeEnd]
}

// invariant: rendering/templates:pi-structured-exploration-contract
func TestPiStructuredExplorationContract(t *testing.T) {
	index := renderPiExtensionFile(t, "index.ts")
	runner := renderPiExtensionFile(t, "runner.ts")
	for _, name := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"} {
		if strings.Count(index, `name: "`+name+`"`) != 1 {
			t.Errorf("tool %s registration count differs from one", name)
		}
	}
	if got := strings.Count(index, `name: "subagent_`); got != 4 {
		t.Errorf("public subagent registration count = %d, want 4", got)
	}
	blocks := map[string]string{
		"grounding": registrationBlock(t, index, "subagent_grounding", `name: "subagent_explore"`),
		"explore":   registrationBlock(t, index, "subagent_explore", `name: "subagent_review"`),
		"review":    registrationBlock(t, index, "subagent_review", `name: "subagent_implement"`),
		"implement": registrationBlock(t, index, "subagent_implement", "export default async function"),
	}
	schemas := map[string]string{
		"grounding": `parameters: Type.Object({ task: Type.String({ minLength: 1 }), model: Type.Optional(Type.String()) }, { additionalProperties: false })`,
		"explore": `parameters: Type.Object({
      task: Type.String({ minLength: 1 }),
      breadth: StringEnum(["targeted", "bounded", "broad"] as const),
      detail: StringEnum(["paths", "summary", "analysis"] as const),
      model: Type.Optional(Type.String()),
    }, { additionalProperties: false })`,
		"review": `parameters: Type.Object({
      kind: StringEnum(["adr", "plan", "code"] as const),
      task: Type.String({ minLength: 1 }),
      model: Type.Optional(Type.String()),
    }, { additionalProperties: false })`,
		"implement": `parameters: Type.Object({
      task: Type.String({ minLength: 1 }),
      allowCommits: Type.Boolean(),
      model: Type.Optional(Type.String()),
    }, { additionalProperties: false })`,
	}
	for role, schema := range schemas {
		if strings.Count(blocks[role], "parameters:") != 1 || !strings.Contains(blocks[role], schema) {
			t.Errorf("%s registration does not carry its exact closed schema:\n%s", role, blocks[role])
		}
	}
	for _, want := range []string{
		`EXPLORE_TOOLS = ["read", "grep", "find", "ls", "bash"]`,
		`rolePrompt("explore", { breadth: params.breadth, detail: params.detail })`,
		`MAX_EXPLORATION_CONCURRENCY = 10`, `createLimiter(MAX_EXPLORATION_CONCURRENCY)`,
		`const metadata = executionMetadata(selected, pi.getThinkingLevel() as ThinkingLevel, { breadth: params.breadth, detail: params.detail });`,
		`publishState(onUpdate, "explore", params.task, "queued", metadata);`,
		`const release = await explorationLimiter.acquire(signal);`,
		`return toolResult("explore", params.task, await run("explore", params.task, EXPLORE_TOOLS, rolePrompt("explore", { breadth: params.breadth, detail: params.detail }), selected.model, metadata, signal, onUpdate), metadata);`,
		`thinkingLevel: metadata.thinkingLevel`,
		`content: [{ type: "text", text: "(running...)" }]`,
		`events: update.events`, `result.output`,
		"Return only the relevant final report, never the search narrative or intermediate activity.",
		"MAX_TASK_PREVIEW_BYTES = 512", "MAX_FALLBACK_BYTES = 2 * 1024",
	} {
		if !strings.Contains(index, want) {
			t.Errorf("Pi extension index missing exploration boundary %q", want)
		}
	}
	for _, absent := range []string{"appendEntry", "appendMessage", "sendMessage", "rawTranscript"} {
		if strings.Contains(index+runner, absent) {
			t.Errorf("Pi extension contains forbidden parent-context channel %q", absent)
		}
	}
	for _, want := range []string{
		`"--mode", "json", "-p", "--no-session"`,
		"\"--model\", `${request.model.provider}/${request.model.id}`",
		`"--thinking", request.thinkingLevel`, `"--tools", request.tools.join(",")`,
		`spawn(invocation.command, args, { cwd: request.cwd, shell: false`,
		"MAX_OUTPUT_BYTES = 50 * 1024", "MAX_OUTPUT_LINES = 2000", "MAX_STDERR_BYTES = 50 * 1024",
		"MAX_DISPLAY_EVENTS = 20", "MAX_DISPLAY_EVENT_BYTES = 2 * 1024", "MAX_FAILURE_BYTES = 2 * 1024",
		`output: truncateOutput(output || "(no output)")`, `stderr: truncateStderr(stderr)`,
	} {
		if !strings.Contains(runner, want) {
			t.Errorf("Pi runner missing exploration process or bound %q", want)
		}
	}
}

// invariant: rendering/templates:pi-subagent-model-routing
func TestPiSubagentModelRouting(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{
		`model: Type.Optional(Type.String())`, `requested.indexOf("/")`,
		`ctx.modelRegistry.find(provider, id)`, `ctx.modelRegistry.hasConfiguredAuth(found)`,
		`return { model: { provider: found.provider, id: found.id }, requested }`,
		`return { model: { provider: ctx.model.provider, id: ctx.model.id }, requested: undefined }`,
		`executionMetadata(selected, pi.getThinkingLevel() as ThinkingLevel`, `resolvedModel:`, `modelSource:`,
		`thinkingLevel: metadata.thinkingLevel`, `usage: result.usage, model: result.model`,
		`modelChanged: result.modelChanged`,
		`modelMismatch: Boolean(result.model && result.model !== details.resolvedModel)`,
		`latestCacheHitRate: result.latestCacheHitRate`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("Pi extension missing model-routing contract %q", want)
		}
	}
	if strings.Contains(content, "fuzzy") || strings.Contains(content, "fallbackModel") {
		t.Fatal("Pi model routing contains a silent fallback path")
	}
	if got := strings.Count(content, `model: Type.Optional(Type.String())`); got != 4 {
		t.Fatalf("optional model schema count = %d, want 4", got)
	}

	blocks := map[string]string{
		"grounding": registrationBlock(t, content, "subagent_grounding", `name: "subagent_explore"`),
		"explore":   registrationBlock(t, content, "subagent_explore", `name: "subagent_review"`),
		"review":    registrationBlock(t, content, "subagent_review", `name: "subagent_implement"`),
		"implement": registrationBlock(t, content, "subagent_implement", "export default async function"),
	}
	contracts := map[string]struct {
		sideEffects []string
		finalPath   []string
	}{
		"grounding": {
			[]string{`await run("grounding"`},
			[]string{`return toolResult("grounding", params.task, await run("grounding", params.task, GROUNDING_TOOLS, rolePrompt("grounding"), selected.model, metadata, signal, onUpdate), metadata);`},
		},
		"explore": {
			[]string{`explorationLimiter.acquire(signal)`, `await run("explore"`},
			[]string{`return toolResult("explore", params.task, await run("explore", params.task, EXPLORE_TOOLS, rolePrompt("explore", { breadth: params.breadth, detail: params.detail }), selected.model, metadata, signal, onUpdate), metadata);`},
		},
		"review": {
			[]string{`loadReviewer(deps, root, params.kind)`, `await run("review"`},
			[]string{`return toolResult("review", params.task, await run("review", params.task, REVIEW_TOOLS, prompt, selected.model, metadata, signal, onUpdate), { ...metadata, kind: params.kind });`},
		},
		"implement": {
			[]string{`implementationTail = new Promise`, `snapshot(pi, root)`, `await run("implement"`},
			[]string{
				`const result = await run("implement", params.task, IMPLEMENT_TOOLS, rolePrompt("implement", { allowCommits: params.allowCommits }), selected.model, metadata, signal, onUpdate);`,
				`const gitDetails = { ...metadata, allowCommits: params.allowCommits, before, after, commitVerification: before.available && after.available ? "verified" : "unavailable" };`,
				`return toolResult("implement", params.task, result, gitDetails);`,
			},
		},
	}
	for role, contract := range contracts {
		block := blocks[role]
		resolvedAt := strings.Index(block, `const selected = resolveChildModel(ctx, params.model)`)
		if resolvedAt < 0 {
			t.Errorf("%s registration does not resolve its model", role)
			continue
		}
		for _, sideEffect := range contract.sideEffects {
			at := strings.Index(block, sideEffect)
			if at < 0 || at < resolvedAt {
				t.Errorf("%s registration does not resolve before side effect %q", role, sideEffect)
			}
		}
		for _, finalPath := range contract.finalPath {
			if !strings.Contains(block, finalPath) {
				t.Errorf("%s registration missing exact final model-result path %q", role, finalPath)
			}
		}
	}
	implement := blocks["implement"]
	for _, want := range []string{
		`const failure = {`, `...result,`, `failed: true,`,
		`failureMessage: ` + "`Implementation committed despite allowCommits=false (HEAD ${before.head} -> ${after.head}); changes were not reverted.`",
		`return toolResult("implement", params.task, failure, gitDetails);`,
	} {
		if !strings.Contains(implement, want) {
			t.Errorf("implementation registration does not preserve the full child result on commit-policy failure %q", want)
		}
	}
}

func explorationFixtureConfig(target string) string {
	return "prefix: example\nskills: [adr-lifecycle, brainstorming, debugging, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [" + target + "]\n"
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

// invariant: rendering/templates:cross-runtime-exploration-dispatch
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
			exploring := files[base+"exploring/SKILL.md"]
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
				body := files[base+skill+"/SKILL.md"]
				if target == "pi" {
					if !strings.Contains(body, "provider/model-id") || !strings.Contains(body, "inherits the parent") {
						t.Errorf("Pi/%s missing optional model guidance", skill)
					}
				} else if strings.Contains(body, "provider/model-id") || strings.Contains(body, "subagent_") {
					t.Errorf("%s/%s leaks Pi model or tool syntax", target, skill)
				}
			}
			for _, consumer := range []string{"brainstorming", "debugging", "refactor-coupling-audit"} {
				body := files[base+consumer+"/SKILL.md"]
				for _, want := range []string{"location is unknown", "and inline search would pollute the parent context", "exact-known-file", "genuinely trivial"} {
					if !strings.Contains(body, want) {
						t.Errorf("%s/%s missing dispatch condition %q", target, consumer, want)
					}
				}
			}
		})
	}
}

// invariant: rendering/templates:bounded-exploration-reporting
func TestBoundedExplorationReporting(t *testing.T) {
	files := explorationRenderedByPath(t, "prefix: example\nskills: [exploring]\nagents: []\ntargets: [pi]\n")
	guidance := files[".pi/skills/example-exploring/SKILL.md"]
	prompt := renderPiExtensionFile(t, "index.ts")
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

// invariant: rendering/templates:pi-subagent-progress-context-isolation
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

// invariant: rendering/templates:pi-subagent-progress-rendering
func TestPiSubagentProgressRendering(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, role := range []string{"grounding", "explore", "review", "implement"} {
		if strings.Count(content, `...renderers("`+role+`")`) != 1 {
			t.Errorf("role %s does not delegate both render hooks", role)
		}
	}
	for _, want := range []string{
		`renderCall(args:`, `renderResult(result:`, `keyHint("app.tools.expand"`, "COLLAPSED_EVENT_COUNT",
		`"queued" | "running"`, `resolvedModel:`, `thinkingLevel:`, `latestCacheHitRate:`,
		`CH${latestCacheHitRate.toFixed(1)}%`, `Input: ${details.usage.input}`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing rendering contract %q", want)
		}
	}
}

// invariant: rendering/templates:pi-subagent-failure-details
func TestPiSubagentFailureDetails(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{
		`awfFailure: true`, `pi.on("tool_result"`, `SUBAGENT_TOOL_NAMES.has(event.toolName)`, `return { isError: true }`,
		`return toolResult("implement", params.task, failure, gitDetails);`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing failure-details contract %q", want)
		}
	}
}

// invariant: rendering/templates:pi-subagent-progress-bounds
func TestPiSubagentProgressBounds(t *testing.T) {
	content := renderPiExtensionFile(t, "runner.ts") + renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{
		"MAX_DISPLAY_EVENTS = 20", "MAX_DISPLAY_EVENT_BYTES = 2 * 1024", "omittedEvents",
		`Buffer.byteLength(JSON.stringify(fitted), "utf8")`, "MAX_TASK_PREVIEW_BYTES", "MAX_FALLBACK_BYTES",
		`usage: { ...usage }`, `latestCacheHitRate`, `modelChanged`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing progress bound %q", want)
		}
	}
	if strings.Contains(content, "rawTranscript") {
		t.Fatal("extension retains a raw transcript")
	}
}

// invariant: rendering/catalog-and-targets:pi-child-tool-boundaries
func TestPiSubagentToolBoundaries(t *testing.T) {
	index := renderPiExtensionFile(t, "index.ts")
	runner := renderPiExtensionFile(t, "runner.ts")
	allowlists := map[string]string{
		"EXPLORE_TOOLS":   `export const EXPLORE_TOOLS = ["read", "grep", "find", "ls", "bash"] as const;`,
		"GROUNDING_TOOLS": `export const GROUNDING_TOOLS = EXPLORE_TOOLS;`,
		"REVIEW_TOOLS":    `export const REVIEW_TOOLS = ["read", "grep", "find", "ls", "bash"] as const;`,
		"IMPLEMENT_TOOLS": `export const IMPLEMENT_TOOLS = ["read", "bash", "edit", "write", "grep", "find", "ls"] as const;`,
	}
	declarations := make(map[string]string, len(allowlists))
	for _, line := range strings.Split(index, "\n") {
		for name := range allowlists {
			if strings.HasPrefix(line, "export const "+name+" = ") {
				declarations[name] = line
			}
		}
	}
	for name, want := range allowlists {
		if declarations[name] != want {
			t.Errorf("%s declaration = %q, want exact role allowlist %q", name, declarations[name], want)
		}
	}
	roleTools := map[string]string{
		"subagent_grounding": "GROUNDING_TOOLS", "subagent_explore": "EXPLORE_TOOLS",
		"subagent_review": "REVIEW_TOOLS", "subagent_implement": "IMPLEMENT_TOOLS",
	}
	for role, tools := range roleTools {
		block := registrationBlock(t, index, role, map[string]string{
			"subagent_grounding": `name: "subagent_explore"`, "subagent_explore": `name: "subagent_review"`,
			"subagent_review": `name: "subagent_implement"`, "subagent_implement": "export default async function",
		}[role])
		if !strings.Contains(block, tools) {
			t.Errorf("%s does not pass exact %s allowlist", role, tools)
		}
	}
	for name, declaration := range declarations {
		for _, extensionTool := range []string{"subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"} {
			if strings.Contains(declaration, extensionTool) {
				t.Errorf("%s declaration recursively includes %s", name, extensionTool)
			}
		}
	}
	for _, want := range []string{
		`return { model: { provider: ctx.model.provider, id: ctx.model.id }, requested: undefined }`,
		`return { model: { provider: found.provider, id: found.id }, requested }`,
		`thinkingLevel: metadata.thinkingLevel`,
		`const MAX_TASK_PREVIEW_BYTES = 512;`,
		`const MAX_FALLBACK_BYTES = 2 * 1024;`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("extension missing boundary %q", want)
		}
	}
	for _, want := range []string{
		`export const MAX_OUTPUT_BYTES = 50 * 1024;`,
		`export const MAX_OUTPUT_LINES = 2000;`,
		`export const MAX_STDERR_BYTES = 50 * 1024;`,
		`export const MAX_DISPLAY_EVENTS = 20;`,
		`export const MAX_DISPLAY_EVENT_BYTES = 2 * 1024;`,
		`export const MAX_FAILURE_BYTES = 2 * 1024;`,
		`const lineLimited = lines.length > MAX_OUTPUT_LINES ? lines.slice(0, MAX_OUTPUT_LINES).join("\n") : value`,
		`const content = utf8Prefix(lineLimited, MAX_OUTPUT_BYTES)`,
		`[Output truncated: ${omittedBytes} bytes and ${omittedLines} lines omitted]`,
		`[stderr truncated: ${total - Buffer.byteLength(value.slice(start), "utf8")} bytes omitted]`,
		`truncateField(value, MAX_FAILURE_BYTES, "\n[failure truncated]")`,
		`const marker = "\n[event truncated]"`,
		`omittedEvents += events.length - MAX_DISPLAY_EVENTS`,
		`events.splice(0, events.length - MAX_DISPLAY_EVENTS)`,
		`output: truncateOutput(output || "(no output)")`,
		`stderr: truncateStderr(stderr)`,
		`failureMessage: failureMessage ? truncateFailure(failureMessage) : undefined`,
	} {
		if !strings.Contains(runner, want) {
			t.Errorf("runner missing concrete retained-output diagnostic path %q", want)
		}
	}
}

// invariant: rendering/templates:pi-implementation-batch-exclusivity
func TestPiImplementationBatchExclusivity(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{
		`pi.on("tool_call"`, `ctx.sessionManager.getLeafEntry()`,
		`leaf.message?.role === "assistant"`, `Array.isArray(leaf.message.content)`,
		`call.id === event.toolCallId && call.name === event.toolName`,
		`calls.length > 1 && calls.some((call: any) => call.name === "subagent_implement")`,
		"A batch containing subagent_implement cannot contain siblings; retry subagent_implement alone.",
		"Cannot verify the current tool batch; retry subagent_implement alone.",
		`return event.toolName === "subagent_implement"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing implementation-batch contract %q", want)
		}
	}
}

// invariant: rendering/catalog-and-targets:pi-child-process-safety
func TestPiSubagentProcessSafetyContract(t *testing.T) {
	content := renderPiExtensionFile(t, "runner.ts")
	for _, want := range []string{"SIGTERM", "SIGKILL", "removeEventListener", "await deps.rm"} {
		if !strings.Contains(content, want) {
			t.Errorf("runner missing process-safety contract %q", want)
		}
	}
}

// invariant: rendering/catalog-and-targets:pi-implementation-state-boundary
func TestPiImplementationStateBoundary(t *testing.T) {
	content := renderPiExtensionFile(t, "index.ts")
	for _, want := range []string{"implementationTail", "allowCommits=false", "changes were not reverted", "commitVerification"} {
		if !strings.Contains(content, want) {
			t.Errorf("extension missing implementation-state contract %q", want)
		}
	}
}

// invariant: rendering/catalog-and-targets:pi-minimum-runtime
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

// invariant: rendering/templates:pi-dedicated-grounding-dispatch
func TestPiDedicatedGroundingDispatch(t *testing.T) {
	config := "prefix: example\nskills: [adr-lifecycle, brainstorming, bugfix, debugging, executing-plans, exploring, proposing-adr, refactor-coupling-audit, retrospective, reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync, subagent-driven-development, tdd, writing-plans]\nagents: [adr-reviewer, code-reviewer, plan-reviewer]\ntargets: [%s]\n"
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
			if strings.Contains(audit, "`subagent_explore`") {
				t.Error("Pi coupling audit bypasses the exploring skill")
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
