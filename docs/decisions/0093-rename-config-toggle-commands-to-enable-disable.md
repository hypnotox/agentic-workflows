---
status: Implemented
date: 2026-07-11
tags: [cli-dispatch, enable-closure]
related: [24, 37, 39, 92, 94]
domains: [tooling, config]
---
# ADR-0093: Rename config-toggle commands to `enable`/`disable`

## Context

`awf add <kind> <name>` and `awf remove <kind> <name>` (ADR-0024) do not create or
delete anything. They flip an artifact's membership in a `.awf/config.yaml` enable
array (or a singleton's `enabled:` scalar) and re-render. The artifact already
exists in the catalog; the command toggles whether *this project* renders it.

The verbs misdescribe that operation on two counts:

- **`add` reads as "create," colliding with `awf new`.** `awf new` genuinely
  scaffolds a project-local artifact (new files); `awf add` only toggles an
  existing catalog entry. The two reading as near-synonyms is the core confusion,
  and the commands' own help already reaches for the opposite vocabulary, summarising
  `add` as "**Enable** a target" and `remove` as "**Disable** a target"
  (cmd/awf/main.go:283,296). The names fight their own documentation.
- **The help noun "target" collides with the `target` kind.** The help uses "target"
  generically for the toggled artifact, but `target` is also a literal kind (an
  adapter runtime), so `awf add target cursor` parses as "enable a *target* of kind
  *target*."

ADR-0024's substance is sound: the required `<kind>` token, per-kind validation, the
block-scoped generic array editor, the doc-gate warning and orphan note, and `awf
list`. Only the two surface verbs and the help grammar are wrong. This is a pre-1.0,
no-alias project surface; ADR-0024 itself set the precedent that a breaking CLI-grammar
change is acceptable ("The bare `awf add <name>` form is removed").

## Decision

1. **Rename the two config-toggle commands to `awf enable <kind> <name>` and `awf
   disable <kind> <name>`,** replacing `add`/`remove` outright with **no
   backward-compat alias.** The verbs mirror the config `enable` arrays and each
   singleton's `enabled:` scalar, and match the verbs the commands' own help already
   used. Because the agent guide and docs are *rendered*, an adopter's `AGENTS.md` and
   rendered docs switch to the new verbs on their next `sync` and their agent follows
   the rendered instructions: the migration propagates without hand-editing.

2. **Supersede ADR-0024 Decision items 1 and 6 (partial-item supersedence:
   `refines: ADR-0024#1`, `refines: ADR-0024#6`):** the
   command names and the help/README/guide grammar. Every other ADR-0024 commitment
   (kind dispatch, per-kind validation, the block-scoped array editor, the doc-gate
   warning and orphan note, `awf list`) stands unchanged, as do its invariants
   `cli-config-kinds` and `remove-block-scoped`: the rename changes command verbs, not
   config-array semantics. Per the partial-item-supersedence convention this ADR links
   the predecessor via `related: [24]` (not `supersedes:`) and does **not** flip its
   status: ADR-0024 stays `Implemented` and both ADRs remain live in `ACTIVE.md`.

3. **Fix the overloaded noun.** In the `enable`/`disable` help and summaries, name the
   toggled thing by its kinds ("a skill, agent, doc, domain, target, bootstrap, or
   hooks") or as "an artifact", never the generic "target." The `sync` and `list` uses
   of "target" denote adapter runtimes correctly and are left unchanged.

4. **Rename only the top-level dispatch handlers** `runAdd`→`runEnable` and
   `runRemove`→`runDisable` to match the commands. The deeper set-membership vocabulary
   (`ResolveAdd`/`ResolveRemove`, `PlanOp.Add`, `addRemoveTarget`, `addRemoveSingleton`)
   and the stable invariant slugs (`cli-config-kinds`, `remove-block-scoped`) accurately
   describe set/config operations and stay. A broader review of the CLI dispatch/resolve
   plumbing (and the duplicated gated-command prose noted below) is deferred to a
   separate effort.

5. **Update every live surface in the same change, editing prose *sources* (not
   rendered outputs) and re-rendering.** In `.awf/agents-doc.yaml`, every live invariant
   summary that names a toggle command (the gated-command list (Binary-version gate),
   the ADR-0050 reviewing-skill/agent pairing line (`awf remove agent`/`awf add skill`),
   and the ADR-0081 "Add applies the closure plan" and "Remove refuses dependents" lines)
   plus the `tooling` domain current-state (`.awf/domains/parts/tooling/current-state.md`),
   which holds several CLI references (the second gated-command copy, the ADR-0024
   `awf add/remove/list` line, the ADR-0081 dependency-graph line, "opt-in via `awf add`",
   "mirroring `awf remove`"). Also: the "Toggle an artifact" guide bullet and its
   `awf add target cursor` example (`.awf/parts/agents-doc/awf-setup.md`,
   `templates/agents-doc/AGENTS.md.tmpl`); the architecture doc part
   (`.awf/docs/parts/architecture/components.md`: the `cmd/awf/` entry-point command
   list and the ADR-0027 `list`/`add` dispatch line); the config-reference field
   descriptions (`internal/configspec`); the glossary "plan op" term
   (`.awf/docs/glossary.yaml`); the `working-with-awf` and `workflow` templates;
   hand-written `README.md`; the `config` domain current-state; and the stale
   `inv: target-cli` backing comment
   (`internal/project/target.go`), which backs ADR-0037's `target-cli` invariant:
   semantics unchanged, only the verb in the descriptive prose moves. These are all live
   rendered/hand-written surfaces; ADR-0037's, ADR-0039's, and ADR-0024's own frozen text
   is **not** edited: only the live `AGENTS.md`/docs lines move, the same "extend the
   live line, keep the frozen ADR text" rule ADR-0092 follows. A changelog entry is
   added. No `.awf/` schema or `migrate.Current()` change.

## Invariants

These are textual contracts; the machine-enforced behavioural invariants are ADR-0024's
`cli-config-kinds` and `remove-block-scoped`, which this rename leaves intact and does
not retire.

- The two config-toggle commands are named `enable` and `disable`; no `add`/`remove`
  command token survives in the CLI dispatch, help, or live docs.
- ADR-0024's `inv: cli-config-kinds` and `inv: remove-block-scoped` continue to hold:
  the rename changes command verbs only, not the kind-dispatch or block-scoped-removal
  semantics they guard.
- No help or doc text uses the generic noun "target" for the artifact a toggle command
  acts on; "target" denotes only an adapter runtime.

## Consequences

- The command verb now matches the operation (config enable/disable) and no longer
  collides with `awf new` (create) or the `target` kind; the CLI reads self-consistently
  with the states `awf list` already prints (`enabled`/`available`).
- **Breaking change:** `awf add`/`awf remove` stop working; adopters and scripts move to
  `awf enable`/`awf disable`. The rendered-guide propagation (Decision 1) makes the
  adopter-side migration automatic on the next `sync`.
- **Pinned-release lag:** an adopter (including `examples/sundial`) whose rendered docs
  are produced by a source-built or newer awf shows `enable`/`disable` while a pinned
  older release binary still only accepts `add`/`remove` until a release ships. Inherent
  to any pre-1.0 CLI change and the same lag ADR-0092 documents; accepted.
- **Coordination with ADR-0092:** both edit the single live binary-version-gate
  gated-command line and `working-with-awf`; whichever lands second re-edits that one
  line. No command-name collision (`context` ≠ `enable`/`disable`).
- **Noted smells, deferred:** the gated-command list is duplicated hand-maintained prose
  (no single source), and the volume of add/remove dispatch/resolve plumbing for two
  commands is heavier than the surface warrants. Dedup and a CLI streamline are out of
  this rename's scope, tracked as a separate effort.
- No `.awf/` schema change; `migrate.Current()` untouched, no `awf upgrade` owed.

Doc-currency obligations the implementing commit(s) must satisfy:

- Re-render all GENERATED docs via `./x sync` (`AGENTS.md`, `config-reference`,
  `glossary`, `architecture`, the `tooling`/`config` domain indexes, `working-with-awf`,
  `workflow`) and update hand-written `README.md`.
- Update the `tooling` and `config` domain current-state narratives' CLI references.
- Update tests carrying hard-coded `add`/`remove` tokens or `runAdd`/`runRemove` calls
  (`run_test.go`, `list_add_test.go`, the `new_test.go` comment); the help-parity tests
  iterate `argSpecs`/`commandOrder` dynamically and need no change.
- This is the no-plan direct-implementation case: the implementing commit flips the
  ADR to `Implemented` (`awf-adr-lifecycle`) and regenerates `docs/decisions/ACTIVE.md`
  via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep `add`/`remove`, reword docs only | Leaves the verb fighting the behaviour and the `awf new` collision intact; the names are the actual defect, not the surrounding prose. |
| Rename but keep `add`/`remove` as aliases | Two vocabularies for one operation is more to learn and document, not less; pre-1.0 and the rendered-guide propagation make a clean break cheap. |
| `on`/`off` or `use`/`drop` verbs | `on`/`off` reads oddly as a subcommand and `use`/`drop` are vaguer about the enabled-in-config meaning; `enable`/`disable` alone mirror the config `enable` arrays and the existing help verbs. |
| Full internal rename including the resolver | Ripples into resolver internals and their tests for no user-facing gain; `PlanOp.Add` and the `ResolveAdd`/`ResolveRemove` pair accurately describe set membership. |
