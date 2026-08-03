import assert from "node:assert/strict";
import test from "node:test";
import { Value } from "typebox/value";
import { activity, EffortProtocolError, type ActivityCondition, type ActivityReply } from "../../../.pi/extensions/awf-effort/client.ts";
import effortExtension, {
  registerEffort,
  type EffortTransferCoordinator,
  type RemotePiAssignedNameDiagnosticPayload,
  type RemotePiCapabilitiesReplyPayload,
  type RemotePiCapabilitiesRequestPayload,
  type RemotePiMetadataReplayRequestPayload,
  type RemotePiMetadataSetPayload,
  type RemotePiNameOverrideReplayRequestPayload,
  type RemotePiNameOverrideSetPayload,
} from "../../../.pi/extensions/awf-effort/index.ts";

const T0 = "2026-08-02T00:00:00Z";
const T1 = "2026-08-02T01:00:00.123456789Z";
const OWNER = "00000000-0000-4000-8000-000000000001";
const PRIOR = "00000000-0000-4000-8000-000000000099";
const metadataReplay: RemotePiMetadataReplayRequestPayload = undefined;
const capabilityRequest: RemotePiCapabilitiesRequestPayload = undefined;
const capabilityReply: RemotePiCapabilitiesReplyPayload = { metadata: { version: 1 }, nameOverride: { version: 1, namespaces: ["awf"] } };
const nameOverrideReplay: RemotePiNameOverrideReplayRequestPayload = undefined;
const assignedDiagnostic: RemotePiAssignedNameDiagnosticPayload = { namespace: "awf", requested: "demo", assigned: "demo#2", active: true, changed: true };

function fact(owner = OWNER, cwd = "/repo", role: "managed" | "receiving" = "managed", heartbeatAt = T1) {
  return { schemaVersion: 1, owner, attachedAt: T0, heartbeatAt, cwd, receivingCheckout: "/repo", role } as const;
}
function effort(slug = "demo") { return { slug, title: `Title ${slug}` }; }
function memory(slug = "demo") { return { effort: slug, phase: "Build", next: "Run tests", updated: T1 }; }
function destination(cwd = "/repo", role: "managed" | "receiving" = "managed") { return { cwd, role, receivingCheckout: "/repo" }; }
function outcome(condition: ActivityCondition, overrides: Record<string, unknown> = {}) {
  return { category: "operation" as const, condition, changedActivity: false, changedMemory: false, changedCwd: false, nextActions: ["repair and retry"], ...overrides };
}
function envelope(condition: ActivityCondition, options: { slug?: string; owner?: string; cwd?: string; role?: "managed" | "receiving"; heartbeatAt?: string; priorHeartbeat?: string; sentinel?: boolean; cause?: string } = {}): ActivityReply {
  const slug = options.slug ?? "demo";
  const owner = options.owner ?? OWNER;
  const cwd = options.cwd ?? "/repo";
  const role = options.role ?? "managed";
  const common = { schemaVersion: 1 as const, condition };
  switch (condition) {
    case "ready": return { ...common, effort: effort(slug), memory: { ...memory(slug), ...(options.sentinel ? { updated: "Not yet updated." } : {}) }, destination: destination(cwd, role) };
    case "attached": return { ...common, effort: effort(slug), memory: memory(slug), activity: fact(owner, cwd, role, options.heartbeatAt) };
    case "taken-over": return { ...common, effort: effort(slug), memory: memory(slug), activity: fact(owner, cwd, role, options.heartbeatAt), priorClaim: fact(PRIOR, "/prior", "receiving", options.priorHeartbeat ?? T0) };
    case "heartbeat":
    case "checkout-updated": return { ...common, effort: effort(slug), memory: memory(slug), activity: fact(owner, cwd, role, options.heartbeatAt) };
    case "detached": return { ...common, effort: effort(slug) };
    case "not-owner": return { ...common, effort: effort(slug), activity: fact(PRIOR, cwd, role), outcome: outcome(condition) };
    case "missing": return { ...common, outcome: outcome(condition) };
    case "invalid-memory": return { ...common, effort: effort(slug), outcome: outcome(condition) };
    case "unsafe-resident":
    case "repository-mismatch": return { ...common, outcome: outcome(condition, options.cause ? { cause: options.cause } : {}) };
  }
}
function line(reply: unknown): string { return `${JSON.stringify(reply)}\n`; }
function clone(value: unknown): any { return JSON.parse(JSON.stringify(value)); }

async function decode(value: unknown) {
  return activity(async () => ({ code: 0, stdout: line(value), stderr: "" }), "/repo", ["resolve", "demo"]);
}

test("activity client accepts every exact protocol matrix and preserves immutable facts", async () => {
  for (const condition of ["ready", "attached", "taken-over", "heartbeat", "checkout-updated", "detached", "not-owner", "missing", "invalid-memory", "unsafe-resident", "repository-mismatch"] as const) {
    const reply = await decode(envelope(condition, condition === "ready" ? { sentinel: true } : {}));
    assert.equal(reply.condition, condition);
    assert.equal(Object.isFrozen(reply), true);
    if (reply.activity) assert.equal(Object.isFrozen(reply.activity), true);
    if (reply.memory) assert.equal(Object.isFrozen(reply.memory), true);
    if (reply.outcome) assert.equal(Object.isFrozen(reply.outcome.nextActions), true);
  }
  const readyWithPrior = { ...envelope("ready"), priorClaim: fact(PRIOR) };
  assert.equal((await decode(readyWithPrior)).priorClaim?.owner, PRIOR);
  assert.equal((await decode({ schemaVersion: 1, condition: "detached" })).effort, undefined);
  const caused = await decode(envelope("repository-mismatch", { cause: "git unavailable" }));
  assert.equal(caused.outcome?.cause, "git unavailable");
});

test("activity client rejects malformed transport and bounds diagnostics", async () => {
  const cases: Array<[() => Promise<any>, RegExp]> = [
    [() => activity(async () => { throw new Error("spawn failed"); }, "/repo", []), /execution failed/],
    [() => activity(async () => { throw "string failure"; }, "/repo", []), /execution failed/],
    [() => activity(async () => ({ code: 3, stderr: "bad" }), "/repo", []), /activity failed/],
    [() => activity(async () => ({ code: 0, stdout: "", stderr: "" }), "/repo", []), /single JSON/],
    [() => activity(async () => ({ code: 0, stdout: "\n", stderr: "" }), "/repo", []), /single JSON/],
    [() => activity(async () => ({ code: 0, stdout: "{}", stderr: "" }), "/repo", []), /single JSON/],
    [() => activity(async () => ({ code: 0, stdout: `${line(envelope("ready"))}${line(envelope("ready"))}`, stderr: "" }), "/repo", []), /single JSON/],
    [() => activity(async () => ({ code: 0, stdout: "{]\n", stderr: "" }), "/repo", []), /malformed JSON/],
    [() => activity(async () => ({ code: 0, stdout: "x".repeat(50 * 1024 + 1), stderr: "" }), "/repo", []), /exceeded bounds/],
    [() => activity(async () => ({ code: 0, stdout: line(envelope("ready")), stderr: "x".repeat(50 * 1024 + 1) }), "/repo", []), /exceeded bounds/],
  ];
  for (const [run, expected] of cases) await assert.rejects(run(), expected);
  const error = await activity(async (command, argv, options) => {
    assert.equal(command, "./awf");
    assert.deepEqual(argv, ["effort", "activity", "heartbeat", "demo", "--json"]);
    assert.equal(options.cwd, "/repo");
    assert.equal(options.timeout, 15_000);
    assert.equal(options.signal?.aborted, false);
    return { stdout: line(envelope("heartbeat")) };
  }, "/repo", ["heartbeat", "demo"], new AbortController().signal);
  assert.equal(error.condition, "heartbeat");
  assert.equal(new EffortProtocolError("x").name, "EffortProtocolError");
});

test("activity client rejects every malformed closed fact and presence branch", async () => {
  const invalid: unknown[] = [null, [], {}, { schemaVersion: 2, condition: "ready" }, { schemaVersion: 1, condition: 2 }, { schemaVersion: 1, condition: "other" }];
  for (const condition of ["ready", "attached", "taken-over", "heartbeat", "checkout-updated", "detached", "not-owner", "missing", "invalid-memory", "unsafe-resident", "repository-mismatch"] as const) {
    const extra = { ...envelope(condition), extra: true };
    invalid.push(extra);
    const source = clone(envelope(condition));
    const removable = Object.keys(source).find((key) => !["schemaVersion", "condition"].includes(key));
    if (removable && !(condition === "detached" && removable === "effort")) { delete source[removable]; invalid.push(source); }
  }
  const badEfforts = [null, [], { slug: "demo" }, { slug: "", title: "x" }, { slug: "demo", title: "" }, { slug: "demo", title: "x", extra: 1 }, { slug: "x".repeat(256), title: "x" }];
  for (const value of badEfforts) invalid.push({ ...envelope("ready"), effort: value });
  const badMemory = [null, [], { effort: "demo" }, { effort: "demo", phase: "", next: "x", updated: T1 }, { effort: "demo", phase: "x", next: "", updated: T1 }, { effort: "demo", phase: "x", next: "x", updated: "bad" }, { effort: "other", phase: "x", next: "x", updated: T1 }, { ...memory(), extra: 1 }, { ...memory(), phase: "x".repeat(501) }];
  for (const value of badMemory) invalid.push({ ...envelope("ready"), memory: value });
  const badDestinations = [null, [], { cwd: "/repo" }, { cwd: "relative", role: "managed", receivingCheckout: "/repo" }, { cwd: "/repo/..", role: "managed", receivingCheckout: "/repo" }, { cwd: "/repo", role: "bad", receivingCheckout: "/repo" }, { cwd: "/repo", role: "managed", receivingCheckout: "relative" }, { ...destination(), extra: 1 }];
  for (const value of badDestinations) invalid.push({ ...envelope("ready"), destination: value });
  const badActivities = [null, [], { schemaVersion: 1 }, { ...fact(), schemaVersion: 2 }, { ...fact(), owner: "BAD" }, { ...fact(), attachedAt: "bad" }, { ...fact(), heartbeatAt: "bad" }, { ...fact(), cwd: "relative" }, { ...fact(), receivingCheckout: "relative" }, { ...fact(), role: "bad" }, { ...fact(), extra: 1 }];
  for (const value of badActivities) invalid.push({ ...envelope("attached"), activity: value });
  const badOutcomes = [null, [], {}, { ...outcome("missing"), category: "bad" }, { ...outcome("missing"), condition: "unsafe-resident" }, { ...outcome("missing"), changedActivity: 1 }, { ...outcome("missing"), changedMemory: 1 }, { ...outcome("missing"), changedCwd: 1 }, { ...outcome("missing"), nextActions: [] }, { ...outcome("missing"), nextActions: [""] }, { ...outcome("missing"), cause: "" }, { ...outcome("missing"), extra: 1 }];
  for (const value of badOutcomes) invalid.push({ ...envelope("missing"), outcome: value });
  invalid.push({ ...envelope("attached"), outcome: outcome("attached") });
  invalid.push({ ...envelope("taken-over"), priorClaim: { ...fact(PRIOR), owner: "bad" } });
  for (const value of invalid) await assert.rejects(decode(value), /invalid envelope/);
});

type Harness = ReturnType<typeof makeHarness>;
type ReplyOverride = ActivityReply | Error | string | ((argv: readonly string[]) => ActivityReply | Promise<ActivityReply>);
function flag(argv: readonly string[], name: string): string | undefined { const index = argv.indexOf(name); return index < 0 ? undefined : argv[index + 1]; }
function makeUUIDSource(start = 1) {
  let next = start;
  return () => `00000000-0000-4000-8000-${String(next++).padStart(12, "0")}`;
}
function makeHarness(options: { cwd?: string; shared?: EffortTransferCoordinator; overrides?: Record<string, ReplyOverride[]>; uuid?: () => string; now?: () => Date; wait?: (milliseconds: number) => Promise<void>; emitThrows?: Set<string>; events?: boolean } = {}) {
  const tools = new Map<string, any>();
  const commands = new Map<string, any>();
  const hooks = new Map<string, any>();
  const listeners = new Map<string, any>();
  const events: Array<[string, any]> = [];
  const calls: readonly string[][] = [];
  const notices: string[] = [];
  const queued: Array<[string, string]> = [];
  const overrides = new Map(Object.entries(options.overrides ?? {}).map(([key, values]) => [key, [...values]]));
  const pi: any = {
    registerTool: (tool: any) => tools.set(tool.name, tool),
    registerCommand: (name: string, command: any) => commands.set(name, command),
    queueCommand: (name: string, argument: string) => queued.push([name, argument]),
    on: (name: string, handler: any) => hooks.set(name, handler),
    ...(options.events === false ? {} : { events: {
      emit: (name: string, payload: any) => { if (options.emitThrows?.has(name)) throw new Error("event failure"); events.push([name, payload]); },
      on: (name: string, handler: any) => listeners.set(name, handler),
    } }),
    exec: async (_command: string, argv: readonly string[]) => {
      (calls as string[][]).push([...argv]);
      const action = argv[2]!;
      const next = overrides.get(action)?.shift();
      if (next instanceof Error) throw next;
      if (typeof next === "string") return { code: 0, stdout: next, stderr: "" };
      const chosen = typeof next === "function" ? await next(argv) : next;
      const slug = argv[3] ?? "demo";
      const owner = flag(argv, "--owner") ?? OWNER;
      const cwd = flag(argv, "--cwd") ?? options.cwd ?? "/repo";
      const role = (flag(argv, "--role") ?? flag(argv, "--destination") ?? "managed") as "managed" | "receiving";
      const fallback = action === "resolve" ? envelope("ready", { slug, cwd: role === "receiving" ? (flag(argv, "--receiving-checkout") ?? cwd) : cwd, role })
        : action === "attach" ? envelope("attached", { slug, owner, cwd, role })
        : action === "checkout" ? envelope("checkout-updated", { slug, owner, cwd, role })
        : action === "heartbeat" ? envelope("heartbeat", { slug, owner, cwd, role })
        : envelope("detached", { slug });
      return { code: 0, stdout: line(chosen ?? fallback), stderr: "" };
    },
  };
  const ctx: any = {
    cwd: options.cwd ?? "/repo",
    changeCwd: async (cwd: string, changeOptions?: { withSession?: (ctx: any) => Promise<void> }) => {
      const changed = cwd !== ctx.cwd;
      if (!changed) return { changed: false, cwd };
      ctx.cwd = cwd;
      await changeOptions?.withSession?.(ctx);
      return { changed: true, cwd };
    },
    ui: {
      notify: (message: string) => notices.push(message),
      setStatus: (_key: string, value: string | undefined) => { if (value) notices.push(value); },
    },
  };
  registerEffort(pi, { uuid: options.uuid ?? makeUUIDSource(), coordinator: options.shared ?? {}, now: options.now, wait: options.wait });
  return { pi, ctx, tools, commands, hooks, listeners, events, calls, notices, queued, overrides };
}
async function request(h: Harness, args: any) { return h.tools.get("using_effort").execute("call", args); }
async function continueRequest(h: Harness, token = h.queued.at(-1)?.[1]) { return h.commands.get("awf-using-effort-continue").handler(token, h.ctx); }
function emitted(h: Harness, name: string) { return h.events.filter(([event]) => event === name).map(([, payload]) => payload); }

async function attachSameCwd(h: Harness, slug = "demo") {
  await request(h, { effort: slug, destination: "managed" });
  await continueRequest(h);
}

test("default extension factory uses the process coordinator and registers the runtime", () => {
  const tools: unknown[] = [];
  const commands: string[] = [];
  effortExtension({
    exec: async () => ({ code: 1 }),
    registerTool: (tool: unknown) => tools.push(tool),
    registerCommand: (name: string) => commands.push(name),
    queueCommand: () => undefined,
  });
  assert.equal(tools.length, 1);
  assert.deepEqual(commands, ["awf-using-effort-continue"]);
});

test("using_effort validates explicit inputs, queues only a private command, and capability-degrades", async () => {
  const h = makeHarness();
  for (const args of [{}, { effort: "Bad", destination: "managed" }, { effort: "demo" }, { effort: "demo", destination: "receiving", receivingCheckout: "relative" }, { detach: true, destination: "managed" }, { detach: true, receivingCheckout: "/repo" }]) await assert.rejects(request(h, args));
  const queued = await request(h, { effort: "demo", destination: "receiving", receivingCheckout: "/repo" });
  assert.equal(queued.terminate, true);
  assert.equal(h.calls.length, 0);
  assert.equal(h.queued[0]?.[0], "awf-using-effort-continue");
  delete h.ctx.changeCwd;
  await continueRequest(h);
  assert.match(h.notices.join(" "), /using_effort requires Pi command-context changeCwd; changedCwd=false changedActivity=false changedMemory=false/);
  assert.equal(h.ctx.cwd, "/repo");
  assert.equal(h.calls.length, 0);
  await h.hooks.get("turn_end")({}, h.ctx);
  assert.equal(h.calls.length, 0);
  await continueRequest(h, "stale-token");
  assert.match(h.notices.join(" "), /no longer current/);
});

test("using_effort accepts 63-byte residents and rejects 64-byte slugs", async () => {
  const resident = "r".repeat(63);
  const rejected = "r".repeat(64);
  const h = makeHarness();
  const schema = h.tools.get("using_effort").parameters;
  assert.equal(Value.Check(schema, { effort: resident, destination: "managed" }), true);
  assert.equal(Value.Check(schema, { effort: rejected, destination: "managed" }), false);
  await attachSameCwd(h, resident);
  assert.deepEqual(h.calls.slice(0, 2).map((argv) => argv[3]), [resident, resident]);
});

test("same-checkout attach, heartbeat, replay, collision diagnostics, and explicit detach use advisory publication", async () => {
  const h = makeHarness();
  assert.equal(emitted(h, "remote-pi:capabilities:request")[0] as RemotePiCapabilitiesRequestPayload, capabilityRequest);
  h.listeners.get("remote-pi:capabilities")(capabilityReply, h.ctx);
  await attachSameCwd(h);
  assert.deepEqual(h.calls.slice(0, 2).map((argv) => argv[2]), ["resolve", "attach"]);
  assert.equal(flag(h.calls[1]!, "--receiving-checkout"), "/repo");
  const metadataSet = emitted(h, "remote-pi:metadata:set").at(-1) as RemotePiMetadataSetPayload;
  const nameOverrideSet = emitted(h, "remote-pi:name-override:set").at(-1) as RemotePiNameOverrideSetPayload;
  assert.equal(metadataSet.value?.effort.slug, "demo");
  assert.equal(nameOverrideSet.value, "demo");
  h.listeners.get("remote-pi:metadata:request")(metadataReplay, h.ctx);
  h.listeners.get("remote-pi:metadata:request")(metadataReplay);
  h.listeners.get("remote-pi:name-override:request")(nameOverrideReplay, h.ctx);
  h.listeners.get("remote-pi:name-override:request")(nameOverrideReplay);
  h.listeners.get("remote-pi:name-override:assigned")(assignedDiagnostic, h.ctx);
  h.listeners.get("remote-pi:name-override:assigned")({ namespace: "awf", requested: "demo", assigned: "demo#3", active: true, changed: true });
  h.listeners.get("remote-pi:name-override:assigned")({ namespace: "other", active: true, changed: true, assigned: "ignored" }, h.ctx);
  assert.match(h.notices.join(" "), /collision-assigned/);
  await h.hooks.get("turn_end")({}, h.ctx);
  assert.equal(h.calls.at(-1)?.[2], "heartbeat");
  await request(h, { detach: true });
  await continueRequest(h);
  assert.equal(h.calls.at(-1)?.[2], "detach");
  assert.equal(emitted(h, "remote-pi:metadata:set").at(-1).value, null);
  assert.equal(emitted(h, "remote-pi:name-override:set").at(-1).value, null);
  await request(h, { detach: true });
  await continueRequest(h);
  assert.equal(emitted(h, "remote-pi:metadata:set").at(-1).value, null);
  await h.hooks.get("turn_end")({}, h.ctx);
});

test("complete Remote Pi absence preserves local resolve, switching, heartbeat, and detach", async () => {
  const shared: EffortTransferCoordinator = {};
  const source = makeHarness({ shared, cwd: "/receiving", events: false });
  source.overrides.set("resolve", [envelope("ready", { cwd: "/managed", role: "managed" })]);
  let managed: Harness | undefined;
  source.ctx.changeCwd = async (cwd: string, options: any) => {
    await source.hooks.get("session_shutdown")({ reason: "cwd", targetCwd: cwd }, source.ctx);
    managed = makeHarness({ shared, cwd, events: false, uuid: makeUUIDSource(100) });
    await managed.hooks.get("session_start")({ reason: "cwd" }, managed.ctx);
    await options.withSession(managed.ctx);
    return { changed: true, cwd };
  };
  await request(source, { effort: "demo", destination: "managed" });
  await continueRequest(source);
  assert.ok(managed);
  assert.equal(managed.ctx.cwd, "/managed");

  managed.overrides.set("resolve", [envelope("ready", { cwd: "/receiving", role: "receiving" })]);
  let receiving: Harness | undefined;
  managed.ctx.changeCwd = async (cwd: string, options: any) => {
    await managed!.hooks.get("session_shutdown")({ reason: "cwd", targetCwd: cwd }, managed!.ctx);
    receiving = makeHarness({ shared, cwd, events: false, uuid: makeUUIDSource(200) });
    await receiving.hooks.get("session_start")({ reason: "cwd" }, receiving.ctx);
    await options.withSession(receiving.ctx);
    return { changed: true, cwd };
  };
  await request(managed, { effort: "demo", destination: "receiving" });
  await continueRequest(managed);
  assert.ok(receiving);
  assert.equal(receiving.ctx.cwd, "/receiving");
  await receiving.hooks.get("turn_end")({}, receiving.ctx);
  await request(receiving, { detach: true });
  await continueRequest(receiving);

  const calls = [...source.calls, ...managed.calls, ...receiving.calls];
  assert.deepEqual(calls.map((argv) => argv[2]), ["resolve", "attach", "resolve", "checkout", "heartbeat", "detach"]);
  assert.equal(flag(calls[1]!, "--cwd"), "/managed");
  assert.equal(flag(calls[1]!, "--role"), "managed");
  assert.equal(flag(calls[3]!, "--cwd"), "/receiving");
  assert.equal(flag(calls[3]!, "--role"), "receiving");
  for (const h of [source, managed, receiving]) {
    assert.equal(h.events.length, 0);
    assert.equal(h.listeners.size, 0);
  }
});

test("receiving attach, same-effort checkout, and different-effort switch use resolved facts and ordered owner operations", async () => {
  const h = makeHarness();
  await request(h, { effort: "demo", destination: "receiving", receivingCheckout: "/repo" });
  await continueRequest(h);
  await request(h, { effort: "demo", destination: "managed" });
  await continueRequest(h);
  await request(h, { effort: "other", destination: "managed" });
  await continueRequest(h);
  assert.deepEqual(h.calls.map((argv) => argv[2]), ["resolve", "attach", "resolve", "checkout", "resolve", "detach", "attach"]);
  assert.equal(flag(h.calls[3]!, "--role"), "managed");
  assert.equal(h.calls[5]?.[3], "demo");
  assert.equal(h.calls[6]?.[3], "other");
});

test("different-effort same-CWD attach refusal clears the detached source association", async () => {
  for (const [reply, notice] of [
    [envelope("not-owner", { slug: "other" }), /prior effort detached: not-owner; changedCwd=false changedActivity=true changedMemory=false/],
    [envelope("attached", { slug: "other", owner: PRIOR }), /prior effort detached: attached;.*Retry from the current checkout/],
  ] as const) {
    const h = makeHarness();
    await attachSameCwd(h);
    h.overrides.set("attach", [reply]);
    await request(h, { effort: "other", destination: "managed" });
    await continueRequest(h);
    assert.deepEqual(h.calls.map((argv) => argv[2]), ["resolve", "attach", "resolve", "detach", "attach", "detach"]);
    assert.equal(emitted(h, "remote-pi:metadata:set").at(-1).value, null);
    assert.match(h.notices.join(" "), notice);
    const calls = h.calls.length;
    await h.hooks.get("turn_end")({}, h.ctx);
    assert.equal(h.calls.length, calls);
  }
});

test("changed CWD transfers through the destination instance and matching cwd shutdown skips source detach", async () => {
  const shared: EffortTransferCoordinator = {};
  const source = makeHarness({ shared, cwd: "/source" });
  let destinationHarness: Harness | undefined;
  source.ctx.changeCwd = async (cwd: string, options: any) => {
    await source.hooks.get("session_shutdown")({ reason: "cwd", targetCwd: cwd }, source.ctx);
    destinationHarness = makeHarness({ shared, cwd, uuid: makeUUIDSource(100) });
    await destinationHarness.hooks.get("session_start")({ reason: "cwd" }, destinationHarness.ctx);
    await options.withSession(destinationHarness.ctx);
    return { changed: true, cwd };
  };
  source.overrides.set("resolve", [envelope("ready", { cwd: "/managed" })]);
  await request(source, { effort: "demo", destination: "managed" });
  await continueRequest(source);
  assert.ok(destinationHarness);
  assert.equal(source.calls.filter((argv) => argv[2] === "detach").length, 0);
  assert.equal(destinationHarness!.calls.some((argv) => argv[2] === "attach"), true);
  assert.equal(emitted(destinationHarness!, "remote-pi:metadata:set").at(-1).value.effort.slug, "demo");
});

test("pre-teardown failure retains the old association and destination receiver absence changes only CWD", async () => {
  const retained = makeHarness();
  await attachSameCwd(retained);
  retained.ctx.changeCwd = async () => { throw new Error("rebind failed"); };
  retained.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  await request(retained, { effort: "demo", destination: "managed" });
  await continueRequest(retained);
  await retained.hooks.get("turn_end")({}, retained.ctx);
  assert.equal(retained.calls.at(-1)?.[2], "heartbeat");
  assert.match(retained.notices.join(" "), /prior association is unchanged/);

  const shared: EffortTransferCoordinator = {};
  const absent = makeHarness({ shared, cwd: "/source" });
  absent.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  absent.ctx.changeCwd = async (cwd: string, options: any) => {
    await options.withSession({ ...absent.ctx, cwd });
    return { changed: true, cwd };
  };
  await request(absent, { effort: "demo", destination: "managed" });
  await continueRequest(absent);
  assert.match(absent.notices.join(" "), /receiver is unavailable/);
  assert.equal(absent.calls.filter((argv) => argv[2] === "attach").length, 0);

  const noCallback = makeHarness({ cwd: "/source" });
  noCallback.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  noCallback.ctx.changeCwd = async (cwd: string) => ({ changed: true, cwd });
  await request(noCallback, { effort: "demo", destination: "managed" });
  await continueRequest(noCallback);
  assert.match(noCallback.notices.join(" "), /without running the replacement-session callback/);
});

test("post-rebind refusal and transfer timeout clear publication and owner-detach recovery", async () => {
  for (const mode of ["refusal", "timeout"] as const) {
    const shared: EffortTransferCoordinator = {};
    const source = makeHarness({ shared, cwd: "/source", wait: mode === "timeout" ? async () => undefined : () => new Promise(() => undefined) });
    source.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
    let destinationHarness: Harness | undefined;
    source.ctx.changeCwd = async (cwd: string, options: any) => {
      destinationHarness = makeHarness({
        shared,
        cwd,
        uuid: makeUUIDSource(100),
        overrides: mode === "refusal"
          ? { attach: [envelope("not-owner", { slug: "demo", cwd })] }
          : { attach: [() => new Promise<ActivityReply>(() => undefined)] },
      });
      await options.withSession(destinationHarness.ctx);
      return { changed: true, cwd };
    };
    await request(source, { effort: "demo", destination: "managed" });
    await continueRequest(source);
    assert.ok(destinationHarness);
    assert.equal(destinationHarness!.calls.some((argv) => argv[2] === "detach"), true);
    assert.equal(emitted(destinationHarness!, "remote-pi:metadata:set").at(-1).value, null);
    assert.match(destinationHarness!.notices.join(" "), mode === "timeout" ? /timed out/ : /not-owner/);
  }

  const shared: EffortTransferCoordinator = {};
  const source = makeHarness({ shared, cwd: "/source", wait: async () => undefined });
  source.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  let destinationHarness: Harness | undefined;
  source.ctx.changeCwd = async (cwd: string, options: any) => {
    destinationHarness = makeHarness({ shared, cwd, uuid: makeUUIDSource(200), overrides: { attach: [() => new Promise<ActivityReply>(() => undefined)], detach: [new Error("cleanup failed")] } });
    await options.withSession(destinationHarness.ctx);
    return { changed: true, cwd };
  };
  await request(source, { effort: "demo", destination: "managed" });
  await continueRequest(source);
  assert.match(destinationHarness!.notices.join(" "), /could not confirm activity cleanup/);
});

test("resolve and commit refusals remain structured, and detach refusal or mechanism failure retains association", async () => {
  const resolveRefused = makeHarness({ overrides: { resolve: [envelope("missing")] } });
  await request(resolveRefused, { effort: "demo", destination: "managed" });
  await continueRequest(resolveRefused);
  assert.match(resolveRefused.notices.join(" "), /destination refused: missing/);
  const resolveFailed = makeHarness({ overrides: { resolve: [new Error("git unavailable")] } });
  await request(resolveFailed, { effort: "demo", destination: "managed" });
  await continueRequest(resolveFailed);
  assert.match(resolveFailed.notices.join(" "), /resolution failed/);

  for (const reply of [envelope("unsafe-resident", { cause: "disk" }), new Error("detach I/O")] as const) {
    const h = makeHarness();
    await attachSameCwd(h);
    h.overrides.set("detach", [reply]);
    await request(h, { detach: true });
    await continueRequest(h);
    await h.hooks.get("turn_end")({}, h.ctx);
    assert.equal(h.calls.at(-1)?.[2], "heartbeat");
  }
});

test("heartbeat ownership loss clears while damaged metadata and mechanism failures degrade advisory-only", async () => {
  for (const reply of [envelope("not-owner"), envelope("missing")]) {
    const h = makeHarness();
    await attachSameCwd(h);
    h.overrides.set("heartbeat", [reply]);
    await h.hooks.get("turn_end")({}, h.ctx);
    assert.equal(emitted(h, "remote-pi:metadata:set").at(-1).value, null);
  }
  for (const reply of [envelope("invalid-memory"), envelope("unsafe-resident", { cause: "disk" }), new Error("heartbeat failed")]) {
    const h = makeHarness();
    await attachSameCwd(h);
    h.overrides.set("heartbeat", [reply]);
    await h.hooks.get("turn_end")({}, h.ctx);
    const metadata = emitted(h, "remote-pi:metadata:set").at(-1).value;
    assert.equal(metadata.effort.slug, "demo");
    assert.equal(metadata.memory, null);
    assert.match(h.notices.join(" "), /heartbeat/);
  }
});

test("fresh and stale takeover warn identically without changing permission", async () => {
  for (const priorHeartbeat of ["2026-08-02T02:30:00Z", "2026-08-01T00:00:00Z"]) {
    const h = makeHarness({ now: () => new Date("2026-08-02T03:00:00Z"), overrides: { attach: [(argv) => envelope("taken-over", { owner: flag(argv, "--owner"), priorHeartbeat })] } });
    await attachSameCwd(h);
    assert.equal(emitted(h, "remote-pi:metadata:set").at(-1).value.effort.slug, "demo");
    assert.match(h.notices.join(" "), /Took over (fresh|stale).*Presence is advisory, not a lock/);
  }
});

test("shutdown reasons detach, matching transfer shutdown skips, and restart begins detached", async () => {
  for (const reason of ["quit", "reload", "new", "resume", "fork"] as const) {
    const h = makeHarness();
    await attachSameCwd(h);
    await h.hooks.get("session_shutdown")({ reason }, h.ctx);
    assert.equal(h.calls.at(-1)?.[2], "detach");
  }
  const restarted = makeHarness();
  await restarted.hooks.get("session_start")({ reason: "startup" }, restarted.ctx);
  assert.equal(emitted(restarted, "remote-pi:metadata:set").at(-1).value, null);
});

test("Remote Pi negotiation is bounded, metadata-only fallback is silent, and publication failures stay advisory", async () => {
  const fallback = makeHarness();
  await attachSameCwd(fallback);
  assert.equal(emitted(fallback, "remote-pi:metadata:set").at(-1).value.effort.slug, "demo");
  assert.equal(emitted(fallback, "remote-pi:name-override:set").length, 0);
  fallback.listeners.get("remote-pi:capabilities")({ metadata: { version: 2 }, nameOverride: { version: 1, namespaces: ["awf"] } }, fallback.ctx);
  fallback.listeners.get("remote-pi:capabilities")({ metadata: { version: 1 }, nameOverride: { version: 2, namespaces: ["awf"] } }, fallback.ctx);
  fallback.listeners.get("remote-pi:capabilities")({ metadata: { version: 1 }, nameOverride: { version: 1, namespaces: "awf" } }, fallback.ctx);
  assert.equal(emitted(fallback, "remote-pi:name-override:set").length, 0);
  fallback.listeners.get("remote-pi:capabilities")({ metadata: { version: 1 }, nameOverride: { version: 1, namespaces: ["awf"] } }, fallback.ctx);
  assert.equal(emitted(fallback, "remote-pi:name-override:set").at(-1).value, "demo");

  const broken = makeHarness({ emitThrows: new Set(["remote-pi:metadata:set", "remote-pi:name-override:set"]) });
  broken.listeners.get("remote-pi:capabilities")({ metadata: { version: 1 }, nameOverride: { version: 1, namespaces: ["awf"] } }, broken.ctx);
  await attachSameCwd(broken);
  broken.listeners.get("remote-pi:capabilities")({ metadata: { version: 1 }, nameOverride: { version: 1, namespaces: ["awf"] } });
  await request(broken, { detach: true });
  await continueRequest(broken);
  assert.match(broken.notices.join(" "), /publication is unavailable|cleanup is unavailable/);
});

test("defensive transfer branches preserve ownership and report exact recovery posture", async () => {
  for (const detachedCondition of ["missing", "not-owner"] as const) {
    const h = makeHarness();
    await attachSameCwd(h);
    h.overrides.set("detach", [envelope(detachedCondition)]);
    await request(h, { effort: "other", destination: "managed" });
    await continueRequest(h);
    assert.equal(h.calls.at(-1)?.[2], "attach");
  }

  const detachRefused = makeHarness();
  await attachSameCwd(detachRefused);
  detachRefused.overrides.set("detach", [envelope("unsafe-resident", { cause: "disk" })]);
  await request(detachRefused, { effort: "other", destination: "managed" });
  await continueRequest(detachRefused);
  assert.match(detachRefused.notices.join(" "), /prior association is unchanged/);

  const wrongOwner = makeHarness({ overrides: { attach: [envelope("attached", { owner: PRIOR })] } });
  await request(wrongOwner, { effort: "demo", destination: "managed" });
  await continueRequest(wrongOwner);
  assert.match(wrongOwner.notices.join(" "), /prior association is unchanged/);

  const shared: EffortTransferCoordinator = {};
  const wrongContext = makeHarness({ shared, cwd: "/source", wait: () => new Promise(() => undefined) });
  wrongContext.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  let destinationHarness: Harness | undefined;
  wrongContext.ctx.changeCwd = async (cwd: string, options: any) => {
    destinationHarness = makeHarness({ shared, cwd: "/wrong", uuid: makeUUIDSource(300) });
    await options.withSession(destinationHarness.ctx);
    return { changed: true, cwd };
  };
  await request(wrongContext, { effort: "demo", destination: "managed" });
  await continueRequest(wrongContext);
  assert.match(destinationHarness!.notices.join(" "), /replacement context does not match/);

  const replacedReceiverShared: EffortTransferCoordinator = {};
  const replacedReceiver = makeHarness({ shared: replacedReceiverShared });
  replacedReceiver.ctx.changeCwd = async (cwd: string) => {
    makeHarness({ shared: replacedReceiverShared, cwd, uuid: makeUUIDSource(400) });
    return { changed: false, cwd };
  };
  await request(replacedReceiver, { effort: "demo", destination: "managed" });
  await continueRequest(replacedReceiver);
  assert.match(replacedReceiver.notices.join(" "), /receiver changed/);

  const detachWithoutOutcome = makeHarness();
  await attachSameCwd(detachWithoutOutcome);
  detachWithoutOutcome.overrides.set("detach", [envelope("heartbeat")]);
  await request(detachWithoutOutcome, { detach: true });
  await continueRequest(detachWithoutOutcome);
  assert.match(detachWithoutOutcome.notices.join(" "), /Retry detach/);

  for (const failure of ["reply-without-outcome", "string-error"] as const) {
    const transferShared: EffortTransferCoordinator = {};
    const transfer = makeHarness({
      shared: transferShared,
      cwd: "/source",
      wait: failure === "string-error" ? async () => { throw "string timeout"; } : () => new Promise(() => undefined),
    });
    transfer.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
    let transferred: Harness | undefined;
    transfer.ctx.changeCwd = async (cwd: string, options: any) => {
      transferred = makeHarness({ shared: transferShared, cwd, uuid: makeUUIDSource(500), overrides: failure === "reply-without-outcome" ? { attach: [envelope("heartbeat")] } : { attach: [() => new Promise<ActivityReply>(() => undefined)] } });
      await options.withSession(transferred.ctx);
      return { changed: true, cwd };
    };
    await request(transfer, { effort: "demo", destination: "managed" });
    await continueRequest(transfer);
    assert.match(transferred!.notices.join(" "), failure === "reply-without-outcome" ? /Retry from the current checkout/ : /string timeout/);
  }

  const stringRebind = makeHarness();
  stringRebind.overrides.set("resolve", [envelope("ready", { cwd: "/other" })]);
  stringRebind.ctx.changeCwd = async () => { throw "string rebind"; };
  await request(stringRebind, { effort: "demo", destination: "managed" });
  await continueRequest(stringRebind);
  assert.match(stringRebind.notices.join(" "), /string rebind/);

  const defaultNow = makeHarness({ overrides: { attach: [(argv) => envelope("taken-over", { owner: flag(argv, "--owner"), priorHeartbeat: T0 })] } });
  await attachSameCwd(defaultNow);
  assert.match(defaultNow.notices.join(" "), /Took over/);
});

test("transition queue serializes heartbeat races", async () => {
  let release: (() => void) | undefined;
  let active = 0;
  let maximum = 0;
  const h = makeHarness({ overrides: { heartbeat: [async (argv) => { active++; maximum = Math.max(maximum, active); await new Promise<void>((resolve) => { release = resolve; }); active--; return envelope("heartbeat", { owner: flag(argv, "--owner") }); }, (argv) => envelope("heartbeat", { owner: flag(argv, "--owner") })] } });
  await attachSameCwd(h);
  const first = h.hooks.get("turn_end")({}, h.ctx);
  const second = h.hooks.get("turn_end")({}, h.ctx);
  await new Promise((resolve) => setImmediate(resolve));
  assert.equal(active, 1);
  release!();
  await Promise.all([first, second]);
  assert.equal(maximum, 1);
});
