---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0200: Package composition and export discipline

## Context

awf's package topology is healthy at the surface and undisciplined underneath. The 44
production package directories effectively all carry a package doc comment, yet nothing
states what form that comment must take, and nothing binds the `cmd/*` mains, whose
purpose statements matter most to a reader navigating eight binaries. Measured by AST over
non-test files this session, 148 of 692 exported declarations carry no doc comment, and the
mechanical enforcement that exists for exactly this rule is deliberately switched off:
.golangci.yml disables revive's `exported` rule and excludes staticcheck's ST1000, because
turning either on would fire on the whole backlog at once. The 2026-07-31 package-topology
survey measured the same gap with a narrower scope (93 of 605).

No production grab-bag package exists today: there is no `util`, `common`, `helpers`, or
`misc` home, and `internal/testsupport` is the one deliberate shared test-support package,
governed by `code-design/single-home`. That absence is worth pinning before it erodes,
because the test lane already shows the legal shape of the temptation:
`internal/project/render_helpers_test.go` is a package-internal test helper file, and
`internal/adr`, `internal/plan`, and `internal/project` each carry an `export_test.go`.
Both patterns are legitimate and must stay so; a composition rule that outlawed them would
be broader than reality.

The pathless `code-design` domain (ADR-0178) governs with a ratchet: a rule is promoted to
a claim only where a violation is always an anti-pattern, the claim binds new or
deliberately converted code, and existing violations remain bounded future candidates.
ADR-0199 is the sibling decision for outcome and error identity; this decision covers
package structure and exported surface, the second area of the same sanctioning pass.

The shipped Maintainable Code Design guide is the judgment-level companion to these
mechanical claims. Its section list is fixed in the catalog
(internal/catalog/standard.go:233: decision-posture, contextual-heuristics,
semantic-modeling, boundaries-and-dependencies, pattern-toolbox, preparatory-refactoring,
failure-modes), pinned by internal/project/docs_sections_test.go and rendered from
templates/docs/maintainable-code-design.md.tmpl. It carries no section on readability, the
judgment territory (naming, comprehension, code that reads as its intent) that the
documentation claims below mechanize only the edges of. An earlier effort recorded a lean
against adding generic guide sections, but that reasoning was specific to outcome-modeling
content, which the current-state topic serves better; readability is genuine
decision-framework material with no topic-shaped home.

## Decision

1. Add `code-design/package-composition` with `applies: global` under the pathless
   `code-design` domain, as the owner of what a package is for, what earns an export, and
   how exported surface stays documented. The topic's identified claims are the durable
   authority; this ADR remains historical rationale. Every claim lands as a reasoned
   contract (`Backing: unbacked` with a concrete `Verify:` instruction), and every claim
   sentence carries its "new or deliberately converted" qualifier inline.

2. One package, one sentence: a new or deliberately converted package states what it owns
   in one sentence in its package doc comment, and the `cmd/*` mains are bound the same
   way. A package whose ownership cannot be stated in one sentence is signalling a split
   or a merge, not exemption from the rule.

3. No grab-bag homes: a new or deliberately converted production package or production
   file is named for the one concern it owns, never as a topical grab-bag (`util`,
   `common`, `helpers`, `misc`, or a file playing that role inside a package). The claim
   is scoped to packages and production files, so package-internal test helper files such
   as `internal/project/render_helpers_test.go` stay legal. Zero production grab-bag
   packages exist today; the claim pins that state.

4. An export earns its consumer: a new exported symbol ships with an outside-package
   consumer in the same green transaction. `export_test.go` files remain legal: they are
   the established in-package test seam, not an outside consumer obligation. For
   composition capabilities (adapters, constructors, option fields), this restates
   `code-design/dependency-composition:concrete-first-consumer`, which remains the
   authority for those symbols; this claim extends the same discipline to every other
   export.

5. Exported surface stays documented: a new or deliberately converted exported
   declaration carries a doc comment, child and DTO types included. The rule the disabled
   linters would enforce becomes claim authority for new code first; re-enabling revive's
   `exported` rule or ST1000 stays a bounded future candidate once the 148-declaration
   backlog converts.

6. Split-signal judgment stays preamble prose, not a claim: a package split is judged by
   fan-out combined with size, never by a line-count threshold alone. A violation of a
   numeric threshold is not always an anti-pattern, so it fails the promotion test.

7. Ship the Readability section: add a `readability` section to the shipped Maintainable
   Code Design guide, extending the catalog section list
   (internal/catalog/standard.go:233), the template
   (templates/docs/maintainable-code-design.md.tmpl), and the pinned section test
   (internal/project/docs_sections_test.go) in the same Implemented transaction. The
   prose is language-agnostic decision-framework material and must pass the leak gate: no
   ".go", "Go package", "internal/", or "./x" substrings. The section name is permanent
   published override surface and is chosen once. It rides this decision because the
   documentation claims above mechanize the edges of exactly this judgment territory.

8. Make the authority visible without copying its normative prose into prompts: add a
   `package-composition-authority` reviewer focus item naming
   `code-design/package-composition` to the adr-reviewer, code-reviewer, and
   plan-reviewer sidecars, comparing each list-valued override with the catalog default
   and preserving every default it replaces, and extend the workflow chain part's
   per-topic consult sentences beside its code-design siblings, all in the same
   Implemented transaction.

9. Record the durable vocabulary: add a docs/glossary.md entry for the grab-bag home and
   the export-earns-consumer discipline, naming ADR-0200 and
   `code-design/package-composition`, in the same Implemented transaction.

10. Declare authority only. No production conversion rides this ADR; the 148 undocumented
    exported declarations and any latent grab-bag files become bounded future conversion
    candidates, enumerable by the roadmap's static-state inventory command.

## State changes

- add `code-design/package-composition:package-owns-one-sentence`
- add `code-design/package-composition:no-grab-bag-homes`
- add `code-design/package-composition:export-earns-consumer`
- add `code-design/package-composition:exported-symbols-documented`

## Consequences

Package structure gains reviewable rules where today only taste operates. A reviewer can
check that a new package states its ownership in one sentence, that a new file is not a
disguised grab-bag, that a new export has a consumer, and that new exported surface is
documented, without re-deriving the judgment each time. The Readability section gives the
published guide the judgment-level counterpart, so awf ships the framework it holds itself
to.

The corpus stays mixed until conversions land: 148 exported declarations remain
undocumented, and the claims deliberately do not force a sweep. All four claims are
reasoned contracts anchored by the `package-composition-authority` focus item, the chain
consult sentence, and each claim's `Verify:` instruction; nothing in the gate fails on a
violation until the linters are re-enabled, which stays future work gated on backlog
conversion.

Export cost rises slightly: a genuinely anticipatory export must wait for its consumer,
mirroring the dependency-composition trade-off, and one-sentence ownership statements
force naming decisions some packages have deferred. The Readability section is a permanent
published surface: its name and presence become part of the standard every adopter
receives, and later renaming would break adopter overrides keyed to it.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A `consumer-side-interfaces` claim | Dual authority with `code-design/dependency-composition:consumer-owned-contracts`, the same ground on which the outcome-modeling effort dropped its payload claim. |
| Line-count split thresholds as a claim | A numeric-threshold violation is not always an anti-pattern; fan-out-with-size judgment stays preamble prose. |
| Re-enable revive `exported` / ST1000 now | The 148-declaration backlog would fire at once, forcing a sweep or nolint noise; the ratchet governs new code first and the linters follow the backlog. |
| The Readability section riding ADR 3 (test-design) | Test design is the narrowest area of the pass; readability's mechanized edges (ownership sentences, doc comments) live in this topic. |
| A standalone readability ADR | The section is a rider production change serving this decision's documentation claims, not an independent authority needing its own record. |
| Folding these claims into `code-design/dependency-composition` | That topic owns dependency selection and wiring; package identity, naming, and documented surface are a different subject. |

## Status history

- 2026-08-01: Proposed
