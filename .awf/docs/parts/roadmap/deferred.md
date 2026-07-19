## Pi and shared Agent Skills discovery

Resolve Pi's collision between its `.pi/skills/` output and the shared
`.agents/skills/` workflow skills that Pi also discovers when Codex is enabled.
Keep Pi's top-level reviewer skills available without duplicate workflow skill
names. ADR-0122's Pi and Codex target layouts may need a successor decision.
## Mechanically detecting a nominal invariant proof

`invariant-proof-exercises-its-claim` has now failed to prevent three sessions
of partially-backed proof markers, the last shipping roughly nine at once and
hiding a real defect behind a green gate. It has been strengthened from a
judgement item to an enumerating one, but that is still rung 3: probabilistic,
and applied only when a reviewer runs.

The rung-2 candidate is mutation testing, which this repo already has tooling
for (`cmd/mutants`, and the deterministic gremlins recipe in `docs/testing.md`).
A mutation run scoped to the check and derivation paths would kill a nominal
proof by construction: mutate the clause, and a marker whose test never
exercised it stays green. What is unresolved is the cost - a full run is slow
and advisory-only today - and whether a scoped, gate-wired subset can be made
fast and deterministic enough to block a commit. Worth an ADR if it can, since
a proof marker that cannot fail is worse than no marker at all.
## The rationale site a token cannot address

`docs/decisions/0057-sandboxed-placeholder-substitution-in-convention-parts.md`
carries a `refines: ADR-0034#1` token that parses with `CarrierItem: 0`: it sits
in the Decision section but before the first column-0 numbered item, so it has
no rationale site. ADR-0129 Decision 2 makes the carrier item the addressable
justification for a claim, and a claim with none is a record that says what
changed but not why, at the one place the model guarantees an answer.

It is deferred rather than fixed because both repairs need a decision first.
Moving the token into item 1 is a content edit to an Implemented ADR, which
append-only forbids; the alternative, widening the model to admit a
section-scoped claim, weakens the guarantee that every claim has a rationale
site. Neither is a mechanical correction, and the token is not wrong today, it
is merely unanchored, and `awf check` is silent on it.

The related shape the citation check declines to resolve is a bare `item N`
hanging off an ADR reference earlier in the same Decision item (ADR-0131
Decision 2 records the measurement behind that boundary). Both are the same
underlying question: how much structure a frozen ADR's prose can be expected
to carry.

## Mechanically catching a commit that does not contain what it claims

Three times now a concurrent session in one checkout has produced a commit
whose contents do not match its message (the pitfalls entry records all three).
The 2026-07-19 instance was the worst shape: an ADR amendment the message
described in detail was absent from the commit, leaving a proof marker
asserting the opposite of the sentence it was marked as proving. No gate can
see it. Prose has failed three times, and a code-review focus item now covers
it, but that is rung 3: probabilistic, and only when a review runs.

The rung-2 candidate is a `cmd/repoaudit` rule over the commit range: when a
commit message names an ADR with an authoring verb (amends, narrows, reopens,
flips, implements), the commit must touch that ADR's file. It is mechanical, it
would have caught the instance above, and repoaudit is the right home rather
than the shipped `awf audit`: the rule is about this repo's authoring
discipline, and repoaudit findings can be advisory, which matters because the
verb detection will have false positives (a commit legitimately citing an ADR
it does not edit).

Deferred rather than built because the cost is a new rule plus tests at the
100% floor, and the session that found it was already long. The generalisation
worth considering at the same time is the inverse direction: a file in the diff
that no part of the message accounts for, which is what catches a `git add -A`
sweeping another effort's work.
