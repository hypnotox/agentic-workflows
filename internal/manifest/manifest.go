// Package manifest reads and writes the .awf/awf.lock and detects drift between rendered output and its sources.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
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
	return os.WriteFile(path, b, 0o644)
}
