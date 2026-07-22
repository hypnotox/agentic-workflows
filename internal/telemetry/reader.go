package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type IntegrityIssue struct {
	Code     string
	Scope    string
	Line     int
	EventIDs []string
	Detail   string
}

type LedgerRecord struct {
	SessionID string
	Line      int
	Raw       json.RawMessage
	Event     *EventEnvelope
	Applied   bool
}

type EffortRead struct {
	Metadata        EffortMetadata
	Events          []EventEnvelope
	EffectApplied   map[string]bool
	RejectedEffects map[string]bool
	Records         []LedgerRecord
	Integrity       []IntegrityIssue
}

func (l *Ledger) ReadEffort(effortID string) (EffortRead, error) {
	return l.readEffort(effortID, false)
}

func (l *Ledger) readEffort(effortID string, allowPendingTombstone bool) (EffortRead, error) {
	if err := validatePathIdentifier("effortId", effortID); err != nil {
		return EffortRead{}, err
	}
	if l.pathExists(l.paths.tombstone(effortID)) && !allowPendingTombstone {
		return EffortRead{}, errors.New("effort was pruned")
	}
	effortPath := l.paths.effort(effortID)
	if err := l.ops.inspect(l.paths.root, effortPath, true); err != nil {
		return EffortRead{}, fmt.Errorf("inspect effort: %w", err)
	}
	metadata, err := l.readMetadata(effortID)
	if err != nil {
		return EffortRead{}, err
	}
	if metadata.EffortID != effortID {
		return EffortRead{}, errors.New("immutable effort metadata identity mismatch")
	}
	sessionsPath := filepath.Join(effortPath, "sessions")
	if err := l.ops.inspect(l.paths.root, sessionsPath, true); err != nil {
		return EffortRead{}, fmt.Errorf("inspect sessions directory: %w", err)
	}
	entries, err := l.ops.readDir(sessionsPath)
	if err != nil {
		return EffortRead{}, fmt.Errorf("read sessions directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	result := EffortRead{Metadata: metadata, Events: []EventEnvelope{}, EffectApplied: map[string]bool{}, RejectedEffects: map[string]bool{}, Records: []LedgerRecord{}, Integrity: []IntegrityIssue{}}
	byEventID := make(map[string]EventEnvelope)
	byLifecycleIdentity := make(map[string]EventEnvelope)
	byPassiveIdentity := make(map[string]EventEnvelope)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".jsonl") {
			result.Integrity = append(result.Integrity, integrity("unsafe-stream-entry", effortID, 0, nil, "sessions contains a non-stream entry"))
			continue
		}
		sessionID := strings.TrimSuffix(name, ".jsonl")
		if err := validatePathIdentifier("sessionId", sessionID); err != nil {
			result.Integrity = append(result.Integrity, integrity("unsafe-stream-identifier", effortID, 0, nil, err.Error()))
			continue
		}
		stream := filepath.Join(sessionsPath, name)
		if err := l.ops.inspect(l.paths.root, stream, false); err != nil {
			result.Integrity = append(result.Integrity, integrity("unsafe-stream-path", sessionID, 0, nil, err.Error()))
			continue
		}
		raw, err := l.ops.readFile(stream)
		if err != nil {
			return EffortRead{}, fmt.Errorf("read event stream: %w", err)
		}
		l.readStream(&result, effortID, sessionID, raw, byEventID, byLifecycleIdentity, byPassiveIdentity)
	}
	for _, event := range result.Events {
		for _, predecessor := range event.Predecessors {
			if _, ok := byEventID[predecessor]; !ok {
				result.Integrity = append(result.Integrity, integrity("broken-predecessor", event.SessionID, 0, []string{event.EventID, predecessor}, "predecessor does not exist"))
			}
		}
	}
	creationCount := 0
	invalidCreationIDs := make(map[string]bool)
	for index := range result.Records {
		record := &result.Records[index]
		if record.Event == nil || record.Event.Kind != "effort_created" || !record.Applied {
			continue
		}
		valid := true
		if record.Line != 1 {
			valid = false
			result.Integrity = append(result.Integrity, integrity("misplaced-creation-event", record.SessionID, record.Line, []string{record.Event.EventID}, "creation event is not the first stream record"))
		}
		if _, creationErr := validateCreation(metadata, record.Raw); creationErr != nil {
			valid = false
			result.Integrity = append(result.Integrity, integrity("creation-metadata-mismatch", effortID, record.Line, []string{record.Event.EventID}, creationErr.Error()))
		}
		if valid && creationCount == 0 {
			creationCount++
			continue
		}
		if valid {
			result.Integrity = append(result.Integrity, integrity("duplicate-creation-event", effortID, record.Line, []string{record.Event.EventID}, "duplicate creation event has no applied state effect"))
		}
		record.Applied = false
		invalidCreationIDs[record.Event.EventID] = true
	}
	_ = invalidCreationIDs // creation evidence remains in the causal graph
	if creationCount != 1 {
		result.Integrity = append(result.Integrity, integrity("missing-or-duplicate-creation-event", effortID, 0, nil, "effort must have exactly one valid creation event"))
	}
	// Every structurally valid unique event remains in Events as causal evidence.
	// EffectApplied is a separate mask: illegal and superseded lifecycle records
	// stay referenceable by descendants and repairs without changing state.
	excluded := make(map[string]bool)
	canonicalApplied := make(map[string]bool)
	for _, record := range result.Records {
		if record.Event == nil {
			continue
		}
		if record.Applied {
			canonicalApplied[record.Event.EventID] = true
		} else {
			excluded[record.Event.EventID] = true
		}
	}
	for eventID := range canonicalApplied {
		delete(excluded, eventID)
	}
	for eventID := range excluded {
		result.RejectedEffects[eventID] = true
	}
	projected := projectLifecycle(result.Events, excluded)
	result.Integrity = append(result.Integrity, projected.Invalid...)
	for _, event := range result.Events {
		applied := !excluded[event.EventID] && (descriptor.Payloads[string(event.Kind)].Class == "passive" || projected.EffectApplied[event.EventID])
		result.EffectApplied[event.EventID] = applied
	}
	for index := range result.Records {
		record := &result.Records[index]
		if record.Event != nil && descriptor.Payloads[string(record.Event.Kind)].Class == "lifecycle" {
			record.Applied = projected.EffectApplied[record.Event.EventID]
		}
	}
	return result, nil
}

func (l *Ledger) readStream(result *EffortRead, effortID, sessionID string, raw []byte, byEventID, byLifecycleIdentity, byPassiveIdentity map[string]EventEnvelope) {
	lines := splitJSONLines(raw)
	for index, line := range lines.complete {
		lineNumber := index + 1
		record := LedgerRecord{SessionID: sessionID, Line: lineNumber, Raw: append(json.RawMessage(nil), line...)}
		event, err := ValidateEvent(line)
		if err != nil {
			code := "malformed-complete-line"
			if strings.Contains(err.Error(), "unsupported protocol major") || strings.Contains(err.Error(), "unknown required kind") {
				code = "unsupported-protocol"
			}
			result.Integrity = append(result.Integrity, integrity(code, sessionID, lineNumber, nil, err.Error()))
			result.Records = append(result.Records, record)
			continue
		}
		record.Event = &event
		if event.EffortID != effortID || event.SessionID != sessionID {
			result.Integrity = append(result.Integrity, integrity("stream-identity-mismatch", sessionID, lineNumber, []string{event.EventID}, "event identity differs from its stream"))
			result.Records = append(result.Records, record)
			continue
		}
		identityClass, identity := eventIdentity(event)
		identityMap := byPassiveIdentity
		if identityClass == "lifecycle" {
			identityMap = byLifecycleIdentity
		}
		prior, duplicateEvent := byEventID[event.EventID]
		priorIdentity, duplicateIdentity := identityMap[identity]
		if duplicateEvent || duplicateIdentity {
			if !duplicateEvent {
				prior = priorIdentity
			}
			equal, compareErr := eventsEqual(prior, line)
			if compareErr != nil || !equal {
				ids := []string{prior.EventID, event.EventID}
				result.Integrity = append(result.Integrity, integrity("conflicting-duplicate", sessionID, lineNumber, ids, "duplicate identity has different content"))
			}
			// A distinct event ID remains causal evidence even when its contract
			// identity conflicts. Same-ID records cannot form distinct graph nodes
			// and remain available through Records.
			if !duplicateEvent {
				byEventID[event.EventID] = event
				result.Events = append(result.Events, event)
			}
			result.Records = append(result.Records, record)
			continue
		}
		byEventID[event.EventID] = event
		identityMap[identity] = event
		if l.validateTransition != nil {
			if err := l.validateTransition(event, result.Events); err != nil {
				result.Integrity = append(result.Integrity, integrity("invalid-transition", sessionID, lineNumber, []string{event.EventID}, err.Error()))
				result.Events = append(result.Events, event)
				result.Records = append(result.Records, record)
				continue
			}
		}
		record.Applied = true
		result.Events = append(result.Events, event)
		result.Records = append(result.Records, record)
	}
	if len(lines.partial) != 0 {
		result.Integrity = append(result.Integrity, integrity("partial-final-line", sessionID, len(lines.complete)+1, nil, "ignored incomplete final JSONL line"))
		result.Records = append(result.Records, LedgerRecord{SessionID: sessionID, Line: len(lines.complete) + 1, Raw: append(json.RawMessage(nil), lines.partial...)})
	}
}

type jsonLines struct {
	complete [][]byte
	partial  []byte
}

func splitJSONLines(raw []byte) jsonLines {
	result := jsonLines{complete: [][]byte{}}
	if len(raw) == 0 {
		return result
	}
	parts := bytes.Split(raw, []byte{'\n'})
	completeCount := len(parts) - 1
	if raw[len(raw)-1] != '\n' {
		result.partial = append([]byte(nil), parts[len(parts)-1]...)
	}
	for _, part := range parts[:completeCount] {
		result.complete = append(result.complete, append([]byte(nil), part...))
	}
	return result
}

func eventIdentity(event EventEnvelope) (string, string) {
	if descriptor.Payloads[string(event.Kind)].Class == "lifecycle" {
		return "lifecycle", event.IdempotencyKey
	}
	return "passive", event.ObservationID
}

func sameContractIdentity(left, right EventEnvelope) bool {
	leftClass, leftIdentity := eventIdentity(left)
	rightClass, rightIdentity := eventIdentity(right)
	return leftClass == rightClass && leftIdentity == rightIdentity
}

func integrity(code, scope string, line int, eventIDs []string, detail string) IntegrityIssue {
	if eventIDs == nil {
		eventIDs = []string{}
	}
	return IntegrityIssue{Code: code, Scope: scope, Line: line, EventIDs: eventIDs, Detail: detail}
}

func readTombstone(raw []byte) (tombstoneRecord, error) {
	var record tombstoneRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&record); err != nil || ensureJSONEOF(decoder) != nil || !validPruneNonce(record.Nonce) || (record.State != "pending" && record.State != "committed") {
		return tombstoneRecord{}, errors.New("ambiguous tombstone record")
	}
	return record, nil
}
