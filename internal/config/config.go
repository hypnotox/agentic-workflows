// Package config loads and validates the per-project .claude/awf.yaml configuration.
package config

import (
	"bytes"
	"fmt"
	"os"
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
	Prefix    string                 `yaml:"prefix"`
	Vars      map[string]any         `yaml:"vars"`
	Skills    map[string]SkillConfig `yaml:"skills"`
	Agents    map[string]SkillConfig `yaml:"agents"`
	Hooks     []string               `yaml:"hooks"`
	AgentsDoc *SkillConfig           `yaml:"agentsDoc"`
	raw       []byte
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
	return &c, nil
}

func (c *Config) Raw() []byte { return c.raw }

func (c *Config) Validate() error {
	if c.Prefix == "" {
		return fmt.Errorf("prefix must not be empty")
	}
	if strings.ContainsAny(c.Prefix, "/\\") || strings.Contains(c.Prefix, "..") {
		return fmt.Errorf("prefix %q must not contain path separators", c.Prefix)
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
	if c.AgentsDoc != nil {
		if err := checkSections("agentsDoc", "agentsDoc", *c.AgentsDoc); err != nil {
			return err
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
