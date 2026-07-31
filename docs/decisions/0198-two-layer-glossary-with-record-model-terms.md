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
construction, and this repository's corpus shows it: 58 entries whose meanings average 394
characters and reach 722, several describing mechanisms that no longer exist.

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

Five mechanical constraints shape the solution.

`withDefaultData` in `internal/project/datamerge.go` merges a sidecar over catalog defaults
by whole key, never deeply, and that behaviour is the backing for
`rendering/project-output-plan:sidecar-key-overrides-default`. Shipping standard vocabulary
into `data.terms` would therefore mean that any adopter authoring a single term of their own
silently discards the entire shipped set.

The `unused-data` drift check in `internal/project/check.go` computes from the authored
sidecar alone, never from catalog defaults. Any key an adopter writes must be textually
referenced by the assembled template or it is drift.

The catalog residue scan walks `cat.Docs[*].Data` and bans the string form `ADR-` followed by
four digits, but it collects strings only, so a non-string scalar passes unread. Shipped
content carrying an ADR reference in any non-string form would pass every gate here and then
fail link validation in an adopter tree that has no such ADR.

No catalog `Docs` entry ships `Data` today; every existing `Data` block in
`internal/catalog/standard.go` belongs to a skill or an agent. `TestConfigspecDataParity`
derives its expected key set from each non-Generated doc's defaults, and the claim
`config/configspec-and-reference:configspec-data-parity` pins the exemption set at exactly
two: the domain template's injected pair and the generated config reference's injected
collections. A doc-level default therefore forces either an adopter-facing descriptor or a
third exemption.

The example adopter must render free of advisory notes. The project runner fails `check` when
`examples/sundial` emits any `note:` line, a rule ADR-0090 established and
`rendering/companion-scripts:sundial-example-dogfoods-rendered-defaults` pins. Any advisory
computed over content awf itself ships therefore bounds what awf may ship.

## Decision

1. The glossary renders from two layers. The catalog ships a standard awf vocabulary as
   `Docs["glossary"].Data["standardTerms"]`; the project authors `data.terms` in
   `.awf/docs/glossary.yaml`. `glossaryTransform` merges the two into the single rendered
   table. The layers occupy distinct keys because whole-key replacement makes a shared key
   unsafe.

2. A project term overrides a standard term of the same case-insensitive `term`. The project
   layer wins because a project that redefines a word means its own definition. An adopter
   removes an unwanted shipped term by defining it, not by suppressing the layer.

3. There is no wholesale suppression switch, and `standardTerms` is not an adopter-authored
   key. An authored `standardTerms` would be `unused-data` drift, since the transform consumes
   it into `terms` and the template never references it textually, so the mechanism an adopter
   would reach for is unavailable rather than merely unwanted. Per-term override is the
   supported removal path.

4. `standardTerms` becomes the third exemption in
   `config/configspec-and-reference:configspec-data-parity`, on the same ground as the existing
   two: it is not adopter-settable, so an adopter-facing descriptor would be a false promise.
   Describing it in `configspec` instead would contradict decision 3 by publishing it in the
   config reference as a key an adopter may write.

5. `data.terms` becomes a list of records rather than a `term: meaning` map. Each record
   carries a required `term` and `meaning` and an optional `domains`. `domains` names
   configured project domains and fails `check` when it names an unconfigured one, the sibling
   of the existing `pitfall-domain` drift check; it does not reach `awf context` under this
   decision.

6. The record carries no `related` and no `aliases`. Neither has a consumer once contextual
   surfacing is out of scope, and inline ADR citations remain legal in the project layer
   because the residue rule binds shipped strings only.

7. A shipped standard term is portable by construction, asserted by a test over the shipped
   set rather than left to authoring discipline: every record carries exactly `term` and
   `meaning`, both strings, with no `domains` key and no value matching `ADR-` followed by four
   digits. Adopter domain names are unknowable at ship time, and any ADR reference would break
   an adopter tree that has no such ADR. Pinning the field set is what closes the non-string
   hole, since a scalar the residue scan never reads cannot exist in the first place.

8. The rendered table stays two columns, `Term` and `Meaning`, ordered case-insensitively by
   term across both layers. `domains` is machinery metadata and is never a column.

9. A meaning longer than 280 characters produces an advisory note in the existing
   `AdvisoryNotes` channel, alongside the unset-var, stub, part-marker, tag-health, and
   plan-commit-scope families. It warns and never fails. The threshold is a compile-time
   constant, not a config key, because `internal/severity` is explicit that there is
   deliberately no suppressing value and an adopter-raisable threshold is a suppressing value
   in a budget's clothing. 280 is two full sentences of ordinary prose, which is the shape
   decision 14 asks for, and it sits below this corpus's 394-character mean so cleanup is real
   work rather than a formality. No standalone producer seam is introduced: the shipped
   pre-commit template runs bare `check` as well as `check --staged`, and bare `check` is the
   form that prints advisory notes, so commit-time visibility needs no new wiring.

10. The advisory evaluates the merged set, so the threshold binds the shipped layer too. The
    project runner fails `check` on any advisory note from `examples/sundial`, and the shipped
    layer merges into that example's glossary, so an over-long shipped meaning would fail this
    repository's own gate. Decision 7's shipped-term contract therefore also requires every
    shipped meaning to satisfy the threshold: what awf publishes as vocabulary is bounded by
    the rule it publishes about vocabulary.

11. The map-to-list change ships with a changelog recipe and no migration, following the
    precedent ADR-0089 set for this same key. No migration in the tree has ever rewritten a
    data key's shape, and the failure mode is a render error naming the sidecar and the
    offending term rather than silent misbehaviour. `examples/sundial` is re-rendered by the
    project runner and never by `awf upgrade`, so its terms are converted by hand in the
    implementing commit.

12. The corpus is cleaned in the same work: stale entries removed, every surviving meaning
    brought under the advisory threshold, and `memory-backed effort` deleted outright rather
    than retained as a retired-term redirect. A glossary states current meaning; the ADR that
    retired a term is where its history belongs.

13. Four surfaces that misstate current reality are corrected. The glossary entry for
    `pitfall entry` and the `pitfalls` data-key description in `internal/configspec/spec.go`
    both drop the claim that domains drive context surfacing, and the claim
    `rendering/doc-outputs:pitfall-domains-resolved` drops the same false trailing clause,
    keeping only its live property that an unconfigured domain fails `check`. The catalog
    `Desc` for the glossary drops "term ownership", which the glossary has never provided and
    which renders into every adopter's agent guide.

14. The documentation standard gains a glossary rule as a refinement of its existing terse
    rule, not as a new principle: one sentence stating what the thing is, a second only when a
    contrast or boundary is load-bearing. The `terms` description in `internal/configspec`
    carries the same guidance where an author will meet it.

## State changes

- add `rendering/guide-and-doc-templates:glossary-standard-vocabulary`
- add `rendering/guide-and-doc-templates:glossary-standard-terms-portable`
- add `rendering/doc-outputs:glossary-terseness-advisory`
- add `rendering/doc-outputs:glossary-domains-resolved`
- add `tooling/cli:terseness-advisory-nonfailing`
- update `rendering/guide-and-doc-templates:glossary-terms-sorted`
- update `rendering/guide-and-doc-templates:glossary-terms-validated`
- update `rendering/doc-outputs:pitfall-domains-resolved`
- update `config/configspec-and-reference:configspec-data-parity`

## Consequences

Adopters receive a working vocabulary for the language awf speaks to them, and it arrives
through the artifact they are already pointed at rather than through a new surface they must
learn. The glossary gains its first mechanical reader in the terseness advisory, which is
what stops the corpus from drifting back.

`glossary-terms-sorted` and `glossary-terms-validated` are both worded to the map
representation today, naming "the authored map order", "a non-string map key", and "the
offending key". All three phrases are encoding-bound and none of the properties they describe
are: both claims are updated rather than retired, and a list of records is identified by term
rather than by key. Validation also becomes layer-aware, since a case-insensitive duplicate
within a layer must keep failing the render while a duplicate across layers is the override in
decision 2.

The terseness advisory needs two claims rather than one because the family's two properties
are homed in different topics throughout the corpus. Its production and threshold belong in
`rendering/doc-outputs` beside `stub-notes-path-keyed`; its never-fails contract belongs in
`tooling/cli`, where every other advisory family states it individually
(`completeness-advisory-nonfailing`, `stub-advisory-nonfailing`). Following the existing split
keeps doc-outputs to one subject.

The map-to-list change is breaking for any adopter with authored terms, mitigated only by a
changelog recipe and a render error that names the offending term. This repeats a cost ADR-0089
already accepted for this key rather than inventing shape-rewriting migration machinery for
one data key, and it keeps this work clear of the contended next schema generation.

The absence of a suppression switch has a real cost, not merely a deferred one. A tree that has
adopted awf's rendering but not its workflow chain still receives vocabulary describing that
chain, and can remove it only term by term. That is accepted because the alternative mechanisms
are worse rather than because the cost is nil: see the Alternatives table.

Contextual surfacing of terms is explicitly out of scope and left to its own decision. The
grounding work established that it is a new context mechanism rather than a reuse: it would
have to revise `tooling/context-and-topic:context-full-authority-packet`, which defines the
eight repeatable facets and `--full` as their byte-identical union, along with its neighbours.
Decision 5 keeps `domains` on the record so that later decision has its data ready, and that
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
| A sidecar boolean or config key disabling the standard layer | A new adopter-facing switch whose only function is to remove documentation; per-term override already covers the real case |
| Treat an authored empty `standardTerms` as opt-out | The authored key is `unused-data` drift, so the off-switch would fail the gate that documents it |
| Describe `standardTerms` in configspec instead of exempting it | Publishes it in the config reference as an adopter-settable key, which it is not |
| Ship the standard vocabulary as its own always-on generated doc | A second place to look up a word, and the split is invisible at the moment of lookup |
| Keep the flat `term: meaning` map and add a sibling key for domains | Two parallel maps keyed by term, with no mechanism keeping them aligned |
| Accept both map and list shapes indefinitely | Two authoring shapes to document, test, and reference forever, and the map form can never carry domains |
| Write a shape-rewriting migration | Novel machinery for one key, and it contends for the next schema generation |
| Hard-fail an over-long meaning at render | Truncates a genuinely subtle term into vagueness to satisfy a constant |
| Make the terseness threshold configurable | A raisable threshold is a suppressing value, which the severity model rules out |
| Wire only the terseness advisory into `check --staged` | Prints one of six advisory families at commit time; the shipped hook already runs bare check |

## Status history

- 2026-07-31: Proposed
