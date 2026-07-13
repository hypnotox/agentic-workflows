---
status: Implemented
date: 2026-06-25
supersedes: []
superseded_by: ""
tags: [active-md, layout-namespace]
related: [1, 4]
domains: [config, adr-system]
---
# ADR-0005: docsDir Layout Key and Built-In ACTIVE.md Generation

## Context

`docs/decisions/ACTIVE.md` (the generated ADR status index) is produced by a *test*
with a write-then-fail side effect: `TestGenerateActiveMD`
(`internal/adrtools/adrtools_test.go:117-140`) regenerates the file and then calls
`t.Fatalf` so the author re-stages it. It is invoked via `./x adr` (= `go test
./internal/adrtools/`). The generator itself, `GenerateActiveMD(decisionsDir)`
(`internal/adrtools/adrtools.go:37`), is a clean pure function — but nothing in the
shipped `awf` binary calls it.

This is hand-rolled, repo-internal tooling masquerading as part of the standard. The
skills already reference the regeneration as adopter-agnostic vars —
`{{ .vars.activeMdRegenCmd }}` and `{{ .vars.activeMdPath }}` in
`templates/skills/proposing-adr/SKILL.md.tmpl:69` and
`templates/skills/adr-lifecycle/SKILL.md.tmpl:75` — so the standard *tells* adopters to
regenerate `ACTIVE.md`, but ships no tool that does it. An adopter who `go install`s the
`awf` binary gets the ADR scaffolding and the skills that reference regeneration, yet
must hand-roll their own generator behind `activeMdRegenCmd`. The generator is the one
piece of the ADR workflow that never became real `awf` tooling.

Compounding this, the project's doc paths are spread across six hand-set free-form vars
in `.claude/awf.yaml` — `adrDir`, `plansDir`, `activeMdPath`, `adrReadme`,
`adrTemplatePath`, `planTemplatePath` — plus a hardcoded `"docs/"` output prefix in two
spots in `internal/project/project.go` (`resolvedDocs` line 143, the `RenderAll` docs
loop line 233). Plans and ADRs are foundational to the workflow, not optional add-ons, so
their location should not be six independent knobs that can drift out of agreement with
each other and with the hardcoded managed-docs prefix.

Grounding discoveries that shape the design (verified against source unless noted):

- **`config.Config` parses with `dec.KnownFields(true)`** (`internal/config/config.go:42`),
  so a new top-level `docsDir` key is a hard prerequisite — an `awf.yaml` carrying it is a
  parse error until the struct field exists — and there is no existing defaults mechanism;
  an absent `docsDir` must be defaulted in code.
- **`Project.data()` returns a fresh `{"prefix", "vars", "data"}` map per call**
  (`internal/project/project.go:122-128`) and is the single choke point where computed
  layout values can be injected into render data. The `Vars` map is not mutated elsewhere.
- **`render.ReferencedVars` only scans `.vars.X`** (`internal/render/vars.go`), and
  `ScaffoldConfig` seeds every referenced `.vars.X` name with `""`
  (`internal/project/scaffold.go:71-74`). Therefore any layout path exposed under a
  namespace *other than* `.vars` is invisible to scaffolding — it will not be seeded as an
  empty var slot. This is what makes the paths "strictly awf-given": users cannot set them.
- **`manifest.Entry` is a generic path→hash record** (`internal/manifest/manifest.go:12-17`)
  with `TemplateID`/`TemplateHash`/`ConfigHash`/`OutputHash`, none assumed non-empty; the
  lock prune logic in `Sync` (`project.go:296-305`) removes any locked path no longer in the
  `want` set. An ACTIVE.md entry with empty `TemplateID`/`TemplateHash` fits with no
  lock-format change.
- **`Project.Check` detects staleness via `TemplateHash`/`ConfigHash`**
  (`project.go:336-340`). Because `ACTIVE.md` is generated from ADR frontmatter, not a
  template, both hashes are empty and that comparison passes silently — an ADR frontmatter
  change would not be flagged. Staleness for `ACTIVE.md` therefore needs a dedicated
  regenerate-and-compare path, separate from the template-hash drift loop.
- **`internal/adrtools` is imported only by its own test** (no `cmd/` or `internal/project`
  import), so moving the call site into `project` owns the integration cleanly.

**User constraints driving the design (verbatim intent):** "The regen can simply be a part
of sync and staleness can be included in the check." "Plans and ADRs are the basis of this
workflow, so they should always be enabled." "We can simply make the docs path configurable,
anything below that is given structure." "The other doc paths, they should all be given
strictly by the awf-given layout." "Remove the adrDir and plansDir entries." "If no ADRs
exist yet, it simply does nothing."

## Decision

1. **Add a first-class `docsDir` config key.** `Config` gains `DocsDir string`
   (`yaml:"docsDir"`); when absent or empty it defaults to `"docs"` in code (the only
   defaulting needed, applied at load). It is the single configurable docs root. With
   `docsDir: "docs"` every rendered/generated path is byte-identical to today; setting it to
   another value relocates the entire docs tree atomically.

2. **awf imposes a fixed layout beneath `docsDir`, exposed under a dedicated `.layout`
   render namespace.** `Project.data()` injects a computed `layout` map alongside
   `prefix`/`vars`/`data`. The canonical paths, all derived from `docsDir`:

   | `.layout` key | value (docsDir = `docs`) |
   |---|---|
   | `docsDir` | `docs` |
   | `adrDir` | `docs/decisions` |
   | `activeMd` | `docs/decisions/ACTIVE.md` |
   | `adrReadme` | `docs/decisions/README.md` |
   | `adrTemplate` | `docs/decisions/template.md` |
   | `plansDir` | `docs/plans` |

   Skill/agent templates **and `agents-doc/AGENTS.md.tmpl`** migrate their
   `{{ .vars.adrDir }}`, `{{ .vars.plansDir }}`, `{{ .vars.activeMdPath }}`,
   `{{ .vars.adrReadme }}`, `{{ .vars.adrTemplatePath }}` references to the corresponding
   `{{ .layout.* }}` keys. `agents-doc/AGENTS.md.tmpl` currently guards these vars with
   `{{ with .vars.X }}…{{ else }}docs/decisions{{ end }}` (line 23) and conditional
   Document-map bullets (lines 65-67); because every `.layout.*` key is always populated,
   those guards collapse to **unconditional** output and the `{{ else }}docs/decisions{{ end }}`
   fallback becomes dead — the migration must drop the guards rather than translate them
   verbatim. `{{ .vars.planTemplatePath }}` is **dropped, not migrated** (see note below): it
   has no plan-template source, so it gets no `.layout` analogue.
   The five migrated doc-path vars *and* `planTemplatePath` are **removed** from config, from
   `ScaffoldConfig`'s seeded set (automatic, since `.layout.*` is not scanned by
   `ReferencedVars`), and from this repo's `.claude/awf.yaml`. Because layout lives outside
   `.vars`, an adopter cannot override a layout path through `vars` — the structure is
   strictly awf-given.

   > **`planTemplatePath` is dropped; plan-template support is deferred.** Unlike
   > `adrTemplatePath` (which resolves to `docs/decisions/template.md`, an existing committed
   > file), there is no plan-template source under `templates/` and `planTemplatePath` ships
   > empty (`""`) in this repo, guarded by `{{ if .vars.planTemplatePath }}` in
   > `writing-plans/SKILL.md.tmpl:51`. Folding it into the always-populated `.layout`
   > namespace would unconditionally enable that section pointing at a non-existent template.
   > This ADR therefore **removes** the `planTemplatePath` var and that conditional section
   > outright (no `.layout.planTemplate` key); introducing a real `templates/plans/template.md.tmpl`
   > and a `.layout.planTemplate` key is left to a future ADR if plan-template scaffolding is
   > ever wanted.

3. **Managed docs unify under `docsDir` (full unification).** Two distinct hardcoded
   `"docs/"` literals are rooted at `docsDir`: the `RenderAll` docs-loop output path
   (`project.go:233`, `"docs/"+name+".md"`) and the `resolvedDocs` Document-map link path
   (`project.go:143`, `"path": "docs/"+name+".md"`) the agents-doc template receives. Both
   become `<docsDir>/<name>.md`. The doc *template* source location
   (`templates/docs/<name>.md.tmpl`, embedded) is unaffected — only the output path is rooted
   at `docsDir`.

4. **`awf sync` generates `ACTIVE.md` natively.** After rendering templates, if
   `<docsDir>/decisions/` exists and contains at least one `NNNN-*.md` ADR, `sync` calls
   `GenerateActiveMD` and writes `<docsDir>/decisions/ACTIVE.md`, recording it as an ordinary
   lock entry (empty `TemplateID`/`TemplateHash`; `OutputHash` = hash of the generated
   content). If the decisions dir is absent or holds zero ADRs, `sync` produces **no**
   `ACTIVE.md` (no empty-file artifact) and the existing prune logic removes any previously
   generated `ACTIVE.md` from disk and lock.

5. **`awf check` detects `ACTIVE.md` staleness via a dedicated path.** The generic `Check`
   lock-iteration loop (`project.go:329-350`) must **skip** the `ACTIVE.md` lock entry: that
   loop looks each locked path up in the `RenderAll` output (line 331) and `ACTIVE.md` is not
   a rendered file, so it would be flagged `orphaned` every run; and its empty `ConfigHash`
   against the live non-empty `cfgHash` (line 336) would otherwise read as `stale`. Excluding
   it from the generic loop and handling it only here avoids both false positives.
   Independently of the template-hash drift loop, `check` regenerates `ACTIVE.md` in memory
   from the current ADR frontmatter and compares it to the on-disk file: a content mismatch
   (frontmatter changed
   since last sync, or hand-edit) yields a drift entry; an absent file when ADRs exist yields
   a `missing` drift; when zero ADRs exist and no `ACTIVE.md` is locked, there is nothing to
   flag.

6. **Retire the test-as-tool mechanism.** Delete the write-then-fail `TestGenerateActiveMD`
   gate (the hermetic unit test `TestGenerateActiveMDGroupsByStatus` stays). Drop the
   `./x adr` target. Repoint `activeMdRegenCmd` from `go test ./internal/adrtools/` to the
   sync command (`./x sync` in this repo; `awf sync` is the adopter default). Update the
   `docs/decisions/README.md` "ACTIVE.md" how-to section, and the agent `docCurrencyItems`
   strings in `.claude/awf.yaml` that read "regenerated via go test ./internal/adrtools/", to
   describe the sync-driven mechanism; re-render the affected agents.

Applying this change to awf's own repo (the dogfood — defaulting/setting `docsDir`, removing
the six doc-path vars, repointing `activeMdRegenCmd`, re-rendering and re-syncing so the
templates and `ACTIVE.md` reflect the new mechanism) is **not** a Decision item: it is
adopter work, the final task of the implementation plan, not a standard-definition commitment
the project must remember. This change earns an ADR because it is load-bearing (new schema
key, new render-data namespace, changed sync/check semantics) and a plan because it is
multi-commit.

## Invariants

Checkable contracts that must hold while this decision stands. (The `// invariant:` test
tagging convention and its enforcement checker are deferred to ADR-0006, which will
retro-apply tagged tests to these bullets; for now they are textual contracts verified by
the implementation plan's tests.)

- `invariant: docsdir-default` — `config.Config` has a `docsDir` field; loading an `awf.yaml` that omits it yields
  `DocsDir == "docs"`, and an explicit value relocates every layout path and managed-doc
  output path to that root.
- `invariant: layout-derivation` — The layout paths (`adrDir`, `activeMd`, `adrReadme`, `adrTemplate`, `plansDir`) are
  reachable in templates under `.layout.*` and are **not** present as `.vars.*` keys;
  `render.ReferencedVars`/`ScaffoldConfig` does not seed them, so a fresh `awf init` emits no
  `adrDir`/`plansDir`/`activeMdPath`/`adrReadme`/`adrTemplatePath` var. `planTemplatePath` is
  likewise no longer seeded (it is removed, not migrated — it has no `.layout` analogue).
- With `docsDir` defaulting to `"docs"`, every rendered file and the generated `ACTIVE.md`
  are byte-identical to the pre-change output (managed docs at `docs/<name>.md`, ADR index at
  `docs/decisions/ACTIVE.md`).
- `invariant: sync-generates-active-md` — `awf sync`, run with ≥1 `NNNN-*.md` ADR under `<docsDir>/decisions/`, writes
  `<docsDir>/decisions/ACTIVE.md` grouped by status and records it in the lock; run with an
  absent or ADR-less decisions dir, it writes no `ACTIVE.md` and prunes any previously locked
  one. (Prune safety: the existing `Sync` prune removes a pruned file's parent dir only when
  empty (`project.go:301-303`); `<docsDir>/decisions/` holds the ADRs themselves, so removing
  a pruned `ACTIVE.md` never deletes the decisions dir.)
- `invariant: check-active-md-stale` — `awf check` reports drift for `ACTIVE.md` when the on-disk content differs from a fresh
  regeneration (frontmatter change, hand-edit) or when it is absent while ADRs exist — via a
  path that does not depend on `TemplateHash`/`ConfigHash`. A synced, unchanged `ACTIVE.md`
  produces **no** drift entry (not `orphaned`, not `stale`): the generic lock-iteration loop
  skips the `ACTIVE.md` path entirely.
- The `manifest`/lock format is unchanged: the `ACTIVE.md` entry carries empty
  `TemplateID`/`TemplateHash` and is an ordinary path→hash record.
- The repository contains no `./x adr` target and no write-then-fail `TestGenerateActiveMD`;
  `activeMdRegenCmd` resolves to the sync command everywhere a skill references it.

## Consequences

Easier:
- The ADR index generator becomes real `awf` tooling that adopters get for free with the
  binary — `sync` generates it, `check` guards it — closing the one gap where the standard
  referenced a tool it did not ship.
- One `docsDir` knob replaces six hand-set path vars plus a hardcoded prefix; the docs tree
  relocates atomically and the layout can no longer drift internally.
- Scaffolding stops emitting empty doc-path var slots, because the layout lives outside the
  `.vars` namespace `ReferencedVars` scans.
- ADR authors no longer run a separate `./x adr`; the normal `./x sync` / `awf check` cycle
  that already precedes every commit keeps `ACTIVE.md` current and honest.

Harder / accepted trade-offs:
- `Project.Check` grows a second drift path for a non-template, generated-from-frontmatter
  artifact **and** an exclusion in its generic lock-iteration loop so the `ACTIVE.md` entry is
  not mis-flagged `orphaned`/`stale`. Bounded to `ACTIVE.md`; mitigated by tests covering
  stale / hand-edited / missing / zero-ADR cases plus a synced-clean case asserting no false
  drift. `Project.Sync` now does I/O against the decisions dir on every run.
- A new render-data namespace (`.layout`) is a contract with consumers: `Project.data`
  (producer), every skill/agent template that references a doc path (migrated from `.vars.*`),
  and `ScaffoldConfig`/`ReferencedVars` (which must continue to ignore it). Enumerated and
  bounded.
- Template churn: every `.vars.{adrDir,plansDir,activeMdPath,adrReadme,adrTemplatePath}`
  reference across the skill/agent templates **and `agents-doc/AGENTS.md.tmpl`** migrates to
  `.layout.*` (`agents-doc`'s `{{ with }}` guards collapse to unconditional output);
  `planTemplatePath` and its `{{ if .vars.planTemplatePath }}` section in `writing-plans` are
  removed outright (Decision item 2). The golden/spine/project tests that assert those
  rendered paths or seed those vars update in lockstep.

Doc-currency obligations the implementing commit(s) must satisfy:
- `docs/decisions/README.md` "ACTIVE.md" section is rewritten to describe sync-driven
  generation (no `go test ./internal/adrtools/`).
- The agent `docCurrencyItems` strings in `.claude/awf.yaml` ("regenerated via
  `go test ./internal/adrtools/`") update to the sync mechanism, and `adr-reviewer` /
  `code-reviewer` re-render.
- `AGENTS.md`'s ACTIVE.md invariant ("`docs/decisions/ACTIVE.md` is generated — never
  hand-edited") stays true; any reference to the regeneration command updates with the
  re-render.
- When this ADR flips to Accepted/Implemented, the same commit regenerates `ACTIVE.md` —
  by then via the new `sync` path. No `docs/decisions/README.md` index row is owed (this
  repo's README is a how-to guide; `ACTIVE.md` is the generated index — per ADR-0003/0004).

Downstream work unblocked: (1) an implementation plan covering the `docsDir` field + default,
the `.layout` namespace + template migration, the `sync` generation step, the `check`
staleness path, the removals, and the dogfood re-sync, with tests at each step; and (2)
ADR-0006, which introduces the `// invariant:` test-tagging convention and its checker and
retro-applies tagged tests to this ADR's Invariants.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Standalone `awf adr` subcommand | Redundant with the generate/verify cycle `sync`/`check` already own; forces adopters to remember a separate command and a separate hook step. Folding into sync/check is the user's stated preference. |
| Keep deriving the paths but inject them under `.vars` | `ReferencedVars` would still collect them as empty scaffold slots, and an adopter could override a layout path via `vars` — contradicting "strictly awf-given". A separate `.layout` namespace dissolves both problems. |
| Keep the six per-area path vars configurable | Six knobs that can drift against each other and the hardcoded managed-docs prefix; plans/ADRs are foundational, so their location should be one root plus fixed structure, not free-form config. |
| Make `ACTIVE.md` a pseudo-template in the manifest | It is generated from ADR frontmatter, not a template render; routing it through `TemplateHash` drift cannot detect frontmatter changes, which is exactly the staleness signal that matters. |
| Gate `ACTIVE.md` generation behind an opt-in `docs:` entry | ADRs/plans are foundational and effectively always present; generation should key off the presence of ADR files, not a config toggle (user: "they should always be enabled"). |
| Make `docsDir` required (no default) | Would force every existing `awf.yaml` — including the dogfood — to add the key before `sync`/`check` work. Defaulting to `"docs"` preserves backward compatibility and keeps the change additive. |
| Combine the `// invariant:` convention into this ADR | The convention is a general workflow rule applying to every ADR, not coupled to docsDir; one-decision-per-ADR keeps it as ADR-0006 (user-selected). |
