// Package projector publishes fixed AWF content and resolves authored topics.
package projector

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
	"github.com/hypnotox/agentic-workflows/internal/knowledge"
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// TopicsPath is the repository-relative directory of canonical topic sources.
const TopicsPath = "docs/topics"

// Topic is one path-routed current-guidance source.
type Topic struct {
	ID         string
	SourcePath string
	Paths      []string
	Global     bool
	Body       []byte
}

type topicMetadata struct {
	Paths []string `yaml:"paths"`
}

// rejectLegacyLayout catches the known retired source locations before an
// operation could silently omit guidance. Conversion is manual, not a fallback.
func rejectLegacyLayout(root string) error {
	for _, legacy := range []string{".awf/project.md", ".awf/agents-doc.yaml", ".awf/parts/agents-doc", ".awf/topics"} {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(legacy))); err == nil {
			return fmt.Errorf("legacy AWF source at %s; preserve authored guidance in AGENTS.md and topics in %s, then retire the old source; run `awf docs integration` for manual migration", legacy, TopicsPath)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", legacy, err)
		}
	}
	return nil
}

// LoadTopics reads and validates topics directly, without project metadata or
// generated files. Known legacy layouts must be reconciled before use.
func LoadTopics(root string) ([]Topic, error) {
	if err := rejectLegacyLayout(root); err != nil {
		return nil, err
	}
	rootPath := filepath.Join(root, filepath.FromSlash(TopicsPath))
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect %s: %w", TopicsPath, err)
	}

	var topics []Topic
	err := filepath.WalkDir(rootPath, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") || knowledge.IsReserved(entry.Name()) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("topic source is not a regular file: %s", displayPath(root, filename))
		}
		content, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		var metadata topicMetadata
		body, found, err := frontmatter.Parse(content, &metadata)
		relative := displayPath(root, filename)
		if err != nil {
			return fmt.Errorf("%s: %w", relative, err)
		}
		if !found {
			return fmt.Errorf("%s: leading frontmatter is required", relative)
		}
		patterns, global, err := NormalizeTopicPatterns(metadata.Paths)
		if err != nil {
			return fmt.Errorf("%s: %w", relative, err)
		}
		topicRelative, err := filepath.Rel(rootPath, filename)
		if err != nil {
			return err
		}
		id := strings.TrimSuffix(filepath.ToSlash(topicRelative), filepath.Ext(topicRelative))
		topics = append(topics, Topic{
			ID:         id,
			SourcePath: relative,
			Paths:      patterns,
			Global:     global,
			Body:       append([]byte(nil), body...),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load topics: %w", err)
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].ID < topics[j].ID })
	return topics, nil
}

// NormalizeTopicPatterns validates topic selectors and returns their normalized
// matching forms plus whether they declare an explicit global topic.
func NormalizeTopicPatterns(patterns []string) ([]string, bool, error) {
	if len(patterns) == 0 {
		return nil, false, fmt.Errorf("paths must contain at least one pattern")
	}
	global := len(patterns) == 1 && patterns[0] == "**"
	normalized := make([]string, len(patterns))
	for i, pattern := range patterns {
		if pattern == "**" && len(patterns) != 1 {
			return nil, false, fmt.Errorf("standalone ** must be the only topic path pattern")
		}
		var err error
		normalized[i], err = normalizeTopicPattern(pattern)
		if err != nil {
			return nil, false, err
		}
	}
	return normalized, global, nil
}

func normalizeTopicPattern(pattern string) (string, error) {
	if pattern == "" {
		return "", fmt.Errorf("topic path pattern must not be empty")
	}

	normalized := strings.ReplaceAll(pattern, `\`, "/")
	if path.IsAbs(normalized) || hasWindowsVolume(normalized) || escapesRoot(normalized) {
		return "", fmt.Errorf("topic path pattern %q must be repository-relative", pattern)
	}
	if strings.HasPrefix(normalized, "!") {
		return "", fmt.Errorf("topic path pattern %q must be positive", pattern)
	}
	if strings.ContainsAny(normalized, "?[]{}") {
		return "", fmt.Errorf("topic path pattern %q may use only literal text, * and **", pattern)
	}

	normalized = path.Clean(normalized)
	for _, component := range strings.Split(normalized, "/") {
		if strings.Contains(component, "**") && component != "**" {
			return "", fmt.Errorf("topic path pattern %q must use ** as a complete path component", pattern)
		}
	}
	if err := pathglob.Validate(normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func escapesRoot(value string) bool {
	cleaned := path.Clean(value)
	return cleaned == ".." || strings.HasPrefix(cleaned, "../")
}

func hasWindowsVolume(value string) bool {
	return len(value) >= 2 && value[1] == ':' && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z'))
}

func displayPath(root, filename string) string {
	relative, err := filepath.Rel(root, filename)
	if err != nil {
		return filepath.ToSlash(filename)
	}
	return filepath.ToSlash(relative)
}
