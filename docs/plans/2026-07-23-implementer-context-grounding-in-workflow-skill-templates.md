---
date: 2026-07-23
adrs: [155]
status: Implemented
---
# Plan: Implementer context grounding in workflow skill templates

## Goal

Implement ADR-0155: give every implementer-chain skill template a concise instruct-style
`awf context` grounding step, convert the reviewer dispatches from pasted `--full` packets to
instructed reviewer-run commands with parent-resolved arguments, close the resync gap, document
the two deliberate omissions, and pin the enlarged caller camps in the projection spine test.
Non-goals: no change to `brainstorming` or `exploring` behavior, no shared grounding include, no
`awf` CLI behavior change, and no topic-hygiene work on over-budget topics.

## Architecture summary

Design and rationale live in ADR-0155
(`docs/decisions/0155-implementer-side-context-grounding-in-workflow-skill-templates.md`). All
behavior changes are prose edits to shipped skill templates under `templates/skills/`, re-rendered
by `./x sync` into `.claude/skills/`, `.pi/awf-workflows/`, `examples/sundial/`, and
`docs/domains/tooling.md`; enforcement is the map extension in
`TestManagedContextCallersChooseProjection` (`internal/project/spine_test.go`); state lands as one
direct V2 transaction (both operations, `add
rendering/workflow-skill-templates:implementer-context-grounding` and `update
tooling/context-and-topic:context-full-authority-packet`, applied atomically with the Implemented
status event in the final commit). Never hand-edit rendered outputs; edit templates and authored
parts, then sync.

## File structure

Created:
- `docs/plans/2026-07-23-implementer-context-grounding-in-workflow-skill-templates.md` (this plan)

Modified (authored):
- `templates/skills/executing-plans/SKILL.md.tmpl`
- `templates/skills/subagent-driven-development/SKILL.md.tmpl`
- `templates/skills/writing-plans/SKILL.md.tmpl`
- `templates/skills/bugfix/SKILL.md.tmpl`
- `templates/skills/debugging/SKILL.md.tmpl`
- `templates/skills/tdd/SKILL.md.tmpl`
- `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`
- `templates/skills/reviewing-impl/SKILL.md.tmpl`
- `templates/skills/reviewing-plan/SKILL.md.tmpl`
- `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`
- `templates/skills/reviewing-adr/SKILL.md.tmpl`
- `internal/project/spine_test.go`
- `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`
- `.awf/topics/parts/tooling/context-and-topic/current-state.md`
- `.awf/domains/parts/tooling/current-state.md`
- `changelog/CHANGELOG.md`
- `docs/decisions/0155-implementer-side-context-grounding-in-workflow-skill-templates.md` (status flip)

Modified (generated, via `./x sync`; never hand-edited): the rendered skills under
`.claude/skills/` and `.pi/awf-workflows/` for the eleven templates above, their
`examples/sundial/` mirrors, `docs/domains/tooling.md`, `docs/domains/rendering.md`,
`docs/topics/rendering/workflow-skill-templates.md`, `docs/topics/rendering/index.md`,
`docs/topics/tooling/context-and-topic.md`, `docs/topics/tooling/index.md`,
`docs/decisions/INDEX.md`, and `.awf/awf.lock`.

Deleted: none.

## Phase 1: Concise grounding steps in the seven implementer skills

Every task in this phase edits template prose only; the shared rationale clause is "concise
first: orient on the owning domains and applicable claims, then drill down with `awf topic` where
an edit touches a claimed surface". No new template vars are introduced anywhere in this plan.
Grounding lines must contain `awf context` without `--full` and must not mention `--full` on the
same line (the spine test scans line-by-line).

- [ ] **Task 1.1: executing-plans grounding bullet.** In
  `templates/skills/executing-plans/SKILL.md.tmpl`, section `procedure-per-task`, insert a new
  first bullet before the existing `- **Implement**` bullet of step 3:

  ```
     - **Ground.** Run `awf context <the task's named paths>` before editing (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic <domain>/<topic>[:<claim>]` where an edit touches a claimed surface). When a task names no paths, fall back to the plan's file-structure header paths.
  ```

- [ ] **Task 1.2: subagent-driven-development resolved-command bullet.** In
  `templates/skills/subagent-driven-development/SKILL.md.tmpl`, section
  `procedure-extract-context`, append a new bullet to the step-3 list (after the project-conventions
  bullet):

  ```
     - The resolved concise grounding command for the task, `awf context <the task's exact paths>`, with the instruction that the subagent runs it first (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where an edit touches a claimed surface) before editing.
  ```

- [ ] **Task 1.3: writing-plans author grounding.** In
  `templates/skills/writing-plans/SKILL.md.tmpl`, section `procedure-write-plan`, replace the
  step-2 body:

  ```
  2. **Write the plan file in one go.** The plan must be self-contained: every step executable by an agent with no prior conversation context. While drafting the file-structure header and tasks, run `awf context <the plan's created and modified paths>` (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where a task touches a claimed surface) so task content is grounded in current authority rather than reconstructed from memory.
  ```

- [ ] **Task 1.4: bugfix grounding sentence.** In `templates/skills/bugfix/SKILL.md.tmpl`,
  procedure step 1, append on the same physical line as step 1, after the outer `{{ end }}` that
  closes `{{ if .skills.tdd }}` (the line's final token; the nested targetWorkflowRouter
  conditional closes earlier on that line):

  ```
   Before writing the test, run `awf context <the implementation and test paths>` (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where the fix touches a claimed surface).
  ```

- [ ] **Task 1.5: tdd grounding step.** In `templates/skills/tdd/SKILL.md.tmpl`, replace
  procedure step 1:

  ```
  1. Run `awf context <the implementation and test paths>` (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where the change touches a claimed surface), then write the failing test capturing the wrong (bug) or missing (feature) behaviour.
  ```

- [ ] **Task 1.6: debugging grounding sentence.** In
  `templates/skills/debugging/SKILL.md.tmpl`, section `test-isolation`, step 4: after the
  sentence "Once the defective surface is located, write the smallest possible test that
  reproduces the failure before touching the fix.", insert:

  ```
  Run `awf context <the suspect paths>` first (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where the fix will touch a claimed surface).
  ```

- [ ] **Task 1.7: refactor-coupling-audit grounding sentence.** In
  `templates/skills/refactor-coupling-audit/SKILL.md.tmpl`, section `audit-shape-selection`,
  append to the "Pick the audit shape" paragraph:

  ```
  Ground the audit before the categories: run `awf context <the refactor's source and destination paths>` (concise first: orient on the owning domains and applicable current-state claims, then drill down with `awf topic` where a moved surface is claimed) so the coupling findings land in the ADR Context section against current authority.
  ```

- [ ] **Task 1.8: extend the concise map.** In `internal/project/spine_test.go`,
  `TestManagedContextCallersChooseProjection`, replace the `concise` map literal:

  ```go
  concise := map[string]bool{
  	"brainstorming":               true,
  	"bugfix":                      true,
  	"debugging":                   true,
  	"executing-plans":             true,
  	"refactor-coupling-audit":     true,
  	"subagent-driven-development": true,
  	"tdd":                         true,
  	"writing-plans":               true,
  }
  ```

  Do not add a proof marker in this phase; the claim it would prove lands in Phase 3.

- [ ] **Task 1.9: changelog entry, implementer portion.** In `changelog/CHANGELOG.md`, insert a
  `### Features` section immediately after the `## [Unreleased]` line (before the existing
  `### Bug fixes`), so the adopter-visible drift this commit creates is documented in the same
  commit:

  ```
  ### Features
  - Workflow skill templates now ground implementers in current-state authority: seven
    implementer-chain skills (executing-plans, subagent-driven-development, writing-plans,
    bugfix, debugging, tdd, refactor-coupling-audit) instruct a concise `awf context` run over
    their touched paths before editing (ADR-0155).
  ```

- [ ] **Task 1.10: sync, validate, commit.** Run `./x sync` (rendered skills, sundial mirrors,
  and the lock regenerate). Stage the complete transaction (templates, test, and every regenerated
  file `git status` reports). Run `awf check --staged` (clean), then `./x gate` (passes). Commit:

  ```commit
  feat(rendering): ground implementer skills in awf context
  ```

  Post-check: `go test ./internal/project -run TestManagedContextCallersChooseProjection` passes,
  and `grep -rln "awf context" templates/skills/ | sort` lists exactly: adr-lifecycle,
  brainstorming, bugfix, debugging, executing-plans, refactor-coupling-audit, reviewing-impl,
  reviewing-plan, subagent-driven-development, tdd, writing-plans (one path each).

## Phase 2: Instruct-style reviewer dispatches

- [ ] **Task 2.1: reviewing-impl paste-to-instruct conversion.** In
  `templates/skills/reviewing-impl/SKILL.md.tmpl`, section `dispatch-subagent`, replace the
  affected-context bullet:

  ```
     - The affected context instruction: with the concrete SHAs substituted, direct the reviewer to run `awf context --full $(git diff --name-only ${baseSha}..${headSha})` itself (`--full`: the reviewer needs the complete authority packet) so the doc-currency and convention lenses know the owning domains, the applicable current-state claims (rules and invariants), and any Accepted pending changes without re-deriving them. Pass the resolved SHA range, not pasted packet output.
  ```

  The bullet stays in the shared bullet list after the `{{ if .targetSubagentTools }}` fork's
  closing `{{ end }}`, so both branches render it.

- [ ] **Task 2.2: reviewing-plan paste-to-instruct conversion.** In
  `templates/skills/reviewing-plan/SKILL.md.tmpl`, section `dispatch-subagent`, replace the
  affected-context bullet:

  ```
     - The affected context instruction: collect the created/modified paths from the plan's file-structure header and direct the reviewer to run `awf context --full <those paths>` itself (`--full`: the reviewer needs the complete authority packet) so the doc-currency and convention-alignment lenses know the owning domains, backed invariants, and related ADRs. Pass the resolved paths, not pasted packet output.
  ```

- [ ] **Task 2.3: reviewing-plan-resync gap close (add, not convert).** In
  `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`, section `dispatch-subagent-narrowed`,
  insert a new bullet between the RESYNC-mode-instruction bullet and the findings-format bullet:

  ```
     - The affected context instruction: collect the created/modified paths from the plan's file-structure header and direct the reviewer to run `awf context --full <those paths>` itself (`--full`: the reviewer needs the complete authority packet) so the doc-currency lens knows the owning domains and applicable current-state claims. Pass the resolved paths, not pasted packet output.
  ```

- [ ] **Task 2.4: reviewing-adr destination-topic hint.** In
  `templates/skills/reviewing-adr/SKILL.md.tmpl`, section `dispatch-subagent`, insert a new
  bullet between the absolute-ADR-path bullet and the findings-format bullet:

  ```
     - The hint that the reviewer may run `awf topic <domain>/<topic>` on each destination topic named in the ADR's State changes when it needs current claim text. No context packet accompanies this dispatch: an explicit ADR path reports lifecycle progress, not path-claim grounding.
  ```

  This line deliberately says `awf topic`, never `awf context`, so the spine test does not
  classify reviewing-adr.

- [ ] **Task 2.5: extend the complete map.** In `internal/project/spine_test.go`,
  `TestManagedContextCallersChooseProjection`, replace the `complete` map literal:

  ```go
  complete := map[string]bool{
  	"adr-lifecycle":         true,
  	"reviewing-impl":        true,
  	"reviewing-plan":        true,
  	"reviewing-plan-resync": true,
  }
  ```

- [ ] **Task 2.6: changelog entry, reviewer portion.** In `changelog/CHANGELOG.md`, extend the
  Task 1.9 bullet so this commit's adopter-visible drift travels documented: replace its closing
  `before editing (ADR-0155).` with:

  ```
  before editing, the reviewer dispatches (reviewing-impl, reviewing-plan, and the previously
    packet-free resync) instruct the reviewer to run `awf context --full` itself with
    parent-resolved arguments instead of pasting packet output into the brief, and the
    ADR-reviewer brief gains an `awf topic` destination-topic hint (ADR-0155). Adopters see the
    skill drift resolved by their next `awf sync`.
  ```

- [ ] **Task 2.7: sync, validate, commit.** Run `./x sync`. Stage the complete transaction. Run
  `awf check --staged` (clean), then `./x gate` (passes). Commit:

  ```commit
  feat(rendering): instruct reviewer-run context packets
  ```

  Post-check: `grep -rn "paste the output of \`awf context" templates/skills/` matches only
  `templates/skills/brainstorming/SKILL.md.tmpl`, and
  `go test ./internal/project -run TestManagedContextCallersChooseProjection` passes.

## Phase 3: State changes, narrative, and the direct Implemented flip

One commit; the direct V2 transaction applies both declared operations atomically with the
status event, per the pitfall that a flip never travels alone.

- [ ] **Task 3.1: author the added claim.** In
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, insert alphabetically
  (after `cross-runtime-exploration-dispatch`, before `mandatory-approval-boundaries`):

  ```
  ### `invariant: implementer-context-grounding`

  Every implementer-chain skill template (executing-plans, subagent-driven-development, writing-plans, bugfix, debugging, tdd, refactor-coupling-audit) carries a concise `awf context` invocation, and the projection-pinning spine test classifies every grounding-carrying skill template into exactly the concise or complete-authority camp.
  Origin: ADR-0155
  Backing: test
  ```

- [ ] **Task 3.2: add the proof marker.** In `internal/project/spine_test.go`, above
  `func TestManagedContextCallersChooseProjection`, alongside the existing
  `// invariant: tooling/context-and-topic:context-full-authority-packet` marker, add:

  ```go
  // invariant: rendering/workflow-skill-templates:implementer-context-grounding
  ```

- [ ] **Task 3.3: apply the claim update.** In
  `.awf/topics/parts/tooling/context-and-topic/current-state.md`, claim
  `context-full-authority-packet`: replace the final clause of the claim sentence, `managed
  complete-authority callers request `--full` explicitly.`, with:

  ```
  managed complete-authority callers (the reviewer dispatches, plan resync included, and the ADR lifecycle) instruct `--full` runs whose arguments the dispatching parent resolves, while implementer and orientation callers stay concise.
  ```

  Change its provenance line `Revised-by: ADR-0153` to `Revised-by: ADR-0153, ADR-0155`.

- [ ] **Task 3.4: update the tooling domain narrative.** In
  `.awf/domains/parts/tooling/current-state.md`, in the ADR-0092/0147 context paragraph, replace
  the final sentence `Managed workflow callers that require complete authority use `--full`,
  while brainstorming orientation remains concise.` with:

  ```
  Managed complete-authority callers (the reviewer dispatches, plan resync included, and the ADR lifecycle) instruct `--full` runs whose arguments the dispatching parent resolves, implementer skills ground concisely and drill down on demand, and a parent pastes packet output into a child brief only when it already holds that output for its own purposes (ADR-0155). Two omissions are deliberate: the ADR reviewer receives no packet, because an explicit ADR path reports lifecycle progress rather than path-claim grounding (its reviewer queries destination topics via `awf topic`), and the generic exploration skill stays grounding-free because its target paths are unknown up front.
  ```

- [ ] **Task 3.5: direct Implemented transition, plan freeze, sync, commit.** Following
  `awf-adr-lifecycle`'s direct path: in
  `docs/decisions/0155-implementer-side-context-grounding-in-workflow-skill-templates.md`, set
  frontmatter `status: Implemented` and append to Status history the line
  `- <today ISO-8601>: Implemented; content-sha256: <checker-reported digest>; state-sequence:
  <checker-reported sequence>` (run `awf check --staged` with the transaction staged; it reports
  the expected digest and sequence when the appended line is missing or wrong). Both declared
  operations apply in this one implicit batch; no Implementing event, no standalone flip. Flip
  this plan's frontmatter to `status: Implemented`, recording surfaced findings under Notes. Run
  `./x sync` (INDEX.md and `docs/domains/tooling.md` regenerate). Stage the complete transaction,
  run `awf check --staged` (clean; ADR-0155 Implemented with both operations Applied), then
  `./x gate full` (passes). Besides INDEX.md and `docs/domains/tooling.md`, this sync also
  regenerates `docs/domains/rendering.md`, `docs/topics/rendering/workflow-skill-templates.md`,
  `docs/topics/rendering/index.md`, `docs/topics/tooling/context-and-topic.md`, and
  `docs/topics/tooling/index.md`; stage them as part of the transaction, not as unexpected
  drift. Commit:

  ```commit
  docs(invariants): apply 0155 grounding claims (implements 0155)
  ```

## Verification

- `./x check` is clean with no `note:` lines attributable to this change.
- `awf topic rendering/workflow-skill-templates:implementer-context-grounding` shows the claim
  active with Origin ADR-0155, Backing test, and a proof site in `internal/project/spine_test.go`.
- `awf topic tooling/context-and-topic:context-full-authority-packet` shows
  `Revised-by: ADR-0153, ADR-0155` and the instruct-style caller wording.
- `grep -rn "paste the output of \`awf context" templates/skills/` matches only the brainstorming
  template.
- `grep -c "awf context" templates/skills/reviewing-plan-resync/SKILL.md.tmpl` returns a non-zero
  count (gap closed).
- The rendered `.claude/skills/awf-executing-plans/SKILL.md` and
  `examples/sundial/.claude/skills/sundial-executing-plans/SKILL.md` both carry the grounding
  bullet (sync propagated; spot-check one of the seven).

## Notes

- The spine test forbids `--full` on any `awf context` line in a concise-map skill and requires
  it on every such line in a complete-map skill; keep each new grounding line free of the literal
  `--full` and each reviewer instruction line carrying it.
- `reviewing-adr` and `exploring` intentionally remain outside both maps with no
  `awf context` invocation.
- Out of scope (deferred, tracked elsewhere): topic hygiene for over-budget topics; the shared
  reviewer-spine/template-partials dedup.
- Implementation findings: execution followed the plan without content deviation. The
  state-sequence for the direct Implemented event is a repo-global counter (the checker
  reported 36, not a per-ADR 1); the checker-reported-value procedure in Task 3.5 absorbed this
  as designed. Commits interleaved with a concurrent session's ADR-0156 work in the same
  checkout; staging by explicit path snapshot kept every transaction exact.
