---
date: 2026-07-19
adrs: [131]
status: Proposed
---
# Plan: Supersession retrofit D: the adjudicated citation backfill

## Goal

Land every relation token, back-pointer edge, invariant redeclaration, and proof-marker retarget
that ADR-0131 decides, so that its citation check reports nothing and Plan C Phase 3 can enable it
green. The verdicts are already adjudicated: the check's first run reported 44 unencoded claims,
all 44 were judged against the cited target's clause set, and the result is 3 retirements, 5
refinements, 34 informational citations, and 2 claims the per-carrier slug scoping dissolves.

**Non-goals:** this plan changes no production Go, does not enable the check (Plan C Phase 3 owns
that), and does not flip ADR-0131 to `Implemented`.

## Architecture summary

Two phases split by blast radius, mirroring the Plan A/B split for the same reason.

Phase 1 lands the item-anchored work: the inert `cites:` tokens and the `refines:` tokens, plus
the `related:` back-pointer edges they owe. No item-anchored verdict retires anything, so no proof
marker moves and no ADR's coverage advances. This is the bulk and it is mechanically safe.

Phase 2 lands the slug-anchored work, one commit per retirement. Every carrier is `Implemented`,
so a `supersedes-invariant:` token retires its slug the moment it lands; the retirement, the
successor declaration, and the proof-marker retarget therefore travel together. Splitting them
would leave a marker naming a slug no ADR declares.

The batch tasks are keyed to the check's own output rather than a hand-copied site list. The
affected-site set is a command, so a rebase, a re-render, or the slug-scoping change cannot stale
it, and the post-check is the same command's count reaching zero.

## File structure

- **Created:** none.
- **Modified:** `docs/decisions/*.md` (relation tokens on carriers, `related:` edges on targets),
  `docs/decisions/ACTIVE.md` and `.awf/awf.lock` (regenerated every commit),
  `internal/project/project_test.go`, `internal/adr/adr_test.go`,
  `internal/project/install_test.go` (marker retargets).
- **Deleted:** none.

## Phase 0: make the check runnable

The check's code sits unlanded on `wip/citation-check`, which trails `main`, and it predates
ADR-0131's per-carrier slug scoping. Every later task's site set comes from its output, so it must
run and be correct before anything is written.

- [ ] **Task 0.1: Rebase the branch onto `main`.** `main` has moved by the backing-guard fix and
  the ADR-0131 amendment.

  ```
  git worktree add /tmp/awf-planD wip/citation-check
  cd /tmp/awf-planD && git rebase main
  ```

  Expected: the rebase applies cleanly. `internal/adr/citations.go` and
  `internal/project/citations.go` are new files that `main` does not touch; the only shared file is
  `internal/project/check.go`, whose Check() wiring is a single added call.

- [ ] **Task 0.2: Implement the per-carrier slug scoping (ADR-0131 Decision 2).** A slug claim is
  satisfied by a `supersedes-invariant:` token for that slug anywhere in the carrier's `## Decision`
  section; a `cites-invariant:` suppresses only at its own item; an item claim is satisfied only at
  its own item. Write the test first, per `awf-tdd`: a carrier whose item 1 states a slug override
  and whose item 9 carries the retirement token must produce no finding, and a carrier whose item 1
  carries only `cites-invariant:` while item 9 states an unencoded override must still report.

  Verify against the two corpus sites the ADR names:

  ```
  go build -o /tmp/awf-planD-bin ./cmd/awf
  /tmp/awf-planD-bin check 2>&1 | grep -c 'adr-unencoded-claim'
  ```

  Expected: `42`. The two dissolved claims are ADR-0081 item 7 on
  `ADR-0013#doc-gated-skill-suppressed` (token at item 10) and ADR-0085 item 1 on
  `ADR-0040#bootstrap-pin` (token at item 9). Confirm both are absent:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep -E 'doc-gated-skill-suppressed|bootstrap-pin'
  ```

  Expected: no output.

- [ ] **Task 0.3: Record the site set.** The later batch tasks reference this command; run it once
  here to confirm it produces the expected shape.

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' \
    | sed -E 's/.*decisions\/([0-9]+)-[^:]*\.md: ADR-[0-9]+ Decision item ([0-9]+) states an override of ADR-([0-9]+)#([^ ]+) but.*/\1 \2 \3 \4/'
  ```

  Expected: 42 lines of `carrier item target anchor`. 30 have a numeric anchor (item-anchored,
  Phase 1); 12 have a slug anchor (Phase 2).

Phase 0 lands no commit of its own: the check code belongs to Plan C Phase 3, whose dead-code gate
(ADR-0063) forbids landing the extraction without its caller. The rebased branch and the built
binary are working state for this plan. Task 0.2's test and scoping change are carried into Plan C
Phase 3's commit, and this plan's Notes record that hand-off.

## Phase 1: item-anchored tokens and their back-pointers

- [ ] **Task 1.1: Insert the 5 `refines:` tokens.** These are the item-anchored verdicts where a
  clause of the target survives but the anchor is adapted. Each is a distinct judgment, so each is
  an exact diff rather than a batch member. Insert the token adjacent to the prose citation that
  already states the claim, which is ADR-0120 Decision 9's third carve-out shape.

  Locate every site by its quoted text, never by line number: plan line numbers in this effort have
  drifted repeatedly.

  | Carrier | Item | Token |
  |---|---|---|
  | `docs/decisions/0120-structured-machine-checked-adr-supersession.md` | 11 | `refines: ADR-0116#6` |
  | `docs/decisions/0131-complete-and-self-enforcing-supersession-records.md` | 6 | `refines: ADR-0116#4` |
  | `docs/decisions/0117-advisory-plain-punctuation-audit-rule-for-authored-prose.md` | 8 | `refines: ADR-0115#9` |
  | `docs/decisions/0118-retroactive-plain-punctuation-sweep-of-authored-adr-and-plan-prose.md` | 1 | `refines: ADR-0115#5` |
  | `docs/decisions/0121-whole-line-authoring-comments-in-templates-and-parts.md` | 2 | `refines: ADR-0083#4` |

  Representative diff, ADR-0118 item 1, whose prose already states the claim:

  ```diff
  -   This is partial-item supersedence of **ADR-0115 Decision item 5**, whose "the heading line
  -   only, never the body" clause is overridden; ADR-0115 stays Implemented and every other item
  -   stands.
  +   This is partial-item supersedence of **ADR-0115 Decision item 5** (`refines: ADR-0115#5`),
  +   whose "the heading line only, never the body" clause is overridden; ADR-0115 stays
  +   Implemented and every other item stands.
  ```

  Verify each token parses and none was mistyped:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep -c 'adr-unencoded-claim'
  ```

  Expected: `37` (42 minus these 5).

- [ ] **Task 1.2: Insert the item-anchored `cites:` tokens (batch).** Every remaining item-anchored
  finding takes an inert `cites:` token: the carrier mentions, quotes, narrates, or reasons from the
  target without changing it.

  **Representative site**, ADR-0129 item 7 narrating ADR-0128's act on a third ADR:

  ```diff
  -   ADR-0120 item 3's single-claimant check incidentally prevented full-supersession cycles, and
  -   ADR-0128 item 1 removes it.
  +   ADR-0120 item 3's single-claimant check (`cites: ADR-0120#3`) incidentally prevented
  +   full-supersession cycles, and ADR-0128 item 1 removes it (`cites: ADR-0128#1`).
  ```

  **Edge site**, ADR-0131 item 5, where one item cites three anchors and each needs its own token
  at that item (ADR-0129 Decision 2 scopes item claims per item, so a token at item 7 does not
  answer a claim at item 5):

  ```diff
  -   ADR-0120 Decision 1 recognizes tokens only inside `## Decision`, so a Context-section citation
  +   ADR-0120 Decision 1 (`cites: ADR-0120#1`) recognizes tokens only inside `## Decision`, so a
  +   Context-section citation
  ```

  **Affected-site set**, the item-anchored remainder after Task 1.1:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -E '#[0-9]+ '
  ```

  **Post-check**, no item-anchored claim survives:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -cE '#[0-9]+ '
  ```

  Expected: `0`.

- [ ] **Task 1.3: Add the back-pointer edges the Phase 1 tokens owe.** A token of any relation from
  a carrier of any status requires the target's `related:` to name the carrier
  (`internal/project/supersession.go:178`); an inert `cites:` is no exception. This is what red-lined
  Plan B Phase 1.

  Derive the exact set rather than trusting a list, because ADR-0020 and ADR-0032 already gained
  their edges in `a4aa636b`:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-token-backpointer'
  ```

  **Arrays are ascending, so check each insert position.** Several are mid-array, not appends;
  appending produces an out-of-order array that reds the gate.

  Representative diff, a mid-array insert on ADR-0114:

  ```diff
  -related: [113, 115, 116]
  +related: [113, 115, 116, 131]
  ```

  Post-check:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep -c 'adr-token-backpointer'
  ```

  Expected: `0`.

- [ ] **Task 1.4: Verify and commit.** Run `./x sync` first: a relation token regenerates
  `.awf/awf.lock` and `docs/decisions/ACTIVE.md`. A `related:`-only edit regenerates nothing, and
  `docs/domains/*.md` moves only when a target gains a **new** retiring ADR, which no Phase 1
  verdict does. Then `./x gate`, and stage the exact paths.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/ .awf/awf.lock
  ```

  Expected: `awf check: clean` except for the `adr-unencoded-claim` findings on the 12 remaining
  slug anchors, and the three pre-existing advisory notes for slugs a still-`Proposed` ADR-0131
  declares (`cites-token-uncounted`, `cites-token-unrendered`, `residue-exemptions-pinned-three`).
  Notes never fail the gate.

  ```commit
  docs(adr): encode the item-anchored citation claims
  ```

## Phase 2: slug retirements, redeclarations, and proof retargets

Every carrier here is `Implemented`, so each `supersedes-invariant:` token retires its slug the
instant it lands. The token, the successor declaration, and the marker retarget must therefore share
a commit. This coupling is hygiene rather than a gate requirement: a stranded marker is only an
advisory note and `awf invariants` still reports clean, so the gate will not catch a split. That is
precisely why the plan states it.

**The rule for every redeclaration in this phase: the successor pins what its retargeted marker
asserts, never what the retired sentence claimed.** ADR-0131 dropped three clauses on exactly this
ground during its own review. Read the marker before writing the declaration.

- [ ] **Task 2.1: Insert the 9 `cites-invariant:` tokens (batch).** The slug-anchored findings that
  are not retirements take the inert key: the carrier names the slug without falsifying its declared
  sentence.

  **Representative site**, ADR-0016 item 7 on ADR-0015's `provenance-banner`, the governing
  precedent (the declaration names no path, so relocating the config root leaves it literally true):

  ```diff
  -   narrows **ADR-0015 `inv: provenance-banner`** only insofar as the banner text now names
  -   `.awf/` rather than `.claude/awf/`.
  +   narrows **ADR-0015 `inv: provenance-banner`** (`cites-invariant: ADR-0015#provenance-banner`)
  +   only insofar as the banner text now names `.awf/` rather than `.claude/awf/`.
  ```

  The shape is identical at every site; there is no edge case, because each is a bare mention
  adjacent to an `inv:` or `invariant:` citation.

  **Affected-site set**, the slug-anchored findings minus the three retirements in Task 2.2:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -vE '#[0-9]+ ' \
    | grep -vE 'sync-generates-active-md|render-active-md|uninstall-removes-lock-tracked'
  ```

  **Post-check**, only the three retirement claims remain:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-unencoded-claim' | grep -cv '#[0-9]* '
  ```

  Expected: `3`.

- [ ] **Task 2.2: Retire `ADR-0005#sync-generates-active-md` with its successor and proofs.** One
  commit. ADR-0020 Decision 6 made `ACTIVE.md` always render, falsifying the declaration's "writes
  no `ACTIVE.md` and prunes any previously locked one" clause.

  Three edits:

  1. `docs/decisions/0020-dead-reference-check.md` item 6 gains the token beside the prose that
     already names the falsified clause:

     ```diff
     -   - **ADR-0005 `inv: sync-generates-active-md`**: its "an absent or ADR-less decisions dir ...
     +   - **ADR-0005 `inv: sync-generates-active-md`**
     +     (`supersedes-invariant: ADR-0005#sync-generates-active-md`): its "an absent or ADR-less
     +     decisions dir ...
     ```

  2. The successor is already declared in ADR-0131's Invariants section as
     `sync-always-writes-active-md`; no ADR edit is needed for it.

  3. Both markers retarget. `internal/project/project_test.go:785` and `:833`:

     ```diff
     -// invariant: sync-generates-active-md
     +// invariant: sync-always-writes-active-md
     ```

     Leave the second marker on `:785` (`// invariant: check-active-md-stale`) untouched.

  ```
  ./x sync && ./x gate
  git add docs/decisions/0020-dead-reference-check.md internal/project/project_test.go docs/decisions/ACTIVE.md .awf/awf.lock
  ```

  Expected: gate green. `awf invariants` reports no unbacked slug: the retirement drops
  `sync-generates-active-md` from owed backing in the same commit the markers stop naming it.

  ```commit
  docs(adr): retire ADR-0005's stale active-md invariant
  ```

- [ ] **Task 2.3: Retire `ADR-0006#render-active-md` with its successor and proofs.** One commit.
  Same carrier item, same cause: the declaration's "returns `\"\"` when the directory holds no ADRs"
  clause is false.

  Two edits: the token on `docs/decisions/0020-dead-reference-check.md` item 6 beside its
  `ADR-0006 inv: render-active-md` prose, in the shape shown in Task 2.2; and all three markers in
  `internal/adr/adr_test.go` (`:18`, `:104`, `:193`) retargeted to
  `render-active-md-grouped-or-placeholder`.

  ```
  ./x sync && ./x gate
  git add docs/decisions/0020-dead-reference-check.md internal/adr/adr_test.go docs/decisions/ACTIVE.md .awf/awf.lock
  ```

  ```commit
  docs(adr): retire ADR-0006's stale render-active-md invariant
  ```

- [ ] **Task 2.4: Retire `ADR-0023#uninstall-removes-lock-tracked` with its successor and proof.**
  One commit. ADR-0032 Decision 7 dropped the declaration's middle conjunct, so the sentence is
  false as written.

  Two edits: the token on `docs/decisions/0032-remove-automatic-hook-handling.md` item 7 beside the
  prose naming the dropped clause; and `internal/project/install_test.go:41` retargeted to
  `uninstall-removes-lock-entries`.

  Do **not** widen that test to match the retired sentence's other clauses. ADR-0131 deliberately
  narrowed the successor to what the marker asserts, and re-establishing the lock-file-removal or
  authored-config-survival clauses is a successor ADR's job with the assertion that earns it.

  ```
  ./x sync && ./x gate
  git add docs/decisions/0032-remove-automatic-hook-handling.md internal/project/install_test.go docs/decisions/ACTIVE.md .awf/awf.lock
  ```

  ```commit
  docs(adr): retire ADR-0023's stale uninstall invariant
  ```

- [ ] **Task 2.5: Add the back-pointer edges Phase 2 owes and commit.** ADR-0005, ADR-0006, and
  ADR-0023 gain `131` in `related:`, per ADR-0131 item 1. Derive any others from the check rather
  than assuming:

  ```
  /tmp/awf-planD-bin check 2>&1 | grep 'adr-token-backpointer'
  ```

  Expected after the edits: no output.

  ```
  ./x sync && ./x check && ./x gate
  git add docs/decisions/ .awf/awf.lock
  ```

  ```commit
  docs(adr): add the retirement back-pointer edges
  ```

## Verification

The whole-effort acceptance check is that the citation check reports nothing on this corpus:

```
/tmp/awf-planD-bin check 2>&1 | grep -c 'adr-unencoded-claim'
```

Expected: `0`. This is the precondition Plan C Task 3.6 assumed and did not get.

Then confirm no ADR's status drifted. No verdict in this plan retires an item anchor, and the three
slug retirements each move a different target by one anchor, so no ADR reaches full coverage:

```
./x check
```

Expected: `awf check: clean`, with only the three advisory notes for slugs ADR-0131 declares while
still `Proposed`.

Finally, confirm the retirements cost no enforcement. Every retired slug's proof survives under its
successor, so the test count is unchanged and no marker is stranded:

```
./x gate
```

Expected: green at 100% coverage, and `awf invariants: clean`.

## Notes

- **This plan does not flip ADR-0131.** Plan C Task 3.6 owns the flip, and it must re-run `./x gate`
  afterwards: ADR-0131's own claims contribute zero coverage while it is `Proposed`
  (`internal/adr/coverage.go:166` excludes it), so they arm only at the flip. This plan's green does
  not carry across that boundary.
- **Phase 0's scoping change belongs to Plan C Phase 3's commit**, not to this plan. The dead-code
  gate (ADR-0063, no ignore directive) forbids landing the extraction or the check without its
  caller, so the code lands once, wired, in Plan C. This plan consumes it as working state.
- **The `cites:` token has no changelog entry.** `grep -n cites changelog/CHANGELOG.md` returns
  nothing, yet the parser change landed in `b1be6de4` after `v0.16.0` and is adopter-facing. Plan C
  Task 3.4's doc sweep owes an `[Unreleased]` entry for the whole citation-check feature; it is
  recorded here because `repoaudit` cannot see it from a later commit range.
- **Deferred, unchanged from Plan C:** `refines: ADR-0034#1` at `docs/decisions/0057-*.md:86` parses
  with `CarrierItem: 0` and has no rationale site (ADR-0129 Decision 2). Fixing it needs its own
  decision, because moving the token is a content edit the append-only rule forbids.
- **A marker re-point is owed at the flip.** `TestCheckTokenRetirementIgnoresCitesInvariant`
  (`internal/invariants/invariants_test.go`) carries `// invariant: token-retirement-implemented-only`,
  a status slug, while the test exercises relation. The exactly-fitting slug is ADR-0131's
  `cites-token-uncounted`, unusable while ADR-0131 is `Proposed`. Re-point it in Plan C Task 3.6.
  Do not downgrade it to `touches-invariant:` in the meantime; that would leave the test unmarked.
- **Verdict provenance.** All 44 claims were adjudicated by fresh-context agents, one per target
  ADR, each returning the target's surviving-or-dead clause quoted with a file:line. The rule was
  read the target's clause set, never the carrier's verb, which had overturned five verdicts earlier
  in this effort. Any verdict this plan encodes can be re-derived from the cited target's text; if a
  site looks wrong during execution, re-read the target rather than the carrier.
