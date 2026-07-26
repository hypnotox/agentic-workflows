package telemetry

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"time"
)

var errInjected = errors.New("injected filesystem failure")

func TestLedgerInjectedFilesystemFailures(t *testing.T) {
	operations := []string{"mkdir", "open", "read", "readdir", "lstat", "rename", "link", "remove", "removeall", "inspect", "nonce", "owner", "syncdir", "locklease"}
	for _, scenario := range []string{"create", "append", "read", "recover", "retain", "purge"} {
		for _, operation := range operations {
			for occurrence := 1; occurrence <= 100; occurrence++ {
				root := newTestProject(t)
				ledger, err := NewLedger(root)
				if err != nil {
					t.Fatal(err)
				}
				metadata, first := testCreation(t)
				if scenario != "create" {
					if _, err := ledger.CreateEffort(metadata, first); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "retain" || scenario == "purge" {
					now := time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC)
					ledger.ops.now = func() time.Time { return now }
					writeRetentionEffort(t, ledger, "fault-terminal", now.Add(-48*time.Hour), now.Add(-24*time.Hour), EffortAbandoned)
				}
				injectOperationFailure(ledger, operation, occurrence)
				switch scenario {
				case "create":
					_, _ = ledger.CreateEffort(metadata, first)
				case "append":
					_, _ = ledger.Append(context.Background(), passiveEvent(t, "fault-event", "fault-observation", metadata.EffortID, nil))
				case "read":
					_, _ = ledger.ReadEffort(metadata.EffortID)
				case "recover":
					_, _ = ledger.Recover()
				case "retain":
					_, _ = ledger.Retain(context.Background(), RetentionPolicy{MaxCompletedEffortAgeDays: 1}, false)
				case "purge":
					_, _ = ledger.Purge(context.Background(), "fault-terminal", true)
				}
			}
		}
	}
}

func TestProtocol21LifecycleWritersPropagateDurabilityFaults(t *testing.T) {
	origin := OriginMetadata{EffortID: "parent", TrajectoryID: "parent-trajectory", AnchorID: "parent-anchor"}
	creationRequests := []LifecycleRequest{
		AdoptLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "adopt", IdempotencyKey: "adopt-key", EventID: "adopt", EffortID: "adopted", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, Phase: "implementation", Workflow: "executing-direct", TrajectoryID: "trajectory", AnchorID: "anchor"},
		StartDetourLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "start-detour", IdempotencyKey: "detour-key", EventID: "detour", EffortID: "detour", SessionID: "session", Timestamp: "2026-07-22T00:00:00Z", Predecessors: []string{}}, CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-start", TrajectoryID: "child", AnchorID: "child-anchor", Workflow: "brainstorming"},
	}
	for _, request := range creationRequests {
		ledger, err := NewLedger(newTestProject(t))
		if err != nil {
			t.Fatal(err)
		}
		ledger.ops.openFile = func(string, int, fs.FileMode) (syncedFile, error) { return nil, errInjected }
		if _, err := ledger.ApplyLifecycle(context.Background(), request); err == nil {
			t.Fatalf("%T durability fault was hidden", request)
		}
	}

	adoptedLedger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	adopted := protocol21Envelope(t, "adopt", "effort_adopted", nil, EffortAdoptedPayload{CreationMode: "adopted", Phase: "implementation", Workflow: "executing-direct", TrajectoryID: "trajectory", AnchorID: "anchor", AssociationOrigin: "manual"})
	adopted.EffortID, adopted.SessionID = "continued", "session"
	if _, err := adoptedLedger.CreateEffort(EffortMetadata{EffortID: "continued", CreatedAt: adopted.Timestamp, CreationMode: "adopted"}, mustJSON(t, adopted)); err != nil {
		t.Fatal(err)
	}
	originalOpen := adoptedLedger.ops.openFile
	adoptedLedger.ops.openFile = func(path string, flag int, mode fs.FileMode) (syncedFile, error) {
		if path == adoptedLedger.paths.stream("continued", "session") {
			return nil, errInjected
		}
		return originalOpen(path, flag, mode)
	}
	continued := ContinuePhaseLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "continue-phase", IdempotencyKey: "continue-key", EventID: "continue", EffortID: "continued", SessionID: "session", Timestamp: "2026-07-22T00:00:01Z", Predecessors: []string{"adopt"}}, Phase: "implementation", StartEventID: "adopt", Workflow: "tdd", Activity: "tdd"}
	if _, err := adoptedLedger.ApplyLifecycle(context.Background(), continued); err == nil {
		t.Fatal("continuation durability fault was hidden")
	}

	detourLedger, err := NewLedger(newTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	started := protocol21Envelope(t, "detour", "detour_started", nil, DetourStartedPayload{CreationMode: "derived", Origin: origin, ReturnPhase: "implementation", ReturnPhaseStartEventID: "parent-start", TrajectoryID: "child", AnchorID: "child-anchor", Workflow: "brainstorming", AssociationOrigin: "detour"})
	started.EffortID, started.SessionID = "returned", "session"
	metadata := EffortMetadata{EffortID: "returned", CreatedAt: started.Timestamp, CreationMode: "derived", Origin: &origin, DetourReturn: &DetourReturnMetadata{SessionID: "session", Phase: "implementation", PhaseStartEventID: "parent-start"}}
	if _, err := detourLedger.CreateEffort(metadata, mustJSON(t, started)); err != nil {
		t.Fatal(err)
	}
	abandon := lifecycleRaw(t, "terminal", "terminal-key", "returned", "effort_abandoned", []string{"detour"}, EffortAbandonedPayload{TerminalEpoch: 1})
	if _, err := detourLedger.Append(context.Background(), abandon); err != nil {
		t.Fatal(err)
	}
	originalOpen = detourLedger.ops.openFile
	detourLedger.ops.openFile = func(path string, flag int, mode fs.FileMode) (syncedFile, error) {
		if path == detourLedger.paths.stream("returned", "session-id") {
			return nil, errInjected
		}
		return originalOpen(path, flag, mode)
	}
	returned := MarkDetourReturnedLifecycleRequest{LifecycleRequestBase: LifecycleRequestBase{Action: "mark-detour-returned", IdempotencyKey: "return-key", EventID: "return", EffortID: "returned", SessionID: "session-id", Timestamp: "2026-07-22T00:00:02Z", Predecessors: []string{"terminal"}}, TerminalOutcome: "abandoned", ParentAssociationEventID: "association"}
	if _, err := detourLedger.ApplyLifecycle(context.Background(), returned); err == nil {
		t.Fatal("detour return durability fault was hidden")
	}
}

func TestSyncedFileFailureSeams(t *testing.T) {
	for _, failure := range []string{"short", "write", "sync", "close"} {
		ledger, metadata, _ := createTestEffort(t)
		original := ledger.ops.openFile
		stream := ledger.paths.stream(metadata.EffortID, "session-id")
		ledger.ops.openFile = func(path string, flag int, mode fs.FileMode) (syncedFile, error) {
			if path == stream {
				return &faultSyncedFile{failure: failure}, nil
			}
			return original(path, flag, mode)
		}
		if _, err := ledger.Append(context.Background(), passiveEvent(t, "fault-event", "fault-observation", metadata.EffortID, nil)); err == nil {
			t.Errorf("%s file failure accepted", failure)
		}
	}
}

type faultSyncedFile struct{ failure string }

func (f *faultSyncedFile) Write(value []byte) (int, error) {
	switch f.failure {
	case "short":
		return len(value) - 1, nil
	case "write":
		return 0, errInjected
	default:
		return len(value), nil
	}
}
func (f *faultSyncedFile) Sync() error {
	if f.failure == "sync" {
		return errInjected
	}
	return nil
}
func (f *faultSyncedFile) Close() error {
	if f.failure == "close" {
		return errInjected
	}
	return nil
}

func injectOperationFailure(ledger *Ledger, operation string, wanted int) {
	count := 0
	hit := func() bool {
		count++
		return count == wanted
	}
	switch operation {
	case "mkdir":
		original := ledger.ops.mkdir
		ledger.ops.mkdir = func(path string, mode fs.FileMode) error {
			if hit() {
				return errInjected
			}
			return original(path, mode)
		}
	case "open":
		original := ledger.ops.openFile
		ledger.ops.openFile = func(path string, flag int, mode fs.FileMode) (syncedFile, error) {
			if hit() {
				return nil, errInjected
			}
			return original(path, flag, mode)
		}
	case "read":
		original := ledger.ops.readFile
		ledger.ops.readFile = func(path string) ([]byte, error) {
			if hit() {
				return nil, errInjected
			}
			return original(path)
		}
	case "readdir":
		original := ledger.ops.readDir
		ledger.ops.readDir = func(path string) ([]os.DirEntry, error) {
			if hit() {
				return nil, errInjected
			}
			return original(path)
		}
	case "lstat":
		original := ledger.ops.lstat
		ledger.ops.lstat = func(path string) (os.FileInfo, error) {
			if hit() {
				return nil, errInjected
			}
			return original(path)
		}
	case "rename":
		original := ledger.ops.rename
		ledger.ops.rename = func(oldPath, newPath string) error {
			if hit() {
				return errInjected
			}
			return original(oldPath, newPath)
		}
	case "link":
		original := ledger.ops.link
		ledger.ops.link = func(oldPath, newPath string) error {
			if hit() {
				return errInjected
			}
			return original(oldPath, newPath)
		}
	case "remove":
		original := ledger.ops.remove
		ledger.ops.remove = func(path string) error {
			if hit() {
				return errInjected
			}
			return original(path)
		}
	case "removeall":
		original := ledger.ops.removeAll
		ledger.ops.removeAll = func(path string) error {
			if hit() {
				return errInjected
			}
			return original(path)
		}
	case "inspect":
		original := ledger.ops.inspect
		ledger.ops.inspect = func(root, path string, directory bool) error {
			if hit() {
				return errInjected
			}
			return original(root, path, directory)
		}
	case "nonce":
		original := ledger.ops.nonce
		ledger.ops.nonce = func() (string, error) {
			if hit() {
				return "", errInjected
			}
			return original()
		}
	case "owner":
		original := ledger.ops.owner
		ledger.ops.owner = func() (string, error) {
			if hit() {
				return "", errInjected
			}
			return original()
		}
	case "syncdir":
		original := ledger.ops.syncDir
		ledger.ops.syncDir = func(path string) error {
			if hit() {
				return errInjected
			}
			return original(path)
		}
	case "locklease":
		original := ledger.ops.lockLease
		ledger.ops.lockLease = func(ctx context.Context, path string) (func() error, error) {
			if hit() {
				return nil, errInjected
			}
			return original(ctx, path)
		}
	}
}

var _ io.Writer = (*faultSyncedFile)(nil)
