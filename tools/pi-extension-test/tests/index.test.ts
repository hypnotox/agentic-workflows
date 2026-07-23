import assert from "node:assert/strict";
import test from "node:test";
import defaultExtension, {
  EXPLORE_TOOLS,
  GROUNDING_TOOLS,
  IMPLEMENT_TOOLS,
  MIN_PI_VERSION,
  REVIEWER_PATHS,
  REVIEW_TOOLS,
  createLimiter,
  registerSubagentTools,
  versionSupported,
  type ExtensionDependencies,
} from "../../../.pi/extensions/awf-subagents/index.ts";
import type { RunRequest, RunResult } from "../../../.pi/extensions/awf-subagents/runner.ts";
import defaultHandoff, { registerHandoff } from "../../../.pi/extensions/awf-handoff/index.ts";
import { defaultLedgerDependencies, registerDashboard } from "../../../.pi/extensions/awf-dashboard/index.ts";
import { initTheme } from "@earendil-works/pi-coding-agent";
import { visibleWidth } from "@earendil-works/pi-tui";
import { Value } from "typebox/value";

initTheme("dark", false);

const event = { sequence: 1, kind: "assistant" as const, text: "working" };
const result: RunResult = {
  output: "child output", stderr: "", events: [event], omittedEvents: 0, failed: false, modelChanged: false,
  usage: { input: 1, output: 2, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 1 },
};
const topLevelUsage = (usage = result.usage) => ({ input: usage.input, output: usage.output, cacheRead: usage.cacheRead, cacheWrite: usage.cacheWrite, totalTokens: usage.input + usage.output + usage.cacheRead + usage.cacheWrite, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: usage.cost } });

function harness(options: {
  version?: string;
  reviewer?: string;
  git?: Array<{ code: number; stdout: string }>;
  run?: (request: RunRequest) => Promise<RunResult>;
  leaf?: any;
  thinking?: () => string;
  queueCommand?: boolean;
  telemetryContexts?: unknown[];
} = {}) {
  const tools = new Map<string, any>();
  const handlers = new Map<string, any>();
  const requests: RunRequest[] = [];
  const notifications: unknown[][] = [];
  const emitted: Array<{ name: string; data: any }> = [];
  const telemetryResponders: Array<(value: unknown) => void> = [];
  const git = [...(options.git ?? [])];
  let reviewerReads = 0;
  let gitCalls = 0;
  let leaf = options.leaf;
  const pi: any = {
    registerTool: (tool: any) => tools.set(tool.name, tool), registerCommand() {}, appendEntry() {},
    on: (name: string, handler: any) => handlers.set(name, handler),
    getThinkingLevel: options.thinking ?? (() => "high"),
    exec: async () => { gitCalls++; return git.shift() ?? { code: 1, stdout: "", stderr: "" }; },
    events: { on() {}, emit: (name: string, data: any) => { if (name === "awf.telemetry.context.request.v1") { telemetryResponders.push(data.respond); for (const value of options.telemetryContexts ?? []) data.respond(value); return; } emitted.push({ name, data }); } },
    ...(options.queueCommand === false ? {} : { queueCommand() {} }),
  };
  const deps: ExtensionDependencies = {
    readFile: async () => { reviewerReads++; return options.reviewer ?? "---\nname: reviewer\ndescription: test\n---\nReview carefully."; },
    runner: { run: async (request) => {
      requests.push(request);
      request.onUpdate?.({
        events: [event], omittedEvents: 0,
        usage: { input: 1, output: 2, cacheRead: 3, cacheWrite: 0, cost: 0.01, turns: 1 },
        model: "test/actual", modelChanged: false, latestCacheHitRate: 75,
      });
      return options.run ? options.run(request) : result;
    } },
    packageVersion: options.version ?? MIN_PI_VERSION,
    extensionFile: "/repo/.pi/extensions/awf-subagents/index.ts",
    now: (() => { let value = 0; return () => (value += 100); })(),
    observationId: () => "observation-1",
  };
  registerSubagentTools(pi, deps);
  const models = new Map([
    ["cheap/model/with/slash", { provider: "cheap", id: "model/with/slash" }],
    ["locked/model", { provider: "locked", id: "model" }],
  ]);
  const ctx: any = {
    cwd: "/repo/subdirectory",
    model: { provider: "test", id: "parent" },
    modelRegistry: {
      find: (provider: string, id: string) => models.get(`${provider}/${id}`),
      hasConfiguredAuth: (model: any) => model.provider !== "locked",
    },
    sessionManager: { getLeafEntry: () => leaf },
    ui: { notify: (...args: unknown[]) => notifications.push(args) },
  };
  return {
    pi, deps, tools, handlers, requests, notifications, emitted, telemetryResponders, ctx,
    setLeaf: (value: any) => { leaf = value; },
    reviewerReads: () => reviewerReads,
    gitCalls: () => gitCalls,
  };
}

async function execute(tool: any, params: any, ctx: any, signal?: AbortSignal, id = "id") {
  const updates: any[] = [];
  const value = await tool.execute(id, params, signal, (update: unknown) => updates.push(update), ctx);
  return { value, updates };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

function assistantLeaf(calls: Array<{ id: string; name: string }>) {
  return {
    type: "message",
    message: { role: "assistant", content: calls.map((call) => ({ type: "toolCall", ...call, arguments: {} })) },
  };
}

test("default factory registers against the installed minimum Pi package", async () => {
  const h = harness();
  const freshTools = new Map<string, any>();
  h.pi.registerTool = (tool: any) => freshTools.set(tool.name, tool);
  await defaultExtension(h.pi);
  assert.deepEqual([...freshTools.keys()], ["subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"]);
});

test("version support is strict and ordered", () => {
  assert.equal(versionSupported("0.81.1"), true);
  assert.equal(versionSupported("0.81.1-beta.1"), true);
  assert.equal(versionSupported("0.81.2"), true);
  assert.equal(versionSupported("0.82.0-beta.1"), true);
  assert.equal(versionSupported("1.0.0"), true);
  assert.equal(versionSupported("0.81.0"), false);
  assert.equal(versionSupported("0.80.8"), false);
  assert.equal(versionSupported("invalid"), false);
});

test("invariant: subagent minimum runtime rejects unsupported version before registration", async () => {
  registerSubagentTools({} as any, { packageVersion: MIN_PI_VERSION } as any);
  const notice = Symbol.for("awf.pi.minimum-runtime-notified");
  for (const [version, queueCommand, detail] of [
    ["0.81.0", true, ""],
    ["unknown", true, ""],
  ] as const) {
    delete (globalThis as any)[notice];
    const h = harness({ version, queueCommand });
    assert.equal(h.tools.size, 0);
    assert.deepEqual([...h.handlers.keys()], ["session_start"]);
    await h.handlers.get("session_start")({}, h.ctx);
    await h.handlers.get("session_start")({}, h.ctx);
    assert.deepEqual(h.notifications, [[`awf Pi extensions require Pi 0.81.1 or newer with their factory event, persistence, tool, command, process, and thinking APIs; found ${version}.${detail} Upgrade Pi and reload.`, "error"]]);
  }
  delete (globalThis as any)[notice];
});

test("invariant: factory guards reject each missing actual dependency before registration", async () => {
  const notice = Symbol.for("awf.pi.minimum-runtime-notified");
  delete (globalThis as any)[notice];
  const starts: any[] = [];
  const notifications: any[] = [];
  const registrations: string[] = [];
  const pi: any = {
    on(name: string, handler: any) { assert.equal(name, "session_start"); starts.push(handler); },
    registerTool() { registrations.push("tool"); },
    registerCommand() { registrations.push("command"); },
    registerShortcut() { registrations.push("shortcut"); },
    registerFlag() { registrations.push("flag"); },
    registerMessageRenderer() { registrations.push("renderer"); },
  };
  await defaultExtension(pi);
  await defaultHandoff(pi);
  assert.deepEqual(registrations, []);
  assert.equal(starts.length, 2);
  const ctx = { ui: { notify: (...args: any[]) => notifications.push(args) } };
  for (const start of starts) await start({}, ctx);
  assert.deepEqual(notifications, [["awf Pi extensions require Pi 0.81.1 or newer with their factory event, persistence, tool, command, process, and thinking APIs; found 0.81.1. Missing runtime APIs: eventsEmit, exec, getThinkingLevel. Upgrade Pi and reload.", "error"]]);
  delete (globalThis as any)[notice];

  const factories: Array<{ name: string; required: string[]; register(pi: any, version: string): void }> = [
    { name: "subagent", required: ["on", "events.emit", "registerTool", "exec", "getThinkingLevel"], register(api, version) { registerSubagentTools(api, { packageVersion: version, extensionFile: "/p/.pi/extensions/awf-subagents/index.ts", readFile: async () => "", runner: { run: async () => result } } as any); } },
    { name: "handoff", required: ["on", "events.emit", "registerTool", "registerCommand", "queueCommand"], register(api, version) { registerHandoff(api, { packageVersion: version } as any); } },
    { name: "dashboard", required: ["on", "events.on", "events.emit", "appendEntry", "registerTool", "registerCommand", "exec"], register(api, version) { registerDashboard(api, { packageVersion: version, extensionFile: "/p/.pi/extensions/awf-dashboard/index.ts", ledger: defaultLedgerDependencies("/p/.pi/extensions/awf-dashboard/index.ts"), exec: api.exec } as any); } },
  ];
  const complete = () => { const registrations: string[] = []; const api: any = { on() { registrations.push("on"); }, events: { on() { registrations.push("events.on"); }, emit() {} }, appendEntry() {}, registerTool() { registrations.push("tool"); }, registerCommand() { registrations.push("command"); }, queueCommand() {}, exec: async () => ({ code: 0, stdout: "", stderr: "" }), getThinkingLevel: () => "high" }; return { api, registrations }; };
  for (const factory of factories) {
    for (const missing of factory.required) { delete (globalThis as any)[notice]; const { api, registrations } = complete(); if (missing.includes(".")) { const [owner, field] = missing.split("."); delete api[owner][field]; } else delete api[missing]; factory.register(api, "0.81.1"); assert.deepEqual(registrations.filter((value) => value !== "on"), [], `${factory.name} accepted missing ${missing}`); }
    const below = complete(); factory.register(below.api, "0.81.0"); assert.deepEqual(below.registrations.filter((value) => value !== "on"), [], `${factory.name} accepted below-minimum version`);
    const boundary = complete(); factory.register(boundary.api, "0.81.1"); assert.equal(boundary.registrations.some((value) => value !== "on"), true, `${factory.name} rejected minimum version`);
  }
  delete (globalThis as any)[notice];
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

test("all role schemas accept optional exact model while preserving closed required fields", () => {
  const h = harness();
  const cases = [
    ["subagent_grounding", { task: "ground" }, ["task"]],
    ["subagent_explore", { task: "inspect", breadth: "bounded", detail: "analysis" }, ["task", "breadth", "detail"]],
    ["subagent_review", { kind: "code", task: "review" }, ["kind", "task"]],
    ["subagent_implement", { task: "change", allowCommits: false }, ["task", "allowCommits"]],
  ] as const;
  for (const [name, params, required] of cases) {
    const schema = h.tools.get(name).parameters;
    assert.equal(Value.Check(schema, params), true, `${name} rejects old shape`);
    assert.equal(Value.Check(schema, { ...params, model: "cheap/model/with/slash" }), true, `${name} rejects model`);
    assert.equal(Value.Check(schema, { ...params, extra: true }), false, `${name} accepts unknown field`);
    for (const field of required) {
      const missing = { ...params } as any;
      delete missing[field];
      assert.equal(Value.Check(schema, missing), false, `${name} no longer requires ${field}`);
    }
    assert.deepEqual(schema.required, required);
  }
});

test("all roles route omitted and exact selected models with inherited thinking and diagnostics", async () => {
  const cases = [
    ["subagent_grounding", { task: "ground" }],
    ["subagent_explore", { task: "inspect", breadth: "bounded", detail: "analysis" }],
    ["subagent_review", { kind: "code", task: "review" }],
    ["subagent_implement", { task: "change", allowCommits: false }],
  ] as const;
  for (const [name, params] of cases) {
    const inherited = harness({ git: Array(4).fill({ code: 1, stdout: "" }) });
    const inheritedValue = await execute(inherited.tools.get(name), params, inherited.ctx);
    assert.deepEqual(inherited.requests[0].model, { provider: "test", id: "parent" });
    assert.equal(inherited.requests[0].thinkingLevel, "high");
    assert.equal(inheritedValue.value.details.requestedModel, undefined);
    assert.equal(inheritedValue.value.details.resolvedModel, "test/parent");
    assert.equal(inheritedValue.value.details.modelSource, "inherited");
    assert.equal(inheritedValue.value.details.thinkingLevel, "high");
    assert.equal(inheritedValue.updates.at(-1)?.details.resolvedModel, "test/parent");

    const selected = harness({
      git: Array(4).fill({ code: 1, stdout: "" }),
      run: async () => ({ ...result, model: "cheap/model/with/slash" }),
    });
    const selectedValue = await execute(selected.tools.get(name), { ...params, model: "cheap/model/with/slash" }, selected.ctx);
    assert.deepEqual(selected.requests[0].model, { provider: "cheap", id: "model/with/slash" });
    assert.equal(selected.requests[0].thinkingLevel, "high");
    assert.equal(selectedValue.value.details.requestedModel, "cheap/model/with/slash");
    assert.equal(selectedValue.value.details.resolvedModel, "cheap/model/with/slash");
    assert.equal(selectedValue.value.details.modelSource, "requested");
    assert.equal(selectedValue.value.details.model, "cheap/model/with/slash");
  }
});

test("invalid explicit models reject before every role side effect", async () => {
  const invalid = [
    ["", /exact provider\/model-id/], ["provider", /exact provider\/model-id/], ["/model", /exact provider\/model-id/],
    ["provider/", /exact provider\/model-id/], ["unknown/model", /not registered/], ["locked/model", /no configured authentication/],
  ] as const;
  const cases = [
    ["subagent_grounding", { task: "ground" }],
    ["subagent_explore", { task: "inspect", breadth: "bounded", detail: "analysis" }],
    ["subagent_review", { kind: "code", task: "review" }],
    ["subagent_implement", { task: "change", allowCommits: false }],
  ] as const;
  for (const [name, params] of cases) {
    for (const [model, pattern] of invalid) {
      const h = harness();
      await assert.rejects(execute(h.tools.get(name), { ...params, model }, h.ctx), pattern);
      assert.equal(h.requests.length, 0);
      assert.equal(h.reviewerReads(), 0);
      assert.equal(h.gitCalls(), 0);
    }
  }
  const noParent = harness();
  await assert.rejects(execute(noParent.tools.get("subagent_grounding"), { task: "ground" }, { ...noParent.ctx, model: undefined }), /active parent model/);
});

test("invariant: subagent context and observations are exact closed bounded and private", async () => {
  const h = harness({ run: async () => ({
    ...result,
    model: "actual/model",
    stopReason: "stop",
    toolCount: 42,
    toolFailureCount: 7,
    usage: { input: 11, output: 12, cacheRead: 13, cacheWrite: 14, cost: 0.5, turns: 2 },
  }) });
  const completed = await execute(h.tools.get("subagent_grounding"), { task: "private task", model: "cheap/model/with/slash" }, h.ctx);
  assert.equal(completed.value.content[0].text, "child output");
  assert.deepEqual(completed.value.usage, topLevelUsage({ input: 11, output: 12, cacheRead: 13, cacheWrite: 14, cost: 0.5, turns: 2 }));
  assert.equal(h.emitted.length, 1);
  const observation = h.emitted[0];
  assert.equal(observation.name, "awf.telemetry.subagent.v1");
  assert.deepEqual(observation.data, {
    version: { major: 2, minor: 0 }, observationId: "observation-1", role: "grounding",
    requestedModel: "cheap/model/with/slash", resolvedModel: "actual/model", thinkingLevel: "high",
    queueDurationMs: 100, runDurationMs: 100,
    inputTokens: 11, outputTokens: 12, cacheReadTokens: 13, cacheWriteTokens: 14, costUsd: 0.5,
    outcome: "success", stopReason: "complete", toolCount: 42, toolFailureCount: 7,
  });
  const serialized = JSON.stringify(observation.data);
  for (const forbidden of ["private task", "child output", "stderr", "events", "tools", "args", "failureMessage"]) {
    assert.equal(serialized.includes(forbidden), false, forbidden);
  }

  const contextual = harness({ telemetryContexts: [{ version: { major: 2, minor: 0 }, context: { effortId: "effort", sessionId: "session", trajectoryId: "trajectory", piAnchorId: "anchor" } }] }); await execute(contextual.tools.get("subagent_grounding"), { task: "context" }, contextual.ctx); assert.deepEqual({ effortId: contextual.emitted[0].data.effortId, sessionId: contextual.emitted[0].data.sessionId, trajectoryId: contextual.emitted[0].data.trajectoryId, piAnchorId: contextual.emitted[0].data.piAnchorId, observationId: contextual.emitted[0].data.observationId }, { effortId: "effort", sessionId: "session", trajectoryId: "trajectory", piAnchorId: "anchor", observationId: "observation-1" });
  contextual.telemetryResponders[0]({ version: { major: 2, minor: 0 }, context: { effortId: "late", sessionId: "session", trajectoryId: "trajectory" } });
  const anchorless = harness({ telemetryContexts: [{ version: { major: 2, minor: 0 }, context: { effortId: "effort", sessionId: "session", trajectoryId: "trajectory" } }] }); await execute(anchorless.tools.get("subagent_grounding"), { task: "anchorless context" }, anchorless.ctx); assert.equal(anchorless.emitted[0].data.piAnchorId, undefined);
  for (const invalid of [null, { version: { major: 3, minor: 0 }, context: { effortId: "effort", sessionId: "session", trajectoryId: "trajectory" } }, { version: { major: 2, minor: 0 }, context: { effortId: "effort", sessionId: "session", trajectoryId: "trajectory", privatePrompt: "leak" } }, { version: { major: 2, minor: 0 }, context: { effortId: "../bad", sessionId: "session", trajectoryId: "trajectory" } }]) { const invalidContext = harness({ telemetryContexts: [invalid] }); await execute(invalidContext.tools.get("subagent_grounding"), { task: "invalid context" }, invalidContext.ctx); assert.equal(invalidContext.emitted[0].data.effortId, undefined); assert.equal(JSON.stringify(invalidContext.emitted[0].data).includes("leak"), false); }
  const duplicateContext = harness({ telemetryContexts: [{ effortId: "first" }, { effortId: "second" }] }); await execute(duplicateContext.tools.get("subagent_grounding"), { task: "duplicate context" }, duplicateContext.ctx); assert.equal(duplicateContext.emitted[0].data.effortId, undefined);
  const isolated = harness();
  isolated.pi.events.emit = () => { throw new Error("listener failed"); };
  const value = await execute(isolated.tools.get("subagent_grounding"), { task: "still succeeds" }, isolated.ctx);
  assert.equal(value.value.content[0].text, "child output");
  for (const [source, bounded] of [["end", "complete"], ["tool", "tool"], ["length", "length"], ["canceled", "canceled"], [undefined, "unknown"]] as const) { const alias = harness({ run: async () => ({ ...result, stopReason: source }) }); await execute(alias.tools.get("subagent_grounding"), { task: "alias" }, alias.ctx); assert.equal(alias.emitted[0].data.stopReason, bounded); }
  const aborted = harness({ run: async () => { throw new Error("aborted child"); } }); const controller = new AbortController(); controller.abort(); await assert.rejects(execute(aborted.tools.get("subagent_grounding"), { task: "aborted" }, aborted.ctx, controller.signal), /aborted child/); assert.equal(aborted.emitted[0].data.outcome, "canceled"); assert.equal(aborted.emitted[0].data.stopReason, "canceled");
  const failedListener = harness({ run: async () => { throw new Error("failed child"); } }); failedListener.pi.events.emit = () => { throw new Error("listener failed"); }; await assert.rejects(execute(failedListener.tools.get("subagent_grounding"), { task: "failed" }, failedListener.ctx), /failed child/);
});

test("exploration limiter starts ten, queues FIFO, removes aborts, and releases on every exit", async () => {
  const pending: Array<ReturnType<typeof deferred<RunResult>>> = [];
  const h = harness({ run: async () => {
    const next = deferred<RunResult>();
    pending.push(next);
    return next.promise;
  } });
  const tool = h.tools.get("subagent_explore");
  const call = (task: string, signal?: AbortSignal) => execute(tool, { task, breadth: "bounded", detail: "summary" }, h.ctx, signal);
  const calls = Array.from({ length: 14 }, (_, index) => call(String(index)));
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(h.requests.map((request) => request.task), Array.from({ length: 10 }, (_, index) => String(index)));
  await assert.rejects(
    execute(tool, { task: "invalid-before-slot", breadth: "bounded", detail: "summary", model: "unknown/model" }, h.ctx),
    /not registered/,
  );
  assert.equal(h.requests.length, 10);

  pending[0].resolve(result);
  await calls[0];
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests[10].task, "10");

  const aborted = new AbortController();
  const queuedAbort = call("aborted", aborted.signal);
  aborted.abort();
  await assert.rejects(queuedAbort, /aborted while queued/);
  assert.equal(h.requests.some((request) => request.task === "aborted"), false);

  pending[1].resolve({ ...result, failed: true, failureMessage: "child failed", stopReason: "error" });
  await calls[1];
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests[11].task, "11");

  const activeAbort = new AbortController();
  const activeCall = call("active-abort", activeAbort.signal);
  pending[2].resolve(result);
  await calls[2];
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests[12].task, "12");
  pending[12].resolve(result);
  await calls[12];
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests[13].task, "13");
  pending[13].resolve(result);
  await calls[13];
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests.at(-1)?.task, "active-abort");
  activeAbort.abort();
  pending.at(-1)!.resolve({ ...result, failed: true, failureMessage: "aborted", stopReason: "aborted" });
  const abortedValue = await activeCall;
  assert.equal(abortedValue.value.details.state, "aborted");

  const setupFailure = call("setup-failure");
  pending[3].reject(new Error("runner setup rejected"));
  await assert.rejects(calls[3], /runner setup rejected/);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests.at(-1)?.task, "setup-failure");
  pending.at(-1)!.resolve(result);
  await setupFailure;

  for (let index = 4; index < pending.length; index++) pending[index].resolve(result);
  await Promise.all(calls.slice(4, 12));
});

test("limiter release is idempotent and pre-aborted acquisition does not consume capacity", async () => {
  const limiter = createLimiter(1);
  const release = await limiter.acquire(undefined);
  release();
  release();
  const controller = new AbortController();
  controller.abort();
  await assert.rejects(limiter.acquire(controller.signal), /aborted while queued/);
  const next = await limiter.acquire(undefined);
  next();
});

test("tool preflight classifies singleton, mixed, stale, and malformed current leaves", async () => {
  const h = harness();
  const handler = h.handlers.get("tool_call");
  h.setLeaf(assistantLeaf([{ id: "impl", name: "subagent_implement" }]));
  assert.equal(await handler({ toolCallId: "impl", toolName: "subagent_implement" }, h.ctx), undefined);

  h.setLeaf(assistantLeaf([{ id: "impl", name: "subagent_implement" }, { id: "read", name: "read" }]));
  for (const event of [{ toolCallId: "impl", toolName: "subagent_implement" }, { toolCallId: "read", toolName: "read" }]) {
    assert.deepEqual(await handler(event, h.ctx), {
      block: true,
      reason: "A batch containing subagent_implement cannot contain siblings; retry subagent_implement alone.",
    });
  }

  h.setLeaf(assistantLeaf([{ id: "old", name: "subagent_implement" }]));
  assert.deepEqual(await handler({ toolCallId: "new", toolName: "subagent_implement" }, h.ctx), {
    block: true,
    reason: "Cannot verify the current tool batch; retry subagent_implement alone.",
  });
  assert.equal(await handler({ toolCallId: "new", toolName: "read" }, h.ctx), undefined);
  for (const leaf of [undefined, { type: "custom" }, { type: "message", message: { role: "user", content: [] } }, { type: "message", message: { role: "assistant", content: "bad" } }]) {
    h.setLeaf(leaf);
    assert.deepEqual(await handler({ toolCallId: "impl", toolName: "subagent_implement" }, h.ctx), {
      block: true,
      reason: "Cannot verify the current tool batch; retry subagent_implement alone.",
    });
  }
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

test("queued children snapshot thinking before waiting and expose role metadata", async () => {
  let thinking = "high";
  const pending: Array<ReturnType<typeof deferred<RunResult>>> = [];
  const h = harness({
    thinking: () => thinking,
    git: Array(8).fill({ code: 1, stdout: "" }),
    run: async () => {
      const next = deferred<RunResult>();
      pending.push(next);
      return next.promise;
    },
  });
  const tool = h.tools.get("subagent_implement");
  const first = execute(tool, { task: "one", allowCommits: false }, h.ctx);
  await new Promise((resolve) => setImmediate(resolve));
  const second = execute(tool, { task: "two", allowCommits: true }, h.ctx);
  thinking = "low";
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal((await Promise.race([second.then(() => "done"), Promise.resolve("pending")])), "pending");
  pending[0].resolve(result);
  await first;
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(h.requests[1].thinkingLevel, "high");
  pending[1].resolve(result);
  const completed = await second;
  assert.equal(completed.updates[0].details.state, "queued");
  assert.equal(completed.updates[0].details.thinkingLevel, "high");
  assert.deepEqual(completed.updates[0].details.options, { allowCommits: true });
  assert.equal(completed.updates.some((update) => update.details.state === "running"), true);
});

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
    const call = rendered(tool.renderCall({ task: "x".repeat(1000), model: "cheap/model" }, theme, {}), 24).join("\n");
    assert.match(call, new RegExp(`${role} subagent`));
    assert.match(call, /cheap\/model/);
    assert.match(call, /task\s+truncated/);
    const collapsed = rendered(tool.renderResult({
      content: [{ type: "text", text: "report" }],
      details: { role, task: "task", state: "completed", events, omittedEvents: 7, usage, requestedModel: "cheap/model", resolvedModel: "cheap/model", modelSource: "requested", model: "proxy/child", modelChanged: false, thinkingLevel: "high", latestCacheHitRate: 50, options: {} },
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
    assert.match(collapsed, /2 turns ↑1 ↓2 R3 W4\s+CH50\.0% \$0\.250/);
    assert.match(collapsed, /model:cheap\/model/);
    assert.match(collapsed, /source:requested/);
    assert.match(collapsed, /actual:proxy\/child/);
    assert.match(collapsed, /model-mismatch/);
    assert.doesNotMatch(collapsed, /model-changed/);
    assert.match(collapsed, /thinking:high/);
    assert.match(collapsed, /to expand/);
  }
  const emptyCall = rendered(h.tools.get("subagent_explore").renderCall({}, theme, {}), 120).join("\n");
  assert.match(emptyCall, /inherit parent/);
  assert.match(emptyCall, /no task/);
  rendered(h.tools.get("subagent_explore").renderCall({ task: "é".repeat(1000), breadth: "broad", detail: "paths" }, theme, {}), 24);
  assert.match(rendered(h.tools.get("subagent_review").renderCall({ task: "review", kind: "code" }, theme, {}), 120).join("\n"), /kind:code/);
  assert.match(rendered(h.tools.get("subagent_implement").renderCall({ task: "change", allowCommits: false }, theme, {}), 120).join("\n"), /allowCommits:false/);

  const formats = [
    [1_200, "↑1.2k"], [120_000, "↑120k"], [1_200_000, "↑1.2m"], [12_000_000, "↑12m"],
  ] as const;
  for (const [input, expected] of formats) {
    const text = rendered(h.tools.get("subagent_grounding").renderResult({
      content: [],
      details: { role: "grounding", task: "task", state: "running", events: [], omittedEvents: 0, usage: { input, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 1 }, resolvedModel: "test/parent", modelSource: "inherited", thinkingLevel: "high", options: {} },
    }, { expanded: false, isPartial: true }, theme, {}), 120).join("\n");
    assert.match(text, new RegExp(expected));
  }
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
    details: { role: "explore", task: "inspect", state: "completed", events, omittedEvents: 0, stderr: "warning", usage, model: "proxy/child", resolvedModel: "test/parent", modelSource: "inherited", modelChanged: true, modelMismatch: true, thinkingLevel: "high", latestCacheHitRate: 37.5, options: { breadth: "bounded", detail: "analysis" } },
  }, { expanded: true, isPartial: false }, theme, {}), 24).join("\n");
  for (const phrase of ["Task", "inspect", "Activity", "Report", "Final report", "Diagnostics", "warning", "Model", "proxy/child", "resolved: test/parent", "Thinking", "high", "breadth: bounded", "detail: analysis", "Input: 1", "Output: 2", "Cache read: 3", "Cache write: 4", "Cache hit: 37.5%", "Cost: $0.250", "Turns: 1", "model changed during run"]) assert.ok(complete.includes(phrase), `expanded rendering missing ${phrase}`);
  assert.match(complete, /actual model differs\s+from resolved model/);

  const partial = rendered(tool.renderResult({ content: [{ type: "text", text: "(running...)" }], details: { role: "explore", task: "", state: "queued", events: [], omittedEvents: 0, usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 0 }, resolvedModel: "test/parent", modelSource: "inherited", thinkingLevel: "high", options: { breadth: "bounded", detail: "summary" } } }, { expanded: true, isPartial: true }, theme, {}), 120).join("\n");
  assert.match(partial, /queued/);
  for (const zero of ["Input: 0", "Output: 0", "Cache read: 0", "Cache write: 0", "Cost: $0.000", "Turns: 0"]) assert.ok(partial.includes(zero), `zero-usage rendering missing ${zero}`);
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

  const runningCollapsed = rendered(tool.renderResult({ content: [], details: { role: "explore", task: "task", state: "running", events: [], omittedEvents: 0, usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 0 }, resolvedModel: "test/parent", modelSource: "inherited", thinkingLevel: "high", options: {} } }, { expanded: false, isPartial: true }, theme, {}), 120).join("\n");
  assert.match(runningCollapsed, /running/);
  assert.match(runningCollapsed, /no omitted events/);
  assert.match(runningCollapsed, /model:test\/parent/);
  assert.doesNotMatch(runningCollapsed, /↑|↓|CH|\$0\.000/);
  assert.doesNotMatch(runningCollapsed, /to expand/);
  const noHint = rendered(tool.renderResult({ content: [], details: { role: "explore", task: "", state: "completed", events: [], omittedEvents: 0 } }, { expanded: false, isPartial: false }, theme, {}), 120).join("\n");
  assert.match(noHint, /model:unknown/);
  assert.doesNotMatch(noHint, /undefined|to expand/);
  const legacy = rendered(tool.renderResult({ content: [], details: { role: "explore", task: "task", state: "completed", events: [], omittedEvents: 0, requestedModel: "legacy/requested" } }, { expanded: true, isPartial: false }, theme, {}), 120).join("\n");
  assert.match(legacy, /Model: legacy\/requested/);
  assert.doesNotMatch(legacy, /undefined/);
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
  assert.equal(updates.at(-1)?.content[0].text, "(running...)");
  assert.deepEqual(updates.at(-1)?.details.events, [event]);
  assert.equal(updates.at(-1)?.details.model, "test/actual");
  assert.equal(updates.at(-1)?.details.latestCacheHitRate, 75);
  assert.deepEqual(updates.at(-1)?.details.usage, { input: 1, output: 2, cacheRead: 3, cacheWrite: 0, cost: 0.01, turns: 1 });
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
  assert.equal(h.requests.some((request) => request.task === " "), false);
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
  assert.deepEqual(value.usage, topLevelUsage());

  const fallback = harness();
  fallback.deps.runner = { run: async () => ({ ...result, failed: true, failureMessage: undefined, stopReason: "error" }) };
  const failed = await execute(fallback.tools.get("subagent_explore"), { task: "fail", breadth: "bounded", detail: "summary" }, fallback.ctx);
  assert.equal(failed.value.content[0].text, "Subagent failed");
  assert.equal(failed.value.details.state, "failed");
  assert.deepEqual(failed.value.usage, topLevelUsage());
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

  const violationUsage = { input: 11, output: 12, cacheRead: 13, cacheWrite: 14, cost: 0.15, turns: 2 };
  const violation = harness({
    git: [
      { code: 0, stdout: "aaa\n" }, { code: 0, stdout: "" },
      { code: 0, stdout: "bbb\n" }, { code: 0, stdout: "" },
    ],
    run: async () => ({ ...result, model: "cheap/actual-model", usage: violationUsage, stderr: "child diagnostic", omittedEvents: 2, stopReason: "stop" }),
  });
  const violated = await execute(violation.tools.get("subagent_implement"), {
    task: "change", allowCommits: false, model: "cheap/model/with/slash",
  }, violation.ctx);
  assert.match(violated.value.content[0].text, /not reverted/);
  assert.deepEqual(violation.requests[0].model, { provider: "cheap", id: "model/with/slash" });
  assert.equal(violated.value.details.awfFailure, true);
  assert.equal(violated.value.details.requestedModel, "cheap/model/with/slash");
  assert.equal(violated.value.details.model, "cheap/actual-model");
  assert.equal(violated.value.details.modelMismatch, true);
  assert.deepEqual(violated.value.details.events, [event]);
  assert.equal(violated.value.details.omittedEvents, 2);
  assert.equal(violated.value.details.stderr, "child diagnostic");
  assert.equal(violated.value.details.stopReason, "stop");
  assert.deepEqual(violated.value.details.usage, violationUsage);
  assert.deepEqual(violated.value.usage, topLevelUsage(violationUsage));
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
