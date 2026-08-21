package project

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

func validateCommandWiring(cfg *config.Config) error {
	value, _ := cfg.Vars["gateCmd"].(string)
	if strings.TrimSpace(value) == "" {
		return errors.New("rendered hook payloads require vars.gateCmd: set it in .awf/config.yaml")
	}
	return nil
}

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func validateArtifact(content []byte, _ AgentDialect) error {
	var fm skillFrontmatter
	_, found, err := frontmatter.Parse(content, &fm)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("missing frontmatter")
	}
	if strings.TrimSpace(fm.Name) == "" {
		return errors.New("frontmatter name is empty")
	}
	if strings.TrimSpace(fm.Description) == "" {
		return errors.New("frontmatter description is empty")
	}
	return nil
}
