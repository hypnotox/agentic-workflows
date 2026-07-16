import assert from "node:assert/strict";
import test from "node:test";
import defaultExtension, {
  EXPLORE_TOOLS,
  IMPLEMENT_TOOLS,
  MIN_PI_VERSION,
  REVIEWER_PATHS,
  REVIEW_TOOLS,
  registerSubagentTools,
  versionSupported,
  type ExtensionDependencies,
} from "../../../.pi/extensions/awf-subagents/index.ts";
import type { RunRequest, RunResult } from "../../../.pi/extensions/awf-subagents/runner.ts";

const result: RunResult = {
  output: "child output", stderr: "", summaries: [],
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
    runner: { run: async (request) => { requests.push(request); request.onUpdate?.({ text: "working", summaries: [] }); return result; } },
    packageVersion: options.version ?? MIN_PI_VERSION,
    extensionFile: "/repo/.pi/extensions/awf-subagents/index.ts",
  };
  registerSubagentTools(pi, deps);
  const ctx: any = { cwd: "/repo", model: { provider: "test", id: "parent" }, ui: { notify: (...args: unknown[]) => notifications.push(args) } };
  return { pi, deps, tools, handlers, requests, notifications, ctx };
}

async function execute(tool: any, params: any, ctx: any, signal?: AbortSignal) {
  const updates: unknown[] = [];
  const value = await tool.execute("id", params, signal, (update: unknown) => updates.push(update), ctx);
  return { value, updates };
}

test("default factory registers against the installed minimum Pi package", async () => {
  const h = harness();
  const freshTools = new Map<string, any>();
  h.pi.registerTool = (tool: any) => freshTools.set(tool.name, tool);
  await defaultExtension(h.pi);
  assert.deepEqual([...freshTools.keys()], ["subagent_explore", "subagent_review", "subagent_implement"]);
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

test("registers exactly three governed public tools", () => {
  const h = harness();
  assert.deepEqual([...h.tools.keys()], ["subagent_explore", "subagent_review", "subagent_implement"]);
  assert.deepEqual(EXPLORE_TOOLS, ["read", "grep", "find", "ls", "bash"]);
  assert.deepEqual(REVIEW_TOOLS, EXPLORE_TOOLS);
  assert.deepEqual(IMPLEMENT_TOOLS, ["read", "bash", "edit", "write", "grep", "find", "ls"]);
  assert.deepEqual(REVIEWER_PATHS, { adr: ".pi/skills/adr-reviewer.md", plan: ".pi/skills/plan-reviewer.md", code: ".pi/skills/code-reviewer.md" });
});

test("explore inherits parent state, streams, and rejects invalid context", async () => {
  const h = harness();
  const { value, updates } = await execute(h.tools.get("subagent_explore"), { task: "inspect" }, h.ctx);
  assert.equal(value.content[0].text, "child output");
  assert.equal(updates.length, 1);
  assert.deepEqual(h.requests[0].model, { provider: "test", id: "parent" });
  assert.equal(h.requests[0].thinkingLevel, "high");
  assert.deepEqual(h.requests[0].tools, EXPLORE_TOOLS);
  assert.match(h.requests[0].systemPrompt, /do not edit files or commit/);
  await assert.rejects(execute(h.tools.get("subagent_explore"), { task: " " }, h.ctx), /non-empty/);
  await assert.rejects(execute(h.tools.get("subagent_explore"), { task: "x" }, { ...h.ctx, model: undefined }), /active parent model/);
  await h.tools.get("subagent_explore").execute("id", { task: "without update" }, undefined, undefined, h.ctx);
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
  await assert.rejects(execute(violation.tools.get("subagent_implement"), { task: "change", allowCommits: false }, violation.ctx), /not reverted/);

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
