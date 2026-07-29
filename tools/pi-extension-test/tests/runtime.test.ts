import assert from "node:assert/strict";
import test from "node:test";
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
  id: "model",
  name: "Runtime fake",
  api: "openai-completions",
  provider: "runtime",
  baseUrl: "http://127.0.0.1/never-called",
  reasoning: false,
  input: ["text"],
  cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
  contextWindow: 4096,
  maxTokens: 256,
};

function terminalMessage(): AssistantMessage {
  return {
    role: "assistant",
    content: [{ type: "text", text: "done" }],
    api: runtimeModel.api,
    provider: runtimeModel.provider,
    model: runtimeModel.id,
    usage: {
      input: 1, output: 1, cacheRead: 0, cacheWrite: 0, totalTokens: 2,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
    },
    stopReason: "stop",
    timestamp: Date.now(),
  };
}

async function runPinnedSession(activeTools: string[]): Promise<{ prompts: string[]; messages: unknown[] }> {
  const prompts: string[] = [];
  const stream = (_model: Model<any>, context: Context) => {
    prompts.push(context.systemPrompt ?? "");
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
    id: "runtime",
    name: "Runtime fake",
    auth: { apiKey: { name: "Runtime fake", resolve: async () => ({ auth: { apiKey: "in-process" }, source: "test" }) } },
    getModels: () => [runtimeModel],
    stream,
    streamSimple: stream,
  };
  const modelRuntime = await ModelRuntime.create({ allowModelNetwork: false, modelsPath: null });
  modelRuntime.registerNativeProvider(provider);
  const model = modelRuntime.getModel("runtime", "model");
  assert.ok(model);

  const cwd = "/tmp/awf-runtime-test";
  const agentDir = "/tmp/awf-runtime-agent";
  const settingsManager = SettingsManager.inMemory({ compaction: { enabled: false }, retry: { enabled: false } });
  const complete = Object.fromEntries(PREFERENCE_FIELDS.map((field) => [field, "runtime/model"]));
  const deps: ExtensionDependencies = {
    packageVersion: "0.81.1",
    extensionFile: `${cwd}/.pi/extensions/awf-subagents/index.ts`,
    agentDir,
    configDirName: ".pi",
    readFile: async (path) => {
      if (path === `${agentDir}/awf-subagents.json`) return JSON.stringify(complete);
      throw Object.assign(new Error("missing"), { code: "ENOENT" });
    },
    writeFile: async () => {}, mkdir: async () => {}, rename: async () => {}, unlink: async () => {},
    runner: { run: async () => { throw new Error("runtime smoke must not execute a subagent tool"); } },
  };
  const loader = new DefaultResourceLoader({
    cwd,
    agentDir,
    settingsManager,
    noSkills: true,
    noPromptTemplates: true,
    noThemes: true,
    noContextFiles: true,
    systemPrompt: "runtime system",
    extensionFactories: [(pi) => registerSubagentTools(pi, deps)],
  });
  await loader.reload();
  const { session } = await createAgentSession({
    cwd,
    agentDir,
    modelRuntime,
    model,
    thinkingLevel: "off",
    tools: activeTools,
    resourceLoader: loader,
    sessionManager: SessionManager.inMemory(cwd),
    settingsManager,
  });
  try {
    await session.prompt("hello");
    return { prompts, messages: [...session.messages] };
  } finally {
    session.dispose();
  }
}

test("pinned runtime injects one run-local routing card without persisting it", async () => {
  const active = await runPinnedSession(["subagent_grounding"]);
  assert.equal(active.prompts.length, 1);
  assert.equal((active.prompts[0].match(/\[awf subagent routing\]/g) ?? []).length, 1);
  assert.match(active.prompts[0], /roles: grounding=runtime\/model/);
  assert.deepEqual(active.messages.map((message: any) => message.role), ["user", "assistant"]);
  assert.equal(JSON.stringify(active.messages).includes("[awf subagent routing]"), false);
  assert.equal(active.messages.some((message: any) => message.customType), false);

  const inactive = await runPinnedSession([]);
  assert.equal(inactive.prompts.length, 1);
  assert.equal(inactive.prompts[0].includes("[awf subagent routing]"), false);
});
