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
import { registerContextUsage } from "../../../.pi/extensions/awf-context-usage/index.ts";
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

async function runPinnedSession(activeTools: string[]): Promise<{ requests: Context[]; messages: unknown[]; entries: unknown[] }> {
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
    extensionFactories: [
      (pi) => registerSubagentTools(pi, deps),
      (pi) => registerContextUsage(pi, { packageVersion: "0.81.1" }),
    ],
  });
  await loader.reload();
  const sessionManager = SessionManager.inMemory(cwd);
  const { session } = await createAgentSession({
    cwd,
    agentDir,
    modelRuntime,
    model,
    thinkingLevel: "off",
    tools: activeTools,
    resourceLoader: loader,
    sessionManager,
    settingsManager,
  });
  try {
    await session.prompt("hello");
    const firstKeptEntryId = sessionManager.getLeafId();
    assert.ok(firstKeptEntryId);
    sessionManager.appendCompaction("runtime smoke compaction", firstKeptEntryId, 2);
    await session.prompt("after compaction");
    return { requests, messages: [...session.messages], entries: sessionManager.getEntries() };
  } finally {
    session.dispose();
  }
}

function contextLines(request: Context): any[] {
  return (request.messages as any[]).filter((message) =>
    message.role === "user" && message.content?.some((content: any) =>
      content.type === "text" && content.text.startsWith("[session context] ")),
  );
}

function contextLine(message: any): string {
  return message.content.find((content: any) => content.type === "text").text;
}

test("pinned runtime refreshes transient context facts in actual requests", async () => {
  const active = await runPinnedSession(["subagent_grounding"]);
  assert.equal(active.requests.length, 2);
  for (const request of active.requests) {
    assert.equal((request.systemPrompt?.match(/\[awf subagent routing\]/g) ?? []).length, 1);
    assert.match(request.systemPrompt ?? "", /roles: grounding=runtime\/model/);
  }
  const firstContext = contextLines(active.requests[0]);
  const secondContext = contextLines(active.requests[1]);
  assert.equal(firstContext.length, 1);
  assert.equal(secondContext.length, 1);
  assert.match(contextLine(firstContext[0]), /^\[session context\] .+\/4\.1k \(.+%\); compactions=0$/);
  assert.match(contextLine(secondContext[0]), /^\[session context\] (?:.+\/4\.1k \(.+%\)|unknown\/4\.1k); compactions=1$/);
  assert.notEqual(contextLine(secondContext[0]), contextLine(firstContext[0]));

  assert.deepEqual(active.messages.map((message: any) => message.role), ["user", "assistant", "user", "assistant"]);
  assert.equal(active.messages.some((message: any) => message.customType === "awf-context-usage"), false);
  assert.equal(active.entries.some((entry: any) => entry.type === "custom" || entry.type === "custom_message"), false);
  const sessionProjection = JSON.stringify({ messages: active.messages, entries: active.entries });
  assert.equal(sessionProjection.includes("awf-context-usage"), false);
  assert.equal(sessionProjection.includes("[session context]"), false);
  assert.doesNotMatch(sessionProjection, /telemetry|workflow-router|selection/i);
  const requestProjection = JSON.stringify(active.requests);
  assert.equal((requestProjection.match(/\[session context\]/g) ?? []).length, 2);

  const inactive = await runPinnedSession([]);
  assert.equal(inactive.requests.length, 2);
  assert.equal((inactive.requests[0].systemPrompt ?? "").includes("[awf subagent routing]"), false);
  assert.equal(contextLines(inactive.requests[0]).length, 1);
  assert.equal(contextLines(inactive.requests[1]).length, 1);
});
