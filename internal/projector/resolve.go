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

// Resolve returns every topic matching at least one lexical repository-relative
// path. The target paths do not need to exist.
func Resolve(root string, values []string) ([]TopicMatch, error) {
	sources, err := Load(root)
	if err != nil {
		return nil, err
	}
	normalized := make([]string, len(values))
	for i, value := range values {
		normalized[i], err = normalizeResolvePath(value)
		if err != nil {
			return nil, err
		}
	}

	matches := make([]TopicMatch, 0)
	for _, topic := range sources.Topics {
		matched := false
		for _, value := range normalized {
			if pathglob.MatchAny(topic.Paths, value) {
				matched = true
				break
			}
		}
		if matched {
			matches = append(matches, TopicMatch{ID: topic.ID, SourcePath: topic.SourcePath})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches, nil
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
