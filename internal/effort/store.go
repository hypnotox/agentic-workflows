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

type durableFile interface {
	Name() string
	Stat() (os.FileInfo, error)
	Write([]byte) (int, error)
	Sync() error
	Close() error
}

type fileSystem interface {
	CreateTemp(string, string) (durableFile, error)
	Rename(string, string) error
	Remove(string) error
	OpenDirectory(string) (durableFile, error)
}

type osFileSystem struct{}

func (osFileSystem) CreateTemp(dir, pattern string) (durableFile, error) {
	return os.CreateTemp(dir, pattern)
}
func (osFileSystem) Rename(oldPath, newPath string) error           { return os.Rename(oldPath, newPath) }
func (osFileSystem) Remove(path string) error                       { return os.Remove(path) }
func (osFileSystem) OpenDirectory(path string) (durableFile, error) { return os.Open(path) }

type store struct {
	paths paths
	fs    fileSystem
}

func (s store) filesystem() fileSystem {
	if s.fs == nil {
		return osFileSystem{}
	}
	return s.fs
}

func (s store) withLock(fn func() error) error {
	if err := s.paths.ensure(s.paths.efforts); err != nil {
		return fmt.Errorf("prepare effort lock directory %s: %w", s.paths.efforts, err)
	}
	path := filepath.Join(s.paths.efforts, ".lock")
	file, identity, err := openRegularNoFollow(path, syscall.O_CREAT|syscall.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open effort lock %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	if fileMode, statErr := file.Stat(); statErr != nil { // coverage-ignore: the no-follow opener already fstat-validated this live descriptor
		return fmt.Errorf("inspect effort lock %s: %w", path, statErr)
	} else if fileMode.Mode().Perm() != 0o600 {
		return safety("unsafe-lock", path, fmt.Errorf("mode is %o, want 600", fileMode.Mode().Perm()))
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil { // coverage-ignore: the descriptor is a validated owned regular file and LOCK_EX has no invalid argument
		return fmt.Errorf("flock effort lock %s: %w", path, err)
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	if err := requireIdentity(path, identity); err != nil { // coverage-ignore: replacing the lock name between adjacent open and flock calls requires a concurrent namespace race
		return fmt.Errorf("verify effort lock identity %s after flock: %w", path, err)
	}
	return fn()
}

func (s store) load(id string) (Record, error) {
	record, _, err := s.loadIdentity(id)
	return record, err
}

func (s store) loadIdentity(id string) (Record, fileIdentity, error) {
	if err := s.paths.validate(s.paths.efforts); err != nil {
		return Record{}, fileIdentity{}, fmt.Errorf("validate effort resident root before load: %w", err)
	}
	if !uuidV4Pattern.MatchString(id) {
		return Record{}, fileIdentity{}, fmt.Errorf("invalid effort id %q", id)
	}
	path := s.paths.record(id)
	raw, identity, err := readRegularNoFollow(path)
	if err != nil {
		return Record{}, fileIdentity{}, fmt.Errorf("read effort record %s: %w", path, err)
	}
	var value persistedRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Record{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Record{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	if err := validatePersisted(value, id); err != nil {
		return Record{}, fileIdentity{}, &CorruptError{Path: path, Err: err}
	}
	return logical(value), identity, nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
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
	if err := s.paths.validate(s.paths.efforts); err != nil {
		return nil, fmt.Errorf("validate effort resident root before list: %w", err)
	}
	entries, err := os.ReadDir(s.paths.efforts)
	if errors.Is(err, os.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil { // coverage-ignore: the validated resident directory is either present or absent unless a kernel fault or concurrent race occurs
		return nil, fmt.Errorf("read effort directory %s: %w", s.paths.efforts, err)
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(s.paths.efforts, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			return nil, safety("symlink", path, nil)
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if entry.IsDir() || (entry.Type() != 0 && !entry.Type().IsRegular()) {
			return nil, safety("file-type", path, fmt.Errorf("mode %s is not regular", entry.Type()))
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !uuidV4Pattern.MatchString(id) {
			return nil, &CorruptError{Path: path, Err: errors.New("invalid effort filename")}
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Record, 0, len(ids))
	for _, id := range ids {
		record, err := s.load(id)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, nil
}

func (s store) replace(record Record, requireExisting bool) error {
	path := s.paths.record(record.ID)
	var expected *fileIdentity
	if requireExisting {
		_, identity, err := s.loadIdentity(record.ID)
		if err != nil {
			return err
		}
		expected = &identity
	} else if err := requireAbsent(path); err != nil {
		return fmt.Errorf("require absent effort record %s: %w", path, err)
	}
	value := persisted(record)
	if err := validatePersisted(value, record.ID); err != nil {
		return fmt.Errorf("validate replacement for %s: %w", path, err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode replacement for %s: %w", path, err)
	}
	return atomicReplaceFS(s.filesystem(), path, raw, expected)
}

func atomicReplaceFS(fs fileSystem, path string, raw []byte, expected *fileIdentity) (returnErr error) {
	dir := filepath.Dir(path)
	temp, err := fs.CreateTemp(dir, ".effort-*.tmp")
	if err != nil {
		return fmt.Errorf("create sibling temporary file in %s for %s: %w", dir, path, err)
	}
	tempPath := temp.Name()
	published := false
	closed := false
	defer func() {
		if !closed {
			if err := temp.Close(); err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("close temporary file %s while cleaning publication of %s: %w", tempPath, path, err))
			}
		}
		if !published {
			if err := fs.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary file %s after failed publication of %s: %w", tempPath, path, err))
			}
		}
	}()
	tempInfo, err := temp.Stat()
	if err != nil {
		return fmt.Errorf("fstat temporary file %s for %s: %w", tempPath, path, err)
	}
	if err := validateLeaf(tempPath, tempInfo); err != nil {
		return fmt.Errorf("validate temporary file %s for %s: %w", tempPath, path, err)
	}
	tempIdentity := fileIdentity{info: tempInfo}
	if filepath.Dir(tempPath) != dir {
		return fmt.Errorf("create sibling temporary file for %s: temporary path %s is outside %s", path, tempPath, dir)
	}
	if count, err := temp.Write(raw); err != nil {
		return fmt.Errorf("write temporary file %s for %s: %w", tempPath, path, err)
	} else if count != len(raw) {
		return fmt.Errorf("write temporary file %s for %s: %w", tempPath, path, io.ErrShortWrite)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("fsync temporary file %s for %s: %w", tempPath, path, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file %s for %s: %w", tempPath, path, err)
	}
	closed = true
	if err := requireIdentity(tempPath, tempIdentity); err != nil {
		return fmt.Errorf("verify temporary file identity %s before publishing %s: %w", tempPath, path, err)
	}
	if expected != nil {
		if err := requireIdentity(path, *expected); err != nil {
			return fmt.Errorf("verify destination identity %s before rename: %w", path, err)
		}
	} else if err := requireAbsent(path); err != nil {
		return fmt.Errorf("verify absent destination %s before rename: %w", path, err)
	}
	if err := fs.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temporary file %s to %s: %w", tempPath, path, err)
	}
	published = true
	directory, err := fs.OpenDirectory(dir)
	if err != nil {
		return fmt.Errorf("open directory %s to sync publication of %s: %w", dir, path, err)
	}
	directoryClosed := false
	defer func() {
		if !directoryClosed {
			if err := directory.Close(); err != nil {
				returnErr = errors.Join(returnErr, fmt.Errorf("close directory %s while finishing publication of %s: %w", dir, path, err))
			}
		}
	}()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("fsync directory %s after publishing %s: %w", dir, path, err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close directory %s after publishing %s: %w", dir, path, err)
	}
	directoryClosed = true
	return nil
}
