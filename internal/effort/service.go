package effort

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// Options supplies deterministic dependencies to focused tests.
type Options struct {
	Clock func() time.Time
	UUID  func() (string, error)
}

// Service owns ordinary effort record and memory operations.
type Service struct {
	paths paths
	store store
	clock func() time.Time
	uuid  func() (string, error)
}

func Open(ctx context.Context, invokingRoot string, options Options) (*Service, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invokingRoot)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePaths(roots)
	if err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
		return nil, err
	}
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	allocator := options.UUID
	if allocator == nil {
		allocator = randomUUIDv4
	}
	return &Service{paths: resolved, store: store{paths: resolved}, clock: clock, uuid: allocator}, nil
}

func randomUUIDv4() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
		return "", err
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	encoded := hex.EncodeToString(raw[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func (s *Service) now() time.Time { return s.clock().UTC() }

func (s *Service) New(title string, withMemory bool) (Record, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return Record{}, err
	}
	var created Record
	err = s.store.withLock(func() error {
		for range 128 {
			id, err := s.uuid()
			if err != nil {
				return fmt.Errorf("allocate effort id: %w", err)
			}
			if !uuidV4Pattern.MatchString(id) {
				return fmt.Errorf("allocator returned invalid UUIDv4 %q", id)
			}
			if _, err := os.Lstat(s.paths.record(id)); err == nil {
				continue
			} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
				return err
			}
			now := s.now()
			created = Record{SchemaVersion: SchemaVersion, ID: id, Title: title, State: StateActive, CreatedAt: now, UpdatedAt: now, Integration: IntegrationNone}
			if withMemory {
				if _, err := s.paths.createMemory(id); err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
					return err
				}
				created.MemoryPresent = true
			}
			return s.store.replace(created, false)
		}
		return errors.New("unable to allocate a unique effort ID after 128 collisions")
	})
	if err != nil {
		return Record{}, err
	}
	return s.joinAssignments(created)
}

func (s *Service) List() ([]Record, error) {
	records, err := s.store.list()
	if err != nil {
		return nil, err
	}
	assignments, err := s.readAssignments()
	if err != nil {
		return nil, err
	}
	for i := range records {
		records[i].AssignedSessionIDs = sessionsFor(assignments, records[i].ID)
	}
	return records, nil
}

func (s *Service) Show(id string) (Record, error) {
	record, err := s.store.load(id)
	if err != nil {
		return Record{}, err
	}
	return s.joinAssignments(record)
}

func (s *Service) Rename(id, title string) (Record, error) {
	title, err := normalizeTitle(title)
	if err != nil {
		return Record{}, err
	}
	return s.mutate(id, func(record *Record) error { record.Title = title; return nil })
}

func (s *Service) Complete(id string) (Record, error) {
	return s.transition(id, StateCompleted)
}
func (s *Service) Abandon(id string) (Record, error) {
	return s.transition(id, StateAbandoned)
}
func (s *Service) Reopen(id string) (Record, error) {
	return s.mutate(id, func(record *Record) error {
		if record.State == StateActive {
			return errors.New("effort is already active")
		}
		record.State = StateActive
		return nil
	})
}

func (s *Service) transition(id string, state State) (Record, error) {
	return s.mutate(id, func(record *Record) error {
		if record.State != StateActive {
			return fmt.Errorf("effort is %s, want active", record.State)
		}
		record.State = state
		return nil
	})
}

func (s *Service) mutate(id string, change func(*Record) error) (Record, error) {
	var result Record
	err := s.store.withLock(func() error {
		record, err := s.store.load(id)
		if err != nil {
			return err
		}
		if err := change(&record); err != nil {
			return err
		}
		now := s.now()
		if !now.After(record.UpdatedAt) {
			now = record.UpdatedAt.Add(time.Nanosecond)
		}
		record.UpdatedAt = now
		if err := s.store.replace(record, true); err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
			return err
		}
		result = record
		return nil
	})
	if err != nil {
		return Record{}, err
	}
	return s.joinAssignments(result)
}

// Memory creates or confirms the owned normalized memory file.
func (s *Service) Memory(id string) (string, Record, error) {
	var path string
	var result Record
	err := s.store.withLock(func() error {
		record, err := s.store.load(id)
		if err != nil {
			return err
		}
		path, err = s.paths.createMemory(id)
		if err != nil {
			return err
		}
		if !record.MemoryPresent {
			now := s.now()
			if !now.After(record.UpdatedAt) {
				now = record.UpdatedAt.Add(time.Nanosecond)
			}
			record.MemoryPresent, record.UpdatedAt = true, now
			if err := s.store.replace(record, true); err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
				return err
			}
		}
		result = record
		return nil
	})
	if err != nil {
		return "", Record{}, err
	}
	result, err = s.joinAssignments(result)
	return path, result, err
}

func (s *Service) Repair(id string) (RepairResult, error) {
	result := RepairResult{SchemaVersion: SchemaVersion, Changes: []RepairChange{}}
	err := s.store.withLock(func() error {
		record, err := s.store.load(id)
		if err != nil {
			return err
		}
		memory, err := s.paths.memoryTruth(id)
		if err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
			return err
		}
		if record.MemoryPresent != memory {
			result.Changes = append(result.Changes, RepairChange{Field: "memoryPresent", From: record.MemoryPresent, To: memory})
			record.MemoryPresent = memory
		}
		if record.Worktree != nil {
			info, statErr := os.Lstat(s.paths.managedWorktree(id))
			switch {
			case errors.Is(statErr, os.ErrNotExist):
				result.Changes = append(result.Changes, RepairChange{Field: "worktree", From: record.Worktree, To: nil})
				record.Worktree = nil
				if record.Integration == IntegrationPending {
					result.Changes = append(result.Changes, RepairChange{Field: "integration", From: IntegrationPending, To: IntegrationNone})
					record.Integration = IntegrationNone
				}
			case statErr != nil: // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
				return statErr
			case info.Mode()&os.ModeSymlink != 0 || !info.IsDir():
				return fmt.Errorf("unsafe managed worktree path %s", s.paths.managedWorktree(id))
			}
		}
		if len(result.Changes) > 0 {
			now := s.now()
			if !now.After(record.UpdatedAt) {
				now = record.UpdatedAt.Add(time.Nanosecond)
			}
			record.UpdatedAt = now
			if err := s.store.replace(record, true); err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
				return err
			}
		}
		result.Record = record
		return nil
	})
	if err != nil {
		return RepairResult{}, err
	}
	record, err := s.joinAssignments(result.Record)
	if err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
		return RepairResult{}, err
	}
	result.Record = record
	return result, nil
}

type assignmentFile struct {
	SchemaVersion int               `json:"schemaVersion"`
	Sessions      map[string]string `json:"sessions"`
}

func (s *Service) readAssignments() (map[string]string, error) {
	raw, err := os.ReadFile(s.paths.assignments())
	if errors.Is(err, os.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
		return nil, err
	}
	var file assignmentFile
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, &CorruptError{Path: s.paths.assignments(), Err: err}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return nil, &CorruptError{Path: s.paths.assignments(), Err: err}
	}
	if file.SchemaVersion != SchemaVersion || file.Sessions == nil {
		return nil, &CorruptError{Path: s.paths.assignments(), Err: errors.New("unsupported assignment schema")}
	}
	for session, id := range file.Sessions {
		if strings.TrimSpace(session) != session || session == "" || len(session) > 160 || !uuidV4Pattern.MatchString(id) {
			return nil, &CorruptError{Path: s.paths.assignments(), Err: errors.New("invalid assignment entry")}
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
	if err != nil { // coverage-ignore: dependency failure is preserved unchanged and covered at the dependency boundary
		return Record{}, err
	}
	record.AssignedSessionIDs = sessionsFor(assignments, record.ID)
	return record, nil
}
