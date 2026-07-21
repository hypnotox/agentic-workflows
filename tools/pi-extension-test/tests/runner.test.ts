import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { resolve } from "node:path";
import { PassThrough } from "node:stream";
import test from "node:test";
import {
  KILL_DELAY_MS,
  MAX_DISPLAY_EVENT_BYTES,
  MAX_DISPLAY_EVENTS,
  MAX_FAILURE_BYTES,
  MAX_OUTPUT_BYTES,
  MAX_OUTPUT_LINES,
  MAX_STDERR_BYTES,
  createRunner,
  fitDisplayEvent,
  productionRunnerDependencies,
  resolvePiInvocation,
  truncateField,
  truncateOutput,
  truncateStderr,
  type RunnerDependencies,
  type SpawnedProcess,
} from "../../../.pi/extensions/awf-subagents/runner.ts";

class FakeProcess extends EventEmitter implements SpawnedProcess {
  stdout = new PassThrough();
  stderr = new PassThrough();
  signals: string[] = [];
  kill(signal: "SIGTERM" | "SIGKILL"): boolean { this.signals.push(signal); return true; }
  close(code = 0): void { this.emit("close", code); }
  fail(error: Error): void { this.emit("error", error); }
}

function harness(process = new FakeProcess()) {
  const writes: Array<{ path: string; mode: number }> = [];
  const removals: string[] = [];
  const timers: Array<() => void> = [];
  let madeDirectories = 0;
  const deps: RunnerDependencies = {
    spawn: () => process,
    mkdtemp: async () => { madeDirectories++; return "/tmp/child"; },
    writeFile: async (path, _content, options) => { writes.push({ path, mode: options.mode }); },
    rm: async (path) => { removals.push(path); },
    setTimer: (callback, delay) => { assert.equal(delay, KILL_DELAY_MS); timers.push(callback); return 1 as any; },
    clearTimer: () => {},
    argv: ["node", "/pi/cli.js"],
    execPath: "/usr/bin/node",
    tempRoot: "/tmp",
  };
  const request = {
    role: "explore" as const,
    task: "inspect",
    cwd: "/repo",
    model: { provider: "test", id: "model" },
    thinkingLevel: "high" as const,
    tools: ["read"],
    systemPrompt: "system",
  };
  return { process, deps, request, writes, removals, timers, madeDirectories: () => madeDirectories };
}

function message(text: string, stopReason = "end", options: { provider?: string; model?: string; responseModel?: string; errorMessage?: string; usage?: Record<string, unknown> } = {}) {
  return JSON.stringify({
    type: "message_end",
    message: {
      role: "assistant",
      content: text ? [{ type: "text", text }] : [],
      provider: options.provider,
      model: options.model ?? "child",
      responseModel: options.responseModel,
      stopReason,
      errorMessage: options.errorMessage,
      usage: options.usage ?? { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: { total: 0.25 } },
    },
  });
}
function toolStart(id: string, name: string, args: unknown) {
  return JSON.stringify({ type: "tool_execution_start", toolCallId: id, toolName: name, args });
}
function toolEnd(id: string, name: string, isError: boolean) {
  return JSON.stringify({ type: "tool_execution_end", toolCallId: id, toolName: name, isError });
}

test("production runner executes the fake Pi fixture", async () => {
  const fixture = resolve("tools/pi-extension-test/fixtures/fake-pi.mjs");
  const runner = createRunner({ ...productionRunnerDependencies, argv: ["node", fixture], execPath: process.execPath });
  const result = await runner.run({
    role: "explore", task: "inspect", cwd: process.cwd(),
    model: { provider: "test", id: "model" }, thinkingLevel: "high", tools: ["read"], systemPrompt: "system",
  });
  assert.equal(result.output, "fixture output");
  assert.deepEqual(result.events.map((event) => event.kind), ["tool-start", "tool-end", "assistant"]);
});

test("production runner escalates a TERM-resistant fixture", async () => {
  const fixture = resolve("tools/pi-extension-test/fixtures/term-resistant-pi.mjs");
  const signals: string[] = [];
  const deps: RunnerDependencies = {
    ...productionRunnerDependencies,
    argv: ["node", fixture],
    execPath: process.execPath,
    spawn: (command, args, options) => {
      const child = productionRunnerDependencies.spawn(command, args, options);
      const kill = child.kill.bind(child);
      child.kill = (signal) => { signals.push(signal); return kill(signal); };
      return child;
    },
    setTimer: (callback) => setImmediate(callback) as any,
    clearTimer: (timer) => clearImmediate(timer as any),
  };
  const controller = new AbortController();
  const pending = createRunner(deps).run({
    role: "implement", task: "wait", cwd: process.cwd(),
    model: { provider: "test", id: "model" }, thinkingLevel: "high", tools: ["bash"], systemPrompt: "system", signal: controller.signal,
  });
  await new Promise((resolve) => setTimeout(resolve, 50));
  controller.abort();
  const result = await pending;
  assert.equal(result.failed, true);
  assert.equal(result.stopReason, "aborted");
  assert.deepEqual(signals, ["SIGTERM", "SIGKILL"]);
});

test("invocation resolution covers script, generic runtime fallback, and binary", () => {
  assert.deepEqual(resolvePiInvocation(["node", "/pi/cli.js"], "/usr/bin/node"), { command: "/usr/bin/node", args: ["/pi/cli.js"] });
  assert.deepEqual(resolvePiInvocation(["bun", "/$bunfs/root/pi"], "/usr/bin/bun"), { command: "pi", args: [] });
  assert.deepEqual(resolvePiInvocation(["node", "node:virtual"], "node.exe"), { command: "pi", args: [] });
  assert.deepEqual(resolvePiInvocation(["pi"], "/usr/bin/pi"), { command: "/usr/bin/pi", args: [] });
});

test("output and field truncation is UTF-8 safe and bounded", () => {
  assert.equal(truncateOutput("ok"), "ok");
  assert.match(truncateOutput("x".repeat(MAX_OUTPUT_BYTES + 10)), /10 bytes/);
  assert.match(truncateOutput(Array(MAX_OUTPUT_LINES + 3).fill("x").join("\n")), /3 lines/);
  assert.match(truncateOutput("é".repeat(MAX_OUTPUT_BYTES)), /Output truncated/);
  assert.equal(truncateStderr("ok"), "ok");
  assert.match(truncateStderr("x".repeat(MAX_STDERR_BYTES + 4)), /4 bytes omitted/);
  assert.match(truncateStderr("é".repeat(MAX_STDERR_BYTES)), /stderr truncated/);
  assert.equal(truncateField("ok", 3, "!"), "ok");
  assert.equal(Buffer.byteLength(truncateField("ééé", 5, "!"), "utf8"), 5);
  assert.equal(truncateField("abcdef", 2, "marker"), "ma");
});

test("fitDisplayEvent bounds assistant, start, and end fields", () => {
  const assistant = fitDisplayEvent({ sequence: 1, kind: "assistant", text: "x".repeat(4000) });
  const start = fitDisplayEvent({ sequence: 2, kind: "tool-start", toolCallId: "i".repeat(500), toolName: "n".repeat(500), argsPreview: "a".repeat(4000) });
  const end = fitDisplayEvent({ sequence: 3, kind: "tool-end", toolCallId: "i".repeat(500), toolName: "n".repeat(500), isError: true });
  assert.match(assistant.kind === "assistant" ? assistant.text : "", /event truncated/);
  assert.match(start.kind === "tool-start" ? start.toolCallId : "", /toolCallId truncated/);
  assert.match(start.kind === "tool-start" ? start.toolName : "", /toolName truncated/);
  assert.match(start.kind === "tool-start" ? start.argsPreview : "", /event truncated/);
  assert.match(end.kind === "tool-end" ? end.toolCallId : "", /toolCallId truncated/);
  for (const event of [assistant, start, end]) assert.ok(Buffer.byteLength(JSON.stringify(event), "utf8") <= MAX_DISPLAY_EVENT_BYTES);
  const short = { sequence: 4, kind: "assistant" as const, text: "ok" };
  assert.deepEqual(fitDisplayEvent(short), short);
});

test("runner streams ordered display events, usage, and cleans up", async () => {
  const h = harness();
  const updates: any[] = [];
  const pending = createRunner(h.deps).run({ ...h.request, onUpdate: (value) => updates.push(value) });
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(toolStart("call-1", "read", {}) + "\n");
  h.process.stdout.write(toolEnd("call-1", "read", false) + "\n");
  const line = message("done");
  h.process.stdout.write(line.slice(0, 10));
  h.process.stdout.write(line.slice(10) + "\n");
  h.process.stderr.write("warning");
  h.process.close();
  const result = await pending;
  assert.equal(result.output, "done");
  assert.equal(result.stderr, "warning");
  assert.equal(result.failed, false);
  assert.deepEqual(result.usage, { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: 0.25, turns: 1 });
  assert.equal(result.model, "child");
  assert.equal(result.modelChanged, false);
  assert.equal(result.latestCacheHitRate, 4 / 11 * 100);
  assert.deepEqual(updates.at(-1)?.usage, result.usage);
  assert.equal(updates.at(-1)?.model, "child");
  assert.equal(updates.at(-1)?.latestCacheHitRate, 4 / 11 * 100);
  assert.deepEqual(result.events, [
    { sequence: 1, kind: "tool-start", toolCallId: "call-1", toolName: "read", argsPreview: "{}" },
    { sequence: 2, kind: "tool-end", toolCallId: "call-1", toolName: "read", isError: false },
    { sequence: 3, kind: "assistant", text: "done" },
  ]);
  assert.equal(result.omittedEvents, 0);
  assert.equal(updates.length, 3);
  assert.deepEqual(h.writes, [{ path: "/tmp/child/explore.md", mode: 0o600 }]);
  assert.deepEqual(h.removals, ["/tmp/child"]);
});

test("runner preserves unmatched completions in observation order", async () => {
  const h = harness();
  const pending = createRunner(h.deps).run(h.request);
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(toolEnd("missing", "read", true) + "\n");
  h.process.stdout.write(toolStart("missing", "read", {}) + "\n");
  h.process.close();
  const result = await pending;
  assert.deepEqual(result.events.map((event) => event.kind), ["tool-end", "tool-start"]);
  assert.deepEqual(result.events.map((event) => event.sequence), [1, 2]);
});

test("runner bounds every event and counts omissions", async () => {
  const h = harness();
  const updates: any[] = [];
  const pending = createRunner(h.deps).run({ ...h.request, onUpdate: (value) => updates.push(value) });
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(message("x".repeat(4000)) + "\n");
  h.process.stdout.write(toolStart("i".repeat(500), "n".repeat(500), { value: "y".repeat(4000) }) + "\n");
  for (let i = 0; i < 25; i++) h.process.stdout.write(toolStart(String(i), "x", {}) + "\n");
  h.process.stdout.write(toolEnd("tail", "x", false) + "\n");
  h.process.close();
  const result = await pending;
  assert.equal(result.events.length, MAX_DISPLAY_EVENTS);
  assert.equal(result.omittedEvents, 8);
  assert.match(updates[0].events[0].text, /event truncated/);
  assert.match(updates[1].events[1].toolCallId, /toolCallId truncated/);
  assert.match(updates[1].events[1].toolName, /toolName truncated/);
  assert.match(updates[1].events[1].argsPreview, /event truncated/);
  for (const update of updates) for (const event of update.events) assert.ok(Buffer.byteLength(JSON.stringify(event), "utf8") <= MAX_DISPLAY_EVENT_BYTES);
});

test("runner reports every completed turn with current usage and actual model identity", async () => {
  const h = harness();
  const updates: any[] = [];
  const pending = createRunner(h.deps).run({ ...h.request, onUpdate: (value) => updates.push(value) });
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(message("", "toolUse", {
    provider: "proxy",
    model: "requested",
    responseModel: "actual-a",
    usage: { input: 10, output: 2, cacheRead: 30, cacheWrite: 10, cost: { total: 0.1 } },
  }) + "\n");
  h.process.stdout.write(message("done", "end", {
    provider: "proxy",
    model: "requested",
    responseModel: "actual-b",
    usage: { input: 5, output: 3, cacheRead: 15, cacheWrite: 0, cost: { total: 0.2 } },
  }) + "\n");
  h.process.close();
  const result = await pending;
  assert.equal(updates.length, 2);
  assert.deepEqual(updates[0].usage, { input: 10, output: 2, cacheRead: 30, cacheWrite: 10, cost: 0.1, turns: 1 });
  assert.equal(updates[0].model, "proxy/actual-a");
  assert.equal(updates[0].latestCacheHitRate, 60);
  assert.deepEqual({ ...updates[1].usage, cost: Number(updates[1].usage.cost.toFixed(3)) }, { input: 15, output: 5, cacheRead: 45, cacheWrite: 10, cost: 0.3, turns: 2 });
  assert.equal(result.model, "proxy/actual-b");
  assert.equal(result.modelChanged, true);
  assert.equal(result.latestCacheHitRate, 75);
});

test("runner handles defensive event shapes and a final unterminated event", async () => {
  const h = harness();
  const pending = createRunner(h.deps).run(h.request);
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write("\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "user", content: [] } }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", content: "invalid", usage: null } }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", content: [null, { type: "text" }], usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: {} } } }) + "\n");
  h.process.stdout.write(message("", "toolUse", { provider: "proxy", model: "requested", usage: {} }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", provider: "proxy", content: [], usage: {} } }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "tool_execution_start" }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "tool_execution_end" }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "unknown" }) + "\n");
  h.process.stdout.write(message("unterminated"));
  h.process.close();
  const result = await pending;
  assert.equal(result.output, "unterminated");
  assert.equal(result.events[0].kind, "tool-start");
  assert.equal(result.events[1].kind, "tool-end");
});

test("runner returns malformed, exit, and model failures with bounded diagnostics", async () => {
  for (const scenario of ["malformed", "exit", "null-exit", "error", "aborted"] as const) {
    const h = harness();
    const pending = createRunner(h.deps).run(h.request);
    await new Promise((resolve) => setImmediate(resolve));
    if (scenario === "malformed") h.process.stdout.write("not-json\nleftover");
    if (scenario === "exit") { h.process.stderr.write("bad"); h.process.close(2); }
    if (scenario === "null-exit") h.process.emit("close", null);
    if (scenario === "error") { h.process.stdout.write(message("bad", "error") + "\n"); h.process.close(); }
    if (scenario === "aborted") { h.process.stdout.write(message("bad", "aborted") + "\n"); h.process.close(); }
    if (scenario === "malformed") h.process.close();
    const result = await pending;
    assert.equal(result.failed, true);
    assert.ok(Buffer.byteLength(result.failureMessage ?? "", "utf8") <= MAX_FAILURE_BYTES);
    assert.deepEqual(h.removals, ["/tmp/child"]);
  }
});

test("runner preserves progress and bounded child errors on asynchronous process failure", async () => {
  const h = harness();
  const updates: any[] = [];
  const pending = createRunner(h.deps).run({ ...h.request, onUpdate: (value) => updates.push(value) });
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(message("progress", "toolUse") + "\n");
  h.process.fail(new Error("pipe failed"));
  const result = await pending;
  h.process.close();
  assert.equal(result.failed, true);
  assert.match(result.failureMessage ?? "", /pipe failed/);
  assert.equal(result.output, "progress");
  assert.equal(result.events.length, 1);
  assert.equal(result.usage.turns, 1);
  assert.equal(updates.length, 1);
  assert.deepEqual(h.removals, ["/tmp/child"]);

  const progressError = harness();
  const progressFailed = createRunner(progressError.deps).run(progressError.request);
  await new Promise((resolve) => setImmediate(resolve));
  progressError.process.stdout.write(message("earlier progress", "toolUse") + "\n");
  progressError.process.stdout.write(message("", "error", { errorMessage: "actual child error" }) + "\n");
  progressError.process.close();
  const progressResult = await progressFailed;
  assert.match(progressResult.failureMessage ?? "", /actual child error/);
  assert.match(progressResult.failureMessage ?? "", /earlier progress/);

  const exitError = harness();
  const exitFailed = createRunner(exitError.deps).run(exitError.request);
  await new Promise((resolve) => setImmediate(resolve));
  exitError.process.stdout.write(message("", "error", { errorMessage: "child exit error" }) + "\n");
  exitError.process.stderr.write("exit stderr");
  exitError.process.close(2);
  const exitResult = await exitFailed;
  assert.match(exitResult.failureMessage ?? "", /child exit error/);
  assert.match(exitResult.failureMessage ?? "", /exit stderr/);

  const childError = harness();
  const failed = createRunner(childError.deps).run(childError.request);
  await new Promise((resolve) => setImmediate(resolve));
  childError.process.stdout.write(message("", "error", { errorMessage: "x".repeat(MAX_FAILURE_BYTES * 2) }) + "\n");
  childError.process.close();
  const childResult = await failed;
  assert.equal(childResult.failed, true);
  assert.match(childResult.failureMessage ?? "", /failure truncated/);
  assert.ok(Buffer.byteLength(childResult.failureMessage ?? "", "utf8") <= MAX_FAILURE_BYTES);

  const emptyError = harness();
  const emptyFailed = createRunner(emptyError.deps).run(emptyError.request);
  await new Promise((resolve) => setImmediate(resolve));
  emptyError.process.stdout.write(message("", "error", { usage: {} }) + "\n");
  emptyError.process.close();
  assert.match((await emptyFailed).failureMessage ?? "", /no output/);
});

test("runner cleans setup failures and rejects pre-abort", async () => {
  const aborted = harness();
  const controller = new AbortController(); controller.abort();
  await assert.rejects(createRunner(aborted.deps).run({ ...aborted.request, signal: controller.signal }), /before start/);
  assert.equal(aborted.madeDirectories(), 0);

  const write = harness();
  write.deps.writeFile = async () => { throw new Error("write failed"); };
  await assert.rejects(createRunner(write.deps).run(write.request), /write failed/);
  assert.deepEqual(write.removals, ["/tmp/child"]);

  const synchronous = harness();
  synchronous.deps.spawn = () => { throw new Error("sync spawn failed"); };
  await assert.rejects(createRunner(synchronous.deps).run(synchronous.request), /sync spawn failed/);
  assert.deepEqual(synchronous.removals, ["/tmp/child"]);
});

test("abort sends TERM, escalates only while open, and removes listener", async () => {
  const raced = harness();
  const raceSignal = new EventTarget() as AbortSignal;
  let abortedReads = 0;
  Object.defineProperty(raceSignal, "aborted", { get: () => ++abortedReads > 1 });
  const racedRun = createRunner(raced.deps).run({ ...raced.request, signal: raceSignal });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(raced.process.signals, ["SIGTERM"]);
  raced.process.close();
  assert.equal((await racedRun).stopReason, "aborted");

  const h = harness();
  const controller = new AbortController();
  const pending = createRunner(h.deps).run({ ...h.request, signal: controller.signal });
  await new Promise((resolve) => setImmediate(resolve));
  controller.abort();
  assert.deepEqual(h.process.signals, ["SIGTERM"]);
  h.timers[0]();
  assert.deepEqual(h.process.signals, ["SIGTERM", "SIGKILL"]);
  h.process.close();
  const aborted = await pending;
  assert.equal(aborted.stopReason, "aborted");
  h.timers[0]();
  assert.deepEqual(h.process.signals, ["SIGTERM", "SIGKILL"]);

  const closed = harness();
  const second = new AbortController();
  const done = createRunner(closed.deps).run({ ...closed.request, signal: second.signal });
  await new Promise((resolve) => setImmediate(resolve));
  closed.process.stdout.write(message("done") + "\n"); closed.process.close(); await done;
  second.abort();
  assert.deepEqual(closed.process.signals, []);

  const sticky = harness();
  const stickySignal = new EventTarget() as AbortSignal & { aborted: boolean; abort(): void };
  Object.defineProperty(stickySignal, "aborted", { value: false, writable: true });
  stickySignal.removeEventListener = () => {};
  stickySignal.abort = () => { (stickySignal as any).aborted = true; stickySignal.dispatchEvent(new Event("abort")); };
  const stickyRun = createRunner(sticky.deps).run({ ...sticky.request, signal: stickySignal });
  await new Promise((resolve) => setImmediate(resolve));
  sticky.process.stdout.write(message("done") + "\n"); sticky.process.close(); await stickyRun;
  stickySignal.abort();
  assert.deepEqual(sticky.process.signals, []);

  const empty = harness();
  const emptyRun = createRunner(empty.deps).run(empty.request);
  await new Promise((resolve) => setImmediate(resolve));
  empty.process.close();
  assert.equal((await emptyRun).output, "(no output)");
});
