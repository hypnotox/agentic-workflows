---
format: current-state-v2
status: Implemented
date: 2026-07-31
---
# ADR-0199: Outcome protocol and error identity modeling

## Context

awf carries outcome identity in message text at nearly every error construction site,
compensated by more than three hundred `strings.Contains` assertions on error text across
fifteen test packages (2026-07-30 code-design audit,
docs/research/code-design-audit-2026-07-30.md; its five-largest-package framing predates the
ADR-0195 decomposition of `internal/project`). Outside `internal/git` the corpus holds three
exported error types, `worktree.RefusalError`, `effort.CorruptError` (declared at
internal/effort/store.go:31), and the context-facet error now living in
`internal/contextq/context_paths.go`, and no `errors.As` consumer matches any of them: the
only production `errors.As` sites in the current tree are cmd/awf/main.go:272 and four sites
inside `internal/git` (lifecycle.go:104, controlroot.go:542, runner.go:95 and :133).
`git.CommandError` and `git.HardSafetyError` are the counter-examples done right: both carry
`Unwrap`, and `internal/git/lifecycle.go:104` branches on `CommandError` through `errors.As`.

Stdlib condition matching is split. Production code holds 34 call sites of the shallow
`os.IsNotExist`/`os.IsExist`/`os.IsPermission` predicates alongside roughly as many
`errors.Is(err, fs.ErrNotExist)`-style checks, verified in the current tree. The enabled
`errorlint` linter has no check covering the shallow predicates, so nothing mechanical stops
their spread. The measurable cost already exists: `unwrappedError` in
`internal/git/controlroot.go` exists purely to strip error wrappers so `os.IsNotExist` can
see through them, and feeds five call sites.

The strongest existing outcome model is `internal/worktree`.
`RefusalError{Category, Condition, ChangedTopology, NextAction, Err}`
(internal/worktree/topology.go:15) with its `refusal` and `refusalCause` constructors has 25
production call sites in the current tree, and `Result` (internal/worktree/manager.go:45),
with 5 production construction sites, is its deliberate success-side counterpart, carrying
the same `Condition`, `ChangedTopology`, and `NextAction` trio alongside its `Path` and
`Branch` payload: the `next action:`
vocabulary is not failure-only, since `cmd/awf/effort.go` prints it on a partial-finish
success path with two changed axes (`changed active rename`, `changed cleanup`). The corpus also honours a latent rule nobody wrote down: when no axis moved, the
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

2. Surface new or deliberately converted refusals and partial progress through the
   actionable outcome protocol when the outcome observes repository, worktree, or effort
   state, the territory where retry safety is the live question and where the protocol was
   derived from all existing `internal/worktree` sites rather than designed fresh. An
   outcome observing other state remains outside the protocol claim until a successor
   decision widens the scope. A conforming outcome carries: `Category`, a closed vocabulary of
   state kinds; `Condition`, a present-tense statement of the observed state, never a
   restatement of what the command attempted, where a failed mechanism call is itself an
   observed state whose detail rides along as the cause; `Changed`, one boolean observation
   per axis whose movement would make a naive retry unsafe, an observation of reality
   rather than an assertion of intent; `Steps`, an ordered remedy list whose entries are
   each independently executable; and `Cause`, present exactly when the condition observes
   a failed call.

3. Bind the remedy to the observations: in a new or deliberately converted outcome, when no
   axis moved, the steps address only the condition; when any axis moved, the steps address
   the residue before retrying. This is the latent rule the whole existing corpus already
   obeys and the strongest reviewable commitment in the protocol.

4. Keep rendering one line, numbering multi-step remedies (`next action: 1) ... 2) ...`);
   a single-step remedy renders exactly as today so no existing site regresses. The
   numbered format is claim authority; the rendering implementation follows
   `code-design/presentation-ownership` (the package owning the outcome model renders it),
   and the moment a second package implements the numbering,
   `code-design/single-home` requires the shared helper, so this decision forks neither
   topic.

5. `Category` keeps the state kind and drops the operation taxonomy, since the operation is
   already implied by the invoked command. The derived vocabulary (`cleanliness`,
   `operation`, `topology`, `ancestry`, `repository-identity`, `merge-conflict`) is
   confirmed during implementation and lands enumerated in the claim text, so the claim's
   `Verify:` line has a membership list to check against.

6. No `Constraints` field: the few prohibitions currently embedded in step prose stay
   there until enough sites earn a field.

7. Type identity a caller must branch on. In new or deliberately converted code, a cause a
   caller distinguishes is a distinct error type or sentinel, exported when the branching
   caller sits outside the defining package, carrying `Unwrap` when it wraps a cause, and
   matched with `errors.Is`/`errors.As`; production control flow never branches on message
   substrings. `git.CommandError` and `git.HardSafetyError` are the exported exemplars with
   `Unwrap` (consumers at internal/git/lifecycle.go:104 and controlroot.go:542), and
   `cmd/awf`'s unexported `usageErr` with `dispatchErr` as its `errors.As` consumer
   (cmd/awf/main.go:272) is the in-package exemplar.

8. Retire the shallow predicates in new or deliberately converted code: stdlib conditions
   are matched with `errors.Is(err, fs.ErrNotExist)`-family identity checks, never the
   `os.IsNotExist`/`os.IsExist`/`os.IsPermission` family, which does not unwrap and forces
   wrapper-stripping helpers like `unwrappedError` into existence.

9. Assert identity in tests: a new or deliberately converted test asserts a produced
   error's identity through `errors.Is`/`errors.As` or the exported type, and asserts
   message text only where the message is itself the contract, such as rendered CLI or
   report output.

10. Ship identity with its consumer: a new exported error identity arrives in the same
    green transaction as at least one consumer that branches on it. This specializes
    `code-design/dependency-composition:concrete-first-consumer` to error identity and
    prevents the current accumulation of exported types no caller matches. One escape
    hatch: an identity may land without an in-repo branching consumer when its consuming
    caller is named and documented in the same transaction.

11. Record the durable vocabulary: add an actionable-outcome-protocol entry to the
    glossary source under .awf/docs/ and regenerate docs/glossary.md via `./x render`,
    naming ADR-0199 and `code-design/outcome-modeling`, in the same Implemented
    transaction.

12. Make the authority visible without copying its normative prose into prompts: add an
    `outcome-modeling-authority` reviewer focus item naming `code-design/outcome-modeling`
    to the adr-reviewer, code-reviewer, and plan-reviewer sidecars, comparing each
    list-valued override with the catalog default and preserving every default it
    replaces, and extend the workflow chain part's per-topic consult sentences beside the
    four existing code-design siblings, all in the same Implemented transaction.

13. Declare authority only. No production conversion rides this ADR; the 34 shallow
    predicate sites, the message-text assertions, and the unmatched exported types become
    bounded future conversion candidates, each future conversion naming its concrete
    consumer. The roadmap records a static-state inventory command (added with this
    decision) as the intended mechanism for enumerating the remaining violations.

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
carries the enforcement weight, anchored by the `outcome-modeling-authority` focus item in
the three reviewer sidecars, the chain consult sentence, and each claim's `Verify:`
instruction. Scoping the protocol claim to outcomes that observe repository, worktree, or
effort state leaves other refusals ungoverned by the protocol until a successor decision
widens it; the identity claims carry no such scope and bind globally.

A genuinely anticipatory error identity must wait for its first consumer or name and
document the caller it anticipates, mirroring the dependency-composition trade-off. Multi-step numbered rendering changes the byte output of
flattened remedies only when a site is deliberately converted, never as a side effect of
this decision.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the severity-vocabulary and finding-rank claims in this topic | They retired with ADR-0183 through ADR-0185; restating them would create dual authority. |
| Fold the rendering half into `code-design/presentation-ownership`, or split protocol and identity into two topics | Presentation-ownership owns which package renders a result model; outcome-modeling owns what an outcome must contain and how its identity is declared and matched. One subject per topic holds on that boundary, and the protocol and identity halves share one lifecycle: an outcome's shape and its matchability are two faces of the same caller contract. |
| A consumer-owned-finding-payloads claim | It specializes `code-design/dependency-composition:consumer-owned-contracts` and would create dual authority over one subject. |
| A failure-modelling section in docs/maintainable-code-design.md | The guide's sections are a fixed catalog list (internal/catalog/standard.go), so adding one is not available without changing the standard; it would also collide with the existing "Failure modes" section name, and the topic's value is being specific where the guide cannot. |
| A generic "test constructs live in _test.go" claim | Broader than this topic's subject and broader than any first consumer proves. |
| Lint-ban the `os.Is*` family now | 34 existing production sites would need immediate conversion or nolint noise; the ratchet governs new code first and the inventory command follows. |
| A `Constraints` field on the outcome shape | Only about four sites embed a prohibition in step prose; a field for four sites is the speculative abstraction dependency-composition refuses. |
| `Backing: test` for the protocol claim | No mechanical detector of protocol conformance exists; a proof marker on an unrelated test would be false backing. |
| Sweep-convert the existing sites in this effort | Repository-wide scope that violates the concrete-first-consumer discipline and would obscure whether the protocol works. |

## Status history

- 2026-07-31: Proposed
- 2026-08-01: Accepted; content-sha256: 30f44fef4616e7315c60d7a8a4504f3960c3c898f22a76a1e845b037ce2d657a
- 2026-08-01: Implementing; content-sha256: 30f44fef4616e7315c60d7a8a4504f3960c3c898f22a76a1e845b037ce2d657a
- 2026-08-01: Applied; operations: add `code-design/outcome-modeling:actionable-outcome-protocol`, add `code-design/outcome-modeling:typed-outcome-for-caller-branching`, add `code-design/outcome-modeling:errors-is-over-os-predicates`, add `code-design/outcome-modeling:test-identity-assertions`
- 2026-08-01: Applied; operations: add `code-design/outcome-modeling:consumed-identity`
- 2026-08-01: Implemented; content-sha256: 30f44fef4616e7315c60d7a8a4504f3960c3c898f22a76a1e845b037ce2d657a
