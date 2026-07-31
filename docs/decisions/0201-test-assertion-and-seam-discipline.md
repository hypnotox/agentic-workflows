---
format: current-state-v2
status: Proposed
date: 2026-08-01
---
# ADR-0201: Test assertion and seam discipline

## Context

awf's Go test corpus is already stdlib-pure: no test file imports an assertion library, and
go.mod carries `stretchr/testify` only as an indirect dependency of lint tooling
(`testifylint` guards a style the project does not use). Nothing pins that purity, so the
first PR that imports an assertion helper would change the corpus's character without
tripping anything. The TypeScript lane (tools/pi-extension-test/tests/*.test.ts) sits
outside `currentState.testGlobs` (`**/*_test.go`), so it is beyond proof-marker reach and
beyond the claims below, which bind Go tests.

The global-seam pattern the dependency-composition architecture retired for production
wiring survives on the test side in a small census: `cmd/awf/main.go:32`'s `isInteractive`
and `internal/coverage/coverage.go:50`'s `hasGoMod` are package-level function variables
existing to be swapped by tests (cmd/awf/init_test.go:163 swaps the first with a cleanup
restore), and `internal/worktree` carries the package-level filesystem-ownership swaps that
its serial test execution exists to protect, as the testing doc's layout section records.
`code-design/dependency-composition:direct-injection-first` already states the production
rule (constructors reject missing dependencies rather than selecting globals; tests use
per-instance fakes and do not swap shared package variables), but its subject is
composition of production dependencies; nothing yet names the test-side act of minting a
new global seam as the anti-pattern it is.

The pathless `code-design` domain governs with a ratchet: a rule is promoted to a claim
only where a violation is always an anti-pattern, the claim binds new or deliberately
converted code, and existing violations remain bounded future candidates. ADR-0199
(outcome and error identity) and ADR-0200 (package composition) are the siblings of this
decision in the same sanctioning pass; this is the third and smallest area, and several of
its original candidates deliberately land elsewhere: shared fixture homes are already
governed by `code-design/single-home`, proof-marker placement is a marker contract owned by
the invariants domain, and table-versus-flat choice, `t.Fatal` versus `t.Error`, and test
size and focus are judgment calls whose violations are not always anti-patterns.

## Decision

1. Add `code-design/test-design` with `applies: global` under the pathless `code-design`
   domain, as the owner of how Go tests assert and how they obtain controlled
   dependencies. The topic's identified claims are the durable authority; this ADR
   remains historical rationale. Both claims land as reasoned contracts
   (`Backing: unbacked` with a concrete `Verify:` instruction), and every claim sentence
   carries its "new or deliberately converted" qualifier inline.

2. Stdlib assertions: a new or deliberately converted Go test asserts with the standard
   library's `testing` package, plain comparisons, and `errors.Is`/`errors.As`, never
   through an assertion or matcher library. Error-identity assertion detail remains
   governed by `code-design/outcome-modeling:test-identity-assertions`; this claim owns
   the no-assertion-library rule. The claim pins existing reality: zero assertion-library
   imports exist today. The TypeScript lane is outside this claim and outside
   `currentState.testGlobs`.

3. No new global seams: a new or deliberately converted test obtains its controlled
   dependencies through constructor or parameter injection per
   `code-design/dependency-composition:direct-injection-first`; minting a new
   package-level variable that exists to be swapped by a test is always an anti-pattern.
   The existing census (`isInteractive` at cmd/awf/main.go:32, `hasGoMod` at
   internal/coverage/coverage.go:50, and `internal/worktree`'s filesystem-ownership
   swaps) stays a set of bounded future conversion candidates, and the worktree
   conversion in particular unlocks parallel execution its serial suite currently
   forgoes.

4. Route the judgment prose, not claims: extend the testing doc's layout part
   (.awf/docs/parts/testing/layout.md, rendered to docs/testing.md) with the
   table-versus-flat choice, `t.Fatal`-versus-`t.Error`, and test size and focus
   guidance, in the same Implemented transaction. That part already carries this species
   of prose. Shared fixture homes get a preamble pointer to `code-design/single-home`
   in the topic text, not a claim.

5. Make the authority visible without copying its normative prose into prompts: add a
   `test-design-authority` reviewer focus item naming `code-design/test-design` to the
   adr-reviewer, code-reviewer, and plan-reviewer sidecars, comparing each list-valued
   override with the catalog default and preserving every default it replaces plus the
   sibling `outcome-modeling-authority` and `package-composition-authority` items
   wherever ADR-0199's and ADR-0200's implementations have already landed them, and
   extend the workflow chain part's per-topic consult sentences the same way, all in the
   same Implemented transaction.

6. Record the durable vocabulary: add a docs/glossary.md entry for the global test seam,
   naming ADR-0201 and `code-design/test-design`, in the same Implemented transaction.

7. Declare authority only. No production conversion rides this ADR; the three-member
   seam census becomes bounded future conversion candidates, enumerable by the roadmap's
   static-state inventory command.

## State changes

- add `code-design/test-design:stdlib-assertions`
- add `code-design/test-design:no-new-global-seams`

## Consequences

The test corpus's two strongest existing properties, stdlib-pure assertions and a
near-empty global-seam census, become pinned state instead of unstated luck. Review gains
a named rule for the two moments that silently change a test corpus's character: the first
assertion-library import and the next swap-me package variable. Because both claims pin
what already holds, the immediate cost is near zero; the cost arrives later as constructor
wiring in tests that would have reached for a global, which is the intended pressure
toward injectable design.

The claims bind the Go lane only. If the TypeScript lane grows, its assertion and seam
discipline is a future decision, and widening `currentState.testGlobs` to reach it is a
marker-reach decision for the invariants domain, not a rider here. Both claims are
reasoned contracts anchored by the `test-design-authority` focus item, the chain consult
sentence, and their `Verify:` instructions; nothing in the gate fails on a violation.

The three-member seam census stays in place until deliberate conversions land, and
`internal/worktree`'s serial test execution persists with it; conversion there carries a
concrete payoff (parallel execution) beyond conformance.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A depguard/forbidigo lint ban on assertion libraries now | Zero violating imports exist; adding lint surface for a problem that has never occurred is speculative, and the claim covers the first occurrence. |
| A `shared-fixture-first` claim | Dual authority: `code-design/single-home` already governs shared test-support homes; a preamble pointer suffices. |
| A proof-marker-placement claim | Marker placement is a contract of the invariants domain's marker system, not code design; that territory already has a dedicated effort. |
| Table-choice and `t.Fatal`-vs-`t.Error` claims | Neither violation is always an anti-pattern; both are judgment calls and land as guidance in the testing doc's layout part. |
| `Backing: test` for either claim | A ratchet scoped to new or deliberately converted tests has no stable population a proof-marked test can assert without inventorying the whole corpus; the anchor and Verify: lines carry enforcement. |
| Widening `currentState.testGlobs` to the TypeScript lane | A marker-reach decision for the invariants domain with its own consequences; nothing in this decision requires it. |

## Status history

- 2026-08-01: Proposed
