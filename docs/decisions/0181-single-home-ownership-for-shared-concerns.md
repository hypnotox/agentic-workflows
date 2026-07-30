---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0181: Single-home ownership for shared concerns

## Context

The 2026-07-30 full-repo code-design audit (docs/research/code-design-audit-2026-07-30.md)
measured a clean split. The repository carries roughly seven ad hoc single-source claims
(`kind-dispatch-single-table`, `cli-command-spec-single-source`, `catalog-go-single-source`,
`git-range-parser-single-definition`, `frontmatter-split`, `audit-shares-adr-parser`,
`commit-gate-shared-rule`), and every concern one of them governs is consolidated and clean.
Every comparable concern without such a claim has forked into two to six parallel
implementations, and at least four have already diverged in behaviour: markdown fence
recognition exists at three fidelity levels across six sites (one a hand-copy inside the same
package, `internal/adr/format.go:230-255`, whose comment says it "mirrors sections()");
native-git invocation exists in five shapes with environment isolation applied at two of
seven sites and two cleanliness oracles reading different porcelain versions; the severity
rank existed in three enums with two spellings until the in-flight severity-unification
chain began consolidating it into one shared package; and
the two staged-scanner gates diverged on their scannability rule. Whether a shared concern
stays single-homed is currently decided per ADR, not by standing authority; the audit's
conclusion is that the mechanism works and its coverage is the gap.

One force actively produces forks. `internal/plan/plan.go:81-85` documents that its fence
scanner "deliberately drops `~~~` to avoid an uncovered branch" (ADR-0111): under the 100%
coverage gate, a shared full-fidelity implementation creates branches a narrow consumer
cannot exercise, so the cheaper move at each site has been a private, weaker copy. The user
ruled on this tension during sanctioning: code quality and architecture outrank escape
minimization, and a refactor's payoff is the architecture itself.

One scope lesson is measured rather than hypothetical. `kind-dispatch-single-table` is
scoped "in the project package", and four kind facts live just outside it in `cmd/awf`
(`cmd/awf/list_add.go:110` `isGraphKind`, `cmd/awf/new.go:31`, `cmd/awf/list_add.go:235,248`).
A single-home claim scoped to a package or to `internal/` invites the fork to move one layer
up; the claim must reach the whole module.

Legitimate divergence exists and must not be conflated with forking. ADR-0073 deliberately
declined to reuse `internal/changelog`'s parser for repoaudit's unreleased-section check
because the two read materially different grammars from different sources, and it recorded
that reasoning in the decision and at the site. A uniqueness rule that cannot express this
distinction would either be violated by a reasoned decision or force wrong mergers of
coincidental similarity; the repo's DRY posture (docs/maintainable-code-design.md) already
draws the line at shared policy, not textual likeness.

This decision deliberately creates no adapter-shape or lifetime authority.
`code-design/dependency-composition` (ADR-0178) owns where a dependency is selected, how a
seam is shaped, and where a mechanism adapter lives; `code-design/state-ownership`
(ADR-0180) owns what a value keeps after construction. Neither states how many
implementations a concern may have. Folding uniqueness into either would make one topic's
claims answer two questions, the argument ADR-0180 itself used to stay separate from
ADR-0178.

Enforcement has a working precedent at both ends. Where a concern has a mechanical
signature, a repo-walking test works: `git-range-parser-single-definition` (ADR-0127) fails
the suite if a second range parser appears. Where "same policy" is a judgement call, the
working precedent is the reviewer focus item, the posture ADR-0178 and ADR-0180 both took
for their reasoned claims; `focusItems` on the three agent sidecars replaces catalog
defaults wholesale, so additions must compare and backfill.

## Decision

1. Add `code-design/single-home` with `applies: global` to the existing pathless
   `code-design` domain. Its identified claims are the durable authority for the rules
   below; this ADR remains historical rationale. Like its two sibling topics, the topic
   governs concerns introduced by new work and forks deliberately converted under its
   authority, and does not make existing parallel implementations nonconforming debt to
   sweep.

2. A policy or mechanism consumed from more than one package has exactly one
   implementation, living in the package that owns the concern, and every consumer uses
   it. The scope is the whole module: `cmd/` executables, test-support packages, and
   rendered runtime sources included. A second implementation is a defect, not a variant,
   and a narrow consumer configures the shared implementation rather than reimplementing
   a reduced copy.

3. Two implementations of similar-looking behaviour are permitted only when they answer
   materially different contracts from distinct sources, and that reasoning is recorded in
   a durable decision and referenced at the site. ADR-0073's unreleased-section extractor
   beside `internal/changelog`'s release parser is the model. Undocumented similarity is
   treated as a fork until reasoned otherwise.

4. A shared implementation is never forked, narrowed, or degraded to avoid an uncovered
   branch or reduce a coverage-escape count. A branch a narrow consumer cannot exercise
   takes a reasoned `// coverage-ignore` or is exercised through the owning package's
   tests. Escape count is an outcome of design, never a design input.

5. Both claims are reasoned contracts (`Backing: unbacked` with `Verify:` instructions).
   Where a converted concern has a mechanical signature (an import, a call pattern, a
   literal), its conversion may add a repo-walking test in the ADR-0127 style; such a test
   backs a concern-specific claim in the topic that owns that concern, not these global
   claims.

6. Add one reviewer focus item naming `code-design/single-home` to
   `.awf/agents/adr-reviewer.yaml`, `.awf/agents/code-reviewer.yaml`, and
   `.awf/agents/plan-reviewer.yaml`, comparing against and backfilling the catalog
   defaults each list replaces.

7. The concrete first consumer is the git-access conversion, decided by its own ADR next
   in this effort and executed by its plan: it converts the git area's forks under this
   authority (the duplicated native runner, two branch-existence probes, two cleanliness
   oracles, three stderr-enrichment copies, the duplicated resident-root resolution, and
   the split fixture-construction lane). This ADR converts nothing itself.

8. The remaining audit-named forks are bounded future candidates under item 1, recorded
   here for discoverability and not converted by this decision: markdown block-structure
   recognition (whose conversion must re-decide the ADR-0111 fidelity split under item 4),
   durable-write tiering, the context-spill notice format constant, template-identifier
   resolution, owner-validated no-follow file I/O, and YAML frontmatter emission.

## State changes

- add `code-design/single-home:single-implementation`
- add `code-design/single-home:no-coverage-fork`

## Consequences

awf gains a third code-design authority answering a question the first two leave open: not
where a dependency comes from or what a value keeps, but how many implementations a shared
concern may have. Future conversions cite the topic instead of re-litigating the principle,
and the audit's fork backlog becomes a conversion queue with a standing rule behind it.

Both claims are reasoned rather than test-backed, so their enforcement is reviewer focus
items and `Verify:` instructions, the same posture and the same noise risk as ADR-0178 and
ADR-0180 accepted. The claims stay narrow through the new-or-deliberately-converted
qualifier and the item 3 reasoned-divergence clause; the failure mode that clause guards is
a reviewer forcing a merger of coincidentally similar code, which the DRY posture already
forbids.

Item 4 removes the incentive that produced at least one recorded fork. ADR-0111's stated
rationale for dropping tilde fences stops describing current preference the moment this
decision is accepted; the ADR remains accurate history, and the markdown-scanner conversion
under item 8 is where the fidelity split is actually re-decided. A reader of ADR-0111 needs
this ADR for why its trade-off is no longer made.

The sidecar edit in item 6 lands in files that two in-flight chains also touch: the
severity-unification chain and the state-ownership plan (both in managed worktrees, their
ADR numbers pinned only at integration) edit the three agent sidecars or adjacent
current-state parts. Application of this decision
sequences after those transactions integrate, which is an ordering cost, not a conflict of
substance.

The rule has a judgement burden by design. "Same policy" has no mechanical oracle, so
review carries the load where no walker exists; item 5 keeps the mechanical option open
per concern without promising it globally. The git-access conversion gives the topic a
first consumer whose evidence is unambiguous, including a duplicated runner that is
byte-for-byte identical across a package boundary and a dead injected git dependency whose
only caller is a coverage-satisfying test.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Fold the uniqueness claims into `code-design/dependency-composition` | Dependency selection and implementation count are different questions; merging them makes one topic's claims answer two, the argument ADR-0180 used to stay separate. |
| A general adapter-pattern topic | ADR-0178's `mechanism-adapters` and `direct-injection-first` already own adapter shape; a second general topic splits one question across two authorities. |
| Keep deciding single-homing per concern in each ADR | The audit measured the result: every claimed concern clean, every unclaimed comparable concern forked, four with behavioural divergence. |
| A mechanical duplication gate | No reliable oracle distinguishes shared policy from coincidental similarity; per-concern walkers remain available under item 5 where a mechanical signature exists. |
| State the coverage-gate rule as prose in docs/testing.md or the design doc | Generic guidance is not review-anchored; a topic claim is specific and reviewable, the same argument ADR-0180 made against a doc section. |

## Status history

- 2026-07-30: Proposed
