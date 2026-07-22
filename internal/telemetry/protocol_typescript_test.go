package telemetry

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestProjectTypeScriptDerivesProtocolContract(t *testing.T) {
	got := ProjectTypeScript()
	if !strings.HasPrefix(got, "// @ts-nocheck\n") {
		t.Fatalf("TypeScript prefix = %q", got[:min(len(got), 40)])
	}
	for _, want := range []string{
		"export const protocolDescriptor = ",
		"export interface EventEnvelope",
		"export interface UsageObservedPayload",
		"export type LifecycleRequest = ",
		"export function validateTelemetryEvent",
		"export function classifyGateTokens",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("projected TypeScript missing %q", want)
		}
	}
	for vocabulary, values := range descriptor.Vocabularies {
		if !strings.Contains(got, "export const "+tsIdentifier(vocabulary)+" = protocolDescriptor.vocabularies."+vocabulary) {
			t.Errorf("missing descriptor-derived vocabulary %s (%v)", vocabulary, values)
		}
	}
	if got != ProjectTypeScript() {
		t.Fatal("TypeScript projection is nondeterministic")
	}
}

func TestCrossLanguageRecoveryFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/recovery-cross-language.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		EffortID, Nonce, StagingName string
		Recoveries, Ambiguous        []string
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if got := stagingName(fixture.EffortID, fixture.Nonce); got != fixture.StagingName {
		t.Fatalf("Go staging name = %q, shared fixture %q", got, fixture.StagingName)
	}
	if effortID, nonce, err := parseStagingName(fixture.StagingName); err != nil || effortID != fixture.EffortID || nonce != fixture.Nonce {
		t.Fatalf("Go recovery fixture parse = %q %q %v", effortID, nonce, err)
	}
	if len(fixture.Recoveries) != 4 || len(fixture.Ambiguous) != 3 {
		t.Fatalf("shared recovery table incomplete: %#v", fixture)
	}
}

func TestTypeScriptIdentifierProjection(t *testing.T) {
	if tsIdentifier("") != "" || exportedIdentifier("") != "" || tsIdentifier("Routes") != "routes" || exportedIdentifier("routes") != "Routes" {
		t.Fatal("identifier projection changed")
	}
	if got := tsFieldType(fieldDescriptor{Type: "array"}); got != "unknown[]" {
		t.Fatalf("nil-item array type = %q", got)
	}
	if got := tsFieldType(fieldDescriptor{Type: "unsupported"}); got != "unknown" {
		t.Fatalf("unsupported field type = %q", got)
	}
}
