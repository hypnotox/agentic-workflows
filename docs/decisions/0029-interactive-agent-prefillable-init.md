---
status: Implemented
date: 2026-06-28
supersedes: []
superseded_by: ""
tags: [init, scaffold, cli, tooling, adoption]
related: [22]
domains: [tooling, config]
---
# ADR-0029: Interactive and Agent-Prefillable `awf init`

## Context

`project.ScaffoldConfig` (internal/project/scaffold.go) generates the `.awf/config.yaml` that
`awf init` writes. It discovers every `{{ .vars.X }}` name referenced by any catalog template (via
`render.ReferencedVars`) and seeds each to the empty string. A fresh adopter therefore inherits a
config with ~11 blank vars (`gateCmd`, `checkCmd`, `testCmd`, `commitScope`, `modulePrefix`, …) and
no guidance on what any of them mean or should hold. The downstream effects are concrete: the
rendered AGENTS.md Commands section is empty, the Workflow section drops its gate line, and the
pre-commit hook runs only `awf check` (no test gate) until the adopter discovers and sets `gateCmd`
by hand. The scaffold is correct but inert.

The first real adopters are the maintainer's own repositories, and onboarding should be **agentic**:
point an agent at a target repo and have it adopt awf **out of the box** — without cloning or
reading the awf repository itself. That constraint means the `awf` binary must *self-describe* the
values it can fill; guidance that lives only in the awf repo's docs or skills fails it.

Two facts block both a human prompt and an agent option-set today: vars carry **no metadata** (they
are bare names scraped from templates — no description, default, or option list), and `awf init` has
no mechanism to receive values. ADR-0022 weighed "interactive per-skill prompts at init" and set it
aside to keep init **non-interactive and scriptable**. This ADR revisits that narrowly: it keeps the
non-interactive path deterministic and prompt-free, adds interactivity only when stdin is a terminal,
plus a fully non-interactive path for agents and scripts — so the scriptability the rejection
protected is preserved (the silent non-interactive output stays byte-identical to today; descriptor
defaults pre-fill prompts and appear in `--describe`, but are never auto-applied on the silent path —
see Invariants). ADR-0022's curated-core Decision is unaffected; only its rejection of *any*
interactivity is superseded in reasoning (`related`, not `supersedes`).

Grounding-check confirmations against the codebase: `ScaffoldConfig` seeds via
`render.ReferencedVars` and an empty string per var (scaffold.go); `catalog.Load` is a non-strict
decode, so a new top-level catalog block is backward-safe; filling the skeleton changes no YAML
shape, so no config-tree schema bump or `awf upgrade` is needed; and TTY detection is available in
the standard library (`os.Stdin.Stat()` + `os.ModeCharDevice`), so no new dependency is required.

## Decision

1. **Value descriptors live in the catalog.** Add a top-level, ordered `vars:` block to
   `templates/catalog.yaml`. Each descriptor is `{ key, kind, description, default, options }` where
   `kind` ∈ `{ string, enum, multiselect }`: `string` is free text (`options` are suggestions),
   `enum` picks one of `options`, `multiselect` picks a subset. The invariants marker/globs are
   modelled as descriptors with language presets in `options`; catalog trim (which skills/docs to
   enable) is a `multiselect` descriptor whose options derive from the catalog itself. `catalog.Load`
   parses the block into a new catalog type; the non-strict decode keeps older configs and the
   embedded catalog backward-safe.

2. **Three resolution modes share one descriptor set.**
   - **Interactive** — when stdin is a TTY and no answers are supplied, `awf init` walks the
     descriptors as prompts, each pre-filled with its `default`. A `default` is therefore a prompt
     suggestion, not an auto-applied value.
   - **Silent (non-interactive)** — when stdin is not a TTY and no answers are supplied, each var is
     seeded empty, with no prompting and no default applied — byte-identical to today's seed-empty
     output. This is the path CI and scripts hit, and it stays deterministic and prompt-free.
     Descriptor defaults do not affect it; an agent or script that wants those values passes them via
     `--describe` + `--answers`.
   - **Explicit answers** — `--set key=value` (repeatable) and `--answers <file.json|yaml>` supply
     values in any TTY state. Provided keys are used verbatim and skip prompting; unprovided keys
     fall back to interactive-or-default per the two modes above. Explicit answers always win.

3. **`awf init --describe` is the agent entrypoint.** It emits the descriptor set as JSON to stdout
   and writes nothing under the target root (read-only). An agent reads this schema — shipped inside
   the binary, so no repo clone — chooses values, and runs `awf init --answers`/`--set`. This is what
   makes onboarding out-of-the-box agentic.

4. **`ScaffoldConfig` takes resolved inputs, not just a prefix.** Its signature changes from
   `ScaffoldConfig(prefix)` to accept resolved inputs (prefix, a `map[string]string` of var values,
   the optional catalog-trim selection, and the optional invariants config). It seeds each var from
   the map (empty when absent), applies any catalog trim over the ADR-0022 curated-core default, and
   writes the chosen invariants marker/globs. The completeness guarantee of ADR-0022
   (`scaffold-seeds-all-vars`) is preserved: every referenced var still appears, now carrying a value
   rather than `""`. ADR-0022's `scaffold-core-only` guarantee is likewise preserved on the default
   path: with no catalog-trim selection, `ScaffoldConfig` still enables exactly the curated core — its
   backing test calls `ScaffoldConfig` with no trim and is unaffected. A trim selection is
   **full-deselectable**: it replaces the enable array verbatim and may drop curated-core targets,
   mirroring `awf remove` (ADR-0024), which already disables core/chain skills post-init with no
   guardrail. The default (no-selection) path is unchanged, so `scaffold-core-only` holds.

5. **Descriptor↔var parity is gated.** A test asserts that every var returned by
   `render.ReferencedVars` over the catalog templates has a matching descriptor in
   `templates/catalog.yaml`, and that no descriptor names a var absent from every template. This is
   backed by a tagged invariant so the descriptor set cannot silently drift from the templates.
   (`prompt: false` is reserved as a future seed-empty-never-prompt affordance; every current var has a
   descriptor, so it is unimplemented.)

6. **Scope boundary: values, not prose.** This ADR covers fillable *values* — vars, the invariants
   marker/globs, and catalog trim. It does **not** cover free-text authoring (the `identity`,
   `you-and-this-project`, and project-invariant narratives), which has no option-set and stays
   manual convention-part authoring guided by `agents-md-standard.md`. An onboarding agent writes
   that prose as open work after init.

## Invariants

- `inv: var-descriptor-parity` — every var referenced by any catalog template has a matching
  descriptor in `templates/catalog.yaml`, and no descriptor names a var that appears in no template.
- `inv: catalog-trim-applied` — a non-nil catalog-trim dimension passed to `ScaffoldConfig` replaces
  the curated-core skills/docs enable array verbatim (the full-deselectable trim); a nil dimension
  keeps exactly the core.
- `inv: init-noninteractive-default` — `awf init` with a non-TTY stdin and no `--set`/`--answers`
  seeds every var empty (no descriptor default applied), byte-identical to the pre-feature seed-empty
  output, preserving scriptable init.
- `inv: describe-read-only` — `awf init --describe` writes no file under the target root and emits
  the descriptor set as valid JSON on stdout.
- `inv: explicit-answers-win` — a value supplied via `--set` or `--answers` is used verbatim and
  suppresses prompting for that key, overriding the descriptor default regardless of TTY state.

## Consequences

- **Out-of-the-box agentic onboarding.** An agent installs awf, runs `awf init --describe`, decides
  values, and runs `awf init --answers` — no clone of the awf repo, no hand-editing generated
  config. Humans on a terminal get guided prompts; CI and scripts stay non-interactive and
  deterministic — the silent path never prompts and its output is byte-identical to today's
  seed-empty output, since descriptor defaults pre-fill prompts only and are never applied silently.
- **No config-tree schema bump.** `vars` descriptors live in the embedded `templates/catalog.yaml`
  (non-strict decode), and the generated `config.yaml` keeps its shape. `migrate.Current()` is
  unchanged; existing adopters need no `awf upgrade`, and the schema-version gate in `awf check` is
  unaffected. The `vars` block is catalog metadata that renders to no file, so the lock/manifest
  hashes are unaffected as well — the config, render, manifest, and migrate consumers all see
  unchanged shapes.
- **New surfaces to maintain.** The catalog gains a `vars` block (kept honest by
  `var-descriptor-parity`); the CLI gains `--describe`, `--set`, and `--answers`, which requires the
  flag parser to support a repeatable value flag (an implementation detail for the plan). TTY
  detection uses the standard library; whether the interactive prompter — notably the `multiselect`
  widget — can stay pure-stdlib or needs a small TUI dependency is left to the plan. The
  non-interactive, `--describe`, and `--set`/`--answers` paths add no dependency regardless.
- **ADR-0022 reasoning revisited, not its decision.** The curated-core default stands; interactivity
  is additive and TTY-gated, so the "breaks scriptable init" objection no longer applies. Recorded
  as `related`, not a supersede.
- **Prose stays out of scope** (Decision 6), so init does not become an authoring tool — it fills
  the option-shaped slots and leaves narrative authoring to the agent/human.
- **Complexity warrants a plan.** Implementation spans catalog descriptors + parsing + validation,
  the three resolution modes, the new flags and `--describe`, the `ScaffoldConfig` signature change,
  and the interactive prompter — multi-commit and interdependent. This ADR hands off to
  `awf-writing-plans` after review.

Doc-currency obligations the implementing commit(s) must satisfy:

- The `tooling` domain narrative gains init's interactive / `--describe` / explicit-answer modes.
- The `config` domain narrative gains the catalog `vars` descriptor block.
- The README/AGENTS.md command surface mentions `--describe`/`--set`/`--answers` where init is
  documented.
- The status flip to `Implemented` regenerates `docs/decisions/ACTIVE.md` via `./x sync`.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Auto-detect values from the target repo (e.g. go.mod → testCmd) | Brittle and guess-prone for the first cut; descriptors plus explicit answers are deterministic. Detection can layer on later without re-deciding this. |
| Flags only, no `--describe` | An agent then has no machine-readable option list; the self-describing schema is precisely what enables out-of-the-box agentic onboarding. |
| Prefill-then-edit (init writes commented placeholders; agent edits the file) | No interactive protocol; pushes config-format parsing onto the agent and re-introduces hand-editing of generated config, which the awf model forbids. |
| Descriptors in a separate `init-questions.yaml` | Splits metadata from the catalog that already owns target declarations and parity tests; one catalog stays the single source of truth. |
| Descriptors as Go struct literals | Not data-driven; harder for non-Go contributors to edit and keeps descriptors away from the YAML catalog entries they describe. |
| Ship onboarding as a doc or skill in the awf repo | Requires the agent to read the awf repository (clone/fetch), violating the out-of-the-box constraint. The binary must self-describe. |
