import assert from "node:assert/strict";
import test from "node:test";
import { registerTelemetry } from "../../../.pi/extensions/awf-telemetry/index.ts";

function piHarness() {
  const commands: any[] = [], tools: any[] = [], callbacks = new Map<string, any>();
  return { commands, tools, callbacks, on(name: string, callback: any) { callbacks.set(name, callback); }, events: { on(name: string, callback: any) { callbacks.set(name, callback); }, emit() {} }, registerCommand(value: any) { commands.push(value); }, registerTool(value: any) { tools.push(value); } } as any;
}

test("invariant: rendering/pi-runtime:minimum-runtime-gate wires the installed Pi package version into compatible registration and visibly refuses old Pi", async () => {
  const compatible = piHarness();
  registerTelemetry(compatible, "/repo/.pi/extensions/awf-telemetry/index.ts", "0.81.1");
  assert.equal(compatible.commands.length, 1);
  assert.equal(compatible.tools.length, 1);
  const incompatible = piHarness();
  delete (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")];
  registerTelemetry(incompatible, "/repo/.pi/extensions/awf-telemetry/index.ts", "0.80.9");
  assert.equal(incompatible.commands.length, 0);
  assert.equal(incompatible.tools.length, 0);
  const notices: any[] = [];
  await incompatible.callbacks.get("session_start")({}, { ui: { notify: (...args: any[]) => notices.push(args) } });
  assert.match(notices[0][0], /found 0.80.9/);
});

test("invariant: rendering/pi-workflows:closed-workflow-router accepts only the generated enabled workflow enum", async () => {
  const pi = piHarness();
  registerTelemetry(pi, "/repo/.pi/extensions/awf-telemetry/index.ts", "0.81.1");
  const tool = pi.tools[0];
  for (const skill of ["../bugfix", "unknown", "", "awf-workflow"]) await assert.rejects(tool.execute("", { skill }, undefined, undefined, {}), /workflow skill/);
});
