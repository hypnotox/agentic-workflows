## Pi and shared Agent Skills discovery

Resolve Pi's collision between its `.pi/skills/` output and the shared
`.agents/skills/` workflow skills that Pi also discovers when Codex is enabled.
Keep Pi's top-level reviewer skills available without duplicate workflow skill
names. ADR-0122's Pi and Codex target layouts may need a successor decision.
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

## A direct first-stamp ADR flip can smuggle unreviewed section content

Since ADR-0188, the stamp chain makes every status flip after the first content stamp
content-pure by validation: a post-Accepted amendment must append its own Amended event in a
separate commit. The residual case is a direct status flip out of Proposed whose commit also
mutates digest-covered content, establishing the first stamp over unreviewed content in one
transaction. A direct terminal flip also freezes that content immediately. The 0154 effort did exactly this when its direct Implemented flip also landed
the forward-only resolution clause, and only reviewer diligence caught it afterwards. Candidate
`awf audit` advisory rule: flag that direct flip shape. Deferred because audit rules ship behind
their own decision; the pitfalls entry recording the occurrence is the interim memory.

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

The `coverage-ignore-reachability` review item and the pitfalls entry behind it have now failed
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

## The test suite leaks temp homes on interrupted runs

An interrupted `go test` run orphans `awf-project-test-home*` directories in the system temp
dir (13 stale ones found on 2026-07-31 while diagnosing a full 16G tmpfs; the sibling leak,
the gate's mktemp coverage profile at ~45MB per interrupted run, was fixed structurally by
ADR-0196's durable `coverage.out`). `t.TempDir` cannot clean up across a kill. Low priority:
either a periodic cleanup note or naming the fixture dirs under one parent so a stale sweep
is one `rm`.

## `awf check drift` and `awf check state`: deliberately kept, currently uninvoked

Neither subcommand is invoked by any hook payload, runner step, or CI job in this
repository - every enforcement path calls bare `awf check`, which runs both halves
together. Surveyed 2026-07-31 during the workflow-friction effort and deliberately
kept: they are cheap, tested, and harmless single-half conveniences for focused
debugging, and removing shipped CLI surface is more churn than a dormant tested
branch. Tripwire, mirroring the removed `--json` precedent: if either subcommand
starts misleading users about what bare `awf check` covers, cut it then. Do not
keep re-asking why they are uninvoked.
