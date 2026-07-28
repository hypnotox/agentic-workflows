import assert from "node:assert/strict";
import { chmod, mkdir, mkdtemp, readFile, rm, stat, symlink, writeFile } from "node:fs/promises";
import { execFile } from "node:child_process";
import { join, resolve } from "node:path";
import { promisify } from "node:util";
import test from "node:test";
import { appendSessionObservation, parseWorktrees, resolveControlRoot, setTelemetryWriterTestFault, setTelemetryWriterTestLockCleanupRace, telemetryWriterTestState } from "../../../.pi/extensions/awf-telemetry/index.ts";
const exec = promisify(execFile);

const root = resolve(".");
const session = "session-1";
const stream = join(root, ".awf", "metrics", "sessions", `${session}.jsonl`);
const lock = `${stream}.lock`;
const corrupt = `${stream}.corrupt`;
let gitReady = false;
async function ensureGit() { if (gitReady) return; const dir = "/tmp/awf-pi-test-bin"; await mkdir(dir, { recursive: true }); const script = `${dir}/git`; await writeFile(script, "#!/bin/sh\ncase \"$*\" in\n  *'rev-parse --is-bare-repository'*) echo false;;\n  *'rev-parse --show-toplevel'*) echo /workspace/repo;;\n  *'rev-parse --path-format=absolute --git-common-dir'*) echo /workspace/repo/.git;;\n  *'worktree list --porcelain -z'*) printf 'worktree /workspace/repo\\0HEAD test\\0branch refs/heads/main\\0\\0';;\nesac\n"); await chmod(script, 0o755); process.env.PATH = `${dir}:${process.env.PATH ?? ""}`; gitReady = true; }
const observation = (id: string, inputTokens = 1) => ({ record: "observation", schemaVersion: 1, observationId: id, timestamp: "2026-07-27T00:00:00Z", kind: "usage", payload: { inputTokens, outputTokens: 2, cacheReadTokens: 3, cacheWriteTokens: 4, costUsd: 0.5 } });
const extension = resolve(".pi/extensions/awf-telemetry/index.ts");
async function clean() { setTelemetryWriterTestFault(); await rm(stream, { force: true }); await rm(lock, { force: true }); await rm(corrupt, { force: true }); }
async function resetMetrics() { await rm(join(root, ".awf", "metrics"), { recursive: true, force: true }); }
async function append(id: string) { await ensureGit(); return appendSessionObservation(extension, session, observation(id)); }
async function bytes() { return readFile(stream, "utf8"); }

// These closed porcelain bytes are mirrored in internal/worktree/topology_test.go.
const porcelain = [
  ["branch", "worktree /x\0HEAD abc\0branch refs/heads/x\0\0", true],
  ["detached", "worktree /x\0HEAD abc\0detached\0\0", true],
  ["bare", "worktree /bare\0bare\0\0", true],
  ["prunable", "worktree /x\0HEAD abc\0branch refs/heads/x\0prunable gone\0\0", true],
  ["missing final delimiter", "worktree /x\0HEAD abc\0branch refs/heads/x\0", false],
  ["missing HEAD", "worktree /x\0branch refs/heads/x\0\0", false],
  ["valueless HEAD", "worktree /x\0HEAD \0branch refs/heads/x\0\0", false],
  ["valueless branch", "worktree /x\0HEAD abc\0branch \0\0", false],
  ["valueless prunable", "worktree /x\0HEAD abc\0branch refs/heads/x\0prunable \0\0", false],
  ["missing state", "worktree /x\0HEAD abc\0\0", false],
  ["branch detached", "worktree /x\0HEAD abc\0branch refs/heads/x\0detached\0\0", false],
  ["detached value", "worktree /x\0HEAD abc\0detached nope\0\0", false],
  ["detached separator", "worktree /x\0HEAD abc\0detached \0\0", false],
  ["locked", "worktree /x\0HEAD abc\0branch refs/heads/x\0locked reason\0\0", false],
  ["unknown", "worktree /x\0HEAD abc\0branch refs/heads/x\0future x\0\0", false],
  ["duplicate HEAD", "worktree /x\0HEAD abc\0HEAD def\0branch refs/heads/x\0\0", false],
  ["duplicate branch", "worktree /x\0HEAD abc\0branch refs/heads/x\0branch refs/heads/y\0\0", false],
  ["duplicate prunable", "worktree /x\0HEAD abc\0branch refs/heads/x\0prunable x\0prunable y\0\0", false],
  ["duplicate bare", "worktree /x\0bare\0bare\0\0", false],
  ["bare fields", "worktree /x\0bare nope\0\0", false],
  ["bare after HEAD", "worktree /x\0HEAD abc\0bare\0\0", false],
  ["bare separator", "bare \0\0", false],
  ["empty record", "worktree /x\0HEAD abc\0branch refs/heads/x\0\0\0\0", false],
  ["duplicate worktree", "worktree /x\0worktree /y\0HEAD abc\0branch refs/heads/x\0\0", false],
  ["HEAD before worktree", "HEAD abc\0worktree /x\0branch refs/heads/x\0\0", false],
  ["branch before worktree", "branch refs/heads/x\0worktree /x\0HEAD abc\0\0", false],
  ["detached before worktree", "detached\0worktree /x\0HEAD abc\0\0", false],
  ["bare before worktree", "bare\0\0", false],
] as const;
for (const [name, raw, accepted] of porcelain) test(`worktree porcelain parity ${name}`, () => { if (accepted) assert.doesNotThrow(() => parseWorktrees(raw)); else assert.throws(() => parseWorktrees(raw)); });

async function nativeGit(...args:string[]) { await exec("git", args); }
async function commitNativeRepo(repo:string) { await writeFile(join(repo,"tracked"),"base\n"); await nativeGit("-C",repo,"add","tracked"); await nativeGit("-C",repo,"-c","user.name=test","-c","user.email=test@example.com","commit","-m","base"); }
test("native Git control-root parity covers primary, linked, and separate git-dir checkouts", async () => { const base=await mkdtemp("/tmp/awf-pi-control-"); try { const primary=join(base,"primary"); await nativeGit("init",primary); await commitNativeRepo(primary); const linked=join(base,"linked"); await nativeGit("-C",primary,"worktree","add","--detach",linked,"HEAD"); assert.deepEqual(await resolveControlRoot(primary),{invokingRoot:primary,primaryRoot:primary}); assert.deepEqual(await resolveControlRoot(linked),{invokingRoot:linked,primaryRoot:primary}); const separate=join(base,"separate"); const common=join(base,"separate.git"); await nativeGit("init","--separate-git-dir",common,separate); await commitNativeRepo(separate); const separateLinked=join(base,"separate-linked"); await nativeGit("-C",separate,"worktree","add","--detach",separateLinked,"HEAD"); assert.deepEqual(await resolveControlRoot(separate),{invokingRoot:separate,primaryRoot:separate}); await assert.rejects(resolveControlRoot(separateLinked),/no unique primary/); } finally { await rm(base,{recursive:true,force:true}); } });

test("invariant: tooling/workflow-telemetry:event-protocol-and-ledger direct writer publishes a linked root stream and retries idempotently", async () => { await clean(); await resetMetrics(); const first = observation("123e4567-e89b-42d3-a456-426614174000"); assert.deepEqual(await append(first.observationId), { idempotent: false }); const original = await bytes(); assert.equal((await appendSessionObservation(extension, session, first)).idempotent, true); assert.equal(await bytes(), original); const second = observation("123e4567-e89b-42d3-a456-426614174001", 9); assert.equal((await appendSessionObservation(extension, session, second)).idempotent, false); assert.equal(telemetryWriterTestState().appendWriteCalls, 1); assert.equal((await bytes()).split("\n").filter(Boolean).length, 3); await clean(); });

test("append preserves inode and prefix without a rename", async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174010"); const before = await bytes(), inode = (await stat(stream)).ino; await append("123e4567-e89b-42d3-a456-426614174011"); assert.equal((await stat(stream)).ino, inode); assert.ok((await bytes()).startsWith(before)); assert.equal(telemetryWriterTestState().appendWriteCalls, 1); await clean(); });

test("short append writes loop to the verified full canonical line", async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174009"); setTelemetryWriterTestFault("short-write"); await appendSessionObservation(extension,session,observation("123e4567-e89b-42d3-a456-426614174008")); const calls=telemetryWriterTestState().appendWriteCalls; setTelemetryWriterTestFault(); assert.ok(calls>1); assert.equal((await bytes()).split("\n").filter(Boolean).length,3); await clean(); });
test("simultaneous identical first publication leaves one canonical observation", async () => { await clean(); await resetMetrics(); const id="123e4567-e89b-42d3-a456-426614174012"; const results=await Promise.allSettled([append(id),append(id)]); assert.ok(results.some((result)=>result.status==="fulfilled")); const text=await bytes(); assert.equal(text.match(new RegExp(id,"g"))?.length,1); await clean(); });
test("simultaneous distinct first publication never replaces a published stream", async () => { await clean(); await resetMetrics(); const results=await Promise.allSettled([append("123e4567-e89b-42d3-a456-426614174013"),append("123e4567-e89b-42d3-a456-426614174014")]); assert.ok(results.some((result)=>result.status==="fulfilled")); const text=await bytes(); assert.equal(text.split("\n").filter(Boolean).length,2); await clean(); });

test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention rejects duplicate IDs before append and preserves bytes", async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174002"); const before = await bytes(); await writeFile(stream, before + before.split("\n")[1] + "\n"); const corrupted = await bytes(); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174003"), /duplicate observation ID/); assert.equal(await bytes(), corrupted); await clean(); });

test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention refuses malformed and unterminated streams without changing bytes", async () => { await clean(); await mkdir(join(root, ".awf", "metrics", "sessions"), { recursive: true }); await writeFile(stream, '{"record":"header"}\n{"broken":true}'); const before = await bytes(); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174004")); assert.equal(await bytes(), before); await clean(); });
test("invariant: tooling/workflow-telemetry:privacy-integrity-and-retention refuses a symlinked resident ancestor without touching outside bytes", async () => { await resetMetrics(); await ensureGit(); const outside = join(root, ".outside-metrics"); await rm(outside, { recursive: true, force: true }); await mkdir(outside, { recursive: true }); await writeFile(join(outside, "sentinel"), "outside bytes"); await mkdir(join(root, ".awf"), { recursive: true }); await symlink(outside, join(root, ".awf", "metrics")); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174005"), /unsafe telemetry directory/); assert.equal(await readFile(join(outside, "sentinel"), "utf8"), "outside bytes"); await rm(join(root, ".awf", "metrics"), { force: true }); await rm(outside, { recursive: true, force: true }); });

for (const stage of ["open", "read", "link", "directory", "directory-fsync", "lock-open", "lock-write", "lock-fsync"] as const) test(`writer fault ${stage} refuses before unsafe mutation`, async () => { await clean(); await resetMetrics(); await ensureGit(); setTelemetryWriterTestFault(stage); await assert.rejects(appendSessionObservation(extension, session, observation("123e4567-e89b-42d3-a456-426614174020"))); assert.equal(await rm(stream,{force:false}).then(()=>false,()=>true),true); setTelemetryWriterTestFault(); });
for (const stage of ["lock-cleanup"] as const) test(`writer ${stage} fault leaves a visible published stream`, async () => { await clean(); await resetMetrics(); await ensureGit(); setTelemetryWriterTestFault(stage); await assert.rejects(appendSessionObservation(extension,session,observation("123e4567-e89b-42d3-a456-426614174029"))); setTelemetryWriterTestFault(); assert.equal((await bytes()).split("\n").filter(Boolean).length,2); await clean(); });
for (const stage of ["write", "fsync"] as const) test(`writer append ${stage} fault preserves prefix and reports corruption`, async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174030"); const before=await bytes(), inode=(await stat(stream)).ino; setTelemetryWriterTestFault(stage); await assert.rejects(appendSessionObservation(extension,session,observation("123e4567-e89b-42d3-a456-426614174031"))); setTelemetryWriterTestFault(); const after=await bytes(); assert.ok(after.startsWith(before)); assert.equal((await stat(stream)).ino,inode); assert.ok(await readFile(corrupt,"utf8")); await clean(); });
test("lock acquisition failures clean empty payloads but never remove a replacement", async () => { await clean(); await resetMetrics(); await ensureGit(); setTelemetryWriterTestFault("lock-write"); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174032")); setTelemetryWriterTestFault(); assert.equal(await rm(lock,{force:false}).then(()=>false,()=>true),true); await clean(); setTelemetryWriterTestFault("lock-fsync"); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174033")); setTelemetryWriterTestFault(); assert.equal(await rm(lock,{force:false}).then(()=>false,()=>true),true); await clean(); setTelemetryWriterTestFault("lock-write"); setTelemetryWriterTestLockCleanupRace(async()=>{await rm(lock,{force:true});await writeFile(lock,"replacement\\n")}); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174038")); setTelemetryWriterTestFault(); setTelemetryWriterTestLockCleanupRace(); assert.equal(await readFile(lock,"utf8"),"replacement\\n"); await clean(); });
test("corruption markers refuse symlinks and directories and accept only the expected safe marker", async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174034"); const outside=join(root,".outside-corrupt"); await rm(outside,{force:true}); await writeFile(outside,"sentinel"); await symlink(outside,corrupt); setTelemetryWriterTestFault("write"); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174035"),/corruption marker/); setTelemetryWriterTestFault(); assert.equal(await readFile(outside,"utf8"),"sentinel"); await rm(corrupt,{force:true}); await mkdir(corrupt); setTelemetryWriterTestFault("write"); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174036"),/corruption marker/); setTelemetryWriterTestFault(); await rm(corrupt,{recursive:true}); await writeFile(corrupt,JSON.stringify({stream:`${session}.jsonl`,reason:"append-failure"})+"\n"); setTelemetryWriterTestFault("write"); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174037")); setTelemetryWriterTestFault(); assert.ok(await readFile(corrupt,"utf8")); await rm(outside,{force:true}); await clean(); });

test("live, malformed, unknown-PID, and symlink locks refuse without stream changes", async () => { await clean(); await resetMetrics(); await append("123e4567-e89b-42d3-a456-426614174040"); const before=await bytes(); for(const value of ["{bad", JSON.stringify({pid:"unknown",sessionId:session,createdAt:new Date().toISOString()})+"\n", JSON.stringify({pid:process.pid,sessionId:session,createdAt:new Date().toISOString()})+"\n"]) { await writeFile(lock,value); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174041")); assert.equal(await bytes(),before); await rm(lock,{force:true}); } await symlink(stream,lock); await assert.rejects(append("123e4567-e89b-42d3-a456-426614174042")); assert.equal(await bytes(),before); await rm(lock,{force:true}); await clean(); });
test("stale dead PID lock recovers", async () => { await clean(); await resetMetrics(); await mkdir(join(root,".awf","metrics","sessions"),{recursive:true}); await writeFile(lock,JSON.stringify({pid:999999,sessionId:session,createdAt:"2000-01-01T00:00:00.000Z"})+"\n"); await append("123e4567-e89b-42d3-a456-426614174043"); assert.equal((await bytes()).split("\n").filter(Boolean).length,2); await clean(); });
