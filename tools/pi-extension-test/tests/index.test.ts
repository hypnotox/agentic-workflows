import assert from "node:assert/strict";
import test from "node:test";
import { Value } from "typebox/value";
import {
  MAX_EXPLORATION_CONCURRENCY,
  registerSubagentTools,
  type ExtensionDependencies,
} from "../../../.pi/extensions/awf-subagents/index.ts";
import type { RunRequest, RunResult } from "../../../.pi/extensions/awf-subagents/runner.ts";
import {
  MODEL_REFERENCE_SCHEMA,
  PREFERENCE_FIELDS,
  RECOMMENDED_PRESET,
  ROUTING_CARD_OVERFLOW_WARNING,
  buildRoutingCard,
  loadPreferenceState,
  effectivePreferenceState,
  emptyPreferenceSource,
  invalidText,
  parseExactModelReference,
  registryFailures,
  resolveChildModel,
  routingPreview,
} from "../../../.pi/extensions/awf-subagents/model-routing.ts";

const GLOBAL = "/agent/awf-subagents.json";
const PROJECT = "/repo/.pi/awf-subagents.local.json";
const GITIGNORE = "/repo/.gitignore";
const GLOBAL_LABEL = `User-global (${GLOBAL})`;
const PROJECT_LABEL = `Project-local (${PROJECT})`;
const PRESET = "Apply recommended GPT-5.6 preset";
const MANUAL = "Configure each slot";
const UNSET = "leave unset (use fallback chain)";
test("routing module builds exact complete, fallback, mixed, maximum, and defensive cards", () => {
  const source = (scope: "global" | "project") => emptyPreferenceSource(scope, `/${scope}`);
  const complete: any = { global: source("global"), project: source("project"), effective: {}, missing: [], invalid: [], blocked: false, errors: [] };
  for (const field of PREFERENCE_FIELDS) complete.effective[field] = { reference: `example/${field}`, scope: "global" };
  assert.equal(buildRoutingCard(complete), `[awf subagent routing]
default: example/default
roles: grounding=example/grounding; exploration=example/exploration; review=example/review; implementation=example/implementation
tiers: small=example/small; standard=example/standard; large=example/large
missing: none
invalid: none
selection: omit model for the role default; otherwise override deliberately with the selected tier's exact provider/model-id.`);

  const fallback = { ...complete, missing: ["grounding", "small"], effective: { ...complete.effective, grounding: undefined, small: undefined } };
  assert.equal(buildRoutingCard(fallback), `[awf subagent routing]
default: example/default
roles: grounding=example/default; exploration=example/exploration; review=example/review; implementation=example/implementation
tiers: small=missing; standard=example/standard; large=example/large
missing: grounding, small
invalid: none
selection: omit model for the role default; otherwise override deliberately with the selected tier's exact provider/model-id.`);

  const mixed = { ...fallback, invalid: [
    { kind: "source", scope: "global", reason: "malformed-json" },
    { kind: "source", scope: "project", reason: "unknown-key" },
    { kind: "field", scope: "project", field: "small", reason: "unavailable" },
  ], blocked: true };
  assert.match(buildRoutingCard(mixed), /missing: grounding, small\ninvalid: global:source:malformed-json; project:source:unknown-key; project:small:unavailable\nrepair: Run \/awf-subagent-models/);

  const boundaryReference = `p/${"x".repeat(254)}`;
  const maximum = { ...complete, effective: Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, { reference: boundaryReference, scope: "global" }])) };
  const maximumCard = buildRoutingCard(maximum);
  assert.ok(Buffer.byteLength(maximumCard, "utf8") <= 4096);
  assert.equal(maximumCard.includes("state: unavailable"), false);

  const huge: any = { ...complete, effective: {} };
  for (const field of PREFERENCE_FIELDS) huge.effective[field] = { reference: `x/${"z".repeat(1000)}`, scope: "global" };
  assert.equal(buildRoutingCard(huge), "[awf subagent routing]\nstate: unavailable (routing card exceeded 4096 UTF-8 bytes)\nrepair: Run /awf-subagent-models and retry; implicit routing remains strict.");
  assert.equal(ROUTING_CARD_OVERFLOW_WARNING, "awf subagent routing card exceeded 4096 UTF-8 bytes; injected a failure card. Run /awf-subagent-models and retry.");

  assert.deepEqual(parseExactModelReference("p/model"), { provider: "p", id: "model" });
  // "/a/b" and "//a" pin the 0x2F hole in the provider class: it is the only thing still
  // rejecting an empty provider now that the slash-position checks are gone.
  for (const value of [undefined, "default", "auto", "inherit parent", "ab", "/x", "ab/", "x/y z", "/a/b", "//a"]) assert.deepEqual(parseExactModelReference(value), { reason: "malformed" });
  assert.deepEqual(parseExactModelReference(`p/${"x".repeat(256)}`), { reason: "overlong" });
});

const baseResult: RunResult = {
  output: "done", stderr: "", events: [], omittedEvents: 0, failed: false, modelChanged: false,
  usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 1 },
};

type Answer = string | boolean | undefined | ((h: ReturnType<typeof harness>) => string | boolean | undefined);

function model(provider: string, id: string) {
  return {
    provider, id, name: id, contextWindow: 1000, maxTokens: 100, reasoning: true, input: ["text"],
    cost: { input: 1, output: 2 },
  };
}

function harness(options: {
  files?: Record<string, string | Error>;
  answers?: Answer[];
  git?: Array<{ code: number; stdout?: string; stderr?: string }> | ((command: string, args: string[], options: { cwd: string }, index: number) => { code: number; stdout?: string; stderr?: string });
  realpath?: (path: string) => Promise<string>;
  run?: (request: RunRequest) => Promise<RunResult>;
  sessionManager?: object;
  activeTools?: string[];
  missingGetActiveTools?: boolean;
} = {}) {
  const files = options.files ?? {};
  const tools = new Map<string, any>();
  const commands = new Map<string, any>();
  const hooks = new Map<string, any>();
  const requests: RunRequest[] = [];
  const notices: any[][] = [];
  const writes: Array<{ op: string; path: string; data?: string; mode?: number; to?: string }> = [];
  const temporary = new Map<string, string>();
  const answers = [...(options.answers ?? [])];
  const prompts: any[] = [];
  const git = Array.isArray(options.git) ? [...options.git] : [];
  const gitCalls: Array<{ command: string; args: string[]; cwd: string }> = [];
  const models = new Map<string, any>();
  const available = new Set<string>();
  const addModel = (reference: string, authenticated = true, isAvailable = true) => {
    const slash = reference.indexOf("/");
    const value = { ...model(reference.slice(0, slash), reference.slice(slash + 1)), authenticated };
    models.set(reference, value);
    if (isAvailable) available.add(reference);
  };
  addModel("test/parent");
  addModel("p/model");
  const pop = () => {
    const answer = answers.shift();
    return typeof answer === "function" ? answer(h) : answer;
  };
  let leaf: any = { type: "message", message: { role: "assistant", content: [] } };
  const pi: any = {
    on: (name: string, handler: any) => hooks.set(name, handler),
    registerTool: (tool: any) => tools.set(tool.name, tool),
    registerCommand: (name: string, command: any) => commands.set(name, command),
    getThinkingLevel: () => "high",
    getActiveTools: () => options.activeTools ?? [],
    events: { emit() {} },
    exec: async (command: string, args: string[], execOptions: { cwd: string }) => {
      const index = gitCalls.length;
      gitCalls.push({ command, args, cwd: execOptions.cwd });
      return typeof options.git === "function"
        ? options.git(command, args, execOptions, index)
        : git.shift() ?? { code: 1, stdout: "", stderr: "" };
    },
  };
  if (options.missingGetActiveTools) delete pi.getActiveTools;
  const deps: ExtensionDependencies = {
    readFile: async (path: string) => {
      if (path.startsWith("/repo/.pi/agents/")) return "---\nname: reviewer\ndescription: test\n---\nReview carefully.";
      const value = files[path];
      if (value instanceof Error) throw value;
      if (value === undefined) throw Object.assign(new Error("missing"), { code: "ENOENT" });
      return value;
    },
    writeFile: async (path: string, data: string, writeOptions: { mode: number }) => {
      writes.push({ op: "write", path, data, mode: writeOptions.mode });
      temporary.set(path, data);
      if (path === GITIGNORE) files[path] = data;
    },
    mkdir: async (path: string) => { writes.push({ op: "mkdir", path }); },
    rename: async (from: string, to: string) => {
      writes.push({ op: "rename", path: from, to });
      files[to] = temporary.get(from)!;
      temporary.delete(from);
    },
    unlink: async (path: string) => { writes.push({ op: "unlink", path }); temporary.delete(path); },
    realpath: options.realpath ?? (async (path: string) => path),
    runner: { run: async (request) => { requests.push(request); return options.run ? options.run(request) : baseResult; } },
    packageVersion: "0.81.1",
    extensionFile: "/repo/.pi/extensions/awf-subagents/index.ts",
    agentDir: "/agent",
    configDirName: ".pi",
    observationId: () => "write-id",
  };
  const sessionManager = options.sessionManager ?? { getLeafEntry: () => leaf };
  const ctx: any = {
    mode: "tui",
    model: { provider: "test", id: "parent" },
    sessionManager,
    modelRegistry: {
      find: (provider: string, id: string) => models.get(`${provider}/${id}`),
      hasConfiguredAuth: (entry: any) => entry.authenticated !== false,
      getAvailable: () => [...available].map((reference) => models.get(reference)),
      getAll: () => [...models.values()],
    },
    ui: {
      notify: (...args: any[]) => notices.push(args),
      select: async (title: string, choices: string[]) => { prompts.push({ kind: "select", title, choices }); return pop(); },
      confirm: async (title: string, message: string) => { prompts.push({ kind: "confirm", title, message }); return pop(); },
    },
  };
  registerSubagentTools(pi, deps);
  const h = {
    files, tools, commands, hooks, requests, notices, writes, prompts, models, available, gitCalls, deps, ctx, addModel,
    runWizard: () => commands.get("awf-subagent-models").handler("", ctx),
    setLeaf: (value: any) => { leaf = value; },
  };
  return h;
}

async function call(h: ReturnType<typeof harness>, name: string, params: any, signal?: AbortSignal) {
  const updates: any[] = [];
  const value = await h.tools.get(name).execute("id", params, signal, (update: any) => updates.push(update), h.ctx);
  return { value, updates };
}

test("routing policy covers sorting, preview, registry, store, and inherited seams", async () => {
  const global = emptyPreferenceSource("global", "/global");
  const project = emptyPreferenceSource("project", "/project");
  global.invalid = [
    { kind: "field", scope: "global", field: "small", reason: "unavailable" },
    { kind: "source", scope: "global", reason: "unknown-key" },
    { kind: "source", scope: "global", reason: "malformed-json" },
    { kind: "field", scope: "global", field: "small", reason: "unavailable" },
  ];
  const sorted = effectivePreferenceState(global, project, []);
  assert.deepEqual(sorted.errors, [
    "global:source:malformed-json", "global:source:unknown-key",
    "global:small:unavailable", "global:small:unavailable",
  ]);
  assert.match(routingPreview(project, global, sorted).join("\n"), /Invalid: global:source:malformed-json/);
  assert.equal(invalidText({ kind: "field", scope: "project", field: "large", reason: "malformed" }), "project:large:malformed");

  const preview = routingPreview(emptyPreferenceSource("project", "/p"), emptyPreferenceSource("global", "/g"));
  assert.match(preview.join("\n"), /grounding: parent \(inherited\)/);
  assert.match(preview.join("\n"), /small: unset/);
  assert.match(preview.join("\n"), /Missing: default, grounding/);
  assert.match(preview.join("\n"), /Invalid: none/);
  const configuredGlobal = emptyPreferenceSource("global", "/g");
  const configuredProject = emptyPreferenceSource("project", "/p");
  configuredGlobal.values = { default: "g/default", small: "g/small" };
  configuredProject.values = { grounding: "p/grounding", small: "p/small" };
  const configuredPreview = routingPreview(configuredProject, configuredGlobal);
  assert.match(configuredPreview.join("\n"), /grounding: p\/grounding \(project-role\)/);
  assert.match(configuredPreview.join("\n"), /exploration: g\/default \(global-default\)/);
  assert.match(configuredPreview.join("\n"), /small: p\/small/);

  const h = harness();
  const malformed = emptyPreferenceSource("global", "/g");
  malformed.values.default = "bad";
  assert.deepEqual(registryFailures(h.ctx.modelRegistry, [malformed]), [{ kind: "field", scope: "global", field: "default", reason: "malformed" }]);
  const state = effectivePreferenceState(emptyPreferenceSource("global", "/g"), emptyPreferenceSource("project", "/p"), []);
  assert.deepEqual(resolveChildModel(h.ctx.modelRegistry, h.ctx.model, "grounding", undefined, state), {
    model: { provider: "test", id: "parent" }, requested: undefined, source: "inherited",
  });
  assert.throws(() => resolveChildModel(h.ctx.modelRegistry, undefined, "grounding", undefined, state), /without an active parent model/);
  assert.throws(() => resolveChildModel(h.ctx.modelRegistry, h.ctx.model, "grounding", "ghost/model", state), /unregistered/);
  h.addModel("locked/model", false);
  assert.throws(() => resolveChildModel(h.ctx.modelRegistry, h.ctx.model, "grounding", "locked/model", state), /unauthenticated/);
  h.addModel("gone/model", true, false);
  assert.throws(() => resolveChildModel(h.ctx.modelRegistry, h.ctx.model, "grounding", "gone/model", state), /unavailable/);

  const primitiveFailure = await loadPreferenceState({ ...h.deps, readFile: async () => { throw "primitive"; } }, h.ctx.modelRegistry);
  assert.deepEqual(primitiveFailure.errors, ["global:source:read-error", "project:source:read-error"]);
});

test("before_agent_start injects current routing once with selectedTools precedence and active-tool fallback", async () => {
  const values = complete();
  const files = { [GLOBAL]: JSON.stringify(values) };
  const h = harness({ files, activeTools: ["subagent_grounding"] });
  registerObject(h, values);
  const hook = h.hooks.get("before_agent_start");

  const selectedInactive = await hook({ systemPrompt: "base", systemPromptOptions: { selectedTools: ["read"] } }, h.ctx);
  assert.equal(selectedInactive, undefined);
  const fallback = await hook({ systemPrompt: "base", systemPromptOptions: {} }, h.ctx);
  assert.deepEqual(Object.keys(fallback), ["systemPrompt"]);
  assert.equal((fallback.systemPrompt.match(/\[awf subagent routing\]/g) ?? []).length, 1);
  assert.match(fallback.systemPrompt, /grounding=p\/grounding/);

  files[GLOBAL] = JSON.stringify({ ...values, grounding: "new/grounding" });
  h.addModel("new/grounding");
  const selectedActive = await hook({ systemPrompt: "chained", systemPromptOptions: { selectedTools: ["subagent_review"] } }, h.ctx);
  assert.equal((selectedActive.systemPrompt.match(/\[awf subagent routing\]/g) ?? []).length, 1);
  assert.match(selectedActive.systemPrompt, /grounding=new\/grounding/);
  assert.equal(selectedActive.systemPrompt.startsWith("chained\n\n"), true);
});

test("routing hook skips notices without active awf tools and notices incomplete state once when active", async () => {
  const h = harness({ activeTools: ["read"] });
  const hook = h.hooks.get("before_agent_start");
  assert.equal(await hook({ systemPrompt: "base", systemPromptOptions: {} }, h.ctx), undefined);
  assert.equal(h.notices.length, 0);
  const activeEvent = { systemPrompt: "base", systemPromptOptions: { selectedTools: ["subagent_grounding"] } };
  await hook(activeEvent, h.ctx);
  await hook(activeEvent, h.ctx);
  assert.equal(h.notices.length, 1);
  assert.match(h.notices[0][0], /incomplete/);
});

test("subagent minimum-runtime guard rejects missing getActiveTools without registration", async () => {
  delete (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")];
  const h = harness({ missingGetActiveTools: true });
  assert.equal(h.tools.size, 0);
  assert.equal(h.commands.size, 0);
  assert.deepEqual([...h.hooks.keys()], ["session_start"]);
  await h.hooks.get("session_start")({}, h.ctx);
  await h.hooks.get("session_start")({}, h.ctx);
  assert.equal(h.notices.length, 1);
  assert.match(h.notices[0][0], /Missing runtime APIs: getActiveTools/);
});

function registerPreset(h: ReturnType<typeof harness>) {
  for (const reference of new Set(Object.values(RECOMMENDED_PRESET))) h.addModel(reference);
}

function complete(prefix = "p") {
  return Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, `${prefix}/${field}`]));
}

function registerObject(h: ReturnType<typeof harness>, values: Record<string, string>) {
  for (const reference of Object.values(values)) h.addModel(reference);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => { resolve = done; });
  return { promise, resolve };
}

test("preference states are complete only after explicit project-over-global field merging", async () => {
  const cases: Array<[string, unknown, unknown, string[], boolean]> = [
    ["absent", undefined, undefined, [...PREFERENCE_FIELDS], false],
    ["partial", { default: "g/default" }, undefined, PREFERENCE_FIELDS.filter((x) => x !== "default"), false],
    ["complete", complete("g"), undefined, [], false],
  ];
  for (const [label, global, project, missing, blocked] of cases) {
    const files: Record<string, string> = {};
    if (global !== undefined) files[GLOBAL] = JSON.stringify(global);
    if (project !== undefined) files[PROJECT] = JSON.stringify(project);
    const h = harness({ files });
    registerObject(h, { ...(global as any), ...(project as any) });
    const state = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
    assert.deepEqual(state.missing, missing, label);
    assert.equal(state.blocked, blocked, label);
  }

  const global = complete("g");
  const project = { exploration: "p/exploration", small: "p/small" };
  const h = harness({ files: { [GLOBAL]: JSON.stringify(global), [PROJECT]: JSON.stringify(project) } });
  registerObject(h, { ...global, ...project });
  const state = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  assert.deepEqual(state.effective.exploration, { reference: "p/exploration", scope: "project" });
  assert.deepEqual(state.effective.small, { reference: "p/small", scope: "project" });
  assert.equal(state.missing.length, 0);
});

test("shared default routes but does not satisfy explicit role completeness", async () => {
  const h = harness({ files: { [GLOBAL]: JSON.stringify({ default: "p/model" }) } });
  const result = await call(h, "subagent_grounding", { task: "ground" });
  assert.equal(result.value.details.modelSource, "global-default");
  const state = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  assert.ok(state.missing.includes("grounding"));
});

test("preference state is derived per call from its parameters rather than a mutable holder", async () => {
  const values = complete("g");
  const h = harness({ files: { [GLOBAL]: JSON.stringify(values) } });
  registerObject(h, values);

  const first = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  const second = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  assert.notStrictEqual(first, second, "each derivation returns its own value");
  assert.deepEqual(second, first, "unchanged sources derive equal state");
  assert.equal(first.blocked, false);

  // The registry is a parameter, so the next derivation reflects a registry
  // change with no reload or revalidate step on any longer-lived value.
  h.available.delete(values.grounding);
  const afterRegistryChange = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  assert.equal(afterRegistryChange.blocked, true);
  assert.deepEqual(afterRegistryChange.errors, ["global:grounding:unavailable"]);
  assert.equal(first.blocked, false, "an earlier derived value is unaffected by a later derivation");
});

test("bounded source and field failures are deterministic, block implicit routing, and do not leak raw input", async () => {
  const cases: Array<[string, string | Error, string]> = [
    ["malformed JSON", "{raw-secret", "global:source:malformed-json"],
    ["non-object JSON", "[]", "global:source:non-object"],
    ["unknown key", JSON.stringify({ secretUnknownKey: "p/model", default: "p/model" }), "global:source:unknown-key"],
    ["read error", new Error("raw disk secret"), "global:source:read-error"],
    ["malformed field", JSON.stringify({ default: "noslash", grounding: "p/model" }), "global:default:malformed"],
    ["overlong field", JSON.stringify({ default: `p/${"x".repeat(255)}` }), "global:default:overlong"],
    ["unregistered", JSON.stringify({ default: "ghost/model" }), "global:default:unregistered"],
  ];
  for (const [label, source, expected] of cases) {
    const h = harness({ files: { [GLOBAL]: source } });
    const state = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
    assert.equal(state.errors[0], expected, label);
    const serialized = JSON.stringify(state);
    for (const raw of ["raw-secret", "secretUnknownKey", "raw disk secret"]) assert.equal(serialized.includes(raw), false, `${label} leaked ${raw}`);
    await assert.rejects(call(h, "subagent_grounding", { task: "x" }), /implicit routing is blocked/);
    const explicit = await call(h, "subagent_grounding", { task: "x", model: "p/model" });
    assert.equal(explicit.value.details.modelSource, "requested");
  }

  const h = harness({ files: { [GLOBAL]: JSON.stringify({ default: "locked/model", grounding: "gone/model" }), [PROJECT]: JSON.stringify({ small: "later/model", large: "bad" }) } });
  h.addModel("locked/model", false, true);
  h.addModel("gone/model", true, false);
  h.addModel("later/model", true, false);
  const state = await loadPreferenceState(h.deps, h.ctx.modelRegistry);
  assert.deepEqual(state.errors, [
    "global:default:unauthenticated", "global:grounding:unavailable", "project:small:unavailable", "project:large:malformed",
  ]);
});

test("an invalid tier blocks every implicit role while explicit valid calls remain usable", async () => {
  const values = complete(); values.small = "gone/model";
  const h = harness({ files: { [GLOBAL]: JSON.stringify(values) } });
  registerObject(h, values); h.available.delete("gone/model");
  for (const [name, params] of [
    ["subagent_grounding", { task: "x" }],
    ["subagent_explore", { task: "x", breadth: "targeted", detail: "paths" }],
    ["subagent_review", { task: "x", kind: "code" }],
    ["subagent_implement", { task: "x", allowCommits: false }],
  ] as const) await assert.rejects(call(h, name, params), /global:small:unavailable/);
  assert.equal((await call(h, "subagent_grounding", { task: "x", model: "p/model" })).value.details.modelSource, "requested");
});

test("all schemas and runtime calls enforce omission or one bounded exact reference", async () => {
  const accepted = `p/${"x".repeat(254)}`;
  const rejected = `${accepted}x`;
  assert.equal(accepted.length, 256);
  assert.equal(rejected.length, 257);
  const h = harness(); h.addModel(accepted);
  const cases = [
    ["subagent_grounding", { task: "x" }],
    ["subagent_explore", { task: "x", breadth: "targeted", detail: "paths" }],
    ["subagent_review", { task: "x", kind: "code" }],
    ["subagent_implement", { task: "x", allowCommits: false }],
  ] as const;
  assert.equal((MODEL_REFERENCE_SCHEMA as any).maxLength, 256);
  for (const [name, params] of cases) {
    const schema = h.tools.get(name).parameters;
    assert.equal(Value.Check(schema, params), true, `${name} rejected omission`);
    assert.equal(Value.Check(schema, { ...params, model: accepted }), true, `${name} rejected 256`);
    assert.equal(Value.Check(schema, { ...params, model: rejected }), false, `${name} accepted 257`);
    await call(h, name, { ...params, model: accepted });
    await assert.rejects(call(h, name, { ...params, model: rejected }), /Omit the model field to use configured or inherited routing\.$/);
    // ADR-0176: the omitted-model display label is proved rejected by every tool schema and by a
    // runtime call, exactly like the sentinels, so a displayed value can never be copied back.
    for (const sentinel of ["default", "auto", "inherit parent", "(configured or inherited)"]) {
      assert.equal(Value.Check(schema, { ...params, model: sentinel }), false, `${name} schema accepted ${sentinel}`);
      await assert.rejects(call(h, name, { ...params, model: sentinel }), /Omit the model field to use configured or inherited routing\.$/);
    }
  }
  assert.deepEqual(parseExactModelReference("a/b"), { provider: "a", id: "b" });

  // ADR-0176: the charset is printable ASCII, so the 256 bound is measure-independent.
  // A non-ASCII reference is rejected by both layers regardless of how long it is.
  const astralShort = `p/${"😀".repeat(4)}`;
  const astralAtBound = `p/${"😀".repeat(254)}`;
  assert.equal(Array.from(astralAtBound).length, 256);
  assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, astralShort), false);
  assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, astralAtBound), false);
  assert.deepEqual(parseExactModelReference(astralShort), { reason: "malformed" });
  // Overlong is reported before form, so a reference that violates both stays overlong.
  assert.deepEqual(parseExactModelReference(`p/${"😀".repeat(300)}`), { reason: "overlong" });
  for (const accented of ["p/mödel", "prövider/model"]) {
    assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, accented), false);
    assert.deepEqual(parseExactModelReference(accented), { reason: "malformed" });
  }

  // Both ends of the permitted range are accepted; the neighbours just outside it are not.
  // Assert the split, not merely the absence of a reason, so a wrong partition at exactly these
  // boundary characters cannot pass.
  for (const [edge, provider, id] of [["\x21/\x21", "\x21", "\x21"], ["\x7E/\x7E", "\x7E", "\x7E"], ["p/a\x21\x7Eb", "p", "a\x21\x7Eb"]] as const) {
    assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, edge), true, `schema rejected ${JSON.stringify(edge)}`);
    assert.deepEqual(parseExactModelReference(edge), { provider, id });
  }
  for (const outside of ["p/a b", "p/a\x7Fb", "p/a\tb", "\x20p/model"]) {
    assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, outside), false, `schema accepted ${JSON.stringify(outside)}`);
    assert.deepEqual(parseExactModelReference(outside), { reason: "malformed" });
  }

  // ADR-0176: the omitted-model display label must never be usable as an argument.
  for (const label of ["(configured or inherited)", "inherit parent"]) {
    assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, label), false, `schema accepted the label ${label}`);
    assert.deepEqual(parseExactModelReference(label), { reason: "malformed" });
  }
});

test("grounding and review reload preferences directly before runner startup", async () => {
  for (const [name, params, role] of [
    ["subagent_grounding", { task: "ground" }, "grounding"],
    ["subagent_review", { task: "review", kind: "code" }, "review"],
  ] as const) {
    const values = complete();
    const files = { [GLOBAL]: JSON.stringify(values) };
    const h = harness({ files }); registerObject(h, values);
    h.addModel(`new/${role}`);
    await new Promise((done) => setImmediate(done));
    const originalRead = h.deps.readFile;
    let globalReads = 0;
    h.deps.readFile = async (path, encoding) => {
      if (path === GLOBAL && ++globalReads === 1) files[GLOBAL] = JSON.stringify({ ...values, [role]: `new/${role}` });
      return originalRead(path, encoding);
    };
    await call(h, name, params);
    assert.equal(`${h.requests[0].model.provider}/${h.requests[0].model.id}`, `new/${role}`, name);
    // Neither role queues, so the dispatch resolves once and reads the
    // preference file once. These paths previously resolved twice and threw the
    // first result away, which cost a redundant read and validation per call.
    assert.equal(globalReads, 1, `${name} reloaded preferences more than once`);
  }
});

test("exploration queue refresh rejects registration and authentication changes before runner startup", async () => {
  for (const [label, invalidate, expected] of [
    ["registration", (h: ReturnType<typeof harness>) => { h.models.delete("p/exploration"); h.available.delete("p/exploration"); }, /unregistered/],
    ["authentication", (h: ReturnType<typeof harness>) => { h.models.get("p\/exploration").authenticated = false; }, /unauthenticated/],
  ] as const) {
    const gate = deferred<RunResult>();
    const values = complete();
    const h = harness({ files: { [GLOBAL]: JSON.stringify(values) }, run: async () => gate.promise });
    registerObject(h, values);
    const active = Array.from({ length: MAX_EXPLORATION_CONCURRENCY }, (_, index) =>
      call(h, "subagent_explore", { task: `${label}-${index}`, breadth: "targeted", detail: "paths" }));
    await new Promise((done) => setImmediate(done));
    assert.equal(h.requests.length, MAX_EXPLORATION_CONCURRENCY);
    const queued = call(h, "subagent_explore", { task: `${label}-queued`, breadth: "targeted", detail: "paths" });
    await new Promise((done) => setImmediate(done));
    invalidate(h);
    gate.resolve(baseResult);
    await Promise.all(active);
    await assert.rejects(queued, expected);
    assert.equal(h.requests.some((request) => request.task === `${label}-queued`), false);
  }
});

test("serialized queue refreshes preferences and registry immediately before runner start", async () => {
  const firstRun = deferred<RunResult>();
  const values = complete();
  const files = { [GLOBAL]: JSON.stringify(values) };
  const h = harness({ files, run: async (request) => request.task === "one" ? firstRun.promise : baseResult });
  registerObject(h, values);
  const first = call(h, "subagent_implement", { task: "one", allowCommits: true });
  await new Promise((done) => setImmediate(done));
  const second = call(h, "subagent_implement", { task: "two", allowCommits: true });
  await new Promise((done) => setImmediate(done));
  files[GLOBAL] = JSON.stringify({ ...values, implementation: "new/model" });
  h.addModel("new/model", true, false);
  firstRun.resolve(baseResult);
  await first;
  await assert.rejects(second, /global:implementation:unavailable/);
  assert.equal(h.requests.some((request) => request.task === "two"), false);
});

test("queued metadata is preflight while running and final metadata use refreshed resolution", async () => {
  const firstRun = deferred<RunResult>();
  const values = complete();
  const files = { [GLOBAL]: JSON.stringify(values) };
  const h = harness({ files, run: async (request) => request.task === "one" ? firstRun.promise : baseResult });
  registerObject(h, values);
  const first = call(h, "subagent_implement", { task: "one", allowCommits: true });
  await new Promise((done) => setImmediate(done));
  const updates: any[] = [];
  const second = h.tools.get("subagent_implement").execute("id", { task: "two", allowCommits: true }, undefined, (value: any) => updates.push(value), h.ctx);
  await new Promise((done) => setImmediate(done));
  files[GLOBAL] = JSON.stringify({ ...values, implementation: "new/model" }); h.addModel("new/model");
  firstRun.resolve(baseResult); await first;
  const final = await second;
  assert.equal(updates[0].details.resolvedModel, "p/implementation");
  assert.equal(updates.at(-1).details.resolvedModel, "new/model");
  assert.equal(final.details.resolvedModel, "new/model");
});

test("incomplete and invalid notices occur once per session-manager identity", async () => {
  const manager1 = { getLeafEntry: () => undefined };
  const h = harness({ sessionManager: manager1 });
  const start = h.hooks.get("session_start");
  await start({}, h.ctx); await start({}, h.ctx);
  assert.equal(h.notices.length, 1);
  assert.match(h.notices[0][0], /incomplete/);
  h.ctx.sessionManager = { getLeafEntry: () => undefined };
  await start({}, h.ctx);
  assert.equal(h.notices.length, 2);

  const bad = harness({ files: { [GLOBAL]: "{" } });
  await bad.hooks.get("session_start")({}, bad.ctx);
  await bad.hooks.get("session_start")({}, bad.ctx);
  assert.equal(bad.notices.length, 1);
  assert.match(bad.notices[0][0], /global:source:malformed-json/);
});

test("wizard preset is eight-field, registry-auth gated, atomic, and refreshes live routing", async () => {
  const files: Record<string, string> = {};
  const h = harness({ files, answers: [GLOBAL_LABEL, true, PRESET, true] }); registerPreset(h);
  await h.runWizard();
  const temp = h.writes.find((entry) => entry.op === "write")!;
  assert.equal(temp.mode, 0o600);
  assert.deepEqual(Object.keys(JSON.parse(temp.data!)), [...PREFERENCE_FIELDS]);
  assert.deepEqual(JSON.parse(temp.data!), RECOMMENDED_PRESET);
  assert.deepEqual(h.writes.map((entry) => entry.op), ["mkdir", "write", "rename"]);
  const routed = await call(h, "subagent_grounding", { task: "x" });
  assert.equal(routed.value.details.resolvedModel, RECOMMENDED_PRESET.grounding);

  const unavailable = harness({ answers: [GLOBAL_LABEL, true, undefined] }); registerPreset(unavailable);
  unavailable.models.get(RECOMMENDED_PRESET.large).authenticated = false;
  await unavailable.runWizard();
  assert.equal(unavailable.prompts.some((prompt) => prompt.title === "Configuration mode"), false);
  assert.match(unavailable.notices[0][0], /preset large model is unauthenticated/);
});

test("wizard manual flow covers fixed role-tier order, leave-unset, and separate preview sections", async () => {
  const chosen = "p/model";
  const h = harness({ answers: [GLOBAL_LABEL, true, MANUAL, chosen, ...Array(7).fill(UNSET), true] }); registerPreset(h);
  await h.runWizard();
  const slotTitles = h.prompts.filter((prompt) => prompt.kind === "select").slice(2).map((prompt) => prompt.title.split(":")[0]);
  assert.deepEqual(slotTitles, [...PREFERENCE_FIELDS]);
  assert.match(h.prompts.find((prompt) => prompt.title.startsWith("small:"))!.title, /narrow, mechanical, low-ambiguity/);
  const summary = h.prompts.find((prompt) => prompt.title === "Save subagent model preferences")!.message;
  for (const section of ["Role defaults:", "Tier mappings:", "Missing:", "Invalid:"]) assert.match(summary, new RegExp(section));
  assert.deepEqual(JSON.parse(h.writes.find((entry) => entry.op === "write")!.data!), { default: chosen });
});

test("wizard cancellation at every legal selection and confirmation boundary writes no preference file", async () => {
  const answerSets: Answer[][] = [
    [undefined], [GLOBAL_LABEL, false], [GLOBAL_LABEL, true, undefined],
    ...PREFERENCE_FIELDS.map((_field, index) => [GLOBAL_LABEL, true, MANUAL, ...Array(index).fill(UNSET), undefined]),
    [GLOBAL_LABEL, true, MANUAL, ...Array(8).fill(UNSET), false],
  ];
  for (const answers of answerSets) {
    const h = harness({ answers }); registerPreset(h); await h.runWizard();
    assert.equal(h.writes.some((entry) => entry.op === "write"), false, JSON.stringify(answers));
  }
  const ignore = harness({ git: [{ code: 0 }, { code: 1 }], answers: [PROJECT_LABEL, true, PRESET, true, false] }); registerPreset(ignore);
  await ignore.runWizard();
  assert.equal(ignore.writes.length, 0);
});

test("wizard enforces project ignore decline, append, existing ignore, and outside-worktree behavior", async () => {
  const appended = harness({ files: { [GITIGNORE]: "existing" }, git: [{ code: 0 }, { code: 1 }], answers: [PROJECT_LABEL, true, PRESET, true, true] }); registerPreset(appended);
  await appended.runWizard();
  assert.equal(appended.files[GITIGNORE], "existing\n.pi/awf-subagents.local.json\n");
  const ignored = harness({ git: [{ code: 0 }, { code: 0 }], answers: [PROJECT_LABEL, true, PRESET, true] }); registerPreset(ignored);
  await ignored.runWizard();
  assert.equal(ignored.writes.some((entry) => entry.path === GITIGNORE), false);
  const outside = harness({ git: [{ code: 1 }], answers: [PROJECT_LABEL, true, PRESET, true] }); registerPreset(outside);
  await outside.runWizard();
  assert.deepEqual(outside.notices[0], ["Not a git work tree; skipping ignore check.", "info"]);

  const ignoreRead = harness({ files: { [GITIGNORE]: new Error("ignore read failed") }, git: [{ code: 0 }, { code: 1 }], answers: [PROJECT_LABEL, true, PRESET, true, true] }); registerPreset(ignoreRead);
  await ignoreRead.runWizard();
  assert.match(ignoreRead.notices.at(-1)![0], /Cannot read .*ignore read failed/);
  assert.equal(ignoreRead.writes.length, 0);

  const ignoreWrite = harness({ git: [{ code: 0 }, { code: 1 }], answers: [PROJECT_LABEL, true, PRESET, true, true] }); registerPreset(ignoreWrite);
  ignoreWrite.deps.writeFile = async (path) => { if (path === GITIGNORE) throw new Error("ignore write failed"); };
  await ignoreWrite.runWizard();
  assert.match(ignoreWrite.notices.at(-1)![0], /Cannot update .*ignore write failed/);
  assert.equal(ignoreWrite.writes.length, 0);
});

test("wizard detects stale writers and handles read, mkdir, write, rename, and cleanup failures", async () => {
  const files: Record<string, string | Error> = { [GLOBAL]: "{}" };
  const stale = harness({ files, answers: [GLOBAL_LABEL, true, PRESET, () => { files[GLOBAL] = "{\"default\":\"p/model\"}"; return true; }] }); registerPreset(stale);
  await stale.runWizard(); assert.match(stale.notices.at(-1)![0], /modified concurrently/);

  const read = harness({ files: { [GLOBAL]: new Error("read failed") }, answers: [GLOBAL_LABEL] });
  await read.runWizard(); assert.match(read.notices.at(-1)![0], /Cannot read/);

  const reread = harness({ answers: [GLOBAL_LABEL, true, PRESET, () => {
    reread.deps.readFile = async () => { throw new Error("reread failed"); };
    return true;
  }] }); registerPreset(reread);
  await reread.runWizard(); assert.match(reread.notices.at(-1)![0], /Cannot re-read .*reread failed/);

  for (const [operation, mutate, expected] of [
    ["mkdir", (h: any) => { h.deps.mkdir = async () => { throw new Error("mkdir failed"); }; }, /Cannot create/],
    ["write", (h: any) => { h.deps.writeFile = async () => { throw new Error("write failed"); }; }, /Save failed: write failed/],
    ["rename", (h: any) => { h.deps.rename = async () => { throw new Error("rename failed"); }; }, /Save failed: rename failed/],
  ] as const) {
    const h = harness({ answers: [GLOBAL_LABEL, true, PRESET, true] }); registerPreset(h); mutate(h);
    await h.runWizard(); assert.match(h.notices.at(-1)![0], expected, operation);
    if (operation !== "mkdir") assert.equal(h.writes.at(-1)?.op, "unlink", operation);
  }

  const cleanup = harness({ answers: [GLOBAL_LABEL, true, PRESET, true] }); registerPreset(cleanup);
  let cleanupAttempts = 0;
  cleanup.deps.rename = async () => { throw new Error("rename primary"); };
  cleanup.deps.unlink = async () => { cleanupAttempts++; throw new Error("cleanup secondary"); };
  await cleanup.runWizard();
  assert.equal(cleanupAttempts, 1);
  assert.match(cleanup.notices.at(-1)![0], /Save failed: rename primary/);
});

const HEAD_BEFORE = { code: 0, stdout: "aaaaaaa\n" };
const HEAD_AFTER = { code: 0, stdout: "bbbbbbb\n" };
const STATUS_CLEAN = { code: 0, stdout: "" };
const STATUS_DIRTY = { code: 0, stdout: " M internal/thing.go\n" };
const ROOT = "/repo";
const WORKTREE = "/repo/.awf/worktrees/verification-checkout";
const COMMON = "/repo/.git";

function checkoutGit(before = "aaaaaaa", after = "bbbbbbb", common = COMMON) {
  let snapshots = 0;
  return (_command: string, args: string[], options: { cwd: string }) => {
    if (args[0] === "rev-parse" && args[1] === "--show-toplevel") return { code: 0, stdout: `${options.cwd}\n` };
    if (args.includes("--git-common-dir")) return { code: 0, stdout: `${options.cwd === ROOT ? COMMON : common}\n` };
    if (args[0] === "rev-parse" && args[1] === "HEAD") return { code: 0, stdout: `${snapshots++ === 0 ? before : after}\n` };
    if (args[0] === "status") return STATUS_CLEAN;
    return { code: 1, stderr: "unexpected git command" };
  };
}

test("implementation verification defaults to the root without changing runner cwd", async () => {
  const h = harness({ git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_AFTER, STATUS_CLEAN] });
  const schema = h.tools.get("subagent_implement").parameters;
  assert.equal(Value.Check(schema, { task: "x", allowCommits: true, verificationCheckout: WORKTREE }), true);
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: true });
  assert.equal(value.details.verificationCheckout, ROOT);
  assert.deepEqual(h.gitCalls.map((entry) => entry.cwd), [ROOT, ROOT, ROOT, ROOT]);
  assert.equal(h.requests[0].cwd, ROOT);
});

test("a linked-worktree HEAD advance satisfies owner verification while root and runner cwd stay fixed", async () => {
  const h = harness({ git: checkoutGit() });
  const { value } = await call(h, "subagent_implement", {
    task: `Edit and commit only under ${WORKTREE}.`, allowCommits: true, verificationCheckout: WORKTREE,
  });
  assert.equal(value.details.state, "completed");
  assert.equal(value.details.verificationCheckout, WORKTREE);
  assert.equal(value.details.before.head, "aaaaaaa");
  assert.equal(value.details.after.head, "bbbbbbb");
  assert.equal(h.requests[0].cwd, ROOT);
  assert.equal(h.gitCalls.filter((entry) => entry.args.includes("HEAD")).every((entry) => entry.cwd === WORKTREE), true);
  assert.equal(h.gitCalls.some((entry) => entry.args[0] === "worktree"), false);
});

test("selected checkout canonicalizes one leading at-sign and filesystem aliases", async () => {
  const alias = "/repo/worktree-link";
  const h = harness({
    git: checkoutGit(),
    realpath: async (path) => path === alias ? WORKTREE : path,
  });
  const { value } = await call(h, "subagent_implement", {
    task: "x", allowCommits: true, verificationCheckout: "@worktree-link",
  });
  assert.equal(value.details.verificationCheckout, WORKTREE);
  assert.equal(h.requests[0].cwd, ROOT);
  assert.equal(h.gitCalls.filter((entry) => entry.cwd !== ROOT).every((entry) => entry.cwd === WORKTREE), true);
});

test("selected checkout detects a forbidden commit and names its resolved identity", async () => {
  const h = harness({ git: checkoutGit() });
  const { value } = await call(h, "subagent_implement", {
    task: "x", allowCommits: false, verificationCheckout: WORKTREE,
  });
  assert.equal(value.details.state, "failed");
  assert.equal(value.details.verificationCheckout, WORKTREE);
  assert.match(value.content[0].text, new RegExp(`committed despite allowCommits=false.*${WORKTREE.replaceAll("/", "\\/")}`));
  assert.equal(h.requests[0].cwd, ROOT);
});

test("invalid explicit verification identities refuse before child dispatch", async () => {
  const cases: Array<{
    label: string; value: string; realpath?: (path: string) => Promise<string>;
    git?: any; expected: RegExp;
  }> = [
    { label: "empty after normalization", value: "@", expected: /verificationCheckout.*empty/ },
    { label: "missing", value: "/missing", realpath: async () => { throw Object.assign(new Error("missing"), { code: "ENOENT" }); }, expected: /verificationCheckout.*does not exist/ },
    { label: "non-Git", value: "/tmp/plain", git: () => ({ code: 1, stderr: "not a git repository" }), expected: /verificationCheckout.*checkout root/ },
    { label: "subdirectory", value: `${WORKTREE}/sub`, git: (_command: string, args: string[]) => args.includes("--show-toplevel") ? { code: 0, stdout: `${WORKTREE}\n` } : { code: 1 }, expected: /verificationCheckout.*checkout root/ },
    { label: "stale", value: WORKTREE, git: () => ({ code: 1, stderr: "not a git repository" }), expected: /verificationCheckout.*registered checkout/ },
    { label: "foreign repository", value: WORKTREE, git: checkoutGit("a", "b", "/foreign/.git"), expected: /verificationCheckout.*same repository/ },
  ];
  for (const item of cases) {
    const h = harness({ git: item.git, realpath: item.realpath });
    await assert.rejects(
      call(h, "subagent_implement", { task: "x", allowCommits: true, verificationCheckout: item.value }),
      item.expected, item.label,
    );
    assert.equal(h.requests.length, 0, item.label);
  }
});

test("a commit-capable implementation that leaves selected HEAD unchanged names retry repair", async () => {
  const h = harness({ git: checkoutGit("aaaaaaa", "aaaaaaa") });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: true, verificationCheckout: WORKTREE });
  assert.equal(value.details.verificationCheckout, WORKTREE);
  assert.match(value.content[0].text, new RegExp(WORKTREE.replaceAll("/", "\\/")));
  assert.match(value.content[0].text, /retry.*verificationCheckout/i);
  assert.equal(h.requests[0].cwd, ROOT);
});

test("a commit-capable implementation that leaves HEAD unchanged fails and demands the stopped inventory", async () => {
  // Two snapshots, two exec calls each: rev-parse then status.
  const h = harness({ git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_BEFORE, STATUS_DIRTY] });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: true });
  assert.equal(value.details.state, "failed");
  assert.equal(value.details.awfFailure, true);
  assert.equal(value.details.commitVerification, "verified");
  const text = value.content[0].text;
  assert.match(text, /commit-capable but created no commit/);
  assert.match(text, /HEAD unchanged at aaaaaaa/);
  for (const clause of [
    /working-tree status/,
    /work completed/,
    /work remaining/,
    /named failing check with its actual output/,
    /what was already tried/,
  ]) assert.match(text, clause);
});

test("the no-commit failure carries the child's own report, and never masks a real failure", async () => {
  const inventory = "stopped\n\ngit status --short:\n M internal/thing.go\n\ncompleted: the parser\nremaining: the encoder\nfailing check: ./x gate, coverage 99.2%\nalready tried: adding a table case";
  const h = harness({
    git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_BEFORE, STATUS_DIRTY],
    run: async () => ({ ...baseResult, output: inventory }),
  });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: true });
  assert.equal(value.details.state, "failed");
  // The inventory is the point of the contract, and a failed result renders only
  // failureMessage, so it must be carried into it rather than replaced.
  assert.match(value.content[0].text, /already tried: adding a table case/);
  assert.match(value.content[0].text, /created no commit/);

  // A run that already failed keeps its own diagnostic, ahead of the demand
  // rather than replaced by it.
  const failed = harness({
    git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_BEFORE, STATUS_DIRTY],
    run: async () => ({ ...baseResult, failed: true, failureMessage: "child exploded", stopReason: "error" }),
  });
  const failedResult = await call(failed, "subagent_implement", { task: "x", allowCommits: true });
  assert.match(failedResult.value.content[0].text, /^child exploded\n\n/);
  assert.match(failedResult.value.content[0].text, /created no commit/);

  // An aborted run keeps its aborted state and its own message, and is still told
  // what a stopped report owes, so the claim holds without exception.
  const aborted = harness({
    git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_BEFORE, STATUS_CLEAN],
    run: async () => ({ ...baseResult, failed: true, failureMessage: "aborted by user", stopReason: "aborted" }),
  });
  const abortedResult = await call(aborted, "subagent_implement", { task: "x", allowCommits: true });
  assert.equal(abortedResult.value.details.state, "aborted");
  assert.match(abortedResult.value.content[0].text, /^aborted by user\n\n/);
  assert.match(abortedResult.value.content[0].text, /created no commit/);
});

test("the loaded implementer contract is prepended with the call's commit authority and stripped of frontmatter", async () => {
  for (const [allowCommits, expected] of [
    [true, "Commits are allowed when the task requests them; you are the phase owner."],
    [false, "Commits are forbidden; do not change HEAD. You are a helper."],
  ] as const) {
    const h = harness();
    withImplementerDoc(h, "---\nname: implementer\ndescription: test\n---\nOwn the transaction.");
    await call(h, "subagent_implement", { task: "x", allowCommits });
    const prompt = h.requests[0].systemPrompt;
    assert.equal(prompt, `You are the governed implementation subagent. ${expected}\n\nOwn the transaction.`);
    // The exact equality above is what backs the frontmatter-strip clause; this
    // line-anchored check states the intent discriminatingly.
    assert.doesNotMatch(prompt, /^---/m);
  }
});

test("a commit-capable implementation that advanced HEAD succeeds", async () => {
  const h = harness({ git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_AFTER, STATUS_CLEAN] });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: true });
  assert.notEqual(value.details.state, "failed");
  assert.equal(value.details.awfFailure, undefined);
});

test("the commit-forbidden violation keeps its original failure", async () => {
  const h = harness({ git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_AFTER, STATUS_CLEAN] });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: false });
  assert.equal(value.details.state, "failed");
  assert.match(value.content[0].text, /committed despite allowCommits=false \(HEAD aaaaaaa -> bbbbbbb\)/);
});

test("a commit-disabled implementation that changed nothing is not a no-commit failure", async () => {
  const h = harness({ git: [HEAD_BEFORE, STATUS_CLEAN, HEAD_BEFORE, STATUS_DIRTY] });
  const { value } = await call(h, "subagent_implement", { task: "x", allowCommits: false });
  assert.notEqual(value.details.state, "failed");
});

// Override only the one agent path under test; every other read (the preference
// sources among them) must still reach the harness stub, or routing blocks first.
function withAgentDoc(h: ReturnType<typeof harness>, relative: string, doc: string | Error) {
  const original = h.deps.readFile;
  h.deps.readFile = async (path: string, encoding: "utf8") => {
    if (path !== `/repo/${relative}`) return original(path, encoding);
    if (doc instanceof Error) throw doc;
    return doc;
  };
}

function withImplementerDoc(h: ReturnType<typeof harness>, doc: string | Error) {
  withAgentDoc(h, ".pi/agents/implementer.md", doc);
}

const ENOENT = () => Object.assign(new Error("missing"), { code: "ENOENT" });

test("the implementation role loads its contract from the rendered agent and fails closed without it", async () => {
  const present = harness();
  withImplementerDoc(present, "---\nname: implementer\ndescription: test\n---\nOwn the transaction.");
  const { value } = await call(present, "subagent_implement", { task: "x", allowCommits: false });
  assert.notEqual(value.details.state, "failed");

  const absent = harness();
  withImplementerDoc(absent, Object.assign(new Error("missing"), { code: "ENOENT" }));
  await assert.rejects(
    call(absent, "subagent_implement", { task: "x", allowCommits: false }),
    /Missing Pi implementer \.pi\/agents\/implementer\.md\. Enable the implementer agent and run awf render\./,
  );

  const bodyless = harness();
  withImplementerDoc(bodyless, "---\nname: implementer\ndescription: test\n---\n   \n");
  await assert.rejects(
    call(bodyless, "subagent_implement", { task: "x", allowCommits: false }),
    /has no instruction body; run awf render\./,
  );
});

test("the exploration role loads its contract from the rendered agent and appends the per-call suffix", async () => {
  const explore = { task: "x", breadth: "targeted" as const, detail: "paths" as const };

  const present = harness();
  withAgentDoc(present, ".pi/agents/explorer.md", "---\nname: explorer\ndescription: test\n---\nExplore within the boundary.");
  const { value } = await call(present, "subagent_explore", explore);
  assert.notEqual(value.details.state, "failed");
  // The suffix is appended to the contract, never a replacement for it.
  const prompt = present.requests[0].systemPrompt;
  assert.match(prompt, /Explore within the boundary\./);
  assert.match(prompt, /Selected breadth maximum: targeted/);
  assert.match(prompt, /Selected report detail: paths/);
  // The shared loader prepends the role's authority line and strips frontmatter,
  // so a per-role divergence from it is caught here and not only for implement.
  assert.equal(prompt.startsWith("You are the governed exploration subagent."), true);
  assert.equal(prompt.includes("name: explorer"), false);

  const absent = harness();
  withAgentDoc(absent, ".pi/agents/explorer.md", ENOENT());
  await assert.rejects(
    call(absent, "subagent_explore", explore),
    /Missing Pi explorer \.pi\/agents\/explorer\.md\. Enable the explorer agent and run awf render\./,
  );

  const bodyless = harness();
  withAgentDoc(bodyless, ".pi/agents/explorer.md", "---\nname: explorer\ndescription: test\n---\n   \n");
  await assert.rejects(
    call(bodyless, "subagent_explore", explore),
    /has no instruction body; run awf render\./,
  );
});

test("the grounding role loads its contract from the rendered agent and fails closed without it", async () => {
  const present = harness();
  withAgentDoc(present, ".pi/agents/grounding-checker.md", "---\nname: grounding-checker\ndescription: test\n---\nTest the premises.");
  const { value } = await call(present, "subagent_grounding", { task: "x" });
  assert.notEqual(value.details.state, "failed");
  const prompt = present.requests[0].systemPrompt;
  assert.match(prompt, /Test the premises\./);
  assert.equal(prompt.startsWith("You are the governed grounding-check subagent."), true);
  assert.equal(prompt.includes("name: grounding-checker"), false);

  const absent = harness();
  withAgentDoc(absent, ".pi/agents/grounding-checker.md", ENOENT());
  await assert.rejects(
    call(absent, "subagent_grounding", { task: "x" }),
    /Missing Pi grounding-checker \.pi\/agents\/grounding-checker\.md\. Enable the grounding-checker agent and run awf render\./,
  );

  const bodyless = harness();
  withAgentDoc(bodyless, ".pi/agents/grounding-checker.md", "---\nname: grounding-checker\ndescription: test\n---\n   \n");
  await assert.rejects(
    call(bodyless, "subagent_grounding", { task: "x" }),
    /has no instruction body; run awf render\./,
  );
});
