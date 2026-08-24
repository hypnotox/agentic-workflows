package effort

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/filepublication"
)

var uuidV4Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

const finishingPrefix = ".finishing-"

// maxTitleBytes bounds the independently persisted outcome title.
const maxTitleBytes = 160

// maxNewSlugBytes keeps newly minted caller-selected identities concise while
// resident validation retains the historical 63-byte compatibility boundary.
const maxNewSlugBytes = 32

// CorruptError identifies resident input that must be preserved byte-for-byte.
type CorruptError struct {
	Path string
	Err  error
}

// residentReadError marks a failed resident read mechanism. Callers that turn
// a resident refusal into a protocol outcome can expose its bounded cause
// without confusing malformed or unsafe content for a mechanism failure.
type residentReadError struct{ error }

func (e *residentReadError) Unwrap() error { return e.error }

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

func (s store) logical(r persistedRecord) Record {
	return Record{SchemaVersion: r.SchemaVersion, ID: r.ID, Slug: r.Slug, Title: r.Title, CreatedAt: r.CreatedAt, MemoryPath: s.paths.publicMemoryPath(r.Slug)}
}

func normalizeTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("outcome title must be nonblank")
	}
	if !utf8.ValidString(title) {
		return "", errors.New("outcome title must be valid UTF-8")
	}
	if len(title) > maxTitleBytes {
		return "", fmt.Errorf("outcome title must be at most %d UTF-8 bytes", maxTitleBytes)
	}
	return title, nil
}

func validateNewSlug(ctx context.Context, validateRef func(context.Context, string) (bool, error), slug string) error {
	if len(slug) < 1 || len(slug) > maxNewSlugBytes {
		return newSlugRefusal(slug, "slug must contain 1-32 bytes")
	}
	if err := validateSlug(slug); err != nil {
		return newSlugRefusal(slug, err.Error())
	}
	// The ref probe runs once at minting time. Resident reads intentionally use
	// only validateSlug so listing never forks Git once per effort.
	branch := "awf/" + slug
	valid, err := validateRef(ctx, branch)
	if err != nil {
		return refusal(fmt.Sprintf("validate Git ref for explicit effort slug %q: %v; changed bytes: no; next action: repair the Git installation and retry with `--slug %q`", slug, err, slug), "explicit effort slug could not be validated", "git", err.Error(), []RecoveryAction{{Text: fmt.Sprintf("repair the Git installation and retry with `--slug %q`", slug)}}, err)
	}
	if !valid {
		return newSlugRefusal(slug, "refs/heads/"+branch+" is not a valid Git ref")
	}
	return nil
}

func newSlugRefusal(slug, condition string) error {
	return refusal("invalid explicit effort slug: "+condition+"; changed bytes: no; next action: provide a different canonical value with `--slug`", fmt.Sprintf("explicit effort slug %q is invalid", slug), "input", condition, []RecoveryAction{{Text: "provide a different canonical value with `--slug`"}}, nil)
}

func invalidSlugRefusal(slug string, err error) error {
	return refusal(fmt.Sprintf("invalid effort slug %q: %v; changed bytes: no; next action: use the exact slug from `awf effort list`", slug, err), fmt.Sprintf("effort slug %q is invalid", slug), "input", err.Error(), []RecoveryAction{{Text: "use the exact slug from `awf effort list`"}}, err)
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

func (s store) reserve(record Record) (string, error) {
	slug := record.Slug
	if err := s.paths.ensure(s.paths.efforts); err != nil {
		return "", fmt.Errorf("prepare efforts root: %w", err)
	}
	if tombstone, err := s.findTombstones(slug); err != nil {
		return "", err
	} else if len(tombstone) > 0 {
		return "", refusal(fmt.Sprintf("effort slug %q is reserved by finishing cleanup; changed bytes: no; next action: run `awf effort finish %s`", slug, slug), fmt.Sprintf("effort slug %q is reserved by finishing cleanup", slug), "resident", "", []RecoveryAction{{Text: fmt.Sprintf("run `awf effort finish %s`", slug)}}, nil)
	}
	dir := s.paths.effort(slug)
	if err := os.Mkdir(dir, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			condition := "an incomplete reservation exists"
			if _, statErr := os.Lstat(s.paths.stateFile(slug)); statErr == nil {
				condition = "an active effort already exists"
			}
			return "", refusal(fmt.Sprintf("effort slug %q collides because %s; changed bytes: no; next action: choose a distinct explicit slug, then retry `awf effort new --slug %q %q` after replacing the quoted slug, or inspect %s", slug, condition, slug, record.Title, dir), fmt.Sprintf("effort slug %q collides", slug), "resident", condition, []RecoveryAction{{Text: "choose a distinct explicit slug"}, {Text: fmt.Sprintf("retry `awf effort new --slug %q %q` after replacing the quoted slug", slug, record.Title)}, {Text: "inspect " + dir}}, nil)
		}
		return "", fmt.Errorf("reserve effort directory %s: %w", dir, err) // coverage-ignore: ensure and tombstone enumeration just proved the parent usable; a non-collision failure requires a concurrent namespace or storage fault
	}
	if err := s.hit("reserve.directory"); err != nil {
		return "", err
	}
	return dir, nil
}

func (s store) create(record Record) error {
	dir, err := s.reserve(record)
	if err != nil {
		return err
	}
	if err := s.publishNew(s.paths.memoryFile(record.Slug), memorySkeleton(record.Slug, record.CreatedAt), "memory"); err != nil {
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

type memoryPublicationError struct {
	Changed bool
	Err     error
}

func (e *memoryPublicationError) Error() string { return e.Err.Error() }
func (e *memoryPublicationError) Unwrap() error { return e.Err }

func memoryPublicationChanged(err error) bool {
	var publication *memoryPublicationError
	return errors.As(err, &publication) && publication.Changed
}

// replaceMemory publishes a fully synced sibling and then atomically replaces
// the old memory. All injected failures before rename leave the old bytes.
func (s store) replaceMemory(path string, raw []byte) (returnErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".memory-update-*.tmp")
	if err != nil { // coverage-ignore: the owned effort directory is validated before update; CreateTemp failure requires a concurrent permission change or storage fault
		return fmt.Errorf("create sibling temporary memory: %w", err)
	}
	tempPath := temp.Name()
	closed, published := false, false
	defer func() {
		if !closed {
			returnErr = errors.Join(returnErr, temp.Close())
		}
		if !published {
			if err := os.Remove(tempPath); err != nil && !errors.Is(err, os.ErrNotExist) { // coverage-ignore: the locally-created sibling can disappear, but a non-ENOENT removal failure requires a kernel or storage fault
				returnErr = errors.Join(returnErr, err)
			}
		}
	}()
	if err := s.hit("memory-update.write"); err != nil {
		return err
	}
	if n, err := temp.Write(raw); err != nil { // coverage-ignore: injected write stages cover the boundary; a local temporary write failure requires a kernel or storage fault
		return fmt.Errorf("write temporary memory: %w", err)
	} else if n != len(raw) { // coverage-ignore: os.File.Write returns a non-nil error when it writes fewer bytes than requested
		return io.ErrShortWrite
	}
	if err := s.hit("memory-update.fsync"); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil { // coverage-ignore: injected fsync stages cover the boundary; a local temporary sync failure requires a kernel or storage fault
		return fmt.Errorf("fsync temporary memory: %w", err)
	}
	if err := temp.Close(); err != nil { // coverage-ignore: a close failure after a successful local write requires a kernel or storage fault
		return fmt.Errorf("close temporary memory: %w", err)
	}
	closed = true
	if err := s.hit("memory-update.rename"); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil { // coverage-ignore: the validated resident parent and sibling temporary make a rename failure a concurrent namespace or storage fault
		return fmt.Errorf("replace memory atomically: %w", err)
	}
	published = true
	if err := s.hit("memory-update.directory-fsync"); err != nil {
		return &memoryPublicationError{Changed: true, Err: err}
	}
	if err := syncDirectory(dir); err != nil { // coverage-ignore: injected directory-fsync stages cover the durability boundary; a real failure requires a kernel or storage fault
		return &memoryPublicationError{Changed: true, Err: fmt.Errorf("fsync effort directory after memory update: %w", err)}
	}
	return nil
}

func (s store) publishNew(path string, raw []byte, label string) error {
	if err := s.hit(label + ".write"); err != nil {
		return err
	}
	if err := s.hit(label + ".fsync"); err != nil {
		return err
	}
	if err := s.hit(label + ".rename"); err != nil {
		return err
	}
	if err := filepublication.Publish(path, raw, 0o600); err != nil { // coverage-ignore: exclusive directory reservation makes a destination collision a same-user namespace race; shared publication behavior is covered directly
		return fmt.Errorf("publish temporary file without replacement to %s: %w", path, err)
	}
	if err := s.hit(label + ".directory-fsync"); err != nil {
		return err
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil { // coverage-ignore: injected directory-fsync stages cover the durability boundary; an actual failure requires a kernel or storage fault
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
		return Record{}, invalidSlugRefusal(slug, err)
	}
	return s.loadDirectory(s.paths.effort(slug), slug, true)
}

func (s store) loadDirectory(dir, expectedSlug string, requireMemory bool) (Record, error) {
	if err := validateOwnedDirectory(dir); err != nil {
		return Record{}, &CorruptError{Path: dir, Err: err}
	}
	entries, err := os.ReadDir(dir)
	if err != nil { // coverage-ignore: validateOwnedDirectory just proved a readable owned directory; failure requires a concurrent namespace or storage fault
		return Record{}, &CorruptError{Path: dir, Err: &residentReadError{err}}
	}
	allowed := map[string]bool{"state.json": true, "memory.md": true, "activity.json": true, "scratch": true}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if !allowed[entry.Name()] {
			return Record{}, &CorruptError{Path: path, Err: errors.New("foreign leaf in effort resident")}
		}
		if entry.Name() == "scratch" {
			// Scratch descendants are intentionally opaque. Validate only the
			// direct child inode and never enumerate or interpret its contents.
			if err := validateOwnedDirectory(path); err != nil {
				return Record{}, &CorruptError{Path: path, Err: err}
			}
		}
	}
	statePath := filepath.Join(dir, "state.json")
	raw, err := readRegularNoFollow(statePath)
	if err != nil {
		return Record{}, &CorruptError{Path: statePath, Err: &residentReadError{err}}
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
			return Record{}, &CorruptError{Path: memoryPath, Err: &residentReadError{fmt.Errorf("published state has absent or invalid owned memory: %w", err)}}
		}
		if err := readMemoryIdentity(memory, expectedSlug); err != nil {
			return Record{}, &CorruptError{Path: memoryPath, Err: fmt.Errorf("published state has memory with a mismatched effort identity: %w", err)}
		}
	}
	return s.logical(value), nil
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
	if err != nil {
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
