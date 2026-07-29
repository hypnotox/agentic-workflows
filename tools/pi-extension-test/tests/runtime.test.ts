import assert from "node:assert/strict";
import test from "node:test";
import { buildRoutingCard, PREFERENCE_FIELDS } from "../../../.pi/extensions/awf-subagents/model-routing.ts";

test("pinned runtime routing-card fixture has one run-local system prompt card", () => {
  const state: any = { global: {}, project: {}, missing: [], invalid: [], blocked: false, errors: [], effective: {} };
  for (const field of PREFERENCE_FIELDS) state.effective[field] = { reference: `runtime/${field}`, scope: "global" };
  const prompt = `system\n\n${buildRoutingCard(state)}`;
  assert.equal((prompt.match(/\[awf subagent routing\]/g) ?? []).length, 1);
  assert.match(prompt, /roles: grounding=runtime\/grounding/);
});
