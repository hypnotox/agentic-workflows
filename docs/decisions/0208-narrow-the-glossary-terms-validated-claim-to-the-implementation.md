---
format: current-state-v3
slug: narrow-the-glossary-terms-validated-claim-to-the-implementation
status: Implemented
date: 2026-08-01
---
# ADR-0208: Narrow the glossary-terms-validated claim to the implementation

## Context

ADR-0207 moved `data.terms` to a list of records and updated
`rendering/guide-and-doc-templates:glossary-terms-validated` to match. The updated body ends
"fails the render, naming the sidecar path and the offending term".

That closing clause is broader than the implementation. Eight of the eighteen violations
`internal/project/glossary.go` rejects can name no term, because the term itself is what failed to
parse, and three of them are conditions the claim enumerates by name:

- an empty term fails with `record 0: term is empty`
- a missing, null, or non-string term fails with `record 0: missing "term"` or
  `record 0: "term" must be a non-empty string`
- a malformed record fails with `record 0 must be a mapping`
- a non-string mapping key fails with `record 0: key 42 is not a string`

The implementation is deliberate and correct: `glossaryTerm` reads the term first precisely so
every later violation can name it, and falls back to the record index only where there is no term
to name. The test comment states this accurately. Only the claim sentence dropped the qualifier.

ADR-0207 cannot correct itself. It declares exactly one `update` for this claim and its second
Applied batch spent it. An ADR declares any given operation at most once, enforced at parse
(`state changes names <id> more than once`) and again in `OperationProgress`
(`applies operation ... more than once`). Amendment does not help either: an `Amended` event
revises an ADR's prose, while claim mutations are authorized only by a newly appended Applied
batch, so a later body edit fails the staged check with `claim <id> was changed with no ADR update
operation in this transition`. A successor decision declaring its own `update` is the mechanism the
corpus already uses for changing an established claim.

## Decision

1. Narrow the claim's closing clause to name the record index where the term did not parse:
   "fails the render, naming the sidecar path, and the offending term where the term itself
   parsed." The enumeration of failing conditions is also corrected to say "an empty, missing, or
   non-string term" rather than "an empty term", since a missing or non-string term fails
   identically and the claim should enumerate what it actually covers.

2. Nothing about the implementation changes. This decision moves the claim to where the code
   already is, rather than moving the code to where the claim said it was. Making every failure
   name a term is not available: an empty or missing term has no term to name, so the broader
   wording was never satisfiable.

## State changes

- update `rendering/guide-and-doc-templates:glossary-terms-validated`

## Consequences

A claim that is read as current authority stops over-promising. The correction is small, but the
claim is exactly the kind of surface this repository treats as binding, and a wrong clause in it is
the failure mode the two-layer glossary work was undertaken to reduce.

The cost is one decision record for one sentence. That is the price of the one-update-per-claim
rule, and paying it here is cheaper than leaving the claim wrong until some later decision happens
to revisit the record model. The alternative of deferring had no scheduled date.

Because this ADR declares a single operation, it never occupies `Implementing`: that status
requires at least one operation still remaining. It goes from `Proposed` to `Implemented` in one
transition, where the implicit terminal application applies its only operation alongside the claim
edit it authorizes.

## Alternatives Considered

**Amend ADR-0207.** Rejected because it does not work, verified three ways rather than reasoned
about: a duplicate `update` line fails at parse, a claim edit carrying only an `Amended` event
fails the staged check on authorization, and a vacuous `Amended` event fails on the digest. The
limit is now recorded as a pitfall, since the amendment-until-terminal rule reads as though it
covers this case.

**Defer the narrowing into the planned context-surfacing decision.** That decision revisits the
record model and could carry the update. Rejected because it has no scheduled date, and the clause
is live authority in the meantime.

**Widen the implementation to match the claim.** Rejected as impossible: a record whose term is
empty, missing, or non-string has no term to name, so no implementation can satisfy the original
wording.

## Status history

- 2026-08-01: Proposed
- 2026-08-01: Implemented; content-sha256: ea6748992565fc731f12c6b343b57ca3181488e3cb2819db39e2add3116df738
