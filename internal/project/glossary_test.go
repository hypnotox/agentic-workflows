package project

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
)

const glossaryCfg = "prefix: example\nintegrationBranch: main\nvars: {}\n"

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
// invariant: rendering/guide-and-doc-templates:glossary-terms-sorted (TestGlossaryRendersSorted)
func TestGlossaryRendersSorted(t *testing.T) {
	a := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - term: zeta\n      meaning: last\n    - term: Alpha\n      meaning: second\n    - term: beta\n      meaning: third\n    - term: aardvark\n      meaning: first\n",
	}))
	b := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
		"docs/glossary.yaml": "data:\n  terms:\n    - term: beta\n      meaning: third\n    - term: aardvark\n      meaning: first\n    - term: zeta\n      meaning: last\n    - term: Alpha\n      meaning: second\n",
	}))
	if a != b {
		t.Errorf("equal record sets must render byte-identically:\n%s\n---\n%s", a, b)
	}
	// aardvark before Alpha is what distinguishes the case-insensitive comparator:
	// under a byte comparison 'A' (0x41) sorts before 'a' (0x61) and this inverts.
	iAard := strings.Index(a, "| aardvark |")
	iAlpha := strings.Index(a, "| Alpha |")
	iBeta := strings.Index(a, "| beta |")
	iZeta := strings.Index(a, "| zeta |")
	if iAard < 0 || iAlpha < 0 || iBeta < 0 || iZeta < 0 || iAard >= iAlpha || iAlpha >= iBeta || iBeta >= iZeta {
		t.Errorf("rows not case-insensitively sorted (aardvark=%d Alpha=%d beta=%d zeta=%d):\n%s", iAard, iAlpha, iBeta, iZeta, a)
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
// invariant: rendering/guide-and-doc-templates:glossary-table-forced (TestGlossaryTableForcedBetweenFraming)
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

// With both layers empty the doc degrades to the coherent placeholder line
// naming the authoring surface - never a zero-row table (ADR-0045 via ADR-0089
// Decision 4). Since ADR-0207 the shipped layer normally supplies rows, so the
// scaffolds here null standardTerms to empty it; a real tree that authored that
// key would be unused-data drift, which is why per-term override, not this, is
// the supported way to drop a shipped term.
func TestGlossaryDegradesWhenBothLayersEmpty(t *testing.T) {
	for name, sidecar := range map[string]string{
		"no-authored-terms": "data:\n  standardTerms:\n",
		"empty-list":        "data:\n  standardTerms:\n  terms: []\n",
		"null-terms":        "data:\n  standardTerms:\n  terms:\n",
	} {
		t.Run(name, func(t *testing.T) {
			out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{"docs/glossary.yaml": sidecar}))
			if !strings.Contains(out, "No terms recorded yet") || !strings.Contains(out, "data.terms") {
				t.Errorf("missing placeholder line:\n%s", out)
			}
			if strings.Contains(out, "| Term | Meaning |") {
				t.Errorf("zero-row table must not render:\n%s", out)
			}
		})
	}
}

// A sidecar carrying neither key leaves the data untouched, so a doc whose
// catalog entry ships no default data still degrades rather than erroring.
func TestGlossaryTransformUntouchedWithoutEitherLayer(t *testing.T) {
	sc := config.Sidecar{Data: map[string]any{"other": "kept"}}
	out, err := glossaryTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := out.Data["terms"]; present {
		t.Error("terms must not be synthesised when neither layer is present")
	}
	if out.Data["other"] != "kept" {
		t.Error("unrelated data keys must carry over")
	}
}

// Content violations fail the render naming the sidecar path, and the offending
// term wherever the term itself parsed.
// invariant: rendering/guide-and-doc-templates:glossary-terms-validated (TestGlossaryContentViolations)
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

// Every shipped standard term carries exactly a string term and a string
// meaning, cites no ADR, and stays under the terseness threshold, so the layer
// is portable into any adopter tree and cannot fail that tree's own advisory.
// invariant: rendering/guide-and-doc-templates:glossary-standard-terms-portable (TestGlossaryStandardTermsPortable)
func TestGlossaryStandardTermsPortable(t *testing.T) {
	raw, ok := catalog.Standard.Docs["glossary"].Data["standardTerms"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatalf("standardTerms must be a non-empty []any, got %T", catalog.Standard.Docs["glossary"].Data["standardTerms"])
	}
	adrRe := regexp.MustCompile(`ADR-[0-9]{4}`)
	for i, el := range raw {
		m, isMap := el.(map[string]any)
		if !isMap {
			t.Fatalf("shipped record %d must be a map[string]any, got %T", i, el)
		}
		if len(m) != 2 {
			t.Errorf("shipped record %d carries %d keys; exactly term and meaning are allowed (no domains: they cannot resolve in an adopter tree)", i, len(m))
		}
		for _, key := range []string{"term", "meaning"} {
			s, isStr := m[key].(string)
			if !isStr || strings.TrimSpace(s) == "" {
				t.Errorf("shipped record %d: %q must be a non-empty string, got %#v", i, key, m[key])
				continue
			}
			if got := adrRe.FindString(s); got != "" {
				t.Errorf("shipped record %d %q cites %s; a citation resolves to nothing in an adopter corpus", i, key, got)
			}
		}
		// Runes, matching what the advisory measures.
		if s, isStr := m["meaning"].(string); isStr && utf8.RuneCountInString(s) > glossaryMeaningMax {
			t.Errorf("shipped meaning for %v is %d chars, over the %d threshold", m["term"], utf8.RuneCountInString(s), glossaryMeaningMax)
		}
	}
}

// The rendered glossary merges the shipped vocabulary with the project's own
// terms into one sorted table, and a project term overrides a shipped term of
// the same case-insensitive name. A project authoring no terms at all still
// receives the shipped rows rather than the empty-state pointer, which is the
// fresh-adoption case this layer exists to fix.
// invariant: rendering/guide-and-doc-templates:glossary-standard-vocabulary (TestGlossaryMergesStandardVocabulary)
func TestGlossaryMergesStandardVocabulary(t *testing.T) {
	t.Run("no authored terms still renders the shipped layer", func(t *testing.T) {
		out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, nil))
		if strings.Contains(out, "No terms recorded yet") {
			t.Errorf("empty-state branch taken despite the shipped layer:\n%s", out)
		}
		if !strings.Contains(out, "| Term | Meaning |") || !strings.Contains(out, "| effort |") {
			t.Errorf("shipped rows missing:\n%s", out)
		}
	})

	t.Run("a project term overrides the shipped term of the same name", func(t *testing.T) {
		out := renderGlossary(t, scaffoldFiles(t, glossaryCfg, map[string]string{
			"docs/glossary.yaml": "data:\n  terms:\n    - term: EFFORT\n      meaning: the project's own wording\n    - term: local-only\n      meaning: not shipped at all\n",
		}))
		if !strings.Contains(out, "| EFFORT | the project's own wording |") {
			t.Errorf("override row missing:\n%s", out)
		}
		if strings.Contains(out, "| effort |") {
			t.Errorf("shipped term survived a case-insensitive override:\n%s", out)
		}
		if !strings.Contains(out, "| local-only | not shipped at all |") {
			t.Errorf("project-only row missing:\n%s", out)
		}
		if !strings.Contains(out, "| drift |") {
			t.Errorf("unoverridden shipped rows must still render:\n%s", out)
		}
	})

	t.Run("standardTerms never reaches the template", func(t *testing.T) {
		sc := config.Sidecar{Data: map[string]any{
			"standardTerms": []any{map[string]any{"term": "shipped", "meaning": "from the catalog"}},
		}}
		out, err := glossaryTransform(sc)
		if err != nil {
			t.Fatal(err)
		}
		if _, still := out.Data["standardTerms"]; still {
			t.Error("standardTerms must be deleted after the merge")
		}
		if got := out.Data["terms"]; got != "| shipped | from the catalog |\n" {
			t.Errorf("unexpected rows: %q", got)
		}
	})

	t.Run("a duplicate inside the shipped layer is a hard error", func(t *testing.T) {
		sc := config.Sidecar{Data: map[string]any{"standardTerms": []any{
			map[string]any{"term": "Dup", "meaning": "one"},
			map[string]any{"term": "dup", "meaning": "two"},
		}}}
		if _, err := glossaryTransform(sc); err == nil || !strings.Contains(err.Error(), "standard vocabulary is malformed") ||
			!strings.Contains(err.Error(), "case-insensitive duplicates") {
			t.Fatalf("want a standard-vocabulary duplicate error, got %v", err)
		}
	})
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
