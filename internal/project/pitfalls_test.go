package project

import (
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/config"
)

const pitfallsCfg = "prefix: example\nvars: {}\nskills: []\nagents: []\ndocs: [pitfalls]\n"

// renderPitfalls opens root and returns the rendered pitfalls doc content.
func renderPitfalls(t *testing.T, root string) string {
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
		if strings.HasSuffix(f.Path, "docs/pitfalls.md") {
			return f.Content
		}
	}
	t.Fatal("no rendered pitfalls in RenderAll output")
	return "" // coverage-ignore: t.Fatal never returns
}

// A valid entry list parses in authored order, trims a trailing body newline, and
// leaves absent domains/related nil.
func TestPitfallEntriesValid(t *testing.T) {
	raw := []any{
		map[string]any{"title": " First ", "body": "one\n", "domains": []any{"rendering", " tooling "}, "related": []any{67, 92}},
		map[string]any{"title": "Second", "body": "two"},
	}
	entries, err := pitfallEntries(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	e := entries[0]
	if e.Title != "First" || e.Body != "one" {
		t.Errorf("title/body trim wrong: %q / %q", e.Title, e.Body)
	}
	if len(e.Domains) != 2 || e.Domains[0] != "rendering" || e.Domains[1] != "tooling" {
		t.Errorf("domains wrong: %v", e.Domains)
	}
	if len(e.Related) != 2 || e.Related[0] != 67 || e.Related[1] != 92 {
		t.Errorf("related wrong: %v", e.Related)
	}
	if entries[1].Domains != nil || entries[1].Related != nil {
		t.Errorf("absent domains/related must be nil: %v / %v", entries[1].Domains, entries[1].Related)
	}
}

// An absent (nil) value yields nil, nil — the template's else branch renders.
func TestPitfallEntriesNil(t *testing.T) {
	entries, err := pitfallEntries(nil)
	if err != nil || entries != nil {
		t.Errorf("nil raw must yield nil,nil; got %v, %v", entries, err)
	}
}

// A map[any]any element with all-string keys parses; a non-string key errors.
func TestPitfallEntriesMapAnyKeys(t *testing.T) {
	entries, err := pitfallEntries([]any{map[any]any{"title": "T", "body": "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Title != "T" {
		t.Fatalf("map[any]any element must parse: %v", entries)
	}
	if _, err := pitfallEntries([]any{map[any]any{42: "x", "title": "T", "body": "b"}}); err == nil || !strings.Contains(err.Error(), "is not a string") {
		t.Errorf("non-string map key must error, got: %v", err)
	}
}

// Every structural violation is a hard error naming the sidecar path.
func TestPitfallEntriesErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		raw     any
		wantErr string
	}{
		"not-a-list":        {"just a string", "must be a list"},
		"element-not-map":   {[]any{42}, "must be a mapping"},
		"missing-title":     {[]any{map[string]any{"body": "b"}}, `missing "title"`},
		"title-not-string":  {[]any{map[string]any{"title": 1, "body": "b"}}, `"title" must be a string`},
		"empty-title":       {[]any{map[string]any{"title": "  ", "body": "b"}}, "title is empty"},
		"newline-title":     {[]any{map[string]any{"title": "a\nb", "body": "b"}}, "contains a newline"},
		"missing-body":      {[]any{map[string]any{"title": "T"}}, `missing "body"`},
		"body-not-string":   {[]any{map[string]any{"title": "T", "body": 2}}, `"body" must be a string`},
		"empty-body":        {[]any{map[string]any{"title": "T", "body": "  "}}, "body is empty"},
		"domains-not-list":  {[]any{map[string]any{"title": "T", "body": "b", "domains": "x"}}, `"domains" must be a list`},
		"domain-not-string": {[]any{map[string]any{"title": "T", "body": "b", "domains": []any{42}}}, "must be non-empty strings"},
		"domain-empty":      {[]any{map[string]any{"title": "T", "body": "b", "domains": []any{" "}}}, "must be non-empty strings"},
		"related-not-list":  {[]any{map[string]any{"title": "T", "body": "b", "related": "x"}}, `"related" must be a list`},
		"related-not-int":   {[]any{map[string]any{"title": "T", "body": "b", "related": []any{"x"}}}, "must be ADR numbers"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := pitfallEntries(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), pitfallsSidecarPath) {
				t.Errorf("want error containing %q and %q, got: %v", tc.wantErr, pitfallsSidecarPath, err)
			}
		})
	}
}

// pitfallsMarkdown renders entries in authored order with optional Domains/Related
// lines, a blank separator between entries, and no sort.
func TestPitfallsMarkdown(t *testing.T) {
	if got := pitfallsMarkdown(nil); got != "" {
		t.Errorf("empty entries must render empty, got %q", got)
	}
	out := pitfallsMarkdown([]pitfallEntry{
		{Title: "Bare", Body: "b1"},
		{Title: "Tagged", Domains: []string{"rendering", "tooling"}, Related: []int{67, 92}, Body: "b2"},
	})
	want := "## Bare\n\nb1\n" + "\n" + "## Tagged\n\n_Domains: rendering, tooling_\n\n_Related: ADR-0067, ADR-0092_\n\nb2\n"
	if out != want {
		t.Errorf("markdown mismatch:\n got %q\nwant %q", out, want)
	}
}

// pitfallsTransform leaves a sidecar without data.pitfalls untouched, computes the
// markdown for a valid list without mutating the caller's map, degrades a null list
// to "", and propagates a structural error.
func TestPitfallsTransform(t *testing.T) {
	// absent key: returned unchanged
	sc := config.Sidecar{Data: map[string]any{"other": "kept"}}
	out, err := pitfallsTransform(sc)
	if err != nil || out.Data["pitfalls"] != nil {
		t.Errorf("absent pitfalls must be untouched, got %v / %v", out.Data["pitfalls"], err)
	}

	// valid list: computed to markdown, original not mutated
	list := []any{map[string]any{"title": "T", "body": "b"}}
	sc = config.Sidecar{Data: map[string]any{"pitfalls": list, "other": "kept"}}
	out, err = pitfallsTransform(sc)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := out.Data["pitfalls"].(string); !ok || !strings.Contains(s, "## T") {
		t.Errorf("pitfalls not computed to markdown: %v", out.Data["pitfalls"])
	}
	if _, still := sc.Data["pitfalls"].([]any); !still {
		t.Error("caller's data map was mutated")
	}
	if out.Data["other"] != "kept" {
		t.Error("unrelated data keys must carry over")
	}

	// null list: degrades to ""
	out, err = pitfallsTransform(config.Sidecar{Data: map[string]any{"pitfalls": nil}})
	if err != nil || out.Data["pitfalls"] != "" {
		t.Errorf("null pitfalls must render empty, got %v / %v", out.Data["pitfalls"], err)
	}

	// structural error propagates
	if _, err := pitfallsTransform(config.Sidecar{Data: map[string]any{"pitfalls": "bad"}}); err == nil || !strings.Contains(err.Error(), "must be a list") {
		t.Errorf("structural error must propagate, got: %v", err)
	}
}

// Absent, empty, and null data all degrade to the coherent placeholder; a valid
// list renders as ## sections with the Domains line.
func TestPitfallsRenderDegradesAndRenders(t *testing.T) {
	for name, files := range map[string]map[string]string{
		"no-sidecar": nil,
		"empty-list": {"docs/pitfalls.yaml": "data:\n  pitfalls: []\n"},
		"null-list":  {"docs/pitfalls.yaml": "data:\n  pitfalls:\n"},
	} {
		t.Run(name, func(t *testing.T) {
			out := renderPitfalls(t, scaffoldFiles(t, pitfallsCfg, files))
			if !strings.Contains(out, "No pitfalls recorded yet") || !strings.Contains(out, "data.pitfalls") {
				t.Errorf("missing placeholder line:\n%s", out)
			}
		})
	}
	out := renderPitfalls(t, scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: Only\n      domains: [rendering]\n      body: |\n        the body\n",
	}))
	if !strings.Contains(out, "## Only") || !strings.Contains(out, "_Domains: rendering_") || !strings.Contains(out, "the body") {
		t.Errorf("entry not rendered:\n%s", out)
	}
}

// A content violation fails the render naming the sidecar path.
func TestPitfallsRenderContentViolation(t *testing.T) {
	p, err := Open(scaffoldFiles(t, pitfallsCfg, map[string]string{
		"docs/pitfalls.yaml": "data:\n  pitfalls:\n    - title: T\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.RenderAll(); err == nil || !strings.Contains(err.Error(), pitfallsSidecarPath) || !strings.Contains(err.Error(), `missing "body"`) {
		t.Errorf("want render error naming the sidecar, got: %v", err)
	}
}
