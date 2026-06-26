// Package config loads and validates the per-project .claude/awf/ configuration:
// a skeleton config.yaml plus per-target sidecar YAMLs and convention parts.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type SectionOverride struct {
	ReplaceWith string `yaml:"replaceWith"`
	Drop        bool   `yaml:"drop"`
}

// Sidecar holds a single target's non-prose configuration: structured render
// data, per-section overrides, and the local flag. It lives at
// <awfDir>/<kind>/<name>.yaml (agents-doc: <awfDir>/agents-doc.yaml). An absent
// sidecar is the zero Sidecar (publication-safe: empty data/sections).
type Sidecar struct {
	Data     map[string]any             `yaml:"data"`
	Sections map[string]SectionOverride `yaml:"sections"`
	Local    bool                       `yaml:"local"`
}

// Config is the skeleton config.yaml: global fields plus flat enable arrays.
// Presence of a name in Skills/Agents/Docs/Hooks enables that target; per-target
// data/sections/local live in sidecars, not here.
type Config struct {
	Prefix     string           `yaml:"prefix"`
	DocsDir    string           `yaml:"docsDir"`
	Vars       map[string]any   `yaml:"vars"`
	Skills     []string         `yaml:"skills"`
	Agents     []string         `yaml:"agents"`
	Hooks      []string         `yaml:"hooks"`
	Docs       []string         `yaml:"docs"`
	Domains    []string         `yaml:"domains"`
	Invariants *InvariantConfig `yaml:"invariants"`
	root       string           // <project>/.claude/awf, for sidecar/part resolution
}

// InvariantConfig configures language-agnostic invariant backing. A nil
// *InvariantConfig (key absent) means "unchecked"; Disabled is the explicit
// opt-out; a non-empty Sources enables enforcement.
type InvariantConfig struct {
	Disabled bool              `yaml:"disabled"`
	Sources  []InvariantSource `yaml:"sources"`
}

// InvariantSource pairs filename globs (matched against a file's basename) with
// the literal comment marker that prefixes a backing `invariant: <slug>` tag.
type InvariantSource struct {
	Globs  []string `yaml:"globs"`
	Marker string   `yaml:"marker"`
}

// Load reads <awfDir>/config.yaml with the strict decoder, records awfDir as the
// sidecar/part resolution root, and defaults DocsDir.
func Load(awfDir string) (*Config, error) {
	b, err := os.ReadFile(filepath.Join(awfDir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.root = awfDir
	if c.DocsDir == "" {
		c.DocsDir = "docs"
	}
	return &c, nil
}

// Sidecar reads <root>/<kind>/<name>.yaml; agents-doc lives at <root>/agents-doc.yaml.
// A missing file yields a zero Sidecar (publication-safe: empty data/sections).
func (c *Config) Sidecar(kind, name string) (Sidecar, error) {
	var rel string
	if kind == "agents-doc" {
		rel = "agents-doc.yaml"
	} else {
		rel = filepath.Join(kind, name+".yaml")
	}
	b, err := os.ReadFile(filepath.Join(c.root, rel))
	if errors.Is(err, os.ErrNotExist) {
		return Sidecar{}, nil
	}
	if err != nil {
		return Sidecar{}, fmt.Errorf("read sidecar %s: %w", rel, err)
	}
	var s Sidecar
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&s); err != nil {
		return Sidecar{}, fmt.Errorf("parse sidecar %s: %w", rel, err)
	}
	if err := checkSections(kind, name, s); err != nil {
		return Sidecar{}, err
	}
	return s, nil
}

// PartPath returns the convention part path for a section of a target.
func (c *Config) PartPath(kind, target, section string) string {
	if kind == "agents-doc" {
		return filepath.Join(c.root, "parts", "agents-doc", section+".md")
	}
	return filepath.Join(c.root, kind, "parts", target, section+".md")
}

func (c *Config) Validate() error {
	if c.Prefix == "" {
		return errors.New("prefix must not be empty")
	}
	if strings.ContainsAny(c.Prefix, "/\\") || strings.Contains(c.Prefix, "..") {
		return fmt.Errorf("prefix %q must not contain path separators", c.Prefix)
	}
	if strings.HasPrefix(c.DocsDir, "/") || strings.Contains(c.DocsDir, "..") {
		return fmt.Errorf("docsDir %q must be a relative path without \"..\"", c.DocsDir)
	}
	for _, d := range c.Domains {
		if d == "" {
			return errors.New("domain name must not be empty")
		}
		if strings.ContainsAny(d, "/\\") || strings.Contains(d, "..") {
			return fmt.Errorf("domain %q must not contain path separators or \"..\"", d)
		}
	}
	if c.Invariants != nil {
		for _, src := range c.Invariants.Sources {
			if src.Marker == "" {
				return fmt.Errorf("invariants source %v has an empty marker; set a literal comment marker (e.g. \"//\", \"#\")", src.Globs)
			}
			for _, g := range src.Globs {
				if strings.Contains(g, "/") {
					return fmt.Errorf("invariants glob %q must be a filename pattern, not a path", g)
				}
				if _, err := filepath.Match(g, "x"); err != nil {
					return fmt.Errorf("invariants glob %q is malformed: %w", g, err)
				}
			}
		}
	}
	return nil
}

// checkSections rejects a section override that sets both drop and replaceWith.
// kind/name are used only for error formatting (name may be empty for a singleton).
func checkSections(kind, name string, sc Sidecar) error {
	label := kind
	if name != "" {
		label = fmt.Sprintf("%s %q", kind, name)
	}
	for sec, ov := range sc.Sections {
		if ov.Drop && ov.ReplaceWith != "" {
			return fmt.Errorf("%s section %q: cannot both drop and replaceWith", label, sec)
		}
	}
	return nil
}
