// Package catalog loads the embedded catalog.yaml that declares the standard's skills, agents, and hooks.
package catalog

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

type SkillSpec struct {
	Sections    []string `yaml:"sections"`
	RequiresDoc string   `yaml:"requiresDoc"`
}

type DocSpec struct {
	Title    string   `yaml:"title"`
	Desc     string   `yaml:"desc"`
	Sections []string `yaml:"sections"`
}

type Catalog struct {
	Skills      map[string]SkillSpec `yaml:"skills"`
	Agents      map[string]SkillSpec `yaml:"agents"`
	Hooks       []string             `yaml:"hooks"`
	AgentsDoc   SkillSpec            `yaml:"agentsDoc"`
	DomainDoc   SkillSpec            `yaml:"domainDoc"`
	AdrReadme   SkillSpec            `yaml:"adrReadme"`
	AdrTemplate SkillSpec            `yaml:"adrTemplate"`
	PlansReadme SkillSpec            `yaml:"plansReadme"`
	Docs        map[string]DocSpec   `yaml:"docs"`
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
