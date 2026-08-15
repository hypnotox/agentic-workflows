package project

import (
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// This deliberately recognizes execution directives rather than every bare
// "awf" token. Bare product names, CLI grammar, historical descriptions, and
// wrapper/PATH resolution are the explicit allowances in ADR-unconditional-
// repository-awf-invocation; none directs a repository-local execution.
var bareRepositoryAwfExecution = regexp.MustCompile(`(?i)(?:\b(?:run|runs|use|execute|invoke|calls?)\s+(?:the\s+)?` + "`awf\\b" + `|\brun awf\b)`)

func invocationRenderData() map[string]any {
	layout := testLayout()
	layout["docs"] = map[string]any{"roadmap": "docs/roadmap.md"}
	return map[string]any{
		"prefix": "example", "vars": map[string]any{}, "layout": layout,
		"data": map[string]any{}, "skills": map[string]bool{},
	}
}

func withInvocationData(data map[string]any, value map[string]any) map[string]any {
	copy := make(map[string]any, len(data))
	for key, item := range data {
		copy[key] = item
	}
	copy["data"] = value
	return copy
}

func withInvocationTarget(data map[string]any, target Target) map[string]any {
	copy := withInvocationData(data, data["data"].(map[string]any))
	for key, value := range target.targetTemplateData() {
		copy[key] = value
	}
	return copy
}

// renderInvocationSurface deliberately permits literal template syntax that is
// documented as syntax in catalog docs; invocation checking does not own the
// general no-token rendering oracle.
func renderInvocationSurface(t *testing.T, template string, data map[string]any) string {
	t.Helper()
	src, err := fs.ReadFile(templates.FS, template)
	if err != nil {
		t.Fatal(err)
	}
	withLayoutDefaults(data)
	expanded, err := render.ExpandIncludes(string(src), templates.FS)
	if err != nil {
		t.Fatal(err)
	}
	assembled, parts := assemble(parseSections(expanded), nil, render.HTMLComment)
	body, err := render.Execute(assembled, data, parts, "repository awf invocation")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func assertNoBareRepositoryAwfExecution(t *testing.T, surface, body string) {
	t.Helper()
	// This historical description reports an already-shipped hook change; it is
	// not an instruction to the reader.
	body = strings.ReplaceAll(body, "2026-07-15 to also run `awf check`", "historical check invocation")
	if bare := bareRepositoryAwfExecution.FindString(body); bare != "" {
		t.Errorf("%s renders bare repository execution directive %q", surface, bare)
	}
}

// Every rendered adopter- or agent-facing execution directive invokes the
// repository wrapper. The sweep covers each catalog class rather than a
// hand-curated subset, so a newly cataloged surface inherits this proof.
// invariant: rendering/workflow-skill-templates:repository-awf-invocation (TestRepositoryAwfInvocation)
func TestRepositoryAwfInvocation(t *testing.T) {
	data := invocationRenderData()

	guide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", withInvocationTarget(data, piTarget))
	assertNoBareRepositoryAwfExecution(t, "AGENTS guide", guide)

	for _, target := range []Target{claudeTarget, piTarget} {
		targetData := withInvocationTarget(data, target)
		for name, skill := range catalog.Standard.Skills {
			assertNoBareRepositoryAwfExecution(t, target.Name+" skill "+name, renderSkillGolden(t, name, withInvocationData(targetData, skill.Data)))
		}
		for name, agent := range catalog.Standard.Agents {
			assertNoBareRepositoryAwfExecution(t, target.Name+" agent "+name, renderAgentGolden(t, name, withInvocationData(targetData, agent.Data)))
		}
		for name, doc := range catalog.Standard.Docs {
			if doc.TID == "" {
				continue
			}
			assertNoBareRepositoryAwfExecution(t, target.Name+" catalog doc "+name, renderInvocationSurface(t, doc.TID, withInvocationData(targetData, doc.Data)))
		}
	}
	for _, template := range []string{
		"hooks/commit-msg.sh.tmpl", "hooks/pre-commit.sh.tmpl", "hooks/pre-merge-commit.sh.tmpl",
		"hooks/pre-push.sh.tmpl", "hooks/reference-transaction.sh.tmpl",
	} {
		assertNoBareRepositoryAwfExecution(t, template, renderGolden(t, template, data))
	}

	pi := renderPiExtensionFile(t, "awf-subagents/index.ts")
	for _, want := range []string{
		"has no instruction body; run ./awf render.",
		"Enable the matching ${c.args.kind}-reviewer agent and run ./awf render.",
		"Enable the implementer agent and run ./awf render.",
		"Enable the explorer agent and run ./awf render.",
		"Enable the grounding-checker agent and run ./awf render.",
	} {
		if !strings.Contains(pi, want) {
			t.Errorf("Pi runtime lacks repository wrapper repair %q", want)
		}
	}
}

// invariant: rendering/guide-and-doc-templates:guide-awf-invocation (TestGuideAwfInvocation)
func TestGuideAwfInvocation(t *testing.T) {
	guide := renderGolden(t, "agents-doc/AGENTS.md.tmpl", withInvocationTarget(invocationRenderData(), piTarget))
	assertNoBareRepositoryAwfExecution(t, "AGENTS guide", guide)
	for _, want := range []string{"run `./awf render` and `./awf check`", "run `./awf check staged`"} {
		if !strings.Contains(guide, want) {
			t.Errorf("guide lacks repository wrapper command %q", want)
		}
	}
}
