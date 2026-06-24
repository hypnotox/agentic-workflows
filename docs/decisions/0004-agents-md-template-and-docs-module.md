---
status: Proposed
date: 2026-06-24
supersedes: []
superseded_by: ""
tags: [tooling, templates]
related: [0001]
---
# ADR-0004: Family-Aligned AGENTS.md Template and Opt-In Docs Module

## Context

The standard's generated agent guide (`templates/agents-doc/AGENTS.md.tmpl`) has drifted
structurally from the mature, hand-written exemplars in the sibling repos that seeded this
standard: `a reference project/AGENTS.md` and `a reference project/AGENTS.md`. Those two share an identical section shape:

```
## You and this project   ## Identity   ## Invariants
## Workflow   ## Commands (via ./x)   ## What this project is NOT   ## Document map
```

The awf template instead emits `Build & Test · Workflow Chain · Repository Layout ·
Conventions & Invariants`. Beyond cosmetic divergence, two substantive problems:

- **The Workflow section misrepresents the skills.** The template enumerates all eight skills
  as a flat linear sequence, including `reviewing-plan-resync` as a top-level step. The actual
  skill design treats reviews as *lightweight* — the grounding-check inside `*-brainstorming`
  subsumes plan/ADR review, and `*-reviewing-impl` is the single terminal review. The generated
  guide therefore teaches a heavier process than the skills implement.
- **No context-gathering surface.** `awf-brainstorming` step 1 instructs the agent to "read
  AGENTS.md, relevant docs (architecture, workflow, testing)". The exemplars carry a `docs/`
  tree (architecture, workflow, testing, development, debugging, pitfalls, glossary, roadmap)
  that a `## Document map` section links and that this step presumes. The awf template references
  none of it, and a fresh adopter project has nothing for the step to read.

awf is "both the tool that publishes the standard and the first adopter of it" (`AGENTS.md`),
so the generated shape and awf's own guide should be the same family shape.

Grounding discoveries that shape the design (verified against source):

- **`agentsDoc.data` already reaches the template as `.data` at root.** `Project.data()`
  (`internal/project/project.go:108-114`) builds `{"prefix":…, "vars":…, "data": nonNil(sc.Data)}`
  and passes it to `render.Render`; `.data.foo` is reachable. `SkillConfig.Data` is
  `map[string]any` (`internal/config/config.go:20`), so config-driven prose needs **no struct
  change** for the narrative fields — they ride the existing `data` map.
- **`{{ range .data.X }}` over an absent key is safe.** `render.Render` always sets
  `missingkey=zero` (`internal/render/render.go:47`); ranging a missing/nil `.data` field no-ops
  with no `<no value>` token. The post-render `<no value>` guard (`project.go:213-214`) still
  forbids bare missing-var *output*, so optional `.data.*` and `.vars.*` interpolations are
  wrapped in `{{ if }}`/`{{ else }}` exactly as ADR-0003 established for `checkCmd`.
- **A new opt-in `docs:` group mirrors `hooks:` and touches exactly five files** — no
  lock-format change. The manifest `Entry` (`internal/manifest/manifest.go:12-17`) is a generic
  path→hash record; each rendered doc is an ordinary entry. Wiring: `Config.Docs []string`
  (`internal/config/config.go`), a `docs:` list in `templates/catalog.yaml`, `Catalog.Docs`
  (`internal/catalog`), a render loop in `internal/project`, and the `//go:embed` line in
  `templates/embed.go` (currently `catalog.yaml skills hooks agents agents-doc`). The
  `Config.Docs` field is a **hard prerequisite**, not merely additive: `config.Load` decodes
  with `dec.KnownFields(true)` (`internal/config/config.go:42`), so a `docs:` key in `awf.yaml`
  is a parse error (`field docs not found`) until the struct field exists. The five-file wiring
  must therefore land before any adopter — including the dogfood — can write `docs:`.
- **Both `agentsDoc` and `docs` are opt-in by construction.** `ScaffoldConfig`
  (`internal/project/scaffold.go`) does not emit an `agentsDoc:` block, so a fresh `awf init`
  already produces no AGENTS.md unless the adopter adds the block; an absent `docs:` key likewise
  yields no docs.

**User constraints driving the design (verbatim intent):** drop the negative-framing
"What this project is NOT" section — "negative phrasing is not good with LLMs"; make the
narrative prose config-driven; keep docs-stub generation "disabled [by default] so we don't
automatically clutter the default templates… simply there as a standard way of doing if needed";
when enabled it "could also include an automatic AGENTS.md section with all of those docs
linked"; and "the AGENTS.md should allow adding more other doc links to be added during sync."

## Decision

1. **Reshape `templates/agents-doc/AGENTS.md.tmpl` to the family shape, minus negative framing.**
   The sections become: `## You and this project`, `## Identity`, `## Invariants`,
   `## Workflow`, `## Commands (via ./x)`, `## Document map`. The exemplars' `## What this
   project is NOT` is **dropped** (negative framing); any genuine non-goal is re-expressed
   positively inside Identity. `## Repository Layout` is **removed from AGENTS.md** — the family
   keeps layout in `docs/architecture.md` and links it from the Document map. `templates/catalog.yaml`
   `agentsDoc.sections` is redefined to the new section markers (replacing `overview, build-test,
   workflow-chain, layout, conventions`).

2. **Config-driven prose via the existing `agentsDoc.data` map.** Recognised keys:
   `ownership` (string), `identity` (string), `invariants` (list of `{text, ref?}`),
   `commands` (list of `{cmd, desc}`), `docMap` (list of `{path, desc}`). Each section renders
   from its data when supplied, else an inline guided fallback (an HTML-comment instruction or a
   default sentence keyed on `{{ .prefix }}`), so a fresh adopter who only sets `agentsDoc: {}`
   still gets a structurally complete, usable file. Every optional `.data.*`/`.vars.*` output is
   guarded so empty config never emits `<no value>` under `missingkey=zero`. No `config.go`
   schema change is required for these narrative fields.

3. **Invariants render in two tiers.** Three universal contracts the standard itself guarantees —
   append-only ADRs (regenerated `ACTIVE.md`), docs-travel-with-the-change, green-gate-before-commit
   — are emitted inline in the template from existing vars (`adrReadme`, `activeMdPath`, `gateCmd`).
   Adopter-supplied `data.invariants` are appended after them. The universal tier is suppressed
   only by the standard overlay mechanism (section `drop`/`replaceWith`), not by config absence.
   **Caveat — these vars can be empty strings, not just absent.** `ScaffoldConfig` seeds every
   referenced var (including `adrReadme`/`activeMdPath`/`gateCmd`) with `""`
   (`internal/project/scaffold.go:74`), and an empty-string var passes the `<no value>` guard
   silently (`missingkey=zero` only zero-fills *absent* keys). A fresh adopter who adds an
   `agentsDoc:` block before filling these vars would otherwise render broken empty-target links
   (`[ACTIVE.md]()`) or an empty gate command that `awf check` cannot catch. Each universal-tier
   line that interpolates such a var must therefore be `{{ if }}`-guarded to fall back to a plain
   sentence (or omit the link) when the var is empty — the same empty-string discipline applied to
   the `.data.*` fallbacks, extended to `.vars.*` here.

4. **Add an opt-in `docs:` module.** A new top-level `docs: []string` key in `awf.yaml`,
   **absent by default** (not emitted by `ScaffoldConfig`; mirrors the `hooks:` list pattern).
   When present it renders a new `templates/docs/*.md.tmpl` set — `architecture, workflow, testing,
   development, debugging, pitfalls, glossary, roadmap` — into the project's `docs/` tree, one
   file per listed entry. Wiring is the five files enumerated in Context plus the new template
   directory; the manifest/lock format is unchanged (ordinary path→hash entries). The catalog
   `docs:` entries carry a `title`/`desc` (richer than bare-string `hooks:`) so the Document map
   can annotate the auto-generated links.

5. **Document map auto-links generated docs and stays extensible.** `internal/project` passes the
   resolved `docs:` list (with catalog titles/descriptions) into the agents-doc template data so
   the `## Document map` section ranges it for annotated links when the module is enabled, and the
   always-present surfaces (ADR `README`, `ACTIVE.md`, plans dir) plus any adopter `data.docMap`
   entries render regardless. Adding a `data.docMap` entry and re-running `awf sync` adds the link
   — no template edit needed.

6. **Dogfood on awf itself.** Rewrite this repo's `.claude/awf.yaml` `agentsDoc.data`
   (ownership, identity, invariants, docMap), enable a small `docs:` subset, regenerate
   `AGENTS.md`, and migrate the current `Repository Layout` content into a generated
   `docs/architecture.md`. `awf check` is clean after `./x sync`.

## Invariants

- `templates/agents-doc/AGENTS.md.tmpl` contains exactly the six headings `## You and this
  project`, `## Identity`, `## Invariants`, `## Workflow`, `## Commands (via ./x)`,
  `## Document map`, and contains neither `## What this project is NOT` nor `## Repository Layout`.
- Rendering the agents-doc template with an empty `agentsDoc.data` (`{}`) produces no `<no value>`
  token and yields every section with its guided fallback content.
- The three universal invariants (append-only ADRs, docs-travel-with-the-change,
  green-gate-before-commit) appear in the rendered Invariants section even when
  `data.invariants` is unset; adopter `data.invariants` entries render after them.
- Rendering the agents-doc template with `adrReadme`, `activeMdPath`, and `gateCmd` all set to
  the empty string (the `ScaffoldConfig` default) produces no `<no value>` token, no empty-target
  markdown link (`]()`), and no empty fenced command block; each universal-tier line degrades to a
  plain sentence.
- With `docs:` absent from `awf.yaml`, no file under `docs/` is rendered and the Document map
  omits the generated-docs block; with `docs:` listing N catalog entries, exactly N files render
  under `docs/` and N annotated links appear in the Document map.
- A fresh `awf init` (`ScaffoldConfig` output) emits neither an `agentsDoc:` block nor a `docs:`
  key — both modules are opt-in.
- The manifest/lock format (`internal/manifest` `Entry` schema) is unchanged by the `docs:`
  module; each rendered doc is an ordinary path→hash entry.
- The rendered Workflow section presents the chain with lightweight reviews (no linear
  `*-reviewing-plan-resync` step in the primary sequence), matching the skill design.

## Consequences

Easier:
- The generated agent guide matches the internal reference projects family shape, so an adopter's AGENTS.md
  and awf's own guide read the same way, and the guide stops teaching a heavier review process
  than the skills run.
- `awf-brainstorming` step 1 finally has real context-gathering targets in any project that
  enables the `docs:` module; the Document map points at them.
- Narrative prose is config-driven and additive — adopters extend Identity/Invariants/Document
  map from `awf.yaml` with no template edits.

Harder / accepted trade-offs:
- The template grows conditional fallbacks for every narrative section; it is more complex to
  read than the current flat template. Mitigated by golden tests covering the data-present,
  data-absent, docs-on, and docs-off render paths.
- The awf schema grows: an opt-in top-level `docs:` key and the recognised `agentsDoc.data`
  keys. Consumers of the `docs:` addition: `Config.Docs` (auto-unmarshals), `Catalog.Docs`,
  the `project` render loop, `embed.go`, and `catalog.yaml` — enumerated and bounded; the lock
  format is untouched.
- Dogfooding moves the `Repository Layout` content out of `AGENTS.md` into `docs/architecture.md`.
  Readers who looked for layout in AGENTS.md now follow the Document map link.
- The catalog `docs:` entries are objects (`title`/`desc`), a slightly richer shape than the
  bare-string `hooks:` list — a small, deliberate asymmetry so links can be annotated.

Doc-currency obligations the implementing commit(s) must satisfy:
- `.claude/awf/parts/agents-doc-{overview,layout,conventions}.md` are static overlay parts that
  `awf check` will **not** flag as drift; the dogfood step must hand-migrate their content
  (overview → the new Identity / You-and-this-project sections; layout → `docs/architecture.md`;
  conventions → the new Invariants / Workflow sections — the current `conventions` part carries
  TDD/gate/commit/doc-currency rules that have no standalone heading in the new shape) and re-sync
  so `AGENTS.md` re-renders.
- **Same-commit hard requirement, not a soft obligation:** redefining the catalog
  `agentsDoc.sections` list to the new section markers makes the existing `.claude/awf.yaml`
  `agentsDoc.sections` overlay (which keys `overview`/`layout`/`conventions`) **fail validation**,
  not merely dangle. `Project.Open` → `validateAgainstCatalog` → `checkSectionsAllowed`
  (`internal/project/project.go:94,129-143`) rejects any override key absent from the catalog list
  with `unknown section … (not declared in the catalog)`, so `awf sync`/`awf check` — and the
  pre-commit hook — error out until the overlay keys are renamed (or dropped) in the same change
  that redefines the catalog. The implementing plan must sequence the catalog redefinition, the
  awf.yaml overlay rewrite, and the re-sync as one atomic step to keep the gate green.
- When this ADR's status flips to Accepted or Implemented, the same commit regenerates
  `docs/decisions/ACTIVE.md` via `go test ./internal/adrtools/`. No `docs/decisions/README.md`
  index row is owed — this repo's README is a how-to guide; `ACTIVE.md` is the generated index
  (per ADR-0003).

Downstream work unblocked: an implementation plan covering (1) the template reshape + catalog
section redefinition, (2) the `docs:` group wiring + the `templates/docs/*.tmpl` set, (3) the
Document-map data pass-through, and (4) the awf dogfood (config rewrite, `docs/architecture.md`,
re-sync), with golden tests at each step.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Keep the thin template, just rename headings | Cosmetic only; leaves the workflow-misrepresentation and the missing context-gathering surface unfixed. |
| Pure placeholder skeleton (no auto-render) | A fresh `agentsDoc: {}` would yield an unusable stub; the family value is in the auto-rendered Workflow/Commands/Document-map sections. |
| Add explicit `config.go` fields for the narrative prose (identity, invariants…) | The existing `agentsDoc.data map[string]any` already carries arbitrary structured data to the template; a typed schema adds surface for no gain. |
| Generate the `docs/` set always-on | Clutters every fresh project with stub docs it may not want; the user requires the module be opt-in. |
| Keep `## What this project is NOT` | Negative framing is a poor instruction shape for an LLM guide (user constraint); non-goals re-expressed positively in Identity. |
| Split into two ADRs (template reshape vs docs module) | The docs module exists to serve the Document-map section; the two are one coupled decision about the agent-guide shape, so one ADR + one multi-task plan. |
| Bare-string `docs:` entries like `hooks:` | The Document map needs a human-readable annotation per link; carrying `title`/`desc` in the catalog entry is the minimal way to get annotated auto-links. |
