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
	"path/filepath"
	"sort"
	"strings"
	"time"

	awfgit "github.com/hypnotox/agentic-workflows/internal/git"
)

// Options supplies deterministic dependencies to focused tests.
type Options struct {
	Clock      func() time.Time
	UUID       func() (string, error)
	Filesystem fileSystem
	Worktrees  func(context.Context, string) ([]awfgit.WorktreeRegistration, error)
}

// Service owns ordinary effort record and memory operations.
type Service struct {
	ctx       context.Context
	paths     paths
	store     store
	clock     func() time.Time
	uuid      func() (string, error)
	worktrees func(context.Context, string) ([]awfgit.WorktreeRegistration, error)
}

func Open(ctx context.Context, invokingRoot string, options Options) (*Service, error) {
	roots, err := awfgit.ResolveControlRoots(ctx, invokingRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve effort control roots from %s: %w", invokingRoot, err)
	}
	resolved, err := resolvePaths(roots)
	if err != nil {
		return nil, fmt.Errorf("resolve effort resident paths from %s: %w", roots.PrimaryRoot, err)
	}
	resolved.fs = options.Filesystem
	clock := options.Clock
	if clock == nil {
		clock = time.Now
	}
	allocator := options.UUID
	if allocator == nil {
		allocator = randomUUIDv4
	}
	worktrees := options.Worktrees
	if worktrees == nil {
		worktrees = awfgit.ListWorktreeRegistrations
	}
	return &Service{ctx: ctx, paths: resolved, store: store{paths: resolved, fs: options.Filesystem}, clock: clock, uuid: allocator, worktrees: worktrees}, nil
}

func randomUUIDv4() (string, error) {
	var raw [16]byte
	_, _ = rand.Read(raw[:]) // crypto/rand.Read fills the slice or terminates the process.
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
			if err := requireAbsent(s.paths.record(id)); errors.Is(err, os.ErrExist) {
				continue
			} else if err != nil {
				return fmt.Errorf("check effort ID collision at %s: %w", s.paths.record(id), err)
			}
			now := s.now()
			created = Record{SchemaVersion: SchemaVersion, ID: id, Title: title, State: StateActive, CreatedAt: now, UpdatedAt: now, Integration: IntegrationNone}
			if withMemory {
				if _, err := s.paths.createMemory(id); err != nil {
					return fmt.Errorf("create default memory for effort %s: %w", id, err)
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
		if err := s.store.replace(record, true); err != nil {
			return fmt.Errorf("replace mutated effort record %s: %w", s.paths.record(id), err)
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
			return fmt.Errorf("create memory for effort %s: %w", id, err)
		}
		if !record.MemoryPresent {
			now := s.now()
			if !now.After(record.UpdatedAt) {
				now = record.UpdatedAt.Add(time.Nanosecond)
			}
			record.MemoryPresent, record.UpdatedAt = true, now
			if err := s.store.replace(record, true); err != nil {
				return fmt.Errorf("publish memory presence in effort record %s: %w", s.paths.record(id), err)
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
		if err != nil {
			return fmt.Errorf("derive memory truth for effort %s: %w", id, err)
		}
		if record.MemoryPresent != memory {
			result.Changes = append(result.Changes, RepairChange{Field: "memoryPresent", From: record.MemoryPresent, To: memory})
			record.MemoryPresent = memory
		}
		if err := s.repairWorktree(&record, &result); err != nil {
			return err
		}
		if len(result.Changes) > 0 {
			now := s.now()
			if !now.After(record.UpdatedAt) {
				now = record.UpdatedAt.Add(time.Nanosecond)
			}
			record.UpdatedAt = now
			if err := s.store.replace(record, true); err != nil {
				return fmt.Errorf("publish repaired effort record %s: %w", s.paths.record(id), err)
			}
		}
		result.Record = record
		return nil
	})
	if err != nil {
		return RepairResult{}, err
	}
	record, err := s.joinAssignments(result.Record)
	if err != nil {
		return RepairResult{}, fmt.Errorf("join assignments after repairing effort %s: %w", id, err)
	}
	result.Record = record
	return result, nil
}

func (s *Service) repairWorktree(record *Record, result *RepairResult) error {
	if err := s.paths.validate(s.paths.worktrees); err != nil {
		return fmt.Errorf("validate worktree resident root before repair: %w", err)
	}
	managed := filepath.Clean(s.paths.managedWorktree(record.ID))
	registrations, err := s.worktrees(s.ctx, s.paths.roots.InvokingRoot)
	if err != nil {
		return fmt.Errorf("list native Git registrations for repair of %s: %w", managed, err)
	}
	wantBranch := "refs/heads/awf/" + record.ID
	var exact []awfgit.WorktreeRegistration
	for _, registration := range registrations {
		pathMatches := filepath.Clean(registration.Path) == managed
		branchMatches := registration.Branch == wantBranch
		if branchMatches && !pathMatches {
			return safety("repository-identity", managed, fmt.Errorf("managed branch %q is registered at unexpected path %s", wantBranch, registration.Path))
		}
		if pathMatches {
			exact = append(exact, registration)
		}
	}
	if len(exact) > 1 {
		return safety("repository-identity", managed, fmt.Errorf("managed path has %d Git registrations", len(exact)))
	}
	pathPresent, err := managedDirectoryTruth(managed)
	if err != nil {
		return err
	}
	if len(exact) == 0 {
		if pathPresent {
			return safety("repository-identity", managed, errors.New("resident directory is not registered by native Git"))
		}
		if record.Worktree != nil {
			result.Changes = append(result.Changes, RepairChange{Field: "worktree", From: record.Worktree, To: nil})
			record.Worktree = nil
			if record.Integration == IntegrationPending {
				result.Changes = append(result.Changes, RepairChange{Field: "integration", From: IntegrationPending, To: IntegrationNone})
				record.Integration = IntegrationNone
			}
		}
		return nil
	}
	registration := exact[0]
	if registration.Bare || registration.Detached || registration.Branch != wantBranch || !objectIDPattern.MatchString(registration.HEAD) {
		return safety("repository-identity", managed, fmt.Errorf("registration has branch %q and HEAD %q, want %q and a full object ID", registration.Branch, registration.HEAD, wantBranch))
	}
	if !pathPresent {
		if record.Worktree == nil {
			return safety("repair-evidence", managed, errors.New("git registration has no present managed checkout"))
		}
		return nil
	}
	roots, err := awfgit.ResolveControlRoots(s.ctx, managed)
	if err != nil {
		return safety("repository-identity", managed, err)
	}
	if filepath.Clean(roots.CommonDir) != filepath.Clean(s.paths.roots.CommonDir) || filepath.Clean(roots.InvokingRoot) != managed {
		return safety("repository-identity", managed, errors.New("registered worktree belongs to a different repository identity"))
	}
	if record.Worktree != nil {
		return nil
	}
	// Phase 1 has no persisted partial-mutation evidence that authoritatively
	// records the attachment base. HEAD is the current worktree tip, not its
	// base, so reconstruction must wait for a later operation to supply such
	// evidence rather than inventing schema-1 metadata.
	return safety("repair-evidence", managed, errors.New("authoritative attachment base evidence is unavailable"))
}

func managedDirectoryTruth(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lstat managed worktree %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, safety("symlink", path, nil)
	}
	if !info.IsDir() {
		return false, safety("file-type", path, fmt.Errorf("mode %s is not a directory", info.Mode()))
	}
	if err := validatePathOwner(path, info, nil); err != nil { // coverage-ignore: requires a foreign-owned directory fixture created by a privileged test process
		return false, err
	}
	return true, nil
}

type assignmentFile struct {
	SchemaVersion int               `json:"schemaVersion"`
	Sessions      map[string]string `json:"sessions"`
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
		if strings.TrimSpace(session) != session || session == "" || len(session) > 160 || !uuidV4Pattern.MatchString(id) {
			return nil, &CorruptError{Path: path, Err: errors.New("invalid assignment entry")}
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
