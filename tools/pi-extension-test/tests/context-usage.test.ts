import assert from "node:assert/strict";
import test from "node:test";
import contextUsageDefault, { formatCount, contextUsageLine, guardMinimumRuntime, registerContextUsage, versionSupported } from "../../../.pi/extensions/awf-context-usage/index.ts";

function make(options: any = {}) {
  const hooks = new Map<string, any>(); const notices: any[] = [];
  let usage: any = options.usage ?? { tokens: 118200, contextWindow: 272000, percent: 1 };
  let branch: any[] = options.branch ?? [];
  const pi: any = { on: (name: string, fn: any) => hooks.set(name, fn), events: { emit() {} } };
  const ctx: any = { getContextUsage: () => usage, sessionManager: { getBranch: () => branch }, ui: { notify: (...args: any[]) => notices.push(args) } };
  return { pi, ctx, hooks, notices, setUsage: (value: any) => { usage = value; }, setBranch: (value: any[]) => { branch = value; } };
}

test("formatCount rounds integer, k, and m values with JavaScript fixed rounding", () => {
  assert.equal(formatCount(0), "0"); assert.equal(formatCount(999.5), "1000");
  assert.equal(formatCount(1000), "1k"); assert.equal(formatCount(118250), "118.3k");
  assert.equal(formatCount(999999), "1000k"); assert.equal(formatCount(1000000), "1m"); assert.equal(formatCount(1250000), "1.3m");
});
test("contextUsageLine uses direct percentage and deterministic unknown or unavailable forms", () => {
  const harness = make({ branch: [{ type: "compaction" }, { type: "message" }, { type: "compaction" }] });
  assert.equal(contextUsageLine(harness.ctx), "[session context] 118.2k/272k (43%); compactions=2");
  harness.setUsage({ tokens: null, contextWindow: 272000 }); assert.equal(contextUsageLine(harness.ctx), "[session context] unknown/272k; compactions=2");
  for (const contextWindow of [undefined, 0, -1, Infinity, NaN]) { harness.setUsage({ tokens: 3, contextWindow }); assert.equal(contextUsageLine(harness.ctx), "[session context] unavailable; compactions=2"); }
});
test("context handler injects a fresh hidden line for each current request without side effects", () => {
  const harness = make({ branch: [{ type: "compaction" }] }); registerContextUsage(harness.pi, { packageVersion: "0.81.1" });
  assert.deepEqual([...harness.hooks.keys()], ["context"]);
  const messages = [{ role: "user", content: "tool result" }]; const first = harness.hooks.get("context")({ messages }, harness.ctx);
  assert.notEqual(first.messages, messages); assert.equal(messages.length, 1); assert.deepEqual(first.messages.slice(0, 1), messages); assert.equal(first.messages[1].role, "custom"); assert.equal(first.messages[1].customType, "awf-context-usage"); assert.equal(first.messages[1].display, false); assert.equal(first.messages[1].content, "[session context] 118.2k/272k (43%); compactions=1");
  harness.setUsage({ tokens: 2000, contextWindow: 4000 }); harness.setBranch([{ type: "compaction" }, { type: "tool" }]); const second = harness.hooks.get("context")({ messages }, harness.ctx);
  assert.notEqual(second.messages, first.messages); assert.equal(second.messages[1].content, "[session context] 2k/4k (50%); compactions=1"); assert.deepEqual(harness.notices, []);
});
test("minimum runtime guard emits one notice and avoids functional registration", () => {
  assert.equal(versionSupported("0.81.1"), true); assert.equal(versionSupported("0.82.0"), true); assert.equal(versionSupported("0.81.0"), false); assert.equal(versionSupported("bad"), false);
  const original = (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")]; delete (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")];
  const first = make(); delete first.pi.events.emit; assert.equal(guardMinimumRuntime(first.pi, { packageVersion: "0.0.0" }, ["on"]), false); first.hooks.get("session_start")({}, first.ctx); first.hooks.get("session_start")({}, first.ctx); assert.equal(first.notices.length, 1);
  delete (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")];
  const missing = make(); delete missing.pi.events.emit; assert.equal(guardMinimumRuntime(missing.pi, { packageVersion: "0.81.1" }, ["eventsEmit"]), false); missing.hooks.get("session_start")({}, missing.ctx);
  const second = make(); delete second.pi.on; assert.equal(registerContextUsage(second.pi, { packageVersion: "0.0.0" }), undefined); assert.equal(second.hooks.size, 0);
  if (original === undefined) delete (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")]; else (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")] = original;
});
test("default registration uses pinned package runtime", async () => { const harness = make(); await contextUsageDefault(harness.pi); assert.equal(harness.hooks.has("context"), true); });
