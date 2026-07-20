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
	"slices"

	"golang.org/x/mod/semver"
)

type Entry struct {
	TemplateID   string `json:"templateId"`
	TemplateHash string `json:"templateHash"`
	ConfigHash   string `json:"configHash"`
	OutputHash   string `json:"outputHash"`
	// RegenChecked marks an entry whose drift is checked by regeneration rather
	// than by the frozen OutputHash - the generated indexes (ACTIVE.md, per-domain
	// docs, the config reference) and in-place-editable files (ADR-0100). Omitted
	// when false so a plain entry's serialization is unchanged.
	RegenChecked bool `json:"regenChecked,omitempty"`
}

type Lock struct {
	AWFVersion    string           `json:"awfVersion"`
	SchemaVersion int              `json:"schemaVersion"`
	Files         map[string]Entry `json:"files"`
	// BridgeAttestation seals a completed current-state upgrade attestation. It
	// is optional (omitted when absent) so a lock written before the bridge
	// tranche parses unchanged with a nil pointer.
	BridgeAttestation *BridgeAttestation `json:"bridgeAttestation,omitempty"`
	// ADRFormatV1From is the permanent format cutoff (the highest ADR number plus
	// one) the final current-state upgrade promotes out of the consumed
	// attestation. Every ADR at or above it is current-state-v1; below it is
	// legacy. Zero is valid only for bridge or pre-tracking authority.
	ADRFormatV1From int `json:"adrFormatV1From,omitempty"`
	// LegacyADRGaps is the sorted set of absent lower ADR numbers the final
	// upgrade promotes alongside the cutoff, closing the migration-time identity
	// set so a listed gap can never be backfilled as legacy. It is absent before
	// cutover and serialized as an explicit array, including [], after cutover.
	LegacyADRGaps []int `json:"legacyAdrGaps,omitempty"`
	// InitializedWithVersion records the binary that completed first adoption.
	// It is absent for older migrated adopters and immutable once present.
	InitializedWithVersion string `json:"initializedWithVersion,omitempty"`

	legacyADRGapsPresent  bool
	authorityFieldsParsed bool
}

// AuthorityState is the closed lock-authority state machine.
type AuthorityState uint8

const (
	AuthorityBridge AuthorityState = iota + 1
	AuthorityPermanent
	AuthorityPreTracking
)

// AuthorityState validates and classifies the lock's current-state authority.
func (l *Lock) AuthorityState() (AuthorityState, error) {
	gapsPresent := l.legacyADRGapsPresent || l.LegacyADRGaps != nil
	if !l.authorityFieldsParsed && l.ADRFormatV1From > 0 {
		gapsPresent = true
	}
	hasBridge := l.BridgeAttestation != nil
	hasCutoff := l.ADRFormatV1From != 0
	hasInit := l.InitializedWithVersion != ""

	if hasBridge {
		if hasCutoff || gapsPresent || hasInit {
			return 0, errors.New("invalid lock authority: bridge attestation cannot be mixed with permanent or initialization authority")
		}
		return AuthorityBridge, nil
	}
	if !hasCutoff && !gapsPresent && !hasInit {
		return AuthorityPreTracking, nil
	}
	if !hasCutoff {
		return 0, errors.New("invalid lock authority: initializedWithVersion or legacyAdrGaps requires adrFormatV1From")
	}
	if l.ADRFormatV1From < 1 {
		return 0, errors.New("invalid lock authority: adrFormatV1From must be positive")
	}
	if !gapsPresent {
		return 0, errors.New("invalid lock authority: adrFormatV1From requires an explicit legacyAdrGaps field")
	}
	previous := 0
	for _, gap := range l.LegacyADRGaps {
		if gap <= 0 || gap >= l.ADRFormatV1From {
			return 0, fmt.Errorf("invalid lock authority: legacyAdrGaps value %d must be positive and below cutoff %d", gap, l.ADRFormatV1From)
		}
		if gap <= previous {
			return 0, errors.New("invalid lock authority: legacyAdrGaps must be sorted and unique")
		}
		previous = gap
	}
	if hasInit {
		v := "v" + l.InitializedWithVersion
		awf := "v" + l.AWFVersion
		if !semver.IsValid(v) {
			return 0, fmt.Errorf("invalid lock authority: initializedWithVersion %q is not semantic version syntax", l.InitializedWithVersion)
		}
		if !semver.IsValid(awf) || semver.Compare(v, awf) > 0 {
			return 0, fmt.Errorf("invalid lock authority: initializedWithVersion %q is later than awfVersion %q", l.InitializedWithVersion, l.AWFVersion)
		}
	}
	return AuthorityPermanent, nil
}

// BridgeAttestation records the sealed identity of a current-state upgrade
// attestation: the format Version, the clean PreparedHead commit it was
// computed against, the TreeDigest over the post-normalization prepared inputs,
// the ADRFormatV1From cutoff (highest ADR number plus one), and the sorted
// absent lower ADR numbers in LegacyADRGaps.
type BridgeAttestation struct {
	Version         int    `json:"version"`
	PreparedHead    string `json:"preparedHead"`
	TreeDigest      string `json:"treeDigest"`
	ADRFormatV1From int    `json:"adrFormatV1From"`
	LegacyADRGaps   []int  `json:"legacyADRGaps"`
}

type Drift struct{ Path, Kind, Detail string }

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

// Parse decodes lock bytes from any snapshot universe.
func Parse(b []byte) (*Lock, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	_, l.legacyADRGapsPresent = raw["legacyAdrGaps"]
	l.authorityFieldsParsed = true
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

// Marshal returns the canonical lock serialization Save writes: indented JSON
// with a trailing newline. Attestation reuses it so the sealed lock bytes match
// what a subsequent Load/Save round-trips.
func (l *Lock) Marshal() ([]byte, error) {
	if _, err := l.AuthorityState(); err != nil {
		return nil, err
	}
	// A pointer distinguishes the permanent post-cutover empty gap set ([])
	// from the pre-cutover absence of gap authority (field omitted).
	var gaps *[]int
	if l.ADRFormatV1From != 0 {
		value := slices.Clone(l.LegacyADRGaps)
		if value == nil {
			value = []int{}
		}
		gaps = &value
	}
	canonical := struct {
		AWFVersion             string             `json:"awfVersion"`
		SchemaVersion          int                `json:"schemaVersion"`
		Files                  map[string]Entry   `json:"files"`
		BridgeAttestation      *BridgeAttestation `json:"bridgeAttestation,omitempty"`
		ADRFormatV1From        int                `json:"adrFormatV1From,omitempty"`
		LegacyADRGaps          *[]int             `json:"legacyAdrGaps,omitempty"`
		InitializedWithVersion string             `json:"initializedWithVersion,omitempty"`
	}{l.AWFVersion, l.SchemaVersion, l.Files, l.BridgeAttestation, l.ADRFormatV1From, gaps, l.InitializedWithVersion}
	b, err := json.MarshalIndent(canonical, "", "  ")
	if err != nil { // coverage-ignore: the canonical lock holds only JSON-supported scalar, slice, map, and struct fields
		return nil, err
	}
	return append(b, '\n'), nil
}

// LoadOptional is the corrupt-lock policy choke point (ADR-0076 Decision 2): a
// missing lock reports found=false with no error so callers keep their no-lock
// semantics; a present-but-unreadable lock is a hard error carrying the one
// recovery hint.
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

// WriteFileAtomic writes data to path via a same-directory temp file renamed
// into place, so a crash can never leave a truncated file at path. Mode is
// 0o644 (CreateTemp's 0o600 is widened before the rename). On error the temp
// file is best-effort removed. Rename-only durability - no fsync - per
// ADR-0076 Decision 1; Go's os.Rename replaces an existing destination on
// every supported OS including Windows.
// touches-state: config/migrations-and-locks:lock-atomic-save - atomic temp-file+rename write site; proof in manifest_test.go
func WriteFileAtomic(path string, data []byte) error {
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
		werr = os.Chmod(name, 0o644)
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
