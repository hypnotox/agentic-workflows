---
status: Implemented
date: 2026-07-15
tags: [invariant-backing, review-agents]
related: [8, 12, 105, 106, 112, 116]
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
the *correct* design, not a defect to engineer away; the gap is narrowed by prose honesty plus a
best-effort human (or agent) reviewer, not by more machinery, and is never fully closed. Raising the scanner's own floor (rejecting column-0
markers, an optional per-project `testFuncPattern`) and hardening awf's own Go invariant tests with
mutation testing are separate, already-tracked efforts and are out of scope here.

## Decision

Correct the positioning so awf's adopter-facing prose matches the ledger-not-proof reality, and
assign the semantic check to the review agent.

1. **Frame backing as a ledger entry, not a proof.** Replace the "test-proven property" /
   "test-backed property" wording on the shipped surfaces that describe a backed invariant with
   wording that says a test is *declared to back* the slug. The four source surfaces are
   `.awf/parts/adr-readme/invariants.md`, its template default `templates/adr-readme/README.md.tmpl`
   (both must change in lockstep, or a fresh adopter with no part still renders the over-claim),
   `templates/adr-template/template.md.tmpl`, and `templates/skills/proposing-adr/SKILL.md.tmpl`
   (discover with `grep -rn 'test-proven\|test-backed' .awf templates`). The append-only ADR-0105 and
   ADR-0106 bodies also carry the phrase and are left untouched as historical record. The change is a
   correction of the claim, not a rename: "proof marker" / "proof comment" stays the mechanical name
   of the marker.

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

This decision is a positioning correction plus a standing review responsibility. It declares no new
backed or unbacked invariant slug: the property it establishes is a division of labour, not a
mechanically checkable state. It is stated instead as a marker-free textual contract, following
ADR-0112's precedent of textual Invariant bullets that add no slug and no gate.

- The `code-reviewer` testing-discipline lens owns the semantic-assertion check the marker scanner
  cannot perform: a backing test must actually assert the invariant it backs, and a marker over a
  trivial or non-asserting test is a false backing. This is a textual contract, not a backed slug.

An `unbacked-invariant:` slug (a reasoned contract carried by a `Verify:` note) was considered and
rejected. A `Verify:` note could only confirm that the lens text renders, which the drift gate
already guarantees, not that the reviewer discharges the duty on any given review. A slug would
relocate the over-claim this ADR corrects into the invariant ledger rather than remove it: whether a
reviewer applies the lens is, by the same argument the ADR makes about the scanner, unclosable trust.

## Consequences

- Adopter-facing prose stops over-claiming, which removes the "but you never test that the test
  asserts anything" critique and makes the documentation honest about the deliberate ledger/reviewer
  split.
- Every adopter's `code-reviewer` gains an explicit duty to catch false backings. This is a new
  standing responsibility, not a one-time check; it is the human/agent counterpart to the mechanical
  scanner.
- The reviewer lens is non-deterministic: a human or agent reviewer can still miss a false backing.
  This ADR relocates the unclosable trust ADR-0105 named from the scanner to the reviewer; it narrows
  the gap with a best-effort check, it does not close it. That residual is the honest counterpart of
  the correction, and is the reason no slug is claimed for the reviewer duty.
- The limitation is now stated in two linked places (testing doc and invariants narrative) rather
  than implied. A future reader learns *why* backing is not proof at the point they learn what backing
  is.
- No gate changes and no new slug, so no adopter re-render risk beyond the standard prose re-render;
  `awf upgrade` is not required for this change alone. The implementing commit that flips this ADR to
  Implemented runs `./x sync` in the same commit to re-render the edited prose surfaces and regenerate
  `docs/decisions/ACTIVE.md`.
- Out of scope and still open: raising the scanner's textual floor (column-0 rejection, optional
  `testFuncPattern`) and making mutation testing usable against awf's own Go invariant tests. Those
  are the engineering half of the deep-analysis finding and are tracked separately.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Rename "proof marker" to "backing/ledger marker" everywhere | Disproportionate churn across templates, skills, docs, glossary, and many ADR bodies for a framing fix; "proof marker" is embedded vocabulary. The over-claim is the *implication that backing proves the property*, not the marker's name. |
| Bind the marker to an assertion via AST parsing | Rejected by language-agnosticism: proof markers live in adopter test files of any language, so a Go-AST binding would lock the shipped mechanism to Go. |
| Reviewer lens only, leave the prose | Leaves the "test-proven property" over-claim standing, so it does not actually correct the positioning. |
| Declare the reviewer duty as an `unbacked-invariant:` slug | Its `Verify:` note could only confirm the lens text renders, which the drift gate already guarantees, not that the reviewer discharges the duty on a given review; a slug would relocate the over-claim into the ledger rather than remove it. |
| Record the caveat only in this ADR | ADRs are append-only history; a caveat about a live mechanism belongs in the current-state narrative an agent reads via `awf context`, not solely in the decision log. |
| Treat as a non-load-bearing prose edit (no ADR) | The change assigns a new standing review responsibility and states a deliberate ledger/reviewer division of labour the project must remember; that is load-bearing. |
