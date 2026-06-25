---
status: Proposed
date: 2026-06-25
supersedes: []
superseded_by: ""
tags: [tooling, schema, workflow]
related: [0007]
---
# ADR-0008: Language-Agnostic Invariant Backing and a Polyglot Standard

## Context

awf's purpose is to scaffold and drift-check a standard agentic workflow into **any** dev
project, not only Go. A Go-coupling audit found the architecture already ~85% language-agnostic:
the render/sync/check engine, the config/catalog/manifest/frontmatter/adr packages, scaffolding
(`ScaffoldConfig` seeds referenced vars empty — no Go defaults), the git hooks, and most
templates are parameterized through `testCmd`/`gateCmd`/`checkCmd`. The load-bearing Go-coupling
is the **invariant checker** introduced by ADR-0007: `internal/invariants` scans only `*.go`
files for a `// invariant:` (C-style) comment. A Python/TypeScript/Rust adopter therefore cannot
back any ADR invariant, so `awf check` (run by the pre-commit hook) would fail indefinitely — the
feature is Go-only. Secondary Go-isms: the `refactor-coupling-audit` skill's hardcoded Go `grep`
procedure (`--include='*.go'`, method-receiver regex, `go:generate`), the `// invariant:` marker
prose in the `proposing-adr` skill and `docs/decisions/template.md`, and the agent-guide identity
that reads "into any **Go** project".

Grounding discoveries (verified against source):

- **A pointer config struct fits the strict loader.** `config.Load` decodes with
  `KnownFields(true)` (`internal/config/config.go:44`); a new `Invariants *InvariantConfig` field
  decodes to `nil` when absent, and `Validate` already uses the `if c.AgentsDoc != nil` nil-guard
  pattern (`config.go:82`).
- **`filepath.Match` matches a basename pattern** (`*.go`) but **not** `**` — so globs are
  filename patterns over a recursive walk, needing no new dependency. A malformed pattern returns
  an error, so it can be rejected in `Validate`.
- **Scanner call sites are bounded:** `internal/invariants.Check(decisionsDir, root)` is called by
  `Project.CheckInvariants` (`internal/project/project.go`), which is called by `runCheck`
  (`cmd/awf/check.go`) and `runInvariants` (`cmd/awf/invariants.go`). `runCheck` already returns
  non-zero when any finding exists — so an "unchecked" finding makes `awf check` fail with no new
  wiring.
- **`cfgHash` already covers `awf.yaml`** (`manifest.Hash(p.Cfg.Raw())`), so adding/editing the
  `invariants` block (and the identity string) re-syncs the lock normally.
- **`ScaffoldConfig` won't emit `invariants`** (it only emits referenced `.vars.X`), so a fresh
  `awf init` yields `invariants: nil` → unchecked. A project with **zero** `inv:` slugs is clean
  under any config (the required set is empty), so non-ADR / not-yet-tagged adopters are unaffected.

**User constraints driving the design (verbatim):** "it should scaffold any dev project, not only
Go, so it should be generic." "Config driven, but … an array of globs and marker, since different
language files can have different markers." "I don't think we need regex support, a marker string
alone should be enough." "If the config is empty, we should just mark all invariants as unchecked.
Disabling it must be explicit. By default invariant tests should be enforced." "Untestable
invariants are a separate category … they fall out of the invariant roster." "update our CLAUDE.md
… make clear that this project is a generic agentic development workflow application which supplies
a default way of setting up things and the tooling to enforce parts of it."

## Decision

1. **Add an `invariants` config block.** `Config` gains `Invariants *InvariantConfig`
   (`yaml:"invariants"`; pointer, so absent ⇒ `nil`):
   - `InvariantConfig{ Disabled bool \`yaml:"disabled"\`; Sources []InvariantSource \`yaml:"sources"\` }`
   - `InvariantSource{ Globs []string \`yaml:"globs"\`; Marker string \`yaml:"marker"\` }`

   `Globs` are **filename (basename) patterns** matched with `filepath.Match` over a recursive walk
   (`*.go`, `*.py`, `*_test.rb`) — no `**`, no new dependency. `Marker` is a **literal string**
   (e.g. `//`, `#`, `--`), never interpreted as a regex. `Config.Validate` rejects a glob that
   `filepath.Match` reports malformed.

2. **Three enforcement states.** The required invariant set is every `inv: <slug>` tag in the
   Invariants section of an `Implemented` ADR (unchanged from ADR-0007; language-neutral). Given a
   non-empty required set:
   - **Unchecked** — `Invariants` is `nil`, or present but `Disabled == false` with no usable
     `Sources`: every required slug is reported `unchecked` and `awf check` / `awf invariants` is
     **non-clean**, advising the adopter to configure `invariants.sources` or set
     `invariants.disabled: true`. Not configuring is never a silent pass — enforcement is the
     default posture.
   - **Disabled** — `Invariants.Disabled == true`: invariant backing is skipped and clean. This is
     the **only** way to opt out, and it is explicit (Disabled wins even if `Sources` is set).
   - **Enforced** — `Sources` non-empty (and not disabled): each required slug must be backed.
   - A project whose `Implemented` ADRs declare no `inv:` slugs is clean under any/empty config.

3. **Generalize the scanner** (`internal/invariants`). `Check` takes the `*InvariantConfig`; it
   walks `root` (skipping `.git`/`vendor`/`node_modules`), matches each file's basename against
   each source's `Globs`, and detects backing by locating the literal `Marker` in a line and then
   extracting `invariant: <slug>` (`slug = [a-z0-9-]+`) after it — marker by string search, slug by
   an internal regex; the marker is never compiled as a regex. `Finding` gains a status
   (`unbacked` | `unchecked`). **This revises ADR-0007 Decision item 3** (the `.go`/`//`-only scan)
   via partial-item supersedence: ADR-0007 stays `Implemented`/live and this ADR carries
   `related: [0007]`; ADR-0007 is **not** flipped to `Superseded`.

4. **Untestable invariants stay untagged.** An invariant that cannot be exercised by a test simply
   carries no `inv:` slug — it remains prose, outside the enforced roster. Every tagged slug is
   enforced; there is no per-slug exemption mechanism.

5. **De-Go-ify the remaining standard surfaces.** Rewrite the `refactor-coupling-audit` skill's
   hardcoded Go `grep` procedure into language-neutral guidance (the coupling categories plus
   "search your project's source with language-appropriate patterns"; Go shown as one example, not
   the assumption). Neutralize the `// invariant:` marker prose in the `proposing-adr` skill and
   `docs/decisions/template.md` to reference the project's configured marker, with per-language
   examples (`//` Go/Rust/TS, `#` Python/Ruby/shell).

6. **Reposition the identity as polyglot.** Update `agentsDoc.data.identity` in `.claude/awf.yaml`
   (→ rendered `AGENTS.md`) and `README.md`: awf is a **generic agentic-development-workflow
   application** that supplies a default project setup and the tooling to enforce parts of it
   (drift, frontmatter, invariant backing), rendering into **any** project; the awf tool itself is
   a Go binary (module `agentic-workflows`, Go 1.26) but the standard it renders is
   language-agnostic. (The project uses `AGENTS.md` as its agent guide; there is no `CLAUDE.md`.)

Applying this to awf's own repo — adding `invariants: { sources: [{globs: ["*.go"], marker: "//"}]
}` to `.claude/awf.yaml` so the repo's own `inv:` slugs (ADRs 0005–0008) stay enforced/backed, and
re-syncing — is adopter/mechanical work in the plan, not a Decision commitment. This earns an ADR
because it is load-bearing (new `awf.yaml` schema key, scanner redesign, revision of ADR-0007, and
a workflow/identity change) and a plan because it is multi-commit.

## Invariants

Checkable contracts, tagged per the convention (each backed by an `internal/invariants` /
`internal/config` test added at implementation):

- `inv: invariants-three-state` — with a non-empty required set: a `nil` `Invariants` (or present
  with no sources and not disabled) reports every slug `unchecked`; `Disabled: true` reports
  nothing (clean); non-empty `Sources` reports only unbacked slugs.
- `inv: invariants-multilang-scan` — a slug backed by `<marker> invariant: <slug>` in a file whose
  basename matches a configured glob is detected as backed for a non-`//`/non-`.go` pair
  (e.g. `marker: "#"`, `globs: ["*.py"]`).
- `inv: invariants-marker-literal` — the marker is matched as a literal string; a marker containing
  regex metacharacters is not interpreted as a regex.
- `inv: invariants-glob-basename` — globs match file basenames via `filepath.Match` (`*.go` matches
  a nested file), and `Config.Validate` rejects a malformed glob pattern.
- `inv: invariants-zero-slugs-clean` — a project whose `Implemented` ADRs declare no `inv:` slugs is
  clean regardless of the `invariants` config (including absent).

## Consequences

Easier:
- Invariant backing works for any language an adopter declares (`{globs, marker}` per language), so
  the standard is genuinely polyglot rather than Go-only.
- Enforcement is the default: an adopter cannot silently leave tagged invariants unverified — they
  must configure sources or explicitly disable.
- The standard's remaining Go-isms (refactor-audit procedure, marker prose, identity) are removed,
  matching the "any dev project" positioning.

Harder / accepted trade-offs:
- New `awf.yaml` schema surface (`invariants` block) and a more complex scanner (multi-source,
  basename globs, status-bearing findings).
- A fresh adopter who tags an invariant before configuring `invariants` gets a failing `awf check`
  until they configure sources or disable — intended friction ("enforce by default; disable must be
  explicit"); projects with no `inv:` tags are unaffected.
- The `refactor-coupling-audit` skill becomes language-neutral guidance rather than ready-to-run Go
  commands; a Go user gets one less copy-paste convenience (Go remains an illustrative example).

Doc-currency obligations the implementing commit(s) must satisfy:
- `proposing-adr` skill, `docs/decisions/template.md`, and `refactor-coupling-audit` skill are
  de-Go-ified; `agentsDoc.data.identity` and `README.md` carry the polyglot positioning; all
  re-rendered via `./x sync`.
- This repo's `.claude/awf.yaml` gains the `invariants` block so ADRs 0005–0008 `inv:` slugs stay
  enforced.
- When this ADR flips to Implemented, `./x sync` regenerates `ACTIVE.md`. No
  `docs/decisions/README.md` index row is owed (this repo's README is a how-to guide; `ACTIVE.md`
  is the generated index — ADR-0003/0004).

Downstream unblocked: the module-path rename (`agentic-workflows` → the real import path) and the
publish can proceed positioning awf as a polyglot tool, not a Go-only one.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Zero-config, comment-style-agnostic marker scan (match any common comment leader, no config) | Simpler for adopters but the user chose explicit config; an array of `{globs, marker}` is predictable, scopes scanning, and avoids matching `invariant:` in non-comment prose. |
| Single global `marker`/`glob` (not an array) | Polyglot repos mix comment styles (`//` and `#`); a per-language array is required. |
| Regex markers in config | Unneeded complexity; a literal marker string plus an internal slug regex suffices (user constraint). |
| Absent config ⇒ silently skip / pass | Lets tagged invariants rot unverified; the user requires enforcement by default with explicit disabling, hence the flagged `unchecked` state. |
| Per-slug "exempt/untestable" marker in ADRs | Untestable invariants simply stay untagged (out of the roster); no exemption mechanism needed (user). |
| Keep `refactor-coupling-audit` Go-specific (or mark it Go-only/opt-in) | Contradicts the polyglot goal and the chosen "everything now" scope; genericized instead. |
| Split into separate ADRs (scanner / prose / identity) | These serve one coherent decision — "awf is language-agnostic; invariant backing is language-configurable" — so one ADR + one plan (user chose a single sweep). |
| Add a `CLAUDE.md` for the positioning | The project standardized on `AGENTS.md` as its agent guide; positioning lives there (via `agentsDoc.data.identity`) and in `README.md`. |
