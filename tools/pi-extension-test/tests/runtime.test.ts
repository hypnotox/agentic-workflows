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
  CustomMessageComponent,
  DefaultResourceLoader,
  getMarkdownTheme,
  initTheme,
  ModelRuntime,
  SessionManager,
  SettingsManager,
  withFileMutationQueue,
} from "@earendil-works/pi-coding-agent";
import { contextUsageLine, registerContextUsage } from "../../../.pi/extensions/awf-context-usage/index.ts";
import { registerEffort } from "../../../.pi/extensions/awf-effort/index.ts";
import { handoffEnvelope } from "../../../.pi/extensions/awf-handoff/index.ts";
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

async function runPinnedSession(activeTools: string[], handoffKickoff?: string): Promise<{
  requests: Context[];
  requestUsages: any[];
  messages: unknown[];
  entries: unknown[];
  registrations: unknown[];
  renderers: unknown[];
}> {
  const requests: Context[] = [];
  const requestUsages: any[] = [];
  let activeSession: any;
  const stream = (_model: Model<any>, context: Context) => {
    requests.push(context);
    const usage = activeSession.getContextUsage();
    requestUsages.push(usage == null ? usage : { ...usage });
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
  const extensionFactories = handoffKickoff === undefined
    ? [
      (pi: any) => registerSubagentTools(pi, deps),
      (pi: any) => registerContextUsage(pi, { packageVersion: "0.81.1" }),
      (pi: any) => registerEffort(pi, { packageVersion: "0.81.1", fileMutationQueue: withFileMutationQueue }),
    ]
    : [
      (pi: any) => pi.registerCommand("runtime-agent-handoff", {
        description: "Exercise replacement-bound custom-message delivery.",
        async handler(_args: string, ctx: any) {
          await ctx.newSession({
            async withSession(next: any) {
              await next.sendMessage({
                customType: "agent-handoff",
                content: handoffEnvelope(handoffKickoff),
                display: true,
              }, { triggerTurn: true });
            },
          });
        },
      }),
    ];
  const loader = new DefaultResourceLoader({
    cwd,
    agentDir,
    settingsManager,
    noSkills: true,
    noPromptTemplates: true,
    noThemes: true,
    noContextFiles: true,
    systemPrompt: "runtime system",
    extensionFactories,
  });
  await loader.reload();
  const extensionResult = loader.getExtensions();
  assert.deepEqual(extensionResult.errors, []);
  const registrations = extensionResult.extensions.map((extension: any) => ({
    tools: [...extension.tools.keys()],
    commands: [...extension.commands.keys()],
    flags: [...extension.flags.keys()],
    handlers: [...extension.handlers.keys()],
  }));
  const renderers = extensionResult.extensions.map((extension: any) => [
    ...extension.messageRenderers.keys(),
  ]);
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
  activeSession = session;
  if (handoffKickoff !== undefined) {
    await session.bindExtensions({
      mode: "tui",
      commandContextActions: {
        async newSession(options: any = {}) {
          const replacementManager = SessionManager.inMemory(cwd);
          await options.setup?.(replacementManager);
          (session as any).sessionManager = replacementManager;
          (session as any).agent.state.messages = [];
          await options.withSession?.(session.createReplacedSessionContext());
          return { cancelled: false };
        },
      } as any,
    });
  }
  try {
    if (handoffKickoff === undefined) {
      await session.prompt("hello");
      const firstKeptEntryId = sessionManager.getLeafId();
      assert.ok(firstKeptEntryId);
      sessionManager.appendCompaction("runtime smoke compaction", firstKeptEntryId, 2);
      await session.prompt("after compaction");
    } else {
      await session.prompt("/runtime-agent-handoff");
      await session.waitForIdle();
    }
    return {
      requests,
      requestUsages,
      messages: [...session.messages],
      entries: session.sessionManager.getEntries(),
      registrations,
      renderers,
    };
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

function expectedContextLine(usage: any, compactions: number): string {
  return contextUsageLine({
    getContextUsage: () => usage,
    sessionManager: { getBranch: () => Array.from({ length: compactions }, () => ({ type: "compaction" })) },
  });
}

const expectedRegistrations = [
  {
    tools: ["subagent_grounding", "subagent_explore", "subagent_review", "subagent_implement"],
    commands: ["awf-subagent-models"],
    flags: [],
    handlers: ["session_start", "before_agent_start", "tool_call", "tool_result"],
  },
  { tools: [], commands: [], flags: [], handlers: ["context"] },
  {
    tools: ["using_effort", "effort_memory_read", "effort_memory_edit", "effort_memory_update"],
    commands: [],
    flags: [],
    handlers: ["context", "turn_end", "session_start", "session_shutdown"],
  },
];

test("pinned replacement runtime persists and renders agent-owned handoff", async () => {
  const kickoff = "  exact runtime kickoff  ";
  const envelope = handoffEnvelope(kickoff);
  const active = await runPinnedSession([], kickoff);

  assert.deepEqual(active.registrations, [{
    tools: [],
    commands: ["runtime-agent-handoff"],
    flags: [],
    handlers: [],
  }]);
  assert.deepEqual(active.renderers, [[]]);
  assert.equal(active.requests.length, 1);
  const providerMessages = active.requests[0]!.messages as any[];
  assert.deepEqual(providerMessages, [{
    role: "user",
    content: [{ type: "text", text: envelope }],
    timestamp: providerMessages[0]!.timestamp,
  }]);

  const customMessages = (active.messages as any[]).filter((message) => message.role === "custom");
  assert.deepEqual(customMessages, [{
    role: "custom",
    customType: "agent-handoff",
    content: envelope,
    display: true,
    details: undefined,
    timestamp: customMessages[0]!.timestamp,
  }]);
  const persisted = (active.entries as any[]).filter((entry) =>
    entry.type === "custom_message" && entry.customType === "agent-handoff");
  assert.equal(persisted.length, 1);
  assert.equal(persisted[0].content, envelope);
  assert.equal(persisted[0].display, true);

  initTheme("dark", false);
  const component = new CustomMessageComponent(
    customMessages[0] as any,
    undefined,
    getMarkdownTheme(),
  );
  const rendered = component.render(120).join("\n").replace(/\x1b\[[0-9;]*m/g, "");
  assert.equal((rendered.match(/\[agent-handoff\]/g) ?? []).length, 1);
  assert.equal((rendered.match(/Agent-authored handoff context; this is not user input:/g) ?? []).length, 1);
  assert.equal(rendered.includes(kickoff), true);
});

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
  const firstLine = contextLine(firstContext[0]);
  const secondLine = contextLine(secondContext[0]);
  assert.equal(firstLine, expectedContextLine(active.requestUsages[0], 0));
  assert.equal(secondLine, expectedContextLine(active.requestUsages[1], 1));
  assert.match(firstLine, /^\[session context\] \d+(?:\.\d)?[km]?\/4\.1k \(\d+%\); compactions=0$/);
  assert.match(secondLine, /^\[session context\] unknown\/4\.1k; compactions=1$/);
  assert.notEqual(firstLine.replace(/; compactions=\d+$/, ""), secondLine.replace(/; compactions=\d+$/, ""));
  assert.deepEqual(active.registrations, expectedRegistrations);

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
  for (const request of inactive.requests) {
    assert.equal((request.systemPrompt ?? "").includes("[awf subagent routing]"), false);
    assert.equal(contextLines(request).length, 1);
  }
  assert.deepEqual(inactive.registrations, expectedRegistrations);
});
