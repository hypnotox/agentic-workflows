package project

import (
	"errors"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/pitfall"
)

const pitfallsCfg = "prefix: example\nintegrationBranch: main\nvars: {}\n"

func pitfallSource(title, extra, body string) string {
	return "---\ntitle: " + title + "\n" + extra + "---\n" + body
}

func renderPitfallFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range files {
		out[f.Path] = f.Content
	}
	return out
}

func TestPitfallCorpusRendersIndexAndLeaves(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/second.md": pitfallSource("Second", "domains: [rendering]\ntags: [proof]\nrelated: [1]\n", "second body\n"),
		"docs/pitfalls/first.md":  pitfallSource("First [literal] | title", "", "first body\n"),
	})
	files := renderPitfallFiles(t, root)
	index := files["docs/pitfalls.md"]
	if !strings.Contains(index, "Pitfalls are durable hazards, not backlog items") ||
		!strings.Contains(index, "[First \\[literal\\] \\| title](pitfalls/first.md)") ||
		!strings.Contains(index, "### Unassigned") || !strings.Contains(index, "### rendering") {
		t.Fatalf("unexpected index:\n%s", index)
	}
	leaf := files["docs/pitfalls/second.md"]
	for _, want := range []string{"<!-- awf:source .awf/docs/pitfalls/second.md -->", "# Second", "**Domains:** rendering", "**Tags:** proof", "ADR-0001", "second body"} {
		if !strings.Contains(leaf, want) {
			t.Errorf("leaf missing %q:\n%s", want, leaf)
		}
	}
	if strings.Contains(index+leaf, "<no value>") {
		t.Fatal("pitfall outputs contain <no value>")
	}
}

type failingPitfallReader struct {
	paths            []string
	pathErr, readErr error
}

func (r failingPitfallReader) Paths(string) ([]string, error)        { return r.paths, r.pathErr }
func (r failingPitfallReader) ReadFile(string) ([]byte, bool, error) { return nil, false, r.readErr }

func TestPitfallCorpusReaderErrorsAndIndexTie(t *testing.T) {
	boom := errors.New("boom")
	if _, err := loadPitfallCorpusFrom(failingPitfallReader{pathErr: boom}); !errors.Is(err, boom) {
		t.Fatalf("paths error = %v", err)
	}
	if _, err := loadPitfallCorpusFrom(failingPitfallReader{paths: []string{".awf/docs/pitfalls/a.md"}, readErr: boom}); err == nil || !strings.Contains(err.Error(), "read pitfall") {
		t.Fatalf("read error = %v", err)
	}
	model := buildPitfallIndex(pitfall.New([]pitfall.Entry{{Slug: "b", Title: "same", Domains: []string{"z"}}, {Slug: "a", Title: "Same", Domains: []string{"a", "z"}}, {Slug: "u", Title: "Untitled"}}))
	if len(model.Entries) != 3 || model.Entries[0].Slug != "a" || len(model.Domains) != 2 || len(model.Unassigned) != 1 {
		t.Fatalf("model = %#v", model)
	}
}

func TestPitfallCorpusEmptyState(t *testing.T) {
	files := renderPitfallFiles(t, scaffoldFiles(t, pitfallsCfg, nil))
	index := files["docs/pitfalls.md"]
	if !strings.Contains(index, "No pitfalls recorded yet") || !strings.Contains(index, ".awf/docs/pitfalls/") {
		t.Fatal(index)
	}
}

func TestPitfallCorpusMalformedSourceFailsRender(t *testing.T) {
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{"docs/pitfalls/bad.md": "---\ntitle: Bad\nunknown: x\n---\nbody\n"})
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), ".awf/docs/pitfalls/bad.md") || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("malformed source error = %v", err)
	}
}
