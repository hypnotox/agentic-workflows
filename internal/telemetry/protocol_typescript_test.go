package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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
		"export interface PhaseTransitionedPayload",
		"export interface TransitionPhaseLifecycleRequest",
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

func TestProjectedTypeScriptEmbedsCompleteProtocol2Descriptor(t *testing.T) {
	projected := ProjectTypeScript()
	const prefix = "export const protocolDescriptor = "
	start := strings.Index(projected, prefix)
	end := strings.Index(projected[start:], " as const;\n")
	if start < 0 || end < 0 {
		t.Fatal("projected TypeScript descriptor declaration is absent")
	}
	generatedJSON := projected[start+len(prefix) : start+end]
	var generated, normative any
	if err := json.Unmarshal([]byte(generatedJSON), &generated); err != nil {
		t.Fatalf("projected descriptor is not JSON: %v", err)
	}
	if err := json.Unmarshal(DescriptorBytes(), &normative); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(generated, normative) {
		t.Fatal("projected TypeScript descriptor differs from normative JSON")
	}
	object := generated.(map[string]any)
	version := object["version"].(map[string]any)
	if version["major"] != float64(2) || version["minor"] != float64(0) {
		t.Fatalf("projected protocol version = %#v", version)
	}
	encoded, _ := json.Marshal(generated)
	if strings.Contains(string(encoded), "checkpoint"+"Id") || strings.Contains(projected, "format === \"check"+"point\"") {
		t.Fatal("projected TypeScript retains protocol-1 checkpoint contract")
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
	for _, state := range fixture.Recoveries {
		switch state {
		case "orphan-staging":
			ledger, _, _ := createTestEffort(t)
			path := filepath.Join(ledger.paths.staging, fixture.StagingName)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			old := time.Now().Add(-2 * time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			report, err := ledger.Recover()
			if err != nil || ledger.pathExists(path) || !containsString(report.Recovered, fixture.EffortID) {
				t.Fatalf("Go orphan-staging outcome = %#v, %v", report, err)
			}
		case "pending-effort":
			ledger, id, _ := tombstoneRecoveryLedger(t, "pending", true, false)
			report, err := ledger.Recover()
			if err != nil || ledger.pathExists(ledger.paths.tombstone(id)) || !containsString(report.Recovered, id) {
				t.Fatalf("Go pending-effort outcome = %#v, %v", report, err)
			}
		case "pending-trash", "committed-trash":
			status := "pending"
			if state == "committed-trash" {
				status = "committed"
			}
			ledger, id, trash := tombstoneRecoveryLedger(t, status, false, true)
			report, err := ledger.Recover()
			if err != nil || ledger.pathExists(trash) || !containsString(report.Recovered, id) {
				t.Fatalf("Go %s outcome = %#v, %v", state, report, err)
			}
		default:
			t.Fatalf("unknown shared recovery state %q", state)
		}
	}
	for _, state := range fixture.Ambiguous {
		ledger, _, _ := createTestEffort(t)
		switch state {
		case "active-staging":
			path := filepath.Join(ledger.paths.staging, fixture.StagingName)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			report, err := ledger.Recover()
			if err != nil || !ledger.pathExists(path) || len(report.Ambiguous) != 0 {
				t.Fatalf("Go active-staging preservation = %#v, %v", report, err)
			}
		case "nonce-mismatch":
			ledger, id, _ := tombstoneRecoveryLedger(t, "pending", false, true)
			raw, _ := json.Marshal(tombstoneRecord{Nonce: "different", State: "pending"})
			if err := os.WriteFile(ledger.paths.tombstone(id), raw, 0o600); err != nil {
				t.Fatal(err)
			}
			report, err := ledger.Recover()
			if err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-prune-state") {
				t.Fatalf("Go nonce-mismatch outcome = %#v, %v", report, err)
			}
		case "trash-without-tombstone":
			path := filepath.Join(ledger.paths.trash, fixture.StagingName)
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
			report, err := ledger.Recover()
			if err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-trash-path") {
				t.Fatalf("Go trash-without-tombstone outcome = %#v, %v", report, err)
			}
		default:
			t.Fatalf("unknown shared ambiguous state %q", state)
		}
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
