// Package catalog loads the embedded catalog.yaml that declares the standard's skills, agents, and hooks.
package catalog

import (
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"
)

// TargetSpec declares the render sections of a target that has no further
// per-target configuration (agents and the always-on singletons).
type TargetSpec struct {
	Sections []string `yaml:"sections"`
}

// SkillSpec declares a skill's render sections plus its optional doc dependency:
// a non-empty RequiresDoc gates the skill on that doc being enabled.
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
	Skills      map[string]SkillSpec  `yaml:"skills"`
	Agents      map[string]TargetSpec `yaml:"agents"`
	Hooks       []string              `yaml:"hooks"`
	AgentsDoc   TargetSpec            `yaml:"agentsDoc"`
	DomainDoc   TargetSpec            `yaml:"domainDoc"`
	AdrReadme   TargetSpec            `yaml:"adrReadme"`
	AdrTemplate TargetSpec            `yaml:"adrTemplate"`
	PlansReadme TargetSpec            `yaml:"plansReadme"`
	Docs        map[string]DocSpec    `yaml:"docs"`
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
