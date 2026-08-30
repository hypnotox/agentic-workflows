package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/pitfall"
	"gopkg.in/yaml.v3"
)

const (
	retirePitfallRelationsName = "replace-pitfall-related-metadata-with-links"
	pitfallRelationsGeneration = 50
	decisionDir                = "docs/decisions"
)

var (
	decisionIdentityPrefix = regexp.MustCompile(`^([0-9]{4})-`)
	numericIdentity        = regexp.MustCompile(`^[0-9]+$`)
)

// retirePitfallRelations replaces the retired numeric relation metadata with
// authored links. It deliberately resolves only direct decision leaves: the
// historical decision archive is a lookup table, not a mutation target.
func retirePitfallRelations(_ context.Context, tree *ProposedTree, changes *Changes) ([]FileMutation, error) {
	paths, err := tree.Paths(pitfall.SourceDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("enumerate authored pitfalls: %w", err)
	}
	decisions, err := decisionTargets(tree)
	if err != nil {
		return nil, err
	}
	var mutations []FileMutation
	for _, sourcePath := range paths {
		if path.Dir(sourcePath) != pitfall.SourceDir || path.Ext(sourcePath) != ".md" {
			continue
		}
		source, mode, err := tree.Read(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", sourcePath, err)
		}
		updated, changed, err := replacePitfallRelations(source, decisions)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", sourcePath, err)
		}
		if changed {
			mutations = append(mutations, FileMutation{Path: sourcePath, Content: updated, Mode: mode})
			changes.Add("replaced related metadata with decision links in " + sourcePath)
		}
	}
	return mutations, nil
}

// decisionTargets returns canonical direct decision leaves keyed by their
// numeric identity. README, INDEX, templates, nested paths, and every other
// filename are intentionally outside this frozen migration lookup.
func decisionTargets(tree *ProposedTree) (map[int]string, error) {
	// Paths deliberately returns only regular files. Inspect matching entries
	// first so a symlink or special-file lookalike cannot silently become a
	// missing relation target.
	if err := tree.files.Walk(decisionDir, func(candidate string, info fs.FileInfo) (bool, error) {
		if candidate == decisionDir && info.Mode()&fs.ModeSymlink != 0 {
			return false, errors.New("unsafe decision archive path")
		}
		if path.Dir(candidate) == decisionDir && path.Ext(candidate) == ".md" && decisionIdentityPrefix.MatchString(path.Base(candidate)) && !info.Mode().IsRegular() {
			return false, fmt.Errorf("unsafe decision target %s", candidate)
		}
		return info.IsDir(), nil
	}); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect decision archive: %w", err)
	}
	paths, err := tree.Paths(decisionDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[int]string{}, nil
		}
		return nil, fmt.Errorf("enumerate decision archive: %w", err)
	}
	return decisionTargetsFromPaths(paths)
}

func decisionTargetsFromPaths(paths []string) (map[int]string, error) {
	targets := map[int]string{}
	for _, candidate := range paths {
		if path.Clean(candidate) != candidate || strings.HasPrefix(candidate, "/") || path.Dir(candidate) != decisionDir || path.Ext(candidate) != ".md" {
			continue
		}
		match := decisionIdentityPrefix.FindStringSubmatch(path.Base(candidate))
		if match == nil {
			continue
		}
		identity, err := strconv.Atoi(match[1])
		if err != nil || identity == 0 {
			return nil, fmt.Errorf("unsafe decision identity in %s", candidate)
		}
		if prior, exists := targets[identity]; exists {
			return nil, fmt.Errorf("ambiguous decision identity %04d: %s and %s", identity, prior, candidate)
		}
		targets[identity] = candidate
	}
	return targets, nil
}

// PitfallBytesForGeneration refuses pre-schema-50 projections because every
// such source may still contain numeric relations. Bytes alone cannot resolve
// those identities to decision filenames without retaining or inventing data.
func PitfallBytesForGeneration(generation int, source []byte) ([]byte, error) {
	if generation >= pitfallRelationsGeneration {
		return slices.Clone(source), nil
	}
	if generation < LiveSchemaFloor || generation > workflowConfigGeneration {
		return nil, fmt.Errorf("schema %d has no supported pitfall projection", generation)
	}
	return nil, fmt.Errorf("schema %d pitfall projection requires decision-tree context", generation)
}

// PitfallBytesForGenerationInTree projects a named source through every
// applicable pitfall migration. It is the tree-aware projection seam required
// to resolve schema-49 numeric relations without pretending bytes contain
// decision filenames.
func PitfallBytesForGenerationInTree(generation int, sourcePath string, tree *ProposedTree) ([]byte, error) {
	source, _, err := tree.Read(sourcePath)
	if err != nil {
		return nil, err
	}
	paths, err := tree.Paths(decisionDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return PitfallBytesForGenerationWithDecisionPaths(generation, sourcePath, source, paths)
}

// PitfallBytesForGenerationWithDecisionPaths is the portable tree-aware
// projection seam. Callers supply the direct decision paths from their own
// read-only tree, preserving the migration's authoritative lookup semantics.
func PitfallBytesForGenerationWithDecisionPaths(generation int, sourcePath string, source []byte, decisionPaths []string) ([]byte, error) {
	if generation < LiveSchemaFloor || generation > pitfallRelationsGeneration {
		return nil, fmt.Errorf("schema %d has no supported pitfall projection", generation)
	}
	if path.Dir(sourcePath) != pitfall.SourceDir || path.Ext(sourcePath) != ".md" {
		return nil, fmt.Errorf("invalid authored pitfall path %s", sourcePath)
	}
	updated := slices.Clone(source)
	var err error
	if generation < 47 {
		updated, _, err = removePitfallTags(updated)
		if err != nil {
			return nil, err
		}
	}
	if generation < pitfallRelationsGeneration {
		decisions, err := decisionTargetsFromPaths(decisionPaths)
		if err != nil {
			return nil, err
		}
		updated, _, err = replacePitfallRelations(updated, decisions)
		if err != nil {
			return nil, err
		}
	}
	return updated, nil
}

func replacePitfallRelations(source []byte, decisions map[int]string) ([]byte, bool, error) {
	lines := splitSourceLines(source)
	if len(lines) == 0 || lines[0].text != "---" || (string(lines[0].raw) != "---\n" && string(lines[0].raw) != "---\r\n") {
		return nil, false, errors.New("missing strict frontmatter")
	}
	lineEnd := "\n"
	if string(lines[0].raw) == "---\r\n" {
		lineEnd = "\r\n"
	}
	close := -1
	for i := 1; i < len(lines); i++ {
		if lines[i].text == "---" {
			close = i
			break
		}
	}
	if close < 0 {
		return nil, false, errors.New("unterminated frontmatter")
	}
	header := joinRaw(lines[1:close])
	var document yaml.Node
	if err := yaml.Unmarshal(header, &document); err != nil {
		return nil, false, err
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		return nil, false, errors.New("frontmatter must be a mapping")
	}
	mapping := document.Content[0]
	var relatedKey, relatedValue *yaml.Node
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if key.Value != "related" {
			continue
		}
		if relatedKey != nil {
			return nil, false, errors.New("duplicate related metadata")
		}
		relatedKey, relatedValue = key, value
	}
	if relatedKey == nil {
		return slices.Clone(source), false, nil
	}
	identities, err := parseRelatedIdentities(relatedValue)
	if err != nil {
		return nil, false, err
	}
	targets := make([]string, len(identities))
	for i, identity := range identities {
		target, found := decisions[identity]
		if !found {
			return nil, false, fmt.Errorf("missing decision target %04d", identity)
		}
		targets[i] = target
	}
	paragraph := relatedLinksParagraph(identities, targets)
	body := joinRaw(lines[close+1:])
	if bytes.Contains(body, []byte(paragraph)) {
		updated, err := removeRelatedLines(lines, relatedKey, mapping, close)
		return updated, true, err
	}
	updated, err := removeRelatedLines(lines, relatedKey, mapping, close)
	if err != nil {
		return nil, false, err
	}
	hadFinalNewline := bytes.HasSuffix(source, []byte(lineEnd))
	if !bytes.HasSuffix(updated, []byte(lineEnd)) {
		updated = append(updated, []byte(lineEnd)...)
	}
	updated = append(updated, []byte(lineEnd)...)
	updated = append(updated, []byte(paragraph)...)
	if hadFinalNewline {
		updated = append(updated, []byte(lineEnd)...)
	}
	return updated, true, nil
}

func parseRelatedIdentities(value *yaml.Node) ([]int, error) {
	if value.Kind != yaml.SequenceNode || len(value.Content) == 0 {
		return nil, errors.New("related metadata must be a non-empty numeric list")
	}
	seen := map[int]bool{}
	identities := make([]int, 0, len(value.Content))
	for _, item := range value.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!int" || !numericIdentity.MatchString(item.Value) {
			return nil, errors.New("related metadata contains a nonnumeric identity")
		}
		identity, err := strconv.Atoi(item.Value)
		if err != nil || identity < 1 || identity > 9999 {
			return nil, errors.New("related metadata contains an out-of-range identity")
		}
		if seen[identity] {
			return nil, fmt.Errorf("duplicate related identity %04d", identity)
		}
		seen[identity] = true
		identities = append(identities, identity)
	}
	return identities, nil
}

func relatedLinksParagraph(identities []int, targets []string) string {
	links := make([]string, len(identities))
	for i, identity := range identities {
		links[i] = fmt.Sprintf("[ADR-%04d](../decisions/%s)", identity, path.Base(targets[i]))
	}
	return "Related decisions: " + strings.Join(links, ", ")
}

func removeRelatedLines(lines []sourceLine, key *yaml.Node, mapping *yaml.Node, closing int) ([]byte, error) {
	start := key.Line // header line number plus opening line, zero-indexed overall
	end := closing
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i] == key {
			if i+2 < len(mapping.Content) {
				end = mapping.Content[i+2].Line
			}
			break
		}
	}
	if start < 1 || start > end || end > closing {
		return nil, errors.New("invalid related metadata source span")
	}
	out := make([]byte, 0)
	for i, line := range lines {
		if i < start || i >= end {
			out = append(out, line.raw...)
		}
	}
	return out, nil
}

func joinRaw(lines []sourceLine) []byte {
	var out []byte
	for _, line := range lines {
		out = append(out, line.raw...)
	}
	return out
}
