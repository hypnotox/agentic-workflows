---
status: Proposed
date: 2026-07-15
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [invariant-backing, review-agents]
related: [8, 105, 106]
domains: [invariants, rendering]
---
# ADR-0114: Invariant backing is a ledger, not a proof


## Context

awf's invariant system lets an ADR declare a backed `` `invariant: <slug>` `` and requires a
matching proof comment `<marker> invariant: <slug>` on a test file (ADR-0008, with test-file scoping
made enforceable by ADR-0105). The check that verifies this backing is a textual line-prefix match:
`internal/invariants/invariants.go` strips leading whitespace from each line and tests
`strings.HasPrefix` against the marker, then matches the slug. It has no awareness of whether the
marker sits inside a test function or relates to any assertion. The 100%-coverage gate (ADR-0012)
forces the backing test file to *execute*, but a test that executes the line without asserting on its
effect still satisfies the gate. So a marker binds an ADR to a test *location*, not to a test that
would fail if the invariant were violated.

This is a rigorous **ledger** (the ADR-to-marker bookkeeping is tightly policed: symmetric
backed/unbacked enforcement, retirement, dangling-marker detection) but a weak **proof** (nothing
binds a marker to an assertion). The 2026-07-15 whole-project deep analysis
(`docs/research/deep-analysis-2026-07-15.md`) named this as the repository's central over-claim: an
agent could "back" a fabricated invariant with a trivial, non-asserting test and pass every
mechanical check.

The insight itself is not new. ADR-0105 already recorded that "the system is an accountability index,
not a proof" and that "the semantic gap, whether the test asserts the right thing, remains unclosable
trust"; ADR-0106 frames the surfaced backing state as a "risk map." What is unresolved is that the
*adopter-facing prose still contradicts that acknowledgement*: several shipped surfaces describe a
backed invariant as a "test-proven property," and no shipped review lens is charged with the semantic
check the scanner structurally cannot perform.

Two constraints shape the fix. First, awf is **language-agnostic**: proof markers live in the
adopter's test files, which may be any language, so a Go-AST binding that reads test bodies is
rejected (it would lock the shipped mechanism to Go). Second, the scanner staying a textual ledger is
the *correct* design, not a defect to engineer away; the gap is closed by prose honesty plus a human
(or agent) reviewer, not by more machinery. Raising the scanner's own floor (rejecting column-0
markers, an optional per-project `testFuncPattern`) and hardening awf's own Go invariant tests with
mutation testing are separate, already-tracked efforts and are out of scope here.

## Decision

Correct the positioning so awf's adopter-facing prose matches the ledger-not-proof reality, and
assign the semantic check to the review agent.

1. **Frame backing as a ledger entry, not a proof.** Replace the "test-proven property" /
   "test-backed property" wording on the shipped surfaces that describe a backed invariant with
   wording that says a test is *declared to back* the slug. The change is a correction of the claim,
   not a rename: "proof marker" / "proof comment" stays the mechanical name of the marker.

2. **State the limitation once, canonically, in the invariants narrative.** Add a short "backing is a
   ledger, not a proof" caveat to the invariants domain current-state doc, modelled on and
   cross-referenced with the existing "Coverage is not verification" section in the testing doc: the
   scanner confirms a marker exists in scope; it does not confirm the test asserts the invariant, and
   that semantic check is the reviewer's job.

3. **Charge the `code-reviewer` with the semantic check.** Extend the shipped `code-reviewer`
   testing-discipline lens so it explicitly checks that a backing test actually asserts the invariant
   it backs, treating a marker over a trivial or non-asserting test as a false backing. The lens ships
   to every adopter because it is a general property of the marker-based backing mechanism. The
   `plan-reviewer` and `adr-reviewer` are not charged: the backing test lives in the implementation
   diff, which only `code-reviewer` sees.

4. **Glossary.** Add a glossary term that defines invariant backing in ledger-versus-proof terms, so
   the distinction is discoverable outside the ADR corpus.

This ADR partially operationalizes ADR-0105 and ADR-0106 (it acts on their already-stated
acknowledgement); it does not supersede them.

## Invariants

This decision is a positioning correction and a review-agent responsibility. Both are textual
contracts, not machine-checkable properties: the very gap this ADR names is that no mechanical check
can confirm a test asserts what it claims, so "the reviewer performs the semantic check" cannot be
gated without reintroducing the rejected AST binding. Consistent with ADR-0112, this ADR declares no
new backed or unbacked invariant slug. The correctness of the prose and the lens is confirmed by the
implementation review and by `awf check`'s existing drift gate over the rendered surfaces.

## Consequences

- Adopter-facing prose stops over-claiming, which removes the "but you never test that the test
  asserts anything" critique and makes the documentation honest about the deliberate ledger/reviewer
  split.
- Every adopter's `code-reviewer` gains an explicit duty to catch false backings. This is a new
  standing responsibility, not a one-time check; it is the human/agent counterpart to the mechanical
  scanner.
- The limitation is now stated in two linked places (testing doc and invariants narrative) rather
  than implied. A future reader learns *why* backing is not proof at the point they learn what backing
  is.
- No gate changes and no new slug, so no adopter re-render risk beyond the standard prose re-render;
  `awf upgrade` is not required for this change alone.
- Out of scope and still open: raising the scanner's textual floor (column-0 rejection, optional
  `testFuncPattern`) and making mutation testing usable against awf's own Go invariant tests. Those
  are the engineering half of the deep-analysis finding and are tracked separately.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rename "proof marker" to "backing/ledger marker" everywhere | Disproportionate churn across templates, skills, docs, glossary, and many ADR bodies for a framing fix; "proof marker" is embedded vocabulary. The over-claim is the *implication that backing proves the property*, not the marker's name. |
| Bind the marker to an assertion via AST parsing | Rejected by language-agnosticism: proof markers live in adopter test files of any language, so a Go-AST binding would lock the shipped mechanism to Go. |
| Reviewer lens only, leave the prose | Leaves the "test-proven property" over-claim standing, so it does not actually correct the positioning. |
| Record the caveat only in this ADR | ADRs are append-only history; a caveat about a live mechanism belongs in the current-state narrative an agent reads via `awf context`, not solely in the decision log. |
| Treat as a non-load-bearing prose edit (no ADR) | The change assigns a new standing review responsibility and states a deliberate ledger/reviewer division of labour the project must remember; that is load-bearing. |
