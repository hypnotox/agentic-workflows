---
date: 2026-07-30
adrs: [0187]
status: Proposed
---
# Plan: Orienting support skill (ADR-0187)

## Goal

Implement ADR-0187 in full: the `orienting` support skill exists in the compile-time catalog and template tree as the single home of the orientation procedure, the grounding-checker contract shares its ladder, the scattered inline copies shrink to references, schema generation 26 backfills the skill wherever brainstorming is enabled, and all six declared claim operations apply with the ADR flipping directly to Implemented. Non-goals: the reviewing-plan-resync merge question, enabling roadmap-graduation in this repo, and cutting the 0.30.0 release itself.

## Architecture summary

Four transactions. First the skill comes into existence everywhere at once (catalog spec and profile, template, shared partial, grounding-checker include, enablement and render in this repo). Second the existing orientation prose shrinks to references (brainstorming, proposing-adr, writing-plans, workflow doc template, this repo's local guide part). Third the config machinery lands (bespoke gen-26 migration, min-version and version bump, upgrades and re-renders of both in-repo trees). Fourth the six claim operations are authored with proof markers and the ADR and this plan flip to Implemented in one direct transaction. The direct single-batch application path is intentional: every operation's backing test exists by the end of phase 3, so nothing is gained by incremental Implementing batches.

## File structure

- **Created:**
  - `templates/partials/orientation-ladder.md`
  - `templates/skills/orienting/SKILL.md.tmpl`
  - `internal/migrate/orientingbackfill.go`
  - `internal/migrate/orientingbackfill_test.go`
  - rendered outputs via `./x render`: `.claude/skills/awf-orienting/SKILL.md`, `.pi/skills/awf-orienting/SKILL.md` (and the sundial equivalents after its upgrade)
- **Modified:**
  - `internal/catalog/standard.go`
  - `templates/agents/grounding-checker.md.tmpl`
  - `templates/skills/brainstorming/SKILL.md.tmpl`
  - `templates/skills/proposing-adr/SKILL.md.tmpl`
  - `templates/skills/writing-plans/SKILL.md.tmpl`
  - `templates/docs/workflow.md.tmpl`
  - `internal/evals/chain_test.go`
  - skill-enumerating test literals in `internal/project/` (batch task 1.5)
  - `internal/migrate/migrate.go`
  - `internal/project/project.go`, `internal/project/version_test.go`
  - `changelog/CHANGELOG.md`
  - `.awf/config.yaml`, `.awf/awf.lock`, `.awf/parts/agents-doc/commands.md`
  - `examples/sundial/.awf/config.yaml`, `examples/sundial/.awf/awf.lock`, sundial rendered outputs
  - `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, `.awf/topics/parts/config/migrations-and-locks/current-state.md` and their rendered `docs/topics/` counterparts
  - `AGENTS.md`, `docs/workflow.md`, rendered `.claude/skills/` and `.pi/skills/` bodies touched by the shrinks
  - `docs/decisions/0187-add-the-orienting-support-skill-as-the-single-home-of-orientation.md`, `docs/decisions/INDEX.md`, this plan (status flips)
- **Deleted:** none.

## Phase 1: The orienting skill exists

**Execution mode: inline.** One transaction: catalog entry, template, shared partial, grounding-checker include, tests, and this repo's enablement land together so the section-parity and profile-validation invariants hold at the phase close.

- [ ] **Task 1.1: Create `templates/partials/orientation-ladder.md`.** Exact content (no `awf:include` directive may appear inside it; the include engine rejects nested includes):

  ```
  Ground guide-first, in order: the agent guide, then the document-map docs relevant to the touched area, then its domain docs, then the recent history of the touched paths (`git log --oneline -20 <path>`). For managed `awf context` calls, start bare: directories provide tier-0 orientation, while exact, staged, and range-selected files also carry tier-1 direct relationships. Request only the named facets required by the active lens, and never prescribe `--full`.
  ```

- [ ] **Task 1.2: Create `templates/skills/orienting/SKILL.md.tmpl`.** Exact content:

  ```
  ---
  name: {{ .prefix }}-orienting
  description: Use when taking up a topic - before brainstorming fresh non-trivial work, when resuming an effort, when taking over a handoff, or when the working set widens mid-chain. Grounds the session in repository truth; single-pass, never a chain gate.
  ---

  # {{ .prefix }}-orienting

  <!-- awf:section when-to-invoke -->
  ## When to invoke

  Four moments call for orientation:

  1. **Fresh work:** before brainstorming any non-trivial outcome.
  2. **Effort resume:** taking up an in-progress effort in a new session, after context summarization, or on re-entering a managed worktree.
  3. **Handoff takeover:** receiving work another session checkpointed.
  4. **Mid-chain re-orientation:** the working set widens into unexamined files or domains, or a durable artifact (an ADR's Context or State changes, a plan's tasks) is about to cite repository facts not verified in the current session.

  This is a support skill: single-pass and advisory, never a chain gate or prerequisite. It never creates an effort, never commits, and never edits shared working memory unless this session is the effort's one user-managed writer.
  <!-- awf:end -->

  <!-- awf:section guide-ladder -->
  ## Grounding ladder

  <!-- awf:include orientation-ladder -->

  When a needed fact's location is unknown and inline search would pollute the parent context, dispatch one or more exploration subagents as fitting: each carries exactly one information need with a chosen breadth and report detail, independent needs may run in parallel, and every child is report-only.
  <!-- awf:end -->

  <!-- awf:section context-command -->
  ## Managed context

  Once candidate files are identified, run `awf context <paths>` to resolve their owning domains and the applicable current-state claims; read the topics and any Accepted pending changes it surfaces, and the ADRs behind a claim only when the rationale matters.
  <!-- awf:include context-spill -->
  <!-- awf:end -->

  <!-- awf:section resume-revalidation -->
  ## Resume revalidation

  When resuming an effort or taking over a handoff, read the memory header (`Effort:`, `Phase:`, `Next:`, `Updated:`) and the handoff log, then verify every load-bearing claim against repository truth before acting on it: commits landed since the checkpoint (`git log` since `Updated:`), worktree topology versus what memory describes (`git worktree list`), cited decision-record statuses against the decision index, and cited plan and file existence. A discrepancy resolves in favor of the repository. Only the effort's one user-managed writer corrects the stale checkpoint before continuing; a dispatched child never edits it.
  <!-- awf:end -->

  <!-- awf:section hand-off -->
  ## Hand-off

  Orientation produces understanding, never commits. Route onward to whichever enabled skill fits the work: brainstorming for fresh design, debugging for a defect, plan execution to continue an accepted plan, plan or ADR writing when a durable artifact is next.
  <!-- awf:end -->
  ```

  Note on the fenced block above: the file carries the plain Go-template placeholder exactly as every sibling `SKILL.md.tmpl` does (compare `templates/skills/exploring/SKILL.md.tmpl`). The description line must not contain a `: ` sequence after the frontmatter key itself (YAML plain-scalar constraint enforced by the templates-valid-frontmatter invariant); the hyphenated form above satisfies that.

- [ ] **Task 1.3: Add the catalog entries in `internal/catalog/standard.go`.** In the `Skills` map, after the `"exploring"` entry:

  ```go
  "orienting": {Core: true, Sections: []string{
      "when-to-invoke", "guide-ladder", "context-command", "resume-revalidation", "hand-off",
  }},
  ```

  In the `init()` profiles map, after the `"exploring"` entry:

  ```go
  "orienting": {Kind: WorkflowSupport, Purpose: "Ground the session in a topic before starting, resuming, or widening work.", Trigger: "Use when taking up a topic: before brainstorming fresh non-trivial work, when resuming an effort, or when taking over a handoff.", CommonFollowUps: []string{"brainstorming", "debugging", "writing-plans", "executing-plans"}},
  ```

  No `RequiresAgent`, no `RequiresDoc`, no `UsuallyFollows`, and no edits to any other skill's profile (ADR-0187 Decision 1).

- [ ] **Task 1.4: Share the ladder with the grounding-checker.** In `templates/agents/grounding-checker.md.tmpl`, inside the `verification-scope` section, append after the convention-fit paragraph:

  ```
  Ground guide-first before verifying:

  <!-- awf:include orientation-ladder -->
  ```

  The include directive sits directly in the agent template, never inside the partial (nested includes hard-fail the render).

- [ ] **Task 1.5: Batch task - extend every skill-enumerating test.** Representative (exact): in `internal/evals/chain_test.go`, the `roles` map in `TestUnifiedEffortWorkflowCoverage` gains the entry `"orienting": "report",` beside `"exploring": "report"`. Edge (exact): in `internal/project/target_test.go`, the config literal returned by the helper at the line currently reading `skills: [adr-lifecycle, brainstorming, ...]` gains `orienting` in alphabetical position. Affected-site set: every test literal or map that enumerates the standard or core skill set, found by `grep -rn '"exploring"' internal/ --include='*_test.go'` plus the failures the post-check surfaces (known candidates: `internal/project/target_test.go`, `internal/project/spine_test.go` including its template table entry `skills/orienting/SKILL.md.tmpl` beside `skills/exploring/SKILL.md.tmpl`, `internal/project/skillrefs_test.go`, `internal/project/scaffold_test.go`, `internal/catalog/graph_test.go`, `internal/catalog/catalog_test.go`). Post-check (deterministic): `go test ./...` exits zero.

- [ ] **Task 1.6: Add the orienting contract test.** In `internal/project/spine_test.go`, following the shape of the existing exploring golden assertions (the `renderSkillGolden` calls), add `TestOrientingSkillContract`: render the orienting skill for the claude and pi targets and assert the body contains, at minimum, the literal fragments `"Four moments call for orientation"`, `"Ground guide-first, in order"`, `"one or more exploration subagents"`, `"one information need"`, `"A discrepancy resolves in favor of the repository"`, `"never creates an effort, never commits"`, and `"never prescribe \`--full\`"`; assert the grounding-checker agent render contains `"Ground guide-first, in order"` (proving the shared partial reaches both consumers). No proof marker yet; markers land with the claims in phase 4.

- [ ] **Task 1.7: Enable and render in this repo.** Run `./awf enable skill orienting`, then `./x render`, then `./x check` (expected: clean). The rendered `.claude/skills/awf-orienting/SKILL.md` and `.pi/skills/awf-orienting/SKILL.md` exist and `AGENTS.md` lists the skill with its purpose and trigger.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(rendering): add the orienting support skill
```

## Phase 2: Reference-and-shrink

**Execution mode: inline.** One transaction: all five prose homes shrink together with their re-render, so the single-home story is coherent at the phase close.

- [ ] **Task 2.1: Shrink brainstorming step 1.** In `templates/skills/brainstorming/SKILL.md.tmpl`, replace the whole bare step-1 block (the line beginning `1. **Explore project context.**` plus its trailing `<!-- awf:include context-spill -->` line) with exactly:

  ```
  1. **Orient in the topic.** Invoke `{{ .prefix }}-orienting` and follow its ladder: guide-first grounding, delegated exploration where fitting, and managed context over the candidate files the work touches.
  ```

  (Plain placeholder in the file, as in Task 1.2's note.) Step 6's own `awf context` paste instruction and its context-spill include are untouched: brainstorming remains a managed context-calling skill there.

- [ ] **Task 2.2: Advisory pointer in proposing-adr.** In `templates/skills/proposing-adr/SKILL.md.tmpl`, inside the `when-to-invoke` section, append as a final paragraph:

  ```
  When grounding is stale - the ADR will cite repository facts not verified in the current session - invoke `{{ .prefix }}-orienting` before writing.
  ```

- [ ] **Task 2.3: Advisory pointer in writing-plans.** In `templates/skills/writing-plans/SKILL.md.tmpl`, inside the `procedure-confirm-scope` section, append to step 1:

  ```
  If the plan is written in a later session than the brainstorm, or the file structure reaches areas not examined this session, invoke `{{ .prefix }}-orienting` first.
  ```

- [ ] **Task 2.4: Route the workflow doc's resume sentence.** In `templates/docs/workflow.md.tmpl`, in the working-memory section, replace the sentence `Resume from `Phase:` and `Next:` only after checking the repository sources and current-state documentation, which remain authoritative over checkpoint prose.` with:

  ```
  Resume from `Phase:` and `Next:` only after revalidating them against the repository sources and current-state documentation, which remain authoritative over checkpoint prose; the rendered orienting skill's resume-revalidation section is the procedural home of that check.
  ```

- [ ] **Task 2.5: Shrink this repo's guide part.** Replace the managed-context paragraph in `.awf/parts/agents-doc/commands.md` (the paragraph beginning `For managed `awf context` calls, start bare:`) with exactly:

  ```
  For managed `awf context` calls, follow the awf-orienting skill's context discipline: start bare, request only the named facets the active lens requires, never prescribe `--full`, and consume spill notices per the shared contract.
  ```

- [ ] **Task 2.6: Batch task - re-render and settle assertions.** Run `./x render`; the rendered brainstorming, proposing-adr, and writing-plans skills, `docs/workflow.md`, and `AGENTS.md` update. Representative (exact): any spine assertion pinning brainstorming's old step-1 text (for example an expected `Invoke `example-exploring`` fragment sourced from step 1) moves to assert the same behavior in the orienting render instead. Affected-site set: every test assertion the post-check fails on brainstorming step-1 prose, the workflow-doc sentence, or context-call classification (known candidates: `internal/project/spine_test.go`, the projection-pinning spine test behind implementer-context-grounding, `internal/evals/chain_test.go` ordered-fragment lists). Constraint: never weaken an assertion; relocate it to the surface the prose moved to. Post-check (deterministic): `go test ./...` exits zero and `./x check` is clean.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(rendering): shrink orientation prose to the orienting skill
```

## Phase 3: Schema generation 26

**Execution mode: inline.** One transaction: migration, version obligations, changelog, and both in-repo upgrades land together so the binary-version gate is consistent at the phase close.

- [ ] **Task 3.1: Create `internal/migrate/orientingbackfill.go`.** Exact content:

  ```go
  package migrate

  import (
      "fmt"
      "io"
      "os"
      "slices"

      "github.com/hypnotox/agentic-workflows/internal/config"
  )

  // applyOrientingSkillBackfill ports schema 25 -> 26 (ADR-0187): the orienting
  // skill becomes the single home of the orientation procedure and the shrunk
  // brainstorming template invokes it by name, a prose reference no structural
  // edge backs (requires-skills-exact forces RequiresSkills empty, and orienting
  // declares no agent or doc requirement), so applyCloseEnabledSet cannot reach
  // it. Any config with brainstorming enabled gains orienting; a config without
  // brainstorming is untouched. Idempotent; the addition is announced.
  func applyOrientingSkillBackfill(root string, out io.Writer) error {
      if _, err := os.Stat(config.ConfigPath(root)); os.IsNotExist(err) {
          return nil // no config: nothing to backfill (idempotent re-run safe)
      }
      cfg, err := loadForMigration(root)
      if err != nil {
          return err
      }
      if !slices.Contains(cfg.Skills, "brainstorming") || slices.Contains(cfg.Skills, "orienting") {
          return nil
      }
      return editConfig(root, func(src []byte) ([]byte, error) {
          b, err := config.SetArrayMember(src, "skills", "orienting", true)
          if err != nil { // coverage-ignore: config.Load already parsed this config, so SetArrayMember cannot error here
              return nil, err
          }
          fmt.Fprintln(out, "orienting-skill-backfill: enabled skill orienting (brainstorming is enabled)")
          return b, nil
      })
  }
  ```

  Adjust the coverage-ignore reason only if the gate rejects it; never fork `editConfig` or `SetArrayMember` to dodge coverage.

- [ ] **Task 3.2: Register generation 26.** In `internal/migrate/migrate.go`, append to `registry`:

  ```go
  {To: 26, Name: "orienting-skill-backfill", Apply: applyOrientingSkillBackfill},
  ```

  `ConfigForCurrentSchema` needs no generation-26 byte branch: the migration only adds a list entry the strict parser already accepts, so a historical committed config still parses (contrast the severity-key removal, which removed keys from the model and did need one). State this in the commit body.

- [ ] **Task 3.3: Create `internal/migrate/orientingbackfill_test.go`.** Table-driven over config fixtures, following the sibling migration tests' fixture style: (a) brainstorming enabled without orienting: config gains `orienting` in the skills array and the run prints the exact announcement line; (b) brainstorming and orienting both enabled: config bytes unchanged, no output; (c) no brainstorming: config bytes unchanged, no output; (d) no config file at root: nil error, no output; (e) idempotence: running twice equals running once. No proof marker yet; it lands in phase 4.

- [ ] **Task 3.4: Version obligations.** In `internal/project/project.go`: `Version` becomes `"0.30.0"` and `minVersionBySchema` gains `26: "0.30.0",`. In `internal/project/version_test.go`: the generation-pin assertion moves from `minVersionBySchema[25]` to `minVersionBySchema[26]` (still compared against `Version`); the historical `[20]` assertions stay.

- [ ] **Task 3.5: Changelog.** In `changelog/CHANGELOG.md` under `## [Unreleased]`, add to Features (creating the category under Unreleased if absent):

  ```
  - Add the `orienting` support skill: the single home of the orientation procedure (guide-first
    grounding ladder, managed `awf context` discipline, and effort-resume revalidation against
    repository truth), shared with the grounding-checker contract via a template partial. Schema
    generation 26 enables it in any config that has `brainstorming` enabled, since the shrunk
    brainstorming template now invokes it by name; configs without `brainstorming` are untouched.
  ```

- [ ] **Task 3.6: Upgrade both in-repo trees.** From the repo root run `./awf upgrade` (expected: the lock advances to generation 26; the backfill prints nothing here because orienting is already enabled - live proof of case (b)). From `examples/sundial/` run `./awf upgrade` (expected: the exact announcement line prints and sundial's config gains `orienting` - live proof of case (a)), then sundial's `./x render` and `./x check` (expected: clean; the rendered sundial orienting skill and guide entries appear). Back at the root, `./x render` and `./x check` (expected: clean).

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(config)!: backfill the orienting skill at schema generation 26
```

## Phase 4: Claims, markers, and the flip

**Execution mode: inline.** One transaction: all six operations of ADR-0187 apply as a single direct Implemented batch, with proof markers and topic renders in the same commit.

- [ ] **Task 4.1: Proof markers.** Add `// invariant: rendering/workflow-skill-templates:orienting-single-home` above `TestOrientingSkillContract` (Task 1.6) and `// invariant: config/migrations-and-locks:orienting-skill-backfill` above the Task 3.3 test's top-level function. The three updated claims keep their existing proof markers; extend the marked tests only if phase 1 or 2 has not already routed the new assertions through them.

- [ ] **Task 4.2: Author the claim mutations.** In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`:
  - Add claim `orienting-single-home` with prose: `The orienting support skill is the single home of the orientation procedure: its rendered body defines the four invocation moments, the guide-first grounding ladder shared as a partial with the grounding-checker contract, multi-child report-only exploration dispatch with one information need per child, the managed context discipline, and effort-resume revalidation that resolves discrepancies in favor of the repository; the skill is single-pass, never a chain gate, never creates an effort, and never commits. Brainstorming's first step invokes it, and proposing-adr and writing-plans carry advisory pointers.` `Origin: ADR-0187`, `Backing: test`.
  - Update `explorer-and-grounding-role-contracts`: append to its prose `The grounding-checker body grounds guide-first through the shared orientation ladder partial.` and append ADR-0187 to `Revised-by`.
  - Update `implementer-context-grounding`: in the sentence enumerating bare-context callers, add orientation so it reads `Brainstorming, orientation, implementation, planning, debugging, test-first, and refactor-orientation calls start with bare  awf context ` (backticked command as in the current prose); append ADR-0187 to `Revised-by`.
  - Update `unified-effort-workflow-coverage`: extend the enumeration `... coupling-audit, exploration, orientation, and roadmap skill ...`; append ADR-0187 to `Revised-by`.

  In `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, update `working-memory-single-home`: append `Resume verification is procedurally homed in the orienting skill's resume-revalidation section; the workflow doc keeps the memory contract and routes to it.` and append ADR-0187 to `Revised-by`.

  In `.awf/topics/parts/config/migrations-and-locks/current-state.md`, add claim `orienting-skill-backfill` with prose: `The schema-26 migration enables the orienting skill in any config that has brainstorming enabled, as a bespoke idempotent atomic edit announced per addition; configs without brainstorming are untouched, and the closure primitive is not used because no structural edge reaches orienting.` `Origin: ADR-0187`, `Backing: test`.

- [ ] **Task 4.3: Flip and render.** In the ADR, append to Status history a direct Implemented event for all six operations in declaration order, carrying the frozen content digest and the batch state sequence in the exact encoding `awf check` validates (the adr-lifecycle contract; compute the sha over the ADR body per the existing Implemented ADRs' precedent). Flip this plan's `status:` to `Implemented` and record any beyond-plan findings in its Notes. Run `./x render` (INDEX.md and `docs/topics/` regenerate). `./x check` is clean.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
docs(adr): apply 0187 state changes and flip to Implemented
```

## Verification

- `./x gate` green at every phase close (100% coverage, dead-code, prose, memory, drift).
- `./awf check` clean at the repo root and in `examples/sundial` after phase 3.
- `git grep -n "Explore project context" templates/ .claude/ .pi/` returns no output after phase 2 (the shrink left no stale copy).
- `./awf list` shows `orienting  enabled` at the root; sundial's list shows the same after phase 3.
- The rendered `.claude/skills/awf-orienting/SKILL.md` and the grounding-checker contract both contain the ladder sentence `Ground guide-first, in order` (shared partial reaches both consumers).
- `awf audit` over the implementation range reports no workflow-conformance findings (advisory).

## Notes

- Out of scope, tracked elsewhere: the reviewing-plan-resync merge candidate, roadmap-graduation's disabled state in this repo, cutting and publishing the 0.30.0 release (docs/releasing.md), and the reviewer-spine dedup.
- After ADR-0186, `orienting-single-home` is the twenty-first `rendering/workflow-skill-templates` claim, exceeding the twenty-claim advisory limit. The user approved this exception for the cohesive topic; the existing `maxClaimsPerTopic` behavior is advisory, and any budget redesign remains future cleanup.
- Indicative only: the batch tasks' known-candidate lists were surveyed at authoring time; the affected-site sets are defined by their greps and post-checks, not by those lists.
