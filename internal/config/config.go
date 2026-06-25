// Package config loads and validates the per-project .claude/awf.yaml configuration.
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

type SkillConfig struct {
	Data     map[string]any             `yaml:"data"`
	Sections map[string]SectionOverride `yaml:"sections"`
	Local    bool                       `yaml:"local"`
}

type Config struct {
	Prefix     string                 `yaml:"prefix"`
	DocsDir    string                 `yaml:"docsDir"`
	Vars       map[string]any         `yaml:"vars"`
	Skills     map[string]SkillConfig `yaml:"skills"`
	Agents     map[string]SkillConfig `yaml:"agents"`
	Hooks      []string               `yaml:"hooks"`
	AgentsDoc  *SkillConfig           `yaml:"agentsDoc"`
	Docs       map[string]SkillConfig `yaml:"docs"`
	Invariants *InvariantConfig       `yaml:"invariants"`
	raw        []byte
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

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	c.raw = b
	if c.DocsDir == "" {
		c.DocsDir = "docs"
	}
	return &c, nil
}

func (c *Config) Raw() []byte { return c.raw }

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
	for name, sc := range c.Skills {
		if err := checkSections("skill", name, sc); err != nil {
			return err
		}
	}
	for name, ac := range c.Agents {
		if err := checkSections("agent", name, ac); err != nil {
			return err
		}
	}
	for name, dc := range c.Docs {
		if err := checkSections("doc", name, dc); err != nil {
			return err
		}
	}
	if c.AgentsDoc != nil {
		if err := checkSections("agentsDoc", "agentsDoc", *c.AgentsDoc); err != nil {
			return err
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
func checkSections(kind, name string, sc SkillConfig) error {
	for sec, ov := range sc.Sections {
		if ov.Drop && ov.ReplaceWith != "" {
			return fmt.Errorf("%s %q section %q: cannot both drop and replaceWith", kind, name, sec)
		}
	}
	return nil
}
