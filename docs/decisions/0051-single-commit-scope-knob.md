---
status: Proposed
date: 2026-07-02
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [config, init, audit]
related: [0017, 0029, 0036]
domains: [config]
---
# ADR-0051: Single commit-scope knob

## Context

Two knobs describe commit scopes today. The `commitScope` init var feeds rendered prose only —
eight `{{ with .vars.commitScope }}` sites across the four reviewing-skill templates tell agents
which scope to commit under. Enforcement reads a different surface entirely:
`audit.allowedScopes` drives both the advisory audit rule (ADR-0017) and the blocking
`awf commit-gate` (ADR-0036). Nothing connects them.

The 2026-07-02 analysis walked into the trap this lays: an adopter set the `commitScope` var and
observed `commit-gate` accept other scopes — the prose knob looks like the enforcement knob,
down to the name. awf's own config carries both (`vars.commitScope: awf` beside
`audit.allowedScopes: [adr, awf, plans]`), which must agree by hand.

The init-descriptor mechanism already supports non-var storage: `VarDescriptor.Target` routes
answers into the invariants block (`invariants-marker`, `invariants-globs`) and the catalog trim
(`catalog-skills`, `catalog-docs`) (ADR-0029). The same seam fits the audit block.

## Decision

1. **`audit.allowedScopes` is the sole commit-scope storage.** The `commitScope` var is deleted
   from the catalog `vars:` block and from awf's own config. A leftover `commitScope` entry in an
   adopter's `vars:` map is inert (unreferenced vars render nothing and trip no check); no
   migration, no schema bump.

2. **New descriptor target `audit-scopes`.** A `commitScopes` string descriptor replaces the
   `commitScope` var descriptor: the answer is comma-split, trimmed, and written to
   `audit.allowedScopes` by `ScaffoldConfig`; an empty answer writes nothing (audit semantics:
   nil = accept any, per ADR-0017). `awf init --describe` documents the comma convention in the
   descriptor description.

3. **Templates read scopes from the render context.** `Project.data()` gains a `commitScopes`
   key: the display-formatted scope list from the resolved audit settings (e.g. `` `adr`, `awf`,
   `plans` ``), empty string when scopes are accept-any. The eight template sites move from
   `.vars.commitScope` to `{{ with .commitScopes }}`, with the ADR-0045 fallback ("the project's
   commit scope conventions"). Prose and gate now agree by construction: both read
   `audit.allowedScopes` through the same `audit.Resolve` path.

4. **Descriptor parity holds.** The var-descriptor parity test (ADR-0029) covers targeted
   descriptors; `commitScopes` joins the descriptor list with its target, and
   `render.ReferencedVars` no longer sees a `commitScope` var to seed.

## Invariants

- `inv: commit-scope-single-storage` — no shipped template reads a `commitScope` var; every
  rendered commit-scope mention derives from `audit.allowedScopes` via the render context.
- `inv: audit-scopes-descriptor-routed` — the `commitScopes` init answer lands in
  `audit.allowedScopes` (comma-split, trimmed), and an empty answer leaves the audit block
  unwritten.
- The rendered prose and `commit-gate` can never disagree on scopes (textual contract — both
  consume `audit.Resolve`).

## Consequences

- One knob: setting scopes at init (or editing `audit.allowedScopes` later) updates both what
  agents are told and what the gate enforces, in the next sync.
- Scope prose becomes plural-aware ("a scope from `adr`, `awf`, `plans`") instead of the old
  singular var — closer to real multi-scope projects, including awf itself.
- Adopters who set only the old var lose the prose mention until they set
  `audit.allowedScopes` (the fallback prose keeps the sentence coherent); the changelog entry
  for the next release names the swap.
- `.awf/config.yaml` in this repo drops `vars.commitScope`; rendered reviewing skills re-render
  with the plural form.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep both knobs, document the difference | The names invite the confusion the analysis reproduced; documentation does not remove a trap, it annotates it. |
| Seed `audit.allowedScopes` from the `commitScope` var at init | Prose and gate still read different storage after init; hand-drift returns the moment either is edited. |
| Expose raw `[]string` scopes to templates | Pushes list-formatting into every template site; a Go-computed display string keeps templates dumb and uniform. |
