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
	"strings"

	"golang.org/x/mod/semver"
)

type Entry struct {
	TemplateID   string `json:"templateId"`
	TemplateHash string `json:"templateHash"`
	ConfigHash   string `json:"configHash"`
	OutputHash   string `json:"outputHash"`
	// RegenChecked marks an entry whose drift is checked by regeneration rather
	// than by the frozen OutputHash - generated indexes and navigation (INDEX.md,
	// topic and domain docs, the config reference) plus in-place-editable files
	// (ADR-0100). Omitted
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
	// ADRFormatV2From is the permanent V2 boundary. It is absent before schema
	// 15 and positive at schema 15 and later.
	ADRFormatV2From int `json:"adrFormatV2From,omitempty"`
	// LegacyADRGaps is the sorted set of absent lower ADR numbers the final
	// upgrade promotes alongside the cutoff, closing the migration-time identity
	// set so a listed gap can never be backfilled as legacy. It is absent before
	// cutover and serialized as an explicit array, including [], after cutover.
	LegacyADRGaps []int `json:"legacyAdrGaps,omitempty"`
	// InitializedWithVersion records the binary that completed first adoption.
	// It is absent for older migrated adopters and immutable once present.
	InitializedWithVersion string `json:"initializedWithVersion,omitempty"`

	legacyADRGapsPresent   bool
	adrFormatV2FromPresent bool
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
	hasBridge := l.BridgeAttestation != nil
	hasCutoff := l.ADRFormatV1From != 0
	hasV2 := l.adrFormatV2FromPresent || l.ADRFormatV2From != 0
	hasInit := l.InitializedWithVersion != ""

	if hasBridge {
		if hasCutoff || hasV2 || gapsPresent || hasInit {
			return 0, errors.New("invalid lock authority: bridge attestation cannot be mixed with permanent or initialization authority")
		}
		if l.BridgeAttestation.LegacyADRGaps == nil {
			return 0, errors.New("invalid lock authority: bridgeAttestation legacyADRGaps must be an array, not null")
		}
		if err := validateBoundary("bridgeAttestation", l.BridgeAttestation.ADRFormatV1From, l.BridgeAttestation.LegacyADRGaps); err != nil {
			return 0, err
		}
		return AuthorityBridge, nil
	}
	if !hasCutoff && !hasV2 && !gapsPresent && !hasInit {
		return AuthorityPreTracking, nil
	}
	if !hasCutoff {
		return 0, errors.New("invalid lock authority: adrFormatV2From, initializedWithVersion, or legacyAdrGaps requires adrFormatV1From")
	}
	if l.ADRFormatV1From < 1 {
		return 0, errors.New("invalid lock authority: adrFormatV1From must be positive")
	}
	if !gapsPresent {
		return 0, errors.New("invalid lock authority: adrFormatV1From requires an explicit non-nil legacyAdrGaps field")
	}
	if l.LegacyADRGaps == nil {
		return 0, errors.New("invalid lock authority: legacyAdrGaps must be an array, not null")
	}
	if err := validateBoundary("lock authority", l.ADRFormatV1From, l.LegacyADRGaps); err != nil {
		return 0, err
	}
	if l.SchemaVersion >= 15 && !hasV2 {
		return 0, errors.New("invalid lock authority: schema 15 permanent authority requires adrFormatV2From")
	}
	if hasV2 {
		if l.ADRFormatV2From < 1 {
			return 0, errors.New("invalid lock authority: adrFormatV2From must be positive")
		}
		if l.ADRFormatV2From < l.ADRFormatV1From {
			return 0, errors.New("invalid lock authority: adrFormatV2From must be greater than or equal to adrFormatV1From")
		}
	}
	if hasInit {
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
	}
	return AuthorityPermanent, nil
}

// NormalizeSemver returns s in the single-leading-v form x/mod/semver requires.
// Lock authority and the CLI version gate share it so historical v-prefixed
// versions are interpreted identically.
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
			return fmt.Errorf("invalid lock authority: %s legacyAdrGaps value %d must be positive and below cutoff %d", owner, gap, cutoff)
		}
		if gap <= previous {
			return fmt.Errorf("invalid lock authority: %s legacyAdrGaps must be sorted and unique", owner)
		}
		previous = gap
	}
	return nil
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
	_, l.adrFormatV2FromPresent = raw["adrFormatV2From"]
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
		gaps = &value
	}
	canonical := struct {
		AWFVersion             string             `json:"awfVersion"`
		SchemaVersion          int                `json:"schemaVersion"`
		Files                  map[string]Entry   `json:"files"`
		BridgeAttestation      *BridgeAttestation `json:"bridgeAttestation,omitempty"`
		ADRFormatV1From        int                `json:"adrFormatV1From,omitempty"`
		ADRFormatV2From        int                `json:"adrFormatV2From,omitempty"`
		LegacyADRGaps          *[]int             `json:"legacyAdrGaps,omitempty"`
		InitializedWithVersion string             `json:"initializedWithVersion,omitempty"`
	}{l.AWFVersion, l.SchemaVersion, l.Files, l.BridgeAttestation, l.ADRFormatV1From, l.ADRFormatV2From, gaps, l.InitializedWithVersion}
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
