package publisher

import (
	"errors"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func validateFrontmatter(content []byte) error {
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

func validateArtifact(content []byte, _ AgentDialect) error { return validateFrontmatter(content) }
