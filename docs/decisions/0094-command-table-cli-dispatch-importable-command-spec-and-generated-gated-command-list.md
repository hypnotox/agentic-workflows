---
status: Implemented
date: 2026-07-11
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [cli-dispatch]
related: [24, 27, 39, 55, 57, 81, 88, 92, 93]
domains: [tooling, rendering]
---
# ADR-0094: Command-table CLI dispatch, importable command spec, and generated gated-command list

## Context

The awf CLI dispatch in `cmd/awf/main.go` (469 lines) is a hand-rolled parser that
has accreted three distinct smells, flagged for review in ADR-0093 Decision item 4
("A broader review of the CLI dispatch/resolve plumbing (and the duplicated
gated-command prose noted below) is deferred to a separate effort"). This ADR is that
effort.

1. **Arguments are parsed three times over.** `checkArgs` validates flags and arity;
   then the `switch args[1]` re-extracts each command's values by hand, reaching back
   into the global `args` slice: `context`, `enable`, and `disable` each independently
   re-run `positionals(args[2:], ...)`, `new` hand-checks `len(args)`, and the four flag
   helpers (`hasFlag`, `valueFlag`, `setFlags`, `baseFlag`) each re-scan `args[2:]` from
   scratch. A handler never receives *parsed* arguments: it receives raw `args` and
   digs. Adding a command touches five hand-maintained places: the `argSpecs` map, the
   `commandOrder` slice, the hardcoded usage string (`main.go:61`), a `switch` arm with
   ad-hoc extraction, and a manual `gate()` call if the command is gated.

2. **`runEnable` / `runDisable` are near-mirror twins.** Both (~55 lines, `list_add.go`)
   walk the identical spine (gate → graph-flag check → target/singleton special-case →
   `PluralKind` → `project.Open` → validate → graph plan → `rewriteConfig` → sync),
   differing only in add-vs-remove direction and their pre/post notes. `newLocalArtifact`
   and `newLocalDoc` (`new.go`) are a second such twin pair (validate name → collision
   guards → write sidecar + stub → enable → sync).

3. **The gated-command list has no source of truth.** Each project-reading command calls
   `gate(root)` by hand before `project.Open` (verified: sync, check, invariants, audit,
   list, config, context, enable, disable, new; ten commands). That set is then
   re-transcribed as hand-maintained prose in two places: the `agents-doc.yaml`
   binary-version-gate invariant line and `.awf/domains/parts/tooling/current-state.md`.
   Nothing ties the prose to the code or the two copies to each other, and they have
   already drifted: `current-state.md` omits `config` and `context` (added by ADR-0088
   and ADR-0092), a staleness that *survived* ADR-0093 item 5 editing that very line.

Two constraints shape the fix. First, `config` and `context` gate *conditionally*: only
inside an adopted tree, degrading to a static answer outside one
(`inv: config-command-static-fallback`, ADR-0088; `inv: context-static-fallback`,
ADR-0092). A blunt pre-handler gate would break both. Second, single-sourcing the
gated-command list into rendered docs requires the source to be importable by
`internal/project` (whose `placeholderRegistry` produces such values); `cmd/awf`'s
`package main` is not importable, so the command metadata cannot stay there. The
render pipeline is single-pass (`inv: parts-raw`): only convention-part bodies pass
through `substitutePlaceholders` (`render.go:173`), while the `agents-doc.yaml`
`data.invariants` text renders as literal template data, so a `{{=awf:...}}` token or a
`{{ ... }}` reference embedded in that data emits verbatim rather than resolving.

A third-party CLI framework (cobra / urfave-cli / kong) was weighed and set aside: see
Alternatives. The chosen direction is a hand-rolled declarative command table.

## Decision

1. **Relocate the declarative command spec to a new importable leaf package
   `internal/clispec`.** Each command is a data value: name, one-line summary, flag
   spec (bool flags, value flags, repeatable value flags), positional bounds, a gating
   classification (Decision 3), a help body, and, for a group command (Decision 4),
   its child subcommands. `clispec` holds **data only**: no handler funcs, and no import
   of `cmd/awf` or `internal/project`, so it is a leaf and creates no import cycle.
   `cmd/awf` attaches handler funcs to the spec to build its runtime dispatcher;
   `internal/project` imports `clispec` to read the gated set (Decision 6). `clispec` is
   the single source for the command set: dispatch, `awf help` order, the top-level
   usage line, and the gated-command list all derive from it, with no parallel
   enumeration (`inv: cli-command-spec-single-source`).

2. **Replace the per-command `switch` with a generic parse-once driver in `cmd/awf`.**
   `run` becomes: handle the bare / `help` / `--help` forms off the spec; look the
   command up in the table (recursing into a group's children on the next positional,
   Decision 4); intercept `--help` / `-h`; parse and validate the arguments **once** into
   an `invocation{positionals, bools, values, multi}` value, folding today's
   `checkArgs`, `positionals`, `hasFlag`, `valueFlag`, `setFlags`, and `baseFlag` into a
   single pass; apply the gating classification (Decision 3); and call the handler with
   the parsed `invocation`. Handlers read the parsed `invocation`, never the raw `args`
   slice. A handler that needs work before its own gate (e.g. `context` resolving
   `--staged`/`--range` to paths) does it at the top of its handler; no generic
   driver-level pre-gate hook is introduced. The per-command `Usage:` / `Flags:` help block generates from the flag and
   positional spec, leaving only the descriptive paragraph hand-written. Adding a command
   becomes one `clispec` entry plus one handler: no `switch` arm, no `commandOrder`
   edit, no usage-string edit, no separate `gate()` call.

3. **Model gating as a three-valued classification, not a boolean.** Each command
   declares one of `ungated`, `gated`, or `gated-in-handler`. The driver applies
   `gate(root)` before the handler only for `gated` commands. `gated-in-handler`
   commands (`config`, `context`, and `new`) gate *inside* their handler rather than
   in the driver: `config`/`context` gate after the project-presence check that lets them
   degrade to a static answer (so `config-command-static-fallback` and
   `context-static-fallback` are preserved unchanged), and `new` gates after its name
   validation. (The name is `gated-in-handler`, not `gated-in-tree`, because `new` gates
   in its handler without any adopted-tree fallback: the classification is about *where*
   the gate runs, not about a tree.) The gate machinery itself (`gate()`,
   `version-compat-gate`, ADR-0039) is unchanged; only the call site moves from
   hand-written per-handler calls to the driver (for `gated`) or stays in the handler
   (for `gated-in-handler`).

4. **Subcommand nesting for `new`; kind-generic data dispatch for `enable` / `disable` /
   `list`.** A `clispec` command is either a leaf (has a handler) or a group (has named
   child subcommands the driver dispatches to on the next positional). `new` becomes a
   group whose children `adr`, `skill`, `agent`, `doc` are leaves with distinct handlers
   and their own arity and help. `enable`, `disable`, and `list` stay single leaf
   commands that dispatch on their `<kind>` positional through the one ordered
   `kindDescriptors` table (`inv: kind-dispatch-single-table`, ADR-0027); they are
   **not** modeled as per-kind subcommands, which would reintroduce the per-kind fan-out
   that invariant forbids and cannot represent freeform `domain` names. The
   `target` / `bootstrap` / `hooks` tokens remain the bespoke non-descriptor paths they
   are today.

5. **Collapse the twin handlers.** Extract the shared `enable` / `disable` prologue
   (gate → graph-flag check → `target`/`singleton` dispatch → kind lookup →
   `project.Open`) into one path and parameterize the direction; each direction keeps
   only its distinct tail (enable: forward-closure plan + domain current-state scaffold;
   disable: dependent-refusal + orphan and unrequired-agent notes). The per-kind enable/
   disable strategy is read from `kindDescriptors` rather than re-branched in each
   handler, extending, not forking, the single kind table. `newLocalArtifact` and
   `newLocalDoc` collapse to one shared local-scaffolder parameterized by kind. This is a
   real but *partial* dedup: the `target`/`singleton` arms and the direction-specific
   tails remain distinct by design, and the ADR does not overstate it as one function.

6. **Generate the gated-command list from one source.** The list is derived once from
   `clispec`: every **top-level** command whose gating classification is not `ungated`,
   in spec order. A group command (Decision 4) contributes only its single top-level
   token: the gating classification attaches to the group node (`new`) and its child
   subcommands are not enumerated separately, so the generated list reproduces the
   existing single `new` token rather than expanding to `new adr` / `new skill` / ... The
   generator yields an ordered list of bare command tokens, not a pre-formatted string;
   each consuming surface applies its own formatting (backticks and separator). The value
   is exposed through two render surfaces fed by that single list, with no hand-maintained
   enumeration surviving in either doc (`inv: gated-commands-generated`):
   - a new `{{=awf:gatedCommands}}` placeholder in the render placeholder registry
     (`internal/project/placeholders.go`), consumed by the `tooling` current-state
     convention part (which already passes such tokens through `substitutePlaceholders`);
     the current-state sentence that today slash-joins a hand-typed subset (omitting
     `config`/`context`) is **reworded** to consume the generated value, not
     token-substituted in place; and
   - a mirrored `gatedCommands` render value made available to the `AGENTS.md` template,
     which the binary-version-gate invariant line consumes. Because the invariant text
     is template *data* today (and the render is single-pass), delivering the value into
     that line lifts the command enumeration out of the hand-authored
     `data.invariants` text: the bullet references the shared value rather than spelling
     the list by hand. The exact assembly seam (a template-source reference to the render
     value versus splicing the value into the invariant data during data assembly, on the
     ADR-0089 doc-data-transform precedent) and the value's precise format (bare tokens
     versus a backticked list) are implementation details the plan fixes; the contract is
     that both surfaces trace to the one `clispec`-derived list and neither carries a
     hand-maintained enumeration.

7. **Complete the ADR-0093-deferred resolver rename.** Rename the plan/resolver
   set-vocabulary to the Enable/Disable surface vocabulary: `ResolveAdd`→`ResolveEnable`,
   `ResolveRemove`→`ResolveDisable`, `PlanOp.Add`→`PlanOp.Enable` (`internal/project/
   resolve.go`), and `addRemoveTarget` / `addRemoveSingleton` →
   `enableDisableTarget` / `enableDisableSingleton` (`cmd/awf/list_add.go`), with their
   tests. This is a **partial-item supersedence of ADR-0093 Decision item 4**: the
   sub-decision that the resolver vocabulary "stays" and the broader review is deferred.
   ADR-0093 anticipated this as the follow-through "when we do the refactor"; the refactor
   is now, and consistent Enable/Disable vocabulary across surface and internals is the
   gain that doing it in isolation lacked. Per the partial-item-supersedence convention
   this ADR links ADR-0093 via `related: [93]` (not `supersedes:`), ADR-0093 stays
   `Implemented`, and the `related:` back-pointer is added to ADR-0093 in this commit.
   The stable invariant slugs (`cli-config-kinds`, `remove-block-scoped`,
   `add-skill-pairs-agent`, `remove-agent-pairing-guard`, `add-applies-closure-plan`,
   `remove-refuses-dependents`) are **not** renamed: a slug is an identifier, not
   surface vocabulary, and renaming a backed slug is a separate, costlier act.

8. **No `.awf/` schema change.** `clispec`, the driver, and the gating classification are
   Go structure; the placeholder and render value are render internals; the
   `agents-doc.yaml` and `current-state.md` edits are config *content*, not schema.
   `migrate.Current()` is untouched and no `awf upgrade` is owed. A changelog entry is
   added.

## Invariants

- `invariant: cli-command-spec-single-source`: stated as a positive derivation the backing
  test asserts by equality: the top-level usage line, the `awf help` overview (its
  command set and order), and the generated gated-command list are each byte-equal to
  their `internal/clispec`-derived rendering, and the gated set is anchored to its known
  membership so a misclassification (e.g. marking `upgrade` gated, or dropping `context`)
  fails the test. There is no second command-order slice, hardcoded usage enumeration, or
  hand-written gated list in `cmd/awf` for these to diverge from.
- `invariant: gated-commands-generated`: the gated-command list rendered into awf-managed
  docs is generated from `internal/clispec` (exactly the top-level commands whose gating
  classification is not `ungated`, a group command contributing only its single token)
  through one generator feeding both the `{{=awf:gatedCommands}}` placeholder and the
  `AGENTS.md` render value; neither the `agents-doc` binary-version-gate line nor the
  `tooling` current-state part carries a hand-maintained enumeration of the set.
- The `config` and `context` static-fallback contracts (`config-command-static-fallback`,
  `context-static-fallback`) continue to hold: the three-valued gating classification
  keeps both commands `gated-in-handler`, gating inside the handler after the
  project-presence check, so neither refuses outside an adopted tree (textual; backed by
  the existing ADR-0088/ADR-0092 slugs).
- ADR-0027's `kind-dispatch-single-table` and `cli-config-kinds` continue to hold: the
  enable/disable strategy is *read from* `kindDescriptors`, extending the one table, not
  forking a per-kind dispatch (textual).
- No `Add` / `Remove` set-vocabulary identifier survives in the plan/resolver layer
  (`internal/project/resolve.go`, `internal/catalog` plan types) or in the
  `cmd/awf` toggle helpers; the exported and unexported dispatch/resolve vocabulary is
  Enable/Disable, matching the command surface (textual).

## Consequences

- **Adding a command is one spec entry plus one handler**: no `switch` arm, no
  `commandOrder` or usage-string edit, no manual `gate()` call. The single-point-of-
  addition goal is met and machine-checked (`cli-command-spec-single-source`).
- **The gated-command drift is designed out**, not merely guarded: the enumeration is
  generated from `clispec`, so a new gated command updates both docs on the next
  `sync`, and the already-stale `current-state.md` copy is corrected as a side effect.
- **`enable`/`disable` and `new` shed their twin duplication**, and the resolver speaks
  one vocabulary end to end.
- **New importable package `internal/clispec`** enters the architecture (a data-only
  leaf); `internal/project` gains a dependency on it for the gated set. Documented in
  the architecture doc.
- **Coverage-gate budget (ADR-0012):** the generic driver's branches (help intercept,
  group-node recursion, arity and unknown-flag errors, unknown command, the three gating
  arms) and every `clispec` helper each need a covering test to hold the 100% floor; a
  genuinely-unreachable branch takes a justified `// coverage-ignore:`. The data-only
  `clispec` poses no dead-code-gate risk (that gate targets functions, ADR-0063).
- **The `{{=awf:gatedCommands}}` value is the first placeholder that is a tool constant**
  rather than adopter-config-derived, but it is universal (every adopter runs the same
  awf binary, so the gated set is identical) and publication-safe (a comma-joined
  command list carries no `{{=awf` token, so `placeholder-value-token-free` cannot
  trip). Its empty-degradation path is effectively unreachable because the value is a
  non-empty compile-time constant; the publication-safety contract holds vacuously.
- **No behaviour or adopter-migration change; help *text* does shift**: the command
  surface, accepted flags, and gating behaviour are unchanged, the resolver rename is
  internal, and the new placeholder is additive, so there is no schema bump and no `awf
  upgrade`. But two output *texts* change: `awf <cmd> --help` `Usage:`/`Flags:` blocks
  are now generated from the spec, and restructuring `new` into subcommands shifts its
  malformed-invocation error messages. The existing help-parity tests (which iterate
  `argSpecs`/`commandOrder`) move to iterate `clispec` and update accordingly.
- **Coordination with ADR-0093:** this completes 0093's deferred item 4; the two do not
  conflict (0093 renamed the commands and top-level handlers, 0094 renames the resolver
  internals and restructures dispatch).

Doc-currency obligations the implementing commits must satisfy:

- Re-render all generated docs via `./x sync` (`AGENTS.md`, the `tooling`/`rendering`
  domain indexes, `architecture` for the new `internal/clispec` package and the
  `cmd/awf` driver, `working-with-awf` for the new `{{=awf:gatedCommands}}` placeholder,
  `ACTIVE.md`) and update the `tooling` current-state narrative's CLI references.
- Add a changelog `[Unreleased]` entry.
- Add the `related:` back-pointer to ADR-0093.
- Add glossary terms if warranted (e.g. "command spec", "gating classification").

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Adopt a third-party CLI framework (cobra / urfave-cli / kong) | Adds a dependency against the project's supply-chain caution; breaks the injected-writer test model (`run(args, stdout, stderr) int`); fights the 100%-coverage and dead-code gates (framework glue and reflection-based parsers strand branches); awf's flag surface is tiny and fixed and its dispatch is kind-generic (verbs × catalog kinds), a poor fit for a fixed subcommand tree. The one unique win, auto-generated help, is reproducible in the table. |
| Targeted de-dup only: keep the parser, single-source the gated list | Leaves the three-times-over arg re-parse and the enable/disable twins in place; the user chose the fuller restructure, and a durable single source for the gated list requires the importable spec anyway. |
| Model `enable`/`disable` kinds as subcommand nesting | Reintroduces the per-kind fan-out that `kind-dispatch-single-table` (ADR-0027) forbids and cannot represent freeform `domain` names; kind dispatch is data (one descriptor table), not a code tree. |
| A drift *guard* (a test asserting the prose matches the code) instead of generation | The code is not even a single source today, and a guard flags drift only after it is authored; variable-backed generation prevents it and was the explicit requirement. |
| Keep the resolver `Add` / `Remove` vocabulary (ADR-0093's recorded choice) | 0093 deferred this to "the refactor," not permanently; done as part of the bundled restructure, one Enable/Disable vocabulary across surface and internals is the gain that the isolated rename lacked. |
| A single pre-handler boolean gate for all gated commands | Would gate `config`/`context` ahead of their static-fallback branch and make them refuse outside an adopted tree, breaking `config-command-static-fallback` and `context-static-fallback`; the three-valued classification is the minimal fix. |
