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

## Bare `awf check` should run every enabled check and report what ran

ADR-0159 was deliberately the first of two decisions: it renamed and regrouped
the verification commands without changing what bare `awf check` does. The
follow-on makes bare `awf check` run drift, state, invariants, and, when their
config knobs are on, the prose and working-memory-citation scans, then report
every check with a ran or skipped verdict and a reason for each skip. The
requirement as the user framed it during the ADR-0159 brainstorm: it should
clearly state what ran and what did not.

Four contracts belong to that decision, all named in ADR-0159 and left
untouched there. The prose and memory scans call `snapshot.IndexTree` before
consulting their own knob, so today a disabled gate hard-errors outside a git
repository instead of reporting itself skipped; the knob check has to move
ahead of the index read, and what bare check does outside git while a knob is
ON still needs deciding. The pre-commit payload and `./x gate` between them
invoke each scan three times, which the report makes visible and which should
be pruned in the same decision. The exit-code contract when every check is
skipped is unsettled. And `--staged` could widen from the bare form to the
children once a report exists to disclose a skip honestly, which ADR-0159
Decision 3 records as the reason it stayed bare-form-only.

One open design question has no home in ADR-0159 and would otherwise be lost.
`examples/sundial` is the deliberately smell-free showcase adopter, and it
enables neither opt-in scan, so a faithful ran/skipped report prints two skip
lines in the one rendered tree held up as the clean example. Either those lines
are acceptable output for a healthy project, or the report suppresses a check
whose knob is off, which weakens the very disclosure the decision exists to
provide. Settle that before writing the report format, not after.

## A frozen-state ADR flip can smuggle unreviewed section content

The commit that moves an ADR to Accepted, Implemented, or Abandoned may also mutate the ADR's
digest-covered sections, because the digest is recomputed at the flip: an amendment folded into
the freeze commit is frozen without a fresh-context review. The 0154 effort did exactly this
(the forward-only resolution clause landed in the flip commit) and only reviewer diligence
caught it afterwards. Candidate `awf audit` advisory rule: flag a status transition into a
frozen state whose commit also changes the ADR's digest-covered section content relative to the
parent snapshot. Deferred because audit rules ship behind their own decision; the pitfalls
entry recording the occurrence is the interim memory.

## Decomposing the `internal/project` god object

`internal/project.Project` carries roughly ninety-five production methods across
thirty production files, imports seventeen internal packages, and is imported by
exactly two. Fourteen of those files touch no `Project` field at all, which is
the clearest signal that several cohesive units are sharing one type.

The split has been deferred repeatedly and, until now, was recorded only in
ephemeral working memory under `.awf/efforts/`, so it vanished whenever an
effort finished. This entry is the durable record.

Two of its three prerequisites are settled. ADR-0178 established
`code-design/dependency-composition`, so dependency direction and wiring have an
authority to answer to. ADR-0180 established `code-design/state-ownership` and
converted the three per-invocation derived fields, so the type no longer holds
state written after construction and a future package boundary cannot inherit a
hidden cache. The remaining prerequisite is a package-cohesion and boundary
pattern, which is where the deferred `receiver-reads-owned-state` rule belongs:
a method reads at least one receiver field, and behaviour that reads none takes
parameters instead. Its evidence is already collected, the fourteen zero-field
files and the four synthetic partial `Project` literals.

Sequencing matters more than usual here. Half of "where does a package boundary
go" is dependency direction, which `dependency-composition` already owns, so a
cohesion pattern authored without reference to it would create dual authority.
The decomposition itself should follow the pattern rather than accompany it.
