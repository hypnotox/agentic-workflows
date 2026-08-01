// Package manifest reads and writes the .awf/awf.lock and detects drift between rendered output and its sources.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
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
	AWFVersion             string             `json:"awfVersion"`
	SchemaVersion          int                `json:"schemaVersion"`
	Files                  map[string]Entry   `json:"files"`
	BridgeAttestation      *BridgeAttestation `json:"bridgeAttestation,omitempty"`
	InitializedWithVersion string             `json:"initializedWithVersion,omitempty"`
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
	var stamp struct {
		SchemaVersion int `json:"schemaVersion"`
	}
	if err := json.Unmarshal(b, &stamp); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	for _, key := range []string{"adrFormatV1From", "adrFormatV2From", "adrFormatV3From", "legacyAdrGaps"} {
		if _, ok := raw[key]; ok && stamp.SchemaVersion >= 31 {
			return nil, fmt.Errorf("parse lock: unknown field %q for schema %d", key, stamp.SchemaVersion)
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
