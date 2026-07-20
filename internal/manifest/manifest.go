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
	// legacy. Zero (omitted) before cutover, when no ADR is current-state-v1.
	ADRFormatV1From int `json:"adrFormatV1From,omitempty"`
	// LegacyADRGaps is the sorted set of absent lower ADR numbers the final
	// upgrade promotes alongside the cutoff, closing the migration-time identity
	// set so a listed gap can never be backfilled as legacy. Omitted when empty.
	LegacyADRGaps []int `json:"legacyAdrGaps,omitempty"`
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
	var l Lock
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	return &l, nil
}

func (l *Lock) Save(path string) error {
	b, err := l.Marshal()
	if err != nil { // coverage-ignore: Marshal fails only on an unsupported type, which Lock never holds
		return err
	}
	return WriteFileAtomic(path, b)
}

// Marshal returns the canonical lock serialization Save writes: indented JSON
// with a trailing newline. Attestation reuses it so the sealed lock bytes match
// what a subsequent Load/Save round-trips.
func (l *Lock) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil { // coverage-ignore: Lock holds only strings, ints, an int slice, a string-keyed map of string fields, and an optional attestation of the same; MarshalIndent has no unsupported type to fail on
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
