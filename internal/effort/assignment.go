package effort

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// Assignment is one current session-to-effort association. The persisted map is
// the only authority; effort records expose this relation only as a logical join.
type Assignment struct {
	SessionID string `json:"sessionId"`
	EffortID  string `json:"effortId"`
}

type assignmentFile struct {
	SchemaVersion int               `json:"schemaVersion"`
	Sessions      map[string]string `json:"sessions"`
}

func validSessionID(id string) bool {
	return id != "" && strings.TrimSpace(id) == id && utf8.ValidString(id) && len([]byte(id)) <= 160 &&
		id != "." && id != ".." && !strings.ContainsAny(id, "/\\\x00\r\n")
}

func (s *Service) readAssignments() (map[string]string, error) {
	if err := s.paths.validate(s.paths.assign); err != nil {
		return nil, fmt.Errorf("validate assignment resident root before read: %w", err)
	}
	path := s.paths.assignments()
	raw, _, err := readRegularNoFollow(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read assignment authority %s: %w", path, err)
	}
	var file assignmentFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, &CorruptError{Path: path, Err: err}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, &CorruptError{Path: path, Err: err}
	}
	if file.SchemaVersion != SchemaVersion || file.Sessions == nil {
		return nil, &CorruptError{Path: path, Err: errors.New("unsupported assignment schema")}
	}
	for session, id := range file.Sessions {
		if !validSessionID(session) || !uuidV4Pattern.MatchString(id) {
			return nil, &CorruptError{Path: path, Err: errors.New("invalid assignment entry")}
		}
	}
	ids := make([]string, 0, len(file.Sessions))
	seen := make(map[string]struct{}, len(file.Sessions))
	for _, id := range file.Sessions {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if _, err := s.store.load(id); err != nil {
			return nil, &CorruptError{Path: path, Err: fmt.Errorf("invalid assignment target %s: %w", id, err)}
		}
	}
	return file.Sessions, nil
}

func sessionsFor(assignments map[string]string, id string) []string {
	var sessions []string
	for session, effortID := range assignments {
		if effortID == id {
			sessions = append(sessions, session)
		}
	}
	sort.Strings(sessions)
	return sessions
}

func (s *Service) joinAssignments(record Record) (Record, error) {
	assignments, err := s.readAssignments()
	if err != nil {
		return Record{}, fmt.Errorf("join assignment authority for effort %s: %w", record.ID, err)
	}
	record.AssignedSessionIDs = sessionsFor(assignments, record.ID)
	return record, nil
}

// Assign explicitly associates sessionID with id. It permits reassignment and
// terminal efforts, and never changes the effort record or its lifecycle state.
func (s *Service) Assign(id, sessionID string) (Assignment, error) {
	if !uuidV4Pattern.MatchString(id) {
		return Assignment{}, fmt.Errorf("invalid effort id %q", id)
	}
	if !validSessionID(sessionID) {
		return Assignment{}, fmt.Errorf("invalid session id %q", sessionID)
	}
	assignment := Assignment{SessionID: sessionID, EffortID: id}
	err := s.store.withLock(func() error {
		if _, err := s.store.load(id); err != nil {
			return fmt.Errorf("assign session %s: %w", sessionID, err)
		}
		assignments, err := s.readAssignments()
		if err != nil {
			return err
		}
		if assignments[sessionID] == id {
			return nil
		}
		if err := s.paths.ensure(s.paths.assign); err != nil { // coverage-ignore: only a concurrent namespace/ownership mutation can invalidate the root after readAssignments validated it
			return fmt.Errorf("prepare assignment resident root: %w", err)
		}
		assignments[sessionID] = id
		raw, err := json.Marshal(assignmentFile{SchemaVersion: SchemaVersion, Sessions: assignments})
		if err != nil { // coverage-ignore: assignmentFile contains only strings and an int and cannot fail JSON encoding
			return fmt.Errorf("encode assignment authority: %w", err)
		}
		var expected *fileIdentity
		if _, identity, err := readRegularNoFollow(s.paths.assignments()); err == nil {
			expected = &identity
		} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: only a concurrent namespace mutation can change validated authority between read and replacement
			return fmt.Errorf("read assignment authority before replacement: %w", err)
		}
		return atomicReplaceFS(s.paths.filesystem(), s.paths.assignments(), raw, expected)
	})
	if err != nil {
		return Assignment{}, err
	}
	return assignment, nil
}

// Unassign removes the current association for sessionID.
func (s *Service) Unassign(sessionID string) (Assignment, error) {
	if !validSessionID(sessionID) {
		return Assignment{}, fmt.Errorf("invalid session id %q", sessionID)
	}
	var removed Assignment
	err := s.store.withLock(func() error {
		assignments, err := s.readAssignments()
		if err != nil {
			return err
		}
		id, ok := assignments[sessionID]
		if !ok {
			return fmt.Errorf("unknown session %q", sessionID)
		}
		removed = Assignment{SessionID: sessionID, EffortID: id}
		delete(assignments, sessionID)
		if err := s.paths.ensure(s.paths.assign); err != nil { // coverage-ignore: only a concurrent namespace/ownership mutation can invalidate the root after readAssignments validated it
			return fmt.Errorf("prepare assignment resident root: %w", err)
		}
		raw, err := json.Marshal(assignmentFile{SchemaVersion: SchemaVersion, Sessions: assignments})
		if err != nil { // coverage-ignore: assignmentFile contains only strings and an int and cannot fail JSON encoding
			return fmt.Errorf("encode assignment authority: %w", err)
		}
		_, identity, err := readRegularNoFollow(s.paths.assignments())
		if err != nil { // coverage-ignore: only a concurrent namespace mutation can remove validated authority before replacement
			return fmt.Errorf("read assignment authority before replacement: %w", err)
		}
		return atomicReplaceFS(s.paths.filesystem(), s.paths.assignments(), raw, &identity)
	})
	if err != nil {
		return Assignment{}, err
	}
	return removed, nil
}

// Assignments returns deterministic session order, optionally narrowed to one effort.
func (s *Service) Assignments(id string) ([]Assignment, error) {
	if id != "" {
		if _, err := s.store.load(id); err != nil {
			return nil, fmt.Errorf("list assignments: %w", err)
		}
	}
	assignments, err := s.readAssignments()
	if err != nil {
		return nil, err
	}
	out := make([]Assignment, 0, len(assignments))
	for sessionID, effortID := range assignments {
		if id == "" || id == effortID {
			out = append(out, Assignment{SessionID: sessionID, EffortID: effortID})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SessionID < out[j].SessionID })
	return out, nil
}
