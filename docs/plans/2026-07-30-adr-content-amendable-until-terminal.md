---
date: 2026-07-30
adrs: [0186]
status: Proposed
---
# Plan: ADR content amendable until terminal

## Goal

Implement ADR-0186: current-state-v2 ADR content stays amendable until a terminal status via
`Amended` stamp-chain history events, the frozen-content and digest validation move to the chain
rules, the Implemented flip moves into the terminal-review flow, and every freeze-asserting prose
surface follows in the same change. Non-goals: no change to current-state-v1 semantics, no corpus
migration or schema-generation bump, and no audit rule for the residual first-stamp case (stays on
the deferred roadmap).

## Architecture summary

Four transactions. Phase 1 lands the grammar and validation change in `internal/adr` plus the
transition wiring in `internal/currentstate`, fully test-covered, while every existing record stays
valid. Phase 2 rewrites the published standard's embedded templates and the catalog states data and
re-renders this repo and the sundial example. Phase 3 updates this repo's local prose parts
(domain narrative, glossary, pitfalls, roadmap). Phase 4 is the deferred application transaction
prescribed by ADR-0186 item 7: it runs only after the applicable terminal review settles, applies
both claim operations as one direct implicit batch, and flips ADR-0186 and this plan to
Implemented. Design rationale lives in ADR-0186; do not re-derive it here.

## File structure

- **Created:** `docs/plans/2026-07-30-adr-content-amendable-until-terminal.md` (this plan).
- **Modified:**
  - `internal/adr/history.go`, `internal/adr/format.go`, `internal/adr/digest.go`,
    `internal/adr/format_test.go` (new cases; possibly a shared fixture helper in the same file),
    `internal/currentstate/transition.go`, `internal/currentstate` pair tests
    (`transition_test.go` or the existing pair-test file found by
    `grep -rln "FrozenContentEqual\|frozen-content" internal/currentstate/*_test.go`).
  - `internal/catalog/standard.go` (adrStates rows) and any test asserting those exact strings
    (found by `grep -rln "the body is frozen" internal/`).
  - Templates: `templates/skills/adr-lifecycle/SKILL.md.tmpl`,
    `templates/skills/reviewing-adr/SKILL.md.tmpl`, `templates/skills/reviewing-impl/SKILL.md.tmpl`,
    `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`,
    `templates/skills/executing-plans/SKILL.md.tmpl`,
    `templates/skills/subagent-driven-development/SKILL.md.tmpl`,
    `templates/agents-doc/AGENTS.md.tmpl`, `templates/adr-readme/README.md.tmpl`,
    `templates/adr-template/template.md.tmpl`.
  - Repo-local parts: `.awf/domains/parts/adr-system/current-state.md`, `.awf/docs/glossary.yaml`,
    `.awf/docs/pitfalls.yaml`, `.awf/docs/parts/roadmap/deferred.md`.
  - Rendered outputs of `./x render` (this repo's `.claude/skills/*`, `AGENTS.md`, docs, and the
    `examples/sundial` renders); `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md` and
    its rendered `docs/topics/adr-system/adr-lifecycle.md`;
    `docs/decisions/0186-adr-content-amendable-until-implemented-via-amended-events.md` (status
    flip); `docs/decisions/INDEX.md`; this plan file (status flip).
- **Deleted:** none.

## Phase 1: Amended event grammar and stamp-chain validation

**Execution mode: inline.** One independently green transaction: every existing record parses
unchanged, and the new grammar and rules are fully covered.

- [ ] **Task 1.1: failing tests for the new grammar and chain rules.** In
  `internal/adr/format_test.go`, add table-driven cases (following the file's existing fixture
  style for V2 records) asserting, before any production change, that these currently fail or
  error as described; after Tasks 1.2-1.4 they must all pass:
  - Parse accepts `- 2026-07-30: Amended; content-sha256: <64 lowercase hex>` inside a V2 Status
    history whose current status at that point is Accepted or Implementing, yielding an event with
    the new kind and the digest.
  - Parse rejects: an Amended line with a malformed digest (not 64 lowercase hex); an Amended event
    while the current status is Proposed; an Amended event after a terminal status event; an
    Amended event whose digest equals the immediately preceding stamp; an Amended event between an
    Implementing status event and its first Applied event; an Amended event between the final
    Applied event and an explicit Implemented status event (expected message: the existing
    "explicit Implemented transition requires a final Applied event immediately before it").
  - Chain rules: a status event whose digest differs from the preceding stamp fails with the new
    repeat-the-stamp message; a record whose latest stamp does not equal the computed content
    digest fails with the new latest-stamp message; a record with an Amended event introducing a
    new digest, followed by status events repeating it, and content matching that digest, parses
    cleanly; a direct Proposed-to-Implemented record establishes its first stamp at the terminal
    event exactly as today.
  - Legacy shape: an unamended record with equal stamps on Accepted, Implementing, and Implemented
    events parses cleanly (no fixture edits: reuse an existing passing V2 fixture unchanged).
  - `FrozenContentEqual`: V2 pairs with before-status Proposed, Accepted, and Implementing return
    true under changed content; V2 pairs with before-status Implemented or Abandoned return false
    under changed content and true under equal content; V1 pairs keep today's behavior (true only
    from Proposed or with equal digests).
  - `HistoryTransitionValid` (AuthoredCommit): a same-status Accepted or Implementing pair
    appending exactly one Amended event is valid; appending an Amended event plus any other event
    in one pair is invalid; a same-status Proposed, Implemented, or Abandoned pair appending an
    Amended event is invalid.
  Run `go test ./internal/adr/` and record the expected failures, then proceed.
- [ ] **Task 1.2: parse the Amended event.** In `internal/adr/history.go`:
  - Add `HistoryAmended` as the third `HistoryEventKind` constant.
  - Add the head regex (exact pattern):
    `amendedHeadRe = regexp.MustCompile("^- (\\d{4}-\\d{2}-\\d{2}): Amended; content-sha256: ([0-9a-f]{64})$")`
    declared beside `appliedHeadRe` in the raw-string style the file already uses.
  - In `parseHistory`, inside the existing `format == CurrentStateV2` branch and beside the
    Applied match, match `amendedHeadRe` and append
    `HistoryEvent{Kind: HistoryAmended, Date: m[1], Digest: m[2]}`. V1 parsing is untouched; an
    Amended line in a V1 history stays a malformed-entry error.
- [ ] **Task 1.3: stamp-chain validation.** In `internal/adr/format.go`:
  - In `validateV2History`, track `lastStamp := ""` through the event loop.
    - New branch before the unknown-kind catch-all: for `HistoryAmended`, error unless the current
      status is Accepted or Implementing ("amended event is allowed only while Accepted or
      Implementing"); error when `event.Digest == lastStamp` ("amended event must record a digest
      different from the preceding stamp"); otherwise set `lastStamp = event.Digest` and continue.
      Update the catch-all's `coverage-ignore` reason to name three closed event kinds.
    - For a status event past Proposed: when `lastStamp` is empty, set `lastStamp = event.Digest`
      (the first stamp); otherwise error when `event.Digest != lastStamp` with a message naming
      both digests ("<status> entry content-sha256 %q does not repeat the preceding stamp %q").
    - After the loop: when `lastStamp` is nonempty and `lastStamp != digest` (the computed content
      digest), error with "latest stamped content-sha256 %q does not match the computed digest %q".
    - Remove the `// coverage-ignore` on the explicit-Implemented guard (`h[i-1].Kind !=
      HistoryApplied`), now reachable via an intervening Amended event and covered by Task 1.1.
  - In `validateV2StatusEntry`, drop the digest parameter and the `e.Digest != digest` comparison;
    add instead: every non-Proposed status entry must carry a nonempty digest ("%s entry must
    carry a content-sha256"). Keep all sequence/rationale shape rules. Adjust its caller.
  - `validateV1History`/`validateHistoryEntry` stay untouched.
- [ ] **Task 1.4: format-aware freeze and transition shape.** In `internal/adr/format.go`:
  - `FrozenContentEqual`: for `before.Format == CurrentStateV2`, return true unless the before
    status is Implemented or Abandoned with differing content digests; other formats keep the
    Proposed rule. Reuse an existing terminal-status helper in the `adr` package if one exists
    (check `internal/adr/status.go`); otherwise add an unexported one beside the status constants.
    Rewrite the function's doc comment: V1 editable only while Proposed; V2 editable until
    terminal, then frozen at the before-state digest.
  - `HistoryTransitionValid`, same-status branch: exactly one added event is valid when it is an
    Applied event at Implementing (today's rule) or an Amended event at Accepted or Implementing;
    every other nonempty same-status append stays invalid. Status-changing shapes are unchanged
    (an amendment never rides a flip commit).
  - In `internal/adr/digest.go`, replace the `ContentDigest` doc-comment sentence "Accepted
    freezes this value; a later terminal Status-history entry must repeat it." with "Each stamped
    history event records this value as of its own append; the latest stamp must always match it,
    and a terminal status freezes it permanently."
- [ ] **Task 1.5: transition finding wording and pair tests.** In
  `internal/currentstate/transition.go`, change the frozen-content finding message to
  `"ADR-%s violates the frozen-content rule: canonical decision content changed after the record froze"`.
  Post-check: `grep -rn "changed after Proposed" internal/` returns no output (update any test
  asserting the old message). Add pair tests in the existing `internal/currentstate`
  transition-test file covering: an Accepted-to-Accepted pair with amended content plus one
  appended Amended event is finding-free; the same pair without the Amended event yields findings
  (the after side fails its latest-stamp parse, surfacing through `CheckPair`); an Implemented
  before side with changed content yields the frozen-content finding; a MergeAggregate pair whose
  appended events interleave Amended and Applied events legally is finding-free.
- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run
  `awf check --staged` then `./x gate`; both must pass with the existing corpus untouched.

```commit
feat(adr-system): accept Amended events in the V2 stamp chain
```

## Phase 2: published standard prose and catalog states

**Execution mode: inline.** Template and catalog changes plus one re-render; the drift check is
the transaction's own verifier.

- [ ] **Task 2.1: catalog states rows.** In `internal/catalog/standard.go` `adrStates`, keep the
  Proposed, Implemented, and Abandoned rows unchanged and replace exactly two rows:
  - Accepted: meaning stays "Design is finalised; implementation authorised but not yet started";
    mutability becomes "Status and append-only Status history; the body stays amendable, each
    amendment appending an Amended event; a schema retrofit may migrate the encoding".
  - Implementing: meaning becomes "A nonempty strict subset of declared operations is applied";
    mutability becomes "Status and append-only Status history; Applied events append while
    operations remain, and the body stays amendable via Amended events".
  Post-check: `grep -rn "Design is frozen" internal/catalog/` returns no output. Update any test
  asserting the old strings (find with `grep -rln "the body is frozen" internal/`), keeping
  Implemented/Abandoned assertions intact.
- [ ] **Task 2.2: adr-lifecycle skill template.** In `templates/skills/adr-lifecycle/SKILL.md.tmpl`:
  - Description (line 6) and intro (line 11): replace "amendment-while-Proposed" with
    "amendment-until-terminal" (both spellings as they appear).
  - Transitions bullet: "`Proposed → Accepted` freezes the body and establishes the content digest
    without applying operations." becomes "`Proposed → Accepted` establishes the first content
    stamp without applying operations; the body stays amendable via Amended events."
  - State-changes section, the **At `Accepted`** bullet: replace "the operations are frozen
    instruction" with "the operations are settled instruction, amendable under the
    amendment-until-terminal rules until an Applied event references them".
  - Procedure step 1: replace "Repeat the frozen digest on status events and use the next sequence
    `awf check` reports." with "Repeat the latest stamp on status events and use the next sequence
    `awf check` reports; an amendment instead appends its own
    `- YYYY-MM-DD: Amended; content-sha256: <digest of the amended content>` event in its own
    commit."
  - Rename the section id `amendment-while-proposed` to `amendment-until-terminal` and replace the
    section body with (exact):

    ```
    ## Amendment-until-terminal

    While `status: Proposed`, all sections may be amended freely, with no history event. Once
    `Accepted` or `Implementing`, the body stays amendable: each amendment commit appends exactly
    one `- YYYY-MM-DD: Amended; content-sha256: <digest of the amended content>` event and changes
    nothing else in the history. At `Implemented` or `Abandoned` the body's meaning is frozen; a
    schema retrofit may migrate its machine-readable encoding.

    - An amendment must not alter or remove a State-changes operation already referenced by an
      Applied event; adding new operations and rewording operations not yet applied stay legal.
    - An amendment that changes a Decision item, the State changes operation set, or the meaning
      of an invariant claim is raised as a user-decision before landing, unless it corrects a
      clear defect with a no-brainer fix; prose-only clarification stays autonomous. A
      load-bearing amendment re-dispatches the ADR reviewer over the amended sections before
      landing.
    - Commit as `docs(adr): amend NNNN <what changed>`. Deferral (scope shrink) stays a Context
      amendment recorded the same way.
    - current-state-v1 records keep the prior rule: frozen once they leave Proposed.
    ```
  - Notes append-only bullet: replace with "**Append-only rule:** the `## Status history` is
    append-only in every state. A V2 body stays amendable until a terminal status, each amendment
    recorded as an Amended event, then freezes as the historical record; append-only protects
    rationale, not bookkeeping - a meaning-preserving schema retrofit may migrate its
    machine-readable encoding."
  - Post-check: `grep -rn "amendment-while-proposed\|amendment-while-Proposed" templates/ .awf/`
    returns no output (also proves no local override part referenced the old section id).
- [ ] **Task 2.3: flip-ownership surfaces.** Exact replacements:
  - `templates/skills/reviewing-adr/SKILL.md.tmpl`, `status-flip` section: replace the sentence
    fragment "the flip to `Accepted`/`Implemented` is owned by the implementation step's final
    commit (`{{ .prefix }}-executing-plans` / `{{ .prefix }}-subagent-driven-development`, or
    `{{ .prefix }}-adr-lifecycle` for the no-plan direct-implementation case)." with "the flip to
    `Implemented` is owned by the terminal-review flow: after the applicable terminal review
    settles, `{{ .prefix }}-reviewing-impl` lands the final Applied batch and the status flip
    immediately before managed-worktree removal and retrospective, with
    `{{ .prefix }}-adr-lifecycle` supplying the mechanics." Keep the rest of the section.
  - `templates/skills/executing-plans/SKILL.md.tmpl`, `procedure-adr-final-commit` section body
    becomes: "4. Apply non-final V2 operation batches atomically with their matching claims and
    lifecycle event in the owning phase. The final batch and the Implemented flip are owned by the
    terminal-review flow and land only after the applicable terminal review settles."
  - `templates/skills/subagent-driven-development/SKILL.md.tmpl`, `final-task-adr-flip` section
    body becomes: "7. Apply non-final V2 batches atomically with matching claims and lifecycle
    events in the owning phase. The final batch and the Implemented flip are owned by the
    terminal-review flow and land only after the applicable terminal review settles."
  - `templates/skills/reviewing-impl/SKILL.md.tmpl`, `hand-off` section, replace the final bullet
    ("- Only after the applicable terminal review has zero findings run ...") with: "- Only after
    the applicable terminal review has zero findings, land the deferred flip transaction: apply
    any final V2 batch with exactly its claim mutations and flip the linked ADR(s) and plan to
    `Implemented` per `{{ .prefix }}-adr-lifecycle`. Then run `awf effort worktree remove <slug>`
    without force and verify path, registration, and branch are absent. Then invoke
    `{{ .prefix }}-retrospective`."
  - `templates/skills/reviewing-plan-resync/SKILL.md.tmpl` line 15: replace "the status flip is
    owned by the implementation step" with "the status flip is owned by the terminal-review flow".
  - Post-check: `grep -rn "implementation step's final commit" templates/` returns no output.
- [ ] **Task 2.4: guide, template, and agent-guide prose.**
  - `templates/adr-readme/README.md.tmpl`: replace the sentence "Status events repeat the frozen
    `content-sha256`." with "Content stays amendable until a terminal status: while Accepted or
    Implementing an amendment appends `- YYYY-MM-DD: Amended; content-sha256: <new digest>`, a
    status event repeats the latest stamp (or establishes the first), and the latest stamp always
    equals the current content, freezing permanently at Implemented or Abandoned. V1 records
    instead freeze once they leave Proposed." Keep the surrounding Applied-grammar prose intact.
  - `templates/adr-template/template.md.tmpl`: replace "Later status events carry the frozen
    content digest; direct Implemented events also carry the batch state sequence." with "Later
    status events carry the latest content stamp, and an Amended event records each post-Accepted
    amendment with its new digest; direct Implemented events also carry the batch state sequence."
  - `templates/agents-doc/AGENTS.md.tmpl` line 33, Append-only ADRs bullet: replace "its meaning
    is frozen once it leaves Proposed (a meaning-preserving schema retrofit may migrate its
    encoding)" with "its meaning freezes at a terminal status, every earlier amendment recorded as
    an Amended history event (a meaning-preserving schema retrofit may migrate its encoding)".
- [ ] **Task 2.5: re-render and reconcile.** Run `./x render` (re-renders this repo and
  `examples/sundial`), stage all regenerated files, run `./x check` (expected: clean). If the gate
  surfaces template-coupled test failures (catalog spine, rendered-output, or eval fixtures),
  update those fixtures to the new prose in this same transaction; consult the docs/pitfalls.md
  entry on V2-ADR fixtures before editing any test fixture ADR.
- [ ] **Phase-close: stage, check, gate, and commit.** `awf check --staged` then `./x gate`, both
  green.

```commit
feat(rendering): publish the amendment-until-terminal lifecycle prose
```

## Phase 3: repo-local prose parts

**Execution mode: inline.** This repo's own narrative surfaces; render closes the loop.

- [ ] **Task 3.1: domain narrative.** In `.awf/domains/parts/adr-system/current-state.md`: replace
  "Status events carry the frozen content digest." with "Status events repeat the latest content
  stamp, and Amended events record each post-Accepted amendment."; rework the paragraph opening
  "Append-only protects rationale, not orthography" so its live-body sentence reads: a V2 body's
  meaning stays amendable until a terminal status with each amendment recorded as an Amended
  event, a V1 body freezes once it leaves Proposed, and a meaning-preserving schema retrofit may
  migrate either encoding (keep the ADR-0115/ADR-0118 orthography prose unchanged).
- [ ] **Task 3.2: glossary, pitfalls, roadmap.**
  - `.awf/docs/glossary.yaml`, term `State changes`: replace the opening "The frozen ADR-to-topic
    operation declaration:" with "The ADR-to-topic operation declaration, each operation frozen
    once an Applied event references it:".
  - `.awf/docs/pitfalls.yaml`, the ADR-0154 amendment-freeze lesson: append one sentence to the
    body: "Since ADR-0186 the amendment window extends to any non-terminal status and each
    post-Accepted amendment appends its own Amended event, so the reviewer re-dispatch travels
    with the amendment commit rather than racing a freeze."
  - `.awf/docs/parts/roadmap/deferred.md`, section "A frozen-state ADR flip can smuggle unreviewed
    section content": rewrite its premise to the residual case: since ADR-0186 the stamp chain
    makes every post-first-stamp flip content-pure by validation, so the candidate audit rule
    narrows to a direct flip out of Proposed whose commit also mutates digest-covered content.
- [ ] **Task 3.3: render.** `./x render`; stage regenerated docs; `./x check` clean.
- [ ] **Phase-close: stage, check, gate, and commit.** `awf check --staged` then `./x gate`, both
  green.

```commit
docs(adr-system): align repo prose with amendable-until-terminal
```

## Phase 4: claim application and Implemented flip (deferred to terminal review)

**Execution mode: inline.** Precondition, per ADR-0186 item 7: execute only after the applicable
terminal implementation review of phases 1-3 settles with zero findings (including any renewed
post-merge review; in the divergent case this phase lands on the integration target). This is the
direct implicit-batch transaction covering both declared operations.

- [ ] **Task 4.1: apply the claim operations.** In
  `.awf/topics/parts/adr-system/adr-lifecycle/current-state.md`:
  - Add the new claim (exact, placed in the file's existing claim ordering):

    ```
    ### `invariant: adr-amendable-until-terminal`

    A current-state-v2 ADR's digest-covered content is amendable while Proposed, Accepted, or
    Implementing and freezes permanently at a terminal status. Post-Accepted amendment is
    recorded as a stamp chain: only an Amended event introduces a new digest, which must differ
    from the preceding stamp; a status event repeats the preceding stamp or establishes the
    first; the latest stamp must equal the computed content digest; and an amendment never
    alters or removes an operation already referenced by an Applied event.
    Origin: ADR-0186
    Backing: test
    ```
  - Update `adr-status-enum-and-matrix`: prose becomes "Every governed ADR is routed by the two
    immutable format cutoffs: V1 retains its four statuses and five legal edges, while V2
    recognizes Proposed, Accepted, Implementing, Implemented, and Abandoned, recognizes status,
    Applied, and Amended history events, and accepts only the format-specific status,
    history-event, digest-chain, and application-cardinality transitions."; `Revised-by:` becomes
    `ADR-0143, ADR-0186`; `Origin:` and `Backing:` unchanged.
  - Add the proof marker `// invariant: adr-system/adr-lifecycle:adr-amendable-until-terminal` on
    the Phase 1 chain-rule test function in `internal/adr/format_test.go` (same comment form as
    the existing markers in `internal/adr/adr_test.go`).
- [ ] **Task 4.2: flip ADR-0186 and this plan.** In
  `docs/decisions/0186-adr-content-amendable-until-implemented-via-amended-events.md`: set
  frontmatter `status: Implemented` and append the terminal event
  `- <today>: Implemented; content-sha256: <digest>; state-sequence: <n>`. Obtain the exact digest
  and next sequence by staging the transaction and reading them from the `awf check --staged`
  findings, then correcting the line and re-staging; the terminal state is `awf check --staged:
  clean`. Set this plan's frontmatter to `status: Implemented`. Run `./x render` to regenerate
  `docs/decisions/INDEX.md` and `docs/topics/adr-system/adr-lifecycle.md`; stage them.
- [ ] **Phase-close: stage, check, gate, and commit.** `awf check --staged` then `./x gate`, both
  green.

```commit
docs(adr): declare the amendable-until-terminal claims (implements 0186)
```

## Verification

- `./x gate` green at every phase close (includes 100% coverage, deadcode, prose and memory
  checks, and the sundial example's render, check, and tests).
- `./x check` clean after each render-bearing phase.
- `grep -rn "frozen once it leaves Proposed" templates/` returns no hit outside a deliberate V1
  statement; `grep -rn "amendment-while-proposed" templates/ .awf/` and
  `grep -rn "implementation step's final commit" templates/` return no output.
- After Phase 4, `./awf topic adr-system/adr-lifecycle` lists `adr-amendable-until-terminal` with
  `backing: test`, and `./awf check` reports its proof marker resolved.
- The full corpus parses under the new binary with zero record edits outside ADR-0186 itself
  (`./awf check` clean at Phase 1 close proves this).

## Notes

- The residual first-stamp smuggle case stays a deferred roadmap audit-rule idea (narrowed in
  Phase 3), not part of this plan.
- ADR-0186 itself takes the direct Proposed-to-Implemented path, exercising first-stamp
  establishment; it never carries an Amended event, and any post-review correction before Phase 4
  is a plain Proposed amendment.
- Phase 4's deferred execution is itself the first live use of the reordered flip; the rendered
  reviewing-impl skill from Phase 2 carries the matching instruction.
