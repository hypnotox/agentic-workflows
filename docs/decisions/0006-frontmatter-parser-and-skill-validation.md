---
status: Implemented
date: 2026-06-25
supersedes: []
superseded_by: ""
tags: [tooling, render]
related: [0001, 0005]
domains: [rendering, adr-system]
---
# ADR-0006: Shared Frontmatter Parser and Rendered Skill/Agent Frontmatter Validation

## Context

The render pipeline emits skill and agent markdown whose leading YAML frontmatter
(`name`, `description`) is what Claude Code parses to discover and load them. Nothing in
`awf` validates that the *rendered* frontmatter is well-formed or carries a non-empty
`name`/`description`. The only safety nets are the `<no value>` guard at render time
(`internal/project/project.go` `renderTemplate`, which only catches an unset interpolated
var) and hash-drift in `Check` — and neither catches structurally-broken or missing
frontmatter, because a broken-but-self-consistent render still matches its own lock hash, so
drift-check stays green. A template bug (a stray edit to a `SKILL.md.tmpl` header, a
mis-guarded conditional) could therefore ship a skill that Claude Code silently fails to
parse, with no signal from `awf sync` or `awf check`.

Separately, ADR frontmatter parsing lives inline in `internal/adrtools`
(`parseFrontmatterAndTitle`), using magic-offset arithmetic (`content[3:end+3]`,
`content[end+7:]`), LF-only, parsing just `status`. Introducing a second consumer of the
same `---`-delimited split (skill/agent validation) must not duplicate that parse — the
project requires one tested location for the concern.

Grounding discoveries that shape the design (verified against source):

- **`internal/project/project.go` is the only non-test caller of `adrtools.GenerateActiveMD`**
  (import at `project.go:12`, call at `project.go:299`), so the rename blast radius is bounded:
  that import + call, the package/test files, and the `doc-architecture.md` part (which
  re-renders `docs/architecture.md`).
- **Skills carry `name: {{ .prefix }}-<name>`; agents carry unprefixed `name: <name>`; both
  carry `description:`.** Hooks (`templates/hooks/*.tmpl`), docs (`templates/docs/*.tmpl`), and
  `templates/agents-doc/AGENTS.md.tmpl` have **no** frontmatter — so validation scopes to
  `skills/` and `agents/` outputs, and the rendered file's kind is known from its template-id
  prefix (`skills/`, `agents/`) in the `RenderAll` loops (`project.go:217-237`).
- **`manifest.Drift.Kind` is a free string** (`internal/manifest/manifest.go:24`), never
  type-switched, so a new `invalid-frontmatter` kind is purely additive.
- **`gopkg.in/yaml.v3` is a direct dependency**, usable by a new `internal/frontmatter` package.
- **Every current skill/agent description renders non-empty** under existing configs (the
  conditionals/ranges in `adr-lifecycle` and others all have non-empty fallbacks), so the
  validation contract will not spuriously fail the existing template set. The contract checks
  *emptiness*, not *quality*: a description that interpolates an empty var but retains surrounding
  literal text (e.g. `roadmap-graduation` renders `Use when a  entry is becoming…` when
  `vars.roadmapDoc` is `""` in this repo) still passes — it is non-empty and Claude-Code-parseable.
  Catching such degraded-but-non-empty renders is out of scope; the `<no value>` guard already
  catches the wholly-unset case.
- The existing magic-offset splitter is LF-only and fragile; a generic primitive replaces it.

**User constraints driving the design (verbatim intent):** "does this mean we currently don't
validate the frontmatter of the skills, where it's paramount that it is the correct format for
Claude Code to work"; "We should generalise it to the point that we can also use it for that,
and make check validate the skills as well"; "testing that our templates produce correct
frontmatters for that is crucial"; "clean code is paramount, and a parser for the same concern
shouldn't be duplicated. It should be at one location and fully tested."

## Decision

1. **Add a single `internal/frontmatter` package** — the one tested primitive for
   `---`-delimited YAML frontmatter. API:
   - `Split(content []byte) (yaml []byte, body []byte, found bool)` — returns `found == false`
     and the original content as `body` when there is no leading `---` block; otherwise the YAML
     block and the body after the closing `---` line.
   - `Parse(content []byte, out any) (body []byte, found bool, err error)` — `Split` then
     `yaml.Unmarshal` the YAML block into `out`, returning the body. YAML unmarshalling complexity
     stays inside the package; callers pass a destination struct.

   All `---`-frontmatter parsing in non-test code routes through this package.

2. **Rename `internal/adrtools` → `internal/adr`, refactored onto `internal/frontmatter`.**
   Exposes `ParseDir(dir) ([]ADR, error)` with `ADR{Number, Title, Status, Filename, Path}`, and
   `RenderActiveMD(dir) (string, error)` (replaces `GenerateActiveMD`; returns `""` for zero
   ADRs). The inline `parseFrontmatterAndTitle` magic-offset logic is deleted in favour of
   `frontmatter.Parse`. `internal/project` updates its import and call site to
   `adr.RenderActiveMD`; the package comment and the `.claude/awf/parts/doc-architecture.md` part
   repoint. ADR *section-body* parsing is intentionally **not** added here — it is deferred to
   the invariant-tooling ADR, which extends `internal/adr`.

3. **Define the rendered skill/agent frontmatter contract.** A validator in `internal/project`
   (using `internal/frontmatter`) requires, for every rendered skill and agent output:
   frontmatter present and YAML-parseable, with a non-empty `name` and a non-empty `description`.
   Scope is keyed by template-id prefix (`skills/`, `agents/`); hooks, docs, and `AGENTS.md` are
   exempt.

4. **Enforce the contract at `sync` and `check`.** `Sync` validates each rendered skill/agent
   and returns an error **before writing any file** when frontmatter is invalid (a template bug
   never reaches disk). `Check` re-parses each on-disk skill/agent file's frontmatter and emits a
   new `invalid-frontmatter` drift entry when it is missing, unparseable, or has an empty
   `name`/`description` — so `awf check` (and the pre-commit hook that runs it) guards the
   contract on the committed tree. The frontmatter check participates in `Check`'s existing
   one-drift-per-path loop and is subordinate to the hash-based kinds: a skill/agent that is
   already `stale`, `orphaned`, `missing`, or `hand-edited` reports that kind (a re-sync is the
   actionable fix and will re-validate); `invalid-frontmatter` is reported only for a file that is
   otherwise in sync yet carries broken frontmatter (e.g. a lock baked from a pre-validation
   template).

5. **Add golden coverage for template frontmatter.** A test renders every catalog skill and
   agent template with representative data and asserts the frontmatter parses with non-empty
   `name`/`description`, proving the standard's own templates are Claude-Code-parseable.

Applying this to `awf`'s own repo (re-sync, the rename ripple) is mechanical adopter work — the
plan's tasks — not a Decision commitment. This change earns an ADR because it is load-bearing
(new `internal/frontmatter` package boundary, a package rename, new `sync`/`check` semantics and
a new drift kind) and a plan because it is multi-commit.

## Invariants

Checkable contracts that must hold while this decision stands. (The `// invariant:` test-tagging
convention and its checker are ADR-0007; for now these are textual contracts verified by the
implementation plan's tests.)

- `internal/frontmatter` is the only non-test code that splits `---`-delimited frontmatter;
  `internal/adr` and the skill/agent validator both call it, and the former
  `parseFrontmatterAndTitle` magic-offset splitter no longer exists.
- `inv: frontmatter-split` — `frontmatter.Split` on content without a leading `---` returns `found == false` and the
  original content as `body`; on well-formed frontmatter it returns the YAML block and the body
  following the closing `---` line.
- `inv: render-active-md` — `internal/adr.RenderActiveMD` produces output byte-identical to the pre-rename generator for
  the same decisions directory (this repo's `ACTIVE.md` is unchanged by the refactor) and returns
  `""` when the directory holds no ADRs.
- `awf sync` returns an error and writes nothing when any rendered skill or agent output has
  missing or unparseable frontmatter, or an empty `name` or `description`.
- `inv: check-invalid-frontmatter` — `awf check` reports an `invalid-frontmatter` drift entry for an on-disk skill/agent file that is
  otherwise in sync (not `stale`/`orphaned`/`missing`/`hand-edited`) but whose frontmatter is
  missing, unparseable, or has an empty `name`/`description`; a clean synced tree reports no such
  entry, and at most one drift entry is reported per path.
- Frontmatter validation applies to skill and agent outputs only; rendering hooks, docs, or
  `AGENTS.md` never triggers it.
- `inv: templates-valid-frontmatter` — Every catalog skill and agent template, rendered with representative data, produces frontmatter
  that parses with a non-empty `name` and a non-empty `description`.

## Consequences

Easier:
- Claude-Code-parseable skill/agent frontmatter is guaranteed by construction at `sync` and
  guarded at `check`; a template bug that breaks a skill's frontmatter fails fast instead of
  silently shipping an unloadable skill.
- One tested `---`-parser replaces the fragile magic-offset ADR splitter; `internal/adr` becomes
  a clean ADR-domain home that the invariant-tooling ADR extends (adding section parsing) rather
  than re-implementing.

Harder / accepted trade-offs:
- A new package boundary (`internal/frontmatter`) and a package rename (`adrtools` → `adr`).
  Bounded blast radius: the `internal/project` import + call site, the package/test files, and
  the architecture-doc part. The frozen ADR-0005 and its plan keep their historical
  `internal/adrtools` references (append-only records — not edited).
- `Sync` gains a validation pass over skill/agent outputs and `Check` a per-file frontmatter
  parse; cost is negligible (a few dozen small files).
- The `check` drift vocabulary grows one kind (`invalid-frontmatter`).

Doc-currency obligations the implementing commit(s) must satisfy:
- `.claude/awf/parts/doc-architecture.md` repoints `internal/adrtools` → `internal/adr` and notes
  `internal/frontmatter`; `docs/architecture.md` re-renders.
- This repo's `AGENTS.md` gains an invariant — rendered skills/agents carry valid
  `name`/`description` frontmatter, which `awf check` validates — via the `agentsDoc.data`
  invariants in `.claude/awf.yaml`.
- When this ADR flips to Implemented, `./x sync` regenerates `ACTIVE.md`. No
  `docs/decisions/README.md` index row is owed (this repo's README is a how-to guide; `ACTIVE.md`
  is the generated index — per ADR-0003/0004).

Downstream work unblocked: ADR-0007 (invariant-backing tooling) reuses `internal/frontmatter` and
extends `internal/adr` with Invariants-section parsing, rather than introducing its own parser.

## Alternatives Considered

| Alternative | Why not chosen |
|---|---|
| Validate frontmatter only in golden tests, not in `sync`/`check` | Tests catch template bugs in CI but not a hand-edit or a stale lock baked from a pre-validation template; the user explicitly wants `awf check` to validate. Enforcing at `sync`+`check` makes the contract hold for every adopter, not just awf's own CI. |
| Keep the inline ADR frontmatter parser; add a separate splitter for skills | Duplicates the same `---`-split concern in two places — exactly what "one tested location" rules out. |
| Don't rename `adrtools`; add exported parse funcs to it | Skill validation would import a package named/scoped to ADRs for a generic concern. The generic split belongs in `internal/frontmatter`; `adrtools`'s ADR-specific role reads clearer as `internal/adr`. |
| Validate inside `renderTemplate` only | `renderTemplate` is per-file and lacks clean skill/agent-kind context; the `RenderAll` skill/agent loops know the kind, and `Check` still needs an independent on-disk pass. |
| Combine with the invariant tooling in one ADR | Three decisions in one record; validation-first was chosen as its own ADR to honour one-decision-per-ADR and land the Claude-Code-correctness fix first. |
| Validate `name`/`description` from a typed catalog/config schema instead of the rendered output | The frontmatter is produced by templates, not config; validating the rendered artifact is what guarantees Claude-Code-parseability regardless of template logic. |
