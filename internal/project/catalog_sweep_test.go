package project

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// skillRefRe matches a rendered example-prefixed skill reference. Greedy, so
// the longest hyphenated token wins ("example-reviewing-plan-resync" never
// reports a nested "example-reviewing-plan").
var skillRefRe = regexp.MustCompile(`example-[a-z][a-z-]*[a-z]`)

// doubleBacktickRe matches a double backtick not adjacent to a third — an
// empty inline-code span or a literal double-backtick quoting span, never a
// triple-backtick code fence. (Spelled out because gofmt rewrites a literal
// double-backtick pair in a doc comment into a curly quote.)
var doubleBacktickRe = regexp.MustCompile("(^|[^`])``([^`]|$)")

// doubleBacktickExempt lists templates whose double-backtick spans are
// deliberate; entries fail when stale (ADR-0080 Decision 7). proposing-adr
// quotes nested backticks in its inv-slug authoring guidance.
var doubleBacktickExempt = map[string]bool{
	"skills/proposing-adr/SKILL.md.tmpl": true,
}

// TestCatalogTemplatesDegradeLeakFree renders every catalog skill and agent
// template under empty adopter data (full awf-given layout, RequiresDoc doc
// seeded) and fails on leak residue, on skill-reference residue outside the
// artifact's RequiresSkills declaration, and on stale declarations or
// exemptions. The artifact set derives from catalog.Standard, never a hand
// list (ADR-0080).
// invariant: catalog-template-sweep
func TestCatalogTemplatesDegradeLeakFree(t *testing.T) {
	cat := catalog.Standard
	sweep := func(tid, self, requiresDoc string, requiresSkills []string) {
		t.Run(tid, func(t *testing.T) {
			layout := testLayout()
			if requiresDoc != "" {
				layout["docs"] = map[string]any{requiresDoc: "docs/" + requiresDoc + ".md"}
			}
			data := map[string]any{
				"prefix": "example",
				"vars":   map[string]any{},
				"data":   map[string]any{},
				"skills": map[string]bool{},
				"layout": layout,
			}
			out := renderGolden(t, tid, data)
			// Declarations are exact: undeclared reference residue and stale
			// RequiresSkills entries both fail (ADR-0080 Decision 2).
			// invariant: requires-skills-exact
			found := map[string]bool{}
			for _, m := range skillRefRe.FindAllString(out, -1) {
				name := strings.TrimPrefix(m, "example-")
				if _, ok := cat.Skills[name]; !ok {
					continue // prose or section-name token, not a skill reference
				}
				found[name] = true
				if name != self && !slices.Contains(requiresSkills, name) {
					t.Errorf("undeclared unconditional reference %q — guard it behind .skills.%s or declare it in RequiresSkills", m, name)
				}
			}
			for _, r := range requiresSkills {
				if !found[r] {
					t.Errorf("stale RequiresSkills entry %q — no longer referenced unconditionally; remove the declaration", r)
				}
			}
			hasDouble := doubleBacktickRe.MatchString(out)
			if hasDouble && !doubleBacktickExempt[tid] {
				t.Errorf("double-backtick span rendered under empty data — fix the template or add a doubleBacktickExempt entry:\n%s", out)
			}
			if !hasDouble && doubleBacktickExempt[tid] {
				t.Errorf("stale doubleBacktickExempt entry — the template no longer renders a double-backtick span")
			}
		})
	}
	for name, spec := range cat.Skills {
		sweep(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), name, spec.RequiresDoc, spec.RequiresSkills)
	}
	for name, spec := range cat.Agents {
		sweep(fmt.Sprintf("agents/%s.md.tmpl", name), "", "", spec.RequiresSkills)
	}
}

// conditionalActionRe matches any template conditional carrying fallback
// prose: if, with, or range actions (with/else is the dominant form).
var conditionalActionRe = regexp.MustCompile(`\{\{-?\s*(if|with|range)\b`)

// TestConditionalTemplatesHaveFallbackCases requires a hand-authored
// unset-data case for every catalog template whose post-include-expansion
// source contains a conditional action — only a human knows what the degraded
// prose should say, so its presence is machine-forced (ADR-0080 Decision 3).
// invariant: conditional-fallback-case-guard
func TestConditionalTemplatesHaveFallbackCases(t *testing.T) {
	covered := map[string]bool{}
	for _, tc := range unsetFallbackCases {
		covered[tc.tmpl] = true
	}
	check := func(tid string) {
		src, err := fs.ReadFile(templates.FS, tid)
		if err != nil {
			t.Fatalf("read %s: %v", tid, err)
		}
		expanded, err := render.ExpandIncludes(string(src), templates.FS)
		if err != nil {
			t.Fatalf("expand %s: %v", tid, err)
		}
		if conditionalActionRe.MatchString(expanded) && !covered[tid] {
			t.Errorf("%s has conditional fallback prose but no unsetFallbackCases entry — add a hand-authored case pinning its degraded output", tid)
		}
	}
	for name := range catalog.Standard.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name))
	}
	for name := range catalog.Standard.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name))
	}
}

// kebabToCamel converts a kebab-case artifact name to its test-func stem
// ("subagent-driven-development" → "SubagentDrivenDevelopment").
func kebabToCamel(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}

// TestEveryCatalogArtifactHasGoldenTest asserts a per-artifact golden test
// func exists in this package's test source for every catalog skill and
// agent — the goldens live in spine_test.go by convention (source-scan
// mechanic, precedent TestArchitectureDocNamesEveryCmd; ADR-0080 Decision 4).
// invariant: golden-test-completeness
func TestEveryCatalogArtifactHasGoldenTest(t *testing.T) {
	src, err := os.ReadFile("spine_test.go")
	if err != nil {
		t.Fatalf("read spine_test.go: %v", err)
	}
	for name := range catalog.Standard.Skills {
		if needle := "func Test" + kebabToCamel(name) + "Template("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for skill %q — add %s to internal/project/spine_test.go", name, needle)
		}
	}
	for name := range catalog.Standard.Agents {
		if needle := "func Test" + kebabToCamel(name) + "Agent("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for agent %q — add %s to internal/project/spine_test.go", name, needle)
		}
	}
}
