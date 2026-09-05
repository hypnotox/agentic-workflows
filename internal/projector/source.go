// Package projector loads AWF sources and projects the fixed generated surface.
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
	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

const (
	// SourceFormat is the only source shape accepted by this binary.
	SourceFormat = 1
	projectPath  = ".awf/project.md"
	topicsPath   = ".awf/topics"
)

// Project is the opaque project-specific guidance from .awf/project.md.
type Project struct {
	Body []byte
}

// Topic is one path-routed current-guidance source.
type Topic struct {
	ID         string
	SourcePath string
	Paths      []string
	Body       []byte
}

// SourceTree is the complete validated source input for one operation.
type SourceTree struct {
	Project Project
	Topics  []Topic
}

type projectMetadata struct {
	Format int `yaml:"format"`
}

type topicMetadata struct {
	Paths []string `yaml:"paths"`
}

// Load reads and validates the AWF source tree rooted at root.
func Load(root string) (SourceTree, error) {
	project, err := loadProject(root)
	if err != nil {
		return SourceTree{}, err
	}
	topics, err := loadTopics(root)
	if err != nil {
		return SourceTree{}, err
	}
	return SourceTree{Project: project, Topics: topics}, nil
}

func loadProject(root string) (Project, error) {
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(projectPath)))
	if err != nil {
		return Project{}, fmt.Errorf("read %s: %w", projectPath, err)
	}
	var metadata projectMetadata
	body, found, err := frontmatter.Parse(content, &metadata)
	if err != nil {
		return Project{}, fmt.Errorf("%s: %w", projectPath, err)
	}
	if !found {
		return Project{}, fmt.Errorf("%s: leading frontmatter is required", projectPath)
	}
	if metadata.Format != SourceFormat {
		return Project{}, fmt.Errorf("unsupported AWF source format %d; this binary accepts format %d", metadata.Format, SourceFormat)
	}
	return Project{Body: append([]byte(nil), body...)}, nil
}

func loadTopics(root string) ([]Topic, error) {
	rootPath := filepath.Join(root, filepath.FromSlash(topicsPath))
	if _, err := os.Stat(rootPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect %s: %w", topicsPath, err)
	}

	var topics []Topic
	err := filepath.WalkDir(rootPath, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
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
		if len(metadata.Paths) == 0 {
			return fmt.Errorf("%s: paths must contain at least one pattern", relative)
		}
		patterns := make([]string, len(metadata.Paths))
		for i, pattern := range metadata.Paths {
			patterns[i], err = normalizeTopicPattern(pattern)
			if err != nil {
				return fmt.Errorf("%s: %w", relative, err)
			}
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
