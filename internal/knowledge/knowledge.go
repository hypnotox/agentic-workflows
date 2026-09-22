// Package knowledge checks the author-owned docs bundle against OKF v0.2 and
// AWF's description and decision-state requirements. It never changes files.
package knowledge

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// Finding is a structural problem at a repository-relative path.
type Finding struct {
	Path    string
	Message string
}

// IsReserved reports whether a Markdown basename is an OKF index or log.
// Match case-insensitively, as AWF already does for Markdown extensions.
func IsReserved(name string) bool {
	return strings.EqualFold(name, "index.md") || strings.EqualFold(name, "log.md")
}

// Check validates every Markdown file beneath docs, without Git, network access,
// link resolution, or optional metadata schema checks. Symlinks are not followed.
func Check(root string) ([]Finding, error) {
	findings := make([]Finding, 0)
	bundle := filepath.Join(root, "docs")
	info, err := os.Lstat(bundle)
	if os.IsNotExist(err) {
		return findings, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect docs: %w", err)
	}
	if !info.IsDir() {
		return append(findings, Finding{Path: "docs", Message: "knowledge bundle is not a directory"}), nil
	}
	err = filepath.WalkDir(bundle, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		relative, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		add := func(message string) {
			findings = append(findings, Finding{Path: relative, Message: message})
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			add("knowledge document is not a regular file")
			return nil
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		for _, message := range validate(relative, content) {
			add(message)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("check knowledge bundle: %w", err)
	}
	return findings, nil
}

func validate(relative string, content []byte) []string {
	if !utf8.Valid(content) {
		return []string{"OKF document must be valid UTF-8"}
	}
	var metadata map[string]any
	body, found, err := frontmatter.Parse(content, &metadata)
	if err != nil {
		return []string{err.Error()}
	}
	name := filepath.Base(relative)
	if IsReserved(name) {
		return validateReserved(relative, body, metadata, found)
	}
	if !found {
		return []string{"OKF concept requires leading YAML frontmatter"}
	}
	var messages []string
	if !nonemptyString(metadata["type"]) {
		messages = append(messages, "OKF type must be a non-empty string")
	}
	if !nonemptyString(metadata["description"]) {
		messages = append(messages, "AWF description must be a non-empty string (required beyond baseline OKF)")
	}
	if strings.HasPrefix(relative, "docs/decisions/") {
		decision, _ := metadata["decision_status"].(string)
		status, _ := metadata["status"].(string)
		want := ""
		switch decision {
		case "pending", "accepted":
			want = "draft"
		case "active":
			want = "stable"
		default:
			messages = append(messages, "ADR decision_status must be pending, accepted, or active; migrate legacy status manually without changing authority")
		}
		if status != "draft" && status != "stable" {
			messages = append(messages, "ADR status must be explicitly draft or stable")
		} else if want != "" && status != want {
			messages = append(messages, fmt.Sprintf("ADR decision_status %q requires status %q", decision, want))
		}
	}
	return messages
}

func nonemptyString(value any) bool {
	text, ok := value.(string)
	return ok && strings.TrimSpace(text) != ""
}
