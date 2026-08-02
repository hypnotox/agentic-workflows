import assert from "node:assert/strict";
import test from "node:test";
import { Value } from "typebox/value";
import handoffDefault, {
  guardMinimumRuntime,
  registerHandoff,
  versionSupported,
} from "../../../.pi/extensions/awf-handoff/index.ts";

function make(options: any = {}) {
  const tools = new Map<string, any>();
  const commands = new Map<string, any>();
  const hooks = new Map<string, any>();
  const queued: any[] = [];
  const notice: any[] = [];
  const editor: string[] = [];
  const sessions: any[] = [];
  const entries: any[] = [];
  const components: any[] = [];
  const intervals: any[] = [];
  const timeouts: any[] = [];
  const clearedIntervals: any[] = [];
  const clearedTimeouts: any[] = [];
  const renderRequests: any[] = [];
  const sends: string[] = [];
  let component: any;
  let done: any;
  let queueFails = Boolean(options.queueFail);
  let sendFails = false;
  let sessionFile: any = options.noFile ? undefined : "old";
  let leaf: any = {
    type: "message",
    message: {
      role: "assistant",
      content: [{ type: "toolCall", id: "id", name: "handoff_session" }],
    },
  };
  const pi: any = {
    on: (name: string, fn: any) => hooks.set(name, fn),
    registerTool: (tool: any) => tools.set(tool.name, tool),
    registerCommand: (name: string, command: any) => commands.set(name, command),
    queueCommand: (name: string, id: string) => {
      if (queueFails) throw Error("queue");
      queued.push([name, id]);
    },
    events: { emit() {} },
  };
  const deps: any = {
    packageVersion: "0.81.1",
    randomUUID: () => "id",
    setInterval: (fn: any, milliseconds: number) => {
      if (options.intervalFail) throw Error("interval");
      const handle = { fn, milliseconds };
      intervals.push(handle);
      return handle;
    },
    clearInterval: (handle: any) => clearedIntervals.push(handle),
    setTimeout: (fn: any, milliseconds: number) => {
      if (options.timeoutFail) throw Error("timeout");
      const handle = { fn, milliseconds };
      timeouts.push(handle);
      return handle;
    },
    clearTimeout: (handle: any) => clearedTimeouts.push(handle),
  };
  registerHandoff(pi, deps);
  const ui: any = {
    notify: (...args: any[]) => notice.push(args),
    setEditorText: (value: string) => editor.push(value),
    custom: async (factory: any) =>
      new Promise((resolve) => {
        done = resolve;
        component = factory(
          { requestRender: () => renderRequests.push("render") },
          {},
          {
            matches: (data: string) => {
              if (options.keyFail) throw Error("key");
              return data === "escape";
            },
          },
          resolve,
        );
        component.complete = resolve;
        components.push(component);
      }),
  };
  const ctx: any = {
    mode: options.mode ?? "tui",
    ui,
    sessionManager: {
      getSessionFile: () => sessionFile,
      getLeafEntry: () => leaf,
    },
    newSession: async (request: any) => {
      sessions.push(request);
      const manager = {
        cleanup: async () => {
          entries.push(["cleanup"]);
          if (options.cleanupFail) throw Error("cleanup");
        },
      };
      await request.setup(manager);
      if (options.newFail) throw Error("new");
      await request.withSession({
        ui,
        sendUserMessage: async (value: string) => {
          sends.push(value);
          if (sendFails) throw Error("send");
          editor.push("sent:" + value);
        },
      });
    },
  };
  return {
    tools,
    commands,
    hooks,
    queued,
    notice,
    editor,
    sessions,
    entries,
    components,
    intervals,
    timeouts,
    clearedIntervals,
    clearedTimeouts,
    renderRequests,
    sends,
    ctx,
    deps,
    get component() {
      return component;
    },
    complete: (result = true) => setImmediate(() => done(result)),
    cancel: () => component.handleInput("escape"),
    sendFails: () => {
      sendFails = true;
    },
    queueWorks: () => {
      queueFails = false;
    },
    setLeaf: (value: any) => {
      leaf = value;
    },
    dropFile: () => {
      sessionFile = undefined;
    },
  };
}

async function execute(h: any, params: any = { kickoff: "go" }) {
  return h.tools
    .get("handoff_session")
    .execute("id", params, undefined, undefined, h.ctx);
}

function continueHandoff(h: any, token = "id") {
  return h.commands.get("awf-handoff-continue").handler(token, h.ctx);
}

test("handoff schema exposes only required bounded kickoff prose", async () => {
  const h = make();
  const schema = h.tools.get("handoff_session").parameters;
  assert.deepEqual(Object.keys(schema.properties), ["kickoff"]);
  assert.deepEqual(schema.required, ["kickoff"]);
  assert.equal(schema.additionalProperties, false);
  assert.equal(schema.properties.kickoff.maxLength, 1000);
  assert.equal(Value.Check(schema, {}), false);
  assert.equal(Value.Check(schema, { kickoff: "go", extra: true }), false);
  assert.equal(Value.Check(schema, { kickoff: "x".repeat(1000) }), true);
  assert.equal(Value.Check(schema, { kickoff: "x".repeat(1001) }), false);
  for (const value of [
    { kickoff: 1 },
    { kickoff: "" },
    { kickoff: "  " },
    { kickoff: "x".repeat(1001) },
  ]) {
    await assert.rejects(execute(h, value), /kickoff/);
  }
  await execute(make(), { kickoff: "😀".repeat(500) });
  await assert.rejects(execute(make(), { kickoff: "😀".repeat(501) }), /1000/);
});

test("handoff preserves exact kickoff through submission and editor fallback", async () => {
  const kickoff = "  keep this exact prose  ";
  const h = make();
  const result = await execute(h, { kickoff });
  assert.deepEqual(result.details, { kickoff });
  assert.equal(result.terminate, true);
  assert.deepEqual(h.queued, [["awf-handoff-continue", "id"]]);
  const pending = continueHandoff(h);
  h.complete();
  await pending;
  assert.deepEqual(h.sends, [kickoff]);
  assert.deepEqual(h.editor, ["sent:" + kickoff]);
  assert.equal(h.sessions.length, 1);

  const fallback = make();
  fallback.sendFails();
  await execute(fallback, { kickoff });
  const fallbackPending = continueHandoff(fallback);
  fallback.complete();
  await fallbackPending;
  assert.deepEqual(fallback.sends, [kickoff]);
  assert.deepEqual(fallback.editor, [kickoff]);
  assert.deepEqual(fallback.notice, [
    ["Automatic kickoff failed; submit the prepared editor text.", "warning"],
  ]);
  assert.equal(fallback.sessions.length, 1);
});

test("handoff preserves exact kickoff through replacement failure recovery", async () => {
  const kickoff = "  recover exactly  ";
  const h = make({ newFail: true });
  await execute(h, { kickoff });
  const pending = continueHandoff(h);
  h.complete();
  await assert.rejects(pending, /new/);
  assert.deepEqual(h.entries, [["cleanup"]]);
  assert.deepEqual(h.editor, [kickoff]);
  assert.deepEqual(h.notice, [
    ["Fresh-session handoff failed; recovery text is in the editor.", "error"],
  ]);
  assert.equal(h.sessions.length, 1);
  assert.deepEqual(h.sends, []);
});

test("handoff preflight exclusively blocks mixed and unverifiable batches", () => {
  const h = make();
  const preflight = h.hooks.get("tool_call");
  assert.equal(
    preflight({ toolCallId: "id", toolName: "handoff_session" }, h.ctx),
    undefined,
  );
  h.setLeaf({
    type: "message",
    message: {
      role: "assistant",
      content: [
        { type: "toolCall", id: "id", name: "handoff_session" },
        { type: "toolCall", id: "x", name: "read" },
      ],
    },
  });
  assert.match(preflight({ toolCallId: "x", toolName: "read" }, h.ctx).reason, /siblings/);
  h.setLeaf(undefined);
  assert.match(
    preflight({ toolCallId: "id", toolName: "handoff_session" }, h.ctx).reason,
    /Cannot verify/,
  );
  assert.equal(preflight({ toolCallId: "x", toolName: "read" }, h.ctx), undefined);
});

test("handoff countdown advances for five seconds and disposes deterministically", async () => {
  const h = make();
  await execute(h);
  const pending = continueHandoff(h);
  assert.equal(h.intervals[0].milliseconds, 1000);
  assert.equal(h.timeouts[0].milliseconds, 5000);
  assert.match(h.component.render(200)[0], /5s/);
  assert.equal(h.component.render(200), h.component.render(200));
  assert.equal(h.component.render(0)[0].length > 0, true);
  h.intervals[0].fn();
  assert.deepEqual(h.renderRequests, ["render"]);
  assert.match(h.component.render(200)[0], /4s/);
  h.component.handleInput("not-cancel");
  h.timeouts[0].fn();
  h.timeouts[0].fn();
  await pending;
  h.component.invalidate();
  h.component.dispose();
  assert.equal(h.clearedIntervals.length >= 2, true);
  assert.equal(h.clearedTimeouts.length >= 2, true);
  assert.equal(h.sessions.length, 1);
});

test("handoff cancellation clears pending and never replaces the session", async () => {
  const h = make();
  await execute(h);
  const pending = continueHandoff(h);
  h.cancel();
  await pending;
  assert.deepEqual(h.notice, [["Fresh-session handoff canceled."]]);
  assert.equal(h.sessions.length, 0);
  await execute(h);
});

test("wrong continuation token preserves the valid pending request", async () => {
  const h = make();
  await execute(h);
  await assert.rejects(continueHandoff(h, "wrong"), /matching/);
  await assert.rejects(execute(h), /already pending/);
  const pending = continueHandoff(h);
  h.complete();
  await pending;
  assert.equal(h.sessions.length, 1);
});

test("handoff rejects a continuation whose pending request changes during countdown", async () => {
  const h = make();
  await execute(h);
  const first = continueHandoff(h);
  const second = continueHandoff(h);
  h.components[1].complete(true);
  await second;
  h.components[0].complete(true);
  await assert.rejects(first, /matching pending/);
});

test("handoff independently rejects unsupported mode and absent persistence", async () => {
  await assert.rejects(execute(make({ mode: "rpc" })), /persisted interactive/);
  await assert.rejects(execute(make({ noFile: true })), /persisted interactive/);
});

test("handoff revalidates persisted session only after countdown", async () => {
  const h = make();
  await execute(h);
  h.dropFile();
  const pending = continueHandoff(h);
  h.complete();
  await assert.rejects(pending, /no longer persisted/);
  assert.deepEqual(h.editor, ["go"]);
  assert.deepEqual(h.notice, [
    ["Fresh-session handoff failed; recovery text is in the editor.", "error"],
  ]);
  assert.equal(h.sessions.length, 0);
});

test("handoff preserves lineage and does not silently retry", async () => {
  const success = make();
  await execute(success);
  const successPending = continueHandoff(success);
  success.complete();
  await successPending;
  assert.equal(success.sessions[0].parentSession, "old");
  assert.equal(success.sessions.length, 1);
  assert.deepEqual(success.sends, ["go"]);

  const failure = make({ newFail: true });
  await execute(failure);
  const failedPending = continueHandoff(failure);
  failure.complete();
  await assert.rejects(failedPending, /new/);
  assert.equal(failure.sessions.length, 1);
  assert.deepEqual(failure.sends, []);
});

test("queue failure clears pending so a later request can succeed", async () => {
  const h = make({ queueFail: true });
  await assert.rejects(execute(h), /queue/);
  h.queueWorks();
  await execute(h);
  assert.deepEqual(h.queued, [["awf-handoff-continue", "id"]]);
});

test("countdown timer, key, and cleanup faults retain the original boundary", async () => {
  const intervalFailure = make({ intervalFail: true });
  await execute(intervalFailure);
  await continueHandoff(intervalFailure);
  assert.deepEqual(intervalFailure.notice, [["Fresh-session handoff canceled."]]);
  assert.equal(intervalFailure.sessions.length, 0);

  const timeoutFailure = make({ timeoutFail: true });
  await execute(timeoutFailure);
  await continueHandoff(timeoutFailure);
  assert.deepEqual(timeoutFailure.notice, [["Fresh-session handoff canceled."]]);
  assert.equal(timeoutFailure.sessions.length, 0);
  assert.equal(timeoutFailure.clearedIntervals.length, 1);

  const keyFailure = make({ keyFail: true });
  await execute(keyFailure);
  const keyPending = continueHandoff(keyFailure);
  assert.throws(() => keyFailure.component.handleInput("escape"), /key/);
  await keyPending;
  assert.deepEqual(keyFailure.notice, [["Fresh-session handoff canceled."]]);

  const cleanupFailure = make({ newFail: true, cleanupFail: true });
  await execute(cleanupFailure);
  const cleanupPending = continueHandoff(cleanupFailure);
  cleanupFailure.complete();
  await assert.rejects(cleanupPending, /new/);
  assert.deepEqual(cleanupFailure.entries, [["cleanup"]]);
  assert.equal(cleanupFailure.sessions.length, 1);
});

test("handoff exercises runtime guard and generated entrypoint", async () => {
  (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")] = undefined;
  let noticeHandler: any;
  const old: any = {
    on: (name: string, handler: any) => {
      if (name === "session_start") noticeHandler = handler;
    },
  };
  registerHandoff(old, {
    ...make().deps,
    packageVersion: "0.80.0",
    eventsEmit: undefined,
  });
  await noticeHandler({}, {
    ui: { notify: (...args: any[]) => assert.equal(args[1], "error") },
  });
  await noticeHandler({}, {
    ui: { notify: () => assert.fail("duplicate notice") },
  });
  assert.equal(versionSupported("0.81.1"), true);
  assert.equal(versionSupported("bad"), false);
  assert.equal(guardMinimumRuntime({} as any, { packageVersion: "bad" }, []), false);
  (globalThis as any)[Symbol.for("awf.pi.minimum-runtime-notified")] = undefined;
  let versionOnly: any;
  assert.equal(
    guardMinimumRuntime(
      { on: (_name: string, handler: any) => (versionOnly = handler) } as any,
      { packageVersion: "0.80.0" },
      [],
    ),
    false,
  );
  await versionOnly({}, {
    ui: {
      notify: (message: string) => assert.equal(message.includes("Missing runtime APIs"), false),
    },
  });
  await handoffDefault({
    on: () => {},
    registerTool() {},
    registerCommand() {},
    queueCommand() {},
  } as any);
});

test("handoff accepts optional child APIs", async () => {
  const child = make();
  child.ctx.newSession = async (request: any) => {
    await request.setup({});
  };
  await execute(child);
  const pending = continueHandoff(child);
  child.complete();
  await pending;
});
