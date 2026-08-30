import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { Value } from "typebox/value";
import {
  createAssistantMessageEventStream,
  type AssistantMessage,
  type Context,
  type Model,
  type Provider,
} from "@earendil-works/pi-ai";
import {
  createAgentSession,
  DefaultResourceLoader,
  ModelRuntime,
  SessionManager,
  SettingsManager,
} from "@earendil-works/pi-coding-agent";
import { registerSubagentTools, type ExtensionDependencies } from "../../../.pi/extensions/awf-subagents/index.ts";
import { PREFERENCE_FIELDS } from "../../../.pi/extensions/awf-subagents/model-routing.ts";

const runtimeModel: Model<"openai-completions"> = {
  id: "model", name: "Runtime fake", api: "openai-completions", provider: "runtime",
  baseUrl: "http://127.0.0.1/never-called", reasoning: false, input: ["text"],
  cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 }, contextWindow: 4096, maxTokens: 256,
};

function terminalMessage(): AssistantMessage {
  return {
    role: "assistant", content: [{ type: "text", text: "done" }], api: runtimeModel.api,
    provider: runtimeModel.provider, model: runtimeModel.id,
    usage: { input: 1, output: 1, cacheRead: 0, cacheWrite: 0, totalTokens: 2, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } },
    stopReason: "stop", timestamp: Date.now(),
  };
}

test("pinned Pi runtime discovers awf skills and delivers protocol-v2 routing without effort association", async () => {
  const requests: Context[] = [];
  const stream = (_model: Model<any>, context: Context) => {
    requests.push(context);
    const events = createAssistantMessageEventStream();
    const message = terminalMessage();
    const partial = { ...message, content: [] };
    queueMicrotask(() => {
      events.push({ type: "start", partial });
      events.push({ type: "text_start", contentIndex: 0, partial });
      events.push({ type: "text_delta", contentIndex: 0, delta: "done", partial: message });
      events.push({ type: "text_end", contentIndex: 0, content: "done", partial: message });
      events.push({ type: "done", reason: "stop", message });
    });
    return events;
  };
  const provider: Provider<"openai-completions"> = {
    id: "runtime", name: "Runtime fake",
    auth: { apiKey: { name: "Runtime fake", resolve: async () => ({ auth: { apiKey: "in-process" }, source: "test" }) } },
    getModels: () => [runtimeModel], stream, streamSimple: stream,
  };
  const modelRuntime = await ModelRuntime.create({ allowModelNetwork: false, modelsPath: null });
  modelRuntime.registerNativeProvider(provider);
  const model = modelRuntime.getModel("runtime", "model");
  assert.ok(model);

  const cwd = process.cwd();
  const agentDir = "/tmp/awf-runtime-agent";
  const preferences = JSON.stringify(Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, "runtime/model"])));
  const deps: ExtensionDependencies = {
    extensionFile: `${cwd}/.pi/extensions/awf-subagents/index.ts`, agentDir, configDirName: ".pi",
    readFile: async (path, encoding) => path === `${agentDir}/awf-subagents.json` ? preferences : readFile(path, encoding),
    writeFile: async () => {}, mkdir: async () => {}, rename: async () => {}, unlink: async () => {},
    realpath: async (path) => path, lstat: async () => ({ isFile: () => true, isSymbolicLink: () => false }),
  };
  let registeredBatch: any;
  let registrationReceipt: any;
  let finalRegistration: any;
  let toolsInstalled = false;
  const extensionFactories = [
    (pi: any) => {
      pi.events.on("pi-tools:subagent-profiles:request", (request: any) => {
        assert.equal(request.protocolVersion, 2);
        pi.events.emit("pi-tools:subagent-profiles:capability", {
          protocolVersion: 2, correlationId: request.correlationId,
          register(batch: any) {
            registeredBatch = batch;
            registrationReceipt = { state: "pending" };
            return registrationReceipt;
          },
        });
      });
      pi.on("session_start", () => {
        if (toolsInstalled || !registeredBatch) return;
        for (const profile of registeredBatch.profiles) {
          pi.registerTool({
            name: profile.toolName, label: profile.label, description: profile.description,
            parameters: profile.parameters,
            async execute() { return { content: [{ type: "text", text: "contract double" }] }; },
          });
        }
        toolsInstalled = true;
        finalRegistration = { protocolVersion: 2, registrationId: registeredBatch.registrationId, state: "registered" };
        assert.equal(toolsInstalled, true);
        pi.events.emit("pi-tools:subagent-profiles:registration-result", finalRegistration);
      });
    },
    (pi: any) => registerSubagentTools(pi, deps),
  ];
  const settingsManager = SettingsManager.inMemory({ compaction: { enabled: false }, retry: { enabled: false } });
  const loader = new DefaultResourceLoader({
    cwd, agentDir, settingsManager, noExtensions: true, noPromptTemplates: true, noThemes: true, noContextFiles: true,
    systemPrompt: "runtime system", extensionFactories,
  });
  await loader.reload();
  assert.deepEqual(loader.getExtensions().errors, []);
  const discovered = loader.getSkills();
  assert.deepEqual(discovered.diagnostics, []);
  assert.ok(discovered.skills.some((skill: any) => skill.name === "awf-repository-context"));
  assert.equal(registeredBatch.registrationId, "awf:subagent-profiles:v2");
  assert.equal(registeredBatch.suppressDefault, true);
  assert.deepEqual(registrationReceipt, { state: "pending" });
  assert.equal(toolsInstalled, false);
  assert.equal(finalRegistration, undefined);
  assert.deepEqual(registeredBatch.profiles.map((profile: any) => profile.toolName), [
    "subagent_grounding", "subagent_explore", "subagent_review_code", "subagent_implement",
  ]);

  const loadedExtensions = loader.getExtensions().extensions;
  const registrations = loadedExtensions.map((extension: any) => [...extension.tools.keys()]);
  assert.ok(registrations.every((tools: string[]) => !tools.includes("using_effort")));
  const runtimeRegistry = {
    find: (provider: string, id: string) => provider === "runtime" && id === "model" ? model : undefined,
    hasConfiguredAuth: () => true,
    getAvailable: () => [model],
  };
  for (const loaded of loadedExtensions) {
    const starts = loaded.handlers.get("session_start") ?? [];
    for (const start of starts) await start({}, { modelRegistry: runtimeRegistry, model, ui: { notify() {} }, sessionManager: {} });
  }
  assert.equal(toolsInstalled, true);
  assert.deepEqual(finalRegistration, { protocolVersion: 2, registrationId: "awf:subagent-profiles:v2", state: "registered" });
  const grounding = registeredBatch.profiles.find((profile: any) => profile.id === "awf-grounding");
  const profileContext: any = {
    args: { task: "ground", model: "runtime/model" },
    parent: { cwd, activeTools: ["subagent_grounding"], model: { provider: "runtime", id: "model", thinkingLevels: ["off"] }, thinkingLevel: "off", trusted: true },
    signal: new AbortController().signal,
  };
  const selected = await grounding.selectModel(profileContext);
  assert.equal(`${selected.provider}/${selected.id}`, "runtime/model");
  const prepared = await grounding.prepare(profileContext);
  assert.equal(prepared.cwd, cwd);
  assert.match(prepared.systemPrompt, /report-only premise checker/);
  const profileState = await grounding.beforeRun(profileContext);
  const postRun = await grounding.afterRun({
    state: "completed", report: "grounded",
    usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } },
    activity: [], omittedActivity: 0, retries: 0, retryActive: false,
  }, profileState);
  assert.equal(Value.Check(grounding.profileDataSchema, postRun.profileData), true);
  assert.deepEqual(postRun.profileData, { requestedModel: "runtime/model", resolvedProvider: "runtime", resolvedModel: "model", resolutionSource: "requested" });
  const sessionManager = SessionManager.inMemory(cwd);
  const { session } = await createAgentSession({
    cwd, agentDir, modelRuntime, model, thinkingLevel: "off", tools: ["subagent_grounding"],
    resourceLoader: loader, sessionManager, settingsManager,
  });
  try {
    await session.prompt("hello");
    assert.equal(requests.length, 1);
    assert.match(requests[0].systemPrompt ?? "", /awf subagent routing/);
    assert.match(requests[0].systemPrompt ?? "", /grounding=runtime\/model/);
  } finally {
    session.dispose();
  }
});
