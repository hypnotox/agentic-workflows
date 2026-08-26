// Package manifest reads and writes the .awf/awf.lock and detects drift between rendered output and its sources.
package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
)

type Entry struct {
	TemplateID   string `json:"templateId"`
	TemplateHash string `json:"templateHash"`
	ConfigHash   string `json:"configHash"`
	OutputHash   string `json:"outputHash"`
	// RegenChecked marks an entry whose drift is checked by regeneration rather
	// than by the frozen OutputHash: generated indexes and navigation plus
	// in-place-editable files. Omitted when false so a plain entry's
	// serialization is unchanged.
	RegenChecked bool `json:"regenChecked,omitempty"`
}

type Lock struct {
	AWFVersion             string           `json:"awfVersion"`
	SchemaVersion          int              `json:"schemaVersion"`
	Files                  map[string]Entry `json:"files"`
	InitializedWithVersion string           `json:"initializedWithVersion,omitempty"`
}

// Clone returns a fully independent lock projection.
func (l *Lock) Clone() *Lock {
	if l == nil {
		return nil
	}
	out := *l
	out.Files = maps.Clone(l.Files)
	return &out
}

// AuthorityState identifies the sole supported permanent lock authority.
type AuthorityState uint8

const AuthorityPermanent AuthorityState = 1

// AuthorityState validates permanent lock authority. Older adopters may omit
// initialization provenance and remain valid permanent authority.
func (l *Lock) AuthorityState() (AuthorityState, error) {
	if l.InitializedWithVersion == "" {
		return AuthorityPermanent, nil
	}
	initialized, ok := NormalizeSemver(l.InitializedWithVersion)
	if !ok {
		return 0, fmt.Errorf("invalid lock authority: initializedWithVersion %q is not semantic version syntax", l.InitializedWithVersion)
	}
	awf, ok := NormalizeSemver(l.AWFVersion)
	if !ok {
		return 0, fmt.Errorf("invalid lock authority: awfVersion %q is not semantic version syntax", l.AWFVersion)
	}
	if semver.Compare(initialized, awf) > 0 {
		return 0, fmt.Errorf("invalid lock authority: initializedWithVersion %q is later than awfVersion %q", l.InitializedWithVersion, l.AWFVersion)
	}
	return AuthorityPermanent, nil
}

func NormalizeSemver(s string) (string, bool) {
	v := "v" + strings.TrimPrefix(s, "v")
	if !semver.IsValid(v) {
		return "", false
	}
	return v, true
}

type Drift struct{ Path, Kind, Detail string }

// Hash returns the stable content address used by the lock and attestation.
func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func Load(path string) (*Lock, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}
	return Parse(b)
}

// Parse rejects retired routing keys at every schema. Historical decoding is
// owned by audit rather than the live manifest model.
func Parse(b []byte) (*Lock, error) { return parse(b) }

func parse(b []byte) (*Lock, error) {
	raw, err := decodeJSONObject(b, "lock")
	if err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	schema, err := parseSchemaVersion(b)
	if err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	if _, ok := raw["bridgeAttestation"]; ok {
		return nil, fmt.Errorf("parse lock: unknown field %q", "bridgeAttestation")
	}
	for _, key := range []string{"adrFormatV1From", "adrFormatV2From", "adrFormatV3From", "legacyAdrGaps"} {
		if _, ok := raw[key]; ok {
			return nil, fmt.Errorf("parse lock: unknown field %q for schema %d", key, schema)
		}
	}
	var l Lock
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	if err := validateInventory(raw["files"]); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	if _, err := l.AuthorityState(); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	return &l, nil
}

func requireJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return fmt.Errorf("trailing input: %w", err)
	}
	return nil
}

func decodeJSONObject(b []byte, label string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, fmt.Errorf("%s must be an object", label)
	}
	fields := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name := tok.(string)
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate %s field %q", label, name)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		fields[name] = value
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if err := requireJSONEOF(dec); err != nil {
		return nil, err
	}
	return fields, nil
}

func validateInventory(raw json.RawMessage) error {
	if raw == nil {
		return errors.New("missing files inventory")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("malformed files inventory: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return errors.New("files inventory must be an object")
	}
	seen := map[string]bool{}
	count := 0
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("malformed files inventory: %w", err)
		}
		name := tok.(string)
		if name == "" || seen[name] {
			return fmt.Errorf("duplicate or empty files inventory entry %q", name)
		}
		seen[name] = true
		count++
		var entryRaw json.RawMessage
		if err := dec.Decode(&entryRaw); err != nil {
			return fmt.Errorf("malformed files inventory entry %q: %w", name, err)
		}
		if _, err := decodeJSONObject(entryRaw, "files inventory entry"); err != nil {
			return fmt.Errorf("malformed files inventory entry %q: %w", name, err)
		}
		var entry Entry
		entryDec := json.NewDecoder(bytes.NewReader(entryRaw))
		entryDec.DisallowUnknownFields()
		if err := entryDec.Decode(&entry); err != nil {
			return fmt.Errorf("malformed files inventory entry %q: %w", name, err)
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("malformed files inventory: %w", err)
	}
	if count == 0 {
		return errors.New("permanent files inventory must be nonempty")
	}
	return nil
}

func ParseLive(b []byte, floor, current int) (*Lock, error) {
	schema, err := parseSchemaVersion(b)
	if err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	if err := ValidateLive(&Lock{SchemaVersion: schema}, floor, current); err != nil {
		return nil, err
	}
	return Parse(b)
}

func (l *Lock) Save(path string) error {
	b, err := l.Marshal()
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, b)
}
func (l *Lock) Marshal() ([]byte, error) {
	if _, err := l.AuthorityState(); err != nil {
		return nil, err
	}
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// ErrUnsupportedLiveSource identifies authority that cannot be dispatched by a live operation.
var ErrUnsupportedLiveSource = errors.New("unsupported live source")

// LiveSourceError carries live compatibility facts without recovery presentation.
type LiveSourceError struct{ Schema, Floor, Current int }

func (e *LiveSourceError) Error() string {
	if e.Schema < e.Floor {
		return fmt.Sprintf("schema %d is below live floor %d", e.Schema, e.Floor)
	}
	return fmt.Sprintf("schema %d is ahead of live schema %d", e.Schema, e.Current)
}
func (e *LiveSourceError) Is(target error) bool { return target == ErrUnsupportedLiveSource }

// PartialAuthorityError identifies an incomplete control pair without prescribing recovery.
type PartialAuthorityError struct{ Config, Lock bool }

func (e *PartialAuthorityError) Error() string {
	return fmt.Sprintf("partial authority: config=%t lock=%t", e.Config, e.Lock)
}

// ValidateLive accepts only authority a user-invoked operation may dispatch on.
func ValidateLive(l *Lock, floor, current int) error {
	if l.SchemaVersion < floor || l.SchemaVersion > current {
		return &LiveSourceError{Schema: l.SchemaVersion, Floor: floor, Current: current}
	}
	return nil
}

func parseSchemaVersion(b []byte) (int, error) {
	var stamp struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(b, &stamp); err != nil {
		return 0, err
	}
	return stamp.SchemaVersion, nil
}

// LoadSchemaOptional reads only the schema stamp. Migration classification uses
// it before deciding whether current authority decoding is permitted.
func LoadSchemaOptional(path string) (int, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("unreadable .awf/awf.lock (read lock: %w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	schema, err := parseSchemaVersion(b)
	if err != nil {
		return 0, false, fmt.Errorf("unreadable .awf/awf.lock (parse lock: %w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	return schema, true, nil
}

// LoadLive reads a lock only after its schema stamp passes the live-source
// boundary, so unsupported bytes never reach current authority interpretation.
func LoadLive(path string, floor, current int) (*Lock, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read lock: %w", err)
	}
	return ParseLive(b, floor, current)
}

// LoadLiveOptional is the optional-file form of LoadLive.
func LoadLiveOptional(path string, floor, current int) (*Lock, bool, error) {
	l, err := LoadLive(path, floor, current)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	return l, true, nil
}

func LoadOptional(path string) (*Lock, bool, error) {
	l, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("unreadable .awf/awf.lock (%w): restore it from version control, or delete it deliberately to re-adopt", err)
	}
	return l, true, nil
}

// WriteFileAtomic writes data through a same-directory temporary file and
// rename, so a failed write never leaves a truncated destination.
func WriteFileAtomic(path string, data []byte) error { return WriteFileAtomicMode(path, data, 0o644) }

// WriteFileAtomicMode is WriteFileAtomic with an explicit final mode, used when
// a restored file's recorded permissions must survive the replacement.
// touches-state: config/migrations-and-locks:lock-atomic-save - atomic temp-file+rename write site; proof in manifest_test.go
func WriteFileAtomicMode(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".awf-atomic-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Chmod(name, mode)
	}
	if werr == nil {
		werr = os.Rename(name, path)
	}
	if werr != nil {
		_ = os.Remove(name)
		return werr
	}
	return nil
}
