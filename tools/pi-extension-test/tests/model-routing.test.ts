import assert from "node:assert/strict";
import test from "node:test";
import { Value } from "typebox/value";
import {
  GLOBAL_PREFERENCES_FILE,
  LOCAL_PREFERENCES_FILE,
  MODEL_REFERENCE_SCHEMA,
  MODEL_FIELD_REPAIR,
  PREFERENCE_FIELDS,
  PREFERENCES_BLOCKED,
  RECOMMENDED_PRESET,
  ROLE_PREFERENCE_KEYS,
  ROUTING_CARD_OVERFLOW_WARNING,
  buildRoutingCard,
  effectivePreferenceState,
  emptyPreferenceSource,
  invalidText,
  loadPreferenceState,
  parseExactModelReference,
  parsePreferenceSource,
  preferredReference,
  registryFailures,
  resolveChildModel,
  routingPreview,
} from "../../../.pi/extensions/awf-subagents/model-routing.ts";

const GLOBAL = "/agent/awf-subagents.json";
const PROJECT = "/repo/.pi/awf-subagents.local.json";

type Registry = ReturnType<typeof registry>;
function registry(entries: Record<string, { auth?: boolean; available?: boolean }> = {}): {
  find(provider: string, id: string): unknown;
  hasConfiguredAuth(model: unknown): boolean;
  getAvailable(): readonly { provider: string; id: string }[];
} {
  const models = new Map(Object.entries(entries).map(([reference, state]) => [reference, { reference, ...state }]));
  return {
    find: (provider, id) => models.get(`${provider}/${id}`),
    hasConfiguredAuth: (model: any) => model.auth !== false,
    getAvailable: () => [...models.values()].filter((model) => model.available !== false).map((model) => {
      const slash = model.reference.indexOf("/");
      return { provider: model.reference.slice(0, slash), id: model.reference.slice(slash + 1) };
    }),
  };
}
function source(scope: "global" | "project", values: Record<string, string> = {}) {
  const result = emptyPreferenceSource(scope, `/${scope}`);
  result.values = values;
  return result;
}
function state(global = source("global"), project = source("project"), invalid: any[] = []) {
  return effectivePreferenceState(global, project, invalid);
}

test("model references have the exact bounded form and reject sentinel values", () => {
  assert.deepEqual(parseExactModelReference("provider/model"), { provider: "provider", id: "model" });
  assert.deepEqual(parseExactModelReference("!/!"), { provider: "!", id: "!" });
  for (const value of [undefined, null, 4, "default", "auto", "inherit parent", "ab", "/model", "p/", "p/a b", "/a/b", "//a", "p\n/model"]) {
    assert.deepEqual(parseExactModelReference(value), { reason: "malformed" }, String(value));
  }
  assert.deepEqual(parseExactModelReference(`p/${"x".repeat(255)}`), { reason: "overlong" });
  assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, "p/model"), true);
  assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, "auto"), false);
  assert.equal(Value.Check(MODEL_REFERENCE_SCHEMA, `p/${"x".repeat(255)}`), false);
  assert.match(MODEL_FIELD_REPAIR, /Omit the model field/);
});

test("parsing accepts only closed preference objects and reports bounded errors", () => {
  assert.deepEqual(parsePreferenceSource("global", "/g", "{bad").invalid, [{ kind: "source", scope: "global", reason: "malformed-json" }]);
  for (const raw of ["null", "[]", "3"]) assert.deepEqual(parsePreferenceSource("project", "/p", raw).invalid, [{ kind: "source", scope: "project", reason: "non-object" }]);
  const unknown = parsePreferenceSource("global", "/g", '{"default":"p/model","secret":"nope"}');
  assert.deepEqual(unknown.invalid, [{ kind: "source", scope: "global", reason: "unknown-key" }]);
  assert.deepEqual(unknown.values, {});
  const parsed = parsePreferenceSource("project", "/p", JSON.stringify({ default: "p/default", grounding: "bad", small: `p/${"x".repeat(255)}`, review: null }));
  assert.deepEqual(parsed.values, { default: "p/default" });
  assert.deepEqual(parsed.invalid, [
    { kind: "field", scope: "project", field: "grounding", reason: "malformed" },
    { kind: "field", scope: "project", field: "review", reason: "malformed" },
    { kind: "field", scope: "project", field: "small", reason: "overlong" },
  ]);
  assert.equal(invalidText(parsed.invalid[0] as any), "project:grounding:malformed");
  assert.equal(invalidText({ kind: "source", scope: "global", reason: "read-error" } as any), "global:source:read-error");
});

test("preference derivation merges field by field, sorts invalid state, and previews all fallback sources", () => {
  const global = source("global", { default: "g/default", exploration: "g/explore", small: "g/small" });
  global.invalid = [
    { kind: "field", scope: "global", field: "small", reason: "unavailable" },
    { kind: "source", scope: "global", reason: "unknown-key" },
    { kind: "source", scope: "global", reason: "malformed-json" },
  ] as any;
  const project = source("project", { grounding: "p/ground", small: "p/small" });
  project.invalid = [{ kind: "field", scope: "project", field: "large", reason: "malformed" }] as any;
  const derived = state(global, project, [{ kind: "field", scope: "project", field: "review", reason: "unregistered" }] as any);
  assert.deepEqual(derived.effective.small, { reference: "p/small", scope: "project" });
  assert.deepEqual(derived.missing, ["review", "implementation", "standard", "large"]);
  assert.deepEqual(derived.errors, ["global:source:malformed-json", "global:source:unknown-key", "global:small:unavailable", "project:review:unregistered", "project:large:malformed"]);
  assert.equal(derived.blocked, true);
  // Exercise both operand orders of the same-scope source/field comparator.
  const reverse = source("global");
  reverse.invalid = [
    { kind: "source", scope: "global", reason: "read-error" },
    { kind: "field", scope: "global", field: "default", reason: "malformed" },
  ] as any;
  assert.deepEqual(state(reverse).errors, ["global:source:read-error", "global:default:malformed"]);
  assert.deepEqual(preferredReference(project, global, "grounding"), { value: "p/ground", source: "project-role" });
  assert.deepEqual(preferredReference(source("project"), global, "explore"), { value: "g/explore", source: "global-role" });
  assert.deepEqual(preferredReference(source("project", { default: "p/default" }), global, "review"), { value: "p/default", source: "project-default" });
  assert.deepEqual(preferredReference(source("project"), source("global", { default: "g/default" }), "implement"), { value: "g/default", source: "global-default" });
  assert.equal(preferredReference(source("project"), source("global"), "review"), undefined);
  const preview = routingPreview(project, global, derived).join("\n");
  assert.match(preview, /grounding: p\/ground \(project-role\)/);
  assert.match(preview, /exploration: g\/explore \(global-role\)/);
  assert.match(preview, /small: p\/small/);
  assert.match(preview, /Missing: review, implementation, standard, large/);
  const emptyPreview = routingPreview(source("project"), source("global")).join("\n");
  assert.match(emptyPreview, /parent \(inherited\)/);
  assert.match(emptyPreview, /small: unset/);
  assert.match(emptyPreview, /Invalid: none/);
  const completePreview = routingPreview(source("project", Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, `p/${field}`]))), source("global"), state(source("global", { default: "g/default" }), source("project", Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, `p/${field}`]))))).join("\n");
  assert.match(completePreview, /Missing: none/);
});

test("stores handle missing, unreadable, malformed and complete global/project files", async () => {
  const good = registry({ "g/default": {}, "p/ground": {} });
  const files: Record<string, string | Error | undefined> = {
    [GLOBAL]: JSON.stringify({ default: "g/default" }),
    [PROJECT]: JSON.stringify({ grounding: "p/ground" }),
  };
  const deps = {
    agentDir: "/agent", extensionFile: "/repo/.pi/extensions/awf-subagents/model-routing.ts", configDirName: ".pi",
    readFile: async (path: string, _encoding: "utf8") => {
      const value = files[path];
      if (value instanceof Error) throw value;
      if (value === undefined) throw Object.assign(new Error("missing"), { code: "ENOENT" });
      return value;
    },
  };
  const loaded = await loadPreferenceState(deps, good);
  assert.equal(loaded.global.path, GLOBAL);
  assert.equal(loaded.project.path, PROJECT);
  assert.deepEqual(loaded.effective.grounding, { reference: "p/ground", scope: "project" });
  delete files[GLOBAL]; delete files[PROJECT];
  assert.deepEqual((await loadPreferenceState(deps, good)).invalid, []);
  files[GLOBAL] = new Error("unreadable secret"); files[PROJECT] = "{broken";
  assert.deepEqual((await loadPreferenceState(deps, good)).errors, ["global:source:read-error", "project:source:malformed-json"]);
  const primitive = await loadPreferenceState({ ...deps, readFile: async () => { throw "no detail"; } }, good);
  assert.deepEqual(primitive.errors, ["global:source:read-error", "project:source:read-error"]);
  assert.equal(GLOBAL_PREFERENCES_FILE, "awf-subagents.json");
  assert.equal(LOCAL_PREFERENCES_FILE, "awf-subagents.local.json");
});

test("registry failures block implicit routes but valid explicit and precedence routes remain usable", () => {
  const models = registry({ "ok/model": {}, "locked/model": { auth: false }, "gone/model": { available: false } });
  const preferences = source("global", { default: "ghost/model", grounding: "locked/model", exploration: "gone/model", review: "bad" });
  assert.deepEqual(registryFailures(models, [preferences]), [
    { kind: "field", scope: "global", field: "default", reason: "unregistered" },
    { kind: "field", scope: "global", field: "grounding", reason: "unauthenticated" },
    { kind: "field", scope: "global", field: "exploration", reason: "unavailable" },
    { kind: "field", scope: "global", field: "review", reason: "malformed" },
  ]);
  const blocked = state(preferences, source("project"), registryFailures(models, [preferences]));
  assert.throws(() => resolveChildModel(models, { provider: "parent", id: "model" }, "grounding", undefined, blocked), new RegExp(PREFERENCES_BLOCKED));
  assert.deepEqual(resolveChildModel(models, undefined, "grounding", "ok/model", blocked), { model: { provider: "ok", id: "model" }, requested: "ok/model", source: "requested" });
  assert.throws(() => resolveChildModel(models, undefined, "grounding", "bad", blocked), /not an exact provider/);
  assert.throws(() => resolveChildModel(models, undefined, "grounding", `p/${"x".repeat(255)}`, blocked), /longer than 256/);
  assert.throws(() => resolveChildModel(models, undefined, "grounding", "ghost/model", blocked), /unregistered/);
  assert.throws(() => resolveChildModel(models, undefined, "grounding", "locked/model", blocked), /unauthenticated/);
  assert.throws(() => resolveChildModel(models, undefined, "grounding", "gone/model", blocked), /unavailable/);
  const preferred = state(source("global", { default: "ok/model" }), source("project", { grounding: "ok/model" }));
  assert.deepEqual(resolveChildModel(models, { provider: "parent", id: "model" }, "grounding", undefined, preferred), { model: { provider: "ok", id: "model" }, requested: undefined, source: "project-role" });
  const inherited = state();
  assert.deepEqual(resolveChildModel(models, { provider: "parent", id: "model" }, "review", undefined, inherited), { model: { provider: "parent", id: "model" }, requested: undefined, source: "inherited" });
  assert.throws(() => resolveChildModel(models, undefined, "review", undefined, inherited), /without an active parent/);
});

test("routing cards render fallbacks, invalid repairs, and the bounded overflow failure", () => {
  const values = Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, `p/${field}`]));
  const complete = state(source("global", values));
  const card = buildRoutingCard(complete);
  assert.match(card, /default: p\/default/);
  assert.match(card, /roles: grounding=p\/grounding/);
  assert.match(card, /invalid: none/);
  const fallback = state(source("global", { default: "p/default" }));
  assert.match(buildRoutingCard(fallback), /grounding=p\/default/);
  const invalid = state(source("global"), source("project"), [{ kind: "field", scope: "project", field: "small", reason: "unavailable" }] as any);
  assert.match(buildRoutingCard(invalid), /repair: Run \/awf-subagent-models/);
  const huge: any = state(source("global"));
  for (const field of PREFERENCE_FIELDS) huge.effective[field] = { reference: `p/${"x".repeat(1000)}`, scope: "global" };
  assert.match(buildRoutingCard(huge), /state: unavailable/);
  assert.match(ROUTING_CARD_OVERFLOW_WARNING, /4096 UTF-8 bytes/);
  assert.deepEqual(RECOMMENDED_PRESET, {
    default: "openai-codex/gpt-5.6-terra", grounding: "openai-codex/gpt-5.6-sol", exploration: "openai-codex/gpt-5.6-luna", review: "openai-codex/gpt-5.6-sol", implementation: "openai-codex/gpt-5.6-terra", small: "openai-codex/gpt-5.6-luna", standard: "openai-codex/gpt-5.6-terra", large: "openai-codex/gpt-5.6-sol",
  });
  assert.deepEqual(ROLE_PREFERENCE_KEYS, { grounding: "grounding", explore: "exploration", review: "review", implement: "implementation" });
});
