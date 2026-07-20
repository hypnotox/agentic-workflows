package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

const glossaryCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [glossary]\n"

// renderGlossary opens root and returns the rendered glossary doc's content.
func renderGlossary(t *testing.T, root string) string {
	t.Helper()
	p, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	files, err := p.RenderAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasSuffix(f.Path, "docs/glossary.md") {
			return f.Content
		}
	}
	t.Fatal("no rendered glossary in RenderAll output")
	return "" // coverage-ignore: t.Fatal never returns
}

// The rendered table is ordered case-insensitively by term regardless of the
// authored map order, and two sidecars carrying the same entries in different
// order render byte-identically.
// invariant: rendering/templates:glossary-terms-sorted
func TestGlossaryRendersSorted(t *testing.T) {
	a := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    zeta: last\n    Alpha: first\n    beta: middle\n",
	}))
	b := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    beta: middle\n    zeta: last\n    Alpha: first\n",
	}))
	if a != b {
		t.Errorf("equal entry sets must render byte-identically:\n%s\n---\n%s", a, b)
	}
	iAlpha := strings.Index(a, "| Alpha |")
	iBeta := strings.Index(a, "| beta |")
	iZeta := strings.Index(a, "| zeta |")
	if iAlpha < 0 || iBeta < 0 || iZeta < 0 || iAlpha >= iBeta || iBeta >= iZeta {
		t.Errorf("rows not case-insensitively sorted (Alpha=%d beta=%d zeta=%d):\n%s", iAlpha, iBeta, iZeta, a)
	}
}

// Pipes in terms and meanings are escaped so a code-span pipe cannot split a
// GFM table cell; the header renders exactly once.
func TestGlossaryEscapesPipes(t *testing.T) {
	out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    pipe-term: \"a `x | y` table\"\n",
	}))
	if !strings.Contains(out, `a `+"`x \\| y`"+` table`) {
		t.Errorf("meaning pipe not escaped:\n%s", out)
	}
	if strings.Count(out, "| Term | Meaning |") != 1 {
		t.Errorf("expected exactly one table header:\n%s", out)
	}
}

// The table renders from plain template text between the prepend and append
// framing sections: parts override the framing, never the table, and a part
// for the retired terms section has no section to claim.
// invariant: rendering/templates:glossary-table-forced
func TestGlossaryTableForcedBetweenFraming(t *testing.T) {
	out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml":             "data:\n  terms:\n    only: entry\n",
		"docs/parts/glossary/prepend.md": "FRAMING-ABOVE\n",
		"docs/parts/glossary/append.md":  "FRAMING-BELOW\n",
	}))
	iPre := strings.Index(out, "FRAMING-ABOVE")
	iRow := strings.Index(out, "| only | entry |")
	iPost := strings.Index(out, "FRAMING-BELOW")
	if iPre < 0 || iRow < 0 || iPost < 0 || iPre >= iRow || iRow >= iPost {
		t.Errorf("table not forced between framing sections (pre=%d row=%d post=%d):\n%s", iPre, iRow, iPost, out)
	}
}

// Absent data, an empty map, and an explicit null all degrade to the coherent
// placeholder line naming the authoring surface - never a zero-row table
// (ADR-0045 via ADR-0089 Decision 4).
func TestGlossaryDegradesWithoutTerms(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no-sidecar": nil,
		"empty-map":  {"docs/glossary.yaml": "data:\n  terms: {}\n"},
		"null-terms": {"docs/glossary.yaml": "data:\n  terms:\n"},
	} {
		t.Run(name, func(t *testing.T) {
			out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, files))
			if !strings.Contains(out, "No terms recorded yet") || !strings.Contains(out, "data.terms") {
				t.Errorf("missing placeholder line:\n%s", out)
			}
			if strings.Contains(out, "| Term | Meaning |") {
				t.Errorf("zero-row table must not render:\n%s", out)
			}
		})
	}
}

// Content violations fail the render naming the sidecar path and the key.
// invariant: rendering/templates:glossary-terms-validated
func TestGlossaryContentViolations(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, wantErr string }{
		"empty-term":      {"data:\n  terms:\n    \" \": meaning\n", "is empty"},
		"empty-meaning":   {"data:\n  terms:\n    term: \"  \"\n", "meaning is empty"},
		"null-meaning":    {"data:\n  terms:\n    term:\n", "must be a non-empty string"},
		"non-string":      {"data:\n  terms:\n    term: 42\n", "must be a non-empty string"},
		"newline-meaning": {"data:\n  terms:\n    term: \"a\\nb\"\n", "contains a newline"},
		"newline-term":    {"data:\n  terms:\n    \"a\\nb\": meaning\n", "contains a newline"},
		"case-dup":        {"data:\n  terms:\n    Foo: one\n    foo: two\n", "case-insensitive duplicates"},
		"non-string-key":  {"data:\n  terms:\n    42: meaning\n", "is not a string"},
		"terms-not-a-map": {"data:\n  terms: just a string\n", "must be a mapping"},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Open(scaffoldFiles(t, glossaryCfg, map[string]string{"docs/glossary.yaml": tc.yaml}))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), glossarySidecarPath) {
				t.Errorf("want render error containing %q and %q, got: %v", tc.wantErr, glossarySidecarPath, err)
			}
		})
	}
}

// glossaryStringMap's map[any]any all-string-keys branch is unreachable via
// yaml.v3 (that shape only arises alongside a non-string key) but reachable by
// any caller handing the transform a constructed value.
func TestGlossaryStringMapAnyKeys(t *testing.T) {
	sc := config.Sidecar{Data: map[string]any{"terms": map[any]any{"b": "two", "a": "one"}}}
	out, err := glossaryTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Data["terms"]; got != "| a | one |\n| b | two |\n" {
		t.Errorf("unexpected rows: %q", got)
	}
}

// The transform never mutates the caller's sidecar data map.
func TestGlossaryTransformClonesData(t *testing.T) {
	terms := map[string]any{"a": "one"}
	sc := config.Sidecar{Data: map[string]any{"terms": terms, "other": "kept"}}
	out, err := glossaryTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if _, still := sc.Data["terms"].(map[string]any); !still {
		t.Error("caller's data map was mutated")
	}
	if out.Data["other"] != "kept" {
		t.Error("unrelated data keys must carry over")
	}
}
