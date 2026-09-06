package projector

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pathglob"
)

// TopicMatch identifies one topic selected by Resolve.
type TopicMatch struct {
	ID         string
	SourcePath string
}

// PathCoverage reports the non-global topics matching one normalized path.
type PathCoverage struct {
	Path    string
	Matches []TopicMatch
}

// Coverage reports explicit globals once and specific routing per queried path.
type Coverage struct {
	Globals []TopicMatch
	Paths   []PathCoverage
}

// Resolve returns explicit global topics and every topic matching at least one
// lexical repository-relative path. The target paths do not need to exist.
func Resolve(root string, values []string) ([]TopicMatch, error) {
	sources, err := Load(root)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeResolvePaths(values)
	if err != nil {
		return nil, err
	}

	matches := make([]TopicMatch, 0)
	for _, topic := range sources.Topics {
		matched := topic.Global
		for _, value := range normalized {
			if pathglob.MatchAny(topic.Paths, value) {
				matched = true
				break
			}
		}
		if matched {
			matches = append(matches, topicMatch(topic))
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches, nil
}

// ResolveCoverage reports explicit globals and matching non-global topics for
// each distinct normalized lexical repository-relative path.
func ResolveCoverage(root string, values []string) (Coverage, error) {
	if len(values) == 0 {
		return Coverage{}, fmt.Errorf("coverage requires at least one path")
	}
	sources, err := Load(root)
	if err != nil {
		return Coverage{}, err
	}
	normalized, err := normalizeResolvePaths(values)
	if err != nil {
		return Coverage{}, err
	}
	sort.Strings(normalized)
	normalized = compactStrings(normalized)

	result := Coverage{Paths: make([]PathCoverage, 0, len(normalized))}
	for _, topic := range sources.Topics {
		if topic.Global {
			result.Globals = append(result.Globals, topicMatch(topic))
		}
	}
	for _, value := range normalized {
		entry := PathCoverage{Path: value}
		for _, topic := range sources.Topics {
			if !topic.Global && pathglob.MatchAny(topic.Paths, value) {
				entry.Matches = append(entry.Matches, topicMatch(topic))
			}
		}
		result.Paths = append(result.Paths, entry)
	}
	return result, nil
}

func normalizeResolvePaths(values []string) ([]string, error) {
	normalized := make([]string, len(values))
	for i, value := range values {
		var err error
		normalized[i], err = normalizeResolvePath(value)
		if err != nil {
			return nil, err
		}
	}
	return normalized, nil
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	write := 1
	for _, value := range values[1:] {
		if value != values[write-1] {
			values[write] = value
			write++
		}
	}
	return values[:write]
}

func topicMatch(topic Topic) TopicMatch {
	return TopicMatch{ID: topic.ID, SourcePath: topic.SourcePath}
}

func normalizeResolvePath(value string) (string, error) {
	value = strings.ReplaceAll(value, `\`, "/")
	if value == "" || path.IsAbs(value) || filepath.IsAbs(value) || hasWindowsVolume(value) {
		return "", fmt.Errorf("resolve path %q must be repository-relative", value)
	}
	cleaned := path.Clean(value)
	if cleaned == "." || escapesRoot(cleaned) {
		return "", fmt.Errorf("resolve path %q must name a path inside the repository", value)
	}
	return cleaned, nil
}
