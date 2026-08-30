package publisher

import (
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/manifest"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/internal/resident"
	"github.com/hypnotox/agentic-workflows/templates"
)

func testLayout() map[string]any {
	return map[string]any{
		"docsDir": "docs", "adrDir": "docs/decisions", "indexMd": "docs/decisions/INDEX.md", "adrReadme": "docs/decisions/README.md", "adrTemplate": "docs/decisions/template.md", "docs": map[string]any{}, "workflowRef": "docs/workflow.md", "docStandard": "docs/doc-standard.md", "agentsMdStandard": "docs/agents-md-standard.md", "workingWithAwf": "docs/working-with-awf.md", "maintainableCodeDesign": "docs/maintainable-code-design.md", "configReference": "docs/config-reference.md", "domainsDir": "docs/domains",
	}
}
func parseSections(src string, markdown ...bool) []render.Segment {
	return render.ParseSourceSections(render.SourceText{Spans: []render.SourceSpan{{Text: src}}}, markdown...)
}
func assemble(segs []render.Segment, plan map[string]render.SectionPlan, style render.CommentStyle) (string, map[string]string) {
	assembled, parts := render.AssembleSourceWithTemplateSource(segs, plan, style, render.TemplateSource{})
	return assembled.AuthoredText(), parts
}
func renderGolden(t *testing.T, path string, data map[string]any) string {
	t.Helper()
	withLayoutDefaults(data)
	src, err := fs.ReadFile(templates.FS, path)
	if err != nil {
		t.Fatal(err)
	}
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	asm, parts := assemble(parseSections(expanded), nil, render.HTMLComment)
	out, err := render.Execute(asm, data, parts, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, residue := range []string{"<no value>", "%!"} {
		if strings.Contains(out, residue) {
			t.Fatalf("render residue %q in %s", residue, path)
		}
	}
	return out
}
func assertV3ADRTemplatePublicationSafe(t *testing.T) {
	out := renderGolden(t, "adr-template/template.md.tmpl", map[string]any{"prefix": "example", "vars": map[string]any{}, "data": map[string]any{}, "skills": map[string]bool{}, "layout": testLayout()})
	if !strings.Contains(out, "format: current-state-v4") || !strings.Contains(out, "- YYYY-MM-DD: Proposed") {
		t.Fatalf("V4 lifecycle example is not publication-safe:\n%s", out)
	}
}

type fallbackCase struct {
	tmpl string
	docs map[string]any
	want []string // fallback prose that must render
	ban  []string // residue that must not render
}

var unsetFallbackCases = []fallbackCase{
	{
		// The glossary has an intentionally empty-data publication fallback.
		tmpl: "docs/glossary.md.tmpl",
		want: []string{"No terms recorded yet", "`data.terms` in `.awf/docs/glossary.yaml`"},
		ban:  []string{"| Term | Meaning |"},
	},
}

// The unset-data half of the implementer contract's third sentence: its
// fallback case above pins the degraded body.

func checkLockedFiles(roots resident.Roots, lock *manifest.Lock, rendered map[string]RenderedFile, _ []manifest.Drift) []manifest.Drift {
	var drift []manifest.Drift
	for path, entry := range lock.Files {
		file, ok := rendered[path]
		if !ok {
			drift = append(drift, manifest.Drift{Path: path, Kind: "orphaned", Detail: "in lock but no longer produced"})
			continue
		}
		if file.Policy.Regenerate {
			onDisk, err := os.ReadFile(roots.ResolveOutput(path))
			if err != nil {
				drift = append(drift, manifest.Drift{Path: path, Kind: "missing", Detail: "file absent; run awf render"})
				continue
			}
			if manifest.Hash(onDisk) != manifest.Hash([]byte(file.Content)) {
				drift = append(drift, manifest.Drift{Path: path, Kind: "hand-edited", Detail: "on-disk output differs from the regenerated file; run awf render to restore awf-owned regions"})
			}
			continue
		}
		if manifest.Hash([]byte(file.Content)) != entry.OutputHash {
			drift = append(drift, manifest.Drift{Path: path, Kind: "stale", Detail: "rendered output out of date; run awf render"})
		}
	}
	return drift
}
