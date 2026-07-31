---
format: current-state-v2
status: Proposed
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
proof sites and cannot fail. Backing contracts are enforced by the corpus loader as a load error
(`internal/project/currentstate.go:493-504`), so every command that opens the project already
fails on an unbacked invariant. `README.md:278` nonetheless describes the command as reporting
invariants "that lack a backing comment in source" and `README.md:185` credits the subcommand
with enforcing backing symmetrically with `awf check`. Both are false. The command ships to every
adopter through `templates/docs/working-with-awf.md.tmpl:34`.

### Three of the roadmap entry's four contracts are already gone

The duplicate scan invocations it queued for pruning were removed by ADR-0196 Decision 3; `x`
no longer names either scan. The all-checks-skipped exit code it left unsettled becomes
unreachable once a universe group always runs its non-optional children. And the ran/skipped
report itself is unnecessary under a fork, because membership in a universe group is the
disclosure. What survives is narrower: the two opt-in scans can still be switched off, and a
silent skip is the defect the entry exists to remove.

### Depth is the structural cost

`resolve` (`cmd/awf/dispatch.go:205-216`) looks up `args[0]`, tests `args[1]` against `Children`,
and returns; it has no recursion. It returns the depth-2 node as `cmd`, and `cmd/awf/main.go:119`
reads `clispec.ResolvedGating(top, cmd)` while `cmd/awf/main.go:169` reads `cmd.StateExempt`.
Left unchanged, `awf check repo prose` would resolve to the `repo` group and lose both its
`Ungated` classification and its project-guard exemption, contradicting
`tooling/cli:group-child-gating-honored` and `tooling/cli:group-child-project-guard-exemption`.
Two further sites fail silently rather than loudly: `globalHelp` (`cmd/awf/main.go:51-54`) and
`clispec.UngatedGroupChildren` (`internal/clispec/clispec.go:498-511`) each iterate one level of
children, so grandchildren would simply stop appearing, and `internal/project/gatedcommands.go:19`
carries a coverage-ignore whose stated assumption ("the table always carries the three ungated
check children") this shape falsifies.

### The staged scanners are broken for a nested adopter

`runProseGate` looks up `.awf/config.yaml` at the index root (`cmd/awf/prosegate.go:22`) while
`stagedTree` opens the *containing* repository. Running `awf check prose` inside
`examples/sundial` therefore reads awf's own config and resolves awf's paths against sundial's
root, failing with a stat error on a path that exists only in the parent. `cmd/awf/memorygate.go:25`
has the same shape. The nested adopter's staged current-state check works today only because it
resolves against the project root instead.

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
   to supply. Two children are excluded from their group's bare aggregate for stated reasons:
   `staged commit` takes a message file and has no input outside a commit-msg hook, and it remains
   directly invocable.

5. `check staged drift` is added: render from the staged config and compare against the staged
   output tree. It closes a hole no gate covers today, where a `.awf/` config change staged without
   its re-rendered output passes commit-time verification because bare check reads a working tree
   that is already rendered. Its scope is the rendered-output comparison only. The config-tree
   hygiene sweep and the dead-reference probe that `repo drift` also performs are explicitly out of
   scope, because a snapshot tree carries neither directory entries nor untracked files and both
   halves need a semantic decision this record does not make. Item 11 records the follow-on.

6. `awf check invariants` is removed with no replacement, together with `runInvariants`,
   `Project.CurrentStateInvariants`, and `InvariantReport`, whose only production caller it is.
   The false descriptions at `README.md:185` and `README.md:278` are removed rather than corrected,
   and `templates/docs/working-with-awf.md.tmpl:34` drops the line it ships to every adopter.

7. A disabled opt-in child prints a note stating it is disabled and naming the knob that disables it.
   Silence is not an acceptable report of a check that did not run.

8. `examples/sundial` enables both `proseGate` and `memoryCite`, so the showcase adopter exercises
   every aspect of the standard rather than only its unconditional parts. This keeps
   `tooling/quality-gates:example-zero-notes` true without a second output channel for item 7,
   because a knob that is on emits no disabled note.

9. The staged scanners resolve their config against the project root rather than the index root, so
   `check repo prose` and `check repo memory` are correct inside a nested adopter tree. Item 8
   depends on this: sundial cannot enable either gate until it holds.

10. `resolve` returns the leaf node and `ResolvedGating` resolves along the full ancestor chain, so a
    grandchild's gating and project-guard exemption are honoured. `globalHelp` and
    `UngatedGroupChildren` recurse to any depth, and the coverage-ignore at
    `internal/project/gatedcommands.go:19` is re-reasoned or removed against the new shape. Arity
    stays with the leaf: fixing `resolve` is preferred to hand-parsing a third token in the handler,
    which would otherwise stop `awf check staged commit extra-junk` being rejected.

11. A new schema migration clears any var whose value invokes the removed command, in either the
    pre-ADR-0159 `invariants` spelling or the `check invariants` spelling. The shipped 18-to-19
    migration at `internal/migrate/renameretiredcommands.go:16-22` is left untouched: editing it
    would change what an adopter replaying from schema 17 passes through and would desync it from
    the ADR that shipped it. The var is cleared rather than retargeted, because a removed command
    has no live replacement and mapping it to one would be false.

12. `docs/roadmap.md` replaces the superseded follow-on entry, recording that three of its four
    contracts are discharged here and carrying forward the two deferred halves of item 5 as a
    check-architecture cleanup.

## State changes

- update `tooling/cli:group-child-gating-honored`
- update `tooling/cli:group-child-project-guard-exemption`
- update `tooling/cli:help-lists-group-children`
- update `tooling/cli:gated-commands-generated`
- add `tooling/cli:check-universe-groups`
- add `tooling/cli:check-disabled-child-disclosure`
- update `tooling/quality-gates:example-adopter-checked`
- update `tooling/quality-gates:memory-citation-gate`
- update `tooling/quality-gates:prose-gate-refuses-without-git`
- update `tooling/audit-and-snapshots:commit-gate-shared-rule`
- update `code-design/dependency-composition:dependency-composition-commit-classification`
- add `rendering/sync-and-drift:staged-drift-rendered-output`

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
citation rules, which is the point, but it also means item 9 must land first or the example cannot
be rendered green.

An adopter upgrading past this schema loses a var value rather than having it rewritten. That is
deliberate: the alternative is a var that names a command which no longer exists, which fails inside
a hook rather than at upgrade time, the exact failure mode the ADR-0159 migration was written to
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
| Build `check staged drift` with the sweep and dead-reference halves included | Both need a semantic decision about what they mean over a tree with no directory entries and no untracked files; deciding them inside a blocking pre-commit gate is where a wrong answer is most expensive |

## Status history

- 2026-07-31: Proposed
