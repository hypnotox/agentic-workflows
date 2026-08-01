---
date: 2026-07-31
adrs: [0207, narrow-the-glossary-terms-validated-claim-to-the-implementation]
status: Proposed
---
# Plan: Two-layer glossary with record-model terms

## Goal

Implement ADR-0207: give the glossary two layers (a catalog-shipped awf vocabulary merged with
project-authored terms), move `data.terms` to a list of records, add a non-failing terseness
advisory, and clean the corpus. Non-goals: contextual surfacing of terms through `awf context`,
and any term-lookup command; both are left to their own decisions by ADR-0207.

## Architecture summary

Four phases, each one green transaction with one closing commit.

Phase 1 corrects the false pitfall-surfacing statements that misled the design, and is
independent of the rest. Phase 2 changes the sidecar data shape and converts both corpora
mechanically. Phase 3 adds the shipped layer on top of the settled shape. Phase 4 adds the
advisory and rewrites the corpus prose against it.

Shape conversion (phase 2) and prose rewriting (phase 4) are deliberately separate transactions:
the first is a reviewable mechanical diff, the second is a judgement diff, and mixing them would
make both unreviewable.

The merge is owned by one helper, `mergedGlossaryRecords`, introduced in phase 3 and consumed by
both `glossaryTransform` and phase 4's advisory producer, so the two-layer merge has a single
home (`code-design/single-home`).

ADR-0207 applies in four batches. Declaration order is enforced within a batch and not across
batches (`internal/adr/history.go`, `parseAppliedOperations` declares its position cursor as a
per-call local and runs once per Applied event), so each batch lists its own operations in
ascending declaration index and no reordering of the ADR's State changes is required.

Phases 1 to 3 each apply their own batch. **Phase 4 applies no batch and authors no claim.** The
fourth batch, the two claims it carries, and the `Implemented` flip are one transaction owned by
the terminal-review flow, per the agent guide: `awf-reviewing-impl` lands the final Applied batch
and the status flip after terminal review settles. Two mechanics force them together rather than
into phase 4: `internal/adr/format.go` rejects an `Implementing` status once every declared
operation is applied and requires the final Applied event immediately before an explicit
`Implemented` transition, while `checkMutations` reports a claim mutation that arrives without its
operation. Phase 4 therefore lands its code, tests, and proof markers only; the ADR rests at
`Implementing` with seven of nine operations applied, which is legal.

## File structure

- **Created:** none.
- **Modified:**
  - `internal/project/glossary.go`, `internal/project/glossary_test.go`
  - `internal/project/check.go`, `internal/project/check_test.go`, `internal/project/notes_test.go`
  - `cmd/awf/initrender_test.go`
  - `internal/catalog/standard.go`
  - `internal/configspec/spec.go`, `internal/configspec/spec_test.go`
  - `templates/docs/glossary.md.tmpl`
  - `templates/docs/doc-standard.md.tmpl`
  - `.awf/docs/glossary.yaml`, `.awf/docs/parts/glossary/prepend.md`
  - `examples/sundial/.awf/docs/glossary.yaml`
  - `.awf/topics/parts/rendering/doc-outputs/current-state.md`
  - `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`
  - `.awf/topics/parts/tooling/cli/current-state.md`
  - `.awf/topics/parts/config/configspec-and-reference/current-state.md`
  - `docs/decisions/0207-two-layer-glossary-with-record-model-terms.md`
  - `changelog/CHANGELOG.md`
  - all rendered outputs `./x render` regenerates (committed with their sources)
- **Deleted:** none.

## Phase 1: Correct the false pitfall-surfacing statements

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. Checkbox tasks are ordered steps, not transaction boundaries. It is independent of
phases 2 to 4 and touches no glossary code.

Background for a fresh owner: `context-surfaces-pitfalls` (ADR-0099) was retired by ADR-0104 and
its replacement `context-surfaces-tiered-pitfalls` was retired by ADR-0134. A pitfall entry's
`domains` today feeds exactly one consumer, the `pitfall-domain` drift check in
`internal/project/check.go`. Two shipped surfaces still assert otherwise.

- [ ] **Task 1.1: Drop the false clause from the `pitfalls` data-key description.** In
  `internal/configspec/spec.go`, the `{Kind: "docs", Artifact: "pitfalls", Key: "pitfalls"}` entry
  currently reads `` `domains` (optional) drive `awf context` surfacing and must resolve to
  configured domains ``. Replace that fragment with `` `domains` (optional) must resolve to
  configured domains ``. Change nothing else in the entry. The description is adopter-facing and
  renders into `docs/config-reference.md`, so it must stay free of ADR citations and repo
  identity (`configspec-description-residue`).

- [ ] **Task 1.2: Update the `pitfall-domains-resolved` claim.** In
  `.awf/topics/parts/rendering/doc-outputs/current-state.md`, replace the claim body

  ```
  check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid and never surfaces through context.
  ```

  with

  ```
  check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid.
  ```

  Add `Revised-by: ADR-0207` to the claim's metadata block, in canonical order (after `Origin:`,
  before `Backing:`). The claim keeps `Backing: test` and its existing proof marker: the live
  property is unchanged, only the false trailing clause is removed.

- [ ] **Task 1.3: Advance ADR-0207 to Implementing and record the first Applied batch in one
  step.** These cannot be separated: `internal/adr/format.go` requires an Implementing status
  event to be immediately followed by an Applied event, so an intermediate state with Implementing
  alone is red and its error aborts validation before the digest comparison runs.

  In `docs/decisions/0207-two-layer-glossary-with-record-model-terms.md`, set
  `status: Implementing` in the frontmatter and append exactly two events to `## Status history`,
  after the existing `- 2026-07-31: Proposed` line:

  ```
  - 2026-07-31: Implementing; content-sha256: 0000000000000000000000000000000000000000000000000000000000000000
  - 2026-07-31: Applied; operations: update `rendering/doc-outputs:pitfall-domains-resolved`
  ```

  Exactly two, and no `Accepted` event: `Proposed -> Implementing` is a direct edge in
  `v2Transitions` (`internal/adr/status.go`), and `HistoryTransitionValid`'s Implementing branch
  requires exactly the two-event `[Status, Applied]` append. A three-event append is rejected.

  The digest is bare, not backticked: `hexDigestRe` in `internal/adr/history.go` is
  `^[0-9a-f]{64}$`, so a backticked value fails with `content-sha256 is not a 64-hex digest`.
  Backticks stay on the `operations:` identifiers. The placeholder must be exactly 64 hex
  characters. Then run `./x check` and replace the placeholder with the digest named in the
  `latest stamped content-sha256 ... does not match the computed digest ...` failure. Follow
  `awf-adr-lifecycle` for the mechanics. This batch carries exactly the claim mutation in task 1.2;
  `awf check --staged` validates the pairing as one HEAD-to-index transaction.

- [ ] **Task 1.4: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  render reports the regenerated config reference and topic doc, and `awf check` prints
  `awf check: clean`.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
fix(rendering): drop the retired pitfall context-surfacing claim
```

## Phase 2: Move `data.terms` to a list of records

**Execution mode: inline.** One independently green transaction. Depends on phase 1 only for the
ADR being in `Implementing`.

This phase changes the authored shape and converts both corpora mechanically. It does not rewrite
any meaning's prose: every meaning moves across byte-identical. Prose is phase 4.

- [ ] **Task 2.1: Rework the transform to read a list of records.** In
  `internal/project/glossary.go`, replace the map-shaped ingestion (`glossaryEntries`,
  `glossaryStringMap`) with list-shaped ingestion, keeping `glossaryTransform`, `glossaryRows`, and
  `glossaryErr` as the surrounding shape. Behaviour to implement:

  - `data.terms` must be a list; a non-list value fails with
    `<sidecar> data.terms: must be a list of {term, meaning} records`.
  - Each element must be a mapping; a non-mapping element fails naming its zero-based index.
  - Each record requires a non-empty string `term` and a non-empty string `meaning`. A missing,
    null, non-string, or empty value fails naming the offending term where the term is known, and
    the record index where it is not.
  - An interior newline in `term` or `meaning` fails naming the term: table rows are single-line.
  - An optional `domains` key is a list of non-empty strings; any other shape fails naming the term.
    Domain *resolution* is not a render concern (task 2.3).
  - An unknown key in a record fails naming the term and the key, so a typo cannot render as silent
    absence.
  - Two records whose `term` values are case-insensitive duplicates fail naming both terms.
  - Rendering is unchanged in output: rows sorted case-insensitively by term, pipes escaped.

  Forbidden: accepting the legacy map shape as a fallback. ADR-0207 decision 11 ships the break
  deliberately, and a silent fallback would leave adopters on the old shape indefinitely.

- [ ] **Task 2.2: Update the glossary tests.** In `internal/project/glossary_test.go`, convert the
  existing cases to the list shape and extend them to cover every branch in task 2.1. Keep the
  proof markers `invariant: rendering/guide-and-doc-templates:glossary-terms-sorted` and
  `invariant: rendering/guide-and-doc-templates:glossary-terms-validated` on the tests that back
  those claims. The sorted test must keep asserting that two sidecars carrying the same records in
  different order render byte-identically. Coverage of `glossary.go` must be complete: the gate
  fails below 100%.

- [ ] **Task 2.3: Add the glossary domains drift check.** In `internal/project/check.go`, add a
  glossary sibling to the existing `pitfall-domain` drift check: a glossary record whose `domains`
  names a domain not configured in the project is a drift finding naming the sidecar path, the
  term, and the unconfigured domain. Follow `checkPitfalls`'s shape exactly, including its
  doc-enabled guard (`if !slices.Contains(p.Cfg.Docs, "glossary")`, returning no findings when the
  doc is disabled). Cover both the finding and the disabled case in `internal/project/check_test.go`.

- [ ] **Task 2.4: Update the template's empty-state prose.** In `templates/docs/glossary.md.tmpl`,
  the `{{ else }}` branch currently reads ``_No terms recorded yet. Add `term: meaning` entries
  under `data.terms` in `.awf/docs/glossary.yaml`._``. After task 2.1 that instruction names a
  shape that now fails the render. Replace it with prose naming the record shape, for example
  ``_No terms recorded yet. Add `- term: <term>` / `  meaning: <meaning>` records under
  `data.terms` in `.awf/docs/glossary.yaml`._``. Docs travel with the change: this belongs in the
  same commit as the shape break.

- [ ] **Task 2.5: Convert this project's corpus to the list shape.** Rewrite
  `.awf/docs/glossary.yaml` from the `term: meaning` map to a list of records. Every meaning moves
  across byte-identical; only the encoding changes. Representative transformation:

  ```yaml
  # before
  data:
    terms:
      "composition root": "The executable or application boundary that has enough production knowledge to select volatile mechanisms and construct their policy consumers explicitly. It is wiring, not a service locator, universal dependency bag, or owner of the consumer's policy."

  # after
  data:
    terms:
      - term: composition root
        meaning: "The executable or application boundary that has enough production knowledge to select volatile mechanisms and construct their policy consumers explicitly. It is wiring, not a service locator, universal dependency bag, or owner of the consumer's policy."
  ```

  Edge case, a meaning containing a double quote or a colon, which must stay quoted so YAML parses
  it as one scalar:

  ```yaml
      - term: example adopter
        meaning: "The committed full-surface adoption at `examples/sundial/` (ADR-0090): a fictional Go CLI in its own module carrying a complete `.awf/` tree with everything enabled and all rendered output checked in. The cold-start onboarding artifact, re-rendered from source by `./x render` and held note-free by `./x check`; see also \"quality oracle\"."
  ```

  Affected sites: every entry in `.awf/docs/glossary.yaml`. Do not add `domains` in this phase and
  do not change any meaning's text. Deterministic post-check for this task alone, root tree only,
  because `./x render` also re-renders the still-unconverted example adopter and would abort:
  `./awf render` succeeds and `git diff --stat docs/glossary.md` reports no change to that file.

- [ ] **Task 2.6: Convert the example adopter's corpus.** Rewrite
  `examples/sundial/.awf/docs/glossary.yaml` to the same list shape, meanings byte-identical.
  `examples/sundial` is re-rendered by `./x render` and never by `awf upgrade`, so this conversion
  is mandatory in this commit or the example's zero-drift gate fails.

- [ ] **Task 2.7: Update the `terms` data-key description.** In `internal/configspec/spec.go`,
  replace the `{Kind: "docs", Artifact: "glossary", Key: "terms"}` Description with one describing
  the list shape: the record fields (`term`, `meaning`, optional `domains`), that the table renders
  always sorted case-insensitively with pipes escaped, that an empty term or meaning, an interior
  newline, an unknown record key, or a case-insensitive duplicate term fails the render naming the
  offending term, and that unset renders a pointer telling the reader where to add terms. Keep it
  free of ADR citations and repo identity.

- [ ] **Task 2.8: Update the two encoding-bound claims and add the domains claim.** In
  `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`:

  For `glossary-terms-sorted`, replace `regardless of the authored map order` with
  `regardless of the authored order`.

  For `glossary-terms-validated`, replace the body with one naming the list shape and identifying
  the failure by term rather than by key: an empty term, an empty, null, or non-string meaning, an
  interior newline in a term or meaning, a malformed record, an unknown record key, or a
  case-insensitive duplicate term **within a single layer** fails the render, naming the sidecar
  path and the offending term.

  The layer qualifier is load-bearing even though only one layer exists at phase 2. ADR-0207
  declares exactly one `update` for this claim and phase 2 spends it, so an unqualified body would
  freeze permanently false the moment task 3.3 makes a cross-layer duplicate the legal override.
  Moving the update into batch 3 instead is wrong: that would leave the map-shaped body live
  through the very commit that breaks it.

  Add `Revised-by: ADR-0207` to both claims' metadata blocks in canonical order.

  In `.awf/topics/parts/rendering/doc-outputs/current-state.md`, add the new claim
  `glossary-domains-resolved` with `Origin: ADR-0207`, `Backing: test`, and a body stating that
  `check` fails a glossary record whose domains list names a domain not configured in the project,
  and that a record with no domains is valid. Place the proof marker
  `invariant: rendering/doc-outputs:glossary-domains-resolved` on the task 2.3 test.

- [ ] **Task 2.9: Record the second Applied batch.** Append to ADR-0207's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: add `rendering/doc-outputs:glossary-domains-resolved`, update `rendering/guide-and-doc-templates:glossary-terms-sorted`, update `rendering/guide-and-doc-templates:glossary-terms-validated`
  ```

  The operations are listed in ascending declaration index, which `internal/adr/history.go`
  enforces within the batch. Update the stamped digest as in task 1.3 if `awf check` reports a
  mismatch.

- [ ] **Task 2.10: Add the breaking-change entry.** Under `## [Unreleased]` then
  `### Breaking changes` in `changelog/CHANGELOG.md`, add an entry stating that `data.terms` in
  `.awf/docs/glossary.yaml` is now a list of `{term, meaning, domains}` records rather than a
  `term: meaning` map, that no migration converts it (following the precedent for this key), and
  giving the conversion recipe: each `"<term>": "<meaning>"` pair becomes a
  `- term: <term>` / `  meaning: <meaning>` record. State that an unconverted tree fails the render
  naming the sidecar.

- [ ] **Task 2.11: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  `awf check: clean`, and `git diff --stat docs/glossary.md examples/sundial/docs/glossary.md`
  reports no change to either rendered file, proving the conversion was encoding-only.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
feat(rendering): model glossary terms as records
```

## Phase 3: Ship the standard vocabulary layer

**Execution mode: inline.** One independently green transaction. Depends on phase 2's record shape.

- [ ] **Task 3.1: Define the terseness threshold constant.** In `internal/project/glossary.go`, add
  an unexported constant for the meaning-length threshold with the value `280` and a comment giving
  ADR-0207 decision 9 as its ground. Its only consumer in this phase is task 3.5's portability test;
  phase 4's advisory consumes the same constant. This is a deliberate exception to landing a
  definition with its first production consumer, recorded here so the phase does not read as a
  doctrine violation: ADR-0207 decision 7 makes the portability bound a required phase-3
  deliverable, and the gate permits it (the dead-code gate is function-scoped and the unused linter
  counts test usage). Do not add the advisory here.

- [ ] **Task 3.2: Ship the standard vocabulary in the catalog.** In `internal/catalog/standard.go`,
  give the `"glossary"` `DocEntry` a `Data` map carrying `"standardTerms"` as a `[]any` of
  `map[string]any` records, each with exactly `"term"` and `"meaning"` string values.

  The shipped set is exactly these fifteen terms, a closed list: `effort`, `managed effort
  worktree`, `working memory`, `current-state topic`, `claim`, `invariant backing`, `drift`,
  `resident root`, `stub`, `check-in`, `mandatory approval check-in`, `routine checkpoint`,
  `continuity notice`, `retrospective`, `promotion ladder`. Do not add or drop a term without
  amending this plan.

  Every meaning must be under the task 3.1 threshold, must carry no ADR citation in any form, and
  must contain no repo identity: `TestCatalogDataResidue` and `TestCatalogDefaultDataIsGeneric` both
  walk this Data. Representative record:

  ```go
  map[string]any{"term": "effort", "meaning": "One active slugged unit of coordination, owning a working-memory file for the duration of a concrete non-minimal outcome. A minimal fix uses none."},
  ```

  Note this is the first `Docs` entry in the catalog to carry `Data`; every existing `Data` block
  belongs to a skill or an agent.

- [ ] **Task 3.3: Introduce the merge helper.** In `internal/project/glossary.go`, add

  ```go
  func mergedGlossaryRecords(sc config.Sidecar) ([]glossaryRecord, error)
  ```

  It ingests `standardTerms` and `terms` with the same per-record validation as task 2.1, then
  merges: a `terms` record overrides a `standardTerms` record whose `term` matches
  case-insensitively. A case-insensitive duplicate *within* either layer stays a hard render error;
  a duplicate *across* layers is the override and must not error. The returned slice is the merged
  set in no guaranteed order; sorting stays in `glossaryRows`.

  This helper is the single home of the merge. Phase 4's advisory producer calls the same function;
  nothing re-implements or re-derives the merge.

- [ ] **Task 3.4: Rewire the transform onto the helper.** `glossaryTransform` currently opens with
  `raw, ok := sc.Data["terms"]; if !ok { return sc, nil }`, which would short-circuit before any
  merge and leave a project that authors no terms rendering the empty-state pointer. Since
  `internal/initspec` seeds no terms, that is exactly the fresh adoption ADR-0207 exists to fix.

  Replace the guard: return early only when BOTH `terms` and `standardTerms` are absent. Otherwise
  call `mergedGlossaryRecords`, pass its result to `glossaryRows`, write the rows to
  `out.Data["terms"]`, and delete `standardTerms` from `out.Data` so the template consumes only
  `terms`.

- [ ] **Task 3.5: Add the shipped-set and merge tests.** In `internal/project/glossary_test.go`:

  A portability test over `catalog.Standard.Docs["glossary"].Data["standardTerms"]` asserting that
  every record carries exactly the keys `term` and `meaning`, that both values are strings, that no
  value matches `ADR-[0-9]{4}`, and that no meaning exceeds the task 3.1 threshold. Mark it
  `invariant: rendering/guide-and-doc-templates:glossary-standard-terms-portable`.

  Merge and override tests marked
  `invariant: rendering/guide-and-doc-templates:glossary-standard-vocabulary`, including a case
  with no authored `terms` at all, asserting the shipped rows still render and the empty-state
  branch is not taken.

- [ ] **Task 3.6: Build the configspec data-key exemption mechanism.** `TestConfigspecDataParity`
  has no exemption set today: its two documented exemptions are structural (the domain template is
  never iterated, and docs are skipped by `if e.Generated { continue }`). Introduce the mechanism
  rather than assuming it.

  In `internal/configspec/spec_test.go`, add an `exemptDataKeys` map keyed by the existing
  `ak{kind, artifact, key}` struct, declared INSIDE `TestConfigspecDataParity` immediately after the
  `type ak` line and before `collect` (the `ak` type is function-scoped, so a package-level map will
  not compile). Consult it in `collect`'s `for k := range defaults` loop to skip an exempt key, and
  populate it with `{kind: "docs", artifact: "glossary", key: "standardTerms"}` carrying a comment
  giving the same ground the existing exemptions use: it is not adopter-settable.
  Leave the two structural exemptions where they are; this adds a third mechanism member without
  migrating them.

  Do NOT add a `configspec` descriptor for `standardTerms`; ADR-0207 decision 3 forbids publishing
  it as a key an adopter may write.

- [ ] **Task 3.7: Correct the glossary document description.** In `internal/catalog/standard.go`,
  change the `"glossary"` entry's `Desc` from `project jargon and term ownership` to
  `project jargon and the awf vocabulary it ships`. The old text promises an ownership concept the
  glossary has never had and renders into every adopter's agent guide document map.

- [ ] **Task 3.8: Reconcile the glossary prepend part.** `.awf/docs/parts/glossary/prepend.md`
  defines `**Effort:**`, `**Managed effort worktree:**`, and `**Finishing tombstone:**` as bullets
  above the table. Task 3.2 ships `effort` and `managed effort worktree` as standard terms, so
  without this task the rendered page would define each twice in two wordings. Delete the `Effort`
  and `Managed effort worktree` bullets from the part; keep `Finishing tombstone`, which the shipped
  set does not cover, or move it into `data.terms` as a project term if it reads better in the
  table.

- [ ] **Task 3.9: Update the `configspec-data-parity` claim.** In
  `.awf/topics/parts/config/configspec-and-reference/current-state.md`, extend the claim's final
  sentence so the exemption set names the third member: the domain template's injected pair, the
  generated config reference's injected collections, and the glossary's shipped standard
  vocabulary. Add `Revised-by: ADR-0207` in canonical order.

- [ ] **Task 3.10: Add the two new guide-and-doc-templates claims.** In
  `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, add:

  `glossary-standard-vocabulary` (`Origin: ADR-0207`, `Backing: test`): the rendered glossary merges
  the catalog's shipped standard vocabulary with the project's authored terms into one sorted table,
  a project term overriding a shipped term of the same case-insensitive name.

  Deliberately stop there. The within-layer duplicate failure belongs to
  `glossary-terms-validated`, which task 2.8 already words layer-aware; restating it here would put
  the same property in two claims of one topic, and task 3.5 specifies no test asserting a duplicate
  inside the shipped layer, so the restatement would be the unbacked half.

  `glossary-standard-terms-portable` (`Origin: ADR-0207`, `Backing: test`): every shipped standard
  term carries exactly a string term and a string meaning, with no domains key, no ADR reference,
  and no meaning exceeding the terseness threshold, so the shipped layer is portable into any
  adopter tree.

- [ ] **Task 3.11: Record the third Applied batch.** Append to ADR-0207's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: add `rendering/guide-and-doc-templates:glossary-standard-vocabulary`, add `rendering/guide-and-doc-templates:glossary-standard-terms-portable`, update `config/configspec-and-reference:configspec-data-parity`
  ```

  Update the stamped digest as in task 1.3 if `awf check` reports a mismatch.

- [ ] **Task 3.12: Add the feature changelog entry.** Under `## [Unreleased]` then `### Features` in
  `changelog/CHANGELOG.md`, add an entry stating that the glossary now renders a shipped awf
  vocabulary merged with the project's own terms, that a project term of the same name overrides the
  shipped one, and that the shipped layer is not disableable. Include the upgrade effect, which is
  what makes this adopter-visible: shipped vocabulary participates in the config hash, so an upgrade
  that changes a standard term surfaces as `stale` drift on the adopter's rendered glossary and is
  resolved by `awf render`, exactly as any other catalog or template change.

- [ ] **Task 3.13: Bring the `terms` description current with the shipped layer.** Task 2.7's
  wording is true only while one layer exists, and this phase falsifies part of it. In
  `internal/configspec/spec.go`, amend the `{Kind: "docs", Artifact: "glossary", Key: "terms"}`
  Description:

  Replace the sentence stating that unset renders a pointer telling the reader where to add terms.
  Task 3.4 makes the transform return early only when both layers are absent, and `withDefaultData`
  always supplies `standardTerms` from the catalog, so an unset `terms` now renders the shipped
  standard vocabulary alone. The empty-state pointer renders only when neither layer supplies a
  term.

  Add a clause stating that a term here overrides a shipped standard term of the same
  case-insensitive name. Per ADR-0207 decisions 2 and 3 that override is the *only* way to remove
  an unwanted shipped term, so an adopter who does not read it has no mechanism at all.

  Name the shipped layer descriptively ("the standard vocabulary awf ships"), never as the
  `standardTerms` key. The residue check bans only ADR citations and repo identity, so naming the
  key would pass the gate and still publish, in the adopter's config reference, a key that
  ADR-0207 decisions 3 and 4 make `unused-data` drift the moment an adopter authors it.

  Keep both free of ADR citations and repo identity (`configspec-description-residue`). Docs travel
  with the change: this belongs in the phase that ships the layer, not phase 4.

- [ ] **Task 3.14: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  `awf check: clean`, and `git diff docs/glossary.md examples/sundial/docs/glossary.md` shows the
  shipped vocabulary appearing in both, proving the layer reaches a real adopter. `./x check` must
  also report no advisory note from `examples/sundial`, which the runner fails on: at this phase
  that covers the five pre-existing note families over an example whose glossary now carries the
  shipped rows. The terseness threshold is enforced here by task 3.5's portability test only; it
  reaches `./x check` in phase 4.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
feat(rendering): ship a standard glossary vocabulary to adopters
```

## Phase 4: Add the terseness advisory and rewrite the corpus

**Execution mode: inline.** One independently green transaction. Depends on phase 3's threshold
constant and merge helper. It applies no ADR batch and authors no claim: those belong to the
terminal transaction described in Notes.

- [ ] **Task 4.1: Add the terseness advisory family.** In `internal/project/check.go`, add a
  glossary terseness note producer to `AdvisoryNotes`, alongside the unset-var, stub, part-marker,
  tag-health, and plan-commit-scope families. Behaviour:

  - It reads the authored sidecar, then wraps it as
    `withDefaultData(sc, p.Cat.Docs["glossary"].Data)` BEFORE calling `mergedGlossaryRecords`,
    mirroring what `internal/project/render.go` does upstream of the transform. This step is
    load-bearing: `p.Cfg.Sidecar("docs", "glossary")` returns the on-disk file only and never
    carries `standardTerms`, so without the wrap the advisory would evaluate authored terms alone
    and the shipped layer would escape the threshold entirely, defeating ADR-0207 decision 10 and
    contradicting the claim body task 4.3 authors.
  - It obtains its records by calling `mergedGlossaryRecords` (task 3.3), never by re-merging.
  - It emits one note per term whose merged meaning exceeds the phase 3 threshold, naming the term
    and its length, sorted, exactly as the sibling families sort theirs.
  - It returns no notes when `glossary` is not in `p.Cfg.Docs`, mirroring `checkPitfalls`'s
    doc-enabled guard.
  - It never affects the exit code.

  Cover the producing, the disabled, and the under-threshold cases in
  `internal/project/notes_test.go` for the 100% floor. The two inherited error returns
  (`p.Cfg.Sidecar` and `mergedGlossaryRecords`) are unreachable once `AdvisoryNotes` has already
  rendered the same sidecar earlier in its own run: give each a
  `// coverage-ignore: <reason>` in the shape `checkPitfalls` uses, rather than contriving a test
  to drive them.

- [ ] **Task 4.2: Prove the non-failing contract at the CLI boundary.** In
  `cmd/awf/initrender_test.go`, add a case asserting that `awf check` exits zero while a glossary
  terseness note is present in its output, carrying the proof marker
  `invariant: tooling/cli:terseness-advisory-nonfailing`. This mirrors where the sibling contracts
  are proven: `completeness-advisory-nonfailing` and `stub-advisory-nonfailing` both carry their
  markers in that file. A marker on the `notes_test.go` producer test would satisfy the textual
  ledger while proving nothing about the exit code.

- [ ] **Task 4.3: Leave both advisory claims unauthored, and their proof markers with them.** This
  is a deliberate non-action, recorded so an executor does not helpfully add either.
  `glossary-terseness-advisory` and `terseness-advisory-nonfailing` are the fourth batch's claims
  and must arrive in the same transaction as their Applied event, which the terminal-review flow
  owns (see Notes). Authoring them here would make `checkMutations` report a claim mutation with no
  operation and phase 4 could not close green.

  The two proof markers stay out of this phase for the mirror-image reason. An earlier revision of
  this plan placed them here, reasoning that backing validation runs claim to marker and never the
  reverse, so an orphaned marker would be inert. That is wrong: the current-state marker scan
  validates marker to claim as well, failing with `unknown claim ID <domain>/<topic>:<slug>` before
  any backing comparison runs, so an orphaned marker is hard-red rather than inert. Both markers
  therefore land in the terminal transaction beside the claims they prove. The tests themselves do
  land in this phase, simply carrying no marker yet; a test without a marker is legal.

- [ ] **Task 4.4: Add the glossary rule to the documentation standard.** In
  `templates/docs/doc-standard.md.tmpl`, add a rule to the `rules` section, immediately after the
  existing **Terse.** bullet it refines:

  ```
  - **Glossary entries are terser still.** One sentence stating what the thing is; a second only when a contrast or boundary is load-bearing. Do not restate what the term's own words already say. An over-long entry raises a non-failing advisory.
  ```

  This is shipped prose: no ADR citation, no repo identity, plain punctuation.

- [ ] **Task 4.5: Carry the same guidance to the authoring surface.** In
  `internal/configspec/spec.go`, extend the `{Kind: "docs", Artifact: "glossary", Key: "terms"}`
  Description with one clause stating that a meaning longer than the threshold raises a non-failing
  advisory, so an author meets the rule where they write.

- [ ] **Task 4.6: Rewrite the corpus.** In `.awf/docs/glossary.yaml`, bring every meaning under the
  threshold, remove entries describing mechanisms that no longer exist, delete the
  `memory-backed effort` entry outright, correct the `pitfall entry` entry to drop its claim that
  domains drive context surfacing, and drop any term the shipped layer now defines at least as well.
  Assign `domains` where an entry clearly belongs to one configured domain, leaving it absent where
  it does not. Representative rewrite:

  ```yaml
  # before
      - term: claimed-path model
        meaning: "The ADR-0086 allowlist deciding what may live under `.awf/`: the skeleton (`config.yaml`, `awf.lock`), the enabled render units, enabled artifacts' sidecars and declared-section parts, and the singleton sidecars/parts, with the owned resident roots `efforts/**` and `worktrees/**` exempt. Derived from config + catalog + the output plan's write files; every entry outside it is failing `awf check` drift, collapsed to the topmost unclaimed directory."

  # after
      - term: claimed-path model
        meaning: "The allowlist deciding what may live under `.awf/`, derived from config, catalog, and the output plan. Anything outside it is failing drift, reported at the topmost unclaimed directory."
        domains: [rendering]
  ```

  Edge case, an entry that is already terse and needs only a domain tag:

  ```yaml
      - term: adaptive maximum
        meaning: "An exploration breadth limit: start with the cheapest targeted lookup, widen only when evidence requires it, and report when the selected breadth is exhausted."
  ```

  Affected sites: every record in `.awf/docs/glossary.yaml`. Forbidden: deleting an entry merely
  because it is long, and padding an entry to look uniform. Deterministic post-check, run after
  task 4.9 so a failing check cannot masquerade as a clean corpus: `./x check` exits zero AND its
  output contains no glossary terseness note.

- [ ] **Task 4.7: Leave ADR-0207 at Implementing.** Another deliberate non-action. Do not append an
  Applied event and do not touch the frontmatter status. After phase 3 the ADR carries seven of its
  nine operations applied, which keeps `Implementing` legal (that status requires at least one
  applied and at least one remaining). The final batch and the flip land in the terminal
  transaction described in Notes.

- [ ] **Task 4.8: Add the advisory changelog entry.** Under `## [Unreleased]` then `### Features` in
  `changelog/CHANGELOG.md`, add an entry stating that `awf check` now reports a non-failing advisory
  for a glossary meaning longer than the threshold, and that the rule is stated in the documentation
  standard.

- [ ] **Task 4.9: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  `awf check` exits zero printing `awf check: clean`, with no glossary advisory note in its output.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
feat(rendering): add the terseness advisory and clean the glossary
```

## Verification

- `./x render && ./x check` exits zero reporting `awf check: clean`, with no glossary advisory note.
- `./x gate` passes, including the 100% statement coverage floor and the dead-code gate.
- `./awf check invariants` resolves every ADR-0207 claim to its proof marker.
- The shipped set in `internal/catalog/standard.go` contains exactly the fifteen terms task 3.2
  names, no more and no fewer.
- `git diff --stat` over `examples/sundial/docs/glossary.md` between the phase 2 and phase 3 commits
  shows the shipped vocabulary arriving in the example adopter, which is the end-to-end proof that
  an adopter receives it.
- `docs/config-reference.md` documents `terms` in its record shape and does not list
  `standardTerms` as an adopter-settable key.
- `docs/glossary.md` defines no term twice: the prepend part and the merged table do not overlap.
- At the end of phase 4, ADR-0207 is at `status: Implementing` with three Applied events covering
  seven of its nine operations. After the terminal transaction it is at `status: Implemented` with
  four Applied events whose operations union to exactly its nine, no operation applied twice.

## Notes

- Contextual surfacing of glossary terms is out of scope by ADR-0207. The `domains` assigned in
  task 4.6 are the data that decision will consume; nothing reads them beyond the task 2.3 drift
  check until then. That later decision should carry pitfall entries too, whose surfacing is
  equally absent today.
- A term-lookup command (`awf term`) stays unjustified until the surfacing work shows whether
  discovery is still a problem.
- The corpus rewrite in task 4.6 is the largest single work item in this plan and is judgement work,
  not mechanical: it is deliberately separated from the phase 2 encoding change so each is
  reviewable on its own terms.
- **Deviation, out of plan.** ADR-0207 decision 13 enumerates four surfaces carrying the false
  pitfall-surfacing statement. Execution found a fifth: `pitfallEntry`'s doc comment in
  `internal/project/pitfalls.go`, which also named `ContextFor` as a consumer that reads no pitfall
  entries (the symbol is now `ContextForOptions`, and nothing under `internal/contextq` or
  `internal/contextdelivery` reads pitfall entries at all). Fixed in its own commit, outside the
  phase 2 transaction, because it is a phase 1 concern rather than a glossary one.
- **Record, within task 4.6's authorization.** The corpus rewrite dropped five entries: `check-in`,
  `continuity notice`, `retrospective`, and `routine checkpoint`, each now defined at least as well
  by the shipped standard vocabulary, plus `memory-backed effort` per ADR-0207 decision 12. Seven
  further project terms deliberately survive as overrides of shipped ones, because each carries
  repo-specific detail the generic shipped wording does not.
- **Terminal transaction, owned by `awf-reviewing-impl` after terminal review settles.** It is one
  commit carrying four things that cannot be separated:

  1. `glossary-terseness-advisory` in `.awf/topics/parts/rendering/doc-outputs/current-state.md`
     (`Origin: ADR-0207`, `Backing: test`): the advisory reports one note per glossary term whose
     meaning exceeds the threshold, over the merged shipped-and-authored set. Its proof marker
     `invariant: rendering/doc-outputs:glossary-terseness-advisory` is added here, on the task 4.1
     producer test `TestGlossaryTersenessNotes` in `internal/project/notes_test.go`.
  2. `terseness-advisory-nonfailing` in `.awf/topics/parts/tooling/cli/current-state.md`
     (`Origin: ADR-0207`, `Backing: test`), worded to match `completeness-advisory-nonfailing` and
     `stub-advisory-nonfailing`: the glossary terseness notes `awf check` prints are informational
     only and never change the command's exit code. Its proof marker
     `invariant: tooling/cli:terseness-advisory-nonfailing` is added here, on the task 4.2 test
     `TestCheckGlossaryTersenessNotesAreNonFailing` in `cmd/awf/initrender_test.go`.

  Both markers land in this transaction rather than in phase 4 because the current-state marker
  scan rejects a marker naming a claim that does not exist yet (task 4.3).
  3. The fourth Applied event:
     `- <date>: Applied; operations: add `rendering/doc-outputs:glossary-terseness-advisory`, add `tooling/cli:terseness-advisory-nonfailing``
  4. `status: Implemented` in the ADR frontmatter plus the matching Implemented history event,
     stamped as in task 1.3 and placed immediately after the Applied event.
  5. `status: Implemented` in THIS PLAN's frontmatter, per the deferred flip transaction in
     `docs/plans/README.md`. Nothing mechanical catches its omission, so it is listed explicitly.
     Task checkboxes stay unticked, matching every implemented plan in the repo.

  They are one transaction because a claim mutation without its operation is a finding, and because
  an `Implementing` status is rejected once every declared operation is applied.
