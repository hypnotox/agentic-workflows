// Package manifest reads and writes the .awf/awf.lock and detects drift between rendered output and its sources.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
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
	AWFVersion             string             `json:"awfVersion"`
	SchemaVersion          int                `json:"schemaVersion"`
	Files                  map[string]Entry   `json:"files"`
	BridgeAttestation      *BridgeAttestation `json:"bridgeAttestation,omitempty"`
	InitializedWithVersion string             `json:"initializedWithVersion,omitempty"`
}

// Clone returns a fully independent lock projection.
func (l *Lock) Clone() *Lock {
	if l == nil {
		return nil
	}
	out := *l
	out.Files = maps.Clone(l.Files)
	if l.BridgeAttestation != nil {
		bridge := *l.BridgeAttestation
		bridge.LegacyADRGaps = slices.Clone(l.BridgeAttestation.LegacyADRGaps)
		out.BridgeAttestation = &bridge
	}
	return &out
}

// AuthorityState distinguishes the frozen bridge input from ordinary locks.
type AuthorityState uint8

const (
	AuthorityBridge AuthorityState = iota + 1
	AuthorityPermanent
)

// AuthorityState validates the one still-active immutable lock value. Historical
// routing fields are compatibility input only and are discarded by Parse.
func (l *Lock) AuthorityState() (AuthorityState, error) {
	if l.BridgeAttestation != nil {
		if l.InitializedWithVersion != "" {
			return 0, errors.New("invalid lock authority: bridge attestation cannot be mixed with initialization authority")
		}
		if l.BridgeAttestation.LegacyADRGaps == nil {
			return 0, errors.New("invalid lock authority: bridgeAttestation legacyADRGaps must be an array, not null")
		}
		if err := validateBoundary("bridgeAttestation", l.BridgeAttestation.ADRFormatV1From, l.BridgeAttestation.LegacyADRGaps); err != nil {
			return 0, err
		}
		return AuthorityBridge, nil
	}
	if l.InitializedWithVersion == "" {
		// Older adopters predate first-adoption version tracking. Their lock is
		// still ordinary authority; only an absent lock is pre-tracking.
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
func validateBoundary(owner string, cutoff int, gaps []int) error {
	if cutoff < 1 {
		return fmt.Errorf("invalid lock authority: %s adrFormatV1From must be positive", owner)
	}
	previous := 0
	for _, gap := range gaps {
		if gap <= 0 || gap >= cutoff {
			return fmt.Errorf("invalid lock authority: %s legacyADRGaps value %d must be positive and below cutoff %d", owner, gap, cutoff)
		}
		if gap <= previous {
			return fmt.Errorf("invalid lock authority: %s legacyADRGaps must be sorted and unique", owner)
		}
		previous = gap
	}
	return nil
}

// BridgeAttestation preserves version-1 bridge bytes as frozen compatibility
// input. Its historical routing payload is never promoted into a Lock.
type BridgeAttestation struct {
	Version         int    `json:"version"`
	PreparedHead    string `json:"preparedHead"`
	TreeDigest      string `json:"treeDigest"`
	ADRFormatV1From int    `json:"adrFormatV1From"`
	LegacyADRGaps   []int  `json:"legacyADRGaps"`
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

// Parse accepts retired routing keys only in schema 30 and earlier, where it
// discards them. Schema 31 seals their retirement by refusing their presence.
func Parse(b []byte) (*Lock, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	schema, err := parseSchemaVersion(b)
	if err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	for _, key := range []string{"adrFormatV1From", "adrFormatV2From", "adrFormatV3From", "legacyAdrGaps"} {
		if _, ok := raw[key]; ok && schema >= 31 {
			return nil, fmt.Errorf("parse lock: unknown field %q for schema %d", key, schema)
		}
	}
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	if _, err := l.AuthorityState(); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	return &l, nil
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
	if err != nil { // coverage-ignore: Lock contains only JSON-supported fields
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
