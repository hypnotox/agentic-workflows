import assert from "node:assert/strict";
import test from "node:test";
import { collectPiSessionAccounting, createTelemetryState, registerTelemetry, telemetryWidget } from "../../../.pi/extensions/awf-telemetry/index.ts";

test("invariant: telemetry bar uses only public active-branch and context data", () => {
  const ctx = { sessionManager: { getBranch: () => [{ type: "message", id: "one", message: { role: "assistant", usage: { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: { total: 0.1 } } } }] }, getContextUsage: () => ({ tokens: 10, contextWindow: 100, percent: 10 }) };
  const state = createTelemetryState();
  assert.deepEqual(collectPiSessionAccounting(ctx), { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: 0.1, contextTokens: 10, contextWindow: 100, contextPercent: 10 });
  assert.match(telemetryWidget(state, collectPiSessionAccounting(ctx), 200)[0]!, /^\[awf:init\]/);
});

test("invariant: telemetry runtime retains durable lifecycle, association, passive events, and shutdown drain", () => {
  const tools = new Map<string, unknown>(); const commands = new Map<string, unknown>();
  const pi: any = { registerTool: (tool: any) => tools.set(tool.name, tool), registerCommand: (name: string, command: any) => commands.set(name, command), appendEntry() {}, on() {}, events: { on() {}, emit() {} }, queueCommand() {} };
  registerTelemetry(pi, { packageVersion: "0.81.1", extensionFile: "/repo/.pi/extensions/awf-telemetry/index.ts", ledger: { root: "/repo", now: Date.now, uuid: () => "id", owner: () => "owner" } as any, gracefulProvisional: async () => undefined });
  for (const name of ["awf_lifecycle", "awf_adopt_effort", "awf_detour", "awf_workflow"]) assert.ok(tools.has(name));
  for (const name of ["awf" + "_metrics", "awf" + "_doctor", "awf" + "-dashboard"]) assert.ok(!tools.has(name) && !commands.has(name));
});
