package publisher

import "github.com/hypnotox/agentic-workflows/internal/generatedcheck"

type skillFrontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func validateFrontmatter(content []byte) error { return generatedcheck.ValidateFrontmatter(content) }
