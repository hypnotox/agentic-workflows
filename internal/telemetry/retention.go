package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RetentionPolicy bounds terminal effort history. A zero value disables that
// dimension independently.
type RetentionPolicy struct {
	MaxCompletedEffortAgeDays int
	MaxCompletedEffortCount   int
}

// RetentionResult reports deterministic prune-order candidates and completed
// removals. Skipped contains candidates which became active before their prune
// lease was acquired.
type RetentionResult struct {
	Candidates []string
	Pruned     []string
	Skipped    []string
}

type retentionCandidate struct {
	EffortID          string
	TerminalTimestamp time.Time
	CreationTimestamp time.Time
}

// Retain selects repository-wide terminal efforts and, unless dryRun is true,
// prunes them oldest first.
func (l *Ledger) Retain(ctx context.Context, policy RetentionPolicy, dryRun bool) (RetentionResult, error) {
	if policy.MaxCompletedEffortAgeDays < 0 || policy.MaxCompletedEffortCount < 0 {
		return RetentionResult{}, errors.New("retention limits must not be negative")
	}
	candidates, err := l.retentionCandidates(policy)
	if err != nil {
		return RetentionResult{}, err
	}
	result := RetentionResult{Candidates: candidateIDs(candidates), Pruned: []string{}, Skipped: []string{}}
	if dryRun {
		return result, nil
	}
	for _, candidate := range candidates {
		nonce, nonceErr := l.ops.nonce()
		if nonceErr != nil {
			return result, fmt.Errorf("create prune nonce for %s: %w", candidate.EffortID, nonceErr)
		}
		pruned, pruneErr := l.pruneEffort(ctx, candidate.EffortID, nonce)
		if pruneErr != nil {
			return result, pruneErr
		}
		if pruned {
			result.Pruned = append(result.Pruned, candidate.EffortID)
		} else {
			result.Skipped = append(result.Skipped, candidate.EffortID)
		}
	}
	return result, nil
}

// Purge performs the only explicitly confirmed recursive removal of a named
// resident effort. Discovery, active, missing, and already-pruned efforts are
// refused.
func (l *Ledger) Purge(ctx context.Context, effortID string, confirmed bool) (RetentionResult, error) {
	result := RetentionResult{Candidates: []string{}, Pruned: []string{}, Skipped: []string{}}
	if !confirmed {
		return result, errors.New("confirmed purge is required")
	}
	if err := validatePathIdentifier("effortId", effortID); err != nil {
		return result, err
	}
	candidate, terminal, err := l.readRetentionCandidate(effortID)
	if err != nil {
		return result, fmt.Errorf("purge effort %s: %w", effortID, err)
	}
	if !terminal {
		return result, errors.New("purge refuses a discovery or active effort")
	}
	result.Candidates = []string{candidate.EffortID}
	nonce, err := l.ops.nonce()
	if err != nil {
		return result, fmt.Errorf("create purge nonce: %w", err)
	}
	pruned, err := l.pruneEffort(ctx, effortID, nonce)
	if err != nil {
		return result, err
	}
	if !pruned {
		result.Skipped = []string{effortID}
		return result, errors.New("purge effort became active while waiting for its lease")
	}
	result.Pruned = []string{effortID}
	return result, nil
}

func (l *Ledger) retentionCandidates(policy RetentionPolicy) ([]retentionCandidate, error) {
	entries, err := l.ops.readDir(l.paths.efforts)
	if err != nil {
		return nil, fmt.Errorf("read efforts for retention: %w", err)
	}
	newest := make([]retentionCandidate, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || validatePathIdentifier("effortId", entry.Name()) != nil {
			return nil, fmt.Errorf("unsafe effort entry %q", entry.Name())
		}
		candidate, terminal, readErr := l.readRetentionCandidate(entry.Name())
		if readErr != nil {
			return nil, fmt.Errorf("read retention candidate %s: %w", entry.Name(), readErr)
		}
		if terminal {
			newest = append(newest, candidate)
		}
	}
	sort.Slice(newest, func(i, j int) bool { return candidateNewer(newest[i], newest[j]) })
	selected := make(map[string]retentionCandidate)
	if policy.MaxCompletedEffortAgeDays > 0 {
		cutoff := l.ops.now().Add(-time.Duration(policy.MaxCompletedEffortAgeDays) * 24 * time.Hour)
		for _, candidate := range newest {
			if candidate.TerminalTimestamp.Before(cutoff) {
				selected[candidate.EffortID] = candidate
			}
		}
	}
	if policy.MaxCompletedEffortCount > 0 && len(newest) > policy.MaxCompletedEffortCount {
		for _, candidate := range newest[policy.MaxCompletedEffortCount:] {
			selected[candidate.EffortID] = candidate
		}
	}
	oldest := make([]retentionCandidate, 0, len(selected))
	for _, candidate := range selected {
		oldest = append(oldest, candidate)
	}
	sort.Slice(oldest, func(i, j int) bool { return candidateNewer(oldest[j], oldest[i]) })
	return oldest, nil
}

func candidateNewer(left, right retentionCandidate) bool {
	if !left.TerminalTimestamp.Equal(right.TerminalTimestamp) {
		return left.TerminalTimestamp.After(right.TerminalTimestamp)
	}
	if !left.CreationTimestamp.Equal(right.CreationTimestamp) {
		return left.CreationTimestamp.After(right.CreationTimestamp)
	}
	return left.EffortID < right.EffortID
}

func candidateIDs(candidates []retentionCandidate) []string {
	ids := make([]string, len(candidates))
	for index, candidate := range candidates {
		ids[index] = candidate.EffortID
	}
	return ids
}

func (l *Ledger) readRetentionCandidate(effortID string) (retentionCandidate, bool, error) {
	read, err := l.ReadEffort(effortID)
	if err != nil {
		return retentionCandidate{}, false, err
	}
	return retentionCandidateFromRead(read, effortID)
}

func retentionCandidateFromRead(read EffortRead, effortID string) (retentionCandidate, bool, error) {
	projection := projectLifecycleFromRead(read)
	if projection.State != EffortCompleted && projection.State != EffortAbandoned {
		return retentionCandidate{}, false, nil
	}
	created, err := time.Parse(time.RFC3339Nano, read.Metadata.CreatedAt)
	if err != nil { // coverage-ignore: immutable metadata validation enforces RFC3339Nano
		return retentionCandidate{}, false, fmt.Errorf("parse creation timestamp: %w", err)
	}
	terminal, err := effectiveTerminalTimestamp(read, projection)
	if err != nil { // coverage-ignore: a terminal projection from a validated EffortRead always has one direct or repaired effective terminal event
		return retentionCandidate{}, false, err
	}
	return retentionCandidate{EffortID: effortID, TerminalTimestamp: terminal, CreationTimestamp: created}, true, nil
}

func effectiveTerminalTimestamp(read EffortRead, projection LifecycleProjection) (time.Time, error) {
	repairs := make(map[string]RepairState, len(projection.Repairs))
	for _, repair := range projection.Repairs {
		repairs[repair.EventID] = repair
	}
	for _, event := range read.Events {
		kind, payload := event.Kind, event.Payload
		if !read.EffectApplied[event.EventID] {
			repairID := projection.SupersededEventIDs[event.EventID]
			repair, repaired := repairs[repairID]
			if !repaired || !read.EffectApplied[repairID] {
				continue
			}
			kind, payload = repair.Replacement.EventKind, repair.Replacement.Payload
		}
		if kind != "effort_completed" && kind != "effort_abandoned" {
			continue
		}
		var terminalPayload EffortTerminalPayload
		if err := json.Unmarshal(payload, &terminalPayload); err != nil { // coverage-ignore: protocol validation enforces direct and repair replacement payload shapes
			return time.Time{}, fmt.Errorf("decode terminal epoch: %w", err)
		}
		if terminalPayload.TerminalEpoch != projection.TerminalEpoch {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, event.Timestamp)
		if err != nil { // coverage-ignore: protocol validation enforces source event RFC3339Nano timestamps
			return time.Time{}, fmt.Errorf("parse terminal timestamp: %w", err)
		}
		return timestamp, nil
	}
	return time.Time{}, errors.New("terminal projection has no effective event in its current epoch") // coverage-ignore: validated terminal projections are created only by direct or repaired terminal effects
}

func (l *Ledger) pruneEffort(ctx context.Context, effortID, pruneNonce string) (bool, error) {
	if err := validatePathIdentifier("effortId", effortID); err != nil {
		return false, err
	}
	if !validPruneNonce(pruneNonce) {
		return false, errors.New("invalid prune nonce")
	}
	leasePath := l.paths.appendLease(effortID)
	leaseNonce, err := l.acquireLease(ctx, leasePath)
	if err != nil {
		return false, fmt.Errorf("acquire prune lease: %w", err)
	}
	stopHeartbeat, heartbeatDone := l.startHeartbeat(ctx, leasePath, leaseNonce)
	released := false
	release := func() error {
		if released {
			return nil
		}
		heartbeatErr := finishHeartbeat(stopHeartbeat, heartbeatDone)
		releaseErr := l.releaseLease(leasePath, leaseNonce)
		released = true
		if heartbeatErr != nil {
			return heartbeatErr
		}
		return releaseErr
	}
	defer func() { _ = release() }()

	tombstonePath := l.paths.tombstone(effortID)
	trashPath := filepath.Join(l.paths.trash, stagingName(effortID, pruneNonce))
	effortPath := l.paths.effort(effortID)
	effortExists := l.pathExists(effortPath)
	trashExists := l.pathExists(trashPath)
	if l.pathExists(tombstonePath) {
		if inspectErr := l.ops.inspect(l.paths.root, tombstonePath, false); inspectErr != nil {
			return false, fmt.Errorf("inspect existing tombstone: %w", inspectErr)
		}
		raw, readErr := l.ops.readFile(tombstonePath)
		if readErr != nil {
			return false, fmt.Errorf("read existing tombstone: %w", readErr)
		}
		record, recordErr := readTombstone(raw)
		if recordErr != nil || record.Nonce != pruneNonce {
			return false, errors.New("ambiguous prune nonce")
		}
		if record.State == "committed" {
			if effortExists || l.hasOtherTrashNonce(effortID, filepath.Base(trashPath)) {
				return false, errors.New("ambiguous committed prune state")
			}
			if err := l.syncDir(l.paths.tombstones); err != nil {
				return false, fmt.Errorf("sync committed tombstone: %w", err)
			}
			if err := release(); err != nil {
				return false, err
			}
			if trashExists {
				if err := l.deleteTrash(trashPath); err != nil {
					return false, err
				}
			}
			return true, nil
		}
		if effortExists {
			read, readErr := l.readEffort(effortID, true)
			if readErr != nil {
				return false, fmt.Errorf("recheck pending terminal effort: %w", readErr)
			}
			_, terminal, readErr := retentionCandidateFromRead(read, effortID)
			if readErr != nil { // coverage-ignore: validated ledger reads have parseable creation and event timestamps
				return false, fmt.Errorf("project pending terminal effort: %w", readErr)
			}
			if !terminal {
				return false, nil
			}
		}
		if err := l.syncDir(l.paths.tombstones); err != nil {
			return false, fmt.Errorf("sync existing pending tombstone: %w", err)
		}
	} else {
		if !effortExists {
			return false, errors.New("effort does not exist and was not durably pruned")
		}
		_, terminal, readErr := l.readRetentionCandidate(effortID)
		if readErr != nil {
			return false, fmt.Errorf("recheck terminal effort: %w", readErr)
		}
		if !terminal {
			return false, nil
		}
		record := tombstoneRecord{Nonce: pruneNonce, State: "pending"}
		raw, marshalErr := json.Marshal(record)
		if marshalErr != nil { // coverage-ignore: tombstone consists only of strings
			return false, marshalErr
		}
		if err := l.writeSynced(tombstonePath, append(raw, '\n')); err != nil {
			return false, fmt.Errorf("write pending tombstone: %w", err)
		}
		if err := l.syncDir(l.paths.tombstones); err != nil {
			return false, fmt.Errorf("sync pending tombstone: %w", err)
		}
	}

	if l.hasOtherTrashNonce(effortID, filepath.Base(trashPath)) {
		return false, errors.New("ambiguous prune trash nonce")
	}
	if effortExists {
		if err := l.ops.inspect(l.paths.root, effortPath, true); err != nil {
			return false, fmt.Errorf("inspect effort before prune: %w", err)
		}
		if trashExists {
			return false, errors.New("ambiguous prune state has effort and trash")
		}
		if err := l.ops.rename(effortPath, trashPath); err != nil {
			return false, fmt.Errorf("move effort to private trash: %w", err)
		}
	} else if !trashExists {
		return false, errors.New("ambiguous pending prune state")
	}
	if err := l.syncDir(l.paths.efforts); err != nil {
		return false, fmt.Errorf("sync pruned efforts: %w", err)
	}
	if err := l.syncDir(l.paths.trash); err != nil {
		return false, fmt.Errorf("sync private trash: %w", err)
	}
	if err := l.promoteTombstone(tombstonePath, tombstoneRecord{Nonce: pruneNonce, State: "pending"}); err != nil {
		return false, fmt.Errorf("commit prune tombstone: %w", err)
	}
	if err := release(); err != nil {
		return false, err
	}
	if err := l.deleteTrash(trashPath); err != nil {
		return false, err
	}
	return true, nil
}

func validPruneNonce(nonce string) bool {
	if nonce == "" || len(nonce) > 128 || strings.ContainsAny(nonce, `/\\.`) {
		return false
	}
	for _, value := range nonce {
		if (value < 'a' || value > 'z') && (value < 'A' || value > 'Z') && (value < '0' || value > '9') && value != '-' && value != '_' {
			return false
		}
	}
	return true
}

func (l *Ledger) hasOtherTrashNonce(effortID, expected string) bool {
	entries, err := l.ops.readDir(l.paths.trash)
	if err != nil {
		return true
	}
	for _, entry := range entries {
		id, _, parseErr := parseStagingName(entry.Name())
		if parseErr != nil || !entry.IsDir() {
			return true
		}
		if id == effortID && entry.Name() != expected {
			return true
		}
	}
	return false
}

func (l *Ledger) deleteTrash(path string) error {
	if err := l.ops.inspect(l.paths.root, path, true); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("inspect committed trash: %w", err)
	}
	if err := l.ops.removeAll(path); err != nil {
		return fmt.Errorf("delete committed trash: %w", err)
	}
	if err := l.syncDir(l.paths.trash); err != nil {
		return fmt.Errorf("sync deleted trash: %w", err)
	}
	return nil
}
