import assert from "node:assert/strict";
import test from "node:test";
import { dirname, isAbsolute, resolve } from "node:path";
import { requestSelectedEffort, validateMemoryEffort, validateMemoryPath } from "../../../.pi/extensions/awf-handoff/index.ts";

const root = "/repo";
function deps(files: Record<string, string>) {
  return { packageVersion: "0.81.1", extensionFile: `${root}/.pi/extensions/awf-handoff/index.ts`, path: { resolve, dirname, isAbsolute }, randomUUID: () => "handoff-id", setInterval: () => 1, clearInterval() {}, setTimeout: () => 1, clearTimeout() {}, lstat: async (path: string) => ({ isSymbolicLink: () => path.includes("link"), isFile: () => path.endsWith(".md"), isDirectory: () => !path.endsWith(".md") }), readFile: async (path: string) => files[path] ?? "" } as any;
}

test("invariant: tooling/workflow-telemetry:canonical-projections-and-diagnostics validates safe memory ownership and effort identity", async () => {
  const path = ".awf/memory/checkpoint.md";
  const d = deps({ ["/repo/.awf/memory/checkpoint.md"]: "Effort: effort-1\n" });
  assert.equal(await validateMemoryPath(path, d), path);
  assert.equal(await validateMemoryEffort(path, d), "effort-1");
  await assert.rejects(validateMemoryPath(".awf/memory/../secret", d));
  await assert.rejects(validateMemoryEffort(path, deps({ ["/repo/.awf/memory/checkpoint.md"]: "Effort: one\nEffort: two\n" })));
});

test("invariant: tooling/workflow-telemetry:event-protocol-and-ledger selection is passive and accepts absent or matching responses", () => {
  const responders: any[] = [];
  const pi = { events: { emit(_name: string, request: any) { responders.push(request.respond); request.respond({ effortId: "effort-1" }); } } } as any;
  assert.equal(requestSelectedEffort(pi), "effort-1");
  assert.equal(requestSelectedEffort({ events: { emit() { throw new Error("unavailable"); } } } as any), undefined);
  assert.equal(requestSelectedEffort({ events: { emit(_name: string, request: any) { request.respond(undefined); } } } as any), undefined);
  assert.equal(responders.length, 1);
});
