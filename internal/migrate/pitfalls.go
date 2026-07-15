package migrate

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// applyPitfallsData ports a pitfalls doc from the retired `entries` convention
// part to the ADR-0099 data.pitfalls sidecar: it splits the part on top-level
// `## ` headings outside fenced code into {title, body} entries, writes
// docs/pitfalls.yaml atomically, deletes the part (and its now-empty dir), and
// prints one provenance line per created entry plus a review instruction. An
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
	sidecarPath := filepath.Join(awfDir, "docs", "pitfalls.yaml")
	if err := manifest.WriteFileAtomic(sidecarPath, []byte(renderPitfallsSidecar(entries))); err != nil { // coverage-ignore: the atomic write faults only on a permission/IO error the test root bypasses
		return err
	}
	if err := os.Remove(partPath); err != nil { // coverage-ignore: removal of the part we just read; fails only on a permission fault root bypasses
		return err
	}
	_ = os.Remove(filepath.Dir(partPath)) // drop the now-empty dir; a non-empty dir is left as-is
	sidecarRel := path.Join(config.DirName, "docs", "pitfalls.yaml")
	if len(entries) == 0 {
		// No `## ` heading to split on: the part is removed but its prose is not
		// carried into the sidecar. Flag it rather than let the deletion be silent.
		fmt.Fprintf(out, "pitfalls-data: no `## ` headings found — wrote an empty %s and removed the part; its prior content is recoverable from git history\n", sidecarRel)
		return nil
	}
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

// renderPitfallsSidecar emits the data.pitfalls YAML: each title as a
// double-quoted scalar, each body as an 8-space-indented block scalar, domains
// and related left for the adopter to add. An empty entry set renders an empty
// list so the sidecar is still valid.
func renderPitfallsSidecar(entries []pitfallSplit) string {
	if len(entries) == 0 {
		return "data:\n  pitfalls: []\n"
	}
	var b strings.Builder
	b.WriteString("data:\n  pitfalls:\n")
	for _, e := range entries {
		esc := strings.ReplaceAll(e.title, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		fmt.Fprintf(&b, "    - title: \"%s\"\n      body: |\n", esc)
		for _, ln := range e.body {
			if strings.TrimSpace(ln) == "" {
				b.WriteString("\n")
			} else {
				b.WriteString("        " + ln + "\n")
			}
		}
	}
	return b.String()
}
