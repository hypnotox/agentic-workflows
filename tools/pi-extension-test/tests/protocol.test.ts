import assert from "node:assert/strict";
import test from "node:test";
import {
  classifyGateTokens,
  protocolDescriptor,
  protocolLimits,
  protocolVersion,
  validateLifecycleRequest,
  validateTelemetryEvent,
} from "../../../.pi/extensions/awf-dashboard/protocol.ts";

function valueFor(field: any): any {
  if (field.vocabulary) return (protocolDescriptor.vocabularies as any)[field.vocabulary][0];
  if (field.type === "string") return field.format === "timestamp" ? "2026-07-22T00:00:00Z" : field.format === "checkpoint" ? ".awf/memory/work.md" : "x";
  if (field.type === "uint16" || field.type === "uint64" || field.type === "number") return 0;
  if (field.type === "array") return Array.from({ length: field.minItems ?? 0 }, () => valueFor(field.items));
  if (field.type === "object") return shapeValue({ fields: field.fields });
  if (field.type === "payload") return {};
  const objectName = field.type === "origin" ? "OriginMetadata" : field.type === "replacement" ? "RepairReplacement" : "RepairProposal";
  return shapeValue((protocolDescriptor.objects as any)[objectName]);
}
function shapeValue(shape: any): any {
  const value: any = {};
  for (const [name, field] of Object.entries(shape.fields) as any) if (field.required) value[name] = valueFor(field);
  return value;
}
function validPayload(kind: string): any {
  const shape: any = (protocolDescriptor.payloads as any)[kind]; const value = shapeValue(shape);
  if (kind === "effort_created") value.creationMode = "independent";
  if (kind === "finding_waived") { value.ruleCode = "WFV1-PHASE-ORDER"; value.reasonCode = "approved-route-deviation"; }
  if (kind === "repair_applied") { value.proposalKind = "supersede-event"; value.replacement = { eventKind: "session_detached", payload: { reason: "manual" } }; }
  return value;
}
function validEvent(kind: string): any {
  const lifecycle = (protocolDescriptor.payloads as any)[kind].class === "lifecycle";
  return { version: { ...protocolVersion }, eventId: `event-${kind}`, [lifecycle ? "idempotencyKey" : "observationId"]: `identity-${kind}`, effortId: "effort", sessionId: "session", timestamp: "2026-07-22T00:00:00Z", kind, predecessors: [], payload: validPayload(kind) };
}

test("descriptor validators accept every closed event and lifecycle shape", () => {
  for (const kind of protocolDescriptor.vocabularies.eventKinds) assert.equal(validateTelemetryEvent(validEvent(kind)), true, kind);
  for (const [action, shape] of Object.entries(protocolDescriptor.lifecycleRequests) as any) {
    const request = shapeValue(shape); request.action = action;
    if (action === "create") request.creationMode = "independent";
    if (action === "waive") { request.ruleCode = "WFV1-PHASE-ORDER"; request.reasonCode = "approved-route-deviation"; }
    if (action === "repair") request.proposal = { kind: "supersede-event", sourceEventIds: ["source"], replacement: { eventKind: "session_detached", payload: { reason: "manual" } } };
    assert.equal(validateLifecycleRequest(request), true, action);
  }
});

test("event validator rejects malformed envelopes, fields, constraints, and identities", () => {
  assert.equal(validateTelemetryEvent(null), false); assert.equal(validateTelemetryEvent({ version: null }), false);
  const base = validEvent("effort_created");
  for (const mutation of [
    { version: { major: 2, minor: 0 } }, { version: { major: 1, minor: "0" } }, { extra: true }, { kind: 1 }, { kind: "future" }, { payload: null }, { payload: { ...base.payload, extra: true } },
    { eventId: "" }, { eventId: "a".repeat(protocolLimits.identifierBytes + 1) }, { eventId: "a/b" }, { timestamp: "bad" }, { predecessors: ["x", "x"] }, { idempotencyKey: undefined }, { observationId: "both" },
  ]) assert.equal(validateTelemetryEvent({ ...base, ...mutation }), false, JSON.stringify(mutation));
  const passive = validEvent("tool_observed"); delete passive.observationId; assert.equal(validateTelemetryEvent(passive), false); passive.observationId = "o"; passive.idempotencyKey = "i"; assert.equal(validateTelemetryEvent(passive), false);
  const derived = validEvent("effort_created"); derived.payload.creationMode = "derived"; assert.equal(validateTelemetryEvent(derived), false); Object.assign(derived.payload, { originEffortId: "e", originTrajectoryId: "t", originAnchorId: "a" }); assert.equal(validateTelemetryEvent(derived), true);
  const independent = validEvent("effort_created"); independent.payload.originEffortId = "e"; assert.equal(validateTelemetryEvent(independent), false);
  const shell = validEvent("shell_observed"); shell.payload.gateMode = "full"; assert.equal(validateTelemetryEvent(shell), true); shell.payload.classification = "unclassified"; assert.equal(validateTelemetryEvent(shell), false);
  const waiver = validEvent("finding_waived"); waiver.payload.reasonCode = "approved-clock-skew"; assert.equal(validateTelemetryEvent(waiver), false);
  const repair = validEvent("repair_applied"); repair.payload.replacement = { eventKind: "tool_observed", payload: validPayload("tool_observed") }; assert.equal(validateTelemetryEvent(repair), false); repair.payload.replacement = { eventKind: "session_detached", payload: {} }; assert.equal(validateTelemetryEvent(repair), false);
  const newer = { ...base, version: { major: 1, minor: 1 }, extension: { safe: true }, payload: { ...base.payload, extension: 1 } }; assert.equal(validateTelemetryEvent(newer), true);
  const constraints = (protocolDescriptor.payloads.effort_created as any).constraints; constraints.push({ kind: "future-constraint" }); assert.equal(validateTelemetryEvent(base), false); constraints.pop();
});

test("field type branches and lifecycle constraints reject invalid values", () => {
  const usage = validEvent("usage_observed"); for (const value of [-1, Number.NaN, Number.POSITIVE_INFINITY]) { usage.payload.costUsd = value; assert.equal(validateTelemetryEvent(usage), false); } usage.payload.costUsd = 0;
  usage.payload.inputTokens = -1; assert.equal(validateTelemetryEvent(usage), false); usage.payload.inputTokens = Number.MAX_SAFE_INTEGER + 1; assert.equal(validateTelemetryEvent(usage), false);
  const version = validEvent("usage_observed"); version.version.minor = 65536; assert.equal(validateTelemetryEvent(version), false); version.version.minor = -1; assert.equal(validateTelemetryEvent(version), false);
  const unknownReplacement = validEvent("repair_applied"); unknownReplacement.payload.replacement = { eventKind: "future", payload: {} }; assert.equal(validateTelemetryEvent(unknownReplacement), false); unknownReplacement.payload.replacement = { eventKind: "repair_applied", payload: validPayload("repair_applied") }; assert.equal(validateTelemetryEvent(unknownReplacement), false);
  const noWaiver = validEvent("finding_waived"); noWaiver.payload.ruleCode = "WFV1-EVENT-INTEGRITY"; assert.equal(validateTelemetryEvent(noWaiver), false);
  const optionalUndefined = validEvent("usage_observed"); optionalUndefined.trajectoryId = undefined; assert.equal(validateTelemetryEvent(optionalUndefined), false);
  const passiveNewer = validEvent("usage_observed"); passiveNewer.version.minor = 1; (passiveNewer as any).extension = true; passiveNewer.payload.extension = true; assert.equal(validateTelemetryEvent(passiveNewer), true);
  assert.equal(validateLifecycleRequest(null), false); assert.equal(validateLifecycleRequest({ action: 1 }), false); assert.equal(validateLifecycleRequest({ action: "future" }), false);
  const badTimestamp = validEvent("usage_observed"); badTimestamp.timestamp = "July 22 2026"; assert.equal(validateTelemetryEvent(badTimestamp), false);
  const create: any = shapeValue(protocolDescriptor.lifecycleRequests.create); create.action = "create"; create.creationMode = "derived"; assert.equal(validateLifecycleRequest(create), false); create.origin = { effortId: "e", trajectoryId: "t", anchorId: "a" }; assert.equal(validateLifecycleRequest(create), true); create.extra = true; assert.equal(validateLifecycleRequest(create), false);
  create.creationMode = "independent"; assert.equal(validateLifecycleRequest(create), false); delete create.origin; create.checkpointId = "/absolute"; assert.equal(validateLifecycleRequest(create), false); create.checkpointId = "a/../b"; assert.equal(validateLifecycleRequest(create), false);
});

test("projected gate classifier persists only exact verified token shapes", () => {
  assert.deepEqual(classifyGateTokens(["./x", "gate"]), { classification: "gate", gateMode: "standard" });
  assert.deepEqual(classifyGateTokens(["./x", "gate", "full"]), { classification: "gate", gateMode: "full" });
  for (const tokens of [[], ["x", "gate"], ["./x"], ["./x", "gate", "--flag"], ["./x", "gate", "full", "extra"]]) assert.equal(classifyGateTokens(tokens), null);
});
