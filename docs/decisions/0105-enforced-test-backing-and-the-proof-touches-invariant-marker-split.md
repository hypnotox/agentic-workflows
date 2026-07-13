---
status: Proposed
date: 2026-07-13
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [invariants, testing, config, adr-system]
related: [7, 8, 31, 64, 77, 98, 104]
domains: [invariants, config, tooling]
---
# ADR-0105: Enforced Test-Backing and the Proof-Touches Invariant Marker Split

## Context

An invariant is backed today when a `<marker> invariant: <slug>` comment for a declared
Implemented-ADR slug appears at the start of a line in any source file matching
`invariants.sources` (ADR-0008, ADR-0064). This is a *presence-of-string* check. It reliably
catches one failure — a declared slug with **zero** backing anywhere — and it powers the
`awf context` Tier-1 governance join (ADR-0104). But it guarantees nothing about the code the
marker sits on. The marker is a comment: deleting the enforcing production logic while leaving the
comment passes `awf check`. In this repo 350 markers span 121 files, only 70 of which are test
files — a large share are inert production annotations that would survive their own logic being
removed. Markers are now placed by the implementing agent (the same agent closing the task), so the
"conscious backing" the original design assumed is weaker than intended: the system is an
accountability index, not a proof.

Three further frictions compound this:

- **A token split with no semantic payload.** The ADR side declares `inv: <slug>` while the source
  side backs with `invariant: <slug>`. The corpora are disjoint and the anchors differ, so the two
  tokens carry no distinction — only cognitive load ("why two names for one concept?").
- **ADR-0008 Decision item 4** ("Untestable invariants stay untagged … every tagged slug is
  enforced; there is no per-slug exemption mechanism") forces a genuinely-untestable but
  load-bearing property — e.g. `context-read-only`, structural single-source properties — to shed
  its slug and become prose. That removes it from the `awf context` governance signal precisely when
  a reader most needs to know it exists.
- **The Invariants section is unstructured.** A flat bullet list makes "backed" an implicit,
  after-the-fact property (does a marker happen to exist?) rather than an explicit authoring
  decision. The plan artifact was given structure and validation (ADR-0098); ADR invariants warrant
  the same — the author should have to *declare* whether an invariant is test-proven or a reasoned
  contract, and the checker should enforce that declaration in both directions.

Invariant backing is a load-bearing input to the context system, which the project values highly.
Strengthening its teeth, making the backed/unbacked split explicit, and enriching what it feeds
context are one effort. This ADR owns the backing-model redesign; a companion ADR amends ADR-0104
for how the two markers and the two classes surface in context.

The unifying token is a public surface of the standard: adopters type `inv:` into ADRs that awf does
not render or migrate (`docs/decisions/**` is user-owned; schema migrations only touch the `.awf/`
tree), and adopters place source markers awf likewise cannot rewrite. The token rename and the
marker relocation are therefore a two-front, un-auto-migratable break. They are **deliberately
bundled**: both are pre-1.0-only breaks, and landing them together spends the adopter's one manual
migration once rather than twice.

## Decision

1. **Unify the declaration token to `invariant:`.** The ADR Invariants-section backed-declaration
   token becomes `invariant: <slug>` (leading a markdown list item), matching the source proof
   marker verbatim. `declRe` is updated accordingly. The declaration-token rewrite (all ADR
   declarations + `declRe`) is **atomic** — one commit — distinct from the phased source-marker
   migration (item 4 consequences). The rename is bundled here deliberately (Context): it is a
   pre-1.0-only break landed alongside the marker relocation. The unification is **total**: beyond the
   parsed declaration token, every prose occurrence of the `inv:` token in awf-managed docs — the
   `(inv: <slug>)` citation prefixes in the rendered agent guide (`.awf/agents-doc.yaml`) and the
   domain current-state narratives — is rewritten to `invariant:` in the same effort, so no second
   spelling survives. These citations are references, not parsed declarations, so their rewrite is
   mechanical.

2. **Two source markers with a strong/weak gradient.**
   - **`invariant: <slug>`** — the *proof* marker. It backs a slug only when it sits in a file
     matching a configured **test glob** (item 3). This is the authoritative, enforced marker; the
     plain token carries the strong meaning so the reflexive choice is the enforced one.
   - **`touches-invariant: <slug>[ — <note>]`** — the *context* marker. It never satisfies backing
     and never fails `awf check`; it records a code site that relates to the invariant, with an
     optional free-form note (everything after the slug, trimmed) describing *how* the site relates.
     It may appear any number of times, anywhere a source glob matches (typically production code).
     "Touches" is deliberately broad, to invite annotating tangentially-related sites.

3. **Add a `testGlobs` config surface for structural teeth, with a source-only fallback.**
   `InvariantConfig` gains `TestGlobs []string` (`yaml:"testGlobs"`), anchored doublestar globs
   matched against the slash-separated repo-relative path exactly as `invariants.sources[].globs`
   (ADR-0077). `Config.Validate` rejects a malformed or path-separatorless `testGlobs` pattern on the
   same rule as source globs.
   - **`testGlobs` configured (non-empty):** a proof `invariant:` marker backs a slug **only** when
     its file matches both a source glob and a `testGlobs` pattern — backing means an executed test
     line.
   - **`testGlobs` absent or empty:** backing falls back to ADR-0008 source-only semantics (any
     source-glob file backs). `testGlobs` is thus an **opt-in teeth upgrade**: existing adopters and
     `examples/sundial` keep working unchanged, and a project switches on hard teeth by configuring
     it. awf's own config sets `testGlobs: ['**/*_test.go']`.
   - `TestGlobs` is registered in `internal/configspec` and surfaced in the regenerated
     `docs/config-reference.md` (ADR-0088 makes an unregistered key a gate failure). Whether the
     field addition warrants a schema-generation bump (with a matching `minVersionBySchema` entry,
     ADR-0049) or lands as an inert optional field within the current schema is settled in the
     mechanism plan; either way its consumers are `Config.Validate`, the checker, and the context
     `MarkersUnder` scan.

4. **Explicit, symmetrically-enforced backed/unbacked classification.** Every invariant is declared
   in the Invariants section as exactly one of two forms — the author must choose:
   - **Backed:** ``- `invariant: <slug>` — <property>``. Requires ≥1 proof marker (item 2) in
     backing scope (item 3). A backed declaration with no proof marker **fails** `awf check`
     ("declared backed but unproven").
   - **Unbacked:** ``- `unbacked-invariant: <slug>` — <property>. **Verify:** <how to confirm it by
     hand>``. A reasoned contract, exempt from the proof requirement, still context-tracked via
     `touches-invariant:` markers. Two hard rules: a proof marker existing for an unbacked slug
     **fails** `awf check` ("declared unbacked but backed in source — reclassify or remove the
     marker"), and an unbacked declaration missing its `Verify:` note **fails** `awf check` (an
     unbacked invariant with no verification guidance is useless).

   This **replaces** the `unbacked_invariants:` frontmatter approach: the classification is inline,
   local, and cannot drift from a separate list. It **supersedes ADR-0008 Decision item 4** via
   partial-item supersedence — ADR-0008 stays `Implemented`/live and this ADR carries `related: [8]`;
   an untestable invariant now keeps its slug as an explicit `unbacked-invariant:` rather than being
   demoted to prose. The structuring mirrors the plan-artifact precedent (ADR-0098).

5. **Advisory (non-failing) surfaces.** Routed through the existing `awf check` `note:` channel
   (ADR-0070), never hard findings: a source `invariant:`/`touches-invariant:` marker naming a slug
   no Implemented ADR declares (a stray or renamed slug), and a `touches-invariant:` marker carrying
   no note (the note is its payload; a bare one is low-signal).

6. **Standard-surface and doc updates land with the change.** `docs/decisions/template.md` Invariants
   guidance shows both the `invariant:` (backed) and `unbacked-invariant:` (backed/unbacked with
   `Verify:`) declaration forms; the `proposing-adr` skill and any marker prose adopt the unified
   token and the proof/touches vocabulary with per-language examples; nothing assumes Go
   (`testGlobs` is glob-and-string, adopters set `**/*_test.py`, `**/*.spec.ts`).

## Invariants

Tagged bullets are enforced once this ADR is `Implemented`; each is backed by a proof marker in a
test file under the new model. (This ADR's own declarations use today's `inv:` token while
`Proposed`; the mechanism plan's atomic rewrite migrates them to `invariant:`.)

- `inv: proof-marker-test-scoped` — with `testGlobs` configured, a proof `invariant: <slug>` marker
  backs a slug only when its file matches a `testGlobs` pattern; the identical marker in a non-test
  source file does not back it.
- `inv: absent-testglobs-source-fallback` — with `testGlobs` absent or empty, backing falls back to
  source-glob scope (ADR-0008 semantics), so a project without `testGlobs` is unaffected.
- `inv: backed-requires-proof` — a `invariant: <slug>` (backed) declaration with no proof marker in
  backing scope fails `awf check`.
- `inv: unbacked-refuses-proof` — an `unbacked-invariant: <slug>` declaration for which a proof
  marker exists in backing scope fails `awf check`.
- `inv: unbacked-requires-verify-note` — an `unbacked-invariant: <slug>` declaration with no
  `Verify:` note fails `awf check`.
- `inv: touches-marker-advisory` — a `touches-invariant: <slug>` marker never satisfies backing and
  never produces a hard `awf check` finding.
- `inv: dangling-marker-advisory` — a source `invariant:`/`touches-invariant:` marker naming a slug
  no Implemented ADR declares yields a non-failing advisory note, never a hard finding.
- `inv: bare-touches-note` — a `touches-invariant:` marker carrying no note yields a non-failing
  advisory note, never a hard finding.
- `inv: testglobs-anchored-validated` — `Config.Validate` rejects a `testGlobs` pattern that is
  malformed or contains no path separator, on the same anchored-glob rule as `invariants.sources`.

## Consequences

- **Backing gains real teeth.** With `testGlobs` set, backing means an *executed test line* (test
  file + the 100%-coverage gate, ADR-0012), not any comment. The semantic gap — whether the test
  asserts the right thing — remains unclosable trust, but the floor rises from "a string exists" to
  "a test exercises this."
- **Context becomes a risk map.** The explicit backed/unbacked class lets the companion ADR-0104
  amendment tell an editing agent which governing invariants auto-trigger on breakage (backed) and
  which need manual reasoning (unbacked, with their `Verify:` guidance).
- **The mechanism is additive; teeth are opt-in.** Because absent `testGlobs` falls back to
  source-only backing, the two-marker mechanism and the classification can land without breaking any
  existing adopter. awf switches on hard teeth by configuring `testGlobs` — which is gated on its own
  migration below.
- **A large, deliberate migration, sliced out.** ~27% of this repo's declared slugs (73 of 268) are
  backed only in production today; each needs a proof marker added on its test and its production
  annotation rewritten to `touches-invariant:`, and each invariant classified backed/unbacked. This
  is its own plan behind the mechanism plan; awf's `testGlobs` is set — flipping enforcement to hard
  — only once that migration completes. **The migration plan carries this ADR's flip to
  `Implemented`** (its slugs are enforceable only once the test-scoped model is live).
- **A two-front adopter break.** Adopters rename `inv:`→`invariant:` in their ADRs and relocate
  production `invariant:` markers to tests + `touches-invariant:`; neither is auto-migratable, so the
  schema migration only emits an advisory note. Accepted as a pre-1.0 cost.
- **Doc-currency obligations (same landing commit).** Regenerate `docs/decisions/ACTIVE.md` via
  `./x sync` on the `Implemented` flip; update the `docs/domains/invariants.md` narrative for the
  proof/touches model and the backed/unbacked classification; reword the AGENTS.md "Backed
  invariants" rule (rendered from `.awf/`) for the two markers, the test-scoped proof, and the
  explicit classification; rewrite the adr-readme Invariant-tagging part
  (`.awf/parts/adr-readme/invariants.md`, rendered into `docs/decisions/README.md`) to the unified
  `invariant:`/`unbacked-invariant:` declarations and the proof/touches marker split; regenerate
  `docs/config-reference.md` for `testGlobs`; add a joint changelog `[Unreleased]` entry covering
  this effort's user-facing changes (the `invariant:`/`unbacked-invariant:` declaration and
  `touches-invariant:` markers, `testGlobs`, and — with ADR-0106 — the `awf context` risk-map labels
  and surfaced notes).
- **`Check` return shape widens.** It must return advisory notes alongside hard findings so
  dangling-marker and bare-touches notes flow to the `note:` channel; `MarkersUnder` (context) must
  scan the union of `sources` and `testGlobs`, or splitting the globs would silently narrow Tier-1
  governance (this repo's current single `**/*.go` source already includes test files).
- **New branches to cover.** proof-only, touches-only, `testGlobs` configured vs. absent (fallback),
  backed-without-proof, unbacked-with-proof, unbacked-without-verify-note, dangling-marker,
  bare-touches, and `Disabled`/`Unchecked` interactions each need an explicit test under the 100%
  gate.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the two-token `inv:`/`invariant:` split unchanged (non-breaking) and ship only the teeth change | A single public token reduces the standard's surface, and the rename is cheapest pre-1.0 — batched with the un-migratable marker relocation, it costs the adopter one migration, not two. |
| `unbacked_invariants:` frontmatter exemption (this ADR's first draft) | A separate list makes "backed" an implicit, drift-prone property; inline `invariant:`/`unbacked-invariant:` declarations force the choice at authoring time and enable symmetric enforcement (unbacked-with-proof is an error). |
| Keep ADR-0008 item 4 — untestable invariants stay untagged prose | Removes load-bearing-but-untestable properties from the `awf context` governance signal. The `unbacked-invariant:` class keeps them visible and honest, with mandatory manual-verification guidance. |
| Require markers on covered lines (coverage-data teeth) instead of test-file scope | Requires ingesting coverage data into the checker — heavy, non-neutral. Glob-based test-scoping is language-neutral and cheap. |
| Plain `invariant:` = context, `backing-invariant:` = proof (initial sketch) | The plainest token is the reflexive choice; making it the weak one turns it into an attractive nuisance. Flipped so plain = proof. |
| `affects-invariant:` for the context marker | "Affects" reads as a causal, enforced-ish claim it isn't; "touches" is honestly advisory and lowers the bar to annotate tangential sites, enriching context. |
| One ADR for the whole redesign incl. the context change | Couples a context-relevance decision to the backing-model decision; split into this ADR + an ADR-0104 amendment, one decision each. |
