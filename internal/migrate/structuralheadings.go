package migrate

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

const structuralHeadingsGeneration = 36

// structuralHeadingSnapshot is the cutover population, deliberately frozen rather
// than derived from templates. Later headings are body content to this migration.
var structuralHeadingSnapshot = []struct{ path, heading string }{
	{"docs/parts/architecture/dependencies.md", "## Key dependencies"},
	{"docs/parts/development/command-runner.md", "## Command runner"}, {"docs/parts/development/dependencies.md", "## Dependencies"}, {"docs/parts/development/setup.md", "## Setup"},
	{"docs/parts/pitfalls/prepend.md", "## Current pitfalls"}, {"docs/parts/releasing/content.md", "## Versioning"}, {"docs/parts/roadmap/deferred.md", "## Historical stale-branch seal-crossing incident"}, {"docs/parts/roadmap/ideas.md", "## Ideas"}, {"docs/parts/testing/gate.md", "## The gate"}, {"docs/parts/testing/tiers.md", "## Tiers"},
	{"domains/parts/adr-system/current-state.md", "## Current state"}, {"domains/parts/invariants/current-state.md", "## Current state"}, {"domains/parts/rendering/current-state.md", "## Current state"},
	{"parts/adr-readme/index.md", "## INDEX.md"}, {"parts/adr-template/body.md", "## Context"}, {"parts/adr-template/frontmatter.md", "# ADR-NNNN: Title"}, {"parts/agents-doc/identity.md", "## Identity"}, {"parts/agents-doc/working-memory.md", "## Working memory"}, {"parts/agents-doc/you-and-this-project.md", "## You and this project"}, {"parts/workflow/commit-discipline.md", "## Commit discipline"}, {"parts/workflow/composing-the-gate.md", "## Composing the gate"}, {"parts/workflow/local-hooks.md", "## Local git hooks"}, {"parts/working-with-awf/commands.md", "### Context spill notices"},
	{"skills/parts/retrospective/procedure.md", "## Procedure"},
}

type structuralHeadingEdit struct {
	path            string
	source, updated []byte
	mode            os.FileMode
}
type structuralHeadingWriter func(string, []byte, os.FileMode) error

// applyStructuralHeadings removes only the exact legacy structural line. It
// performs complete preflight before replacing any file, so ambiguity leaves the
// tree untouched; each replacement itself is atomic and a retry is a no-op.
func applyStructuralHeadings(root string, out io.Writer) error {
	return applyStructuralHeadingsWithWriter(root, out, manifest.WriteFileAtomicMode)
}

func applyStructuralHeadingsWithWriter(root string, out io.Writer, write structuralHeadingWriter) error {
	var edits []structuralHeadingEdit
	for _, entry := range structuralHeadingSnapshot {
		path := filepath.Join(root, config.DirName, filepath.FromSlash(entry.path))
		source, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil { // coverage-ignore: a path read immediately above can only fail here through an OS-specific directory or permission fault; migration callers surface it unchanged
			return err
		}
		start := leadingStructuralOffset(source)
		remaining := source[start:]
		lineEnd := bytes.IndexByte(remaining, '\n')
		line := remaining
		if lineEnd >= 0 {
			line = remaining[:lineEnd]
		}
		if bytes.Equal(line, []byte(entry.heading)) {
			// A second adjacent ATX heading is ambiguous: it could be a custom
			// heading intended as body content or a second legacy structure.
			after := remaining[len(line):]
			if len(after) > 0 {
				after = after[1:]
			}
			if firstLineIsATX(after) {
				return structuralHeadingRefusal(root, path, entry.heading)
			}
			info, err := os.Stat(path)
			if err != nil { // coverage-ignore: the file was read in this preflight and no concurrent filesystem mutation is representable by migration input
				return err
			}
			updated := append([]byte{}, source[:start]...)
			updated = append(updated, after...)
			edits = append(edits, structuralHeadingEdit{path, source, updated, info.Mode().Perm()})
			continue
		}
		if firstLineIsATX(remaining) {
			return structuralHeadingRefusal(root, path, entry.heading)
		}
	}
	for _, e := range edits {
		if err := write(e.path, e.updated, e.mode); err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, e.path)
		fmt.Fprintf(out, "structural-headings: updated %s\n", filepath.ToSlash(rel))
	}
	return nil
}

// leadingStructuralOffset skips only authoring comments, which are removed before
// part assembly and therefore cannot make a copied heading body content.
func leadingStructuralOffset(source []byte) int {
	offset := 0
	for {
		end := bytes.IndexByte(source[offset:], '\n')
		if end < 0 {
			return offset
		}
		end += offset
		line := strings.TrimSpace(string(source[offset:end]))
		if !strings.HasPrefix(line, "<!-- awf:comment ") || !strings.HasSuffix(line, " -->") {
			return offset
		}
		offset = end + 1
	}
}

func firstLineIsATX(source []byte) bool {
	if end := bytes.IndexByte(source, '\n'); end >= 0 {
		source = source[:end]
	}
	if len(source) < 2 || source[0] != '#' {
		return false
	}
	i := 0
	for i < len(source) && source[i] == '#' {
		i++
	}
	return i <= 6 && i < len(source) && (source[i] == ' ' || source[i] == '\t')
}

func structuralHeadingRefusal(root, path, heading string) error {
	rel, _ := filepath.Rel(root, path)
	rel = filepath.ToSlash(rel)
	return fmt.Errorf("operation: structural-heading migration refuses because %s must begin with the exact removable heading %q or unambiguously body content; changed bytes: no; changed index: no; changed message: no; changed merge state: no; next actions: 1. edit %s so its leading heading is %q or body content 2. run `awf upgrade`", rel, heading, rel, heading)
}
