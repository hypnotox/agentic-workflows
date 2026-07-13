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
  fallback, ADR-0105). Add `internal/config/config_test.go` cases: a valid `testGlobs: ['**/*_test.go']`
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

  `DeclaringADRs` returns `map[string]Decl` where `type Decl struct { ADR string; Class Class }`;
  duplicate-slug and dangling-retirement logic unchanged. Update its callers minimally: `Check`
  (this phase) and `ContextFor` (Phase 3 — but keep the return shape stable so `context.go` compiles
  now; `context.go` reads `.ADR`, and `.Class` is consumed in Phase 3).

- [ ] **Task 2.2 — Require the `Verify:` note on unbacked declarations.** `DeclaringADRs` (or a
  sibling that also reads the bullet text) records, per unbacked slug, whether its bullet carries a
  `Verify:` segment. Capture the full bullet line for unbacked declarations and test it against
  ``regexp.MustCompile(`(?i)\bVerify:\s*\S`)``; a missing/empty `Verify:` is surfaced as a hard
  finding in Task 2.5.

- [ ] **Task 2.3 — Split proof and touches markers in the scan.** In `scanTags` (`invariants.go:154`)
  recognise two marker forms after the literal source marker: `invariant: <slug>` (proof) and
  `touches-invariant: <slug>[ — <note>]` (touches). Return, per file scanned, the set of proof slugs
  and the set of `(touches-slug, note)` pairs. A proof marker counts toward backing only when the
  file matches a `TestGlobs` pattern **or** `TestGlobs` is empty (source-only fallback). Extend
  `slugRe` handling to strip an optional trailing note (everything after the slug, trimmed) for
  touches markers. Preserve the line-leading rule (`invariants.go:203`).

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
  lines matched by `grep -rnE '^[[:space:]]*[-*][[:space:]]+`*inv: ' docs/decisions/`. Do **not** touch
  `inv:` appearing mid-prose (non-leading), nor code/other files. (Every current declaration is
  `Backed`; no ADR converts to `unbacked-invariant:` in this plan — that is the migration plan's
  classification work.)

  Post-check (all must hold): `grep -rnE '^[[:space:]]*[-*][[:space:]]+`*inv: ' docs/decisions/ | wc -l`
  prints `0`; `go test ./internal/invariants/...` passes (declRe recognises `invariant:`); `./x check`
  is clean (every rewritten slug still backed under source-only fallback).

  Add `internal/invariants/invariants_test.go` cases (synthetic fixtures — awf's corpus has no
  `unbacked-invariant:` yet): backed-with-proof-in-test passes; backed-without-proof fails;
  proof-in-non-test-file with `TestGlobs` set fails; same passes with `TestGlobs` empty (fallback);
  unbacked-with-`Verify` passes; unbacked-without-`Verify` fails; unbacked-with-a-proof-marker fails;
  dangling-marker and bare-touches each yield a note not a finding.

  `./x gate` && `./x check`. `git add` the changed `internal/invariants/*`, `cmd/awf/check.go`,
  `cmd/awf/invariants.go`, `internal/project/project.go`, their tests, and every rewritten
  `docs/decisions/*.md`. Commit: `feat(invariants): two-marker backing, testGlobs teeth, classification`.

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
  `feat(context): backed-aware two-marker context surfacing`.

## Phase 4 — Standard surfaces and changelog

- [ ] **Task 4.1 — Update the ADR authoring surfaces (config parts).** Reword, under `.awf/`, the ADR
  template Invariants guidance to show both declaration forms — ``- `invariant: <slug>` — …`` (backed)
  and ``- `unbacked-invariant: <slug>` — …. **Verify:** …`` (unbacked) — and the `proposing-adr` skill
  part and `.awf/parts/adr-readme/invariants.md` to the unified `invariant:` token, the proof/touches
  marker split (with per-language markers), and the classification. Reword the AGENTS.md source parts'
  "Backed invariants" and `context` rules for the two markers, the test-scoped proof + source
  fallback, the backed/unbacked classification, and `context-tier1-marker-union`. Run `./x sync`;
  `./x check` must be clean (rendered `AGENTS.md`, `docs/decisions/README.md`, template, skill all
  regenerate without drift).

- [ ] **Task 4.2 — Joint changelog entry.** Add to `changelog/CHANGELOG.md` `[Unreleased]` a
  Breaking/Features entry covering: the `inv:`→`invariant:` declaration rename; the proof
  `invariant:` + advisory `touches-invariant:` markers; `invariants.testGlobs`; the inline
  backed/unbacked classification with symmetric enforcement; and the backed/unbacked-aware
  `awf context` output incl. the `--json` shape carrying per-invariant class and notes.

- [ ] **Task 4.3 — Verify and commit.** `./x gate` && `./x check`. `git add` the changed `.awf/`
  parts, the regenerated rendered docs, and `changelog/CHANGELOG.md`. Commit:
  `docs(invariants): document two-marker model and testGlobs`.

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
  `Implemented`, retiring the `context-tier1-governs` marker, and updating the domain current-state
  narratives. That plan carries the enforcement flip and both ADR status flips.
- The `Check`/`MarkersUnder` signature changes (Tasks 2.5, 3.1) thread through `CheckInvariants`,
  `runCheck`, `runInvariants`, and `context.go`; each is updated in the same phase that changes the
  signature, per the self-contained-phase rule.
