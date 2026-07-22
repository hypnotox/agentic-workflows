package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestReaderReportsCorruptionDeduplicatesAndRetainsEvidence(t *testing.T) {
	t.Parallel()
	ledger, metadata, first := createTestEffort(t)
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	identical := append(append([]byte(nil), first...), '\n')
	broken := passiveEvent(t, "broken-event", "broken-observation", metadata.EffortID, []string{"missing-event"})
	unsupportedMap := validEvent("usage_observed", 0)
	unsupportedMap["eventId"] = "unsupported-event"
	unsupportedMap["observationId"] = "unsupported-observation"
	unsupportedMap["version"] = map[string]any{"major": 99, "minor": 0}
	unsupported := mustJSON(t, unsupportedMap)
	contents := append([]byte(nil), first...)
	contents = append(contents, '\n')
	contents = append(contents, identical...)
	contents = append(contents, []byte("{malformed}\n")...)
	contents = append(contents, broken...)
	contents = append(contents, '\n')
	contents = append(contents, unsupported...)
	contents = append(contents, '\n')
	contents = append(contents, []byte(`{"eventId":"partial"`)...)
	if err := os.WriteFile(stream, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Events) != 2 {
		t.Fatalf("deduplicated valid events = %d, want 2", len(read.Events))
	}
	if !read.EffectApplied["event-id"] {
		t.Fatal("identical physical duplicate suppressed the canonical lifecycle effect")
	}
	for _, code := range []string{"malformed-complete-line", "unsupported-protocol", "partial-final-line", "broken-predecessor"} {
		if !hasIntegrityCode(read.Integrity, code) {
			t.Errorf("missing integrity code %s: %+v", code, read.Integrity)
		}
	}
	if len(read.Records) != 6 || read.Records[2].Event != nil || read.Records[2].Applied {
		t.Fatalf("malformed complete evidence was not retained: %+v", read.Records)
	}
}

func TestAppendRefusesPartialTargetStream(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	file, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"eventId":"partial"`); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), passiveEvent(t, "event-after-partial", "observation-after-partial", metadata.EffortID, nil)); err == nil {
		t.Fatal("append accepted a partial target stream")
	}
}

func TestReaderReportsConflictingDuplicateIdentities(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	first := passiveEvent(t, "event-a", "same-observation", metadata.EffortID, nil)
	secondMap := map[string]any{}
	if err := json.Unmarshal(first, &secondMap); err != nil {
		t.Fatal(err)
	}
	secondMap["eventId"] = "event-b"
	secondMap["payload"].(map[string]any)["inputTokens"] = float64(99)
	second := mustJSON(t, secondMap)
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	file, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(append(first, '\n'), append(second, '\n')...)); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIntegrityCode(read.Integrity, "conflicting-duplicate") || len(read.Events) != 3 || read.EffectApplied["event-b"] {
		t.Fatalf("duplicate conflict evidence/effect separation is wrong: events=%d mask=%v integrity=%+v", len(read.Events), read.EffectApplied, read.Integrity)
	}
	projection := ProjectWorkflow(read)
	if !containsString(projection.EvidenceEventIDs, "event-b") || containsString(projection.AllWorkEventIDs, "event-b") {
		t.Fatalf("conflicting duplicate leaked into effect projection: %#v", projection)
	}
}

func TestDuplicateCreationHasNoAppliedStateEffect(t *testing.T) {
	t.Parallel()
	ledger, metadata, first := createTestEffort(t)
	var duplicate map[string]any
	if err := json.Unmarshal(first, &duplicate); err != nil {
		t.Fatal(err)
	}
	duplicate["eventId"] = "duplicate-create-event"
	duplicate["idempotencyKey"] = "duplicate-create-key"
	duplicate["sessionId"] = "z-duplicate-session"
	stream := ledger.paths.stream(metadata.EffortID, "z-duplicate-session")
	if err := os.WriteFile(stream, append(mustJSON(t, duplicate), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Events) != 2 || read.EffectApplied["duplicate-create-event"] || !read.EffectApplied["event-id"] || !hasIntegrityCode(read.Integrity, "duplicate-creation-event") {
		t.Fatalf("duplicate creation evidence/effect separation is wrong: %+v", read)
	}
	for _, record := range read.Records {
		if record.Event != nil && record.Event.EventID == "duplicate-create-event" && record.Applied {
			t.Fatal("duplicate creation record was marked applied")
		}
	}
}

func TestLifecycleAndPassiveIdentitiesUseSeparateContracts(t *testing.T) {
	t.Parallel()
	ledger, metadata, first := createTestEffort(t)
	var creation map[string]any
	if err := json.Unmarshal(first, &creation); err != nil {
		t.Fatal(err)
	}
	shared := creation["idempotencyKey"].(string)
	passive := passiveEvent(t, "passive-shared-id", shared, metadata.EffortID, nil)
	if _, err := ledger.Append(context.Background(), passive); err != nil {
		t.Fatalf("passive observation collided with lifecycle idempotency key: %v", err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil || len(read.Events) != 2 || hasIntegrityCode(read.Integrity, "conflicting-duplicate") {
		t.Fatalf("contract-specific identity comparison failed: events=%d integrity=%+v err=%v", len(read.Events), read.Integrity, err)
	}
}

func TestReaderInvalidTransitionSeamRetainsWithoutApplying(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	invalid := passiveEvent(t, "invalid-event", "invalid-observation", metadata.EffortID, nil)
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	file, err := os.OpenFile(stream, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(invalid, '\n')); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	ledger.validateTransition = func(event EventEnvelope, applied []EventEnvelope) error {
		if event.EventID == "invalid-event" {
			return os.ErrInvalid
		}
		return nil
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIntegrityCode(read.Integrity, "invalid-transition") || len(read.Events) != 2 || read.EffectApplied["invalid-event"] {
		t.Fatalf("invalid transition evidence/effect separation is wrong: %+v", read)
	}
	last := read.Records[len(read.Records)-1]
	if last.Event == nil || last.Applied || !strings.Contains(read.Integrity[len(read.Integrity)-1].Detail, "invalid") {
		t.Fatalf("invalid transition evidence was not retained: %+v", last)
	}
}

func TestReaderRetainsUnappliedCreationThatDiffersFromImmutableMetadata(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	metadata.CheckpointID = "changed.md"
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	path := ledger.paths.effort(metadata.EffortID) + "/effort.json"
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Events) != 1 || read.EffectApplied["event-id"] || !hasIntegrityCode(read.Integrity, "creation-metadata-mismatch") || !hasIntegrityCode(read.Integrity, "missing-or-duplicate-creation-event") {
		t.Fatalf("mismatched creation evidence/effect separation is wrong: %+v", read)
	}
}

func TestReaderRefusesUnsafeSessionEntries(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	sessions := ledger.paths.effort(metadata.EffortID) + "/sessions"
	if err := os.Mkdir(sessions+"/directory.jsonl", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessions+"/not-a-stream", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, issue := range read.Integrity {
		if issue.Code == "unsafe-stream-entry" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("unsafe entries reported %d times, want 2: %+v", count, read.Integrity)
	}
}

func TestReaderIdentityAndPlacementRejectionMatrix(t *testing.T) {
	ledger, metadata, first := createTestEffort(t)
	if _, err := ledger.ReadEffort("../bad"); err == nil {
		t.Fatal("unsafe effort read accepted")
	}
	if err := os.WriteFile(ledger.paths.tombstone(metadata.EffortID), []byte(`{"nonce":"n","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.ReadEffort(metadata.EffortID); err == nil {
		t.Fatal("tombstoned read accepted")
	}
	if err := os.Remove(ledger.paths.tombstone(metadata.EffortID)); err != nil {
		t.Fatal(err)
	}

	metadataPath := ledger.paths.effort(metadata.EffortID) + "/effort.json"
	wrong := metadata
	wrong.EffortID = "other-effort"
	raw, _ := json.Marshal(wrong)
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.ReadEffort(metadata.EffortID); err == nil {
		t.Fatal("metadata identity mismatch accepted")
	}
	metadata.EffortID = "effort-id"
	raw, _ = json.Marshal(metadata)
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	unsafeName := ledger.paths.effort(metadata.EffortID) + "/sessions/bad\\name.jsonl"
	if err := os.WriteFile(unsafeName, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	var misplaced map[string]any
	if err := json.Unmarshal(first, &misplaced); err != nil {
		t.Fatal(err)
	}
	misplaced["eventId"] = "misplaced-create"
	misplaced["idempotencyKey"] = "misplaced-create"
	if err := os.WriteFile(stream, append(append(passiveEvent(t, "before-create", "before-create", metadata.EffortID, nil), '\n'), append(mustJSON(t, misplaced), '\n')...), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIntegrityCode(read.Integrity, "unsafe-stream-identifier") || !hasIntegrityCode(read.Integrity, "misplaced-creation-event") {
		t.Fatalf("reader rejection matrix missing issues: %+v", read.Integrity)
	}

	identityMismatch := passiveEvent(t, "identity-mismatch", "identity-mismatch", metadata.EffortID, nil)
	var mismatch map[string]any
	if err := json.Unmarshal(identityMismatch, &mismatch); err != nil {
		t.Fatal(err)
	}
	mismatch["sessionId"] = "other-session"
	if err := os.WriteFile(stream, append(mustJSON(t, mismatch), '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err = ledger.ReadEffort(metadata.EffortID)
	if err != nil || !hasIntegrityCode(read.Integrity, "stream-identity-mismatch") {
		t.Fatalf("stream identity mismatch not reported: %+v %v", read.Integrity, err)
	}
}

func TestSplitJSONLines(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw      string
		complete int
		partial  string
	}{
		{"", 0, ""},
		{"{}\n", 1, ""},
		{"{}", 0, "{}"},
		{"{}\n[]", 1, "[]"},
	}
	for _, test := range cases {
		result := splitJSONLines([]byte(test.raw))
		if len(result.complete) != test.complete || string(result.partial) != test.partial {
			t.Errorf("split %q = %d/%q", test.raw, len(result.complete), result.partial)
		}
	}
}

func hasIntegrityCode(issues []IntegrityIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
