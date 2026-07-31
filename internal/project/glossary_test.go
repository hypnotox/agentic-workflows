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
	p, err := Open(testContext(t), root)
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
// authored order, and two sidecars carrying the same records in a different
// order render byte-identically.
// invariant: rendering/guide-and-doc-templates:glossary-terms-sorted
func TestGlossaryRendersSorted(t *testing.T) {
	a := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - term: zeta\n      meaning: last\n    - term: Alpha\n      meaning: first\n    - term: beta\n      meaning: middle\n",
	}))
	b := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - term: beta\n      meaning: middle\n    - term: zeta\n      meaning: last\n    - term: Alpha\n      meaning: first\n",
	}))
	if a != b {
		t.Errorf("equal record sets must render byte-identically:\n%s\n---\n%s", a, b)
	}
	iAlpha := strings.Index(a, "| Alpha |")
	iBeta := strings.Index(a, "| beta |")
	iZeta := strings.Index(a, "| zeta |")
	if iAlpha < 0 || iBeta < 0 || iZeta < 0 || iAlpha >= iBeta || iBeta >= iZeta {
		t.Errorf("rows not case-insensitively sorted (Alpha=%d beta=%d zeta=%d):\n%s", iAlpha, iBeta, iZeta, a)
	}
}

// An optional domains list is accepted and is not rendered: it feeds the
// checkGlossary drift check, never the table.
func TestGlossaryDomainsAcceptedAndUnrendered(t *testing.T) {
	out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		// Domain names chosen so no generated framing text can contain them.
		"docs/glossary.yaml": "data:\n  terms:\n    - term: tagged\n      meaning: a meaning\n      domains: [zzz-alpha, zzz-beta]\n",
	}))
	if !strings.Contains(out, "| tagged | a meaning |") {
		t.Errorf("record with domains did not render:\n%s", out)
	}
	if strings.Contains(out, "zzz-") {
		t.Errorf("domains must not reach the rendered table:\n%s", out)
	}
}

// Pipes in terms and meanings are escaped so a code-span pipe cannot split a
// GFM table cell; the header renders exactly once.
func TestGlossaryEscapesPipes(t *testing.T) {
	out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - term: pipe-term\n      meaning: \"a `x | y` table\"\n",
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
// invariant: rendering/guide-and-doc-templates:glossary-table-forced
func TestGlossaryTableForcedBetweenFraming(t *testing.T) {
	out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml":             "data:\n  terms:\n    - term: only\n      meaning: entry\n",
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

// Absent data, an empty list, and an explicit null all degrade to the coherent
// placeholder line naming the authoring surface - never a zero-row table
// (ADR-0045 via ADR-0089 Decision 4).
func TestGlossaryDegradesWithoutTerms(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no-sidecar": nil,
		"empty-list": {"docs/glossary.yaml": "data:\n  terms: []\n"},
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

// Content violations fail the render naming the sidecar path, and the offending
// term wherever the term itself parsed.
// invariant: rendering/guide-and-doc-templates:glossary-terms-validated
func TestGlossaryContentViolations(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, wantErr string }{
		"terms-not-a-list":    {"data:\n  terms: just a string\n", "must be a list of {term, meaning} records"},
		"record-not-a-map":    {"data:\n  terms:\n    - just a string\n", "record 0 must be a mapping"},
		"non-string-key":      {"data:\n  terms:\n    - 42: meaning\n", "record 0: key 42 is not a string"},
		"missing-term":        {"data:\n  terms:\n    - meaning: orphan\n", `record 0: missing "term"`},
		"null-term":           {"data:\n  terms:\n    - term:\n      meaning: m\n", `record 0: "term" must be a non-empty string`},
		"non-string-term":     {"data:\n  terms:\n    - term: 42\n      meaning: m\n", `record 0: "term" must be a non-empty string`},
		"empty-term":          {"data:\n  terms:\n    - term: \"  \"\n      meaning: m\n", "record 0: term is empty"},
		"newline-term":        {"data:\n  terms:\n    - term: \"a\\nb\"\n      meaning: m\n", "contains a newline"},
		"missing-meaning":     {"data:\n  terms:\n    - term: lonely\n", `term "lonely": missing "meaning"`},
		"null-meaning":        {"data:\n  terms:\n    - term: t\n      meaning:\n", `term "t": meaning must be a non-empty string`},
		"non-string-meaning":  {"data:\n  terms:\n    - term: t\n      meaning: 42\n", `term "t": meaning must be a non-empty string`},
		"empty-meaning":       {"data:\n  terms:\n    - term: t\n      meaning: \"  \"\n", `term "t": meaning is empty`},
		"newline-meaning":     {"data:\n  terms:\n    - term: t\n      meaning: \"a\\nb\"\n", "meaning contains a newline"},
		"domains-not-a-list":  {"data:\n  terms:\n    - term: t\n      meaning: m\n      domains: rendering\n", `term "t": "domains" must be a list`},
		"domains-non-string":  {"data:\n  terms:\n    - term: t\n      meaning: m\n      domains: [42]\n", `term "t": "domains" entries must be non-empty strings`},
		"domains-empty-entry": {"data:\n  terms:\n    - term: t\n      meaning: m\n      domains: [\"  \"]\n", `term "t": "domains" entries must be non-empty strings`},
		"unknown-key":         {"data:\n  terms:\n    - term: t\n      meaning: m\n      alias: nope\n", `term "t": unknown key "alias"`},
		"case-dup":            {"data:\n  terms:\n    - term: Foo\n      meaning: one\n    - term: foo\n      meaning: two\n", "case-insensitive duplicates"},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := Open(testContext(t), scaffoldFiles(t, glossaryCfg, map[string]string{"docs/glossary.yaml": tc.yaml}))
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
	sc := config.Sidecar{Data: map[string]any{"terms": []any{
		map[any]any{"term": "b", "meaning": "two"},
		map[any]any{"term": "a", "meaning": "one"},
	}}}
	out, err := glossaryTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.Data["terms"]; got != "| a | one |\n| b | two |\n" {
		t.Errorf("unexpected rows: %q", got)
	}
}

// The transform never mutates the caller's sidecar data map, and never reorders
// the caller's record slice.
func TestGlossaryTransformClonesData(t *testing.T) {
	terms := []any{
		map[string]any{"term": "zeta", "meaning": "last"},
		map[string]any{"term": "alpha", "meaning": "first"},
	}
	sc := config.Sidecar{Data: map[string]any{"terms": terms, "other": "kept"}}
	out, err := glossaryTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if _, still := sc.Data["terms"].([]any); !still {
		t.Error("caller's data map was mutated")
	}
	if first := terms[0].(map[string]any)["term"]; first != "zeta" {
		t.Errorf("caller's record slice was reordered: first is %q", first)
	}
	if out.Data["other"] != "kept" {
		t.Error("unrelated data keys must carry over")
	}
}
