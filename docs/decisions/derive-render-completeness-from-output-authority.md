---
format: current-state-v4
slug: derive-render-completeness-from-output-authority
status: Implementing
date: 2026-08-05
---
# ADR-derive-render-completeness-from-output-authority: Derive render completeness from output authority


## Context

The rendering inventory retains several hazards with the same mechanical shape: one output
fact is represented in more than one place, and a green check can therefore coexist with a
missing declaration or with output produced by changed Go render logic. Conditional config-tree
units are enumerated separately by output declaration and render dispatch. Live configuration
values are projected through a hand-maintained switch. Embedded template roots and singleton
conditional inputs are checked through surfaces that know only the already-declared set. For
ordinary outputs, check compares current template and config hashes and then the on-disk bytes to
the lock, but it does not compare the current render result to the locked output hash.

The repository already has the right authority boundary. Kind descriptors, target descriptors,
singleton declarations, the declaration builder, and the operation-owned `OutputPlan` collectively
state what renders. Introducing a generalized render registry beside them would duplicate template
identity and output policy rather than remove duplication. The needed refactor is bounded to facts
that are genuinely shared by existing producers.

A second class of hazards is semantic: a concept can survive through paraphrase, two individually
valid prose fragments can contradict one another, and a literal placeholder can be either intended
syntax or an accidental consumable token. Those cases need instructions and review at the authoring
boundary. A generic synonym, contradiction, or intent detector would be speculative and would turn
human meaning into unreliable mechanical policy.

## Decision

1. `decision: existing-output-authority` Mechanical render completeness derives from the existing catalog, kind, target, singleton, declaration-builder, and `OutputPlan` authority. No parallel generalized registry is introduced. Conditional config-tree units that share declaration and render facts use one bounded declaration model consumed by both paths, with test backing that fails when declaration and render dispatch diverge, while format-specific encoding, policy, and lifecycle behavior remain explicit at their owning seams.
2. `decision: live-template-completeness` Every live template identity resolves in the embedded filesystem, with historical recognition-only identities classified separately. Test-backed parity derives the live population from its owning declarations rather than restating a second closed set.
3. `decision: exhaustive-live-state-classification` Every config-reference field is classified exhaustively as either a declared live-state projection or an explicitly static not-applicable representation. Test backing fails both an omitted field and a field assigned to the wrong class.
4. `decision: live-singleton-conditionals` Every singleton template conditional is backed by a value on that artifact's render-context path and by tests for both outcomes. The check derives its conditional population and context from the artifact's existing template and render declarations.
5. `decision: current-render-freshness` Drift checking compares each ordinary frozen output's freshly rendered bytes with its locked output hash after its template and config hashes match, before classifying an on-disk difference as a hand edit. Regenerated and in-place outputs retain their existing fresh-render policy, and staged drift applies the same semantic distinction. Test backing covers the ordinary clean, binary-derived stale, hand-edited, regenerated, in-place, and staged cases. Therefore a change in Go render derivation cannot remain check-clean merely because template and config inputs are unchanged.
6. `decision: semantic-boundary` Mechanically knowable rendering contracts are enforced at their owning declaration or output boundary. Meaning-dependent hazards are carried by focused planning and review instructions with narrow tests that prove those instructions reach enabled targets. Affected templates retain missingkey=zero behavior and coherent empty-value fallbacks, with no unresolved no-value token. awf does not infer synonyms, prose contradictions, placeholder intent, or a universal output-language validator.

## State changes

- update `rendering/project-output-plan:output-plan-complete`
- add `rendering/project-output-plan:conditional-unit-single-source`
- update `rendering/project-output-plan:template-id-single-derivation`
- add `config/configspec-and-reference:live-state-projection-explicit`
- add `rendering/templates:singleton-conditional-key-live`
- add `rendering/sync-and-drift:ordinary-render-freshness`
- add `rendering/workflow-skill-templates:semantic-rendering-review`

## Consequences

Mechanical omissions fail close at the authority that already owns the fact, and binary-side
render changes become visible without changing lock-version policy. Adding a conditional unit or
live projection requires fewer synchronized edits, while target-specific render behavior remains
readable rather than hidden behind a universal graph.

The bounded declarations need enough structure to serve both declaration and render paths. They
must not absorb unrelated policy merely to make the table look uniform. Live template parity must
exclude historical lock-recognition identities deliberately, and Markdown or runtime-specific
validation continues to use explicit output policy rather than filename inference.

Semantic hazards do not disappear. Their mitigation becomes discoverable at the planning and
review moments that can judge meaning, and the remaining pitfalls are narrowed to genuinely ad hoc
or external-runtime cases. The trade-off is accepted over an unreliable semantic checker whose
green result would imply more assurance than it provides.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Introduce one universal render graph | The current output plan, catalog, and target seams already own the relevant facts; another graph would become a competing template and policy authority. |
| Add independent parity tests around every duplicated list | This detects selected omissions while preserving the synchronization burden that causes them. |
| Add a render-logic fingerprint to the lock | Comparing the fresh ordinary render to the existing output hash detects the real byte-level mismatch without adding a manually maintained version axis. |
| Attempt generic semantic prose checks | Synonym, contradiction, and placeholder-intent inference cannot fail reliably enough to serve as a deterministic guard. |

## Status history

- 2026-08-05: Proposed
- 2026-08-05: Accepted; content-sha256: cd4d014f2286f70a31c399fd110df9e73aeb6410ff39ff6065dda51e36c985b8
- 2026-08-05: Implementing; content-sha256: cd4d014f2286f70a31c399fd110df9e73aeb6410ff39ff6065dda51e36c985b8
- 2026-08-05: Applied; operations: update `rendering/project-output-plan:output-plan-complete`, add `rendering/project-output-plan:conditional-unit-single-source`, update `rendering/project-output-plan:template-id-single-derivation`, add `config/configspec-and-reference:live-state-projection-explicit`, add `rendering/templates:singleton-conditional-key-live`
- 2026-08-05: Applied; operations: add `rendering/sync-and-drift:ordinary-render-freshness`
