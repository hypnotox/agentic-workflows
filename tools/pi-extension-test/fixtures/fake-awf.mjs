#!/usr/bin/env node
const args = process.argv.slice(2);
const protocol = { schemaVersion: 1, protocol: { major: 2, minor: 0 }, compatibleMajor: 2, descriptorSha256: "fixture", awfVersion: "0.23.0", projectVersion: "0.22.0" };
const metrics = { schemaVersion: 1, protocolMajor: 2, generatedAt: "2026-07-22T00:00:00Z", selector: {}, efforts: [], retention: {}, integrity: [] };
const doctor = { schemaVersion: 1, protocolMajor: 2, generatedAt: "2026-07-22T00:00:00Z", selector: {}, findings: [], integrity: [] };
function orderedSelectors(values) {
  const order = new Map([["--effort", 0], ["--session", 1], ["--phase", 2], ["--since", 3], ["--until", 4]]);
  let prior = -1;
  while (values.length > 0) {
    const current = order.get(values[0]);
    if (current === undefined || current <= prior || values.length < 2 || values[1].startsWith("-")) return false;
    prior = current;
    values = values.slice(2);
  }
  return true;
}
let output;
if (args.join(" ") === "metrics protocol --json") output = protocol;
else if (args[0] === "metrics" && args[1] === "--json" && orderedSelectors(args.slice(2))) output = metrics;
else if (args[0] === "doctor" && args[1] === "--json" && orderedSelectors(args.slice(2))) output = doctor;
else process.exitCode = 2;
if (output) process.stdout.write(`${JSON.stringify(output)}\n`);
