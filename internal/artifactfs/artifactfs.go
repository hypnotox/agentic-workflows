// Package artifactfs creates author-owned plan, decision, and topic scaffolds.
package artifactfs

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/projector"
)

// NewPlan creates a tracked implementation plan scaffold.
func NewPlan(root, slug string) (string, error) {
	if err := validateSlug("plan", slug); err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "plans", slug+".md")
	return create(root, relative, "plan", slug, planStarter(slug))
}

// NewADR creates a tracked architecture decision record scaffold.
func NewADR(root, slug string) (string, error) {
	if err := validateSlug("decision", slug); err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "decisions", slug+".md")
	return create(root, relative, "decision record", slug, adrStarter(slug))
}

// NewTopic creates a path-routed topic source with the supplied selectors.
func NewTopic(root, id string, selectors []string) (string, error) {
	if err := validateTopicID(id); err != nil {
		return "", err
	}
	if _, _, err := projector.NormalizeTopicPatterns(selectors); err != nil {
		return "", err
	}

	relative := filepath.Join(".awf", "topics", filepath.FromSlash(id)+".md")
	return create(root, relative, "topic", id, topicStarter(id, selectors))
}

func create(root, relative, kind, name, body string) (string, error) {
	filename := filepath.Join(root, relative)
	if err := ensureDirectory(root, filepath.Dir(relative)); err != nil {
		return "", fmt.Errorf("create %s directory: %w", kind, err)
	}
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s %q already exists", kind, name)
		}
		return "", fmt.Errorf("create %s %q: %w", kind, name, err)
	}
	if _, err := io.WriteString(file, body); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("write %s %q: %w", kind, name, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close %s %q: %w", kind, name, err)
	}
	return relative, nil
}

func ensureDirectory(root, relative string) error {
	current := root
	for _, component := range strings.Split(filepath.Clean(relative), string(filepath.Separator)) {
		if component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			if mkdirErr := os.Mkdir(current, 0o755); mkdirErr == nil {
				continue
			} else if !os.IsExist(mkdirErr) {
				return fmt.Errorf("create %s: %w", filepath.ToSlash(relative), mkdirErr)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return fmt.Errorf("inspect %s: %w", filepath.ToSlash(relative), err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", filepath.ToSlash(relative))
		}
	}
	return nil
}

func validateSlug(kind, slug string) error {
	if slug == "" {
		return fmt.Errorf("invalid %s slug: use letters, numbers, hyphens, or underscores", kind)
	}
	for i, char := range slug {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || (i > 0 && (char == '-' || char == '_')) {
			continue
		}
		return fmt.Errorf("invalid %s slug %q: use letters, numbers, hyphens, or underscores", kind, slug)
	}
	return nil
}

func validateTopicID(id string) error {
	if id == "" || strings.HasSuffix(strings.ToLower(id), ".md") || strings.Contains(id, `\`) || path.IsAbs(id) || path.Clean(id) != id {
		return fmt.Errorf("invalid topic ID %q: use slash-separated letters, numbers, hyphens, or underscores without .md", id)
	}
	for _, component := range strings.Split(id, "/") {
		if err := validateSlug("topic ID", component); err != nil {
			return fmt.Errorf("invalid topic ID %q: use slash-separated letters, numbers, hyphens, or underscores without .md", id)
		}
	}
	return nil
}

func planStarter(slug string) string {
	return "# Plan: " + slug + "\n\n" +
		"## Outcome and success criteria\n\nState the intended result, scope boundaries, and observable completion criteria.\n\n" +
		"## Implementation approach\n\nDescribe ownership, important dependencies, and any necessary enabling refactor without copying ADR rationale.\n\n" +
		"## Work sequence\n\nList verifiable units, their locations and purpose, and ordering dependencies.\n\n" +
		"## Verification\n\nName focused checks and how to assess the combined result against the outcome.\n"
}

func adrStarter(slug string) string {
	return "---\nstatus: pending\n---\n\n# Decision: " + slug + "\n\n" +
		"## Context and question\n\nExplain the problem, relevant constraints, and evidence that makes this choice necessary.\n\n" +
		"## Material alternatives\n\nRecord the credible alternatives actually considered; a second option is not required.\n\n" +
		"## Decision and rationale\n\nState the accepted choice and rationale, or clearly identify the question as open.\n\n" +
		"## Consequences\n\nCapture meaningful benefits, costs, limitations, and trade-offs.\n\n" +
		"## Affected topics and decisions\n\nLink relevant topics and prior decisions, explaining intended supersession where applicable.\n"
}

func topicStarter(id string, selectors []string) string {
	var body strings.Builder
	body.WriteString("---\npaths:\n")
	for _, selector := range selectors {
		body.WriteString("  - ")
		body.WriteString(strconv.Quote(selector))
		body.WriteByte('\n')
	}
	body.WriteString("---\n\n# ")
	body.WriteString(id)
	body.WriteString("\n\nState the focused purpose of this topic.\n\n")
	body.WriteString("## Current behavior and structure\n\nExplain implemented behavior and ownership a future change needs to understand.\n\n")
	body.WriteString("## Constraints and rationale\n\nRecord current constraints and practical implications, linking active decisions where useful.\n\n")
	body.WriteString("## Working in this area\n\nRecord useful change and verification guidance and non-obvious lessons.\n")
	return body.String()
}
