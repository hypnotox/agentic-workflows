---
format: current-state-v2
status: Proposed
date: 2026-07-26
---
# ADR-0159: Regroup the verification commands under check and rename sync to render

## Context

awf's command surface has accumulated three unrelated naming schemes for a single concept. `awf check` is a bare verb with no object; `awf invariants` is a bare noun; `awf commit-gate`, `awf prose-gate`, and `awf memory-gate` are noun-gate compounds. All five are the same kind of thing: scan a defined surface, report findings, exit non-zero. Nothing in the surface explains why prose earns a `-gate` and drift earns a `check`, and a reader cannot tell from `awf help` which commands verify and which act.

The word `gate` compounds the problem by meaning four different things: `./x gate` (this project's full verification run), `awf <noun>-gate` (a single scanner), the binary-version compatibility gate (a function literally named `gate` in `cmd/awf/gate.go`, adjacent to `prosegate.go`, `memorygate.go`, and `commitgate.go` while meaning something unrelated), and the `gateCmd` var. Only the first two are adopter-facing, and they disagree about scope.

`awf sync` is separately unclear. It renders the `.awf/` config tree into the project; `sync` suggests a bidirectional reconciliation the command does not perform, and it does not read as the counterpart of the command that verifies its output.

A fresh-context grounding check (2026-07-26) verified the premises and surfaced four couplings the naming analysis had missed.

### Bare `awf check` decomposes into fewer parts than it appears to

`runCheck` is a version-ahead note, then `AdvisoryNotes()`, then `(*Project).Check()`, then `(*Project).CheckCurrentState()`. Config-tree hygiene is not a peer of drift: `sweepConfigTree` is called from inside `Check()` and appends to the same `[]manifest.Drift` slice, so a `drift` child carries hygiene automatically without further decomposition. `AdvisoryNotes()` is project-level and belongs to neither half.

### `--staged` is a mode switch that reaches three layers

`runCheck` branches to `runCheckStaged` and skips the drift oracle entirely, so `awf check` and `awf check --staged` verify disjoint surfaces under one name. The flag is not confined to the handler: the driver swaps the gate function on it (`if top.Name == "check" && inv.bools["--staged"] { gateFn = gateStaged }`), `gateStaged` reads the index lock rather than the working lock, `guardProjectState` computes its own `staged` from the same flag, and `checkLockVsBinary` branches on it for the ahead-note.

### Per-child gating is welded shut, and a second name-keyed preflight exists

The driver reads gating from the top-level node only, and `TestGroupChildrenCarryNoGating` fails if any group child sets `Gating != Ungated`, with the comment "A child that declared its own gating would be silently ignored - this guards that trap shut." The existing doc-comment anticipates a *Gated group* whose children inherit; this decision needs the inverse, an **Ungated child under a Gated group**, which no current mechanism expresses.

A second preflight is keyed on the same top-level name and was not visible from the command table at all. `guardProjectState` hard-exempts exactly `version`, `changelog`, `commit-gate`, `prose-gate`, and `memory-gate`. Folding those three under `check` would set `top.Name` to `check` and silently drop the exemption, so during a committed current-state journal or an attested lock `awf check commit` would refuse. That is precisely the window in which a commit-msg hook must still function, and the failure would surface as a blocked commit rather than a diagnostic.

### Grouping costs discoverability unless `awf help` is fixed

`globalHelp()` iterates `clispec.Commands` and prints name and summary with no recursion into `Children`. Today `prose-gate`, `memory-gate`, `commit-gate`, and `invariants` each appear in `awf help`; after regrouping all four would be reachable only through `awf check --help`, exactly as the `new` and `metrics` children are today. Since the motivation for this decision is discoverability, regrouping without fixing help would trade one opacity for another.

### The blast radius is concentrated in generated output and frozen history

Tracked mentions are roughly 1091 for `awf check`, 1084 for `./x gate`, and 709 for `awf sync`. The authored surface is far smaller: about 180, 79, and 98 respectively. The remainder lives under `docs/decisions/` and `docs/plans/`, which are append-only history. Those files are not rewritten by this decision. A completed ADR or plan naming `awf sync` records what the command was called when the decision was made, and correcting it forward is the job of the current-state corpus, not of edits to retained records.

Twelve current-state claims name a renamed command in their prose and need an `update` operation. Because bare `awf check` keeps its exact behaviour under this decision, the many claims that describe what `awf check` fails on remain true verbatim and are untouched.

### Precedent

ADR-0093 renamed `awf add`/`awf remove` to `awf enable`/`awf disable` with no backward-compatibility alias, and bundled both renames into one decision because they were one vocabulary change. This ADR follows both halves of that precedent: a clean break, and one record for one coherent renaming of the verification surface.

### Scope

This is the first of two decisions. It changes names and structure only; bare `awf check` behaves byte-identically before and after. A follow-on decision will make bare `awf check` run every enabled check and report what ran and what was skipped, which is where the exit-code contracts, the git precondition on the prose and memory scans, and the hook invocation counts are settled. Separating them keeps a large mechanical rename reviewable apart from a set of behavioural contract changes.

## Decision

1. Rename `awf sync` to `awf render`. The command renders the `.awf/` config tree into the project, and `render` says so where `sync` implies a reconciliation it does not perform. The name also pairs with the `drift` child added below, which verifies what `render` wrote. No alias is kept.

2. Make `check` a group command whose children name what each one checks:

   | new | replaces |
   |---|---|
   | `awf check` | `awf check` (unchanged behaviour) |
   | `awf check drift` | (new) the render-drift half of `awf check`, carrying config-tree hygiene |
   | `awf check state` | (new) the current-state half of `awf check` |
   | `awf check invariants` | `awf invariants` |
   | `awf check prose` | `awf prose-gate` |
   | `awf check memory` | `awf memory-gate` |
   | `awf check commit <FILE>` | `awf commit-gate <FILE>` |

   Bare `awf check` runs drift and current-state and prints the advisory notes, exactly as today, with byte-identical output. `AdvisoryNotes()` stays on the bare form only: it is project-level context belonging to neither child, and printing it from `check prose` would force a project open onto a command that must stay cheap.

3. Keep `--staged` as a flag on the group rather than promoting it to a child. It selects the universe to check against, not a thing to check, which is why `check memory` is already staged-only and `check drift` has no staged meaning. Keeping it a flag also preserves the existing `top.Name == "check" && --staged` special cases in the driver and in `guardProjectState` unchanged, and leaves the `awf check --staged` line in the rendered pre-commit payload and in the agent guide's staged-authority invariant working as written.

4. Let a group child declare a gating classification weaker than its parent's, and have the driver honour the child's. `check` is `Gated`; `check prose`, `check memory`, and `check commit` stay `Ungated` so a hook keeps invoking them without a version gate. `TestGroupChildrenCarryNoGating` is replaced by a test asserting the resolved gating, and the driver reads gating from the resolved node rather than from `top`. A child that sets no gating continues to inherit its parent's, so `metrics` and `new` are unaffected.

5. Give `guardProjectState` the same per-child treatment. Its exemption set moves from a list of top-level names to a property resolved on the same node the driver dispatches, so `check prose`, `check memory`, and `check commit` keep the exemption that `prose-gate`, `memory-gate`, and `commit-gate` hold today. Without this, a commit-msg hook would refuse during a committed journal or an attested lock. `check`, `check drift`, `check state`, and `check invariants` are not exempt, matching `check`'s current treatment.

6. Make `globalHelp()` list every group's children beneath their parent, so no command is discoverable only by knowing to ask a parent for help. This applies to `metrics` and `new` as well, which have the same defect today.

7. Widen `check`'s `MaxPos` to `-1` and give its handler ownership of the unknown-subcommand message, the treatment `new` already uses. Without this, `awf check bogus` dies in `parseArgs` with a generic "unexpected arguments" error before the handler can name the valid children. The handler restores the arity check that `MaxPos: 0` provided, so `awf check --staged extra-junk` still fails.

8. Leave every `*Cmd` var key unchanged, and migrate their values. The keys (`gateCmd`, `checkCmd`, `commitGateCmd`, `proseGateCmd`, `memoryGateCmd`, and the rest) are pinned as an exact set by `rendering/catalog-and-targets:var-descriptor-set-pinned`, are named literally by `validateCommandWiring`, and are hardcoded in `internal/render/vars.go`'s placeholder regex; renaming them is a distinct decision with its own migration and buys nothing this decision needs. `gateCmd` in particular keeps the `gate` word legitimately, because after this decision `gate` names exactly one thing: the project's own full verification run.

   Their *values* do name retired subcommands, and a clean break would otherwise fail inside an adopter's hook at commit time rather than at upgrade time. A schema-generation bump ships a `rename-retired-commands` migration that rewrites a var value consisting of an awf invocation token (`awf`, `./awf`, or a path ending in `/awf`) followed by exactly one retired subcommand token, preserving any trailing arguments: `sync` becomes `render`, `invariants` becomes `check invariants`, `prose-gate` becomes `check prose`, `memory-gate` becomes `check memory`, and `commit-gate` becomes `check commit`. Any other value is left untouched, including `checkCmd: ./x check`, which names the adopter's own runner whose verbs awf cannot know. Descriptor descriptions and `Options` lists are corrected in the same change, `activeMdRegenCmd` and `commitScopes` included, both of which name retired commands today.

9. Update the two `{{ else }}` fallbacks in `templates/hooks/pre-commit.sh.tmpl` so an unset `proseGateCmd` or `memoryGateCmd` degrades to `check prose` and `check memory` rather than to retired command names, and update the `x` runner's two hardcoded gate steps.

10. Do not rewrite `docs/decisions/**` or `docs/plans/**`. A retained record naming `awf sync` states what the command was called when that decision was made. This ADR is the forward correction; the current-state corpus carries the present names.

11. Claim slugs are identities, not descriptions. `tooling/quality-gates:prose-gate-refuses-without-git`, `tooling/quality-gates:memory-citation-gate`, and `tooling/audit-and-snapshots:commit-gate-shared-rule` keep their slugs and have their prose updated to name the new commands. Renaming a slug is a remove plus an add, which retires an id that can never be reused and discards the claim's provenance, for no gain.

12. Every status transition of this ADR regenerates `docs/decisions/INDEX.md` (and `docs/config-reference.md`, once the descriptor text lands) via `./x sync` in the same commit.

## State changes

- add `tooling/cli:group-child-gating-honored`
- add `tooling/cli:help-lists-group-children`
- update `tooling/cli:gated-commands-generated`
- update `config/migrations-and-locks:noop-autobump`
- update `config/migrations-and-locks:upgrade-gate`
- update `rendering/companion-scripts:runner-singleton-toggle`
- update `rendering/doc-outputs:topic-output-complete`
- update `rendering/inplace-and-placeholders:part-placeholder-sandboxed`
- update `rendering/singletons-and-payloads:memory-gitignore-always-on`
- update `rendering/sync-and-drift:sync-always-writes-active-md`
- update `rendering/sync-and-drift:sync-backs-up-foreign`
- update `tooling/quality-gates:example-adopter-checked`
- update `tooling/quality-gates:prose-gate-refuses-without-git`
- update `tooling/quality-gates:memory-citation-gate`
- update `tooling/audit-and-snapshots:commit-gate-shared-rule`

## Consequences

- The verification surface reads as one family. A reader of `awf help` sees `render` and a `check` group whose children each name their subject, instead of five commands in three naming schemes. `gate` narrows to exactly one meaning, the project's own full verification run, which is what `gateCmd` has always held.
- `awf check drift` and `awf check state` are newly runnable alone. Neither exists today: the only way to run the drift oracle is to run the current-state evaluation with it. This is additive; nothing depends on their absence.
- `gated-commands-generated` stops being a top-level-only projection. Its backing test pins a literal command list, so the claim cannot be quietly falsified: the list fails until it is edited to carry the resolved per-child classification, and the generated gated-command list in the agent guide and the config reference follows.
- Per-child gating and the per-child project-state exemption are two changes to a driver that deliberately had neither. The `guardProjectState` half is the one that matters for correctness: without it, this decision would break commit-msg hooks in exactly the recovery windows where they are most needed. Both are covered by the new `group-child-gating-honored` invariant so the property is pinned rather than implied.
- The help fix improves `metrics` and `new` as a side effect. Their children are undiscoverable today for the same reason, and there is no reason to fix the defect only where this decision introduces it.
- Adopters take one break with a migration behind it. The realistic population is covered: a var value pointing at a retired awf subcommand is rewritten at `awf upgrade`, and a value naming the adopter's own runner is untouched because awf cannot know what verbs that runner has. An adopter whose value is neither shape (an inline shell fragment, say) is not migrated and will see the failure in their hook; the changelog names the rename explicitly, and the population is small enough that a permanent retired-subcommand denylist in config validation is the wrong trade - it would be a forever-cost carried to catch a one-release break.
- Twelve claims are reworded and none change meaning. The rename is mechanical for all of them, and the current-state handshake forces them to land with the change rather than drifting behind it.
- Frozen history keeps the old names, so a reader moving between a 2026-07 plan and the current corpus sees both vocabularies. Decision 10 accepts this deliberately: the alternative is rewriting roughly 900 lines across retained ADRs and plans, which the append-only invariant forbids and which would falsify what those records actually decided.
- Bare `awf check` is byte-identical, so this decision cannot regress the pre-commit path, `./x check`, or the example adopter's output. Every behavioural question the grounding check raised (the git precondition on the prose and memory scans, the triple invocation in the pre-commit payload, the exit code when every check is skipped) belongs to the follow-on decision and is untouched here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the flat surface and only rewrite the help text | Leaves three naming schemes for one concept and leaves `--staged` a hidden mode switch; the confusion is structural, not a wording problem |
| Rename flatly to `check-drift`, `check-prose`, and so on without grouping | Gets descriptive names with no driver changes, but forgoes a shared `--staged` and a single help entry, and leaves seven top-level commands where one group communicates the family |
| Promote `staged` to a child alongside `prose` and `memory` | Puts a universe next to a set of subjects; `check memory` is already staged-only, so the two axes visibly cross, and it would break the `awf check --staged` line in the rendered payload and the agent guide invariant |
| Require a subcommand, with no bare `awf check` | Maximally explicit, but rewrites the single most-invoked command in every doc, skill, hook, and CI line for no gain over a default that already means something coherent |
| Rename `sync` to `apply` | Reads as config-as-source-of-truth, but is no more descriptive than `sync` about what happens and costs the same rename |
| Keep `sync`, on the grounds that in-place sections read output back | The read-back exists as machinery but has had no consumer since ADR-0156 replaced ADR-0101's `x` with a single-section wrapper carrying no in-place regions, and the effort owner confirmed `render` is the clearer name regardless of whether that primitive is ever used again |
| Rename the `*Cmd` var keys to match | A separate decision with its own migration: the key set is pinned by a claim whose test asserts it exactly, the keys are named literally in `validateCommandWiring`, and two are hardcoded in a placeholder regex; `gateCmd` also keeps the `gate` word legitimately |
| Leave adopter var values alone and document the break in the changelog | The failure would surface inside a hook at commit time with no diagnostic pointing at the cause, which is the worst place to discover a rename |
| Refuse in `validateCommandWiring` on any value naming a retired subcommand | Requires carrying a retired-name denylist permanently to catch a single release's break; the migration covers the realistic population at no ongoing cost |
| Rename the three claim slugs that carry old command names | A slug is an identity, not a description; renaming is a remove plus an add that burns an id forever and discards provenance |
| Rewrite the old command names throughout `docs/decisions/` and `docs/plans/` | Forbidden by the append-only invariant, and it would misrepresent what those records decided at the time |
| Fold the behaviour change into this ADR | Mixes a large mechanical rename with four contract changes and a git-precondition regression in one review surface; the seam between them is clean, so they are two decisions |

## Status history

- 2026-07-26: Proposed
