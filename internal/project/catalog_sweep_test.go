package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hypnotox/agentic-workflows/internal/catalog"
	"github.com/hypnotox/agentic-workflows/internal/config"
	"github.com/hypnotox/agentic-workflows/internal/render"
	"github.com/hypnotox/agentic-workflows/templates"
)

// skillRefRe matches a rendered example-prefixed skill reference. Greedy, so
// the longest hyphenated token wins ("example-reviewing-plan-resync" never
// reports a nested "example-reviewing-plan").
var skillRefRe = regexp.MustCompile(`example-[a-z][a-z-]*[a-z]`)

// doubleBacktickRe matches a double backtick not adjacent to a third - an
// empty inline-code span or a literal double-backtick quoting span, never a
// triple-backtick code fence. (Spelled out because gofmt rewrites a literal
// double-backtick pair in a doc comment into a curly quote.)
var doubleBacktickRe = regexp.MustCompile("(^|[^`])``([^`]|$)")

// doubleBacktickExempt lists templates whose double-backtick spans are
// deliberate; entries fail when stale (ADR-0080 Decision 7). No skill or agent
// template renders a double-backtick span under the current-state authority
// model; the map stays declared so a future deliberate span registers here.
var doubleBacktickExempt = map[string]bool{}

// TestCatalogTemplatesDegradeLeakFree renders every catalog skill and agent
// template under empty adopter data (full awf-given layout, RequiresDoc doc
// seeded) and fails on leak residue, on skill-reference residue outside the
// artifact's RequiresSkills declaration, and on stale declarations or
// exemptions. The artifact set derives from catalog.Standard, never a hand
// list (ADR-0080).
// invariant: rendering/templates:catalog-template-sweep (TestCatalogTemplatesDegradeLeakFree)
func TestCatalogTemplatesDegradeLeakFree(t *testing.T) {
	assertV3ADRTemplatePublicationSafe(t)
	cat := catalog.Standard
	sweep := func(tid, requiresDoc string) {
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
			// invariant: rendering/catalog-and-targets:requires-skills-exact (TestCatalogTemplatesDegradeLeakFree)
			found := map[string]bool{}
			for _, m := range skillRefRe.FindAllString(out, -1) {
				name := strings.TrimPrefix(m, "example-")
				if _, ok := cat.Skills[name]; !ok {
					continue // prose or section-name token, not a skill reference
				}
				found[name] = true
				// Workflow-profile neighbors are advisory and are not structural
				// requirements. Only artifact references declared in RequiresSkills
				// are checked by the catalog sweep.
			}
			// Standard workflow relationships are intentionally not required to
			// appear as unconditional rendered references.
			hasDouble := doubleBacktickRe.MatchString(out)
			if hasDouble && !doubleBacktickExempt[tid] {
				t.Errorf("double-backtick span rendered under empty data - fix the template or add a doubleBacktickExempt entry:\n%s", out)
			}
			if !hasDouble && doubleBacktickExempt[tid] {
				t.Errorf("stale doubleBacktickExempt entry - the template no longer renders a double-backtick span")
			}
		})
	}
	for name, spec := range cat.Skills {
		sweep(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name), spec.RequiresDoc)
	}
	for name := range cat.Agents {
		sweep(fmt.Sprintf("agents/%s.md.tmpl", name), "")
	}
}

// conditionalActionRe matches any template conditional carrying fallback
// prose: if, with, or range actions (with/else is the dominant form).
var conditionalActionRe = regexp.MustCompile(`\{\{-?\s*(if|with|range)\b`)

// TestConditionalTemplatesHaveFallbackCases requires a hand-authored
// unset-data case for every catalog template whose post-include-expansion
// source contains a conditional action - only a human knows what the degraded
// prose should say, so its presence is machine-forced (ADR-0080 Decision 3).
// invariant: rendering/templates:conditional-fallback-case-guard (TestConditionalTemplatesHaveFallbackCases)
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
			t.Errorf("%s has conditional fallback prose but no unsetFallbackCases entry - add a hand-authored case pinning its degraded output", tid)
		}
	}
	for name := range catalog.Standard.Skills {
		check(fmt.Sprintf("skills/%s/SKILL.md.tmpl", name))
	}
	for name := range catalog.Standard.Agents {
		check(fmt.Sprintf("agents/%s.md.tmpl", name))
	}
}

var singletonConditionalPathRe = regexp.MustCompile(`\{\{-?\s*(?:if|with|range)\s+\.([A-Za-z][A-Za-z0-9_]*)(?:\.([A-Za-z][A-Za-z0-9_]*))?`)

func cloneRenderData(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			copy := make(map[string]any, len(nested))
			for nestedKey, nestedValue := range nested {
				copy[nestedKey] = nestedValue
			}
			out[key] = copy
			continue
		}
		out[key] = value
	}
	return out
}

// TestSingletonConditionalKeysUseLiveRenderContext derives the conditional
// config-tree singleton population from conditionalUnits, extracts every
// direct condition path from the shipped templates, proves that path belongs
// to the real render data authority, and renders both outcomes.
// invariant: rendering/templates:singleton-conditional-key-live (TestSingletonConditionalKeysUseLiveRenderContext)
func TestSingletonConditionalKeysUseLiveRenderContext(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := Open(testContext(t), root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, eff, err := p.deriveOperationState()
	if err != nil {
		t.Fatal(err)
	}
	base := p.data(config.Sidecar{}, eff)
	seenTemplates, seenConditions := 0, 0
	for _, unit := range conditionalUnits() {
		raw, err := fs.ReadFile(templates.FS, unit.tid)
		if err != nil {
			t.Fatal(err)
		}
		expanded, err := render.ExpandIncludes(string(raw), templates.FS)
		if err != nil {
			t.Fatal(err)
		}
		matches := singletonConditionalPathRe.FindAllStringSubmatch(expanded, -1)
		if len(matches) == 0 {
			continue
		}
		seenTemplates++
		seen := map[string]bool{}
		for _, match := range matches {
			path := match[1]
			if match[2] != "" {
				path += "." + match[2]
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			seenConditions++

			rootValue, rootExists := base[match[1]]
			if !rootExists {
				t.Errorf("%s conditional %s has no root on the real render context", unit.tid, path)
				continue
			}
			if match[2] != "" {
				if _, ok := rootValue.(map[string]any); !ok {
					t.Errorf("%s conditional %s traverses non-map render data", unit.tid, path)
					continue
				}
				declared := false
				for _, descriptor := range catalog.Standard.Vars {
					if descriptor.Key == match[2] {
						declared = true
						break
					}
				}
				if !declared {
					t.Errorf("%s conditional %s names no declared config var", unit.tid, path)
					continue
				}
			}

			zero, set := cloneRenderData(base), cloneRenderData(base)
			if zeroVars, ok := zero["vars"].(map[string]any); ok {
				setVars := set["vars"].(map[string]any)
				for key := range zeroVars {
					zeroVars[key], setVars[key] = "", ""
				}
			}
			if match[2] == "" {
				zero[match[1]], set[match[1]] = false, true
			} else {
				zeroVars := zero[match[1]].(map[string]any)
				setVars := set[match[1]].(map[string]any)
				zeroVars[match[2]], setVars[match[2]] = "", "fixture-value"
			}
			if without, with := renderGolden(t, unit.tid, zero), renderGolden(t, unit.tid, set); without == with {
				t.Errorf("%s conditional %s did not exercise distinct outcomes", unit.tid, path)
			}
		}
	}
	if seenTemplates == 0 || seenConditions == 0 {
		t.Fatalf("conditional singleton census was vacuous: templates=%d conditions=%d", seenTemplates, seenConditions)
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
// agent - the goldens live in spine_test.go by convention (source-scan
// mechanic, precedent TestArchitectureDocNamesEveryCmd; ADR-0080 Decision 4).
// invariant: rendering/templates:golden-test-completeness (TestEveryCatalogArtifactHasGoldenTest)
func TestEveryCatalogArtifactHasGoldenTest(t *testing.T) {
	src, err := os.ReadFile("spine_test.go")
	if err != nil {
		t.Fatalf("read spine_test.go: %v", err)
	}
	for name := range catalog.Standard.Skills {
		if needle := "func Test" + kebabToCamel(name) + "Template("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for skill %q - add %s to internal/project/spine_test.go", name, needle)
		}
	}
	for name := range catalog.Standard.Agents {
		if needle := "func Test" + kebabToCamel(name) + "Agent("; !strings.Contains(string(src), needle) {
			t.Errorf("no golden test for agent %q - add %s to internal/project/spine_test.go", name, needle)
		}
	}
}

// goldenFuncRe matches a golden-shaped test declaration in spine_test.go:
// Test<Stem>Template or Test<Stem>Agent with the suffix directly before the
// parenthesis (TestAgentsDocTemplateConfigDriven is not golden-shaped).
var goldenFuncRe = regexp.MustCompile(`func Test([A-Za-z0-9]+)(Template|Agent)\(`)

// nonArtifactGoldens lists the golden-shaped Template test stems in
// spine_test.go that test non-catalog artifacts (doc singletons); entries
// fail when stale (ADR-0080 Decision 7).
var nonArtifactGoldens = map[string]bool{
	"DocArchitecture":   true,
	"Glossary":          true,
	"RoadmapGraduation": true,
}

// TestNoOrphanGoldenTest is the reverse of TestEveryCatalogArtifactHasGoldenTest:
// every golden-shaped test func in spine_test.go must name a current catalog
// artifact, so a golden orphaned by a catalog removal fails here even while
// its lingering .tmpl file keeps it rendering.
func TestNoOrphanGoldenTest(t *testing.T) {
	src, err := os.ReadFile("spine_test.go")
	if err != nil {
		t.Fatalf("read spine_test.go: %v", err)
	}
	skills, agents := map[string]bool{}, map[string]bool{}
	for name := range catalog.Standard.Skills {
		skills[kebabToCamel(name)] = true
	}
	for name := range catalog.Standard.Agents {
		agents[kebabToCamel(name)] = true
	}
	seenExempt := map[string]bool{}
	for _, m := range goldenFuncRe.FindAllStringSubmatch(string(src), -1) {
		stem, kind := m[1], m[2]
		switch {
		case kind == "Template" && nonArtifactGoldens[stem]:
			seenExempt[stem] = true
		case kind == "Template" && !skills[stem]:
			t.Errorf("orphan golden Test%sTemplate: no catalog skill matches - remove it or list it in nonArtifactGoldens", stem)
		case kind == "Agent" && !agents[stem]:
			t.Errorf("orphan golden Test%sAgent: no catalog agent matches - remove it", stem)
		}
	}
	for stem := range nonArtifactGoldens {
		if !seenExempt[stem] {
			t.Errorf("stale nonArtifactGoldens entry %q: no such golden-shaped func in spine_test.go", stem)
		}
	}
}
