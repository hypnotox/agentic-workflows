# 2026-07-09 — Catalog-derived test coverage

**Goal:** implement [ADR-0080](../decisions/0080-catalog-derived-test-coverage-for-skill-and-agent-templates.md)
(catalog-declared skill coupling via `RequiresSkills`, a derived unset-data sweep over every
catalog template, a conditional-fallback case guard, a golden-completeness guard, and a
catalog-derived chain-closure fixture), closing the `docs/pitfalls.md` "hand-enumerated test
touch points" entry. Design rationale lives in the ADR — not duplicated here.

**Architecture summary:** production change is one catalog field (`RequiresSkills` on
`SkillSpec`/`TargetSpec`) plus its declarations in `catalog.Standard`; everything else is
test code in `internal/project` and `internal/catalog`, and docs. The sweep and guards live in
a new `internal/project/catalog_sweep_test.go`; the hand case list in `spine_test.go` is
hoisted to a package-level var so the guard can introspect it. Exemptions follow ADR-0080
Decision 7: default inclusion, explicit entries, stale entries fail. All reference lists and
degraded-output phrases below were derived empirically on 2026-07-09 by rendering each
template under empty adopter data (`prefix: example`, empty `vars`/`data`/`skills`, full
`testLayout()`); if a want-phrase mismatches at execution time, re-derive it from the actual
unset render — the assertion pins degraded prose, not a guess.

**Tech stack:** Go 1.26; stdlib only (`regexp`, `slices`, `sort`, `strings`, `io/fs`, `os`).
Packages touched: `internal/catalog` (field + declarations + validation test),
`internal/project` (tests only), `changelog/`, `.awf/` parts.

**File structure:**

- Created: `internal/project/catalog_sweep_test.go`,
  `docs/plans/2026-07-09-catalog-derived-test-coverage.md` (this plan)
- Modified: `internal/catalog/catalog.go`, `internal/catalog/standard.go`,
  `internal/catalog/catalog_test.go`, `internal/project/spine_test.go`,
  `internal/project/drift_test.go`, `changelog/CHANGELOG.md`,
  `.awf/docs/parts/pitfalls/entries.md`, `.awf/agents-doc.yaml`,
  `.awf/domains/parts/rendering/current-state.md`,
  `docs/decisions/0080-catalog-derived-test-coverage-for-skill-and-agent-templates.md`
  (status flip), plus rendered files refreshed by `./x sync` (`AGENTS.md`, `CLAUDE.md`
  bridge targets, `docs/pitfalls.md`, `docs/domains/rendering.md`,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`)
- Deleted: none

---

## Phase 1 — `RequiresSkills` catalog field, declarations, validation

- [ ] In `internal/catalog/catalog.go`, add the field to `TargetSpec` (after the `Base` field):

      ```go
      	// RequiresSkills names the catalog skills this artifact's template references
      	// unconditionally — rendered into its output even when the referenced skill is
      	// not enabled (deliberate chain coupling; the agent guide's "disable them as a
      	// unit"). Declarations are exact: the template test sweep fails on an
      	// undeclared unconditional reference AND on a stale entry (ADR-0080). Data,
      	// not gated validation — promoting it to add/remove pairing UX is deferred.
      	RequiresSkills []string `yaml:"requiresSkills"`
      ```

- [ ] In the same file, add to `SkillSpec` (after the `Base` field, before `Data`):

      ```go
      	// RequiresSkills: see TargetSpec.RequiresSkills (ADR-0080).
      	RequiresSkills []string `yaml:"requiresSkills"`
      ```

- [ ] In `internal/catalog/standard.go`, add `RequiresSkills` to exactly these entries
      (lists are sorted; derived from the empty-data render on 2026-07-09):

      | Entry | Declaration |
      |---|---|
      | skill `brainstorming` | `RequiresSkills: []string{"proposing-adr", "reviewing-adr", "reviewing-impl", "writing-plans"},` |
      | skill `executing-plans` | `RequiresSkills: []string{"reviewing-impl", "subagent-driven-development"},` |
      | skill `proposing-adr` | `RequiresSkills: []string{"adr-lifecycle", "reviewing-adr"},` |
      | skill `reviewing-adr` | `RequiresSkills: []string{"adr-lifecycle", "executing-plans", "proposing-adr", "reviewing-plan-resync", "subagent-driven-development", "writing-plans"},` |
      | skill `reviewing-impl` | `RequiresSkills: []string{"executing-plans", "retrospective", "subagent-driven-development"},` |
      | skill `reviewing-plan` | `RequiresSkills: []string{"reviewing-plan-resync", "writing-plans"},` |
      | skill `reviewing-plan-resync` | `RequiresSkills: []string{"executing-plans", "reviewing-adr", "reviewing-plan", "subagent-driven-development"},` |
      | skill `subagent-driven-development` | `RequiresSkills: []string{"executing-plans", "reviewing-impl"},` |
      | skill `writing-plans` | `RequiresSkills: []string{"adr-lifecycle", "proposing-adr", "reviewing-plan", "reviewing-plan-resync"},` |
      | agent `plan-reviewer` | `RequiresSkills: []string{"reviewing-plan-resync"},` |

      All other skills, agents, and `DomainDoc` stay undeclared (empty).

- [ ] In `internal/catalog/catalog_test.go`, add beside `TestReviewingSkillSpecsArePaired`:

      ```go
      // TestRequiresSkillsDeclarationsValid rejects a RequiresSkills entry naming a
      // non-catalog skill or the artifact itself, and any RequiresSkills outside the
      // skills and agents maps — the domain-doc spec shares TargetSpec and the field
      // is meaningless there; a silent no-op would invite drift (ADR-0080 Decision 1).
      func TestRequiresSkillsDeclarationsValid(t *testing.T) {
      	cat := Standard
      	for name, spec := range cat.Skills {
      		for _, r := range spec.RequiresSkills {
      			if _, ok := cat.Skills[r]; !ok {
      				t.Errorf("skill %q: requiresSkills entry %q is not a catalog skill", name, r)
      			}
      			if r == name {
      				t.Errorf("skill %q: requiresSkills must not name itself", name)
      			}
      		}
      	}
      	for name, spec := range cat.Agents {
      		for _, r := range spec.RequiresSkills {
      			if _, ok := cat.Skills[r]; !ok {
      				t.Errorf("agent %q: requiresSkills entry %q is not a catalog skill", name, r)
      			}
      		}
      	}
      	if len(cat.DomainDoc.RequiresSkills) != 0 {
      		t.Error("domainDoc: requiresSkills is only valid on skills and agents (ADR-0080 Decision 1)")
      	}
      }
      ```

- [ ] In `changelog/CHANGELOG.md`, under `## [Unreleased]` → `### Others`, append:

      ```markdown
      - The catalog now declares each skill's and agent's unconditional chain-skill
        coupling (`requiresSkills`), and the standard's test suite enforces the
        declarations both ways (undeclared reference and stale declaration each
        fail). Data only — no CLI or rendering behavior changes.
      ```

- [ ] Run `./x gate` — expect `coverage: 100.0%`, `0 issues.`, all tests pass.
- [ ] Commit: `feat(rendering): declare chain coupling in the catalog (ADR-0080)`
      — body: names Decision 1, notes declarations are proven exact by the Phase-2 sweep.

## Phase 2 — derived unset-data sweep

- [ ] Create `internal/project/catalog_sweep_test.go`:

      ```go
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
      // empty inline-code span or a literal ``…`` quoting span, never a ``` fence.
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
      ```

- [ ] Run `go test ./internal/project/ -run TestCatalogTemplatesDegradeLeakFree -v` —
      expect PASS with one subtest per catalog template (16 skills + 3 agents = 19).
      A failure here means a Phase-1 declaration is wrong: fix the declaration (or the
      template), never the sweep.
- [ ] Run `./x gate` — green.
- [ ] Commit: `test(rendering): sweep catalog templates under empty data (ADR-0080)`
      — body: backs `catalog-template-sweep` + `requires-skills-exact`.

## Phase 3 — hoist the case list, guard conditionals, backfill 11 cases

- [ ] In `internal/project/spine_test.go`, hoist the case list out of
      `TestUnsetFallbackRenders`: above the func, add

      ```go
      // fallbackCase pins one template's hand-authored degraded output: want
      // phrases must render under empty data, ban phrases must not; docs (when
      // set) replaces the layout docs map — used by RequiresDoc-gated templates
      // whose doc path must resolve. TestConditionalTemplatesHaveFallbackCases
      // requires an entry per conditional catalog template (ADR-0080).
      type fallbackCase struct {
      	tmpl string
      	docs map[string]any
      	want []string
      	ban  []string
      }

      var unsetFallbackCases = []fallbackCase{
      ```

      and move the existing 11 case literals into it unchanged (drop the anonymous
      struct header and its closing `}`; the literals keep their `tmpl`/`want`/`ban`
      fields). The func body becomes:

      ```go
      func TestUnsetFallbackRenders(t *testing.T) {
      	for _, tc := range unsetFallbackCases {
      		t.Run(tc.tmpl, func(t *testing.T) {
      			layout := testLayout()
      			if tc.docs != nil {
      				layout["docs"] = tc.docs
      			}
      			data := map[string]any{
      				"prefix": "example",
      				"vars":   map[string]any{},
      				"data":   map[string]any{},
      				"skills": map[string]bool{},
      				"layout": layout,
      			}
      			out := renderGolden(t, tc.tmpl, data)
      			for _, phrase := range tc.want {
      				if !strings.Contains(out, phrase) {
      					t.Errorf("missing fallback phrase %q:\n%s", phrase, out)
      				}
      			}
      			for _, phrase := range tc.ban {
      				if strings.Contains(out, phrase) {
      					t.Errorf("unset render must not contain %q:\n%s", phrase, out)
      				}
      			}
      		})
      	}
      }
      ```

- [ ] Append these 11 new cases to `unsetFallbackCases` (phrases verified against the
      2026-07-09 empty-data renders; keep them verbatim):

      ```go
      	{
      		tmpl: "skills/adr-lifecycle/SKILL.md.tmpl",
      		want: []string{"the multi-state lifecycle", "Run `awf sync` to regenerate"},
      	},
      	{
      		tmpl: "skills/brainstorming/SKILL.md.tmpl",
      		want: []string{
      			"hard prerequisite for any non-trivial change",
      			"The design lands in the ADR (if load-bearing) or the plan (if not)",
      		},
      	},
      	{
      		tmpl: "skills/executing-plans/SKILL.md.tmpl",
      		want: []string{"the project's gate (fast tier)", "Auto-commit when green"},
      	},
      	{
      		tmpl: "skills/proposing-adr/SKILL.md.tmpl",
      		want: []string{"follow the ADR template's section order", "Run `awf check` to confirm."},
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
      		want: []string{
      			"(or this project's runner alias for it)",
      			"using the project's commit scope conventions",
      		},
      	},
      	{
      		tmpl: "skills/reviewing-plan/SKILL.md.tmpl",
      		want: []string{"Only the plan file is edited", "using the project's commit scope conventions"},
      	},
      	{
      		tmpl: "skills/reviewing-plan-resync/SKILL.md.tmpl",
      		want: []string{"an amendment-while-Proposed edit", "using the project's commit scope conventions"},
      		ban:  []string{"example-adr-lifecycle"},
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
      	{
      		tmpl: "skills/subagent-driven-development/SKILL.md.tmpl",
      		want: []string{"**Gate per commit.** Fast tier by default.", "Sequential dispatch only — never parallel"},
      	},
      	{
      		tmpl: "skills/writing-plans/SKILL.md.tmpl",
      		want: []string{"per the example plan convention", "the project's gate runs before every commit"},
      	},
      ```

- [ ] In `internal/project/catalog_sweep_test.go`, append the guard (add `"io/fs"`,
      `"github.com/hypnotox/agentic-workflows/internal/render"`, and
      `"github.com/hypnotox/agentic-workflows/templates"` to the imports):

      ```go
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
      ```

- [ ] Run `go test ./internal/project/ -run 'TestUnsetFallbackRenders|TestConditionalTemplatesHaveFallbackCases' -v`
      — expect PASS (22 fallback subtests + the guard). A want-phrase failure means the
      2026-07-09 derivation drifted: re-render the template under the case's exact data
      and re-pin the phrase from the actual degraded output.
- [ ] Run `./x gate` — green.
- [ ] Commit: `test(rendering): force a fallback case per conditional (ADR-0080)`
      — body: backs `conditional-fallback-case-guard`; notes the 11 backfilled cases
      close the live gap found 2026-07-09.

## Phase 4 — golden completeness guard + the missing tdd golden

- [ ] In `internal/project/spine_test.go`, add after `TestBugfixTemplate`:

      ```go
      func TestTddTemplate(t *testing.T) {
      	data := map[string]any{
      		"prefix": "example",
      		"vars": map[string]any{
      			"testCmd": "go test ./...",
      			"gateCmd": "./x gate",
      		},
      		"data":   map[string]any{},
      		"skills": map[string]bool{},
      	}

      	out := renderSkillGolden(t, "tdd", data)

      	if !strings.Contains(out, "name: example-tdd") {
      		t.Errorf("expected 'name: example-tdd' in output:\n%s", out)
      	}

      	loadBearing := []string{
      		"confirm it fails for the right reason: `go test ./...`",
      		"Run the gate: `./x gate`",
      		"A test never observed failing proves nothing.",
      		"Fix the code, not the oracle.",
      	}
      	for _, phrase := range loadBearing {
      		if !strings.Contains(out, phrase) {
      			t.Errorf("expected phrase %q in output:\n%s", phrase, out)
      		}
      	}
      }
      ```

- [ ] In `internal/project/catalog_sweep_test.go`, append (add `"os"` to imports):

      ```go
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
      ```

- [ ] Run `go test ./internal/project/ -run 'TestTddTemplate|TestEveryCatalogArtifactHasGoldenTest' -v`
      — expect PASS.
- [ ] Run `./x gate` — green.
- [ ] Commit: `test(rendering): enforce a golden test per catalog artifact (ADR-0080)`
      — body: backs `golden-test-completeness`; notes tdd's golden was the day-one gap.

## Phase 5 — catalog-derived chain-closure fixture

- [ ] In `internal/project/drift_test.go`, replace
      `TestScopesEditReflagsReferencingArtifacts`'s local `cfg` closure: delete the
      hand-written skills/agents string lists and define (file-level, near the test;
      add `"sort"` and `"github.com/hypnotox/agentic-workflows/internal/catalog"` to
      the file's imports):

      ```go
      // chainClosureConfig derives the chain-unit enabled set from the catalog:
      // the Chain-flagged skills, their transitive RequiresSkills closure, and the
      // RequiresAgent agents of every skill in that combined set (ADR-0080
      // Decision 5) — never a hand list.
      func chainClosureConfig(scope string) string {
      	set := map[string]bool{}
      	var add func(name string)
      	add = func(name string) {
      		if set[name] {
      			return
      		}
      		set[name] = true
      		for _, r := range catalog.Standard.Skills[name].RequiresSkills {
      			add(r)
      		}
      	}
      	for name, spec := range catalog.Standard.Skills {
      		if spec.Chain {
      			add(name)
      		}
      	}
      	agents := map[string]bool{}
      	skills := make([]string, 0, len(set))
      	for name := range set {
      		skills = append(skills, name)
      		if a := catalog.Standard.Skills[name].RequiresAgent; a != "" {
      			agents[a] = true
      		}
      	}
      	sort.Strings(skills)
      	agentList := make([]string, 0, len(agents))
      	for a := range agents {
      		agentList = append(agentList, a)
      	}
      	sort.Strings(agentList)
      	var b strings.Builder
      	b.WriteString("prefix: example\nvars: {}\nskills:\n")
      	for _, s := range skills {
      		b.WriteString("  - " + s + "\n")
      	}
      	b.WriteString("agents:\n")
      	for _, a := range agentList {
      		b.WriteString("  - " + a + "\n")
      	}
      	b.WriteString("audit:\n  allowedScopes:\n    - " + scope + "\n")
      	return b.String()
      }
      ```

      In the test body, replace `cfg("awf")` / `cfg("core")` with
      `chainClosureConfig("awf")` / `chainClosureConfig("core")` and delete the `cfg`
      closure. The derived set is 11 skills (the current hand list minus `tdd`:
      adr-lifecycle, brainstorming, executing-plans, proposing-adr, retrospective,
      reviewing-adr, reviewing-impl, reviewing-plan, reviewing-plan-resync,
      subagent-driven-development, writing-plans) and 3 agents (adr-reviewer,
      code-reviewer, plan-reviewer).

- [ ] Re-point the test's negative control (per ADR-0080 Decision 5 — `tdd` is no
      longer rendered, so the old assertion would pass vacuously): replace

      ```go
      	if flagged[".claude/skills/example-tdd/SKILL.md"] {
      		t.Error("scopes edit reflagged the non-referencing tdd skill")
      	}
      ```

      with

      ```go
      	if flagged[".claude/skills/example-brainstorming/SKILL.md"] {
      		t.Error("scopes edit reflagged the non-referencing brainstorming skill")
      	}
      ```

- [ ] Run `go test ./internal/project/ -run TestScopesEditReflagsReferencingArtifacts -v`
      — expect PASS.
- [ ] Run `./x gate` — green.
- [ ] Commit: `test(config): derive the chain fixture from the catalog (ADR-0080)`
      — body: notes the deliberate tdd membership delta and the re-pointed negative
      control.

## Phase 6 — docs, guide invariants, ADR flip

- [ ] In `.awf/docs/parts/pitfalls/entries.md`, replace the entire section
      `## Adding a catalog skill: hand-enumerated test touch points` (heading through
      the paragraph ending "adding a skill means updating these by hand.") with:

      ```markdown
      ## Adding a catalog skill: what the guards force

      A new `SkillSpec` in `internal/catalog/standard.go` is covered automatically by the
      catalog-derived eval fixture (ADR-0053) and by ADR-0080's derived guards, which fail
      loudly and name the missing piece:

      - `TestCatalogTemplatesDegradeLeakFree` sweeps the template under empty data — an
        unconditional reference to another skill must be declared in `RequiresSkills`
        (exact both ways: undeclared references and stale declarations each fail).
      - `TestConditionalTemplatesHaveFallbackCases` requires a hand-authored
        `unsetFallbackCases` entry when the template carries conditional fallback prose —
        the degraded phrases themselves stay human-authored.
      - `TestEveryCatalogArtifactHasGoldenTest` requires a `Test<Skill>Template` golden in
        `internal/project/spine_test.go`.
      - Chain-enabling fixtures derive from the catalog (`chainClosureConfig`), so a new
        chain skill joins them without a hand edit.

      Exemptions follow default-inclusion semantics (ADR-0080 Decision 7): every exception
      is an explicit entry that itself fails when stale.
      ```

- [ ] In `.awf/agents-doc.yaml`, append to `data.invariants` — one bullet per new
      invariant slug, per the ADR's "four new invariant bullets" commitment; match
      the existing entries' indentation (8 spaces before `- ref:`):

      ```yaml
              - ref: ADR-0080
                text: '**Catalog-derived template sweep.** Every catalog skill/agent template renders under empty data in a catalog-derived loop banning leak residue and skill references outside the artifact''s `requiresSkills` declaration; the artifact set derives from the catalog, never a hand list.'
              - ref: ADR-0080
                text: '**Exact coupling declarations.** `requiresSkills` declarations are exact: an undeclared unconditional skill reference and a stale declaration both fail the sweep, and every other exemption is an explicit entry that fails when stale.'
              - ref: ADR-0080
                text: '**Conditional-fallback case guard.** Every catalog template whose post-expansion source carries a conditional action has a hand-authored unset-data case pinning its degraded prose.'
              - ref: ADR-0080
                text: '**Golden-test completeness.** Every catalog skill and agent has its per-artifact golden test in `internal/project`.'
      ```

- [ ] In `.awf/domains/parts/rendering/current-state.md`, append as the final
      sentence of the `## Current state` narrative (after the ADR-0069
      `.awf/memory/.gitignore` sentence that currently ends the file):

      ```markdown
      The catalog also declares each artifact's unconditional chain-skill coupling
      (`requiresSkills`, ADR-0080); a catalog-derived test sweep renders every template
      under empty data and holds those declarations exact.
      ```

- [ ] Flip the ADR: in
      `docs/decisions/0080-catalog-derived-test-coverage-for-skill-and-agent-templates.md`
      frontmatter, change `status: Proposed` → `status: Implemented`.
- [ ] Run `./x sync` — regenerates `docs/pitfalls.md`, `AGENTS.md` (+ bridge targets),
      `docs/domains/rendering.md`, `docs/decisions/ACTIVE.md`, `.awf/awf.lock`.
- [ ] Run `./x check` — expect `awf check: clean` (all four new inv slugs are backed by
      the markers landed in Phases 2–4).
- [ ] Run `./x gate` — green.
- [ ] Stage the `.awf/` parts, the rendered files, and the ADR; commit:
      `docs(adr): implement 0080 catalog-derived test coverage` — body: summarises the
      guards, names the closed pitfalls entry, notes the AGENTS.md invariant bullets and
      ACTIVE.md regen land here per the ADR's Consequences commitment.

## Execution notes

- Each phase's closing commit passes `./x gate` on its own; no cross-phase forward
  references (the sweep file grows across Phases 2–4 but each addition is self-contained).
- Never hand-edit a rendered file (`docs/pitfalls.md`, `AGENTS.md`, `docs/domains/*.md`,
  `ACTIVE.md`) — change the `.awf/` part and run `./x sync`.
- `./x audit-local` runs at impl review; the Phase-1 changelog entry covers the
  adopter-facing `internal/catalog/` touch.
- If any verify step's expected output mismatches, stop and fix the cause (or re-derive
  the pinned phrase from the actual render, for Phase-3 want-phrases only) — never weaken
  an assertion to pass.
