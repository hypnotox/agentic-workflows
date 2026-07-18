---
date: 2026-07-18
adrs: [131]
status: Proposed
---
# Plan: Supersession Retrofit A: Relation Corrections and Slug Retirements

## Goal

Land the semantic half of ADR-0131 item 9's retrofit: correct the 6 inline relation tokens that
assert a refinement where the carrier's prose asserts a retirement, and insert the 3
`supersedes-invariant:` tokens that retire ADR-0009's `config-root` and `parts-convention` and
ADR-0082's `residue-exemptions-pinned`, each committed together with the proof edit it authorises.

Non-goals: the 15 prose-only token backfills and the remaining `related:` back-pointers (Plan B),
and building or enabling the citation check itself (Plan C).

## Architecture summary

Two kinds of edit, in two phases, because they fail differently.

**Corrections** rewrite an existing token's relation key in place. Each moves an anchor from
uncontested to retired, feeding ADR-0129's coverage model, and coverage completion forces a target's
status to `Superseded` (`internal/project/supersession.go:194-216`). ADR-0131 measured the headroom
and found none of the 6 completes a coverage set, so no status flip is expected. `./x check` is the
oracle: an `adr-coverage-status` finding means the measurement was wrong and the plan stops.

**Retirements** insert a new token beside prose that already states the claim (ADR-0120 Decision 9
shape 3). Their carriers (ADR-0015, ADR-0016, ADR-0085) are all `Implemented`, so
`internal/invariants/invariants.go:155-165` retires the slug the moment the token lands. The proof
edit must land in the same commit: a token without its proof edit strands a marker on a retired
slug, and a proof edit without its token leaves a live slug unbacked. Either half alone reds
`./x gate`, which is why Phase 2 is not sliced per file.

ADR-0131 stays `Proposed` throughout; its status flip belongs to Plan C.

## File structure

- **Created:** none.
- **Modified:** `docs/decisions/0015-in-file-provenance-and-convention-only-overrides.md`,
  `docs/decisions/0016-tool-agnostic-target-seam-and-awf-relocation.md`,
  `docs/decisions/0082-source-level-template-residue-guard.md`,
  `docs/decisions/0085-self-contained-adopter-upgrade-flow.md`,
  `docs/decisions/0093-rename-config-toggle-commands-to-enable-disable.md`,
  `docs/decisions/0105-enforced-test-backing-and-the-proof-touches-invariant-marker-split.md`,
  `docs/decisions/0119-repo-wide-plain-punctuation-the-remaining-surfaces-and-an-opt-in-prose-gate.md`,
  `docs/decisions/0120-structured-machine-checked-adr-supersession.md`,
  `docs/decisions/0123-pi-workflow-subagent-extension.md`,
  `docs/decisions/ACTIVE.md` (regenerated), `internal/config/config_test.go`,
  `internal/project/residue_scan_test.go`, `.awf/awf.lock` (regenerated).
- **Deleted:** none.

## Phase 1: Relation corrections

The six sites are distinct judgements (different carriers, targets, and surviving-clause analyses),
so they are exact-diff tasks rather than a batch. They share one commit because they share one
rationale: ADR-0128's migration downgraded every pre-existing token, and ADR-0131 item 9 corrects
the six it got wrong.

Line numbers are as of commit `fecbaf01`. If a line has moved, locate the token by its exact text;
no two sites share a token string.

- [ ] **Task 1.1: Correct ADR-0015's token into ADR-0001 item 2.** In
  `docs/decisions/0015-in-file-provenance-and-convention-only-overrides.md:93`, replace:

  ```
     2** (`refines: ADR-0001#2`; the `replaceWith` overlay step, per Decision item 4). It also overrides **ADR-0009 Decision
  ```

  with:

  ```
     2** (`supersedes: ADR-0001#2`; the `replaceWith` overlay step, per Decision item 4). It also overrides **ADR-0009 Decision
  ```

  ADR-0001 item 2 is entirely the `replaceWith` overlay step, and ADR-0015 item 4 removes the
  `replaceWith` field, so nothing of the target item survives.

- [ ] **Task 1.2: Correct ADR-0093's token into ADR-0024 item 6.** In
  `docs/decisions/0093-rename-config-toggle-commands-to-enable-disable.md:47`, replace:

  ```
     `refines: ADR-0024#1`, `refines: ADR-0024#6`):** the
  ```

  with:

  ```
     `refines: ADR-0024#1`, `supersedes: ADR-0024#6`):** the
  ```

  The `ADR-0024#1` token on the same line stays `refines:` deliberately. ADR-0024 item 1's
  required-kind rule and its `skill`-to-`skills` mapping both survive the rename; only item 6's
  help/README/guide grammar is replaced wholesale. Changing both is the most likely error here, and
  ADR-0093 item 2's own "Supersede ADR-0024 Decision items 1 and 6" heading invites it.

- [ ] **Task 1.3: Correct ADR-0105's token into ADR-0008 item 4.** In
  `docs/decisions/0105-enforced-test-backing-and-the-proof-touches-invariant-marker-split.md:109`,
  replace:

  ```
     (`refines: ADR-0008#4`) via
  ```

  with:

  ```
     (`supersedes: ADR-0008#4`) via
  ```

  Both clauses of ADR-0008 item 4 die: "it remains prose, outside the enforced roster" is reversed
  by `unbacked-invariant:`, and "there is no per-slug exemption mechanism" is exactly what that
  token creates.

- [ ] **Task 1.4: Correct ADR-0119's token into ADR-0115 item 4.** In
  `docs/decisions/0119-repo-wide-plain-punctuation-the-remaining-surfaces-and-an-opt-in-prose-gate.md:170`,
  replace:

  ```
     item 4** (`refines: ADR-0115#4`), whose two reasons are both answered: the gofmt reason is refuted above, and "test
  ```

  with:

  ```
     item 4** (`supersedes: ADR-0115#4`), whose two reasons are both answered: the gofmt reason is refuted above, and "test
  ```

  ADR-0115 item 4 is a scope exclusion plus its two supporting reasons; ADR-0119 item 4 answers both
  and puts Go comments and `_test.go` in scope.

  Leave `refines: ADR-0115#7` at line 181 unchanged. ADR-0131 records it as ambiguous: the carrier
  claims displacement "on all three of its clauses" but then preserves a live scope for the target
  ("it remains correct for ADR-0115's own gate"). The weaker relation is correct when the reading is
  contested.

- [ ] **Task 1.5: Correct ADR-0120's token into ADR-0116 item 5.** In
  `docs/decisions/0120-structured-machine-checked-adr-supersession.md:89`, replace:

  ```
     successor's number. This `refines: ADR-0116#5` reverses ADR-0116's choice to keep
  ```

  with:

  ```
     successor's number. This `supersedes: ADR-0116#5` reverses ADR-0116's choice to keep
  ```

  ADR-0116 item 5 is "No mechanical check, for now. This ADR adds no gate, no `awf check` note, and
  no `repoaudit` rule"; ADR-0120 item 4 makes it an `awf check` failure.

- [ ] **Task 1.6: Correct ADR-0123's token into ADR-0122 item 4.** In
  `docs/decisions/0123-pi-workflow-subagent-extension.md:54`, replace:

  ```
     ADR-0122. `refines: ADR-0122#4`
  ```

  with:

  ```
     ADR-0122. `supersedes: ADR-0122#4`
  ```

  ADR-0122 item 4 defers a Pi subagent orchestrator and ships no Pi extension; ADR-0123 ships it.
  The remaining clause is a permission ADR-0123 exercises, not a constraint that survives.

- [ ] **Task 1.7: Verify no status flip was forced, then commit.** Run `./x sync` (regenerates
  `docs/decisions/ACTIVE.md` and `.awf/awf.lock`), then `./x check`. Expect `awf check: clean` and
  `awf invariants: clean`.

  An `adr-coverage-status` finding means a correction completed some ADR's anchor coverage, which
  ADR-0131's headroom measurement said would not happen. Do **not** flip a status to silence it:
  stop, and reopen the measurement in ADR-0131 (still `Proposed`, so amendable).

  Then run `./x gate`, `git add` the six ADR files plus `docs/decisions/ACTIVE.md` and
  `.awf/awf.lock`, and commit:

  ```commit
  docs(adr): correct six downgraded relation tokens
  ```

## Phase 2: Slug retirements with their proof edits

Each task inserts one `supersedes-invariant:` token and makes the proof edit that token authorises.
Token and proof edit are inseparable, per the Architecture summary.

- [ ] **Task 2.1: Retire `parts-convention` on ADR-0015.** In
  `docs/decisions/0015-in-file-provenance-and-convention-only-overrides.md:94`, replace:

  ```
     item 4** (`refines: ADR-0009#4`) and its `inv: parts-convention`, collapsing that ADR's four-tier precedence
  ```

  with:

  ```
     item 4** (`refines: ADR-0009#4`) and its `inv: parts-convention` (`supersedes-invariant: ADR-0009#parts-convention`), collapsing that ADR's four-tier precedence
  ```

  The `refines: ADR-0009#4` token on the same line is unchanged: ADR-0009 item 4 is narrowed while
  the slug is retired. Different anchors, different relations, one line.

  `parts-convention` has no proof marker to move (it is currently owed and unbacked), so this task
  makes no test edit. ADR-0009's `related:` already names 15, so no back-pointer is owed.

- [ ] **Task 2.2: Retire `config-root` on ADR-0016 and drop its duplicated proof marker.** In
  `docs/decisions/0016-tool-agnostic-target-seam-and-awf-relocation.md:150`, replace:

  ```
     `inv: config-root` (config now loads from `.awf/config.yaml`, lock at `.awf/awf.lock`), and
  ```

  with:

  ```
     `inv: config-root` (`supersedes-invariant: ADR-0009#config-root`; config now loads from `.awf/config.yaml`, lock at `.awf/awf.lock`), and
  ```

  Then in `internal/config/config_test.go`, delete line 119 exactly:

  ```
  // invariant: config-root
  ```

  leaving the following line (`// invariant: awf-config-root`) in place, so the test backs the live
  successor alone.

  Leave the `narrows **ADR-0015 \`inv: provenance-banner\`**` clause at line 151 untokenized.
  ADR-0131 examined and rejected it: that slug declares only that every rendered file carries the
  banner as its first line, and names no path, so the relocation leaves it true and owing nothing.

  ADR-0009's `related:` already names 16, so no back-pointer is owed.

- [ ] **Task 2.3: Retire `residue-exemptions-pinned` on ADR-0085 and retarget its proof.** Three
  edits, one commit-atomic unit.

  First, add the missing back-pointer. In
  `docs/decisions/0082-source-level-template-residue-guard.md:5`, replace:

  ```
  related: [1, 45, 80, 131]
  ```

  with:

  ```
  related: [1, 45, 80, 85, 131]
  ```

  This edge is owed by *this* task's token (which targets ADR-0082 from ADR-0085), not by Plan B:
  without it `./x check` reports `adr-token-backpointer` in this commit. Plan B's ADR-0082
  back-pointer task is therefore already satisfied and must be struck when Plan B is executed.

  Second, in `docs/decisions/0085-self-contained-adopter-upgrade-flow.md:104`, replace:

  ```
     each still failing when stale. `inv: residue-exemptions-pinned` is reworded accordingly
  ```

  with:

  ```
     each still failing when stale. `inv: residue-exemptions-pinned` is retired and redeclared by ADR-0131 (`supersedes-invariant: ADR-0082#residue-exemptions-pinned`)
  ```

  This is the one insertion that also corrects the prose beside it, because "is reworded
  accordingly" describes an edit the append-only rule cannot perform and which never happened
  (ADR-0082's declaration still reads "exactly two"). It is ADR-0120 Decision 9 shape 3 plus the
  minimum wording change that keeps the sentence true. If the plan reviewer judges that beyond the
  carve-out, fall back to inserting the token alone, leave the false clause standing, and record the
  residue in Notes.

  Third, in `internal/project/residue_scan_test.go`, make three edits:
  1. Line 28: retarget `// invariant: residue-exemptions-pinned` to
     `// invariant: residue-exemptions-pinned-three`.
  2. Lines 26-27: the `identityExempt` doc comment reads "(ADR-0082 Decision 2, extended to three
     entries by ADR-0085 Decision 5)". Add ADR-0131 as the ADR that now declares the pin.
  3. Line 47: the guard's failure message ends "(ADR-0082, last extended by ADR-0085)". Add
     ADR-0131 for the same reason.

- [ ] **Task 2.4: Verify both slugs left the owed roster, then commit.** Run `./x sync`, then
  `./x check`. Expect `awf check: clean` and `awf invariants: clean`. Specifically confirm:
  - no dangling-marker note for `residue-exemptions-pinned` (retired, and its marker now names the
    successor);
  - no unbacked finding for `residue-exemptions-pinned-three` (declared by ADR-0131, which is still
    `Proposed`, so not yet owed backing; the marker is in place early so Plan C's flip need not
    touch this test again);
  - no unbacked finding for `config-root` or `parts-convention` (both now retired).

  Then run `./x gate`, `git add` the four ADR files, `internal/config/config_test.go`,
  `internal/project/residue_scan_test.go`, `docs/decisions/ACTIVE.md`, and `.awf/awf.lock`, and
  commit:

  ```commit
  docs(adr): retire three stale invariant slugs
  ```

## Verification

After both phases:

- `./x gate` passes and `./x check` reports `awf check: clean` and `awf invariants: clean`.
- `git log --oneline -2` shows exactly the two commits above.
- The six corrections landed:
  `grep -ho 'supersedes: ADR-0001#2\|supersedes: ADR-0024#6\|supersedes: ADR-0008#4\|supersedes: ADR-0115#4\|supersedes: ADR-0116#5\|supersedes: ADR-0122#4' docs/decisions/*.md | sort -u | wc -l`
  returns `6`.
- The three retirements landed:
  `grep -ho 'supersedes-invariant: ADR-0009#parts-convention\|supersedes-invariant: ADR-0009#config-root\|supersedes-invariant: ADR-0082#residue-exemptions-pinned' docs/decisions/*.md | sort -u | wc -l`
  returns `3`.
- The stale marker is gone: `grep -c 'invariant: config-root' internal/config/config_test.go`
  returns `1` (the surviving `awf-config-root` line matches this substring; confirm by eye that the
  bare `config-root` marker is absent).
- The ambiguous tokens were not touched:
  `grep -c 'refines: ADR-0115#7' docs/decisions/0119-*.md` returns `1`, and
  `grep -c 'refines: ADR-0002#5' docs/decisions/0101-*.md` returns `1`.

## Notes

- **ADR-0131 stays `Proposed`.** Its status flip, and the backing proofs for its declared
  citation-check invariants, belong to Plan C's final commit.
- **`residue-exemptions-pinned-three` is declared but not yet owed** until ADR-0131 is
  `Implemented`. Its proof marker lands here anyway.
- **Carried forward from ADR-0131's review, not actioned here:** the marker in
  `internal/project/residue_scan_test.go` sits on the `identityExempt` var, while the
  `len(identityExempt) != 3` assertion that actually proves the pin lives inside
  `TestTemplateSourceResidue` under `template-source-residue`'s marker. File-glob backing passes
  either way. Moving it onto the assertion is a small improvement for Plan C, if it can be done
  without disturbing the sibling slug's backing.
- **Deferred, out of scope by decision:** `refines: ADR-0034#1` at `docs/decisions/0057-*.md:86`
  sits in a trailing paragraph after Decision item 6, so it parses with `CarrierItem: 0` and has no
  rationale site, contra ADR-0129 Decision 2. Fixing it means either moving the token (a content
  edit the append-only rule forbids) or appending a bookkeeping item under ADR-0120 Decision 9
  shape 2. It needs its own decision; record it in the roadmap.
- **Expected non-findings:** ADR-0009 moves from zero to two covered anchors of fifteen, ADR-0082 to
  one of six. Neither approaches full coverage, so neither status flips.
