import assert from "node:assert/strict";
import test from "node:test";
import { boundedAdd } from "./smoke.ts";

test("boundedAdd returns uncapped and capped sums", () => {
  assert.equal(boundedAdd(2, 3, 10), 5);
  assert.equal(boundedAdd(7, 8, 10), 10);
});
