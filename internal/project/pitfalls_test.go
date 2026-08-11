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

func TestPitfallMetadataProjectionKeepsMarkdownStructure(t *testing.T) {
	const domain = "# [domain] | `code` \\ path\r\nnext"
	const tag = "tag | [literal] `tick` \\ value\r\ncontinued"
	root := scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls/punctuation.md": pitfallSource("'# [Title] | `tick` \\ literal'", "domains: [\"# [domain] | `code` \\\\ path\\r\\nnext\"]\ntags: [\"tag | [literal] `tick` \\\\ value\\r\\ncontinued\"]\n", "body bytes | remain `verbatim`\n"),
	})
	files := renderPitfallFiles(t, root)
	index := files["docs/pitfalls.md"]
	wantHeading := "### " + pitfall.EscapeHeading(domain)
	if !strings.Contains(index, wantHeading+"\n") {
		t.Fatalf("domain heading projection missing %q:\n%s", wantHeading, index)
	}
	var row string
	for _, line := range strings.Split(index, "\n") {
		if strings.Contains(line, "pitfalls/punctuation.md") && strings.HasPrefix(line, "|") {
			row = line
			break
		}
	}
	if row == "" || countMarkdownTableDelimiters(row) != 5 {
		t.Fatalf("metadata row structure = %q, delimiters=%d", row, countMarkdownTableDelimiters(row))
	}
	for _, want := range []string{pitfall.EscapeTableCell(domain), pitfall.EscapeTableCell(tag), "[\\# \\[Title\\] \\| \\`tick\\` \\\\ literal](pitfalls/punctuation.md)"} {
		if !strings.Contains(row, want) {
			t.Fatalf("metadata row missing literal projection %q: %s", want, row)
		}
	}
	if strings.ContainsAny(row, "\r\n") || strings.Contains(wantHeading, "\r") || strings.Contains(wantHeading, "\n") {
		t.Fatalf("metadata projection retained structural line break: heading=%q row=%q", wantHeading, row)
	}
	if !strings.Contains(index, "&#13;&#10;") {
		t.Fatalf("metadata line breaks are not visibly escaped:\n%s", index)
	}
	leaf := files["docs/pitfalls/punctuation.md"]
	if !strings.HasSuffix(leaf, "body bytes | remain `verbatim`\n") {
		t.Fatalf("body bytes changed:\n%s", leaf)
	}
}

func countMarkdownTableDelimiters(line string) int {
	count := 0
	for i, r := range line {
		if r != '|' {
			continue
		}
		backslashes := 0
		for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			count++
		}
	}
	return count
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
