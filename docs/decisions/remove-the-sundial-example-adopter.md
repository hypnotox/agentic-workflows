---
format: current-state-v4
slug: remove-the-sundial-example-adopter
status: Accepted
date: 2026-08-04
---
# ADR-remove-the-sundial-example-adopter: Remove the Sundial Example Adopter


## Context

ADR-0090 introduced `examples/sundial` to serve two purposes: a browsable onboarding
artifact and a committed rendered-output quality oracle. The example has since become a
full-surface second adoption rather than a small worked example. It currently contains 135
tracked files and occupies about 1.1 MB. Its independent config, lock, generated target
trees, hooks, runner, fictional application, ADRs, plan, and docs all travel with catalog,
template, schema, and workflow changes.

That breadth imposes repository-specific coupling. The root runner builds awf again to
render and check the nested adoption and runs its separate Go module tests. The root
pre-commit hook carries a helper solely for the nested staged transition. Active docs,
current-state claims, configuration exclusions, migration instructions, and tests all
special-case Sundial. Several tests also search its generated prose for phrases already
owned by focused template tests. The committed tree therefore increases change fan-out and
review noise while no longer giving a new adopter a representative starting point.

The executable value does not require a permanent second tree. The evaluation suite already
constructs a catalog-derived temporary adoption, renderer tests already own drift, target,
runner, singleton, sidecar, and convention-part contracts, and migration tests already use
historical temporary configurations. These fixtures can exercise the remaining interactions
without a second committed lock or generated-output mirror. A temporary fixture cannot
replace a stable browsable example or subjective review of committed assembled prose; this
decision deliberately gives up those benefits.

The removal changes ADR-0090's lasting repository topology and retires active claims created
for that topology, so it requires a successor decision rather than an implementation-only
cleanup. Historical ADRs, completed plans, changelog entries, and research reports remain
append-only records of the repository state in which they were written.

## Decision

1. `decision: no-committed-example-adopter` The repository carries no committed example
   adopter. Sundial and the repository-specific runner, hook, configuration, documentation,
   and test machinery that exists to maintain it are removed rather than reduced or replaced
   with another checked-in example.

2. `decision: temporary-contract-fixtures` Executable confidence formerly obtained from
   Sundial is owned by focused temporary fixtures at the subsystem boundaries for catalog
   rendering, project render-and-drift checking, target outputs, runner behavior, and the CLI
   composition of upgrade followed by render and check. Fixture membership is derived from
   catalog or target authority where completeness matters. No fixture becomes a second
   durable adopter tree or a committed generated-output oracle.

3. `decision: generic-nested-adopter-capability-remains` Product behavior for operating on a
   nested adopted project remains supported and tested independently of Sundial. Only the
   awf repository's special treatment of one fixed nested path is removed.

4. `decision: history-remains-history` Retained historical records continue to describe
   Sundial when that is historically accurate. Metadata required to validate those records,
   including the historical tag vocabulary, remains valid but does not imply a current
   example-adopter feature.

## State changes

- remove `tooling/quality-gates:example-adopter-checked`
- remove `tooling/quality-gates:example-module-isolated`
- remove `tooling/quality-gates:example-zero-notes`
- remove `rendering/companion-scripts:runner-example-adopted`

## Consequences

Easier:

- Template, catalog, workflow, schema, and release changes no longer create a second rendered
  diff, lock update, migration transaction, or staging inventory.
- Root render, check, and pre-commit orchestration no longer know a fixed nested adopter path.
- Contract failures are reported by the package or CLI boundary that owns them instead of by
  a broad example check.
- Active documentation and onboarding no longer present a maximal adoption as the normal
  starting point.

Harder:

- The repository loses its stable, browsable full-adoption output and the ordinary code-review
  diff that exposed assembled template changes.
- Temporary fixtures prove mechanical contracts but cannot gate subjective prose quality.
  Template review, focused tests, and the repository's own rendered adoption remain the
  available review surfaces.
- Historical mentions of Sundial must be distinguished from stale active guidance. Removing
  all textual occurrences would corrupt retained records, while leaving active instructions
  would misdescribe current behavior.

The removal does not narrow awf's support for nested repositories, multiple targets, runner
resolution, schema upgrades, or adopter drift checks. It removes only the permanent fixture
and the claims whose truth depended on that fixture.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Reduce Sundial to a smaller committed example | A smaller tree could improve onboarding, but retaining both onboarding and oracle duties would allow the same full-surface coupling to regrow; retaining onboarding alone was not considered worth permanent repository machinery. |
| Keep the full tree but remove prose assertions and some gate steps | This preserves the dominant generated-diff, lock, migration, and active-documentation costs while weakening the rationale for keeping the tree. |
| Delete Sundial without replacement coverage | This maximizes simplification but needlessly discards useful cross-feature render, check, target, runner, and upgrade confidence that temporary fixtures can own precisely. |
| Move the example to a separate repository | It improves browsing isolation but introduces cross-repository synchronization and still requires maintaining a durable example; no demonstrated onboarding demand justifies that publication workflow. |
| Replace Sundial with one end-to-end temporary CLI fixture | One broad fixture would recreate unclear ownership and coarse failures in test form; focused package fixtures plus one CLI upgrade-composition seam preserve the contracts more directly. |

## Status history

- 2026-08-04: Proposed
- 2026-08-04: Accepted; content-sha256: 1eee9119e4cc9e1056375bdcfd26a076de131e397afe43af3d610714ffadf60d
