---
date: 2026-07-28
adrs: [173]
status: Proposed
---
# Plan: Request-Sensitive Context Authority Tiers

## Goal

Implement [ADR-0173](../decisions/0173-request-sensitive-context-authority-tiers.md) so bare
directory context is bounded orientation, bare file context reports only actual marker relationships,
and explicit facets recover broader authority without changing request ownership or grouping.

Non-goals are changing snapshot selection, pending-operation relevance, summary derivation, artifact
authority, output spill delivery, uncovered reporting, or adding a structured result format.

## Architecture summary

Phase 1 introduces the final marker-kind relationship seam behind the existing projection, while
retaining the old rendered behavior. This behavior-preserving commit makes the later contract
transaction reviewable without leaving an active current-state claim false.

Phase 2 is one intentionally coupled contract transaction. It removes the temporary legacy rosters,
changes request assembly, grouping, authority projection, facets, CLI rendering, managed callers,
documentation, and all six current-state claims together. Those surfaces cannot be split into
separately shippable commits: changing bare file defaults before managed reviewers request
`invariants` temporarily removes invariant authority, while changing facets or grouping alone makes
the existing concise/full/path-attribution claims false. The transaction applies all six ADR-0173
operations in declaration order and closes the ADR through its direct implementation transition.
The plan remains Proposed until Phase 3 records and settles implementation review. All three
phases are inline main-thread work; no helper partitions are declared.

The final model owns relationships at two levels. Each selected file carries sorted `State`,
`Touches`, and `Proofs` claim-ID sets derived from marker sites on that file. Each directory request
carries the union across its included descendants. Topic authority remains globally deduplicated,
while directly related claim summaries retain sorted request-index and marker-kind sources.
Applicable-topic counts and non-direct claim expansion remain topic-level concerns rather than path
group inputs.

## File structure

- **Created:** none beyond this plan.
- **Modified (production and direct tests):** `internal/project/context.go`,
  `internal/project/context_paths.go`, `internal/project/context_projection.go`,
  `internal/project/context_paths_test.go`, `internal/project/context_projection_test.go`,
  `internal/project/context_artifacts_test.go`, `internal/project/context_test.go`,
  `cmd/awf/context.go`, `cmd/awf/context_test.go`, `internal/clispec/clispec.go`,
  `internal/clispec/clispec_test.go`, `internal/project/spine_test.go`.
- **Modified (authored workflow and documentation):**
  `templates/skills/reviewing-impl/SKILL.md.tmpl`,
  `templates/skills/reviewing-plan/SKILL.md.tmpl`,
  `templates/skills/reviewing-plan-resync/SKILL.md.tmpl`,
  `templates/docs/working-with-awf.md.tmpl`, `templates/docs/agents-md-standard.md.tmpl`,
  `.awf/parts/working-with-awf/commands.md`, `.awf/parts/agents-doc/commands.md`,
  `.awf/docs/glossary.yaml`, `.awf/docs/parts/testing/gate.md`,
  `.awf/docs/parts/architecture/components.md`, `.awf/docs/parts/architecture/data-flow.md`,
  `.awf/topics/parts/tooling/context-and-topic/current-state.md`,
  `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md`, `README.md`, and
  `changelog/CHANGELOG.md`.
- **Modified (lifecycle and generated outputs):**
  `docs/decisions/0173-request-sensitive-context-authority-tiers.md`, this plan,
  `docs/decisions/INDEX.md`, `docs/working-with-awf.md`, `docs/agents-md-standard.md`,
  `docs/glossary.md`, `docs/testing.md`, `docs/architecture.md`,
  `docs/topics/tooling/context-and-topic.md`,
  `docs/topics/rendering/workflow-skill-templates.md`, `AGENTS.md`, `.awf/awf.lock`, the root
  `.claude/skills/awf-reviewing-{impl,plan,plan-resync}/SKILL.md` and
  `.pi/skills/awf-reviewing-{impl,plan,plan-resync}/SKILL.md`, plus the corresponding generated
  Sundial docs including `examples/sundial/docs/architecture.md`, agent guide, locks, and
  `sundial-reviewing-{impl,plan,plan-resync}` skill copies
  under `.agents`, `.claude`, `.cursor`, `.gemini`, `.github`, and `.pi`.
- **Deleted:** the temporary legacy path roster fields introduced or retained during Phase 1;
  no files.

## Phase 1: Introduce marker-kind relationships without changing output

- [ ] **Task 1.1: Pin the old observable projection around the refactor.** In
  `internal/project/context_paths_test.go`, `internal/project/context_projection_test.go`, and
  `cmd/awf/context_test.go`, add table cases that capture the pre-ADR-0173 behavior which Phase 1
  must preserve:
  - a file carrying `state: alpha/one:order`, `touches-state: alpha/one:stable`, and an
    `invariant: alpha/one:tested` proof marker produces deterministic direct-claim input while the
    existing renderer still emits its old `Direct rules`, applicable `Invariants`, and backed
    `Proofs` lines;
  - duplicate marker sites on one file deduplicate by claim and kind;
  - a directory plus an exact file preserves first-seen request indexes and the old globally
    deduplicated direct authority result;
  - `ParseContextFacets` still accepts exactly the existing six facets in their current canonical
    order in this phase.

  Keep the existing proof markers on their current tests. Run
  `go test ./internal/project ./cmd/awf`; it must pass before production edits and again after Task
  1.3 with byte-identical expected rendering.

- [ ] **Task 1.2: Add the final internal relationship declarations.** In
  `internal/project/context_paths.go`, add these contract-bearing declarations exactly:

  ```go
  type ContextRelationships struct {
      State   []string
      Touches []string
      Proofs  []string
  }

  ```

  Add `Relationships ContextRelationships` to `ContextPathImpact` and `ContextDirectory`. Keep
  `DirectRuleIDs`, `InvariantIDs`, `ProofIDs`, and `ContextPathTopic.DirectClaimIDs` temporarily and
  mark them in a normal comment as Phase-2 legacy projection inputs; they must not survive Phase 2. Constructors
  must initialize every slice non-nil.

  Add private helpers in `internal/project/context.go` or `context_paths.go` with this behavior:
  - collect marker sites for one selected path by switching on `topic.StateMarker`,
    `topic.TouchesMarker`, and `topic.ProofMarker`;
  - append the qualified claim ID only to the matching relationship-kind set;
  - sort and compact every set;
  - union relationship values kind-by-kind without changing the inputs.

  Unknown marker kinds are impossible after topic parsing and must not receive a fallback label.
  Do not derive `Proofs` from `claim.Backing`; only a `topic.ProofMarker` site on the selected path
  creates that relationship.

- [ ] **Task 1.3: Populate file and request relationships behind the old renderer.** In
  `internal/project/context.go` `assembleContextUniverse` and `makeImpact`, populate the new
  `ContextPathImpact.Relationships` from actual marker sites while continuing to populate the four
  temporary legacy fields exactly as before. In `internal/project/context_paths.go`
  `buildContextRequests`, union every included descendant's relationships into
  `ContextDirectory.Relationships`; excluded descendants, nested-root boundaries, and symlinks do
  not contribute. Preserve repeated positional requests as separate request reports.

  Add test assertions that the new fields contain exact marker-kind sets and actual proof-marker
  filtering while all Phase-1 renderer strings remain unchanged. Run:

  ```sh
  go test ./internal/project ./cmd/awf
  git diff --check
  ```

  Both commands must succeed, and no current-state part, rendered doc, help text, or managed skill
  changes in this phase.

- [ ] **Task 1.4: Verify and commit the behavior-preserving seam.** Run `./x render` and require no
  generated output other than a lock refresh attributable to an authored input; normally there is
  no render diff. Stage exactly:

  ```sh
  git add internal/project/context.go internal/project/context_paths.go \
    internal/project/context_paths_test.go internal/project/context_projection_test.go \
    cmd/awf/context_test.go
  ```

  If a listed file has no diff, omit it; do not stage any other path. Run `./awf check --staged` and
  `./x gate`; both must pass. Commit:

  ```commit
  refactor(tooling): represent context marker relationships
  ```

## Phase 2: Apply the request-tier contract atomically

- [ ] **Task 2.1: Write the final failing semantic tests first.** Extend
  `internal/project/context_paths_test.go`, `internal/project/context_projection_test.go`,
  `internal/project/context_artifacts_test.go`, and `internal/project/context_test.go` before
  changing production behavior. Fixtures must use at least two topics, a directory with several
  descendants whose invisible markers differ, an exact file, a mixed directory/file invocation,
  all three marker kinds, direct invariant and rule claims, non-direct invariant and rule claims,
  a reference edge, evidence sites, artifact provenance with different detailed edges, and pending
  operations. Assert:
  - bare directories expose no relationship rosters and group on tier-0-visible classification,
    compact provenance, domains, topics, warnings, and ADR navigation only;
  - hidden relationship differences do not split a bare group;
  - `--show artifacts` refines groups when detailed source/output/navigation edges differ, while
    each of `relationships`, `invariants`, `all-rules`, `evidence`, and `references` leaves the
    directory grouping identical to bare output;
  - directory request relationships are the sorted union of included descendants, and excluded
    paths never contribute;
  - exact, staged, and range-selected files expose only their actual `State`, `Touches`, and
    `Proofs` sets by default;
  - a proof relationship exists only on the file carrying the proof marker, irrespective of which
    applicable claims have `Backing: test`;
  - a mixed directory/exact invocation does not make directory-only claims directly visible unless
    `relationships` is selected;
  - direct claim bodies deduplicate globally, but each retains sorted sources by request index and
    canonical marker kind;
  - per-topic counts equal active invariant claims and active non-invariant rule claims, remain
    separate from pending counts, and do not depend on request relationships;
  - `invariants` adds applicable non-direct invariant summaries, `all-rules` adds applicable
    non-direct rule summaries, and a directly related claim stays only in the closest category;
  - `evidence` and `references` start only from claims visible through the default request tier or
    `relationships`, `invariants`, or `all-rules`; evidence never reveals a hidden origin claim,
    while one-level referenced targets remain under referenced context; and
  - bounded pending summaries remain visible without `pending`, while `pending` expands operations.

  Extend the facet parser test to require this exact canonical union:

  ```go
  []ContextFacet{
      FacetRelationships, FacetInvariants, FacetAllRules, FacetEvidence,
      FacetSelectors, FacetReferences, FacetPending, FacetArtifacts,
  }
  ```

  Run `go test ./internal/project`; the new cases must fail against Phase 1 for the expected old
  roster, grouping, facet, or visibility behavior, not because fixture setup is invalid.

- [ ] **Task 2.2: Replace legacy path rosters with final request-sensitive assembly.** In
  `internal/project/context_paths.go`:
  - declare `FacetRelationships = "relationships"` and `FacetInvariants = "invariants"` before
    `FacetAllRules`, and make `allContextFacets` exactly the Task 2.1 order;
  - delete `DirectRuleIDs`, `InvariantIDs`, `ProofIDs`, and
    `ContextPathTopic.DirectClaimIDs` from the final model;
  - add `ContextAuthorityCounts` with exact fields `Invariants int` and `Rules int`, and add
    `Counts ContextAuthorityCounts` to `TopicImpact`; and
  - change `contextGroupKey` to accept the selected facets. Its base key includes only fields
    rendered for a tier-0 descendant. Compact provenance includes role and identity, but source,
    output, and navigation edges enter the key only when `FacetArtifacts` is selected. Relationship
    sets and authority-only facets never enter the key.

  Also add these source-attribution declarations at their first production use:

  ```go
  type ContextRelationshipSource struct {
      RequestIndex int
      Kinds        []string
  }
  ```

  Add `Sources []ContextRelationshipSource` to `ContextClaimImpact`, initialized non-nil by its
  constructors. Add the private helper that converts a claim's relationship membership into
  canonical kind labels ordered `State`, `Touches`, `Proofs`.

  In `internal/project/context.go`:
  - stop constructing applicability/backing rosters on each path;
  - retain all applicable topics for each request;
  - add exact/Git file relationships to direct authority by default;
  - add directory aggregate relationships only when `FacetRelationships` is selected;
  - key every direct claim's sources by request index, union marker kinds for the same request, sort
    sources by request index, and order kinds `State`, `Touches`, `Proofs`; and
  - never union directory relationships into an exact file's `ContextPathImpact`.

  In `internal/project/context_projection.go`, change `projectTopicImpact` to accept direct source
  attribution rather than a flat direct-ID slice. Always compute topic counts from all active
  claims. Select categories in closest order: sourced direct claim, non-direct invariant only with
  `FacetInvariants`, non-direct rule only with `FacetAllRules`, referenced context, then pending.
  Apply evidence and outgoing/incoming reference expansion only after the visible direct,
  invariant, and additional categories are built. Preserve deterministic sorting, summary
  derivation, selector behavior, one-level references, and bounded pending behavior.

  Run `go test ./internal/project`; every Task 2.1 semantic case must pass.

- [ ] **Task 2.3: Render tiers, relationships, counts, and sources.** In `cmd/awf/context.go`:
  - exact and Git-selected request blocks render each non-empty relationship kind as
    `State: <ids>`, `Touches: <ids>`, and `Proofs: <ids>` in that order;
  - directory group path impacts never render relationship lines; with `FacetRelationships`, the
    directory request renders its aggregate using the same three labels after its groups;
  - every topic renders `Authority counts: invariants=<n>, rules=<n>` immediately after its
    summary;
  - direct claims render one `Sources:` line whose entries are ordered by request index and use
    exact entry grammar `request <n> [State, Touches, Proofs]`, omitting absent kinds and joining
    entries with `; `;
  - empty claim categories and empty relationship kinds render nothing; and
  - existing selectors, claim summaries, evidence, references, pending, ADR navigation, artifact
    detail, and spill delivery retain their grammar unless ADR-0173 explicitly changes visibility.

  Update `cmd/awf/context_test.go` with exact golden fragments for directory tier 0, exact tier 1,
  all eight facets, full-as-union, request source attribution, static fallback, and standard errors.
  Add a repository regression test in this file that opens `../..`, calls
  `ContextForOptions` with no facets, renders through `renderContext`, and asserts each result is at
  most `contextdelivery.MaxDirectBytes` for these exact request sets:

  ```text
  internal/project cmd/awf
  cmd/awf/context.go
  ```

  The test must fail on an `AWF_CONTEXT_SPILL_V1` descriptor or oversized buffer and report the
  observed byte length without hard-coding the expected smaller length. Run
  `go test ./cmd/awf ./internal/project`; it must pass.

- [ ] **Task 2.4: Update the closed CLI help contract.** In `internal/clispec/clispec.go`, replace
  the context help body so it states directory tier 0, exact/Git tier 1, the three marker-kind
  labels, per-topic counts, the eight facets in canonical order, artifact-only group refinement,
  enrichment-only evidence/references, and `--full` as the eight-facet union. Change the flag line
  to `add all eight facets`. Preserve JSON rejection, uncovered incompatibility, selection, cap,
  spill ownership, and deletion guidance.

  Update `internal/clispec/clispec_test.go` and `cmd/awf/context_test.go` to require
  `relationships`, `invariants`, `all eight facets`, and the tier distinction, and to reject an
  unknown facet plus any `--show`/`--full` combination with `--uncovered`. Run
  `go test ./internal/clispec ./cmd/awf`; it must pass.

- [ ] **Task 2.5: Update managed review callers and publication-safety guards.** Apply this exact
  command-policy replacement in the three authored templates and
  `internal/project/spine_test.go` `TestManagedContextCallersChooseProjection`:

  | Template | Required context facet sequence |
  |---|---|
  | `templates/skills/reviewing-impl/SKILL.md.tmpl` | `--show invariants --show all-rules --show evidence --show pending` |
  | `templates/skills/reviewing-plan/SKILL.md.tmpl` | `--show invariants --show all-rules --show evidence --show pending` |
  | `templates/skills/reviewing-plan-resync/SKILL.md.tmpl` | `--show invariants --show all-rules --show pending` |

  Update each explanation to name non-direct invariant summaries in addition to its existing lens
  inputs. Keep orientation and implementation callers bare, ADR lifecycle on `pending`, every
  `--full`/`--json` ban, reviewer-run command ownership, and the shared spill contract unchanged.
  Extend the catalog-derived empty-data and `missingkey=zero` sweep in `spine_test.go` only if the
  existing sweep does not execute all three changed templates; every affected template must render
  with empty-string variables without `<no value>`, unresolved tokens, malformed empty inline code,
  or marker residue. Run `go test ./internal/project ./internal/evals`; it must pass.

- [ ] **Task 2.6: Update adopter-facing and current documentation.** Make the following authored
  changes; do not edit historical Implemented ADRs or historical plans:
  - `README.md`: replace the current context overview and command-table row with the tier-0/tier-1
    distinction, marker-kind relationships, per-topic counts, eight facets, artifact-only group
    refinement, unchanged cap, and spill ownership.
  - `templates/docs/working-with-awf.md.tmpl`: replace the adopter-default context command bullet
    with the same operational contract and retain the JSON-removal and spill instructions.
  - `.awf/parts/working-with-awf/commands.md`: make the corresponding project override describe the
    same contract so this repository's rendered working guide changes with the adopter default.
  - `templates/docs/agents-md-standard.md.tmpl`: remove the stale claim that concise context reports
    omitted topic-wide claims; direct readers to begin bare, request `relationships`, `invariants`,
    or `all-rules` for the needed authority tier, and reserve `--full` for explicit interactive
    detail rather than managed routine use.
  - `.awf/parts/agents-doc/commands.md`: state that bare directories are tier-0 orientation and
    bare exact/staged/range files carry tier-1 direct relationships before its existing
    named-facet, no-managed-`--full`, and spill-consumption guidance.
  - `.awf/docs/glossary.yaml`: add `context relationship` as the marker-kind `State`, `Touches`, or
    `Proofs` association between a selected file/request and a qualified claim, and `context tier`
    as directory tier 0 versus exact/Git tier 1 with explicit facet promotion.
  - `.awf/docs/parts/testing/gate.md`: state that repository regression tests hold the two bare
    ADR-0173 request sets within direct delivery, while explicit detail remains spill-capable; do
    not reintroduce JSON parity or routine managed `--full` guidance.
  - `.awf/docs/parts/architecture/components.md` and
    `.awf/docs/parts/architecture/data-flow.md`: update the context component and request-to-output
    flow to name request-sensitive tiers, marker-kind file/directory relationships, topic-level
    authority expansion, projection-sensitive grouping, and unchanged spill delivery.
  - `changelog/CHANGELOG.md`: add one Unreleased bullet for request-sensitive tiers and revise the
    existing Unreleased six-facet bullet to say eight facets. Leave older released/history bullets
    unchanged.

  Run `./x render`. Inspect `git diff --name-only` and require every generated path to be explained
  by the authored templates, parts, topic claims, lifecycle files, or their root/Sundial fan-out.
  Read the rendered root and Sundial skill commands, both rendered `docs/architecture.md` files,
  `docs/working-with-awf.md`, `docs/agents-md-standard.md`, `docs/glossary.md`, `docs/testing.md`,
  and `AGENTS.md`; no generated file may be edited directly.

- [ ] **Task 2.7: Apply all State changes and freeze the artifacts in the same transaction.** In
  `.awf/topics/parts/tooling/context-and-topic/current-state.md`, replace the six affected claim
  bodies with prose that states the implemented contract below. Preserve each `Origin`, preserve
  the existing `Revised-by` prefix and append `ADR-0173`, and preserve backing mode and proof marker
  ownership:
  - `context-default-excludes-history` (`Backing: unbacked`): bare directories expose tier-0
    census/group/classification/provenance/domain/topic/count/pending orientation; bare exact and
    Git files additionally expose only actual marker-kind relationships; claim summaries require
    `relationships`, `invariants`, or `all-rules`; history and full prose remain excluded. Its
    `Verify:` fixture must cover indirect claims, mixed request kinds, references, pending, and
    history, and compare each named facet independently.
  - `context-concise-projection` (`Backing: test`): topic summaries and counts render once; direct
    claim summaries deduplicate globally with sorted request/marker sources; non-direct invariant
    and rule summaries require their own facets; closest-category ordering and bounded summaries
    remain deterministic.
  - `context-full-authority-packet` (`Backing: test`): repeatable facets compose in order
    `relationships`, `invariants`, `all-rules`, `evidence`, `selectors`, `references`, `pending`,
    `artifacts`; `--full` is their byte-identical union; evidence/references enrich visible origins;
    no facet restores full prose, a descendant census, or per-path authority repetition.
  - `context-known-artifact-navigation` (`Backing: test`): compact deterministic provenance remains
    in tier 0; `artifacts` expands edges and is the only facet allowed to refine directory groups.
  - `context-path-attribution` (`Backing: test`): request order/repeats remain; files carry actual
    marker-kind relationships; directories retain a separate descendant aggregate; direct claim
    bodies deduplicate with request sources; mixed requests never promote directory-only
    relationships into default file detail.
  - `implementer-context-grounding` in
    `.awf/topics/parts/rendering/workflow-skill-templates/current-state.md` (`Backing: test`): plan
    and implementation review request `invariants`, `all-rules`, `evidence`, `pending`; resync
    requests `invariants`, `all-rules`, `pending`; every other existing caller and spill clause
    remains unchanged.

  In ADR-0173, use the direct `Proposed` to `Implemented` transition: set frontmatter
  `status: Implemented` and append exactly one `Implemented` event carrying the checker-required
  content digest and checker-reported next global state sequence. The transition's implicit batch
  applies exactly these operations in declaration order:

  ```text
  update tooling/context-and-topic:context-default-excludes-history
  update tooling/context-and-topic:context-concise-projection
  update tooling/context-and-topic:context-full-authority-packet
  update tooling/context-and-topic:context-known-artifact-navigation
  update tooling/context-and-topic:context-path-attribution
  update rendering/workflow-skill-templates:implementer-context-grounding
  ```

  Do not append `Implementing` or `Applied` events for this direct transition. Keep this plan's
  frontmatter `status: Proposed` through implementation review. Run `./x render` after the final
  ADR lifecycle and claim state, staging
  `docs/decisions/INDEX.md`, all rendered topic/docs/skills, both root and Sundial locks, and every
  generated output with its authored source.

- [ ] **Task 2.8: Verify the atomic contract transaction and commit.** Run these commands in order:

  ```sh
  go test ./internal/project ./cmd/awf ./internal/clispec ./internal/evals
  ./awf context --show relationships --show invariants cmd/awf/context.go
  ./awf context --full cmd/awf/context.go
  ./awf context --show not-a-facet cmd/awf/context.go
  ./awf context --show relationships --uncovered internal/project
  ./x render
  git diff --check
  allowed=(
    internal/project/context.go
    internal/project/context_paths.go
    internal/project/context_projection.go
    internal/project/context_paths_test.go
    internal/project/context_projection_test.go
    internal/project/context_artifacts_test.go
    internal/project/context_test.go
    internal/project/spine_test.go
    cmd/awf/context.go
    cmd/awf/context_test.go
    internal/clispec/clispec.go
    internal/clispec/clispec_test.go
    templates/skills/reviewing-impl/SKILL.md.tmpl
    templates/skills/reviewing-plan/SKILL.md.tmpl
    templates/skills/reviewing-plan-resync/SKILL.md.tmpl
    templates/docs/working-with-awf.md.tmpl
    templates/docs/agents-md-standard.md.tmpl
    .awf/parts/working-with-awf/commands.md
    .awf/parts/agents-doc/commands.md
    .awf/docs/glossary.yaml
    .awf/docs/parts/testing/gate.md
    .awf/docs/parts/architecture/components.md
    .awf/docs/parts/architecture/data-flow.md
    .awf/topics/parts/tooling/context-and-topic/current-state.md
    .awf/topics/parts/rendering/workflow-skill-templates/current-state.md
    README.md
    changelog/CHANGELOG.md
    docs/decisions/0173-request-sensitive-context-authority-tiers.md
    docs/plans/2026-07-28-request-sensitive-context-authority-tiers.md
    docs/decisions/INDEX.md
    docs/working-with-awf.md
    docs/agents-md-standard.md
    docs/glossary.md
    docs/testing.md
    docs/architecture.md
    docs/topics/tooling/context-and-topic.md
    docs/topics/rendering/workflow-skill-templates.md
    AGENTS.md
    .awf/awf.lock
    examples/sundial/docs/working-with-awf.md
    examples/sundial/docs/agents-md-standard.md
    examples/sundial/docs/architecture.md
    examples/sundial/.awf/awf.lock
  )
  for runtime in .claude .pi; do
    for skill in reviewing-impl reviewing-plan reviewing-plan-resync; do
      allowed+=("$runtime/skills/awf-$skill/SKILL.md")
    done
  done
  for runtime in .agents .claude .cursor .gemini .github .pi; do
    for skill in reviewing-impl reviewing-plan reviewing-plan-resync; do
      allowed+=("examples/sundial/$runtime/skills/sundial-$skill/SKILL.md")
    done
  done
  mapfile -t observed < <(git diff HEAD --name-only --diff-filter=ACMRTUXB)
  unexpected=$(comm -23 \
    <(printf '%s\n' "${observed[@]}" | sort -u) \
    <(printf '%s\n' "${allowed[@]}" | sort -u))
  test -z "$unexpected" || {
    printf 'undeclared transaction path:\n%s\n' "$unexpected" >&2
    exit 1
  }
  git add -- "${allowed[@]}"
  ./awf check --staged
  ./x gate
  ```

  The first two manual context commands succeed (following and deleting a valid spill descriptor
  if explicit detail spills). The unknown facet and uncovered/facet commands fail with usage
  diagnostics. The second render produces no diff. The staged check is clean and the full gate
  passes. Confirm the two bare repository requests print direct output rather than a spill notice:

  ```sh
  ./awf context internal/project cmd/awf
  ./awf context cmd/awf/context.go
  ```

  Commit the complete coupled transaction:

  ```commit
  feat(tooling): add request-sensitive context tiers (implements 0173)
  ```

## Phase 3: Review and freeze the implementation record

- [ ] **Task 3.1: Run governed implementation review over the concrete range.** From a clean tree,
  resolve `<implementation-base>` as the parent of the Phase 1 commit and
  `<implementation-head>` as the Phase 2 commit. Invoke `awf-reviewing-impl` with that concrete
  range, ADR-0173, and this plan. In addition to its ordinary lenses, require review of request-kind
  isolation, actual proof-marker filtering, facet composition and enrichment boundaries,
  projection-sensitive grouping, source attribution, byte-budget regressions, spill preservation,
  managed caller parity, generated documentation, and the direct ADR state transaction. The
  governed workflow owns audits, context grounding, finding routing, corrective commits, and its
  single verify pass. Any finding that changes ADR-0173's settled design requires user approval and
  a successor correction because ADR-0173 is Implemented.

- [ ] **Task 3.2: Record review settlement and freeze the plan.** Require the implementation review
  verify pass to report zero residual findings. Under Notes, record the concrete implementation and
  corrective commit IDs, audit results, review findings and their disposition, and verify result.
  Then set this plan's frontmatter to `status: Implemented`; do not alter its checked task text.
  Run `./x render && ./x check`, inspect any renderer-reported plan records, and stage only this plan
  plus those explained generated paths with explicit pathspecs. Run `./awf check --staged` and
  `./x gate`; both must pass. Commit:

  ```commit
  docs(plans): implement request-sensitive context tiers
  ```

## Verification

After all three phase commits, run:

```sh
./x render
./x check
git diff --exit-code
go test ./...
./x gate
./awf topic tooling/context-and-topic:context-full-authority-packet
./awf topic rendering/workflow-skill-templates:implementer-context-grounding
```

The tree is drift-free and clean; all tests and the full gate pass; both claims show ADR-0173 in
`Revised-by` with their original backing modes; ADR-0173 and this plan are Implemented; and the
repository byte-budget tests keep both bare request sets within direct delivery. Explicit
`--full` remains complete and may spill without changing semantics.

## Notes

The Phase-2 single commit is deliberate current-state atomicity, not convenience. If implementation
finds a behavior-preserving refactor beyond Phase 1 that can pass the gate without changing any
active claim, it may be added to Phase 1 after plan review. Any change to tier visibility, source
attribution, facet composition, grouping refinement, or managed caller policy is load-bearing and
requires amending Proposed ADR-0173 plus fresh review before execution continues.
