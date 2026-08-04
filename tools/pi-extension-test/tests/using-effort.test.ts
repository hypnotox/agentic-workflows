import assert from "node:assert/strict";
import test from "node:test";
import { mkdir } from "node:fs/promises";
import { Value } from "typebox/value";
import { activity, EffortProtocolError } from "../../../.pi/extensions/awf-effort/client.ts";
import effortExtension, { registerEffort } from "../../../.pi/extensions/awf-effort/index.ts";

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

function harness(replies: any[] = [], opts: { directory?: any; emitThrows?: boolean } = {}) {
  const tools = new Map<string, any>(), hooks = new Map<string, any>(), listeners = new Map<string, any>(); const events: any[] = [], calls: any[] = [], options: any[] = [];
  const pi: any = { registerTool: (x: any) => tools.set(x.name, x), on: (n: string, h: any) => hooks.set(n, h), events: { emit: (n: string, p: any) => { if (opts.emitThrows) throw new Error("emit"); events.push([n, p]); }, on: (n: string, h: any) => listeners.set(n, h) }, exec: async (_: string, args: string[], option: any) => { calls.push(args); options.push(option); const r = replies.shift() ?? success(args[2] as any, args[5], args[3]); if (r instanceof Error) throw r; return { code: 0, stdout: typeof r === "string" ? r : line(r), stderr: "" }; } };
  let n = 0; registerEffort(pi, { uuid: () => n++ ? OTHER : OWNER, isDirectory: opts.directory ?? (async () => true) });
  const tool = () => tools.get("using_effort"); const ctx = { cwd: "/repo" };
  return { pi, tools, hooks, listeners, events, calls, options, tool, ctx };
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

test("switching preserves prior association on detach refusal and otherwise remains detached on attach refusal", async () => {
  const retain = harness([success(), refusal("unsafe-resident")]); await request(retain, { effort: "demo" }); assert.match(lastText(await request(retain, { effort: "other" })), /operation/); await retain.hooks.get("turn_end")({}, retain.ctx); assert.equal(retain.calls.at(-1)[2], "heartbeat");
  const detached = harness([success(), { schemaVersion: 2, condition: "detached" }, refusal("missing", ["first", "second"])]); await request(detached, { effort: "demo" }); assert.equal(lastText(await request(detached, { effort: "other" })), "operation; resident is absent; changedActivity=false; 1. first 2. second"); assert.equal(detached.hooks.get("context")({ messages: [] }, detached.ctx), undefined);
  const repeat = harness([success(), success("attached", OWNER)]); await request(repeat, { effort: "demo" }); await request(repeat, { effort: "demo" }); assert.deepEqual(repeat.calls.map((x: any) => x[2]), ["attach", "attach"]); assert.deepEqual(repeat.calls.map((x: any) => x[5]), [OWNER, OWNER]);
});

test("heartbeat refreshes presence and clears or degrades advisory snapshots conservatively", async () => {
  for (const loss of ["not-owner", "missing"]) { const h = harness([success(), refusal(loss)]); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx), undefined); }
  for (const result of [refusal("invalid-memory"), refusal("unsafe-resident", ["repair"], { cause: "disk" }), new Error("broken")]) { const h = harness([success(), result]); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); const c = h.hooks.get("context")({ messages: [] }, h.ctx); assert.equal(c.messages[0].content, "[awf effort] active=demo memory=.awf/efforts/demo/memory.md"); }
  let checks = 0; const h = harness([success(), success("heartbeat")], { directory: async () => ++checks === 1 }); await request(h, { effort: "demo" }); await h.hooks.get("turn_end")({}, h.ctx); assert.equal(checks, 2); assert.equal(h.hooks.get("context")({ messages: [] }, h.ctx).messages[0].content.includes("managedWorktree"), false);
});

test("direct association handles impossible success, detach refusal, and shutdown cleanup", async () => {
  const wrongOwner = harness([success("attached", OTHER)]);
  await assert.rejects(request(wrongOwner, { effort: "demo" }), /incomplete attached reply/);
  const refusalDetach = harness([success(), refusal("unsafe-resident", ["retry"], { cause: "disk" })]);
  await request(refusalDetach, { effort: "demo" });
  assert.equal(lastText(await request(refusalDetach, { detach: true })), "operation; resident is absent; changedActivity=false; cause=disk; retry");
  assert.match((refusalDetach.hooks.get("context")({ messages: [] }, refusalDetach.ctx)).messages[0].content, /active=demo/);
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
  const pi: any = { registerTool: (tool: any) => tools.set(tool.name, tool), on: (name: string, hook: any) => hooks.set(name, hook), exec: async (_: string, args: string[]) => {
    calls.push(args); if (calls.length === 1) { firstStarted(); await firstSettled; return { code: 0, stdout: line(success("attached", OWNER, "first")), stderr: "" }; }
    if (args[2] === "detach") return { code: 0, stdout: line({ schemaVersion: 2, condition: "detached" }), stderr: "" };
    return { code: 0, stdout: line(success("attached", OWNER, "second")), stderr: "" };
  } };
  registerEffort(pi, { uuid: () => OWNER, isDirectory: async () => false });
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
  const failurePi: any = { registerTool: (tool: any) => failureTools.set(tool.name, tool), on: (name: string, hook: any) => failureHooks.set(name, hook), exec: async (_: string, args: string[]) => {
    failureCalls.push(args); if (failureCalls.length === 1) { failureStarted(); await failureSettled; return { code: 0, stdout: line(success("attached", OWNER, "first")), stderr: "" }; }
    if (args[2] === "detach") return { code: 0, stdout: line({ schemaVersion: 2, condition: "detached" }), stderr: "" };
    return { code: 0, stdout: line(refusal("missing")), stderr: "" };
  } };
  registerEffort(failurePi, { uuid: () => OWNER, isDirectory: async () => false });
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

test("remote Pi display suffix capability, replay, lifecycle clears, and failures remain advisory", async () => {
  let defaultTool: any; const standalone: any = { exec: async (_: string, argv: string[]) => ({ stdout: line(success("attached", argv[5], argv[3])) }), registerTool: (tool: any) => { defaultTool = tool } }; effortExtension(standalone); // default factory is intentionally usable
  await mkdir("/tmp/.awf/worktrees/demo", { recursive: true });
  await defaultTool.execute("id", { effort: "demo" }, new AbortController().signal, () => {}, { cwd: "/tmp" });
  let missingDirectoryTool: any; effortExtension({ exec: standalone.exec, registerTool: (tool: any) => { missingDirectoryTool = tool } });
  await missingDirectoryTool.execute("id", { effort: "missing-dir" }, new AbortController().signal, () => {}, { cwd: "/definitely-missing" });
  assert.throws(() => registerEffort({ exec: async () => ({ stdout: "" }), registerTool: () => {} } as any, { uuid: () => "bad" }), /lowercase UUIDv4/);
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
