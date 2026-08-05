
The former cutoff-based integration incident is superseded by intrinsic ADR formats and stale-merge authorization. Schema generation 31 retires permanent format cutoffs; a real merge may instead import an exact older-format incoming-parent ADR when its final message has the shared authorization trailer pair. `awf audit` replays that authorization for committed schema-31-and-later merges. The historical account below is retained as the incident record that motivated the successor decision.

Three separate enforcement rules refused a legal integration, and the 2026-08-01
named-proof-markers integration had to relax all three inline to land. They needed one
decision record covering the class, against the ADR-0202/0203/0204 lineage.

The shape: an effort branch forked before schema generation 29 merges an integration
branch already past it. ADR-0202's hand-rename path for a slugless record whose number
was taken assumes a free number below the v3 cutoff, but the numbering was dense through
0202 with the cutoff at 203 and `legacyAdrGaps` empty, so the record could only be
renumbered INTO the v3 range. Living there forces the v3 encoding, and that retrofit is
what the three rules refuse.

What was relaxed. `validatePermanentLockTransition` gained an inherited-cutoff edge: the
transition crosses generation 29 in one step, so the cutoff arrives from the other parent
already computed against a corpus neither tree holds, and re-deriving it would lower it
under records already sealed above it. `renumberAliases` keeps its before side
slugless-only, exactly as ADR-0204 item 5 wrote it, and admits one further record on the
after side: a NUMBERED record whose slug is new in the transition, which is what a record
renumbered across the cutoff becomes when it takes the v3 encoding. Requiring a number is
what keeps the opening shut, since a pending record can only ever be an addition. And the
governed-format-change rule admits exactly a v2 record renumbered to a v3 one.

A first attempt at that middle relaxation admitted any record whose slug was new on its own
side, and terminal review found it accepted four transitions the rule had refused: a
numbered record losing its number, a retained v3 slug changed at one number, a
frozen-content amendment escaping through an unrelated pending twin, and a genuine deletion
laundered into a rename. That is the strongest argument for writing the decision rather
than leaving these three as inline relaxations.

Two current-state sentences are FALSE as written until that decision lands, both authored
by terminal records so neither can be corrected without one. `config/migrations-and-locks:
adr-v2-cutoff-atomic-immutable` says the transition admits exactly two edges and that
"Both edges require the new value to equal the corpus's computed next identity"; the
inherited-cutoff edge is a third and derives nothing, taking the published value on trust.
`adr-system/adr-lifecycle:renumber-digest-paired` is false in two independent clauses: it
says "The digest step considers only a record carrying no slug", where it now also
considers a numbered record whose slug is new in the transition, and it says the step
"re-keys only where the two ends hold different numbers", which the code has never done
since a slugged after end keys on its slug and the numbers are never compared. The second
clause predates this work; the first sentence of `renumberAliases` carries the same
inaccuracy. The neighbouring `adr-number-immutable` and `adr-slug-frontmatter-mandatory`
were briefly falsified by a first, too-broad version of the pairing change and are true
again under the shipped rule.

What the decision should settle: whether these three belong together as one sanctioned
"stale branch crosses the seal" transition or are three independent edges; whether the
inherited cutoff can be verified against anything rather than taken on trust, which is
the weakest of the three; and whether a fourth rule is waiting behind them for the next
branch older than this one.

The review that caught the first version is the argument for writing it soon. That version
read correctly and silently accepted four transitions the rule had refused, including
laundering a genuine deletion plus an unrelated pending addition into a rename, which is
the fail-closed promise ADR-0204 item 4 makes explicitly.

## Mechanically detecting a nominal invariant proof

`invariant-proof-exercises-its-claim` has now failed to prevent three sessions
of partially-backed proof markers, the last shipping roughly nine at once and
hiding a real defect behind a green gate. A standing instance sits in
`internal/migrate/dropworkflowtelemetry_test.go`, whose
`workflow-telemetry-config-migration` marker covers a body that only pins the
current schema generation, while the claim is about generation 21 removing two
resident roots: the marker exercises nothing it backs, and no gate notices. It has been strengthened from a
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
whose contents do not match its message.
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

## A direct first-stamp ADR flip can smuggle unreviewed section content

Since ADR-0188, the stamp chain makes every status flip after the first content stamp
content-pure by validation: a post-Accepted amendment must append its own Amended event in a
separate commit. The residual case is a direct status flip out of Proposed whose commit also
mutates digest-covered content, establishing the first stamp over unreviewed content in one
transaction. A direct terminal flip also freezes that content immediately. The 0154 effort did exactly this when its direct Implemented flip also landed
the forward-only resolution clause, and only reviewer diligence caught it afterwards. Candidate
`awf audit` advisory rule: flag that direct flip shape. Deferred because audit rules ship behind
their own decision; the active pitfalls rule still depends on manual review.

## Decomposing the `internal/project` god object

Decided and executed by ADR-0195: `internal/contextq` (the context query behind
the `ContextState` seam) and `internal/resident` (resident-root policy and
anchoring) are carved out, the core keeps the cycle-bound sync engine, and the
export surface shrank with each carve. The ADR's Context records why the
sequencing reversed relative to this entry's earlier prescription: the
boundaries were measured empirically (a cluster map, two verified cycles, a
per-symbol coupling census), which grounds this package's split more strongly
than a generic cohesion pattern would, and the direction half of any such
pattern is already owned by `code-design/dependency-composition`.

What stays open is the generalization, not the split: the deferred
`receiver-reads-owned-state` rule (a method reads at least one receiver field;
behaviour that reads none takes parameters instead) remains unowned by any
topic and belongs to a future package-cohesion pattern that generalizes from
ADR-0195's evidence rather than gating it. Further decomposition of the
remaining core is likewise accepted at decision time as future-effort
territory rather than silent scope; this entry is that record.

## A `coverage-ignore` the profile records as executed is a false ignore

The `coverage-ignore-reachability` review item and the manual pitfalls rule have now failed
to prevent eight recurrences of a false exclusion. Both are rung-3 and rung-4 controls:
probabilistic, and applied only when a reviewer runs. One whole subclass is mechanically
decidable and needs no judgement at all: an exclusion sitting on a guarded body that the
coverage profile records as EXECUTED is false by construction, because the branch it declares
unreachable was just reached.

`cmd/covercheck` already parses the profile and already applies the ignore filter, so it holds
both halves of the comparison; the rule is to fail when a block is both ignored and counted.
That turns the most common shape into a gate failure at write time rather than a finding a
reviewer may or may not reach.

What deferred it until now was an unmeasured cost, which the 2026-07-30 state-ownership review
finally supplied: a sweep found EIGHT excluded-but-covered sites currently in the repository,
seven of them predating that session. So the work is the rule plus its own tests at the 100%
floor, plus resolving eight existing sites that span several efforts' territory, each needing
its own judgement about whether to delete the exclusion or cover the branch. That is a bounded
but real slice, and it should be one deliberate effort rather than a drive-by.

Worth settling in the same decision: whether the rule is an error or a warning during the
transition, and whether `./x audit-local`'s existing advisory `coverage-ignore-added` warning is
subsumed by it or kept as the complementary "touched, re-evaluate" signal.

## The rendered pre-commit payload validates the worktree, not the staged slice

A partial-staging commit whose staged subset is drift-inconsistent (a rendered,
lock, or config hunk left unstaged while the fixing hunk sits in the worktree)
passes a pre-commit gate that checks the worktree, and lands a broken HEAD. It
bit this repo at commit a85bd6a and the repo-local hook was extended on
2026-07-15 to also run `awf check` on a checkout-index slice, but the shipped
payload (ADR-0048) still checks the worktree, so adopter repos keep the gap.

A fix is feasible and language-agnostic: checkout-index to a temporary tree and
run the pinned `awf check` there, read-only, safer than `git stash
--keep-index`. It is deferred because it changes ADR-0048's deliberately
minimal, inert payload contract and adds per-commit latency to catch a
power-user footgun (adopters who stage everything never hit it), so it needs
its own ADR. The user chose repo-local-now, standard-level-recorded on
2026-07-15.

## Live-agent outcome evals

The deterministic harness-integrity half shipped as ADR-0053 and ADR-0054: a
fixture-based eval suite that pins chain handoffs and skill parity without
executing an agent. The other half, live-agent outcome evals over a golden-task
corpus (ADR-0017's original framing), stays deferred as cost-prohibitive; the
field is choking on exactly that cost, which is why the deterministic in-lane
variant was built instead. Revisit only with a concrete budget and a scoring
harness design.

## Partial-amendment back-pointer check

When an ADR body cites another ADR's specific Decision item (a partial
amendment), the cited ADR's `related:` should name the citing ADR, or the
amendment is invisible from the amended side. The promotion trigger has fired:
recorded misses at ADR-0065 (missed pointer to ADR-0079) and ADR-0093 (missed
pointer to ADR-0024), each caught only in retrospective. Deferred because the
rung is expensive: a detection heuristic over citation prose, tests at the 100%
floor, and false-positive risk on citations that do not amend. Worth its own
focused effort, and the ADR-0188 amendable-lifecycle machinery may have changed
the natural shape of the fix.

## Unmanaged Go `t.TempDir` directories survive abrupt process death

Managed TestMain homes are bounded below one recoverable root, but arbitrary Go `t.TempDir`
directories cannot clean up across abrupt process death. The test-temp manager deliberately
does not select them, so any broader cleanup policy needs a separate safety decision.

## Remove Windows from release and cross-compile policy

A future release-policy change should remove Windows from `.goreleaser.yaml` and the
cross-compile gate. Test-temp management retains Windows compile compatibility now, but owns
real behavior only on Linux and macOS; it must not approximate Windows ACL safety.
