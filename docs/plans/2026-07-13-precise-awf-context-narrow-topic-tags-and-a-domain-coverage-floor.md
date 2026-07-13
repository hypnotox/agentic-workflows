---
date: 2026-07-13
adrs: [109, 110]
status: Proposed
---
# Plan: Precise awf context — narrow-topic tags and a domain-coverage floor

## Goal

Implement ADR-0110 (domain-coverage floor + `contextIgnore`) and ADR-0109 (narrow-topic tag taxonomy)
so `awf context` reports precise, tiered relevance: `--uncovered` reaches zero on awf's own tree, and
a domain-scoped query returns a tight topical cluster instead of a third of the corpus. Non-goals: the
`awf doctor`/housekeeping command surface (a future effort — the frequency signal ships only as an
advisory `awf check` note here); making the tag-frequency threshold configurable; and re-tagging any
adopter beyond awf itself (the example adopter carries no vocabulary and stays inert).

## Architecture summary

Two ADRs, sequenced coverage-first then taxonomy, each flipping to `Implemented` at the end of its own
work so their invariant slug-renames land cleanly:

- **ADR-0110 (Phases 1–2):** extend the five domains' sidecar `paths:` to home every orphan code
  package (no sixth domain); rewrite `Project.Uncovered` to also subtract `PlannedOutputs()` and a new
  absent-safe top-level `contextIgnore` glob list; retire/rename `uncovered-lists-unowned-only` →
  `uncovered-lists-unowned-unignored` (folding the absent-safe case into its proof).
- **ADR-0109 (Phases 3–4):** add two advisory `awf check` note producers (frequency + coverage), inert
  under an empty vocabulary; then, in one atomic commit, re-curate `.awf/config.yaml` `tags:` to a
  narrow-topic vocabulary, re-tag all 108 ADRs + 46 pitfalls, add the exact `tag ≠ domain name` gate,
  and drop Tier 2's domain-name filter (retire/rename `context-tier2-topical` →
  `context-tier2-precise-tag`).

Invariant slug-renames move both a proof marker (a `*_test.go` `// invariant:` comment) and, for the
uncovered slug, an advisory `touches-invariant` marker in a docstring, in the flip commit.

## File structure

- **Created:** `docs/plans/2026-07-13-precise-awf-context-narrow-topic-tags-and-a-domain-coverage-floor.md` (this plan).
- **Modified:**
  - `.awf/domains/rendering.yaml`, `.awf/domains/config.yaml`, `.awf/domains/tooling.yaml`, `.awf/domains/adr-system.yaml` (domain path folds).
  - `internal/config/config.go` (`ContextIgnore` field), `internal/configspec/*` (its `KeyEntry`), `docs/config-reference.md` (regenerated).
  - `internal/project/context.go` (`Uncovered` rewrite; Tier-2 filter removal; both invariant markers), `internal/project/context_test.go` (renamed proof markers + new coverage cases), `internal/project/check.go` (`tag ≠ domain` gate; note producers), plus the note-producer wiring in `cmd/awf/check.go`.
  - `.awf/config.yaml` (`tags:` re-curation; `contextIgnore:`), every `docs/decisions/NNNN-*.md` and `.awf/docs/pitfalls.yaml` entry (re-tag), `.awf/agents-doc.yaml` (two invariant bullets reworded), and all regenerated surfaces (`AGENTS.md`, `docs/decisions/ACTIVE.md`, domain docs, `.awf/awf.lock`).
  - `docs/decisions/0109-*.md`, `docs/decisions/0110-*.md` (status flips).
- **Deleted:** none.

## Phase 1 — Home every code package in a domain (ADR-0110)

- [ ] **Task 1.1 — Extend the domain sidecar `paths:` to fold the orphan packages.** Edit the four
  domain sidecars, appending to each `paths:` list (anchored doublestar dialect, ADR-0077). Exact
  additions:
  - `.awf/domains/rendering.yaml`: add `- internal/project/**` and `- internal/refs/**`.
  - `.awf/domains/config.yaml`: add `- internal/configspec/**` and `- internal/pathglob/**`.
  - `.awf/domains/tooling.yaml`: add `- internal/clispec/**`, `- internal/initspec/**`, `- internal/git/**`.
  - `.awf/domains/adr-system.yaml`: add `- internal/plan/**` and `- internal/frontmatter/**`.

  Do not touch any domain's current-state part in this task (a Proposed→state doc must not front-run
  implementation prose; the folds' effect on `awf context` is behavioural, not a state-doc claim).
- [ ] **Task 1.2 — Verify and commit.** Run `./x sync` (regenerates the affected domain docs + lock),
  then `./x check` — expect `awf check: clean`. Confirm the folds took effect:
  `/tmp/awf context --uncovered 2>&1 | grep -E '^\s+internal/(project|refs|configspec|pathglob|clispec|initspec|git|plan|frontmatter)/'`
  must print **nothing** (all nine now domain-owned; `testsupport` remains, handled in Phase 2). Run
  `./x gate` (expect `GATE_OK` / 100% coverage — no code changed). `git add` the four sidecars plus the
  `./x sync`-regenerated `docs/domains/*.md` and `.awf/awf.lock`; commit
  `config(domains): fold orphan code packages into the five domains`.

## Phase 2 — contextIgnore + generated-exclusion + uncovered rename, flip ADR-0110

This phase's tasks land in one closing commit (Task 2.7): the `contextIgnore` field, its consumer in
`Uncovered`, the invariant rename, and the ADR-0110 flip are mutually dependent — the field is unused
until `Uncovered` reads it, and the flip requires the renamed invariant already backed — so they
cannot be sliced into independently-gate-passing sub-commits.

- [ ] **Task 2.1 — Add the `contextIgnore` config field.** In `internal/config/config.go`, add to
  `Config` immediately after the `Tags` field:
  ```go
  	ContextIgnore []string          `yaml:"contextIgnore"`
  ```
- [ ] **Task 2.2 — Describe the key in configspec.** Add one `KeyEntry` for `contextIgnore` mirroring
  the existing top-level `domains` key entry in `internal/configspec` (Type: a list of anchored
  globs; Availability: always; Default: `[]`; Description: one line — "Globs for tracked paths that no
  domain should own; `awf context --uncovered` treats them as legitimately unowned."). Locate the
  entry list by `grep -rn '"domains"' internal/configspec/*.go` and add the parallel entry so
  `go test ./internal/configspec/ -run TestConfigspecKeyParity` passes.
- [ ] **Task 2.3 — Rewrite `Uncovered` to subtract generated + ignored paths.** In
  `internal/project/context.go`, in `Uncovered`, after the domain-glob `covered` closure is built:
  (a) compute the generated set once — `planned, err := p.PlannedOutputs()` (propagate the error),
  into a `map[string]bool` keyed by `filepath.ToSlash`; (b) build an `ignored(path)` closure matching
  `p.Cfg.ContextIgnore` globs via `pathglob.Match`; (c) in the per-path loop, treat a path as excluded
  when `covered(clean) || planned[clean] || ignored(clean)` (fold generated+ignored into the existing
  covered-branch so their ancestors also mark `coveredDirs`, preserving collapse). Move the
  `// invariant:` marker on this function to `uncovered-lists-unowned-unignored` and update the
  advisory `touches-invariant: uncovered-lists-unowned-only` marker in the `Uncovered` docstring
  (context.go ~:297) to the new slug.
- [ ] **Task 2.4 — Update the uncovered tests.** In `internal/project/context_test.go`: rename the
  proof marker at ~:486 to `// invariant: uncovered-lists-unowned-unignored`; add two assertions to
  `TestUncovered…` — a tracked path present in `PlannedOutputs()` is not reported, and a path matched
  by a configured `contextIgnore` glob is not reported — and one absent/empty-`contextIgnore` case
  asserting the report is unchanged from domain+generated exclusion alone (backs the folded
  absent-safe clause). Reuse the existing synthetic-project fixture; assert exact `Entries`.
- [ ] **Task 2.5 — Add awf's own `contextIgnore` list.** In `.awf/config.yaml`, add a top-level
  `contextIgnore:` list with: `.awf/**`, `docs/**`, `examples/**`, `.github/**`, `.githooks/**`,
  `changelog/**`, `internal/testsupport/**`, `LICENSE`, `go.mod`, `go.sum`, `README.md`, `codecov.yml`,
  `.gitignore`, `.golangci.yml`, `.goreleaser.yaml`, `.gremlins.yaml`.
- [ ] **Task 2.6 — Reword the agent-guide invariant bullet and flip ADR-0110.** In
  `.awf/agents-doc.yaml`, find the `uncovered-lists-unowned-only` bullet and rename it to
  `uncovered-lists-unowned-unignored`, widen its wording (adds "not generated, not `contextIgnore`-matched"),
  and re-cite ADR-0110. In `docs/decisions/0110-*.md` set `status: Implemented`.
- [ ] **Task 2.7 — Verify, regenerate, commit.** Run `./x sync` (regenerates `AGENTS.md`,
  `docs/config-reference.md`, `ACTIVE.md`, lock). Run `./x check` — expect `awf check: clean` and
  `awf invariants: clean` (the renamed slug is now backed; the retired one is gone). Confirm the floor:
  `/tmp/awf context --uncovered` (rebuild `/tmp/awf` first: `go build -o /tmp/awf ./cmd/awf`) prints
  the zero-state header with **no** path entries. Run `./x gate` (expect 100%). `git add` the config,
  configspec, context.go/_test.go, agents-doc, the ADR, and every regenerated surface; commit
  `feat(tooling): add contextIgnore and drive awf context --uncovered to zero (ADR-0110)`.

## Phase 3 — Advisory tag-health note producers (ADR-0109), no flip

- [ ] **Task 3.1 — Add the frequency + coverage note producers, inert under an empty vocabulary.** Add
  a `Project` method (e.g. `tagHealthNotes() []string`) in `internal/project/check.go` returning
  non-failing note strings, guarded by `if len(p.Cfg.Tags) == 0 { return nil }` so an un-curated
  adopter (and the example) stays note-free. It reads the ADR + pitfall tag sets (reuse
  `adr.ParseDir(p.decisionsDir())` and `p.pitfallTagEntries()`, as `checkTagVocabulary` does) and emits:
  - **frequency:** for each vocabulary tag carried by strictly more than 25% of the artifacts carrying
    ≥1 vocabulary tag (denominator = tag-bearing ADRs + pitfalls; numerator = artifacts carrying that
    tag), a `note:`-prefixed line naming the tag and its share.
  - **coverage:** for each ADR or pitfall carrying zero tags, or (only when a governed vocabulary is
    non-empty is this branch reachable in a green tree — so exercise it under an empty-vocabulary test
    fixture) only tags equal to a configured domain name, a `note:`-prefixed line naming the artifact.

  Define the 25% threshold as a named constant with a one-line comment tying it to ADR-0109 item 4.
- [ ] **Task 3.2 — Wire the notes into the check-notes path and test.** Emit `tagHealthNotes()` through
  the same non-failing `note:` channel as the existing advisories (see `cmd/awf/check.go`; do not route
  them through `checkTagVocabulary`, which returns hard `Drift`). Add `internal/project` tests: a
  coarse-vocabulary fixture yields the expected frequency note; a zero-tag-artifact fixture yields the
  coverage note; an **empty-vocabulary** fixture yields **no** notes (the sundial-safety case). Mark
  the frequency and coverage assertions with backed `// invariant: tag-frequency-note` and
  `// invariant: tag-coverage-note` proof markers (declared by ADR-0109, enforced once it flips in
  Phase 4).
- [ ] **Task 3.3 — Verify and commit.** Run `./x gate` (100%). `./x check` on awf will now print
  frequency `note:` lines for `tooling`/`rendering` — advisory, non-failing, and expected until the
  Phase-4 re-tag; confirm the exit is still clean. Confirm the example stays note-free:
  `cd examples/sundial && /tmp/awf check` prints no `note:` line (empty vocabulary → inert). `git add`
  `internal/project/check.go`, its test, and any `cmd/awf/check.go` wiring; commit
  `feat(tooling): advisory tag-frequency and tag-coverage check notes (ADR-0109)`.

## Phase 4 — Narrow vocabulary, re-tag, gate, Tier-2 simplification, flip ADR-0109

These land in one atomic closing commit (Task 4.6): the `tag ≠ domain name` gate and
`tag-vocabulary-governed` both fail mid-way unless the vocabulary re-curation and the whole-corpus
re-tag are simultaneous, and ADR-0109's flip (which retires `context-tier2-topical`) requires every
new slug already backed. This is the deliberate unsliceable exception.

- [ ] **Task 4.1 — Design the narrow-topic vocabulary (curation sub-process).** Produce a replacement
  `.awf/config.yaml` `tags:` map of ~60–90 sub-domain topics, each with a one-line meaning, satisfying:
  no member equals a configured domain name (`adr-system`, `config`, `invariants`, `rendering`,
  `tooling`); each member intended to land on ~2–10 artifacts. Method: dispatch a **sequential**
  subagent fan-out (one batch of ADRs/pitfalls per subagent) that reads each artifact's decision and
  proposes 1–3 narrow topic labels; then a single merge pass reconciles synonyms into the governed
  vocabulary. Record the resulting vocabulary in `.awf/config.yaml`. (This task produces data validated
  by the post-checks in 4.2/4.6, not an exact diff — the label set is the curation output.)
- [ ] **Task 4.2 — Re-tag all 108 ADRs and 46 pitfalls (batch).** Rewrite each artifact's `tags:` to
  members of the new vocabulary.
  - **Representative** (`docs/decisions/0109-*.md` frontmatter):
    `tags: [context, governance]` → `tags: [context-tiering, tag-taxonomy]` (illustrative narrow
    labels; actual labels per 4.1).
  - **Edge** (`.awf/docs/pitfalls.yaml`, an entry whose `tags:` list is inline):
    `tags: [tooling, audit]` → the entry's narrow topics (e.g. `tags: [git-open, audit-scope]`).
  - **Affected-site set:** every file from
    `git ls-files 'docs/decisions/[0-9]*.md'` plus every entry under `.awf/docs/pitfalls.yaml data.pitfalls[].tags`.
  - **Post-check:** `./x check` prints `awf check: clean` (no `adr-tag`/`pitfall-tag` unknown-tag Drift,
    proving every used tag ∈ vocabulary), **and** a frequency probe shows no tag over 25%:
    rebuild `/tmp/awf` and run `/tmp/awf check` — **zero** `note:` lines about tag frequency (proves the
    re-tag achieved narrowness). Also re-tag `docs/decisions/0110-*.md` so its `[context, domains]`
    become vocabulary members (cross-dependency with ADR-0110).
- [ ] **Task 4.3 — Add the `tag ≠ domain name` gate.** In `internal/project/check.go`
  `checkTagVocabulary`, after the empty-vocabulary early return, append a check: for each
  `tag ∈ p.Cfg.Tags`, if `slices.Contains(p.Cfg.Domains, tag)` emit a `manifest.Drift` (`Path` =
  `config.yaml`, Kind `tag-domain-collision`, Detail naming the tag). Mark a new test in
  `internal/project` with `// invariant: tag-not-domain-name` asserting a domain-named vocabulary member
  fails and an empty-domains/empty-vocabulary project is inert.
- [ ] **Task 4.4 — Simplify Tier 2 (drop the domain-name filter) and rename its invariant.** In
  `internal/project/context.go`, in the tier-assembly, delete the `domainName` map and the
  `if !domainName[tag]` guard so the precise set is the plain union of Tier-1 tags; rename the
  `// invariant: context-tier2-topical` marker (~:186) to `// invariant: context-tier2-precise-tag`.
  In `internal/project/context_test.go` rename the proof marker (~:102) and **replace** the
  domain-mirror-*exclusion* assertion (now inverted) with a `tag-not-domain-name` gate assertion or a
  precise-union assertion. In `docs/decisions/0109-*.md` confirm `retires_invariants:
  [context-tier2-topical]` (already present).
- [ ] **Task 4.5 — Reword the agent-guide Tier-2 bullet and flip ADR-0109.** In `.awf/agents-doc.yaml`
  reword the `context-tier2-topical` invariant bullet to `context-tier2-precise-tag` (drop the
  "minus any tag naming a domain" clause; keep the `related:`-tail and empty-precise-set clauses) and
  re-cite ADR-0109. Set `status: Implemented` in `docs/decisions/0109-*.md`.
- [ ] **Task 4.6 — Verify, regenerate, commit (atomic).** Run `./x sync` (regenerates `AGENTS.md`,
  `ACTIVE.md`, domain docs, lock). Run `./x check` — expect `awf check: clean` and `awf invariants:
  clean` (all four ADR-0109 slugs backed; `context-tier2-topical` retired). Rebuild `/tmp/awf` and
  confirm the payoff: `/tmp/awf context internal/adr` now lists a **small** "Related ADRs" set (single
  digits, not ~32) and a tight pitfall set. Confirm the example still clean:
  `cd examples/sundial && /tmp/awf check` — no `note:` line. Run `./x gate` (100%). `git add` the
  config, every re-tagged ADR + pitfalls sidecar, check.go/context.go and their tests, agents-doc, the
  ADR, and all regenerated surfaces; commit
  `feat(tooling): narrow-topic tag taxonomy and precise Tier 2 (ADR-0109)`.

## Verification

- `awf context --uncovered` over awf's tree reports **zero** entries (Phase 2).
- `awf context internal/adr` (and any single-domain path) reports a single-digit "Related ADRs" count,
  down from ~32 (Phase 4).
- `./x check` is clean and `./x gate` is green after every phase; `awf invariants: clean` with
  `context-tier2-topical` and `uncovered-lists-unowned-only` retired and their successors backed.
- `cd examples/sundial && awf check` emits no `note:` line (empty-vocabulary inertness holds).
- No vocabulary tag exceeds 25% of tag-bearing artifacts, and none equals a domain name.

## Notes

- **Out of scope / follow-up:** the `awf doctor` housekeeping command (make the 25% threshold
  configurable, surface unused-vocabulary members and orphaned working-memory files) — its own future
  effort, per ADR-0109 Consequences.
- **Curation is the risk.** Task 4.1's vocabulary is a judgment output, not a mechanical diff; the
  4.2/4.6 post-checks (unknown-tag Drift = 0, frequency notes = 0, the coverage-note backstop) are the
  deterministic backstop, but whether each chosen tag is the *right* topic remains review discipline.
- **Atomicity.** Phase 4's re-tag + gate + vocabulary change share one commit by necessity; Phase 2's
  field + consumer + flip likewise. Both are stated exceptions to per-task commits, not conveniences.
