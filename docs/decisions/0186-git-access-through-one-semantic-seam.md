---
format: current-state-v2
status: Proposed
date: 2026-07-30
---
# ADR-0186: Git access through one semantic seam

## Context

Git access is the most-forked concern in the repository, measured by the 2026-07-30
code-design audit (docs/research/code-design-audit-2026-07-30.md) and this effort's
grounding pass. Seven production sites construct a git subprocess in five shapes
(`internal/git/controlroot.go:457`, `internal/git/git.go:168`, `internal/worktree/git.go:20`,
`internal/effort/service.go:92,102`, `internal/effort/store.go:103`, `cmd/repoaudit/main.go:53`),
and only two apply `IsolatedGitEnvironment`, the deliberately-written policy that strips
inherited `GIT_*` state and disables credential prompts; the five that bypass it include both
paths that touch effort state. `internal/worktree/git.go:18-31` is a byte-for-byte copy of
`internal/git/controlroot.go:455-468` across a package boundary. Branch existence is probed
twice (`internal/effort/service.go:101`, `internal/worktree/git.go:65`), exit-code-1-as-answer
is hand-implemented twice more (`internal/worktree/git.go:77-90` `ancestor`), stderr
enrichment exists in three copies, and working-tree cleanliness has two oracles reading
different porcelain versions (`internal/git/git.go:168` at v2 without isolation or stderr
capture; `internal/worktree/git.go:48` at v1, isolated) that can disagree about the same
checkout. `exec.CommandContext` is threaded through five packages, yet all nine production
feeds pass `context.Background()` (`cmd/awf/effort.go:19,62,78`, `cmd/awf/sync.go:26`,
`internal/migrate/remove_workflow_residents.go:29`, `internal/migrate/unified_effort_residents.go:171,175`,
`internal/project/install.go:175`, `internal/project/project.go:131`) and no production site
sets a deadline, so a git blocked on a stale `index.lock` hangs awf indefinitely, including
inside the pre-commit hook; the `project.go:131` feed sits inside `project.Open`, which every
gated command reaches.

The library side leaks. Four packages import go-git (`internal/git`, `internal/audit`,
`internal/migrate`, `internal/testsupport/gitfixture`). Three exported
`internal/git` signatures expose go-git types (`OpenRepo`, `OpenContainingRepo` at
`git.go:537,298` returning `*gogit.Repository`; `GlobalExcludePatterns` at `git.go:220`, which
has no external production caller). Production code in `internal/migrate` matches the go-git
sentinel directly (`errors.Is(err, gogit.ErrRepositoryNotExists)` at
`remove_workflow_residents.go:24`, `unified_effort_residents.go:166`). `internal/audit/git.go:42-211`
performs a full commit-graph walk against the raw repository object (revision resolution,
merge-base, commit iteration, tree diffs with per-file stats, old and new blob text) and owns
the neutral `Commit`/`FileChange` types that walk produces. And
`cmd/awf/testmain_test.go:12` asserts awf "drives git purely through go-git", which the seven
exec sites falsify; the assertion is pre-existing drift.

Neither backend can be dropped. The worktree lifecycle, control-root topology, and
`check-ref-format` have no go-git equivalent, and go-git's semantics diverge from real git in
ways this repo has paid for three times (docs/pitfalls.md: `Worktree().Status()` ignores the
global and system gitignore; status exposes files below an ignored managed-worktree root,
which produced eight false untracked files during the ADR-0168 terminal audit; `PlainOpen`
refuses repos carrying `extensions.worktreeConfig`, worked around by `OpenRepo`). Native git
is therefore the authority for refs, worktrees, and working-tree truth. go-git's genuine
strength is in-process object reads, which feed `internal/snapshot`
(`WorkingTree`/`IndexTree`/`CommitTree`/`RangePair` at `working.go:18`, `index.go:15`,
`commit.go:14`, `range.go:15`; ten production call sites in seven files) and the audit's
range walk. The user's direction for this decision: one seam with a single implementation
per entrypoint, the backend an internal detail chosen per entrypoint; a swap must be
supported by tests that pin behaviour; nothing switches now, this pass organises; and the
conversion is whole, because this is not an example introducing a pattern but the
application effort converting an area that needs it.

The composition around git also contradicts live authority.
`code-design/dependency-composition:direct-injection-first` forbids a hidden production
default, yet `effort.Open` silently defaults seven dependencies
(`internal/effort/service.go:51-74`) and `worktree.Open` defaults its runner and hard-wires
an inner `effort.Open(ctx, invoking, effort.Options{})` (`internal/worktree/manager.go:44-46`),
so nothing reachable through a `Manager` can be injected; tests compensate with 34
post-construction private-field writes. `Options.Git` is never set by any caller and
`Service.git` is written once and never read, so `nativeGit` (`service.go:91-99`) is a
production function whose only caller is a coverage-satisfying test, and its `Fault` sibling
(`service.go:22`) is test-only production indirection. `cmd/awf/effort.go:13` holds
`openWorktreeManager` as a mutable package global. Both `worktree.Manager` and
`effort.Service` store a `context.Context` at construction (`manager.go:31`, `service.go:31`)
and reuse it for every later operation, including destructive ones a caller cannot cancel.
Adjacent, the resident-root resolution is duplicated near-byte-identically at
`internal/project/project.go:130-141` and `cmd/awf/sync.go:25-35`, and
`internal/git/controlroot.go` carries 39 byte-identical `coverage-ignore` justifications
across an unrolled check-act-recheck identity ladder.

The test lane splits the same way. Fourteen test files import go-git outside
`internal/testsupport/gitfixture` (three via `PlainInit`; the rest build states gitfixture
cannot express: unmerged index entries, submodule gitlinks, allow-empty commits, add-all
staging, branch refs), and gitfixture's own signatures return go-git types
(`*git.Repository`, `plumbing.Hash`), so no single-home claim over test fixtures is possible
without reshaping its API. Further test files build git state by shelling out to git
(`internal/project/context_test.go`, `internal/project/topics_test.go`,
`internal/worktree/manager_test.go`, `internal/effort/store_test.go`,
`cmd/awf/topic_test.go`, `cmd/awf/context_test.go`, `cmd/awf/effort_test.go`,
`cmd/awf/run_test.go`, `internal/migrate/remove_workflow_residents_test.go`), including
registered managed worktrees, a state go-git cannot express at all. gitfixture cannot consume the production seam:
`tooling/quality-gates:testsupport-zero-internal-deps` forbids it importing any internal
package and explicitly permits go-git within gitfixture, so its carve-out is mechanically
forced, not stylistic.

This decision applies standing authority rather than creating pattern authority.
`code-design/single-home:single-implementation` (ADR-0181) supplies the uniqueness rule;
`code-design/dependency-composition` (ADR-0178) supplies the seam shape rules the
conversion must satisfy. Two reconciliations are stated here because a reviewer would
otherwise raise them: `internal/git` is the designated mechanism-adapter package, so
housing both backends inside it satisfies `mechanism-adapters` (the adapter stays outside
the policy packages it serves); and consumers keep their own narrow contracts
(`consumer-owned-contracts`), with the seam handle as the provider their production wiring
binds, so no consumer depends on a broad facade. The backend-agnostic contract suites are
not anticipated reuse under `concrete-first-consumer`: their first consumer is this
conversion itself, which needs pinned semantics to prove behaviour preservation while
seven call shapes collapse into one. And moving `Commit`/`FileChange` into the seam does
not invert audit's ownership: they are the walk's output representation, the values the
mechanism yields, exactly as `WorktreeRegistration` is already an `internal/git`-owned
value consumed by effort policy; audit's policy is its rules over those values, and its
narrow consumer-owned walk contract survives, expressed against seam-owned types.

Topic mechanics, verified during grounding: proof markers are constrained by
`currentState.testGlobs`, not topic paths, so repo-walking proofs may live anywhere; claim
operations are `add`/`update`/`remove` only, so moving a claim between topics is a remove
plus an add whose re-added claim carries this ADR as `Origin` (ADR-0127's provenance
survives in claim references and prose, not in `Origin`); `topicCoverage: error` requires
the new topic to be claim-bearing in the same transaction that narrows
`tooling/audit-and-snapshots` off `internal/git/**`; and no `state:`/`touches-state:` marker
exists under `internal/git/`, so the path transfer is marker-safe.

## Decision

1. Add `tooling/git-access` with paths `internal/git/**` and
   `internal/testsupport/gitfixture/**`; the second selector is deliberate, so that the
   `fixture-single-home` claim is visible to an agent editing gitfixture (topic path
   overlap is already tolerated, and `tooling/test-infrastructure` keeps its broader
   ownership of `internal/testsupport/**`). In the same applied transaction, narrow
   `tooling/audit-and-snapshots` to `internal/audit/**` and `internal/snapshot/**`, and
   move the two ADR-0127 range-parser claims into the new topic as remove-plus-add
   operations whose re-added prose preserves ADR-0127's provenance by reference. The two
   proof markers at `internal/git/parserange_test.go:11,69` are rewritten to the new
   qualified ids in the same commit. All ten operations apply in exactly one batch on a
   direct Proposed-to-Implemented transition: because item 11 authors every claim as
   completed reality, no partition into Applied and Remaining subsets is coherent, and
   the remove-plus-add pairs in particular are indivisible.

2. `internal/git`'s package root becomes the seam: semantic entrypoints only, with both
   backends as unexported implementation files inside the package (seam-in-place; no
   package split, which stays with the designated `internal/project` decomposition
   decision). A handle constructed by `git.Open(root)` absorbs the tolerant
   `worktreeConfig` open and exposes the entrypoints as methods; the pure range parser
   stays a free function. No go-git or go-billy type, sentinel, or error value crosses the
   seam surface in either direction.

3. The entrypoint inventory, grouped by consumer need: object and tree reads (staged-tree
   blobs, commit-tree blobs, range-pair blobs, working-tree paths with authoritative
   ignore semantics, head existence and hash, and changed paths across working, staged,
   and range selections - today's exported `ChangedPaths`, which becomes a handle
   method); the commit-range walk (revision resolution, merge base exposed as its own
   entrypoint, commit enumeration with metadata, per-file change stats, changed paths
   and unified diff text for a revision range, and old and new blob text - the last
   three serving the repo-local audit tooling), whose neutral `Commit`/`FileChange`
   types move from `internal/audit` into the seam; repository topology (control roots,
   worktree registrations); the worktree
   lifecycle (add, remove, list, ancestor); effort operations (branch existence, ref-name
   validation); and exactly one cleanliness oracle (working-tree change counts) consumed
   by both the audit rule and worktree refusal. Which backend serves an entrypoint is
   internal, invisible to consumers, and swappable per entrypoint.

4. One native runner, unexported: it pins the repository with the validated root, applies
   the isolated environment unconditionally (`IsolatedGitEnvironment` becomes unexported
   in the same transaction that deletes its one remaining consumer, the duplicated
   `internal/worktree` runner), refuses a context without a deadline as a hard
   error (activated in the same transaction that converts every production feed, since
   two feeds swallow errors into a silent resident-root fallback and must change shape
   with the enforcement), and translates a non-zero exit into a `CommandError` carrying the arguments,
   exit code, and captured stderr. The exit-code-1-as-answer idiom lives once, inside the
   runner, surfacing as `(bool, error)` probes. Deadline values are chosen at the command
   boundary in `cmd/awf`; the implementation plan names the magnitude, chosen as a
   hang-prevention ceiling generous enough that no observed-normal local operation
   (including full-tree status on a large working tree) approaches it, not as a latency
   budget; and all nine `context.Background()` feeds convert.

5. The seam owns an `errors.Is`/`errors.As`-matchable vocabulary: `CommandError`, and a
   not-a-repository identity replacing the leaked go-git sentinel matches in
   `internal/migrate`. `HardSafetyError` is retained exactly as is. Ref absence is
   answered by the `(bool, error)` probes of item 4 and gains no identity of its own until
   a consumer needs to distinguish it (`concrete-first-consumer`). Mechanism errors are
   translated at the boundary and never cross it.

6. The conversion is whole within the area; no production carve-out. Every consumer
   converts: `internal/snapshot` (its four constructors take the handle; ten call sites in
   seven files thread it), `internal/audit` (its `git.go` deletes; `Collect` consumes the
   commit-range walk), `internal/migrate` (three files drop go-git and the sentinel
   matches), `internal/upgrade` (`HeadHash`) and `internal/project` (`HeadExists`; and the
   duplicated resident-root resolution at `internal/project/project.go:130-141` and
   `cmd/awf/sync.go:25-35` collapses to one home in `internal/git`, which already owns
   `ResolveControlRoots` and `ResidentRoot`: `cmd/awf`'s composition point calls it
   directly and the transitional `project.Open` calls the same function internally, so
   no import inverts, `outer-composition` stays intact, and
   `sync-project-loader-wiring`'s canonical text survives unchanged with no operation
   owed against it), `internal/worktree` (its `git.go`
   deletes; its consumer-owned runner contract is satisfied by seam-backed wiring;
   `ancestor` becomes a seam entrypoint), `internal/effort` (`nativeGit`,
   `nativeBranchExists`, and the inline `check-ref-format` exec delete), `cmd/awf`
   (composition wiring), and `cmd/repoaudit` (`realGit`, `gitError`, and the raw-argv
   `gitFunc` contract convert: the tool's needs are met by a narrow consumer-owned
   contract over the merge-base, range changed-paths, range diff-text, and file-text
   entrypoints; ADR-0073's standalone posture governs coupling to `internal/audit`, not
   to `internal/git`, which repoaudit already imports). `project.Open` and `Loader.Open`
   gain a context parameter, so every gated command's entry path threads a deadlined
   context; this is the widest single signature change in the conversion and is
   deliberate. The handle itself is a construction-time dependency of the loader and
   the project value it opens - selected at the composition root and written once at
   construction, a dependency under `outer-composition` rather than operation-derived
   state - while cancellation stays per-operation.

7. The composition around the seam converts in the same effort under existing ADR-0178
   authority: `effort.Open` and `worktree.Open` take their volatile dependencies
   explicitly with no silent defaults (the `project.NewLoader` panic-on-nil model), the
   hard-wired inner `effort.Open` is deleted and `worktree.Manager` receives the composed
   service, the test-only `Fault` option is removed, the `openWorktreeManager` package
   global is retired, and the stored `Manager.ctx`/`Service.ctx` fields are deleted so
   every operation that reaches git takes a context parameter (`Service.New` included;
   operations with no git dependency take none).

8. The check-act-recheck identity ladder in `internal/git/controlroot.go` is extracted
   into named operations. Its 39 byte-identical coverage escapes collapse to the few the
   named operations genuinely need; the count is an outcome of the extraction, not a
   target, per `code-design/single-home:no-coverage-fork`.

9. Every entrypoint carries a backend-agnostic contract suite that pins its semantics:
   ignore-rule behaviour, cleanliness edges, topology, ref validation, error identities,
   and the deadline refusal, with the three pitfall incidents (global-gitignore scope,
   ignored-worktree-root exposure, `worktreeConfig` open) as named regression cases. The
   suites are what make a later backend swap safe: swapping an entrypoint's implementation
   must leave its suite green or fail visibly. The existing repo-walking guard
   `TestWorktreeStatusInjectsGlobalExcludes` (`internal/git/git_test.go:46-68`) is retired
   in the same pass: its substring check goes vacuous once ignore semantics live inside
   the seam, and the working-tree-paths contract suite pins the same property directly.
   A mutation-testing pass over `internal/git` is an advisory post-implementation option,
   not a gate.

10. The test fixture lane converges on the same rule, totally ("we want to clean
    everything that interfaces with git in any way"): `internal/testsupport/gitfixture`
    becomes the only constructor of git fixtures, with two internal lanes behind one
    neutral API. The go-git lane gains the capabilities the fourteen direct-importing
    test files need (unmerged index entries, explicit filemodes including gitlinks,
    allow-empty commits, add-all staging, branch refs); a native-git lane expresses what
    go-git cannot (registered managed worktrees and their topology), converting the
    test files that build git state by shelling out. Exported signatures reshape to
    neutral types so a consuming test needs neither a go-git import nor a git
    subprocess; every file either walker flags converts (twenty unique files at the
    authoring census, indicative). The `fixture-single-home`
    walker covers both forms, flagging any test file outside its allowlist that imports
    go-git or constructs a git subprocess; the allowlist is gitfixture itself and
    `internal/git`'s own test files, which exercise the mechanism the seam owns.
    gitfixture keeps go-git and process execution internally under its
    `testsupport-zero-internal-deps` carve-out (the library and the standard library
    alike are outside-internal dependencies). The false "purely through go-git"
    assertion at `cmd/awf/testmain_test.go:12` is corrected in the same pass.

11. Claim backing: `all-access-via-seam` (production repo-walker; its allowlist is
    `internal/git/**` plus the gitfixture carve-out), `fixture-single-home` (test-file
    repo-walker), `pinned-entrypoint-semantics` (the entrypoint inventory is pinned in
    one table-driven test that fails when a named entrypoint lacks a passing suite, so
    the claim asserts exactly what that table proves), and `isolated-deadlined-native`
    (direct runner tests: polluted-environment isolation, deadline refusal,
    stderr-carrying errors) are `Backing: test`.
    `one-implementation-per-entrypoint` and `single-cleanliness-oracle` are reasoned
    contracts with `Verify:` instructions. The moved range-parser claims keep their
    existing test backing. Because the conversion is whole, every claim is authored as a
    statement of completed reality with no new-or-converted qualifier.

12. Application sequences after the two in-flight transactions that touch adjacent
    surfaces integrate: the severity-unification chain (branch
    `awf/drop-severity-settings-and-unify-the-rank`; its ADR numbers pin at integration)
    also edits `tooling/audit-and-snapshots`, and ADR-0180's plan owns
    `internal/project/project.go` and `internal/project/topics.go`, both of which item 6
    also touches. ADR-0181's application (which this decision cites as authority) is part
    of the same ordering.

13. Documentation travels with the conversion, in the same commits: the three
    docs/pitfalls.md entries that mandate the current API route (`OpenRepo` at the repo
    opens entry, the `GlobalExcludePatterns` injection instruction, and the gitfile
    resolution heading) are rewritten to name the seam entrypoints while their underlying
    incidents stay recorded as the contract suites' named regression cases;
    docs/architecture.md's `internal/worktree` native-git ownership bullet and its
    go-git dependency note (repository control-root resolution) are
    updated to the seam shape; and docs/testing.md gains the contract-suite category and
    the serial-by-construction exception for the isolation and missing-binary suites.

## State changes

- add `tooling/git-access:all-access-via-seam`
- add `tooling/git-access:one-implementation-per-entrypoint`
- add `tooling/git-access:pinned-entrypoint-semantics`
- add `tooling/git-access:isolated-deadlined-native`
- add `tooling/git-access:single-cleanliness-oracle`
- add `tooling/git-access:fixture-single-home`
- remove `tooling/audit-and-snapshots:git-range-parser-single-definition`
- remove `tooling/audit-and-snapshots:git-range-rejects-malformed`
- add `tooling/git-access:git-range-parser-single-definition`
- add `tooling/git-access:git-range-rejects-malformed`

## Consequences

The area's forks disappear as a class: one runner, one branch probe, one ancestor probe,
one cleanliness oracle, one stderr translation, one resident-root resolution, one fixture
constructor. Environment isolation and deadlines become structural rather than per-site
accidents, which changes observable behaviour deliberately: a git invocation that today
hangs on a credential prompt or a stale lock becomes a timely error, and effort operations
gain isolation they currently lack. The deadline refusal carries two named costs: it is a
runtime failure, not a compile-time one, so a wiring site that forgets a deadline fails at
execution (including on the pre-commit path, which is why the refusal is itself
test-backed and every feed converts in this effort); and a fixed magnitude could abort a
legitimately slow operation, which item 4's hang-prevention-ceiling framing exists to
prevent. Error messages change shape where `CombinedOutput`
gave way to captured stderr, and converted-area tests move from message substrings to
`errors.Is`/`errors.As`; the repo-wide error-identity decision remains open and untouched.

The signature ripple is the accepted cost: four snapshot constructors and ten call sites
thread a handle, roughly fifteen worktree/effort methods gain or lose parameters, 34
post-construction test writes are retired in favour of per-instance fakes, and converted
packages become parallelisable except the isolation and missing-binary suites, which
`t.Setenv` keeps serial by construction. The gitfixture reshape is the largest single work
item, now spanning two construction lanes and the whole converted fixture census; it is
what the total fixture-single-home claim costs, and the user chose it deliberately over a
staged posture, twice.

A later backend migration, either direction, becomes a per-entrypoint decision guarded by
that entrypoint's contract suite, which is the option value this seam buys. Nothing
switches now; go-git remains a dependency with its consumer surface shrunk to one package,
and dropping it entirely becomes a future decision that can proceed entrypoint by
entrypoint instead of as a rewrite.

Coverage escapes in `internal/git` drop substantially through the ladder extraction, and
the runner's new refusal branches need tests rather than escapes. The dead-code gate
interacts with the deletions: exported symbols whose callers become seam wiring must be
unexported or deleted in the same transaction, which the plan sequences.

The claim moves reattribute `Origin` for the two range-parser claims to this ADR, with
ADR-0127 preserved by reference; readers of ADR-0127 need this ADR for where its claims
now live. Narrowing `tooling/audit-and-snapshots` while the severity chain edits the same
topic makes integration ordering load-bearing; item 12 records it, and the worktree
integration step resolves it mechanically rather than by merge improvisation.

This decision widens `internal/git`'s content while the designated decomposition decision
will later split it; that is deliberate sequencing, not churn. The seam surface and its
claims are the stable interface the split preserves; only the topic's path selectors move
when packages do.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Free-function entrypoints taking a root string | Repeats the root-as-ambient-dependency shape at 64 audited sites; a handle validates the root once, makes construction the composition point, and gives the contract suites their object. |
| One exported Git interface at the seam | `direct-injection-first` introduces an interface only for a cohesive multi-operation contract owned by its consumer; a seam-wide interface would be a provider-owned facade nobody consumes whole. Consumers keep their narrow contracts and the concrete handle is the provider their wiring binds. |
| Backend subpackages (`internal/git/native`, `internal/git/gogit`) | Physically visible swap points, but performs part of the package split this decision deliberately leaves to the decomposition ADR, and parent-only imports need their own guard. |
| A new facade package over the existing `internal/git` | Pure indirection; every importer renames for no ownership gain, against the posture doc's abstraction-must-earn-its-cost rule. |
| Switch to native-only git now | The migration becomes cheap per entrypoint once suites pin semantics; coupling it to the organise pass would put a snapshot-layer rewrite inside an already-wide transaction for no immediate gain. |
| go-git-only | Impossible: no worktree lifecycle, no `check-ref-format`, and status semantics that diverge from real git in three documented, paid-for ways. |
| Keep `cmd/repoaudit` standalone with its own git shim | Rejected by the user ("a production tooling that rolls their own implementation sounds simply bad"); ADR-0073's standalone posture covers `internal/audit` coupling only, and repoaudit already imports `internal/git`. |
| Bounded-candidate posture instead of whole-area conversion | Rejected by the user: this is the application effort for standing patterns, not an introduction needing an example slice; partial conversion would also force qualified claims instead of statements of completed reality. |
| Staged fixture-lane conversion (convert the three `PlainInit` sites, leave the rest as candidates) | Rejected by the user twice: first for the fourteen go-git importers, then for the eight exec-based builders; the exotic states are exactly the capabilities gitfixture should own, and a staged claim would be honest but weaker. |

## Status history

- 2026-07-30: Proposed
