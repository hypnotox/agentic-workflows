# Plan: Doc Cross-References via Awf-Given Layout (ADR-0013)

Design & rationale: [docs/decisions/0013-doc-cross-references-via-layout.md](../decisions/0013-doc-cross-references-via-layout.md). This plan is the execution record only — do not restate rationale; link to the ADR.

## Goal

Move documentation cross-references in the standard's skill/agent/AGENTS.md templates from hand-set `.vars.*` onto the awf-given `.layout.*` namespace; add doc-gated-skill suppression; delete six non-generic vars; and enable the docs that earn their keep in this repo. End state: `awf check` clean, `awf gate` green at 100% coverage, ADR-0013 `Implemented`, all five tagged invariants backed.

## Architecture summary

- `internal/project.layout()` gains three computed members: `docs` (enabled-doc → path map), `workflowRef` (workflow doc path or `AGENTS.md` fallback), `domainsDir`.
- `internal/project.RenderAll()` skills loop gains a suppression skip: a skill whose `catalog.SkillSpec.RequiresDoc` is set and not in enabled docs is omitted.
- `catalog.SkillSpec` gains an optional `RequiresDoc string`. `roadmap-graduation` sets `requiresDoc: roadmap`.
- Every template doc reference moves to `.layout.*`. Six vars (`oracleStateDoc`, five `*AdrRef`) leave the standard; one whole catalog section (`host-git-constraint`, content was solely `hostGitAdrRef`) is removed from two skills.
- This repo enables docs `workflow`/`testing`/`development`/`pitfalls`/`glossary` and drops the deleted/migrated vars from its config.

## Tech stack

- Go 1.26. Packages touched: `internal/catalog`, `internal/project`. No new dependencies.
- Templates under `templates/` (Go `text/template`, marker sections). Config tree under `.claude/awf/`.
- Gate: `./x gate` (`go test ./... -coverpkg` at 100% via `cmd/covercheck`, `go vet`, `golangci-lint`). Drift: `./x check`.

## File structure

**Modified (Go):** `internal/catalog/catalog.go`, `internal/project/project.go`, `internal/project/project_test.go`, `internal/project/frontmatter_test.go`.
**Modified (templates):** `templates/catalog.yaml`; skills `brainstorming`, `bugfix`, `debugging`, `executing-plans`, `subagent-driven-development`, `proposing-adr`, `adr-lifecycle`, `reviewing-plan`, `reviewing-adr`, `writing-plans`, `refactor-coupling-audit`, `roadmap-graduation` (`SKILL.md.tmpl`); `templates/agents-doc/AGENTS.md.tmpl`; `templates/docs/workflow.md.tmpl`.
**Modified (config):** `.claude/awf/config.yaml`.
**Modified (decisions):** `docs/decisions/0013-doc-cross-references-via-layout.md` (status flip).
**Re-rendered (committed alongside config/template changes):** files under `.claude/skills/`, `.claude/agents/`, `AGENTS.md`, `docs/*.md`, `docs/decisions/ACTIVE.md`, `.claude/awf/awf.lock`. **Deleted:** `.claude/skills/awf-roadmap-graduation/SKILL.md` (suppressed).

---

## Phase 1 — Layout extension, catalog field, render suppression (Go)

No template or rendered-output change in this phase; suppression is exercised by synthetic tests only. `requiresDoc: roadmap` is **not** set here (Phase 2).

### Task 1.1 — Add `RequiresDoc` to `catalog.SkillSpec`

- [ ] In `internal/catalog/catalog.go`, change the `SkillSpec` struct:

```go
type SkillSpec struct {
	Sections []string `yaml:"sections"`
}
```

to:

```go
type SkillSpec struct {
	Sections    []string `yaml:"sections"`
	RequiresDoc string   `yaml:"requiresDoc"`
}
```

### Task 1.2 — Extend `layout()` with `docs`, `workflowRef`, `domainsDir`

- [ ] In `internal/project/project.go`, replace the `layout()` body. Current:

```go
func (p *Project) layout() map[string]any {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dec := d + "/decisions"
	return map[string]any{
		"docsDir":     d,
		"adrDir":      dec,
		"activeMd":    dec + "/ACTIVE.md",
		"adrReadme":   dec + "/README.md",
		"adrTemplate": dec + "/template.md",
		"plansDir":    d + "/plans",
	}
}
```

Replace with:

```go
func (p *Project) layout() map[string]any {
	d := strings.TrimRight(p.Cfg.DocsDir, "/")
	dec := d + "/decisions"
	// docs maps every enabled doc name to its output path. Local docs are
	// included: the file still exists at that path and is citable. A key is
	// present iff the doc is enabled (inv: layout-docs-enabled-only).
	docs := map[string]any{}
	for _, name := range p.Cfg.Docs {
		docs[name] = p.docOutPath(name)
	}
	// workflowRef is the workflow doc's path when enabled, else AGENTS.md, so
	// the ~always-cited workflow reference always resolves (inv: workflow-ref-fallback).
	workflowRef := "AGENTS.md"
	if wp, ok := docs["workflow"]; ok {
		workflowRef = wp.(string)
	}
	return map[string]any{
		"docsDir":     d,
		"adrDir":      dec,
		"activeMd":    dec + "/ACTIVE.md",
		"adrReadme":   dec + "/README.md",
		"adrTemplate": dec + "/template.md",
		"plansDir":    d + "/plans",
		"docs":        docs,
		"workflowRef": workflowRef,
		"domainsDir":  d + "/domains", // inv: domains-dir-given
	}
}
```

### Task 1.3 — Add doc-gated-skill suppression to `RenderAll`

- [ ] In `internal/project/project.go`, in the `RenderAll()` skills loop, after the `sc.Local` skip (currently lines ~305-307), add the suppression skip. Current:

```go
	for _, name := range sortedStrings(p.Cfg.Skills) {
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
```

Replace with:

```go
	enabledDocs := sliceSet(p.Cfg.Docs)
	for _, name := range sortedStrings(p.Cfg.Skills) {
		sc, err := p.Cfg.Sidecar("skills", name)
		if err != nil {
			return nil, err
		}
		if sc.Local {
			continue
		}
		// Doc-gated skill: omit from the render set when its required doc is not
		// enabled (inv: doc-gated-skill-suppressed).
		if req := p.Cat.Skills[name].RequiresDoc; req != "" && !enabledDocs[req] {
			continue
		}
```

(`sliceSet` already exists — used by `orphans()` at ~L586.)

### Task 1.4 — Update/extend Go tests; back four invariants

- [ ] In `internal/project/project_test.go`, extend `TestLayoutDerivesFromDocsDir`. `workflowRef` and `domainsDir` are strings (add them to the existing `want map[string]string`); `docs` is a `map[string]any`, so assert it separately (e.g. compare against an expected `map[string]any` for the test's `Docs` config). Cover **both** `workflowRef` arms — one sub-case/Project with `workflow` in `Docs` (→ `<docsDir>/workflow.md`) and one without (→ `AGENTS.md`). Tag the relevant assertions `// invariant: layout-docs-enabled-only` (`docs`), `// invariant: workflow-ref-fallback` (`workflowRef`, both arms), `// invariant: domains-dir-given` (`domainsDir`).
- [ ] Add `TestRenderAllSuppressesDocGatedSkill` to `internal/project/project_test.go`: construct a `Project` whose `Cat.Skills` contains a skill with `RequiresDoc: "roadmap"`; assert its output path is absent from `RenderAll()` when `Cfg.Docs` excludes `roadmap` and present when it includes `roadmap`. Tag with `// invariant: doc-gated-skill-suppressed`. Note: in the present arm `RenderAll` actually renders the chosen skill's real template; seed `Cfg.Vars` (and `Docs`) so that render does not trip the `<no value>` guard — in Phase 1 `roadmap-graduation` still cites `.vars.roadmapDoc` unguarded, so seed `roadmapDoc` (or pick a skill name and seed whatever unguarded vars its template references).
- [ ] Verify both arms of each new branch are covered (workflowRef enabled/fallback; suppression skip taken/not-taken).

### Task 1.5 — Verify & commit Phase 1

- [ ] Run `./x gate` — expect `coverage: 100.0%` and `0 issues`.
- [ ] Run `./x check` — expect `awf check: clean` (no rendered-output change this phase).
- [ ] Commit (gate runs via pre-commit hook):

```
git add internal/catalog/catalog.go internal/project/project.go internal/project/project_test.go
git commit -m "feat(awf): extend layout with doc paths and add doc-gated skill suppression

Adds .layout.docs/workflowRef/domainsDir and a RequiresDoc-driven render-set
skip. Backs inv layout-docs-enabled-only, workflow-ref-fallback,
domains-dir-given, doc-gated-skill-suppressed. ADR-0013."
```

---

## Phase 2 — Template & catalog migration; delete non-generic vars

One commit. Re-renders the standard; `roadmap-graduation` becomes suppressed in this repo (its rendered file is removed). Throughout, do **not** alter text other than as specified.

### Task 2.1 — Catalog: requiresDoc + remove the emptied section

- [ ] In `templates/catalog.yaml`, under `roadmap-graduation:`, add `requiresDoc: roadmap` as a sibling of `sections:`:

```yaml
  roadmap-graduation:
    requiresDoc: roadmap
    sections:
      - when-fires
```

- [ ] Under `executing-plans:` `sections:`, delete the line `      - host-git-constraint`.
- [ ] Under `subagent-driven-development:` `sections:`, delete the line `      - host-git-constraint`.

### Task 2.2 — Uniform workflow-doc swap

- [ ] In every file below, replace **all** occurrences of the literal `{{ .vars.workflowDoc }}` with `{{ .layout.workflowRef }}` (string replace, all occurrences):
  - `templates/skills/brainstorming/SKILL.md.tmpl`
  - `templates/skills/bugfix/SKILL.md.tmpl`
  - `templates/skills/debugging/SKILL.md.tmpl`
  - `templates/skills/executing-plans/SKILL.md.tmpl`
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl`
  - `templates/skills/proposing-adr/SKILL.md.tmpl`
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl`
  - `templates/skills/reviewing-plan/SKILL.md.tmpl`
  - `templates/skills/reviewing-adr/SKILL.md.tmpl`
  - `templates/skills/writing-plans/SKILL.md.tmpl`
  - `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`
  - `templates/agents-doc/AGENTS.md.tmpl`
- [ ] Verify none remain: `grep -rl '\.vars\.workflowDoc' templates/` must print nothing.

### Task 2.3 — Soften dangling workflow-doc anchors

After Task 2.2 these sites read `` `{{ .layout.workflowRef }}` "<anchor>" ``. Remove the trailing ` "<anchor>"` (the space, quotes, and quoted text only), leaving `` `{{ .layout.workflowRef }}` `` followed by its existing punctuation:

- [ ] `templates/skills/brainstorming/SKILL.md.tmpl` — drop `"Planning files"`.
- [ ] `templates/skills/executing-plans/SKILL.md.tmpl` — drop `"Regression-test discipline under the tier split"`, `"Planning files / Lifecycle"`, `"Auto-commit when green"`.
- [ ] `templates/skills/writing-plans/SKILL.md.tmpl` — drop `"Commit granularity vs gate cost"`.
- [ ] `templates/skills/refactor-coupling-audit/SKILL.md.tmpl` — drop `"Refactor playbook"` (twice) and `"Use subagents for exploration and file-heavy work"`.
- [ ] Verify no anchor still immediately follows the reference: `grep -rn 'workflowRef }}` "' templates/skills/` must print nothing. (A looser `workflowRef.*"` is not a clean signal — it also matches legitimate prose like `` `…workflowRef }}` step 6 ("Implementation")``, which is not an anchor and stays.)

### Task 2.4 — `stateDocsPath` → `domainsDir`; reword "state doc" → "domain doc"

- [ ] `templates/skills/brainstorming/SKILL.md.tmpl`: replace `Check state docs at `{{ .vars.stateDocsPath }}`.` with `Check domain docs under `{{ .layout.domainsDir }}`.`
- [ ] `templates/skills/proposing-adr/SKILL.md.tmpl`: replace `1. **Update or create the relevant state doc** under `{{ .vars.stateDocsPath }}` if the ADR materially shifts a domain's current state. Include this file in the same commit as the ADR.` with `1. **Update or create the relevant domain doc** under `{{ .layout.domainsDir }}` if the ADR materially shifts a domain's current state. Include this file in the same commit as the ADR.`
- [ ] `templates/skills/adr-lifecycle/SKILL.md.tmpl`: replace `1. **Update any state doc** under `{{ .vars.stateDocsPath }}` whose domain this ADR materially shifts.` with `1. **Update any domain doc** under `{{ .layout.domainsDir }}` whose domain this ADR materially shifts.`

### Task 2.5 — Guarded optional-doc citations (`debugging`, `pitfalls`)

- [ ] `templates/skills/debugging/SKILL.md.tmpl`: replace `{{ if .vars.debuggingDoc }} Full debugging recipes and surface-specific commands are in `{{ .vars.debuggingDoc }}`.{{ end }}` with `{{ if .layout.docs.debugging }} Full debugging recipes and surface-specific commands are in `{{ .layout.docs.debugging }}`.{{ end }}`
- [ ] `templates/skills/bugfix/SKILL.md.tmpl`: the `pitfallsDoc` site is currently **ungated**:

```
1. **Check `{{ .vars.pitfallsDoc }}` for known-tricky areas.** The pitfalls list catalogues recurring traps; verify the fix is not re-introducing one that bit before.
```

Replace with a guarded form (entire step omitted when the doc is disabled):

```
{{ if .layout.docs.pitfalls }}1. **Check `{{ .layout.docs.pitfalls }}` for known-tricky areas.** The pitfalls list catalogues recurring traps; verify the fix is not re-introducing one that bit before.
{{ end }}
```

(Confirm this step sits inside its own marker section; preserve the surrounding `<!-- awf:section … -->`/`<!-- awf:end -->` lines and the numbered-list flow.)

### Task 2.6 — `roadmap-graduation`: roadmapDoc → `.layout.docs.roadmap` (unguarded)

In `templates/skills/roadmap-graduation/SKILL.md.tmpl`, replace **all** occurrences of `{{ .vars.roadmapDoc }}` with `{{ .layout.docs.roadmap }}` — including the frontmatter `description:` on line 3. Unguarded is safe: the skill renders only when the roadmap doc is enabled (Task 2.1 + Phase 1 suppression).

- [ ] Verify: `grep -rl '\.vars\.roadmapDoc' templates/` prints nothing.

### Task 2.7 — Remove the `host-git-constraint` section (both skills)

- [ ] `templates/skills/executing-plans/SKILL.md.tmpl`: delete the block (including its blank-line separator):

```

<!-- awf:section host-git-constraint -->
{{ if .vars.hostGitAdrRef }}   **Git runs on the host, never in the container.** `git add` / `git commit` / `git push` run on the host shell only. The container git shim blocks write-mode git inside. (See `{{ .vars.hostGitAdrRef }}`.)
{{ end }}<!-- awf:end -->
```

- [ ] `templates/skills/subagent-driven-development/SKILL.md.tmpl`: delete the identical block (same four lines + separator).

### Task 2.8 — `executing-plans` project-invariants: drop keyInvariantAdrRef, generic-ify oracle

- [ ] In `templates/skills/executing-plans/SKILL.md.tmpl`, replace the `project-invariants` section body. Current:

```
<!-- awf:section project-invariants -->
{{ if .vars.keyInvariantAdrRef }}- **Project invariants are non-negotiable.** See `{{ .vars.keyInvariantAdrRef }}` for the key invariants that must be preserved across all commits.
{{ end }}{{ if .vars.oracleStateDoc }}- **The oracle state doc (`{{ .vars.oracleStateDoc }}`) is non-negotiable.** Never adjust expected output to make a test pass.
{{ end }}<!-- awf:end -->
```

Replace with:

```
<!-- awf:section project-invariants -->
- **Expected output is non-negotiable.** Never adjust an expected/golden value to make a test pass; fix the cause or the test's premise.
<!-- awf:end -->
```

### Task 2.9 — `subagent-driven-development`: drop perTaskReviewAdrRef, keyInvariantAdrRef, generic-ify oracle

- [ ] In `templates/skills/subagent-driven-development/SKILL.md.tmpl`, in the `per-task-review` section, delete the inline clause `{{ if .vars.perTaskReviewAdrRef }} (Convention: `{{ .vars.perTaskReviewAdrRef }}`){{ end }}` (leave the surrounding sentence intact).
- [ ] In the `notes` section, replace:

```
- One concern per commit; auto-commit when green.{{ if .vars.keyInvariantAdrRef }}
- **Project invariants are non-negotiable.** See `{{ .vars.keyInvariantAdrRef }}` for the key invariants that must travel into every dispatched subagent prompt.{{ end }}{{ if .vars.oracleStateDoc }}
- **The oracle state doc (`{{ .vars.oracleStateDoc }}`) is non-negotiable.** Never adjust expected output to make a test pass. This rule travels into the dispatched prompt verbatim.{{ end }}
```

with:

```
- One concern per commit; auto-commit when green.
- **Expected output is non-negotiable.** Never adjust an expected/golden value to make a test pass; this rule travels into the dispatched prompt verbatim.
```

### Task 2.10 — `brainstorming`: drop autonomousAdrRef & noDivingAdrRef clauses

- [ ] In `templates/skills/brainstorming/SKILL.md.tmpl`, remove `{{ if .vars.autonomousAdrRef }} (See `{{ .vars.autonomousAdrRef }}`.){{ end }}` (leave the preceding sentence and its period).
- [ ] Remove `{{ if .vars.noDivingAdrRef }} (See `{{ .vars.noDivingAdrRef }}`.){{ end }}` likewise.

### Task 2.11 — `workflow.md`: concise ADR trigger

- [ ] In `templates/docs/workflow.md.tmpl`, in the `chain` section, append a sentence after the existing closing paragraph (before `<!-- awf:end -->`):

```
For the detailed criteria of when a decision is load-bearing enough to warrant an ADR — and the ADR format itself — see [`{{ .layout.adrReadme }}`]({{ .layout.adrReadme }}).
```

### Task 2.12 — Add the `no-doc-path-vars` invariant test

- [ ] Add `TestNoDocPathVarsInTemplates` to `internal/project` (e.g. a new `templates_vars_test.go`): walk `templates.FS` for `*.tmpl`, assert none contain any of the 11 literals: `workflowDoc`, `debuggingDoc`, `pitfallsDoc`, `roadmapDoc`, `stateDocsPath`, `oracleStateDoc`, `autonomousAdrRef`, `hostGitAdrRef`, `keyInvariantAdrRef`, `noDivingAdrRef`, `perTaskReviewAdrRef`. Tag with `// invariant: no-doc-path-vars`.

### Task 2.13 — Extend the frontmatter render-all fixture

- [ ] In `internal/project/frontmatter_test.go`, the static layout fixture must seed the new members so unguarded references resolve: add `"workflowRef": "AGENTS.md"`, `"domainsDir": "docs/domains"`, and a `docs` map (`map[string]any`). Parametrise the test (or add a second case) over an empty-docs base layout **and** one with `docs: {workflow, pitfalls, debugging}` populated, asserting valid frontmatter and no `<no value>` in both.
- [ ] **Doc-gated skills cite `.layout.docs.<doc>` unguarded** (e.g. `roadmap-graduation` interpolates `.layout.docs.roadmap` into its frontmatter `description`). This test renders every catalog skill directly — it does **not** apply the `RenderAll` suppression — so for each skill it must add that skill's `cat.Skills[name].RequiresDoc` (when non-empty) to the `docs` map for that render, mirroring the suppression guarantee that such a skill renders only when its doc is enabled. Without this, `roadmap-graduation`'s `description` resolves to `<no value>` (a missing `map[string]any` key under `missingkey=zero`) under **both** parametrised cases and the test fails — reddening the Phase 2 gate. (Agents carry no `RequiresDoc`.)

### Task 2.14 — Re-render, verify, commit

- [ ] Run `./x sync` — expect `awf sync: done`. This removes `.claude/skills/awf-roadmap-graduation/SKILL.md` (now suppressed) and rewrites the migrated skills + `AGENTS.md` + `.claude/awf/awf.lock`.
- [ ] Run `./x check` — expect `awf check: clean`.
- [ ] Run `./x gate` — expect `coverage: 100.0%`, `0 issues`.
- [ ] Stage all changes including the deletion, then commit:

```
git add templates/ internal/project/ .claude/ AGENTS.md
git add -u .claude/skills/awf-roadmap-graduation
git commit -m "feat(awf): migrate doc cross-references to layout; drop non-generic vars

Swap workflowDoc->layout.workflowRef; debugging/pitfalls->guarded layout.docs;
stateDocsPath->layout.domainsDir (domain doc); roadmap-graduation cites
layout.docs.roadmap and is doc-gated. Delete oracleStateDoc + five *AdrRef vars
and the now-empty host-git-constraint section. Soften dangling workflow anchors;
add a concise ADR trigger to workflow.md. Backs inv no-doc-path-vars. ADR-0013."
```

---

## Phase 3 — Dogfood: enable docs, drop migrated vars

### Task 3.1 — Enable docs

- [ ] In `.claude/awf/config.yaml`, change the `docs:` list from:

```yaml
docs:
    - architecture
```

to:

```yaml
docs:
    - architecture
    - development
    - glossary
    - pitfalls
    - testing
    - workflow
```

### Task 3.2 — Remove deleted/migrated vars

- [ ] In `.claude/awf/config.yaml`, delete these keys from `vars:`: `autonomousAdrRef`, `debuggingDoc`, `hostGitAdrRef`, `keyInvariantAdrRef`, `noDivingAdrRef`, `oracleStateDoc`, `perTaskReviewAdrRef`, `pitfallsDoc`, `roadmapDoc`, `stateDocsPath`, `workflowDoc`. (Leave `invariantTestPath` and all command/format vars — they are out of scope.)

### Task 3.3 — Re-render, verify, commit

- [ ] Run `./x sync` — renders `docs/workflow.md`, `docs/testing.md`, `docs/development.md`, `docs/pitfalls.md`, `docs/glossary.md`; flips `workflowRef` to `docs/workflow.md` across rendered skills; surfaces the `bugfix` pitfalls step; rewrites root `AGENTS.md` (its workflow-rules link flips to `docs/workflow.md` and the Document map gains the five enabled docs); updates `.claude/awf/awf.lock`.
- [ ] Run `./x check` — expect `awf check: clean`.
- [ ] Run `./x gate` — expect green.
- [ ] Commit:

```
git add .claude/ docs/ AGENTS.md
git commit -m "feat(awf): enable workflow/testing/development/pitfalls/glossary docs

First-adopter dogfood of ADR-0013. Drops the migrated/deleted vars from config;
workflowRef now resolves to docs/workflow.md. ADR-0013."
```

> Note: `development`, `glossary`, and `pitfalls` render their template default (skeleton) content. Populating their sections via convention parts (`.claude/awf/docs/parts/<name>/<section>.md`) is optional follow-up, out of this plan's scope.

---

## Phase 4 — Flip ADR-0013 to Implemented

### Task 4.1 — Status flip + verify backings

- [ ] In `docs/decisions/0013-doc-cross-references-via-layout.md`, change frontmatter `status: Accepted` → `status: Implemented`.
- [ ] Run `./x sync` (moves ADR-0013 to the Implemented group in `ACTIVE.md`).
- [ ] Run `./x check` — now the invariant checker enforces ADR-0013's five tagged slugs (`no-doc-path-vars`, `layout-docs-enabled-only`, `workflow-ref-fallback`, `domains-dir-given`, `doc-gated-skill-suppressed`). Expect `awf check: clean` (all backed in Phases 1–2). If any slug is reported unbacked, add the missing `// invariant: <slug>` comment to the test that exercises it before committing.
- [ ] Run `./x gate` — expect green.
- [ ] Commit:

```
git add docs/decisions/0013-doc-cross-references-via-layout.md docs/decisions/ACTIVE.md .claude/awf/awf.lock
git commit -m "docs(adr): mark 0013 Implemented"
```

### Task 4.2 — Terminal step

- [ ] Invoke `awf-reviewing-impl` against the Phase 1–4 commit range.

---

## Verification checklist (end state)

- [ ] `grep -rl '\.vars\.\(workflowDoc\|debuggingDoc\|pitfallsDoc\|roadmapDoc\|stateDocsPath\|oracleStateDoc\|autonomousAdrRef\|hostGitAdrRef\|keyInvariantAdrRef\|noDivingAdrRef\|perTaskReviewAdrRef\)' templates/` prints nothing.
- [ ] `./x check` clean; `./x gate` green at 100% coverage.
- [ ] `.claude/skills/awf-roadmap-graduation/` is absent (suppressed; roadmap doc deferred to the domain-docs ADR).
- [ ] `docs/workflow.md`, `docs/testing.md`, `docs/development.md`, `docs/pitfalls.md`, `docs/glossary.md` exist and render cleanly.
- [ ] ADR-0013 `Implemented` in `ACTIVE.md`.
