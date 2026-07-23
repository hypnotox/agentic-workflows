package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestProtocolLedgerContract(t *testing.T) {
	// The protocol ledger atomically creates an effort, preserves immutable
	// metadata, and appends complete durable lines without recreating state.
	ledger, metadata, first := createTestEffort(t)
	result, err := ledger.CreateEffort(metadata, first)
	if err != nil || !result.Idempotent {
		t.Fatalf("identical retry = (%v, %v), want idempotent success", result, err)
	}
	changed := metadata
	changed.CreatedAt = "2026-07-22T12:34:57Z"
	if _, err := ledger.CreateEffort(changed, first); err == nil {
		t.Fatal("conflicting immutable retry accepted")
	}

	event := passiveEvent(t, "event-2", "observation-2", metadata.EffortID, nil)
	appended, err := ledger.Append(context.Background(), event)
	if err != nil || appended.Idempotent {
		t.Fatalf("append = (%v, %v)", appended, err)
	}
	retry, err := ledger.Append(context.Background(), event)
	if err != nil || !retry.Idempotent {
		t.Fatalf("append retry = (%v, %v)", retry, err)
	}
	creationRetry, err := ledger.CreateEffort(metadata, first)
	if err != nil || !creationRetry.Idempotent {
		t.Fatalf("creation retry after later events = (%v, %v)", creationRetry, err)
	}
	var conflict map[string]any
	if err := json.Unmarshal(event, &conflict); err != nil {
		t.Fatal(err)
	}
	conflict["payload"].(map[string]any)["inputTokens"] = float64(99)
	if _, err := ledger.Append(context.Background(), mustJSON(t, conflict)); err == nil {
		t.Fatal("conflicting append retry accepted")
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil || len(read.Events) != 2 {
		t.Fatalf("read after append = %d events, %v", len(read.Events), err)
	}
	raw, err := os.ReadFile(ledger.paths.stream(metadata.EffortID, "session-id"))
	if err != nil || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		t.Fatalf("stream is not complete JSONL: %q, %v", raw, err)
	}
	newer := validEvent("usage_observed", 1)
	newer["futureEnvelope"] = map[string]any{"value": true}
	newer["payload"].(map[string]any)["futurePayload"] = []any{"opaque"}
	validated, err := ValidateEvent(mustJSON(t, newer))
	if err != nil || len(validated.EnvelopeExtensions) != 1 || len(validated.PayloadExtensions) != 1 {
		t.Fatalf("compatible-minor extensions were not preserved: %#v, %v", validated, err)
	}
	privacy := validEvent("usage_observed", 1)
	privacy["future"] = map[string]any{"prompt": "forbidden"}
	if _, err := ValidateEvent(mustJSON(t, privacy)); err == nil {
		t.Fatal("nested privacy-forbidden extension accepted")
	}
	if err := validatePathIdentifier("sessionId", "../escape"); err == nil {
		t.Fatal("unsafe stream identifier accepted")
	}
	recovery, err := ledger.Recover()
	if err != nil || len(recovery.Ambiguous) != 0 {
		t.Fatalf("clean leased ledger recovery = %#v, %v", recovery, err)
	}
	for _, path := range []string{ledger.paths.root, ledger.paths.effort(metadata.EffortID), ledger.paths.stream(metadata.EffortID, "session-id")} {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("stat resident ledger path %s: %v", path, statErr)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("resident ledger path is not owner-only: %s mode=%v", path, info.Mode())
		}
	}
	if _, err := os.Lstat(ledger.paths.appendLease(metadata.EffortID)); !os.IsNotExist(err) {
		t.Fatalf("append lease was not released: %v", err)
	}
	file, err := os.OpenFile(ledger.paths.stream(metadata.EffortID, "session-id"), os.O_APPEND|os.O_WRONLY, 0)
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
	corrupt, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil || !hasIntegrityCode(corrupt.Integrity, "partial-final-line") || len(corrupt.Events) != 2 {
		t.Fatalf("partial stream evidence was not isolated: events=%d integrity=%#v err=%v", len(corrupt.Events), corrupt.Integrity, err)
	}
}

// invariant: tooling/workflow-telemetry:event-protocol-and-ledger
func TestProtocol2TransitionAppendRetryAndConflict(t *testing.T) {
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	create := map[string]any{
		"action": "create", "idempotencyKey": "create-key", "eventId": "create-event", "effortId": "effort-id", "sessionId": "session-id",
		"timestamp": "2026-07-22T00:00:00Z", "predecessors": []string{}, "creationMode": "independent",
	}
	applyProtocol2Request := func(request map[string]any) (AppendResult, error) {
		t.Helper()
		decoded, decodeErr := DecodeLifecycleRequest(mustJSON(t, request))
		if decodeErr != nil {
			return AppendResult{}, decodeErr
		}
		return ledger.ApplyLifecycle(context.Background(), decoded)
	}
	if result, err := applyProtocol2Request(create); err != nil || result.Idempotent {
		t.Fatalf("protocol 2 create = %#v, %v", result, err)
	}
	start := map[string]any{
		"action": "start-phase", "idempotencyKey": "start-key", "eventId": "brainstorm-start", "effortId": "effort-id", "sessionId": "session-id",
		"timestamp": "2026-07-22T00:00:01Z", "predecessors": []string{"create-event"}, "phase": "brainstorming",
	}
	if _, err := applyProtocol2Request(start); err != nil {
		t.Fatalf("start first phase: %v", err)
	}
	transition := protocol2TransitionRequest("transition", "brainstorm-start", []string{"brainstorm-start"}, "brainstorming", "implementation", "select", "direct")
	if result, err := applyProtocol2Request(transition); err != nil || result.Idempotent {
		t.Fatalf("transition append = %#v, %v", result, err)
	}
	if result, err := applyProtocol2Request(transition); err != nil || !result.Idempotent {
		t.Fatalf("identical transition retry = %#v, %v", result, err)
	}
	conflict := cloneMap(t, transition)
	conflict["nextPhase"] = "planning"
	if _, err := applyProtocol2Request(conflict); err == nil {
		t.Fatal("conflicting transition idempotency reuse accepted")
	}
	read, err := ledger.ReadEffort("effort-id")
	if err != nil || len(read.Events) != 3 || read.Events[2].Kind != "phase_transitioned" {
		t.Fatalf("transition durability = events %#v, %v", read.Events, err)
	}
}

func TestCreateEffortRejectsPreexistingOrSymlinkedStaging(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprintf("symlink=%v", symlink), func(t *testing.T) {
			root := newTestProject(t)
			ledger, err := NewLedger(root)
			if err != nil {
				t.Fatal(err)
			}
			nonce := "00000000000000000000000000000000"
			ledger.ops.nonce = func() (string, error) { return nonce, nil }
			metadata, first := testCreation(t)
			staging := filepath.Join(ledger.paths.staging, stagingName(metadata.EffortID, nonce))
			outside := t.TempDir()
			if symlink {
				if err := os.Symlink(outside, staging); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			} else if err := os.Mkdir(staging, 0o700); err != nil {
				t.Fatal(err)
			}
			if _, err := ledger.CreateEffort(metadata, first); err == nil {
				t.Fatal("pre-existing staging entry accepted")
			}
			if _, err := os.Lstat(filepath.Join(outside, "effort.json")); !os.IsNotExist(err) {
				t.Fatalf("creation escaped through staging entry: %v", err)
			}
		})
	}
}

func TestCreateEffortCommitPointAndInjectedFailures(t *testing.T) {
	t.Parallel()
	root := newTestProject(t)
	ledger, err := NewLedger(root)
	if err != nil {
		t.Fatal(err)
	}
	metadata, first := testCreation(t)
	originalRename := ledger.ops.rename
	ledger.ops.rename = func(oldPath, newPath string) error {
		if strings.HasPrefix(oldPath, ledger.paths.staging) && newPath == ledger.paths.effort(metadata.EffortID) {
			return errors.New("injected rename")
		}
		return originalRename(oldPath, newPath)
	}
	if _, err := ledger.CreateEffort(metadata, first); err == nil || ledger.pathExists(ledger.paths.effort(metadata.EffortID)) {
		t.Fatal("failed pre-commit creation became visible")
	}
	entries, err := os.ReadDir(ledger.paths.staging)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed staging not cleaned: %v %v", entries, err)
	}

	ledger.ops.rename = originalRename
	originalSyncDir := ledger.ops.syncDir
	var effortDirSyncs atomic.Int32
	ledger.ops.syncDir = func(path string) error {
		if path == ledger.paths.efforts && effortDirSyncs.Add(1) == 1 {
			return errors.New("injected post-commit sync")
		}
		return originalSyncDir(path)
	}
	if _, err := ledger.CreateEffort(metadata, first); err == nil || !ledger.pathExists(ledger.paths.effort(metadata.EffortID)) {
		t.Fatal("post-rename failure did not preserve committed effort")
	}
	ledger.ops.syncDir = originalSyncDir
	if result, err := ledger.CreateEffort(metadata, first); err != nil || !result.Idempotent {
		t.Fatalf("retry after commit-point failure = (%v, %v)", result, err)
	}
}

func TestConcurrentEffortCreationHasOneCommit(t *testing.T) {
	t.Parallel()
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	metadata, first := testCreation(t)
	const writers = 10
	var wait sync.WaitGroup
	results := make(chan AppendResult, writers)
	errorsFound := make(chan error, writers)
	for range writers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, createErr := ledger.CreateEffort(metadata, first)
			if createErr != nil {
				errorsFound <- createErr
				return
			}
			results <- result
		}()
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	for createErr := range errorsFound {
		t.Errorf("concurrent create: %v", createErr)
	}
	newCount := 0
	for result := range results {
		if !result.Idempotent {
			newCount++
		}
	}
	if newCount != 1 {
		t.Fatalf("new creation count = %d, want 1", newCount)
	}
}

func TestAppendRefusesMissingTombstonedAndUnsafeEfforts(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	event := passiveEvent(t, "event-2", "observation-2", metadata.EffortID, nil)
	if err := os.RemoveAll(ledger.paths.effort(metadata.EffortID)); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), event); err == nil || ledger.pathExists(ledger.paths.effort(metadata.EffortID)) {
		t.Fatal("missing effort was recreated")
	}
	if err := os.WriteFile(ledger.paths.tombstone(metadata.EffortID), []byte(`{"nonce":"n","state":"committed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Append(context.Background(), event); err == nil {
		t.Fatal("tombstoned effort accepted append")
	}

	liveLedger, liveMetadata, liveFirst := createTestEffort(t)
	if err := os.WriteFile(liveLedger.paths.tombstone(liveMetadata.EffortID), []byte(`{"nonce":"n","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := liveLedger.CreateEffort(liveMetadata, liveFirst); err == nil || !strings.Contains(err.Error(), "pruned") {
		t.Fatalf("tombstone was not rejected before creation retry: %v", err)
	}
}

func TestAppendLeaseStaleGraceAndCompareBeforeRemove(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	leasePath := ledger.paths.appendLease(metadata.EffortID)
	writeLease := func(nonce string, expiry time.Time) {
		t.Helper()
		record, _ := json.Marshal(leaseRecord{Nonce: nonce, Owner: "owner", ExpiresAt: expiry.Format(time.RFC3339Nano)})
		if err := os.WriteFile(leasePath, record, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeLease("fresh", now.Add(-ledger.leaseGrace))
	if removed, err := ledger.removeStaleLease(leasePath); err != nil || removed {
		t.Fatalf("lease removed at grace boundary: %v %v", removed, err)
	}
	writeLease("stale", now.Add(-ledger.leaseGrace-time.Nanosecond))
	originalRead := ledger.ops.readFile
	readStarted := make(chan struct{})
	allowRead := make(chan struct{})
	var once sync.Once
	ledger.ops.readFile = func(path string) ([]byte, error) {
		if path == leasePath {
			once.Do(func() {
				close(readStarted)
				<-allowRead
			})
		}
		return originalRead(path)
	}
	recoveryDone := make(chan error, 1)
	go func() {
		removed, recoverErr := ledger.removeStaleLease(leasePath)
		if recoverErr == nil && !removed {
			recoverErr = errors.New("stale lease was not removed")
		}
		recoveryDone <- recoverErr
	}()
	<-readStarted
	refreshDone := make(chan error, 1)
	go func() { refreshDone <- ledger.refreshLease(leasePath, "stale") }()
	select {
	case err := <-refreshDone:
		t.Fatalf("compare-to-replace contender bypassed operation guard: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(allowRead)
	if err := <-recoveryDone; err != nil {
		t.Fatal(err)
	}
	if err := <-refreshDone; err == nil {
		t.Fatal("heartbeat replaced a lease after serialized stale recovery")
	}
	ledger.ops.readFile = originalRead
}

func TestLeaseHeartbeatRefreshesOnlyItsNonce(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	ledger.ops.now = func() time.Time { return now }
	path := ledger.paths.appendLease(metadata.EffortID)
	nonce, err := ledger.acquireExclusiveLease(path, ledger.leaseDuration)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(10 * time.Second)
	if err := ledger.refreshLease(path, nonce); err != nil {
		t.Fatalf("refresh lease: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record leaseRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	wantExpiry := now.Add(ledger.leaseDuration).Format(time.RFC3339Nano)
	if record.ExpiresAt != wantExpiry {
		t.Fatalf("heartbeat expiry = %s, want %s", record.ExpiresAt, wantExpiry)
	}
	if err := ledger.refreshLease(path, "other-nonce"); err == nil {
		t.Fatal("foreign nonce refreshed lease")
	}
	if err := ledger.releaseLease(path, "other-nonce"); err == nil {
		t.Fatal("wrong lease nonce release succeeded")
	}
	if !ledger.pathExists(path) {
		t.Fatal("foreign nonce released lease")
	}
	if err := ledger.releaseLease(path, nonce); err != nil {
		t.Fatal(err)
	}
	if ledger.pathExists(path) {
		t.Fatal("lease owner failed to release lease")
	}
}

func TestCrossProcessLedgerWritersSerialize(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	const writers = 6
	commands := make([]*exec.Cmd, writers)
	for index := range writers {
		command := exec.Command(os.Args[0], "-test.run=^TestLedgerCrossProcessHelper$")
		command.Env = append(os.Environ(),
			"AWF_TELEMETRY_HELPER=1",
			"AWF_TELEMETRY_PROJECT="+ledger.paths.project,
			"AWF_TELEMETRY_EFFORT="+metadata.EffortID,
			"AWF_TELEMETRY_INDEX="+strconv.Itoa(index),
		)
		commands[index] = command
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatalf("cross-process writer: %v", err)
		}
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil || len(read.Events) != writers+1 || len(read.Integrity) != 0 {
		t.Fatalf("cross-process result: events=%d integrity=%+v err=%v", len(read.Events), read.Integrity, err)
	}
}

func TestLedgerCrossProcessHelper(t *testing.T) {
	if os.Getenv("AWF_TELEMETRY_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	ledger, err := NewLedger(os.Getenv("AWF_TELEMETRY_PROJECT"))
	if err != nil {
		t.Fatal(err)
	}
	index := os.Getenv("AWF_TELEMETRY_INDEX")
	eventMap := validEvent("usage_observed", 0)
	eventMap["eventId"] = "process-event-" + index
	eventMap["observationId"] = "process-observation-" + index
	eventMap["effortId"] = os.Getenv("AWF_TELEMETRY_EFFORT")
	eventMap["sessionId"] = "process-session-" + index
	if _, err := ledger.Append(context.Background(), mustJSON(t, eventMap)); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentLedgerWritersSerialize(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	const writers = 20
	var wait sync.WaitGroup
	errorsFound := make(chan error, writers)
	for index := 0; index < writers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			raw := passiveEvent(t, fmt.Sprintf("event-%d", index+2), fmt.Sprintf("observation-%d", index+2), metadata.EffortID, nil)
			if _, err := ledger.Append(context.Background(), raw); err != nil {
				errorsFound <- err
			}
		}()
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent append: %v", err)
	}
	read, err := ledger.ReadEffort(metadata.EffortID)
	if err != nil || len(read.Events) != writers+1 || len(read.Integrity) != 0 {
		t.Fatalf("concurrent result: events=%d integrity=%v err=%v", len(read.Events), read.Integrity, err)
	}
}

func TestStagingNameRoundTrip(t *testing.T) {
	t.Parallel()
	name := stagingName("effort.with.dots", "nonce")
	effortID, nonce, err := parseStagingName(name)
	if err != nil || effortID != "effort.with.dots" || nonce != "nonce" {
		t.Fatalf("staging round trip = %q %q %v", effortID, nonce, err)
	}
	for _, invalid := range []string{"", ".", "bad...", "***.nonce"} {
		if _, _, err := parseStagingName(invalid); err == nil {
			t.Errorf("invalid staging name %q accepted", invalid)
		}
	}
}

func TestRecoveryValidatesStagingNonceAgainstCreationLease(t *testing.T) {
	t.Parallel()
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Hour)
	ledger.ops.now = func() time.Time { return now }
	staging := filepath.Join(ledger.paths.staging, stagingName("staged-effort", "staging-nonce"))
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-2 * time.Hour)
	if err := os.Chtimes(staging, old, old); err != nil {
		t.Fatal(err)
	}
	record, _ := json.Marshal(leaseRecord{Nonce: "different-nonce", Owner: "owner", ExpiresAt: now.Add(time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(ledger.paths.creationLease("staged-effort"), record, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ledger.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if !hasIntegrityCode(report.Ambiguous, "ambiguous-staging-lease") || !ledger.pathExists(staging) {
		t.Fatalf("mismatched creation lease guessed: %+v", report)
	}
}

func TestLedgerRecoveryTable(t *testing.T) {
	t.Parallel()
	ledger, metadata, _ := createTestEffort(t)
	now := time.Now().Add(time.Hour)
	ledger.ops.now = func() time.Time { return now }

	staging := filepath.Join(ledger.paths.staging, stagingName("orphan", "nonce"))
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	old := now.Add(-2 * time.Hour)
	if err := os.Chtimes(staging, old, old); err != nil {
		t.Fatal(err)
	}

	pendingID := "pending-effort"
	pending := tombstoneRecord{Nonce: "prune-nonce", State: "pending"}
	pendingRaw, _ := json.Marshal(pending)
	if err := os.WriteFile(ledger.paths.tombstone(pendingID), pendingRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	trash := filepath.Join(ledger.paths.trash, stagingName(pendingID, pending.Nonce))
	if err := os.Mkdir(trash, 0o700); err != nil {
		t.Fatal(err)
	}

	ambiguousID := "ambiguous-effort"
	if err := os.WriteFile(ledger.paths.tombstone(ambiguousID), []byte(`{"nonce":"x","state":"pending"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	originalRemoveAll := ledger.ops.removeAll
	leaseObserved := false
	committedObserved := false
	ledger.ops.removeAll = func(path string) error {
		if path == trash {
			leaseObserved = ledger.pathExists(ledger.paths.appendLease(pendingID))
			tombstoneRaw, _ := os.ReadFile(ledger.paths.tombstone(pendingID))
			tombstone, _ := readTombstone(tombstoneRaw)
			committedObserved = tombstone.State == "committed"
		}
		return originalRemoveAll(path)
	}
	originalSyncDir := ledger.ops.syncDir
	syncedTrash := false
	ledger.ops.syncDir = func(path string) error {
		if path == ledger.paths.trash {
			syncedTrash = true
		}
		return originalSyncDir(path)
	}
	report, err := ledger.Recover()
	if err != nil {
		t.Fatal(err)
	}
	if leaseObserved || !committedObserved || !syncedTrash {
		t.Fatalf("tombstone recovery durability/lease: lease=%v committed=%v trashSync=%v", leaseObserved, committedObserved, syncedTrash)
	}
	if len(report.Recovered) < 2 || len(report.Ambiguous) != 1 {
		t.Fatalf("unexpected recovery report: %+v", report)
	}
	if ledger.pathExists(staging) || ledger.pathExists(trash) {
		t.Fatal("recoverable staging or committed trash remains")
	}
	if !ledger.pathExists(ledger.paths.effort(metadata.EffortID)) {
		t.Fatal("committed effort was removed by recovery")
	}
}

func createTestEffort(t *testing.T) (*Ledger, EffortMetadata, json.RawMessage) {
	t.Helper()
	ledger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	metadata, first := testCreation(t)
	if _, err := ledger.CreateEffort(metadata, first); err != nil {
		t.Fatal(err)
	}
	return ledger, metadata, first
}

func newTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".awf"), 0o700); err != nil {
		t.Fatal(err)
	}
	return root
}

func testCreation(t *testing.T) (EffortMetadata, json.RawMessage) {
	t.Helper()
	event := validEvent("effort_created", 0)
	payload := event["payload"].(map[string]any)
	metadata := EffortMetadata{
		EffortID:     event["effortId"].(string),
		CreatedAt:    event["timestamp"].(string),
		CreationMode: CreationMode(payload["creationMode"].(string)),
		Origin: &OriginMetadata{
			EffortID:     payload["originEffortId"].(string),
			TrajectoryID: payload["originTrajectoryId"].(string),
			AnchorID:     payload["originAnchorId"].(string),
		},
	}
	return metadata, mustJSON(t, event)
}

func passiveEvent(t *testing.T, eventID, observationID, effortID string, predecessors []string) json.RawMessage {
	t.Helper()
	event := validEvent("usage_observed", 0)
	event["eventId"] = eventID
	event["observationId"] = observationID
	event["effortId"] = effortID
	if predecessors != nil {
		event["predecessors"] = predecessors
	}
	return mustJSON(t, event)
}
