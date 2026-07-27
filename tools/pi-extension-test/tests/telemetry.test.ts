import assert from "node:assert/strict";
import { access, mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import {
  collectPiSessionAccounting,
  createLedgerWriter,
  defaultLedgerDependencies,
  defaultTelemetryDependencies,
  guardMinimumRuntime,
  projectLocalLifecycle,
  registerTelemetry,
  versionSupported,
} from "../../../.pi/extensions/awf-telemetry/index.ts";
import { protocolVersion } from "../../../.pi/extensions/awf-telemetry/protocol.ts";

async function waitFor(predicate: () => boolean, message: string) {
  const deadline = Date.now() + 2000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(message);
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}

async function waitForEvents(ledger: any, effortId: string, count: number) {
  const deadline = Date.now() + 10_000;
  for (;;) {
    try { if ((await projectLocalLifecycle(ledger, effortId)).events.size >= count) return; } catch {}
    if (Date.now() >= deadline) throw new Error("provisional telemetry did not flush");
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}

async function telemetryHarness(root: string) {
  const tools = new Map<string, any>(); const commands = new Map<string, any>(); const hooks = new Map<string, any>(); const entries: any[] = []; const queued: any[] = [];
  const ledger = defaultLedgerDependencies(join(root, ".pi/extensions/awf-telemetry/index.ts")); let sequence = 0; ledger.uuid = () => `uuid-${++sequence}`;
  let widget: any; let renders = 0;
  const pi: any = {
    registerTool(tool: any) { tools.set(tool.name, tool); }, registerCommand(name: string, command: any) { commands.set(name, command); }, appendEntry(type: string, data: any) { entries.push({ type, data }); }, queueCommand(name: string, args: string) { queued.push([name, args]); },
    on(name: string, handler: any) { hooks.set(name, handler); }, events: { on() {}, emit() {} },
  };
  registerTelemetry(pi, { packageVersion: "0.81.1", extensionFile: join(root, ".pi/extensions/awf-telemetry/index.ts"), ledger, gracefulProvisional: (identity: any) => identity.graceful() });
  const sessionManager = { getSessionId: () => "session", getLeafId: () => "leaf", getBranch: () => [] as any[] };
  const ctx: any = { sessionManager, getContextUsage: () => ({ tokens: 10, contextWindow: 100, percent: 10 }), ui: { setWidget(_name: string, factory: any) { if (factory) widget = factory({ requestRender() { renders++; } }, { fg: (_style: string, line: string) => line }); } } };
  return { tools, commands, hooks, entries, queued, ledger, ctx, widget: () => widget, renders: () => renders };
}

test("telemetry minimum-runtime guard and defaults cover supported and rejected hosts", async () => {
  assert.equal(versionSupported("0.81.1"), true);
  assert.equal(versionSupported("0.82.0"), true);
  assert.equal(versionSupported("0.81.0"), false);
  assert.equal(versionSupported("invalid"), false);
  const defaults = defaultTelemetryDependencies({} as any);
  assert.equal(defaults.packageVersion, process.env.npm_package_version ?? "0.81.1");
  let handler: any; const notifications: any[] = [];
  const pi: any = { on(name: string, callback: any) { assert.equal(name, "session_start"); handler = callback; } };
  assert.equal(guardMinimumRuntime(pi, { packageVersion: "0.80.0" }, ["on", "exec"]), false);
  await handler({}, { ui: { notify(message: string, level: string) { notifications.push([message, level]); } } });
  await handler({}, { ui: { notify(message: string, level: string) { notifications.push([message, level]); } } });
  assert.equal(notifications.length, 1);
  (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")] = false;
  assert.equal(guardMinimumRuntime(pi, { packageVersion: "0.80.0" }, ["on"]), false);
  await handler({}, { ui: { notify(message: string, level: string) { notifications.push([message, level]); } } });
  assert.equal(notifications.length, 2);
});

// invariant: rendering/pi-workflows:pi-workflow-telemetry-public-contract
test("invariant: telemetry bar uses only public active-branch and context data", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-telemetry-public-"));
  try {
    for (const path of [".pi/extensions/awf-subagents/index.ts", ".pi/extensions/awf-subagents/runner.ts", ".pi/extensions/awf-handoff/index.ts", ".pi/extensions/awf-telemetry/index.ts", ".pi/extensions/awf-telemetry/protocol.ts"]) await access(path);
    await mkdir(join(root, ".awf"), { recursive: true }); await mkdir(join(root, ".pi/awf-workflows"), { recursive: true }); await writeFile(join(root, ".pi/awf-workflows/brainstorming.md"), "brainstorming body\n");
    const h = await telemetryHarness(root);
    const restored = { effortId: "restored-effort", sessionId: "session", trajectoryId: "trajectory", associationOrigin: "manual" };
    h.ctx.sessionManager.getBranch = () => [
      { type: "custom", customType: "awf.telemetry.association.v1", data: restored },
      { type: "message", id: "parent", message: { role: "assistant", usage: { input: 2, output: 3, cacheRead: 5, cacheWrite: 7, cost: { total: 0.25 } } } },
      { type: "message", id: "parent", message: { role: "assistant", usage: { input: 200, output: 300, cacheRead: 500, cacheWrite: 700, cost: { total: 25 } } } },
      { type: "message", id: "nested", message: { role: "assistant", usage: { input: 11, output: 13, cacheRead: 17, cacheWrite: 19, cost: { total: 1.5 } } } },
      { type: "message", id: "user", message: { role: "user", usage: { input: 999 } } },
    ];
    assert.deepEqual(collectPiSessionAccounting(h.ctx), { input: 13, output: 16, cacheRead: 22, cacheWrite: 26, cost: 1.75, contextTokens: 10, contextWindow: 100, contextPercent: 10 }, "restored nested branch accounting charges each assistant entry exactly once");
    h.ctx.sessionManager.getBranch = () => [];
    assert.deepEqual([...h.tools.keys()], ["awf_lifecycle", "awf_adopt_effort", "awf_detour", "awf_workflow"]);
    assert.deepEqual([...h.commands.keys()], ["awf-resume-effort", "awf-resume-effort-continue"]);
    for (const name of ["awf_metrics", "awf_doctor", "awf-dashboard", "dashboard", "refresh"]) assert.equal(h.tools.has(name) || h.commands.has(name) || h.hooks.has(name), false);
    await h.hooks.get("session_start")({}, h.ctx);
    assert.match(h.widget().render(200)[0], /^\[awf:init\].*10\.0%\/100$/);
    for (let index = 0; index < 255; index++) h.hooks.get("tool_execution_end")({ toolCallId: `buffer-${index}`, toolName: "read", isError: false }, h.ctx);
    h.hooks.get("tool_execution_end")({ toolCallId: "overflow", toolName: "read", isError: false }, h.ctx);
    await waitFor(() => h.entries.some((entry) => entry.type === "awf.telemetry.association.v1"), "provisional overflow did not settle");
    const provisional = h.entries.find((entry) => entry.type === "awf.telemetry.association.v1").data;
    await waitForEvents(h.ledger, provisional.effortId, 260);
    const workflow = await h.tools.get("awf_workflow").execute("workflow", { skill: "brainstorming" }, undefined, undefined, h.ctx);
    assert.equal(workflow.details.durable, true); assert.match(h.widget().render(200)[0], /^\[awf:brainstorming\]/);
    const badgeAfterAction = h.widget().render(200)[0];
    await assert.rejects(h.tools.get("awf_lifecycle").execute("bad", { action: "start-phase" }, undefined, undefined, h.ctx), /invalid lifecycle request/);
    assert.equal(h.widget().render(200)[0], badgeAfterAction, "failed explicit actions do not update the badge");
    const seed = createLedgerWriter(h.ledger); const at = "2026-07-27T00:00:00Z";
    await seed.create({ effortId: "resume-target", createdAt: at, creationMode: "independent" }, { version: protocolVersion, eventId: "resume-create", idempotencyKey: "resume-create-key", effortId: "resume-target", sessionId: "replacement", timestamp: at, kind: "effort_created", predecessors: [], payload: { creationMode: "independent" } } as any);
    await seed.mutateLifecycle({ action: "start-trajectory", idempotencyKey: "resume-trajectory-key", eventId: "resume-trajectory", effortId: "resume-target", sessionId: "replacement", timestamp: at, predecessors: ["resume-create"], trajectoryId: "resume-trajectory", anchorId: "resume-anchor" } as any); await seed.shutdown();
    await h.commands.get("awf-resume-effort").handler("resume-target");
    const [, requestID] = h.queued.at(-1);
    const replacementEntries: any[] = [];
    await h.commands.get("awf-resume-effort-continue").handler(requestID, { newSession: async ({ setup, withSession }: any) => { const manager = { getSessionId: () => "replacement", appendCustomEntry(type: string, data: any) { replacementEntries.push({ type, data }); } }; await setup(manager); await withSession(); return {}; } });
    assert.equal(replacementEntries[0]?.type, "awf.telemetry.association.v1");
    const candidate = await projectLocalLifecycle(h.ledger, provisional.effortId);
    assert.equal(candidate.state, "abandoned", "overflow candidate is settled before explicit resume");
    await h.hooks.get("session_shutdown")({}, h.ctx);
  } finally { await rm(root, { recursive: true, force: true }); }
});

// invariant: rendering/adapter-outputs:pi-workflow-telemetry-runtime
test("invariant: telemetry runtime retains durable lifecycle, association, passive events, and shutdown drain", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-telemetry-runtime-"));
  try {
    await mkdir(join(root, ".awf"), { recursive: true });
    const h = await telemetryHarness(root); await h.hooks.get("session_start")({}, h.ctx);
    const at = "2026-07-27T00:00:00Z"; const lifecycle = h.tools.get("awf_lifecycle");
    await lifecycle.execute("create", { action: "create", idempotencyKey: "create-key", eventId: "create", effortId: "runtime-effort", sessionId: "session", timestamp: at, predecessors: [], creationMode: "independent" });
    await lifecycle.execute("trajectory", { action: "start-trajectory", idempotencyKey: "trajectory-key", eventId: "trajectory", effortId: "runtime-effort", sessionId: "session", timestamp: at, predecessors: ["create"], trajectoryId: "trajectory", anchorId: "anchor" });
    const associated = await lifecycle.execute("associate", { action: "associate", idempotencyKey: "associate-key", eventId: "associate", effortId: "runtime-effort", sessionId: "session", timestamp: at, predecessors: ["trajectory"], trajectoryId: "trajectory", associationOrigin: "created" });
    assert.equal(associated.details.durable, true); assert.match(h.widget().render(200)[0], /^\[awf:init\]/);
    const projection = await projectLocalLifecycle(h.ledger, "runtime-effort");
    assert.equal(projection.effectApplied.has("associate"), true); assert.equal(projection.associations.get("session")?.trajectoryId, "trajectory");

    const originalLstat = h.ledger.lstat; let passiveFailure = false;
    h.ledger.lstat = async () => { passiveFailure = true; throw new Error("passive storage failure"); };
    assert.doesNotThrow(() => h.hooks.get("tool_execution_end")({ toolCallId: "passive-failure", toolName: "read", isError: false }, h.ctx), "passive storage failure is nonblocking");
    await waitFor(() => passiveFailure, "passive observation did not attempt storage");
    h.ledger.lstat = originalLstat;

    await lifecycle.execute("route", { action: "select-route", idempotencyKey: "route-key", eventId: "route", effortId: "runtime-effort", sessionId: "session", timestamp: at, predecessors: ["associate"], route: "direct" });
    h.ledger.lstat = async () => { throw new Error("explicit lifecycle storage failure"); };
    await assert.rejects(lifecycle.execute("phase-failure", { action: "start-phase", idempotencyKey: "phase-failure-key", eventId: "phase-failure", effortId: "runtime-effort", sessionId: "session", timestamp: at, predecessors: ["route"], phase: "brainstorming" }), /explicit lifecycle storage failure/);
    h.ledger.lstat = originalLstat;
    assert.match(h.widget().render(200)[0], /^\[awf:init\]/, "rejected lifecycle persistence does not report badge success");
    assert.equal((await projectLocalLifecycle(h.ledger, "runtime-effort")).effectApplied.has("phase-failure"), false);

    await lifecycle.execute("association-create", { action: "create", idempotencyKey: "association-create-key", eventId: "association-create", effortId: "association-failure", sessionId: "session", timestamp: at, predecessors: [], creationMode: "independent" });
    await lifecycle.execute("association-trajectory", { action: "start-trajectory", idempotencyKey: "association-trajectory-key", eventId: "association-trajectory", effortId: "association-failure", sessionId: "session", timestamp: at, predecessors: ["association-create"], trajectoryId: "association-trajectory", anchorId: "association-anchor" });
    h.ledger.lstat = async () => { throw new Error("explicit association storage failure"); };
    await assert.rejects(lifecycle.execute("association-failure", { action: "associate", idempotencyKey: "association-failure-key", eventId: "association-failure", effortId: "association-failure", sessionId: "session", timestamp: at, predecessors: ["association-trajectory"], trajectoryId: "association-trajectory", associationOrigin: "created" }), /explicit association storage failure/);
    h.ledger.lstat = originalLstat;
    assert.match(h.widget().render(200)[0], /^\[awf:init\]/, "rejected association persistence does not report badge success");
    assert.equal((await projectLocalLifecycle(h.ledger, "association-failure")).effectApplied.has("association-failure"), false);

    h.hooks.get("tool_execution_end")({ toolCallId: "drain", toolName: "read", isError: false }, h.ctx);
    await h.hooks.get("session_shutdown")({}, h.ctx);
    const events = (await projectLocalLifecycle(h.ledger, "runtime-effort")).events;
    assert.ok([...events.values()].some((event: any) => event.kind === "tool_observed" && event.payload.tool === "read"), "shutdown drains queued passive telemetry");
  } finally { await rm(root, { recursive: true, force: true }); }
});
