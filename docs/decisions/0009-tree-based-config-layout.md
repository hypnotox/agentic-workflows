---
status: Implemented
date: 2026-06-25
tags: [config-tree, sidecar-fields]
related: [1, 5, 6, 7, 8, 10, 15, 16, 131]
domains: [config]
---
# ADR-0009: Tree-Based Config Layout Under a Single `.claude/awf/` Root

## Context

A project's entire awf configuration lives in one monolithic `.claude/awf.yaml`
(~141 lines in this repo). That single file carries four very different kinds of
content at once: global skeleton (`prefix`, `invariants.sources`, `vars`, `hooks`),
per-target structured `data` arrays (`adrStates`, `testSurfaces`, `focusItems`,
`correctnessTraps`, ...), large inline prose (the `agentsDoc.data.ownership` and
`agentsDoc.data.identity` blocks: the biggest prose in the whole file), and
per-target section overrides (`drop` / `replaceWith`). Everything for every skill,
agent, doc, and the agent guide is interleaved in one place, so the file grows
without bound and a small per-target tweak means editing the central file.

A `parts/` mechanism already exists but is barely used: a section override
`replaceWith: parts/<file>.md` pulls a markdown body from
`.claude/awf/<file>`: the part base dir is already `.claude/awf/`
(`internal/project/project.go:121`, `filepath.Join(p.Root, ".claude", "awf", name)`).
Today only two parts exist (`.claude/awf/parts/debugging-surfaces.md`,
`.claude/awf/parts/doc-architecture.md`), each pointed at by an explicit
`replaceWith` string in the monolith.

The goal: keep files concise by splitting config into a file tree, and consolidate
all awf config under one folder. The deployment/migration mechanism that lets an
existing adopter move from the old single-file layout to this tree (a versioned
lock plus an `awf upgrade` command gated by drift) is **out of scope here** and is
the subject of a forthcoming companion ADR (referred to below as ADR-B); this ADR
defines the target layout and is implementable on its own by hand-porting this
repo's config (the final task of its plan).

Grounding discoveries that shape the design (verified against source unless noted):

- **`config.Load` reads exactly one file with `dec.KnownFields(true)`**
  (`internal/config/config.go`, `Load`), and `cmd/awf` hardcodes the path
  `.claude/awf.yaml` in three places (`cmd/awf/main.go:46` `runInit`,
  `cmd/awf/list_add.go:59` `runAdd`, `internal/project/project.go:39`
  `config.Load`). The lock path is hardcoded to `.claude/awf.lock`
  (`internal/project/project.go:398`). Any relocation is a multi-site change, and
  a new key shape is a hard parse error until the structs change (strict fields).
- **Enablement is "presence of a map key."** `Config.Skills`, `Config.Agents`,
  `Config.Docs` are `map[string]SkillConfig`; `SkillConfig` carries
  `Data`/`Sections`/`Local` (`internal/config/config.go:18-31`). `validateAgainstCatalog`
  and `RenderAll` iterate those maps and `continue` on `sc.Local`
  (`internal/project/project.go:58-71, 97-105, 161`). `AgentsDoc` is a singleton
  `*SkillConfig`.
- **`Local` targets are not frontmatter-checked today.** `validateFrontmatter`
  (`internal/project/project.go:225-241`) runs only inside the rendered-output loop
  (`project.go:352`), and local targets are skipped from rendering, so a local
  skill's hand-authored frontmatter is validated nowhere. The user requires local
  skills to be frontmatter-checked.
- **Section precedence is `drop > replaceWith > default`** in `render.Assemble`
  (`internal/render/render.go:24-35`); the `PartFunc` is `func(name string)
  (string, error)` resolving a path under `.claude/awf/`.
- **agentsDoc prose comes from `data` scalars filling section bodies**, not from the
  template body. `templates/agents-doc/AGENTS.md.tmpl:9` is
  `{{ with .data.ownership }}{{ . }}{{ else }}...default...{{ end }}` and `:15` the same
  for `.data.identity`; `invariants` loops `.data.invariants` (`:26`) and `docMap`
  loops `.data.docMap` (`:69`). The catalog declares `agentsDoc.sections:
  [you-and-this-project, identity, invariants, workflow, commands, document-map]`
  (`templates/catalog.yaml`).
- **Drift hashing currently uses the whole raw config.** `manifest.Entry` is a
  generic `path→hash` record (`internal/manifest/manifest.go:12-17`) and the
  per-file `ConfigHash` derives from the single config file's bytes
  (`internal/project/project.go`). Splitting config across many files means the
  drift signal must span the full per-target source set or edits to a sidecar/part
  go undetected.
- **The lock already carries a version field.** `manifest.Lock` has `AWFVersion`
  (`internal/manifest/manifest.go:20`) and `Version = "0.1.0"`
  (`internal/project/project.go:23`). This ADR only relocates the lock file; the
  versioned-upgrade *gate* is ADR-B.
- **`awf list`/`awf add` manipulate the `skills:` map textually.** `runList`
  reads the `p.Cfg.Skills` map; `appendSkill` (`cmd/awf/list_add.go:76-90`) inserts
  `  <skill>: {}` after the `skills:` line and converts the `skills: {}` empty-map
  form. `skillState` reports `available | enabled | tuned | local`
  (`cmd/awf/list_add.go:15-28`).
- **`Config.DocsDir` defaults to `"docs"`** (`internal/config/config.go`,
  ADR-0005). It names the *project* docs root (rendered ADRs/plans), distinct from
  the new `.claude/awf/docs/` config branch; the two co-exist as unrelated trees.

**User constraints driving the design (verbatim intent):** "keep files concise and
split them more"; the root is "`.claude/awf/`" holding "a single folder for all
config"; sidecars are "a single YAML file per doc file that does the linking ... stores
the overrides config if present"; "parts would be convention, typically located at
e.g. `.claude/awf/docs/parts/architecture`"; split scope is "prose bodies only" with
`data` kept "in the per-target sidecar"; agentsDoc prose is "re-model[led] as section
parts"; enablement is "Flat array + local, but be sure that local skills must also be
tested against the frontmatter check"; naming is "config.yaml + awf.lock"; and the ADR
split is "Two ADRs" with ADR-B owning "versioned lock + awf upgrade + gate."

## Decision

1. **`.claude/awf/` is the single config root.** Configuration loads from
   `.claude/awf/config.yaml`; the lock lives at `.claude/awf/awf.lock`. No normal
   load/render/sync/check path reads or writes the legacy `.claude/awf.yaml` or
   `.claude/awf.lock`; the sole exception is the `internal/migrate` package under
   `awf upgrade`, which reads the legacy file once to port it forward (ADR-0010).
   Per-kind branches `skills/`, `agents/`, `docs/` live directly under
   `.claude/awf/`, each containing optional `<target>.yaml` sidecars and a local
   `parts/<target>/<section>.md` convention directory. Concretely:

   ```
   .claude/awf/
     config.yaml
     awf.lock
     skills/  { <target>.yaml, parts/<target>/<section>.md }
     agents/  { <target>.yaml, parts/<target>/<section>.md }
     docs/    { <target>.yaml, parts/<target>/<section>.md }
   ```

   The `.claude/awf/docs/` branch (awf config) and the project `docsDir`
   (default `docs/`, rendered ADRs/plans; ADR-0005) are independent trees that
   co-exist with no interaction.

2. **`config.yaml` is the skeleton only.** It carries `prefix`,
   `invariants` (`sources`/`disabled`; ADR-0008), `vars`, `hooks`, `docsDir`, and
   **enable lists that are plain string arrays**: `skills`, `agents`, `docs` become
   `[]string` (target names); `hooks` stays `[]string`. Presence of a name enables
   that target. The root file carries **no** per-target `data`, `sections`, or
   `local`: those move to sidecars. `config.Config`'s `Skills`/`Agents`/`Docs`
   fields change from `map[string]SkillConfig` to `[]string`; a `data:`/`sections:`
   key at the root is a parse error (`KnownFields(true)`). `agentsDoc` is **not**
   one of these lists: it remains the always-on singleton `config.Config.AgentsDoc`
   and is excluded from the enable-list schema (item 7).

3. **Per-target sidecars hold everything non-prose.** A target's structured `data`,
   its `sections` overrides (`drop` / explicit `replaceWith`), and its `local` flag
   live in `.claude/awf/<kind>/<target>.yaml`. Sidecars are **optional and located
   by keyed lookup**, not filesystem scan: for each name in an enable list the
   loader reads `<kind>/<name>.yaml` if it exists. An enabled target with **no**
   sidecar resolves to an empty override set and renders from template defaults,
   preserving the publication-safe contract (ADR-0001): missing `data` renders as
   empty under `missingkey=zero`, never a no-value token. Enablement remains
   answerable from `config.yaml` alone; a stray sidecar for an unlisted target is
   ignored (and reported by `check` as an orphan, see item 6).

4. **Prose parts bind by convention.** For section `<sec>` of `<kind>/<target>`, if
   `.claude/awf/<kind>/parts/<target>/<sec>.md` exists, its contents replace that
   section body: no `replaceWith` pointer needed. The per-section precedence
   generalizes today's rule to four tiers:

   > sidecar `drop` > sidecar explicit `replaceWith` > convention part file > template default

   Convention-part resolution is a distinct precedence tier evaluated in section
   assembly. `render.Assemble`'s `PartFunc` is today a flat `func(name string)
   (string, error)` with no per-section kind/target context (`render.go:12`,
   `project.go:119-124`), so the convention lookup is owned by the project layer
   (which alone knows kind/target and can stat the tree). The chosen, lower-churn
   shape: the project layer pre-computes the per-section override map it passes to
   `Assemble`, injecting a synthetic `SectionOverride{ReplaceWith: <convention
   path>}` for any section whose convention part exists and that the sidecar did not
   already `drop` or explicitly `replaceWith`. This reuses the existing
   `replaceWith` tier in `Assemble`'s switch (`render.go:24-35`) without adding an
   `Assemble` parameter, while preserving the four-tier ordering. Explicit
   `replaceWith` in a sidecar remains an escape hatch for pointing at a
   non-conventional path and wins over the convention file (the project layer skips
   injection when the sidecar already set the section); `drop` still wins over both.

5. **`local` lives in the sidecar and local targets are now frontmatter-checked.** A
   local target is named in its kind's enable list and carries `local: true` in its
   sidecar (`.claude/awf/<kind>/<name>.yaml`). As today, local targets are skipped
   from catalog validation and from awf rendering (they are project-authored, not
   catalog-rendered). **New:** `sync` and `check` validate a declared local
   skill's/agent's hand-authored frontmatter at the conventional output path awf
   would otherwise render it to, reusing `validateFrontmatter`. Because local targets
   are skipped from `RenderAll` they have no `RenderedFile`, so the output path is
   derived *structurally* from `prefix`+`name` by a shared helper (skill:
   `.claude/skills/<prefix>-<name>/SKILL.md`; agent: `.claude/agents/<name>.md`; the
   same formulas `RenderAll` uses, `project.go:255,269`) without invoking the
   template. `sync`/`check` read that on-disk file and run `validateFrontmatter` on it;
   a local target whose
   on-disk file has missing/empty `name`/`description` fails with the same
   `invalid-frontmatter` signal as a rendered target (ADR-0006). A declared local
   target with no file at that path is a `check`/`sync` error.

6. **Drift uses a per-target projection, not a whole-tree hash.** Today
   `cfgHash = manifest.Hash(p.Cfg.Raw())` is the whole config file's bytes, shared
   by every rendered file (`project.go:362,417`); once config splits, `Raw()` no
   longer captures sidecars or parts. The replacement composes, **per rendered
   file**, a `ConfigHash` over exactly that file's effective inputs: (a) the global
   skeleton fields it actually reads (`prefix`, the `vars` it references, its own
   enable-list membership), (b) its sidecar's `data`/`sections`/`local` (the
   resolved override set, or empty if no sidecar), and (c) the bytes of any
   convention part file it consumed. These are assembled into a deterministic
   canonical encoding (a re-marshalled per-target sub-document plus the appended
   part bytes in fixed order) and hashed. The operational consequence is the point
   of choosing projection over a single whole-tree digest: editing `skills/tdd.yaml`
   re-flags only `tdd`'s outputs, not every rendered file. A sidecar or part file
   matching no enabled/declared target is reported as an orphan. `manifest.Entry`
   stays a generic `path→hash` record (`ConfigHash` remains a single string per
   entry), so the lock **format** is unchanged; only how each entry's `ConfigHash`
   is computed changes. The lock file relocates to `.claude/awf/awf.lock`; its
   `AWFVersion` field is untouched here (ADR-B owns version-gating).

7. **agentsDoc stays a distinct singleton kind; its prose moves to parts.**
   `agentsDoc` is **not** folded into the `Docs` map: it remains the top-level
   `config.Config.AgentsDoc *SkillConfig` singleton with its six named sections,
   always-on, rendering to the repo-root `AGENTS.md` (not under `docsDir`); folding
   it into `Docs` (a body-only `DocSpec` with title/desc, rendered under `docsDir`
   and listed in the agents-doc Document-map by `resolvedDocs()`, `project.go:158-173`)
   would give it fields it has no use for and make it list itself. Only its config
   home and prose-delivery change: its sidecar lives at `.claude/awf/agents-doc.yaml`
   and its convention parts at `.claude/awf/parts/agents-doc/<section>.md` (a
   top-level `parts/` branch for the singleton, reusing today's `.claude/awf/parts/`
   location). `templates/agents-doc/AGENTS.md.tmpl` is restructured so the
   `you-and-this-project` and `identity` section **bodies** carry **generic,
   adopter-neutral** prose directly (publication-safe, with no awf-specific content),
   replacing the `{{ with .data.ownership }}` / `{{ with .data.identity }}` scalar
   indirection; the `ownership` and `identity` data scalars are removed. A project
   overrides that prose via convention parts at
   `.claude/awf/parts/agents-doc/{you-and-this-project,identity}.md`. This repo's own
   specific identity/ownership prose (currently `agentsDoc.data.{ownership,identity}`
   in `.claude/awf.yaml`) therefore moves into those convention parts, so the
   byte-identical-output invariant below depends on those parts existing for the
   dogfood port. The `invariants[]` and `docMap[]` structured data stay as `data` in
   `.claude/awf/agents-doc.yaml`. Re-modelled section bodies must render
   publication-safe with no part override and with empty `invariants`/`docMap`
   (ADR-0001).

8. **CLI and scaffolding move to the tree.** `cmd/awf` path constants point at
   `.claude/awf/config.yaml` / `.claude/awf/awf.lock`. `init`/`ScaffoldConfig` emit
   the new tree (a skeleton `config.yaml` plus the `skills/ agents/ docs/` branches).
   `awf list` reads the array enable lists; `awf add` appends a name to the relevant
   array and creates a sidecar only when overrides are needed; `skillState` keeps the
   `available | enabled | tuned | local` vocabulary, with `tuned` now meaning "has a
   sidecar with data/sections" and `local` read from the sidecar.

Applying this layout to awf's own repo (hand-porting `.claude/awf.yaml` into
`.claude/awf/config.yaml` + the per-kind sidecars + the extracted parts (including
this repo's specific identity/ownership prose into
`.claude/awf/parts/agents-doc/*.md`), then re-syncing so rendered output is
byte-identical) is **not** a Decision item: it is adopter (dogfood) work, the final
task of the implementation plan, not a standard-definition commitment. This change
earns an ADR because it is load-bearing (new top-level config layout, changed config
schema, new render precedence tier, new drift composition, changed local-target
semantics) and a plan because it is multi-commit.

## Invariants

Checkable contracts that must hold while this decision stands. Tagged slugs are
backed by tests landing with implementation (enforced by `awf check` once this ADR
is `Implemented`; ADR-0008); untagged bullets are textual contracts.

- `invariant: config-root`: Config loads from `.claude/awf/config.yaml` and the lock is
  written to/read from `.claude/awf/awf.lock`; no normal load/render/sync/check path
  reads or writes `.claude/awf.yaml` or `.claude/awf.lock`. The `internal/migrate`
  package under `awf upgrade` is the single named exception, reading the legacy file
  only to port it forward (ADR-0010 `legacy-read-isolation`).
- `invariant: enable-arrays`: `config.Config.Skills`/`Agents`/`Docs` are string arrays
  whose entries enable targets by presence; a `data:`, `sections:`, or `local:` key
  at the root of `config.yaml` is rejected at load (`KnownFields(true)`).
- `invariant: sidecar-optional`: An enabled target with no `<kind>/<name>.yaml` sidecar
  renders successfully from template defaults, emitting no `<no value>` token for any
  absent `data` field (`missingkey=zero`, ADR-0001).
- `invariant: parts-convention`: A section is replaced by
  `.claude/awf/<kind>/parts/<target>/<section>.md` when that file exists, and the
  per-section precedence is `drop > explicit replaceWith > convention part >
  template default`.
- `invariant: local-frontmatter`: A declared `local` skill/agent has its on-disk
  frontmatter validated by `sync` and `check` at its conventional output path;
  missing/empty `name`/`description` fails identically to a rendered target, and an
  absent file for a declared local target is an error.
- `invariant: drift-source-set`: Each rendered file's `ConfigHash` is a per-target
  projection over only its own effective inputs (the skeleton fields it reads, its
  sidecar, its consumed parts); `awf check` reports that file stale when any of those
  inputs change since the last `sync`, and editing one target's sidecar or part does
  **not** flag unrelated targets' files. A sidecar or part file matching no
  enabled/declared target is reported as an orphan.
- `invariant: agentsdoc-parts`: The `agents-doc` `you-and-this-project` and `identity`
  section bodies are overridable via convention parts under
  `.claude/awf/parts/agents-doc/`, and render publication-safe with no override
  and empty `invariants`/`docMap`.
- The lock **format** is unchanged: `manifest.Entry` remains a generic `path→hash`
  record with a single-string `ConfigHash`; only the relocation and the
  hash-composition broadening apply here.
- With the dogfood ported, every rendered file (skills, agents, docs, the root
  `AGENTS.md`) is byte-identical to its pre-change output.

## Consequences

Easier:
- A per-target tweak edits one small sidecar or one part file instead of the central
  monolith; `config.yaml` shrinks to a skeleton whose enable lists answer "what's on"
  at a glance.
- The largest prose in the config (agentsDoc identity/ownership) becomes plain
  `.md` files editable as prose, not multi-line YAML scalars.
- Adding a skill no longer forces an inline `{}` placeholder in a shared map; an
  override file exists only when there is something to override.
- Local skills are finally held to the same frontmatter contract as rendered ones,
  closing a silent gap.

Harder / accepted trade-offs:
- Config loading grows from a single `ReadFile` to a root-plus-keyed-sidecar
  assembly; `config.Load`'s signature/shape and `config.Validate` must account for
  sidecars (validating a sidecar's section names against the catalog, reporting which
  file an error came from). Bounded and covered by tests.
- The `map[string]SkillConfig` → `[]string` change breaks every map-iterating call
  site that reads a target's `Local`/`Sections`/`Data` directly off the map value;
  each must now recover those from a sidecar lookup keyed by enable-list name:
  `validateAgainstCatalog` (reads `sc.Local`/`sc.Sections`, `project.go:57-117`),
  `RenderAll` (reads `sc.Local`/`sc.Sections`/`sc.Data`, `project.go:245-309`),
  `resolvedDocs` (reads `Docs[name].Local`, `project.go:158-173`), and
  `Config.Validate`→`checkSections` (reads `sc.Sections`, `config.go:84-98`).
  `runList`'s map-membership test (`sc, ok := p.Cfg.Skills[n]`, `list_add.go:42`)
  becomes a slice-membership test plus a sidecar load, and `skillState`'s signature
  changes from `(config.SkillConfig, bool)` to take a name plus a loaded sidecar (or
  nil) so its `tuned` detection reads `data`/`sections` from the sidecar. Compile-time
  breakage; covered by tests.
- Drift detection must compose a multi-file source set per rendered file; mis-scoping
  it would either miss edits or over-flag. Mitigated by tests covering "edit a
  sidecar", "edit a part", "orphan sidecar/part", and "byte-identical after port".
- A new convention-part precedence tier is a contract among `render.Assemble`, the
  project layer (which supplies the convention lookup), and the sidecar `replaceWith`
  escape hatch. Enumerated in Decision item 4.
- `awf list`/`add` rewrite from map-keyed YAML editing to array editing plus optional
  sidecar creation; `appendSkill`'s string surgery changes from `  <name>: {}` to a
  `- <name>` array append.
- The agents-doc template restructure moves prose from data scalars into section
  default bodies; the golden/spine tests asserting AGENTS.md output and any test
  seeding `agentsDoc.data.ownership`/`identity` update in lockstep.
- This ADR alone leaves existing adopters (including this repo) to migrate by hand;
  the ergonomic, gated migration path is deferred to ADR-B and is why the two are a
  tight sequence.

Doc-currency obligations the implementing commit(s) must satisfy:
- `docs/architecture.md` describes the new `.claude/awf/` layout (config root, per-kind
  branches, sidecars, convention parts) and the relocated lock.
- `AGENTS.md`'s "`awf check` is the drift oracle" invariant text and any reference to
  `.claude/awf.yaml` update to the `.claude/awf/config.yaml` path and the broadened
  source set. That text lives in the agents-doc invariants data (today
  `agentsDoc.data.invariants` in `.claude/awf.yaml`, relocating to
  `.claude/awf/agents-doc.yaml`'s `data.invariants`); edit the `.claude/awf.yaml`
  reference there and the publication-safe invariant's "`awf check` after any sync"
  phrasing if affected, then re-render from the ported config.
- When this ADR flips to Accepted/Implemented, the same commit regenerates `ACTIVE.md`
  via `./x sync`. No `docs/decisions/README.md` index row is owed (this repo's README
  is a how-to guide; `ACTIVE.md` is the generated index, ADR-0005).

Downstream work unblocked: (1) an implementation plan covering the config struct/loader
change, sidecar discovery and validation, the convention-part precedence tier, the
drift-source-set composition, the local-frontmatter check, the agents-doc re-model, the
CLI/scaffold updates, and the dogfood port, with tests at each step; and (2) ADR-B,
the versioned-lock + `awf upgrade` + drift-gate mechanism that migrates existing
adopters from the single-file layout to this tree.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep enablement as a map with thin values (`name: {}` / `name: {local: true}`) | Lowest churn and keeps `local` inline, but the user chose flat arrays; arrays make "what's on" a clean list and push every override (including `local`) into the sidecar where the rest of a target's config already lives. |
| Convention-only parts (drop explicit `replaceWith`) | Removes the escape hatch for pointing at a non-conventional path; keeping `replaceWith` as a higher-precedence override costs nothing and preserves today's two existing pointers' semantics during migration. |
| Auto-discover targets by scanning the tree for sidecars | Makes enablement implicit in file presence, so "what's on" is no longer answerable from `config.yaml`, and a stray file silently enables a target. Keyed lookup from the enable list keeps the root authoritative. |
| Externalize `data` arrays into `.md`/separate files too | Scope was explicitly "prose bodies only"; structured `data` stays as YAML in the sidecar where it is schema-checkable and close to its `sections`. |
| Keep agentsDoc prose as `data` scalars in its sidecar | Leaves the config's largest prose as multi-line YAML, partially defeating the goal; re-modelling the two section bodies into convention parts is the one template change that yields the biggest conciseness win. |
| Fold the migration mechanism (versioned lock + `awf upgrade` + gate) into this ADR | Couples a reusable, forward-looking migration mechanism to one layout change; one-decision-per-ADR keeps it as ADR-B (user-selected two-ADR split). |
| Fold `agents-doc` into the `Docs` map for a uniform per-kind tree | `agentsDoc` is a 6-section, always-on singleton rendering to repo-root `AGENTS.md`; `Docs` is a body-only `DocSpec` (title/desc) rendered under `docsDir` and folded into the Document-map by `resolvedDocs()`. Folding would force unused title/desc, push 6 sections through a body-only spec, and make it list itself: more special-casing than keeping it a distinct singleton kind (item 7). |
| Whole-config-tree hash instead of per-target projection (item 6) | A single digest over `config.yaml` + all sidecars + all parts is simpler, but any edit re-flags every rendered file as stale, erasing the locality that makes drift output actionable. Per-target projection costs a per-target re-encode and is worth it. |
| Change the lock format to record per-source hashes | Per-source hashes (one each for skeleton-slice / sidecar / part) would let `check` name *which* source drifted, not just which target. But `ConfigHash` is a single string per `manifest.Entry`; per-source hashes need a format change, and the per-target projection already localizes drift to the offending target: `check` can re-hash a target's components on demand to attribute further. The format change isn't warranted now. |
