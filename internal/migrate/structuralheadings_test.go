package migrate

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
)

// invariant: config/migrations-and-locks:structural-heading-part-migration (TestStructuralHeadingsCompleteCutoverFixture)
// TestStructuralHeadingsCompleteCutoverFixture proves the frozen literal covers
// the entire declaration-derived cutover population, rather than only the parts
// this adopter happened to override before schema 36.
func TestStructuralHeadingsCompleteCutoverFixture(t *testing.T) {
	const cutoverPopulation = 171
	if len(structuralHeadingSnapshot) != cutoverPopulation {
		t.Fatalf("snapshot population = %d, want %d", len(structuralHeadingSnapshot), cutoverPopulation)
	}
	files := make(map[string]string, len(structuralHeadingSnapshot))
	seen := map[string]bool{}
	for _, entry := range structuralHeadingSnapshot {
		if seen[entry.path] {
			t.Fatalf("duplicate cutover path %q", entry.path)
		}
		seen[entry.path] = true
		path := strings.Replace(entry.path, "*", "example", 1)
		files[path] = entry.heading + "\nbody\n"
	}
	root := closeFixture(t, "prefix: ex\n", files)
	if err := applyStructuralHeadings(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	for path := range files {
		got, err := os.ReadFile(filepath.Join(root, ".awf", path))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "body\n" {
			t.Errorf("%s = %q, want body only", path, got)
		}
	}
}

// invariant: config/migrations-and-locks:structural-heading-part-migration (TestStructuralHeadingsMigration)
func TestStructuralHeadingsMigration(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"docs/parts/testing/gate.md":              "## The gate\nbody\n",
		"docs/parts/testing/tiers.md":             "body without a heading\n",
		"docs/parts/architecture/dependencies.md": "<!-- awf:comment source-only -->\n## Key dependencies\nbody\n",
		"docs/parts/roadmap/ideas.md":             "## Ideas\n### A second candidate\nbody\n",
		"docs/parts/development/setup.md":         "## Custom setup\nbody\n",
		"docs/parts/future/body.md":               "## Future heading\nbody\n",
	})
	before := snapshotTree(t, root)
	err := applyStructuralHeadings(root, io.Discard)
	if err == nil {
		t.Fatal("multiple/custom leading headings must refuse")
	}
	refusal := err.Error()
	ordered := []string{
		"operation:",
		".awf/docs/parts/development/setup.md",
		`exact removable heading "## Setup"`,
		"changed bytes: no",
		"changed index: no",
		"changed message: no",
		"changed merge state: no",
		"next actions: 1. edit .awf/docs/parts/development/setup.md",
		"2. run `awf upgrade`",
	}
	position := -1
	for _, clause := range ordered {
		next := strings.Index(refusal[position+1:], clause)
		if next < 0 {
			t.Fatalf("refusal missing ordered clause %q: %s", clause, refusal)
		}
		position += next + 1
	}
	if strings.Contains(refusal, "cause:") {
		t.Fatalf("operation refusal must carry no cause: %s", refusal)
	}
	if !sameSnapshot(before, snapshotTree(t, root)) {
		t.Fatal("preflight refusal changed fixture bytes")
	}
	for _, rel := range []string{"docs/parts/roadmap/ideas.md", "docs/parts/development/setup.md"} {
		if err := os.WriteFile(filepath.Join(root, ".awf", rel), []byte("body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var out bytes.Buffer
	if err := applyStructuralHeadings(root, &out); err != nil {
		t.Fatal(err)
	}
	for rel, want := range map[string]string{
		"docs/parts/testing/gate.md":              "body\n",
		"docs/parts/testing/tiers.md":             "body without a heading\n",
		"docs/parts/architecture/dependencies.md": "<!-- awf:comment source-only -->\nbody\n",
		"docs/parts/future/body.md":               "## Future heading\nbody\n",
	} {
		got, err := os.ReadFile(filepath.Join(root, ".awf", rel))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if !strings.Contains(out.String(), "structural-headings: updated .awf/docs/parts/testing/gate.md") {
		t.Errorf("missing announcement: %q", out.String())
	}
	after := snapshotTree(t, root)
	out.Reset()
	if err := applyStructuralHeadings(root, &out); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !sameSnapshot(after, snapshotTree(t, root)) {
		t.Fatal("rerun was not a silent no-op")
	}
}

func TestStructuralHeadingsRefusesMultipleAndSupportsUnterminatedHeading(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{"docs/parts/roadmap/ideas.md": "## Ideas\n### second\nbody\n"})
	before := snapshotTree(t, root)
	if err := applyStructuralHeadings(root, io.Discard); err == nil || !strings.Contains(err.Error(), "operation:") {
		t.Fatalf("multiple heading refusal = %v", err)
	}
	if !sameSnapshot(before, snapshotTree(t, root)) {
		t.Fatal("multiple-heading refusal mutated files")
	}
	root = closeFixture(t, "prefix: ex\n", map[string]string{"docs/parts/testing/gate.md": "## The gate"})
	if err := applyStructuralHeadings(root, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, ".awf/docs/parts/testing/gate.md"))
	if string(got) != "" {
		t.Fatalf("unterminated heading = %q", got)
	}
}

// invariant: config/migrations-and-locks:structural-heading-part-migration (TestStructuralHeadingsRetryAfterWriteFailure)
func TestStructuralHeadingsRetryAfterWriteFailure(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"docs/parts/testing/gate.md":  "## The gate\nbody\n",
		"docs/parts/testing/tiers.md": "## Tiers\nbody\n",
	})
	writes := 0
	injected := errors.New("injected write failure")
	err := applyStructuralHeadingsWithWriter(root, io.Discard, func(path string, b []byte, mode os.FileMode) error {
		writes++
		if writes == 2 {
			return injected
		}
		return manifest.WriteFileAtomicMode(path, b, mode)
	})
	if !errors.Is(err, injected) {
		t.Fatalf("first apply = %v", err)
	}
	if err := applyStructuralHeadings(root, io.Discard); err != nil {
		t.Fatalf("retry = %v", err)
	}
}

type structuralHeadingFailWriter struct{ err error }

func (w structuralHeadingFailWriter) Write([]byte) (int, error) { return 0, w.err }

func TestStructuralHeadingsReportsAnnouncementFailure(t *testing.T) {
	root := closeFixture(t, "prefix: ex\n", map[string]string{
		"docs/parts/testing/gate.md": "## The gate\nbody\n",
	})
	injected := errors.New("announcement failed")
	err := applyStructuralHeadings(root, structuralHeadingFailWriter{err: injected})
	if !errors.Is(err, injected) || !strings.Contains(err.Error(), "announce structural-heading update") {
		t.Fatalf("announcement failure = %v", err)
	}
}
