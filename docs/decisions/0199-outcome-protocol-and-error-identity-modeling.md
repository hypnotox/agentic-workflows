---
format: current-state-v2
status: Proposed
date: 2026-07-31
---
# ADR-0199: Outcome protocol and error identity modeling

## Context

awf carries outcome identity in message text. The 2026-07-30 code-design audit
(docs/research/code-design-audit-2026-07-30.md) measured zero exported error types or
sentinels in the five largest packages, compensated by more than three hundred
`strings.Contains` assertions on error text across fifteen test packages. The typed errors
that do exist are mostly never matched: `worktree.RefusalError`, `effort.CorruptError`
(internal/effort/service.go), and the context-facet error now living in
`internal/contextq/context_paths.go` have no `errors.As` consumer between them.
`git.CommandError` and `git.HardSafetyError` are the counter-examples done right: both carry
`Unwrap`, and `internal/git/lifecycle.go:103` branches on `CommandError` through `errors.As`.

Stdlib condition matching is split. Production code holds 34 call sites of the shallow
`os.IsNotExist`/`os.IsExist`/`os.IsPermission` predicates alongside roughly as many
`errors.Is(err, fs.ErrNotExist)`-style checks, verified in the current tree. The enabled
`errorlint` linter has no check covering the shallow predicates, so nothing mechanical stops
their spread. The measurable cost already exists: `unwrappedError` in
`internal/git/controlroot.go` exists purely to strip error wrappers so `os.IsNotExist` can
see through them, and feeds five call sites.

The strongest existing outcome model is `internal/worktree`.
`RefusalError{Category, Condition, ChangedTopology, NextAction, Err}`
(internal/worktree/topology.go:15) with its `refusal` and `refusalCause` constructors has 27
production construction calls, and `Result{Condition, ChangedTopology, NextAction}`
(internal/worktree/manager.go:45) is its deliberate success-side counterpart: the `next
action:` vocabulary is not failure-only, since `cmd/awf/effort.go` prints it on a
partial-finish success path with two changed axes (`changed active rename`, `changed
cleanup`). The corpus also honours a latent rule nobody wrote down: when no axis moved, the
remedy addresses only the observed condition; when any axis moved, the remedy addresses the
residue before retrying. Every existing site obeys it.

Two of the five claim candidates this effort originally carried retired with their
predecessor: ADR-0183 through ADR-0185 unified the finding rank and severity vocabulary, so
this decision covers only what survives, the outcome protocol and error identity. The
pathless `code-design` domain (ADR-0178) is the established home for repository-wide
structure authority, and its topics govern with a ratchet: a rule is promoted to a claim
only where a violation is always an anti-pattern, the claim binds new or deliberately
converted code, and existing violations remain bounded future conversion candidates rather
than sweep debt.

## Decision

1. Add `code-design/outcome-modeling` with `applies: global` under the pathless
   `code-design` domain, as the owner of how awf represents, surfaces, matches, and tests
   outcomes, failure and success alike. The topic's identified claims are the durable
   authority for the rules below; this ADR remains historical rationale. Every claim lands
   as a reasoned contract (`Backing: unbacked` with a concrete `Verify:` instruction), and
   every claim sentence carries its "new or deliberately converted" qualifier inline, not
   only in the topic's applicability paragraph.

2. Surface refusals and partial progress through the actionable outcome protocol, derived
   from all existing `internal/worktree` sites rather than designed fresh. A conforming
   outcome carries: `Category`, a closed vocabulary of state kinds; `Condition`, a
   present-tense statement of the observed state, never a restatement of what the command
   attempted, where a failed mechanism call is itself an observed state whose detail rides
   along as the cause; `Changed`, one boolean observation per axis whose movement would make
   a naive retry unsafe, an observation of reality rather than an assertion of intent;
   `Steps`, an ordered remedy list whose entries are each independently executable; and
   `Cause`, present exactly when the condition observes a failed call.

3. Bind the remedy to the observations: when no axis moved, the steps address only the
   condition; when any axis moved, the steps address the residue before retrying. This is
   the latent rule the whole existing corpus already obeys and the strongest reviewable
   commitment in the protocol.

4. Keep rendering one line, numbering multi-step remedies (`next action: 1) ... 2) ...`);
   a single-step remedy renders exactly as today so no existing site regresses. `Category`
   keeps the state kind and drops the operation taxonomy, since the operation is already
   implied by the invoked command; the derived vocabulary (`cleanliness`, `operation`,
   `topology`, `ancestry`, `repository-identity`, `merge-conflict`) is confirmed during
   implementation. No `Constraints` field: the few prohibitions currently embedded in step
   prose stay there until enough sites earn a field.

5. Export identity a caller must branch on. In new or deliberately converted code, a cause
   a caller distinguishes is an exported sentinel or error type with `Unwrap`, matched with
   `errors.Is`/`errors.As`; production control flow never branches on message substrings.
   `git.CommandError` with its `errors.As` consumer, `git.HardSafetyError`, and
   `cmd/awf`'s `usageErr`/`dispatchErr` pair are the in-repo exemplars.

6. Retire the shallow predicates in new or deliberately converted code: stdlib conditions
   are matched with `errors.Is(err, fs.ErrNotExist)`-family identity checks, never the
   `os.IsNotExist`/`os.IsExist`/`os.IsPermission` family, which does not unwrap and forces
   wrapper-stripping helpers like `unwrappedError` into existence.

7. Assert identity in tests: a new or deliberately converted test asserts a produced
   error's identity through `errors.Is`/`errors.As` or the exported type, and asserts
   message text only where the message is itself the contract, such as rendered CLI or
   report output.

8. Ship identity with its consumer: a new exported error identity arrives in the same green
   transaction as at least one consumer that branches on it. This specializes
   `code-design/dependency-composition:concrete-first-consumer` to error identity and
   prevents the current accumulation of exported types no caller matches.

9. Declare authority only. No production conversion rides this ADR; the 34 shallow
   predicate sites, the message-text assertions, and the unmatched exported types become
   bounded future conversion candidates, each future conversion naming its concrete
   consumer. The roadmap's static-state inventory command is the intended mechanism for
   enumerating the remaining violations.

## State changes

- add `code-design/outcome-modeling:actionable-outcome-protocol`
- add `code-design/outcome-modeling:typed-outcome-for-caller-branching`
- add `code-design/outcome-modeling:errors-is-over-os-predicates`
- add `code-design/outcome-modeling:test-identity-assertions`
- add `code-design/outcome-modeling:consumed-identity`

## Consequences

Implementation and review gain one authority for outcome surfacing and error identity where
today each package improvises. The protocol claims make refusal quality reviewable: a
reviewer can check that a condition is stated in the present tense, that changed axes are
observed, and that a post-movement remedy addresses the residue, instead of judging prose
taste. The identity claims give tests something stronger than substring matching to hold
onto, and the consumed-identity rule keeps the added API surface honest.

The corpus stays mixed for a while. The ratchet deliberately leaves the existing shallow
predicates, message-text assertions, and unmatched types in place as bounded candidates, so
readers will see conforming and nonconforming sites side by side until conversions land.
All five claims are reasoned contracts: nothing in the gate fails on a violation, so review
carries the enforcement weight, aided by each claim's `Verify:` instruction.

A genuinely anticipatory error identity must wait for its first consumer, mirroring the
dependency-composition trade-off. Multi-step numbered rendering changes the byte output of
flattened remedies only when a site is deliberately converted, never as a side effect of
this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the severity-vocabulary and finding-rank claims in this topic | They retired with ADR-0183 through ADR-0185; restating them would create dual authority. |
| A consumer-owned-finding-payloads claim | It specializes `code-design/dependency-composition:consumer-owned-contracts` and would create dual authority over one subject. |
| A failure-modelling section in docs/maintainable-code-design.md | It would collide with the existing "Failure modes" section name, and the topic's value is being specific and verifiable where the guide cannot. |
| A generic "test constructs live in _test.go" claim | Broader than this topic's subject and broader than any first consumer proves. |
| Lint-ban the `os.Is*` family now | 34 existing production sites would need immediate conversion or nolint noise; the ratchet governs new code first and the inventory command follows. |
| A `Constraints` field on the outcome shape | Only about four sites embed a prohibition in step prose; a field for four sites is the speculative abstraction dependency-composition refuses. |
| `Backing: test` for the protocol claim | No mechanical detector of protocol conformance exists; a proof marker on an unrelated test would be false backing. |
| Sweep-convert the existing sites in this effort | Repository-wide scope that violates the concrete-first-consumer discipline and would obscure whether the protocol works. |

## Status history

- 2026-07-31: Proposed
