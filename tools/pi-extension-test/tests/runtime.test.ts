import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { createAssistantMessageEventStream } from "@earendil-works/pi-ai";
import {
  createAgentSession,
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
    modelRuntime.setRuntimeApiKey("runtime-test", "test-key");
    const sessionManager = SessionManager.inMemory(root);
    const { session } = await createAgentSession({
      cwd: root,
      agentDir: root,
      model: { provider: "runtime-test", id: "runtime-test", api: "openai-completions" } as any,
      modelRuntime,
      tools: ["subagent_grounding", "subagent_implement", "runtime_sibling"],
      noTools: "builtin",
      resourceLoader: loader,
      settingsManager: SettingsManager.inMemory(),
      sessionManager,
    });
    const ends: any[] = [];
    session.subscribe((event) => { if (event.type === "tool_execution_end") ends.push(event); });
    session.agent.streamFn = () => {
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

test("Pi 0.80.9 session preserves partial details and patched error results", async () => {
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
    modelRuntime.setRuntimeApiKey("runtime-test", "test-key");
    const { session } = await createAgentSession({
      cwd: root,
      agentDir: root,
      model: { provider: "runtime-test", id: "runtime-test", api: "openai-completions" } as any,
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
    session.agent.streamFn = () => {
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
