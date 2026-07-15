---
status: Implemented
date: 2026-06-27
supersedes: []
superseded_by: ""
tags: [cli-dispatch, kind-descriptor]
related: [9, 14, 22, 93]
domains: [tooling, config]
---
# ADR-0024: CLI Config Management Across Kinds

## Context

`awf add <skill>` and `awf list` (cmd/awf/list_add.go) manage only the `skills:` enable array: `add`
appends a bare skill name and re-syncs, `list` shows catalog skills with their per-project state.
The `.awf/config.yaml` schema has five enable arrays, though: `skills`, `agents`, `docs`, `hooks`,
`domains` (ADR-0009); and ADR-0022 made opt-in a first-class flow by scaffolding only a curated
core, so docs in particular are now opted in as routinely as skills. Today docs, agents, hooks, and
domains can only be toggled by hand-editing the config; the CLI covers one array of five, and there
is no `remove`.

A bare `add <name>` cannot be generalised as-is: a name can exist under more than one kind (the
catalog ships both a `debugging` skill and a `debugging` doc), so a kind is required to disambiguate.
Four of the five arrays are catalog-backed (the name must be a catalog entry); `domains` is
freeform: the adopter invents domain names and authors each domain's current-state narrative
(ADR-0014 validates the names), so `add domain` *creates* rather than *enables*.

## Decision

1. **Generalise to `awf add <kind> <name>` and `awf remove <kind> <name>`,** with `<kind>` a required
   singular token mapping to its config array: `skill`→`skills`, `agent`→`agents`, `doc`→`docs`,
   `hook`→`hooks`, `domain`→`domains`. The bare `awf add <name>` form is removed (a breaking change;
   pre-1.0, and the kind is needed to resolve cross-kind name collisions). To ease the migration, an
   `add` invocation with a single argument fails with a targeted message (kind is now required, use
   `awf add <kind> <name>`) rather than the generic usage error.

2. **Validate by kind.** For the catalog-backed kinds the name must be a catalog entry (the
   `catalog.Skills`/`Agents`/`Docs` maps, the `Hooks` slice). For `domain` the name is validated for
   path-safety through the same `config` rule that backs ADR-0014's `inv: domain-name-validated`, with
   no catalog check: `add domain` creates a new freeform domain. That rule is inline in
   `config.Validate` today; this ADR extracts it into an exported `config` helper (e.g.
   `ValidateDomainName`) that both `config.Validate` and the CLI `add domain` path call **before any
   write**, so an invalid name is rejected up front rather than after a partial config write. Because
   `config.Validate` still applies the check (now via the helper), ADR-0014's `inv:
   domain-name-validated` backing (asserted through `config.Validate`) stays intact. `add` errors if
   the target is already enabled; `remove` errors if it is not.

3. **Edit one array generically, re-sync.** A single config-array editor handles every kind and every
   array form it may encounter: the key present with items (the only shape `ScaffoldConfig` emits via
   `writeArray`), the empty `key: []` form and a bare `key:` (which a hand-edit, or removing the last
   item, can leave), and (for `domains`, which `ScaffoldConfig` does not emit) the key absent
   entirely (it is appended). Removal is scoped to the named key's block so a name shared across kinds (`debugging`)
   is removed from the right array only; removing the last item leaves a bare `key:` (a valid empty
   array). Both commands re-render via the normal sync, so `remove` drops the now-unproduced rendered
   file through the existing Sync prune.

4. **Warn, don't silently mislead.** `add skill <name>` where the skill's `requiresDoc` is not an
   enabled doc prints a warning that the skill stays suppressed until that doc is enabled (it would
   otherwise render nothing, per `inv: doc-gated-skill-suppressed`). `remove` prints a note when the
   removed target still has a sidecar or convention parts on disk; they are now orphaned (what `awf
   check` reports) and are left in place rather than deleted, so a re-`add` restores the customisation.

5. **`awf list [<kind>]` covers every kind.** Without an argument it groups all kinds; with a kind it
   shows just that one. Catalog-backed kinds show `enabled`/`available` (and the existing
   `local`/`tuned` states for sidecar-carrying kinds); `domains`, having no catalog pool, lists the
   configured set only.

6. **Reflect the surface in help and docs.** The `help` text, the README command table and adoption
   section, and the agent guide's "Working with awf" guidance are updated to the `add`/`remove`/`list`
   `<kind>` grammar in the same change.

## Invariants

- `invariant: cli-config-kinds`: `awf add`/`remove` operate on exactly the five config enable arrays via a
  required `<kind>` token, validating catalog-backed kinds against the catalog and `domain` names
  through the `config` path-safety rule.
- `invariant: remove-block-scoped`: `awf remove <kind> <name>` deletes `<name>` only from the named kind's
  array block, leaving a same-named entry under any other kind untouched.

## Consequences

- Opting any catalog target in or out, and creating or dropping a domain, is a single command with
  consistent grammar; the config arrays are no longer a skills-only CLI surface.
- `awf add tdd` (bare) stops working; users and any scripts move to `awf add skill tdd`. Documented in
  the same change. No `.awf/` schema change: the arrays already exist (ADR-0009); `migrate.Current()`
  is untouched and no `awf upgrade` is owed.
- The generic array editor replaces `appendSkill`; its block-scoped removal and key-absent handling
  need direct test coverage (add/remove for each kind, the empty/bare/absent array forms, a removal
  scoped past a shared name, the doc-gate warning, and the orphan note) to clear the 100% gate.
- `remove` leaving sidecars/parts behind means a removed-then-re-added target keeps its customisation,
  at the cost of a transient orphan that `awf check` surfaces until the adopter re-adds or deletes it.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` and `config` domain narratives gain the generalised CLI management.
- `help` text, README, and the agent guide reflect the new grammar.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Drop `add`/`list`, make config edit-only | Loses CLI discoverability/convenience exactly when ADR-0022 made opt-in routine; the tool's job is managing this config. |
| Keep `add` bare, infer kind, `--kind` to disambiguate | Reintroduces the collision logic the required kind removes; less self-documenting than an explicit token. |
| Manage only `skills`+`docs` | Leaves agents/hooks/domains as a separate hand-edit path; a uniform five-kind surface is simpler to learn and document. |
| Round-trip the YAML through a parser | Loses the scaffold's hand-authored formatting/comments; the string editor that ADR-0009-era code established is kept and generalised. |
