import assert from "node:assert/strict";
import { cp, mkdir, mkdtemp, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createAssistantMessageEventStream } from "@earendil-works/pi-ai";
import {
  createAgentSession,
  createAgentSessionFromServices,
  createAgentSessionRuntime,
  createAgentSessionServices,
  DefaultResourceLoader,
  ModelRuntime,
  SessionManager,
  SettingsManager,
} from "@earendil-works/pi-coding-agent";
import { Type } from "typebox";
import {
  MIN_PI_VERSION,
  registerSubagentTools,
  type ExtensionDependencies,
} from "../../../.pi/extensions/awf-subagents/index.ts";
import type { RunResult } from "../../../.pi/extensions/awf-subagents/runner.ts";

const usage = {
  input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0,
  cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
};

function assistant(content: any[], stopReason: "toolUse" | "stop") {
  return {
    role: "assistant" as const,
    content,
    api: "openai-completions" as const,
    provider: "runtime-test",
    model: "runtime-test",
    usage,
    stopReason,
    timestamp: Date.now(),
  };
}

function registerRuntimeTestProvider(modelRuntime: ModelRuntime, streamSimple: () => any) {
  modelRuntime.registerProvider("runtime-test", {
    api: "openai-completions",
    apiKey: "test-key",
    baseUrl: "http://runtime.test",
    streamSimple,
    models: [{
      id: "runtime-test", name: "Runtime Test", reasoning: false, input: ["text"],
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 }, contextWindow: 10000, maxTokens: 1000,
    }],
  });
  const model = modelRuntime.getModel("runtime-test", "runtime-test");
  if (!model) throw new Error("runtime test model was not registered");
  return model;
}

async function waitFor(predicate: () => boolean, message: string) {
  const deadline = Date.now() + 2000;
  while (!predicate()) {
    if (Date.now() >= deadline) throw new Error(message);
    await new Promise((resolve) => setTimeout(resolve, 5));
  }
}

async function realHandoffRuntime(root: string, options: { cancel?: boolean; preReplacementError?: Error; failReplacementCreation?: boolean } = {}) {
  const order: string[] = [];
  const notifications: any[] = [];
  const editor: string[] = [];
  const streamCalls: number[] = [];
  const toolEnds: any[] = [];
  let creations = 0;
  let runtime: any;
  const extensionFactories = [
    (pi: any) => { pi.on("session_shutdown", () => { order.push("session_shutdown"); }); },
  ];
  const createRuntime = async ({ cwd, sessionManager, sessionStartEvent }: any) => {
    creations++;
    if (options.failReplacementCreation && creations === 2) throw new Error("replacement creation failed after teardown");
    const settingsManager = SettingsManager.inMemory();
    const services = await createAgentSessionServices({
      cwd,
      agentDir: root,
      settingsManager,
      resourceLoaderOptions: { extensionFactories },
    });
    assert.ok(services.resourceLoader instanceof DefaultResourceLoader);
    const creation = creations;
    const model = registerRuntimeTestProvider(services.modelRuntime, () => {
      streamCalls.push(creation);
      const stream = createAssistantMessageEventStream();
      const message = creation === 1
        ? assistant([{ type: "toolCall", id: "handoff-real", name: "handoff_session", arguments: { memoryPath: ".awf/memory/work.md", kickoff: "continue runtime verification" } }], "toolUse")
        : assistant([{ type: "text", text: "child kickoff accepted" }], "stop");
      queueMicrotask(() => {
        stream.push({ type: "start", partial: message });
        stream.push({ type: "done", reason: message.stopReason, message });
      });
      return stream;
    });
    const created = await createAgentSessionFromServices({
      services,
      sessionManager,
      sessionStartEvent,
      model,
      tools: ["handoff_session"],
      noTools: "builtin",
    });
    return { ...created, services, diagnostics: services.diagnostics };
  };
  await mkdir(join(root, ".awf/memory"), { recursive: true });
  await writeFile(join(root, ".awf/memory/work.md"), "checkpoint");
  await mkdir(join(root, ".pi/extensions"), { recursive: true });
  await cp(join(process.cwd(), ".pi/extensions/awf-handoff"), join(root, ".pi/extensions/awf-handoff"), { recursive: true });
  await cp(join(process.cwd(), ".pi/extensions/awf-subagents"), join(root, ".pi/extensions/awf-subagents"), { recursive: true });
  await cp(join(process.cwd(), ".pi/extensions/awf-dashboard"), join(root, ".pi/extensions/awf-dashboard"), { recursive: true });
  await symlink(join(process.cwd(), "node_modules"), join(root, "node_modules"), "dir");
  const sessionDir = join(root, "sessions");
  await mkdir(sessionDir, { recursive: true });
  runtime = await createAgentSessionRuntime(createRuntime, { cwd: root, agentDir: root, sessionManager: SessionManager.create(root, sessionDir) });
  const bind = async (session: any) => {
    session.subscribe((event: any) => {
      if (event.type === "tool_execution_end") {
        order.push("tool_execution_end");
        toolEnds.push(event);
      }
      if (event.type === "agent_settled") order.push("agent_settled");
    });
    const uiContext: any = {
      notify: (...args: any[]) => notifications.push(args),
      setEditorText: (text: string) => editor.push(text),
      setWidget() {},
      custom: async (factory: any) => new Promise((resolve, reject) => {
        try {
          const done = (value: unknown) => resolve(value);
          const component = factory({ requestRender() {} }, {}, { matches: () => options.cancel === true }, done);
          Promise.resolve(component).then((resolved: any) => {
            if (options.cancel) resolved.handleInput("escape");
            else done(true);
            resolved.dispose?.();
          }, reject);
        } catch (error) { reject(error); }
      }),
    };
    const unsupported = async () => ({ cancelled: false });
    await session.bindExtensions({
      mode: "tui",
      uiContext,
      commandContextActions: {
        waitForIdle: async () => {},
        newSession: async (newSessionOptions: any) => {
          if (options.preReplacementError) throw options.preReplacementError;
          return runtime.newSession(newSessionOptions);
        },
        fork: unsupported,
        navigateTree: unsupported,
        switchSession: unsupported,
        reload: async () => {},
      },
      onError: (error: any) => order.push(`extension_error:${error.error}`),
    });
  };
  runtime.setRebindSession(bind);
  await bind(runtime.session);
  return { runtime, order, notifications, editor, streamCalls, toolEnds, creations: () => creations };
}

test("real Pi parallel preflight enforces current-leaf implementation batch exclusivity", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-pi-batch-"));
  let turn = 0;
  try {
    const childResult: RunResult = {
      output: "child ok", stderr: "", events: [], omittedEvents: 0, failed: false, modelChanged: false,
      usage: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, cost: 0, turns: 1 },
      model: "runtime-test/runtime-test",
    };
    const deps: ExtensionDependencies = {
      readFile: async () => "---\nname: reviewer\ndescription: test\n---\nReview.",
      runner: { run: async () => childResult },
      packageVersion: MIN_PI_VERSION,
      extensionFile: join(root, ".pi/extensions/awf-subagents/index.ts"),
    };
    const loader = new DefaultResourceLoader({
      cwd: root,
      agentDir: root,
      settingsManager: SettingsManager.inMemory(),
      extensionFactories: [(pi) => {
        pi.registerTool({
          name: "runtime_sibling",
          label: "Runtime Sibling",
          description: "Runtime sibling probe.",
          parameters: Type.Object({}),
          async execute() { return { content: [{ type: "text" as const, text: "sibling ok" }], details: {} }; },
        });
        pi.on("tool_call", (event, ctx) => {
          if (event.toolCallId === "stale-impl") {
            (ctx.sessionManager as any).appendMessage(assistant([{ type: "toolCall", id: "old", name: "subagent_implement", arguments: {} }], "toolUse"));
          }
          if (event.toolCallId === "malformed-read") {
            (ctx.sessionManager as any).appendMessage({ role: "user", content: [{ type: "text", text: "malformed leaf" }], timestamp: Date.now() });
          }
        });
        registerSubagentTools(pi, deps);
      }],
    });
    await loader.reload();
    const modelRuntime = await ModelRuntime.create({ authPath: join(root, "auth.json"), modelsPath: join(root, "models.json") });
    let streamSimple!: () => any;
    const model = registerRuntimeTestProvider(modelRuntime, () => streamSimple());
    const sessionManager = SessionManager.inMemory(root);
    const { session } = await createAgentSession({
      cwd: root,
      agentDir: root,
      model,
      modelRuntime,
      tools: ["subagent_grounding", "subagent_implement", "runtime_sibling"],
      noTools: "builtin",
      resourceLoader: loader,
      settingsManager: SettingsManager.inMemory(),
      sessionManager,
    });
    const ends: any[] = [];
    session.subscribe((event) => { if (event.type === "tool_execution_end") ends.push(event); });
    streamSimple = () => {
      const stream = createAssistantMessageEventStream();
      const messages = [
        assistant([{ type: "toolCall", id: "solo-impl", name: "subagent_implement", arguments: { task: "solo", allowCommits: false } }], "toolUse"),
        assistant([
          { type: "toolCall", id: "mixed-impl", name: "subagent_implement", arguments: { task: "mixed", allowCommits: false } },
          { type: "toolCall", id: "mixed-ground", name: "subagent_grounding", arguments: { task: "ground" } },
        ], "toolUse"),
        assistant([{ type: "toolCall", id: "stale-impl", name: "subagent_implement", arguments: { task: "stale", allowCommits: false } }], "toolUse"),
        assistant([{ type: "toolCall", id: "malformed-read", name: "runtime_sibling", arguments: {} }], "toolUse"),
        assistant([{ type: "text", text: "done" }], "stop"),
      ];
      const message = messages[turn++];
      queueMicrotask(() => {
        stream.push({ type: "start", partial: message });
        stream.push({ type: "done", reason: message.stopReason, message });
      });
      return stream;
    };
    try {
      await session.prompt("exercise guard");
      const byID = new Map(ends.map((entry) => [entry.toolCallId, entry]));
      assert.equal(byID.get("solo-impl").isError, false);
      for (const id of ["mixed-impl", "mixed-ground"]) {
        assert.equal(byID.get(id).isError, true);
        assert.match(byID.get(id).result.content[0].text, /batch containing subagent_implement cannot contain siblings/);
      }
      assert.equal(byID.get("stale-impl").isError, true);
      assert.match(byID.get("stale-impl").result.content[0].text, /Cannot verify the current tool batch/);
      assert.equal(byID.get("malformed-read").isError, false);
      assert.equal(byID.get("malformed-read").result.content[0].text, "sibling ok");
      const stored = session.messages.filter((message: any) => message.role === "toolResult") as any[];
      for (const id of ["mixed-impl", "mixed-ground", "stale-impl"]) assert.equal(stored.find((message) => message.toolCallId === id)?.isError, true);
      assert.equal(sessionManager.getLeafEntry()?.type, "message");
    } finally {
      session.dispose();
    }
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});

test("Pi fork 0.81.1 queues handoff after settlement, terminates the turn, and creates a parent-linked kickoff", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-pi-handoff-real-"));
  let fixture: Awaited<ReturnType<typeof realHandoffRuntime>> | undefined;
  try {
    fixture = await realHandoffRuntime(root);
    const oldSession = fixture.runtime.session;
    const oldFile = oldSession.sessionFile;
    assert.ok(oldFile);
    await oldSession.prompt("perform handoff");
    await waitFor(() => fixture!.runtime.session !== oldSession, "queued handoff did not replace the session");
    assert.deepEqual(fixture.streamCalls.filter((creation) => creation === 1), [1]);
    assert.equal(fixture.toolEnds[0].result.terminate, true);
    assert.ok(fixture.order.indexOf("tool_execution_end") >= 0);
    assert.ok(fixture.order.indexOf("agent_settled") > fixture.order.indexOf("tool_execution_end"));
    assert.ok(fixture.order.indexOf("session_shutdown") > fixture.order.indexOf("agent_settled"));
    const child = fixture.runtime.session;
    assert.notEqual(child.sessionFile, oldFile);
    assert.equal(child.sessionManager.isPersisted(), true);
    assert.equal(child.sessionManager.getHeader()?.parentSession, oldFile);
    await waitFor(() => fixture!.streamCalls.includes(2), "replacement kickoff did not run");
    const childUser = child.messages.find((message: any) => message.role === "user") as any;
    const wrapper = childUser.content.map((part: any) => part.text ?? "").join("");
    assert.match(wrapper, /Read \.awf\/memory\/work\.md first/);
    assert.match(wrapper, /continue runtime verification/);
  } finally {
    await fixture?.runtime.dispose();
    await rm(root, { recursive: true, force: true });
  }
});

test("Pi fork 0.81.1 cancellation preserves the active persisted session", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-pi-handoff-cancel-"));
  let fixture: Awaited<ReturnType<typeof realHandoffRuntime>> | undefined;
  try {
    fixture = await realHandoffRuntime(root, { cancel: true });
    const oldSession = fixture.runtime.session;
    const oldFile = oldSession.sessionFile;
    await oldSession.prompt("cancel handoff");
    await waitFor(() => fixture!.notifications.length > 0, "cancellation notification did not arrive");
    assert.equal(fixture.runtime.session, oldSession);
    assert.equal(oldSession.sessionFile, oldFile);
    assert.deepEqual(fixture.notifications, [["Fresh-session handoff canceled."]]);
    assert.deepEqual(fixture.streamCalls, [1]);
  } finally {
    await fixture?.runtime.dispose();
    await rm(root, { recursive: true, force: true });
  }
});

test("Pi fork 0.81.1 exposes truthful pre- and post-teardown replacement failure boundaries", async (t) => {
  await t.test("failure before runtime replacement preserves the old active session", async () => {
    const root = await mkdtemp(join(tmpdir(), "awf-pi-handoff-pre-fail-"));
    let fixture: Awaited<ReturnType<typeof realHandoffRuntime>> | undefined;
    try {
      fixture = await realHandoffRuntime(root, { preReplacementError: new Error("replacement rejected before teardown") });
      const oldSession = fixture.runtime.session;
      await oldSession.prompt("fail before replacement");
      await waitFor(() => fixture!.order.some((entry) => entry.includes("replacement rejected before teardown")), "pre-replacement failure was not reported");
      assert.equal(fixture.runtime.session, oldSession);
      assert.equal(fixture.creations(), 1);
      assert.equal(fixture.order.includes("session_shutdown"), false);
    } finally {
      await fixture?.runtime.dispose();
      await rm(root, { recursive: true, force: true });
    }
  });

  await t.test("failure creating the replacement occurs after old-session teardown", async () => {
    const root = await mkdtemp(join(tmpdir(), "awf-pi-handoff-post-fail-"));
    let fixture: Awaited<ReturnType<typeof realHandoffRuntime>> | undefined;
    try {
      fixture = await realHandoffRuntime(root, { failReplacementCreation: true });
      const oldSession = fixture.runtime.session;
      let promptSettled = false;
      const promptTask = oldSession.prompt("fail after replacement teardown").then(
        () => { promptSettled = true; },
        () => { promptSettled = true; },
      );
      void promptTask.catch(() => {});
      await waitFor(() => fixture!.creations() === 2, "queued handoff did not attempt replacement creation");
      await waitFor(() => fixture!.order.includes("session_shutdown"), "old session did not shut down before replacement creation failure");
      await waitFor(
        () => fixture!.order.some((entry) => entry.includes("replacement creation failed after teardown")),
        "post-teardown replacement failure was not reported through the extension",
      );
      assert.equal(fixture.toolEnds[0].result.terminate, true);
      assert.ok(fixture.order.indexOf("agent_settled") > fixture.order.indexOf("tool_execution_end"));
      assert.ok(fixture.order.indexOf("session_shutdown") > fixture.order.indexOf("agent_settled"));
      assert.equal(fixture.runtime.session, oldSession);
      assert.equal(fixture.creations(), 2);
      assert.equal(promptSettled, true, "the terminated old-session prompt should settle even though replacement failed");
    } finally {
      await fixture?.runtime.dispose().catch(() => {});
      await rm(root, { recursive: true, force: true });
    }
  });
});

test("Pi fork 0.81.1 session preserves partial details and patched error results", async () => {
  const root = await mkdtemp(join(tmpdir(), "awf-pi-runtime-"));
  let turn = 0;
  try {
    const loader = new DefaultResourceLoader({
      cwd: root,
      agentDir: root,
      settingsManager: SettingsManager.inMemory(),
      extensionFactories: [(pi) => {
        pi.registerTool({
          name: "runtime_probe",
          label: "Runtime Probe",
          description: "Probe Pi tool result semantics.",
          parameters: Type.Object({}),
          async execute(_id, _params, _signal, onUpdate) {
            onUpdate?.({
              content: [{ type: "text", text: "(running...)" }],
              details: { events: [{ sequence: 1, kind: "assistant", text: "private progress" }] },
            });
            return {
              content: [{ type: "text", text: "final failure" }],
              details: { awfFailure: true, events: [{ sequence: 1, kind: "assistant", text: "private progress" }] },
            };
          },
        });
        pi.on("tool_result", (event) => {
          if (event.toolName === "runtime_probe" && (event.details as any)?.awfFailure === true) return { isError: true };
        });
      }],
    });
    await loader.reload();
    const modelRuntime = await ModelRuntime.create({ authPath: join(root, "auth.json"), modelsPath: join(root, "models.json") });
    let streamSimple!: () => any;
    const model = registerRuntimeTestProvider(modelRuntime, () => streamSimple());
    const { session } = await createAgentSession({
      cwd: root,
      agentDir: root,
      model,
      modelRuntime,
      tools: ["runtime_probe"],
      noTools: "builtin",
      resourceLoader: loader,
      settingsManager: SettingsManager.inMemory(),
      sessionManager: SessionManager.inMemory(root),
    });
    const updates: any[] = [];
    const ends: any[] = [];
    session.subscribe((event) => {
      if (event.type === "tool_execution_update") updates.push(event.partialResult);
      if (event.type === "tool_execution_end") ends.push(event);
    });
    streamSimple = () => {
      const stream = createAssistantMessageEventStream();
      const message = turn++ === 0
        ? assistant([{ type: "toolCall", id: "probe-1", name: "runtime_probe", arguments: {} }], "toolUse")
        : assistant([{ type: "text", text: "done" }], "stop");
      queueMicrotask(() => {
        stream.push({ type: "start", partial: message });
        stream.push({ type: "done", reason: message.stopReason, message });
      });
      return stream;
    };
    try {
      await session.prompt("run probe");
      assert.equal(updates[0].content[0].text, "(running...)");
      assert.equal(updates[0].details.events[0].text, "private progress");
      assert.equal(ends[0].result.content[0].text, "final failure");
      assert.equal(ends[0].result.details.events[0].text, "private progress");
      assert.equal(ends[0].isError, true);
      const stored = session.messages.find((message: any) => message.role === "toolResult") as any;
      assert.equal(stored.content[0].text, "final failure");
      assert.equal(stored.content.some((part: any) => part.text === "private progress"), false);
      assert.equal(stored.details.events[0].text, "private progress");
      assert.equal(stored.isError, true);
    } finally {
      session.dispose();
    }
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
