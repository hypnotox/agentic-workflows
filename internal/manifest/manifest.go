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
}

type Lock struct {
	AWFVersion    string           `json:"awfVersion"`
	SchemaVersion int              `json:"schemaVersion"`
	Files         map[string]Entry `json:"files"`
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
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil { // coverage-ignore: Lock holds only strings, an int, and a string-keyed map of string fields; MarshalIndent has no unsupported type to fail on
		return err
	}
	b = append(b, '\n')
	return WriteFileAtomic(path, b)
}

// LoadOptional is the corrupt-lock policy choke point (ADR-0076 Decision 2): a
// missing lock reports found=false with no error so callers keep their no-lock
// semantics; a present-but-unreadable lock is a hard error carrying the one
// recovery hint.
// invariant: corrupt-lock-refuses
func LoadOptional(path string) (*Lock, bool, error) {
	l, err := Load(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("unreadable .awf/awf.lock (%w) — restore it from version control, or delete it deliberately to re-adopt", err)
	}
	return l, true, nil
}

// WriteFileAtomic writes data to path via a same-directory temp file renamed
// into place, so a crash can never leave a truncated file at path. Mode is
// 0o644 (CreateTemp's 0o600 is widened before the rename). On error the temp
// file is best-effort removed. Rename-only durability — no fsync — per
// ADR-0076 Decision 1; Go's os.Rename replaces an existing destination on
// every supported OS including Windows.
// invariant: lock-atomic-save
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
