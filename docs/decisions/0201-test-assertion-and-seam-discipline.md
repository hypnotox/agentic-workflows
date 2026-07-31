---
format: current-state-v2
status: Accepted
date: 2026-08-01
---
# ADR-0201: Test assertion and seam discipline

## Context

awf's Go test corpus is already stdlib-pure: no test file imports an assertion library.
go.mod carries `stretchr/testify` only as a transitive module dependency of golangci-lint
via testifylint, which .golangci.yml's `default: none` enable list leaves disabled, so
nothing in the toolchain currently observes assertion-library usage and nothing pins the
purity: the first PR that imports an assertion helper would change the corpus's character
without tripping anything. The TypeScript lane (tools/pi-extension-test/tests/*.test.ts)
sits outside `currentState.testGlobs` (`**/*_test.go`), so it is beyond proof-marker reach
and beyond the claims below, which bind Go tests.

The global-seam pattern the dependency-composition architecture retired for production
wiring survives at scale on the test side. An AST census this session finds 31
package-level function-valued variables across 11 production packages (excluding
`internal/testsupport` and the examples), each existing to be reassigned by a test:
exemplars are `isInteractive` (cmd/awf/main.go:32, swapped at cmd/awf/init_test.go:163
with a cleanup restore), the eight-member syscall seam block at
internal/contextspill/log.go:32-39, the five-member scaffolding block at
cmd/awf/new.go:195-201, and the ownership seam that spans two packages as
`managedOwner` (internal/worktree/topology.go:74) and `residentOwner`
(internal/effort/safeio.go:96). The generic swap helper `internal/testsupport.SwapVar`
has 82 call sites, and the `getwd` seam alone is swapped at 53. The testing doc's layout
section records that `internal/worktree`'s package-level filesystem-ownership swaps are
what keep its suite serial. `code-design/dependency-composition:direct-injection-first`
states the adjacent production rule (a one-operation dependency is injected as a function
and an immutable input as a value; a required dependency never silently defaults), and its
`Verify:` line already instructs reviewers to reject test-only production indirection;
what no claim yet owns is the test lane itself, where minting the next swap-me package
variable is the act that grows this census.

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
   dependencies. Both claims govern the character of a test's setup and its assertions,
   judged at the same review moment on the same diff, which is what makes them one topic.
   The topic's identified claims are the durable authority; this ADR remains historical
   rationale. Both claims land as reasoned contracts (`Backing: unbacked` with a concrete
   `Verify:` instruction), and every claim sentence carries its "new or deliberately
   converted" qualifier inline.

2. Stdlib assertions: a new or deliberately converted Go test asserts with the standard
   library's `testing` package, plain comparisons, and `errors.Is`/`errors.As`, never
   through an assertion or matcher library. Error-identity assertion detail remains
   governed by `code-design/outcome-modeling:test-identity-assertions`; this claim owns
   the no-assertion-library rule. The claim pins existing reality: zero assertion-library
   imports exist today. The TypeScript lane is outside this claim and outside
   `currentState.testGlobs`.

3. No new global seams: in a new or deliberately converted test, minting a new
   package-level variable that exists to be swapped by a test is always an anti-pattern;
   a dependency the test newly introduces arrives through constructor or parameter
   injection per `code-design/dependency-composition:direct-injection-first`. A new test
   may swap an existing seam from the measured census until that seam is deliberately
   converted; the census (31 seams across 11 packages, per Context) stays a set of
   bounded future conversion candidates, and the worktree conversion in particular
   unlocks parallel execution its serial suite currently forgoes.
   `internal/testsupport.SwapVar` stays for the existing census and is not a licence to
   mint a new seam; its doc comment gains a pointer to
   `code-design/test-design:no-new-global-seams` in the same Implemented transaction.
   The boundary with dependency-composition holds: `direct-injection-first` owns how
   production code composes its dependencies, and its `Verify:` line polices test-only
   production indirection at production-composition review; this claim names the
   test-side act itself and binds the test lane.

4. Route the judgment prose, not claims: extend the testing doc's layout part
   (.awf/docs/parts/testing/layout.md, regenerated to docs/testing.md via `./x render`;
   the sundial adopter renders its testing doc from its own parts, so no second copy is
   affected) with the table-versus-flat choice, `t.Fatal`-versus-`t.Error`, and test
   size and focus guidance, in the same Implemented transaction. That part already
   carries this species of prose. The topic text carries two preamble pointers, not
   claims: shared fixture homes to `code-design/single-home`, and error-identity
   assertion detail to `code-design/outcome-modeling:test-identity-assertions`.

5. Make the authority visible without copying its normative prose into prompts: add a
   `test-design-authority` reviewer focus item naming `code-design/test-design` to the
   adr-reviewer, code-reviewer, and plan-reviewer sidecars, comparing each list-valued
   override with the catalog default and preserving every default it replaces plus the
   sibling `outcome-modeling-authority` and `package-composition-authority` items
   wherever ADR-0199's and ADR-0200's implementations have already landed them, and
   extend the workflow chain part's per-topic consult sentences the same way, all in the
   same Implemented transaction.

6. Record the durable vocabulary: add a global-test-seam entry to the glossary source
   under .awf/docs/ and regenerate docs/glossary.md via `./x render`, naming ADR-0201 and
   `code-design/test-design`, in the same Implemented transaction.

7. Declare authority only. No production conversion rides this ADR; the 31-seam census
   becomes bounded future conversion candidates, enumerable by the roadmap's
   static-state inventory command, whose entry gains the global-seam census with this
   decision.

## State changes

- add `code-design/test-design:stdlib-assertions`
- add `code-design/test-design:no-new-global-seams`

## Consequences

The two claims sit at opposite ends of the ratchet. Stdlib-pure assertions is a genuinely
pinned property: zero violations exist, so the claim costs nothing today and guards the
one moment (the first assertion-library import) that would silently change the corpus's
character. No-new-global-seams is the opposite: a ratchet over a substantial existing
backlog, 31 seams across 11 packages with 82 `SwapVar` call sites, so readers will see
conforming and nonconforming test setups side by side until conversions land. Its
immediate cost is also near zero, but its later cost is real: constructor wiring in tests
that would have reached for a global, which is the intended pressure toward injectable
design.

The claims bind the Go lane only. If the TypeScript lane grows, its assertion and seam
discipline is a future decision, and widening `currentState.testGlobs` to reach it is a
marker-reach decision for the invariants domain, not a rider here. Both claims are
reasoned contracts anchored by the `test-design-authority` focus item, the chain consult
sentence, and their `Verify:` instructions; nothing in the gate fails on a violation.

The 31-seam census stays in place until deliberate conversions land, and
`internal/worktree`'s serial test execution persists with it; conversion there carries a
concrete payoff (parallel execution) beyond conformance. Because a new test may still
swap an existing seam, `SwapVar` call counts can keep growing until the seams themselves
convert; only the seam count is ratcheted.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| A depguard/forbidigo lint ban on assertion libraries now | Viable at zero conversion cost since zero violating imports exist, and it stays a bounded future candidate; this pass's settled packaging keeps all claims reasoned contracts, and a gate-behavior change deserves its own small decision rather than riding a docs transaction. |
| Update `code-design/dependency-composition:direct-injection-first` instead of a second claim | dependency-composition owns how production code composes its dependencies; test-design owns how a test obtains controlled ones. Folding the test lane's dominant anti-pattern into a production-composition claim would bury it where test authors do not look. |
| A `shared-fixture-first` claim | Dual authority: `code-design/single-home` already governs shared test-support homes; a preamble pointer suffices. |
| A proof-marker-placement claim | Marker placement is a contract of the invariants domain's marker system, not code design; that territory already has a dedicated effort. |
| Table-choice and `t.Fatal`-vs-`t.Error` claims | Neither violation is always an anti-pattern; both are judgment calls and land as guidance in the testing doc's layout part. |
| `Backing: test` for either claim | A ratchet scoped to new or deliberately converted tests has no stable population a proof-marked test can assert without inventorying the whole corpus; the anchor and Verify: lines carry enforcement. |
| Widening `currentState.testGlobs` to the TypeScript lane | A marker-reach decision for the invariants domain with its own consequences; nothing in this decision requires it. |
| Split assertion style and seam discipline into two topics | Both rules are read at the same review moment on the same diff; two one-claim topics would double navigation without adding authority. |

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Accepted; content-sha256: 75442eb0251d20dbe4fed1bfe4581f2ca64cd3ed0dfa86e9a6bb719673d51fca
