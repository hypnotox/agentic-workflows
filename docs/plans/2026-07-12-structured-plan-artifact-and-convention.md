# Plan: Structured Plan Artifact and Convention

Implements **[ADR-0097](../decisions/0097-plan-convention-sections-granularity-and-lifecycle.md)**
(convention) and **[ADR-0098](../decisions/0098-structured-plan-artifact-frontmatter-template-and-linking.md)**
(structured artifact), both Proposed. Design lives in those ADRs — this plan is the execution record.
Per ADR-0097's ordering note, β's frontmatter mechanism (Phases 1–4) lands before α's convention
prose (Phase 5); the ADRs flip to `Implemented` in Phase 5's final commit.

## Goal

Give plans a machine-readable spine and a uniform authored shape: a `internal/plan` package (frontmatter
parse + scaffold), two new `awf check` checks (plan→ADR link resolution, frontmatter validation), a
`plans-template` singleton rendered to `docs/plans/template.md`, an `awf new plan` command, `awf context`
plan surfacing, and the ADR-0097 convention prose across the writing/executing/reviewing artifacts.

## Architecture summary

- **New package `internal/plan`** mirrors `internal/adr`'s parse/scaffold split but is decoupled from
  sequential numbering: `ParseDir` scans `docs/plans/` for `YYYY-MM-DD-*.md` (excluding `template.md`
  and `README.md`), `parse` reads `date`/`adrs`/`status` frontmatter, `NewFile` scaffolds from the
  rendered `docs/plans/template.md`.
- **Two check functions** append to `Project.Check()`'s drift slice, reading `docs/plans/` from disk
  (the `adr.ParseDir` precedent), skipping frontmatter-less plans.
- **`plans-template`** is one `catalog.DocEntry`; `SingletonKinds`/`plainSingletons`/`layout.templateMap`
  auto-derive rendering, the `plansTemplate` layout key, and drift-tracking (ADR-0061).
- **`awf new plan`** is a `new` group child dispatching to `newPlan` → `project.NewPlan` → `plan.NewFile`.
- **`awf context`** gains `Plans []PlanRef` on the single `ContextResult`; the join is transitive
  (path → domain → ADR → plans linking that ADR), preserving ADR-0092's read-only/parity/fallback.
- **Phase 5** edits template/doc prose only.

## Tech stack

Go 1.26; module `github.com/hypnotox/agentic-workflows`. Packages touched: `internal/plan` (new),
`internal/project` (check, context, layout, tests), `internal/catalog` (DocEntry), `cmd/awf`
(new-plan dispatch), `templates/` (plans-template, writing-plans, executing-plans, plan-reviewer,
plans-readme, agents-doc). Gate: `./x gate` (100% coverage, deadcode, pincheck) before every commit;
`./x check` for drift. Example adopter `examples/sundial` re-renders via `./x sync` and must stay
zero-notes (ADR-0090).

## File structure

- **Created:**
  - `internal/plan/plan.go`, `internal/plan/plan_test.go`
  - `templates/plans-template/template.md.tmpl`
- **Modified:**
  - `internal/project/check.go`, `internal/project/check_test.go`
  - `internal/project/context.go`, `cmd/awf/context.go`, and their tests
  - `internal/project/project.go` (NewPlan wrapper), `internal/project/project_test.go`
  - `internal/project/golden_test.go` (plans-template golden)
  - `internal/catalog/standard.go` (DocEntry)
  - `cmd/awf/new.go`, `cmd/awf/new_test.go`, `internal/clispec/clispec.go`, `internal/clispec/clispec_test.go`
  - `templates/skills/writing-plans/SKILL.md.tmpl`, `templates/skills/executing-plans/SKILL.md.tmpl`,
    `templates/skills/subagent-driven-development/SKILL.md.tmpl`, `templates/agents/plan-reviewer.md.tmpl`,
    `templates/plans-readme/README.md.tmpl`
  - `.awf/agents-doc.yaml` (`data.invariants` + `data.commands`), `AGENTS.md` (regenerated)
  - `docs/decisions/0097-*.md`, `docs/decisions/0098-*.md` (status flip), `docs/decisions/ACTIVE.md` (regen)
  - `.awf/awf.lock`, `docs/domains/rendering.md`, `docs/domains/tooling.md` (regen), `examples/sundial/**` (re-render)
- **Deleted:** none

---

## Phase 1 — `internal/plan` parsing + the two checks

Coupled phase (single commit — genuinely un-sliceable): the parser and both check functions must land
together, because the parser is production-dead-code without the checks that call it and the checks
cannot compile without the parser. The scaffold half (`NewFile`) lands in Phase 3 with its only
production caller (`awf new plan`), so it does not appear here — placing it here would fail the
deadcode gate.

- [ ] **Task 1.1 — Create `internal/plan/plan.go` (parser only).** Exact content:

```go
// Package plan parses plan files under docs/plans and scaffolds new plans from
// the rendered plans template (awf new plan). Unlike internal/adr it is not
// coupled to sequential numbering — plans are date-prefixed.
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/hypnotox/agentic-workflows/internal/frontmatter"
)

// ValidStatuses are the two plan lifecycle states (ADR-0097): mutable while
// Proposed, frozen at Implemented.
var ValidStatuses = map[string]bool{"Proposed": true, "Implemented": true}

// FilenameRe matches a plan filename (YYYY-MM-DD-slug.md); it excludes
// template.md and README.md just as adr.FilenameRe's numeric form does.
var FilenameRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-.+\.md$`)

// Plan is a parsed plan record. HasFrontmatter is false for the grandfathered
// pre-convention corpus (ADR-0098), which the checks skip.
type Plan struct {
	Filename       string
	Path           string
	Date           string
	ADRs           []int
	Status         string
	HasFrontmatter bool
}

type planFrontmatter struct {
	Date   string `yaml:"date"`
	ADRs   []int  `yaml:"adrs"`
	Status string `yaml:"status"`
}

// ParseDir scans dir for plan files (YYYY-MM-DD-*.md) and parses each. Files
// without frontmatter parse to a Plan with HasFrontmatter false.
func ParseDir(dir string) ([]Plan, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", dir, err)
	}
	var plans []Plan
	for _, path := range matches {
		base := filepath.Base(path)
		if !FilenameRe.MatchString(base) {
			continue // skip template.md, README.md, and any non-plan file
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		var fm planFrontmatter
		_, found, err := frontmatter.Parse(data, &fm)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", base, err)
		}
		plans = append(plans, Plan{
			Filename: base, Path: path, Date: fm.Date, ADRs: fm.ADRs,
			Status: fm.Status, HasFrontmatter: found,
		})
	}
	return plans, nil
}
```

The scaffold half (`NewFile` + its `now`/`slugify`/`replaceOnce`/`markerLineRe` helpers, which add the
`strings` and `time` imports) is added to this same file in Phase 3 Task 3.1, alongside its `awf new
plan` caller.

- [ ] **Task 1.2 — Wire the two checks into `Project.Check()`.** In `internal/project/check.go`, add
  the `plan` import (`"github.com/hypnotox/agentic-workflows/internal/plan"`), and after line 431
  (`drift = append(drift, p.checkDeadSkillRefs(...)...)`) append:

```go
	planDrift, err := p.checkPlans()
	if err != nil {
		return nil, err
	}
	drift = append(drift, planDrift...)
```

  Then add these functions (place after `checkDeadRefs`):

```go
// checkPlans validates plan frontmatter and plan→ADR links over docs/plans/,
// scanning the YYYY-MM-DD-*.md set only (excluding template.md and README.md).
// Frontmatter-less plans (the grandfathered corpus, ADR-0098) are skipped.
// invariant: plan-frontmatter-validated
// invariant: plan-adr-link-resolved
func (p *Project) checkPlans() ([]manifest.Drift, error) {
	plansDir := filepath.Join(p.Root, p.Cfg.DocsDir, "plans")
	plans, err := plan.ParseDir(plansDir)
	if err != nil {
		return nil, err
	}
	adrs, err := adr.ParseDir(p.decisionsDir())
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, a := range adrs {
		known[a.Number] = true
	}
	rel := filepath.ToSlash(filepath.Join(p.Cfg.DocsDir, "plans"))
	var drift []manifest.Drift
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		path := rel + "/" + pl.Filename
		if !plan.ValidStatuses[pl.Status] {
			drift = append(drift, manifest.Drift{Path: path, Kind: "plan-frontmatter", Detail: fmt.Sprintf("status %q not in {Proposed, Implemented}", pl.Status)})
		}
		for _, n := range pl.ADRs {
			if !known[fmt.Sprintf("%04d", n)] {
				drift = append(drift, manifest.Drift{Path: path, Kind: "plan-adr-link", Detail: fmt.Sprintf("ADR-%04d", n)})
			}
		}
	}
	return drift, nil
}
```

  Add the `adr` import to `check.go` if not present (`"github.com/hypnotox/agentic-workflows/internal/adr"`).
  Note: a plan whose YAML frontmatter is malformed surfaces as a parse error from `plan.ParseDir`
  (returned as the `err` above → a hard check error), covering the "unparseable YAML" half of
  `plan-frontmatter-validated`; the `status` enum is the drift half.

- [ ] **Task 1.3 — Create `internal/plan/plan_test.go` (parser tests).** Cover `ParseDir` on a temp
  dir with: a frontmatter plan (fields parsed, `HasFrontmatter` true), a frontmatter-less plan
  (`HasFrontmatter` false), a `template.md` and `README.md` present (both skipped by `FilenameRe`),
  and a malformed-YAML plan (error). 100% statement coverage of the parser half. (`NewFile`'s tests
  land with `NewFile` in Phase 3 Task 3.1.)

- [ ] **Task 1.4 — Extend `internal/project/check_test.go`** with a fixture project that has a plan
  under `docs/plans/` linking a nonexistent ADR (asserts a `plan-adr-link` drift), a bad `status:`
  (asserts a `plan-frontmatter` drift), a valid plan (no drift), and a frontmatter-less plan (no
  drift). Reuse the package's existing project-fixture helper.

- [ ] **Task 1.5 — Verify and commit.**
  - `./x gate` → ends with `coverage: 100.0%`, `0 issues.`, `deadcodecheck: no production dead code`,
    `pincheck: all workflow references pinned`.
  - `./x check` → `awf check: clean`.
  - `git add internal/plan/plan.go internal/plan/plan_test.go internal/project/check.go internal/project/check_test.go`
  - `git commit -m "feat(tooling): add internal/plan parsing and awf check plan validation"`
    (body: names inv plan-frontmatter-validated + plan-adr-link-resolved). The parser + checks are
    the whole of Phase 1; `NewFile` is deliberately absent (it would be production dead-code without
    its Phase 3 caller).

---

## Phase 2 — `plans-template` singleton

- [ ] **Task 2.1 — Create `templates/plans-template/template.md.tmpl`.** Exact content (embodies
  ADR-0097's taxonomy; the `date: YYYY-MM-DD` and `# Plan: Title` placeholders are the `NewFile` fill
  points):

```
---
date: YYYY-MM-DD
adrs: []
status: Proposed
---
# Plan: Title

<!-- awf:section positioning -->
One-line statement of what this plan implements. When ADRs drive the work, link them here and in
`adrs:` above — the design lives in the ADR(s); this plan is the execution record, not a place to
re-argue design.
<!-- awf:end -->

<!-- awf:section header -->
## Goal

What this plan achieves, in one or two sentences.

## Architecture summary

The execution shape — the structural moves the phases make. Not the rationale (that lives in the
linked ADR).

## Tech stack

Language version and the key packages/files touched. The gate command run before each commit.

## File structure

- **Created:** new files.
- **Modified:** changed files.
- **Deleted:** removed files (or `none`).
<!-- awf:end -->

<!-- awf:section phases -->
## Phase 1 — <name>

- [ ] **Task 1.1 — <what>.** Exact paths, the exact new-file content or diff, and the exact verify
  command with its expected output. A task is one reviewable, logically-coherent change (a whole new
  file is one task). For a transformation repeated across 3+ sites, use the batch form (a
  representative diff, an edge diff, the affected-site set, and a post-check).
- [ ] **Task 1.2 — Verify and commit.** Run the gate; `git add` the exact paths; commit with a
  Conventional-Commits subject. Every phase's closing commit passes the gate on its own — unless the
  change genuinely cannot be sliced, in which case mark the coupled phases and share one closing
  commit, stating why.
<!-- awf:end -->

<!-- awf:section verification -->
## Verification (optional)

Whole-effort end-state checks beyond the per-phase gates — the acceptance criteria for "done".
<!-- awf:end -->

<!-- awf:section notes -->
## Notes (optional)

Out-of-scope items, follow-ups, and findings surfaced during implementation (a wrong diff, an
unsliceable phase, a bad estimate). The `status: Implemented` flip in the final commit records these
alongside the freeze.
<!-- awf:end -->
```

- [ ] **Task 2.2 — Register the DocEntry.** In `internal/catalog/standard.go`, after the
  `plans-readme` entry (line 192) add:

```go
			"plans-template": {Mandatory: true, Path: "plans/template.md", TemplateKey: "plansTemplate", TID: "plans-template/template.md.tmpl", Sections: []string{"positioning", "header", "phases", "verification", "notes"}},
```

  `SingletonKinds`, `plainSingletons`, and `layout.templateMap` derive `plansTemplate` automatically
  (ADR-0061) — no other production wiring.

- [ ] **Task 2.3 — Update the layout-key test list.** In `internal/project/project_test.go` (the
  layout-key enumeration around lines 35 and 721 — confirm exact lines at execution), add
  `"plansTemplate"` to the expected `.layout` key set alongside `"adrTemplate"`/`"plansReadme"`.

- [ ] **Task 2.4 — Add the plans-template golden + taxonomy check.** In
  `internal/project/golden_test.go`, alongside the plans-readme render assertion (near line 42), add
  a render assertion for `docs/plans/template.md` that asserts (backing `inv: plans-template-taxonomy`)
  the rendered output contains the frontmatter keys `date:`, `adrs:`, `status:` and the taxonomy
  headings `# Plan:`, `## Goal`, `## Architecture summary`, `## Tech stack`, `## File structure`, a
  `## Phase`, `## Verification`, and `## Notes`, and is marker/leak-free. Add the
  `// invariant: plans-template-taxonomy` marker on the test.

- [ ] **Task 2.5 — Verify and commit.**
  - `./x sync` renders `docs/plans/template.md` (new) and updates `.awf/awf.lock` + `examples/sundial`.
  - `./x gate` → `coverage: 100.0%`, `0 issues.`, deadcode + pincheck clean.
  - `./x check` → `awf check: clean` (and zero `note:` lines for `examples/sundial`).
  - `git add templates/plans-template/template.md.tmpl internal/catalog/standard.go internal/project/project_test.go internal/project/golden_test.go docs/plans/template.md .awf/awf.lock examples/sundial`
  - `git commit -m "feat(rendering): ship the plans-template singleton"`

---

## Phase 3 — `awf new plan` command

- [ ] **Task 3.1 — Add `plan.NewFile` + helpers to `internal/plan/plan.go`.** Add `"strings"` and
  `"time"` to the import block, then append:

```go
// now returns the current time; overridden in tests (mirrors adr.now).
var now = time.Now

var markerLineRe = regexp.MustCompile(`(?m)^<!-- (GENERATED by awf|awf:edit).*-->\n`)
var slugNonAlnumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) (string, error) {
	s := slugNonAlnumRe.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "", fmt.Errorf("plan: title %q has no usable characters for a filename", title)
	}
	return s, nil
}

func replaceOnce(s, old, replacement string) (string, error) {
	if !strings.Contains(s, old) {
		return "", fmt.Errorf("plan: template missing expected %q", old)
	}
	return strings.Replace(s, old, replacement, 1), nil
}

// NewFile scaffolds a new plan under dir from the rendered plans template
// (dir/template.md): today's date filled, marker comments stripped, named
// YYYY-MM-DD-slug.md. No sequential number is allocated. Refuses to overwrite.
// invariant: plan-new-unnumbered
func NewFile(dir, title string) (string, error) {
	title = strings.TrimSpace(title)
	slug, err := slugify(title)
	if err != nil {
		return "", err
	}
	date := now().Format("2006-01-02")
	path := filepath.Join(dir, date+"-"+slug+".md")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("plan: %s already exists", path)
	} else if !os.IsNotExist(err) { // coverage-ignore: Stat fails here only on a permission fault a test cannot trigger
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "template.md"))
	if err != nil {
		return "", fmt.Errorf("plan: read template: %w", err)
	}
	content := markerLineRe.ReplaceAllString(string(raw), "")
	content, err = replaceOnce(content, "date: YYYY-MM-DD", "date: "+date)
	if err != nil {
		return "", err
	}
	content, err = replaceOnce(content, "# Plan: Title", "# Plan: "+title)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { // coverage-ignore: post-Stat write; fails only on a permission fault a test cannot trigger
		return "", err
	}
	return path, nil
}
```

  Add the `NewFile` cases to `internal/plan/plan_test.go`: happy path (asserts filename
  `YYYY-MM-DD-slug.md`, date + title filled, markers stripped), overwrite refusal, a title with no
  usable characters (error), a template missing the `date:`/`# Plan:` placeholder (error via
  `replaceOnce`), and a missing `template.md` (error). Override `now` for a deterministic date.
  Coverage stays 100%.

- [ ] **Task 3.2 — Add `project.NewPlan`.** In `internal/project/project.go`, alongside `NewADR`, add:

```go
// NewPlan scaffolds a new plan under docsDir/plans from the rendered plans
// template. Mirrors NewADR minus sequential numbering (ADR-0098).
func (p *Project) NewPlan(title string) (string, error) {
	return plan.NewFile(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"), title)
}
```

  Add the `plan` import.

- [ ] **Task 3.3 — Dispatch the command.** In `cmd/awf/new.go`, add a `"plan"` case to `runNew`'s
  switch and a `newPlan` function mirroring `newADR`:

```go
	case "plan":
		return newPlan(root, args, stdout)
```

```go
func newPlan(root string, titleWords []string, stdout io.Writer) error {
	if len(titleWords) == 0 {
		return &usageErr{"usage: awf new plan <title>"}
	}
	if err := gate(root); err != nil {
		return err
	}
	p, err := project.Open(root)
	if err != nil {
		return err
	}
	path, err := p.NewPlan(strings.Join(titleWords, " "))
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}
```

  Update the `default` usage string in `runNew` to `"unknown kind %q (want: adr, plan, skill, agent, doc)"`.

- [ ] **Task 3.4 — Register the clispec child.** In `internal/clispec/clispec.go`, add a `plan` child
  to the `new` command's `Children` (mirroring the `adr` child), update the `new` `Summary`/`HelpBody`
  to include `plan`, and in `internal/clispec/clispec_test.go` change the child-count assertion from
  `4` to `5` and add a `newCmd.Child("plan")` presence check.

- [ ] **Task 3.5 — Test the command.** In `cmd/awf/new_test.go`, add cases: `awf new plan "Some
  Title"` scaffolds `docs/plans/<today>-some-title.md` and prints the path; missing title → usage
  error; a pre-existing target → the overwrite error. Use the existing new-command test fixture
  (which has a rendered `docs/plans/template.md` after sync).

- [ ] **Task 3.6 — Verify and commit.**
  - `./x gate` → all clean, `coverage: 100.0%`.
  - `git add cmd/awf/new.go cmd/awf/new_test.go internal/clispec/clispec.go internal/clispec/clispec_test.go internal/project/project.go internal/plan/plan.go internal/plan/plan_test.go`
  - `git commit -m "feat(tooling): add the awf new plan scaffold command"`

---

## Phase 4 — `awf context` plan surfacing

- [ ] **Task 4.1 — Add `PlanRef` + `Plans` to `ContextResult`.** In `internal/project/context.go`,
  add the field to the struct (after `ADRs`):

```go
	Plans      []PlanRef   `json:"plans"`
```

  and the type:

```go
// PlanRef is a plan surfaced because its adrs: links an ADR reported for the
// query. Path is docsDir-rooted; ADRs are the linked ADR numbers.
type PlanRef struct {
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	ADRs     []int  `json:"adrs"`
}
```

- [ ] **Task 4.2 — Populate it in `ContextFor`.** After the ADR loop (before the final `sort` / return),
  build the set of surfaced ADR numbers and scan plans (add the `plan` import):

```go
	surfaced := map[int]bool{}
	for _, a := range res.ADRs {
		if n, err := strconv.Atoi(a.Number); err == nil { // coverage-ignore: a.Number is always a 4-digit numeral from FilenameRe
			surfaced[n] = true
		}
	}
	plans, err := plan.ParseDir(filepath.Join(p.Root, p.Cfg.DocsDir, "plans"))
	if err != nil {
		return ContextResult{}, err
	}
	for _, pl := range plans {
		if !pl.HasFrontmatter {
			continue
		}
		for _, n := range pl.ADRs {
			if surfaced[n] {
				res.Plans = append(res.Plans, PlanRef{
					Filename: pl.Filename, Path: lay.PlansDir + "/" + pl.Filename,
					Status: pl.Status, ADRs: pl.ADRs,
				})
				break
			}
		}
	}
	sort.Slice(res.Plans, func(i, j int) bool { return res.Plans[i].Filename < res.Plans[j].Filename })
```

  Add the `strconv` import. `lay.PlansDir` is already available (`lay := p.layout()`). Mark the
  function `// invariant: context-surfaces-linked-plans`.

- [ ] **Task 4.3 — Render plans in the human output.** In `cmd/awf/context.go`'s `printContext`, after
  the Related-ADRs block (before the Unowned block) add:

```go
	if len(res.Plans) > 0 {
		fmt.Fprintln(stdout, "\n## Related plans")
		for _, pl := range res.Plans {
			fmt.Fprintf(stdout, "  %s (%s) — %s\n", pl.Filename, pl.Status, pl.Path)
		}
	}
```

  The `--json` path already serializes the new field (output parity preserved by the single
  `ContextResult`).

- [ ] **Task 4.4 — Test.** In the context test (`internal/project/context_test.go` and
  `cmd/awf/context_test.go`), extend the fixture with a plan linking a domain-owned ADR and assert it
  appears in `res.Plans` and in both renderings; assert a frontmatter-less plan and a plan linking an
  unsurfaced ADR do not appear; assert the static-fallback path leaves `Plans` empty.

- [ ] **Task 4.5 — Verify and commit.**
  - `./x gate` → all clean, `coverage: 100.0%`.
  - `git add internal/project/context.go internal/project/context_test.go cmd/awf/context.go cmd/awf/context_test.go`
  - `git commit -m "feat(tooling): surface linked plans in awf context"`

---

## Phase 5 — ADR-0097 convention prose + docs + ADR flip

- [ ] **Task 5.1 — Retire wall-clock granularity + add coupled-phase escape in the writing-plans
  skill.** In `templates/skills/writing-plans/SKILL.md.tmpl`, in the `conventions-tasks` section
  replace `bite-sized (~2-5 min each)` with `one reviewable, logically-coherent change each (a whole
  new file is a single task)`, and append to the `Self-contained phases` bullet a coupled-phase
  sentence: `When a change genuinely cannot be sliced into independently-gate-passing phases (a
  signature threaded through many callers, a struct-shape rewrite), mark the coupled phases as a group
  and share one closing commit, stating why it cannot be sliced — the exception, never a convenience.`
  In the `conventions-header` bullet, name the canonical four fields and the optional Verification/Notes
  tails, and state the title is the `# Plan:` H1. Fill the empty `plan-template-ref` section with a
  pointer to copy `{{ .layout.plansTemplate }}` (or run `{{ .prefix }} new plan "<Title>"`). Rewrite
  the `plan-lifecycle` section to the two-state plan-own model — mutable while `status: Proposed`,
  frozen at `status: Implemented` — removing the `# Implementation complete (YYYY-MM-DD)` line entirely
  (so the Task 5.3 post-check passes), and update `positioning`/`notes` to mention the `adrs:` frontmatter
  as the structured link.

- [ ] **Task 5.2 — Reframe the plan-reviewer executability lens.** In
  `templates/agents/plan-reviewer.md.tmpl` (universal-lenses section, the `executability` bullet),
  replace `tasks are bite-sized (~2–5 min)` with `each task is one reviewable, logically-coherent
  change (a whole new file is one task)`, and change `each phase's closing commit passes the project's
  gate on its own — flag any definition whose first production use lands in a later phase` to add the
  coupled-phase exception (`unless the plan marks a coupled-phase group that genuinely cannot be
  sliced`). Add three focus items to `.awf/agents/plan-reviewer.yaml` `focusItems` — `section-taxonomy`
  (the canonical frontmatter + header/phases/tails shape), `coupled-phase-escape` (a multi-phase single
  commit is marked with the reason it cannot be sliced, never used as convenience), and
  `lifecycle-freeze` (the two-state `status: Implemented` freeze, findings recorded in Notes) —
  covering all three items ADR-0097's Consequences require.

- [ ] **Task 5.3 — Unify the freeze marker in the executing skills.** In
  `templates/skills/executing-plans/SKILL.md.tmpl`: in `procedure-adr-final-commit`, keep the existing
  ADR `status:` flip (`Proposed → Accepted`/`Implemented`) and *add* a step flipping the plan's own
  `status: Proposed → Implemented` frontmatter and recording implementation findings in the plan's
  Notes section; rewrite `procedure-non-adr-final-commit` to flip the plan's `status: Proposed →
  Implemented` (replacing the `# Implementation complete (YYYY-MM-DD)` line) so ADR-driven and non-ADR
  plans freeze identically via the plan's own status; and update `procedure-resolve-plan`'s mutability
  wording to key off the plan's own status, not the ADR's. In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`, whose single combined final section
  (`Final task: ADR status flip and/or plan freeze`, ~line 69) covers both paths, apply the equivalent
  edit: retain any ADR flip and add the plan `status: Proposed → Implemented` freeze + findings-recording.
  Post-check: `grep -rn "Implementation complete" templates/skills/` returns nothing (also confirms
  Task 5.1's `plan-lifecycle` rewrite).

- [ ] **Task 5.4 — Update the plans-readme + AGENTS.md source.** In
  `templates/plans-readme/README.md.tmpl` (`structure` and freeze prose) reflect the frontmatter, the
  canonical sections, the reviewability granularity, and the two-state freeze. In `.awf/agents-doc.yaml`
  (AGENTS.md renders its Commands from `data.commands` and its Invariants from `data.invariants`), add
  an `awf new plan` entry under `data.commands` and five `- text:` bullets under `data.invariants` for
  `plan-frontmatter-validated`, `plan-adr-link-resolved`, `plans-template-taxonomy`,
  `plan-new-unnumbered`, and `context-surfaces-linked-plans` (each phrased like the existing bullets,
  citing ADR-0097/0098). `./x sync` (Task 5.5) regenerates `AGENTS.md` from these.

- [ ] **Task 5.5 — Flip both ADRs and re-render.** Set `status: Proposed → Implemented` in
  `docs/decisions/0097-*.md` and `docs/decisions/0098-*.md`. Run `./x sync` (regenerates `ACTIVE.md`,
  the rendering/tooling domain docs, `AGENTS.md`, `.awf/awf.lock`, and re-renders `examples/sundial`).

- [ ] **Task 5.6 — Verify and commit (Tasks 5.1–5.5 as one coherent commit).** Phase 5 is a single
  *coherent-atomic* "adopt the convention" commit — not a coupled-phase escape: the prose edits alone
  pass the gate while the ADRs are still Proposed (the invariant markers landed in Phases 1–4 and the
  ADR→marker check only fires on Implemented ADRs), so it *could* be split, but the convention prose,
  the AGENTS.md currency, and the ADR flip are one indivisible unit of meaning and land together.
  - `./x gate` → all clean, `coverage: 100.0%`.
  - `./x check` → `awf check: clean`, zero `note:` lines for `examples/sundial`.
  - `git add` the exact modified templates, `.awf/agents-doc.yaml`, `AGENTS.md`, both ADR files,
    `docs/decisions/ACTIVE.md`, `docs/domains/rendering.md`, `docs/domains/tooling.md`,
    `.awf/awf.lock`, and `examples/sundial`.
  - `git commit -m "feat(rendering): adopt the ADR-0097 plan convention"` (body: flips ADR-0097 and
    ADR-0098 to Implemented; lists the convention changes and the five now-enforced invariants).

## Verification

- `./x gate` and `./x check` are green at every phase commit.
- `awf new plan "Test"` creates `docs/plans/<today>-test.md` with valid frontmatter; deleting it leaves the tree clean.
- A plan linking a nonexistent ADR fails `./x check` with a `plan-adr-link` drift; a bad `status:` fails with `plan-frontmatter`.
- `awf context internal/project/context.go` lists this plan under "Related plans" (once this plan carries frontmatter — it does not, being grandfathered; verify instead with a scratch frontmatter plan, then delete it).
- `examples/sundial` `awf check` output has zero `note:` lines.
- The five invariants each have a backing `// invariant:` marker and are enforced once the ADRs are Implemented.

## Notes

- This plan is itself written under the pre-convention (frontmatter-less) shape — the tooling it builds
  does not exist until it lands. It is grandfathered like the other 67 corpus plans and freezes via the
  ADR flip, not a plan `status:` field.
- Phase 1 is the plan's one genuine **coupled-phase escape** (ADR-0097): the parser is production
  dead-code without its checks and the checks cannot compile without the parser, so they share one
  commit because the change cannot be sliced. Phase 5 is a **coherent-atomic** single commit (it
  *could* be split but is one unit of meaning) — not the escape. The distinction is exactly the one
  the new convention draws.
