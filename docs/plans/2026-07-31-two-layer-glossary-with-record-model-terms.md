---
date: 2026-07-31
adrs: [0198]
status: Proposed
---
# Plan: Two-layer glossary with record-model terms

## Goal

Implement ADR-0198: give the glossary two layers (a catalog-shipped awf vocabulary merged with
project-authored terms), move `data.terms` to a list of records, add a non-failing terseness
advisory, and clean the corpus. Non-goals: contextual surfacing of terms through `awf context`,
and any term-lookup command; both are left to their own decisions by ADR-0198.

## Architecture summary

Four phases, each one green transaction with one closing commit.

Phase 1 corrects the false pitfall-surfacing statements that misled the design, and is
independent of the rest. Phase 2 changes the sidecar data shape and converts both corpora
mechanically. Phase 3 adds the shipped layer on top of the settled shape. Phase 4 adds the
advisory and rewrites the corpus prose against it.

Shape conversion (phase 2) and prose rewriting (phase 4) are deliberately separate transactions:
the first is a reviewable mechanical diff, the second is a judgement diff, and mixing them would
make both unreviewable.

ADR-0198 applies in four batches, one per phase. Declaration order is enforced within a batch
and not across batches (`internal/adr/history.go`, `parseAppliedOperations` resets its position
cursor per Applied event), so each batch lists its own operations in ascending declaration
index and no reordering of the ADR's State changes is required.

## File structure

- **Created:** none.
- **Modified:**
  - `internal/project/glossary.go`, `internal/project/glossary_test.go`
  - `internal/project/check.go`, `internal/project/check_test.go`, `internal/project/notes_test.go`
  - `internal/catalog/standard.go`
  - `internal/configspec/spec.go`, `internal/configspec/spec_test.go`
  - `.awf/docs/glossary.yaml`
  - `examples/sundial/.awf/docs/glossary.yaml`
  - `templates/docs/doc-standard.md.tmpl`
  - `.awf/topics/parts/rendering/doc-outputs/current-state.md`
  - `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`
  - `.awf/topics/parts/tooling/cli/current-state.md`
  - `.awf/topics/parts/config/configspec-and-reference/current-state.md`
  - `docs/decisions/0198-two-layer-glossary-with-record-model-terms.md`
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

- [ ] **Task 1.1: Advance ADR-0198 to Implementing.** Incremental application requires a
  non-terminal status before the first Applied event. In
  `docs/decisions/0198-two-layer-glossary-with-record-model-terms.md`, set `status: Implementing`
  in the frontmatter and append two events to `## Status history`, after the existing
  `- 2026-07-31: Proposed` line:

  ```
  - 2026-07-31: Accepted; content-sha256: `<digest>`
  - 2026-07-31: Implementing; content-sha256: `<digest>`
  ```

  `<digest>` is the content stamp `awf check` expects for the current body; run `./x check` and
  take the expected digest from the failure message rather than computing it by hand. Follow
  `awf-adr-lifecycle` for the mechanics. Expected terminal state after this task alone: `./x check`
  reports no ADR-0198 lifecycle finding.

- [ ] **Task 1.2: Drop the false clause from the `pitfalls` data-key description.** In
  `internal/configspec/spec.go`, the `{Kind: "docs", Artifact: "pitfalls", Key: "pitfalls"}` entry
  currently reads `` `domains` (optional) drive `awf context` surfacing and must resolve to
  configured domains ``. Replace that fragment with `` `domains` (optional) must resolve to
  configured domains ``. Change nothing else in the entry. The description is adopter-facing and
  renders into `docs/config-reference.md`, so it must stay free of ADR citations and repo
  identity (`configspec-description-residue`).

- [ ] **Task 1.3: Update the `pitfall-domains-resolved` claim.** In
  `.awf/topics/parts/rendering/doc-outputs/current-state.md`, replace the claim body

  ```
  check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid and never surfaces through context.
  ```

  with

  ```
  check fails a pitfall entry whose domains list names a domain not configured in the project; an entry with no domains is valid.
  ```

  Add `Revised-by: ADR-0198` to the claim's metadata block, in canonical order (after `Origin:`,
  before `Backing:`). The claim keeps `Backing: test` and its existing proof marker: the live
  property is unchanged, only the false trailing clause is removed.

- [ ] **Task 1.4: Record the first Applied batch.** Append to ADR-0198's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: update `rendering/doc-outputs:pitfall-domains-resolved`
  ```

  This batch carries exactly the claim mutation in task 1.3; `awf check --staged` validates the
  pairing as one HEAD-to-index transaction.

- [ ] **Task 1.5: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
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

  Forbidden: accepting the legacy map shape as a fallback. ADR-0198 decision 11 ships the break
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
  term, and the unconfigured domain. Follow the pitfall check's shape and severity exactly. Cover
  it in `internal/project/check_test.go`.

- [ ] **Task 2.4: Convert this project's corpus to the list shape.** Rewrite
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
  do not change any meaning's text. Deterministic post-check: `./x render` leaves
  `docs/glossary.md` byte-identical to its pre-conversion content except for row ordering being
  unchanged, verified by `git diff --stat docs/glossary.md` reporting no change to that file.

- [ ] **Task 2.5: Convert the example adopter's corpus.** Rewrite
  `examples/sundial/.awf/docs/glossary.yaml` to the same list shape, meanings byte-identical.
  `examples/sundial` is re-rendered by `./x render` and never by `awf upgrade`, so this conversion
  is mandatory in this commit or the example's zero-drift gate fails.

- [ ] **Task 2.6: Update the `terms` data-key description.** In `internal/configspec/spec.go`,
  replace the `{Kind: "docs", Artifact: "glossary", Key: "terms"}` Description with one describing
  the list shape: the record fields (`term`, `meaning`, optional `domains`), that the table renders
  always sorted case-insensitively with pipes escaped, that an empty term or meaning, an interior
  newline, an unknown record key, or a case-insensitive duplicate term fails the render naming the
  offending term, and that unset renders a pointer telling the reader where to add terms. Keep it
  free of ADR citations and repo identity.

- [ ] **Task 2.7: Update the two encoding-bound claims.** In
  `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`:

  For `glossary-terms-sorted`, replace `regardless of the authored map order` with
  `regardless of the authored order`.

  For `glossary-terms-validated`, replace the body with one naming the list shape and identifying
  the failure by term rather than by key: an empty term, an empty, null, or non-string meaning, an
  interior newline in a term or meaning, a malformed record, an unknown record key, or a
  case-insensitive duplicate term fails the render, naming the sidecar path and the offending term.

  Add `Revised-by: ADR-0198` to both claims' metadata blocks in canonical order.

  In `.awf/topics/parts/rendering/doc-outputs/current-state.md`, add the new claim
  `glossary-domains-resolved` with `Origin: ADR-0198`, `Backing: test`, and a body stating that
  `check` fails a glossary record whose domains list names a domain not configured in the project,
  and that a record with no domains is valid. Place the proof marker
  `invariant: rendering/doc-outputs:glossary-domains-resolved` on the task 2.3 test.

- [ ] **Task 2.8: Record the second Applied batch.** Append to ADR-0198's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: add `rendering/doc-outputs:glossary-domains-resolved`, update `rendering/guide-and-doc-templates:glossary-terms-sorted`, update `rendering/guide-and-doc-templates:glossary-terms-validated`
  ```

  The operations are listed in ascending declaration index, which
  `internal/adr/history.go` enforces within the batch.

- [ ] **Task 2.9: Add the breaking-change entry.** Under `## [Unreleased]` then
  `### Breaking changes` in `changelog/CHANGELOG.md`, add an entry stating that `data.terms` in
  `.awf/docs/glossary.yaml` is now a list of `{term, meaning, domains}` records rather than a
  `term: meaning` map, that no migration converts it (following the precedent for this key), and
  giving the conversion recipe: each `"<term>": "<meaning>"` pair becomes a
  `- term: <term>` / `  meaning: <meaning>` record. State that an unconverted tree fails the render
  naming the sidecar.

- [ ] **Task 2.10: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
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
  ADR-0198 decision 9 as its ground. Its first consumer is task 3.4's portability test; the
  advisory in phase 4 consumes the same constant. Do not add the advisory here.

- [ ] **Task 3.2: Ship the standard vocabulary in the catalog.** In `internal/catalog/standard.go`,
  give the `"glossary"` `DocEntry` a `Data` map carrying `"standardTerms"` as a `[]any` of
  `map[string]any` records, each with exactly `"term"` and `"meaning"` string values. Author the
  awf vocabulary an adopter meets in their rendered artifacts: at minimum `effort`, `managed effort
  worktree`, `working memory`, `current-state topic`, `claim`, `invariant backing`, `drift`,
  `resident root`, `stub`, `check-in`, `mandatory approval check-in`, `routine checkpoint`,
  `continuity notice`, `retrospective`, and `promotion ladder`. Every meaning must be under the
  task 3.1 threshold, must carry no ADR citation in any form, and must contain no repo identity:
  `TestCatalogDataResidue` and `TestCatalogDefaultDataIsGeneric` both walk this Data. Representative
  record:

  ```go
  map[string]any{"term": "effort", "meaning": "One active slugged unit of coordination, owning a working-memory file for the duration of a concrete non-minimal outcome. A minimal fix uses none."},
  ```

  Note this is the first `Docs` entry in the catalog to carry `Data`; every existing `Data` block
  belongs to a skill or an agent.

- [ ] **Task 3.3: Merge the two layers in the transform.** In `internal/project/glossary.go`, have
  `glossaryTransform` ingest `standardTerms` with the same record validation as `terms`, then merge:
  a project record overrides a standard record whose `term` matches case-insensitively, and the
  merged set renders as one sorted table. A case-insensitive duplicate *within* either layer stays a
  hard render error; a duplicate *across* layers is the override and must not error. Remove
  `standardTerms` from the transformed sidecar data so the template consumes only `terms`.

- [ ] **Task 3.4: Add the shipped-set portability test.** In `internal/project/glossary_test.go`,
  add a test over `catalog.Standard.Docs["glossary"].Data["standardTerms"]` asserting that every
  record carries exactly the keys `term` and `meaning`, that both values are strings, that no value
  matches `ADR-[0-9]{4}`, and that no meaning exceeds the task 3.1 threshold. Mark it with the proof
  marker `invariant: rendering/guide-and-doc-templates:glossary-standard-terms-portable`. Add merge
  and override tests marked
  `invariant: rendering/guide-and-doc-templates:glossary-standard-vocabulary`.

- [ ] **Task 3.5: Exempt `standardTerms` from configspec data parity.** In
  `internal/configspec/spec_test.go`, add `standardTerms` to the exemption set that
  `TestConfigspecDataParity` applies, alongside the domain template's injected pair and the
  generated config reference's injected collections, with a comment giving the same ground the
  existing exemptions use: it is not adopter-settable. Do NOT add a `configspec` descriptor for it;
  ADR-0198 decision 3 forbids publishing it as a key an adopter may write.

- [ ] **Task 3.6: Correct the glossary document description.** In `internal/catalog/standard.go`,
  change the `"glossary"` entry's `Desc` from `project jargon and term ownership` to
  `project jargon and the awf vocabulary it ships`. The old text promises an ownership concept the
  glossary has never had and renders into every adopter's agent guide document map.

- [ ] **Task 3.7: Update the `configspec-data-parity` claim.** In
  `.awf/topics/parts/config/configspec-and-reference/current-state.md`, extend the claim's final
  sentence so the exemption set names the third member: the domain template's injected pair, the
  generated config reference's injected collections, and the glossary's shipped standard
  vocabulary. Add `Revised-by: ADR-0198` in canonical order.

- [ ] **Task 3.8: Add the two new guide-and-doc-templates claims.** In
  `.awf/topics/parts/rendering/guide-and-doc-templates/current-state.md`, add:

  `glossary-standard-vocabulary` (`Origin: ADR-0198`, `Backing: test`): the rendered glossary merges
  the catalog's shipped standard vocabulary with the project's authored terms into one sorted table,
  a project term overriding a shipped term of the same case-insensitive name, while a duplicate term
  within either layer fails the render.

  `glossary-standard-terms-portable` (`Origin: ADR-0198`, `Backing: test`): every shipped standard
  term carries exactly a string term and a string meaning, with no domains key, no ADR reference,
  and no meaning exceeding the terseness threshold, so the shipped layer is portable into any
  adopter tree.

- [ ] **Task 3.9: Record the third Applied batch.** Append to ADR-0198's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: add `rendering/guide-and-doc-templates:glossary-standard-vocabulary`, add `rendering/guide-and-doc-templates:glossary-standard-terms-portable`, update `config/configspec-and-reference:configspec-data-parity`
  ```

- [ ] **Task 3.10: Add the feature changelog entry.** Under `## [Unreleased]` then `### Features` in
  `changelog/CHANGELOG.md`, add an entry stating that the glossary now renders a shipped awf
  vocabulary merged with the project's own terms, that a project term of the same name overrides the
  shipped one, and that the shipped layer is not disableable.

- [ ] **Task 3.11: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  `awf check: clean`, and `git diff docs/glossary.md examples/sundial/docs/glossary.md` shows the
  shipped vocabulary appearing in both, proving the layer reaches a real adopter. `./x check` must
  report no advisory note from `examples/sundial`: the runner fails the check on any, and the
  shipped layer now merges into that example's glossary.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
feat(rendering): ship a standard glossary vocabulary to adopters
```

## Phase 4: Add the terseness advisory and rewrite the corpus

**Execution mode: inline.** One independently green transaction. Depends on phase 3's threshold
constant and merged transform.

- [ ] **Task 4.1: Add the terseness advisory family.** In `internal/project/check.go`, add a
  glossary terseness note producer to `AdvisoryNotes`, alongside the unset-var, stub, part-marker,
  tag-health, and plan-commit-scope families. It emits one sorted note per term whose merged-set
  meaning exceeds the phase 3 threshold, naming the term and its length, and it never affects the
  exit code. It evaluates the merged set, so a shipped term is in scope. Cover it in
  `internal/project/notes_test.go`, keyed the way the existing families are.

- [ ] **Task 4.2: Add the two advisory claims.** In
  `.awf/topics/parts/rendering/doc-outputs/current-state.md`, add `glossary-terseness-advisory`
  (`Origin: ADR-0198`, `Backing: test`): the advisory reports one note per glossary term whose
  meaning exceeds the threshold, over the merged shipped-and-authored set.

  In `.awf/topics/parts/tooling/cli/current-state.md`, add `terseness-advisory-nonfailing`
  (`Origin: ADR-0198`, `Backing: test`), worded to match its siblings
  `completeness-advisory-nonfailing` and `stub-advisory-nonfailing`: the glossary terseness notes
  `awf check` prints are informational only and never change the command's exit code.

  Place proof markers for both on the task 4.1 tests.

- [ ] **Task 4.3: Add the glossary rule to the documentation standard.** In
  `templates/docs/doc-standard.md.tmpl`, add a rule to the `rules` section, immediately after the
  existing **Terse.** bullet it refines:

  ```
  - **Glossary entries are terser still.** One sentence stating what the thing is; a second only when a contrast or boundary is load-bearing. Do not restate what the term's own words already say. An over-long entry raises a non-failing advisory.
  ```

  This is shipped prose: no ADR citation, no repo identity, plain punctuation.

- [ ] **Task 4.4: Carry the same guidance to the authoring surface.** In
  `internal/configspec/spec.go`, extend the `{Kind: "docs", Artifact: "glossary", Key: "terms"}`
  Description with one clause stating that a meaning longer than the threshold raises a non-failing
  advisory, so an author meets the rule where they write.

- [ ] **Task 4.5: Rewrite the corpus.** In `.awf/docs/glossary.yaml`, bring every meaning under the
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
  because it is long, and padding an entry to look uniform. Deterministic post-check:
  `./x check 2>&1 | grep 'glossary'` returns no output, meaning the advisory reports nothing.

- [ ] **Task 4.6: Record the final Applied batch.** Append to ADR-0198's `## Status history`:

  ```
  - 2026-07-31: Applied; operations: add `rendering/doc-outputs:glossary-terseness-advisory`, add `tooling/cli:terseness-advisory-nonfailing`
  ```

  Leave the status at `Implementing`. The flip to `Implemented` belongs to the terminal-review
  transaction, not to this phase.

- [ ] **Task 4.7: Add the advisory changelog entry.** Under `## [Unreleased]` then `### Features` in
  `changelog/CHANGELOG.md`, add an entry stating that `awf check` now reports a non-failing advisory
  for a glossary meaning longer than the threshold, and that the rule is stated in the documentation
  standard.

- [ ] **Task 4.8: Re-render and verify.** Run `./x render && ./x check`. Expected terminal state:
  `awf check: clean` with no glossary advisory note.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the
  one phase-closing commit; it requires `awf check --staged` and `./x gate` to pass, enforced by a
  wired pre-commit hook or run manually first in a clone without one (checkable with
  `git config core.hooksPath`):

```commit
feat(rendering): add the terseness advisory and clean the glossary
```

## Verification

- `./x render && ./x check` reports `awf check: clean` with no glossary advisory note.
- `./x gate` passes, including the 100% statement coverage floor and the dead-code gate.
- `./awf check invariants` resolves every ADR-0198 claim to its proof marker.
- `git diff --stat` over `examples/sundial/docs/glossary.md` between the phase 2 and phase 3 commits
  shows the shipped vocabulary arriving in the example adopter, which is the end-to-end proof that
  an adopter receives it.
- `docs/config-reference.md` documents `terms` in its record shape and does not list
  `standardTerms` as an adopter-settable key.
- ADR-0198 carries four Applied events whose operations union to exactly its nine declared
  operations, with no operation applied twice.

## Notes

- Contextual surfacing of glossary terms is out of scope by ADR-0198. The `domains` assigned in
  task 4.5 are the data that decision will consume; nothing reads them beyond the task 2.3 drift
  check until then. That later decision should carry pitfall entries too, whose surfacing is
  equally absent today.
- A term-lookup command (`awf term`) stays unjustified until the surfacing work shows whether
  discovery is still a problem.
- The corpus rewrite in task 4.5 is the largest single work item in this plan and is judgement work,
  not mechanical: it is deliberately separated from the phase 2 encoding change so each is
  reviewable on its own terms.
