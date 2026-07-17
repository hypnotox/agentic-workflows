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

// A part with no top-level `##` heading is retained because it cannot be
// migrated losslessly.
func TestPitfallsDataEmptyList(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md")
	testsupport.WriteFile(t, part, "just prose, no headings\n")
	if err := applyPitfallsData(root, io.Discard); err == nil || err.Error() != "pitfalls-data: no top-level entries to migrate" {
		t.Fatalf("applyPitfallsData error = %v, want no-entry error", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".awf", "docs", "pitfalls.yaml")); !os.IsNotExist(err) {
		t.Errorf("empty part must not write a sidecar: %v", err)
	}
	if _, err := os.Stat(part); err != nil {
		t.Errorf("part should be retained: %v", err)
	}
}

// An absent part is a clean no-op - no output, no sidecar written - so a re-run
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

func TestPitfallsDataRejectsInvalidSerializationBeforeDeletion(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md")
	original := []byte("## Retained\n\nbody\n")
	testsupport.WriteFile(t, part, string(original))
	testsupport.SwapVar(t, &renderPitfalls, func([]pitfallSplit) ([]byte, error) {
		return []byte("not: [valid"), nil
	})

	if err := applyPitfallsData(root, io.Discard); err == nil {
		t.Fatal("applyPitfallsData succeeded after invalid serialization")
	}
	got, err := os.ReadFile(part)
	if err != nil {
		t.Fatalf("read retained part: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("part changed after invalid serialization: got %q, want %q", got, original)
	}
}

func TestPitfallsDataRetainsSourceOnRenderError(t *testing.T) {
	root := t.TempDir()
	part := filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md")
	original := []byte("## Retained\n\nbody\n")
	testsupport.WriteFile(t, part, string(original))
	testsupport.SwapVar(t, &renderPitfalls, func([]pitfallSplit) ([]byte, error) {
		return nil, io.ErrUnexpectedEOF
	})

	if err := applyPitfallsData(root, io.Discard); err == nil {
		t.Fatal("applyPitfallsData succeeded after render error")
	}
	got, err := os.ReadFile(part)
	if err != nil {
		t.Fatalf("read retained part: %v", err)
	}
	if !bytes.Equal(got, original) {
		t.Errorf("part changed after render error: got %q, want %q", got, original)
	}
}

func TestValidatePitfallsSidecarRejectsChangedEntries(t *testing.T) {
	entries := []pitfallSplit{{title: "Expected", body: []string{"body"}}}
	for _, sidecar := range [][]byte{
		[]byte("data:\n  pitfalls: []\n"),
		[]byte("data:\n  pitfalls:\n    - title: Changed\n      body: changed\n"),
	} {
		if err := validatePitfallsSidecar(sidecar, entries); err == nil {
			t.Errorf("validatePitfallsSidecar(%q) succeeded", sidecar)
		}
	}
}

func TestPitfallsDataPreservesIndentedBodies(t *testing.T) {
	root := t.TempDir()
	testsupport.WriteFile(t, filepath.Join(root, ".awf", "docs", "parts", "pitfalls", "entries.md"), "## Mixed code and prose\n\n    code first\n    still code\n\ncolumn-zero prose\n\n## Only code\n\n    all code\n    remains indented\n")

	if err := applyPitfallsData(root, io.Discard); err != nil {
		t.Fatalf("applyPitfallsData: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".awf", "docs", "pitfalls.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var sidecar struct {
		Data struct {
			Pitfalls []struct {
				Title string `yaml:"title"`
				Body  string `yaml:"body"`
			} `yaml:"pitfalls"`
		} `yaml:"data"`
	}
	if err := yaml.Unmarshal(b, &sidecar); err != nil {
		t.Fatalf("sidecar is not valid YAML: %v\n%s", err, b)
	}
	want := []struct{ title, body string }{
		{"Mixed code and prose", "    code first\n    still code\n\ncolumn-zero prose"},
		{"Only code", "    all code\n    remains indented"},
	}
	if len(sidecar.Data.Pitfalls) != len(want) {
		t.Fatalf("got %d entries, want %d: %s", len(sidecar.Data.Pitfalls), len(want), b)
	}
	for i, entry := range sidecar.Data.Pitfalls {
		if entry.Title != want[i].title || entry.Body != want[i].body {
			t.Errorf("entry %d = (%q, %q), want (%q, %q)", i, entry.Title, entry.Body, want[i].title, want[i].body)
		}
	}
	if !bytes.Contains(b, []byte(`body: "    code first`)) || !bytes.Contains(b, []byte(`body: "    all code`)) {
		t.Errorf("rendered YAML lost code indentation:\n%s", b)
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
