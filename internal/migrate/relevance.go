package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"gopkg.in/yaml.v3"
)

const retireRelevanceMetadataName = "retire-relevance-metadata"

// ConfigBytesForGeneration projects a supported pre-migration config into the
// current schema for snapshot-pair validation. Upgrade remains the only writer.
func ConfigBytesForGeneration(generation int, source []byte) ([]byte, error) {
	if generation < LiveSchemaFloor || generation > pitfallRelationsGeneration {
		return nil, fmt.Errorf("schema %d has no supported config projection", generation)
	}
	updated := slices.Clone(source)
	var err error
	if generation < 47 {
		updated, _, err = removeTopLevelYAMLKeys(updated, "tags", "contextIgnore")
		if err != nil {
			return nil, err
		}
	}
	if generation < 49 {
		updated, _, err = retireWorkflowConfigBytes(updated)
		if err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func retireRelevanceMetadata(_ context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	var mutations []FileMutation
	configPath := config.DirName + "/config.yaml"
	body, mode, err := tree.Read(configPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", configPath, err)
	}
	updated, err := ConfigBytesForGeneration(LiveSchemaFloor, body)
	removed := []string{}
	if err == nil {
		_, removed, err = removeTopLevelYAMLKeys(body, "tags", "contextIgnore")
	}
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", configPath, err)
	}
	if len(removed) > 0 {
		mutations = append(mutations, FileMutation{Path: configPath, Content: updated, Mode: mode})
		for _, key := range removed {
			changes.Add("removed " + key + " from .awf/config.yaml")
		}
	}

	paths, err := tree.Paths(pitfall.SourceDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("enumerate authored pitfalls: %w", err)
	}
	for _, sourcePath := range paths {
		if path.Dir(sourcePath) != pitfall.SourceDir || path.Ext(sourcePath) != ".md" {
			continue
		}
		source, sourceMode, err := tree.Read(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", sourcePath, err)
		}
		updated, found, err := removePitfallTags(source)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", sourcePath, err)
		}
		if found {
			mutations = append(mutations, FileMutation{Path: sourcePath, Content: updated, Mode: sourceMode})
			changes.Add("removed tags from " + sourcePath)
		}
	}
	slices.SortFunc(mutations, func(a, b FileMutation) int { return strings.Compare(a.Path, b.Path) })
	return mutations, nil
}

func removePitfallTags(source []byte) ([]byte, bool, error) {
	lineEnd := []byte("\n")
	if bytes.HasPrefix(source, []byte("---\r\n")) {
		lineEnd = []byte("\r\n")
	}
	opening := append([]byte("---"), lineEnd...)
	if !bytes.HasPrefix(source, opening) {
		return nil, false, errors.New("missing frontmatter")
	}
	rest := source[len(opening):]
	closing := append([]byte("---"), lineEnd...)
	i := bytes.Index(rest, closing)
	if i < 0 {
		return nil, false, errors.New("unterminated frontmatter")
	}
	header := rest[:i]
	updated, removed, err := removeTopLevelYAMLKeys(header, "tags")
	if err != nil {
		return nil, false, err
	}
	if len(removed) == 0 {
		return source, false, nil
	}
	out := make([]byte, 0, len(source))
	out = append(out, opening...)
	out = append(out, updated...)
	out = append(out, rest[i:]...)
	return out, true, nil
}

// removeTopLevelYAMLKeys removes complete source-line spans selected by the
// parsed mapping. Flow and block values are both handled because the following
// top-level key, rather than the value's textual shape, bounds each span.
func removeTopLevelYAMLKeys(source []byte, keys ...string) ([]byte, []string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(source, &document); err != nil {
		return nil, nil, err
	}
	if len(document.Content) == 0 || document.Content[0].Kind != yaml.MappingNode {
		return nil, nil, errors.New("expected a top-level mapping")
	}
	mapping := document.Content[0]
	wanted := map[string]bool{}
	for _, key := range keys {
		wanted[key] = true
	}
	type span struct{ start, end int }
	var spans []span
	var removed []string
	for i := 0; i < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if !wanted[key.Value] {
			continue
		}
		start := key.Line - 1
		end := bytes.Count(source, []byte("\n")) + 1
		if i+2 < len(mapping.Content) {
			end = mapping.Content[i+2].Line - 1
		}
		spans = append(spans, span{start: start, end: end})
		removed = append(removed, key.Value)
	}
	if len(spans) == 0 {
		return slices.Clone(source), nil, nil
	}
	lines := bytes.SplitAfter(source, []byte("\n"))
	for i := len(spans) - 1; i >= 0; i-- {
		sp := spans[i]
		if sp.start < 0 || sp.start > len(lines) || sp.end < sp.start || sp.end > len(lines) {
			return nil, nil, errors.New("invalid YAML source span")
		}
		lines = append(lines[:sp.start], lines[sp.end:]...)
	}
	return bytes.Join(lines, nil), removed, nil
}
