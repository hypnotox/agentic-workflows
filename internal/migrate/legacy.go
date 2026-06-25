package migrate

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// This file carries a FROZEN snapshot of the pre-ADR-0009 single-file config
// shape (the monolithic `.claude/awf.yaml`). It is the SOLE reader of the legacy
// file and the single named exemption to ADR-0009 `inv: config-root`
// (ADR-0010 `inv: legacy-read-isolation`). It must never import internal/config
// and must not drift toward the live config structs; it is a snapshot, not an
// alias, so future config changes do not silently change what a migration reads.

type legacySectionOverride struct {
	ReplaceWith string `yaml:"replaceWith"`
	Drop        bool   `yaml:"drop"`
}

type legacySidecar struct {
	Data     map[string]any                   `yaml:"data"`
	Sections map[string]legacySectionOverride `yaml:"sections"`
	Local    bool                             `yaml:"local"`
}

type legacyInvariantSource struct {
	Globs  []string `yaml:"globs"`
	Marker string   `yaml:"marker"`
}

type legacyInvariantConfig struct {
	Disabled bool                    `yaml:"disabled"`
	Sources  []legacyInvariantSource `yaml:"sources"`
}

type legacyConfig struct {
	Prefix     string                   `yaml:"prefix"`
	DocsDir    string                   `yaml:"docsDir"`
	Vars       map[string]any           `yaml:"vars"`
	Skills     map[string]legacySidecar `yaml:"skills"`
	Agents     map[string]legacySidecar `yaml:"agents"`
	Hooks      []string                 `yaml:"hooks"`
	AgentsDoc  *legacySidecar           `yaml:"agentsDoc"`
	Docs       map[string]legacySidecar `yaml:"docs"`
	Invariants *legacyInvariantConfig   `yaml:"invariants"`
}

// readLegacy parses a pre-ADR-0009 monolithic .claude/awf.yaml with a strict
// decoder. It is the only function in the codebase that reads that file.
func readLegacy(awfYAMLPath string) (*legacyConfig, error) {
	b, err := os.ReadFile(awfYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("read legacy config: %w", err)
	}
	var c legacyConfig
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse legacy config: %w", err)
	}
	return &c, nil
}
