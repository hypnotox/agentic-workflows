// Package adrfs creates author-owned decision record scaffolds.
package adrfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// New creates a decision record scaffold and returns its repository-relative path.
func New(root, slug string) (string, error) {
	if err := validateSlug(slug); err != nil {
		return "", err
	}

	relative := filepath.Join("docs", "decisions", slug+".md")
	filename := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return "", fmt.Errorf("create decisions directory: %w", err)
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("decision record %q already exists", slug)
		}
		return "", fmt.Errorf("create decision record %q: %w", slug, err)
	}
	if _, err := io.WriteString(file, starter(slug)); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write decision record %q: %w", slug, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close decision record %q: %w", slug, err)
	}
	return relative, nil
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("invalid decision slug: use letters, numbers, hyphens, or underscores")
	}
	for i, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || (i > 0 && (char == '-' || char == '_')) {
			continue
		}
		return fmt.Errorf("invalid decision slug %q: use letters, numbers, hyphens, or underscores", slug)
	}
	return nil
}

func starter(slug string) string {
	return "# Decision: " + slug + "\n\n" +
		"## Context and question\n\n" +
		"## Material alternatives\n\n" +
		"## Decision and rationale\n\n" +
		"## Consequences\n\n" +
		"## Affected topics\n"
}
