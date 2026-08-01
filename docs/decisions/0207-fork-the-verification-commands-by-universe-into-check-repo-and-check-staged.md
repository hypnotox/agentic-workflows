---
format: current-state-v2
status: Accepted
date: 2026-07-31
---
# ADR-0207: Fork the verification commands by universe into check repo and check staged

## Context

ADR-0159 regrouped the verification commands under `awf check` and left `--staged` as a
boolean on the bare group form. Its own Context section named the result a smell: "`runCheck`
branches to `runCheckStaged` and skips the drift oracle entirely, so `awf check` and
`awf check --staged` verify disjoint surfaces under one name." It deferred the fix to a
follow-on, recorded in `docs/roadmap.md` as "Bare `awf check` should run every enabled check
and report what ran", whose central device was a ran/skipped report that would let `--staged`
widen to the children while disclosing any check the flag caused to do nothing.

`cmd/awf/check.go:22-24` is the branch in question: bare `check` returns into `runCheckStaged`
before either the advisory notes or the drift oracle run. Every child already declares
`--staged` in its `BoolFlags` (`internal/clispec/clispec.go:97-146`) and `cmd/awf/dispatch.go:77-78`
rejects it with a tailored usage error, so the flag's acceptance is built and deliberately refused.

### The flag conflates two independent axes

A check has a **subject**: either a state property, true of one tree, or a transition property,
requiring a before and an after. It separately has a **source**: the tree it reads, working
tree or Git index. `--staged` moves both at once for the current-state check while moving only
the source for nothing else, because nothing else offers both.

Only the current-state check is transition-shaped. `CheckStaged`
(`internal/project/currentstate.go:168-192`) loads HEAD and the index and validates the move
between them. Everything else under `check` asserts a property of a single tree.

### Prose and memory are repo-state checks, not staged-changes checks

`stagedTree` (`cmd/awf/gate.go:95-101`) calls `snapshot.IndexTree`, which calls `repo.IndexBlobs`:
every tracked file, not a diff. `runProseGate` then scans `tree.List()` whole, and `runMemoryGate`
scans every file under the docs decisions and plans prefixes. Both are whole-corpus scans that
read the index as a snapshot source. `proseGate.exemptions` carrying a `Count` settles it: a
count-based exemption is meaningful only over a fixed whole corpus, never over a changing diff.
`check prose`'s own help text already says "the project's tracked text files", agreeing with the
code and disagreeing with the flag that gates it.

The index is also the *correct* source for both. The property they enforce is about tracked
files; the working tree additionally holds untracked ones the property never covered.

### `check invariants` verifies nothing

`runInvariants` (`cmd/awf/invariants.go:16-37`) prints every invariant claim's backing mode and
proof sites and cannot fail. Backing contracts are enforced when the corpus loads
(`internal/topic/markers.go:157-160`, with the metadata-shape half at
`internal/topic/topic.go:318-341`), so every command that opens the project already
fails on an unbacked invariant. `README.md:278` nonetheless describes the command as reporting
invariants "that lack a backing comment in source" and `README.md:185` credits the subcommand
with enforcing backing symmetrically with `awf check`. Both are false. The command ships to every
adopter through `templates/docs/working-with-awf.md.tmpl:34`.

### Where the roadmap entry's four contracts stand

The entry at `.awf/docs/parts/roadmap/deferred.md:101-121` queues four. Its duplicate scan
invocations were removed by ADR-0196 Decision 3; `x` no longer names either scan. Its
all-checks-skipped exit code becomes unreachable once a universe group always runs its
non-optional children. Its `--staged` widening is answered by the fork rather than performed,
because membership in a universe group is the disclosure a ran/skipped report would have had to
narrate.

Its first contract is the one this decision must still discharge: the two scans call
`snapshot.IndexTree` before consulting their own knob, so a disabled gate hard-errors outside a
git repository, and what bare check does outside git while a knob is on was left open. Decision
items 4 and 9 settle both halves.

### Depth is the structural cost

`resolve` (`cmd/awf/dispatch.go:205-216`) looks up `args[0]`, tests `args[1]` against `Children`,
and returns; it has no recursion. It returns the depth-2 node as `cmd`, and `cmd/awf/main.go:119`
reads `clispec.ResolvedGating(top, cmd)` while `cmd/awf/main.go:169` reads `cmd.StateExempt`.
Left unchanged, `awf check staged commit` would resolve to the `staged` group and lose its
project-guard exemption, contradicting `tooling/cli:group-child-project-guard-exemption`. The
parallel gating problem is real today but is dissolved rather than fixed by item 13, which leaves
no group child declaring its own classification. Two further sites fail silently rather than
loudly: `globalHelp` (`cmd/awf/main.go:51-54`) and `clispec.UngatedGroupChildren`
(`internal/clispec/clispec.go:498-511`) each iterate one level of children, so grandchildren would
simply stop appearing, and `internal/project/gatedcommands.go:18` carries a coverage-ignore whose
stated assumption ("the table always carries the three ungated check children") this decision
falsifies from both directions.

### Blast radius, measured

Counting tracked files, split by how each population is maintained:

| change | append-only history | rendered output | authored inputs | total |
|---|---|---|---|---|
| `awf check --staged` respelled | 60 | 61 | 30 | 151 |
| `awf check invariants` dropped | 9 | 4 | 14 | 27 |

Only the authored column is hand-edited. The rendered column is rewritten by `awf render`, and
the append-only column records what the commands were called when those decisions were made.

## Decision

1. `check` gains two universe sub-groups, `repo` and `staged`, and `--staged` is removed from the
   command surface entirely. `awf check --staged` becomes `awf check staged`. No alias is kept,
   following the ADR-0093 and ADR-0159 precedent of a clean break.

   The pre-commit payload stops naming the staged universe and the two scans at all. Bare check
   covers both universes after item 4, so `templates/hooks/pre-commit.sh.tmpl` drops its
   `{{ . }} --staged` line and its `check prose` and `check memory` lines, leaving the configured
   `checkCmd` and the gate. This removes the append-a-token composition rather than respelling it,
   which matters because `--staged` was a flag and `staged` is a positional: appending it to an
   adopter `checkCmd` that already carried a flag would have rendered an invocation the group
   handler rejects for subcommand order. No new adopter-visible var is introduced.

   `proseGateCmd` and `memoryGateCmd` are retired from the catalog and config-spec availability map
   with their payload consumers. Keeping either descriptor after deleting its payload line would
   publish a configurable command that no rendered artifact invokes. The runner-disabled hook
   validation drops both keys from its required set as well: `checkCmd` now owns both scans through
   the bare aggregate, while `commitGateCmd` remains live in the separate commit-msg payload. Its
   requirement therefore narrows to `checkCmd` and `commitGateCmd`, in addition to the independently
   required `gateCmd`.

   It also completes ADR-0196 Decision 3 rather than regressing it. That decision made the payload's
   standalone scan lines the single local enforcement point; item 4 makes `checkCmd` cover them, so
   keeping the standalone lines would run each scan and the staged universe twice per commit.

2. Membership follows the subject axis. `check repo` holds `drift`, `state`, `prose`, and `memory`.
   `check staged` holds `state` (the HEAD-to-index transition check), `drift` (new, item 5), and
   `commit`. A child name may repeat across universes; `repo state` and `staged state` are
   different checks, static corpus validation and transition validation respectively, which is
   why the universe belongs in the path rather than in a flag.

3. Each child reads its natural source, and no uniform source flag is introduced. `repo drift` and
   `repo state` read the working tree; `repo prose` and `repo memory` read the index, which is the
   tracked corpus their property is about. The universe group names the subject its children assert,
   not the tree each one happens to read.

4. Bare `awf check repo` and bare `awf check staged` run their children. Bare `awf check` runs both.
   No ran-versus-not-applicable report is produced: every child of a universe group applies to that
   universe by construction, which structurally supplies what ADR-0159 Decision 3 expected a report
   to supply. One child is excluded from its group's bare aggregate for a stated reason:
   `staged commit` takes a message file and has no input outside a commit-msg hook. It remains
   directly invocable.

   Outside a git repository, bare `awf check` runs the repo universe alone and reports that the
   staged universe was unavailable. A repository with no commit yet is not such a case: `CheckStaged`
   supports it deliberately, taking the empty universe as the before side
   (`internal/project/currentstate.go:164-166`), which is what lets an adopter's first commit pass a
   wired pre-commit hook. This is the one place the
   design states that something did not run, and it reports an environmental fact rather than a gap
   in the matrix. It is deliberately not a refusal: an adopter actually using awf is expected to be
   in a git repository, but a tree that is not one should degrade rather than break. A backed claim
   depends on this, `rendering/project-output-plan:curated-init-skill-refs-clean`, whose proof runs
   `awf check` against a freshly-scaffolded directory that is not a repository; degrading keeps that
   claim true verbatim and needs no operation on it. Directly invoking `awf check staged` outside a
   repository still fails, because there the universe was named rather than inferred.

   The two project-level notes bare `check` owns today move to `check repo` and are emitted once,
   by the repo group, whether it was invoked directly or through bare `check`. ADR-0159 Decision 2
   kept the advisory notes (`cmd/awf/check.go:29-37`) and the version-ahead note (`:18-21`) on the
   bare form because it was then the only form owning project-level context; the repo universe now
   owns it, and neither note is a transition property. Each universe group compares the lock its own
   source implies: `check repo` the working lock, `check staged` the index lock, which is what
   `checkLockVsBinary`'s `staged` bool selected before the flag was retired.

   Widening bare `check` into an aggregate rescopes one claim. `tooling/cli:invariants-in-check`
   currently reads that `awf check` exits non-zero on an error-severity current-state issue "and
   stays clean when it reports none", which stops being true of the whole command once a prose
   finding can also fail it. Its prose narrows to the current-state evaluation's own contribution
   rather than to the exit status of every check the aggregate runs.

5. `check staged drift` is added: render from the staged config and compare against the staged
   output tree. It closes a hole no gate covers today, where a `.awf/` config change staged without
   its re-rendered output passes commit-time verification because bare check reads a working tree
   that is already rendered.

   Its scope is bounded positively, not by exclusion: `staged drift` emits exactly the stale and
   hand-edited comparison of re-rendered bytes against the staged output tree, and every other kind
   `p.Check` produces is out of scope. That explicitly excludes, among others, the config-tree
   hygiene sweep, the dead-reference probe, stale-backup flagging, invalid-frontmatter drift,
   orphaned-path drift, and provenance-banner and managed-output-attribution checks. The sweep and
   the dead-reference probe are named because they are the two that need a semantic decision this
   record does not make: a snapshot tree carries neither directory entries nor untracked files, so
   a legitimate rendered link would otherwise flip to `dead-reference` inside a blocking gate.
   Item 12 records the follow-on.

6. `awf check invariants` is removed with no replacement, together with `runInvariants`,
   `Project.CurrentStateInvariants`, and `InvariantReport`, whose only production caller it is.
   The false descriptions at `README.md:185` and `README.md:278` are removed rather than corrected,
   `templates/docs/working-with-awf.md.tmpl:34` drops the line it ships to every adopter,
   `.awf/domains/parts/invariants/current-state.md:9` drops the sentence describing "the standalone
   `awf check invariants` report", and `x:99` drops the invocation it runs inside `examples/sundial`,
   which is the line the `tooling/quality-gates:example-adopter-checked` update removes.

7. A disabled opt-in child prints a note stating it is disabled and naming the knob that disables it.
   Silence is not an acceptable report of a check that did not run.

8. `examples/sundial` enables both `proseGate` and `memoryCite`, so the showcase adopter exercises
   every aspect of the standard rather than only its unconditional parts. This keeps
   `tooling/quality-gates:example-zero-notes` true without a second output channel for item 7,
   because a knob that is on emits no disabled note.

   `./x check`'s example invocation is scoped to `awf check repo`. Sundial is a nested tree inside
   this repository, so its staged universe would otherwise be evaluated against the containing
   repository's index, which is neither what the example asserts nor a property sundial owns. The
   nested staged transition check keeps its existing separate invocation at
   `.githooks/check-nested-staged:7`, which hardcodes the spelling and so respells by direct edit
   rather than through either composition site item 1 names.

9. The staged scanners consult their enablement knob before reading any index. Today they open the
   repository first (`cmd/awf/prosegate.go:17` ahead of the knob at `:29`), so a disabled gate
   hard-errors outside a git repository instead of reporting itself disabled. That ordering is what
   would otherwise defeat item 4's degradation: the repo universe carries these two index readers,
   so bare check outside a repository would fail on them even with the staged universe degraded, and
   `rendering/project-output-plan:curated-init-skill-refs-clean` would still break. With the knob
   read first, a disabled scan reports disabled without touching git and an enabled scan still
   refuses, which is the substantive half of the
   `tooling/quality-gates:prose-gate-refuses-without-git` update: that claim narrows from refusing
   unconditionally to refusing when the gate is on.

   Nothing else about these two scanners changes. Their corpus and path resolution are already
   project-root correct: `internal/git/handle.go` reroots every index entry through the project
   prefix that `OpenContaining` computes, so a nested adopter already reads its own config and its
   own subtree.

10. `resolve` returns the leaf node, so a grandchild's project-guard exemption is read from the node
    the driver dispatches rather than from its group. `globalHelp` recurses to any depth. Gating
    needs no chain, because item 13 leaves every `check` descendant inheriting the group's
    classification. Arity stays with the leaf: fixing `resolve` is preferred to hand-parsing a third
    token in the handler,
    which would otherwise stop `awf check staged commit extra-junk` being rejected.

    The resolved path, not a single token, reaches the handler. `cmdCtx.sub` is one string today
    (`cmd/awf/dispatch.go:31`) and `runCheckGroup` selects on it with a flat switch, so `repo state`
    and `staged state` would collide on the only token the handler sees, which would make item 2's
    name repetition unimplementable. `sub` therefore carries the resolved child path, and the
    unknown-subcommand message enumerates the leaf set of the group actually addressed rather than
    the one flattened level `checkSubcommands()` produces today (`cmd/awf/dispatch.go:51-58`).

11. A new schema migration covers every var this decision invalidates. It clears the retired
    `proseGateCmd` and `memoryGateCmd` keys themselves before inspecting values, whatever values they
    hold. In every other var, it retargets `check prose` to `check repo prose`, `check memory` to
    `check repo memory`, and `check commit` to `check staged commit`, preserving any trailing
    arguments. Those live commands remain meaningful regardless of which var composes their awf
    invocation. It clears any var invoking the removed command in its `check invariants` spelling. The pre-ADR-0159 bare `invariants` spelling is deliberately not
    matched: an awf-invoked value was already rewritten to `check invariants` by the 18-to-19
    migration and so never reaches this one, and the values that still spell it bare are the
    non-awf-runner values ADR-0159 declined to own on the stated ground that awf does not own another
    runner's vocabulary (`internal/migrate/renameretiredcommands.go:57-60`). Retargeting the retired
    keys would preserve descriptors with no consumer; leaving a stale live spelling elsewhere would
    instead reproduce exactly the failure ADR-0159's migration was written to prevent: an
    unknown-subcommand error inside a hook rather than at upgrade time.

    The migration matches a three-token invocation. `retiredCommandRe`
    (`internal/migrate/renameretiredcommands.go:28`) is anchored to a two-token form and cannot be
    copied for this.

    The shipped 18-to-19 migration at `internal/migrate/renameretiredcommands.go:16-22` is left
    untouched: editing it would change what an adopter replaying from schema 17 passes through and
    would desync it from the ADR that shipped it.

12. The authored inputs behind the roadmap and the affected domain summaries update in the same
    commit as the change they describe. Every authored live invocation of the three moved commands is
    swept rather than relying on the older affected-site inventory. This explicitly includes
    `.awf/parts/working-with-awf/commands.md`, the workflow parts, `templates/docs/workflow.md.tmpl`,
    `templates/docs/working-with-awf.md.tmpl`, `README.md`, and command-related config-spec prose;
    `awf render` carries those edits into every rendered document. The superseded follow-on entry at
    `.awf/docs/parts/roadmap/deferred.md:101` is removed and its
    carried-forward remainder lands in `.awf/docs/parts/roadmap/ideas.md`, recording that three of its four contracts are discharged here and carrying
    forward the two deferred halves of item 5 as a check-architecture cleanup; the deferred entry at
    `.awf/docs/parts/roadmap/deferred.md:235` ("`awf check drift` and `awf check state`: deliberately
    kept, currently uninvoked") is resolved by item 4, which invokes both.
    `.awf/domains/parts/adr-system/current-state.md:7` takes the respelled invocation from item 1.
    `.awf/parts/workflow/local-hooks.md:3` takes a semantic rewrite rather than a respelling, because
    it enumerates the payload's steps and item 1 deletes three of them; it joins
    `.awf/parts/workflow/composing-the-gate.md:9-14` and `.awf/docs/parts/testing/gate.md:11-12` in
    describing the payload's new shape.
    `.awf/parts/workflow/composing-the-gate.md:9-14` and `.awf/docs/parts/testing/gate.md:11-12`
    take a semantic update rather than a respelling: both describe the two scans as separate
    non-gate steps the payload runs on its own, which item 4 and item 1 together retire.

13. Gating is unconditional across the whole `check` family: `check prose`, `check memory`, and
    `check commit` lose the `Ungated` classification ADR-0159 Decision 4 gave them and inherit the
    group's `Gated`. Ungated is reserved for a command that repairs a gate problem (`upgrade`) or one
    genuinely disconnected from the config (`version`, `changelog`). All three read `.awf/config.yaml`
    (the enablement knobs, and `docsDir` for the memory prefixes), so none qualifies, and a binary
    behind the project cannot be repaired by committing. The behaviour change is accepted and named:
    an adopter invoking these three directly on a stale binary now gets a refusal where they
    previously got a scan.

    The per-child gating mechanism goes with them. Those three are the only group children in the
    table that declare their own classification (`internal/clispec/clispec.go:119`, `:132`, `:147`),
    so with them inheriting, `ResolvedGating`'s child branch and `UngatedGroupChildren` have no
    exercised path; the 100% coverage gate and the dead-code gate would force each to a vacuous
    always-false branch or an always-empty result rather than let it stand. Both are removed, the
    published projection collapses from two lists to one, and the coverage-ignore at
    `internal/project/gatedcommands.go:18` is removed with the branch it guards.

    The project-state exemption is a different guard and is retained where awf's own multi-commit
    operations need it. `StateExempt` stays on `check commit`, because incremental claim application
    spans commits by design and a commit-msg hook must function inside that window, which was
    ADR-0159 Decision 5's actual justification. It is dropped from `check prose` and `check memory`,
    which no longer run standalone from the payload after item 1.

## State changes

- update `tooling/cli:gated-commands-generated`
- remove `tooling/cli:group-child-gating-honored`
- update `tooling/cli:group-child-project-guard-exemption`
- update `tooling/cli:help-lists-group-children`
- update `tooling/cli:invariants-in-check`
- add `tooling/cli:check-universe-groups`
- update `rendering/catalog-and-targets:var-descriptor-set-pinned`
- update `config/validation:hooks-commands-resolvable`
- update `rendering/companion-scripts:hook-payloads-fallback-safe`
- update `tooling/quality-gates:memory-citation-gate`
- update `tooling/audit-and-snapshots:commit-gate-shared-rule`
- update `code-design/dependency-composition:dependency-composition-commit-classification`
- update `tooling/quality-gates:prose-gate-refuses-without-git`
- add `rendering/sync-and-drift:staged-drift-rendered-output`
- update `tooling/quality-gates:example-adopter-checked`
- add `tooling/cli:check-disabled-child-disclosure`

## Consequences

The command path states which universe a check belongs to, so a reader no longer has to know that a
flag silently swaps the surface. `check memory` being index-only stops being an exception that
crosses the axes and becomes an ordinary member of the repo group.

A real enforcement gap closes. Staging a config change without its rendered output is caught at
commit time by item 5 rather than by the next person's working-tree check.

The ran/skipped report the roadmap queued is not built, and the exit-code contract it left open is
not settled, because the fork makes both unnecessary rather than deferring them. Anyone reading that
entry for the report should read item 4 instead.

Three costs are accepted. The rename touches 151 tracked files, of which 30 are hand-edited and 60
are append-only records that keep the old spelling as historical fact. Three-level dispatch is a
driver change across roughly twenty sites, two of which fail silently if missed, so item 10 is a
correctness prerequisite rather than a tidy-up. And `check staged drift` ships narrower than
`check repo drift`, so the two are not interchangeable until the deferred halves land; item 5 states
the difference rather than leaving it to be discovered.

Enabling both gates in sundial (item 8) makes the showcase adopter subject to the punctuation and
citation rules, which is the point. It carries no ordering dependency on item 9: the scanners are
already correct in a nested tree, and item 9's knob-first change binds only when a gate is off.

Respelling a flag as a positional makes the hook payload's composition order-sensitive. Appending
`--staged` to a configured command tolerated a command that already carried flags; appending
`staged` would not, because a subcommand must precede them. Item 1 removes the append rather than
leaving adopters to discover the ordering rule from a rejected invocation.

An adopter upgrading past this schema loses the retired `proseGateCmd` and `memoryGateCmd` keys, and
loses any other var value that names the removed invariants command, rather than having those values
rewritten. That is deliberate: the two retired keys have no consumer, while the removed command has
no live target. Preserving either shape would publish inert configuration or defer an
unknown-subcommand failure into a hook, the exact failure mode the ADR-0159 migration was written to
prevent.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Widen `--staged` to the children once a ran/skipped report exists, as ADR-0159 Decision 3 and the roadmap entry propose | Keeps a universe encoded as a flag and needs a report to disclose honestly what the command structure can state by construction |
| Two new top-level commands, one per universe | Reintroduces the top-level proliferation ADR-0159 removed, and forfeits the single `check` help entry that grouping bought |
| Promote `staged` to a child alongside `prose` and `memory`, as ADR-0159 weighed and rejected | Puts a universe beside a set of subjects; the axes visibly cross, which is the defect, not the fix |
| Keep `check invariants` and correct the README instead | Preserves a command whose name promises a verification the corpus loader already performs, in a group where every other member returns a verdict |
| Give `check repo` a uniform source flag so every child can read either tree | Selects a source uniformly for children whose correct source differs; for prose and memory a working-tree default would scan untracked files their property never covered and would read the enablement knob from a different config than today |
| Edit the shipped 18-to-19 migration to retarget the retired name | Changes what an adopter replaying from schema 17 passes through and desyncs the migration from the ADR that shipped it |
| Make bare `awf check` refuse outside a git repository, since this workflow assumes git | Breaks `rendering/project-output-plan:curated-init-skill-refs-clean`, whose proof checks a freshly-scaffolded non-repository tree; assuming git is right for an adopter using awf, not for a tree that merely has not become one yet |
| Make bare `awf check` mean `check repo` alone, with the staged universe always named explicitly | Preserves today's bare-form semantics at no cost, but gives the bare command no way to verify the thing about to be committed even where it could |
| Build `check staged drift` with the sweep and dead-reference halves included | Both need a semantic decision about what they mean over a tree with no directory entries and no untracked files; deciding them inside a blocking pre-commit gate is where a wrong answer is most expensive |
| Keep `proseGateCmd` and `memoryGateCmd` declared but unconsumed | Publishes configuration with no rendered consumer after item 1 deletes the only two payload lines that read it; the bare `checkCmd` aggregate is the new enforcement point |

## Status history

- 2026-07-31: Proposed
- 2026-08-01: Accepted; content-sha256: 984a70036cd01f5853a54ec52f071992b11075e0c5057232e2f66b0fee40fc9c
