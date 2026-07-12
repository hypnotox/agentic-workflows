---
status: Implemented
date: 2026-07-04
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [testing]
related: [12, 13, 17, 22, 46, 52]
domains: [tooling]
---
# ADR-0053: Deterministic workflow-chain golden-task eval suite

## Context

awf verifies the *code* an agent produces — the gate, 100% statement coverage (ADR-0012),
rendered-file drift, invariant-backing — and, since ADR-0017, one property of the agent's *process*
(`awf audit` over git history). What nothing verifies is that the **rendered harness itself** — the
suite of skills, agents, and `AGENTS.md` awf renders — still guides an agent correctly *as a composed
system* after a template change. That is the second, still-open half of the "verify the agent" goal.

ADR-0017 named this deferred half a "golden-task eval corpus **run against a live agent** … it needs
agent execution and a scoring harness" (ADR-0017 Context and Consequences), and set it aside as a
separate concern from a renderer/drift-checker. The field this project is benchmarked against agrees
the *live* version is prohibitively expensive (published sweeps run $40K–$320K) and treats eval cost
as the binding constraint. This ADR therefore does **not** build ADR-0017's live-agent corpus. It
takes a different, deterministic path the project's own landscape research recommends as the highest-
leverage in-lane move: a *fixture-based* golden-task suite that renders the real templates and asserts,
without any model, that the composed harness still hangs together. It relates to ADR-0017's deferral
rather than fulfilling it literally.

**The non-redundancy question is load-bearing.** awf already renders and asserts a great deal:

- `internal/project/spine_test.go` renders individual skill/agent templates *in isolation* against a
  fabricated render context and asserts single-artifact content (e.g. `TestBugfixTemplate` already
  checks the rendered bugfix skill contains "regression test", `example-reviewing-impl`, and
  `example-tdd`).
- `internal/project/golden_test.go` (`TestEndToEndGolden`) runs a *real* `Project.Sync`, but only over
  `sampleYAML` (which enables `tdd` + `code-reviewer`), and asserts spine-splice presence plus drift-
  clean — not chain semantics.
- ADR-0046 already fails `awf check` on a rendered reference to a skill outside the enabled set (dead-
  reference integrity — the *target exists*, not that the guidance forms a coherent chain).

The project rejects tests redundant with existing coverage (ADR-0017 itself excluded audit rules
"redundant with drift"). The genuinely uncovered surface is therefore narrow and specific: a **full-
catalog `Sync`** (not a single isolated template, not a two-artifact sample) combined with assertions
on **relationships that span two or more rendered artifacts along a workflow an agent actually walks**
— e.g. a skill's terminal handoff naming a skill that is present in the same rendered set, or a
reviewing *skill* dispatching a reviewer *agent* that in turn carries the shared review-spine partial
(ADR-0052). No existing test renders the whole chain at once and asserts these cross-artifact seams.
That, and only that, is this suite's mandate.

Four constraints shape the design:

- **No live agent, no scoring harness, no new command.** This is an internal regression suite, not a
  shipped capability — it stays in awf's deterministic lane and in the ordinary `go test ./...` gate.
- **Strict non-redundancy.** Every assertion must span ≥2 artifacts and depend on a real full-catalog
  render; re-checking single-artifact content that `spine_test.go`/`initrender_test.go` already own is
  out of scope.
- **The fixture must not silently rot.** A hand-written "enable everything" list would fall behind the
  catalog as skills are added, and the suite would quietly stop covering exactly the new chain seams it
  exists to guard. The enabled set must be derived from the catalog, not enumerated by hand.
- **100% coverage gate (ADR-0012).** Any new package with production statements owes 100% coverage; the
  suite must not create an untestable surface.

## Decision

1. **New test-only Go package `internal/evals`.** It contains only `_test.go` files (package `evals`)
   and no production `.go` source, so it contributes zero coverable statements and satisfies ADR-0012's
   100% gate vacuously. It imports `internal/project`, `internal/catalog`, `internal/testsupport`, and
   the `templates` embed FS. (A one-line build spike confirms a directory of only `_test.go` files
   compiles and runs under `go test`; there is no prior test-only package in this repo, hence the note.)

2. **Each golden task renders the real embedded templates via a full `Project.Sync`.** The fixture
   config's enabled skill/agent set is **derived from `catalog.Load(templates.FS)`** — every catalog
   skill and every catalog agent enabled — plus, for each enabled skill carrying a `requiresDoc` gate
   (ADR-0013), the required doc is also enabled so that skill is not silently suppressed from the
   rendered set. Deriving the set from the catalog (rather than hand-listing) keeps the fixture
   exhaustive as the catalog grows: a newly-added chain skill is covered automatically. This
   full-catalog fixture is deliberately the inverse of the curated `awf init` default
   ([ADR-0022](0022-curated-init-default.md)) — the suite must exercise every seam, not the
   shipped subset.

3. **Assertions are strictly cross-artifact.** Every golden-task assertion depends on the full-catalog
   render and asserts a relationship spanning two or more rendered artifacts along a workflow path — a
   skill→skill terminal-handoff naming a skill present in the same rendered set; a reviewing skill→its
   dispatched reviewer agent; composed multi-skill guidance an agent reads across a chain step. The
   suite does **not** re-assert single-artifact content already covered by `spine_test.go`,
   `initrender_test.go`, or `golden_test.go`. This scope discipline is the suite's entire justification.

4. **Assertions are code-expressed Go, table-driven,** with small matcher helpers (e.g.
   `assertHandoff(from, to)`, `assertComposedGuidance(chain, token)`). No YAML/JSON scenario DSL is
   introduced — a mini-language would be a maintenance surface disproportionate to an internal suite,
   and Go matches how awf's other fixture tests are written.

5. **Fixture reuse via exported primitives.** The suite writes its own thin scaffold wrapper over the
   exported leaf helpers `testsupport.WriteAwfConfig` / `testsupport.WriteFile` followed by
   `project.Open` / `project.Sync`. It does not depend on `internal/project`'s package-private
   `scaffold`/`scaffoldFiles` test funcs, which an external package cannot import.

6. **A representative seed corpus, not an exhaustive path enumeration.** The initial corpus exercises
   the distinct cross-artifact seams — for example: the bugfix path (rendered `bugfix` skill hands off
   to `reviewing-impl`, which dispatches the `code-reviewer` agent that carries the review-spine
   partial); and the load-bearing brainstorm→adr→plan→impl→review handoff chain (each rendered skill's
   terminal handoff names the next skill and that skill is present in the rendered set). Corpus growth
   is expected but not gated; coverage of the seam *types* is the goal, not every permutation.

7. **Document the new test category in `docs/testing.md`** by editing its part source
   `.awf/docs/parts/testing/layout.md` and running `./x sync`; the generated `docs/testing.md` is never
   hand-edited (docs travel with the change, ADR-0005/0020).

## Invariants

- `inv: evals-full-catalog-coverage` — the golden-task fixture's enabled skill/agent set is derived
  from `catalog.Load(templates.FS)` and includes every catalog skill and agent; a test in
  `internal/evals` fails if any catalog skill or agent is absent from the fixture's enabled set, so the
  suite cannot silently stop covering a newly-added chain artifact. Backed by
  `// invariant: evals-full-catalog-coverage` in an `internal/evals` `_test.go` (matching
  `invariants.sources` glob `*.go`).
- The suite asserts only cross-artifact relationships depending on a full-catalog render — a textual
  contract (no single mechanical check can prove a *negative* about redundancy; enforced by review).
- Each golden-task assertion holds against the current rendered templates — enforced by the suite
  itself running in the gate.

## Consequences

- **Easier:** a template edit that severs a cross-artifact workflow seam — a dropped handoff, an
  unlinked spine partial, a reviewing skill whose dispatched agent silently falls out of the enabled
  set — now fails `go test ./...`, even when every artifact individually still has valid frontmatter and
  renders clean. This closes the deterministic, in-lane portion of the "verify the agent" gap.
- **Harder / cost:** a small, ongoing coupling of the suite to template prose (the matcher tokens). This
  is the same brittleness `spine_test.go` already accepts; it is bounded by keeping tokens to load-
  bearing guidance words, not incidental phrasing.
- **Catalog-coupling accepted:** deriving the enabled set from `catalog.Load` couples the suite to the
  catalog shape. Accepted deliberately — it is the mechanism that keeps the fixture exhaustive; the
  alternative (a hand-list) trades that coupling for silent rot, which defeats the suite's purpose.
- **No new command, no config schema change, no new dependency, no gate tier.** The suite is ordinary
  Go test code in the existing gate. Adopters gain nothing to run and nothing to configure; this is
  awf dogfooding its own harness-integrity check.
- **Explicitly ruled out:** a live-agent eval corpus and scoring harness (ADR-0017's framing of this
  half) — deferred indefinitely as out-of-lane and cost-prohibitive; a shipped `awf eval` command; a
  YAML scenario DSL; re-asserting single-artifact content already covered elsewhere.
- **Relationship to ADR-0017:** this is a *reinterpretation* of ADR-0017's deferred second half, not its
  fulfilment. ADR-0017 remains Implemented and unchanged; its live-agent corpus stays deferred.

Doc-currency obligations the implementing commit(s) must satisfy:

- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.
- The new backed invariant `evals-full-catalog-coverage` gains an AGENTS.md invariants entry (the
  `data.invariants` list in `.awf/agents-doc.yaml`), citing this ADR, in the implementing range —
  mirroring the precedent [ADR-0046](0046-skill-reference-integrity.md) set for a new backed check.
- `docs/testing.md` documents the new test category via its part source
  `.awf/docs/parts/testing/layout.md` (Decision item 7); the `tooling` domain narrative is refreshed
  in the implementing range if the new suite materially shifts it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Live-agent golden-task corpus (ADR-0017's original phase-2 framing) | Needs model execution + a scoring harness; non-reproducible and cost-prohibitive ($40K–$320K field sweeps); out of a renderer/drift-checker's lane. |
| Shipped `awf eval` command + on-disk fixture corpus | Much larger surface (new command, likely new config block, renderable docs, coverage) for an internal regression need; the deterministic value is fully captured by a Go test suite. |
| Hand-listed "enable everything" fixture config | Silently rots as the catalog grows — the suite would stop covering new chain seams exactly when it matters. Deriving from `catalog.Load` keeps it exhaustive. |
| YAML/JSON scenario DSL consumed by one runner | Reads as documentation but adds a mini-language and a 100%-covered runner; disproportionate to an internal suite. Go table-driven tests match existing convention. |
| Expand `spine_test.go` / `golden_test.go` in place | Those assert single-artifact content or a two-artifact sample; a full-catalog cross-artifact suite is a distinct concern that would bloat and blur the intent of the existing per-template tests. |
| Nothing — rely on existing unit tests | No existing test renders the full chain and asserts cross-artifact seams; the failure mode (a template edit severing a handoff while each artifact stays individually valid) is genuinely uncovered. |
