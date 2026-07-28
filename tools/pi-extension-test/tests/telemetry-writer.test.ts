import assert from "node:assert/strict";
import { chmod, mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import test from "node:test";
import { appendSessionObservation } from "../../../.pi/extensions/awf-telemetry/index.ts";

const root = resolve(".");
const session = "session-1";
const stream = join(root, ".awf", "metrics", "sessions", `${session}.jsonl`);
const lock = `${stream}.lock`;
let gitReady = false;
async function ensureGit() {
  if (gitReady) return;
  const dir = "/tmp/awf-pi-test-bin";
  await mkdir(dir, { recursive: true });
  const script = `${dir}/git`;
  await writeFile(script, "#!/bin/sh\ncase \"$*\" in\n  *'rev-parse --show-toplevel'*) echo /workspace/repo;;\n  *'rev-parse --path-format=absolute --git-common-dir'*) echo /workspace/repo/.git;;\n  *'worktree list --porcelain -z'*) printf 'worktree /workspace/repo\\0HEAD test\\0branch refs/heads/main\\0\\0';;\n  *'rev-parse --path-format=absolute --git-dir'*) echo /workspace/repo/.git;;\nesac\n");
  await chmod(script, 0o755);
  process.env.PATH = `${dir}:${process.env.PATH ?? ""}`;
  gitReady = true;
}
const observation = (id: string, inputTokens = 1) => ({ record: "observation", schemaVersion: 1, observationId: id, timestamp: "2026-07-27T00:00:00Z", kind: "usage", payload: { inputTokens, outputTokens: 2, cacheReadTokens: 3, cacheWriteTokens: 4, costUsd: 0.5 } });

async function clean() { await rm(stream, { force: true }); await rm(lock, { force: true }); }
async function resetMetrics() { await rm(join(root, ".awf", "metrics"), { recursive: true, force: true }); }

test("invariant: tooling/workflow-telemetry:event-protocol-and-ledger direct writer publishes a linked root stream and retries idempotently", async () => {
  await clean();
  await resetMetrics();
  await ensureGit();
  const first = observation("123e4567-e89b-42d3-a456-426614174000");
  assert.deepEqual(await appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, first), { idempotent: false });
  const bytes = await readFile(stream, "utf8");
  assert.equal((await appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, first)).idempotent, true);
  assert.equal(await readFile(stream, "utf8"), bytes);
  const second = observation("123e4567-e89b-42d3-a456-426614174001", 9);
  assert.equal((await appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, second)).idempotent, false);
  assert.equal((await readFile(stream, "utf8")).split("\n").filter(Boolean).length, 3);
  await clean();
});

test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention rejects duplicate IDs before append and preserves bytes", async () => {
  await clean();
  await resetMetrics();
  await ensureGit();
  const first = observation("123e4567-e89b-42d3-a456-426614174002");
  await appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, first);
  const before = await readFile(stream, "utf8");
  await writeFile(stream, before + before.split("\n")[1] + "\n");
  const corrupted = await readFile(stream, "utf8");
  await assert.rejects(appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, observation("123e4567-e89b-42d3-a456-426614174003")), /duplicate observation ID/);
  assert.equal(await readFile(stream, "utf8"), corrupted);
  await clean();
});

test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention rejects malformed and unterminated streams without changing bytes", async () => {
  await clean();
  await mkdir(join(root, ".awf", "metrics", "sessions"), { recursive: true });
  await writeFile(stream, '{"record":"header"}\n{"broken":true}');
  const before = await readFile(stream, "utf8");
  await assert.rejects(appendSessionObservation(resolve(".pi/extensions/awf-telemetry/index.ts"), session, observation("123e4567-e89b-42d3-a456-426614174004")));
  assert.equal(await readFile(stream, "utf8"), before);
  await clean();
});
