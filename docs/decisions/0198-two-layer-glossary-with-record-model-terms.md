---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0198: Two-layer glossary with record-model terms

## Context

The glossary has no reader. Nothing under `internal/contextq` or `internal/contextdelivery`
consults it; `glossaryTransform` renders `data.terms` into a sorted table and the agent
guide's document map cites the output. A document that only ever gets written rots by
construction, and this repository's corpus shows it: 58 entries with a median meaning of
392 characters and a maximum of 751, several describing mechanisms that no longer exist.

Adopters get worse than a rotting glossary. They get nothing. `internal/catalog/standard.go`
ships the glossary entry with no `Data`, and `internal/initspec` seeds no terms, so a fresh
adoption renders the empty-state pointer. Meanwhile every artifact awf renders into that
adopter's tree is dense with awf vocabulary that is defined nowhere the adopter's agent will
look: effort, claim, current-state topic, check-in, routine checkpoint, drift, resident root,
stub. The tool teaches a language and ships no dictionary for it.

The rot is not hypothetical, and it is not confined to prose an agent can discount. The
design work behind this decision was itself misled by a stale entry. The glossary term for
`pitfall entry` asserts that an entry's `domains` "drive `awf context` surfacing", and
`internal/configspec/spec.go` tells every adopter the same thing in the config reference.
Both statements have been false since ADR-0134: `context-surfaces-pitfalls` (ADR-0099) was
retired by ADR-0104, its replacement `context-surfaces-tiered-pitfalls` was retired by
ADR-0134, and a pitfall entry's domains today feed exactly one consumer, the `pitfall-domain`
drift check in `internal/project/check.go`. The claim
`rendering/doc-outputs:pitfall-domains-resolved` carries the same false trailing clause. A
stale glossary entry propagated a wrong premise into a design about stale glossary entries,
which is the strongest available evidence that an unread glossary is an active hazard rather
than a neglected nicety.

Three mechanical constraints shape the solution.

`withDefaultData` in `internal/project/datamerge.go` merges a sidecar over catalog defaults
by whole key, never deeply, and that behaviour is the backing for
`rendering/project-output-plan:sidecar-key-overrides-default`. Shipping standard vocabulary
into `data.terms` would therefore mean that any adopter authoring a single term of their own
silently discards the entire shipped set.

The `unused-data` drift check in `internal/project/check.go` computes from the authored
sidecar alone, never from catalog defaults. Any key an adopter writes must be textually
referenced by the assembled template or it is drift.

The catalog residue scan walks `cat.Docs[*].Data` and bans the string form `ADR-` followed by
four digits, but an integer evades it. Shipped content carrying ADR references as numbers
would pass every gate here and then fail link validation in an adopter tree that has no such
ADR.

## Decision

1. The glossary renders from two layers. The catalog ships a standard awf vocabulary as
   `Docs["glossary"].Data["standardTerms"]`; the project authors `data.terms` in
   `.awf/docs/glossary.yaml`. `glossaryTransform` merges the two into the single rendered
   table. The layers occupy distinct keys because whole-key replacement makes a shared key
   unsafe.

2. A project term overrides a standard term of the same case-insensitive `term`. The project
   layer wins because a project that redefines a word means its own definition. An adopter
   removes an unwanted shipped term by defining it, not by suppressing the layer.

3. There is no wholesale suppression switch. An authored `standardTerms` key would be
   `unused-data` drift, since the transform consumes it into `terms` and the template never
   references it textually. Per-term override covers every real need, so the layer is not
   adopter-disableable and `standardTerms` is not an adopter-authored key.

4. `data.terms` becomes a list of records rather than a `term: meaning` map. Each record
   carries a required `term` and `meaning` and an optional `domains`. `domains` names
   configured project domains and is validated exactly as a pitfall entry's `domains` is,
   feeding a drift check; it does not reach `awf context` under this decision.

5. The record carries no `related` and no `aliases`. Neither has a consumer once contextual
   surfacing is out of scope, and inline ADR citations remain legal in the project layer
   because the residue rule binds shipped strings only.

6. Shipped standard terms carry neither `domains` nor any ADR reference in any form,
   including integer form. Adopter domain names are unknowable at ship time, and an integer
   ADR reference would evade the residue scan and break an adopter tree. This is enforced by
   test rather than left to authoring discipline.

7. The rendered table stays two columns, `Term` and `Meaning`, ordered case-insensitively by
   term across both layers. `domains` is machinery metadata and is never a column.

8. A meaning longer than a fixed character threshold produces an advisory note in the
   existing `AdvisoryNotes` channel, alongside the unset-var, stub, part-marker, tag-health,
   and plan-commit-scope families. It warns and never fails. The threshold is a compile-time
   constant, not a config key, because `internal/severity` is explicit that there is
   deliberately no suppressing value and an adopter-raisable threshold is a suppressing value
   in a budget's clothing. No standalone producer seam is introduced: the shipped pre-commit
   template runs bare `check` as well as `check --staged`, and bare `check` is the form that
   prints advisory notes, so commit-time visibility needs no new wiring.

9. The map-to-list change ships with a changelog recipe and no migration, following the
   precedent ADR-0089 set for this same key. No migration in the tree has ever rewritten a
   data key's shape, and the failure mode is a render error naming the sidecar and the
   offending key rather than silent misbehaviour. `examples/sundial` is re-rendered by the
   project runner and never by `awf upgrade`, so its terms are converted by hand in the
   implementing commit.

10. The corpus is cleaned in the same work: stale entries removed, every surviving meaning
    brought under the advisory threshold, and `memory-backed effort` deleted outright rather
    than retained as a retired-term redirect. A glossary states current meaning; the ADR that
    retired a term is where its history belongs.

11. Three surfaces that misstate current reality are corrected. The glossary entry for
    `pitfall entry` and the `pitfalls` data-key description in `internal/configspec/spec.go`
    both drop the claim that domains drive context surfacing. The catalog `Desc` for the
    glossary drops "term ownership", which the glossary has never provided and which renders
    into every adopter's agent guide.

12. The documentation standard gains a glossary rule as a refinement of its existing terse
    rule, not as a new principle: one sentence stating what the thing is, a second only when a
    contrast or boundary is load-bearing. The `terms` description in `internal/configspec`
    carries the same guidance where an author will meet it.

## State changes

- add `rendering/guide-and-doc-templates:glossary-standard-vocabulary`
- add `rendering/guide-and-doc-templates:glossary-standard-terms-portable`
- add `rendering/doc-outputs:glossary-terseness-advisory`
- update `rendering/guide-and-doc-templates:glossary-terms-sorted`
- update `rendering/guide-and-doc-templates:glossary-terms-validated`
- update `rendering/doc-outputs:pitfall-domains-resolved`

## Consequences

Adopters receive a working vocabulary for the language awf speaks to them, and it arrives
through the artifact they are already pointed at rather than through a new surface they must
learn. The glossary gains its first mechanical reader in the terseness advisory, which is
what stops the corpus from drifting back.

`glossary-terms-sorted` and `glossary-terms-validated` are both worded to the map
representation today, naming "the authored map order" and "a non-string map key". Both are
updated rather than retired: the properties survive, their encoding does not. Validation also
becomes layer-aware, since a case-insensitive duplicate within a layer must keep failing the
render while a duplicate across layers is the override in decision 2.

The map-to-list change is breaking for any adopter with authored terms, mitigated only by a
changelog recipe and a render error that names the offending key. This repeats a cost ADR-0089
already accepted for this key rather than inventing shape-rewriting migration machinery for
one data key, and it keeps this work clear of the contended next schema generation.

Contextual surfacing of terms is explicitly out of scope and left to its own decision. The
grounding work established that it is a new context mechanism rather than a reuse: it would
have to revise `tooling/context-and-topic:context-full-authority-packet`, which defines the
eight repeatable facets and `--full` as their byte-identical union, along with its neighbours.
Decision 4 keeps `domains` on the record so that later decision has its data ready, and that
later decision should carry pitfall entries too, whose surfacing is equally absent today.

A future term-lookup command stays possible and stays unjustified for now. Nothing in this
decision anticipates it.

Shipped vocabulary participates in the config hash, so an awf upgrade that changes a standard
term surfaces to adopters as `stale` drift resolved by `awf render`, exactly as any other
template or catalog change does.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Ship standard terms into `data.terms` directly | Whole-key merge means one authored term discards the whole shipped set |
| Ship the standard vocabulary as its own always-on generated doc | A second place to look up a word, and the split is invisible at the moment of lookup |
| Keep the flat `term: meaning` map and add a sibling key for domains | Two parallel maps keyed by term, with no mechanism keeping them aligned |
| Accept both map and list shapes indefinitely | Two authoring shapes to document, test, and reference forever, and the map form can never carry domains |
| Write a shape-rewriting migration | Novel machinery for one key, and it contends for the next schema generation |
| Hard-fail an over-long meaning at render | Truncates a genuinely subtle term into vagueness to satisfy a constant |
| Make the terseness threshold configurable | A raisable threshold is a suppressing value, which the severity model rules out |
| Wire only the terseness advisory into `check --staged` | Prints one of six advisory families at commit time; the shipped hook already runs bare check |

## Status history

- 2026-07-31: Proposed
