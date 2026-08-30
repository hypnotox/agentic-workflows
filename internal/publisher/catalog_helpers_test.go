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
		tmpl: "agents/grounding-checker.md.tmpl",
		want: []string{"Ground guide-first, in order", "Current-state documentation is what binds"},
	},
	{
		tmpl: "agents/implementer.md.tmpl",
		want: []string{"the project's gate command"},
		ban:  []string{"Shortcuts that are never acceptable here", "``"},
	},
	{
		tmpl: "skills/executing-direct/SKILL.md.tmpl",
		want: []string{"Evaluate review independently", "locally obvious, low-risk, directly verified", "the effort lifecycle owner"},
		ban:  []string{"awf_workflow", "example-effort-workflow"},
	},
	{
		tmpl: "skills/tdd/SKILL.md.tmpl",
		want: []string{
			"Pick the smallest surface that can prove the behaviour",
			"confirm it fails for the right reason.",
			"Run the gate.",
		},
		ban: []string{"``"},
	},
	{
		tmpl: "skills/bugfix/SKILL.md.tmpl",
		want: []string{
			"confirm it with a falsifiable check before touching code",
			"Exercise the selected evidence against the unfixed behaviour",
			"The project's gate is the default",
			"the project's docs",
			"Evaluate implementation review independently",
			"the project's review step",
			"the effort lifecycle owner",
		},
		ban: []string{"example-tdd", "example-debugging", "example-reviewing-impl", "``"},
	},
	{
		tmpl: "skills/debugging/SKILL.md.tmpl",
		want: []string{
			"apply it directly under the durable-oracle rule in that case",
			"Apply the durable-oracle rule above directly.",
			"the project's gate",
			"apply the fix under the durable-oracle rule",
			"a design discussion before changing behaviour",
		},
		ban: []string{"example-bugfix", "example-tdd", "example-brainstorming", "``"},
	},
	{
		tmpl: "skills/exploring/SKILL.md.tmpl",
		want: []string{"target-native fresh-context exploration subagent"},
		ban:  []string{"subagent_explore"},
	},
	{
		tmpl: "skills/orienting/SKILL.md.tmpl",
		want: []string{"Ground guide-first, in order", "`example-exploring`"},
	},
	{
		tmpl: "skills/refactor-coupling-audit/SKILL.md.tmpl",
		want: []string{"<module-prefix>/", "the project's decision process"},
		ban:  []string{"example-proposing-adr"},
	},
	{
		// Every conditional rung/reference degrades to generic prose when its
		// skill/var/doc is absent - no empty inline code, no dangling reference
		// (ADR-0045/ADR-0020 publication-safety; ADR-0067 rung-4 pitfalls obligation).
		tmpl: "skills/retrospective/SKILL.md.tmpl",
		want: []string{
			"the effort lifecycle owner",
			"the project's pitfalls notes",
			"the project's decision process",
			"Run `./awf new pitfall \"<Title>\"`, then edit the reported authored source under `.awf/docs/pitfalls/`",
		},
		ban: []string{"example-reviewing-impl", "example-proposing-adr", "``"},
	},
	// invariant: rendering/workflow-skill-templates:reviewers-report-only (agents/adr-reviewer.md.tmpl)
	{
		tmpl: "agents/adr-reviewer.md.tmpl",
		want: []string{"post-implementation", "counterfactual", "reasoned finding", "unordered membership within each batch"},
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits", "Edit the", "Apply a fix", "Commit the change", "Loop a re-review"},
	},
	{
		tmpl: "agents/code-reviewer.md.tmpl",
		ban:  []string{"For each item below", "Apply mechanical and reasoned fixes directly", "apply the fix directly", "3-round soft cap", "as new commits", "Edit the", "Apply a fix", "Commit the change", "Loop a re-review"},
	},
	{
		tmpl: "agents-doc/AGENTS.md.tmpl",
		want: []string{"Conventional Commits; one concern per commit."},
		ban:  []string{"Chain skills", "Task skills", "example-brainstorming"},
	},
	{
		tmpl: "skills/adr-lifecycle/SKILL.md.tmpl",
		want: []string{"the multi-state lifecycle", "Run `./awf render` to regenerate", "whose members may appear in any order", "settled review later appends only Implemented"},
	},
	{
		tmpl: "skills/brainstorming/SKILL.md.tmpl",
		want: []string{"material choice or clarification", "does not create an effort"},
	},
	{
		tmpl: "skills/grounding/SKILL.md.tmpl",
		want: []string{"broad or uncertain repository premises", "Dispatch the `grounding-checker` agent exactly once"},
	},
	{
		tmpl: "skills/effort-workflow/SKILL.md.tmpl",
		want: []string{"sole owner of the effort lifecycle", "Continue autonomously or through a target-native successor"},
	},
	{
		tmpl: "skills/proposing-adr/SKILL.md.tmpl",
		want: []string{"follow the ADR template's section order", "Run `./awf render` to regenerate"},
	},
	{
		tmpl: "skills/reviewing-adr/SKILL.md.tmpl",
		want: []string{
			"using the project's commit scope conventions",
			"exactly one fresh `adr-reviewer` verify pass",
		},
	},
	{
		tmpl: "skills/reviewing-impl/SKILL.md.tmpl",
		want: []string{"locally obvious, low-risk, directly verified", "Effort-free review creates no effort"},
	},
	{
		tmpl: "skills/roadmap-graduation/SKILL.md.tmpl",
		docs: map[string]any{"roadmap": "docs/roadmap.md"},
		want: []string{
			"Write the ADR per the project's decision process.",
			"moving an item out of `docs/roadmap.md`",
		},
		ban: []string{"example-proposing-adr"},
	},
	// Voluntary doc entry (ADR-0089): the ADR-0080 guard covers skills and
	// agents only, so the glossary's conditional is pinned by hand.
	{
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
