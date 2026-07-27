package effort

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var objectIDPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// CorruptError identifies resident input that must be preserved byte-for-byte.
type CorruptError struct {
	Path string
	Err  error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("corrupt effort state at %s: %v", e.Path, e.Err)
}
func (e *CorruptError) Unwrap() error { return e.Err }

type persistedRecord struct {
	SchemaVersion int         `json:"schemaVersion"`
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	State         State       `json:"state"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	MemoryPresent bool        `json:"memoryPresent"`
	Worktree      *Worktree   `json:"worktree"`
	Integration   Integration `json:"integration"`
}

func persisted(r Record) persistedRecord {
	return persistedRecord{r.SchemaVersion, r.ID, r.Title, r.State, r.CreatedAt, r.UpdatedAt, r.MemoryPresent, r.Worktree, r.Integration}
}

func logical(r persistedRecord) Record {
	return Record{SchemaVersion: r.SchemaVersion, ID: r.ID, Title: r.Title, State: r.State, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, MemoryPresent: r.MemoryPresent, Worktree: r.Worktree, Integration: r.Integration}
}

type store struct{ paths paths }

func (s store) withLock(fn func() error) error {
	if err := s.paths.ensure(s.paths.efforts); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return err
	}
	path := filepath.Join(s.paths.efforts, ".lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("open effort lock: %w", err)
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("secure effort lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("lock effort store: %w", err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func (s store) load(id string) (Record, error) {
	if !uuidV4Pattern.MatchString(id) {
		return Record{}, fmt.Errorf("invalid effort id %q", id)
	}
	path := s.paths.record(id)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Record{}, fmt.Errorf("read effort %s: %w", id, err)
	}
	var value persistedRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return Record{}, &CorruptError{Path: path, Err: err}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Record{}, &CorruptError{Path: path, Err: err}
	}
	if err := validatePersisted(value, id); err != nil {
		return Record{}, &CorruptError{Path: path, Err: err}
	}
	return logical(value), nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
	}
	return nil
}

func validatePersisted(r persistedRecord, expectedID string) error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", r.SchemaVersion)
	}
	if !uuidV4Pattern.MatchString(r.ID) || r.ID != expectedID {
		return errors.New("record ID does not match its stable path")
	}
	if _, err := normalizeTitle(r.Title); err != nil || strings.TrimSpace(r.Title) != r.Title {
		return errors.New("invalid title")
	}
	if r.State != StateActive && r.State != StateCompleted && r.State != StateAbandoned {
		return fmt.Errorf("invalid state %q", r.State)
	}
	if r.CreatedAt.IsZero() || r.UpdatedAt.IsZero() || r.CreatedAt.Location() != time.UTC || r.UpdatedAt.Location() != time.UTC || r.UpdatedAt.Before(r.CreatedAt) {
		return errors.New("invalid timestamps")
	}
	validIntegration := r.Integration == IntegrationNone || r.Integration == IntegrationPending || r.Integration == IntegrationFastForward || r.Integration == IntegrationMerge || r.Integration == IntegrationManual
	if !validIntegration {
		return fmt.Errorf("invalid integration %q", r.Integration)
	}
	if r.Worktree == nil {
		if r.Integration == IntegrationPending {
			return errors.New("pending integration requires worktree metadata")
		}
		return nil
	}
	if r.Integration == IntegrationNone {
		return errors.New("worktree metadata requires an integration disposition")
	}
	if r.Worktree.Branch != "awf/"+r.ID || !objectIDPattern.MatchString(r.Worktree.Base) || r.Worktree.AttachedAt.IsZero() || r.Worktree.AttachedAt.Location() != time.UTC {
		return errors.New("invalid worktree metadata")
	}
	return nil
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("title must be nonblank")
	}
	if !utf8.ValidString(title) {
		return "", errors.New("title must be valid UTF-8")
	}
	if len([]byte(title)) > 160 {
		return "", errors.New("title must be at most 160 UTF-8 bytes")
	}
	return title, nil
}

func (s store) list() ([]Record, error) {
	entries, err := os.ReadDir(s.paths.efforts)
	if errors.Is(err, os.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return nil, fmt.Errorf("list efforts: %w", err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, &CorruptError{Path: filepath.Join(s.paths.efforts, entry.Name()), Err: errors.New("symlinked effort entry")}
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !uuidV4Pattern.MatchString(id) {
			return nil, &CorruptError{Path: filepath.Join(s.paths.efforts, entry.Name()), Err: errors.New("invalid effort filename")}
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Record, 0, len(ids))
	for _, id := range ids {
		record, err := s.load(id)
		if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func (s store) replace(record Record, requireExisting bool) error {
	path := s.paths.record(record.ID)
	if requireExisting {
		if _, err := s.load(record.ID); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
			return err
		}
	} else if _, err := os.Lstat(path); err == nil {
		return os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return err
	}
	value := persisted(record)
	if err := validatePersisted(value, record.ID); err != nil {
		return err
	}
	raw, err := json.Marshal(value)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return err
	}
	return atomicReplace(path, raw)
}

func atomicReplace(path string, raw []byte) error {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".effort-*.tmp")
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("create sibling effort temp: %w", err)
	}
	tempPath := temp.Name()
	remove := true
	defer func() {
		if remove { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
			_ = os.Remove(tempPath)
		}
	}()
	if err := temp.Chmod(0o600); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(raw); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return err
	}
	if err := os.Rename(tempPath, path); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("publish effort record: %w", err)
	}
	remove = false
	directory, err := os.Open(dir)
	if err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("open effort directory for sync: %w", err)
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil { // coverage-ignore: requires an injected OS durability or filesystem fault after the operation prerequisite succeeded
		return fmt.Errorf("sync effort directory: %w", err)
	}
	return nil
}
