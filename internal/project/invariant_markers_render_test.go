package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
)

// invariant: invariant-markers-derived
//
// The adr-readme invariant-tagging guidance renders its comment-marker mapping
// from invariants.sources via the template default's .invariantMarkers key, and
// degrades to marker-agnostic prose when no sources are configured (ADR-0064).
func TestInvariantMarkersRenderedFromSources(t *testing.T) {
	// Multi-source (polyglot) config - awf's own dogfood is single-source, so the
	// multi-marker rendering must be covered explicitly (ADR-0064 Consequences).
	cfg := "prefix: example\nvars: {}\nskills: []\nagents: []\n" +
		"invariants:\n  sources:\n" +
		"    - {globs: [\"*.go\"], marker: \"//\"}\n" +
		"    - {globs: [\"*.py\"], marker: \"#\"}\n"
	root := scaffold(t, cfg)
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	readme := renderedByPath(t, files, "docs/decisions/README.md")
	if !strings.Contains(readme, "`*.go` → `//`") || !strings.Contains(readme, "`*.py` → `#`") {
		t.Errorf("README invariant guidance missing derived markers:\n%s", readme)
	}
	// No bare hardcoded marker literal survives from the template.
	if strings.Contains(readme, "`// invariant: <slug>`") {
		t.Errorf("README still carries a hardcoded `// invariant: <slug>` literal:\n%s", readme)
	}

	// No sources: the guidance degrades to marker-agnostic prose.
	bare := scaffold(t, "prefix: bare\nvars: {}\nskills: []\nagents: []\n")
	bp, err := Open(bare)
	if err != nil {
		t.Fatal(err)
	}
	bfiles, err := bp.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	bareReadme := renderedByPath(t, bfiles, "docs/decisions/README.md")
	if !strings.Contains(bareReadme, "your project's comment marker") {
		t.Errorf("no-sources README missing graceful fallback:\n%s", bareReadme)
	}
}

// renderedByPath returns the content of the RenderAll output at path, failing if absent.
func renderedByPath(t *testing.T, files []RenderedFile, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("no rendered file at %s", path)
	return ""
}

// invariant: invariant-markers-in-confighash (regression on both hash-fold paths)
//
// Editing invariants.sources reflags an artifact that references the marker
// mapping - via the template default's .invariantMarkers key or an override
// part's {{=awf:invariantMarker*}} placeholder - stale in Check (ADR-0064).
func TestInvariantMarkersEditReflags(t *testing.T) {
	cfg := func(marker string) string {
		return "prefix: example\nvars: {}\nskills: []\nagents: []\n" +
			"invariants:\n  sources:\n    - {globs: [\"*.go\"], marker: \"" + marker + "\"}\n"
	}

	// (a) Template-reference path: default adr-readme references .invariantMarkers.
	tmplRoot := scaffold(t, cfg("//"))
	syncClean(t, tmplRoot)
	testsupport.WriteAwfConfig(t, tmplRoot, cfg("##")) // marker edit
	if !reflaggedStale(t, tmplRoot, "docs/decisions/README.md") {
		t.Error("template-path: sources edit did not reflag docs/decisions/README.md")
	}

	// (b) Part-placeholder path: an override part splices the sentence placeholder.
	partRoot := scaffoldFiles(t, cfg("//"), map[string]string{
		"parts/adr-readme/invariants.md": "## Invariant tagging\n\n{{=awf:invariantMarkerSentence}}\n",
	})
	syncClean(t, partRoot)
	testsupport.WriteAwfConfig(t, partRoot, cfg("##"))
	if !reflaggedStale(t, partRoot, "docs/decisions/README.md") {
		t.Error("part-path: sources edit did not reflag docs/decisions/README.md")
	}
}

// syncClean opens+syncs root and fails on any residual drift.
func syncClean(t *testing.T, root string) {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Sync(); err != nil {
		t.Fatal(err)
	}
}

// reflaggedStale re-opens root and reports whether path is flagged stale in Check.
func reflaggedStale(t *testing.T, root, path string) bool {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	drift, err := p.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range drift {
		if d.Path == path && d.Kind == "stale" {
			return true
		}
	}
	return false
}
