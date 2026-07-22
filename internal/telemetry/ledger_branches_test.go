package telemetry

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreationAndAppendRejectionMatrix(t *testing.T) {
	metadata, first := testCreation(t)
	invalidMetadata := []EffortMetadata{
		{},
		func() EffortMetadata { value := metadata; value.CreatedAt = "bad"; return value }(),
		func() EffortMetadata { value := metadata; value.EffortID = "other"; return value }(),
		func() EffortMetadata { value := metadata; value.Origin = nil; return value }(),
	}
	for index, candidate := range invalidMetadata {
		if _, err := validateCreation(candidate, first); err == nil {
			t.Errorf("invalid creation metadata %d accepted", index)
		}
	}
	if _, err := validateCreation(metadata, json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid first event accepted")
	}
	var changed map[string]any
	if err := json.Unmarshal(first, &changed); err != nil {
		t.Fatal(err)
	}
	changed["timestamp"] = "2026-01-01T00:00:00Z"
	if _, err := validateCreation(metadata, mustJSON(t, changed)); err == nil {
		t.Fatal("mismatched first event accepted")
	}
	changed = map[string]any{}
	if err := json.Unmarshal(first, &changed); err != nil {
		t.Fatal(err)
	}
	changed["payload"].(map[string]any)["checkpointId"] = "other.md"
	if _, err := validateCreation(metadata, mustJSON(t, changed)); err == nil {
		t.Fatal("mismatched creation payload accepted")
	}

	ledger, _, _ := createTestEffort(t)
	for _, raw := range []json.RawMessage{json.RawMessage(`{}`), first} {
		if _, err := ledger.Append(context.Background(), raw); err == nil {
			t.Errorf("invalid append %s accepted", raw)
		}
	}
	unsafeEffort := passiveEvent(t, "unsafe-effort", "unsafe-effort", "../bad", nil)
	if _, err := ledger.Append(context.Background(), unsafeEffort); err == nil {
		t.Fatal("unsafe append effort accepted")
	}
	unsafeSessionMap := validEvent("usage_observed", 0)
	unsafeSessionMap["sessionId"] = "../bad"
	if _, err := ledger.Append(context.Background(), mustJSON(t, unsafeSessionMap)); err == nil {
		t.Fatal("unsafe append session accepted")
	}
}

func TestAppendMetadataAndNewStreamSyncFailures(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	metadataPath := filepath.Join(ledger.paths.effort(metadata.EffortID), "effort.json")
	wrong := metadata
	wrong.EffortID = "other-effort"
	raw, _ := json.Marshal(wrong)
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), passiveEvent(t, "wrong-metadata", "wrong-metadata", metadata.EffortID, nil)); err == nil {
		t.Fatal("append accepted mismatched immutable metadata")
	}
	raw, _ = json.Marshal(metadata)
	if err := os.WriteFile(metadataPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	eventMap := validEvent("usage_observed", 0)
	eventMap["eventId"] = "new-session-sync"
	eventMap["observationId"] = "new-session-sync"
	eventMap["effortId"] = metadata.EffortID
	eventMap["sessionId"] = "new-session"
	originalSync := ledger.ops.syncDir
	ledger.ops.syncDir = func(path string) error {
		if path == filepath.Join(ledger.paths.effort(metadata.EffortID), "sessions") {
			return errInjected
		}
		return originalSync(path)
	}
	if _, err := ledger.Append(context.Background(), mustJSON(t, eventMap)); err == nil {
		t.Fatal("new stream directory sync failure accepted")
	}
}

func TestMetadataAndRetryComparisonBranches(t *testing.T) {
	ledger, metadata, first := createTestEffort(t)
	variants := []EffortMetadata{
		func() EffortMetadata { value := metadata; value.EffortID = "other"; return value }(),
		func() EffortMetadata { value := metadata; value.Origin = nil; return value }(),
	}
	for _, variant := range variants {
		if metadataEqual(metadata, variant) {
			t.Fatalf("different metadata compared equal: %+v", variant)
		}
	}
	independent := metadata
	independent.Origin = nil
	if !metadataEqual(independent, independent) {
		t.Fatal("nil-origin metadata did not compare equal")
	}

	stream := ledger.paths.stream(metadata.EffortID, "session-id")
	original, err := os.ReadFile(stream)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		raw  []byte
	}{
		{"partial", []byte(`{}`)},
		{"invalid", []byte("{}\n")},
		{"different", append(passiveEvent(t, "different", "different", metadata.EffortID, nil), '\n')},
	}
	for _, test := range cases {
		if err := os.WriteFile(stream, test.raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.CreateEffort(metadata, first); err == nil {
			t.Errorf("%s initial stream accepted as retry", test.name)
		}
	}
	if err := os.WriteFile(stream, original, 0o600); err != nil {
		t.Fatal(err)
	}
	metadataPath := filepath.Join(ledger.paths.effort(metadata.EffortID), "effort.json")
	for _, malformed := range [][]byte{[]byte("{\n"), []byte(`{}`)} {
		if err := os.WriteFile(metadataPath, malformed, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.CreateEffort(metadata, first); err == nil {
			t.Fatalf("malformed immutable metadata accepted: %q", malformed)
		}
	}
	changed := metadata
	changed.CheckpointID = "different.md"
	changedRaw, _ := json.Marshal(changed)
	if err := os.WriteFile(metadataPath, changedRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.CreateEffort(metadata, first); err == nil {
		t.Fatal("different on-disk immutable metadata accepted")
	}
	metadataRaw, _ := json.Marshal(metadata)
	if err := os.WriteFile(metadataPath, metadataRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(stream); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.CreateEffort(metadata, first); err == nil {
		t.Fatal("missing initial stream accepted as retry")
	}
	if err := os.WriteFile(stream, original, 0o600); err != nil {
		t.Fatal(err)
	}
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(path string) ([]byte, error) {
		if path == stream {
			return nil, errInjected
		}
		return originalRead(path)
	}
	if _, err := ledger.CreateEffort(metadata, first); err == nil {
		t.Fatal("initial stream read failure accepted as retry")
	}
}

func TestCreationCompareToCommitRacesRejectTombstoneAndReplacement(t *testing.T) {
	for _, tombstoneRace := range []bool{true, false} {
		ledger, err := NewLedger(newTestProject(t))
		if err != nil {
			t.Fatal(err)
		}
		metadata, first := testCreation(t)
		target := ledger.paths.effort(metadata.EffortID)
		if tombstoneRace {
			target = ledger.paths.tombstone(metadata.EffortID)
		}
		originalLstat := ledger.ops.lstat
		var calls int
		ledger.ops.lstat = func(path string) (os.FileInfo, error) {
			if path == target {
				calls++
				if calls == 1 {
					if tombstoneRace {
						if err := os.WriteFile(target, []byte(`{"nonce":"race","state":"pending"}`), 0o600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.MkdirAll(target, 0o700); err != nil {
						t.Fatal(err)
					}
					return nil, os.ErrNotExist
				}
			}
			return originalLstat(path)
		}
		if _, err := ledger.CreateEffort(metadata, first); err == nil {
			t.Fatalf("creation compare-to-commit race accepted (tombstone=%v)", tombstoneRace)
		}
	}
}

func TestLeaseFaultAndHeartbeatBranches(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)

	originalLock := ledger.ops.lockLease
	ledger.ops.lockLease = func(context.Context, string) (func() error, error) {
		return func() error { return errInjected }, nil
	}
	if err := ledger.withLeaseOperation(context.Background(), func() error { return nil }); err == nil {
		t.Fatal("unlock failure accepted")
	}
	ledger.ops.lockLease = originalLock

	originalOpen := ledger.ops.openFile
	for _, failure := range []string{"short", "write", "sync", "close"} {
		ledger.ops.openFile = func(string, int, fs.FileMode) (syncedFile, error) { return &faultSyncedFile{failure: failure}, nil }
		if _, err := ledger.acquireExclusiveLease(path, time.Second); err == nil {
			t.Errorf("lease %s failure accepted", failure)
		}
	}
	ledger.ops.openFile = originalOpen

	originalSleep := ledger.ops.sleep
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errInjected }
	cancel, done := ledger.startHeartbeat(context.Background(), path, "missing")
	cancel()
	if err := <-done; err == nil || !strings.Contains(err.Error(), "wait") {
		t.Fatalf("heartbeat wait failure not returned: %v", err)
	}
	ledger.ops.sleep = func(context.Context, time.Duration) error { return nil }
	cancel, done = ledger.startHeartbeat(context.Background(), path, "missing")
	if err := <-done; err == nil || !strings.Contains(err.Error(), "heartbeat") {
		t.Fatalf("heartbeat refresh failure not returned: %v", err)
	}
	cancel()
	ledger.ops.sleep = originalSleep

	record, _ := json.Marshal(leaseRecord{Nonce: "held", Owner: "owner", ExpiresAt: time.Now().Add(time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(path, record, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	stop()
	if _, err := ledger.acquireLease(ctx, path); err == nil {
		t.Fatal("canceled lease acquisition accepted")
	}
}

func TestOperationHeartbeatFailuresAreReturned(t *testing.T) {
	for _, operation := range []string{"create", "append"} {
		ledger, err := NewLedger(newTestProject(t))
		if err != nil {
			t.Fatal(err)
		}
		metadata, first := testCreation(t)
		if operation == "append" {
			if _, err := ledger.CreateEffort(metadata, first); err != nil {
				t.Fatal(err)
			}
		}
		ledger.ops.sleep = func(context.Context, time.Duration) error { return errInjected }
		var operationErr error
		if operation == "create" {
			_, operationErr = ledger.CreateEffort(metadata, first)
		} else {
			_, operationErr = ledger.Append(context.Background(), passiveEvent(t, "heartbeat-fault", "heartbeat-fault", metadata.EffortID, nil))
		}
		if operationErr == nil {
			t.Errorf("%s heartbeat failure accepted", operation)
		}
	}
}

func TestStaleLeaseAndRecoveryErrorMatrices(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)
	now := time.Now()
	ledger.ops.now = func() time.Time { return now }
	write := func(raw string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{`{}`, `{"nonce":"n","owner":"o","expiresAt":"bad"}`} {
		write(raw)
		if _, err := ledger.removeStaleLease(path); err == nil {
			t.Errorf("ambiguous lease %q accepted", raw)
		}
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if removed, err := ledger.removeStaleLease(path); err != nil || !removed {
		t.Fatalf("missing lease recovery = %v %v", removed, err)
	}

	bad := filepath.Join(ledger.paths.leases, "directory.json")
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger.paths.leases, "odd"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale, _ := json.Marshal(leaseRecord{Nonce: "stale", Owner: "owner", ExpiresAt: now.Add(-2 * time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(filepath.Join(ledger.paths.leases, "stale.json"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ledger.Recover()
	if err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-lease-path") {
		t.Fatalf("unsafe lease recovery = %+v %v", report, err)
	}
}

func TestRecoveryStateAndFaultMatrix(t *testing.T) {
	ledger, _, _ := createTestEffort(t)
	now := time.Now().Add(time.Hour)
	ledger.ops.now = func() time.Time { return now }

	if err := os.WriteFile(filepath.Join(ledger.paths.staging, "bad"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(ledger.paths.staging, stagingName("fresh", "nonce"))
	if err := os.Mkdir(fresh, 0o700); err != nil {
		t.Fatal(err)
	}
	both := filepath.Join(ledger.paths.staging, stagingName("both", "nonce"))
	if err := os.Mkdir(both, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(both, now.Add(-time.Hour*2), now.Add(-time.Hour*2)); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ledger.paths.effort("both"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.Mkdir(filepath.Join(ledger.paths.tombstones, "directory.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger.paths.tombstones, "bad.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ledger.paths.tombstones, "bad\\id.json"), []byte(`{"nonce":"n","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	pendingID := "recoverable-pending"
	if err := os.MkdirAll(ledger.paths.effort(pendingID), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger.paths.tombstone(pendingID), []byte(`{"nonce":"pending","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	committedID := "already-committed"
	if err := os.WriteFile(ledger.paths.tombstone(committedID), []byte(`{"nonce":"committed","state":"committed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatchID := "mismatched-trash"
	if err := os.WriteFile(ledger.paths.tombstone(mismatchID), []byte(`{"nonce":"expected","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ledger.paths.trash, stagingName(mismatchID, "other")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ledger.paths.trash, "orphan"), 0o700); err != nil {
		t.Fatal(err)
	}
	report, err := ledger.Recover()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"ambiguous-staging-path", "ambiguous-staging-commit", "ambiguous-tombstone-path", "ambiguous-tombstone", "ambiguous-prune-state", "ambiguous-trash-path"} {
		if !hasIntegrityCode(report.Ambiguous, code) {
			t.Errorf("missing recovery issue %s: %+v", code, report)
		}
	}
}

func TestJSONAndRandomFailureBranches(t *testing.T) {
	if _, err := compactLine([]byte(`{`)); err == nil {
		t.Fatal("invalid compact input accepted")
	}
	if _, err := eventsEqual(EventEnvelope{}, json.RawMessage(`{}`)); err == nil {
		t.Fatal("invalid comparison input accepted")
	}
	if _, err := canonicalJSON([]byte(`{`)); err == nil {
		t.Fatal("invalid canonical input accepted")
	}

	event, err := ValidateEvent(json.RawMessage(validEventJSON(t, "usage_observed")))
	if err != nil {
		t.Fatal(err)
	}
	event.EnvelopeExtensions = map[string]json.RawMessage{"future": json.RawMessage(`1`)}
	event.PayloadExtensions = map[string]json.RawMessage{"futurePayload": json.RawMessage(`true`)}
	if raw := eventToRaw(event); !strings.Contains(string(raw), "futurePayload") {
		t.Fatalf("event extensions lost: %s", raw)
	}

	originalReader := crand.Reader
	crand.Reader = errorReader{}
	if _, err := randomNonce(); err == nil {
		t.Fatal("random source failure accepted")
	}
	crand.Reader = originalReader
}

func validEventJSON(t *testing.T, kind string) []byte {
	t.Helper()
	return mustJSON(t, validEvent(kind, 0))
}
