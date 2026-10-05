package projector

import (
	"fmt"
	"path"
	"path/filepath"
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

// Resolve reports explicit globals and non-global topics for every supplied
// lexical repository-relative path, in argument order, including duplicates.
// The target paths do not need to exist.
func Resolve(root string, values []string) (Coverage, error) {
	topics, err := LoadTopics(root)
	if err != nil {
		return Coverage{}, err
	}
	result := Coverage{Paths: make([]PathCoverage, len(values))}
	for i, value := range values {
		normalized, err := normalizeResolvePath(value)
		if err != nil {
			return Coverage{}, err
		}
		result.Paths[i].Path = normalized
	}
	for _, topic := range topics {
		if topic.Global {
			result.Globals = append(result.Globals, topicMatch(topic))
		}
	}
	for i := range result.Paths {
		entry := &result.Paths[i]
		for _, topic := range topics {
			if !topic.Global && pathglob.MatchAny(topic.Paths, entry.Path) {
				entry.Matches = append(entry.Matches, topicMatch(topic))
			}
		}
	}
	return result, nil
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
