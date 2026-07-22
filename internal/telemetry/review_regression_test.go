package telemetry

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRecoveryFinishesSyncedTombstoneCommitTemporary(t *testing.T) {
	ledger, id, trash := tombstoneRecoveryLedger(t, "pending", false, true)
	committed, _ := json.Marshal(tombstoneRecord{Nonce: "prune-nonce", State: "committed"})
	if err := ledger.writeSynced(ledger.paths.tombstone(id)+".commit", append(committed, '\n')); err != nil {
		t.Fatal(err)
	}

	report, err := ledger.Recover()
	if err != nil || len(report.Ambiguous) != 0 || !containsString(report.Recovered, id) || ledger.pathExists(trash) || ledger.pathExists(ledger.paths.tombstone(id)+".commit") {
		t.Fatalf("commit-temporary restart recovery = %#v, %v", report, err)
	}
	raw, err := os.ReadFile(ledger.paths.tombstone(id))
	if err != nil {
		t.Fatal(err)
	}
	record, err := readTombstone(raw)
	if err != nil || record.State != "committed" || record.Nonce != "prune-nonce" {
		t.Fatalf("recovered tombstone = %#v, %v", record, err)
	}
}

func TestTombstoneCommitTemporaryRecoveryBranches(t *testing.T) {
	setup := func(t *testing.T, base, temporary []byte) (*Ledger, string, string) {
		t.Helper()
		ledger := newRetentionLedger(t)
		path := ledger.paths.tombstone("target")
		if base != nil {
			if err := os.WriteFile(path, base, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		temporaryPath := path + ".commit"
		if err := os.WriteFile(temporaryPath, temporary, 0o600); err != nil {
			t.Fatal(err)
		}
		return ledger, path, temporaryPath
	}
	pending := []byte(`{"nonce":"nonce","state":"pending"}`)
	committed := []byte(`{"nonce":"nonce","state":"committed"}`)
	for _, test := range []struct {
		name      string
		base      []byte
		temporary []byte
		fault     string
		wantError bool
	}{
		{name: "inspect", base: pending, temporary: committed, fault: "inspect"},
		{name: "read", base: pending, temporary: committed, fault: "read", wantError: true},
		{name: "invalid", base: pending, temporary: []byte(`{}`)},
		{name: "not-committed", base: pending, temporary: pending},
		{name: "no-base", temporary: committed},
		{name: "base-read", base: pending, temporary: committed, fault: "base-read", wantError: true},
		{name: "invalid-base", base: []byte(`{}`), temporary: committed},
		{name: "nonce-mismatch", base: []byte(`{"nonce":"other","state":"pending"}`), temporary: committed},
		{name: "rename", base: pending, temporary: committed, fault: "rename", wantError: true},
		{name: "redundant", base: committed, temporary: committed},
		{name: "remove", base: committed, temporary: committed, fault: "remove", wantError: true},
		{name: "sync", base: pending, temporary: committed, fault: "sync", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			ledger, path, temporary := setup(t, test.base, test.temporary)
			originalInspect, originalRead := ledger.ops.inspect, ledger.ops.readFile
			originalRename, originalRemove, originalSync := ledger.ops.rename, ledger.ops.remove, ledger.ops.syncDir
			switch test.fault {
			case "inspect":
				ledger.ops.inspect = func(root, target string, directory bool) error {
					if target == temporary {
						return errInjected
					}
					return originalInspect(root, target, directory)
				}
			case "read", "base-read":
				ledger.ops.readFile = func(target string) ([]byte, error) {
					if test.fault == "read" && target == temporary || test.fault == "base-read" && target == path {
						return nil, errInjected
					}
					return originalRead(target)
				}
			case "rename":
				ledger.ops.rename = func(string, string) error { return errInjected }
			case "remove":
				ledger.ops.remove = func(string) error { return errInjected }
			case "sync":
				ledger.ops.syncDir = func(target string) error {
					if target == ledger.paths.tombstones {
						return errInjected
					}
					return originalSync(target)
				}
			}
			report := RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
			err := ledger.recoverTombstoneCommitTemporary(&report, "target", path, temporary)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %v", err, test.wantError)
			}
			if err == nil && test.name != "redundant" && test.name != "sync" && len(report.Recovered) == 0 && len(report.Ambiguous) == 0 {
				t.Fatal("recovery branch produced no result")
			}
			ledger.ops.inspect, ledger.ops.readFile = originalInspect, originalRead
			ledger.ops.rename, ledger.ops.remove, ledger.ops.syncDir = originalRename, originalRemove, originalSync
		})
	}

	ledger := newRetentionLedger(t)
	ledger.ops.readDir = func(string) ([]os.DirEntry, error) { return nil, errInjected }
	if _, err := ledger.recoverTombstoneUnderLease(&RecoveryReport{}, "target"); err == nil {
		t.Fatal("trash re-read failure accepted")
	}

	ledger = newRetentionLedger(t)
	path := ledger.paths.tombstone("target")
	if err := os.WriteFile(path, pending, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".commit", committed, 0o600); err != nil {
		t.Fatal(err)
	}
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(target string) ([]byte, error) {
		if target == path+".commit" {
			return nil, errInjected
		}
		return originalRead(target)
	}
	if _, err := ledger.recoverTombstoneUnderLease(&RecoveryReport{}, "target"); err == nil {
		t.Fatal("commit-temporary recovery failure accepted")
	}

	ledger = newRetentionLedger(t)
	orphan := filepath.Join(ledger.paths.trash, stagingName("target", "nonce"))
	if err := os.Mkdir(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	report := RecoveryReport{}
	if _, err := ledger.recoverTombstoneUnderLease(&report, "target"); err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-trash-path") {
		t.Fatalf("orphan trash re-read = %#v, %v", report, err)
	}
}

func TestApplyLifecycleReportsHeldLeaseHeartbeatAndReleaseFailures(t *testing.T) {
	newRequest := func(t *testing.T) (*Ledger, RouteLifecycleRequest) {
		t.Helper()
		ledger, metadata, firstRaw := createTestEffort(t)
		first, err := ValidateEvent(firstRaw)
		if err != nil {
			t.Fatal(err)
		}
		base := LifecycleRequestBase{Action: "select-route", EventID: "route", IdempotencyKey: "route-key", EffortID: metadata.EffortID, SessionID: first.SessionID, Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Predecessors: []string{first.EventID}}
		return ledger, RouteLifecycleRequest{LifecycleRequestBase: base, Route: "direct"}
	}
	ledger, request := newRequest(t)
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errInjected }
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("lifecycle heartbeat failure accepted")
	}

	ledger, request = newRequest(t)
	originalRemove := ledger.ops.remove
	ledger.ops.remove = func(path string) error {
		if path == ledger.paths.appendLease(request.EffortID) {
			return errInjected
		}
		return originalRemove(path)
	}
	if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
		t.Fatal("lifecycle lease release failure accepted")
	}
}

func TestRetentionUsesCurrentTerminalEpochDespiteClockSkew(t *testing.T) {
	ledger := newRetentionLedger(t)
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	firstTerminal := created.Add(100 * 24 * time.Hour)
	currentTerminal := created.Add(20 * 24 * time.Hour)
	writeRetentionEffort(t, ledger, "skewed", created, firstTerminal, EffortCompleted)

	read, err := ledger.ReadEffort("skewed")
	if err != nil {
		t.Fatal(err)
	}
	events := append([]EventEnvelope(nil), read.Events...)
	events = appendEvent(events, "reopen-two", "effort_reopened", EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "epoch-two", AnchorID: "anchor"})
	events[len(events)-1].Timestamp = created.Add(10 * 24 * time.Hour).Format(time.RFC3339Nano)
	events = appendFinishedPhase(events, "epoch-two-investigation", "investigation", "epoch-two")
	events = appendFinishedPhase(events, "epoch-two-retrospective", "retrospective", "epoch-two")
	events = appendEvent(events, "complete-two", "effort_completed", EffortTerminalPayload{TerminalEpoch: 2})
	for index := len(read.Events) + 1; index < len(events)-1; index++ {
		events[index].Timestamp = created.Add(time.Duration(index) * time.Hour).Format(time.RFC3339Nano)
	}
	events[len(events)-1].Timestamp = currentTerminal.Format(time.RFC3339Nano)
	var stream []byte
	for _, event := range events {
		event.EffortID = "skewed"
		raw, _ := json.Marshal(event)
		stream = append(stream, raw...)
		stream = append(stream, '\n')
	}
	if err := os.WriteFile(ledger.paths.stream("skewed", "session"), stream, 0o600); err != nil {
		t.Fatal(err)
	}

	candidate, terminal, err := ledger.readRetentionCandidate("skewed")
	if err != nil || !terminal || !candidate.TerminalTimestamp.Equal(currentTerminal) {
		t.Fatalf("current epoch candidate = %#v, %v, %v", candidate, terminal, err)
	}
	ledger.ops.now = func() time.Time { return currentTerminal.Add(2 * 24 * time.Hour) }
	result, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, true)
	if err != nil || !reflect.DeepEqual(result.Candidates, []string{"skewed"}) {
		t.Fatalf("clock-skew retention = %#v, %v", result, err)
	}
}

func TestRecoveryRereadsPruneStateUnderEffortLease(t *testing.T) {
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	writeRetentionEffort(t, ledger, "target", now, now.Add(time.Hour), EffortAbandoned)
	renamed := make(chan struct{})
	resume := make(chan struct{})
	originalRename := ledger.ops.rename
	ledger.ops.rename = func(oldPath, newPath string) error {
		if oldPath == ledger.paths.effort("target") {
			if err := originalRename(oldPath, newPath); err != nil {
				return err
			}
			close(renamed)
			<-resume
			return nil
		}
		return originalRename(oldPath, newPath)
	}
	pruneDone := make(chan error, 1)
	go func() {
		pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce")
		if err == nil && !pruned {
			err = errInjected
		}
		pruneDone <- err
	}()
	<-renamed
	recoveryDone := make(chan struct {
		report RecoveryReport
		err    error
	}, 1)
	go func() {
		report, err := ledger.Recover()
		recoveryDone <- struct {
			report RecoveryReport
			err    error
		}{report, err}
	}()
	select {
	case result := <-recoveryDone:
		t.Fatalf("recovery did not wait for prune lease: %#v, %v", result.report, result.err)
	case <-time.After(25 * time.Millisecond):
	}
	close(resume)
	if err := <-pruneDone; err != nil {
		t.Fatal(err)
	}
	result := <-recoveryDone
	if result.err != nil || hasIntegrityCode(result.report.Ambiguous, "ambiguous-trash-path") || ledger.pathExists(ledger.paths.effort("target")) {
		t.Fatalf("leased recovery after prune = %#v, %v", result.report, result.err)
	}
}

func TestAppendAndReopenLinearizeWithPruneLease(t *testing.T) {
	for _, operation := range []string{"append", "reopen"} {
		t.Run(operation+"-first", func(t *testing.T) { testWriterPruneLinearization(t, operation, true) })
		t.Run("prune-first-"+operation, func(t *testing.T) { testWriterPruneLinearization(t, operation, false) })
	}
}

func testWriterPruneLinearization(t *testing.T, operation string, writerFirst bool) {
	t.Helper()
	ledger := newRetentionLedger(t)
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	writeRetentionEffort(t, ledger, "target", now.Add(-time.Hour), now, EffortCompleted)
	read, err := ledger.ReadEffort("target")
	if err != nil {
		t.Fatal(err)
	}
	terminalID := read.Events[len(read.Events)-1].EventID
	base := LifecycleRequestBase{Action: "reopen", EventID: "reopen-" + operation, IdempotencyKey: "key-" + operation, EffortID: "target", SessionID: "session", Timestamp: now.Add(time.Hour).Format(time.RFC3339Nano), Predecessors: []string{terminalID}}
	rawEvent := causalEvent(base.EventID, base.SessionID, "effort_reopened", base.Predecessors, EffortReopenedPayload{TerminalEpoch: 2, TrajectoryID: "new-trajectory", AnchorID: "anchor"})
	rawEvent.EffortID, rawEvent.Timestamp, rawEvent.IdempotencyKey = base.EffortID, base.Timestamp, base.IdempotencyKey
	rawEvent.TrajectoryID = "new-trajectory"
	raw, _ := json.Marshal(rawEvent)
	write := func() error {
		if operation == "append" {
			_, err := ledger.Append(context.Background(), raw)
			return err
		}
		_, err := ledger.ApplyLifecycle(context.Background(), ReopenLifecycleRequest{LifecycleRequestBase: base, TrajectoryID: "new-trajectory", AnchorID: "anchor"})
		return err
	}

	writerDone := make(chan error, 1)
	pruneDone := make(chan struct {
		pruned bool
		err    error
	}, 1)
	if writerFirst {
		blocked := make(chan struct{})
		resume := make(chan struct{})
		originalOpen := ledger.ops.openFile
		var paused atomic.Bool
		ledger.ops.openFile = func(path string, flag int, mode os.FileMode) (syncedFile, error) {
			if path == ledger.paths.stream("target", "session") && flag&os.O_APPEND != 0 && paused.CompareAndSwap(false, true) {
				close(blocked)
				<-resume
			}
			return originalOpen(path, flag, mode)
		}
		go func() { writerDone <- write() }()
		<-blocked
		go func() {
			pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce")
			pruneDone <- struct {
				pruned bool
				err    error
			}{pruned, err}
		}()
		if !ledger.pathExists(ledger.paths.appendLease("target")) {
			t.Fatal("writer did not hold effort lease")
		}
		close(resume)
		if err := <-writerDone; err != nil {
			t.Fatal(err)
		}
		prune := <-pruneDone
		if prune.err != nil || prune.pruned {
			t.Fatalf("prune did not preserve reopened effort: %#v", prune)
		}
		read, err := ledger.ReadEffort("target")
		if err != nil || ProjectLifecycle(read.Events).State != EffortActive {
			t.Fatalf("writer-first effort = %#v, %v", read, err)
		}
		return
	}

	blocked := make(chan struct{})
	resume := make(chan struct{})
	originalRename := ledger.ops.rename
	ledger.ops.rename = func(oldPath, newPath string) error {
		if oldPath == ledger.paths.effort("target") {
			close(blocked)
			<-resume
		}
		return originalRename(oldPath, newPath)
	}
	go func() {
		pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce")
		pruneDone <- struct {
			pruned bool
			err    error
		}{pruned, err}
	}()
	<-blocked
	go func() { writerDone <- write() }()
	select {
	case err := <-writerDone:
		t.Fatalf("writer did not contend on prune lease: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	close(resume)
	prune := <-pruneDone
	if prune.err != nil || !prune.pruned {
		t.Fatalf("prune-first result = %#v", prune)
	}
	if err := <-writerDone; err == nil || !strings.Contains(err.Error(), "pruned") {
		t.Fatalf("late writer was not refused: %v", err)
	}
	if ledger.pathExists(ledger.paths.effort("target")) || !ledger.pathExists(filepath.Join(ledger.paths.tombstones, "target.json")) {
		t.Fatal("late writer recreated pruned effort")
	}
}

func TestConcurrentMutationsConflictAcrossPassiveTips(t *testing.T) {
	create := causalEvent("create", "root", "effort_created", nil, EffortCreatedPayload{CheckpointID: "x", CreationMode: "independent"})
	leftTip := passiveProjectionEvent("left-tip", "")
	leftTip.SessionID, leftTip.Predecessors = "left", []string{"create"}
	rightTip := passiveProjectionEvent("right-tip", "")
	rightTip.SessionID, rightTip.Predecessors = "right", []string{"create"}
	left := causalEvent("left-route", "left", "route_selected", []string{"left-tip"}, RoutePayload{Route: "direct"})
	right := causalEvent("right-route", "right", "route_selected", []string{"right-tip"}, RoutePayload{Route: "plan"})

	projection := ProjectLifecycle([]EventEnvelope{create, leftTip, rightTip, left, right})
	if projection.Route != "" || !hasInvalidEvent(projection.Invalid, left.EventID) || !hasInvalidEvent(projection.Invalid, right.EventID) {
		t.Fatalf("distinct passive tips hid an incomparable conflict: %#v", projection)
	}
}

func TestCausalLifecycleIsInvariantUnderEventIDRenaming(t *testing.T) {
	build := func(ids map[string]string) []EventEnvelope {
		create := causalEvent(ids["create"], "root", "effort_created", nil, EffortCreatedPayload{CheckpointID: "x", CreationMode: "independent"})
		started := causalEvent(ids["started"], "tree", "trajectory_started", []string{ids["create"]}, TrajectoryPayload{TrajectoryID: "parent", AnchorID: "parent-anchor"})
		started.TrajectoryID = "parent"
		associated := causalEvent(ids["associated"], "session", "session_associated", []string{ids["started"]}, SessionAssociatedPayload{AssociationOrigin: "manual", TrajectoryID: "parent"})
		forked := causalEvent(ids["forked"], "tree", "trajectory_forked", []string{ids["associated"]}, TrajectoryForkedPayload{TrajectoryID: "child", ParentTrajectoryID: "parent", ForkAnchorID: "fork-anchor"})
		forked.TrajectoryID = "child"
		resumed := causalEvent(ids["resumed"], "session", "trajectory_resumed", []string{ids["forked"]}, TrajectoryPayload{TrajectoryID: "parent", AnchorID: "parent-anchor"})
		resumed.TrajectoryID = "parent"
		return []EventEnvelope{create, started, associated, forked, resumed}
	}
	first := ProjectLifecycle(build(map[string]string{"create": "z-create", "started": "a-start", "associated": "z-associate", "forked": "a-fork", "resumed": "z-resume"}))
	second := ProjectLifecycle(build(map[string]string{"create": "a-create", "started": "z-start", "associated": "a-associate", "forked": "z-fork", "resumed": "a-resume"}))
	if len(first.Invalid) != 0 || len(second.Invalid) != 0 || first.ActiveTrajectoryID != "parent" || second.ActiveTrajectoryID != "parent" || len(first.Associations) != len(second.Associations) {
		t.Fatalf("renaming event IDs changed causal trajectory state: first=%#v second=%#v", first, second)
	}
}

func TestRepairWithoutApplicableSourceHasNoEffect(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "excluded-route", "route_selected", RoutePayload{Route: "direct"})
	replacement, _ := json.Marshal(RoutePayload{Route: "plan"})
	events = appendEvent(events, "repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"excluded-route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	projection := projectLifecycle(events, map[string]bool{"excluded-route": true})
	if projection.EffectApplied["excluded-route"] || projection.EffectApplied["repair"] {
		t.Fatalf("repair without an applicable source changed effects: %#v", projection)
	}
}

func TestRepairsConflictWhenSourceSetsOverlap(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "route", "route_selected", RoutePayload{Route: "direct"})
	events = appendEvent(events, "phase", "phase_started", PhaseStartedPayload{Phase: "brainstorming"})
	replacement, _ := json.Marshal(RoutePayload{Route: "plan"})
	left := causalEvent("left-repair", "left", "repair_applied", []string{"phase"}, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"route", "phase"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	right := causalEvent("right-repair", "right", "repair_applied", []string{"phase"}, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"phase"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	projection := ProjectLifecycle(append(events, left, right))
	if !hasInvalidEvent(projection.Invalid, left.EventID) || !hasInvalidEvent(projection.Invalid, right.EventID) {
		t.Fatalf("overlapping repair source sets did not conflict: %#v", projection)
	}
	disjoint := right
	disjoint.Payload, _ = json.Marshal(RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"other"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	if repairSourcesOverlap(left, disjoint) {
		t.Fatal("disjoint repair source sets overlap")
	}
}

func TestTrajectoryCloseRejectsUnknownInactiveAndNonActive(t *testing.T) {
	base := lifecycleBaseEvents()
	unknown := appendEvent(base, "unknown-close", "trajectory_closed", TrajectoryPayload{TrajectoryID: "missing", AnchorID: "anchor"})
	if projection := ProjectLifecycle(unknown); !hasInvalidDetail(projection.Invalid, "unknown-close", "unknown") {
		t.Fatalf("unknown close accepted: %#v", projection)
	}

	closed := appendEvent(base, "started", "trajectory_started", TrajectoryPayload{TrajectoryID: "one", AnchorID: "one-anchor"})
	closed = appendEvent(closed, "closed", "trajectory_closed", TrajectoryPayload{TrajectoryID: "one", AnchorID: "one-anchor"})
	closed = appendEvent(closed, "closed-again", "trajectory_closed", TrajectoryPayload{TrajectoryID: "one", AnchorID: "one-anchor"})
	if projection := ProjectLifecycle(closed); !hasInvalidDetail(projection.Invalid, "closed-again", "inactive") {
		t.Fatalf("inactive close accepted: %#v", projection)
	}

	nonActive := appendEvent(base, "one", "trajectory_started", TrajectoryPayload{TrajectoryID: "one", AnchorID: "one-anchor"})
	nonActive = appendEvent(nonActive, "two", "trajectory_started", TrajectoryPayload{TrajectoryID: "two", AnchorID: "two-anchor"})
	nonActive = appendEvent(nonActive, "close-one", "trajectory_closed", TrajectoryPayload{TrajectoryID: "one", AnchorID: "one-anchor"})
	if projection := ProjectLifecycle(nonActive); !hasInvalidDetail(projection.Invalid, "close-one", "non-active") {
		t.Fatalf("non-active close accepted: %#v", projection)
	}
}

func TestReaderMarksSupersededSourceRecordUnapplied(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	route := lifecycleRaw(t, "source-route", "route-key", metadata.EffortID, "route_selected", []string{"event-id"}, RoutePayload{Route: "direct"})
	replacement, _ := json.Marshal(RoutePayload{Route: "plan"})
	repair := lifecycleRaw(t, "repair", "repair-key", metadata.EffortID, "repair_applied", []string{"source-route"}, RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"source-route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	file, err := openAppend(ledger.paths.stream(metadata.EffortID, "session-id"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(append(append(route, '\n'), append(repair, '\n')...)); err != nil {
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
	states := map[string]bool{}
	for _, record := range read.Records {
		if record.Event != nil {
			states[record.Event.EventID] = record.Applied
		}
	}
	if len(read.Events) != 3 || states["source-route"] || !states["repair"] || read.EffectApplied["source-route"] {
		t.Fatalf("superseded record was not retained unapplied: records=%#v mask=%#v", read.Records, read.EffectApplied)
	}
}

func TestSupersededEvidenceIsRetainedButUnapplied(t *testing.T) {
	events := lifecycleBaseEvents()
	events = appendEvent(events, "source-route", "route_selected", RoutePayload{Route: "direct"})
	replacement, _ := json.Marshal(RoutePayload{Route: "plan"})
	events = appendEvent(events, "repair", "repair_applied", RepairAppliedPayload{ProposalKind: "supersede-event", SourceEventIDs: []string{"source-route"}, Replacement: RepairReplacement{EventKind: "route_selected", Payload: replacement}})
	descendant := passiveProjectionEvent("descendant", "")
	descendant.Predecessors = []string{"source-route"}
	events = append(events, descendant)

	projection := ProjectWorkflow(EffortRead{Metadata: EffortMetadata{EffortID: "effort"}, Events: events})
	if !containsString(projection.EvidenceEventIDs, "source-route") || projection.EffectApplied["source-route"] || containsString(projection.AllWorkEventIDs, "source-route") || !projection.EffectApplied["repair"] || hasIssue(projection.Integrity, "broken-predecessor") {
		t.Fatalf("superseded evidence/effect projection is wrong: %#v", projection)
	}
}
