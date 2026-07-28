import test from "node:test";
import assert from "node:assert/strict";
import { validateObservation } from "../../../.pi/extensions/awf-telemetry/protocol.ts";
test("session-v1 descriptor validates usage", () => assert.equal(validateObservation({ record:"observation", schemaVersion:1, observationId:"123e4567-e89b-42d3-a456-426614174000", timestamp:"2026-07-27T00:00:00Z", kind:"usage", payload:{ inputTokens:0, outputTokens:0, cacheReadTokens:0, cacheWriteTokens:0, costUsd:0 } }), true));
