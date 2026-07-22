package telemetry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLeaseMutationFaultMatrix(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)
	now := time.Now()
	ledger.ops.now = func() time.Time { return now }
	valid := func(expiry time.Time) []byte {
		raw, _ := json.Marshal(leaseRecord{Nonce: "nonce", Owner: "owner", ExpiresAt: expiry.Format(time.RFC3339Nano)})
		return raw
	}
	write := func(raw []byte) {
		t.Helper()
		_ = os.Remove(path)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	originalInspect := ledger.ops.inspect
	ledger.ops.inspect = func(root, target string, directory bool) error {
		if target == path {
			return errInjected
		}
		return originalInspect(root, target, directory)
	}
	if _, err := ledger.removeStaleLease(path); err == nil {
		t.Fatal("lease inspection failure accepted")
	}
	ledger.ops.inspect = originalInspect

	write(valid(now.Add(-time.Hour)))
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(target string) ([]byte, error) {
		if target == path {
			return nil, os.ErrNotExist
		}
		return originalRead(target)
	}
	if removed, err := ledger.removeStaleLease(path); err != nil || !removed {
		t.Fatalf("lease disappearing after inspection = %v %v", removed, err)
	}
	ledger.ops.readFile = func(target string) ([]byte, error) {
		if target == path {
			return nil, errInjected
		}
		return originalRead(target)
	}
	if _, err := ledger.removeStaleLease(path); err == nil {
		t.Fatal("lease read failure accepted")
	}
	ledger.ops.readFile = originalRead

	write(valid(now.Add(-time.Hour)))
	originalRemove := ledger.ops.remove
	ledger.ops.remove = func(target string) error {
		if target == path {
			return errInjected
		}
		return originalRemove(target)
	}
	if _, err := ledger.removeStaleLease(path); err == nil {
		t.Fatal("stale lease remove failure accepted")
	}
	ledger.ops.remove = originalRemove

	write(valid(now.Add(-time.Hour)))
	originalSync := ledger.ops.syncDir
	ledger.ops.syncDir = func(target string) error {
		if target == ledger.paths.leases {
			return errInjected
		}
		return originalSync(target)
	}
	if _, err := ledger.removeStaleLease(path); err == nil {
		t.Fatal("stale lease parent sync failure accepted")
	}
	ledger.ops.syncDir = originalSync

	write([]byte(`{`))
	if err := ledger.releaseLease(path, "nonce"); err == nil {
		t.Fatal("malformed lease release succeeded")
	}
	if _, err := ledger.acquireLease(context.Background(), path); err == nil {
		t.Fatal("ambiguous stale lease accepted by acquisition")
	}
}

func TestHeartbeatAndSyncedWriteFaultMatrix(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)
	nonce, err := ledger.acquireExclusiveLease(path, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(target string) ([]byte, error) {
		if target == path {
			return nil, errInjected
		}
		return originalRead(target)
	}
	if err := ledger.refreshLease(path, nonce); err == nil {
		t.Fatal("heartbeat lease read failure accepted")
	}
	ledger.ops.readFile = originalRead

	originalOpen := ledger.ops.openFile
	ledger.ops.openFile = func(target string, flag int, mode fs.FileMode) (syncedFile, error) {
		if filepath.Ext(target) == ".heartbeat" {
			return nil, errInjected
		}
		return originalOpen(target, flag, mode)
	}
	if err := ledger.refreshLease(path, nonce); err == nil {
		t.Fatal("heartbeat replacement write failure accepted")
	}
	ledger.ops.openFile = originalOpen

	originalRename := ledger.ops.rename
	ledger.ops.rename = func(oldPath, newPath string) error {
		if newPath == path {
			return errInjected
		}
		return originalRename(oldPath, newPath)
	}
	if err := ledger.refreshLease(path, nonce); err == nil {
		t.Fatal("heartbeat replacement rename failure accepted")
	}
	ledger.ops.rename = originalRename

	for _, failure := range []string{"short", "write", "sync", "close"} {
		target := filepath.Join(ledger.paths.staging, "synced-"+failure)
		ledger.ops.openFile = func(string, int, fs.FileMode) (syncedFile, error) { return &faultSyncedFile{failure: failure}, nil }
		if err := ledger.writeSynced(target, []byte("value")); err == nil {
			t.Errorf("writeSynced %s failure accepted", failure)
		}
	}
	ledger.ops.openFile = originalOpen
}

func TestLeaseAndStagingDecodeErrorMatrix(t *testing.T) {
	ledger, metadata, _ := createTestEffort(t)
	path := ledger.paths.appendLease(metadata.EffortID)
	for _, raw := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"nonce":"n","owner":"o","expiresAt":"bad"}`),
	} {
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ledger.readLeaseNonce(path); err == nil {
			t.Errorf("invalid lease nonce record accepted: %s", raw)
		}
	}
	originalInspect := ledger.ops.inspect
	ledger.ops.inspect = func(string, string, bool) error { return errInjected }
	if _, err := ledger.readLeaseNonce(path); err == nil {
		t.Fatal("lease nonce inspection failure accepted")
	}
	ledger.ops.inspect = originalInspect
	originalRead := ledger.ops.readFile
	ledger.ops.readFile = func(string) ([]byte, error) { return nil, errInjected }
	if _, err := ledger.readLeaseNonce(path); err == nil {
		t.Fatal("lease nonce read failure accepted")
	}
	ledger.ops.readFile = originalRead

	invalidUTF8 := base64.RawURLEncoding.EncodeToString([]byte{0xff}) + ".nonce"
	unsafeID := base64.RawURLEncoding.EncodeToString([]byte("../bad")) + ".nonce"
	for _, name := range []string{invalidUTF8, unsafeID} {
		if _, _, err := parseStagingName(name); err == nil {
			t.Errorf("invalid staging name %q accepted", name)
		}
	}
}

func TestRecoveryLeaseFilenameValidation(t *testing.T) {
	if _, err := recoveryLeaseEffortID(".append.json"); err == nil {
		t.Fatal("empty effort ID in lease filename accepted")
	}
	if _, err := recoveryLeaseEffortID("unknown.json"); err == nil {
		t.Fatal("unknown lease suffix accepted")
	}
}

func TestEnsureLayoutAndRecoveryFaults(t *testing.T) {
	root := newTestProject(t)
	paths, err := newLedgerPaths(root)
	if err != nil {
		t.Fatal(err)
	}
	ledger := &Ledger{paths: paths, ops: defaultLedgerOps(), leaseDuration: defaultLeaseDuration, leaseGrace: defaultLeaseGrace, leasePoll: defaultLeasePoll, leaseHeartbeat: 10 * time.Second}
	ledger.ops.mkdirAll = func(string, fs.FileMode) error { return errInjected }
	if err := ledger.ensureLayout(); err == nil {
		t.Fatal("layout mkdir failure accepted")
	}

	ledger, _, _ = createTestEffort(t)
	if err := os.WriteFile(filepath.Join(ledger.paths.leases, "malformed.append.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := ledger.Recover()
	if err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-lease") {
		t.Fatalf("malformed lease recovery = %+v %v", report, err)
	}

	staging := filepath.Join(ledger.paths.staging, stagingName("inspect-fault", "nonce"))
	if err := os.Mkdir(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	originalInspect := ledger.ops.inspect
	ledger.ops.inspect = func(root, target string, directory bool) error {
		if target == staging {
			return errInjected
		}
		return originalInspect(root, target, directory)
	}
	report = RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
	if err := ledger.recoverStaging(&report); err != nil || !hasIntegrityCode(report.Ambiguous, "ambiguous-staging-path") {
		t.Fatalf("staging inspection recovery = %+v %v", report, err)
	}
	ledger.ops.inspect = originalInspect
}

func TestPromoteTombstoneFaults(t *testing.T) {
	for _, operation := range []string{"write", "rename"} {
		ledger, _, _ := createTestEffort(t)
		path := ledger.paths.tombstone("promote")
		if err := os.WriteFile(path, []byte(`{"nonce":"n","state":"pending"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if operation == "write" {
			original := ledger.ops.openFile
			ledger.ops.openFile = func(target string, flag int, mode fs.FileMode) (syncedFile, error) {
				if target == path+".commit" {
					return nil, errInjected
				}
				return original(target, flag, mode)
			}
		} else {
			original := ledger.ops.rename
			ledger.ops.rename = func(oldPath, newPath string) error {
				if newPath == path {
					return errInjected
				}
				return original(oldPath, newPath)
			}
		}
		if err := ledger.promoteTombstone(path, tombstoneRecord{Nonce: "n", State: "pending"}); err == nil {
			t.Errorf("promote tombstone %s failure accepted", operation)
		}
	}
}
