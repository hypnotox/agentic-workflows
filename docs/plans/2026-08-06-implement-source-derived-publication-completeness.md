---
format: plan-v2
date: 2026-08-06
adrs:
  - derive-publication-completeness-from-source-authorities
status: Proposed
---
# Plan: Implement Source-Derived Publication Completeness

## Goal

Make the README command block, template embedding, and config-reference live-state projections fail
closed against their existing authorities, then retire the three human pitfalls those proofs replace.
The change does not redesign singleton fan-out, infer arbitrary prose meaning, reorganize the pitfalls
taxonomy, or consolidate reviewer lenses.

## Architecture summary

Implementation proceeds through three independently green subagent-driven transactions. Each surface
keeps its current owner: a repository-only clispec test owns the README Markdown projection, the
templates package compares source files with its embedded filesystem, and configspec derives
live/static classification from path structure while the project package owns current-value
presentation. Each phase lands its proof before removing the matching pitfall, applies exactly one
ADR operation with the matching claim and proof marker, regenerates all affected outputs, and leaves
the ADR `Implementing`. Terminal review owns the later status-only ADR completion and plan freeze.

## Phase 1: Derive the README command block from clispec

**Execution mode: subagent-driven.**

Advances: ["selected-pitfalls-retired"]
Completes: ["readme-command-projection"]

### Task 1.1: Add the repository-only README projection proof
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["internal/clispec/readme_test.go", "internal/clispec/clispec.go"]

Establish the phase baseline before editing: `git status --short` produces no output, `./x check`
reports zero findings, and `./x gate` exits zero.

Create `internal/clispec/readme_test.go`; do not add production Markdown rendering or a new package.
Update the `Commands` ownership comment and its `touches-state` explanation in `clispec.go` to name
the bounded README command projection and both proof locations. Preserve the existing proof marker
in `clispec_test.go` without editing that file. Derive the expected bounded block from `Commands` in
source order. For each top-level command, use
its first `Help.Usage` value as the `Command` cell and its `Summary` as the `Purpose` cell, wrap the
usage in a Markdown code span, and escape table-cell pipe characters without changing the command
text. Include the table header and the exact unique boundary comments
`<!-- awf:clispec-commands:start -->` and `<!-- awf:clispec-commands:end -->` in the expected block.

Add table-driven formatter tests covering ordinary text, a usage containing a pipe, stable source
order, and a summary requiring table-cell escaping. Add comparison tests over synthetic README text
for a missing row, reordered row, reworded summary, missing marker, duplicate marker, and exact
match. The repository test reads `../../README.md`, compares its one bounded block byte-for-byte,
and on mismatch prints the exact expected replacement block. Mark that repository test as the proof
for `tooling/cli:cli-command-spec-single-source`. Demonstrate the instrument before editing the
README: `go test ./internal/clispec -run 'TestREADMECommandBlock'` must fail because the current
manual table has no markers, with the expected block in the diagnostic.

### Task 1.2: Replace the manual README table and retire its pitfall
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["README.md", ".awf/docs/pitfalls.yaml"]

Replace only the current `## Commands` table with the exact block emitted by Task 1.1. Preserve the
heading, `Run awf help for the full synopsis.`, and all surrounding hand-authored prose. The bounded
block contains every top-level command exactly once and no selected child-command rows; nested detail
remains owned by structured `awf help`. Run the repository test and require it to pass.

Delete the complete YAML record titled `README.md is outside the drift oracle` from
`.awf/docs/pitfalls.yaml` only after the repository test is green. Read the resulting README section
as a focused semantic check: the concise summaries must not contradict the adoption examples above
it, and the following full-synopsis sentence must still truthfully route detailed and nested help.

### Task 1.3: Apply the CLI authority operation and regenerate outputs
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["docs/decisions/derive-publication-completeness-from-source-authorities.md", ".awf/topics/parts/tooling/cli/current-state.md", "docs/topics/tooling/cli.md", "docs/domains/tooling.md", "docs/decisions/INDEX.md", "docs/pitfalls.md", ".awf/awf.lock"]

Transition the ADR from `Proposed` to `Accepted`, then to `Implementing`, and append one Applied event
for exactly `update tooling/cli:cli-command-spec-single-source`. Update that claim to include the
bounded root README command block derived in top-level clispec order, preserve its Origin and existing
Revised-by sequence, append this pending ADR as Revised-by, and keep its Backing marker paired with
the repository test from Task 1.1.

Run `./x render`. Read back every reported mutation and retain the generated CLI topic, tooling
domain, decision index, pitfalls document, and lock changes belonging to this transaction. Authority
checks: `go test ./internal/clispec`, `./x check`, and `./awf topic tooling/cli:cli-command-spec-single-source`
all exit zero; the topic output describes the README projection and its proof. State check: the
README and rendered pitfalls document contain no `README.md is outside the drift oracle` heading or
title. Establish the absence probe ran by capturing `rg -nF`'s exit status and accepting only status
1 before printing `README pitfall absent`; any other status fails.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
feat(tooling): derive README command projection (applies ADR batch)
```

## Phase 2: Prove every source template is embedded

**Execution mode: subagent-driven.**

Advances: ["selected-pitfalls-retired"]
Completes: ["template-source-embed-parity"]

### Task 2.1: Add source-to-embed file-set parity
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["templates/embed_test.go"]

Establish the phase baseline before editing: `git status --short` produces no output, `./x check`
reports zero findings, and `./x gate` exits zero.

Create `templates/embed_test.go` in package `templates`. Keep the parity implementation test-only.
Walk arbitrary `fs.FS` inputs into sorted slash-relative regular-file sets. For the repository source
population, walk `os.DirFS(".")` and include every file below a root template directory while
excluding only root Go source and test files; do not maintain an expected directory allowlist. For
the embedded population, walk `FS` from its root. Compare the two sets in both directions and report
sorted `missing from embed` and `unexpected in embed` paths.

Use `fstest.MapFS` cases to prove exact parity, a source-only file, an embed-only file, a source-only
new top-level directory, and a dot- or underscore-prefixed file missing because `all:` semantics were
not supplied. The source-only new-directory case is the durable negative proof: require its focused
test to report the exact `missing from embed` path from test-owned filesystems, with no repository
mutation. Add the real repository parity test and mark it as the proof for the new
`rendering/templates:source-embed-parity` claim. Run
`go test ./templates -run 'Test(SourceEmbed|TemplateFile)'` and require success. Keep the explicit
compile-time patterns in `templates/embed.go` unchanged: they remain the Go embedding mechanism, not
a second unchecked authority, because the exact source-to-embedded file-set test fails whenever a
pattern omits a new directory, ordinary file, dot-prefixed file, or underscore-prefixed file. Do not
replace them with a package-wide wildcard that would embed root Go source and test files, and do not
move the template tree under a new common root.

### Task 2.2: Apply template parity authority and retire its pitfall
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["docs/decisions/derive-publication-completeness-from-source-authorities.md", ".awf/topics/parts/rendering/templates/current-state.md", ".awf/docs/pitfalls.yaml", "docs/topics/rendering/templates.md", "docs/domains/rendering.md", "docs/decisions/INDEX.md", "docs/pitfalls.md", ".awf/awf.lock"]

Append one Applied event to the Implementing ADR for exactly
`add rendering/templates:source-embed-parity`. Add the invariant claim with this pending ADR as
Origin, Backing test, and the proof marker from Task 2.1. Its prose names the source population,
root-Go-file exclusion, bidirectional exact file-set comparison, and missing/unexpected diagnostics;
it must not claim semantic validation or alter missingkey behavior.

Delete the complete YAML record titled `Add every new template directory to the embed allowlist`
only after the real and synthetic parity tests pass. Run `./x render`, read back every reported
mutation, and retain the generated template topic, rendering domain, decision index, pitfalls doc,
and lock changes. Authority checks: `go test ./templates`, `./x check`, and
`./awf topic rendering/templates:source-embed-parity` exit zero. State check: capture the exit status
of `rg -nF 'Add every new template directory to the embed allowlist' .awf/docs/pitfalls.yaml docs/pitfalls.md`,
accept only status 1, and print `template embed pitfall absent` so empty output cannot masquerade as
an unrun probe.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
test(rendering): prove source template embedding (applies ADR batch)
```

## Phase 3: Derive config-reference live-state classification

**Execution mode: subagent-driven.**

Completes: ["config-live-state-structure", "selected-pitfalls-retired", "repository-green"]

### Task 3.1: Derive classifications and prove resolver completeness
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["internal/configspec/spec.go", "internal/configspec/spec_test.go", "internal/project/configreference.go", "internal/project/configreference_test.go"]

Establish the phase baseline before editing: `git status --short` produces no output, `./x check`
reports zero findings, and `./x gate` exits zero.

Replace the explicit `LiveStateClassifications` membership map with a derivation over `Keys()`.
Classify a path as `StaticNotApplicable` exactly when it starts with `sidecar.` or contains `[]` or
`<name>`; classify every other project config path as `LiveStateProjection`. Retain the typed class
and the bidirectional `validateLiveStateAuthority` boundary. Add configspec tests for representative
root values, dotted project values, list roots, list-item leaves, named-map leaves, and sidecar
fields, plus exhaustive actual-key coverage. A newly added root or dotted project value must become
live without editing a second membership list.

Add current-value resolvers for `tags`, `contextIgnore`, `commitPolicy.grandfatheredThrough`,
`commitPolicy.allowedIdentities`, `commitPolicy.requireSignedCommits`, and
`commitPolicy.allowedSigners`. Use these exact representations: empty tags `(none)`, otherwise
`<n> tags`; empty contextIgnore `(none)`, otherwise `<n> patterns`; absent or empty
`grandfatheredThrough` `(none)`, otherwise the value inside one Markdown code span; empty allowed
identities `(none)`, otherwise `<n> identities`; empty allowed signers `(none)`, otherwise
`<n> signers`; and requireSignedCommits `false (default)` only when the entire commitPolicy mapping
is absent, otherwise its effective `true` or `false`. Collection and boundary summaries carry no
`(default)` suffix. Counts disclose no identity emails, signer principals, signer keys, tag values,
or ignored paths. Follow the existing config-reference conventions of `(none)` for empty
collections, code spans for concrete scalar identifiers, plural count summaries, and `(default)`
only when the displayed effective scalar comes from an absent owning config block.

Before replacing the explicit map, add
`TestLiveStateClassificationsDeriveProjectValues` in `internal/configspec/spec_test.go`; it requires
`tags`, `contextIgnore`, and every commitPolicy collection/scalar root to be
`LiveStateProjection`, and requires representative `[]`, `<name>`, and `sidecar.` paths to remain
`StaticNotApplicable`. Run
`go test ./internal/configspec -run '^TestLiveStateClassificationsDeriveProjectValues$'` against the
old map and require failure containing `tags class = StaticNotApplicable, want LiveStateProjection`;
a pass or another failure blocks implementation. After deriving the classifications, rerun the
same command and require success.

Update resolver-authority mutation tests: an omitted derived-live resolver must name the exact key;
a resolver attached to a structural static leaf must be rejected; an extra resolver and unknown
class must remain rejected. Extend live model and generated-reference cases with configured and
absent tags, context ignores, and commit-policy values, asserting the exact strings above and that
those project rows never render `n/a` while item leaves and sidecar rows still do. Run
`go test ./internal/configspec ./internal/project -run 'Test(LiveState|ConfigReference)'` and require
success.

### Task 3.2: Apply config authority, retire the final pitfall, and publish the change
Latitude: exact
Applying: ["derive-publication-completeness-from-source-authorities:source-derived-publication-completeness"]
Paths: ["docs/decisions/derive-publication-completeness-from-source-authorities.md", ".awf/topics/parts/config/configspec-and-reference/current-state.md", ".awf/docs/pitfalls.yaml", "changelog/CHANGELOG.md", "docs/topics/config/configspec-and-reference.md", "docs/domains/config.md", "docs/config-reference.md", "docs/decisions/INDEX.md", "docs/pitfalls.md", ".awf/awf.lock"]

Append one Applied event to the Implementing ADR for exactly
`update config/configspec-and-reference:live-state-projection-explicit`. Rewrite the invariant to
state structural classification and resolver completeness, preserve its Origin and prior Revised-by
sequence, append this pending ADR as Revised-by, and retain Backing test with proof markers on both
the configspec classification and project resolver/model tests.

Delete the complete YAML record titled
`A new config field needs a config-reference live-state projection case` only after the mutation and
render tests prove the original omission fails. Add one concise Unreleased changelog feature entry
covering all three source-derived completeness proofs and the removal of their manual reminders.
Run `./x render`, read back every reported mutation, and retain the generated config topic, config
domain, config reference, decision index, pitfalls doc, and lock changes.

Perform the focused semantic rendering review over `README.md`, `docs/config-reference.md`, and
`docs/pitfalls.md`: the README block reads as a concise top-level index rather than detailed help;
configured tags, context ignores, and commit-policy rows show non-secret current summaries without
contradicting their defaults or nested `n/a` rows; and none of the three retired hazards survives by
title or equivalent instruction in the active pitfalls catalog. Authority checks:
`go test ./internal/configspec ./internal/project ./internal/clispec ./templates`, `./x check`, and
`./awf topic config/configspec-and-reference:live-state-projection-explicit` all exit zero. Run this
exact final state check from the repository root; `./x check` is the structural YAML/render parse,
`rg` checks the closed title set, status 1 is the only clean absence, and the sentinel is printed only
after both complete:

```sh
set -e
./x check
set +e
rg -nF \
  -e 'README.md is outside the drift oracle' \
  -e 'Add every new template directory to the embed allowlist' \
  -e 'A new config field needs a config-reference live-state projection case' \
  .awf/docs/pitfalls.yaml docs/pitfalls.md
pitfall_status=$?
set -e
test "$pitfall_status" -eq 1
printf '%s\n' 'selected pitfalls absent'
```

Confirm the ADR remains `Implementing` with all three declared operations Applied and the plan
remains `Proposed` for terminal review.

### Phase close

Stage the complete phase, inspect `git diff --cached --check`, run `./awf check staged` and
`./x gate`, and create the single closing commit.

```commit
refactor(config): derive live-state projections (applies ADR batch)
```

## Definition of done

- `dod: readme-command-projection` The bounded root README command block exactly follows top-level clispec usage, summary, and order, and its mismatch test prints the replacement block.
- `dod: template-source-embed-parity` Every source template file, including new-directory and hidden-file cases, participates in bidirectional parity with `templates.FS`.
- `dod: config-live-state-structure` Every project config path structurally requires a live resolver while item-schema and sidecar paths remain static, with safe current summaries for all previously omitted values.
- `dod: selected-pitfalls-retired` The three selected pitfall records and rendered headings are absent only after their preventing proofs pass.
- `dod: repository-green` All three ADR operations are Applied with matching current-state claims and proof markers, generated outputs and changelog are current, the ADR remains `Implementing`, the plan remains `Proposed`, and `./x gate` passes at 100% statement coverage.

## Notes

Inline owners immediately correct stale instructions and record reasoned deviations here. Delegated
owners may report rather than edit; the parent supplies the report to phase review and reconciles it
with findings in one focused post-review settlement commit before checkpointing or later execution.
Record deviations, review findings, and any representation adjustment required by existing
config-reference presentation conventions; do not widen those adjustments into singleton fan-out or
semantic prose inference.

Phase 1 review found that its duplicate-marker fixture could pass through byte inequality without
proving both unique marker-count guards; settlement added explicit duplicate-start and duplicate-end
error assertions.

Phase 2 review found incomplete fixture proof for the exact root-Go exclusion and sorted unexpected
paths; settlement added root non-Go, nested Go, and multiple unexpected-path coverage. Its proposed
nested-checkout concern did not apply: `os.DirFS(".")` runs from the `templates` package directory,
so the walk is bounded to the settled template source tree rather than the repository root.
