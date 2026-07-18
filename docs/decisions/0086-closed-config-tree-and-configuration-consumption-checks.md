---
status: Implemented
date: 2026-07-10
tags: [closed-config-tree, sidecar-fields]
related: [11, 22, 45, 68, 69, 70, 76, 77, 81, 83, 88]
domains: [config, rendering]
---
# ADR-0086: Closed config tree and configuration-consumption checks

## Context

`.awf/` residue is invisible today. The strict decoder rejects unknown YAML keys
(`KnownFields(true)`, `internal/config/config.go`) and the orphan sweep flags wrong-*name*
sidecars and parts (`orphans()`, `internal/project/check.go`; ADR-0011), but four hygiene gaps
remain, each verified against source:

- **Stray files.** `orphans()` scans only `*.yaml` in kind directories and `*.md` in parts
  trees. A non-yaml file in `.awf/skills/`, a non-md file in a parts directory, or an unknown
  top-level entry (`.awf/notes.md`, `.awf/scratch/`) is never flagged, never rendered, never
  pruned; an adopter can reasonably conclude the file *does* something.
- **Unused vars.** A `vars:` key referenced by no rendered artifact is silently ignored. The
  existing advisory covers only the inverse direction: referenced-but-unset (ADR-0045).
- **Unused sidecar data.** `data:` is `map[string]any` rendered under `missingkey=zero`;
  publication-safe degradation (ADR-0001/0045) means a typo'd data key produces coherent
  generic prose with no signal that the override never took effect: the degradation
  philosophy's one genuinely harmful side effect.
- **Inert fields.** `paths:` on a non-domain sidecar parses and does nothing (it is read only
  by the audit's domain loop, `internal/project/project.go`; ADR-0077). Symmetrically (found
  during design grounding), a *domain* sidecar's `data:` and `sections:` are wholly inert:
  domain rendering passes an empty sidecar and overwrites the data map with
  `{domain, decisions}` (`generateDomainDocs`, `internal/project/render.go`), while the domain
  template itself references `.data.domain`, so a naive consumption check would wrongly bless
  exactly this smell.

The severity question was settled by the user: these are "clear signals of smell ... The configs
and directories should be clean, so adopters don't e.g. assume they can put a file there when
it's not intended, or possibly even thinking something does something when it doesn't."
Errors, not advisories. This *sharpens* rather than contradicts the advisory precedents:
ADR-0070/0083 cover **unauthored** content (stub defaults, marker residue: possibly
deliberate, never failing); this ADR covers **authored-but-unconsumed** configuration, which
is always a defect or dead weight.

Forces shaping the design:

- Consumption is mechanically extractable. Templates read `{{ .vars.X }}` and `{{ .data.K }}`
  from the dot-map (`internal/project/render.go` `data()`); `render.ReferencedVars` already
  regex-extracts var references from assembled sources. A sweep of the embedded templates FS
  confirmed no template uses a bare `.data`/`.vars`, `range`/`with`/`index` form that would
  defeat a `ReferencedDataKeys` sibling.
- Convention parts are raw, never templated (ADR-0034), except the closed `{{=awf:key}}`
  placeholder registry, and only the `gateCmd`/`checkCmd` registry keys reach into `vars:`
  (`internal/project/placeholders.go`). Part placeholders are therefore a second, bounded
  consumption channel. Domain docs render *outside* `RenderAll` and drop their assembled
  source, but their parts do pass `substitutePlaceholders`; a var-consumption union computed
  over `RenderAll` output alone would raise false unused-var errors.
- `awf init` deliberately seeds `vars:` with every catalog template's referenced vars (including
  artifacts outside the scaffolded enable set), so a later `awf add` renders
  cleanly (ADR-0022, `invariant: scaffold-seeds-all-vars`). A naive "set-but-unreferenced"
  definition would make a fresh init immediately red.
- Sync itself writes collision backups *inside* `.awf/` when adopting over pre-existing
  files (`BackupFile` → `<path>.awf-bak`, `internal/project/install.go`); a closed-tree sweep
  with no backup policy would fail on a file awf created one command earlier.
- `orphans()` is check-only (single caller: `Project.Check`); sync and the porcelain
  commands never act on orphan drift, which is what keeps `awf remove`/`awf sync` usable to
  *repair* flagged state. The new checks must preserve that property.
- Dogfood is already clean: awf's own `.awf/` tree and the `internal/evals` full-catalog
  fixture were enumerated against the proposed model during design: no unused vars, no
  unused data keys, no strays, no inert fields. The checks land green here.
- External adopters will see new hard failures on upgrade; every new error
  must therefore name the exact file or key and the edit that fixes it.

## Decision

1. **The `.awf/` tree is closed.** One allowlist sweep (subsuming `orphans()`) walks the
   entire `.awf/` directory and classifies every entry against a *claimed-path model*
   computed from config, catalog, and the render-unit set: the skeleton (`config.yaml`,
   `awf.lock`); the enabled render units (`bootstrap.sh` + `upgrade.sh` when the bootstrap
   singleton is enabled, `hooks/*` when hooks is enabled, `memory/.gitignore` always);
   sidecars `<kind>/<name>.yaml` for enabled artifacts and the singleton sidecars; parts
   `<kind>/parts/<name>/<section>.md` for enabled artifacts' declared sections (declared
   sections resolve against the *effective* catalog, so synthesized local artifacts'
   `content` sections are claimed, ADR-0068) and `parts/<kind>/<section>.md` for singletons.
   An artifact whose sidecar sets `local: true` renders nothing, so its parts are *not*
   claimed and report drift: the parts mirror of Decision 4's local data-key rule.
   Domains are exempt from that sentence: domain rendering never reads the domain
   sidecar (`generateDomainDocs` passes an empty one), so `local:` cannot stop it and
   domain parts stay claimed; and a domain sidecar carrying `local: true` at all is
   rejected at open (Decision 5), so the sweep never sees one.
   `memory/**` is wholly exempt (ADR-0069 session scratch). Every other entry is failing
   drift from `awf check`, `Kind: "orphaned"`. The pre-existing orphan cases keep their
   detail strings byte-identical; genuinely new cases report an `unclaimed` detail; reporting
   collapses to the highest fully-unclaimed directory, as the parts sweep does today.
   ADR-0011's backing slugs (`drift-source-set`, `section-orphan-flagged`) move with the
   rewritten sweep in the same commit: the behaviors survive; only the code moves.
2. **Backup files are flagged, not exempt.** A sync-written collision backup under `.awf/`
   (both forms awf writes, `<path>.awf-bak` and the numbered `<path>.awf-bak.<N>`
   (`freeBackupPath`, `internal/project/install.go`)) reports drift with a distinct
   self-describing detail ("stale awf-bak backup: review and delete"). The backup is a
   to-do, not a resident; silent exemption would let the exact residue this ADR targets
   accumulate invisibly.
3. **Unused vars are drift.** A `vars:` key whose value is **non-empty** and that is
   referenced by no rendered artifact (neither a `.vars.X` reference in any assembled
   template source (all targets, `RenderAll` output *and* the generated domain docs, which
   render outside it) nor, for `gateCmd`/`checkCmd`, a
   `{{=awf:gateCmd}}`/`{{=awf:checkCmd}}` placeholder in any consumed convention part
   *including domain-doc parts*) is failing drift reported by `awf check` against
   `.awf/config.yaml`. Empty-valued keys are exempt: they mirror ADR-0045's unset definition
   (`nil`/`""`), so ADR-0022's seed-all-vars scaffold stays legal unchanged. A bare `.vars`
   reference without a key selector conservatively marks all vars consumed.
4. **Unused sidecar data keys are drift.** Per artifact, a sidecar `data:` key with no
   `.data.K` reference in that artifact's assembled sources, unioned across its enabled
   targets, is failing drift reported against the sidecar file. Nested access `.data.a.b`
   claims top-level key `a`; a bare `.data` reference marks all keys consumed. A key
   referenced only inside a section the sidecar `drop`s counts as unused (the key genuinely
   does nothing), and the error hints at the drop. A `local: true` sidecar renders nothing,
   so all its data keys report unused, deliberate: such a sidecar legitimately carries only
   the flag.
5. **Inert sidecar fields are rejected at project open.** A non-domain sidecar with
   non-empty `paths:`, and a domain sidecar carrying anything but `paths:` (non-empty
   `data:` or `sections:`, or `local: true`: a domain sidecar is paths-only; no code
   path reads any other field on it), fail every
   gated command at project open with a message naming the file and the fix, placed
   *before* the `local:` skip in the validation loop so `local: true` sidecars cannot carry
   inert fields either. This matches the corrupt-lock and closure-violation precedents
   (ADR-0076, ADR-0081): the repair is a file edit, not an awf command, so open-time refusal
   costs nothing. No struct split, no migration: the error is the repair instruction.
6. **Interactive init prompts only for consumed vars.** The interactive scaffold path
   prompts only for vars referenced by the chosen enabled set; the seeded skeleton still
   carries the full catalog var union as empty keys (ADR-0022 intact). This closes the one
   remaining init-goes-red path: a user-typed value for a var whose only referencing
   artifact was trimmed away.
7. **Check-only placement.** Decisions 1-4 report through `Project.Check` exactly like
   today's orphan drift (never through sync, add, or remove), so awf's own commands remain
   usable to repair every state they flag.

## Invariants

- `invariant: closed-config-tree`: every filesystem entry under `.awf/` outside the claimed-path
  model of Decision 1 (with `memory/**` exempt) is reported as failing drift by `awf check`.
- `invariant: awf-bak-flagged`: a `*.awf-bak` or `*.awf-bak.<N>` file under `.awf/` (outside
  `memory/**`) reports drift with the stale-backup detail, never passes silently.
- `invariant: unused-var-drift`: a non-empty `vars:` key referenced by no assembled template
  source and no `gateCmd`/`checkCmd` part placeholder (domain-doc parts included) is failing
  drift; empty-valued keys never are.
- `invariant: unused-data-drift`: a sidecar `data:` key unreferenced by its artifact's assembled
  sources across all enabled targets is failing drift keyed to the sidecar path.
- `invariant: inert-sidecar-field-rejected`: non-empty `paths:` on a non-domain sidecar
  (regardless of its `local:` flag) and any non-`paths:` field on a domain sidecar
  (non-empty `data:`/`sections:`, or `local: true`) fail every gated command at
  project open.
- `invariant: init-prompts-enabled-vars`: interactive `awf init` prompts only for vars referenced
  by the scaffolded enabled set's templates.
- Textual: the advisory tier is untouched; ADR-0070 stub notes and ADR-0083 marker notes
  remain non-failing; ADR-0045's unset-var advisory remains the *unset* direction's contract.
- Textual: existing orphan drift detail strings are preserved byte-identical through the
  sweep rewrite.

## Consequences

- Adopters get a hard guarantee: a file under `.awf/` either does something or `awf check`
  says it doesn't; a configuration value either feeds rendering or is flagged. The
  "publication-safe degradation hides typos" hole closes without weakening degradation
  itself (unset-and-referenced stays legal; set-and-unreferenced becomes the error).
- **Upgrade friction, accepted:** external adopters may go red on their first post-upgrade
  `awf check`. Every error self-describes its repair; the changelog entry flags the
  strictness change. Toggling a render unit off (`awf remove hooks`) can newly strand a var
  (`commitGateCmd`) and require a same-commit vars edit, intended, documented. Likewise a
  brownfield adopt: init/sync writing `.awf-bak` backups over pre-existing files makes the
  very next `awf check` red until the backups are reviewed and deleted, intended to-do
  surfacing, not a bug.
- Disabling an artifact now surfaces *all* its residue (sidecar, parts, data keys, vars it
  alone consumed) as errors: the enabled-set-relative semantics orphaned parts already had,
  extended uniformly.
- The claimed-path model is a second place that must learn about any *new* kind of
  config-tree resident. Mitigated structurally: it derives from config + catalog + the
  render-unit set rather than a hand list, so a new render unit added to `RenderAll` joins
  the model through the same code path that writes it.
- `ReferencedDataKeys` freezes the template dialect's data-access forms: a future template
  using `index .data "k"` or ranging over `.data` would silently widen the conservative
  escape. The catalog-derived template sweep (ADR-0080) is the natural home for a guard if
  that ever changes.
- Downstream work: rewrite of `orphans()` into the sweep, two new drift producers, two
  open-time validations, an init prompting change, per-producer fixture tests, dogfood
  proofs (evals fixture + awf's own tree), sequenced by the implementation plan (small
  standalone rejections first, then consumption checks, then the sweep). Docs travel with
  their commits: the rendered agent guide's Invariants entries for the six new slugs (via
  `.awf` parts + sync), `docs/working-with-awf.md`'s drift-category and repair guidance,
  the changelog strictness entry, and the `./x sync` ACTIVE.md regeneration in the
  status-flip commit.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Advisory notes (ADR-0070 tier) for all four gaps | Rejected by the severity decision: unconsumed configuration is a defect signal, and an advisory invites the exact "assume it does something" failure mode this exists to kill. |
| Additive stray-file walk beside `orphans()` | Two overlapping sweeps encode the claimed-set twice; a future render unit added to one but not the other silently reopens gap 1: the failure mode this ADR exists to close. |
| Parse-time `paths:` rejection via per-kind sidecar structs + schema-9 migration stripping inert keys | Duplicates the sidecar shape for one field; silent migration-time deletion of adopter-written config is worse UX than a loud error naming the exact edit. |
| Silently exempt `*.awf-bak` from the sweep | Stale backups accumulate invisibly: the targeted smell. A distinct drift detail keeps them visible and self-repairing. |
| Seed only the enabled set's vars at init | Weakens ADR-0022's "later `awf add` renders cleanly" property; the non-empty-and-unreferenced definition preserves it at zero cost. |
| Treat domain sidecar `data:`/`sections:` as all-unused drift | Asymmetric with the `paths:` handling for an identical smell (inert field on the wrong kind); open-time rejection is louder and simpler than special-casing domains inside the consumption check. |
| Have `awf sync` prune or repair unclaimed entries automatically | Acting on adopter-authored files is destructive; check-only placement (Decision 7) is what keeps awf's own commands usable to repair every state they flag. |
