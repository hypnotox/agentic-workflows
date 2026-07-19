import assert from "node:assert/strict";
import test from "node:test";
import defaultExtension, {
  EXPLORE_TOOLS,
  GROUNDING_TOOLS,
  IMPLEMENT_TOOLS,
  MIN_PI_VERSION,
  REVIEWER_PATHS,
  REVIEW_TOOLS,
  registerSubagentTools,
  versionSupported,
  type ExtensionDependencies,
} from "../../../.pi/extensions/awf-subagents/index.ts";
import type { RunRequest, RunResult } from "../../../.pi/extensions/awf-subagents/runner.ts";
import { initTheme } from "@earendil-works/pi-coding-agent";
import { visibleWidth } from "@earendil-works/pi-tui";
import { Value } from "typebox/value";

initTheme("dark", false);

const event = { sequence: 1, kind: "assistant" as const, text: "working" };
const result: RunResult = {
  output: "child output", stderr: "", events: [event], omittedEvents: 0, failed: false,
  usage: { input: 1, output: 2, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 1 },
};

function harness(options: { version?: string; reviewer?: string; git?: Array<{ code: number; stdout: string }> } = {}) {
  const tools = new Map<string, any>();
  const handlers = new Map<string, any>();
  const requests: RunRequest[] = [];
  const notifications: unknown[][] = [];
  const git = [...(options.git ?? [])];
  const pi: any = {
    registerTool: (tool: any) => tools.set(tool.name, tool),
    on: (name: string, handler: any) => handlers.set(name, handler),
    getThinkingLevel: () => "high",
    exec: async () => git.shift() ?? { code: 1, stdout: "", stderr: "" },
  };
  const deps: ExtensionDependencies = {
    readFile: async () => options.reviewer ?? "---\nname: reviewer\ndescription: test\n---\nReview carefully.",
    runner: { run: async (request) => { requests.push(request); request.onUpdate?.({ events: [event], omittedEvents: 0 }); return result; } },
    packageVersion: options.version ?? MIN_PI_VERSION,
    extensionFile: "/repo/.pi/extensions/awf-subagents/index.ts",
  };
  registerSubagentTools(pi, deps);
  const ctx: any = { cwd: "/repo/subdirectory", model: { provider: "test", id: "parent" }, ui: { notify: (...args: unknown[]) => notifications.push(args) } };
  return { pi, deps, tools, handlers, requests, notifications, ctx };
}

async function execute(tool: any, params: any, ctx: any, signal?: AbortSignal) {
  const updates: any[] = [];
  const value = await tool.execute("id", params, signal, (update: unknown) => updates.push(update), ctx);
  return { value, updates };
}

test("default factory registers against the installed minimum Pi package", async () => {
  const h = harness();
  const freshTools = new Map<string, any>();
  h.pi.registerTool = (tool: any) => freshTools.set(tool.name, tool);
  await defaultExtension(h.pi);
  assert.deepEqual([...freshTools.keys()], ["subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"]);
});

test("version support is strict and ordered", () => {
  assert.equal(versionSupported("0.80.9"), true);
  assert.equal(versionSupported("0.80.9-beta.1"), true);
  assert.equal(versionSupported("0.80.10"), true);
  assert.equal(versionSupported("0.81.0-beta.1"), true);
  assert.equal(versionSupported("1.0.0"), true);
  assert.equal(versionSupported("0.80.8"), false);
  assert.equal(versionSupported("0.79.99"), false);
  assert.equal(versionSupported("invalid"), false);
});

test("unsupported version registers one startup notification and no tools", async () => {
  for (const version of ["0.80.8", "unknown"]) {
    const h = harness({ version });
    assert.equal(h.tools.size, 0);
    assert.deepEqual([...h.handlers.keys()], ["session_start"]);
    await h.handlers.get("session_start")({}, h.ctx);
    assert.deepEqual(h.notifications, [[`awf Pi subagents require Pi 0.80.9 or newer; found ${version}. Upgrade Pi and reload.`, "error"]]);
  }
});

test("registers exactly four governed public tools with structured exploration schema", () => {
  const h = harness();
  assert.deepEqual([...h.tools.keys()], ["subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"]);
  assert.deepEqual(EXPLORE_TOOLS, ["read", "grep", "find", "ls", "bash"]);
  assert.deepEqual(GROUNDING_TOOLS, EXPLORE_TOOLS);
  const schema = h.tools.get("subagent_grounding").parameters;
  assert.equal(Value.Check(schema, { task: "ground" }), true);
  assert.equal(Value.Check(schema, {}), false);
  assert.equal(Value.Check(schema, { task: "" }), false);
  assert.equal(Value.Check(schema, { task: "ground", extra: true }), false);
  assert.equal(schema.properties.task.minLength, 1);
  assert.equal(schema.additionalProperties, false);
  const exploreSchema = h.tools.get("subagent_explore").parameters;
  for (const breadth of ["targeted", "bounded", "broad"])
    for (const detail of ["paths", "summary", "analysis"])
      assert.equal(Value.Check(exploreSchema, { task: "inspect", breadth, detail }), true);
  for (const invalid of [
    {}, { task: "inspect" }, { task: "", breadth: "targeted", detail: "paths" },
    { task: "inspect", breadth: "targeted" }, { task: "inspect", detail: "paths" },
    { task: "inspect", breadth: "unbounded", detail: "paths" },
    { task: "inspect", breadth: "targeted", detail: "verbose" },
    { task: "inspect", breadth: "targeted", detail: "paths", extra: true },
  ]) assert.equal(Value.Check(exploreSchema, invalid), false);
  assert.deepEqual(exploreSchema.required, ["task", "breadth", "detail"]);
  assert.equal(exploreSchema.additionalProperties, false);
  assert.deepEqual(REVIEW_TOOLS, EXPLORE_TOOLS);
  assert.deepEqual(IMPLEMENT_TOOLS, ["read", "bash", "edit", "write", "grep", "find", "ls"]);
  assert.deepEqual(REVIEWER_PATHS, { adr: ".pi/skills/adr-reviewer.md", plan: ".pi/skills/plan-reviewer.md", code: ".pi/skills/code-reviewer.md" });
});

test("grounding uses the fixed read-only confidence-classified role", async () => {
  const h = harness();
  const { value } = await execute(h.tools.get("subagent_grounding"), { task: "ground design" }, h.ctx);
  assert.equal(value.details.role, "grounding");
  assert.deepEqual(h.requests[0].tools, GROUNDING_TOOLS);
  const prompt = h.requests[0].systemPrompt;
  for (const phrase of [
    "do not edit files or commit", "factual premises", "unstated assumptions and edge cases",
    "ADR, a plan, or narrower scope", "ADR and invariant fit", "open-question | possible-issue",
    "topic, detail, grounding", "verified | interpreted | unverified",
    "verified means mechanically confirmed", "interpreted means the reading requires judgment",
    "unverified means the claim could not be confirmed",
  ]) assert.match(prompt, new RegExp(phrase.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  for (const request of h.requests) for (const tool of request.tools) assert.equal(tool.startsWith("subagent_"), false);
});

const theme: any = {
  fg: (_color: string, text: string) => text,
  bold: (text: string) => text,
};

function rendered(component: any, width: number): string[] {
  const lines = component.render(width);
  for (const line of lines) assert.ok(visibleWidth(line) <= width, `${visibleWidth(line)} > ${width}: ${line}`);
  return lines;
}

test("all subagent renderers cover calls and bounded collapsed states", () => {
  const h = harness();
  const events = Array.from({ length: 12 }, (_, index) => index % 3 === 0
    ? { sequence: index + 1, kind: "assistant", text: `assistant ${index}` }
    : index % 3 === 1
      ? { sequence: index + 1, kind: "tool-start", toolCallId: `id-${index}`, toolName: `read-${index}`, argsPreview: "{}" }
      : { sequence: index + 1, kind: "tool-end", toolCallId: `id-${index}`, toolName: `read-${index}`, isError: index === 2 });
  const usage = { input: 1, output: 2, cacheRead: 3, cacheWrite: 4, cost: 0.25, turns: 2 };
  for (const [name, role] of [["subagent_grounding", "grounding"], ["subagent_explore", "explore"], ["subagent_review", "review"], ["subagent_implement", "implement"]]) {
    const tool = h.tools.get(name);
    const call = rendered(tool.renderCall({ task: "x".repeat(1000) }, theme, {}), 24).join("\n");
    assert.match(call, new RegExp(`${role} subagent`));
    assert.match(call, /task\s+truncated/);
    const collapsed = rendered(tool.renderResult({
      content: [{ type: "text", text: "report" }],
      details: { role, task: "task", state: "completed", events, omittedEvents: 7, usage, model: "child" },
    }, { expanded: false, isPartial: false }, theme, {}), 24).join("\n");
    assert.match(collapsed, /completed/);
    assert.match(collapsed, /earlier retained\s+events/);
    assert.match(collapsed, /7 older events omitted/);
    const orderedActivity = [
      collapsed.indexOf("7 older events omitted"),
      collapsed.search(/earlier retained\s+events/),
      collapsed.indexOf("<- read-2 error"),
      collapsed.indexOf("assistant 3"),
      collapsed.indexOf("-> read-4 {}"),
      collapsed.indexOf("<- read-5 ok"),
      collapsed.indexOf("assistant 6"),
      collapsed.indexOf("-> read-7 {}"),
      collapsed.indexOf("<- read-8 ok"),
      collapsed.indexOf("assistant 9"),
      collapsed.indexOf("-> read-10 {}"),
      collapsed.indexOf("<- read-11 ok"),
    ];
    assert.ok(orderedActivity.every((position, index) => position >= 0 && (index === 0 || orderedActivity[index - 1] < position)), "collapsed activity is not chronological");
    assert.match(collapsed, /2 turns/);
    assert.match(collapsed, /to expand/);
  }
  const emptyCall = rendered(h.tools.get("subagent_explore").renderCall({}, theme, {}), 120).join("\n");
  assert.match(emptyCall, /no task/);
  rendered(h.tools.get("subagent_explore").renderCall({ task: "é".repeat(1000) }, theme, {}), 24);
});

test("renderer covers expanded, partial, failure, abort, and fallback states", () => {
  const h = harness();
  const tool = h.tools.get("subagent_explore");
  const events = [
    { sequence: 1, kind: "tool-start", toolCallId: "id", toolName: "read", argsPreview: "{}" },
    { sequence: 2, kind: "tool-end", toolCallId: "id", toolName: "read", isError: false },
    { sequence: 3, kind: "assistant", text: "assistant output" },
  ];
  const usage = { input: 1, output: 2, cacheRead: 3, cacheWrite: 4, cost: 0.25, turns: 1 };
  const complete = rendered(tool.renderResult({
    content: [{ type: "text", text: "# Final report" }],
    details: { role: "explore", task: "inspect", state: "completed", events, omittedEvents: 0, stderr: "warning", usage, model: "child" },
  }, { expanded: true, isPartial: false }, theme, {}), 24).join("\n");
  for (const phrase of ["Task", "inspect", "Activity", "Report", "Final report", "Diagnostics", "warning", "1 turn", "child"]) assert.match(complete, new RegExp(phrase));

  const partial = rendered(tool.renderResult({ content: [{ type: "text", text: "(running...)" }], details: { role: "explore", task: "", state: "running", events: [], omittedEvents: 0 } }, { expanded: true, isPartial: true }, theme, {}), 120).join("\n");
  assert.match(partial, /running/);
  assert.match(partial, /no task/);
  assert.match(partial, /no activity/);
  assert.doesNotMatch(partial, /Report/);

  for (const state of ["failed", "aborted"]) {
    const text = rendered(tool.renderResult({ content: [{ type: "text", text: `${state} reason` }], details: { role: "explore", task: "task", state, events: [], omittedEvents: 0 } }, { expanded: true, isPartial: false }, theme, {}), 120).join("\n");
    assert.match(text, new RegExp(state));
    assert.match(text, /Failure/);
  }

  const fallback = rendered(tool.renderResult({ content: [{ type: "text", text: "z".repeat(4000) }] }, { expanded: false, isPartial: false }, theme, {}), 24).join("\n");
  assert.match(fallback, /display\s+truncated/);
  const missing = rendered(tool.renderResult({ content: [] }, { expanded: true, isPartial: false }, theme, {}), 120).join("\n");
  assert.match(missing, /no output/);
  rendered(tool.renderResult({}, { expanded: false, isPartial: false }, theme, {}), 120);
  rendered(tool.renderResult({ content: [{ type: "image" }], details: { events: "invalid" } }, { expanded: false, isPartial: false }, theme, {}), 120);

  const runningCollapsed = rendered(tool.renderResult({ content: [], details: { role: "explore", task: "task", state: "completed", events: [], omittedEvents: 0, usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 0 }, model: "child" } }, { expanded: false, isPartial: true }, theme, {}), 120).join("\n");
  assert.match(runningCollapsed, /running/);
  assert.match(runningCollapsed, /no omitted events/);
  assert.match(runningCollapsed, /child/);
  assert.doesNotMatch(runningCollapsed, /to expand/);
  const noHint = rendered(tool.renderResult({ content: [], details: { role: "explore", task: "", state: "completed", events: [], omittedEvents: 0 } }, { expanded: false, isPartial: false }, theme, {}), 120).join("\n");
  assert.doesNotMatch(noHint, /to expand/);
});

test("exploration forwards every breadth and detail into the fixed prompt", async () => {
  for (const breadth of ["targeted", "bounded", "broad"])
    for (const detail of ["paths", "summary", "analysis"]) {
      const h = harness();
      await execute(h.tools.get("subagent_explore"), { task: "inspect", breadth, detail }, h.ctx);
      assert.match(h.requests[0].systemPrompt, new RegExp(`Selected breadth maximum: ${breadth}`));
      assert.match(h.requests[0].systemPrompt, new RegExp(`Selected report detail: ${detail}`));
    }
});

test("explore isolates partial activity from final content", async () => {
  const h = harness();
  const { value, updates } = await execute(h.tools.get("subagent_explore"), { task: "inspect", breadth: "bounded", detail: "summary" }, h.ctx);
  assert.equal(value.content[0].text, "child output");
  assert.deepEqual(value.details.events, [event]);
  assert.equal(value.content[0].text.includes("working"), false);
  assert.equal(updates[0].content[0].text, "(running...)");
  assert.deepEqual(updates[0].details.events, [event]);
  assert.deepEqual(h.requests[0].model, { provider: "test", id: "parent" });
  assert.equal(h.requests[0].cwd, "/repo");
  assert.equal(h.requests[0].thinkingLevel, "high");
  assert.deepEqual(h.requests[0].tools, EXPLORE_TOOLS);
  const prompt = h.requests[0].systemPrompt;
  for (const phrase of [
    "adaptive maximum", "targeted locates one declaration", "bounded investigates within a named symbol", "broad searches across the project search universe",
    "tracked files plus non-ignored untracked working-tree files", "ignored files", ".git", "nested repositories", "external dependencies",
    "Not found within <breadth> boundary:", "successful execution", "project search universe", "searched surfaces", "inconclusive", "unverified", "insufficient",
    "evidence-grounded", "one information need", "paths", "summary", "analysis", "do not edit files or commit", "do not recursively delegate",
    "a new fresh-context call", "corrects the task", "changes report detail", "widens breadth",
  ]) assert.ok(prompt.includes(phrase), `prompt missing ${phrase}`);
  await assert.rejects(execute(h.tools.get("subagent_explore"), { task: " ", breadth: "bounded", detail: "summary" }, h.ctx), /non-empty/);
  await assert.rejects(execute(h.tools.get("subagent_explore"), { task: "x", breadth: "bounded", detail: "summary" }, { ...h.ctx, model: undefined }), /active parent model/);
  await h.tools.get("subagent_explore").execute("id", { task: "without update", breadth: "bounded", detail: "summary" }, undefined, undefined, h.ctx);
});

test("result middleware marks only failed awf subagents", async () => {
  const h = harness();
  const handler = h.handlers.get("tool_result");
  assert.deepEqual(await handler({ toolName: "subagent_explore", details: { awfFailure: true } }), { isError: true });
  assert.equal(await handler({ toolName: "subagent_explore", details: {} }), undefined);
  assert.equal(await handler({ toolName: "other", details: { awfFailure: true } }), undefined);
  assert.equal(await handler({ toolName: "subagent_review", details: undefined }), undefined);
});

test("failed runner results preserve details", async () => {
  const h = harness();
  h.deps.runner = { run: async () => ({ ...result, failed: true, failureMessage: "bounded failure", stopReason: "aborted", stderr: "diagnostic" }) };
  const { value } = await execute(h.tools.get("subagent_explore"), { task: "fail", breadth: "bounded", detail: "summary" }, h.ctx);
  assert.equal(value.content[0].text, "bounded failure");
  assert.equal(value.details.state, "aborted");
  assert.equal(value.details.stderr, "diagnostic");
  assert.equal(value.details.awfFailure, true);

  const fallback = harness();
  fallback.deps.runner = { run: async () => ({ ...result, failed: true, failureMessage: undefined, stopReason: "error" }) };
  const failed = await execute(fallback.tools.get("subagent_explore"), { task: "fail", breadth: "bounded", detail: "summary" }, fallback.ctx);
  assert.equal(failed.value.content[0].text, "Subagent failed");
  assert.equal(failed.value.details.state, "failed");
});

test("review maps all kinds and reports missing or empty reviewer files", async () => {
  for (const kind of ["adr", "plan", "code"] as const) {
    const h = harness();
    const { value } = await execute(h.tools.get("subagent_review"), { kind, task: "review" }, h.ctx);
    assert.equal(value.details.kind, kind);
    assert.match(h.requests[0].systemPrompt, new RegExp(`governed ${kind} reviewer`));
    assert.deepEqual(h.requests[0].tools, REVIEW_TOOLS);
  }
  const missing = harness();
  missing.deps.readFile = async () => { throw new Error("missing"); };
  await assert.rejects(execute(missing.tools.get("subagent_review"), { kind: "adr", task: "x" }, missing.ctx), /Enable the matching adr-reviewer agent and run awf sync/);
  const empty = harness({ reviewer: "---\nname: x\ndescription: x\n---\n" });
  await assert.rejects(execute(empty.tools.get("subagent_review"), { kind: "plan", task: "x" }, empty.ctx), /no instruction body/);
});

test("implementation reports git state and commit policy", async () => {
  const clean = harness({ git: [
    { code: 0, stdout: "aaa\n" }, { code: 0, stdout: "" },
    { code: 0, stdout: "aaa\n" }, { code: 0, stdout: " M file\n" },
  ] });
  const activeSignal = new AbortController();
  const { value } = await execute(clean.tools.get("subagent_implement"), { task: "change", allowCommits: false }, clean.ctx, activeSignal.signal);
  assert.equal(value.details.commitVerification, "verified");
  assert.equal(value.details.after.status, " M file\n");
  assert.deepEqual(clean.requests[0].tools, IMPLEMENT_TOOLS);
  assert.match(clean.requests[0].systemPrompt, /forbidden/);

  const allowed = harness({ git: [
    { code: 0, stdout: "aaa\n" }, { code: 0, stdout: "" },
    { code: 0, stdout: "bbb\n" }, { code: 0, stdout: "" },
  ] });
  await execute(allowed.tools.get("subagent_implement"), { task: "change", allowCommits: true }, allowed.ctx);
  assert.match(allowed.requests[0].systemPrompt, /allowed/);

  const violation = harness({ git: [
    { code: 0, stdout: "aaa\n" }, { code: 0, stdout: "" },
    { code: 0, stdout: "bbb\n" }, { code: 0, stdout: "" },
  ] });
  const violated = await execute(violation.tools.get("subagent_implement"), { task: "change", allowCommits: false }, violation.ctx);
  assert.match(violated.value.content[0].text, /not reverted/);
  assert.equal(violated.value.details.awfFailure, true);
  assert.equal(violated.value.details.before.head, "aaa");
  assert.equal(violated.value.details.after.head, "bbb");

  const nongit = harness({ git: [{ code: 1, stdout: "" }, { code: 1, stdout: "" }] });
  const unavailable = await execute(nongit.tools.get("subagent_implement"), { task: "change", allowCommits: false }, nongit.ctx);
  assert.equal(unavailable.value.details.commitVerification, "unavailable");

  const statusFailure = harness({ git: [
    { code: 0, stdout: "aaa\n" }, { code: 1, stdout: "" },
    { code: 0, stdout: "aaa\n" }, { code: 1, stdout: "" },
  ] });
  const statusUnavailable = await execute(statusFailure.tools.get("subagent_implement"), { task: "change", allowCommits: false }, statusFailure.ctx);
  assert.equal(statusUnavailable.value.details.commitVerification, "unavailable");

  const afterUnavailable = harness({ git: [
    { code: 0, stdout: "aaa\n" }, { code: 0, stdout: "" }, { code: 1, stdout: "" },
  ] });
  const mixed = await execute(afterUnavailable.tools.get("subagent_implement"), { task: "change", allowCommits: false }, afterUnavailable.ctx);
  assert.equal(mixed.value.details.commitVerification, "unavailable");
});

test("implementation queue is sequential and queued abort releases it", async () => {
  const h = harness({ git: Array(8).fill({ code: 1, stdout: "" }) });
  let release!: () => void;
  h.deps.runner = { run: (request) => new Promise<RunResult>((resolve) => { h.requests.push(request); release = () => resolve(result); }) };
  const tool = h.tools.get("subagent_implement");
  const first = execute(tool, { task: "one", allowCommits: false }, h.ctx);
  await new Promise((resolve) => setImmediate(resolve));
  const controller = new AbortController();
  const second = execute(tool, { task: "two", allowCommits: false }, h.ctx, controller.signal);
  controller.abort();
  assert.equal(h.requests.length, 1);
  release();
  await first;
  await assert.rejects(second, /aborted while queued/);
});
