package telemetry

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRecoversStableStaleLease(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)
	now := time.Now()
	ledger.ops.now = func() time.Time { return now }
	raw, _ := json.Marshal(leaseRecord{Nonce: "stale", Owner: "owner", ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339Nano)})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	nonce, err := ledger.acquireLease(context.Background(), path)
	if err != nil || nonce == "" || nonce == "stale" {
		t.Fatalf("stale lease acquisition = %q %v", nonce, err)
	}
	if err := ledger.releaseLease(path, nonce); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryStagingRemainingBranches(t *testing.T) {
	ledger, _, _ := createTestEffort(t)
	if err := os.Remove(ledger.paths.leaseGuard()); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(ledger.paths.leaseGuard(), 0o700); err != nil {
		t.Fatal(err)
	}
	report := RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
	if err := ledger.recoverLeases(&report); err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-lease-path") {
		t.Fatalf("directory operation guard recovery = %+v %v", report, err)
	}
	if err := os.Remove(ledger.paths.leaseGuard()); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(ledger.paths.staging, stagingName("actually-fresh", "nonce"))
	if err := os.Mkdir(fresh, 0o700); err != nil {
		t.Fatal(err)
	}
	report = RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
	if err := ledger.recoverStaging(&report); err != nil || !ledger.pathExists(fresh) {
		t.Fatalf("fresh staging recovery = %+v %v", report, err)
	}

	infoFaultName := stagingName("info-fault", "nonce")
	if err := os.Mkdir(filepath.Join(ledger.paths.staging, infoFaultName), 0o700); err != nil {
		t.Fatal(err)
	}
	originalReadDir := ledger.ops.readDir
	ledger.ops.readDir = func(path string) ([]os.DirEntry, error) {
		if path == ledger.paths.staging {
			return []os.DirEntry{faultDirEntry{name: infoFaultName, directory: true}}, nil
		}
		return originalReadDir(path)
	}
	if err := ledger.recoverStaging(&report); err == nil {
		t.Fatal("staging Info failure accepted")
	}
	ledger.ops.readDir = originalReadDir

	old := filepath.Join(ledger.paths.staging, stagingName("lease-decode", "nonce"))
	if err := os.Mkdir(old, 0o700); err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(old, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger.paths.creationLease("lease-decode"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report = RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
	if err := ledger.recoverStaging(&report); err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-staging-lease") {
		t.Fatalf("staging lease decode recovery = %+v %v", report, err)
	}

	if err := os.Remove(ledger.paths.creationLease("lease-decode")); err != nil {
		t.Fatal(err)
	}
	originalRemoveAll := ledger.ops.removeAll
	ledger.ops.removeAll = func(path string) error {
		if path == old {
			return errInjected
		}
		return originalRemoveAll(path)
	}
	if err := ledger.recoverStaging(&report); err == nil {
		t.Fatal("expired staging remove failure accepted")
	}
}

type faultDirEntry struct {
	name      string
	directory bool
}

func (f faultDirEntry) Name() string               { return f.name }
func (f faultDirEntry) IsDir() bool                { return f.directory }
func (f faultDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (f faultDirEntry) Info() (os.FileInfo, error) { return nil, errInjected }

func tombstoneRecoveryLedger(t *testing.T, state string, effort, trash bool) (*Ledger, string, string) {
	t.Helper()
	ledger, _, _ := createTestEffort(t)
	id := "recovery-target"
	nonce := "prune-nonce"
	if effort {
		if err := os.MkdirAll(ledger.paths.effort(id), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	trashPath := filepath.Join(ledger.paths.trash, stagingName(id, nonce))
	if trash {
		if err := os.Mkdir(trashPath, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(tombstoneRecord{Nonce: nonce, State: state})
	if err := os.WriteFile(ledger.paths.tombstone(id), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return ledger, id, trashPath
}

func TestTombstoneRecoveryRemainingFaultBranches(t *testing.T) {
	ledger, _, _ := tombstoneRecoveryLedger(t, "pending", true, false)
	originalLock := ledger.ops.lockLease
	ledger.ops.lockLease = func(context.Context, string) (func() error, error) { return nil, errInjected }
	if err := ledger.recoverTombstones(&RecoveryReport{}); err == nil {
		t.Fatal("tombstone recovery lease failure accepted")
	}
	ledger.ops.lockLease = originalLock

	var id string
	for _, operation := range []string{"inspect-tombstone", "read-tombstone", "inspect-effort", "remove-tombstone", "sync-tombstone"} {
		ledger, id, _ = tombstoneRecoveryLedger(t, "pending", true, false)
		tombstone := ledger.paths.tombstone(id)
		effort := ledger.paths.effort(id)
		switch operation {
		case "inspect-tombstone", "inspect-effort":
			original := ledger.ops.inspect
			ledger.ops.inspect = func(root, target string, directory bool) error {
				if (operation == "inspect-tombstone" && target == tombstone) || (operation == "inspect-effort" && target == effort) {
					return errInjected
				}
				return original(root, target, directory)
			}
		case "read-tombstone":
			original := ledger.ops.readFile
			ledger.ops.readFile = func(target string) ([]byte, error) {
				if target == tombstone {
					return nil, errInjected
				}
				return original(target)
			}
		case "remove-tombstone":
			original := ledger.ops.remove
			ledger.ops.remove = func(target string) error {
				if target == tombstone {
					return errInjected
				}
				return original(target)
			}
		case "sync-tombstone":
			original := ledger.ops.syncDir
			ledger.ops.syncDir = func(target string) error {
				if target == ledger.paths.tombstones {
					return errInjected
				}
				return original(target)
			}
		}
		if err := ledger.recoverTombstones(&RecoveryReport{}); operation == "inspect-tombstone" || operation == "inspect-effort" {
			if err != nil {
				t.Errorf("%s should report ambiguity, got %v", operation, err)
			}
		} else if err == nil {
			t.Errorf("%s failure accepted", operation)
		}
	}

	for _, operation := range []string{"promote", "inspect-trash", "remove-trash", "sync-tombstone", "sync-trash"} {
		ledger, id, trash := tombstoneRecoveryLedger(t, "pending", false, true)
		tombstone := ledger.paths.tombstone(id)
		switch operation {
		case "inspect-trash":
			original := ledger.ops.inspect
			ledger.ops.inspect = func(root, target string, directory bool) error {
				if target == trash {
					return errInjected
				}
				return original(root, target, directory)
			}
		case "promote":
			original := ledger.ops.openFile
			ledger.ops.openFile = func(target string, flag int, mode fs.FileMode) (syncedFile, error) {
				if target == tombstone+".commit" {
					return nil, errInjected
				}
				return original(target, flag, mode)
			}
		case "remove-trash", "sync-tombstone", "sync-trash":
			// Start committed so the fault targets trash cleanup directly.
			raw, _ := json.Marshal(tombstoneRecord{Nonce: "prune-nonce", State: "committed"})
			if err := os.WriteFile(tombstone, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			if operation == "remove-trash" {
				original := ledger.ops.removeAll
				ledger.ops.removeAll = func(target string) error {
					if target == trash {
						return errInjected
					}
					return original(target)
				}
			} else {
				original := ledger.ops.syncDir
				ledger.ops.syncDir = func(target string) error {
					if (operation == "sync-tombstone" && target == ledger.paths.tombstones) || (operation == "sync-trash" && target == ledger.paths.trash) {
						return errInjected
					}
					return original(target)
				}
			}
		}
		err := ledger.recoverTombstones(&RecoveryReport{})
		if operation == "inspect-trash" {
			if err != nil {
				t.Errorf("inspect-trash should report ambiguity, got %v", err)
			}
		} else if err == nil {
			t.Errorf("%s failure accepted", operation)
		}
	}
}

func TestCommittedTombstoneRecoveryHeartbeatAndReleaseFailures(t *testing.T) {
	ledger, _, _ := tombstoneRecoveryLedger(t, "committed", false, true)
	ledger.ops.sleep = func(context.Context, time.Duration) error { return errInjected }
	if err := ledger.recoverTombstones(&RecoveryReport{}); err == nil {
		t.Fatal("recovery heartbeat failure accepted")
	}

	ledger, _, _ = tombstoneRecoveryLedger(t, "committed", false, true)
	originalLock := ledger.ops.lockLease
	calls := 0
	ledger.ops.lockLease = func(ctx context.Context, path string) (func() error, error) {
		calls++
		if calls == 2 {
			return nil, errInjected
		}
		return originalLock(ctx, path)
	}
	if err := ledger.recoverTombstones(&RecoveryReport{}); err == nil {
		t.Fatal("recovery lease release failure accepted")
	}
}
