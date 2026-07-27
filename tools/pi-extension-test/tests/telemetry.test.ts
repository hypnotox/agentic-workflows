import assert from "node:assert/strict";
import { mkdir, mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  collectPiSessionAccounting,
  createLedgerWriter,
  createTelemetryState,
  defaultLedgerDependencies,
  projectLocalLifecycle,
  registerTelemetry,
  restoreAssociation,
  telemetryWidget,
} from "../../../.pi/extensions/awf-telemetry/index.ts";
import { protocolVersion } from "../../../.pi/extensions/awf-telemetry/protocol.ts";

// invariant: rendering/pi-workflows:pi-workflow-telemetry-public-contract
test("invariant: telemetry bar uses only public active-branch and context data", () => {
  const association = { effortId: "restored-effort", sessionId: "session", trajectoryId: "trajectory", associationOrigin: "manual" } as const;
  const ctx = { sessionManager: { getBranch: () => [
    { type: "custom", customType: "awf.telemetry.association.v1", data: association },
    { type: "message", id: "parent", message: { role: "assistant", usage: { input: 2, output: 3, cacheRead: 4, cacheWrite: 5, cost: { total: 0.1 } } } },
    { type: "message", id: "parent", message: { role: "assistant", usage: { input: 200, output: 300, cacheRead: 400, cacheWrite: 500, cost: { total: 10 } } } },
    { type: "message", id: "nested", message: { role: "assistant", usage: { input: 7, output: 11, cacheRead: 13, cacheWrite: 17, cost: { total: 0.2 } } } },
    { type: "message", id: "user", message: { role: "user", usage: { input: 999 } } },
  ] }, getContextUsage: () => ({ tokens: 10, contextWindow: 100, percent: 10 }) };
  assert.deepEqual(restoreAssociation(ctx), association);
  const accounting = collectPiSessionAccounting(ctx);
  assert.deepEqual({ ...accounting, cost: 0 }, { input: 9, output: 14, cacheRead: 17, cacheWrite: 22, cost: 0, contextTokens: 10, contextWindow: 100, contextPercent: 10 });
  assert.equal(accounting.cost, 0.1 + 0.2);
  assert.deepEqual(collectPiSessionAccounting({ sessionManager: { getBranch: () => [] }, getContextUsage: () => ({ tokens: 10, contextWindow: 0, percent: 10 }) }), { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0 });

  let updates = 0;
  const state = createTelemetryState(() => updates++);
  assert.match(telemetryWidget(state, accounting, 200)[0]!, /^\[awf:init\]/);
  state.acceptLocalLifecycle({ version: protocolVersion, eventId: "passive", observationId: "passive", effortId: "effort", sessionId: "session", timestamp: "2026-07-27T00:00:00Z", kind: "usage_observed", predecessors: [], payload: { model: "model", inputTokens: 1, outputTokens: 1, cacheReadTokens: 0, cacheWriteTokens: 0, costUsd: 0, durationMs: 1 } } as any);
  assert.equal(updates, 0);
  state.acceptLocalLifecycle({ version: protocolVersion, eventId: "phase", idempotencyKey: "phase", effortId: "effort", sessionId: "session", timestamp: "2026-07-27T00:00:00Z", kind: "phase_started", predecessors: [], payload: { phase: "implementation" } } as any);
  assert.equal(updates, 1);
  assert.match(telemetryWidget(state, accounting, 200)[0]!, /^\[awf:implementation\]/);
  state.setWidgetAssociation(undefined, true);
  assert.equal(updates, 2);

  const tools = new Map<string, unknown>(); const commands = new Map<string, unknown>();
  const pi: any = { registerTool: (tool: any) => tools.set(tool.name, tool), registerCommand: (name: string, command: any) => commands.set(name, command), appendEntry() {}, on() {}, events: { on() {}, emit() {} }, queueCommand() {} };
  registerTelemetry(pi, { packageVersion: "0.81.1", extensionFile: "/repo/.pi/extensions/awf-telemetry/index.ts", ledger: { root: "/repo", now: Date.now, uuid: () => "id", owner: () => "owner" } as any, gracefulProvisional: async () => undefined });
  for (const name of ["awf_lifecycle", "awf_adopt_effort", "awf_detour", "awf_workflow"]) assert.ok(tools.has(name));
  for (const name of ["awf_metrics", "awf_doctor", "awf-dashboard"]) assert.ok(!tools.has(name) && !commands.has(name));
});

// invariant: rendering/adapter-outputs:pi-workflow-telemetry-runtime
test("invariant: telemetry runtime retains durable lifecycle, association, passive events, and shutdown drain", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-telemetry-"));
  try {
    await mkdir(join(root, ".awf"), { recursive: true });
    const deps = defaultLedgerDependencies(join(root, ".pi/extensions/awf-telemetry/index.ts"));
    let sequence = 0; deps.uuid = () => `uuid-${++sequence}`;
    const writer = createLedgerWriter(deps); const at = "2026-07-27T00:00:00Z";
    await writer.create({ effortId: "effort", createdAt: at, creationMode: "independent" }, { version: protocolVersion, eventId: "create", idempotencyKey: "create-key", effortId: "effort", sessionId: "session", timestamp: at, kind: "effort_created", predecessors: [], payload: { creationMode: "independent" } } as any);
    assert.equal(await writer.passive({} as any), undefined);
    assert.equal(writer.isDegraded(), true);
    const trajectory = await writer.mutateLifecycle({ action: "start-trajectory", idempotencyKey: "trajectory-key", eventId: "trajectory", effortId: "effort", sessionId: "session", timestamp: at, predecessors: ["create"], trajectoryId: "trajectory", anchorId: "anchor" } as any);
    const association = writer.mutateLifecycle({ action: "associate", idempotencyKey: "associate-key", eventId: "associate", effortId: "effort", sessionId: "session", timestamp: at, predecessors: [trajectory.event.eventId], trajectoryId: "trajectory", associationOrigin: "created" } as any);
    await writer.shutdown();
    await association;
    const projection = await projectLocalLifecycle(deps, "effort");
    assert.equal(projection.effectApplied.has("associate"), true);
    assert.equal(projection.associations.get("session")?.trajectoryId, "trajectory");
  } finally { await rm(root, { recursive: true, force: true }); }
});
