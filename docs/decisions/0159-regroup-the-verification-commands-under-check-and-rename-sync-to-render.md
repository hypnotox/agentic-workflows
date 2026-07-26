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

`runCheck` is a version-ahead note, then `AdvisoryNotes()`, then `(*Project).Check()`, then `(*Project).CheckCurrentState()`. Config-tree hygiene is not a peer of drift: `sweepConfigTree` is called from inside `Check()` and appends to the same `[]manifest.Drift` slice, so a `drift` child carries hygiene automatically without further decomposition. `AdvisoryNotes()` and the version-ahead note are both project-level and belong to neither half.

### `--staged` is a mode switch that reaches three layers

`runCheck` branches to `runCheckStaged` and skips the drift oracle entirely, so `awf check` and `awf check --staged` verify disjoint surfaces under one name. The flag is not confined to the handler: the driver swaps the gate function on it (`if top.Name == "check" && inv.bools["--staged"] { gateFn = gateStaged }`), `gateStaged` reads the index lock rather than the working lock, `guardProjectState` computes its own `staged` from the same flag, and `checkLockVsBinary` branches on it for the ahead-note.

All three predicates key on `top.Name`, which the driver deliberately resolves to the top-level node. Regrouping therefore widens each of them from one invocation to seven without a single character changing, which is why Decision 3 re-scopes them explicitly rather than claiming they are preserved.

### Per-child gating is welded shut, and a second name-keyed preflight exists

The driver reads gating from the top-level node only, and `TestGroupChildrenCarryNoGating` fails if any group child sets `Gating != Ungated`, with the comment "A child that declared its own gating would be silently ignored - this guards that trap shut." The existing doc-comment anticipates a *Gated group* whose children inherit; this decision needs the inverse, an **Ungated child under a Gated group**, which no current mechanism expresses.

A second preflight is keyed on the same top-level name and was not visible from the command table at all. `guardProjectState` hard-exempts exactly `version`, `changelog`, `commit-gate`, `prose-gate`, and `memory-gate`. Folding those three under `check` would set `top.Name` to `check` and silently drop the exemption, so during a committed current-state journal or an attested lock `awf check commit` would refuse. That is precisely the window in which a commit-msg hook must still function, and the failure would surface as a blocked commit rather than a diagnostic.

### Grouping costs discoverability unless `awf help` is fixed

`globalHelp()` iterates `clispec.Commands` and prints name and summary with no recursion into `Children`. Today `prose-gate`, `memory-gate`, `commit-gate`, and `invariants` each appear in `awf help`; after regrouping all four would be reachable only through `awf check --help`, exactly as the `new` and `metrics` children are today. Since the motivation for this decision is discoverability, regrouping without fixing help would trade one opacity for another.

### The blast radius, measured

Tracked mentions divide into three populations with different costs. Counting `git grep -F` hits over the whole tree, then over `docs/decisions/` plus `docs/plans/`, then over the authored inputs (`.awf/`, `templates/`, `cmd/`, `internal/`, `x`, `changelog/`, `tools/`, `.github/`), with the remainder being rendered output:

| term | total | append-only history | authored inputs | rendered output |
|---|---|---|---|---|
| `awf check` | 1113 | 733 | 180 | 200 |
| `awf sync` | 714 | 249 | 98 | 367 |
| `./x gate` | 1084 | 855 | 79 | 150 |

Only the authored column is hand-edited. The rendered column is rewritten automatically by a re-render, and the append-only column is not rewritten at all: a completed ADR or plan naming `awf sync` records what the command was called when that decision was made, and correcting it forward is the job of the current-state corpus, not of edits to retained records.

### Which claims need updating, and why

A claim is updated when its prose names a renamed command **as an invocation**; prose that uses `sync` or `check` as a generic operation noun is left alone. Twelve claims meet that test. Two more are included for a different reason: `config/configuration:awf-config-root` and `config/migrations-and-locks:legacy-read-isolation` both enumerate "no ordinary load, render, sync, or check path", already using `render` for a distinct pipeline stage, so renaming the command makes each enumeration name the same word twice. Two final ones are updated for mechanism rather than vocabulary: `tooling/cli:gated-commands-generated` for the per-child gating change, and `config/configuration:config-serialization-owned` for the string-valued config editor the migration needs (Decision 8), whose prose enumerates the serialization funnel's members exactly.

Because bare `awf check` keeps its exact behaviour under this decision, the many claims that describe what `awf check` fails on remain true verbatim and are untouched.

### Precedent

ADR-0093 renamed `awf add`/`awf remove` to `awf enable`/`awf disable` with no backward-compatibility alias, and bundled both renames into one decision because they were one vocabulary change. This ADR follows both halves of that precedent: a clean break, and one record for one coherent renaming of the verification surface.

### Scope

This is the first of two decisions. It changes names and structure only: bare `awf check` runs the same checks over the same surfaces and returns the same exit status before and after, its output differing only where a renamed command appears inside a message. A follow-on decision will make bare `awf check` run every enabled check and report what ran and what was skipped, which is where the exit-code contracts, the git precondition on the prose and memory scans, and the hook invocation counts are settled. Separating them keeps a large mechanical rename reviewable apart from a set of behavioural contract changes.

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

   Bare `awf check` runs drift and current-state, prints the advisory notes, and prints the version-ahead note, exactly as today: the same checks over the same surfaces, the same exit status, and the same output apart from the renamed command inside the version-ahead note. Both notes stay on the bare form only. They are project-level context belonging to neither child, and printing them from `check prose` would force a project open onto a command that must stay cheap. The version-ahead note's message text changes with Decision 1, from `run awf sync to re-pin` to `run awf render to re-pin`.

3. Keep `--staged` as a flag on the bare group form, and reject it on every child with a usage error. It selects the universe to check against, not a thing to check, which is why `check memory` is already staged-only and `check drift` has no staged meaning. Nothing is lost: bare `awf check --staged` already is the staged current-state check, so no child needs to spell it a second way. The follow-on decision may widen the flag to children once a ran/skipped report exists to report a skip honestly; shipping it earlier would let `awf check drift --staged` exit zero having silently done nothing, the exact hidden-mode defect Decision 2 removes.

   The three `--staged` predicates are re-scoped, not preserved. `top.Name == "check"` is true for all seven invocations after regrouping, so the driver's gate switch, `guardProjectState`'s staged computation, and `checkLockVsBinary`'s ahead-note branch each change from a top-level-name test to a bare-check test (the group resolved with no child). This is a code change in three places, and the flag's rejection on children is what keeps it correct.

4. Let a group child declare a gating classification weaker than its parent's, and have the driver honour the child's. `check` is `Gated`; `check prose`, `check memory`, and `check commit` stay `Ungated` so a hook keeps invoking them without a version gate. `TestGroupChildrenCarryNoGating` is replaced by a test asserting the resolved gating, and the driver reads gating from the resolved node rather than from `top`. A child that sets no gating continues to inherit its parent's, so `metrics` and `new` are unaffected.

   The published projection stays two separate lists rather than one flat slice, because a member and an exclusion are different kinds of fact and a single `[]string` gives a renderer no way to tell them apart. `GatedCommandNames()` keeps returning only genuinely gated names, and a sibling projection returns the group children that are **weaker** than their parent (an ungated child under a gated parent). A child that merely *differs* is not automatically an exclusion: a hypothetical gated child under an ungated parent is a member, and the two projections classify it that way.

   Today `GatedCommandNames()` returns thirteen names. After this decision the gated set is twelve, because `invariants` is no longer top-level and `sync` is spelled `render`: `render`, `check`, `audit`, `metrics`, `doctor`, `list`, `config`, `context`, `topic`, `new`, `enable`, `disable`. The sibling projection returns `check prose`, `check memory`, and `check commit`, which the doc surface renders as named exclusions. The `gated-commands-generated` update is worded to that two-list rule.

5. Give `guardProjectState` the same per-child treatment. Its exemption set moves from a list of top-level names to a property resolved on the same node the driver dispatches, so `check prose`, `check memory`, and `check commit` keep the exemption that `prose-gate`, `memory-gate`, and `commit-gate` hold today. Without this, a commit-msg hook would refuse during a committed journal or an attested lock. `check`, `check drift`, `check state`, and `check invariants` are not exempt, matching `check`'s current treatment.

6. Make `globalHelp()` list every group's children beneath their parent, so no command is discoverable only by knowing to ask a parent for help. This applies to `metrics` and `new` as well, which have the same defect today. The property lands as a `Backing: test` claim, with its proof marker on an extension of `cmd/awf/help_test.go`'s `TestCliCommandSpecSingleSource`, which today asserts only that top-level names appear in clispec order.

7. Widen `check`'s `MaxPos` to `-1` and give its handler ownership of the unknown-subcommand message, the treatment `new` already uses. Without this, `awf check bogus` dies in `parseArgs` with a generic "unexpected arguments" error before the handler can name the valid children. The handler restores the arity check that `MaxPos: 0` provided, so `awf check --staged extra-junk` still fails.

   `resolve` tests only `args[1]` for a child name, so `awf check --staged drift` reaches the handler as the bare group with `drift` as a positional. The handler must distinguish a positional that names a valid child from genuine junk and say the subcommand comes first (`awf check drift`), rather than listing `drift` among the valid children while refusing an invocation that named one.

8. Leave every `*Cmd` var key unchanged, and migrate their values. The keys (`gateCmd`, `checkCmd`, `commitGateCmd`, `proseGateCmd`, `memoryGateCmd`, and the rest) are pinned as an exact set by `rendering/catalog-and-targets:var-descriptor-set-pinned`, are named literally by `validateCommandWiring`, and are hardcoded in `internal/render/vars.go`'s placeholder regex; renaming them is a distinct decision with its own migration and buys nothing this decision needs. `gateCmd` in particular keeps the `gate` word legitimately, because after this decision `gate` names exactly one thing: the project's own full verification run.

   Their *values* do name retired subcommands, and a clean break would otherwise fail inside an adopter's hook at commit time rather than at upgrade time. This repository's own `activeMdRegenCmd: ./x sync` is such a value. A schema-generation bump ships a `rename-retired-commands` migration that rewrites a var value consisting of an awf invocation token (`awf`, `./awf`, or a path ending in `/awf`) followed by exactly one retired subcommand token, preserving any trailing arguments: `sync` becomes `render`, `invariants` becomes `check invariants`, `prose-gate` becomes `check prose`, `memory-gate` becomes `check memory`, and `commit-gate` becomes `check commit`. Any other value is left untouched, including one naming an adopter's own runner, whose verbs awf cannot know; this repository updates its `activeMdRegenCmd` by hand alongside the `x` rename in Decision 9.

   The migration needs a config editor that does not exist. `internal/config`'s funnel writes a bool (`SetMappingScalar`), an int (`SetMappingInteger`), sequences (`SetArray`, `SetArrayMember`), and a string into an *absent* key (`SeedVarKey`); nothing rewrites a present string value. A string-valued sibling of the typed editors is added, and because `config/configuration:config-serialization-owned` enumerates the funnel's members exactly, this decision updates that claim rather than leaving it false. Hand-rolling the write instead was rejected: the funnel exists so no caller emits YAML directly.

   Five var descriptors name a retired command in their description or `Options` and are corrected in the same change: `activeMdRegenCmd`, `commitGateCmd`, `proseGateCmd`, `memoryGateCmd`, and `commitScopes`. `commitScopes` is not a config var: it carries `Target: "audit-scopes"` and its answer lands under `audit.allowedScopes`, so it is outside the migration's reach by construction rather than by the matcher's shape rule, and only its description prose ("enforced by awf commit-gate/audit") needs correcting.

   Five `internal/configspec` key entries carry retired names in their `Description` or `Availability` prose and are corrected alongside them: `audit.allowedTypes`, `audit.allowedScopes`, `audit.subjectMaxLength`, `proseGate.enabled`, and `memoryCite.enabled`. These are neither var descriptors nor call sites, and they render verbatim into `docs/config-reference.md`, so regenerating that file without correcting them would faithfully reproduce the retired names.

9. Update every rendered and hand-written call site that names a retired command.

   Three hook-template fallbacks degrade to a retired name when their var is unset, and all three are corrected: `proseGateCmd` and `memoryGateCmd` in `templates/hooks/pre-commit.sh.tmpl`, and `commitGateCmd` in `templates/hooks/commit-msg.sh.tmpl`. The third is the one that matters most, because a fallback degrading to a nonexistent `commit-gate` would land in the commit-msg hook, the exact surface Decision 5 exists to keep working, and because the publication-safe-templates invariant requires an unset var to degrade to something coherent rather than to a command that no longer exists.

   The provenance banner is the rename's highest-multiplicity mention and is easy to miss because it is not a call site. `internal/project/banner.go`'s `bannerText` reads "GENERATED by awf: do not edit; change .awf/ and run `awf sync`" and is stamped into the first line of every managed file: 318 tracked files in this repository, and every rendered file in every adopter tree. It changes with the command.

   The `x` runner is renamed and rewired in the same change. Its `sync` verb becomes `render`, so the runner and the tool use one word for one operation, and its five retired-command call sites are updated: `./awf prose-gate` and `./awf memory-gate` (the gate step), `./awf sync` (the render verb), `"$bindir/awf" sync` (the example re-render), and `"$bindir/awf" invariants` (the example invariant run). `./x check` and `./x gate` keep their names.

10. Do not rewrite retained history. The rule is about records, not directories: the ADR and plan files themselves, the dated analyses under `docs/research/`, and the released version sections of `changelog/`. A retained record naming `awf sync` states what the command was called when that decision was made, a dated analysis states what was true when it was written, and a released changelog entry states what shipped under that version; rewriting any of them falsifies it.

   Two categories inside those directories are explicitly *not* retained history and are rewritten normally. The awf-managed files that live among the records (`docs/decisions/INDEX.md`, and the `README.md` and `template.md` in both `docs/decisions/` and `docs/plans/`) are rendered output: they are lock-tracked, they carry the provenance banner Decision 9 changes, and preserving them would produce drift that fails `awf check`. And the changelog's `## [Unreleased]` section is in-flight, not released, so it gains an entry for this break.

   This ADR is the forward correction; the current-state corpus carries the present names.

11. Claim slugs and topic ids are identities, not descriptions. Six slugs carry retired vocabulary and all six keep it: `tooling/quality-gates:prose-gate-refuses-without-git`, `tooling/quality-gates:memory-citation-gate`, `tooling/audit-and-snapshots:commit-gate-shared-rule`, `rendering/sync-and-drift:sync-always-writes-active-md`, and `rendering/sync-and-drift:sync-backs-up-foreign` have their prose updated to name the new commands; `tooling/quality-gates:prose-gate-tracked-file-scan` appears in no operation list because its body says "the prose scanner" and never names the command, so only its slug carries the old word. The topic id `rendering/sync-and-drift`, which prefixes seventeen claims, keeps its name on the same grounds. Renaming a slug is a remove plus an add, which retires an id that can never be reused and discards the claim's provenance, for no gain.

12. The Implemented commit carries the whole authored surface, not only the generated index. Thirty tracked files under `.awf/` name a retired command (`git grep -lE 'awf (sync|invariants)|(prose|memory|commit)-gate' -- .awf`): five are rendered payloads that a re-render rewrites (`.awf/hooks/{pre-commit,commit-msg,pre-push}.sh`, `.awf/memory/.gitignore`, `.awf/metrics/.gitignore`), leaving twenty-five authored inputs. Those include `.awf/domains/parts/{config,tooling,rendering,invariants}/current-state.md`, `.awf/parts/workflow/{commit-discipline,composing-the-gate,local-hooks}.md`, `.awf/docs/glossary.yaml`, `.awf/docs/pitfalls.yaml`, `.awf/agents/plan-reviewer.yaml`, and `.awf/config.yaml`. AGENTS.md renders from several of these and today carries retired names in three invariants and in the generated gated-command list, so the agent guide is updated at its source. `./x render` then regenerates AGENTS.md, `docs/decisions/INDEX.md`, and `docs/config-reference.md` from those inputs, in the same commit as every status transition of this ADR.

## State changes

- add `tooling/cli:group-child-gating-honored`
- add `tooling/cli:group-child-project-guard-exemption`
- add `tooling/cli:help-lists-group-children`
- update `tooling/cli:gated-commands-generated`
- update `config/configuration:awf-config-root`
- update `config/configuration:config-serialization-owned`
- update `config/migrations-and-locks:noop-autobump`
- update `config/migrations-and-locks:upgrade-gate`
- update `config/migrations-and-locks:legacy-read-isolation`
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
- `gated-commands-generated` stops being a top-level-only projection without the published list bloating: Decision 4 keeps the gated set to genuinely gated names and reports weaker children separately, so the agent guide shows twelve gated top-level names plus three named exclusions rather than a dozen inherited entries. Two tests move: `internal/clispec/clispec_test.go`'s `TestGatedCommandNames` pins the literal thirteen-name list and fails until edited, and the marker-carrying `TestGatedCommandsDisplay` in `internal/project` asserts the rendered string is exactly the comma-joined gated names, so it fails too once the exclusion clause joins the rendering. Both must land with the change.
- Per-child gating and the per-child project-state exemption are two changes to a driver that deliberately had neither, and they land as two claims because they are two independently enforceable properties in two code paths, each provable by its own test. The `guardProjectState` half is the one that matters for correctness: without it, this decision would break commit-msg hooks in exactly the recovery windows where they are most needed.
- The help fix improves `metrics` and `new` as a side effect. Their children are undiscoverable today for the same reason, and there is no reason to fix the defect only where this decision introduces it.
- **Every adopter is forced through `awf upgrade`, including those with nothing to migrate.** Registering the `rename-retired-commands` migration bumps the schema generation, which trips `config/migrations-and-locks:upgrade-gate`: a project below the new generation is refused on `check` and `render` with a run-awf-upgrade message until it upgrades, whether or not any of its var values matched the rewrite. `examples/sundial` is such a project, setting only `gateCmd`, `gateCmdFull`, `testCmd`, and `invariantTestPath`. This is accepted because the alternative is shipping the descriptor and template corrections with no registered migration at all, which would leave every adopter whose value *does* match discovering the break inside a hook at commit time; a forced upgrade with a clear message is the better failure. Adopters who track releases run `awf upgrade` on every schema bump already.
- Twelve claims are reworded for the rename and two more are corrected because their `render`/`sync` enumeration would otherwise name one concept twice. None change meaning, and the current-state handshake forces them to land with the change rather than drifting behind it.
- Changing the provenance banner rewrites the first line of every rendered file. The Implemented commit therefore carries a 318-file diff here that is entirely content-free, and every adopter's first `awf render` after upgrading produces the same whole-tree churn. That is unavoidable once the command is renamed (the banner tells a reader which command to run) and is the reason the banner is named explicitly in Decision 9 rather than left to be discovered as drift.
- Frozen history keeps the old names, so a reader moving between a 2026-07 plan and the current corpus sees both vocabularies. Decision 10 accepts this deliberately: the alternative is rewriting the 733 `awf check` and 249 `awf sync` mentions across retained ADRs and plans, which the append-only invariant forbids and which would falsify what those records actually decided.
- Adopters take one break with a migration behind it. A var value pointing at a retired awf subcommand is rewritten at upgrade, and a value naming the adopter's own runner is untouched because awf cannot know what verbs that runner has. An adopter whose value is neither shape (an inline shell fragment, say) is not migrated and will see the failure in their hook; the changelog names the rename explicitly, and the population is small enough that a permanent retired-subcommand denylist in config validation is the wrong trade - it would be a forever-cost carried to catch a one-release break.
- Bare `awf check` keeps its check selection and its exit status, so this decision cannot regress the pre-commit path, `./x check`, or the example adopter's verdict; the only output that moves is a renamed command inside the version-ahead note. Every behavioural question the grounding check raised (the git precondition on the prose and memory scans, the triple invocation in the pre-commit payload, the exit code when every check is skipped) belongs to the follow-on decision and is untouched here.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the flat surface and only rewrite the help text | Leaves three naming schemes for one concept and leaves `--staged` a hidden mode switch; the confusion is structural, not a wording problem |
| Rename flatly to `check-drift`, `check-prose`, and so on without grouping | Gets descriptive names with no driver changes, but forgoes a shared `--staged` and a single help entry, and leaves seven top-level commands where one group communicates the family |
| Keep the retired names as deprecating aliases for one release | Doubles the command surface this decision exists to shrink, and puts two spellings of every check in `awf help` at the moment the point is to make the family legible; the migration already moves adopters at upgrade time rather than leaving them to discover the break at commit time, which is what an alias would buy |
| Promote `staged` to a child alongside `prose` and `memory` | Puts a universe next to a set of subjects; `check memory` is already staged-only, so the two axes visibly cross, and it would break the `awf check --staged` line in the rendered payload and the agent guide invariant |
| Accept `--staged` on every child | With no ran/skipped report until the follow-on decision, `awf check drift --staged` would exit zero having silently done nothing, reintroducing the hidden skip this ADR removes |
| Require a subcommand, with no bare `awf check` | Maximally explicit, but rewrites the single most-invoked command in every doc, skill, hook, and CI line for no gain over a default that already means something coherent |
| Rename `sync` to `apply` | Reads as config-as-source-of-truth, but is no more descriptive than `sync` about what happens and costs the same rename |
| Keep `sync`, on the grounds that in-place sections read output back | The read-back exists as machinery but has had no consumer since ADR-0156 replaced ADR-0101's `x` with a single-section wrapper carrying no in-place regions, and the effort owner confirmed `render` is the clearer name regardless of whether that primitive is ever used again |
| Keep the `x` runner's `sync` verb and change only the awf command it calls | Leaves the runner and the tool using different words for one operation, which is the split this decision exists to close; `x` is hand-written and repo-local, so the rename costs only its own call sites |
| Rename the `*Cmd` var keys to match | A separate decision with its own migration: the key set is pinned by a claim whose test asserts it exactly, the keys are named literally in `validateCommandWiring`, and two are hardcoded in a placeholder regex; `gateCmd` also keeps the `gate` word legitimately |
| Leave adopter var values alone and document the break in the changelog | The failure would surface inside a hook at commit time with no diagnostic pointing at the cause, which is the worst place to discover a rename |
| Refuse in `validateCommandWiring` on any value naming a retired subcommand | Requires carrying a retired-name denylist permanently to catch a single release's break; the migration covers the realistic population at no ongoing cost |
| Rename the three claim slugs that carry old command names | A slug is an identity, not a description; renaming is a remove plus an add that burns an id forever and discards provenance |
| Rewrite the old command names throughout `docs/decisions/` and `docs/plans/` | Forbidden by the append-only invariant, and it would misrepresent what those records decided at the time |
| Fold the behaviour change into this ADR | Mixes a large mechanical rename with four contract changes and a git-precondition regression in one review surface; the seam between them is clean, so they are two decisions |

## Status history

- 2026-07-26: Proposed
