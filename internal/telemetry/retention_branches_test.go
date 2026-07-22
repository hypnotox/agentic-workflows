package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestRetentionMutationErrorAndSuccessBranches(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	newTerminal := func(t *testing.T) *Ledger {
		ledger := newRetentionLedger(t)
		ledger.ops.now = func() time.Time { return now }
		writeRetentionEffort(t, ledger, "target", now.Add(-72*time.Hour), now.Add(-48*time.Hour), EffortAbandoned)
		return ledger
	}

	ledger := newTerminal(t)
	ledger.ops.nonce = func() (string, error) { return "", errInjected }
	result, err := ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, false)
	if err == nil || !reflect.DeepEqual(result.Candidates, []string{"target"}) {
		t.Fatalf("retain nonce failure = %#v, %v", result, err)
	}

	ledger = newTerminal(t)
	ledger.ops.nonce = func() (string, error) {
		writeTombstoneForTest(t, ledger, "target", "other", "pending")
		return "wanted", nil
	}
	result, err = ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, false)
	if err == nil || !reflect.DeepEqual(result.Candidates, []string{"target"}) {
		t.Fatalf("retain prune failure = %#v, %v", result, err)
	}

	ledger = newTerminal(t)
	result, err = ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, false)
	if err != nil || !reflect.DeepEqual(result.Pruned, []string{"target"}) {
		t.Fatalf("retain prune success = %#v, %v", result, err)
	}

	ledger = newTerminal(t)
	if _, err := ledger.Purge(context.Background(), "bad/id", true); err == nil {
		t.Fatal("purge accepted unsafe ID")
	}
	ledger.ops.nonce = func() (string, error) { return "", errInjected }
	if _, err := ledger.Purge(context.Background(), "target", true); err == nil {
		t.Fatal("purge nonce failure accepted")
	}

	ledger = newTerminal(t)
	originalNonce := ledger.ops.nonce
	injected := false
	ledger.ops.nonce = func() (string, error) {
		if !injected {
			injected = true
			// A completed effort, unlike this abandoned fixture, can reopen. Replace
			// the fixture before selection in the dedicated race test instead; here
			// force prune's terminal recheck result through the same method seam.
			if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
				t.Fatal(err)
			}
			writeRetentionEffort(t, ledger, "target", now, time.Time{}, EffortActive)
		}
		return originalNonce()
	}
	result, err = ledger.Purge(context.Background(), "target", true)
	if err == nil || !reflect.DeepEqual(result.Skipped, []string{"target"}) {
		t.Fatalf("purge late activation = %#v, %v", result, err)
	}
}

func TestPrunePendingCommittedAndAmbiguousStateMatrix(t *testing.T) {
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	newTerminal := func(t *testing.T) *Ledger {
		ledger := newRetentionLedger(t)
		writeRetentionEffort(t, ledger, "target", now, now.Add(time.Hour), EffortAbandoned)
		return ledger
	}
	trashPath := func(ledger *Ledger, nonce string) string {
		return filepath.Join(ledger.paths.trash, stagingName("target", nonce))
	}

	ledger := newRetentionLedger(t)
	writeTombstoneForTest(t, ledger, "other-target", "nonce", "pending")
	if err := os.Remove(ledger.paths.tombstone("other-target")); err != nil {
		t.Fatal(err)
	}
	writeRetentionEffort(t, ledger, "target", now, time.Time{}, EffortActive)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	if pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err != nil || pruned || !ledger.pathExists(ledger.paths.effort("target")) {
		t.Fatalf("pending tombstone active recheck = %v, %v", pruned, err)
	}

	ledger = newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	if err := os.Mkdir(trashPath(ledger, "nonce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("effort plus matching trash accepted")
	}

	ledger = newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "committed")
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("committed tombstone plus effort accepted")
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("pending tombstone without effort or trash accepted")
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	if err := os.Mkdir(trashPath(ledger, "nonce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err != nil || !pruned {
		t.Fatalf("pending rename recovery = %v, %v", pruned, err)
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "committed")
	if err := os.Mkdir(trashPath(ledger, "nonce"), 0o700); err != nil {
		t.Fatal(err)
	}
	if pruned, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err != nil || !pruned || ledger.pathExists(trashPath(ledger, "nonce")) {
		t.Fatalf("committed trash retry = %v, %v", pruned, err)
	}

	ledger = newTerminal(t)
	if err := os.WriteFile(filepath.Join(ledger.paths.trash, "malformed"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("malformed trash state accepted")
	}

	ledger = newTerminal(t)
	if err := os.Mkdir(trashPath(ledger, "other"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("other trash nonce accepted")
	}
}

func TestPruneAndTrashFaultBranches(t *testing.T) {
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	newTerminal := func(t *testing.T) *Ledger {
		ledger := newRetentionLedger(t)
		writeRetentionEffort(t, ledger, "target", now, now.Add(time.Hour), EffortAbandoned)
		return ledger
	}

	ledger := newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	originalInspect := ledger.ops.inspect
	ledger.ops.inspect = func(root, path string, directory bool) error {
		if path == ledger.paths.tombstone("target") {
			return errInjected
		}
		return originalInspect(root, path, directory)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("tombstone inspection failure accepted")
	}

	ledger = newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	originalInspect = ledger.ops.inspect
	ledger.ops.inspect = func(root, path string, directory bool) error {
		if path == ledger.paths.effort("target") {
			return errInjected
		}
		return originalInspect(root, path, directory)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("pending effort recheck failure accepted")
	}

	ledger = newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	originalSync := ledger.ops.syncDir
	ledger.ops.syncDir = func(path string) error {
		if path == ledger.paths.tombstones {
			return errInjected
		}
		return originalSync(path)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("pending tombstone sync failure accepted")
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "committed")
	originalSync = ledger.ops.syncDir
	ledger.ops.syncDir = func(path string) error {
		if path == ledger.paths.tombstones {
			return errInjected
		}
		return originalSync(path)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("committed tombstone sync failure accepted")
	}

	ledger = newTerminal(t)
	writeTombstoneForTest(t, ledger, "target", "nonce", "pending")
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(path string) ([]byte, error) {
		if path == ledger.paths.tombstone("target") {
			return nil, errInjected
		}
		return originalRead(path)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("tombstone read failure accepted")
	}

	ledger = newTerminal(t)
	originalReadDir := ledger.ops.readDir
	ledger.ops.readDir = func(path string) ([]os.DirEntry, error) {
		if path == ledger.paths.trash {
			return nil, errInjected
		}
		return originalReadDir(path)
	}
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("trash enumeration failure accepted")
	}

	ledger = newTerminal(t)
	ledger.ops.inspect = func(string, string, bool) error { return errInjected }
	if err := ledger.deleteTrash(filepath.Join(ledger.paths.trash, "anything")); err == nil {
		t.Fatal("trash inspection failure accepted")
	}
	ledger.ops.inspect = func(string, string, bool) error { return os.ErrNotExist }
	if err := ledger.deleteTrash(filepath.Join(ledger.paths.trash, "missing")); err != nil {
		t.Fatalf("missing committed trash was not idempotent: %v", err)
	}

	ledger = newTerminal(t)
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errors.New("heartbeat failed") }
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("prune heartbeat failure accepted")
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "committed")
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errors.New("heartbeat failed") }
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("committed retry heartbeat failure accepted")
	}

	ledger = newTerminal(t)
	if err := os.RemoveAll(ledger.paths.effort("target")); err != nil {
		t.Fatal(err)
	}
	writeTombstoneForTest(t, ledger, "target", "nonce", "committed")
	committedTrash := filepath.Join(ledger.paths.trash, stagingName("target", "nonce"))
	if err := os.Mkdir(committedTrash, 0o700); err != nil {
		t.Fatal(err)
	}
	ledger.ops.removeAll = func(string) error { return errInjected }
	if _, err := ledger.pruneEffort(context.Background(), "target", "nonce"); err == nil {
		t.Fatal("committed retry deletion failure accepted")
	}
}

func writeTombstoneForTest(t *testing.T, ledger *Ledger, effortID, nonce, state string) {
	t.Helper()
	raw, _ := json.Marshal(tombstoneRecord{Nonce: nonce, State: state})
	if err := os.WriteFile(ledger.paths.tombstone(effortID), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
