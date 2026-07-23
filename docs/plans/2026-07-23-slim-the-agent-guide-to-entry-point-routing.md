---
date: 2026-07-23
adrs:
  - 0157
status: Proposed
---
# Plan: Slim the agent guide to entry-point routing

## Goal

Implement ADR-0157: the guide template's workflow section becomes a catalog-derived entry-skill trigger table, its working-memory and awf-setup sections shrink to routing minimums, the working-memory protocol relocates to a new canonical `working-memory` section in the workflow doc with all pointer surfaces re-pointed, the authoring standard codifies the shape, and this repo's convention parts conform. Non-goals: no workflow-chain skill body changes beyond the single pointer sentence, no changes to the invariants or document-map guide sections, no new skill artifact.

## Architecture summary

Design rationale lives in ADR-0157. Execution order: (1) build the canonical home and re-point every pointer surface, (2) slim the guide template and retarget/author proofs, (3) codify the standard and fold setup detail into the usage doc, (4) conform this repo's parts and close out. Claim operations apply as V2 batches aligned with the phase that makes each claim true, and the partition keeps the Implementing state legal (at least one applied and one remaining operation at every intermediate commit): the Implementing event plus a first Applied batch (update `workflow-chain-adr-before-plan`) in phase 1's commit, a middle Applied batch (update `plan-task-detail-modes`) in phase 2's commit, and the final Applied batch (both `add` operations) paired with the Implemented flip in phase 4's commit. State-sequence numbers are repo-global, unique, and contiguous: never hardcode them; use the next value awf reports at execution time (the in-flight 0156 effort shares this checkout and may advance the counter first).

## File structure

- **Created:**
  - `.awf/parts/workflow/working-memory.md` (repo part: Pi effort-identity/ledger sentences for the new section)
- **Modified:**
  - `templates/docs/workflow.md.tmpl`, `templates/agents-doc/AGENTS.md.tmpl`, `templates/docs/agents-md-standard.md.tmpl`, `templates/docs/working-with-awf.md.tmpl`
  - `templates/partials/checkpoint-routine.md`, `templates/partials/checkpoint-approval.md`, `templates/skills/brainstorming/SKILL.md.tmpl`
  - `internal/catalog/catalog.go`, `internal/catalog/standard.go`, `internal/project/render.go`
  - `internal/project/spine_test.go`, `internal/project/plan_detail_modes_test.go`, `internal/project/docs_sections_test.go`, `internal/catalog/catalog_test.go`
  - `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`
  - `.awf/parts/agents-doc/awf-setup.md`, `.awf/parts/agents-doc/commands.md`, `.awf/parts/agents-doc/identity.md`, `.awf/parts/workflow/chain.md`, `.awf/parts/working-with-awf/commands.md`
  - `changelog/CHANGELOG.md`, `docs/decisions/0157-slim-the-agent-guide-to-entry-point-routing.md`, this plan (status flip)
  - Rendered outputs via `./x sync` in every phase: `AGENTS.md`, `docs/workflow.md`, `docs/working-with-awf.md`, `docs/agents-md-standard.md`, `docs/decisions/INDEX.md`, `docs/topics/rendering/workflow-skill-templates.md`, `docs/topics/rendering/guide-and-doc-templates.md`, `docs/domains/rendering.md`, `.awf/awf.lock`, `.claude/skills/**`, `.pi/awf-workflows/**`, and the `examples/sundial` adopter renders (`examples/sundial/AGENTS.md`, `examples/sundial/docs/workflow.md`, `examples/sundial/docs/working-with-awf.md`, `examples/sundial/docs/agents-md-standard.md`, its five target skill trees, and `examples/sundial/.awf/awf.lock`)
- **Deleted:**
  - `.awf/parts/agents-doc/workflow.md` (superseded by the template Pi branch; recorded against ADR Decision 9 at resync)

## Disposition table

Authoritative move-not-delete record for every evicted guide-default paragraph (line refs are pre-change `templates/agents-doc/AGENTS.md.tmpl`). "Delete" means the prose already renders at the cited canonical home; "move" means this plan relocates it. Owning claim and proof site named where one exists.

| # | Source (guide default) | Disposition | Canonical home | Owning claim / proof |
|---|---|---|---|---|
| 1 | awf-setup toggle detail (L11: kinds list, singleton toggles, requirement closure) | delete | `docs/working-with-awf.md` overview + commands + config-and-overrides sections (task 3.2 verifies and folds any missing clause) | none |
| 2 | awf-setup override-part + sync bullets (L12-14) | keep, compressed to the body in task 2.3 | guide (compressed) + `docs/working-with-awf.md` | none |
| 3 | workflow chain diagram (L57-59) | delete | `docs/workflow.md` chain section L14-16 | `workflow-chain-adr-before-plan`; proof moves in task 1.5 |
| 4 | warrant/resync/retrospective prose (L61 first half) | delete | `docs/workflow.md` chain section L18 | `workflow-chain-surfaces-resync`; proof moves in task 1.5 |
| 5 | plan task-form contract (L61 middle: "A plan may use exact content/diffs ... no prior conversation context") | delete | writing-plans skill, plans README, plans template, plan-reviewer (all proof-carrying) | `plan-task-detail-modes`; guide cases removed in task 2.6 |
| 6 | review/terminal-review sentence + chain-skill roster (L61 end, L63) | delete/replace | chain self-routing via skill terminal steps; roster replaced by trigger table (task 2.2) | new `guide-entry-point-routing`; proof task 2.5 |
| 7 | V2 batch semantics (L65) | delete | adr-lifecycle, executing-plans, subagent-driven-development skill templates | none (skill bodies unchanged) |
| 8 | Pi telemetry workflow paragraph (L67) | delete | `docs/workflow.md` chain section L20 (Pi-gated) renders it | none |
| 9 | exploration dispatch policy (L69 first sentence) | delete | exploring/brainstorming/debugging/refactor-coupling-audit skill bodies (test-asserted, `internal/project/target_test.go:722-729`) | none |
| 10 | concurrency/refinement + lower-cost-model sentences (L69 middle) | move | `docs/workflow.md` chain section, task 1.2 | none |
| 11 | sequential-implementation-subagent guidance (L69 end) | delete | `templates/docs/working-with-awf.md.tmpl:192-195` (Pi-subagents section) | none |
| 12 | gate sentence (L71 first half) | delete | guide invariants section L40 (unconditional) | none |
| 13 | "Conventional Commits; one concern per commit." + workflow pointer (L71 end) | keep verbatim | guide (unconditional closing line) | fallback pin `spine_test.go:1147` |
| 14 | working-memory intro + check-on-demand + resume rules (L77-79) | keep, compressed to the body in task 2.4; full prose moves to the workflow doc (task 1.2) | guide (compressed) + `docs/workflow.md` working-memory section | new `working-memory-single-home` |
| 15 | JIT retrieval bullet (L80) | move | `docs/workflow.md` working-memory section (task 1.2) | new `working-memory-single-home` |
| 16 | boundary/check-in protocol bullet (L81) | delete (chain-section copy relocates into the working-memory section, task 1.2) | `docs/workflow.md` working-memory section; chain skills embed the protocols | `memory-checkpoint-chain-coverage`, `mandatory-approval-boundaries` (untouched) |
| 17 | file skeleton bullet (L82) | move | `docs/workflow.md` working-memory section (task 1.2) | new `working-memory-single-home`; proof task 2.5 |
| 18 | ground rules bullet (L83) | move (one-liners kept in guide per ADR item 2) | `docs/workflow.md` working-memory section (task 1.2) | new `working-memory-single-home` |

Repo-part dispositions (phase 4): `commands.md` extension prose moves to `.awf/parts/working-with-awf/commands.md`; `identity.md` Pi-telemetry sentences delete (canonical: `docs/domains/rendering.md` narrative and the Pi topic claims); `workflow.md` part deletes (router paragraph superseded by the template Pi branch from task 2.2; dashboard prose already in `.awf/parts/workflow/chain.md` paragraph 2; deviation from ADR Decision 9 recorded at resync); `working-memory.md` part shrinks to the Pi resume pointer per ADR Decision 9, with its unique protocol sentences merged into `.awf/parts/workflow/working-memory.md` (task 1.4); `chain.md` part paragraph 3 moves there too; `awf-setup.md` re-trims against the slimmed default preserving the current runner/bootstrap sentences.

## Phase 1: canonical working-memory home

- [ ] **Task 1.1: Extend the workflow doc's catalog sections.** In `internal/catalog/standard.go`, change the `workflow` doc descriptor's `Sections` to insert `"working-memory"` after `"chain"`, giving `{"principles", "chain", "working-memory", "commit-discipline", "doc-currency", "composing-the-gate", "local-hooks", "ci"}`. `TestAdrSingletonSectionParity` (via `plainSingletons`) enforces catalog/template parity, so this lands with task 1.2 in the same commit.

- [ ] **Task 1.2: Add the working-memory section and slim the chain section in `templates/docs/workflow.md.tmpl`.** Insert after the chain section's `<!-- awf:end -->`:

  ```
  <!-- awf:section working-memory -->
  ## Working memory

  Session context is volatile; the chain's working state must not be. `.awf/memory/` (kept out of version control by a rendered self-ignoring `.gitignore`) holds one working-memory file per in-flight effort: `.awf/memory/<effort-slug>.md`. Check it on demand, not as a fixed startup step: when a request implies earlier work to continue, or when a fresh or context-compacted session finds it non-empty and unaccounted for. If an effort file matches, resume from its recorded `Phase:`/`Next:` lines instead of restarting; if several match or none can be verified against in-flight work, ask the user which (if any) to resume; never silently resume a stale effort. While working, prefer just-in-time retrieval: hold lightweight identifiers (file paths, ADR numbers, doc names) in the file and read the sources on demand.

  Checkpoints are durable; check-ins are deliberate. A boundary runs three steps in order: persist working memory first (an existing or deliberately created file updates in its own tool batch), decide whether user attention is required (authority drift, materially changed choices, scope expansion, unresolved correctness or safety concerns, blockers, failed required verification), then either raise a check-in and stop or state a continuity notice and continue. A routine checkpoint summary is a continuity notice, not a stop. The end of brainstorming and the settled ADR review are the two mandatory approval check-ins: present the summary, explicitly request approval, and continue only after approval is granted and persisted.{{if .targetSessionHandoff}} On the routine clear branch, Pi TUI guidance then invokes `handoff_session` alone with the validated memory path and immediate successor kickoff; automatic parent-linked continuation follows unless the user cancels during its five-second window, and a failed handoff leaves the checkpoint valid and becomes a check-in.{{else}} Target-native continuation follows without an unsupported session-replacement claim.{{end}} Long implementation phases repeat the routine protocol after independently resumable committed and reviewed tasks.

  File skeleton (a convention, not a schema; no tool parses it): a header (`# <effort title>`, `Phase:`, `Next:`, `Updated:`), then `## Brief` (the evolving design brief: problem, settled decisions, user constraints verbatim, rejected approaches), `## Handoff log` (one line per completed phase), and `## Scratch` (open questions, references).

  Ground rules: the file is session state, never a design artifact. Never commit it, never cite it in an ADR, plan, or commit message, and delete it when the effort's chain terminates; the retrospective alone deletes the file. Files orphaned by an abandoned effort are harmless gitignored residue; delete them when noticed. `awf uninstall` leaves a non-empty `.awf/memory/` in place.
  <!-- awf:end -->
  ```

  In the chain section (L18), replace the span from `Throughout, each chain skill checkpoints its position` through `The retrospective alone deletes the file.` (including the embedded `targetSessionHandoff` conditional, now carried by the new section) with:

  ```
  Throughout, each chain skill checkpoints its position (and brainstorming its evolving design brief) to a working-memory file under `.awf/memory/`, so a session death or context compaction resumes instead of restarting; the working-memory section below is the canonical home of the checkpoint protocol, file skeleton, and ground rules. Independent fresh-context exploration may run concurrently where supported, but refinement of an earlier result stays sequential; a lower-cost child model is selected deliberately when the runtime supports it, and shared-checkout implementation stays alone.
  ```

  Constraints: the Pi telemetry paragraph (L20) and every other section stay byte-identical; the chain diagram and warrant prose stay; no new vars (publication safety unchanged).

- [ ] **Task 1.3: Re-point the three pointer surfaces.** In `templates/partials/checkpoint-routine.md` L4 and `templates/partials/checkpoint-approval.md` L4, replace the exact sentence `The file skeleton and ground rules live in the agent guide's working-memory section.` with `The file skeleton and ground rules live in the workflow doc's working-memory section.` In `templates/skills/brainstorming/SKILL.md.tmpl` L21, replace `(see the agent guide's working-memory section)` with `(see the workflow doc's working-memory section)`.

- [ ] **Task 1.4: Relocate the chain part's working-memory paragraph.** In `.awf/parts/workflow/chain.md`, delete paragraph 3 (`Working memory is optional. ... never mines prose or filenames for identity.`). Create `.awf/parts/workflow/working-memory.md` containing `{{=awf:sectionDefault}}`, a blank line, that paragraph verbatim, then a second paragraph merging the four sentences unique to `.awf/parts/agents-doc/working-memory.md`: effort identity is one-way; do not create a memory file merely to satisfy telemetry or handoff; the ledger never stores or infers a memory path; completed efforts must be reopened separately, and abandoned or pruned efforts cannot be resumed.

- [ ] **Task 1.5: Move the two chain proofs.** In `internal/project/spine_test.go`, delete the two assertions at L984-991 (chain-order and resync string checks against the guide render) together with their `// invariant:` marker comments. In `internal/project/docs_sections_test.go`, in the workflow-doc default-render test (add `TestWorkflowDocChainOrder` rendering `docs/workflow.md.tmpl` with `testLayout()` and empty vars if none exists), assert the rendered output contains `ADR (if warranted) → plan (if warranted)` under marker `// invariant: rendering/workflow-skill-templates:workflow-chain-adr-before-plan`, and contains `resync (when both)` under marker `// invariant: rendering/workflow-skill-templates:workflow-chain-surfaces-resync`.

- [ ] **Task 1.6: Apply claim batch 1.** In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, rewrite the `workflow-chain-adr-before-plan` claim text to exactly `The rendered workflow.md workflow-chain string presents the ADR step before the plan step.` and append `Revised-by: ADR-0157` after its existing provenance lines. In `docs/decisions/0157-slim-the-agent-guide-to-entry-point-routing.md`, append to Status history the `Implementing` event with the frozen content digest, then an `Applied` event whose `state-sequence` is the next repo-global value awf reports at execution time (never a hardcoded number) and whose operations list reads `update \`rendering/workflow-skill-templates:workflow-chain-adr-before-plan\`` with the qualified ID in an inline code span, per the ADR template's event format. This leaves one applied and three remaining operations, keeping the Implementing state legal.

- [ ] **Task 1.7: Sync, verify, commit.** Fresh `git status --short` (the 0156 effort shares this checkout), `./x sync`, stage all phase files plus rendered outputs (including every regenerated `examples/sundial` render) and `.awf/awf.lock` by explicit pathspec, `./x check --staged` reports clean, `./x gate` reports green.

```commit
feat(rendering): move the working-memory protocol to the workflow doc
```

## Phase 2: guide template slimming

- [ ] **Task 2.1: Add trigger metadata to the catalog.** In `internal/catalog/catalog.go`, add `Trigger string` to `SkillSpec` with a doc comment: the one-line guide trigger for non-chain (task) skills, empty for chain skills. In `internal/catalog/standard.go`, set exact values:
  - `adr-lifecycle`: `transitioning an ADR between lifecycle states`
  - `bugfix`: `applying a fix whose root cause is already known`
  - `debugging`: `investigating a bug or unexpected behaviour before any fix`
  - `exploring`: `fresh-context repository exploration when inline search would pollute the parent context`
  - `refactor-coupling-audit`: `scoping a refactor that moves files between packages or inverts dependencies`
  - `tdd`: `writing the failing test before the implementation change`
  - `roadmap-graduation`: `graduating a shipped roadmap item out of the roadmap doc`
  In `internal/catalog/catalog_test.go`, add `TestTaskSkillTriggers`: every non-`Chain` skill in `Standard.Skills` has a nonempty `Trigger` not ending in a period; every `Chain` skill has an empty one.

- [ ] **Task 2.2: Replace the guide workflow section with the trigger table.** In `internal/project/render.go`, replace `taskSkillsDisplay()` with `taskSkillRows()`: one line per enabled non-chain catalog skill, sorted by name, formatted `- ` + backticked `<prefix>-<name>` + `: ` + `Trigger` + `.`, joined by newlines, empty string when none; render key `"taskSkills"` becomes `"taskSkillRows"`; keep the derived-roster-guarantee comment. In `templates/agents-doc/AGENTS.md.tmpl`, replace the workflow section body (L55-71) with:

  ```
  {{ if .skills.brainstorming }}Non-trivial work starts with `{{ .prefix }}-brainstorming`: it settles intent and design, then hands off through the chain (ADR and plan when warranted, implementation, review, retrospective), each skill's terminal step naming its successor.

  {{ with .taskSkillRows }}Task skills for specific situations:

  {{ . }}

  {{ end }}{{if .targetSessionHandoff}}With the Pi router active, enter every governed skill through the `awf_workflow` tool with the semantic skill name; never load a governed body directly.

  {{end}}{{ end }}Conventional Commits; one concern per commit.{{ with .layout.workflowRef }} Full rules: [{{ . }}]({{ . }}).{{ end }}
  ```

  Constraints: no chain diagram, no `Chain skills`/`Task skills` labels, no gate sentence, no V2/plan-form/exploration prose. With brainstorming disabled the section renders exactly the closing line (satisfying the fallback pin and the no-dangling-introduction rule in `cmd/awf/initrender_test.go:76-84`).

- [ ] **Task 2.3: Shrink the awf-setup section.** Replace the awf-setup section body (guide template L7-16) with:

  ```
  ## Working with awf

  This project's rendered skills, agents, and docs (and this guide) are produced by [awf](https://github.com/hypnotox/agentic-workflows) from the `.awf/` config tree. Every rendered file is generated: never hand-edit one. Change the config, a var, or a convention part, then run `awf sync` and `awf check`, and commit the rendered files with the config change. Toggle artifacts and adapters with `awf enable <kind> <name>` / `awf disable <kind> <name>`.

  See [{{ .layout.workingWithAwf }}]({{ .layout.workingWithAwf }}) for the full usage guide: commands, overrides, placeholders, and the sync/check loop.
  ```

- [ ] **Task 2.4: Shrink the working-memory section.** Replace the working-memory section body (guide template L75-83) with:

  ```
  ## Working memory

  `.awf/memory/<effort-slug>.md` (gitignored) holds one working-memory file per in-flight effort. Check `.awf/memory/` when a request implies earlier work to continue, or when a fresh session finds it non-empty and unaccounted for; resume from the file's `Phase:`/`Next:` lines rather than restarting, and ask before resuming anything you cannot verify. Never commit the file or cite it in an ADR, plan, or commit message; delete it when the effort's chain terminates. The checkpoint protocol, file skeleton, and ground rules live in the workflow doc's working-memory section.
  ```

  The pre-change section's `targetSessionHandoff` conditional (L81) disappears with the evicted protocol bullet.

- [ ] **Task 2.5: Update guide-render tests and author the new proofs in `internal/project/spine_test.go`.**
  - `TestAgentsDocTemplate`: drop expectations for evicted phrases; assert the render contains `example-brainstorming` and the exact row `- ` + backticked `example-bugfix` + `: applying a fix whose root cause is already known.` when bugfix is enabled, and contains none of: `brainstorming → ADR`, `warranted by`, `A plan may use exact content/diffs`, `V2 ADR`, `pollute parent context`, `Chain skills`.
  - `TestAgentsDocTaskSkillsGating` (L1321-1352): replace the exact-sentence check at L1346 with per-row checks: each enabled non-chain skill's trigger row present, each disabled one absent.
  - Fallback case (L1128-1148): keep `want` `Conventional Commits; one concern per commit.` and the existing bans.
  - Assert (unmarked in this phase): the workflow-doc render contains `## Working memory` and `File skeleton (a convention, not a schema`; the guide render and both rendered checkpoint-partial-carrying skill outputs contain `the workflow doc's working-memory section`; the guide render does not contain `File skeleton`.
  - Proof markers for the two new claims are NOT added in this phase: a `// invariant:` marker naming a not-yet-existing claim ID is a hard error, and the claims land in phase 4's final batch (task 4.4). The assertions above go in now; task 4.4 adds the markers.

- [ ] **Task 2.6: Remove the guide surfaces from `internal/project/plan_detail_modes_test.go`.** Delete the `{"default agent guide", defaultAgentGuide, ...}` row (L50), the `{name: "AGENTS.md", ...}` row (L61), and the now-unused `defaultAgentGuide` render block (L37-43).

- [ ] **Task 2.7: Apply the middle claim batch.** In `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, rewrite the `plan-task-detail-modes` claim text to exactly `The rendered plan-authoring skill, plan reviewer, and implementation-plans README accept exact content/diffs or implementation-ready pseudocode with a closed application contract, require exact form for machine-consumed and other contract-bearing representations, preserve the specialized batch task and no-placeholder boundary, and render coherently with empty variables.` and append `Revised-by: ADR-0157`. In the ADR, append an `Applied` event with the next repo-global `state-sequence` and operations `update \`rendering/workflow-skill-templates:plan-task-detail-modes\`` (inline code span). This leaves two applied and two remaining operations; the two `add` operations stay Remaining until phase 4.

- [ ] **Task 2.8: Sync, verify, commit.** Fresh `git status --short`, `./x sync`, explicit pathspec staging (including every regenerated `examples/sundial` render), `./x check --staged` clean, `./x gate` green.

```commit
feat(rendering): slim the agent guide to entry-point routing
```

## Phase 3: codify the standard and fold setup detail

- [ ] **Task 3.1: Codify the shape in `templates/docs/agents-md-standard.md.tmpl`.** Layout entries 5 and 6 become: `**Workflow**: awf-given; entry-skill triggers only, derived from the catalog.` and `**Working memory**: awf-given; the resume trigger, ground-rule one-liners, and the canonical-protocol pointer only.` In the rules section, after the existing terse-bar sentence, insert: `The workflow section names entry skills and their triggers, never procedure: the chain routes itself through skill handoffs, and chain rules live in the workflow doc. The working-memory protocol has one canonical home in the workflow doc's working-memory section; the guide carries the resume trigger and pointers only.` Remove no existing rule prose.

- [ ] **Task 3.2: Verify disposition row 1 and fold gaps.** For each clause of pre-change guide L11 (kind list, adapter targets, bootstrap/hooks/runner singleton semantics, requirement closure, `--with-dependents`), confirm it appears in `templates/docs/working-with-awf.md.tmpl` (overview, commands, or config-and-overrides sections); fold any missing clause into the config-and-overrides section verbatim. Post-check: `grep` the rendered `docs/working-with-awf.md` for each of `requirement closure`, `--with-dependents`, `nameless singleton`, `once per enabled target` returns a match.

- [ ] **Task 3.3: Sync, verify, commit.** Fresh `git status --short`, `./x sync`, explicit pathspec staging (including every regenerated `examples/sundial` render), `./x check --staged` clean, `./x gate` green.

```commit
docs(rendering): codify entry-point routing in the authoring standard
```

## Phase 4: repo-part conformance and close-out

- [ ] **Task 4.1: Re-trim `.awf/parts/agents-doc/awf-setup.md` with 0156 awareness.** Run fresh `git status --short` and `git log --oneline -5` first. If ADR-0156's implementation has landed and rewrote the runner/bootstrap sentences, preserve the landed wording. Re-derive the part from the slimmed template default plus exactly the repo-specific clauses (awf builds from source and disables bootstrap/runner or their 0156 successors, `.githooks/` stubs delegate to `.awf/hooks/`, `examples/sundial` demonstrates the runner artifact). If nothing repo-specific remains beyond the template default, delete the part instead.

- [ ] **Task 4.2: Trim the remaining repo parts per the disposition table.** `commands.md`: keep `{{=awf:sectionDefault}}` plus the sentence `Command specifics, metrics and lifecycle contracts, and upgrade behaviour: see docs/working-with-awf.md.`; append its evicted prose to `.awf/parts/working-with-awf/commands.md`, deduplicating against that part's existing paragraphs (where both carry a context/topic sentence, keep the working-with-awf wording). `identity.md`: drop the widget/ledger/telemetry sentences; keep what/stack/module path/Go version/maturity plus one clause that Pi receives generated subagent, handoff, and dashboard extensions. Delete `.awf/parts/agents-doc/workflow.md` (superseded by the template Pi branch; dashboard prose already lives in `.awf/parts/workflow/chain.md` paragraph 2; this deviation from ADR Decision 9 is recorded at resync). Shrink `.awf/parts/agents-doc/working-memory.md` to `{{=awf:sectionDefault}}`, a blank line, and exactly one sentence: `In a fresh Pi session, continue an explicitly named active effort with /awf-resume-effort <effort-id>.` (its remaining unique protocol sentences were merged into `.awf/parts/workflow/working-memory.md` by task 1.4; diff to confirm no sentence is lost).

- [ ] **Task 4.3: Changelog entry.** Add to `changelog/CHANGELOG.md` under the unreleased heading, following the existing entry format: the guide is now an entry-point router and adopters re-render a much smaller AGENTS.md, with the upgrade note verbatim: `If you replaced the workflow doc's chain section with a full-replacement part, the checkpoint protocol prose relocated to the new working-memory section and your part will not receive it; re-derive your part or adopt the new section.`

- [ ] **Task 4.4: Apply the final claim batch and close out.** In `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, append the two claims with `Origin: ADR-0157` and `Backing: test`, exact texts:
  - `### \`invariant: guide-entry-point-routing\``: `The rendered guide's workflow section is a catalog-derived entry-skill trigger table: every catalog entry and task skill appears iff enabled with its catalog trigger line, and none of the evicted prose classes renders (chain diagram, warrant definitions, plan-form contract, V2 batch semantics, exploration/subagent policy, duplicated gate sentence).`
  - `### \`invariant: working-memory-single-home\``: `The file skeleton, ground rules, and just-in-time retrieval prose render canonically in the workflow doc's working-memory section; the guide, the shared checkpoint partials, and the chain section point to that content rather than carrying copies of it.`
  In `internal/project/spine_test.go`, add the proof markers deferred from task 2.5: `// invariant: rendering/guide-and-doc-templates:guide-entry-point-routing` on the trigger-table assertions and `// invariant: rendering/guide-and-doc-templates:working-memory-single-home` on the single-home assertions. In the ADR, append the final `Applied` event (next repo-global `state-sequence`; operations `add \`rendering/guide-and-doc-templates:guide-entry-point-routing\`, add \`rendering/guide-and-doc-templates:working-memory-single-home\`` in inline code spans) and then the `Implemented` status event with the content digest in the same commit (the final pair; INDEX regenerates via sync, ADR item 11). Flip this plan's `status:` to `Implemented`. Fresh `git status --short`, `./x sync`, explicit pathspec staging (including every regenerated `examples/sundial` render), `./x check --staged` clean, `./x gate` green.

```commit
docs(config): conform awf's own guide parts to 0157 and close out
```

## Verification

- `./x check` clean and `./x gate` green at HEAD (each phase already gated).
- `grep "A plan may use exact content/diffs" AGENTS.md` returns no output; `grep -c "workflow doc's working-memory section" AGENTS.md` returns at least 1.
- `git grep -l "agent guide's working-memory section" -- templates .claude .pi docs AGENTS.md examples/sundial ':!docs/decisions' ':!docs/plans'` returns no output (tracked files only, so the untracked 0156 worktree under `.claude/worktrees/` is skipped, and the pathspec exclusions skip frozen ADR/plan history, which legitimately retains the old phrase).
- `./x topic rendering/guide-and-doc-templates` lists `guide-entry-point-routing` and `working-memory-single-home` with test backing; `./x topic rendering/workflow-skill-templates` shows both updated claims with `Revised-by: ADR-0157`.
- Disposition spot-checks (rows 10, 15, 17, 18): `grep` `docs/workflow.md` for `lower-cost child model`, `just-in-time`, `File skeleton`, `harmless gitignored residue` each returns a match.
- Indicative, not gated: `wc -c AGENTS.md` reports roughly a third of the pre-change size.

## Notes

- The 0156 effort shares this checkout and edits `.awf/parts/agents-doc/awf-setup.md` content; task 4.1 carries the ordering rule. Fresh `git status` before every staging step; pathspec staging only.
- The `Trigger` strings are catalog metadata for the guide table; skill SKILL.md frontmatter descriptions are unchanged.
- Out of scope: adopter-side part updates (the changelog note covers them); the reviewer-spine dedup backlog item.
