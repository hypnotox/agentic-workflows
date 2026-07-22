---
date: 2026-07-22
adrs: [147]
status: Proposed
---
# Plan: Topic-Grouped Context Output and Selectors-Only Rendered Applicability

## Goal

Implement ADR-0147: `awf context` output grouped by topic with each topic's authority rendered exactly once per invocation, the matched-paths census replaced by a count plus `--coverage` drilldown, concise output made claim-visible (uncapped claim-ID rosters, deduplicated direct-claim detail, an explicit detail-omission line), an `eligible-unowned` remediation hint, documented `--json` intent, and the rendered topic-doc Applicability paragraph reduced to selectors-only. Non-goals: implementer-side grounding guidance in workflow skills, topic hygiene for over-budget topics, and any change to `awf topic --coverage` or to path-classification semantics.

## Architecture summary

The grouped semantic model lands in `internal/project` (a serialized invocation-level `Topics` collection replaces both the per-path authority duplication and the deprecated never-serialized aggregates), the text renderer in `cmd/awf/context.go` gains a `## Topics` section before a shrunken `## Effective paths` section, and `internal/topic/render.go` stops baking the census into rendered topic docs. `awf topic` (including `--coverage`) is untouched. Design rationale: ADR-0147. The four `tooling/cli` claim updates apply with the context-output batch; the `invariants/topics-and-markers` add applies with the rendered-doc batch; ADR-0147 flips to Implemented in the final commit together with this plan.

## File structure

- **Created:** none.
- **Modified:**
  - `internal/project/context_paths.go` (grouped model types)
  - `internal/project/context.go` (assembly)
  - `internal/project/context_projection.go` (per-topic projection)
  - `cmd/awf/context.go` (text renderer)
  - `internal/project/context_paths_test.go`, `internal/project/context_projection_test.go`, `internal/project/context_test.go`, `internal/project/output_plan_test.go`, `internal/project/spine_test.go`, `cmd/awf/context_test.go` (shape tests)
  - `internal/clispec/clispec.go` (context help text)
  - `.awf/parts/agents-doc/commands.md`, `.awf/parts/working-with-awf/commands.md`, `templates/docs/working-with-awf.md.tmpl`, `.awf/docs/parts/glossary/prepend.md`, `.awf/domains/parts/tooling/current-state.md` (prose)
  - `.awf/topics/parts/tooling/cli/current-state.md` (four claim re-authorings)
  - `internal/topic/render.go`, `internal/topic/render_test.go` (selectors-only applicability)
  - `.awf/topics/parts/invariants/topics-and-markers/current-state.md` (new claim)
  - `docs/decisions/0147-topic-grouped-context-output-and-selectors-only-rendered-applicability.md` (status events), `docs/decisions/INDEX.md`, `.awf/awf.lock`, this plan (final status flip), rendered docs under `docs/` and `examples/sundial/` (sync fallout)
- **Deleted:** none.

## Phase 1: Begin implementation

- [ ] **Task 1.1: Flip ADR-0147 to Implementing.** Following `awf-adr-lifecycle`, append the Implementing status event with the frozen content digest to `docs/decisions/0147-topic-grouped-context-output-and-selectors-only-rendered-applicability.md` (`## Status history` gains `- <today>: Implementing; content-sha256: <digest>`; the digest form and computation are validated by `awf check`, which fails on a wrong digest and names the expected value). Run `./x sync` (regenerates `docs/decisions/INDEX.md`), stage the ADR, `INDEX.md`, and `.awf/awf.lock`, run `go run ./cmd/awf check --staged` (expect `clean`), run `./x gate` (expect pass), commit.

```commit
docs(adr): begin 0147 implementation
```

## Phase 2: Grouped context output (Applied batch 1: the four `tooling/cli` updates)

One closing commit for the whole phase: the V2 handshake requires the claim mutations, the Applied status event, and the code whose tests prove the new claim prose to travel in one staged transaction, so the phase's tasks cannot land as independent commits.

- [ ] **Task 2.1: Grouped model types in `internal/project/context_paths.go`.** Replace the deprecated in-memory aggregate block on `ContextResult` (the `json:"-"` fields `Domains`, `Topics`, `Pending`, `Unowned` and their comment, lines 46-51) with one serialized invocation-level collection, and shrink the per-path topic shape to attribution. Exact declarations:

  ```go
  type ContextResult struct {
  	Projection ContextProjection        `json:"projection"`
  	Requests   []ContextRequest         `json:"requests"`
  	Topics     []InvocationTopicContext `json:"topics"`
  	Paths      []ContextPath            `json:"paths"`
  }

  type InvocationTopicContext struct {
  	ID                 string                  `json:"id"`
  	Title              string                  `json:"title"`
  	Summary            string                  `json:"summary"`
  	Applicability      TopicApplicabilityBrief `json:"applicability"`
  	ClaimIDs           []string                `json:"claimIDs"`
  	DirectClaims       []ClaimDetail           `json:"directClaims"`
  	OmittedDetailCount int                     `json:"omittedDetailCount"`
  	TopicCommand       string                  `json:"topicCommand"`
  	CoverageCommand    string                  `json:"coverageCommand"`
  	Full               *FullTopicContext       `json:"full,omitempty"`
  }

  type TopicApplicabilityBrief struct {
  	DomainPaths      []string `json:"domainPaths"`
  	TopicPaths       []string `json:"topicPaths"`
  	DeclaredGlobal   bool     `json:"declaredGlobal"`
  	MatchedPathCount int      `json:"matchedPathCount"`
  }

  type PathTopicRef struct {
  	ID             string   `json:"id"`
  	DirectClaimIDs []string `json:"directClaimIDs"`
  }
  ```

  In `ContextPath`, change `Topics []PathTopicContext` to `Topics []PathTopicRef` and delete the `Pending []PendingChange` field (pending operations are topic-scoped and already live in `FullTopicContext.Pending`). Delete explicitly: the `PathTopicContext` type, the deprecated aggregate types `TopicContext` and `ClaimRef` (`internal/project/context.go:31-47`), the `applicableClaims` helper (`internal/project/context.go:272`), and the aggregation loop that populated the deprecated fields (`internal/project/context.go`, currently around lines 226-261). The dead-code gate does not flag orphaned exported types, so these deletions must not be left to it. `DomainRef` stays for `ContextPath.Domains`; `ClaimDetail`, `FullTopicContext`, the `ADR*` types, classification, and request types are unchanged. Forbidden: keeping the deprecated fields alongside the new collection. Every `ContextResult` literal initializes `Topics` to a non-nil empty slice, specifically the static-fallback literal (`cmd/awf/context.go:80`) and the `assembleContextUniverse` result literal (`internal/project/context.go:197`), so JSON never emits `"topics": null`.

- [ ] **Task 2.2: Assembly and projection in `internal/project/context.go` and `internal/project/context_projection.go`.** Replace `projectPathTopic` (context_projection.go:10) with two functions:
  - `projectInvocationTopic(t topic.Topic, corpus topic.Corpus, selectingPaths []string, currentPaths []string, pending []PendingChange, projection ContextProjection) InvocationTopicContext`: computes applicability via the existing `topic.ApplicabilityForTopic` and reduces it to `TopicApplicabilityBrief` with `MatchedPathCount = len(a.MatchedPaths)`; sets `ClaimIDs` to every current claim ID, sorted ascending, uncapped; sets `DirectClaims` to the `contextClaimDetail` of exactly those claims having at least one marker site whose `Path` is in `selectingPaths` (the union across the invocation's effective paths), sorted by ID, each detail's `Sites` filtered to sites on `selectingPaths` and references excluded (as concise detail today); sets `OmittedDetailCount = len(ClaimIDs) - len(DirectClaims)`; sets `TopicCommand = "awf topic " + t.ID.String()` and `CoverageCommand = TopicCommand + " --coverage"`; when `projection == ContextFull`, leaves `DirectClaims` empty and `OmittedDetailCount` zero (per-claim detail renders exactly once, under `Full`; per-path direct attribution survives in `PathTopicRef.DirectClaimIDs`) and attaches `Full` exactly as today (every claim via `contextClaimDetail(claim, corpus, "", true)`, sorted, plus the topic's pending changes).
  - `pathTopicRef(t topic.Topic, corpus topic.Corpus, path string) PathTopicRef`: per-path attribution; `DirectClaimIDs` computed with the existing exact-path marker match, sorted.
  In `context.go`, build `res.Topics` as the union of applicable topics across all covered effective paths, sorted by topic ID, each projected once with `selectingPaths` = the sorted set of effective paths that selected it; per-path blocks receive `PathTopicRef`s. Pending-change computation moves from per-path to per-topic (feed `pendingChanges` output for the topic into `projectInvocationTopic`). Direct-claim selection semantics (exact-path `state:`, `touches-state:`, and proof markers) are unchanged; only where detail renders moves. Constraint: a topic applicable to several paths is projected exactly once; determinism comes from sorted topic IDs and sorted `selectingPaths`.

- [ ] **Task 2.3: Text renderer in `cmd/awf/context.go`.** Rewrite `printContext`: after `## Requests`, print `## Topics` (header always present, even with zero entries, including the static-fallback mode), one block per `res.Topics` entry in order:

  ```
  <id> - <title>
    Domain paths: [<space-joined>]
    Topic paths: [<space-joined>]
    Both domain and topic selectors must match.
    Matched paths: <N> (drill down: awf topic <id> --coverage)
    Claims (<len(ClaimIDs)>): <id1>, <id2>, ...
    Direct claims:
      <printClaimDetail per DirectClaims entry, label "Direct claim">
    Details omitted for <OmittedDetailCount> claim(s); drill down: awf topic <id>
    Full authority:
      <printClaimDetail per Full.Claims entry, label "Claim">
      Pending: ADR-<n> <op> <claim>   (per Full.Pending entry, as today)
  ```

  Branch rules: for `DeclaredGlobal` topics the two selector lines and the both-must-match line are replaced by `Global topic within owning domain selectors: [<space-joined>]`; the `Direct claims:` block prints only when `DirectClaims` is non-empty (never in the full projection, where Task 2.2 leaves it empty so each claim's detail renders exactly once); the `Details omitted` line prints only when `OmittedDetailCount > 0`; the `Full authority:` block prints only when `Full != nil`, and never together with the omission line. Then `## Effective paths`, per path: the existing classification line, `Nested root`/symlink lines, `Domain:` lines, then one `  Topic: <id>` line per `PathTopicRef` followed by `    Direct claims: <comma-joined ids>` when non-empty; Artifact and ADR-navigation blocks are unchanged and stay per-path. A path classified `eligible-unowned` additionally prints `  No domain owns this path; add a domain glob to a configured domain to own it (see: awf context --uncovered)`. `printClaimDetail` and `printUncovered` are unchanged. JSON continues to encode `res` directly, so text/JSON parity holds by construction.

- [ ] **Task 2.4: Shape tests.** Update the pinned tests to the grouped shape, preserving every existing invariant proof marker and each test's intent:
  - `cmd/awf/context_test.go`: the parity test (marker `tooling/cli:context-output-parity`) asserts text and `--json` render the same grouped result and concise JSON contains no `"full"` key; the static-fallback test (marker `tooling/cli:context-static-fallback`) expects the header plus empty `## Topics` and `## Effective paths` sections and `"topics": []` (not null) in JSON; the read-only test (marker `tooling/cli:context-read-only`) is unchanged in intent; `TestPrintContextTitlelessTopic` asserts a zero-claim topic renders `Claims (0):` with no omission line; `TestPrintContextFullHumanAndWriteErrors` rebuilds its fixture as an `InvocationTopicContext` in `res.Topics` while keeping its write-error branches intact. Add assertions: a two-path single-topic invocation renders the topic block exactly once (substring count of the topic title line in the text output equals 1), full output contains no `Details omitted` line, and full output renders each claim's detail exactly once per topic.
  - `internal/project/context_paths_test.go` (marker `tooling/cli:context-path-attribution`): unique-path emission and request attribution unchanged; assert path topic collections are ID-plus-direct-claim-ID refs and result collections stay non-null.
  - `internal/project/context_projection_test.go` (marker `tooling/cli:context-full-authority-packet`): full projection emits every current claim once per topic at invocation level with `DirectClaims` empty; the concise direct-claim union dedupes a claim selected by two paths into one detail; concise sites are filtered to selecting paths while full sites are unfiltered. Add here (or in `cmd/awf/context_test.go`) a second proof site `// invariant: tooling/cli:context-applicability-navigation` asserting `TopicApplicabilityBrief.MatchedPathCount` and `CoverageCommand` render the count-plus-drilldown form, mirroring `context-full-authority-packet`'s two-site pattern (the existing site at `internal/topic/coverage_test.go:109` keeps proving the shared evidence model).
  - `internal/project/context_test.go`: port the aggregate-based assertions (sorted topics, global flag, state-narrowed claim sets, pending; currently against `res.Domains`/`res.Topics`/`res.Pending`/`res.Unowned` at lines 64, 151, 168-195, 222, 236, 292-296, 337-340, 459-462, 534-535) to the invocation-level `res.Topics` grouped shape, folding any now-redundant ones into the grouped-model tests.
  - `internal/project/output_plan_test.go`: retype the `func(topic PathTopicContext) bool` closure (line 262) to `PathTopicRef`.
  - `internal/project/spine_test.go` (second `context-full-authority-packet` site, `TestManagedContextCallersChooseProjection`): the caller-projection expectations stand; adjust only pinned output substrings that moved.
  Command: `go test ./cmd/awf/... ./internal/project/...` reaches `ok` for every package.

- [ ] **Task 2.5: Re-author the four claims.** In `.awf/topics/parts/tooling/cli/current-state.md`, rewrite the canonical prose of the four claims, preserving each claim's `Origin` and existing `Revised-by` entries, appending `ADR-0147`, and keeping each backing mode:
  - `context-full-authority-packet` (Backing: test): "Context assembles one topic entry per applicable topic per invocation: concise entries carry the uncapped current claim-ID roster, the full detail of exactly the marker-selected direct-claim union, and, when any rostered claim's detail is omitted, an explicit detail-omission line with the topic drilldown, while context --full renders every current claim's full detail and pending operations once per topic with no omission line, from the same non-recursive semantic model; managed complete-authority callers request --full explicitly."
  - `context-default-excludes-history` (Backing: unbacked, no proof marker): keep the exact-path selection sentence, replace the omitted-claims clause with the roster-plus-omission-line form, and update the `Verify:` line to compare grouped concise output against both `context --full` and explicit `awf topic <claim-id> --history` output on a fixture with claim provenance and references.
  - `context-path-attribution` (Backing: test): "Context preserves normalized request queries separately from sorted effective paths, records directory expansion status, and emits each unique effective path once with every sorted request that selected it and non-null result collections; a path's topic collection is an attribution of topic IDs and direct-claim IDs, while topic authority lives in the sorted invocation-level topic collection."
  - `context-applicability-navigation` (Backing: test): "Topic and context applicability share one evidence model that lists owning-domain selectors and topic selectors separately and states that both must match; awf topic --coverage reports the sorted concrete matched paths and marker sites, while context reports the matched-path count with a coverage drilldown, and neither claims symbolic glob intersection."
  The wording is implementation-ready; the executor lands it verbatim or tightens phrasing without changing meaning.

- [ ] **Task 2.6: Prose and help text.**
  - `internal/clispec/clispec.go`, context command `HelpBody` (around line 209): describe the grouped shape (each applicable topic once with roster and direct-claim detail; per-path attribution; matched-path count with `--coverage` drilldown) and change the `--json` flag line to `--json  emit the selected projection as JSON for machine consumption; agents reading output should prefer the text form`.
  - `.awf/parts/agents-doc/commands.md` (the `Start with awf context ...` passage) and `.awf/parts/working-with-awf/commands.md` (the representative-query paragraph): rewrite the `awf context` sentences for the grouped shape; keep the ADR-path, `--uncovered`, and `--staged` sentences.
  - `templates/docs/working-with-awf.md.tmpl`: apply the same rewrite to the shipped default's `awf context` description (line 28 region) so the adopter-facing text matches the binary.
  - `.awf/docs/parts/glossary/prepend.md`: re-define **Concise context** (claim-ID roster, direct-claim detail union, explicit detail-omission line) and **Full authority packet** (every applicable claim once per topic; matched-path count with coverage drilldown in place of matched paths).
  - `.awf/domains/parts/tooling/current-state.md`: update the projection-narrative sentences that describe per-path authority and the census.
  Non-contractual prose: qualifying instructions above suffice; no literal wording is mandated beyond the `--json` flag line. Run `./x sync`; `AGENTS.md`, `docs/working-with-awf.md`, `docs/glossary.md`, `docs/domains/tooling.md`, and `docs/topics/tooling/cli.md` regenerate.

- [ ] **Task 2.7: Applied event and batch commit.** Append to the ADR's `## Status history` one Applied event naming, in declaration order, exactly the four update operations (`- <today>: Applied; state-sequence: <n>; operations: update `tooling/cli:context-full-authority-packet`, update `tooling/cli:context-default-excludes-history`, update `tooling/cli:context-path-attribution`, update `tooling/cli:context-applicability-navigation``; the sequence number must be the next state sequence, and a mismatch fails `awf check --staged` naming the expected value). Stage the complete transaction (code, tests, claim part, ADR, prose parts, every regenerated rendered file, `.awf/awf.lock`), run `go run ./cmd/awf check --staged` (expect `clean`), run `./x gate` (expect pass), commit.

```commit
feat(tooling): group context output by topic
```

## Phase 3: Selectors-only rendered applicability (Applied batch 2: the add; final)

One closing commit: the claim add, its Applied event, the ADR and plan status flips, and the renderer change whose test backs the new claim form one staged transaction.

- [ ] **Task 3.1: Selectors-only summary in `internal/topic/render.go`.** Change `applicabilitySummary` to take the qualified topic ID (adjust `BuildTopicModel` to pass `t.ID.String()`) and return the census-free form. Exact output (contract-bearing rendered prose):
  - non-global: ``Owning domain selectors: `<a>`, `<b>`. Topic selectors: `<c>`. Both domain and topic selectors must match. Run `awf topic <id> --coverage` for current matched paths and marker sites.``
  - global: ``Global topic within owning domain selectors `<a>`, `<b>`. Run `awf topic <id> --coverage` for current matched paths and marker sites.``
  - An empty selector list renders the word `none` in place of the backticked list (publication-safe: no empty backticks, no unresolved token).
  Delete the marker-site and matched-path formatting from the function; `ApplicabilityForTopic` and `TopicApplicability` are unchanged (still consumed by `awf topic --coverage` and the context brief).

- [ ] **Task 3.2: Render test and invariant proof.** In `internal/topic/render_test.go`, update the applicability assertions to the selectors-only form and add a test asserting: the summary contains the drilldown sentence and the selector lists; contains neither `Current matched paths` nor `Marker sites:`; the global and empty-selector variants render their specified forms. Annotate the new test `// invariant: invariants/topics-and-markers:rendered-applicability-selectors-only`. `go test ./internal/topic/...` reaches `ok`.

- [ ] **Task 3.3: Author the new claim.** In `.awf/topics/parts/invariants/topics-and-markers/current-state.md`, add under `## Claims`:

  ```markdown
  ### `invariant: rendered-applicability-selectors-only`

  A rendered topic document's applicability paragraph carries only the owning-domain selectors, the topic selectors, the both-must-match rule (or the global-topic variant), and a drilldown to `awf topic <id> --coverage`; it never embeds current matched paths or marker sites, and an empty selector list degrades to coherent prose.
  Origin: ADR-0147
  Backing: test
  ```

- [ ] **Task 3.4: Re-render and glossary.** Update the **Applicability evidence** entry in `.awf/docs/parts/glossary/prepend.md` (the concrete census lives in `awf topic --coverage`; rendered docs and context carry selectors and counts). Run `./x sync`, which regenerates every rendered topic doc without the census and also re-renders `examples/sundial` from the freshly built binary; `internal/project/example_wiring_test.go` and the golden tests pass, updating any golden fixture that embeds the old Applicability paragraph. Post-check: `grep -rn "Current matched paths" docs/topics/ docs/domains/ examples/sundial/docs/` returns no output (the plan file itself quotes the literal, so `docs/plans/` is excluded from the scope).

- [ ] **Task 3.5: Final transaction.** Append to the ADR the second Applied event (`- <today>: Applied; state-sequence: <n>; operations: add `invariants/topics-and-markers:rendered-applicability-selectors-only``) and the Implemented status event per `awf-adr-lifecycle`; flip this plan's frontmatter to `status: Implemented`, recording any implementation findings under Notes. Run `./x sync`, stage the complete transaction (renderer, tests, claim part, glossary part, ADR, plan, all regenerated docs including `examples/sundial`, `.awf/awf.lock`), run `go run ./cmd/awf check --staged` (expect `clean`), run `./x gate` (expect pass), commit.

```commit
feat(invariants): render selectors-only applicability
```

## Verification

- `go run ./cmd/awf context --full internal/migrate` renders exactly one `Full authority:` block per applicable topic (for that single-topic directory: one), and the output no longer repeats claim detail per file.
- `go run ./cmd/awf context internal/project/render.go` shows the claim-ID roster, the direct-claim detail once, a matched-path count with the coverage drilldown, no census line, and an explicit `Details omitted` line.
- `go run ./cmd/awf context --full docs/decisions/0147-topic-grouped-context-output-and-selectors-only-rendered-applicability.md` still renders the per-path ADR-navigation block with operation progress.
- `grep -rn "Current matched paths" docs/topics/ docs/domains/ examples/sundial/docs/` returns no output, while `go run ./cmd/awf topic tooling/cli --coverage` still lists matched paths and marker sites.
- `./x check` is clean and `./x gate` passes at each phase boundary and at HEAD.

## Notes

- Deferred, recorded in ADR-0147 Consequences: implementer-side grounding in workflow skills; topic hygiene for over-budget topics.
- Adopters see a one-time topic-doc drift after upgrading, resolved by their next sync (awfVersion-gated).
- If `TestManagedContextCallersChooseProjection` pins output substrings that moved, adjust only the pinned substrings; the caller-projection expectations themselves are out of scope.
