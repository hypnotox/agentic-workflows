---
date: 2026-07-18
adrs: [128, 129, 130]
status: Implemented
---
# Plan: Coverage-Derived Supersession Over One Parsed Corpus

## Goal

Implement ADR-0128 (supersession encoding), ADR-0129 (anchor-coverage model), and ADR-0130 (one
parsed corpus view) as a single ordered effort: delete the `supersedes:`/`superseded_by:`
frontmatter keys, split the inline token into a retirement and a refinement relation, derive full
supersession from anchor coverage, and route every ADR consumer through one corpus view built from
one parse. The authoring surface does change: two frontmatter keys an author previously filled on
every new ADR disappear, the `Superseded` status becomes hand-authored in a bare form that
`awf check` enforces against derived coverage, and the `refines:` relation is new. Non-goal: no
re-argument of the design, which lives in the three ADRs.

## Architecture summary

Seven phases, bottom-up. The corpus view and its status predicates land first (phases 1 to 3) so
the supersession work has a structure to hang on rather than threading a fifth one by hand. The
refinement grammar and the coverage model follow (phases 4 and 5) while the old frontmatter
encoding is still present and every check still passes; the model reads the old `Supersedes` field
transitionally so ACTIVE.md's chain rendering never goes dark. Phase 6 is a single coupled commit:
the schema removal, the coverage checks, the generation-12 migration, this repo's corpus retrofit,
and both rendering changes all hinge on the frontmatter keys disappearing, and none of them
compiles without the others. Phase 7 lands the documentation and flips all three ADRs plus this
plan to `Implemented`.

Two rules govern the slicing. Every phase before 6 must pass `./x gate` alone, which means no
production function may be introduced before the phase that first calls it. And every
`supersedes-invariant:` retirement declared by these ADRs takes effect only when its carrier
reaches `Implemented` in Phase 7, so no proof marker for a retired slug may be removed before
that commit, however dead the code it marks becomes.

## File structure

- **Created:** `internal/adr/status.go`, `internal/adr/corpus.go`, `internal/adr/coverage.go`,
  `internal/adr/corpus_test.go`, `internal/adr/coverage_test.go`,
  `internal/migrate/supersessionkeys.go`, `internal/migrate/supersessionkeys_test.go`
- **Modified:** `internal/adr/adr.go`, `internal/adr/adr_test.go`, `internal/adr/domain.go`,
  `internal/adr/domain_test.go`, `internal/project/check.go`,
  `internal/project/supersession.go`, `internal/project/supersession_test.go`,
  `internal/project/context.go`, `internal/project/render.go`, `internal/project/project.go`,
  `internal/invariants/invariants.go`, `internal/audit/audit.go`, `internal/migrate/migrate.go`,
  `internal/migrate/retirementtokens.go`, `internal/testsupport/testsupport.go`,
  `internal/testsupport/testsupport_test.go`, `.awf/parts/adr-template/frontmatter.md`,
  `templates/adr-template/template.md.tmpl`, `templates/adr-readme/README.md.tmpl`,
  `templates/skills/adr-lifecycle/SKILL.md.tmpl`,
  `templates/skills/proposing-adr/SKILL.md.tmpl`, `.awf/agents/adr-reviewer.yaml`,
  `.awf/domains/parts/adr-system/current-state.md`,
  `.awf/domains/parts/invariants/current-state.md`,
  `.awf/docs/parts/architecture/components.md`, `.awf/docs/glossary.yaml`,
  `.awf/docs/pitfalls.yaml`, every file under `docs/decisions/`,
  `examples/sundial/docs/decisions/template.md`
- **Regenerated output committed alongside its source:** `docs/glossary.md`, `docs/pitfalls.md`,
  `docs/architecture.md`, `docs/domains/adr-system.md`, `docs/domains/invariants.md`,
  `docs/decisions/ACTIVE.md`, `docs/decisions/README.md`, the rendered skill and agent files, and
  the whole `examples/sundial/` rendered tree (its ADR README and template, plus the skill and
  agent renders under `.claude/`, `.agents/`, and `.github/`, once per enabled target). None of
  these is hand-edited; each carries the generated banner and is produced by `./x sync` from the
  sources above.
- **Migrated by `awf upgrade`:** every file under `docs/decisions/` and under
  `examples/sundial/docs/decisions/`.
- **Deleted:** `SupersessionIndex`, `Override`, and `Override.Label` from `internal/adr/adr.go`;
  `statusOf` and `domainsOf` from `internal/audit/audit.go`; `WithSupersededBy` and
  `WithSupersedes` from `internal/testsupport/testsupport.go`

## Phase 1: Status predicates on the ADR value

- [ ] **Task 1.1: Add `internal/adr/status.go`.** Per ADR-0130 item 3 the predicates hang off the
  `ADR` value, not the corpus, so a consumer holding a single parsed record can use them. Create
  the file with exactly:

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

  // Bucket is the ACTIVE.md section an ADR belongs to. Every superseded ADR folds
  // into one group regardless of the successor its status names.
  func (a ADR) Bucket() string {
  	if a.IsSuperseded() {
  		return statusSuperseded
  	}
  	return a.Status
  }
  ```

  `HasStatus` is deliberately absent: its only caller is `audit.go:218`, which lands in Phase 3,
  and the dead-code gate refuses a production method no `main` can reach. It is added in Task 3.2.

- [ ] **Task 1.2: Re-point every status literal comparison at the predicates (batch).**

  *Representative site*, `internal/project/supersession.go:169`:

  ```diff
  -		live := a.Status == "Accepted" || a.Status == "Implemented"
  +		live := a.IsLive()
  ```

  *Edge site*, `internal/adr/adr.go:27-32`. This one is not matched by the affected-site command
  below, because `bucketKey`'s parameter is a lowercase `status` string rather than an `ADR` field:

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

  *Affected-site set*, exactly the ten sites this command lists today:

  ```
  grep -rn 'Status == "Accepted"\|Status == "Implemented"\|Status != "Implemented"\|Status == "Proposed"\|HasPrefix(.*Status, "Superseded")' --include='*.go' internal/ | grep -v _test.go
  ```

  Expected today: `supersession.go:155`, `:169`, `:194`, `:207`, `:212`; `context.go:189`;
  `invariants.go:135`, `:175`; `adr.go:128`, `:133`. Plus `adr.go:28`, handled by the edge diff
  above and not matched by this pattern.

  Two exclusions. Leave `context.go:189` on its own prefix test: ADR-0129 item 4 enumerates it as
  the one consumer that keeps one. `internal/audit`'s literals (`audit.go:218`, `:318`, `:321`)
  compare a local `st` variable rather than an ADR field, so this pattern never reaches them; they
  move in Phase 3 when the bytes seam gives audit parsed records.

  *Post-check*, only `context.go:189` remains:

  ```
  grep -rn 'Status == "Accepted"\|Status == "Implemented"\|Status != "Implemented"\|Status == "Proposed"\|HasPrefix(.*Status, "Superseded")' --include='*.go' internal/ | grep -v _test.go | wc -l
  ```

  Expected output: `1`

- [ ] **Task 1.3: Back `corpus-owns-status-literals` with a source-scan test.** Add to
  `internal/adr/adr_test.go` a test walking `internal/`, skipping `internal/adr`, `_test.go` files,
  and `internal/project/context.go`'s Tier-2 line, that fails on any ADR-status literal comparison.
  Mark it `// invariant: corpus-owns-status-literals`. Scope it to ADR-field comparisons only, so
  `internal/audit`'s local-variable literals do not trip it before Phase 3 converts them; Task 3.2
  widens the scan to cover them.

- [ ] **Task 1.4: Verify and commit.** `./x gate` must end `prose-gate: clean` with
  `coverage: 100.0%`.

  ```commit
  refactor(adr-system): name every ADR status predicate once
  ```

## Phase 2: The corpus view and one parse

- [ ] **Task 2.1: Add `internal/adr/corpus.go`.** Exported surface for this phase, every method
  having a production caller by the end of Task 2.3: `NewCorpus([]ADR) Corpus`,
  `Corpus.All() []ADR`, `Corpus.ByNumber(string) (ADR, bool)`, `Corpus.Has(string) bool`,
  `Corpus.DecisionItems(string) []int`, `Corpus.DeclaredSlugs(string) []string`,
  `Corpus.Claims(target string) []SupersessionRef`, and `Corpus.Raw(string) ([]byte, error)`.

  `Corpus.Live()` is deliberately absent: Phase 1's liveness sites are per-ADR `a.IsLive()` calls,
  not list queries, so nothing in Phase 2 would call it and the dead-code gate would refuse it. It
  is added in Phase 5, where the coverage model needs it.

- [ ] **Task 2.2: Thread the corpus through `*Project` and collapse the load sites (batch).**

  *Representative site*, `internal/project/check.go:615`:

  ```diff
  -	adrs, err := adr.ParseDir(p.decisionsDir())
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

  *Affected-site set*, exactly these ten production `adr.ParseDir` call sites:
  `supersession.go:23`, `:69`; `context.go:126`; `check.go:81`, `:615`, `:702`, `:754`, `:799`;
  `invariants.go:242`; `migrate/retirementtokens.go:51`. Confirm with:

  ```
  grep -rn 'adr\.ParseDir(' --include='*.go' internal/ cmd/ | grep -v _test.go
  ```

  That command returns exactly those ten lines today. An earlier draft of this plan claimed twelve,
  counting `check.go:454` and `:459`; those `coverage-ignore` comments read `adr.ParseDir here`
  with no opening paren, so the pattern never matched them. Leave both comments alone regardless.

  Do not re-point `internal/adr/adr.go:427` (`NextNumber`): it runs on the `awf new adr` path,
  which holds no corpus, and Task 2.4's test enumerates it as the one in-package exception.

  `internal/migrate` cannot take a threaded view: migrations resolve their own decisions directory
  and run before a `Project` can be opened. Both it and `*Project.Corpus` construct through
  `adr.LoadCorpus`, leaving `adr.ParseDir` with no caller outside `internal/adr`. ADR-0130's
  `corpus-parsed-once` was amended in place (it was still `Proposed`) to state that stronger rule.

  *Post-check*, no production call site outside `internal/adr`:

  ```
  grep -rn 'adr\.ParseDir(' --include='*.go' internal/ cmd/ | grep -v _test.go | wc -l
  ```

  Expected output: `0`

- [ ] **Task 2.3: Move `ADR.Refs` and `ADR.Sections` reads behind the view, including
  `internal/migrate`.** `invariants.go:138` and `:197` read `Sections["Invariants"]`;
  `invariants.go:178` and `supersession.go:171` read `Refs`. Route each through
  `Corpus.DeclaredSlugs` and `Corpus.Claims`. Route `migrate/retirementtokens.go:51`'s `ParseDir`
  through the corpus and its `os.ReadFile` at `:71` through `Corpus.Raw`, so both enumerated raw
  consumers (ADR-0130 item 6) go through the accessor from this phase on.

- [ ] **Task 2.4: Back `corpus-parsed-once`, `corpus-owns-field-reads`, and
  `corpus-raw-access-enumerated`.** Three source-scan tests in `internal/adr/corpus_test.go`, each
  marked with its slug. The parse-once test asserts one real `ParseDir` caller outside
  `internal/adr` (ignoring comment lines), that neither `RenderDomainIndex` nor `RenderActiveMD`
  parses, and that `NextNumber` is the sole in-package exception. The raw-access test asserts
  `Corpus.Raw` has exactly two call sites, `internal/migrate/retirementtokens.go` and
  `internal/project/supersession.go`, and that no file outside `internal/adr` calls `os.ReadFile`
  on an `ADR.Path`.

- [ ] **Task 2.5: Verify and commit.** `./x gate`.

  ```commit
  refactor(adr-system): parse the ADR corpus once per invocation
  ```

## Phase 3: The bytes seam, audit, and one identity key

- [ ] **Task 3.1: Export a bytes-level parse entry point preserving the tri-state.** `parse`
  (`internal/adr/adr.go:208`) discards `frontmatter.Parse`'s `found` bool
  (`internal/frontmatter/frontmatter.go:31`). Export
  `ParseBytes(name string, data []byte) (ADR, bool, error)`, returning that bool so a record with
  absent frontmatter is distinguishable from a malformed one. `name` is the ADR's base filename:
  `ParseBytes` populates `Filename` from it and derives `Number` via `FilenameRe`, and leaves
  `Path` empty, since a blob-sourced record has no working-tree path. `parse` becomes a thin
  caller.

- [ ] **Task 3.2: Re-point `internal/audit`, delete its parsers, and add `HasStatus`.** Add
  `func (a ADR) HasStatus() bool { return a.Status != "" }` to `internal/adr/status.go` in this
  task, where its first caller lands. Replace `statusOf` (`audit.go:487-502`) and `domainsOf`
  (`audit.go:399-407`) with `adr.ParseBytes`. Preserve the contract `audit.go:192` depends on:
  empty text and absent frontmatter both yield a clean empty status while present-but-unparseable
  is a finding. Replace `audit.go:218`'s `st == ""` with `!rec.HasStatus()`, and `:318`/`:321`
  with `rec.IsImplemented()`. Widen the Task 1.3 scan to cover local-variable status literals in
  the same task.

- [ ] **Task 3.3: Make the ADR number the sole identity.** `invariants.Decl.ADR`
  (`invariants.go:159`) holds `a.Filename`; change it to `a.Number` and delete the `byFile`
  translation map at `context.go:145-148`, re-pointing its uses at the number.

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
  counterpart distinguish "superseded by" from "refined by", which is what makes `Relation`
  reachable in this phase. Extend both annotation tests with a refinement case.

  `context-annotates-superseded-anchors` is ADR-0120's, is not retired, and keeps its existing
  marker. `active-md-annotates-superseded-anchors` is **new** in ADR-0128, re-declared as the
  surviving half of the retired `active-md-supersedence-rendering`, and has no marker anywhere in
  the tree: add `// invariant: active-md-annotates-superseded-anchors` to the RenderActiveMD
  annotation test in `internal/adr/adr_test.go`, alongside the `active-md-supersedence-rendering`
  marker that stays until Task 7.3. Without this the Phase 7 flip leaves ADR-0128 declaring a
  backed slug with no proof and `awf check` fails on the plan's own final commit.

- [ ] **Task 4.3: Widen the back-pointer check to targets of any status.** Per ADR-0128 item 5,
  remove `supersession.go:206`'s live-target guard; both relations owe the back-pointer. Add a
  proof marker for `supersession-backpointer-any-status`.

  Do **not** remove `supersession-backpointer`'s existing marker at
  `internal/project/supersession_test.go:344`. ADR-0128 retires that slug, but retirement applies
  only from an `Implemented` carrier (`internal/invariants/invariants.go:174-176`), and ADR-0128 is
  `Proposed` until Phase 7. Removing the marker here leaves the slug owed by the still-Implemented
  ADR-0120 and `awf check` reports it `Unbacked`, failing the gate. The marker is removed in
  Task 7.3.

- [ ] **Task 4.4: Verify and commit.** `./x gate`, then `./x check`, which must stay
  `awf check: clean`: no corpus ADR currently tokens a superseded target, so widening the
  back-pointer adds no drift today.

  ```commit
  feat(adr-system): add the refinement relation for adapted anchors
  ```

## Phase 5: The anchor-coverage model

- [ ] **Task 5.1: Add `internal/adr/coverage.go`, and `Corpus.Live()`.** Per ADR-0129 items 2
  and 3: anchors as nodes, claims as edges carrying relation, claiming ADR number, and the claiming
  ADR's Decision item number; derived `Live`/`Partial`/`Covered` with `Partial` as the residual by
  construction and a zero-anchor ADR `Live` rather than vacuously `Covered`. Constructed inside
  `NewCorpus`, so `corpus-model-not-rebuilt` holds by construction. Add `Corpus.Live()` here, where
  the derivation first needs it.

  **Transitional read.** Until Phase 6 deletes the field, the model also derives chains from the
  `Supersedes` frontmatter list, so ACTIVE.md's `### Chains` section keeps rendering the three
  legacy pairs across Phases 5 and 6 and `./x check` stays drift-free. Task 6.3 deletes that
  branch when it deletes the field. Mark the branch with a comment naming Task 6.3.

- [ ] **Task 5.2: Add the acyclicity and irreflexivity check.** Per ADR-0129 item 7: fail on a
  token whose target ADR is its own carrier, and on a cycle in the retirement relation restricted
  to ADRs the model classifies as `Covered`. It runs after state derivation, which is why it is a
  second pass. Mark `// invariant: supersession-graph-acyclic`.

- [ ] **Task 5.3: Re-point the consumers and delete the old index.** `bucketKey` buckets from
  derived state and `statusOrder` orders on the derived bucket; `awf context`'s annotation path
  (`context.go:133`, `:269`, `:282`) queries the model; `SupersessionIndex`, `Override`, and
  `Override.Label` are deleted, their ACTIVE.md use replaced by the model's chain query from
  Task 5.1.

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

## Phase 6 (coupled, single commit): schema removal, checks, migration, retrofit, rendering

**Why this cannot be sliced.** Registering `{To: 12, ...}` makes `migrate.Current()`
(`internal/migrate/migrate.go:46`) report 12 while this repo's lock says 11, and the
binary-version gate (`cmd/awf/gate.go:43`) then refuses every gated command until `awf upgrade`
runs. But the upgrade strips the very keys `computeSupersession`'s reverse-symmetry half
(`supersession.go:155-157`) still reads, so the checks must go in the same change. Deleting the
`SupersededBy` field likewise breaks `domain.go:38-39` and `internal/testsupport`, so the domain
rendering and the test helpers come too. Tasks 6.1 to 6.8 share one closing commit.

- [ ] **Task 6.1: Add the coverage-versus-status check.** Per ADR-0128 items 3 and 4: fail when a
  `Covered` ADR is not `Superseded`, naming the covering carriers, and when a `Superseded` ADR has
  an uncovered anchor, naming the anchor. Add proof markers for
  `supersession-coverage-derives-status`, `supersession-coverage-implemented-only`,
  `supersession-contested-anchor-advisory`, and `refines-token-never-covers`. Leave the existing
  markers for `supersession-full-symmetry` (`supersession_test.go:191`),
  `supersession-flavour-exclusive` (`:372`), and `supersession-conflict-advisory` (`:389`) in
  place; they are removed in Task 7.3 for the reason Task 4.3 records.

- [ ] **Task 6.2: Add `supersession-keys-refused` and delete the superseded checks.** Delete
  `computeSupersession`'s `claimants` map build (`supersession.go:122-129`, which reads
  `a.Supersedes` and feeds only the reverse-symmetry half), both symmetry halves (`:138-166`), and
  `adr-token-exclusive` (`:174-177`). `adr-retired-key` (`:39-61`) stays and gains a sibling
  raw-frontmatter scan for the two removed keys. ADR-0128 item 1 and
  `supersession-keys-refused` both require the failure to carry upgrade guidance, so the new
  finding's message names `awf upgrade` as the remedy exactly as `adr-retired-key`'s does, and the
  proof test asserts that guidance text is present.

- [ ] **Task 6.3: Remove the keys from the parser, the model, the templates, and testsupport.**
  Drop `SupersededBy` and `Supersedes` from `ADR` and `adrFrontmatter`, and delete Task 5.1's
  transitional chain branch. Remove both lines from `.awf/parts/adr-template/frontmatter.md`,
  `templates/adr-template/template.md.tmpl`, and `examples/sundial/docs/decisions/template.md`.
  Delete `WithSupersededBy` and `WithSupersedes` from `internal/testsupport/testsupport.go:101-110`
  and update their callers in `internal/project/supersession_test.go`,
  `internal/adr/domain_test.go`, `internal/adr/adr_test.go`, and
  `internal/testsupport/testsupport_test.go`.

- [ ] **Task 6.4: Rewrite the domain index and the ACTIVE.md chains.** Per ADR-0129 item 5, a
  domain entry for an ADR with claimed anchors names the claiming ADR numbers and no individual
  anchor, replacing `domain.go:38-41`'s `SupersededBy` arrow, which no longer has a field to read.
  Per ADR-0129 item 6, render a `Covered` ADR against every ADR that retired one of its anchors;
  the `Superseded` bucket entry line stays a bare roster by design. Mark
  `// invariant: domain-index-surfaces-partial` and `// invariant: active-md-chains-one-to-many`.

  In the same task, rewrite the hand-authored `.awf/domains/parts/adr-system/current-state.md`,
  which still describes the scalar back-pointer, the three-way symmetry, and chains-as-pairs.
  ADR-0129 item 5 makes this a *same-commit* obligation with the domain-index rewrite, not a
  later documentation pass, so it belongs here rather than in Phase 7. Rewrite
  `.awf/domains/parts/invariants/current-state.md` too: ADR-0128 declares the `invariants` domain,
  and that part still says "fully superseding a token-carrier lapses its retirements", which the
  coverage model replaces, and ends its migration history at generation 10.

- [ ] **Task 6.5: Add `internal/migrate/supersessionkeys.go` as generation 12.** Register
  `{To: 12, Name: "supersession-keys", Apply: applySupersessionKeys}` in
  `internal/migrate/migrate.go` after the `{To: 11, ...}` entry. The migration runs in this order:
  rewrite every pre-existing inline item token to the refinement relation against the pre-append
  body; strip both keys; then for each ADR that carried a non-empty `supersedes:`, append one
  bookkeeping Decision item carrying a retirement token per anchor of each named predecessor,
  insert the carrier's number into each target's `related:` when absent, and rewrite each
  predecessor's suffixed status to bare `Superseded`. The order is load-bearing: reversed, the
  rewrite would downgrade the retirement tokens the append had just written and deliver the three
  legacy pairs into a coverage failure. Idempotency rests on the generation gate. Mark
  `// invariant: upgrade-migrates-supersession-keys`.

  The `related:` insertion is genuinely needed: ADR-0003's `related:` is `[2, 30]`, ADR-0031's is
  `[8]`, and ADR-0113's is `[82, 112]`, none of which names its claimant.

- [ ] **Task 6.6: Run the migration over both corpora.** Run `./x build && ./awf upgrade`, then
  `(cd examples/sundial && ../../awf upgrade)`. The committed example adopter is not optional
  here: `x:57` runs `awf check` inside `examples/sundial` as part of `./x check`, and all three of
  its ADRs carry `supersedes: []` / `superseded_by: ""`, so Task 6.2's `supersession-keys-refused`
  fails Task 6.8's own verify step until sundial is migrated too.

- [ ] **Task 6.7: Hand-correct ADR-0128's retirement token.** In
  `docs/decisions/0128-coverage-derived-adr-supersession.md`, Decision item 1, the migration will
  have rewritten the inline token `supersedes: ADR-0120#3` to the refinement form. Change it back
  to the retirement form: it is a genuine retirement, and the mechanical downgrade is deliberately
  conservative per ADR-0128 item 8. Leave both `supersedes-invariant:` slug tokens in that same
  item untouched; the migration does not rewrite slug tokens. Confirm that ADR-0128's tokens on
  `ADR-0120#4` and `ADR-0120#5`, and ADR-0129's token on `ADR-0120#10`, all remain in the
  refinement form, which is what ADR-0128 lines 94-99 and ADR-0129 item 6 require.

- [ ] **Task 6.8: Verify and commit.** `./x sync && ./x check` must report `awf check: clean`, and
  `./x gate` must pass. Stage the regenerated `docs/domains/adr-system.md`,
  `docs/domains/invariants.md`, `docs/decisions/ACTIVE.md`, every migrated file under
  `docs/decisions/`, and the migrated `examples/sundial/docs/decisions/` tree alongside their
  sources; the repo forbids `git add -A`, so each path is staged explicitly.

  Confirm the adopter tree migrated:

  ```
  grep -rn '^supersedes:\|^superseded_by:' examples/sundial/docs/decisions/
  ```

  Expected output: none.

  Confirm the status rewrite happened and nothing derived `Superseded` unintentionally:

  ```
  grep -c '^status: Superseded$' docs/decisions/*.md | grep -v ':0' | wc -l
  ```

  Expected output: `3` (ADR-0003, ADR-0031, ADR-0113 only). And:

  ```
  grep -l '^status: Superseded by' docs/decisions/*.md | wc -l
  ```

  Expected output: `0`

  ```commit
  feat(adr-system): derive full supersession from anchor coverage
  ```

## Phase 7: Documentation and the status flips

- [ ] **Task 7.1: Update the authored prose, at its sources.** Every target below is a config-tree
  source, never a rendered file: `docs/glossary.md`, `docs/pitfalls.md`, and `docs/architecture.md`
  all carry the `GENERATED by awf: do not edit` banner, and hand-editing one is drift that
  `./x check` reverts and reports.

  - `templates/adr-readme/README.md.tmpl`: the supersession section, where two flavours become one
    relation plus refinement.
  - `templates/skills/adr-lifecycle/SKILL.md.tmpl`: the `supersedence-full` and
    `supersedence-partial` sections, and also the `transitions` section (line 29) and
    `procedure-predecessor-flip` (line 71), both of which still instruct the author to write the
    suffixed `Superseded by ADR-NNNN` form that ADR-0128 item 4 retires. Replace it with the bare
    `Superseded` status and item 4's trigger: the commit that brings the final covering carrier to
    `Implemented`.
  - `templates/skills/proposing-adr/SKILL.md.tmpl`: line 31 lists `supersedes` and `superseded_by`
    as **required** frontmatter and line 49 tells the author to fill `supersedes`. After Phase 6
    those are keys `awf check` refuses, so the chain's own authoring skill would be instructing
    every author to write a failing ADR. Lines 33 (`Predecessor flip`) and 57 (procedure step 4)
    likewise still say to write `status: Superseded by ADR-NNNN` "if fully superseding an earlier
    ADR"; rewrite both to the bare `Superseded` status and ADR-0128 item 4's coverage-completion
    trigger. Also update its partial-supersedence description, which predates the
    retirement/refinement split.
  - `.awf/agents/adr-reviewer.yaml`: its lens checks only for a `supersedes:`/`supersedes-invariant:`
    token on a partial override "of a live ADR". Teach it both relations, drop the live-target
    restriction per ADR-0128 item 5, and add the rationale-site check ADR-0128 item 9 delegates to
    this reviewer as the whole compensating control for what `awf check` cannot prove.
  - `.awf/docs/glossary.yaml`: the back-pointer and supersession-token entries.
  - `.awf/docs/parts/architecture/components.md`: the migrate package's generation history stops at
    generation 10; add generation 12, and describe `internal/adr`'s new exported surface (the
    `Corpus` view, its anchor-coverage facet, and the bytes-level parse seam), which ADR-0130's
    Consequences call out as net public API growth.

- [ ] **Task 7.2: Record the token-example pitfall at its source.** Add to `.awf/docs/pitfalls.yaml`
  (not the generated `docs/pitfalls.md`) that a token quoted as an example inside an ADR's Decision
  section parses as a real claim and demands a back-pointer from the cited target; the grammar has
  no escape for examples. This surfaced while writing ADR-0128 and cost one `awf check` failure.

- [ ] **Task 7.3: Flip the statuses and remove every retired proof marker, in one commit.** Set
  ADR-0128, ADR-0129, and ADR-0130 to `Implemented` and this plan to `Implemented`. In the same
  commit remove the proof markers for the five slugs these ADRs retire, which take effect only now
  that their carriers are `Implemented`:

  - `supersession-full-symmetry`, `internal/project/supersession_test.go:191`
  - `supersession-backpointer`, `internal/project/supersession_test.go:344`
  - `supersession-flavour-exclusive`, `internal/project/supersession_test.go:372`
  - `supersession-conflict-advisory`, `internal/project/supersession_test.go:389`
  - `active-md-supersedence-rendering`, `internal/adr/adr_test.go:144`, and its
    `touches-invariant:` marker at `internal/adr/adr.go:260`

  Removing any of these earlier fails the gate; leaving any of them now makes it a dangling marker
  naming a slug no Implemented ADR declares.

- [ ] **Task 7.4: Verify and commit.** `./x sync && ./x check && ./x gate full`. The sync
  regenerates `docs/glossary.md`, `docs/pitfalls.md`, `docs/architecture.md`, and the rendered
  skill and agent files from the sources Task 7.1 edited; stage all of them with their sources.

  ```commit
  docs(adr): implement 0128, 0129, and 0130
  ```

## Verification

- `./x gate full` passes and `awf check` reports `clean` with no drift and no invariant issues.
- `grep -rn 'adr\.ParseDir(' --include='*.go' internal/ cmd/ | grep -v _test.go | wc -l` returns
  `0`: every consumer constructs through `adr.LoadCorpus`.
- `grep -rn 'SupersessionIndex\|adr\.Override' --include='*.go' internal/ cmd/ | wc -l` returns `0`.
- No file under `docs/decisions/` carries `supersedes:` or `superseded_by:` in frontmatter.
- Exactly three ADRs carry the bare `status: Superseded`, and none carries the suffixed form.
- `./awf upgrade` on an already-migrated tree reports no migrations applied.
- All 21 invariants declared across the three ADRs have proof markers, and no marker names a
  retired slug, confirmed by `./x check` reporting `awf invariants: clean`.

## Notes

- **Landed early, in Phase 2 rather than Phase 3: Task 3.3 (the identity key).** `invariants.Decl`
  is populated in the same block the corpus rewrite already replaced, and `context.go`'s `byFile`
  translation map had no remaining purpose once `Decl.ADR` held a number. Splitting it out would
  have meant writing the filename form and immediately rewriting it. Task 3.3 is therefore a
  no-op by the time Phase 3 runs; its scan test still lands there.
- **The declared-invariant-slug grammar moved into `internal/adr` in Phase 2.**
  `corpus-owns-field-reads` forbids reading `ADR.Sections` outside the package, and
  `Corpus.DeclaredSlugs` needs the declaration grammar, so leaving `declRe` in
  `internal/invariants` would have meant two sources of truth for it. `internal/invariants` keeps
  the backing-class and `Verify:`-note logic and reads declarations through the new accessor.
- **`RenderActiveMD` and `RenderDomainIndex` lost their error returns.** With parsing hoisted out
  neither has a failure mode, and a permanently-nil error is an uncoverable branch under the 100%
  gate. Their parse-error tests moved onto `adr.LoadCorpus`.
- **The corpus cache is per-invocation, not per-`Project`.** Caching for the receiver's lifetime
  broke `TestDomainDocStaleOnAdrRetag` and `TestCheckFailsOnMalformedADRIndex`, both of which write
  an ADR after `Sync` and expect the following `Check` to observe it. Every public operation that
  reads ADRs opens with `beginInvocation`.
- **`Corpus.Claims(target)` was deferred to Phase 5 and replaced in Phase 2 by
  `Corpus.RefsOf(carrier)`.** The consumers that had to stop reading `ADR.Refs` ask the
  by-carrier question; nothing asks the by-target question until the coverage model does, and the
  dead-code gate refuses a method with no production caller.
- **Phases 6 and 7 landed as ONE commit, not two.** Two independent couplings forced it, and
  neither was visible when the plan was written. First, `supersession-full-symmetry` and
  `supersession-flavour-exclusive` lose their proof markers in Phase 6 because the checks they
  covered are deleted and their tests cannot compile - but those retirements only take effect
  from an `Implemented` carrier, so Task 7.3's deferral would have left both slugs
  owed-but-unbacked for exactly one commit, i.e. a red gate. Task 6.1's instruction to "leave
  the existing markers in place" assumed the tests could survive Phase 6; they cannot. Second,
  `proposing-adr/SKILL.md.tmpl` lists both keys as *required* frontmatter, so the moment Phase 6
  lands the chain's own authoring skill instructs every author to write an ADR `awf check`
  refuses - "docs travel with the change" puts Task 7.1 in the same commit regardless.
- **Two migration bugs reached a real corpus run and were caught by `awf check`, not by review.**
  Slug anchors need the `supersedes-invariant:` key rather than `supersedes:`, or the emitted
  tokens are inert and the legacy pairs land in a coverage failure. And `DecisionEnd` offsets go
  stale the moment pass 1 rewrites the body: the `supersedes:`-to-`refines:` downgrade shortens
  it three bytes per token, so on a token-dense ADR the append landed inside the following
  heading, corrupting `## Invariants` into `## Inv13. **Supersedence bookkeeping...` and
  silently deleting every slug that ADR declared. That surfaced as unrelated `adr-token-ref`
  drift about missing declarations, which is a long way from the cause. Both are now pinned by
  regression tests and recorded in `.awf/docs/pitfalls.yaml`.
- **The superseded-target advisory had to be deleted, which the plan did not list.** ADR-0128
  item 5 drops it, and the Phase 6 fixtures made the reason concrete: once supersession is
  coverage-derived, a token into a `Superseded` target is the normal shape of every completed
  supersedence, so the note fires on every retirement and means nothing.
- **The dead-code gate shaped Phase 5 more than expected.** `Corpus.Live`, `UncoveredAnchors`
  and `Anchors` all had to be written in Phase 5, removed, and re-added in Phase 6 with their
  first callers. Building a model bottom-up fights a gate that demands top-down reachability;
  a future plan of this shape should place model methods in the phase that consumes them, not
  the phase that conceives them.
- **Several fixtures had to change premise rather than expectation.** A superseded target now
  owes a back-pointer; a `Proposed` target short-circuits before the back-pointer check ever
  runs; a retirement cycle needs `Implemented` carriers to be `Covered` at all; and a
  single-item target is fully covered by one token, so tests about something else needed a
  second anchor to survive.
- Phase 6 is the only shared-commit group, and it absorbed what were originally two separate
  rendering tasks: deleting `SupersededBy` breaks `domain.go` and the ACTIVE.md chain path in the
  same compile, so they cannot follow in a later phase. It also carries the two domain
  current-state parts, because ADR-0129 item 5 makes the adr-system one a same-commit obligation
  with the domain-index rewrite.
- Every prose target in Phase 7 is a config-tree source. `docs/glossary.md`, `docs/pitfalls.md`,
  and `docs/architecture.md` are generated; the first draft of this plan named them directly,
  which would have been reverted by the next `./x sync` and reported as drift.
- The invariant-retirement rule caught this plan out once already: retirements apply only from an
  `Implemented` carrier, so every marker for a retired slug survives until Task 7.3 even though its
  code dies in Phase 4 or 6. Re-read `internal/invariants/invariants.go:174-176` before touching a
  marker.
- `internal/project/supersession.go` carries roughly 58 of the effort's ~90 field reads and is
  touched by all three ADRs. It is the riskiest single file.
- The generation is 11 to 12: ADR-0127's `drop-audit-base` took 10 to 11 on 2026-07-18. Re-derive
  the head from `internal/migrate/migrate.go`'s registry rather than trusting this line.
- ADR-0130 items 4 and 5 (identity key, audit seam) are separable, as that ADR's Consequences
  records. If the effort needs to shrink, Phase 3 is the one to defer; nothing in Phases 4 to 7
  depends on it.
