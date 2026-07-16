import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import { resolve } from "node:path";
import { PassThrough } from "node:stream";
import test from "node:test";
import {
  KILL_DELAY_MS,
  MAX_OUTPUT_BYTES,
  MAX_OUTPUT_LINES,
  MAX_STDERR_BYTES,
  createRunner,
  productionRunnerDependencies,
  resolvePiInvocation,
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
  const deps: RunnerDependencies = {
    spawn: () => process,
    mkdtemp: async () => "/tmp/child",
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
  return { process, deps, request, writes, removals, timers };
}

function message(text: string, stopReason = "end") {
  return JSON.stringify({ type: "message_end", message: { role: "assistant", content: [{ type: "text", text }], model: "child", stopReason, usage: { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: { total: 0.25 } } } });
}

test("production runner executes the fake Pi fixture", async () => {
  const fixture = resolve("tools/pi-extension-test/fixtures/fake-pi.mjs");
  const runner = createRunner({ ...productionRunnerDependencies, argv: ["node", fixture], execPath: process.execPath });
  const result = await runner.run({
    role: "explore", task: "inspect", cwd: process.cwd(),
    model: { provider: "test", id: "model" }, thinkingLevel: "high", tools: ["read"], systemPrompt: "system",
  });
  assert.equal(result.output, "fixture output");
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
  await assert.rejects(pending);
  assert.deepEqual(signals, ["SIGTERM", "SIGKILL"]);
});

test("invocation resolution covers script, generic runtime fallback, and binary", () => {
  assert.deepEqual(resolvePiInvocation(["node", "/pi/cli.js"], "/usr/bin/node"), { command: "/usr/bin/node", args: ["/pi/cli.js"] });
  assert.deepEqual(resolvePiInvocation(["bun", "/$bunfs/root/pi"], "/usr/bin/bun"), { command: "pi", args: [] });
  assert.deepEqual(resolvePiInvocation(["node", "node:virtual"], "node.exe"), { command: "pi", args: [] });
  assert.deepEqual(resolvePiInvocation(["pi"], "/usr/bin/pi"), { command: "/usr/bin/pi", args: [] });
});

test("output truncation handles short, byte, line, stderr, and UTF-8 input", () => {
  assert.equal(truncateOutput("ok"), "ok");
  assert.match(truncateOutput("x".repeat(MAX_OUTPUT_BYTES + 10)), /10 bytes/);
  assert.match(truncateOutput(Array(MAX_OUTPUT_LINES + 3).fill("x").join("\n")), /3 lines/);
  assert.match(truncateOutput("é".repeat(MAX_OUTPUT_BYTES)), /Output truncated/);
  assert.equal(truncateStderr("ok"), "ok");
  assert.match(truncateStderr("x".repeat(MAX_STDERR_BYTES + 4)), /4 bytes omitted/);
  assert.match(truncateStderr("é".repeat(MAX_STDERR_BYTES)), /stderr truncated/);
});

test("runner streams fragmented JSON, summaries, usage, and cleans up", async () => {
  const h = harness();
  const updates: unknown[] = [];
  const pending = createRunner(h.deps).run({ ...h.request, onUpdate: (value) => updates.push(value) });
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write('{"type":"tool_execution_start","toolName":"read","args":{}}\n');
  const line = message("done");
  h.process.stdout.write(line.slice(0, 10));
  h.process.stdout.write(line.slice(10) + "\n");
  h.process.stderr.write("warning");
  h.process.close();
  const result = await pending;
  assert.equal(result.output, "done");
  assert.equal(result.stderr, "warning");
  assert.deepEqual(result.usage, { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: 0.25, turns: 1 });
  assert.equal(result.model, "child");
  assert.equal(result.summaries.length, 2);
  assert.ok(updates.length >= 2);
  assert.deepEqual(h.writes, [{ path: "/tmp/child/explore.md", mode: 0o600 }]);
  assert.deepEqual(h.removals, ["/tmp/child"]);
});

test("runner bounds malformed and repeated summaries", async () => {
  const h = harness();
  const pending = createRunner(h.deps).run(h.request);
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", content: "invalid" } }) + "\n");
  h.process.stdout.write("\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", content: [null, { type: "toolCall" }, { type: "text" }] } }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "message_end", message: { role: "assistant", content: [], usage: {} } }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "tool_execution_start" }) + "\n");
  h.process.stdout.write(JSON.stringify({ type: "unknown" }) + "\n");
  for (let i = 0; i < 25; i++) h.process.stdout.write(JSON.stringify({ type: "tool_execution_start", toolName: "x", args: { value: "y".repeat(3000) } }) + "\n");
  h.process.stdout.write(message("tail"));
  h.process.close();
  const result = await pending;
  assert.equal(result.summaries.length, 20);
  assert.match(result.summaries[0].text, /summary truncated/);
});

test("runner rejects malformed terminal streams with bounded diagnostics", async () => {
  const h = harness();
  const pending = createRunner(h.deps).run(h.request);
  await new Promise((resolve) => setImmediate(resolve));
  h.process.stdout.write("not-json\n");
  h.process.close();
  await assert.rejects(pending, /Malformed child event: not-json/);
  assert.deepEqual(h.process.signals, ["SIGTERM"]);
});

test("runner rejects pre-abort, spawn errors, exits, and model failures", async () => {
  const aborted = harness();
  const controller = new AbortController(); controller.abort();
  await assert.rejects(createRunner(aborted.deps).run({ ...aborted.request, signal: controller.signal }), /before start/);

  for (const scenario of ["spawn", "exit", "stop", "model-error"] as const) {
    const h = harness();
    const pending = createRunner(h.deps).run(h.request);
    await new Promise((resolve) => setImmediate(resolve));
    if (scenario === "spawn") { h.process.fail(new Error("spawn failed")); h.process.close(); }
    if (scenario === "exit") { h.process.stderr.write("bad"); h.process.close(2); }
    if (scenario === "stop") { h.process.stdout.write(message("bad", "aborted") + "\n"); h.process.close(); }
    if (scenario === "model-error") { h.process.stdout.write(message("bad", "error") + "\n"); h.process.close(); }
    await assert.rejects(pending);
    assert.deepEqual(h.removals, ["/tmp/child"]);
  }
});

test("abort sends TERM, escalates only while open, and removes listener", async () => {
  const raced = harness();
  const raceSignal = new EventTarget() as AbortSignal;
  let abortedReads = 0;
  Object.defineProperty(raceSignal, "aborted", { get: () => ++abortedReads > 1 });
  const racedRun = createRunner(raced.deps).run({ ...raced.request, signal: raceSignal });
  await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(raced.process.signals, ["SIGTERM"]);
  raced.process.stdout.write(message("stopped") + "\n"); raced.process.close(); await racedRun;

  const h = harness();
  const controller = new AbortController();
  const pending = createRunner(h.deps).run({ ...h.request, signal: controller.signal });
  await new Promise((resolve) => setImmediate(resolve));
  controller.abort();
  assert.deepEqual(h.process.signals, ["SIGTERM"]);
  h.timers[0]();
  assert.deepEqual(h.process.signals, ["SIGTERM", "SIGKILL"]);
  h.process.stdout.write(message("stopped") + "\n");
  h.process.close();
  await pending;
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

  const nullExit = harness();
  const nullRun = createRunner(nullExit.deps).run(nullExit.request);
  await new Promise((resolve) => setImmediate(resolve));
  nullExit.process.emit("close", null);
  await assert.rejects(nullRun, /exited null/);
});
