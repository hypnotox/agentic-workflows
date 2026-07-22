package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultLeaseDuration = 30 * time.Second
	defaultLeaseGrace    = 30 * time.Second
	defaultLeasePoll     = 10 * time.Millisecond
)

type syncedFile interface {
	io.Writer
	Sync() error
	Close() error
}

type ledgerOps struct {
	mkdirAll  func(string, fs.FileMode) error
	openFile  func(string, int, fs.FileMode) (syncedFile, error)
	readFile  func(string) ([]byte, error)
	readDir   func(string) ([]os.DirEntry, error)
	lstat     func(string) (os.FileInfo, error)
	rename    func(string, string) error
	link      func(string, string) error
	remove    func(string) error
	removeAll func(string) error
	inspect   func(string, string, bool) error
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error
	nonce     func() (string, error)
	owner     func() (string, error)
	syncDir   func(string) error
	lockLease func(context.Context, string) (func() error, error)
}

func defaultLedgerOps() ledgerOps {
	return ledgerOps{
		mkdirAll: os.MkdirAll,
		openFile: func(path string, flag int, mode fs.FileMode) (syncedFile, error) {
			return os.OpenFile(path, flag, mode)
		},
		readFile:  os.ReadFile,
		readDir:   os.ReadDir,
		lstat:     os.Lstat,
		rename:    os.Rename,
		link:      os.Link,
		remove:    os.Remove,
		removeAll: os.RemoveAll,
		inspect:   inspectConfined,
		now:       time.Now,
		sleep: func(ctx context.Context, duration time.Duration) error {
			timer := time.NewTimer(duration)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
		nonce:     randomNonce,
		owner:     currentOwnerIdentity,
		syncDir:   syncDirectory,
		lockLease: lockLeaseOperations,
	}
}

type Ledger struct {
	paths              ledgerPaths
	ops                ledgerOps
	leaseDuration      time.Duration
	leaseGrace         time.Duration
	leasePoll          time.Duration
	leaseHeartbeat     time.Duration
	validateTransition func(EventEnvelope, []EventEnvelope) error
}

type AppendResult struct {
	Event      EventEnvelope
	Idempotent bool
}

type heldEffortLeaseKey struct{}

type leaseRecord struct {
	Nonce     string `json:"nonce"`
	Owner     string `json:"owner"`
	ExpiresAt string `json:"expiresAt"`
}

type tombstoneRecord struct {
	Nonce string `json:"nonce"`
	State string `json:"state"`
}

func NewLedger(root string) (*Ledger, error) {
	paths, err := newLedgerPaths(root)
	if err != nil {
		return nil, err
	}
	ledger := &Ledger{paths: paths, ops: defaultLedgerOps(), leaseDuration: defaultLeaseDuration, leaseGrace: defaultLeaseGrace, leasePoll: defaultLeasePoll, leaseHeartbeat: 10 * time.Second}
	if err := ledger.ensureLayout(); err != nil {
		return nil, err
	}
	return ledger, nil
}

func (l *Ledger) ensureLayout() error {
	// The caller supplies the project root, not a preselected metrics path. Both
	// existing anchor components are checked before MkdirAll can traverse them.
	if err := inspectProjectAnchor(l.paths.project, l.paths.awf); err != nil {
		return fmt.Errorf("inspect telemetry project anchor: %w", err)
	}
	for _, path := range []string{l.paths.root, l.paths.efforts, l.paths.leases, l.paths.staging, l.paths.tombstones, l.paths.trash} {
		if err := l.ops.mkdirAll(path, ownerOnlyMode); err != nil {
			return fmt.Errorf("create telemetry directory: %w", err)
		}
		if err := l.ops.inspect(l.paths.root, path, true); err != nil {
			return fmt.Errorf("inspect telemetry directory: %w", err)
		}
	}
	return nil
}

func (l *Ledger) CreateEffort(metadata EffortMetadata, first json.RawMessage) (AppendResult, error) {
	event, err := validateCreation(metadata, first)
	if err != nil {
		return AppendResult{}, err
	}
	// A tombstone permanently reserves the effort ID. Check before considering
	// retry success, then check again under the creation lease below.
	if l.pathExists(l.paths.tombstone(metadata.EffortID)) {
		return AppendResult{}, errors.New("effort ID was pruned")
	}
	leasePath := l.paths.creationLease(metadata.EffortID)
	nonce, err := l.acquireLease(context.Background(), leasePath)
	if err != nil {
		return AppendResult{}, fmt.Errorf("acquire creation lease: %w", err)
	}
	stopHeartbeat, heartbeatDone := l.startHeartbeat(context.Background(), leasePath, nonce)
	heartbeatFinished := false
	defer func() {
		stopHeartbeat()
		if !heartbeatFinished {
			<-heartbeatDone
		}
		_ = l.releaseLease(leasePath, nonce)
	}()

	if l.pathExists(l.paths.tombstone(metadata.EffortID)) {
		return AppendResult{}, errors.New("effort ID was pruned")
	}
	if retry, retryErr := l.identicalCreation(metadata, event, first); retryErr != nil {
		return AppendResult{}, retryErr
	} else if retry {
		return AppendResult{Event: event, Idempotent: true}, nil
	}
	if l.pathExists(l.paths.effort(metadata.EffortID)) {
		return AppendResult{}, errors.New("effort ID already exists or was pruned")
	}

	staging := filepath.Join(l.paths.staging, stagingName(metadata.EffortID, nonce))
	if err := l.ops.mkdirAll(filepath.Join(staging, "sessions"), ownerOnlyMode); err != nil {
		return AppendResult{}, fmt.Errorf("create effort staging directory: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = l.ops.removeAll(staging)
		}
	}()
	metadataBytes, err := json.Marshal(metadata)
	if err != nil { // coverage-ignore: validated metadata contains only JSON-safe values
		return AppendResult{}, fmt.Errorf("encode effort metadata: %w", err)
	}
	if err := l.writeSynced(filepath.Join(staging, "effort.json"), append(metadataBytes, '\n')); err != nil {
		return AppendResult{}, fmt.Errorf("write effort metadata: %w", err)
	}
	line, err := compactLine(first)
	if err != nil { // coverage-ignore: ValidateEvent already proved valid JSON
		return AppendResult{}, err
	}
	if err := l.writeSynced(filepath.Join(staging, "sessions", event.SessionID+".jsonl"), line); err != nil {
		return AppendResult{}, fmt.Errorf("write first event stream: %w", err)
	}
	if err := l.syncDir(filepath.Join(staging, "sessions")); err != nil {
		return AppendResult{}, err
	}
	if err := l.syncDir(staging); err != nil {
		return AppendResult{}, err
	}
	if err := l.ops.rename(staging, l.paths.effort(metadata.EffortID)); err != nil {
		return AppendResult{}, fmt.Errorf("commit effort creation: %w", err)
	}
	committed = true
	if err := l.syncDir(l.paths.efforts); err != nil {
		return AppendResult{}, fmt.Errorf("sync effort creation: %w", err)
	}
	heartbeatErr := finishHeartbeat(stopHeartbeat, heartbeatDone)
	heartbeatFinished = true
	if heartbeatErr != nil {
		return AppendResult{}, heartbeatErr
	}
	return AppendResult{Event: event}, nil
}

func (l *Ledger) Append(ctx context.Context, raw json.RawMessage) (AppendResult, error) {
	event, err := ValidateEvent(raw)
	if err != nil {
		return AppendResult{}, err
	}
	if event.Kind == "effort_created" {
		return AppendResult{}, errors.New("effort_created is valid only at atomic creation")
	}
	if err := validatePathIdentifier("effortId", event.EffortID); err != nil { // coverage-ignore: ValidateEvent already validated the effort identifier
		return AppendResult{}, err
	}
	if err := validatePathIdentifier("sessionId", event.SessionID); err != nil { // coverage-ignore: ValidateEvent already validated the session identifier
		return AppendResult{}, err
	}
	leasePath := l.paths.appendLease(event.EffortID)
	leaseHeld := ctx.Value(heldEffortLeaseKey{}) == event.EffortID
	var nonce string
	var stopHeartbeat context.CancelFunc
	var heartbeatDone <-chan error
	heartbeatFinished := leaseHeld
	if !leaseHeld {
		nonce, err = l.acquireLease(ctx, leasePath)
		if err != nil {
			return AppendResult{}, err
		}
		stopHeartbeat, heartbeatDone = l.startHeartbeat(ctx, leasePath, nonce)
		defer func() {
			stopHeartbeat()
			if !heartbeatFinished {
				<-heartbeatDone
			}
			_ = l.releaseLease(leasePath, nonce)
		}()
	}

	if l.pathExists(l.paths.tombstone(event.EffortID)) {
		return AppendResult{}, errors.New("effort was pruned")
	}
	effortPath := l.paths.effort(event.EffortID)
	if err := l.ops.inspect(l.paths.root, effortPath, true); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return AppendResult{}, errors.New("effort does not exist and will not be recreated")
		}
		return AppendResult{}, fmt.Errorf("inspect effort: %w", err)
	}
	metadata, err := l.readMetadata(event.EffortID)
	if err != nil {
		return AppendResult{}, err
	}
	if metadata.EffortID != event.EffortID {
		return AppendResult{}, errors.New("immutable effort metadata identity mismatch")
	}

	read, err := l.ReadEffort(event.EffortID)
	if err != nil {
		return AppendResult{}, err
	}
	for _, issue := range read.Integrity {
		if issue.Code == "partial-final-line" && issue.Scope == event.SessionID {
			return AppendResult{}, errors.New("event stream has an incomplete final line")
		}
	}
	for _, record := range read.Records {
		if record.Event == nil {
			continue
		}
		prior := *record.Event
		if prior.EventID == event.EventID || sameContractIdentity(prior, event) {
			equal, compareErr := eventsEqual(prior, raw)
			if compareErr != nil { // coverage-ignore: both events were validated before canonical comparison
				return AppendResult{}, compareErr
			}
			if equal && record.Applied {
				return AppendResult{Event: prior, Idempotent: true}, nil
			}
			if equal {
				return AppendResult{}, errors.New("existing duplicate event has no applied effect")
			}
			return AppendResult{}, errors.New("conflicting duplicate event identity")
		}
	}
	if descriptor.Payloads[string(event.Kind)].Class == "lifecycle" {
		projection := projectLifecycle(append(append([]EventEnvelope(nil), read.Events...), event), lifecycleExclusions(read))
		for _, issue := range projection.Invalid {
			if containsString(issue.EventIDs, event.EventID) {
				return AppendResult{}, fmt.Errorf("invalid lifecycle transition: %s", issue.Detail)
			}
		}
	}

	sessions := filepath.Join(effortPath, "sessions")
	if err := l.ops.inspect(l.paths.root, sessions, true); err != nil {
		return AppendResult{}, fmt.Errorf("inspect sessions directory: %w", err)
	}
	stream := l.paths.stream(event.EffortID, event.SessionID)
	streamExists := l.pathExists(stream)
	if streamExists {
		if err := l.ops.inspect(l.paths.root, stream, false); err != nil {
			return AppendResult{}, fmt.Errorf("inspect event stream: %w", err)
		}
	}
	line, err := compactLine(raw)
	if err != nil { // coverage-ignore: ValidateEvent already proved valid JSON
		return AppendResult{}, err
	}
	file, err := l.ops.openFile(stream, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return AppendResult{}, fmt.Errorf("open event stream: %w", err)
	}
	var written int
	if written, err = file.Write(line); err == nil && written != len(line) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return AppendResult{}, fmt.Errorf("append event: %w", err)
	}
	if closeErr != nil {
		return AppendResult{}, fmt.Errorf("close event stream: %w", closeErr)
	}
	if !streamExists {
		if err := l.syncDir(sessions); err != nil {
			return AppendResult{}, fmt.Errorf("sync new event stream: %w", err)
		}
	}
	if !leaseHeld {
		heartbeatErr := finishHeartbeat(stopHeartbeat, heartbeatDone)
		heartbeatFinished = true
		if heartbeatErr != nil {
			return AppendResult{}, heartbeatErr
		}
	}
	return AppendResult{Event: event}, nil
}

func validateCreation(metadata EffortMetadata, raw json.RawMessage) (EventEnvelope, error) {
	metadataRaw, err := json.Marshal(metadata)
	if err != nil { // coverage-ignore: metadata contains only JSON-safe values
		return EventEnvelope{}, err
	}
	if err := validateDescriptorObject("effort metadata", metadataRaw, "EffortMetadata"); err != nil {
		return EventEnvelope{}, err
	}
	if _, err := time.Parse(time.RFC3339Nano, metadata.CreatedAt); err != nil { // coverage-ignore: descriptor decoding already validated timestamp format
		return EventEnvelope{}, fmt.Errorf("effort metadata createdAt: %w", err)
	}
	event, err := ValidateEvent(raw)
	if err != nil {
		return EventEnvelope{}, err
	}
	if event.Kind != "effort_created" || event.EffortID != metadata.EffortID || event.Timestamp != metadata.CreatedAt {
		return EventEnvelope{}, errors.New("first event does not match immutable effort metadata")
	}
	var payload EffortCreatedPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil { // coverage-ignore: protocol validation proved payload shape
		return EventEnvelope{}, err
	}
	if payload.CheckpointID != metadata.CheckpointID || payload.CreationMode != metadata.CreationMode {
		return EventEnvelope{}, errors.New("first event creation payload differs from metadata")
	}
	wantOrigin := OriginMetadata{EffortID: payload.OriginEffortID, TrajectoryID: payload.OriginTrajectoryID, AnchorID: payload.OriginAnchorID}
	if (metadata.Origin == nil && wantOrigin != (OriginMetadata{})) || (metadata.Origin != nil && *metadata.Origin != wantOrigin) {
		return EventEnvelope{}, errors.New("first event origin differs from metadata")
	}
	return event, nil
}

func (l *Ledger) identicalCreation(metadata EffortMetadata, event EventEnvelope, first json.RawMessage) (bool, error) {
	if !l.pathExists(l.paths.effort(metadata.EffortID)) {
		return false, nil
	}
	existing, err := l.readMetadata(metadata.EffortID)
	if err != nil {
		return false, err
	}
	if !metadataEqual(existing, metadata) {
		return false, errors.New("effort ID collides with different immutable metadata")
	}
	stream := l.paths.stream(metadata.EffortID, event.SessionID)
	if err := l.ops.inspect(l.paths.root, stream, false); err != nil {
		return false, fmt.Errorf("inspect initial event stream: %w", err)
	}
	raw, err := l.ops.readFile(stream)
	if err != nil {
		return false, fmt.Errorf("read initial event stream: %w", err)
	}
	lines := splitJSONLines(raw)
	if len(lines.complete) == 0 {
		return false, errors.New("creation retry found no complete first event")
	}
	existingFirst, err := ValidateEvent(lines.complete[0])
	if err != nil {
		return false, fmt.Errorf("validate existing first event: %w", err)
	}
	equal, err := eventsEqual(existingFirst, first)
	if err != nil { // coverage-ignore: both creation events were validated before comparison
		return false, err
	}
	if !equal {
		return false, errors.New("effort ID collides with a different first event")
	}
	return true, nil
}

func metadataEqual(left, right EffortMetadata) bool {
	if left.EffortID != right.EffortID || left.CreatedAt != right.CreatedAt || left.CheckpointID != right.CheckpointID || left.CreationMode != right.CreationMode {
		return false
	}
	if left.Origin == nil || right.Origin == nil {
		return left.Origin == nil && right.Origin == nil
	}
	return *left.Origin == *right.Origin
}

func (l *Ledger) readMetadata(effortID string) (EffortMetadata, error) {
	path := filepath.Join(l.paths.effort(effortID), "effort.json")
	if err := l.ops.inspect(l.paths.root, path, false); err != nil {
		return EffortMetadata{}, fmt.Errorf("inspect effort metadata: %w", err)
	}
	raw, err := l.ops.readFile(path)
	if err != nil {
		return EffortMetadata{}, fmt.Errorf("read effort metadata: %w", err)
	}
	if err := validateJSONStructure(raw, "effort metadata", nil); err != nil {
		return EffortMetadata{}, err
	}
	if err := validateDescriptorObject("effort metadata", raw, "EffortMetadata"); err != nil {
		return EffortMetadata{}, err
	}
	var metadata EffortMetadata
	if err := json.Unmarshal(raw, &metadata); err != nil { // coverage-ignore: descriptor validation proved destination shape
		return EffortMetadata{}, err
	}
	return metadata, nil
}

func (l *Ledger) withLeaseOperation(ctx context.Context, operation func() error) error {
	unlock, err := l.ops.lockLease(ctx, l.paths.leaseGuard())
	if err != nil {
		return fmt.Errorf("lock lease operation guard: %w", err)
	}
	operationErr := operation()
	unlockErr := unlock()
	if operationErr != nil {
		return operationErr
	}
	if unlockErr != nil {
		return fmt.Errorf("unlock lease operation guard: %w", unlockErr)
	}
	return nil
}

func (l *Ledger) acquireExclusiveLease(path string, duration time.Duration) (string, error) {
	var acquired string
	err := l.withLeaseOperation(context.Background(), func() error {
		nonce, err := l.ops.nonce()
		if err != nil {
			return err
		}
		owner, err := l.ops.owner()
		if err != nil {
			return fmt.Errorf("determine lease owner: %w", err)
		}
		record := leaseRecord{Nonce: nonce, Owner: owner, ExpiresAt: l.ops.now().Add(duration).Format(time.RFC3339Nano)}
		raw, err := json.Marshal(record)
		if err != nil { // coverage-ignore: lease consists only of strings
			return err
		}
		temporary := path + "." + nonce + ".tmp"
		file, err := l.ops.openFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		leaseBytes := append(append([]byte(nil), raw...), '\n')
		var written int
		if written, err = file.Write(leaseBytes); err == nil && written != len(leaseBytes) {
			err = io.ErrShortWrite
		}
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err != nil {
			_ = l.ops.remove(temporary)
			return err
		}
		if closeErr != nil {
			_ = l.ops.remove(temporary)
			return closeErr
		}
		if err := l.ops.link(temporary, path); err != nil {
			_ = l.ops.remove(temporary)
			return err
		}
		_ = l.ops.remove(temporary)
		if err := l.syncDir(l.paths.leases); err != nil {
			_ = l.ops.remove(path)
			return err
		}
		acquired = nonce
		return nil
	})
	return acquired, err
}

func (l *Ledger) acquireLease(ctx context.Context, path string) (string, error) {
	for {
		nonce, err := l.acquireExclusiveLease(path, l.leaseDuration)
		if err == nil {
			return nonce, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return "", fmt.Errorf("acquire effort lease: %w", err)
		}
		stale, staleErr := l.removeStaleLease(path)
		if staleErr != nil {
			return "", staleErr
		}
		if stale {
			continue
		}
		if err := l.ops.sleep(ctx, l.leasePoll); err != nil {
			return "", fmt.Errorf("wait for effort lease: %w", err)
		}
	}
}

func (l *Ledger) startHeartbeat(ctx context.Context, path, nonce string) (context.CancelFunc, <-chan error) {
	heartbeatContext, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		for {
			if err := l.ops.sleep(heartbeatContext, l.leaseHeartbeat); err != nil {
				if errors.Is(err, context.Canceled) {
					done <- nil
					return
				}
				done <- fmt.Errorf("wait to heartbeat effort lease: %w", err)
				return
			}
			if err := l.refreshLease(path, nonce); err != nil {
				done <- fmt.Errorf("heartbeat effort lease: %w", err)
				return
			}
		}
	}()
	return cancel, done
}

func (l *Ledger) refreshLease(path, nonce string) error {
	return l.withLeaseOperation(context.Background(), func() error {
		if err := l.ops.inspect(l.paths.root, path, false); err != nil {
			return err
		}
		raw, err := l.ops.readFile(path)
		if err != nil {
			return err
		}
		var existing leaseRecord
		if err := json.Unmarshal(raw, &existing); err != nil || existing.Nonce != nonce {
			return errors.New("effort lease ownership changed")
		}
		existing.ExpiresAt = l.ops.now().Add(l.leaseDuration).Format(time.RFC3339Nano)
		updated, err := json.Marshal(existing)
		if err != nil { // coverage-ignore: lease consists only of strings
			return err
		}
		temporary := path + "." + nonce + ".heartbeat"
		if err := l.writeSynced(temporary, append(updated, '\n')); err != nil {
			_ = l.ops.remove(temporary)
			return err
		}
		if err := l.ops.rename(temporary, path); err != nil {
			_ = l.ops.remove(temporary)
			return err
		}
		return l.syncDir(l.paths.leases)
	})
}

func finishHeartbeat(cancel context.CancelFunc, done <-chan error) error {
	cancel()
	return <-done
}

func (l *Ledger) removeStaleLease(path string) (bool, error) {
	removed := false
	err := l.withLeaseOperation(context.Background(), func() error {
		if err := l.ops.inspect(l.paths.root, path, false); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				removed = true
				return nil
			}
			return fmt.Errorf("inspect effort lease: %w", err)
		}
		raw, err := l.ops.readFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			removed = true
			return nil
		}
		if err != nil {
			return fmt.Errorf("read effort lease: %w", err)
		}
		var record leaseRecord
		if err := json.Unmarshal(raw, &record); err != nil || record.Nonce == "" || record.Owner == "" {
			return errors.New("ambiguous effort lease record")
		}
		expires, err := time.Parse(time.RFC3339Nano, record.ExpiresAt)
		if err != nil {
			return errors.New("ambiguous effort lease expiry")
		}
		if !l.ops.now().After(expires.Add(l.leaseGrace)) {
			return nil
		}
		if err := l.ops.remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove stale effort lease: %w", err)
		}
		if err := l.syncDir(l.paths.leases); err != nil {
			return err
		}
		removed = true
		return nil
	})
	return removed, err
}

func (l *Ledger) releaseLease(path, nonce string) error {
	return l.withLeaseOperation(context.Background(), func() error {
		if err := l.ops.inspect(l.paths.root, path, false); err != nil {
			return fmt.Errorf("inspect released lease: %w", err)
		}
		raw, err := l.ops.readFile(path)
		if err != nil {
			return fmt.Errorf("read released lease: %w", err)
		}
		var record leaseRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return fmt.Errorf("decode released lease: %w", err)
		}
		if record.Nonce != nonce {
			return errors.New("released lease ownership changed")
		}
		if err := l.ops.remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return l.syncDir(l.paths.leases)
	})
}

func (l *Ledger) writeSynced(path string, raw []byte) error {
	file, err := l.ops.openFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	var written int
	if written, err = file.Write(raw); err == nil && written != len(raw) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func (l *Ledger) syncDir(path string) error {
	if err := l.ops.syncDir(path); err != nil {
		return fmt.Errorf("sync directory: %w", err)
	}
	return nil
}

func (l *Ledger) pathExists(path string) bool {
	_, err := l.ops.lstat(path)
	return err == nil
}

type RecoveryReport struct {
	Recovered []string
	Ambiguous []IntegrityIssue
}

func (l *Ledger) Recover() (RecoveryReport, error) {
	report := RecoveryReport{Recovered: []string{}, Ambiguous: []IntegrityIssue{}}
	if err := l.recoverLeases(&report); err != nil {
		return report, err
	}
	if err := l.recoverStaging(&report); err != nil {
		return report, err
	}
	if err := l.recoverTombstones(&report); err != nil {
		return report, err
	}
	sort.Strings(report.Recovered)
	unique := report.Recovered[:0]
	for _, effortID := range report.Recovered {
		if len(unique) == 0 || unique[len(unique)-1] != effortID {
			unique = append(unique, effortID)
		}
	}
	report.Recovered = unique
	return report, nil
}

func (l *Ledger) recoverLeases(report *RecoveryReport) error {
	entries, err := l.ops.readDir(l.paths.leases)
	if err != nil {
		return fmt.Errorf("read lease directory: %w", err)
	}
	for _, entry := range entries {
		if entry.Name() == ".operations.lock" {
			if entry.IsDir() {
				report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-lease-path", "leases", 0, nil, entry.Name()))
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-lease-path", "leases", 0, nil, entry.Name()))
			continue
		}
		effortID, nameErr := recoveryLeaseEffortID(entry.Name())
		if nameErr != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-lease-path", "leases", 0, nil, entry.Name()))
			continue
		}
		path := filepath.Join(l.paths.leases, entry.Name())
		removed, recoverErr := l.removeStaleLease(path)
		if recoverErr != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-lease", entry.Name(), 0, nil, recoverErr.Error()))
			continue
		}
		if removed {
			report.Recovered = append(report.Recovered, effortID)
		}
	}
	return nil
}

func recoveryLeaseEffortID(name string) (string, error) {
	for _, suffix := range []string{".create.json", ".append.json"} {
		if strings.HasSuffix(name, suffix) {
			effortID := strings.TrimSuffix(name, suffix)
			if err := validatePathIdentifier("effortId", effortID); err != nil {
				return "", err
			}
			return effortID, nil
		}
	}
	return "", errors.New("unknown lease filename")
}

func (l *Ledger) recoverStaging(report *RecoveryReport) error {
	entries, err := l.ops.readDir(l.paths.staging)
	if err != nil {
		return fmt.Errorf("read staging directory: %w", err)
	}
	for _, entry := range entries {
		effortID, stagingNonce, nameErr := parseStagingName(entry.Name())
		if !entry.IsDir() || nameErr != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-staging-path", "staging", 0, nil, entry.Name()))
			continue
		}
		stagingPath := filepath.Join(l.paths.staging, entry.Name())
		if inspectErr := l.ops.inspect(l.paths.root, stagingPath, true); inspectErr != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-staging-path", effortID, 0, nil, inspectErr.Error()))
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return fmt.Errorf("inspect staging entry: %w", statErr)
		}
		if !l.ops.now().After(info.ModTime().Add(l.leaseDuration + l.leaseGrace)) {
			continue
		}
		if l.pathExists(l.paths.effort(effortID)) {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-staging-commit", effortID, 0, nil, "staging and committed effort both exist"))
			continue
		}
		creationLease := l.paths.creationLease(effortID)
		if l.pathExists(creationLease) {
			leaseNonce, leaseErr := l.readLeaseNonce(creationLease)
			if leaseErr != nil || leaseNonce != stagingNonce {
				detail := "staging nonce does not match creation lease"
				if leaseErr != nil {
					detail = leaseErr.Error()
				}
				report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-staging-lease", effortID, 0, nil, detail))
			}
			continue
		}
		if err := l.ops.removeAll(stagingPath); err != nil {
			return fmt.Errorf("remove expired staging directory: %w", err)
		}
		report.Recovered = append(report.Recovered, effortID)
	}
	return nil
}

func (l *Ledger) readLeaseNonce(path string) (string, error) {
	if err := l.ops.inspect(l.paths.root, path, false); err != nil {
		return "", err
	}
	raw, err := l.ops.readFile(path)
	if err != nil {
		return "", err
	}
	var record leaseRecord
	if err := json.Unmarshal(raw, &record); err != nil || record.Nonce == "" || record.Owner == "" {
		return "", errors.New("ambiguous effort lease record")
	}
	if _, err := time.Parse(time.RFC3339Nano, record.ExpiresAt); err != nil {
		return "", errors.New("ambiguous effort lease expiry")
	}
	return record.Nonce, nil
}

func stagingName(effortID, nonce string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(effortID)) + "." + nonce
}

func parseStagingName(name string) (string, string, error) {
	encoded, nonce, found := strings.Cut(name, ".")
	if !found || encoded == "" || nonce == "" || strings.Contains(nonce, ".") {
		return "", "", errors.New("invalid staging name")
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", errors.New("invalid staging effort encoding")
	}
	if !utf8.Valid(raw) {
		return "", "", errors.New("invalid staging effort encoding")
	}
	effortID := string(raw)
	if err := validatePathIdentifier("effortId", effortID); err != nil {
		return "", "", err
	}
	return effortID, nonce, nil
}

func (l *Ledger) recoverTombstones(report *RecoveryReport) error {
	tombstoneEntries, err := l.ops.readDir(l.paths.tombstones)
	if err != nil {
		return fmt.Errorf("read tombstone directory: %w", err)
	}
	trashEntries, err := l.ops.readDir(l.paths.trash)
	if err != nil {
		return fmt.Errorf("read trash directory: %w", err)
	}

	// Enumeration is only a work list. Every decision below is made from state
	// re-read while holding the same per-effort lease used by append and prune.
	ids := map[string]bool{}
	for _, entry := range tombstoneEntries {
		name := entry.Name()
		id := ""
		if !entry.IsDir() && strings.HasSuffix(name, ".json.commit") {
			id = strings.TrimSuffix(name, ".json.commit")
		} else if !entry.IsDir() && strings.HasSuffix(name, ".json") {
			id = strings.TrimSuffix(name, ".json")
		}
		if id == "" || validatePathIdentifier("effortId", id) != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-tombstone-path", "tombstones", 0, nil, name))
			continue
		}
		ids[id] = true
	}
	for _, entry := range trashEntries {
		id, _, parseErr := parseStagingName(entry.Name())
		if parseErr != nil || !entry.IsDir() {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-trash-path", "trash", 0, nil, entry.Name()))
			continue
		}
		ids[id] = true
	}
	orderedIDs := make([]string, 0, len(ids))
	for id := range ids {
		orderedIDs = append(orderedIDs, id)
	}
	sort.Strings(orderedIDs)

	for _, effortID := range orderedIDs {
		leasePath := l.paths.appendLease(effortID)
		nonce, leaseErr := l.acquireLease(context.Background(), leasePath)
		if leaseErr != nil {
			return fmt.Errorf("acquire tombstone recovery lease: %w", leaseErr)
		}
		cancel, done := l.startHeartbeat(context.Background(), leasePath, nonce)
		trashToDelete, recoverErr := l.recoverTombstoneUnderLease(report, effortID)
		heartbeatErr := finishHeartbeat(cancel, done)
		releaseErr := l.releaseLease(leasePath, nonce)
		if recoverErr != nil {
			return recoverErr
		}
		if heartbeatErr != nil {
			return fmt.Errorf("finish tombstone recovery heartbeat: %w", heartbeatErr)
		}
		if releaseErr != nil {
			return fmt.Errorf("release tombstone recovery lease: %w", releaseErr)
		}
		if trashToDelete != "" {
			if err := l.deleteTrash(trashToDelete); err != nil {
				return err
			}
			report.Recovered = append(report.Recovered, effortID)
		}
	}
	return nil
}

func (l *Ledger) recoverTombstoneUnderLease(report *RecoveryReport, effortID string) (string, error) {
	path := l.paths.tombstone(effortID)
	temporary := path + ".commit"
	if l.pathExists(temporary) {
		if err := l.recoverTombstoneCommitTemporary(report, effortID, path, temporary); err != nil {
			return "", err
		}
	}

	trashEntries, err := l.ops.readDir(l.paths.trash)
	if err != nil {
		return "", fmt.Errorf("re-read trash directory: %w", err)
	}
	trashNames := []string{}
	for _, entry := range trashEntries {
		id, _, parseErr := parseStagingName(entry.Name())
		if parseErr == nil && entry.IsDir() && id == effortID {
			trashNames = append(trashNames, entry.Name())
		}
	}
	sort.Strings(trashNames)

	if !l.pathExists(path) {
		for _, name := range trashNames {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-trash-path", "trash", 0, nil, name))
		}
		return "", nil
	}
	if err := l.ops.inspect(l.paths.root, path, false); err != nil {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-tombstone-path", effortID, 0, nil, err.Error()))
		return "", nil
	}
	raw, err := l.ops.readFile(path)
	if err != nil {
		return "", fmt.Errorf("read tombstone: %w", err)
	}
	record, err := readTombstone(raw)
	if err != nil {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-tombstone", effortID, 0, nil, err.Error()))
		return "", nil
	}
	trashName := stagingName(effortID, record.Nonce)
	if len(trashNames) > 1 || len(trashNames) == 1 && trashNames[0] != trashName {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, "trash nonce does not match tombstone"))
		return "", nil
	}
	trash := filepath.Join(l.paths.trash, trashName)
	effortPath := l.paths.effort(effortID)
	effortExists := l.pathExists(effortPath)
	trashExists := len(trashNames) == 1
	if effortExists {
		if err := l.ops.inspect(l.paths.root, effortPath, true); err != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, err.Error()))
			return "", nil
		}
	}
	if trashExists {
		if err := l.ops.inspect(l.paths.root, trash, true); err != nil {
			report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, err.Error()))
			return "", nil
		}
	}
	if record.State == "pending" && effortExists && !trashExists {
		if err := l.ops.remove(path); err != nil {
			return "", fmt.Errorf("remove uncommitted tombstone: %w", err)
		}
		if err := l.syncDir(l.paths.tombstones); err != nil {
			return "", fmt.Errorf("sync removed tombstone: %w", err)
		}
		report.Recovered = append(report.Recovered, effortID)
		return "", nil
	}
	if record.State == "pending" && !effortExists && trashExists {
		if err := l.promoteTombstone(path, record); err != nil {
			return "", fmt.Errorf("commit pending tombstone: %w", err)
		}
		record.State = "committed"
	}
	if record.State == "committed" && !effortExists && trashExists {
		if err := l.syncDir(l.paths.tombstones); err != nil {
			return "", fmt.Errorf("sync committed recovery tombstone: %w", err)
		}
		return trash, nil
	}
	if record.State == "committed" && !effortExists && !trashExists {
		report.Recovered = append(report.Recovered, effortID)
		return "", nil
	}
	report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, "effort, trash, and tombstone state do not match"))
	return "", nil
}

func (l *Ledger) recoverTombstoneCommitTemporary(report *RecoveryReport, effortID, path, temporary string) error {
	if err := l.ops.inspect(l.paths.root, temporary, false); err != nil {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-tombstone-path", effortID, 0, nil, err.Error()))
		return nil
	}
	raw, err := l.ops.readFile(temporary)
	if err != nil {
		return fmt.Errorf("read tombstone commit temporary: %w", err)
	}
	committed, err := readTombstone(raw)
	if err != nil || committed.State != "committed" {
		detail := "commit temporary is not committed"
		if err != nil {
			detail = err.Error()
		}
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-tombstone", effortID, 0, nil, detail))
		return nil
	}
	if !l.pathExists(path) {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, "commit temporary has no base tombstone"))
		return nil
	}
	baseRaw, err := l.ops.readFile(path)
	if err != nil {
		return fmt.Errorf("read base tombstone for commit recovery: %w", err)
	}
	base, matches := func() (tombstoneRecord, bool) {
		record, decodeErr := readTombstone(baseRaw)
		return record, decodeErr == nil && record.Nonce == committed.Nonce
	}()
	if !matches {
		report.Ambiguous = append(report.Ambiguous, integrity("ambiguous-prune-state", effortID, 0, nil, "commit temporary nonce does not match base tombstone"))
		return nil
	}
	if base.State == "pending" {
		if err := l.ops.rename(temporary, path); err != nil {
			return fmt.Errorf("finish tombstone commit rename: %w", err)
		}
	} else {
		if err := l.ops.remove(temporary); err != nil {
			return fmt.Errorf("remove redundant tombstone commit temporary: %w", err)
		}
	}
	if err := l.syncDir(l.paths.tombstones); err != nil {
		return fmt.Errorf("sync recovered tombstone commit: %w", err)
	}
	report.Recovered = append(report.Recovered, effortID)
	return nil
}

func (l *Ledger) promoteTombstone(path string, record tombstoneRecord) error {
	record.State = "committed"
	raw, err := json.Marshal(record)
	if err != nil { // coverage-ignore: tombstone consists only of strings
		return err
	}
	temporary := path + ".commit"
	if err := l.writeSynced(temporary, append(raw, '\n')); err != nil {
		_ = l.ops.remove(temporary)
		return err
	}
	if err := l.ops.rename(temporary, path); err != nil {
		_ = l.ops.remove(temporary)
		return err
	}
	return l.syncDir(l.paths.tombstones)
}

func compactLine(raw []byte) ([]byte, error) {
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return nil, err
	}
	buffer.WriteByte('\n')
	return buffer.Bytes(), nil
}

func eventsEqual(event EventEnvelope, raw json.RawMessage) (bool, error) {
	if _, err := ValidateEvent(raw); err != nil {
		return false, err
	}
	left, err := canonicalJSON(eventToRaw(event))
	if err != nil { // coverage-ignore: validated event encoding always produces canonicalizable JSON
		return false, err
	}
	right, err := canonicalJSON(raw)
	if err != nil { // coverage-ignore: ValidateEvent already proved the compared raw JSON decodable
		return false, err
	}
	return bytes.Equal(left, right), nil
}

func eventToRaw(event EventEnvelope) json.RawMessage {
	raw, _ := json.Marshal(event)
	if len(event.EnvelopeExtensions) == 0 && len(event.PayloadExtensions) == 0 {
		return raw
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(raw, &root)
	for key, value := range event.EnvelopeExtensions {
		root[key] = value
	}
	var payload map[string]json.RawMessage
	_ = json.Unmarshal(root["payload"], &payload)
	for key, value := range event.PayloadExtensions {
		payload[key] = value
	}
	root["payload"], _ = json.Marshal(payload)
	raw, _ = json.Marshal(root)
	return raw
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func randomNonce() (string, error) {
	var value [16]byte
	if _, err := io.ReadFull(rand.Reader, value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}
