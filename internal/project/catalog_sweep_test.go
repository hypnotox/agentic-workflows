package project

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
)

// skillRefRe matches a rendered example-prefixed skill reference. Greedy, so
// the longest hyphenated token wins ("example-reviewing-plan-resync" never
// reports a nested "example-reviewing-plan").
var skillRefRe = regexp.MustCompile(`example-[a-z][a-z-]*[a-z]`)

// doubleBacktickRe matches a double backtick not adjacent to a third — an
// empty inline-code span or a literal “…“ quoting span, never a ``` fence.
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
