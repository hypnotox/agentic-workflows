package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// invariant: tooling/workflow-telemetry:privacy-integrity-and-retention
func TestPrivacyIntegrityAndRetention(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	writeRetentionEffort(t, ledger, "active", now.Add(-100*24*time.Hour), time.Time{}, EffortActive)
	writeRetentionEffort(t, ledger, "exact", now.Add(-20*24*time.Hour), now.Add(-10*24*time.Hour), EffortAbandoned)
	writeRetentionEffort(t, ledger, "old", now.Add(-30*24*time.Hour), now.Add(-10*24*time.Hour-time.Nanosecond), EffortAbandoned)
	writeRetentionEffort(t, ledger, "new", now.Add(-2*24*time.Hour), now.Add(-24*time.Hour), EffortAbandoned)

	result, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 10}, true)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"old"}) || len(result.Pruned) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("age dry run = %#v, %v", result, err)
	}
	result, err = ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortCount: 1}, true)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"old", "exact"}) {
		t.Fatalf("count dry run = %#v, %v", result, err)
	}
	result, err = ledger.Retain(context.Background(), RetentionPolicy{}, true)
	if err != nil || len(result.Candidates) != 0 {
		t.Fatalf("disabled dimensions = %#v, %v", result, err)
	}
	if _, err := ledger.ReadEffort("active"); err != nil {
		t.Fatalf("active effort was not preserved: %v", err)
	}

	privacy := validEvent("usage_observed", 0)
	privacy["prompt"] = "forbidden"
	if _, err := ValidateEvent(mustJSON(t, privacy)); err == nil {
		t.Fatal("privacy-forbidden conversational field accepted")
	}
	unsupported := validEvent("usage_observed", 0)
	unsupported["version"] = map[string]any{"major": 99, "minor": 0}
	if _, err := ValidateEvent(mustJSON(t, unsupported)); err == nil {
		t.Fatal("unsupported protocol interpretation accepted")
	}
	if err := validatePathIdentifier("effortId", "../escape"); err == nil {
		t.Fatal("unsafe resident path identifier accepted")
	}
	originalRename := ledger.ops.rename
	originalSyncDir := ledger.ops.syncDir
	pendingUnderLease, committedAfterTrash := false, false
	syncedTrash, syncedTombstones := false, false
	ledger.ops.rename = func(oldPath, newPath string) error {
		if oldPath == ledger.paths.effort("exact") && strings.HasPrefix(newPath, ledger.paths.trash+string(filepath.Separator)) {
			raw, readErr := os.ReadFile(ledger.paths.tombstone("exact"))
			_, leaseErr := os.Lstat(ledger.paths.appendLease("exact"))
			pendingUnderLease = readErr == nil && strings.Contains(string(raw), `"state":"pending"`) && leaseErr == nil
		}
		if strings.HasSuffix(oldPath, ".commit") && newPath == ledger.paths.tombstone("exact") {
			entries, readErr := os.ReadDir(ledger.paths.trash)
			committedAfterTrash = readErr == nil && len(entries) == 1
		}
		return originalRename(oldPath, newPath)
	}
	ledger.ops.syncDir = func(path string) error {
		syncedTrash = syncedTrash || path == ledger.paths.trash
		syncedTombstones = syncedTombstones || path == ledger.paths.tombstones
		return originalSyncDir(path)
	}
	purged, err := ledger.Purge(context.Background(), "exact", true)
	if err != nil || !reflect.DeepEqual(purged.Pruned, []string{"exact"}) {
		t.Fatalf("confirmed terminal purge = %#v, %v", purged, err)
	}
	if _, err := os.Lstat(ledger.paths.tombstone("exact")); err != nil {
		t.Fatalf("purge did not leave durable committed tombstone: %v", err)
	}
	if _, err := os.Lstat(ledger.paths.effort("exact")); !os.IsNotExist(err) {
		t.Fatalf("purged effort remains resident: %v", err)
	}
	if !pendingUnderLease || !committedAfterTrash || !syncedTrash || !syncedTombstones {
		t.Fatalf("prune durability order missing: pendingUnderLease=%v committedAfterTrash=%v syncedTrash=%v syncedTombstones=%v", pendingUnderLease, committedAfterTrash, syncedTrash, syncedTombstones)
	}
}

func TestRetentionUsesEffectiveRepairedTerminalTimestamp(t *testing.T) {
	for _, replacementKind := range []EventKind{"effort_completed", "effort_abandoned"} {
		t.Run(string(replacementKind), func(t *testing.T) {
			events := completedRoute("direct")
			terminalTimestamp := events[len(events)-1].Timestamp
			rejected := passiveProjectionEvent("rejected-passive", "")
			rejected.Predecessors = []string{"complete"}
			events = append(events, rejected)
			replacement, err := json.Marshal(EffortTerminalPayload{TerminalEpoch: 1})
			if err != nil {
				t.Fatal(err)
			}
			events = appendEvent(events, "terminal-repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"complete"}, Replacement: RepairReplacement{EventKind: replacementKind, Payload: replacement}})
			projection := ProjectLifecycle(events)
			projection.EffectApplied["rejected-passive"] = false
			read := EffortRead{
				Metadata:        EffortMetadata{EffortID: "repaired", CreatedAt: "2026-07-22T00:00:00Z", CreationMode: "independent"},
				Events:          events,
				EffectApplied:   projection.EffectApplied,
				RejectedEffects: map[string]bool{"rejected-passive": true},
			}
			candidate, terminal, err := retentionCandidateFromRead(read, "repaired")
			if err != nil || !terminal || candidate.TerminalTimestamp.Format(time.RFC3339Nano) != terminalTimestamp {
				t.Fatalf("repaired terminal candidate = %#v, terminal=%v err=%v projection=%#v", candidate, terminal, err, projection)
			}
			terminalEvent := events[len(events)-2]
			if terminalEvent.EventID != "rejected-passive" {
				t.Fatalf("unexpected repaired fixture ordering: %s", terminalEvent.EventID)
			}
			directTerminal := events[len(events)-3]
			if _, err := effectiveTerminalTimestamp(EffortRead{Events: []EventEnvelope{terminalEvent, directTerminal}, EffectApplied: map[string]bool{terminalEvent.EventID: false, directTerminal.EventID: true}}, LifecycleProjection{TerminalEpoch: 1}); err != nil {
				t.Fatalf("unapplied non-repair evidence blocked direct terminal timestamp: %v", err)
			}
		})
	}
}

func TestRetentionStableNewestAndInversePruneTies(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	terminal := now.Add(-48 * time.Hour)
	writeRetentionEffort(t, ledger, "a", now.Add(-5*24*time.Hour), terminal, EffortCompleted)
	writeRetentionEffort(t, ledger, "b", now.Add(-4*24*time.Hour), terminal, EffortCompleted)
	writeRetentionEffort(t, ledger, "c", now.Add(-4*24*time.Hour), terminal, EffortAbandoned)

	all, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, true)
	if err != nil || !reflect.DeepEqual(all.Candidates, []string{"a", "c", "b"}) {
		t.Fatalf("inverse tie order = %#v, %v", all, err)
	}
	count, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortCount: 1}, true)
	if err != nil || !reflect.DeepEqual(count.Candidates, []string{"a", "c"}) {
		t.Fatalf("newest count order = %#v, %v", count, err)
	}
}

func TestRetentionPruneDurabilityNonceAndWriterLinearization(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	writeRetentionEffort(t, ledger, "terminal", now.Add(-48*time.Hour), now.Add(-24*time.Hour), EffortAbandoned)

	leasePresentDuringDelete := false
	committedDuringDelete := false
	originalRemoveAll := ledger.ops.removeAll
	ledger.ops.removeAll = func(path string) error {
		if strings.HasPrefix(path, ledger.paths.trash+string(filepath.Separator)) {
			leasePresentDuringDelete = ledger.pathExists(ledger.paths.appendLease("terminal"))
			raw, _ := os.ReadFile(ledger.paths.tombstone("terminal"))
			record, _ := readTombstone(raw)
			committedDuringDelete = record.State == "committed"
		}
		return originalRemoveAll(path)
	}
	pruned, err := ledger.pruneEffort(context.Background(), "terminal", "fixed-nonce")
	if err != nil || !pruned {
		t.Fatalf("prune = %v, %v", pruned, err)
	}
	if leasePresentDuringDelete || !committedDuringDelete {
		t.Fatalf("delete ordering lease=%v committed=%v", leasePresentDuringDelete, committedDuringDelete)
	}
	if ledger.pathExists(ledger.paths.effort("terminal")) || !ledger.pathExists(ledger.paths.tombstone("terminal")) {
		t.Fatal("prune commit state is not durable")
	}
	if _, err := ledger.Append(context.Background(), passiveEvent(t, "late", "late-observation", "terminal", nil)); err == nil {
		t.Fatal("late append recreated a pruned effort")
	}
	retryMetadata := EffortMetadata{EffortID: "terminal", CreatedAt: now.Format(time.RFC3339Nano), CreationMode: "independent"}
	retryEvent := causalEvent("retry-create", "session", "effort_created", []string{}, EffortCreatedPayload{CreationMode: "independent"})
	retryEvent.EffortID = "terminal"
	retryEvent.Timestamp = retryMetadata.CreatedAt
	retryRaw, _ := json.Marshal(retryEvent)
	if _, err := ledger.CreateEffort(retryMetadata, retryRaw); err == nil || !strings.Contains(err.Error(), "pruned") {
		t.Fatalf("creation reused pruned ID: %v", err)
	}
	pruned, err = ledger.pruneEffort(context.Background(), "terminal", "fixed-nonce")
	if err != nil || !pruned {
		t.Fatalf("same prune nonce retry = %v, %v", pruned, err)
	}
	if _, err := ledger.pruneEffort(context.Background(), "terminal", "different-nonce"); err == nil {
		t.Fatal("different prune nonce was not ambiguous")
	}
}

func TestRetentionRechecksTerminalUnderLeaseAfterReopen(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	writeRetentionEffort(t, ledger, "completed", now.Add(-72*time.Hour), now.Add(-48*time.Hour), EffortCompleted)
	originalNonce := ledger.ops.nonce
	injected := false
	ledger.ops.nonce = func() (string, error) {
		if !injected {
			injected = true
			appendExternalReopen(t, ledger, "completed", now.Add(-time.Hour))
		}
		return originalNonce()
	}
	result, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, false)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"completed"}) || !reflect.DeepEqual(result.Skipped, []string{"completed"}) || len(result.Pruned) != 0 {
		t.Fatalf("reopen race = %#v, %v", result, err)
	}
	read, err := ledger.ReadEffort("completed")
	if err != nil || ProjectLifecycle(read.Events).State != EffortActive {
		t.Fatalf("reopen did not win linearization: %v", err)
	}
}

func TestConfirmedPurgeRefusalsAndSuccess(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	writeRetentionEffort(t, ledger, "active", now, time.Time{}, EffortActive)
	writeRetentionEffort(t, ledger, "terminal", now, now.Add(time.Hour), EffortAbandoned)
	if _, err := ledger.Purge(context.Background(), "terminal", false); err == nil {
		t.Fatal("unconfirmed purge succeeded")
	}
	if _, err := ledger.Purge(context.Background(), "active", true); err == nil {
		t.Fatal("active purge succeeded")
	}
	if _, err := ledger.Purge(context.Background(), "missing", true); err == nil {
		t.Fatal("missing purge succeeded")
	}
	result, err := ledger.Purge(context.Background(), "terminal", true)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"terminal"}) || !reflect.DeepEqual(result.Pruned, []string{"terminal"}) {
		t.Fatalf("confirmed purge = %#v, %v", result, err)
	}
}

func TestRetentionRefusesInvalidLimitsEntriesAndNonces(t *testing.T) {
	ledger := newRetentionLedger(t)
	if _, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: -1}, true); err == nil {
		t.Fatal("negative retention accepted")
	}
	if err := os.WriteFile(filepath.Join(ledger.paths.efforts, "resident-file"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortCount: 1}, true); err == nil {
		t.Fatal("non-directory effort entry accepted")
	}
	if _, err := ledger.pruneEffort(context.Background(), "bad/id", "nonce"); err == nil {
		t.Fatal("unsafe effort ID accepted")
	}
	for _, nonce := range []string{"bad.nonce", "bad nonce", strings.Repeat("x", 129)} {
		if _, err := ledger.pruneEffort(context.Background(), "missing", nonce); err == nil {
			t.Errorf("unsafe prune nonce %q accepted", nonce)
		}
	}
	for _, raw := range [][]byte{
		[]byte(`{"nonce":"n","state":"pending","extra":true}`),
		[]byte(`{"nonce":"n","state":"pending"} {}`),
	} {
		if _, err := readTombstone(raw); err == nil {
			t.Errorf("ambiguous tombstone %q accepted", raw)
		}
	}
}

func newRetentionLedger(t *testing.T) *Ledger {
	t.Helper()
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	return ledger
}

func writeRetentionEffort(t *testing.T, ledger *Ledger, effortID string, created, terminal time.Time, state EffortState) {
	t.Helper()
	effortPath := ledger.paths.effort(effortID)
	if err := os.MkdirAll(filepath.Join(effortPath, "sessions"), 0o700); err != nil {
		t.Fatal(err)
	}
	metadata := EffortMetadata{EffortID: effortID, CreatedAt: created.Format(time.RFC3339Nano), CreationMode: "independent"}
	metadataRaw, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(effortPath, "effort.json"), append(metadataRaw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	var events []EventEnvelope
	if state == EffortCompleted {
		events = completedRoute("investigation-only")
	} else {
		events = lifecycleBaseEvents()
		if state == EffortAbandoned {
			events = appendEvent(events, "abandon", "effort_abandoned", EffortTerminalPayload{TerminalEpoch: 1})
		}
	}
	for index := range events {
		events[index].EffortID = effortID
		if events[index].Predecessors == nil {
			events[index].Predecessors = []string{}
		}
		events[index].Timestamp = created.Add(time.Duration(index) * time.Second).Format(time.RFC3339Nano)
		if index == 0 {
			events[index].Payload, _ = json.Marshal(EffortCreatedPayload{CreationMode: "independent"})
		}
	}
	if !terminal.IsZero() {
		events[len(events)-1].Timestamp = terminal.Format(time.RFC3339Nano)
	}
	var stream []byte
	for _, event := range events {
		raw, _ := json.Marshal(event)
		stream = append(stream, raw...)
		stream = append(stream, '\n')
	}
	if err := os.WriteFile(ledger.paths.stream(effortID, "session"), stream, 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendExternalReopen(t *testing.T, ledger *Ledger, effortID string, timestamp time.Time) {
	t.Helper()
	read, err := ledger.ReadEffort(effortID)
	if err != nil {
		t.Fatal(err)
	}
	terminalID := ""
	for _, event := range read.Events {
		if event.Kind == "effort_completed" && read.EffectApplied[event.EventID] {
			terminalID = event.EventID
		}
	}
	event := causalEvent("reopen", "session", "effort_reopened", []string{terminalID}, EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "reopened", AnchorID: "anchor"})
	event.EffortID = effortID
	event.Timestamp = timestamp.Format(time.RFC3339Nano)
	if event.Predecessors == nil {
		event.Predecessors = []string{}
	}
	raw, _ := json.Marshal(event)
	file, err := os.OpenFile(ledger.paths.stream(effortID, "session"), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
