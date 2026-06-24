// Package catalog loads the embedded catalog.yaml that declares the standard's skills, agents, and hooks.
package catalog

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

type SkillSpec struct {
	Sections []string `yaml:"sections"`
}

type Catalog struct {
	Skills map[string]SkillSpec `yaml:"skills"`
	Agents map[string]SkillSpec `yaml:"agents"`
	Hooks  []string             `yaml:"hooks"`
}

func Load(fsys fs.FS) (*Catalog, error) {
	b, err := fs.ReadFile(fsys, "catalog.yaml")
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var c Catalog
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	return &c, nil
}
