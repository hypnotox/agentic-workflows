---
date: 2026-07-18
adrs: [128, 129, 130]
status: Proposed
---
# Plan: Coverage-Derived Supersession Over One Parsed Corpus

## Goal

Implement ADR-0128 (supersession encoding), ADR-0129 (anchor-coverage model), and ADR-0130 (one
parsed corpus view) as a single ordered effort: delete the `supersedes:`/`superseded_by:`
frontmatter keys, split the inline token into a retirement and a refinement relation, derive full
supersession from anchor coverage, and route every ADR consumer through one corpus view built from
one parse. Non-goals: no change to what the workflow chain asks an author to do beyond the new
refinement relation, and no re-argument of the design, which lives in the three ADRs.

## Architecture summary

Eight phases, bottom-up. The corpus view and its status predicates land first (phases 1 to 3) so
the supersession work has a structure to hang on rather than threading a fifth one by hand. The
refinement grammar and the coverage model follow (phases 4 and 5) while the old frontmatter
encoding is still present and every check still passes. Phase 6 is a single coupled commit: the
schema removal, the coverage checks, the generation-12 migration, and this repo's own corpus
retrofit cannot be sliced, because the moment the keys leave the schema every ADR carrying them
fails `supersession-keys-refused` until the migration has rewritten the corpus. Phases 7 and 8 land
the rendering changes and the documentation, and flip all three ADRs plus this plan to
`Implemented`.

Phase 6's shared commit is the exception the plan convention allows, and its size is why phases 1
to 5 are sliced as finely as they are: everything that can pass the gate independently does so
before the coupled step.

## File structure

- **Created:** `internal/adr/status.go`, `internal/adr/corpus.go`, `internal/adr/coverage.go`,
  `internal/adr/corpus_test.go`, `internal/adr/coverage_test.go`,
  `internal/migrate/supersessionkeys.go`, `internal/migrate/supersessionkeys_test.go`
- **Modified:** `internal/adr/adr.go`, `internal/adr/adr_test.go`, `internal/adr/domain.go`,
  `internal/project/check.go`, `internal/project/supersession.go`,
  `internal/project/supersession_test.go`, `internal/project/context.go`,
  `internal/project/render.go`, `internal/project/project.go`,
  `internal/invariants/invariants.go`, `internal/audit/audit.go`, `internal/migrate/migrate.go`,
  `.awf/parts/adr-template/frontmatter.md`, `templates/adr-template/template.md.tmpl`,
  `templates/adr-readme/README.md.tmpl`, `templates/skills/adr-lifecycle/SKILL.md.tmpl`,
  `.awf/domains/parts/adr-system/current-state.md`, `docs/glossary.md`, `docs/pitfalls.md`,
  every file under `docs/decisions/`, `examples/sundial/docs/decisions/template.md`
- **Deleted:** `SupersessionIndex`, `Override`, and `Override.Label` from `internal/adr/adr.go`;
  `statusOf` and `domainsOf` from `internal/audit/audit.go`

## Phase 1: Status predicates on the ADR value

- [ ] **Task 1.1: Add `internal/adr/status.go` with the full status vocabulary.** Per ADR-0130
  item 3 the predicates hang off the `ADR` value, not the corpus, so a consumer holding a single
  parsed record can use them. Create the file with exactly:

  ```go
  package adr

  import "strings"

  // Status literals live here and nowhere else (ADR-0130 item 3). Every consumer
  // asks a predicate rather than comparing a string, which is what stops the
  // three-way "is live" and five-way "is superseded" divergences from recurring.
  const (
  	statusAccepted    = "Accepted"
  	statusImplemented = "Implemented"
  	statusProposed    = "Proposed"
  	statusSuperseded  = "Superseded"
  )

  // IsLive reports whether the ADR's decisions are current guidance.
  func (a ADR) IsLive() bool {
  	return a.Status == statusAccepted || a.Status == statusImplemented
  }

  // IsSuperseded reports whether the ADR has been retired. The prefix test
  // tolerates the pre-generation-12 suffixed form as well as the bare status
  // ADR-0128 item 4 moves to.
  func (a ADR) IsSuperseded() bool { return strings.HasPrefix(a.Status, statusSuperseded) }

  // IsImplemented reports whether the ADR's decisions have shipped. Invariant
  // backing and token retirement are both gated on this.
  func (a ADR) IsImplemented() bool { return a.Status == statusImplemented }

  // IsProposed reports whether the ADR's body is still mutable.
  func (a ADR) IsProposed() bool { return a.Status == statusProposed }

  // HasStatus reports whether the record carried a status at all. Audit needs the
  // distinction between absent frontmatter, legitimate on an old commit, and a
  // real status; see ADR-0130 item 3.
  func (a ADR) HasStatus() bool { return a.Status != "" }

  // Bucket is the ACTIVE.md section an ADR belongs to. Every superseded ADR folds
  // into one group regardless of the successor its status names.
  func (a ADR) Bucket() string {
  	if a.IsSuperseded() {
  		return statusSuperseded
  	}
  	return a.Status
  }
  ```

- [ ] **Task 1.2: Re-point every status literal comparison at the predicates (batch).**

  *Representative site*, `internal/project/supersession.go:169`:

  ```diff
  -		live := a.Status == "Accepted" || a.Status == "Implemented"
  +		live := a.IsLive()
  ```

  *Edge site*, `internal/adr/adr.go:27-32`, where `bucketKey` becomes a seam over the new
  predicate rather than carrying the rule itself:

  ```diff
  -func bucketKey(status string) string {
  -	if strings.HasPrefix(status, "Superseded") {
  -		return "Superseded"
  -	}
  -	return status
  -}
  +// bucketKey is retained as the grouping seam; the rule itself is ADR.Bucket.
  +func bucketKey(a ADR) string { return a.Bucket() }
  ```

  Update `groupByStatus` (`internal/adr/adr.go:293-320`) to pass the ADR rather than its status.

  *Affected-site set*, exactly the eleven sites this command lists:

  ```
  grep -rn 'Status == "Accepted"\|Status == "Implemented"\|Status != "Implemented"\|Status == "Proposed"\|HasPrefix(.*Status, "Superseded")' --include='*.go' internal/ | grep -v _test.go
  ```

  Expected today: `adr.go:28`, `adr.go:128`, `adr.go:133`, `context.go:189`,
  `supersession.go:155`, `supersession.go:169`, `supersession.go:194`, `supersession.go:207`,
  `supersession.go:212`, `invariants.go:135`, `invariants.go:175`.

  Two exclusions. Leave `context.go:189` on its own prefix test: ADR-0129 item 4 enumerates it as
  the one consumer that keeps one. Leave `internal/audit`'s three sites (`audit.go:218`, `:318`,
  `:321`) alone in this phase; they move in Phase 3, when the bytes seam gives audit parsed records
  to call predicates on.

  *Post-check*, after the batch only `context.go:189` remains outside `internal/audit`:

  ```
  grep -rn 'Status == "Accepted"\|Status == "Implemented"\|Status != "Implemented"\|Status == "Proposed"\|HasPrefix(.*Status, "Superseded")' --include='*.go' internal/ | grep -v _test.go | grep -v internal/audit | wc -l
  ```

  Expected output: `1`

- [ ] **Task 1.3: Back `corpus-owns-status-literals` with a source-scan test.** Add to
  `internal/adr/adr_test.go` a test walking `internal/`, skipping `internal/adr`, `_test.go` files,
  and `internal/project/context.go`'s Tier-2 line, that fails on any ADR-status literal comparison.
  Mark it `// invariant: corpus-owns-status-literals`. The test exempts `internal/audit` with a
  comment naming Phase 3 as where that exemption is removed.

- [ ] **Task 1.4: Verify and commit.** `./x gate` must end `prose-gate: clean` with
  `coverage: 100.0%`. Stage `internal/adr/status.go`, `internal/adr/adr.go`,
  `internal/adr/adr_test.go`, `internal/project/supersession.go`,
  `internal/invariants/invariants.go`.

  ```commit
  refactor(adr-system): name every ADR status predicate once
  ```

## Phase 2: The corpus view and one parse

- [ ] **Task 2.1: Add `internal/adr/corpus.go`.** The view holds the parsed slice, a number-keyed
  index, and the existence set the duplicated `known[a.Number]` builds recompute. Exported surface
  for this phase: `NewCorpus([]ADR) Corpus`, `Corpus.All() []ADR`,
  `Corpus.ByNumber(string) (ADR, bool)`, `Corpus.Has(string) bool`, `Corpus.Live() []ADR`,
  `Corpus.DecisionItems(string) []int`, `Corpus.DeclaredSlugs(string) []string`,
  `Corpus.Claims(target string) []SupersessionRef`, and `Corpus.Raw(string) ([]byte, error)` for
  the two enumerated raw consumers (ADR-0130 item 6). Every method needs a production caller by the
  end of Task 2.3 or the dead-code gate fails.

- [ ] **Task 2.2: Thread the corpus through `*Project` and collapse the load sites (batch).**

  *Representative site*, `internal/project/check.go:615`:

  ```diff
  -	adrs, err := adr.ParseDir(p.adrDir())
  -	if err != nil {
  -		return nil, err
  -	}
  -	known := map[string]bool{}
  -	for _, a := range adrs {
  -		known[a.Number] = true
  -	}
  +	corpus, err := p.Corpus()
  +	if err != nil {
  +		return nil, err
  +	}
  ```

  with each `known[n]` test becoming `corpus.Has(n)`.

  *Edge site*, `internal/project/render.go:764`, where the per-domain loop re-parses once per
  configured domain: hoist the corpus above the loop and pass it into `adr.RenderDomainIndex`,
  whose signature changes to take a `Corpus`. `adr.RenderActiveMD` changes the same way.

  *Affected-site set*, the ten production `ParseDir` callers:

  ```
  grep -rn 'adr\.ParseDir\|func ParseDir\|ParseDir(' --include='*.go' internal/ cmd/ | grep -v _test.go
  ```

  *Post-check*, exactly one production call site outside `internal/adr`:

  ```
  grep -rn 'adr\.ParseDir' --include='*.go' internal/ cmd/ | grep -v _test.go | wc -l
  ```

  Expected output: `1`

- [ ] **Task 2.3: Move `ADR.Refs` and `ADR.Sections` reads behind the view.** `invariants.go:138`
  and `:197` read `Sections["Invariants"]`; `invariants.go:178` and `supersession.go:171` read
  `Refs`. Route each through `Corpus.DeclaredSlugs` and `Corpus.Claims`.

- [ ] **Task 2.4: Back `corpus-parsed-once`, `corpus-owns-field-reads`, and
  `corpus-raw-access-enumerated`.** Three source-scan tests in `internal/adr/corpus_test.go`, each
  marked with its slug. The parse-once test asserts one `ParseDir` caller outside `internal/adr`
  and that neither `RenderDomainIndex` nor `RenderActiveMD` parses. The raw-access test asserts
  `Corpus.Raw` has exactly two call sites and that no file outside `internal/adr` calls
  `os.ReadFile` on an `ADR.Path`.

- [ ] **Task 2.5: Verify and commit.** `./x gate`, then stage the corpus file, its test, and every
  re-pointed consumer.

  ```commit
  refactor(adr-system): parse the ADR corpus once per invocation
  ```

## Phase 3: The bytes seam, audit, and one identity key

- [ ] **Task 3.1: Export a bytes-level parse entry point preserving the tri-state.** `parse`
  (`internal/adr/adr.go:208`) discards `frontmatter.Parse`'s `found` bool
  (`internal/frontmatter/frontmatter.go:31`). Export
  `ParseBytes(name string, data []byte) (ADR, bool, error)` returning it, so a record with absent
  frontmatter is distinguishable from a malformed one. `parse` becomes a thin caller.

- [ ] **Task 3.2: Re-point `internal/audit` and delete its parsers.** Replace `statusOf`
  (`audit.go:487-502`) and `domainsOf` (`audit.go:399-407`) with `adr.ParseBytes`. Preserve the
  contract `audit.go:192` depends on: empty text and absent frontmatter both yield a clean empty
  status while present-but-unparseable is a finding. Replace `audit.go:218`'s `st == ""` with
  `!rec.HasStatus()`, and `:318`/`:321` with `rec.IsImplemented()`. Remove the `internal/audit`
  exemption from the Task 1.3 test in the same task.

- [ ] **Task 3.3: Make the ADR number the sole identity.** `invariants.Decl.ADR`
  (`invariants.go:159`) holds `a.Filename`; change it to `a.Number` and delete the `byFile`
  translation map at `context.go:145-148`, re-pointing its uses at the number directly.

- [ ] **Task 3.4: Back `audit-shares-adr-parser` and `corpus-single-identity-key`.** One test
  asserting `internal/audit` declares no frontmatter struct of its own, and one asserting every
  `Decl.ADR` value over the real corpus matches `^[0-9]{4}$`.

- [ ] **Task 3.5: Verify and commit.** `./x gate`.

  ```commit
  refactor(adr-system): share one ADR parser and one identity key
  ```

## Phase 4: The refinement relation

- [ ] **Task 4.1: Parse refinement tokens.** Add to `internal/adr/adr.go` a `refinesItemRe`
  mirroring `itemRefRe`, and a `Relation` field on `SupersessionRef` distinguishing retirement from
  refinement. Both relations parse only from `Sections["Decision"]`, unchanged, so a token quoted
  elsewhere in an ADR stays inert.

- [ ] **Task 4.2: Render refinements.** ACTIVE.md's anchor annotations and `awf context`'s
  counterpart distinguish "superseded by" from "refined by". `active-md-annotates-superseded-anchors`
  and `context-annotates-superseded-anchors` keep their proof markers; extend both tests with a
  refinement case.

- [ ] **Task 4.3: Widen the back-pointer check to targets of any status.** Per ADR-0128 item 5,
  remove `supersession.go:206`'s live-target guard; both relations owe the back-pointer. Retire
  `supersession-backpointer`'s proof marker and add one for
  `supersession-backpointer-any-status`.

- [ ] **Task 4.4: Verify and commit.** `./x gate`, then `./x check`, which must stay
  `awf check: clean`: no corpus ADR currently tokens a superseded target, so widening the
  back-pointer adds no drift today.

  ```commit
  feat(adr-system): add the refinement relation for adapted anchors
  ```

## Phase 5: The anchor-coverage model

- [ ] **Task 5.1: Add `internal/adr/coverage.go`.** Per ADR-0129 items 2 and 3: anchors as nodes,
  claims as edges carrying relation, claiming ADR number, and the claiming ADR's Decision item
  number; derived `Live`/`Partial`/`Covered` with `Partial` as the residual by construction and a
  zero-anchor ADR `Live` rather than vacuously `Covered`. Constructed inside `NewCorpus`, so
  `corpus-model-not-rebuilt` holds by construction.

- [ ] **Task 5.2: Add the acyclicity and irreflexivity check.** Per ADR-0129 item 7: fail on a
  token whose target ADR is its own carrier, and on a cycle in the retirement relation restricted
  to ADRs the model classifies as `Covered`. It runs after state derivation, which is why it is a
  second pass. Mark `// invariant: supersession-graph-acyclic`.

- [ ] **Task 5.3: Re-point the consumers and delete the old index.** `bucketKey` buckets from
  derived state and `statusOrder` orders on the derived bucket; `awf context`'s annotation path
  (`context.go:133`, `:269`, `:282`) queries the model; `SupersessionIndex`, `Override`, and
  `Override.Label` are deleted.

  *Post-check*:

  ```
  grep -rn 'SupersessionIndex\|adr\.Override' --include='*.go' internal/ cmd/ | wc -l
  ```

  Expected output: `0`

- [ ] **Task 5.4: Back the model's four invariants.** `supersession-model-single-source` (the
  greppable identifier scan), `supersession-model-anchor-nodes`,
  `supersession-model-derives-state`, and `corpus-model-not-rebuilt`. The state test needs table
  cases for all three states including both residuals: anchors claimed only by refinements, and
  anchors claimed only by retirements on non-`Implemented` carriers.

- [ ] **Task 5.5: Verify and commit.** `./x gate`.

  ```commit
  feat(adr-system): derive supersession state from anchor coverage
  ```

## Phase 6 (coupled, single commit): schema removal, coverage checks, migration, retrofit

**Why this cannot be sliced.** Deleting `supersedes:`/`superseded_by:` from the schema makes
`supersession-keys-refused` fail on every ADR that still carries them, and the migration that
strips them is part of the same change. Landing the migration first leaves it unreachable and the
dead-code gate refuses it; landing the schema removal first leaves the tree red. This repo's corpus
retrofit is in the same commit for the same reason. Tasks 6.1 to 6.6 share one closing commit.

- [ ] **Task 6.1: Add the coverage-versus-status check.** Per ADR-0128 items 3 and 4: fail when a
  `Covered` ADR is not `Superseded`, naming the covering carriers, and when a `Superseded` ADR has
  an uncovered anchor, naming the anchor. Retire the proof markers for
  `supersession-full-symmetry`, `supersession-flavour-exclusive`, and
  `supersession-conflict-advisory`; add `supersession-coverage-derives-status`,
  `supersession-coverage-implemented-only`, `supersession-contested-anchor-advisory`, and
  `refines-token-never-covers`.

- [ ] **Task 6.2: Add `supersession-keys-refused` and delete the superseded checks.**
  `computeSupersession`'s forward and reverse symmetry halves (`supersession.go:138-166`) and
  `adr-token-exclusive` (`:174-177`) go. `adr-retired-key` (`:39-61`) stays and gains a sibling
  raw-frontmatter scan for the two removed keys.

- [ ] **Task 6.3: Remove the keys from the parser and every template.** Drop `SupersededBy` and
  `Supersedes` from `ADR` and `adrFrontmatter`; remove both lines from
  `.awf/parts/adr-template/frontmatter.md`, `templates/adr-template/template.md.tmpl`, and
  `examples/sundial/docs/decisions/template.md`.

- [ ] **Task 6.4: Add `internal/migrate/supersessionkeys.go` as generation 12.** Register
  `{To: 12, Name: "supersession-keys", Apply: applySupersessionKeys}` in
  `internal/migrate/migrate.go` after the existing `{To: 11, ...}` entry. The migration runs in
  this order: rewrite every pre-existing inline item token to the refinement relation against the
  pre-append body; strip both keys; then for each ADR that carried a non-empty `supersedes:`,
  append one bookkeeping Decision item carrying a retirement token per anchor of each named
  predecessor, insert the carrier's number into each target's `related:` when absent, and rewrite
  each predecessor's suffixed status to bare `Superseded`. The order is load-bearing: reversed, the
  rewrite would downgrade the retirement tokens the append had just written and deliver the three
  legacy pairs into a coverage failure. Idempotency rests on the generation gate. Mark
  `// invariant: upgrade-migrates-supersession-keys`.

- [ ] **Task 6.5: Run the migration over this repo's corpus and hand-correct.** Run
  `./x build && ./awf upgrade`. Then hand-correct ADR-0128's item-1 token back to the retirement
  relation: it is a genuine retirement, and the mechanical downgrade is deliberately conservative
  per ADR-0128 item 8. Confirm ADR-0129's `ADR-0120#10` token is now a refinement, which ADR-0129
  item 6 requires before that ADR reaches `Implemented`.

- [ ] **Task 6.6: Verify and commit.** `./x sync && ./x check` must report `awf check: clean`, and
  `./x gate` must pass. Confirm nothing derived `Superseded` unintentionally:

  ```
  grep -l '^status: Superseded' docs/decisions/*.md | wc -l
  ```

  Expected output: `3` (ADR-0003, ADR-0031, ADR-0113 only, matching the pre-migration full pairs).

  ```commit
  feat(adr-system): derive full supersession from anchor coverage
  ```

## Phase 7: Rendering

- [ ] **Task 7.1: Bound the domain-index partial annotation.** Per ADR-0129 item 5, a domain entry
  for an ADR with claimed anchors names the claiming ADR numbers and no individual anchor. Replace
  `domain.go:38-41`'s `SupersededBy` arrow, which no longer has a field to read. Mark
  `// invariant: domain-index-surfaces-partial`.

- [ ] **Task 7.2: Make ACTIVE.md chains one-to-many.** Per ADR-0129 item 6, render a `Covered` ADR
  against every ADR that retired one of its anchors. The `Superseded` bucket entry line stays a
  bare roster by design. Mark `// invariant: active-md-chains-one-to-many`.

- [ ] **Task 7.3: Verify and commit.** `./x sync` produces a large diff across
  `docs/decisions/ACTIVE.md` and every `docs/domains/*.md`; stage it alongside the code. `./x gate`.

  ```commit
  feat(rendering): surface partial supersession in every index
  ```

## Phase 8: Documentation and the status flips

- [ ] **Task 8.1: Update the authored prose.** The supersession section of
  `templates/adr-readme/README.md.tmpl` (two flavours become one relation plus refinement), the
  `supersedence-full` and `supersedence-partial` sections of
  `templates/skills/adr-lifecycle/SKILL.md.tmpl`, the back-pointer and supersession-token entries
  in `docs/glossary.md`, and `.awf/domains/parts/adr-system/current-state.md`, which still
  describes the scalar back-pointer, the three-way symmetry, and chains-as-pairs.

- [ ] **Task 8.2: Record the token-example pitfall.** Add to `docs/pitfalls.md` that a token quoted
  as an example inside an ADR's Decision section parses as a real claim and demands a back-pointer
  from the cited target; the grammar has no escape for examples. This surfaced while writing
  ADR-0128 and cost one `awf check` failure.

- [ ] **Task 8.3: Flip the statuses.** Set ADR-0128, ADR-0129, and ADR-0130 to `Implemented` and
  this plan to `Implemented`, in this commit. The three ADRs' retirement tokens take effect at this
  moment, so `./x check` here is the first run in which the retired ADR-0120 slugs stop being owed.

- [ ] **Task 8.4: Verify and commit.** `./x sync && ./x check && ./x gate full`.

  ```commit
  docs(adr): implement 0128, 0129, and 0130
  ```

## Verification

- `./x gate full` passes and `awf check` reports `clean` with no drift and no invariant issues.
- `grep -rn 'adr\.ParseDir' --include='*.go' internal/ cmd/ | grep -v _test.go | wc -l` returns `1`.
- `grep -rn 'SupersessionIndex\|adr\.Override' --include='*.go' internal/ cmd/ | wc -l` returns `0`.
- No file under `docs/decisions/` carries `supersedes:` or `superseded_by:` in frontmatter.
- Exactly three ADRs carry `status: Superseded`, matching the pre-migration full pairs.
- `./awf upgrade` on an already-migrated tree reports no migrations applied.
- Every invariant declared across ADR-0128, ADR-0129, and ADR-0130 has a proof marker, confirmed by
  `./x check` reporting `awf invariants: clean`.

## Notes

- Phase 6 is the only shared-commit group. If Phase 5's model turns out to need the schema gone
  before it can be built cleanly, merge Phase 5 into the Phase 6 group rather than splitting
  Phase 6 further.
- `internal/project/supersession.go` carries roughly 58 of the effort's ~90 field reads and is
  touched by all three ADRs. It is the riskiest single file; if a phase runs long it will be one
  that touches it.
- The generation is 11 to 12, not 10 to 11: ADR-0127's `drop-audit-base` took 10 to 11 on
  2026-07-18. Re-derive the head from `internal/migrate/migrate.go`'s registry before writing the
  migration rather than trusting this line.
- ADR-0130 items 4 and 5 (identity key, audit seam) are separable, as that ADR's Consequences
  records. If the effort needs to shrink, Phase 3 is the one to defer; nothing in Phases 4 to 8
  depends on it.
