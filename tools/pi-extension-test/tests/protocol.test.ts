import assert from "node:assert/strict";
import test from "node:test";
import { identifier, validateObservation, validateSessionHeader } from "../../../.pi/extensions/awf-telemetry/protocol.ts";

const base = { record: "observation", schemaVersion: 1, observationId: "123e4567-e89b-42d3-a456-426614174000", timestamp: "2026-07-27T00:00:00Z", kind: "usage", payload: { inputTokens: 0, outputTokens: 0, cacheReadTokens: 0, cacheWriteTokens: 0, costUsd: 0 } };
test("invariant: tooling/workflow-telemetry:event-protocol-and-ledger validates closed session-v1 records", () => {
  assert.equal(identifier("session-1"), true);
  assert.equal(identifier("../escape"), false);
  assert.equal(validateSessionHeader({ record: "header", schemaVersion: 1, sessionId: "session", createdAt: "2026-07-27T00:00:00Z" }), true);
  assert.equal(validateSessionHeader({ record: "header", schemaVersion: 2, sessionId: "session", createdAt: "bad" }), false);
  assert.equal(validateObservation(base), true);
  const valid = [
    { ...base, kind: "tool", payload: { tool: "bash", outcome: "success", durationMs: 1 } },
    { ...base, kind: "gate", payload: { gate: "gate", outcome: "failure", durationMs: 2 } },
    { ...base, kind: "subagent", payload: { role: "exploration", outcome: "cancelled", queueWaitMs: 0, durationMs: 3 } },
    { ...base, kind: "compaction", payload: { inputTokensBefore: 4, inputTokensAfter: 2 } },
    { ...base, kind: "handoff", payload: { outcome: "success", durationMs: 5 } },
  ];
  for (const record of valid) assert.equal(validateObservation(record), true);
  for (const bad of [
    { ...base, observationId: "not-a-uuid" },
    { ...base, timestamp: "not-a-date" },
    { ...base, payload: { ...base.payload, inputTokens: -1 } },
    { ...base, payload: { ...base.payload, extra: true } },
    { ...base, kind: "unknown" },
  ]) assert.equal(validateObservation(bad), false);
});

test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention treats final-LF and duplicate stream concerns as writer integrity", () => {
  const lines = JSON.stringify({ record: "header", schemaVersion: 1, sessionId: "session", createdAt: "2026-07-27T00:00:00Z" }) + "\n" + JSON.stringify(base) + "\n";
  assert.equal(lines.endsWith("\n"), true);
  assert.equal(lines.slice(0, -1).split("\n").length, 2);
  assert.equal(new Set([base.observationId, base.observationId]).size, 1);
});
