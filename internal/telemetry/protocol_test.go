package telemetry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const testObservationID = "123e4567-e89b-42d3-a456-426614174000"

func testHeaderRaw(id string) []byte {
	return []byte(fmt.Sprintf(`{"record":"header","schemaVersion":1,"sessionId":%q,"createdAt":"2026-07-27T00:00:00Z"}`, id))
}

func testObservationRaw(id, timestamp, kind, payload string) []byte {
	return []byte(fmt.Sprintf(`{"record":"observation","schemaVersion":1,"observationId":%q,"timestamp":%q,"kind":%q,"payload":%s}`, id, timestamp, kind, payload))
}

func mustObservation(t *testing.T, id, timestamp, kind, payload string) Observation {
	t.Helper()
	o, err := ValidateObservation(testObservationRaw(id, timestamp, kind, payload))
	if err != nil {
		t.Fatalf("ValidateObservation(%s): %v", kind, err)
	}
	return o
}

// invariant: tooling/workflow-telemetry:event-protocol-and-ledger
func TestSessionProtocolAcceptsClosedObservation(t *testing.T) {
	raw := testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":0,"outputTokens":0,"cacheReadTokens":0,"cacheWriteTokens":0,"costUsd":0}`)
	o, err := ValidateObservation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(o.Raw, raw) {
		t.Fatalf("raw = %s, want %s", o.Raw, raw)
	}
	raw[0] = 'x'
	if o.Raw[0] != '{' {
		t.Fatal("observation retained caller-owned raw bytes")
	}
}

func TestDescriptorIdentifiersAndTypeScriptProjection(t *testing.T) {
	var got descriptor
	if err := json.Unmarshal(DescriptorBytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Limits.IdentifierBytes != 160 || got.Limits.CategoryBytes != 64 || got.Limits.IntegerMaximum != maxSafeInteger {
		t.Fatalf("descriptor = %#v", got)
	}
	copy := DescriptorBytes()
	copy[0] = 'x'
	if DescriptorBytes()[0] != '{' {
		t.Fatal("DescriptorBytes returned its backing storage")
	}
	for _, value := range []string{"session", strings.Repeat("a", 160)} {
		if !Identifier(value) {
			t.Errorf("Identifier(%q) = false", value)
		}
	}
	for _, value := range []string{"", ".", "..", "a/b", `a\\b`, "a\x00b", "a\rb", "a\nb", string([]byte{0xff}), strings.Repeat("a", 161)} {
		if Identifier(value) {
			t.Errorf("Identifier(%q) = true", value)
		}
	}
	for _, value := range []string{"tool", strings.Repeat("x", 64)} {
		if !category(value) {
			t.Errorf("category(%q) = false", value)
		}
	}
	for _, value := range []string{"", string([]byte{0xff}), strings.Repeat("x", 65)} {
		if category(value) {
			t.Errorf("category(%q) = true", value)
		}
	}
	if validText("", 1) || validText("ab", 1) || validText(string([]byte{0xff}), 2) {
		t.Fatal("validText accepted an invalid boundary")
	}
	ts := ProjectTypeScript()
	for _, want := range []string{"protocolDescriptor", "validateSessionHeader", "validateObservation", `"schemaVersion":1`, "TextEncoder"} {
		if !strings.Contains(ts, want) {
			t.Errorf("TypeScript projection lacks %q", want)
		}
	}
}

func TestDescriptorRejectsInvalidEmbeddedData(t *testing.T) {
	original := descriptorJSON
	t.Cleanup(func() { descriptorJSON = original })
	descriptorJSON = []byte(`{"schemaVersion":0,"limits":{}}`)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid semantic descriptor did not panic")
			}
		}()
		_ = parseDescriptor()
	}()
	descriptorJSON = []byte(`{`)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid JSON descriptor did not panic")
			}
		}()
		_ = ProjectTypeScript()
	}()
}

func TestHeaderAndClosedObjectBoundaries(t *testing.T) {
	h, err := ValidateHeader(testHeaderRaw("session"))
	if err != nil || h.SessionID != "session" || h.Record != "header" {
		t.Fatalf("valid header = %#v, %v", h, err)
	}
	badHeaders := [][]byte{
		[]byte("not json"),
		[]byte(`[]`),
		[]byte(`{"record":"header","schemaVersion":1,"sessionId":"s","createdAt":"2026-07-27T00:00:00Z","extra":true}`),
		[]byte(`{"record":"header","schemaVersion":1,"sessionId":"s"}`),
		[]byte(`{"record":"header","schemaVersion":1,"createdAt":"2026-07-27T00:00:00Z","unknown":true}`),
		[]byte(`{"record":"header","record":"header","schemaVersion":1,"sessionId":"s","createdAt":"2026-07-27T00:00:00Z"}`),
		[]byte(`{"record":"header","schemaVersion":"1","sessionId":"s","createdAt":"2026-07-27T00:00:00Z"}`),
		[]byte(`{"record":"other","schemaVersion":1,"sessionId":"s","createdAt":"2026-07-27T00:00:00Z"}`),
		[]byte(`{"record":"header","schemaVersion":2,"sessionId":"s","createdAt":"2026-07-27T00:00:00Z"}`),
		[]byte(`{"record":"header","schemaVersion":1,"sessionId":"../s","createdAt":"2026-07-27T00:00:00Z"}`),
		[]byte(`{"record":"header","schemaVersion":1,"sessionId":"s","createdAt":"0001-01-01T00:00:00Z"}`),
	}
	for _, raw := range badHeaders {
		if _, err := ValidateHeader(raw); err == nil {
			t.Errorf("ValidateHeader accepted %q", raw)
		}
	}
	for _, raw := range [][]byte{
		{0xff},
		[]byte(`[]`),
		[]byte(`{"a":1,"a":2}`),
		[]byte(`{"a":1,"b":2}`),
		[]byte(`{"a":1} {"a":2}`),
	} {
		if _, err := closedObject(raw, []string{"a"}); err == nil {
			t.Errorf("closedObject accepted %q", raw)
		}
	}
	if got, err := closedObject([]byte(`{"a":1}`), []string{"a"}); err != nil || string(got["a"]) != "1" {
		t.Fatalf("closedObject valid = %v, %v", got, err)
	}
	for _, raw := range [][]byte{
		[]byte(`{"a":{"b":1,"b":2}}`),
		[]byte(`{"a":1} {"b":2}`),
		[]byte(`{"a":`),
		[]byte(`{"a" 1}`),
		[]byte(`{"a":1,`),
		[]byte(`{`),
		[]byte(`[`),
		[]byte(`[1`),
		[]byte(`[{"a":1,"a":2}]`),
	} {
		if err := noDuplicateJSON(raw); err == nil {
			t.Errorf("noDuplicateJSON accepted %q", raw)
		}
	}
	if err := noDuplicateJSON([]byte(`[1,{"a":[2]}]`)); err != nil {
		t.Fatal(err)
	}
}

func TestObservationPayloadKindsAndValidationBoundaries(t *testing.T) {
	payloads := map[string]string{
		"usage":      `{"inputTokens":1,"outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4,"costUsd":1.5}`,
		"tool":       `{"tool":"go test","outcome":"success","durationMs":5}`,
		"gate":       `{"gate":"gate","outcome":"failure","durationMs":6}`,
		"subagent":   `{"role":"implementation","outcome":"cancelled","queueWaitMs":7,"durationMs":8}`,
		"compaction": `{"inputTokensBefore":9,"inputTokensAfter":10}`,
		"handoff":    `{"outcome":"success","durationMs":11}`,
	}
	for kind, payload := range payloads {
		t.Run(kind, func(t *testing.T) {
			o, err := ValidateObservation(testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", kind, payload))
			if err != nil || o.Kind != kind {
				t.Fatalf("%s = %#v, %v", kind, o, err)
			}
		})
	}
	bad := []struct {
		name string
		raw  []byte
	}{
		{"non-object", []byte(`[]`)},
		{"unknown-envelope-field", []byte(`{"record":"observation","schemaVersion":1,"observationId":"123e4567-e89b-42d3-a456-426614174000","timestamp":"2026-07-27T00:00:00Z","kind":"usage","payload":{},"extra":true}`)},
		{"bad-envelope-types", []byte(`{"record":"observation","schemaVersion":"1","observationId":"123e4567-e89b-42d3-a456-426614174000","timestamp":"2026-07-27T00:00:00Z","kind":"usage","payload":{}}`)},
		{"bad-record", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "unknown", `{}`)},
		{"bad-observation-id", testObservationRaw("123E4567-e89b-42d3-a456-426614174000", "2026-07-27T00:00:00Z", "usage", payloads["usage"])},
		{"bad-timestamp", testObservationRaw(testObservationID, "not-a-time", "usage", payloads["usage"])},
		{"unknown-kind", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "other", `{}`)},
		{"usage-closed", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":1}`)},
		{"usage-integer", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":-1,"outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4,"costUsd":0}`)},
		{"usage-non-integer", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":"one","outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4,"costUsd":0}`)},
		{"usage-cost", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "usage", `{"inputTokens":1,"outputTokens":2,"cacheReadTokens":3,"cacheWriteTokens":4,"costUsd":"free"}`)},
		{"tool-category", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "tool", `{"tool":"","outcome":"success","durationMs":1}`)},
		{"tool-outcome", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "tool", `{"tool":"go","outcome":"other","durationMs":1}`)},
		{"tool-duration", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "tool", `{"tool":"go","outcome":"success","durationMs":1.5}`)},
		{"gate-category", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "gate", `{"gate":"","outcome":"success","durationMs":1}`)},
		{"subagent-role", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "subagent", `{"role":"other","outcome":"success","queueWaitMs":1,"durationMs":1}`)},
		{"subagent-outcome", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "subagent", `{"role":"grounding","outcome":"other","queueWaitMs":1,"durationMs":1}`)},
		{"subagent-integer", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "subagent", `{"role":"grounding","outcome":"success","queueWaitMs":9007199254740992,"durationMs":1}`)},
		{"compaction-integer", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "compaction", `{"inputTokensBefore":1,"inputTokensAfter":-1}`)},
		{"handoff-outcome", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "handoff", `{"outcome":"other","durationMs":1}`)},
		{"handoff-duration", testObservationRaw(testObservationID, "2026-07-27T00:00:00Z", "handoff", `{"outcome":"success","durationMs":-1}`)},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateObservation(tc.raw); err == nil {
				t.Fatalf("ValidateObservation accepted %s", tc.raw)
			}
		})
	}
}

// invariant: tooling/init-and-enablement:init-hooks-default-on
