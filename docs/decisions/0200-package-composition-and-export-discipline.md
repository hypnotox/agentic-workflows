---
format: current-state-v2
status: Accepted
date: 2026-08-01
---
# ADR-0200: Package composition and export discipline

## Context

awf's package topology is healthy at the surface and undisciplined underneath. The 44
production package directories effectively all carry a package doc comment, yet nothing
states what form that comment must take, and nothing binds the `cmd/*` mains, whose
purpose statements matter most to a reader navigating eight binaries. Measured by AST over
non-test production files this session, counting exported package-level functions, types,
constants, and variables plus methods on exported types, with only a leading doc comment
accepted, 141 of 676 exported declarations carry no doc comment. The mechanical
enforcement that exists for exactly this rule is deliberately switched off: .golangci.yml
disables revive's `exported` rule and excludes staticcheck's ST1000, because turning
either on would fire on the whole backlog at once.

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

4. An export earns its consumer: a new or deliberately converted exported symbol ships
   with an outside-package production consumer in the same green transaction.
   `export_test.go` files remain legal: they are the established in-package test seam, not
   an outside consumer obligation, and a black-box `_test` package does not satisfy the
   consumer requirement. Two deferences bound the claim: for composition capabilities
   (adapters, constructors, option fields),
   `code-design/dependency-composition:concrete-first-consumer` remains the first-consumer
   authority and this claim adds the outside-package requirement on top, since an exported
   composition symbol consumed only in-package should not be exported; and exported error
   identities are governed by `code-design/outcome-modeling:consumed-identity`, whose
   documented-consumer escape hatch this claim does not revoke.

5. Exported surface stays documented: a new or deliberately converted exported
   declaration carries a doc comment. The bound kinds are exported package-level
   functions, types, constants, and variables, and methods on exported types, auxiliary
   and DTO types reachable from an exported API included; exported struct fields and
   interface methods are outside the claim, matching the reach of the lint that would
   back it. The rule the disabled linters would enforce becomes claim authority for new
   code first; re-enabling revive's `exported` rule or ST1000 stays a bounded future
   candidate once the 141-declaration backlog converts.

6. Split-signal judgment stays preamble prose, not a claim: a package split is judged by
   fan-out combined with size, never by a line-count threshold alone. A violation of a
   numeric threshold is not always an anti-pattern, so it fails the promotion test.

7. Ship the Readability section: add a `readability` section to the shipped Maintainable
   Code Design guide, placed after `semantic-modeling` with the rendered heading
   `## Readability`, because readability is the reader-facing half of the code-shape
   judgment sections and precedes the structural ones. The Implemented transaction
   extends the catalog section list (internal/catalog/standard.go:233), the template
   (templates/docs/maintainable-code-design.md.tmpl), and the pinned section test
   (internal/project/docs_sections_test.go), regenerates both committed rendered copies
   (docs/maintainable-code-design.md and
   examples/sundial/docs/maintainable-code-design.md) via `./x render`, and rewrites the
   `rendering/guide-and-doc-templates:maintainable-code-design-guide` claim's section
   enumeration to include readability, preserving its Origin and appending ADR-0200 to
   its Revised-by. The catalog `Desc` deliberately stays as is: it summarizes the
   framework's territory, not the section list. The prose is language-agnostic
   decision-framework material and must pass the section test's forbidden-substring leak
   gate (internal/project/docs_sections_test.go:179), which bans repository-flavoured
   tokens such as ".go", "Go package", "internal/", "./x", and the module path. The
   section name is permanent published override surface and is chosen once. It rides this
   decision because the documentation claims above mechanize the edges of exactly this
   judgment territory.

8. Make the authority visible without copying its normative prose into prompts: add a
   `package-composition-authority` reviewer focus item naming
   `code-design/package-composition` to the adr-reviewer, code-reviewer, and
   plan-reviewer sidecars, comparing each list-valued override with the catalog default
   and preserving every default it replaces plus the sibling `outcome-modeling-authority`
   item wherever ADR-0199's implementation has already landed it, and extend the workflow
   chain part's per-topic consult sentences beside its code-design siblings, likewise
   preserving ADR-0199's sentence when present, all in the same Implemented transaction.

9. Record the durable vocabulary: add grab-bag-home and export-earns-consumer entries to
   the glossary source under .awf/docs/ and regenerate docs/glossary.md via `./x render`,
   naming ADR-0200 and `code-design/package-composition`, in the same Implemented
   transaction.

10. Declare authority only. Beyond the Readability rider, no production conversion rides
    this ADR; the 141 undocumented exported declarations and any latent grab-bag files
    become bounded future conversion candidates, enumerable by the roadmap's static-state
    inventory command.

## State changes

- add `code-design/package-composition:package-owns-one-sentence`
- add `code-design/package-composition:no-grab-bag-homes`
- add `code-design/package-composition:export-earns-consumer`
- add `code-design/package-composition:exported-symbols-documented`
- update `rendering/guide-and-doc-templates:maintainable-code-design-guide`

## Consequences

Package structure gains reviewable rules where today only taste operates. A reviewer can
check that a new package states its ownership in one sentence, that a new file is not a
disguised grab-bag, that a new export has a consumer, and that new exported surface is
documented, without re-deriving the judgment each time. The Readability section gives the
published guide the judgment-level counterpart, so awf ships the framework it holds itself
to.

The corpus stays mixed until conversions land: 141 exported declarations remain
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
| Re-enable revive `exported` / ST1000 now | The 141-declaration backlog would fire at once, forcing a sweep or nolint noise; the ratchet governs new code first and the linters follow the backlog. |
| `Backing: test` for any of the four claims | A ratchet scoped to new or deliberately converted code has no stable file population a proof-marked test or scoped lint can assert without an exemption inventory of the whole backlog; backing arrives by re-enabling the linters after conversion, not by a proof marker now. |
| The Readability section riding ADR 3 (test-design) | Readability's mechanized edges (ownership sentences, documented exported surface) are this topic's claims, so the judgment-level companion belongs beside them; test design carries none of those edges. |
| A standalone readability ADR | The section is a rider production change serving this decision's documentation claims, not an independent authority needing its own record. |
| Folding these claims into `code-design/dependency-composition` | That topic owns dependency selection and wiring; package identity, naming, and documented surface are a different subject. |

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Accepted; content-sha256: 459cb85be4140d180fc1a0354ae04fcd70fd26e6d340a92656ec2979bff1c590
