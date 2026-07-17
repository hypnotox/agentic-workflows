package migrate

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"gopkg.in/yaml.v3"
)

// applyPitfallsData ports a pitfalls doc from the retired `entries` convention
// part to the ADR-0099 data.pitfalls sidecar: it splits the part on top-level
// `## ` headings outside fenced code into {title, body} entries, validates and
// writes docs/pitfalls.yaml atomically, then deletes the part (and its now-empty
// dir), and prints one provenance line per created entry plus a review instruction. An
// absent part is a no-op - so a re-run after a prior split (the part gone, the
// sidecar present) does nothing.
func applyPitfallsData(root string, out io.Writer) error {
	awfDir := config.RootDir(root)
	partPath := filepath.Join(awfDir, "docs", "parts", "pitfalls", "entries.md")
	data, err := os.ReadFile(partPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil { // coverage-ignore: ReadFile faults only on a permission error the test root bypasses; the absent-part no-op is the tested branch
		return err
	}
	entries := splitPitfallEntries(string(data))
	if len(entries) == 0 {
		return errors.New("pitfalls-data: no top-level entries to migrate")
	}
	sidecar, err := renderPitfalls(entries)
	if err != nil {
		return err
	}
	if err := validatePitfallsSidecar(sidecar, entries); err != nil {
		return err
	}
	sidecarPath := filepath.Join(awfDir, "docs", "pitfalls.yaml")
	if err := manifest.WriteFileAtomic(sidecarPath, sidecar); err != nil { // coverage-ignore: the atomic write faults only on a permission/IO error the test root bypasses
		return err
	}
	if err := os.Remove(partPath); err != nil { // coverage-ignore: removal of the part we just read; fails only on a permission fault root bypasses
		return err
	}
	_ = os.Remove(filepath.Dir(partPath)) // drop the now-empty dir; a non-empty dir is left as-is
	sidecarRel := path.Join(config.DirName, "docs", "pitfalls.yaml")
	for _, e := range entries {
		fmt.Fprintf(out, "pitfalls-data: split entry %q\n", e.title)
	}
	fmt.Fprintf(out, "pitfalls-data: review %s and tag each entry's domains: (untagged entries do not surface in awf context)\n", sidecarRel)
	return nil
}

// pitfallSplit is one migrated entry: a heading title and its verbatim body.
type pitfallSplit struct {
	title string
	body  []string
}

// splitPitfallEntries divides the part into entries at each top-level `## `
// heading outside a fenced code block. A ``` line toggles fenced state so a `## `
// inside a code fence is not mis-split. Leading/trailing blank body lines are
// trimmed.
func splitPitfallEntries(part string) []pitfallSplit {
	var entries []pitfallSplit
	fenced := false
	for _, line := range strings.Split(part, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
		}
		if !fenced && strings.HasPrefix(line, "## ") {
			entries = append(entries, pitfallSplit{title: strings.TrimPrefix(line, "## ")})
			continue
		}
		if len(entries) > 0 {
			e := &entries[len(entries)-1]
			e.body = append(e.body, line)
		}
	}
	for i := range entries {
		entries[i].body = trimBlankEdges(entries[i].body)
	}
	return entries
}

// trimBlankEdges drops leading and trailing blank lines.
func trimBlankEdges(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type pitfallsSidecar struct {
	Data struct {
		Pitfalls []pitfallSidecarEntry `yaml:"pitfalls"`
	} `yaml:"data"`
}

type pitfallSidecarEntry struct {
	Title string      `yaml:"title"`
	Body  pitfallBody `yaml:"body"`
}

type pitfallBody string

// MarshalYAML uses a quoted scalar so leading Markdown indentation cannot
// affect the YAML block indentation chosen by yaml.v3.
func (b pitfallBody) MarshalYAML() (interface{}, error) {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: string(b), Style: yaml.DoubleQuotedStyle}, nil
}

var renderPitfalls = renderPitfallsSidecar

// renderPitfallsSidecar serializes the data.pitfalls YAML and decodes it again
// before destructive cleanup can proceed.
func renderPitfallsSidecar(entries []pitfallSplit) ([]byte, error) {
	var sidecar pitfallsSidecar
	for _, entry := range entries {
		sidecar.Data.Pitfalls = append(sidecar.Data.Pitfalls, pitfallSidecarEntry{
			Title: entry.title,
			Body:  pitfallBody(strings.Join(entry.body, "\n")),
		})
	}
	b, err := yaml.Marshal(sidecar)
	if err != nil { // coverage-ignore: the typed sidecar contains only strings and yaml.Marshal cannot reject it
		return nil, err
	}
	if err := validatePitfallsSidecar(b, entries); err != nil { // coverage-ignore: yaml.Marshal of the validated typed model cannot emit a different or invalid model
		return nil, err
	}
	return b, nil
}

func validatePitfallsSidecar(b []byte, entries []pitfallSplit) error {
	var decoded pitfallsSidecar
	if err := yaml.Unmarshal(b, &decoded); err != nil {
		return fmt.Errorf("parse rendered pitfalls sidecar: %w", err)
	}
	if len(decoded.Data.Pitfalls) != len(entries) {
		return fmt.Errorf("validate rendered pitfalls sidecar: got %d entries, want %d", len(decoded.Data.Pitfalls), len(entries))
	}
	for i, entry := range entries {
		if decoded.Data.Pitfalls[i].Title != entry.title || string(decoded.Data.Pitfalls[i].Body) != strings.Join(entry.body, "\n") {
			return fmt.Errorf("validate rendered pitfalls sidecar: entry %d differs", i)
		}
	}
	return nil
}
