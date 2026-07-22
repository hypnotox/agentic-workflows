#!/usr/bin/env node
const args = process.argv.slice(2);
const protocol = { schemaVersion: 1, protocol: { major: 1, minor: 0 }, compatibleMajor: 1, descriptorSha256: "fixture", awfVersion: "0.23.0", projectVersion: "0.22.0" };
const metrics = { schemaVersion: 1, protocolMajor: 1, generatedAt: "2026-07-22T00:00:00Z", selector: {}, efforts: [], retention: {}, integrity: [] };
const doctor = { schemaVersion: 1, protocolMajor: 1, generatedAt: "2026-07-22T00:00:00Z", selector: {}, findings: [], integrity: [] };
let output;
if (args.join(" ") === "metrics protocol --json") output = protocol;
else if (args[0] === "metrics" && args.at(-1) === "--json") output = metrics;
else if (args[0] === "doctor" && args.at(-1) === "--json") output = doctor;
else process.exitCode = 2;
if (output) process.stdout.write(`${JSON.stringify(output)}\n`);
