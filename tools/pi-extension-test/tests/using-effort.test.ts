import assert from "node:assert/strict";
import test from "node:test";
import { mkdir } from "node:fs/promises";
import { Value } from "typebox/value";
import { EventEmitter } from "node:events";
import { activity, createChildMemoryExecutor, EffortProtocolError, MEMORY_CLOSE_DELAY_MS, MEMORY_KILL_DELAY_MS, MEMORY_STDERR_MAX, MEMORY_STDOUT_MAX, memoryEdit, memoryRead, memoryUpdate, productionChildMemoryDependencies } from "../../../.pi/extensions/awf-effort/client.ts";
import effortExtension, { registerDefaultEffort, registerEffort } from "../../../.pi/extensions/awf-effort/index.ts";

const OWNER = "00000000-0000-4000-8000-000000000001";
const OTHER = "00000000-0000-4000-8000-000000000002";
const TIME = "2026-08-03T00:00:00Z";
const line = (x: unknown) => `${JSON.stringify(x)}\n`;
function success(condition: "attached" | "taken-over" | "heartbeat" = "attached", owner = OWNER, slug = "demo") {
  return { schemaVersion: 2, condition, effort: { slug, title: "Demo" }, memory: { effort: slug, phase: "Build", next: "Test", updated: TIME }, activity: { schemaVersion: 2, owner, attachedAt: TIME, heartbeatAt: TIME } };
}
function refusal(condition = "missing", actions = ["repair and retry"], extra: any = {}) { return { schemaVersion: 2, condition, outcome: { category: "operation", condition: "resident is absent", changedActivity: false, nextActions: actions, ...extra } }; }

async function decode(value: unknown, op: any = "attach") { return activity(async () => ({ code: 0, stdout: line(value), stderr: "" }), "/repo", op, "demo", OWNER); }

test("client accepts exact v2 success and refusal matrices with immutable facts", async () => {
  for (const condition of ["attached", "taken-over", "heartbeat"] as const) {
    const reply = await decode(success(condition));
    assert.equal(reply.condition, condition); assert.equal(Object.isFrozen(reply), true); assert.equal(Object.isFrozen(reply.activity), true);
  }
  assert.deepEqual(await decode({ schemaVersion: 2, condition: "detached" }, "detach"), { schemaVersion: 2, condition: "detached" });
  for (const condition of ["not-owner", "missing", "invalid-memory", "unsafe-resident"]) assert.equal((await decode(refusal(condition))).condition, condition);
});

test("client strictly rejects transport, closed envelopes, facts, and outcomes", async () => {
  const bad: any[] = [null, [], {}, { schemaVersion: 1, condition: "attached" }, { schemaVersion: 2, condition: "other" }, { ...success(), extra: true }, { ...success(), effort: { slug: "", title: "x" } }, { ...success("attached", OWNER, "other"), memory: { effort: "other", phase: "x", next: "x", updated: TIME } }, { ...success(), memory: { effort: "other", phase: "x", next: "x", updated: TIME } }, { ...success(), activity: { schemaVersion: 2, owner: "BAD", attachedAt: TIME, heartbeatAt: TIME } }, { ...success(), activity: { schemaVersion: 2, owner: OWNER, attachedAt: "", heartbeatAt: TIME } }, { ...success(), activity: { schemaVersion: 2, owner: OWNER, attachedAt: "bad", heartbeatAt: TIME } }, { ...success(), activity: { schemaVersion: 2, owner: OWNER, attachedAt: "2026-02-30T00:00:00Z", heartbeatAt: TIME } }, { ...success(), memory: { ...success().memory, updated: "2026-02-30T00:00:00Z" } }, { ...refusal(), outcome: { category: "bad", condition: "x", changedActivity: false, nextActions: ["x"] } }, { ...refusal(), outcome: { category: "operation", condition: "x", changedActivity: false, nextActions: [] } }, { ...refusal(), outcome: { category: "operation", condition: "x", changedActivity: false, nextActions: [""] } }, { ...refusal(), outcome: { category: "operation", condition: "x", changedActivity: false, nextActions: ["x"], cause: "" } }];
  for (const value of bad) await assert.rejects(decode(value), /invalid envelope/);
  await assert.rejects(activity(async () => ({ stdout: line(success()) }), "/repo", "attach", "bad_slug", OWNER), /invocation is invalid/);
  const transports: Array<[any, RegExp]> = [
    [async () => { throw new Error("spawn") }, /execution failed/], [async () => ({ code: 1, stderr: "bad" }), /activity failed/], [async () => ({ stdout: "" }), /single JSON/], [async () => ({ stdout: "{}\n{}\n" }), /single JSON/], [async () => ({ stdout: "{]\n" }), /malformed JSON/], [async () => ({ stdout: "x".repeat(50 * 1024 + 1) }), /exceeded bounds/], [async () => ({ stdout: line(success()), stderr: "x".repeat(50 * 1024 + 1) }), /exceeded bounds/],
  ];
  for (const [exec, pattern] of transports) await assert.rejects(activity(exec, "/repo", "attach", "demo", OWNER), pattern);
  const signal = new AbortController().signal;
  let argv: readonly string[] = [];
  await activity(async (command, a, options) => { assert.equal(command, "./awf"); argv = a; assert.equal(options.signal, signal); return { stdout: line(success()) }; }, "/repo", "detach", "demo", OWNER, signal);
  assert.deepEqual(argv, ["effort", "activity", "detach", "demo", "--owner", OWNER, "--json"]); assert.equal(new EffortProtocolError("x").name, "EffortProtocolError");
});

function harness(replies: any[] = [], opts: { directory?: any; emitThrows?: boolean; factoryCapabilities?: unknown; active?: string[]; queue?: any; memoryExec?: any } = {}) {
  const tools = new Map<string, any>(), hooks = new Map<string, any>(), listeners = new Map<string, any>(); const events: any[] = [], calls: any[] = [], options: any[] = [], queueCalls: any[] = [];
  let active = [...(opts.active ?? [])]; const queue = opts.queue ?? (async (path: string, work: any) => { queueCalls.push(path); return work(); }); const pi: any = { getActiveTools: () => active, setActiveTools: (value: string[]) => { active = [...value]; }, registerTool: (value: any) => tools.set(value.name, value), on: (name: string, hook: any) => hooks.set(name, hook), events: { emit: (name: string, payload: any) => { if (opts.emitThrows) throw new Error("emit"); events.push([name, payload]); if (name === "remote-pi:capabilities:request" && Object.hasOwn(opts, "factoryCapabilities")) listeners.get("remote-pi:capabilities")(opts.factoryCapabilities); }, on: (name: string, hook: any) => listeners.set(name, hook) }, exec: async (_: string, args: string[], option: any) => { calls.push(args); options.push(option); const reply = replies.shift() ?? success(args[2] as any, args[5], args[3]); if (reply instanceof Error) throw reply; return { code: 0, stdout: typeof reply === "string" ? reply : line(reply), stderr: "" }; } };
  let n = 0; registerEffort(pi, { uuid: () => n++ ? OTHER : OWNER, isDirectory: opts.directory ?? (async () => true), packageVersion: "0.81.1", fileMutationQueue: queue, memoryExec: opts.memoryExec });
  const tool = () => tools.get("using_effort"); const ctx = { cwd: "/repo" };
  return { pi, tools, hooks, listeners, events, calls, options, queueCalls, active: () => active, tool, ctx };
}
async function request(h: any, args: any, signal = new AbortController().signal) { return h.tool().execute("id", args, signal, () => {}, h.ctx); }
function lastText(r: any) { return r.content[0].text; }

test("client covers sentinel metadata, exact condition shapes, and cancellation transport", async () => {
  const sentinel = success(); sentinel.memory.updated = "Not yet updated.";
  assert.equal((await decode(sentinel)).memory?.updated, "Not yet updated.");
  const nano = success(); nano.activity.attachedAt = "2026-08-03T00:00:00.123456789Z"; nano.activity.heartbeatAt = "2026-08-03T00:00:00.1Z";
  assert.equal((await decode(nano)).activity?.attachedAt, nano.activity.attachedAt);
  const invalidSentinel = success(); invalidSentinel.activity.attachedAt = "Not yet updated.";
  await assert.rejects(decode(invalidSentinel), /invalid envelope/);
  const malformed = [
    { schemaVersion: 2, condition: "attached", effort: success().effort, memory: success().memory },
    { schemaVersion: 2, condition: "detached", outcome: refusal().outcome },
    { schemaVersion: 2, condition: "missing" },
    { ...refusal("missing"), effort: success().effort },
    { ...success(), activity: undefined },
    { ...success(), memory: undefined },
    { ...success(), effort: undefined },
    { ...refusal("missing"), outcome: undefined },
    { ...success(), memory: { ...success().memory, updated: "bad" } },
  ];
  for (const value of malformed) await assert.rejects(decode(value), /invalid envelope/);
  const controller = new AbortController(); controller.abort();
  await activity(async (_command, _argv, options) => { assert.equal(options.signal?.aborted, true); return { stdout: line(success()) }; }, "/repo", "attach", "demo", OWNER, controller.signal);
});

test("using_effort accepts 63-byte resident slugs and rejects 64-byte slugs", () => {
  const h = harness();
  const schema = h.tool().parameters;
  assert.equal(Value.Check(schema, { effort: "r".repeat(63) }), true);
  assert.equal(Value.Check(schema, { effort: "r".repeat(64) }), false);
});

test("using_effort directly validates, attaches, context-injects cached paths, and detaches", async () => {
  const h = harness([success(), { schemaVersion: 2, condition: "detached" }]);
  for (const value of [{}, { effort: "demo", detach: true }, { effort: "Bad" }, { effort: "demo", extra: true }, { detach: false }]) await assert.rejects(request(h, value));
  assert.equal(lastText(await request(h, { effort: "demo" })), "Attached to demo.");
  assert.deepEqual(h.calls[0], ["effort", "activity", "attach", "demo", "--owner", OWNER, "--json"]);
  const cancellation = new AbortController(); const h2 = harness([success()]); await request(h2, { effort: "demo" }, cancellation.signal); assert.equal(h2.calls[0][5], OWNER); assert.equal(h2.options[0].signal, cancellation.signal);
  const context = h.hooks.get("context")({ messages: [] }, h.ctx); assert.equal(context.messages.length, 1); assert.equal(context.messages[0].display, false); assert.equal(context.messages[0].content, "[awf effort] active=demo memory=.awf/efforts/demo/memory.md managedWorktree=.awf/worktrees/demo");
  assert.equal(lastText(await request(h, { detach: true })), "Detached."); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx), undefined);
});

test("switching clears prior association on detach refusal and remains detached on attach refusal", async () => {
  const retain = harness([success(), refusal("unsafe-resident"), refusal("missing")]); await request(retain, { effort: "demo" }); assert.match(lastText(await request(retain, { effort: "other" })), /operation/); assert.equal(retain.hooks.get("context")({ messages: [] }, retain.ctx), undefined);
  const detached = harness([success(), { schemaVersion: 2, condition: "detached" }, refusal("missing", ["first", "second"])]); await request(detached, { effort: "demo" }); assert.equal(lastText(await request(detached, { effort: "other" })), "operation; resident is absent; changedActivity=false; 1. first 2. second"); assert.equal(detached.hooks.get("context")({ messages: [] }, detached.ctx), undefined);
  const repeat = harness([success(), success("attached", OWNER)]); await request(repeat, { effort: "demo" }); await request(repeat, { effort: "demo" }); assert.deepEqual(repeat.calls.map((x: any) => x[2]), ["attach", "attach"]); assert.deepEqual(repeat.calls.map((x: any) => x[5]), [OWNER, OWNER]);
});

test("heartbeat refreshes presence and clears or degrades advisory snapshots conservatively", async () => {
  for (const loss of ["not-owner", "missing"]) { const h = harness([success(), refusal(loss)]); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx), undefined); }
  const unsafe = harness([success(), refusal("unsafe-resident", ["repair"], { cause: "disk" })]); await request(unsafe, { effort: "demo" }); await unsafe.hooks.get("turn_end")({}, unsafe.ctx); assert.equal(unsafe.hooks.get("context")({ messages: [] }, unsafe.ctx), undefined);
  for (const result of [refusal("invalid-memory"), new Error("broken")]) { const h = harness([success(), result]); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); const c = h.hooks.get("context")({ messages: [] }, h.ctx); assert.equal(c.messages[0].content, "[awf effort] active=demo memory=.awf/efforts/demo/memory.md"); }
  let checks = 0; const h = harness([success(), success("heartbeat")], { directory: async () => ++checks === 1 }); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); assert.equal(checks, 2); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content.includes("managedWorktree"), false);
});

test("direct association handles impossible success, detach refusal, and shutdown cleanup", async () => {
  const wrongOwner = harness([success("attached", OTHER)]);
  await assert.rejects(request(wrongOwner, { effort: "demo" }), /incomplete attached reply/);
  const refusalDetach = harness([success(), refusal("unsafe-resident", ["retry"], { cause: "disk" })]);
  await request(refusalDetach, { effort: "demo" });
  assert.equal(lastText(await request(refusalDetach, { detach: true })), "operation; resident is absent; changedActivity=false; cause=disk; retry");
  assert.equal(refusalDetach.hooks.get("context")({ messages: [] }, refusalDetach.ctx), undefined);
  const shutdownFailure = harness([success(), new Error("disk")]);
  await request(shutdownFailure, { effort: "demo" }); await shutdownFailure.hooks.get("session_shutdown")({}, shutdownFailure.ctx);
  assert.equal(shutdownFailure.hooks.get("context")({ messages: [] }, shutdownFailure.ctx), undefined);
  const shutdownRefusal = harness([success(), refusal("unsafe-resident")]);
  await request(shutdownRefusal, { effort: "demo" }); await shutdownRefusal.hooks.get("session_shutdown")({}, shutdownRefusal.ctx);
  assert.equal(shutdownRefusal.hooks.get("context")({ messages: [] }, shutdownRefusal.ctx), undefined);
  const restart = harness([success()]); await request(restart, { effort: "demo" }); restart.hooks.get("session_start")({});
  assert.equal(restart.hooks.get("context")({ messages: [] }, restart.ctx), undefined);
  assert.deepEqual(restart.events.at(-3), ["remote-pi:metadata:set", { namespace: "awf", value: null }]);
});

test("association lifecycle covers idle turns, malformed heartbeat facts, and serialized recovery", async () => {
  const idle = harness(); await idle.hooks.get("turn_end")({}, idle.ctx);
  const mismatchedHeartbeat = harness([success(), success("heartbeat", OTHER)]);
  await request(mismatchedHeartbeat, { effort: "demo" }); await mismatchedHeartbeat.hooks.get("turn_end")({}, mismatchedHeartbeat.ctx);
  assert.equal(mismatchedHeartbeat.hooks.get("context")({ messages: [] }, mismatchedHeartbeat.ctx).messages[0].content, "[awf effort] active=demo memory=.awf/efforts/demo/memory.md");
  const serial = harness([success("attached", OTHER), success("attached", OWNER)]);
  await assert.rejects(request(serial, { effort: "demo" }), /incomplete attached reply/);
  assert.equal(lastText(await request(serial, { effort: "demo" })), "Attached to demo.");
});

test("using_effort serializes overlapping invocations in invocation order", async () => {
  let releaseFirst!: () => void;
  let firstStarted!: () => void;
  const firstSettled = new Promise<void>(resolve => { releaseFirst = resolve; });
  const firstInvoked = new Promise<void>(resolve => { firstStarted = resolve; });
  const tools = new Map<string, any>(); const hooks = new Map<string, any>(); const calls: string[][] = [];
  let active:string[]=[]; const pi: any = { getActiveTools:()=>active, setActiveTools:(x:string[])=>{active=[...x]}, withFileMutationQueue:async(_p:string,f:any)=>f(), registerTool: (tool: any) => tools.set(tool.name, tool), on: (name: string, hook: any) => hooks.set(name, hook), exec: async (_: string, args: string[]) => {
    calls.push(args); if (calls.length === 1) { firstStarted(); await firstSettled; return { code: 0, stdout: line(success("attached", OWNER, "first")), stderr: "" }; }
    if (args[2] === "detach") return { code: 0, stdout: line({ schemaVersion: 2, condition: "detached" }), stderr: "" };
    return { code: 0, stdout: line(success("attached", OWNER, "second")), stderr: "" };
  } };
  registerEffort(pi, { uuid: () => OWNER, isDirectory: async () => false, packageVersion: "0.81.1", fileMutationQueue: async (_path, work) => work() });
  const execute = tools.get("using_effort").execute;
  const first = execute("first", { effort: "first" }, new AbortController().signal, () => {}, { cwd: "/repo" });
  await firstInvoked;
  const second = execute("second", { effort: "second" }, new AbortController().signal, () => {}, { cwd: "/repo" });
  assert.deepEqual(calls.map(args => args[2]), ["attach"], "second binary invocation began before the first settled");
  releaseFirst();
  assert.equal(lastText(await first), "Attached to first.");
  assert.equal(lastText(await second), "Attached to second.");
  assert.deepEqual(calls.map(args => args[2]), ["attach", "detach", "attach"]);
  assert.match(hooks.get("context")({ messages: [] }, { cwd: "/repo" }).messages[0].content, /active=second/, "successful switch did not retain the second association");

  let releaseFailure!: () => void;
  let failureStarted!: () => void;
  const failureSettled = new Promise<void>(resolve => { releaseFailure = resolve; });
  const failureInvoked = new Promise<void>(resolve => { failureStarted = resolve; });
  const failureTools = new Map<string, any>(); const failureHooks = new Map<string, any>(); const failureCalls: string[][] = [];
  let failureActive:string[]=[]; const failurePi: any = { getActiveTools:()=>failureActive, setActiveTools:(x:string[])=>{failureActive=[...x]}, withFileMutationQueue:async(_p:string,f:any)=>f(), registerTool: (tool: any) => failureTools.set(tool.name, tool), on: (name: string, hook: any) => failureHooks.set(name, hook), exec: async (_: string, args: string[]) => {
    failureCalls.push(args); if (failureCalls.length === 1) { failureStarted(); await failureSettled; return { code: 0, stdout: line(success("attached", OWNER, "first")), stderr: "" }; }
    if (args[2] === "detach") return { code: 0, stdout: line({ schemaVersion: 2, condition: "detached" }), stderr: "" };
    return { code: 0, stdout: line(refusal("missing")), stderr: "" };
  } };
  registerEffort(failurePi, { uuid: () => OWNER, isDirectory: async () => false, packageVersion: "0.81.1", fileMutationQueue: async (_path, work) => work() });
  const failureExecute = failureTools.get("using_effort").execute;
  const attached = failureExecute("first", { effort: "first" }, new AbortController().signal, () => {}, { cwd: "/repo" });
  await failureInvoked;
  const refused = failureExecute("second", { effort: "second" }, new AbortController().signal, () => {}, { cwd: "/repo" });
  assert.deepEqual(failureCalls.map(args => args[2]), ["attach"], "failed switch began before the first attach settled");
  releaseFailure();
  await attached;
  assert.match(lastText(await refused), /operation/);
  assert.deepEqual(failureCalls.map(args => args[2]), ["attach", "detach", "attach"]);
  assert.equal(failureHooks.get("context")({ messages: [] }, { cwd: "/repo" }), undefined, "refused switch retained the detached prior association");
});

test("factory negotiation is synchronous and suffix changes preserve the complete association", async () => {
  const h = harness([success(), { schemaVersion: 2, condition: "detached" }], { factoryCapabilities: { unrelated: true, displaySuffix: { version: 1 } } });
  assert.deepEqual(h.events, [["remote-pi:capabilities:request", undefined], ["remote-pi:display-suffix:set", { value: null }]], "factory response did not run through preinstalled listeners");
  await request(h, { effort: "demo" });
  const contextBefore = h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content;
  const metadataBefore = h.events.filter(([name]: any) => name === "remote-pi:metadata:set");
  assert.deepEqual(metadataBefore.at(-1), ["remote-pi:metadata:set", { namespace: "awf", value: { effort: { slug: "demo", title: "Demo" }, memory: { phase: "Build", next: "Test", updated: TIME }, activity: { heartbeatAt: TIME } } }]);
  h.listeners.get("remote-pi:capabilities")({ displaySuffix: { version: 2 } });
  h.listeners.get("remote-pi:capabilities")({ displaySuffix: { version: 1 } });
  h.listeners.get("remote-pi:display-suffix:request")();
  assert.deepEqual(h.events.filter(([name]: any) => name === "remote-pi:metadata:set"), metadataBefore, "suffix negotiation republished or changed metadata");
  assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content, contextBefore, "suffix negotiation changed the immutable snapshot");
  assert.equal(lastText(await request(h, { detach: true })), "Detached.");
  assert.deepEqual(h.calls.at(-1), ["effort", "activity", "detach", "demo", "--owner", OWNER, "--json"], "suffix negotiation changed snapshot identity");
});

test("ownership loss clears suffix and thrown optional emissions preserve heartbeat and detach", async () => {
  const loss = harness([success(), refusal("not-owner")]);
  await request(loss, { effort: "demo" });
  loss.listeners.get("remote-pi:capabilities")({ displaySuffix: { version: 1 } });
  await loss.hooks.get("turn_end")({}, loss.ctx);
  assert.deepEqual(loss.events.slice(-2), [["remote-pi:metadata:set", { namespace: "awf", value: null }], ["remote-pi:display-suffix:set", { value: null }]]);
  assert.equal(loss.hooks.get("context")({ messages: [] }, loss.ctx), undefined);

  const broken = harness([success(), success("heartbeat"), { schemaVersion: 2, condition: "detached" }], { emitThrows: true });
  assert.equal(lastText(await request(broken, { effort: "demo" })), "Attached to demo.");
  await broken.hooks.get("turn_end")({}, broken.ctx);
  assert.match(broken.hooks.get("context")({ messages: [] }, broken.ctx).messages[0].content, /active=demo/);
  assert.equal(broken.calls.at(-1)[2], "heartbeat");
  assert.equal(lastText(await request(broken, { detach: true })), "Detached.");
  assert.equal(broken.calls.at(-1)[2], "detach");
  assert.equal(broken.hooks.get("context")({ messages: [] }, broken.ctx), undefined);
});

test("remote Pi display suffix capability, replay, lifecycle clears, and failures remain advisory", async () => {
  let defaultTool: any; const standalone: any = { getActiveTools:()=>[], setActiveTools:()=>{}, on:()=>{}, exec: async (_: string, argv: string[]) => ({ stdout: line(success("attached", argv[5], argv[3])) }), registerTool: (tool: any) => { if(tool.name==="using_effort") defaultTool = tool } }; await effortExtension(standalone); // default factory is intentionally usable
  await mkdir("/tmp/.awf/worktrees/demo", { recursive: true });
  await defaultTool.execute("id", { effort: "demo" }, new AbortController().signal, () => {}, { cwd: "/tmp" });
  let missingDirectoryTool: any; await effortExtension({ getActiveTools:()=>[], setActiveTools:()=>{}, on:()=>{}, exec: standalone.exec, registerTool: (tool: any) => { if(tool.name==="using_effort") missingDirectoryTool = tool } });
  await missingDirectoryTool.execute("id", { effort: "missing-dir" }, new AbortController().signal, () => {}, { cwd: "/definitely-missing" });
  assert.throws(() => registerEffort({ on:()=>{}, exec: async () => ({ stdout: "" }), registerTool: () => {}, getActiveTools:()=>[], setActiveTools:()=>{} } as any, { uuid: () => "bad", packageVersion: "0.81.1", fileMutationQueue: async (_path, work) => work() }), /lowercase UUIDv4/);
  const h = harness([success(), { schemaVersion: 2, condition: "detached" }, success("attached", OWNER, "other"), { schemaVersion: 2, condition: "detached" }]);
  assert.deepEqual(h.events, [["remote-pi:capabilities:request", undefined]]);
  assert.equal(typeof h.listeners.get("remote-pi:capabilities"), "function"); assert.equal(typeof h.listeners.get("remote-pi:display-suffix:request"), "function");
  await request(h, { effort: "demo" });
  const snapshot = h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content;
  h.listeners.get("remote-pi:capabilities")({ metadata: { version: 99 }, unrelated: true, displaySuffix: { version: 1 } });
  assert.deepEqual(h.events.at(-1), ["remote-pi:display-suffix:set", { value: "demo" }]);
  h.listeners.get("remote-pi:display-suffix:request")(); assert.deepEqual(h.events.at(-1), ["remote-pi:display-suffix:set", { value: "demo" }]);
  for (const caps of [null, [], { displaySuffix: [] }, { displaySuffix: { version: 1, extra: true } }, {}, { displaySuffix: null }, { displaySuffix: { version: 2 } }]) { h.listeners.get("remote-pi:capabilities")(caps); assert.deepEqual(h.events.at(-1), ["remote-pi:display-suffix:set", { value: null }]); }
  assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content, snapshot);
  h.listeners.get("remote-pi:capabilities")({ displaySuffix: { version: 1 } });
  await request(h, { effort: "other" });
  assert.deepEqual(h.events.filter(([name]: any) => name === "remote-pi:display-suffix:set").slice(-3), [["remote-pi:display-suffix:set", { value: "demo" }], ["remote-pi:display-suffix:set", { value: null }], ["remote-pi:display-suffix:set", { value: "other" }]]);
  await request(h, { detach: true }); assert.deepEqual(h.events.at(-1), ["remote-pi:display-suffix:set", { value: null }]);
  h.hooks.get("session_start")({}); assert.deepEqual(h.events.slice(-3), [["remote-pi:metadata:set", { namespace: "awf", value: null }], ["remote-pi:display-suffix:set", { value: null }], ["remote-pi:capabilities:request", undefined]]);
  await h.hooks.get("session_shutdown")({}, h.ctx); assert.deepEqual(h.events.at(-1), ["remote-pi:display-suffix:set", { value: null }]);
  const missingDetach = harness([success(), refusal("missing")]); await request(missingDetach, { effort: "demo" }); assert.equal(lastText(await request(missingDetach, { detach: true })), "Detached.");
  const takeover = harness([success("taken-over")]); assert.equal(lastText(await request(takeover, { effort: "demo" })), "Attached to demo.");
  const noCurrent = harness(); noCurrent.listeners.get("remote-pi:display-suffix:request")(); assert.deepEqual(noCurrent.events.at(-1), ["remote-pi:display-suffix:set", { value: null }]);
  const broken = harness([success()], { emitThrows: true, directory: async () => { throw new Error("stat") } }); await request(broken, { effort: "demo" }); assert.equal(broken.hooks.get("context")({ messages: [] }, broken.ctx).messages[0].content.includes("managedWorktree"), false);
});

const memoryFact = (slug = "demo") => ({ effort: slug, phase: "Build", next: "Test", updated: TIME });
const memoryReadReply = (extra: any = {}) => ({ schemaVersion: 1, condition: "read", memory: memoryFact(), content: "line\n", range: { startLine: 1, endLine: 1, totalLines: 1, nextOffset: null, truncatedBy: "none" }, ...extra });
const memoryEditReply = (extra: any = {}) => ({ schemaVersion: 1, condition: "edited", memory: memoryFact(), replacementCount: 1, diff: { text: "diff", firstChangedLine: 1, truncated: false }, ...extra });
const memoryUpdateReply = (extra: any = {}) => ({ schemaVersion: 1, condition: "updated", memory: memoryFact(), ...extra });
const memoryOutcome = (condition: string, changedMemory = condition === "memory-failure", extra: any = {}) => ({ schemaVersion: 1, condition, outcome: { category: "operation", condition: "memory state requires attention", changedMemory, nextActions: ["read memory"], ...(condition === "memory-failure" ? { cause: "publication uncertain" } : {}) }, ...extra });
async function decodeMemoryReply(value: unknown, operation: "read" | "edit" | "update" = "read") {
  const exec = async () => ({ code: 0, stdout: line(value), stderr: "" });
  if (operation === "edit") return memoryEdit(exec, "/repo", "demo", OWNER, [{ oldText: "old", newText: "new" }]);
  if (operation === "update") return memoryUpdate(exec, "/repo", "demo", OWNER, { phase: "Done" });
  const offset = (value as any)?.condition === "offset-out-of-range" ? (value as any).range?.offset : undefined;
  return memoryRead(exec, "/repo", "demo", OWNER, offset === undefined ? {} : { offset });
}

test("memory client accepts and recursively freezes every success and refusal shape", async () => {
  for (const [value, operation] of [[memoryReadReply(), "read"], [memoryEditReply(), "edit"], [memoryUpdateReply(), "update"]] as const) {
    const reply = await decodeMemoryReply(value, operation);
    assert.equal(Object.isFrozen(reply), true); assert.equal(Object.isFrozen(reply.memory), true);
    if (reply.range) assert.equal(Object.isFrozen(reply.range), true);
    if (reply.diff) assert.equal(Object.isFrozen(reply.diff), true);
  }
  const refusals = [
    memoryOutcome("not-owner"), memoryOutcome("missing"), memoryOutcome("unsafe-activity"), memoryOutcome("invalid-memory"), memoryOutcome("unsafe-memory"),
    memoryOutcome("offset-out-of-range", false, { range: { offset: 2, totalLines: 1 } }),
    memoryOutcome("no-match", false, { edit: { index: 0 } }),
    memoryOutcome("ambiguous-match", false, { edit: { index: 0, occurrences: 2 } }),
    memoryOutcome("overlapping-edits", false, { edits: { firstIndex: 0, secondIndex: 1 } }),
    memoryOutcome("result-too-large", false, { size: { bytes: 2, maxBytes: 1 } }), memoryOutcome("memory-failure"), memoryOutcome("memory-failure", false),
  ];
  for (const value of refusals) {
    const reply = await decodeMemoryReply(value);
    assert.equal(reply.condition, value.condition); assert.equal(Object.isFrozen(reply.outcome), true); assert.equal(Object.isFrozen(reply.outcome?.nextActions), true);
    for (const key of ["range", "edit", "edits", "size"] as const) if ((reply as any)[key]) assert.equal(Object.isFrozen((reply as any)[key]), true);
  }
});

test("memory client rejects every closed success and refusal boundary", async () => {
  const invalid: any[] = [
    null, [], {}, { schemaVersion: 2, condition: "read" }, { schemaVersion: 1, condition: "unknown" },
    { ...memoryReadReply(), extra: true }, { ...memoryReadReply(), memory: { ...memoryFact(), effort: "other" } }, { ...memoryReadReply(), content: "x".repeat(MEMORY_STDERR_MAX + 1) },
    { ...memoryReadReply(), range: null }, { ...memoryReadReply(), range: { startLine: 0, endLine: 1, totalLines: 1, nextOffset: null, truncatedBy: "none" } },
    { ...memoryReadReply(), range: { startLine: 2, endLine: 1, totalLines: 1, nextOffset: null, truncatedBy: "none" } }, { ...memoryReadReply(), range: { startLine: 1, endLine: 2, totalLines: 1, nextOffset: null, truncatedBy: "none" } },
    { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: 3, truncatedBy: "bytes" } }, { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: 2, truncatedBy: "other" } },
    { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: null, truncatedBy: "bytes" } }, { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: 2, truncatedBy: "none" } },
    { ...memoryEditReply(), replacementCount: 0 }, { ...memoryEditReply(), replacementCount: 129 }, { ...memoryEditReply(), memory: { ...memoryFact(), effort: "other" } },
    { ...memoryEditReply(), diff: null }, { ...memoryEditReply(), diff: { text: "x".repeat(MEMORY_STDERR_MAX + 1), firstChangedLine: 1, truncated: false } }, { ...memoryEditReply(), diff: { text: "", firstChangedLine: 0, truncated: false } }, { ...memoryEditReply(), diff: { text: "", firstChangedLine: null, truncated: "no" } },
    { ...memoryUpdateReply(), extra: true }, { ...memoryUpdateReply(), memory: { ...memoryFact(), effort: "other" } },
    memoryOutcome("not-owner", true), { ...memoryOutcome("missing"), outcome: { ...memoryOutcome("missing").outcome, cause: "forbidden" } }, { ...memoryOutcome("memory-failure"), outcome: { ...memoryOutcome("memory-failure").outcome, cause: undefined } },
    { ...memoryOutcome("missing"), extra: true }, memoryOutcome("offset-out-of-range"), memoryOutcome("offset-out-of-range", false, { range: { offset: 1, totalLines: 1 } }), memoryOutcome("offset-out-of-range", false, { range: { offset: 2, totalLines: 0 } }),
    memoryOutcome("no-match", false, { edit: { index: -1 } }), memoryOutcome("ambiguous-match", false, { edit: { index: 0, occurrences: 1 } }), memoryOutcome("overlapping-edits", false, { edits: { firstIndex: 0, secondIndex: 0 } }), memoryOutcome("result-too-large", false, { size: { bytes: 1, maxBytes: 1 } }),
  ];
  for (const value of invalid) await assert.rejects(decodeMemoryReply(value), /invalid envelope/);
});

test("memory invocation validates arguments, exact argv, stdin, and bounded transport", async () => {
  const calls: any[] = []; const exec = async (command: string, argv: readonly string[], options: any) => { calls.push([command, argv, options]); const condition = argv[2]; const readReply = argv.includes("--offset") ? { ...memoryReadReply(), range: { startLine: 2, endLine: 2, totalLines: 2, nextOffset: null, truncatedBy: "none" } } : memoryReadReply(); return { code: 0, stdout: line(condition === "read" ? readReply : condition === "edit" ? memoryEditReply() : memoryUpdateReply()), stderr: "" }; };
  const signal = new AbortController().signal;
  await memoryRead(exec, "/repo", "demo", OWNER, { offset: 2, limit: 3 }, signal);
  await memoryEdit(exec, "/repo", "demo", OWNER, [{ oldText: "old", newText: "new" }], signal);
  await memoryUpdate(exec, "/repo", "demo", OWNER, { phase: "Done", next: "Review" }, signal);
  assert.deepEqual(calls[0][1], ["effort", "memory", "read", "demo", "--offset", "2", "--limit", "3", "--owner", OWNER, "--json"]);
  assert.deepEqual(calls[1][1], ["effort", "memory", "edit", "demo", "--owner", OWNER, "--json"]); assert.equal(calls[1][2].stdin, JSON.stringify({ edits: [{ oldText: "old", newText: "new" }] }));
  assert.deepEqual(calls[2][1], ["effort", "memory", "update", "demo", "--phase", "Done", "--next", "Review", "--owner", OWNER, "--json"]); assert.equal(calls[2][2].signal, signal);
  await assert.rejects(memoryRead(exec, "/repo", "bad_slug", OWNER)); await assert.rejects(memoryRead(exec, "/repo", "demo", "bad"));
  for (const invoke of [
    () => memoryRead(exec, "/repo", "demo", OWNER, { offset: 0 }), () => memoryRead(exec, "/repo", "demo", OWNER, { limit: 1.5 }),
    () => memoryUpdate(exec, "/repo", "demo", OWNER, {}), () => memoryUpdate(exec, "/repo", "demo", OWNER, { phase: "" }), () => memoryUpdate(exec, "/repo", "demo", OWNER, { next: "x".repeat(501) }),
    () => memoryEdit(exec, "/repo", "demo", OWNER, []), () => memoryEdit(exec, "/repo", "demo", OWNER, [{ oldText: "", newText: "" }]), () => memoryEdit(exec, "/repo", "demo", OWNER, [{ oldText: "x", newText: "x", extra: true } as any]), () => memoryEdit(exec, "/repo", "demo", OWNER, Array(129).fill({ oldText: "x", newText: "" })),
  ]) assert.throws(invoke);
  const transports: Array<[any, RegExp]> = [
    [async () => { throw new Error("spawn"); }, /execution failed/], [async () => ({ code: 1, stderr: "bad" }), /memory failed/], [async () => ({ stdout: "" }), /single JSON/], [async () => ({ stdout: "{}\n{}\n" }), /single JSON/], [async () => ({ stdout: "{]\n" }), /malformed JSON/], [async () => ({ stdout: "x".repeat(MEMORY_STDOUT_MAX + 1) }), /exceeded bounds/], [async () => ({ stdout: line(memoryReadReply()), stderr: "x".repeat(MEMORY_STDERR_MAX + 1) }), /exceeded bounds/],
  ];
  for (const [transport, pattern] of transports) await assert.rejects(memoryRead(transport, "/repo", "demo", OWNER), pattern);
});

test("memory tools stay inactive while detached, preserve unrelated tools, carry native guidance, and use the shared queue", async () => {
  const replies = [memoryReadReply(), memoryEditReply(), memoryUpdateReply()];
  const memoryExec = async (_command: string, argv: readonly string[], options: any) => ({ code: 0, stdout: line(replies.shift()), stderr: "" });
  const h = harness([success(), { schemaVersion: 2, condition: "detached" }], { active: ["read", "effort_memory_read"], memoryExec });
  h.hooks.get("session_start")({}); assert.deepEqual(h.active(), ["read"]); assert.equal(h.tools.size, 4);
  for (const name of ["effort_memory_read", "effort_memory_edit", "effort_memory_update"]) { const tool = h.tools.get(name); assert.equal(Array.isArray(tool.promptGuidelines), true); assert.match(tool.promptGuidelines[0], new RegExp(name)); }
  assert.equal(Value.Check(h.tools.get("effort_memory_read").parameters, { offset: 1, limit: 2 }), true); assert.equal(Value.Check(h.tools.get("effort_memory_read").parameters, { path: "x" }), false);
  assert.equal(Value.Check(h.tools.get("effort_memory_edit").parameters, { edits: [{ oldText: "x", newText: "" }] }), true); assert.equal(Value.Check(h.tools.get("effort_memory_update").parameters, {}), true);
  await request(h, { effort: "demo" }); assert.deepEqual(h.active(), ["read", "effort_memory_read", "effort_memory_edit", "effort_memory_update"]);
  const signal = new AbortController().signal;
  assert.equal(lastText(await h.tools.get("effort_memory_read").execute("id", {}, signal, () => {}, h.ctx)), "line\n");
  assert.equal(lastText(await h.tools.get("effort_memory_edit").execute("id", { edits: [{ oldText: "old", newText: "new" }] }, signal, () => {}, h.ctx)), "diff");
  assert.equal(lastText(await h.tools.get("effort_memory_update").execute("id", { phase: "Done" }, signal, () => {}, h.ctx)), "Memory metadata updated.");
  assert.deepEqual(h.queueCalls, ["/repo/.awf/efforts/demo/memory.md", "/repo/.awf/efforts/demo/memory.md"]);
  await request(h, { detach: true }); assert.deepEqual(h.active(), ["read"]);
  await assert.rejects(h.tools.get("effort_memory_read").execute("id", {}, signal, () => {}, h.ctx), /Attach an effort/);
  await assert.rejects(h.tools.get("effort_memory_update").execute("id", {}, signal, () => {}, h.ctx), /phase or next/);
});

test("memory refusals render the memory axis and ownership losses clear only companion state", async () => {
  for (const condition of ["not-owner", "missing", "unsafe-activity"]) {
    const memoryExec = async () => ({ code: 0, stdout: line(memoryOutcome(condition)), stderr: "" });
    const h = harness([success()], { active: ["read"], memoryExec }); await request(h, { effort: "demo" });
    const result = await h.tools.get("effort_memory_read").execute("id", {}, new AbortController().signal, () => {}, h.ctx);
    assert.match(lastText(result), /changedMemory=false/); assert.equal(lastText(result).includes("changedActivity"), false); assert.deepEqual(h.active(), ["read"]); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx), undefined);
  }
  const retained = harness([success()], { memoryExec: async () => ({ code: 0, stdout: line(memoryOutcome("invalid-memory")), stderr: "" }) }); await request(retained, { effort: "demo" });
  const refusal = await retained.tools.get("effort_memory_read").execute("id", {}, new AbortController().signal, () => {}, retained.ctx); assert.match(lastText(refusal), /changedMemory=false/); assert.ok(retained.hooks.get("context")({ messages: [] }, retained.ctx));
  const uncertain = harness([success()], { memoryExec: async () => ({ code: 0, stdout: line(memoryOutcome("memory-failure")), stderr: "" }) }); await request(uncertain, { effort: "demo" });
  assert.match(lastText(await uncertain.tools.get("effort_memory_read").execute("id", {}, new AbortController().signal, () => {}, uncertain.ctx)), /cause=publication uncertain/);
});

test("explicit detach and unsafe heartbeat clear association and memory tools", async () => {
  const detached = harness([success(), refusal("unsafe-resident")], { active: ["read"] }); await request(detached, { effort: "demo" }); assert.match(lastText(await request(detached, { detach: true })), /operation/); assert.deepEqual(detached.active(), ["read"]); assert.equal(detached.hooks.get("context")({ messages: [] }, detached.ctx), undefined);
  const heartbeat = harness([success(), refusal("unsafe-resident")], { active: ["read"] }); await request(heartbeat, { effort: "demo" }); await heartbeat.hooks.get("turn_end")({}, heartbeat.ctx); assert.deepEqual(heartbeat.active(), ["read"]); assert.equal(heartbeat.hooks.get("context")({ messages: [] }, heartbeat.ctx), undefined);
});

class FakeStream extends EventEmitter { override on(event: string, listener: (...args: any[]) => void) { return super.on(event, listener); } }
class FakeStdin extends EventEmitter { values: string[] = []; destroyed = false; throwOnEnd = false; end(value = "") { if (this.throwOnEnd) throw new Error("end"); this.values.push(value); } destroy() { this.destroyed = true; } override once(event: string, listener: (...args: any[]) => void) { return super.once(event, listener); } }
class FakeChild extends EventEmitter { pid: number | undefined = 123; stdin = new FakeStdin(); stdout = new FakeStream(); stderr = new FakeStream(); signals: string[] = []; kill(signal: any) { this.signals.push(signal); return true; } override once(event: string, listener: (...args: any[]) => void) { return super.once(event, listener); } }
function childHarness(options: { throwSpawn?: boolean; throwEnd?: boolean } = {}) { const child = new FakeChild(); child.stdin.throwOnEnd = Boolean(options.throwEnd); const timers: Array<{ callback: () => void; delay: number; cleared: boolean }> = []; const kills: string[] = []; const deps: any = { spawn: (_command: string, _argv: readonly string[], spawnOptions: any) => { assert.deepEqual(spawnOptions, { cwd: "/repo", shell: false, detached: true, stdio: ["pipe", "pipe", "pipe"] }); if (options.throwSpawn) throw new Error("spawn"); return child; }, setTimer: (callback: () => void, delay: number) => { const timer = { callback, delay, cleared: false }; timers.push(timer); return timer; }, clearTimer: (timer: any) => { timer.cleared = true; }, kill: (_child: any, signal: string) => { kills.push(signal); } }; return { child, timers, kills, exec: createChildMemoryExecutor(deps) }; }

test("stdin child executor settles success, spawn failure, process error, and pre-abort without leaked handles", async () => {
  const successHarness = childHarness(); const successPromise = successHarness.exec("./awf", ["x"], { cwd: "/repo", timeout: 15, stdin: "request" }); successHarness.child.stdout.emit("data", "out"); successHarness.child.stderr.emit("data", Buffer.from("err")); successHarness.child.emit("close", 0); assert.deepEqual(await successPromise, { code: 0, stdout: "out", stderr: "err" }); assert.deepEqual(successHarness.child.stdin.values, ["request"]); assert.equal(successHarness.timers.every((timer) => timer.cleared), true); assert.equal(successHarness.child.listenerCount("close"), 0);
  await assert.rejects(childHarness({ throwSpawn: true }).exec("./awf", [], { cwd: "/repo", timeout: 1 }), /spawn/);
  const errorHarness = childHarness(); let errorSettled = false; const errorPromise = errorHarness.exec("./awf", [], { cwd: "/repo", timeout: 1 }).catch((error) => { errorSettled = true; throw error; }); errorHarness.child.emit("error", new Error("child")); await Promise.resolve(); assert.equal(errorSettled, false); errorHarness.child.emit("close", null); await assert.rejects(errorPromise, /child/); assert.equal(errorHarness.timers.every((timer) => timer.cleared), true);
  const controller = new AbortController(); controller.abort(); const pre = childHarness(); await assert.rejects(pre.exec("./awf", [], { cwd: "/repo", timeout: 1, signal: controller.signal }), /before start/); assert.equal(pre.timers.length, 0);
});

test("stdin child executor terminates process groups on abort, timeout, overflow, and stdin failure", async () => {
  const cases: Array<[string, (h: ReturnType<typeof childHarness>, controller: AbortController) => void, RegExp]> = [
    ["abort", (_h, controller) => controller.abort(), /completion is unknown/],
    ["timeout", (h) => h.timers[0].callback(), /timed out/],
    ["stdout", (h) => h.child.stdout.emit("data", "x".repeat(MEMORY_STDOUT_MAX + 1)), /stdout exceeded/],
    ["stderr", (h) => h.child.stderr.emit("data", "x".repeat(MEMORY_STDERR_MAX + 1)), /stderr exceeded/],
    ["stdin", (h) => h.child.stdin.emit("error", new Error("pipe")), /stdin failed/],
  ];
  for (const [_name, trigger, pattern] of cases) { const h = childHarness(); const controller = new AbortController(); const promise = h.exec("./awf", [], { cwd: "/repo", timeout: 20, signal: controller.signal, stdin: "x" }); trigger(h, controller); assert.deepEqual(h.kills, ["SIGTERM"]); assert.equal(h.child.stdin.destroyed, true); const killTimer = h.timers.at(-1)!; assert.equal(killTimer.delay, MEMORY_KILL_DELAY_MS); killTimer.callback(); assert.deepEqual(h.kills, ["SIGTERM", "SIGKILL"]); h.child.emit("close", null); await assert.rejects(promise, pattern); assert.equal(h.timers.every((timer) => timer.cleared), true); }
  const ended = childHarness({ throwEnd: true }); const promise = ended.exec("./awf", [], { cwd: "/repo", timeout: 20 }); ended.child.emit("close", null); await assert.rejects(promise, /stdin failed/);
});

test("production stdin child executor writes stdin and uses detached direct spawn", async () => {
  const result = await createChildMemoryExecutor(productionChildMemoryDependencies)(process.execPath, ["-e", "process.stdin.setEncoding('utf8');let x='';process.stdin.on('data',d=>x+=d);process.stdin.on('end',()=>{process.stdout.write(x);process.stderr.write('e')})"], { cwd: process.cwd(), timeout: 5000, stdin: "production" });
  assert.deepEqual(result, { code: 0, stdout: "production", stderr: "e" });
});

test("effort runtime guard checks actual version, active-tool methods, and package-exported queue before registration", async () => {
  const marker = Symbol.for("awf.pi.minimum-runtime-notified"); const original = (globalThis as any)[marker];
  const guarded = (overrides: any = {}, deps: any = {}) => { const tools: any[] = []; const hooks = new Map<string, any>(); const notices: any[] = []; const pi: any = { on: (name: string, hook: any) => hooks.set(name, hook), registerTool: (tool: any) => tools.push(tool), exec: async () => ({ stdout: "" }), getActiveTools: () => [], setActiveTools: () => {}, ...overrides }; registerEffort(pi, { packageVersion: "0.81.1", fileMutationQueue: async (_path, work) => work(), ...deps }); return { tools, hooks, notices, start: () => hooks.get("session_start")?.({}, { ui: { notify: (...args: any[]) => notices.push(args) } }) }; };
  try {
    delete (globalThis as any)[marker]; const old = guarded({}, { packageVersion: "0.80.0" }); assert.deepEqual(old.tools, []); await old.start(); assert.match(old.notices[0][0], /found 0.80.0/); await old.start(); assert.equal(old.notices.length, 1);
    delete (globalThis as any)[marker]; const helper = guarded({}, { fileMutationQueue: undefined }); assert.deepEqual(helper.tools, []); await helper.start(); assert.match(helper.notices[0][0], /withFileMutationQueue/); await helper.start(); assert.equal(helper.notices.length, 1);
    delete (globalThis as any)[marker]; const methods = guarded({ setActiveTools: undefined }); assert.deepEqual(methods.tools, []); await methods.start(); assert.match(methods.notices[0][0], /setActiveTools/);
    const noHook = guarded({ on: undefined }, { packageVersion: "bad", fileMutationQueue: undefined }); assert.deepEqual(noHook.tools, []); assert.equal(noHook.hooks.size, 0);
    const noSharedHook = guarded({ on: undefined }, { packageVersion: "bad" }); assert.deepEqual(noSharedHook.tools, []); assert.equal(noSharedHook.hooks.size, 0);
  } finally { if (original === undefined) delete (globalThis as any)[marker]; else (globalThis as any)[marker] = original; }
});

test("default effort registration derives package facts and guards missing or unreadable runtime exports", async () => {
  const makePi = () => { const tools: any[] = []; const hooks = new Map<string, any>(); let active: string[] = []; return { tools, hooks, pi: { on: (name: string, hook: any) => hooks.set(name, hook), registerTool: (tool: any) => tools.push(tool), exec: async () => ({ stdout: "" }), getActiveTools: () => active, setActiveTools: (value: string[]) => { active = value; } } as any }; };
  const queue = async (_path: string, work: any) => work();
  const supported = makePi(); await registerDefaultEffort(supported.pi, { getPackageDir: () => "/pkg", withFileMutationQueue: queue }, (async (path: any, encoding: any) => { assert.equal(path, "/pkg/package.json"); assert.equal(encoding, "utf8"); return JSON.stringify({ version: "0.81.1" }); }) as any); assert.equal(supported.tools.length, 4);
  const marker = Symbol.for("awf.pi.minimum-runtime-notified"); const original = (globalThis as any)[marker];
  try {
    delete (globalThis as any)[marker]; const unreadable = makePi(); await registerDefaultEffort(unreadable.pi, {}, (async () => { throw new Error("read"); }) as any); assert.deepEqual(unreadable.tools, []); const notices: any[] = []; await unreadable.hooks.get("session_start")({}, { ui: { notify: (...args: any[]) => notices.push(args) } }); assert.match(notices[0][0], /found unknown/);
    delete (globalThis as any)[marker]; const malformed = makePi(); await registerDefaultEffort(malformed.pi, { getPackageDir: () => "/pkg", withFileMutationQueue: queue }, (async () => JSON.stringify({ version: 81 })) as any); assert.deepEqual(malformed.tools, []);
  } finally { if (original === undefined) delete (globalThis as any)[marker]; else (globalThis as any)[marker] = original; }
});

test("default memory adapter routes owner-scoped read to pi.exec and edit stdin to the child adapter", async () => {
  const tools = new Map<string, any>(); let active: string[] = []; const calls: any[] = []; const pi: any = { on: () => {}, getActiveTools: () => active, setActiveTools: (value: string[]) => { active = value; }, registerTool: (tool: any) => tools.set(tool.name, tool), exec: async (_command: string, argv: readonly string[]) => { calls.push(argv); return { code: 0, stdout: line(argv[1] === "activity" ? success("attached", OWNER, "demo") : memoryReadReply()), stderr: "" }; } };
  registerEffort(pi, { uuid: () => OWNER, packageVersion: "0.81.1", fileMutationQueue: async (_path, work) => work(), isDirectory: async () => false }); const ctx = { cwd: "/definitely-missing" }; const signal = new AbortController().signal;
  await tools.get("using_effort").execute("id", { effort: "demo" }, signal, () => {}, ctx); assert.equal(lastText(await tools.get("effort_memory_read").execute("id", {}, signal, () => {}, ctx)), "line\n"); assert.equal(calls.at(-1)[1], "memory");
  await assert.rejects(tools.get("effort_memory_edit").execute("id", { edits: [{ oldText: "old", newText: "new" }] }, signal, () => {}, ctx), /execution failed/);
});

test("memory rendering numbers multiple next actions on its own mutation axis", async () => {
  const memoryExec = async () => ({ code: 0, stdout: line({ ...memoryOutcome("invalid-memory"), outcome: { ...memoryOutcome("invalid-memory").outcome, nextActions: ["first", "second"] } }), stderr: "" });
  const h = harness([success()], { memoryExec }); await request(h, { effort: "demo" }); const text = lastText(await h.tools.get("effort_memory_read").execute("id", {}, new AbortController().signal, () => {}, h.ctx)); assert.match(text, /1\. first 2\. second/);
});

test("memory client covers optional argument branches, aggregate request cap, and UTF-8 diagnostic truncation", async () => {
  const nextExec = async () => ({ stdout: line(memoryUpdateReply()), stderr: "" }); await memoryUpdate(nextExec, "/repo", "demo", OWNER, { next: "Only next" });
  const huge = "x".repeat(MEMORY_STDOUT_MAX); assert.throws(() => memoryEdit(nextExec, "/repo", "demo", OWNER, Array(17).fill({ oldText: huge, newText: huge })), /request exceeded/);
  let error: any; try { await memoryRead(async () => { throw new Error("🙂".repeat(MEMORY_STDERR_MAX)); }, "/repo", "demo", OWNER); } catch (caught) { error = caught; } assert.ok(error instanceof EffortProtocolError); assert.ok(Buffer.byteLength(error.stderr, "utf8") <= MEMORY_STDERR_MAX);
  await assert.rejects(memoryRead(async () => ({ stdout: "\n" }), "/repo", "demo", OWNER), /single JSON/);
});

test("child executor ignores repeated terminal triggers and production group killing falls back safely", async () => {
  const h = childHarness(); const promise = h.exec("./awf", [], { cwd: "/repo", timeout: 5 }); h.child.stdout.emit("data", "x".repeat(MEMORY_STDOUT_MAX + 1)); h.child.stderr.emit("data", "x".repeat(MEMORY_STDERR_MAX + 1)); h.child.stdin.destroy = () => { throw new Error("destroy"); }; h.child.stdin.emit("error", new Error("late")); const killTimer = h.timers.at(-1)!; h.child.emit("close", null); await assert.rejects(promise, /stdout exceeded/); killTimer.callback(); assert.deepEqual(h.kills, ["SIGTERM"]);
  const noPid = new FakeChild(); noPid.pid = undefined; productionChildMemoryDependencies.kill(noPid as any, "SIGTERM"); assert.deepEqual(noPid.signals, ["SIGTERM"]);
  const missingPid = new FakeChild(); missingPid.pid = 2147483647; productionChildMemoryDependencies.kill(missingPid as any, "SIGKILL"); assert.deepEqual(missingPid.signals, ["SIGKILL"]);
});

test("child executor terminal guards tolerate close and output races", async () => {
  const settled = childHarness(); const promise = settled.exec("./awf", [], { cwd: "/repo", timeout: 5 }); const close = settled.child.listeners("close")[0] as (code: number | null) => void; const error = settled.child.listeners("error")[0] as (error: Error) => void; const stdout = settled.child.stdout.listeners("data")[0] as (chunk: string) => void; settled.child.emit("close", 0); assert.equal((await promise).code, 0); close(0); error(new Error("late")); stdout("x".repeat(MEMORY_STDOUT_MAX + 1)); assert.deepEqual(settled.kills, []);
  const destroy = childHarness(); destroy.child.stdin.destroy = () => { throw new Error("destroy"); }; const failed = destroy.exec("./awf", [], { cwd: "/repo", timeout: 5 }); destroy.child.stdout.emit("data", "x".repeat(MEMORY_STDOUT_MAX + 1)); destroy.child.emit("close", null); await assert.rejects(failed, /stdout exceeded/); assert.deepEqual(destroy.kills, ["SIGTERM"]);
  const errorDestroy = childHarness(); errorDestroy.child.stdin.destroy = () => { throw new Error("destroy"); }; const errorFailed = errorDestroy.exec("./awf", [], { cwd: "/repo", timeout: 5 }); errorDestroy.child.emit("error", new Error("child error")); errorDestroy.child.emit("close", null); await assert.rejects(errorFailed, /child error/);
});

test("effort guard treats an omitted package version as incompatible", async () => {
  const hooks = new Map<string, any>(); const tools: any[] = []; let active: string[] = []; registerEffort({ on: (name: string, hook: any) => hooks.set(name, hook), registerTool: (tool: any) => tools.push(tool), exec: async () => ({ stdout: "" }), getActiveTools: () => active, setActiveTools: (value: string[]) => { active = value; } }, { fileMutationQueue: async (_path, work) => work() }); assert.deepEqual(tools, []); assert.equal(typeof hooks.get("session_start"), "function");
});

test("activity and memory outcomes retain separate cardinality and cause bounds", async () => {
  const manyActions = Array.from({ length: 9 }, (_, index) => `action ${index}`);
  assert.equal((await decode(refusal("missing", manyActions))).outcome?.nextActions.length, 9);
  await assert.rejects(decode(refusal("missing", ["recover"], { cause: "x".repeat(4097) })), /invalid envelope/);
  await assert.rejects(decodeMemoryReply({ ...memoryOutcome("invalid-memory"), outcome: { ...memoryOutcome("invalid-memory").outcome, nextActions: manyActions } }), /invalid envelope/);
  const longCause = "x".repeat(4097); const failure = { ...memoryOutcome("memory-failure"), outcome: { ...memoryOutcome("memory-failure").outcome, cause: longCause } }; assert.equal((await decodeMemoryReply(failure)).outcome?.cause, longCause);
});

test("read decoder ties range facts to the request, selected content, and remaining document", async () => {
  const exec = (value: unknown) => async () => ({ stdout: line(value), stderr: "" });
  const valid = { ...memoryReadReply(), content: "second\n", range: { startLine: 2, endLine: 2, totalLines: 3, nextOffset: 3, truncatedBy: "limit" } };
  assert.equal((await memoryRead(exec(valid), "/repo", "demo", OWNER, { offset: 2, limit: 1 })).range?.startLine, 2);
  assert.equal((await memoryRead(exec({ ...memoryReadReply(), content: "" }), "/repo", "demo", OWNER)).content, "");
  assert.equal((await memoryRead(exec({ ...memoryReadReply(), content: "line" }), "/repo", "demo", OWNER)).content, "line");
  const invalid = [
    { ...valid, range: { ...valid.range, startLine: 1 } },
    { ...valid, content: "second\nthird\n" },
    { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: null, truncatedBy: "none" } },
    { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 1, nextOffset: 2, truncatedBy: "bytes" } },
  ];
  for (const value of invalid) await assert.rejects(memoryRead(exec(value), "/repo", "demo", OWNER, { offset: value === invalid[0] || value === invalid[1] ? 2 : 1 }), /invalid envelope/);
  const mismatchedOffset = memoryOutcome("offset-out-of-range", false, { range: { offset: 3, totalLines: 1 } }); await assert.rejects(memoryRead(exec(mismatchedOffset), "/repo", "demo", OWNER, { offset: 2 }), /invalid envelope/);
  const overLimit = { ...memoryReadReply(), content: "one\ntwo\n", range: { startLine: 1, endLine: 2, totalLines: 3, nextOffset: 3, truncatedBy: "limit" } }; await assert.rejects(memoryRead(exec(overLimit), "/repo", "demo", OWNER, { limit: 1 }), /invalid envelope/);
  const overLineCap = { ...memoryReadReply(), content: "\n".repeat(2001), range: { startLine: 1, endLine: 2001, totalLines: 2002, nextOffset: 2002, truncatedBy: "lines" } }; await assert.rejects(memoryRead(exec(overLineCap), "/repo", "demo", OWNER), /invalid envelope/);
  const exactLineCap = { ...memoryReadReply(), content: "\n".repeat(2000), range: { startLine: 1, endLine: 2000, totalLines: 2001, nextOffset: 2001, truncatedBy: "lines" } }; assert.equal((await memoryRead(exec(exactLineCap), "/repo", "demo", OWNER)).range?.endLine, 2000);
  const limitWithoutRequest = { ...memoryReadReply(), range: { startLine: 1, endLine: 1, totalLines: 2, nextOffset: 2, truncatedBy: "limit" } }; await assert.rejects(memoryRead(exec(limitWithoutRequest), "/repo", "demo", OWNER), /invalid envelope/);
  const zeroMaximum = memoryOutcome("result-too-large", false, { size: { bytes: 1, maxBytes: 0 } }); assert.equal((await decodeMemoryReply(zeroMaximum)).condition, "result-too-large");
});

test("SIGKILL keeps the executor pending for close and uses only a final bounded fallback", async () => {
  const observed = childHarness(); let settled = false; const promise = observed.exec("./awf", [], { cwd: "/repo", timeout: 5 }).catch((error) => { settled = true; throw error; }); observed.timers[0].callback(); observed.timers[1].callback(); observed.child.emit("error", new Error("after kill")); await Promise.resolve(); assert.equal(settled, false); assert.deepEqual(observed.kills, ["SIGTERM", "SIGKILL"]); assert.equal(observed.timers[2].delay, MEMORY_CLOSE_DELAY_MS); observed.child.emit("close", null); await assert.rejects(promise, /after kill/);
  const fallback = childHarness(); const fallbackPromise = fallback.exec("./awf", [], { cwd: "/repo", timeout: 5 }); fallback.timers[0].callback(); fallback.timers[1].callback(); fallback.timers[2].callback(); await assert.rejects(fallbackPromise, /timed out/); assert.equal(fallback.timers.every((timer) => timer.cleared), true);
});
