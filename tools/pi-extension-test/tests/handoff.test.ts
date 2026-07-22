import assert from "node:assert/strict";
import test from "node:test";
import { posix } from "node:path";
import { visibleWidth } from "@earendil-works/pi-tui";
import { Value } from "typebox/value";
import defaultHandoff, { MIN_PI_VERSION, buildKickoffWrapper, registerHandoff, requestHandoffAssociation, validateMemoryPath, validateTelemetryAssociation, versionSupported, type HandoffDependencies } from "../../../.pi/extensions/awf-handoff/index.ts";

const FILE = { isSymbolicLink: () => false, isFile: () => true, isDirectory: () => false };
const DIR = { isSymbolicLink: () => false, isFile: () => false, isDirectory: () => true };
const LINK = { isSymbolicLink: () => true, isFile: () => false, isDirectory: () => false };
function leaf(calls: Array<{ id: string; name: string }>) { return { type: "message", message: { role: "assistant", content: calls.map((call) => ({ type: "toolCall", ...call })) } }; }

function harness(options: { version?: string; entries?: Record<string, any>; mode?: string; persisted?: boolean; isPersistedAPI?: boolean; file?: string; queueCommandAPI?: boolean; queueError?: Error; uuidError?: Error; timerError?: Error; newSessionError?: Error; associationResponses?: unknown[]; associationListenerError?: Error; associationAppendError?: Error; childSessionId?: string } = {}) {
  const tools = new Map<string, any>(); const commands = new Map<string, any>(); const hooks = new Map<string, any>();
  const queued: any[] = []; const notifications: any[] = []; const newSessions: any[] = []; const editor: string[] = []; const associationEntries: any[] = []; const telemetry: any[] = [];
  const intervals: any[] = []; const timeouts: any[] = []; const clearedIntervals: any[] = []; const clearedTimeouts: any[] = [];
  const entries: Record<string, any> = { "/repo": DIR, "/repo/.awf": DIR, "/repo/.awf/memory": DIR, "/repo/.awf/memory/work.md": FILE, ...options.entries };
  let currentLeaf: any = leaf([{ id: "handoff", name: "handoff_session" }]); let component: any; let customDone: any; let sendError: Error | undefined; let inputError: Error | undefined; let sessionFile = options.file === undefined ? "/sessions/old.jsonl" : options.file;
  const pi: any = {
    on: (name: string, fn: any) => hooks.set(name, fn), registerTool: (tool: any) => tools.set(tool.name, tool), registerCommand: (name: string, command: any) => commands.set(name, command),
    appendEntry() {},
    events: { on() {}, emit: (name: string, request: any) => {
      if (options.associationListenerError) throw options.associationListenerError;
      if (name === "awf.telemetry.handoff.v1") { telemetry.push(request); return; }
      for (const response of options.associationResponses ?? []) request.respond(response);
    } },
    ...(options.queueCommandAPI === false ? {} : { queueCommand: (name: string, args: string) => { if (options.queueError) throw options.queueError; queued.push([name, args]); } }),
  };
  const deps: HandoffDependencies = {
    packageVersion: options.version ?? MIN_PI_VERSION, extensionFile: "/repo/.pi/extensions/awf-handoff/index.ts", path: posix,
    lstat: async (path) => { if (!(path in entries)) throw new Error(`ENOENT ${path}`); return entries[path]; },
    randomUUID: () => { if (options.uuidError) throw options.uuidError; return "uuid"; },
    setInterval: (callback, milliseconds) => { if (options.timerError) throw options.timerError; const handle = { callback, milliseconds }; intervals.push(handle); return handle; },
    clearInterval: (handle) => clearedIntervals.push(handle), setTimeout: (callback, milliseconds) => { const handle = { callback, milliseconds }; timeouts.push(handle); return handle; }, clearTimeout: (handle) => clearedTimeouts.push(handle),
  };
  registerHandoff(pi, deps);
  const sessionManager = {
    ...(options.isPersistedAPI === false ? {} : { isPersisted: () => options.persisted ?? true }),
    getSessionFile: () => sessionFile,
    getLeafEntry: () => currentLeaf,
  };
  const ui = {
    notify: (...args: any[]) => notifications.push(args), setEditorText: (text: string) => editor.push(text),
    custom: async (factory: any) => new Promise<boolean>((resolve, reject) => { customDone = (value: boolean) => queueMicrotask(() => resolve(value)); try { component = factory({ requestRender() {} }, {}, { matches: (data: string) => { if (inputError) throw inputError; return data === "escape" || data === "ctrl+c"; } }, customDone); } catch (e) { reject(e); } }),
  };
  const ctx: any = { mode: options.mode ?? "tui", sessionManager, ui, newSession: async (args: any) => { newSessions.push(args); if (options.newSessionError) throw options.newSessionError; await args.setup?.({ ...(options.childSessionId ? { getSessionId: () => options.childSessionId } : {}), appendCustomEntry: (customType: string, data: unknown) => { if (options.associationAppendError) throw options.associationAppendError; associationEntries.push({ customType, data }); } }); const replacementCtx = { ui, sendUserMessage: async (text: string) => { if (sendError) throw sendError; editor.push("sent:" + text); } }; await args.withSession(replacementCtx); return { cancelled: false }; } };
  return { pi, deps, tools, commands, hooks, queued, notifications, newSessions, editor, associationEntries, telemetry, intervals, timeouts, clearedIntervals, clearedTimeouts, entries, ctx,
    setLeaf: (value: any) => { currentLeaf = value; }, component: () => component, done: (value: boolean) => customDone(value), setSendError: (error: Error) => { sendError = error; }, setInputError: (error: Error) => { inputError = error; }, setSessionFile: (value: string) => { sessionFile = value; } };
}

async function execute(h: ReturnType<typeof harness>, params: any = { memoryPath: ".awf/memory/work.md", kickoff: "continue tests" }) { return h.tools.get("handoff_session").execute("handoff", params, undefined, undefined, h.ctx); }
async function startCommand(h: ReturnType<typeof harness>, id = "uuid") { return h.commands.get("awf-handoff-continue").handler(id, h.ctx); }

const association = { effortId: "effort", sessionId: "session", trajectoryId: "trajectory", associationOrigin: "handoff" } as const;
const associationResponse = { version: { major: 1, minor: 0 }, association };

const notice = Symbol.for("awf.pi.minimum-runtime-notified");
test("minimum guard and default factory use exact Pi 0.81.1", async () => {
  registerHandoff({} as any, { packageVersion: MIN_PI_VERSION } as any);
  assert.equal(versionSupported("0.81.1"), true); assert.equal(versionSupported("0.81.1-beta"), true); assert.equal(versionSupported("0.82.0"), true);
  for (const value of ["0.81.0", "invalid", "1.2", ""]) assert.equal(versionSupported(value), false);
  delete (globalThis as any)[notice]; const old = harness({ version: "0.81.0" }); assert.equal(old.tools.size, 0); assert.deepEqual([...old.hooks.keys()], ["session_start"]);
  await old.hooks.get("session_start")({}, old.ctx); await old.hooks.get("session_start")({}, old.ctx); assert.equal(old.notifications.length, 1); delete (globalThis as any)[notice];
  const official = harness({ queueCommandAPI: false }); assert.equal(official.tools.size, 0); assert.equal(official.commands.size, 0); assert.deepEqual([...official.hooks.keys()], ["session_start"]);
  await official.hooks.get("session_start")({}, official.ctx); assert.match(official.notifications[0][0], /Missing runtime APIs: queueCommand/); delete (globalThis as any)[notice];
  const fresh = harness(); const tools = new Map(); fresh.pi.registerTool = (tool: any) => tools.set(tool.name, tool); await defaultHandoff(fresh.pi); assert.equal(tools.has("handoff_session"), true);
});

test("closed schema and kickoff boundaries", async () => {
  const h = harness(); const schema = h.tools.get("handoff_session").parameters;
  assert.deepEqual(schema.required, ["memoryPath", "kickoff"]); assert.equal(schema.additionalProperties, false); assert.equal(schema.properties.kickoff.maxLength, 1000);
  assert.equal(Value.Check(schema, { memoryPath: ".awf/memory/work.md", kickoff: "x" }), true); assert.equal(Value.Check(schema, { memoryPath: "x", kickoff: "x", extra: true }), false);
  for (const kickoff of ["", " \t "]) await assert.rejects(execute(h, { memoryPath: ".awf/memory/work.md", kickoff }), /non-whitespace/);
  assert.equal((await execute(h, { memoryPath: ".awf/memory/work.md", kickoff: "x".repeat(1000) })).terminate, true);
  const astral = harness(); await assert.rejects(execute(astral, { memoryPath: ".awf/memory/work.md", kickoff: "😀".repeat(501) }), /UTF-16/);
});

test("path validation rejects every confinement, canonical, type, symlink, and missing class", async () => {
  const base = harness(); assert.equal(await validateMemoryPath(".awf/memory/work.md", base.deps), ".awf/memory/work.md");
  for (const value of ["", "/repo/.awf/memory/work.md", ".awf\\memory\\work.md", ".awf/memory", ".awf/memory/../work.md", ".awf/memory//work.md", ".awf/memory/./work.md", "other/work.md"])
    await assert.rejects(validateMemoryPath(value, base.deps));
  await assert.rejects(validateMemoryPath(".awf/memory/missing.md", base.deps), /Cannot access .*missing/);
  const escaping = { ...base.deps, path: { ...posix, relative: () => "../escape" } };
  await assert.rejects(validateMemoryPath(".awf/memory/work.md", escaping), /outside the repository/);
  for (const [path, entry] of [["/repo/.awf", LINK], ["/repo/.awf/memory/work.md", LINK], ["/repo/.awf/memory", FILE], ["/repo/.awf/memory/work.md", DIR]] as const) {
    const h = harness({ entries: { [path]: entry } }); await assert.rejects(validateMemoryPath(".awf/memory/work.md", h.deps));
  }
});

test("preflight fails closed and blocks every mixed sibling", async () => {
  const h = harness(); const preflight = h.hooks.get("tool_call");
  assert.equal(await preflight({ toolCallId: "handoff", toolName: "handoff_session" }, h.ctx), undefined);
  h.setLeaf(leaf([{ id: "handoff", name: "handoff_session" }, { id: "read", name: "read" }]));
  for (const event of [{ toolCallId: "handoff", toolName: "handoff_session" }, { toolCallId: "read", toolName: "read" }]) assert.match((await preflight(event, h.ctx)).reason, /cannot contain siblings/);
  for (const malformed of [undefined, { type: "custom" }, { type: "message", message: { role: "user", content: [] } }, leaf([{ id: "old", name: "handoff_session" }])]) {
    h.setLeaf(malformed); assert.match((await preflight({ toolCallId: "new", toolName: "handoff_session" }, h.ctx)).reason, /Cannot verify/);
    assert.equal(await preflight({ toolCallId: "new", toolName: "read" }, h.ctx), undefined);
  }
});

test("mode, persistence, pending, random, and queue failures are fail closed", async () => {
  for (const h of [harness({ mode: "print" }), harness({ mode: "json" }), harness({ mode: "rpc" }), harness({ isPersistedAPI: false }), harness({ persisted: false }), harness({ file: "" })]) {
    await assert.rejects(execute(h), /compatible persisted interactive/);
    assert.equal(h.queued.length, 0);
    assert.equal(h.newSessions.length, 0);
  }
  await assert.rejects(execute(harness({ uuidError: new Error("uuid failed") })), /uuid failed/);
  const queued = harness({ queueError: new Error("queue failed") }); await assert.rejects(execute(queued), /queue failed/); queued.pi.queueCommand = (name: string, id: string) => queued.queued.push([name, id]); assert.equal((await execute(queued)).terminate, true);
  const pending = harness(); const result = await execute(pending); assert.equal(result.terminate, true); assert.deepEqual(pending.queued, [["awf-handoff-continue", "uuid"]]); await assert.rejects(execute(pending), /already pending/);
  await assert.rejects(startCommand(pending, "wrong"), /matching pending/); await assert.rejects(startCommand(harness(), ""), /matching pending/);
});

test("countdown cancellation renders exact bounded text and consumes request", async () => {
  const h = harness(); await execute(h); const command = startCommand(h); await new Promise((resolve) => setImmediate(resolve));
  assert.deepEqual(h.component().render(200), ["Handoff to a fresh session in 5s - Esc/Ctrl+C to cancel"]); assert.ok(visibleWidth(h.component().render(12)[0]) <= 12);
  h.intervals[0].callback(); assert.match(h.component().render(200)[0], /4s/); h.component().invalidate(); h.component().handleInput("other"); h.component().handleInput("escape"); h.setInputError(new Error("input failed")); assert.throws(() => h.component().handleInput("x"), /input failed/); h.timeouts[0].callback(); await command;
  assert.deepEqual(h.notifications, [["Fresh-session handoff canceled."]]); assert.equal(h.newSessions.length, 0); assert.equal(h.telemetry.length, 1); assert.equal(h.telemetry[0].outcome, "failure"); assert.equal(h.telemetry[0].errorCategory, "handoff"); assert.ok(h.clearedIntervals.length > 0); assert.ok(h.clearedTimeouts.length > 0);
  await assert.rejects(startCommand(h), /matching pending/); h.component().dispose();
  const listenerFailure = harness({ associationListenerError: new Error("telemetry listener") }); await execute(listenerFailure); const listenerCommand = startCommand(listenerFailure); await new Promise((resolve) => setImmediate(resolve)); listenerFailure.component().handleInput("escape"); await listenerCommand; assert.equal(listenerFailure.notifications.length, 1);
});

test("timer setup, pending-window races, and pre-replacement failures preserve the old session", async () => {
  const timer = harness({ timerError: new Error("timer setup failed") }); await execute(timer); await assert.rejects(startCommand(timer), /timer setup failed/); assert.equal(timer.newSessions.length, 0); await assert.rejects(startCommand(timer), /matching pending/);
  const raced = harness(); await execute(raced); const raceCommand = startCommand(raced); await new Promise((resolve) => setImmediate(resolve)); raced.entries["/repo/.awf/memory/work.md"] = LINK; raced.timeouts[0].callback(); await assert.rejects(raceCommand, /symbolic link/); assert.equal(raced.newSessions.length, 0); await assert.rejects(startCommand(raced), /matching pending/);
  const unpersisted = harness(); await execute(unpersisted); const unpersistedCommand = startCommand(unpersisted); await new Promise((resolve) => setImmediate(resolve)); unpersisted.setSessionFile(""); unpersisted.timeouts[0].callback(); await assert.rejects(unpersistedCommand, /no longer persisted/); assert.equal(unpersisted.newSessions.length, 0);
  const h = harness(); await execute(h); const command = startCommand(h); await new Promise((resolve) => setImmediate(resolve)); h.timeouts[0].callback(); await command;
  assert.equal(h.newSessions[0].parentSession, "/sessions/old.jsonl"); assert.equal(h.telemetry.length, 0); assert.match(h.editor[0], /^sent:Read \.awf\/memory\/work\.md first/); assert.match(h.editor[0], /Repository sources and current-state documentation are authoritative/); assert.match(h.editor[0], /continue tests/); await assert.rejects(startCommand(h), /matching pending/); assert.equal(h.entries["/repo/.awf/memory/work.md"], FILE);
  assert.equal(buildKickoffWrapper(".awf/memory/work.md", "next"), "Read .awf/memory/work.md first. Repository sources and current-state documentation are authoritative over the checkpoint. Then continue with this immediate action: next");
});

test("invariant: handoff transfers exact association and success setup data", async () => {
  assert.equal(validateTelemetryAssociation(association), true);
  for (const invalid of [null, [], { ...association, extra: true }, { ...association, effortId: "../bad" }, { ...association, effortId: "x".repeat(129) }, { ...association, associationOrigin: "guess" }]) {
    assert.equal(validateTelemetryAssociation(invalid), false);
  }
  const h = harness({ associationResponses: [associationResponse], childSessionId: "child-session" });
  await execute(h); const command = startCommand(h); await new Promise((resolve) => setImmediate(resolve)); h.timeouts[0].callback(); await command;
  assert.deepEqual(h.associationEntries.map((entry) => entry.customType), ["awf.telemetry.association.v1", "awf.telemetry.handoff.pending.v1"]); assert.deepEqual(h.associationEntries[0].data, association); assert.deepEqual(Object.keys(h.associationEntries[1].data).sort(), ["durationMs", "observationId", "startedAt", "version"]); assert.equal(h.telemetry.length, 0);

  for (const options of [
    {},
    { associationResponses: [{ version: { major: 2, minor: 0 }, association }] },
    { associationResponses: [{ version: { major: 1, minor: 1 }, association }] },
    { associationResponses: [{ version: { major: 1, minor: 0 }, association: { ...association, effortId: "bad/id" } }] },
    { associationResponses: [associationResponse, associationResponse] },
    { associationListenerError: new Error("listener failed") },
    { associationResponses: [associationResponse], associationAppendError: new Error("append failed") },
  ]) {
    const degraded = harness(options as any);
    await execute(degraded); const next = startCommand(degraded); await new Promise((resolve) => setImmediate(resolve)); degraded.timeouts[0].callback(); await next;
    assert.deepEqual(degraded.associationEntries, []);
    assert.equal(degraded.editor.length, 1);
  }

  let lateRespond: ((value: unknown) => void) | undefined;
  const late: any = { events: { emit: (_name: string, request: any) => { lateRespond = request.respond; } } };
  assert.equal(requestHandoffAssociation(late), undefined);
  lateRespond?.(associationResponse);
  assert.equal(requestHandoffAssociation({ events: { emit() {} } } as any), undefined);
});

test("replacement rejection consumes pending without kickoff or memory deletion", async () => {
  const h = harness({ newSessionError: new Error("replacement failed") }); await execute(h); const command = startCommand(h); await new Promise((resolve) => setImmediate(resolve)); h.timeouts[0].callback();
  await assert.rejects(command, /replacement failed/); assert.equal(h.newSessions.length, 1); assert.equal(h.telemetry[0].outcome, "failure"); assert.deepEqual(h.editor, []); await assert.rejects(startCommand(h), /matching pending/); assert.equal(h.entries["/repo/.awf/memory/work.md"], FILE);
});

test("kickoff rejection leaves exact wrapper in replacement editor and no deletion API exists", async () => {
  const h = harness(); h.setSendError(new Error("kickoff failed")); await execute(h); const command = startCommand(h); await new Promise((resolve) => setImmediate(resolve)); h.timeouts[0].callback(); await command;
  assert.match(h.editor[0], /^Read \.awf\/memory\/work\.md first/); assert.deepEqual(h.notifications, [["Automatic kickoff failed; submit the prepared editor text.", "warning"]]); assert.equal(h.entries["/repo/.awf/memory/work.md"], FILE); await assert.rejects(startCommand(h), /matching pending/);
  for (const object of [h.pi, h.ctx, h.deps]) for (const key of Object.keys(object)) assert.doesNotMatch(key, /delete|unlink|rm/i);
});
