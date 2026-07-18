---
date: 2026-07-18
adrs: [131]
status: Proposed
---
# Plan: Supersession Retrofit B: Token Backfill and Back-Pointers

## Goal

Land the mechanical half of ADR-0131 item 9's retrofit: insert the 17 relation tokens whose claims
are stated in prose and never encoded (16 citing a Decision item, 1 citing an invariant slug), and
add the 6 missing `related:` back-pointers those tokens require.

Non-goals: the relation corrections and slug retirements (Plan A, which must land first), and
building or enabling the citation check (Plan C).

## Architecture summary

Every token here is authorised by ADR-0120 Decision 9 shape 3: inserted beside an existing prose
citation that already states the claim, on a frozen ADR, changing no meaning. None of these is a
judgement about *whether* an override happened; the carrier already said so. The only judgement is
`supersedes:` versus `refines:`, decided per site by whether any clause of the target item survives.

Three tokens land on ADR-0015 Decision item **4**, whose citations of `ADR-0001#2`, `ADR-0009#4` and
`inv: parts-convention` duplicate claims Plan A tokenizes in ADR-0015 Decision item **6**. This is
not redundant: ADR-0131 Decision 2 scopes the check per Decision item, because ADR-0129 Decision 2
requires each claim to sit at its own rationale site. A token in item 6 does not satisfy item 4.

Back-pointers are a genuine batch: one identical edit shape across six frontmatter arrays. They
land in the **same phase and commit** as the tokens, not a later one. Plan review established this
empirically: a token whose target's `related:` does not name the carrier produces an
`adr-token-backpointer` drift and `./x check` exits 1, and `./x sync` does not repair it. Six of
this plan's tokens are in that position, so deferring the edges to a second phase would leave the
first phase unable to pass its own gate. One phase, one commit.

**Sequencing:** Plan A must be fully landed before this plan starts. This plan assumes ADR-0082's
`related:` already names 85 (Plan A Task 2.3 adds it), which is why ADR-0082 is absent from the
back-pointer set below.

## File structure

- **Created:** none.
- **Modified:** `docs/decisions/0004-*.md`, `0015-*.md`, `0022-*.md`, `0024-*.md`, `0026-*.md`,
  `0028-*.md`, `0040-*.md`, `0043-*.md`, `0045-*.md`, `0047-*.md`, `0049-*.md`, `0069-*.md`,
  `0075-*.md`, `0079-*.md`, `0081-*.md`, `0085-*.md`, `0087-*.md`, `0094-*.md` under
  `docs/decisions/`, plus `docs/decisions/ACTIVE.md` (regenerated) and `.awf/awf.lock`
  (regenerated). ADR-0013 is a token *target* only; its `related:` already names 81, so no file
  edit is owed there.
- **Deleted:** none.

## Phase 1: Token backfill and back-pointers

Line numbers are as of commit `fecbaf01`, **before** Plan A's edits. Plan A touches ADR-0015 (line
93-94), ADR-0085 (line 104) and ADR-0082 (line 5) only, so the sites below shift by at most a line
in those three files. Locate each site by its quoted text, not by number.

- [ ] **Task 1.1: Tokenize ADR-0015 Decision item 4's three citations.** In
  `docs/decisions/0015-in-file-provenance-and-convention-only-overrides.md`, replace lines 79-80:

  ```
     **ADR-0001 Decision item 2** (the `replaceWith` overlay step) and the precedence documented by
     **ADR-0009** (its Decision item 4 and `inv: parts-convention`).
  ```

  with:

  ```
     **ADR-0001 Decision item 2** (`supersedes: ADR-0001#2`; the `replaceWith` overlay step) and the precedence documented by
     **ADR-0009** (its Decision item 4, `refines: ADR-0009#4`, and `inv: parts-convention`, `supersedes-invariant: ADR-0009#parts-convention`).
  ```

  All three anchors are also claimed from item 6 by Plan A. That is intended, per the Architecture
  summary. `ADR-0001#2` takes `supersedes:` to match Plan A Task 1.1; `ADR-0009#4` takes `refines:`
  because ADR-0009 item 4 is narrowed, not retired.

  ADR-0001's and ADR-0009's `related:` already name 15, so no back-pointer is owed.

- [ ] **Task 1.2: Tokenize ADR-0026's refinement of ADR-0024 item 3.** In
  `docs/decisions/0026-config-serialization-ownership.md`, replace lines 42-43:

  ```
     with `SetIndent(2)`, so the on-disk format has exactly one definition. This **reverses ADR-0024
     Decision item 3** (the generic string-surgery array editor) and overturns ADR-0024's rejected
  ```

  with:

  ```
     with `SetIndent(2)`, so the on-disk format has exactly one definition. This **reverses ADR-0024
     Decision item 3** (`refines: ADR-0024#3`; the generic string-surgery array editor) and overturns ADR-0024's rejected
  ```

  `refines:`, despite the carrier's "reverses". Plan review found a surviving clause: item 3 closes
  with "Both commands re-render via the normal sync, so `remove` drops the now-unproduced rendered
  file through the existing Sync prune", which concerns the sync flow rather than the serialization
  mechanism and which ADR-0026 leaves untouched. ADR-0026's "ADR-0024 otherwise stands" enumerates
  other items and does not settle item 3's internal clause set.

- [ ] **Task 1.3: Tokenize ADR-0028's override of ADR-0004 item 1.** In
  `docs/decisions/0028-workflow-chain-adr-first-visible-resync.md`, replace line 81:

  ```
     ADR-0004 Decision item 1 ("presents reviews as lightweight"). The guide prose describes the real
  ```

  with:

  ```
     ADR-0004 Decision item 1 (`refines: ADR-0004#1`; "presents reviews as lightweight"). The guide prose describes the real
  ```

  `refines:` because only the hide-it clause is overridden.

- [ ] **Task 1.4: Tokenize ADR-0043's partial reversal of ADR-0022 item 4.** In
  `docs/decisions/0043-mandatory-singleton-status-for-workflow-and-documentation-standards.md`,
  replace line 147:

  ```
  13. **This ADR partially reverses ADR-0022's Decision item 4** for exactly these three docs.
  ```

  with:

  ```
  13. **This ADR partially reverses ADR-0022's Decision item 4** (`refines: ADR-0022#4`) for exactly these three docs.
  ```

  `refines:` because the carrier says "partially" and "for exactly these three docs"; ADR-0022 keeps
  governing every other doc.

- [ ] **Task 1.5: Tokenize ADR-0049's refinement of ADR-0030 item 4.** In
  `docs/decisions/0049-single-version-authority.md`, replace lines 56-57:

  ```
     parenthetical true as written. Amends ADR-0030 Decision 4 (the precedence chain is retired
     with its invariant, and ADR-0030's textual contract that `.goreleaser.yaml` injects
  ```

  with:

  ```
     parenthetical true as written. Amends ADR-0030 Decision 4 (`refines: ADR-0030#4`; the precedence chain is retired
     with its invariant, and ADR-0030's textual contract that `.goreleaser.yaml` injects
  ```

  `refines:`, despite the carrier's "is retired with its invariant". Plan review found item 4's
  third clause survives intact: "`project.Version` remains the source of truth for the lock's
  `AWFVersion` and the dev/test fallback, and is bumped per release." ADR-0049 does not retire that
  clause, it promotes it to single-version authority. ADR-0030's `version-ldflags-precedence` slug
  is separately retired by an existing token, so the invariant half of the claim is already
  encoded.

  The adjacent citation of ADR-0039 Decision 3 on line 55 is informational ("making ADR-0039
  Decision 3's parenthetical true as written") and takes no relation token. Under ADR-0131 Decision
  4 it takes `cites: ADR-0039#3` instead, which Plan C adds along with the rest of the corpus's
  informational citations; leave it alone here.

- [ ] **Task 1.6: Tokenize ADR-0075's refinement of ADR-0069 item 5.** In
  `docs/decisions/0075-working-memory-check-is-on-demand-not-a-startup-step.md`, replace lines
  64-65:

  ```
  4. **Supersedence scope.** This ADR refines, and does not replace, **ADR-0069 Decision
     item 5**: it overrides only that item's "at session start check" clause, leaving the
  ```

  with:

  ```
  4. **Supersedence scope.** This ADR refines, and does not replace, **ADR-0069 Decision
     item 5** (`refines: ADR-0069#5`): it overrides only that item's "at session start check" clause, leaving the
  ```

  `refines:` is stated outright by the carrier.

- [ ] **Task 1.7: Tokenize ADR-0079's partial amendment of ADR-0065 item 3.** In
  `docs/decisions/0079-release-and-ci-supply-chain-hygiene.md`, replace line 87:

  ```
  4. **Codecov uploads are token-optional.** *(Partially amends ADR-0065 Decision 3,
  ```

  with:

  ```
  4. **Codecov uploads are token-optional.** *(Partially amends ADR-0065 Decision 3, `refines: ADR-0065#3`,
  ```

  `refines:` because the `fail_ci_if_error: true` clause survives when the token is present.

- [ ] **Task 1.8: Tokenize ADR-0081's three generalizations of ADR-0050 and its refinement of
  ADR-0013 item 4.** In `docs/decisions/0081-enforced-dependency-graph-over-catalog-requires-declarations.md`,
  four edits.

  Line 77 (the citation wraps from line 76, so the token goes on the line carrying the anchor
  phrase, exactly as the line 102-103 edit below does):

  ```
     Decision item 2 (its `RequiresAgent` check becomes the skill→agent edge of
  ```

  becomes:

  ```
     Decision item 2 (`refines: ADR-0050#2`; its `RequiresAgent` check becomes the skill→agent edge of
  ```

  Leave line 76 untouched. Appending the token there would produce "This generalizes ADR-0050
  (`refines: ADR-0050#2`) Decision item 2 (...)", splitting the anchor phrase from its token.

  Line 94:

  ```
     rewrite, printing the plan. This generalizes ADR-0050 Decision item 5 and
  ```

  becomes:

  ```
     rewrite, printing the plan. This generalizes ADR-0050 Decision item 5 (`refines: ADR-0050#5`) and
  ```

  Lines 102-103:

  ```
     applies the whole plan in one rewrite. This generalizes ADR-0050 Decision
     item 4 (the agent guard becomes the reverse walk's length-1 case) and
  ```

  becomes:

  ```
     applies the whole plan in one rewrite. This generalizes ADR-0050 Decision
     item 4 (`refines: ADR-0050#4`; the agent guard becomes the reverse walk's length-1 case) and
  ```

  Lines 119-120:

  ```
     (Decision 3), so the ADR-0013 Decision item 4 suppression semantics are
     superseded: the render-time gate (`skillDocGateOpen` and the suppression
  ```

  becomes:

  ```
     (Decision 3), so the ADR-0013 Decision item 4 suppression semantics are
     superseded (`refines: ADR-0013#4`): the render-time gate (`skillDocGateOpen` and the suppression
  ```

  All four claims are `refines:`. The three ADR-0050 items each survive in specialised form. The
  ADR-0013 claim looked like a retirement, but the carrier scopes it itself: "the ADR-0013 Decision
  item 4 *suppression semantics* are superseded". Two other clauses of item 4 survive ADR-0081 item
  7: the `requiresDoc` catalog-field declaration, promoted to `RequiresDoc` in the hard graph rather
  than deleted, and the rule that a doc-gated skill may reference `.layout.docs.<doc>` unguarded yet
  safely, still true because a refused config state still guarantees the doc is enabled. Same shape
  as Task 1.11, same relation.

  The citation of ADR-0050 Decision 3 at line 111 is informational ("unchanged") and takes no
  relation token here; Plan C gives it `cites:`.

- [ ] **Task 1.9: Tokenize ADR-0085's amendment of ADR-0082 item 2.** In
  `docs/decisions/0085-self-contained-adopter-upgrade-flow.md`, replace line 101:

  ```
  5. **ADR-0082's identity-exemption list gains a third entry.** Per ADR-0082 Decision 2's own
  ```

  with:

  ```
  5. **ADR-0082's identity-exemption list gains a third entry** (`refines: ADR-0082#2`). Per ADR-0082 Decision 2's own
  ```

  `refines:` because ADR-0082 item 2 survives with a pinned three-entry list rather than being
  replaced. This is the same Decision item Plan A Task 2.3 edits (line 104); apply Plan A first and
  re-locate by text.

  ADR-0082's `related:` names 85 after Plan A Task 2.3, so no back-pointer is owed here.

- [ ] **Task 1.10: Tokenize ADR-0087's narrowing of ADR-0045 item 4.** In
  `docs/decisions/0087-deletion-as-acknowledgement-for-unset-var-notes.md`, replace line 75:

  ```
     note trigger of ADR-0045 Decision 4 (partial-item supersedence recorded via `related`;
  ```

  with:

  ```
     note trigger of ADR-0045 Decision 4 (`refines: ADR-0045#4`; partial-item supersedence recorded via `related`;
  ```

  `refines:` because only the note trigger is narrowed. Note the carrier claims the override is
  "recorded via `related`" while that back-pointer is absent; Task 1.13 adds it.

  The citation of ADR-0045 Decision 3 at line 76 is informational ("is untouched") and takes no
  relation token; Plan C gives it `cites:`.

- [ ] **Task 1.11: Tokenize ADR-0094's partial supersedence of ADR-0093 item 4.** In
  `docs/decisions/0094-command-table-cli-dispatch-importable-command-spec-and-generated-gated-command-list.md`,
  replace line 158:

  ```
     tests. This is a **partial-item supersedence of ADR-0093 Decision item 4**: the
  ```

  with:

  ```
     tests. This is a **partial-item supersedence of ADR-0093 Decision item 4** (`refines: ADR-0093#4`): the
  ```

  `refines:` because only the "resolver vocabulary stays" sub-decision falls.

  This site is why ADR-0131 Decision 2 enumerates verb surface forms rather than generating them:
  the only override word here is "supersedence", which a stem-plus-suffix rule does not produce.

- [ ] **Task 1.12: Tokenize ADR-0047's refinement of ADR-0040 item 1.** In
  `docs/decisions/0047-*.md`, replace lines 48-49:

  ```
  1. **Render the bootstrap at `.awf/bootstrap.sh`** (supersedes
     [ADR-0040](0040-self-pinning-rendered-bootstrap.md) Decision item 1; recorded via
  ```

  with:

  ```
  1. **Render the bootstrap at `.awf/bootstrap.sh`** (supersedes
     [ADR-0040](0040-self-pinning-rendered-bootstrap.md) Decision item 1, `refines: ADR-0040#1`; recorded via
  ```

  `refines:`, despite the carrier's "supersedes". ADR-0040 item 1 bundles three clause groups: the
  rendered path and filename, the script's behaviour (detect OS/arch, download the tarball and
  `checksums.txt`, verify the SHA-256, extract, install into a cache dir, print the path), and the
  URL convention (v-prefixed git tag in the path, no-v form in the asset filename). ADR-0047
  replaces only the first. Neither survivor is carried by any of ADR-0040's six sibling items, so
  both survive inside item 1 itself, and ADR-0047's own Context agrees: "no slug encodes the path,
  so nothing retires: only the backing test's path assertion moves."

  This site was missed by the original audit sweep and by all three plan reviews, and found while
  reading ADR-0040's frontmatter for an unrelated reason. ADR-0131's own Context names it, so the
  ADR knew about it and the enumeration did not. Its first draft here also asserted `supersedes:`
  on the strength of the carrier's verb without reading item 1's clause set, which is the exact
  failure mode this plan's closing Note describes for the three overturned verdicts. It was caught
  in the verify pass.

  The carrier claims the override is "recorded via `related`". It is not: ADR-0040's `related:` is
  `[24, 27, 30, 39, 85]`. Task 1.13 adds the edge.

- [ ] **Task 1.13: Add the six missing back-pointer edges.** Batch task: one identical edit shape
  across six frontmatter arrays. Each target ADR's `related:` gains the number of the carrier whose
  token points at it (ADR-0128 Decision 5 requires the edge on a target of any status).

  These land in **this** commit, not a later one. Six of this phase's tokens target an ADR whose
  `related:` does not yet name the carrier, and each such token is an `adr-token-backpointer` drift
  until its edge exists, so splitting them out would leave this phase unable to pass its own gate.

  Representative site, an append: `docs/decisions/0004-*.md:5`

  ```
  related: [1]
  ```

  becomes:

  ```
  related: [1, 28]
  ```

  Edge site, a **mid-array insert**: `docs/decisions/0024-*.md:5`. The arrays are ascending, so an
  append is wrong here.

  ```
  related: [9, 14, 22, 93]
  ```

  becomes:

  ```
  related: [9, 14, 22, 26, 93]
  ```

  The affected-site set is exactly these six:

  | Target ADR | add | position | required by carrier |
  |---|---|---|---|
  | `0004-*.md` | 28 | append | Task 1.3 |
  | `0022-*.md` | 43 | **mid-insert** (`[4, 11, 43, 86]`) | Task 1.4 |
  | `0024-*.md` | 26 | **mid-insert** (`[9, 14, 22, 26, 93]`) | Task 1.2 |
  | `0040-*.md` | 47 | **mid-insert** (`[24, 27, 30, 39, 47, 85]`) | Task 1.12 |
  | `0045-*.md` | 87 | append | Task 1.10 |
  | `0069-*.md` | 75 | append | Task 1.6 |

  Three of the six are mid-inserts. ADR-0082 is deliberately absent: Plan A Task 2.3 already added
  85 to it. If that edit is missing, Plan A did not fully land and this plan started too early.

  Post-check: `./x check 2>&1 | grep -c adr-token-backpointer` returns `0`. Note `grep -c` exits 1
  when the count is zero, so do not chain it with `&&`.

- [ ] **Task 1.14: Verify no status flip was forced, then commit.** Run `./x sync`, then `./x check`.
  Expect `awf check: clean` and `awf invariants: clean`.

  An `adr-token-backpointer` finding means a seventh edge exists that the audit missed. Add it here
  and note it; do not defer it, since this phase must gate clean.

  An `adr-coverage-status` finding means a token was written with the wrong relation; find it rather
  than flipping a status. Fifteen of this plan's 17 tokens are `refines:`, which counts toward no
  anchor's coverage (ADR-0128 Decision 4). The two exceptions are Task 1.1's
  `supersedes: ADR-0001#2` and `supersedes-invariant: ADR-0009#parts-convention`, and neither can
  move coverage either: both duplicate anchors ADR-0015 retires from its own item 6 in Plan A, and a
  claim set is per-ADR, so re-asserting an anchor the same carrier already retired adds nothing.

  Then run `./x gate` and stage the twelve carrier ADR files, the six target ADR files,
  `docs/decisions/ACTIVE.md`, `.awf/awf.lock`, and any `docs/domains/*.md` that `./x sync` rewrote
  (`git status --short docs/domains/` lists them; new annotations land there whenever a token is
  added). Commit:

  ```commit
  docs(adr): backfill unencoded relation tokens and back-pointers
  ```

## Verification

- `./x gate` passes and `./x check` reports `awf check: clean` and `awf invariants: clean`.
- All 17 tokens are present. Capture the baseline **before** starting:
  `grep -oh 'refines: ADR-\|supersedes: ADR-\|supersedes-invariant: ADR-' docs/decisions/*.md | wc -l`
  and confirm it rises by exactly 17. Use `-oh ... | wc -l`, not `grep -c`: `grep -c` reports
  matching *lines* per file, and Task 1.1 puts two tokens on one line.
- No back-pointer drift: `./x check 2>&1 | grep -c adr-token-backpointer` returns `0`.
- No status flipped: `git diff HEAD~1 --unified=0 -- docs/decisions/ | grep -c '^[-+]status:'`
  returns `0`.

## Notes

- **Informational citations are deliberately left bare** at ADR-0049 line 55, ADR-0081 line 111 and
  ADR-0087 line 76. They cite an anchor without claiming it, so a relation token would record a
  supersession that did not happen. ADR-0131 Decision 4's `cites:` token is the right encoding, and
  the parser does not recognise it until Plan C, so all `cites:` insertions land there together.
- **ADR-0016's citation of ADR-0015's `provenance-banner` is not in this plan.** ADR-0131 examined
  and rejected it: that slug names no path, so relocating the config root leaves it true. This is
  also why ADR-0015 owes no `related: [16]` edge.
- **The three tokens on ADR-0015 item 4 duplicate anchors Plan A claims from item 6.** Intended, per
  the Architecture summary. Plan review confirmed the duplication is harmless in the supersession
  checks (`seenAnchor` is per-ADR, `isRetired` is boolean) but that it **does** double the rendered
  rows in `docs/decisions/ACTIVE.md` and `awf context`, because neither renderer dedups. Plan C
  Task 3.5 adds (anchor, carrier, relation) dedup to both. Until that lands, the doubled rows in
  ACTIVE.md are expected output of this plan, not drift.
- **Four verdicts were overturned in review**, all from `supersedes:` to `refines:`
  (ADR-0024#3, ADR-0030#4, ADR-0013#4, and ADR-0040#1 in the verify pass). Each had a surviving clause the first reading missed, and in
  three of the four the carrier's own prose says "reverses", "is retired", or "supersedes", which is
  what misled it.
  Carrier verbs describe intent; the relation follows from the target's clause set.
