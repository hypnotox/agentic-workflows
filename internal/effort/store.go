package effort

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const finishingPrefix = ".finishing-"

// CorruptError identifies resident input that must be preserved byte-for-byte.
type CorruptError struct {
	Path string
	Err  error
}

func (e *CorruptError) Error() string {
	return fmt.Sprintf("unusable effort resident at %s: %v; changed bytes: no; next action: preserve the resident and inspect it for manual cleanup", e.Path, e.Err)
}
func (e *CorruptError) Unwrap() error { return e.Err }

type persistedRecord struct {
	SchemaVersion int       `json:"schemaVersion"`
	ID            string    `json:"id"`
	Slug          string    `json:"slug"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"createdAt"`
}

func persisted(r Record) persistedRecord {
	return persistedRecord{SchemaVersion: r.SchemaVersion, ID: r.ID, Slug: r.Slug, Title: r.Title, CreatedAt: r.CreatedAt}
}

func logical(r persistedRecord) Record {
	return Record{SchemaVersion: r.SchemaVersion, ID: r.ID, Slug: r.Slug, Title: r.Title, CreatedAt: r.CreatedAt, MemoryPath: memoryPublicPath(r.Slug)}
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("outcome title must be nonblank")
	}
	if !utf8.ValidString(title) {
		return "", errors.New("outcome title must be valid UTF-8")
	}
	return title, nil
}

func deriveSlug(title string) (string, error) {
	if !utf8.ValidString(title) {
		return "", slugRepairError("outcome title is not valid UTF-8")
	}
	var b strings.Builder
	separator := false
	for _, r := range title {
		switch {
		case r >= 'A' && r <= 'Z':
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			separator = false
			b.WriteByte(byte(r + ('a' - 'A')))
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if separator && b.Len() > 0 {
				b.WriteByte('-')
			}
			separator = false
			b.WriteRune(r)
		default:
			separator = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if err := validateSlug(slug); err != nil {
		return "", slugRepairError(err.Error())
	}
	return slug, nil
}

func slugRepairError(condition string) error {
	return fmt.Errorf("cannot derive effort slug: %s; changed bytes: no; next action: provide a shorter outcome title with ASCII words or digits", condition)
}

func validateSlug(slug string) error {
	if len(slug) < 1 || len(slug) > 63 {
		return errors.New("slug must contain 1-63 bytes")
	}
	if !slugPattern.MatchString(slug) {
		return fmt.Errorf("slug must match %s", slugPattern)
	}
	if filepath.Base(slug) != slug || filepath.Clean(slug) != slug { // coverage-ignore: the stricter ASCII slug regexp already excludes separators, dot segments, and platform volume syntax
		return errors.New("slug must be one confined path segment")
	}
	if err := exec.Command("git", "check-ref-format", "--branch", "awf/"+slug).Run(); err != nil { // coverage-ignore: the bounded lowercase ASCII grammar with a fixed awf/ prefix is always a valid branch ref; this remains defense in depth
		return fmt.Errorf("refs/heads/awf/%s is not a valid Git ref", slug)
	}
	return nil
}

func validatePersisted(r persistedRecord, expectedSlug string) error {
	if r.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", r.SchemaVersion)
	}
	if !uuidV4Pattern.MatchString(r.ID) {
		return errors.New("invalid internal UUIDv4")
	}
	if r.Slug != expectedSlug {
		return errors.New("state slug does not match its stable directory")
	}
	if err := validateSlug(r.Slug); err != nil {
		return fmt.Errorf("invalid slug: %w", err)
	}
	title, err := normalizeTitle(r.Title)
	if err != nil || title != r.Title {
		return errors.New("invalid title")
	}
	if r.CreatedAt.IsZero() || r.CreatedAt.Location() != time.UTC {
		return errors.New("invalid createdAt")
	}
	return nil
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

type store struct {
	paths paths
	fault func(string) error
}

func (s store) hit(stage string) error {
	if s.fault == nil {
		return nil
	}
	if err := s.fault(stage); err != nil {
		return fmt.Errorf("injected failure at %s: %w", stage, err)
	}
	return nil
}

func (s store) reserve(slug string) (string, error) {
	if err := s.paths.ensure(s.paths.efforts); err != nil {
		return "", fmt.Errorf("prepare efforts root: %w", err)
	}
	if tombstone, err := s.findTombstones(slug); err != nil {
		return "", err
	} else if len(tombstone) > 0 {
		return "", fmt.Errorf("effort slug %q is reserved by finishing cleanup; changed bytes: no; next action: run `awf effort finish %s`", slug, slug)
	}
	dir := s.paths.effort(slug)
	if err := os.Mkdir(dir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			condition := "an incomplete reservation exists"
			if _, statErr := os.Lstat(s.paths.stateFile(slug)); statErr == nil {
				condition = "an active effort already exists"
			}
			return "", fmt.Errorf("effort slug %q collides because %s; changed bytes: no; next action: choose a distinct outcome title or inspect %s", slug, condition, dir)
		}
		return "", fmt.Errorf("reserve effort directory %s: %w", dir, err) // coverage-ignore: ensure and tombstone enumeration just proved the parent usable; a non-collision failure requires a concurrent namespace or storage fault
	}
	if err := s.hit("reserve.directory"); err != nil {
		return "", err
	}
	return dir, nil
}

func (s store) create(record Record) error {
	dir, err := s.reserve(record.Slug)
	if err != nil {
		return err
	}
	if err := s.publishNew(s.paths.memoryFile(record.Slug), memorySkeleton(record.Slug), "memory"); err != nil {
		return err
	}
	raw, err := json.Marshal(persisted(record))
	if err != nil { // coverage-ignore: persistedRecord contains only JSON-native scalar and time fields
		return fmt.Errorf("encode effort state: %w", err)
	}
	if err := s.publishNew(s.paths.stateFile(record.Slug), raw, "state"); err != nil {
		return err
	}
	if err := s.hit("efforts-root.fsync"); err != nil {
		return err
	}
	if err := syncDirectory(s.paths.efforts); err != nil { // coverage-ignore: fault injection covers the ordered root-sync boundary; an actual failure requires a kernel or storage fault
		return fmt.Errorf("fsync efforts root after publishing %s: %w", dir, err)
	}
	return nil
}

func (s store) publishNew(path string, raw []byte, label string) (returnErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, "."+label+"-*.tmp")
	if err != nil { // coverage-ignore: reservation created the owned writable directory; CreateTemp failure requires a concurrent permission change or storage fault
		return fmt.Errorf("create sibling temporary file for %s: %w", path, err)
	}
	tempPath := temp.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, temp.Close()) // coverage-ignore: fault stages cover pre-close cleanup; a close failure itself requires a kernel or storage fault
		}
		if !published {
			if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) { // coverage-ignore: the owned sibling temporary remains removable absent a concurrent namespace or storage fault
				returnErr = errors.Join(returnErr, fmt.Errorf("remove temporary file %s: %w", tempPath, err))
			}
		}
	}()
	if err := s.hit(label + ".write"); err != nil {
		return err
	}
	if n, err := temp.Write(raw); err != nil { // coverage-ignore: injected write stages cover the boundary; os.File write failure requires a kernel or storage fault
		return fmt.Errorf("write temporary file for %s: %w", path, err)
	} else if n != len(raw) { // coverage-ignore: os.File.Write returns a non-nil error on a short write
		return fmt.Errorf("write temporary file for %s: %w", path, io.ErrShortWrite)
	}
	if err := s.hit(label + ".fsync"); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil { // coverage-ignore: injected fsync stages cover the boundary; os.File.Sync failure requires a kernel or storage fault
		return fmt.Errorf("fsync temporary file for %s: %w", path, err)
	}
	if err := temp.Close(); err != nil { // coverage-ignore: closing a successfully synced local file has no userspace failure trigger
		return fmt.Errorf("close temporary file for %s: %w", path, err)
	}
	closed = true
	if err := s.hit(label + ".rename"); err != nil {
		return err
	}
	if err := publishAtomic(tempPath, path, nil); err != nil { // coverage-ignore: exclusive directory reservation makes a destination collision a same-user namespace race; platform refusal behavior is covered directly
		return fmt.Errorf("publish temporary file without replacement to %s: %w", path, err)
	}
	published = true
	if err := s.hit(label + ".directory-fsync"); err != nil {
		return err
	}
	if err := syncDirectory(dir); err != nil { // coverage-ignore: injected directory-fsync stages cover the boundary; an actual failure requires a kernel or storage fault
		return fmt.Errorf("fsync effort directory after publishing %s: %w", path, err)
	}
	return nil
}

func syncDirectory(path string) error {
	dir, err := openDirectoryForSync(path)
	if err != nil { // coverage-ignore: callers pass an existing validated directory; open failure requires a concurrent namespace or kernel fault
		return err
	}
	if err := dir.Sync(); err != nil { // coverage-ignore: sync failure requires a kernel or storage fault
		_ = dir.Close()
		return err
	}
	return dir.Close()
}

func (s store) load(slug string) (Record, error) {
	if err := validateSlug(slug); err != nil {
		return Record{}, fmt.Errorf("invalid effort slug %q: %w; changed bytes: no; next action: use the exact slug from `awf effort list`", slug, err)
	}
	return s.loadDirectory(s.paths.effort(slug), slug, true)
}

func (s store) loadDirectory(dir, expectedSlug string, requireMemory bool) (Record, error) {
	if err := validateOwnedDirectory(dir); err != nil {
		return Record{}, &CorruptError{Path: dir, Err: err}
	}
	entries, err := os.ReadDir(dir)
	if err != nil { // coverage-ignore: validateOwnedDirectory just proved a readable owned directory; failure requires a concurrent namespace or storage fault
		return Record{}, &CorruptError{Path: dir, Err: err}
	}
	allowed := map[string]bool{"state.json": true, "memory.md": true}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return Record{}, &CorruptError{Path: filepath.Join(dir, entry.Name()), Err: errors.New("foreign leaf in effort resident")}
		}
	}
	statePath := filepath.Join(dir, "state.json")
	raw, err := readRegularNoFollow(statePath)
	if err != nil {
		return Record{}, &CorruptError{Path: statePath, Err: err}
	}
	var value persistedRecord
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return Record{}, &CorruptError{Path: statePath, Err: err}
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Record{}, &CorruptError{Path: statePath, Err: err}
	}
	if err := validatePersisted(value, expectedSlug); err != nil {
		return Record{}, &CorruptError{Path: statePath, Err: err}
	}
	if requireMemory {
		memoryPath := filepath.Join(dir, "memory.md")
		memory, err := readRegularNoFollow(memoryPath)
		if err != nil {
			return Record{}, &CorruptError{Path: memoryPath, Err: fmt.Errorf("published state has absent or invalid owned memory: %w", err)}
		}
		if !strings.HasPrefix(string(memory), "Effort: "+expectedSlug+"\n") {
			return Record{}, &CorruptError{Path: memoryPath, Err: errors.New("published state has memory with a mismatched effort identity")}
		}
	}
	return logical(value), nil
}

func validateOwnedDirectory(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return safety("symlink", path, nil)
	}
	if !info.IsDir() {
		return safety("file-type", path, fmt.Errorf("mode %s is not a directory", info.Mode()))
	}
	return ValidateCurrentOwner(path, info)
}

func (s store) list() ([]Record, error) {
	if err := s.paths.validate(s.paths.efforts); err != nil {
		return nil, fmt.Errorf("validate effort resident root before list: %w", err)
	}
	entries, err := os.ReadDir(s.paths.efforts)
	if errors.Is(err, os.ErrNotExist) {
		return []Record{}, nil
	}
	if err != nil { // coverage-ignore: resident-root validation just proved an owned directory; failure requires a concurrent namespace or storage fault
		return nil, fmt.Errorf("read efforts root: %w", err)
	}
	result := make([]Record, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(s.paths.efforts, name)
		if name == ".gitignore" {
			continue
		}
		if strings.HasPrefix(name, finishingPrefix) {
			if _, _, ok := parseTombstoneName(name); !ok {
				return nil, &CorruptError{Path: path, Err: errors.New("malformed finishing reservation")}
			}
			if err := validateOwnedDirectory(path); err != nil {
				return nil, &CorruptError{Path: path, Err: err}
			}
			continue
		}
		if err := validateSlug(name); err != nil {
			return nil, &CorruptError{Path: path, Err: fmt.Errorf("foreign or invalid effort entry: %w", err)}
		}
		if err := validateOwnedDirectory(path); err != nil {
			return nil, &CorruptError{Path: path, Err: err}
		}
		if _, err := os.Lstat(filepath.Join(path, "state.json")); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil { // coverage-ignore: local lstat returns either an inode or os.ErrNotExist absent a kernel fault
			return nil, &CorruptError{Path: filepath.Join(path, "state.json"), Err: err}
		}
		record, err := s.loadDirectory(path, name, true)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Slug < result[j].Slug })
	return result, nil
}

func tombstoneName(record Record) string {
	return finishingPrefix + record.ID + "-" + record.Slug
}

func parseTombstoneName(name string) (id, slug string, ok bool) {
	if !strings.HasPrefix(name, finishingPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(name, finishingPrefix)
	if len(rest) < 38 || rest[36] != '-' {
		return "", "", false
	}
	id, slug = rest[:36], rest[37:]
	if !uuidV4Pattern.MatchString(id) || validateSlug(slug) != nil {
		return "", "", false
	}
	return id, slug, true
}

func (s store) findTombstones(slug string) ([]string, error) {
	entries, err := os.ReadDir(s.paths.efforts)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil { // coverage-ignore: callers validate the efforts root; ReadDir failure requires a concurrent namespace or storage fault
		return nil, err
	}
	var matches []string
	for _, entry := range entries {
		id, foundSlug, ok := parseTombstoneName(entry.Name())
		if !ok || foundSlug != slug {
			continue
		}
		path := filepath.Join(s.paths.efforts, entry.Name())
		record, loadErr := s.loadDirectory(path, slug, false)
		if loadErr != nil || record.ID != id {
			return nil, &CorruptError{Path: path, Err: errors.New("finishing name does not match stored slug and UUID")}
		}
		matches = append(matches, path)
	}
	sort.Strings(matches)
	return matches, nil
}

type durableFile interface {
	Sync() error
	Close() error
}
