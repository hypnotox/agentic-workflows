// Package artifactfs creates author-owned change documents, ADRs, and topics.
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

// NewIntent creates a tracked change intent scaffold.
func NewIntent(root, slug string) (string, error) {
	return newChangeDocument(root, slug, "intent", intentStarter(slug))
}

// NewSpec creates a tracked change specification scaffold.
func NewSpec(root, slug string) (string, error) {
	return newChangeDocument(root, slug, "spec", specStarter(slug))
}

// NewPlan creates a tracked implementation plan scaffold.
func NewPlan(root, slug string) (string, error) {
	if err := validateSlug("plan", slug); err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "plans", slug+".md")
	return create(root, relative, "plan", slug, planStarter(slug))
}

func newChangeDocument(root, slug, kind, body string) (string, error) {
	if err := validateSlug(kind, slug); err != nil {
		return "", err
	}
	relative := filepath.Join("docs", "changes", slug, kind+".md")
	return create(root, relative, kind, slug, body)
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

	relative := filepath.Join(filepath.FromSlash(projector.TopicsPath), filepath.FromSlash(id)+".md")
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

func intentStarter(slug string) string {
	return "# Intent: " + slug + "\n\n" +
		"Adapt or omit sections. Remove prompts and content that does not help this document serve its purpose.\n\n" +
		"## Problem\n\nExplain the problem and why it matters.\n\n" +
		"## Desired outcome\n\nState the result we want and what success looks like.\n\n" +
		"## Scope and constraints\n\nDefine scope, non-goals, and actual constraints. Distinguish requirements from proposed mechanisms.\n\n" +
		"## Open questions\n\nKeep material unknowns and proposals separate from agreed requirements.\n"
}

func specStarter(slug string) string {
	return "# Specification: " + slug + "\n\n" +
		"Adapt or omit sections. Remove prompts and content that does not help this document serve its purpose.\n\n" +
		"## Basis\n\nReference the intent or existing requirements and applicable ADRs.\n\n" +
		"## Behavior and design\n\nDescribe the agreed behavior, interactions, and important design boundaries needed to plan the change. Omit incidental implementation details.\n\n" +
		"## Acceptance criteria\n\nAdd observable conditions and examples that make success precise without repeating the intent.\n\n" +
		"## Open questions\n\nIdentify material choices still unresolved; do not present proposals as agreements.\n"
}

func planStarter(slug string) string {
	return "# Plan: " + slug + "\n\n" +
		"Adapt or omit sections. Remove prompts and content that does not help this document serve its purpose.\n\n" +
		"## Basis\n\nReference the intent, specification, or other agreed outcome and criteria, plus applicable ADRs. State the basis briefly when no separate document is needed.\n\n" +
		"## Implementation approach\n\nDescribe important ownership boundaries, dependencies, and settled design choices without copying ADR rationale.\n\n" +
		"## Work sequence\n\nDescribe coherent changes in dependency order. Include concrete locations or mechanics only when they preserve an important decision or materially clarify the route.\n\n" +
		"## Verification\n\nName proportionate checks against the agreed outcome and acceptance criteria, including the combined result.\n"
}

func adrStarter(slug string) string {
	return "---\nstatus: pending\n---\n\n# Decision: " + slug + "\n\n" +
		"Adapt or omit sections. Remove prompts and content that does not help this document serve its purpose.\n\n" +
		"## Context\n\nExplain the problem, relevant constraints, and evidence that makes this choice necessary. Reference the originating intent or specification when applicable.\n\n" +
		"## Decision and rationale\n\nState the consequential choice, its scope, and rationale worth retaining after the originating change is complete. Distinguish a proposal from an established agreement.\n\n" +
		"## Consequences\n\nCapture meaningful benefits, costs, limitations, and trade-offs.\n\n" +
		"## Related decisions\n\nLink relevant authority and explain intended supersession, including where retained commitments and rationale will live. Omit unrelated links and topic inventories.\n\n" +
		"## Material alternatives\n\nRecord the credible alternatives actually considered; a second option is not required.\n"
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
	body.WriteString("\n\nAdapt or omit sections. Remove prompts and content that does not help this document serve its purpose.\n\n")
	body.WriteString("State the focused purpose of this topic.\n\n")
	body.WriteString("## Current behavior and structure\n\nExplain current behavior, ownership boundaries, and relationships that matter to future changes, not an inventory of files and functions.\n\n")
	body.WriteString("## Constraints and practical implications\n\nExplain current constraints and what they mean for changes in this area. Keep useful local explanations and link active ADRs rather than repeating their full rationale.\n")
	return body.String()
}
