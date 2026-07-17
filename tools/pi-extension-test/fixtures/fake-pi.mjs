#!/usr/bin/env node
const events = [
  { type: "tool_execution_start", toolCallId: "call-1", toolName: "read", args: {} },
  { type: "tool_execution_end", toolCallId: "call-1", toolName: "read", isError: false },
  {
    type: "message_end",
    message: {
      role: "assistant",
      content: [{ type: "text", text: "fixture output" }],
      model: "fixture-model",
      stopReason: "end",
      usage: { input: 1, output: 1, cacheRead: 0, cacheWrite: 0, cost: { total: 0 } },
    },
  },
];
for (const event of events) process.stdout.write(`${JSON.stringify(event)}\n`);
