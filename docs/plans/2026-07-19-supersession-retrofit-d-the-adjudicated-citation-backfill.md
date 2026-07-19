---
date: 2026-07-19
adrs: [131]
status: Proposed
---
# Plan: Supersession retrofit D: the adjudicated citation backfill

## Goal

Land every relation token, back-pointer edge, and proof-marker retarget that ADR-0131 decides, so
its citation check reports nothing and Plan C Phase 3 can enable it green.

**Non-goals:** this plan changes no production Go, does not enable the check (Plan C Phase 3 owns
that), and does not flip ADR-0131 to `Implemented`.

**No step in this plan asserts a count.** Every verification is a command whose output must reach
a terminal state, because the site set is live: it moved once when the branch fell behind main, and
again when ADR-0131's own amendment added claims to the very Decision items that define the check.
A literal figure written here is stale the moment the corpus moves, and a stale figure reads as a
failed step. The check's output is the authority; the executor's job is to drive it to zero, not to
match a number.

## Architecture summary

Two phases split by blast radius, mirroring the Plan A/B split for the same reason.

Phase 1 lands the item-anchored work: the `refines:` tokens where a clause of the target survives
but the anchor is adapted, the inert `cites:` tokens everywhere else, and the `related:` edges they
owe. No item-anchored verdict retires anything, so no proof marker moves and no ADR's coverage
advances.

Phase 2 lands the slug-anchored work, one commit per retirement. Every carrier is `Implemented`, so
a `supersedes-invariant:` token retires its slug the moment it lands; the retirement and its
proof-marker retarget therefore travel together.

Bulk tasks take the batch form, keyed to the check's output rather than a copied list.

## File structure

- **Created:** none.
- **Modified:** `docs/decisions/*.md` (relation tokens on carriers, `related:` edges on targets),
  `docs/decisions/ACTIVE.md` and `.awf/awf.lock` (regenerated), `internal/project/project_test.go`,
  `internal/adr/adr_test.go`, `internal/project/install_test.go` (marker retargets).
- **Deleted:** none.

## Phase 0: make the check runnable and current

The check's code sits unlanded on `wip/citation-check`, which trails `main`, and it predates
ADR-0131's per-carrier slug scoping. Every later site set comes from its output.

- [ ] **Task 0.1: Rebase onto `main` and record the result durably.**

  ```
  git worktree add /tmp/awf-planD wip/citation-check
  cd /tmp/awf-planD && git rebase main
  ```

  The rebase applies cleanly: `internal/adr/citations.go` and `internal/project/citations.go` are
  new files `main` does not touch, and the only shared file is `internal/project/check.go`, whose
  wiring is one added call.

  Then force-update the branch so the rebased state survives this session:
  `git branch -f wip/citation-check HEAD`. A `/tmp` worktree is not a durable record, and every
  verification in this plan runs through a binary built from this branch.

- [ ] **Task 0.2: Implement the per-carrier slug scoping (ADR-0131 Decision 2).** A slug claim is
  satisfied by a `supersedes-invariant:` token for that slug anywhere in the carrier's `## Decision`
  section; a `cites-invariant:` suppresses only at its own item; an item claim is satisfied only at
  its own item. Write the test first per `awf-tdd`: a carrier stating a slug override at item 1 with
  the retirement token at item 9 must produce no finding, and the same carrier with only
  `cites-invariant:` at item 1 must still report the item 9 claim.

  **The test carries `// invariant: citation-check-slug-claims-per-carrier`.** The amendment
  declared that slug in ADR-0131's Invariants section, and a backed slug with no proof marker reds
  `awf check` the moment ADR-0131 becomes `Implemented`. Both halves of the rule need exercising,
  since the second is what keeps the scoping from costing recall.

  ```
  go build -o /tmp/awf-planD-bin ./cmd/awf
  /tmp/awf-planD-bin check 2>&1 | grep -E 'doc-gated-skill-suppressed|bootstrap-pin'
  ```

  Expected: no output. Those are the two claims the scoping rule dissolves (ADR-0081 item 7, whose
  token sits at item 10; ADR-0085 item 1, whose token sits at item 9). Their disappearance is the
  assertion, not the total.

- [ ] **Task 0.3: Confirm the site-set command works.** The batch tasks below reference it.

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' \
    | sed -E 's/.*decisions\/([0-9]+)-[^:]*\.md: ADR-[0-9]+ Decision item ([0-9]+) states an override of ADR-([0-9]+)#([^ ]+) but.*/\1 \2 \3 \4/'
  ```

  Expected shape: lines of `carrier item target anchor`. A numeric fourth field is item-anchored
  (Phase 1); a slug is slug-anchored (Phase 2). Read the split off this output rather than from any
  figure written in this plan or in Plan C's execution status, both of which have been wrong.

Phase 0 lands no commit. The check code belongs to Plan C Phase 3's commit, whose dead-code gate
(ADR-0063, no ignore directive) forbids landing the extraction without its caller, and whose 100%
coverage gate forbids landing it untested. Task 0.2's test and scoping change are carried into that
commit; the Notes record the hand-off.

## Phase 1: item-anchored tokens and their back-pointers

- [ ] **Task 1.1: Insert the `refines:` tokens.** These are the item-anchored verdicts where a
  clause of the target survives but the anchor is adapted. Each is a distinct clause-set judgment,
  so each gets its own diff.

  | Carrier | Item | Token |
  |---|---|---|
  | `0120-structured-machine-checked-adr-supersession.md` | 11 | `refines: ADR-0116#6` |
  | `0131-complete-and-self-enforcing-supersession-records.md` | 6 | `refines: ADR-0116#4` |
  | `0117-advisory-plain-punctuation-audit-rule-for-authored-prose.md` | 8 | `refines: ADR-0115#9` |
  | `0118-retroactive-plain-punctuation-sweep-of-authored-adr-and-plan-prose.md` | 1 | `refines: ADR-0115#5` |
  | `0121-whole-line-authoring-comments-in-templates-and-parts.md` | 2 | `refines: ADR-0083#4` |

  Locate every site by its quoted text, never by line number: line numbers in this effort have
  drifted repeatedly. Insert the token adjacent to the prose that already states the claim, which is
  ADR-0120 Decision 9's third carve-out shape. Before writing each one, re-read the target item and
  confirm a clause survives; if none does, the verdict is a retirement and belongs in Phase 2.

  Representative, ADR-0118 item 1:

  ```diff
  -   This is partial-item supersedence of **ADR-0115 Decision item 5**, whose "the heading line
  +   This is partial-item supersedence of **ADR-0115 Decision item 5** (`refines: ADR-0115#5`),
  +   whose "the heading line
  ```

  The other four follow the same shape at their own citation sites.

- [ ] **Task 1.2: Insert the item-anchored `cites:` tokens (batch).** Every remaining item-anchored
  finding takes an inert token: the carrier mentions, quotes, narrates, or reasons from the target
  without changing it.

  **Representative**, ADR-0129 item 7 narrating ADR-0128's act on a third ADR:

  ```diff
  -   ADR-0120 item 3's single-claimant check incidentally prevented full-supersession cycles, and
  -   ADR-0128 item 1 removes it.
  +   ADR-0120 item 3's single-claimant check (`cites: ADR-0120#3`) incidentally prevented
  +   full-supersession cycles, and ADR-0128 item 1 removes it (`cites: ADR-0128#1`).
  ```

  **Edge**, ADR-0131 item 5, where one item cites three anchors and each needs its own token at that
  item, because ADR-0129 Decision 2 scopes item claims per item and a token at item 7 does not
  answer a claim at item 5:

  ```diff
  -   ADR-0120 Decision 1 recognizes tokens only inside `## Decision`, so a Context-section citation
  +   ADR-0120 Decision 1 (`cites: ADR-0120#1`) recognizes tokens only inside `## Decision`, so a
  +   Context-section citation
  ```

  **Affected-site set**, item-anchored findings after Task 1.1:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -E '#[0-9]+ '
  ```

  Several of these sit in ADR-0131's own Decision items 2 and 9, added by its amendment: item 2
  names ADR-0081 item 7, ADR-0085 item 1, and ADR-0129 Decision 2 while explaining the scoping rule,
  and item 9 names ADR-0020 Decision 6 and ADR-0032 Decision 7 while explaining when their tokens go
  live. All take `cites:`. Item 9's two are the subtle case and were decided explicitly: item 1
  already carries `refines:` on both anchors, but item 9 asserts nothing about what those items
  commit to, so every clause survives and the relation differs per item because the act does.

  **Post-check**, no item-anchored claim survives:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -cE '#[0-9]+ '
  ```

  Expected: `0`.

- [ ] **Task 1.3: Add the back-pointer edges the Phase 1 tokens owe (batch).** A token of any
  relation from a carrier of any status requires the target's `related:` to name the carrier
  (`internal/project/supersession.go:178-179`); an inert `cites:` is no exception. This is what
  red-lined Plan B Phase 1.

  **Affected-site set**, derived rather than listed, because ADR-0020 and ADR-0032 already gained
  their edges in `a4aa636b`:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-token-backpointer'
  ```

  **Arrays are ascending.** An edge from a low-numbered carrier is a mid-array insert; appending
  produces an out-of-order array that reds the gate. Read each target's current array before
  editing rather than assuming a position: edges from ADR-0131 append because 131 is the corpus
  maximum, but edges from ADR-0129 and other mid-corpus carriers do not.

  **Post-check:**

  ```
  /tmp/awf-planD-bin check 2>&1 | grep -c 'adr-token-backpointer'
  ```

  Expected: `0`.

- [ ] **Task 1.4: Verify and commit.** Run `./x sync` first: a relation token regenerates
  `.awf/awf.lock` and `docs/decisions/ACTIVE.md`. A `related:`-only edit regenerates nothing, and
  `docs/domains/*.md` moves only when a target gains a **new** retiring ADR, which no Phase 1
  verdict does: every Phase 1 token is a `refines:` or a `cites:`, and neither annotates a domain
  index line. Phase 2 is the opposite case and stages domain files in three of its tasks.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/ .awf/awf.lock
  ```

  Expected from `./x check`: a bare `awf check: clean`, with **no** citation findings at all. The
  check is not wired into `awf check` on `main` (`internal/project/citations.go` does not exist
  there), so the remaining slug claims are visible only through `/tmp/awf-planD-bin`. That is the
  load-bearing reason this phase is independently gateable, and it means a clean run here is the
  expected result rather than evidence the step did nothing.

  ```commit
  docs(adr): encode the item-anchored citation claims
  ```

## Phase 2: slug retirements and proof retargets

Every carrier here is `Implemented`, so each `supersedes-invariant:` token retires its slug the
instant it lands.

**The splitting hazard is asymmetric, and only one direction is caught.** Landing a marker retarget
without its token leaves the old slug declared by an Implemented ADR with no proof, which is an
`Unbacked` finding and reds the gate. Landing the token without the retarget leaves a marker naming
a retired slug, which is only an advisory `note:` and never enters the failure count
(`internal/invariants/invariants.go`). So the gate protects the harmless order and not the harmful
one. Land the token and its retarget together; if they must be separated, put the token last.

**Every redeclaration pins what its retargeted marker asserts, never what the retired sentence
claimed.** ADR-0131 dropped three clauses on exactly this ground during its own review. Read the
marker before trusting the declaration.

- [ ] **Task 2.1: Insert the `cites-invariant:` tokens (batch).** Slug-anchored findings that are
  not retirements take the inert key: the carrier names the slug without falsifying its declared
  sentence.

  **Representative**, ADR-0016 item 7 on ADR-0015's `provenance-banner`, the governing precedent
  (the declaration names no path, so relocating the config root leaves it literally true):

  ```diff
  -   narrows **ADR-0015 `inv: provenance-banner`** only insofar as the banner text now names
  +   narrows **ADR-0015 `inv: provenance-banner`** (`cites-invariant: ADR-0015#provenance-banner`)
  +   only insofar as the banner text now names
  ```

  **Edge**, ADR-0032 item 7, which Task 2.4 also edits for a retirement, so one Decision item ends
  up carrying both keys for different slugs:

  ```diff
  -   ADR-0023's `inv: init-force-backs-up` and the lock-tracked-removal substance of
  +   ADR-0023's `inv: init-force-backs-up` (`cites-invariant: ADR-0023#init-force-backs-up`) and
  +   the lock-tracked-removal substance of
  ```

  **Affected-site set**, slug-anchored findings minus the three retirements below:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -v '#[0-9]* ' \
    | grep -vE 'sync-generates-active-md|render-active-md|uninstall-removes-lock-tracked'
  ```

  **Post-check**, only the three retirement claims remain:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -v '#[0-9]* ' \
    | grep -cE 'sync-generates-active-md|render-active-md|uninstall-removes-lock-tracked'
  ```

  Expected: the output of the site-set command above is empty, and this count equals the number of
  retirement claims still pending (three before Task 2.2, zero after Task 2.4).

  **This task commits on its own.** Its tokens owe back-pointer edges like any others, and the
  pre-commit hook runs `./x check`, so they cannot be deferred to a later task. Derive them:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-token-backpointer'
  ```

  Expected after the edits: no output.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/ docs/domains/ .awf/awf.lock
  ```

  ```commit
  docs(adr): encode the inert slug citations
  ```

- [ ] **Task 2.2: Retire `ADR-0005#sync-generates-active-md`.** One commit. ADR-0020 Decision 6 made
  `ACTIVE.md` always render, falsifying the declaration's "writes no `ACTIVE.md` and prunes any
  previously locked one" clause.

  Two edits. The token on `docs/decisions/0020-dead-reference-check.md` item 6, beside the prose
  that already names the falsified clause:

  ```diff
  -   - **ADR-0005 `inv: sync-generates-active-md`**: its "an absent or ADR-less decisions dir ...
  +   - **ADR-0005 `inv: sync-generates-active-md`**
  +     (`supersedes-invariant: ADR-0005#sync-generates-active-md`): its "an absent or ADR-less
  +     decisions dir ...
  ```

  And both markers in `internal/project/project_test.go` (on
  `TestSyncGeneratesActiveMDAndCheckDetectsStaleness` and
  `TestSyncRendersPlaceholderActiveMDWithoutADRs`) retargeted:

  ```diff
  -// invariant: sync-generates-active-md
  +// invariant: sync-always-writes-active-md
  ```

  The adjacent `// invariant: check-active-md-stale` marker on the first test is a different slug;
  leave it untouched. The successor is already declared in ADR-0131's Invariants section, so no ADR
  Invariants edit is needed.

  **Third edit: ADR-0005 gains two `related:` entries in this same commit**, `20` and `131`
  (`related: [1, 4]` today, so both append). They are different obligations and neither satisfies
  the other. `20` is required by the check: `internal/project/supersession.go:179` demands the
  target name the **carrier** of the token, and the carrier here is ADR-0020, not ADR-0131. `131`
  is the documentation edge ADR-0131 Decision 1 asks for, so a reader of ADR-0005 reaches the ADR
  that redeclared its slug. ADR-0009 is the landed precedent for carrying both: its `related:`
  names its carriers `15` and `16` alongside `131`.

  **Domain docs move here, unlike in Phase 1.** A retirement annotates the target's generated
  domain index line (`docs/domains/tooling.md:57` shows the landed shape,
  `ADR-0023 ... -> superseded by ADR-0032`). ADR-0005 is indexed in `docs/domains/adr-system.md`
  and `docs/domains/config.md`, both currently unannotated, so `./x sync` regenerates both.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/0020-dead-reference-check.md docs/decisions/0005-docsdir-layout-and-builtin-active-md.md internal/project/project_test.go docs/decisions/ACTIVE.md docs/domains/adr-system.md docs/domains/config.md .awf/awf.lock
  ```

  `./x check` is not optional here and `./x gate` does not cover it: the gate runs tests, coverage,
  vet, lint, deadcode, and pincheck, but never `awf check`. Every corpus finding this phase can
  produce is invisible to the gate alone. Expected: `awf check: clean`.

  Expected: gate green, `awf invariants: clean`. The advisory note list grows by one, because the
  retargeted marker now names a slug only the still-`Proposed` ADR-0131 declares. A growing note
  list across Phase 2 is expected, not a regression; notes never fail the gate.

  ```commit
  docs(adr): retire ADR-0005's stale active-md invariant
  ```

- [ ] **Task 2.3: Retire `ADR-0006#render-active-md`.** One commit. Same carrier item, same cause:
  the declaration's "returns `\"\"` when the directory holds no ADRs" clause is false.

  The token on `docs/decisions/0020-dead-reference-check.md` item 6 beside its
  `ADR-0006 inv: render-active-md` prose, in the shape shown above; and all three
  `// invariant: render-active-md` markers in `internal/adr/adr_test.go` (on
  `TestRenderActiveMDGroupsByStatus`, `TestRenderActiveMDGroupsSupersededVariants`, and
  `TestRenderActiveMDPlaceholderWhenNoADRs`) retargeted to
  `render-active-md-grouped-or-placeholder`.

  **ADR-0006 gains `20` and `131` in `related:` in this same commit** (`related: [1, 5]` today, so
  both append), for the reasons given in Task 2.2: `20` is the carrier edge the check enforces,
  `131` the documentation edge Decision 1 asks for.

  ADR-0006 is indexed in `docs/domains/adr-system.md` and `docs/domains/rendering.md`, so both
  gain the retirement annotation.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/0020-dead-reference-check.md docs/decisions/0006-frontmatter-parser-and-skill-validation.md internal/adr/adr_test.go docs/decisions/ACTIVE.md docs/domains/adr-system.md docs/domains/rendering.md .awf/awf.lock
  ```

  ```commit
  docs(adr): retire ADR-0006's stale render-active-md invariant
  ```

- [ ] **Task 2.4: Retire `ADR-0023#uninstall-removes-lock-tracked`.** One commit. ADR-0032
  Decision 7 dropped the declaration's middle conjunct, so the sentence is false as written.

  The token on `docs/decisions/0032-remove-automatic-hook-handling.md` item 7 beside the prose
  naming the dropped clause; and the `// invariant: uninstall-removes-lock-tracked` marker in
  `internal/project/install_test.go` (inside `TestUninstallSkipsEscapingLockPaths`) retargeted to
  `uninstall-removes-lock-entries`.

  Do **not** widen that test to match the retired sentence's other clauses. ADR-0131 deliberately
  narrowed the successor to what the marker asserts; re-establishing the lock-file-removal or
  authored-config-survival clauses is a successor ADR's job, with the assertion that earns it.

  **ADR-0023 gains `131` only** (`related: [3, 16, 32]` today, so an append). Unlike Tasks 2.2 and
  2.3 it already carries its carrier edge `32`, so only the documentation edge is owed here.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/0032-remove-automatic-hook-handling.md docs/decisions/0023-safe-adoption-existing-repos.md internal/project/install_test.go docs/decisions/ACTIVE.md .awf/awf.lock
  ```

  ```commit
  docs(adr): retire ADR-0023's stale uninstall invariant
  ```

- [ ] **Task 2.5: Sweep for any back-pointer edge still owed.** Tasks 2.2 to 2.4 each land their
  own edges, because they must: `.awf/hooks/pre-commit.sh` runs `./x check`, and a
  `supersedes-invariant:` token whose target does not name the carrier raises `adr-token-backpointer`
  at commit time. Deferring the edges to a trailing commit would make those three tasks
  uncommittable, which is the defect this plan records as having red-lined Plan B Phase 1.

  This task is therefore a derived sweep, not a scheduled edit. Run:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-token-backpointer'
  ```

  Expected: no output, and no commit. If the command reports anything, an earlier task's edge was
  missed; fix it in a follow-up commit naming the target, and record which task under-scheduled it
  in the Notes.

## Verification

The acceptance check is that the citation check reports nothing on this corpus:

```
/tmp/awf-planD-bin check 2>&1 | grep -c 'adr-unencoded-claim'
```

Expected: `0`. This is the precondition Plan C Task 3.6 assumed and did not get.

No ADR's status may drift. No verdict here retires an item anchor, and the three slug retirements
each move a different target by one anchor, so none reaches full coverage:

```
./x check
```

Expected: `awf check: clean`.

The retirements must cost no enforcement: every retired slug's proof survives under its successor,
so no marker is stranded and the test count is unchanged.

```
./x gate && ./x invariants
```

Expected: green at 100% coverage, and `awf invariants: clean`. The advisory notes now cover six
slugs rather than three: the original `cites-token-uncounted`, `cites-token-unrendered`, and
`residue-exemptions-pinned-three`, plus the three successors this plan retargets markers onto. All
six resolve when Plan C Task 3.6 flips ADR-0131 to `Implemented`.

## Notes

- **This plan does not flip ADR-0131.** Plan C Task 3.6 owns the flip and must re-run `./x gate`
  afterwards: ADR-0131's own claims contribute zero coverage while it is `Proposed`
  (`internal/adr/coverage.go:166` excludes it), so they arm only at the flip. This plan's green does
  not carry across that boundary.
- **Plan C Phase 3 inherits a second rebase.** `wip/citation-check` will trail `main` again by this
  plan's commits, so its first step is to rebase onto this plan's final commit. Phase 0's Task 0.1
  force-updates the branch so that rebase starts from the corrected state.
- **Phase 0's scoping change belongs to Plan C Phase 3's commit**, not to this plan, under the
  dead-code and coverage gates. This plan consumes it as working state.
- **Counts are deliberately absent.** An earlier draft of this plan asserted a total of 44 and a
  30/12 item/slug split. Both were wrong: 44 was a merge-base measurement that became 49 once
  ADR-0131's own amendment added claims to the Decision items defining the check, and the split was
  never right at any commit. Plan C's execution status carries a third figure. Correct that line
  when Plan C Phase 3 resumes, or delete it in favour of the command.
- **Plan C Task 3.3's backing enumeration is stale.** It says "back all twelve ADR-0131
  invariants", a list written before the amendment. The ADR now declares more: this plan's Task 0.2
  adds a proof for `citation-check-slug-claims-per-carrier`, and Phase 2 retargets markers onto
  `sync-always-writes-active-md`, `render-active-md-grouped-or-placeholder`, and
  `uninstall-removes-lock-entries`. Task 3.3 must cover the residue rather than the twelve, or
  ADR-0131's flip lands an unbacked slug. Enumerate from the ADR's Invariants section, not from the
  plan's figure.
- **Plan C Task 3.4's doc sweep names the right surfaces but predates the amendment.** Its surface
  list (`.awf/docs/glossary.yaml`, the two `current-state.md` parts, `adr-reviewer.yaml`) is still
  correct; its content is not. The sweep must additionally teach the per-carrier slug-scoping split
  (slug claims satisfied anywhere in the carrier's Decision section, `cites-invariant:` suppressing
  only at its own item) and name the three redeclared slugs in the invariants domain current-state.
- **The `cites:` token has no changelog entry.** `grep -n cites changelog/CHANGELOG.md` returns
  nothing, yet the parser change landed in `b1be6de4` after `v0.16.0` and is adopter-facing. Plan C
  Task 3.4's doc sweep owes an `[Unreleased]` entry; recorded here because `repoaudit` cannot see it
  from a later commit range.
- **Deferred, unchanged from Plan C:** `refines: ADR-0034#1` at `docs/decisions/0057-*.md:86` parses
  with `CarrierItem: 0` and has no rationale site (ADR-0129 Decision 2). Fixing it needs its own
  decision, because moving the token is a content edit the append-only rule forbids.
- **A marker re-point is owed at the flip.** `TestCheckTokenRetirementIgnoresCitesInvariant`
  (`internal/invariants/invariants_test.go`) carries `// invariant: token-retirement-implemented-only`,
  a status slug, while the test exercises relation. The fitting slug is ADR-0131's
  `cites-token-uncounted`, unusable while ADR-0131 is `Proposed`. Re-point it in Plan C Task 3.6;
  do not downgrade it to `touches-invariant:` meanwhile, which would leave the test unmarked.
- **Verdict provenance.** The claims were adjudicated by fresh-context agents, one per target ADR,
  each returning the target's surviving-or-dead clause quoted with a file:line, under the rule read
  the target's clause set, never the carrier's verb. If a site looks wrong during execution, re-read
  the target rather than the carrier.
