package migrate

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/testsupport"
	"gopkg.in/yaml.v3"
)

// A `##`-delimited part splits into a data.pitfalls sidecar (fenced `##` lines
// not mis-split), the part file and its dir are removed, and one provenance line
// per entry plus the review instruction print.
func TestPitfallsDataSplits(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md")
	testsupport.WriteFile(t, part,
		"## First pitfall\n\nbody one\n\nwith two paragraphs\n\n"+
			"## Second pitfall\n\n```\n## not a heading inside a fence\n```\nbody two\n")

	var out bytes.Buffer
	if err := applyPitfallsData(root, &out); err != nil {
		t.Fatalf("applyPitfallsData: %v", err)
	}

	// Provenance + review lines.
	for _, want := range []string{
		`pitfalls-data: split entry "First pitfall"` + "\n",
		`pitfalls-data: split entry "Second pitfall"` + "\n",
		"pitfalls-data: review .awf/docs/pitfalls.yaml and tag each entry's domains:",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("missing output %q in:\n%s", want, out.String())
		}
	}

	// The part and its now-empty dir are gone.
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Errorf("part not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(part)); !os.IsNotExist(err) {
		t.Errorf("empty part dir not removed: %v", err)
	}

	// The sidecar parses into exactly two entries, with the fenced `##` kept in
	// the body (not split into a third entry).
	b, err := os.ReadFile(filepath.Join(root, ".awf", "docs", "pitfalls.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var sc struct {
		Data struct {
			Pitfalls []struct {
				Title string `yaml:"title"`
				Body  string `yaml:"body"`
			} `yaml:"pitfalls"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal(b, &sc); err != nil {
		t.Fatalf("sidecar is not valid YAML: %v\n%s", err, b)
	}
	if len(sc.Data.Pitfalls) != 2 {
		t.Fatalf("want 2 entries, got %d: %s", len(sc.Data.Pitfalls), b)
	}
	if sc.Data.Pitfalls[0].Title != "First pitfall" || !strings.Contains(sc.Data.Pitfalls[0].Body, "body one\n\nwith two paragraphs") {
		t.Errorf("entry 0 wrong (internal blank line must survive): %+v", sc.Data.Pitfalls[0])
	}
	if !strings.Contains(sc.Data.Pitfalls[1].Body, "## not a heading inside a fence") {
		t.Errorf("fenced ## should stay in the body: %+v", sc.Data.Pitfalls[1])
	}
}

// A part with no top-level `##` heading yields an empty-list sidecar (still valid).
func TestPitfallsDataEmptyList(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md")
	testsupport.WriteFile(t, part, "just prose, no headings\n")
	if err := applyPitfallsData(root, io.Discard); err != nil {
		t.Fatalf("applyPitfallsData: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".awf", "docs", "pitfalls.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(b)) != "data:\n  pitfalls: []" {
		t.Errorf("empty part must yield an empty list, got:\n%s", b)
	}
}

// An absent part is a clean no-op — no output, no sidecar written — so a re-run
// after a prior split does nothing.
func TestPitfallsDataNoOp(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "config.yaml"), "prefix: ex\n")
	var out bytes.Buffer
	if err := applyPitfallsData(root, &out); err != nil {
		t.Fatalf("applyPitfallsData: %v", err)
	}
	if out.String() != "" {
		t.Errorf("absent part must print nothing, got:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "docs", "pitfalls.yaml")); !os.IsNotExist(err) {
		t.Errorf("absent part must not write a sidecar: %v", err)
	}
}

// A title with a double-quote is escaped so the emitted YAML stays valid.
func TestPitfallsDataEscapesTitle(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md"),
		"## A \"quoted\" and \\slashed title\n\nbody\n")
	if err := applyPitfallsData(root, io.Discard); err != nil {
		t.Fatalf("applyPitfallsData: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".awf", "docs", "pitfalls.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var sc struct {
		Data struct {
			Pitfalls []struct {
				Title string `yaml:"title"`
			} `yaml:"pitfalls"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal(b, &sc); err != nil {
		t.Fatalf("escaped title broke YAML: %v\n%s", err, b)
	}
	if len(sc.Data.Pitfalls) != 1 || sc.Data.Pitfalls[0].Title != `A "quoted" and \slashed title` {
		t.Errorf("title not round-tripped: %+v", sc.Data.Pitfalls)
	}
}
