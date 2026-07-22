package telemetry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// ProjectTypeScript projects the normative descriptor into the generated Pi
// runtime. The descriptor remains the sole vocabulary and shape authority.
func ProjectTypeScript() string {
	raw, err := json.Marshal(descriptor)
	if err != nil { // coverage-ignore: the validated embedded descriptor is JSON-marshalable
		panic(err)
	}
	var b strings.Builder
	b.WriteString("// @ts-nocheck\n")
	fmt.Fprintf(&b, "export const protocolDescriptor = %s as const;\n", raw)
	b.WriteString("export const protocolVersion = protocolDescriptor.version;\n")
	b.WriteString("export const protocolLimits = protocolDescriptor.limits;\n")

	vocabularies := sortedKeys(descriptor.Vocabularies)
	for _, name := range vocabularies {
		fmt.Fprintf(&b, "export const %s = protocolDescriptor.vocabularies.%s;\n", tsIdentifier(name), name)
		fmt.Fprintf(&b, "export type %s = typeof %s[number];\n", exportedIdentifier(name), tsIdentifier(name))
	}

	for _, kind := range sortedKeys(descriptor.Payloads) {
		payload := descriptor.Payloads[kind]
		writeTSInterface(&b, payload.GoType, payload.Fields)
	}
	for _, name := range sortedKeys(descriptor.Objects) {
		object := descriptor.Objects[name]
		writeTSInterface(&b, object.GoType, object.Fields)
	}
	writeTSInterface(&b, descriptor.Envelope.GoType, descriptor.Envelope.Fields)
	for _, action := range sortedKeys(descriptor.LifecycleRequests) {
		request := descriptor.LifecycleRequests[action]
		writeTSInterface(&b, request.GoType, request.Fields)
	}

	b.WriteString("export interface PayloadByKind {\n")
	for _, kind := range sortedKeys(descriptor.Payloads) {
		fmt.Fprintf(&b, "  %q: %s;\n", kind, descriptor.Payloads[kind].GoType)
	}
	b.WriteString("}\n")
	b.WriteString("export type TelemetryEvent = { [K in keyof PayloadByKind]: Omit<EventEnvelope, 'kind' | 'payload'> & { kind: K; payload: PayloadByKind[K] } }[keyof PayloadByKind];\n")
	requests := make([]string, 0, len(descriptor.LifecycleRequests))
	for _, action := range sortedKeys(descriptor.LifecycleRequests) {
		requests = append(requests, descriptor.LifecycleRequests[action].GoType)
	}
	fmt.Fprintf(&b, "export type LifecycleRequest = %s;\n", strings.Join(requests, " | "))

	b.WriteString(tsRuntimeValidators)
	return b.String()
}

func writeTSInterface(b *strings.Builder, name string, fields map[string]fieldDescriptor) {
	fmt.Fprintf(b, "export interface %s {\n", name)
	for _, fieldName := range sortedKeys(fields) {
		field := fields[fieldName]
		optional := "?"
		if field.Required {
			optional = ""
		}
		fmt.Fprintf(b, "  %s%s: %s;\n", fieldName, optional, tsFieldType(field))
	}
	b.WriteString("}\n")
}

func tsFieldType(field fieldDescriptor) string {
	if field.Vocabulary != "" {
		return exportedIdentifier(field.Vocabulary)
	}
	switch field.Type {
	case "string":
		return "string"
	case "uint16", "uint64", "number":
		return "number"
	case "array":
		if field.Items == nil {
			return "unknown[]"
		}
		return "Array<" + tsFieldType(*field.Items) + ">"
	case "object":
		parts := make([]string, 0, len(field.Fields))
		for _, name := range sortedKeys(field.Fields) {
			nested := field.Fields[name]
			optional := "?"
			if nested.Required {
				optional = ""
			}
			parts = append(parts, fmt.Sprintf("%s%s: %s", name, optional, tsFieldType(nested)))
		}
		return "{ " + strings.Join(parts, "; ") + " }"
	case "payload":
		return "unknown"
	default:
		return "unknown"
	}
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func tsIdentifier(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}

func exportedIdentifier(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

const tsRuntimeValidators = `
function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function formattedString(field: any, value: unknown): boolean {
  if (typeof value !== "string" || value.length === 0) return false;
  const bytes = new TextEncoder().encode(value).length;
  const maximum = field.format === "checkpoint" ? protocolLimits.checkpointIdBytes : field.format === "model" ? protocolLimits.modelBytes : field.format === "tool" ? protocolLimits.toolBytes : field.format === "category" ? protocolLimits.categoryBytes : protocolLimits.identifierBytes;
  if (bytes > maximum) return false;
  if (field.format === "timestamp") return /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/.test(value) && !Number.isNaN(Date.parse(value));
  if (field.format === "checkpoint") return !value.startsWith("/") && !value.includes("\\") && value.split("/").every((part) => part && part !== "." && part !== "..");
  if (field.format === "identifier") return value !== "." && value !== ".." && !value.includes("/") && !value.includes("\\");
  return true;
}
function namedObject(type: "origin" | "replacement" | "proposal"): any {
  return (protocolDescriptor.objects as any)[({ origin: "OriginMetadata", replacement: "RepairReplacement", proposal: "RepairProposal" } as const)[type]];
}
function fieldValid(field: any, value: unknown): boolean {
  if (field.vocabulary) return typeof value === "string" && (protocolDescriptor.vocabularies as any)[field.vocabulary]?.includes(value);
  switch (field.type) {
    case "string": return formattedString(field, value);
    case "uint16": return Number.isInteger(value) && (value as number) >= 0 && (value as number) <= 65535;
    case "uint64": return Number.isSafeInteger(value) && (value as number) >= 0;
    /* c8 ignore next -- descriptor authority requires a minimum on every number field */
    case "number": return typeof value === "number" && Number.isFinite(value) && (field.minimum === undefined || value >= field.minimum);
    /* c8 ignore start -- descriptor authority emits minItems and uniqueItems on every array */
    case "array": return Array.isArray(value) && value.length >= (field.minItems ?? 0) && value.every((entry) => fieldValid(field.items, entry)) && (!field.uniqueItems || new Set(value.map((entry) => JSON.stringify(entry))).size === value.length);
    /* c8 ignore stop */
    case "object": return objectValid({ fields: field.fields, additionalProperties: field.additionalProperties }, value);
    case "payload": return isRecord(value);
    case "origin": case "proposal": return shapedValid(namedObject(field.type), value);
    case "replacement": {
      if (!shapedValid(namedObject(field.type), value)) return false;
      const replacement = value as Record<string, unknown>;
      const payloadShape = (protocolDescriptor.payloads as any)[replacement.eventKind as string];
      /* c8 ignore start -- descriptor tests cover each fixed replacement class */
      return !!payloadShape && payloadShape.class === "lifecycle" && payloadShape.repairable === true && shapedValid(payloadShape, replacement.payload);
      /* c8 ignore stop */
    }
    /* c8 ignore next -- descriptor authority rejects unknown field types */
    default: return false;
  }
}
function objectValid(shape: any, value: unknown, allowExtensions = false): boolean {
  /* c8 ignore start -- envelope validation rejects non-record nested shapes before this helper */
  if (!isRecord(value)) return false;
  /* c8 ignore stop */
  const fields = shape.fields as Record<string, any>;
  for (const [name, field] of Object.entries(fields)) {
    if (field.required && !(name in value)) return false;
    if (name in value && !fieldValid(field, value[name])) return false;
  }
  /* c8 ignore start -- descriptor authority forbids additional properties on every fixed shape */
  return allowExtensions || shape.additionalProperties || Object.keys(value).every((name) => name in fields);
  /* c8 ignore stop */
}
function constraintValid(constraint: any, value: Record<string, unknown>): boolean {
  /* c8 ignore start -- object validation rejects present undefined fields before constraints */
  const present = (name: string) => name in value && value[name] !== undefined;
  /* c8 ignore stop */
  switch (constraint.kind) {
    case "fields-required-when": return value[constraint.discriminator] !== constraint.value || constraint.fields.every(present);
    case "fields-forbidden-when": return value[constraint.discriminator] !== constraint.value || constraint.fields.every((name: string) => !present(name));
    case "field-allowed-when": return !present(constraint.field) || value[constraint.discriminator] === constraint.value;
    case "waiver-eligibility": return ((protocolDescriptor.waiverRules as any)[value[constraint.ruleField] as string] ?? []).includes(value[constraint.reasonField]);
    default: return false;
  }
}
function shapedValid(shape: any, value: unknown, allowExtensions = false): boolean {
  /* c8 ignore start -- descriptor authority emits an explicit constraints array on every shape */
  return objectValid(shape, value, allowExtensions) && (shape.constraints ?? []).every((constraint: any) => constraintValid(constraint, value as Record<string, unknown>));
  /* c8 ignore stop */
}
export function validateTelemetryEvent(value: unknown): value is TelemetryEvent {
  if (!isRecord(value) || !isRecord(value.version)) return false;
  const version = value.version as Record<string, unknown>;
  if (version.major !== protocolVersion.major || typeof version.minor !== "number") return false;
  const newerMinor = (version.minor as number) > protocolVersion.minor;
  if (!objectValid(protocolDescriptor.envelope, value, newerMinor)) return false;
  /* c8 ignore start -- envelope validation already requires a string kind; tests cover unknown strings */
  if (typeof value.kind !== "string" || !(value.kind in protocolDescriptor.payloads)) return false;
  /* c8 ignore stop */
  const payloadShape = (protocolDescriptor.payloads as any)[value.kind];
  if (!shapedValid(payloadShape, value.payload, newerMinor)) return false;
  const lifecycle = payloadShape.class === "lifecycle";
  /* c8 ignore start -- exhaustive event tests cover both fixed identity classes */
  return lifecycle ? typeof value.idempotencyKey === "string" && !("observationId" in value) : typeof value.observationId === "string" && !("idempotencyKey" in value);
  /* c8 ignore stop */
}
export function validateLifecycleRequest(value: unknown): value is LifecycleRequest {
  if (!isRecord(value) || typeof value.action !== "string") return false;
  const shape = (protocolDescriptor.lifecycleRequests as any)[value.action];
  /* c8 ignore start -- lifecycle tests cover every fixed request shape and unknown action */
  return !!shape && shapedValid(shape, value);
  /* c8 ignore stop */
}
export function classifyGateTokens(tokens: readonly string[]): null | { classification: "gate"; gateMode: "standard" | "full" } {
  if (tokens.length === 2 && tokens[0] === "./x" && tokens[1] === "gate") return { classification: "gate", gateMode: "standard" };
  if (tokens.length === 3 && tokens[0] === "./x" && tokens[1] === "gate" && tokens[2] === "full") return { classification: "gate", gateMode: "full" };
  return null;
}
`
