---
status: Proposed
date: 2026-07-10
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [config, docs, cli, discoverability, catalog]
related: [1, 29, 39, 45, 60, 61, 80, 82, 84, 86, 87]
domains: [config, rendering, tooling]
---
# ADR-0088: Adopter config reference: configspec authority, generated doc, and awf config command

## Context

An adopter editing `.awf/config.yaml` or a sidecar has no way to learn which keys exist,
what each does, or when a value is actually consumed. A survey of every surface confirmed
the fragmentation:

- The best key documentation is Go doc comments on `config.Config`/`Sidecar`
  (`internal/config/config.go`) and the audit knobs (`internal/audit`, `audit.Resolve`) —
  invisible at runtime, never published, unreadable from an adopter repo that installed a
  binary.
- The only structured surface is `awf init --describe`: JSON for the eight functional
  catalog var descriptors plus the two init multiselects (`catalog.Vars`,
  `internal/catalog/standard.go`) — nothing about the structural keys (`audit.*`,
  `invariants.*`, enable arrays, `docsDir`, `prefix`, bootstrap/hooks) or sidecar fields,
  and the flag is documented nowhere adopter-facing.
- `docs/working-with-awf.md` names key *groups* in one sentence and documents the glob
  dialect and sidecar placement rules in scattered prose; the `audit.*` knobs appear in no
  adopter doc at all. `awf list` shows enable state only.
- Consumption is already mechanically known but discarded: `render.ReferencedVars`/
  `ReferencedDataKeys`/`PlaceholderVarRefs` (`internal/render/vars.go`) drive the ADR-0086
  unused-var/data drift and the init prompt filter (`project.NeededVars`,
  `internal/project/scaffold.go`), yet no surface answers "which artifacts consume var X"
  or "what would enabling Y activate". An adopter learns a var is dormant only when a
  check fails.

The user constraint is explicit: "I want the adopters to get all the info they need, with
full descriptions of each thing they touch. I don't want them to guess." Primary audience:
AI agents working inside adopter repos (greppable docs first), humans second.

Forces and grounding discoveries shaping the design:

- **Var-description authority already exists and is pinned.** `catalog.VarDescriptor`
  entries are the single var authority; `var-descriptor-parity` (ADR-0029) keeps them
  bidirectionally matched to template references, and `var-descriptor-set-pinned`
  (ADR-0084) makes extending the set a successor-ADR act. A new descriptor system must
  compose with that lineage, not duplicate it.
- **Hash-checked docs cannot carry live config state.** `awf check` compares content
  hashes; `artifactConfigHash` folds only the vars an assembled source literally
  references plus opt-in channels (`internal/project/confighash.go`). A doc rendering
  every var's current value and consumer set depends on inputs its source never mentions —
  it would go stale with `check` green, breaking the drift oracle. The generated-index
  class (ACTIVE.md, domain docs) is checked by full regeneration instead
  (`regenDrift`, `internal/project/check.go`) and fits a computed document.
- **A bare `.vars` range is a trap.** `ReferencesBareVars` conservatively marks all vars
  consumed, which would neutralize the ADR-0086 unused-var check project-wide the moment a
  reference-doc template ranged over `.vars`. Live state must arrive as dedicated data
  keys (the agents-doc precedent: `data["docs"]`, `internal/project/render.go`).
- **Dormancy needs a full-catalog scan; the precedent is raw-source.** Only `NeededVars`
  reads templates outside the enabled set, via raw `templates.FS` bytes without
  `awf:include` expansion — currently correct for vars because no partial references
  `.vars` (verified; partials *do* reference per-key `.data` — the review-spine
  partials — so any include-blind scan must stay vars-scoped), but a scan built the
  same way inherits that latent blind spot the moment a partial gains a `.vars`
  reference.
- **Sidecar `data:` keys have defaults but no descriptions.** The catalog declares
  per-artifact data defaults (`SkillSpec.Data` etc., `internal/catalog/catalog.go`);
  no description text exists anywhere. "Each thing they touch" includes these.
- **Descriptions ship into adopter repos.** The ADR-0082 residue scan walks only the
  embedded templates FS (`internal/project/residue_scan_test.go`); configspec description
  strings are Go source rendered into adopter docs, where a concrete `ADR-0077` citation
  or repo-identity literal would be meaningless residue the existing scan cannot see.
- **Var value states are three-way under ADR-0087.** Present-with-value (set),
  present-empty (open to-do), absent (declined). A reference surface reporting var state
  must speak that vocabulary, not a boolean set/unset.
- **`config.Load` refuses outside an adopted tree** ("not an awf project (run `awf
  init`)", `internal/config/config.go`), but pre-adoption discovery — an agent evaluating
  awf before init — is a real audience for a static reference.
- **`awf describe` would collide softly** with `awf init --describe` (different output
  contract); the user chose `awf config` as the command name.
- Deliberately out of scope, recorded as backlog: a man page (`argSpecs` in
  `cmd/awf/main.go` is already structured summary+help data; delivery would be bootstrap
  installing `share/man` and/or a roff-emitting subcommand) and a JSON Schema for editor
  validation. Both are additional projections of the same authority this ADR creates.

## Decision

1. **New leaf package `internal/configspec` — the adopter-facing description authority.**
   A compile-time Go value (no embedded YAML — the `catalog.Standard` pattern, ADR-0060)
   holding one entry per adopter-touchable surface, in three groups:
   - **Config keys:** every adopter-writable leaf of `config.Config` (including all
     `audit.*` and `invariants.*` knobs, `bootstrap.enabled`, `hooks.enabled`, `docsDir`,
     `prefix`, the enable arrays, `vars`) — dotted YAML path, value type, default (as
     rendered prose), full adopter-voiced description, and an availability clause stating
     when the key has effect.
   - **Sidecar fields:** the four fields (`data`, `sections`, `local`, `paths`) with their
     per-kind placement rules (ADR-0086 Decision 5) stated as availability. The walk
     recurses into map-of-struct values, so `sections.<name>.drop` is its own described
     leaf — adopters write it, so parity covers it.
   - **Per-artifact data keys:** one description per adopter-settable sidecar `data:` key,
     keyed by artifact. The key universe is what the artifact's include-expanded template
     actually references (`.data.K`), of which the catalog-declared defaults are a subset —
     an optional key with no default (the agents-doc's `invariants`, a local base template's
     `description`) is exactly as touchable as a defaulted one. The injected domain-doc keys
     (`domain`, `decisions`) are exempt: domain sidecars are paths-only (ADR-0086), so no
     adopter can set them. Descriptions live in configspec; values stay in the catalog.
   Var descriptions are **derived from `catalog.Vars` at construction** — the catalog
   remains the sole var authority (ADR-0029 parity and ADR-0084 pinning untouched);
   configspec adds no second var-description home. A derived var entry carries the
   `catalog.VarDescriptor` description text verbatim; the only content configspec may
   attach to it is the availability clause (structural metadata, per key). `awf init
   --describe` output is behaviorally unchanged.
2. **Parity is machine-enforced in both directions.** A reflection walk over
   `config.Config` and `Sidecar` yaml tags (skipping unexported fields; treating the
   free-form `Vars`/`Data` maps as namespaces, not leaves; recursing into map-of-struct
   values such as `sections`) fails when a key lacks a Spec entry or an
   entry names a dead key; a second check matches data-key description entries one-to-one
   against the catalog's declared data keys. Adding a config field or a catalog data key
   without describing it fails the gate.
3. **A new always-on singleton doc, `docs/config-reference.md`, renders the full
   reference per-project.** One `DocEntry` with a static path (ADR-0060's "a single
   entry"). Content is
   project-aware, rendered from configspec plus the live tree: each config key with its
   current effective value; each var with its three-way ADR-0087 state, its consumers
   (enabled artifacts whose assembled sources or `gateCmd`/`checkCmd` part placeholders
   reference it — the ADR-0086 Decision 3 union), and, when dormant, which catalog
   artifacts would consume it if enabled; each artifact's data keys with defaults,
   overrides, and descriptions. All computed state reaches the template as dedicated data
   keys — the template never ranges `.vars` or `.data` bare. Each generated section
   degrades to a coherent "none configured" line when its data is empty (a repo with no
   vars set, no domains, no data overrides renders prose, never an empty table skeleton —
   ADR-0001/0045), pinned by the doc's hand-added golden. The doc is drift-checked by
   **full regeneration** (the generated-index class), not content hashing. Prose framing
   sections remain convention-part overridable; the generated reference tables are not.
   The config-reference sidecar is accordingly sections/local-only: a non-empty `data:`
   on it fails every gated command at project open (the ADR-0086 Decision 5 pattern) —
   the reference's data namespace is injected at generation, so authored `data:` would be
   silently overwritten while the injected key names make it look consumed, defeating the
   unused-data check.
4. **A new CLI command, `awf config [<key-or-var>]`.** Bare, it prints the same reference
   with live state; with an argument, the one matching entry (description, current value,
   consumers or dormancy hint). Inside an adopted tree it is a gated command
   (ADR-0039 binary-version gate at open, corrupt-lock refusal per ADR-0076); outside one
   it degrades to the **static catalog-wide reference** — descriptions, defaults, and
   availability only, no live state — serving pre-adoption discovery instead of erroring.
   An unknown argument is an exit-1 error naming the token and pointing at the bare form.
   The gated-command enumeration in the rendered agent guide, the working-with-awf
   command list, `docs/architecture.md` (the new package and command), and the config
   domain's current-state part update in the same change. The implementing change flips
   this ADR's status and regenerates `docs/decisions/ACTIVE.md` via `./x sync` in its
   final commit.
5. **One shared describe-model builder in `internal/project`** computes entries, live
   values, consumers, and dormancy once, feeding both the doc renderer and the command.
   Enabled-set consumption reuses the ADR-0086 union (assembled sources across targets,
   domain docs, part placeholders); potential consumers come from a raw-template scan over
   the full catalog (the `NeededVars` precedent), with a guard test asserting no
   `templates/partials/` file references `.vars` so the raw-source shortcut stays sound.
   The guard is deliberately vars-scoped: the review-spine partials legitimately reference
   per-key `.data` today, and the raw-catalog scan exists solely for var dormancy —
   data-key consumption is catalog-declared per artifact and checked on include-expanded
   assembled sources (ADR-0086).
6. **The residue discipline extends to descriptions.** Every configspec description
   string (including the availability clauses attached to derived var entries and the
   data-key descriptions) is free of concrete `ADR-` citations and repo-identity literals, enforced
   by a test alongside the ADR-0082 scan, which keeps its templates-FS scope.

## Invariants

- `inv: configspec-key-parity` — every adopter-writable `config.Config`/`Sidecar` leaf
  key has exactly one configspec entry with a non-empty description, and every config-key
  entry names a live field, enforced by a reflection walk over the yaml tags.
- `inv: configspec-data-parity` — configspec's per-artifact data-key descriptions match,
  one-to-one in both directions, the data keys each catalog artifact's include-expanded
  template references (union its catalog-declared defaults), with the injected keys
  exempt: the domain-doc pair and the config reference's own injected collections
  (neither is adopter-settable — domain sidecars are paths-only, the config-reference
  sidecar rejects `data:`).
- `inv: configspec-var-derivation` — configspec's var entries are derived from
  `catalog.Vars` and cover exactly that set, carrying the descriptor description text
  verbatim; configspec attaches only availability clauses — no second var-description
  authority exists.
- `inv: config-reference-regen-drift` — `docs/config-reference.md` is drift-checked by
  full regeneration, never by content hash alone.
- `inv: config-command-static-fallback` — `awf config` outside an adopted tree prints the
  static catalog reference and exits zero; inside one it runs gated at open.
- `inv: configspec-description-residue` — no configspec description string carries a
  concrete `ADR-` citation or a repo-identity literal.
- `inv: config-reference-no-bare-vars` — the reference-doc template's assembled source
  contains no bare `.vars` or `.data` reference (all computed state arrives via dedicated
  data keys), so it never widens ADR-0086's conservative consumption escapes.
- `inv: config-reference-data-rejected` — a non-empty `data:` block on the
  config-reference sidecar fails every gated command at project open.
- Textual: descriptions are adopter-voiced — they explain effect and availability in the
  adopter's terms, and availability claims match the real consumption channels (only
  `gateCmd`/`checkCmd` flow through part placeholders).

## Consequences

- Adopters stop guessing: every key, var, sidecar field, and data key they can touch has
  a full description reachable by grepping `docs/config-reference.md` or running
  `awf config`, including the previously invisible dormancy answer ("no enabled artifact
  references this; enabling X would"). The `audit.*` knobs become documented for the
  first time outside Go source.
- Every adopter repo (fleet, go-php) gains `docs/config-reference.md` at its next sync —
  confirmed desired. The doc's content changes whenever config changes; regeneration
  checking makes that drift visible instead of silent.
- Maintenance cost is the point: a new config field, catalog var, or data key cannot land
  without adopter-voiced prose, enforced by the gate. Descriptions become part of the
  public surface and must be written for publication (no ADR citations — rationale stays
  in ADRs; the reference states behavior).
- The describe model adds a second full-catalog raw-template scan; the partials guard
  test converts its latent include-expansion blind spot (shared with `NeededVars`) from
  silent to failing.
- `awf config` joins the gated set — the AGENTS.md invariant line enumerating gated
  commands, the ADR-0039 lineage prose, and awf's own `.awf` parts update in the same
  change. Accepted friction: inside an adopted tree a version-behind binary refuses
  `awf config` like every gated command, making the discovery tool unavailable exactly
  mid-upgrade — consistent with ADR-0039, mitigated by the refusal's upgrade hint, and
  the ungated pre-adoption static output is unaffected. The name occupies a plausible
  future subcommand slot (config *editing*); if editing porcelain ever arrives it
  extends this command rather than replacing it.
- Upper end of one effort: configspec + parity, the doc, the command, docs travel. The
  CLI command is the most separable slice if trimming proves necessary. Backlog recorded:
  man page and JSON Schema as further projections; folding `awf init --describe` into
  `awf config` as a possible successor.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| `go:generate` AST extraction of the existing Go doc comments | Comments would serve two audiences with one voice; adds the repo's first generate step; availability clauses need a side table anyway — converges on the hand-authored table with extra machinery. |
| Description struct tags on config fields | Multi-sentence descriptions and availability clauses do not fit tag strings; collapses under the "full descriptions" requirement. |
| Static catalog-wide doc (same content in every repo) | Discards the consumption graph the user explicitly wants surfaced; live state only via CLI would leave the greppable surface incomplete for the primary (agent) audience. |
| Hash-channel fold (a new `References*` channel covering the whole effective config) | Enumerating every config input into the hash re-encodes the config shape a second time; regeneration checking already exists for exactly this generated-index class. |
| Toggleable doc instead of always-on singleton | The reference must never be absent from an adopter repo — an undiscoverable reference re-creates the guessing problem; user chose always-on. |
| Prefix-derived filename (`docs/<prefix>-config-reference.md`) | Breaks the static-path DocEntry model in three projections for no benefit once the doc is understood as repo config state; user reversed the initial prefix request. |
| Command name `awf describe` | Soft collision with `awf init --describe` (different output contract); `awf config` reads naturally for single-key queries. |
| Ship the JSON Schema and man page in the same effort | Both are pure projections of configspec; deferring them trims scope without foreclosing anything. Recorded as backlog. |
| Error outside an adopted tree (adopted-repos-only command) | Pre-adoption discovery is a real audience (an agent evaluating awf before init); the static fallback costs one branch. |
