#!/usr/bin/env node
const event = {
  type: "message_end",
  message: {
    role: "assistant",
    content: [{ type: "text", text: "fixture output" }],
    model: "fixture-model",
    stopReason: "end",
    usage: { input: 1, output: 1, cacheRead: 0, cacheWrite: 0, cost: { total: 0 } },
  },
};
process.stdout.write(`${JSON.stringify(event)}\n`);
