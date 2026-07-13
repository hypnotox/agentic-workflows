---
date: 2026-07-13
adrs: [105, 106]
status: Proposed
---
# Plan: Enforced Test-Backing and Two-Marker Mechanism

Builds the ADR-0105 backing-model redesign and the ADR-0106 context surfacing **additively** — the
new checker, `testGlobs` config, two-marker vocabulary, inline backed/unbacked classification, and
backed-aware context all land while awf's own `invariants.testGlobs` stays **unset** (source-only
fallback keeps the gate green). ADR-0105 and ADR-0106 stay `Proposed`; the enforcement flip and the
73-slug migration are the separate follow-up plan. Design rationale lives in ADR-0105 and ADR-0106 —
this plan does not re-argue it.

## Goal

Ship the two-marker invariant mechanism (proof `invariant:` + advisory `touches-invariant:`), the
`testGlobs` teeth config with an absent-testGlobs source-only fallback, the symmetrically-enforced
inline `invariant:`/`unbacked-invariant:` declaration classification, and backed/unbacked-aware
`awf context` — with 100% coverage and no adopter-visible breakage yet (fallback active, ADRs still
Proposed).

## Architecture summary

- **Config** (`internal/config`): `InvariantConfig` gains `TestGlobs []string`; `Validate` checks it
  as an anchored glob list; `internal/configspec` describes it; `docs/config-reference.md` regenerates.
- **Checker** (`internal/invariants`): `declRe` parses two declaration tokens with a class
  (`invariant:` = backed, `unbacked-invariant:` = unbacked); the ADR corpus is rewritten `inv:` →
  `invariant:` atomically with that regex change; `scanTags` distinguishes the proof `invariant:`
  marker (scoped to `TestGlobs`, or all sources when `TestGlobs` empty) from the advisory
  `touches-invariant:` marker and captures notes; `Check` enforces backed-requires-proof,
  unbacked-refuses-proof, unbacked-requires-verify-note, and returns advisory notes (dangling-marker,
  bare-touches) alongside hard `Finding`s.
- **Command surface** (`cmd/awf`): `runCheck`/`runInvariants` print the advisory notes through the
  existing `note:` channel and the new hard findings.
- **Context** (`internal/project`): `MarkersUnder` scans the union of `sources` + `testGlobs`,
  recognises both markers, and returns per-slug marker kind + note; `ContextFor` labels each
  governing invariant backed/unbacked and surfaces `Verify:`/touches notes on `ContextResult`.
- **Standard surfaces**: ADR template, `proposing-adr` skill, adr-readme part, AGENTS.md rules, and a
  joint changelog `[Unreleased]` entry adopt the unified token and the proof/touches + classification
  vocabulary.

## Tech stack

Go 1.26. Packages: `internal/config`, `internal/configspec`, `internal/invariants`,
`internal/project`, `cmd/awf`; config tree under `.awf/` (parts, docs), `docs/decisions/**`. Gate:
`./x gate` before every commit; `./x check` for drift. Corpus rewrite verified by `awf check` +
`declRe` unit tests.

## File structure

- **Created:** none (all changes extend existing files and their `_test.go` siblings).
- **Modified:** `internal/config/config.go`, `internal/config/config_test.go`,
  `internal/configspec/spec.go`, `docs/config-reference.md` (regenerated),
  `internal/invariants/invariants.go`, `internal/invariants/invariants_test.go`,
  `internal/project/context.go`, `internal/project/context_test.go`, `cmd/awf/check.go`,
  `cmd/awf/invariants.go` (+ their tests), every `docs/decisions/NNNN-*.md` carrying an `inv:`
  declaration (batch), `.awf/docs/decisions-template/*` (ADR template), the `proposing-adr` skill
  part, `.awf/parts/adr-readme/invariants.md`, the AGENTS.md source parts, `changelog/CHANGELOG.md`.
- **Deleted:** none.

## Phase 1 — testGlobs config surface

- [ ] **Task 1.1 — Add `TestGlobs` to `InvariantConfig` and validate it.** In
  `internal/config/config.go`, extend the struct and `Validate`:

  ```go
  type InvariantConfig struct {
  	Disabled  bool              `yaml:"disabled"`
  	Sources   []InvariantSource `yaml:"sources"`
  	TestGlobs []string          `yaml:"testGlobs"`
  }
  ```

  In `Validate` (the `if c.Invariants != nil {` block, after the `Sources` loop at
  `config.go:245`), add:

  ```go
  		for _, g := range c.Invariants.TestGlobs {
  			if err := validatePathGlob(g); err != nil {
  				return fmt.Errorf("invariants.testGlobs: %w", err)
  			}
  		}
  ```

  Update the struct doc comment to note `TestGlobs` scopes the proof marker (empty ⇒ source-only
  fallback, ADR-0105). `TestGlobs` lands as an **inert optional field within the current schema** — no
  schema-generation bump and no `minVersionBySchema` entry (ADR-0049), since an absent field degrades
  to the source-only fallback; this discharges ADR-0105 Decision item 3's delegated schema question.
  Add `internal/config/config_test.go` cases: a valid `testGlobs: ['**/*_test.go']`
  passes; a basename-only `testGlobs: ['*_test.go']` and a malformed glob each fail with the
  `invariants.testGlobs:` prefix.

- [ ] **Task 1.2 — Describe `testGlobs` in configspec and regenerate the reference.** In
  `internal/configspec/spec.go`, add an entry after the `invariants.sources[].marker` entry
  (`spec.go:139`):

  ```go
  	{
  		Path: "invariants.testGlobs", Type: "string list", Default: "none — proof markers fall back to source-glob scope",
  		Description:  "Anchored path globs (same dialect as `invariants.sources[].globs`) identifying test files. When non-empty, a proof `invariant: <slug>` marker backs an invariant only in a file matching one of these globs — backing means an executed test line. When empty or absent, backing falls back to source-glob scope (the pre-ADR-0105 behaviour). The `touches-invariant:` context marker is unaffected.",
  		Availability: "Within `invariants`; opt-in teeth for the proof marker.",
  	},
  ```

  Run `./x sync` to regenerate `docs/config-reference.md`; confirm the reflection-parity test passes
  (`go test ./internal/configspec/...`).

- [ ] **Task 1.3 — Verify and commit.** `./x gate` && `./x check` (both clean). `git add
  internal/config/config.go internal/config/config_test.go internal/configspec/spec.go
  docs/config-reference.md`. Commit: `feat(config): add invariants.testGlobs proof-scope glob`.

## Phase 2 — Checker redesign + atomic token rewrite (coupled commit)

> Coupled: changing `declRe` to parse `invariant:` and rewriting the ADR corpus's `inv:`
> declarations cannot pass the gate apart — a `declRe` that no longer matches `inv:` with the corpus
> unchanged silently empties the required set. They share one closing commit (Task 2.7).

- [ ] **Task 2.1 — Parse two declaration tokens with a class.** In
  `internal/invariants/invariants.go`, replace `declRe` (`invariants.go:58`) and give `DeclaringADRs`
  a class. Add a `Class` type (`Backed`/`Unbacked`) and change the join to carry it:

  ```go
  // declRe matches a backed (`invariant:`) or unbacked (`unbacked-invariant:`) declaration leading a
  // markdown list item. Group 1 is the optional `unbacked-` prefix, group 2 the slug.
  declRe = regexp.MustCompile("(?m)^[ \\t]*[-*][ \\t]+[`\\t ]*(unbacked-)?invariant:\\s*([a-z0-9-]+)")
  ```

  `DeclaringADRs` returns `map[string]Decl` where
  `type Decl struct { ADR string; Class Class; VerifyNote bool }` (`VerifyNote` populated in Task
  2.2); duplicate-slug and dangling-retirement logic unchanged. This changes the return type from the
  current `map[string]string`, so **both** non-test callers are updated in this phase (they compile
  against the new shape in the Phase-2 commit, Task 2.7): `Check` (`invariants.go:106`) and
  `context.go:120` — the latter currently uses the value as an ADR-filename string (`declaring[slug]`
  → `byFile[fn]` at `context.go:131/135`), so it is edited to read `.ADR` (the `.Class` label is
  consumed in Phase 3).

- [ ] **Task 2.2 — Require the `Verify:` note on unbacked declarations.** `DeclaringADRs` captures,
  for each `Unbacked` slug, the full text of its declaration bullet (extend `declRe` or scan the
  bullet's line range from the `Invariants` section already parsed by `DeclaringADRs`) and sets
  `Decl.VerifyNote = verifyRe.MatchString(bulletText)` where
  ``verifyRe = regexp.MustCompile(`(?i)\bVerify:\s*\S`)``. `VerifyNote == false` on an `Unbacked`
  slug is surfaced as a hard finding in Task 2.5. (`Backed` slugs leave `VerifyNote` unused.)

- [ ] **Task 2.3 — Split proof and touches markers in the scan.** Change `scanTags`
  (`invariants.go:154`) to take the full `*config.InvariantConfig` (for `TestGlobs`) instead of only
  `[]config.InvariantSource`, and to recognise two marker forms after the literal source marker:
  `invariant: <slug>` (proof) and `touches-invariant: <slug>[ — <note>]` (touches). Its per-file
  result is `type scanResult struct { proof map[string]bool; touches []touchMark }` with
  `type touchMark struct { Slug, Note string }` (aggregated across files into the same two
  collections). A proof slug counts toward backing only when its file matches a `TestGlobs` pattern
  **or** `TestGlobs` is empty (source-only fallback); track proof slugs seen only in non-test files
  separately so Task 2.4 can distinguish "unbacked" from "backed elsewhere". Extend `slugRe` handling
  to strip an optional trailing note (everything after the slug, trimmed) for touches markers.
  Preserve the line-leading rule (`invariants.go:203`).

- [ ] **Task 2.4 — Enforce backing with the fallback.** In `Check` (`invariants.go:101`) compute the
  proof-backing scope from `cfg.TestGlobs` (non-empty ⇒ test-glob files; empty ⇒ all source files,
  the ADR-0008 semantics). A `Backed` declared slug with no proof marker in scope is `Unbacked`
  (existing `Finding`). Keep the `Disabled`/`Unchecked` short-circuits.

- [ ] **Task 2.5 — Symmetric classification + advisory notes.** Extend `Check`'s return to
  `([]Finding, []Note, error)` (or a result struct) where `Note` is a non-failing advisory. Emit:
  - hard `Finding` — a `Backed` slug unproven (Task 2.4); an `Unbacked` slug for which a proof marker
    exists in scope ("declared unbacked but backed in source"); an `Unbacked` slug whose declaration
    lacks a `Verify:` note.
  - advisory `Note` — a proof/touches marker naming a slug no Implemented ADR declares
    (dangling-marker); a `touches-invariant:` marker with no note (bare-touches).

  `Project.CheckInvariants` (`internal/project/project.go:288`) threads the notes through.

- [ ] **Task 2.6 — Print notes in the command surface.** In `cmd/awf/check.go`, fold the invariant
  advisory notes into the existing `note:` emission (`check.go:30-33`) and keep hard findings in the
  failure count (`check.go:46-53`). Mirror in `cmd/awf/invariants.go` (`runInvariants`): print notes,
  fail only on hard findings.

- [ ] **Task 2.7 — Rewrite the ADR corpus token + all tests, verify, commit (batch + coupled).**

  **Batch — `inv:` → `invariant:` across `docs/decisions/**`.** Representative (single-backtick form):

  ```diff
  -- `inv: context-read-only` — the command entry point holds no writer dependency …
  ++ `invariant: context-read-only` — the command entry point holds no writer dependency …
  ```

  Edge (double-backtick literal form used by ADR-0007):

  ```diff
  --   ``  `inv: adr-parse` …  ``
  ++   ``  `invariant: adr-parse` …  ``
  ```

  Affected-site set: every leading-bullet `inv:` declaration under `docs/decisions/`, exactly the
  lines matched by the declRe character class (`[`\t ]*` between bullet and token — **must** include
  the space so ADR-0007's double-backtick-with-space form `` - `` `inv: <slug>` `` `` is caught):
  `grep -rnE '^[[:space:]]*[-*][[:space:]]+[`[:space:]]*inv: ' docs/decisions/` (287 lines; the
  backticks-only pattern misses ADR-0007's 5 and would silently drop those slugs). Do **not** touch
  `inv:` appearing mid-prose (non-leading), nor code/other files. (Every current declaration is
  `Backed`; no ADR converts to `unbacked-invariant:` in this plan — that is the migration plan's
  classification work.)

  Post-check (all must hold): the same declRe-class grep for remaining `inv:` prints `0`
  (`grep -rnE '^[[:space:]]*[-*][[:space:]]+[`[:space:]]*inv: ' docs/decisions/ | wc -l` → `0`); the
  **positive** count `grep -rnE '^[[:space:]]*[-*][[:space:]]+[`[:space:]]*invariant: ' docs/decisions/ | wc -l`
  equals the pre-rewrite `inv:` total (287), proving no declaration was dropped rather than converted;
  `go test ./internal/invariants/...` passes (declRe recognises `invariant:`); and `./x check` is
  clean with **no new `note:` dangling-marker lines** (every rewritten slug still declared and backed
  under source-only fallback).

  Add `internal/invariants/invariants_test.go` cases (synthetic fixtures — awf's corpus has no
  `unbacked-invariant:` yet): backed-with-proof-in-test passes; backed-without-proof fails;
  proof-in-non-test-file with `TestGlobs` set fails; same passes with `TestGlobs` empty (fallback);
  unbacked-with-`Verify` passes; unbacked-without-`Verify` fails; unbacked-with-a-proof-marker fails;
  dangling-marker and bare-touches each yield a note not a finding. (No commit yet — the authoring-doc
  updates in Task 2.8 must co-travel with the token rename in one coupled commit, Task 2.9.)

- [ ] **Task 2.8 — Update the backing-model authoring surfaces (co-travel with the token rename).**
  Because Task 2.7 makes `invariant:` the live declaration token, the surfaces that *instruct* how to
  declare an invariant must change in the same commit — else a new ADR authored against the stale
  template would use `inv:`, which `declRe` no longer parses, silently dropping its invariants. Reword,
  under `.awf/`: the ADR template Invariants guidance to show both forms — ``- `invariant: <slug>` —
  …`` (backed) and ``- `unbacked-invariant: <slug>` — …. **Verify:** …`` (unbacked); the `proposing-adr`
  skill part and `.awf/parts/adr-readme/invariants.md` to the unified `invariant:` token and the
  proof/touches marker split (per-language markers); and the AGENTS.md source part's **"Backed
  invariants"** rule for the two markers, the test-scoped proof + source-only fallback, and the
  backed/unbacked classification with symmetric enforcement. Also fix the peripheral `inv:`→`invariant:`
  token-spelling mentions in the guide sources that co-travel with the rename: `.awf/docs/glossary.yaml`
  (the invariant term), `.awf/docs/parts/architecture/components.md`, the working-with-awf source part,
  any `.awf/docs/pitfalls.yaml` entry naming the token, and the **declaration-token spelling only** in
  `.awf/domains/parts/invariants/current-state.md` (sentence 1's `inv: <slug>` → `invariant: <slug>`).
  **Do not touch the AGENTS.md `context` rules** (`context-tier1-governs` etc.) nor the *proof/touches
  model and Tier-1/context wording* in the domain current-state narratives — those stay live/enforced
  until the migration plan flips ADR-0106; see Notes. Run `./x sync`.

  **Batch — prose `(inv: <slug>)` citation prefixes → `(invariant: <slug>)`** (ADR-0105 item 1's total
  unification). Representative: in `.awf/agents-doc.yaml` a bullet ending ``… (`inv: local-doc-catalog-clone`).``
  becomes ``… (`invariant: local-doc-catalog-clone`).``. No edge variant — every citation is the same
  shape. Affected-site set: `grep -rnE '\binv: ' .awf/agents-doc.yaml .awf/domains/parts/*/current-state.md`
  (the ~24 agent-guide invariant-bullet citations plus the rendering/adr-system/config narrative
  citations); rewrite each `inv:` token occurrence to `invariant:`, leaving surrounding prose intact.
  Post-check: `grep -rnE '\binv: ' .awf/agents-doc.yaml .awf/domains/parts/*/current-state.md | wc -l`
  prints `0`. Run `./x sync` after.

- [ ] **Task 2.9 — Verify and commit (coupled Phase-2 commit).** `./x gate` && `./x check` (clean, no
  new `note:` lines). `git add` the changed `internal/invariants/*`, `cmd/awf/check.go`,
  `cmd/awf/invariants.go`, `internal/project/project.go`, **`internal/project/context.go`** (the
  `DeclaringADRs` return-type consumer, Task 2.1), their tests, every rewritten `docs/decisions/*.md`,
  the changed `.awf/` authoring parts, and the regenerated rendered surfaces (`AGENTS.md`,
  `docs/decisions/README.md`, the ADR template, the `proposing-adr` skill, and the regenerated
  `docs/glossary.md`, `docs/architecture.md`, `docs/working-with-awf.md`, `docs/pitfalls.md`, and the
  regenerated domain docs `docs/domains/{invariants,rendering,adr-system,config}.md`). Commit:
  `feat(invariants): two-marker backing, testGlobs teeth, classification`.

## Phase 3 — Backed-aware context

- [ ] **Task 3.1 — Union scan recognising both markers.** In `internal/invariants/invariants.go`,
  change `MarkersUnder` (`invariants.go:220`) to scan the union of `sources` and `testGlobs` globs and
  to recognise both `invariant:` and `touches-invariant:` markers as present-under-a-path, returning
  per slug the surfacing marker kind and any touches note. Update its signature to take the full
  `*config.InvariantConfig` (for `TestGlobs`) rather than only `Sources`.

- [ ] **Task 3.2 — Label governing invariants and surface notes.** In `internal/project/context.go`,
  update the `MarkersUnder` call (`context.go:105`) to the new signature and consume
  `DeclaringADRs`'s `Class` (`context.go:120-139`) to label each Tier-1 governing invariant
  backed/unbacked. Add the class and the site notes (`Verify:` for unbacked; touches note) to the
  `res.Invariants` representation / `ContextResult`, keeping both the human and `--json` renderings
  derived from the one value (preserve `context-output-parity`); the read-only and static-fallback
  paths are untouched.

- [ ] **Task 3.3 — Tests.** Add `internal/invariants/invariants_test.go` +
  `internal/project/context_test.go` cases: a slug surfaced via a proof-only marker in a test file
  under a queried prod path (union scan), via a touches-only marker, and via both; a backed vs an
  unbacked governing invariant labelled correctly; a `Verify:` note and a touches note surfaced; the
  `--json` and human renderings agree.

- [ ] **Task 3.4 — Verify and commit.** `./x gate` && `./x check`. `git add internal/invariants/*
  internal/project/context.go internal/project/context_test.go` (+ any test fixtures). Commit:
  `feat(tooling): backed-aware two-marker context surfacing` (`context` is not an `audit.allowedScopes`
  member; `tooling` owns the `awf context` command).

## Phase 4 — Changelog

> The backing-model authoring surfaces (ADR template, adr-readme part, `proposing-adr` skill,
> AGENTS.md "Backed invariants" rule) are updated in Phase 2 (Task 2.8), co-travelling with the token
> rename. The AGENTS.md/domain `context` invariant wording (`context-tier1-governs` →
> `context-tier1-marker-union`) is deliberately **not** touched in this plan — that slug is still
> live/enforced until ADR-0106 flips in the migration plan (see Notes).

- [ ] **Task 4.1 — Joint changelog entry.** Add to `changelog/CHANGELOG.md` `[Unreleased]` a
  Breaking/Features entry covering: the `inv:`→`invariant:` declaration rename; the proof
  `invariant:` + advisory `touches-invariant:` markers; `invariants.testGlobs`; the inline
  backed/unbacked classification with symmetric enforcement; and the backed/unbacked-aware
  `awf context` output incl. the `--json` shape carrying per-invariant class and notes.

- [ ] **Task 4.2 — Verify and commit.** `./x gate` && `./x check`. `git add changelog/CHANGELOG.md`.
  Commit: `docs(invariants): changelog for two-marker model and testGlobs`.

## Verification

- `./x gate` and `./x check` clean at every phase boundary and at the end.
- `grep -rnE '^[[:space:]]*[-*][[:space:]]+`*inv: ' docs/decisions/` prints nothing (corpus migrated).
- awf's `.awf/config.yaml` has **no** `invariants.testGlobs` — source-only fallback active; no
  existing slug goes Unbacked; ADR-0105 and ADR-0106 remain `status: Proposed`.
- `awf context <a prod path governed by a test-backed invariant>` labels the invariant `backed`;
  a synthetic `unbacked-invariant:` fixture labels `unbacked` and surfaces its `Verify:` note.
- 100% coverage holds; every new branch (proof-only, touches-only, both, fallback, backed-without-
  proof, unbacked-with-proof, unbacked-without-Verify, dangling, bare-touches) has an explicit test.

## Notes

- **Out of scope (migration plan):** classifying awf's existing invariants backed/unbacked, adding
  proof markers on tests for the 73 production-only slugs (+ converting their prod annotations to
  `touches-invariant:`), setting awf's `invariants.testGlobs`, flipping ADR-0105/0106 to
  `Implemented`, retiring the `context-tier1-governs` marker and swapping the AGENTS.md/domain
  `context` invariant wording to `context-tier1-marker-union`, and updating the domain current-state
  narratives. That plan carries the enforcement flip and both ADR status flips.
- **Signature-threading (self-contained-phase rule).** The `DeclaringADRs` return-type change
  (Task 2.1: `map[string]string` → `map[string]Decl`) and the `Check` return-shape change (Task 2.5:
  adds advisory notes) thread through `CheckInvariants`, `runCheck`, `runInvariants`, and
  `context.go` — all updated within Phase 2 (Task 2.9's commit). The `MarkersUnder` signature change
  (Task 3.1) is absorbed by `context.go` within Phase 3. No signature change outlives its phase.
