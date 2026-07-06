---
status: Implemented
date: 2026-07-06
supersedes: []
retires_invariants: []
superseded_by: ""
tags: [catalog, rendering, cli, adopter-extensibility]
related: [22, 37, 45, 50, 53, 54, 57, 58, 60, 61]
domains: []
---
# ADR-0068: Project-local skills and agents

## Context

awf's whole purpose is authoring Claude-Code skills and review agents, yet an
adopter cannot add one. The catalog of skills/agents is the compile-time Go value
`catalog.Standard` (`internal/catalog/standard.go`), baked into the binary
(ADR-0060, ADR-0061); skill/agent templates live in the embedded `templates/` FS.
A released `awf` binary in an adopter's repo therefore has no way to grow a new
skill: it cannot edit its own Go source, and it cannot add to its own embedded
templates. Adopters can only *trim* the standard set (`awf add/remove`), never
extend it. `awf new` compounds the gap — it scaffolds only ADRs
(`cmd/awf/new.go`, guard `if kind != "adr"`).

Two facts discovered while grounding this decision shape the design:

- **The existing `local: true` sidecar flag already admits non-Standard names.**
  A `local: true` artifact is skipped by render (`internal/project/render.go`) and
  *exempted from the catalog-pool check* by validation
  (`internal/project/validate.go`, `checkKindAgainstCatalog`). So an adopter can
  already list an arbitrary name in `skills:`/`agents:` and hand-author the final
  file. The only missing capability is having awf **render** such a name from a
  template — which is exactly what makes rendering worthwhile: `{{ .prefix }}`
  naming, `{{=awf:key}}` placeholder substitution (ADR-0057/0058), and
  **once-per-target output** (ADR-0037). A skill defined once would then be emitted
  for every enabled harness (Claude Code, Cursor, …), making an adopter's workflow
  agent-harness-agnostic. That last property is the load-bearing motivation.

- **The catalog value is a shared package global and skill/agent template ids are
  name-derived.** `project.Open` sets the project's catalog to `catalog.Standard`
  directly, and the template id for a skill/agent is computed by convention
  (`skills/<name>/SKILL.md.tmpl`), never read from a catalog field — only
  `DocEntry` carries a `TID` (ADR-0061). Rendering many local artifacts from one
  shared template therefore requires (a) a per-project *clone* of the catalog so
  synthesized entries never corrupt the global, and (b) a template-id resolution
  hook so a local entry can point at the shared base template.

The central tension is with ADR-0060/0061: "the standard catalog has exactly one
authoritative representation — the compile-time Go value; no embedded
`catalog.yaml`, no runtime catalog parse" (`inv: catalog-go-single-source`). Read
against its backing test, that invariant governs the **source of the standard
set** — it forbids re-introducing a runtime YAML parse of the standard and asserts
`catalog.Standard` is populated. It does not assert `project.catalog ==
catalog.Standard`. A per-project layer that *augments a clone* of the standard with
project-defined entries is a distinct concern, and this ADR treats it as such.

## Decision

1. **A local artifact is an enabled skill/agent whose name is not in
   `catalog.Standard`.** It is declared by the presence of a sidecar
   (`.awf/skills/<name>.yaml` or `.awf/agents/<name>.yaml`). A non-Standard,
   non-`local:true` enabled name **without** a sidecar remains a hard "unknown
   skill/agent" error, so typos still fail. A local name **may not equal** a
   Standard name (no shadowing).

2. **Local artifacts render from one awf-owned base template per kind.** awf ships
   exactly two base templates — one for skills, one for agents — each carrying
   frontmatter and a single `content` section. Every local artifact renders from
   the base template of its kind; **adopters never author a template**, only
   content. The base templates are embedded through explicit `//go:embed` entries
   (the bare directory-walk form silently skips a reserved, underscore-prefixed
   name).

3. **The effective catalog is a per-project clone of `catalog.Standard` plus
   synthesized local entries.** `project.Open` clones the Standard
   `Skills`/`Agents` maps into a fresh `Catalog` (a shallow `maps.Clone` suffices —
   synthesis is insert-only and never mutates an existing spec) and inserts, for each local name,
   a synthesized `SkillSpec`/`TargetSpec` with `Sections: ["content"]` marked to
   resolve to the base template. `catalog.Standard` is **never mutated**. Every
   catalog-ranging path — render, validate, list, add/remove — inherits locals
   with no per-site change beyond item 4.

4. **`SkillSpec`/`TargetSpec` gain a base-template hook.** A field on the spec
   (a `Base` flag, or a `TID` override) makes template-id resolution return the
   shared base template for a marked local entry instead of the name-derived path.
   Standard entries leave it unset; their rendered output and drift hashes stay
   byte-identical.

5. **The author surface is the `content` part plus the sidecar.** A local
   artifact's body is the convention part
   `.awf/skills/parts/<name>/content.md` (agents: `.awf/agents/parts/<name>/…`),
   spliced verbatim with `{{=awf:key}}` placeholder substitution — never
   Go-template-executed (ADR-0057/0058, `inv: parts-raw`). The base template
   sources the artifact's `name` (`{{ .prefix }}-<slug>` for skills; an unprefixed
   `<slug>` for agents, matching existing awf agent naming) and `description` from
   synthesized/sidecar `data`, each guarded so an unset value degrades to
   publication-safe generic prose, never `<no value>` (ADR-0045).

6. **`local: true` is unchanged and introduces no new flag.** The existing flag
   keeps its meaning — opt out of rendering entirely for a fully hand-authored
   file — and composes with local names (a local artifact may be hand-authored
   instead of base-rendered). No `custom:` boolean is added, so no sidecar-schema
   change and no drift-hash churn.

7. **`awf new skill|agent <name> "<description>"` scaffolds a complete local
   artifact:** it enables the name, writes the declaring sidecar (carrying the
   description), and writes a starter `content` part, then syncs. Arg-parsing
   branches by kind — name and description are distinct arguments, unlike
   `new adr`'s single joined title. The `new` help text and the unknown-kind
   error (and its test) update in the same commit.

8. **Local names are filesystem-validated.** A shared
   `ValidateArtifactName(kind, name)` check rejects path separators and `..`,
   mirroring `ValidateDomainName`, because a local name reaches `.awf/…` and
   `.claude/…` paths. The reserved base-template name is not usable as a local name.

9. **Harness-agnostic output is inherited, not built.** A local artifact renders
   once per enabled target through the existing multi-target loop (ADR-0037); a
   single definition yields per-harness output with no adapter-specific authoring.

## Invariants

- `inv: local-catalog-clone` — `project.Open` synthesizes local entries into a
  clone of `catalog.Standard`; the package global's `Skills`/`Agents` maps are
  never mutated by opening a project.
- `inv: local-requires-declaration` — a non-Standard, non-`local:true` enabled
  skill/agent name without a declaring sidecar is a hard error at project open.
- `inv: local-no-shadow` — a local (non-Standard) skill/agent name equal to a
  `catalog.Standard` name is rejected.
- `inv: local-renders-from-base` — a rendered local artifact resolves its template
  id to the shared base template for its kind, not the name-derived path.
- `inv: local-base-publication-safe` — the base skill and agent templates render
  leak-free (no `<no value>`, no marker/leak residue) under empty data and no
  `content` part.
- `inv: local-name-validated` — local skill/agent names are rejected when they
  contain a path separator or `..`.

## Consequences

- **Adopters extend the workflow without forking awf.** A project-defined skill or
  agent is authored as one content part plus a one-line sidecar, and awf owns all
  the structure (frontmatter, prefixing, publication-safety, per-target layout).
- **Harness-agnostic by construction.** One local definition is emitted for every
  enabled target; adopters stop hand-maintaining per-harness copies. This is the
  feature's headline payoff.
- **The effective catalog becomes project-scoped.** "The catalog" is now
  `Standard` for the standard set and a per-project clone-plus-locals at runtime.
  The compile-time authority of `catalog.Standard` over the *standard set* is
  preserved (`inv: catalog-go-single-source` stays true — no embedded
  `catalog.yaml`, no runtime parse of the standard); the clone is an augmentation
  layer, not a second source of the standard.
- **Local artifacts sit outside the chain/eval/parity machinery.** They are not in
  `catalog.Standard`, so the workflow-chain handoff invariants (ADR-0054), the
  full-catalog eval coverage (ADR-0053), and the Standard section/descriptor
  parity tests do not cover them. The base templates need their *own* parity and
  publication-safety locks (`inv: local-base-publication-safe`), since the
  Standard-ranging parity tests never see them.
- **One localized render change, not zero.** Item 4 adds a spec field and a branch
  in template-id resolution; the earlier hope of a purely additive merge was
  optimistic. The change is small and confined to the render path.
- **Cost accepted:** local artifacts get a single `content` section, not arbitrary
  sections; adopters wanting richer structure compose it inside the one part.
- **Unblocks** the follow-on question of local artifacts declaring a required
  paired agent (ADR-0050 style); deliberately out of scope here — synthesized
  specs leave `RequiresAgent` empty, so no pairing is enforced or offered for v1.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| New `custom: true` sidecar flag as the discriminator | Overloads the sidecar schema and churns every artifact's drift hash; redundant once "non-Standard name + sidecar" plus the existing `local` vocabulary already distinguish the cases. |
| Adopter-authored templates loaded from `.awf/` | Forces adopters to learn the template dialect, section markers, includes, and publication-safety rules; far larger surface. A base template + content part gives the same reach with almost nothing to author and keeps awf owning the structure. |
| Adopter edits `catalog.Standard` (the contributor model) | Impossible from a released binary; serves awf contributors, not project-local extension — the stated goal. |
| A dedicated `customSkills:`/`customAgents:` config section | Adds top-level schema and splits an artifact's declaration from the sidecar/parts surface adopters already use for overrides. |
| Multi-section local artifacts | Keeps the base template non-trivial and harder to prove publication-safe; a single `content` part already carries arbitrarily rich body content. |
