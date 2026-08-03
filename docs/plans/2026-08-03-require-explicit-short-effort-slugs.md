---
format: plan-v2
date: 2026-08-03
adrs: [require-explicit-short-effort-slugs]
status: Implemented
---
# Plan: Require Explicit Short Effort Slugs

## Goal

Require one explicit canonical slug of at most 32 bytes for every new effort, preserve existing
63-byte residents, and synchronize every active command and confirmation surface without changing
the schema-2 record or managed-topology model.

## Architecture summary

`internal/effort` owns a named creation input and the distinct new-slug and resident-slug validation
policies. `cmd/awf` translates the interspersed required `--slug` flag into that input;
`internal/worktree` carries the input and created record through default worktree creation and
rollback without revalidating it. Authoring templates and convention parts own the three-field
confirmation and active signature, render projects and adopters, and are pinned by catalog projection
plus a closed-root stale-signature test. Phase 1 lands the complete behavior and active documentation
with the first five ADR operations. Phase 2 closes exhaustive deterministic coverage while leaving
the final unified-workflow operation and both artifact freezes to the established post-review flip.

## Phase 1: Land explicit creation and synchronized active guidance

**Execution mode: inline.**

Advances: ["resident-compatibility"]
Completes: ["explicit-creation-contract", "active-signature-sync"]

### Task 1.1: Specify the creation and compatibility contract in tests
Kind: batch
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:require-explicit-slug", "require-explicit-short-effort-slugs:separate-creation-input", "require-explicit-short-effort-slugs:bound-new-slugs", "require-explicit-short-effort-slugs:preserve-resident-compatibility", "require-explicit-short-effort-slugs:retain-record-and-topology", "require-explicit-short-effort-slugs:prove-boundaries"]
Paths: ["internal/effort/types_test.go", "internal/effort/service_test.go", "internal/effort/store_test.go", "internal/effort/branches_test.go", "internal/effort/activity_test.go", "internal/effort/memory_metadata_test.go", "internal/effort/memory_test.go", "internal/effort/safety_test.go", "internal/worktree/manager_test.go", "cmd/awf/effort_test.go", "cmd/awf/effort_worktree_test.go", "cmd/awf/main_test.go", "cmd/awf/gate_test.go", "internal/clispec/clispec_test.go"]
Representative: "Replace title-only creation calls with named `effort.NewInput{Slug: ..., Title: ...}` values and assert that the supplied slug, not a title transformation, becomes record identity, resident path, worktree path, and branch suffix."
Edge: "Prove 1- and 32-byte new slugs succeed, a 33-byte new slug fails without mutation, a persisted canonical 63-byte resident still loads and finishes, Unicode-only titles succeed with an explicit ASCII slug, and malformed grammar, invalid Git refs, collisions, allocator failures, and rollback retain actionable identity-specific errors."
Post-check: "`go test ./internal/effort ./internal/worktree ./internal/clispec ./cmd/awf -run 'Test.*(Effort|Slug|New|Worktree|ParseArgs|CommandSpec)'` exits nonzero only because the new `NewInput` API and required `--slug` behavior are not implemented yet; its output names that missing contract and contains no unrelated failure. Record this expected red state. Task 1.3 reruns the same package set to a passing terminal state after production consumers land."

Update the existing derivation table into two explicit policy tables rather than retaining a helper
whose name implies derivation. The new-creation table must cover empty, uppercase, underscore,
leading or repeated hyphen, slash, 32-byte success, 33-byte refusal, and a Git-ref rejection seam.
The resident table must retain the canonical 1-through-63-byte rules used by load, list, show,
activity, memory, worktree, and finish operations. Use deliberately different sample values so a test
cannot pass by applying the 32-byte limit globally.

At the CLI boundary, add cases for:

- `awf effort new --slug short-slug "Descriptive outcome"`;
- `awf effort new "Descriptive outcome" --slug short-slug`;
- missing, valueless, and duplicate `--slug`;
- `--slug` combined with `--json`, `--no-worktree`, and `--base` in both legal orders;
- the existing `--base` with `--no-worktree` refusal before mutation;
- a title beginning with `-`, which remains a flag-shaped positional refusal unless separated by the
  command's existing grammar rather than introducing a new parser exception.

Assert missing required input fails before the composer is invoked. Preserve protocol-2 JSON shape
assertions exactly: only the supplied slug value changes. Update `cmd/awf/gate_test.go`'s gated
command probe with a valid explicit slug so the version-gate test exercises the command rather than
its new grammar refusal.

For compatibility, construct a 63-byte canonical resident through the existing test publication seam
or a valid schema-2 fixture, not through new creation. Exercise `List`, `Show`, activity selection,
memory update, worktree lookup, and restartable finish at the layers that own them. Pi attachment is
covered in Task 2.2. Do not weaken resident validation or create a migration path.

### Task 1.2: Introduce the named input and split slug policy
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:separate-creation-input", "require-explicit-short-effort-slugs:bound-new-slugs", "require-explicit-short-effort-slugs:preserve-resident-compatibility", "require-explicit-short-effort-slugs:synchronize-active-signatures"]
Paths: ["internal/effort/types.go", "internal/effort/service.go", "internal/effort/store.go"]

Add a documented exported `NewInput` in `internal/effort/types.go` with `Slug` and `Title` fields.
Change `Service.New` to accept that value. Normalize and validate the title independently, then
validate the supplied slug through a creation-only function before allocating a UUID or reserving a
directory. Creation validation must:

1. reuse the canonical resident grammar/confinement policy;
2. impose the additional 32-byte maximum with an error that says the new slug must contain 1-32
   bytes;
3. probe `refs/heads/awf/<slug>` exactly once at minting time;
4. distinguish Git mechanism failure from a ref-format refusal; and
5. report `changed bytes: no` plus one corrective action naming explicit `--slug` input.

Retain `validateSlug` as the resident/selection validator with its current 1-through-63-byte contract.
Name the new function so callers cannot mistake it for resident validation. Delete `deriveSlug` and
`slugRepairError`; remove now-unused imports and tests. Do not transliterate, lowercase, truncate,
hash, suffix, compare semantic similarity with the title, or accept an empty slug.

Build the record from the normalized title and the already-validated supplied slug. Keep field order,
schema version, UUID behavior, created-at UTC behavior, memory path calculation, and store
publication unchanged. Rewrite allocator and collision recovery text to reconstruct
`awf effort new --slug %q %q` with slug and normalized title in that order. Use Go formatting that
quotes both values safely; do not build an unquoted shell command. Collision recovery must direct the
caller to choose a distinct explicit slug, not a distinct outcome title.

Run `gofmt` on the changed Go files. At this task boundary, the package may not compile until Task 1.3
updates worktree and command consumers; do not add a compatibility overload or title-derived adapter
to make the intermediate task green.

### Task 1.3: Carry explicit identity through worktree orchestration and CLI grammar
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:require-explicit-slug", "require-explicit-short-effort-slugs:separate-creation-input", "require-explicit-short-effort-slugs:retain-record-and-topology", "require-explicit-short-effort-slugs:synchronize-active-signatures"]
Paths: ["internal/worktree/manager.go", "cmd/awf/effort.go", "cmd/awf/dispatch.go", "internal/clispec/clispec.go", "internal/migrate/unified_effort_residents.go"]

Change `Manager.NewEffort` to accept `effort.NewInput` plus the existing base. Pass the input to
`Service.New`; continue using the returned record slug for Add, topology inspection, finish, and
reporting. Carry the input or complete record into rollback so every retry that reconstructs creation
uses `awf effort new --slug <quoted-slug> <quoted-title>`. Preserve rollback classification and the
rule that only proven-absent managed topology permits resident deletion. Do not validate slug or title
again in `internal/worktree`.

In `internal/clispec/clispec.go`, declare `--slug` as a value flag for `effort new`, retain exactly one
outcome-title positional, and change help to the canonical signature
`awf effort new --slug <slug> <outcome-title> [--json] [--no-worktree] [--base <ref>]`. State that the
caller supplies an immutable 1-through-32-byte slug, the title is independent, flags are
interspersed, and existing default worktree/base behavior is unchanged. Do not add a second
positional or make `--slug` optional in prose.

In `validateEffortGrammar`, require presence of the `--slug` map entry before composing services or
checking semantic combinations. Let the shared parser continue to own missing values, duplicates,
unknown flags, and interspersed ordering. In `runEffort`, keep the first positional as title, create a
named `effort.NewInput` from `--slug` and title, and pass it through both worktree and no-worktree
paths. Rename misleading local `slug` variables in the `new` case without changing selection for
other subcommands.

Update the current generation-22 migration recovery message in
`internal/migrate/unified_effort_residents.go` to show the required explicit signature because it is
live recovery guidance, not immutable history. Preserve the migration's reset behavior and output
ordering.

Run:

```sh
gofmt -w internal/effort/types.go internal/effort/service.go internal/effort/store.go internal/worktree/manager.go cmd/awf/effort.go cmd/awf/dispatch.go internal/clispec/clispec.go internal/migrate/unified_effort_residents.go
go test ./internal/effort ./internal/worktree ./internal/clispec ./cmd/awf
```

Both commands must succeed. Inspect refusal output to ensure invalid title text never advises changing
slug and invalid slug text never advises shortening title.

### Task 1.4: Author the three-field workflow and synchronize active signatures
Kind: batch
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:confirm-three-fields", "require-explicit-short-effort-slugs:synchronize-active-signatures"]
Paths: ["README.md", ".awf/parts/", ".awf/docs/", ".awf/skills/", ".awf/topics/", "templates/", "changelog/CHANGELOG.md", ".awf/awf.lock", "AGENTS.md", "docs/", ".pi/", ".claude/", "examples/"]
Representative: "Change the shared first-creation block to present `Outcome:`, `Effort title:`, and `Effort slug: <proposed-short-slug>`, require later confirmation of all three, and invoke `awf effort new --slug <confirmed-slug> \"<confirmed-title>\"`."
Edge: "Update active help, recovery, architecture, guide, workflow, skill, README, current-state, Pi, Claude, and Sundial output while preserving old signatures under `docs/decisions/`, `docs/plans/`, and `changelog/`; render generated files from their sources and do not hand-edit them."
Post-check: "After `./x render`, `./x check` reports clean drift; the exact tracked search reports only the byte-unchanged Remaining `unified-effort-workflow-coverage` claim in its authoring source and rendered topic copy. All other active title-only creation signatures and title-derived guidance are absent; historical matches remain under the three exclusions."

Update `templates/partials/outcome-confirmation.md` as the single operative first-creation protocol.
It must present the three labels, ask the user to confirm creation, stop without mutation, require a
clear response in a later turn, and bind requested revision or ambiguity to all three fields. Retain
minimal-fix, fixed-effort resume, failed-creation, and context-loss behavior. Update every template or
project convention part that currently says outcome/title pair, proposed title, pair, both fields, or
`awf effort new "<confirmed title>"` so it says three fields and uses the required command. Do not
silently derive the proposed slug in instructions; the agent proposes a canonical short value for
explicit user approval.

Use the canonical command spelling below in active workflow examples unless a test intentionally
exercises interspersed ordering:

```text
awf effort new --slug <confirmed-slug> "<confirmed-title>"
```

Update `.awf/docs/parts/architecture/data-flow.md` so effort creation validates and exclusively
reserves the supplied slug rather than deriving it. Update `.awf/docs/parts/development/command-runner.md`,
`.awf/parts/working-with-awf/commands.md`, `.awf/docs/pitfalls.yaml`, README command tables, glossary
sources, workflow/agent-guide templates, skill templates, and current authoring parts wherever their
live wording states the old signature or derived identity. A bare factual mention of the command must
still make the required slug discoverable when it is presenting creation mechanics.

In `templates/pi/awf-effort/index.ts.tmpl`, change the `using_effort` input's maximum from 255 to 63
characters. This is resident attachment compatibility, not new creation, so do not use 32. Keep the
canonical lowercase/hyphen pattern and activity protocol unchanged. Generated `.pi/extensions`
content changes only through render.

Add an `[Unreleased]` changelog feature entry describing required explicit 32-byte new slugs,
three-field confirmation, unchanged schema/topology, and continued 63-byte existing-resident support.
The changelog entry is new history; do not rewrite prior entries that accurately describe derived
creation at their release time.

Before rendering, enumerate active occurrences with the exact command below and classify each match
as authored source, direct current file, generated output, or current-behavior fixture. Edit sources
and direct files only. After rendering, rerun the same command and require exactly two lifecycle-
authorized findings: the byte-unchanged Remaining `unified-effort-workflow-coverage` claim in
`.awf/topics/parts/rendering/workflow-skill-templates/current-state.md` and its rendered copy in
`docs/topics/rendering/workflow-skill-templates.md`. The deferred terminal flip removes both; only
then must the same command reach the zero-finding terminal state.

```sh
set +e
stale=$(git grep -n -E \
  -e 'awf effort new[^<]*<(confirmed title|outcome|outcome-title)>' \
  -e '[Ee]ffort (creation )?deriv(e|es|ed|ing)[^[:cntrl:]]{0,40}slug' \
  -e '[Dd]eriv(e|es|ed|ing) an immutable slug' \
  -e 'outcome/title (pair|confirmation)' \
  -e 'labeled outcome and (proposed )?effort title' \
  -e 'confirms? the pair' \
  -e 'both fields' \
  -- cmd internal .awf/parts .awf/docs .awf/skills .awf/topics templates \
  AGENTS.md README.md docs .pi .claude examples \
  ':(exclude)docs/decisions/**' ':(exclude)docs/plans/**' \
  ':(exclude)changelog/**')
status=$?
set -e
if [ "$status" -gt 1 ]; then
  printf '%s\n' "$stale" >&2
  exit "$status"
fi
printf '%s' "$stale"
test "$status" -eq 0
test "$(printf '%s\n' "$stale" | wc -l)" -eq 2
printf '%s\n' "$stale" | grep -F '.awf/topics/parts/rendering/workflow-skill-templates/current-state.md:' >/dev/null
printf '%s\n' "$stale" | grep -F 'docs/topics/rendering/workflow-skill-templates.md:' >/dev/null
```

Run `./x render` once all authoring changes are complete, then inspect representative Pi, Claude,
root, and Sundial brainstorming and command guidance. Never edit `docs/decisions/INDEX.md`, generated
topics, skills, guides, extensions, or adopter outputs by hand.

### Task 1.5: Apply the first five claim updates and enter Implementing
Kind: batch
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:require-explicit-slug", "require-explicit-short-effort-slugs:bound-new-slugs", "require-explicit-short-effort-slugs:retain-record-and-topology", "require-explicit-short-effort-slugs:confirm-three-fields", "require-explicit-short-effort-slugs:synchronize-active-signatures", "require-explicit-short-effort-slugs:render-lifecycle-index"]
Paths: ["docs/decisions/require-explicit-short-effort-slugs.md", ".awf/topics/parts/tooling/effort-management/current-state.md", ".awf/topics/parts/tooling/cli/current-state.md", ".awf/topics/parts/rendering/guide-and-doc-templates/current-state.md", ".awf/topics/parts/rendering/workflow-skill-templates/current-state.md", "docs/decisions/INDEX.md", "docs/topics/", ".awf/awf.lock"]
Representative: "Update `effort-record-authority` to distinguish caller-supplied 32-byte creation identity from 63-byte resident compatibility, preserving record/activity ownership and provenance before appending this ADR slug."
Edge: "Apply exactly operations 1 through 5 in declaration order; leave `unified-effort-workflow-coverage` byte-for-byte unchanged and Remaining for the deferred post-review flip."
Post-check: "`./x render && ./x check` is clean; `awf context docs/decisions/require-explicit-short-effort-slugs.md` reports the first five operations Applied and only `unified-effort-workflow-coverage` Remaining; no undeclared or Canceled operation appears."

Use the `awf-adr-lifecycle` procedure. Move the ADR directly from Proposed to Implementing, append the
Implementing content-stamp event, then append one Applied event listing these operations in exact
declaration order:

```text
update `tooling/effort-management:effort-record-authority`, update `tooling/effort-management:default-worktree-creation`, update `tooling/cli:effort-command-contract`, update `rendering/guide-and-doc-templates:working-memory-single-home`, update `rendering/workflow-skill-templates:mandatory-approval-boundaries`
```

Update the five matching claim blocks in their `.awf/topics/parts/` authoring sources in the same
staged transaction. Preserve each `Origin:` and prior `Revised-by:` sequence, append
`ADR-require-explicit-short-effort-slugs`, and keep `Backing: test` with its existing proof marker.
The claims must state current behavior, not implementation steps:

- `effort-record-authority`: supplied new slug, 32-byte minting boundary, 63-byte resident reads, and
  otherwise unchanged schema-2 ownership;
- `default-worktree-creation`: required explicit slug feeds the same path, branch, Add, base, and
  rollback behavior;
- `effort-command-contract`: canonical required flag, one title positional, interspersed ordering,
  unchanged protocol shapes and other subcommands;
- `working-memory-single-home`: three-field proposal and later confirmation before creation;
- `mandatory-approval-boundaries`: the first stop presents and confirms all three labels.

Do not update `unified-effort-workflow-coverage` or append its operation. Use the lifecycle digest
probe required by the repository; never commit a zero placeholder. Run `./x render` to regenerate
INDEX, lock, topic docs, skills, guides, extensions, and adopter output, then run the Post-check.

### Phase close

Stage the complete Phase 1 transaction explicitly, including production code, tests, authored
current-state and workflow sources, the ADR lifecycle batch, changelog, and render-derived outputs.
Confirm the final unified-workflow operation is absent from both staged claim changes and the Applied
event. Run `awf check staged` and `./x gate`; both must pass with clean drift and 100% statement
coverage. Create one commit:

```commit
feat(awf): require explicit short effort slugs
```

## Phase 2: Pin exhaustive active-surface and compatibility coverage

**Execution mode: inline.**

Completes: ["resident-compatibility", "deterministic-drift-coverage"]

### Task 2.1: Gate stale signatures over the closed active path policy
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:gate-signature-drift", "require-explicit-short-effort-slugs:prove-boundaries"]
Paths: ["internal/project/spine_test.go"]

Before modifying Phase 2 files, rerun the exact shell block in Task 1.4 and require its documented
exact two-finding intermediate state. Any other match returns to Phase 1 ownership; an exit above 1
is a search failure. Do not expand Phase 2 into conditional source cleanup. The deterministic test
may recognize only these two exact claim locations while the owning ADR operation is Remaining; the
deferred terminal flip must remove that allowance and prove the same scan reaches zero.

Add one deterministic repository-surface test using the file-reading and project-root helpers already
owned by `internal/project` tests. Encode the ADR's closed policy exactly:

- authoring roots: `cmd/`, `internal/`, `.awf/parts/`, `.awf/docs/`, `.awf/skills/`, `.awf/topics/`,
  and `templates/`;
- rendered/current surfaces: `AGENTS.md`, `README.md`, `docs/`, `.pi/`, `.claude/`, and `examples/`;
- excluded historical roots: `docs/decisions/`, `docs/plans/`, and `changelog/`.

Walk regular tracked/project files deterministically, skip ignored resident/worktree state, and fail
with path plus offending contract when an active file contains a title-only creation signature,
title-derived creation instruction, or two-field confirmation that omits `Effort slug:`. Construct
forbidden literals from fragments or otherwise keep the test's own matcher declarations from
self-matching; do not exempt arbitrary individual files. Treat current-behavior test fixtures as
active. Report every finding in stable path order so one run exposes the complete cleanup set.

The test is a contract pin, not a second prose source: require semantic anchors rather than one exact
paragraph, and rely on existing projection tests for ordering and target completeness. Run the new
test against a temporary copy with one representative active file rewritten to the former signature
and assert the diagnostic names that file; prove an equivalent historical fixture under each excluded
root is ignored. Restore all temporary bytes through test cleanup.

### Task 2.2: Complete CLI, workflow projection, and Pi boundary proofs
Kind: batch
Latitude: exact
Applying: ["require-explicit-short-effort-slugs:prove-boundaries"]
Paths: ["internal/evals/chain_test.go", "internal/project/spine_test.go", "internal/project/example_wiring_test.go", "tools/pi-extension-test/tests/using-effort.test.ts", "internal/effort/types_test.go", "cmd/awf/effort_test.go", "internal/worktree/manager_test.go"]
Representative: "Require every rendered discovery owner to present `Outcome:`, `Effort title:`, and `Effort slug:` before its later-response anchor and before the required explicit creation command."
Edge: "Reject a 64-character Pi attachment slug while accepting a canonical 63-character existing slug; reject a 33-byte new CLI slug while proving the same 33-byte identity remains selectable as a preexisting resident."
Post-check: "`go test ./internal/evals ./internal/project ./internal/effort ./internal/worktree ./cmd/awf` passes; `./x pi-test run` passes all TypeScript extension tests; the stale-signature test reports zero active findings."

Extend the catalog-derived workflow assertions rather than adding a parallel target list. For Pi,
Claude, this repository, and Sundial, prove the three labels precede confirmation, the stop and later
response precede creation, the confirmed slug placeholder appears in `--slug`, and no downstream or
checkpoint path creates a missing effort. Retain the complete role map, fixed-effort behavior,
minimal-fix exception, ambiguity/revision handling, and unresolved-token checks.

In CLI tests, verify both accepted interspersed orderings reach the same `NewInput`, while missing and
duplicate flags fail at grammar. Verify new/refusal diagnostics contain the supplied slug where safe,
title and slug corrections remain independent, JSON remains schema 2, and worktree rollback includes
a reconstructible quoted command. Do not assert incidental full prose where condition, mutation axis,
and next action are sufficient.

In `tools/pi-extension-test/tests/using-effort.test.ts`, add exact 63/64 boundary cases without
changing activity authority or checkout semantics. Verify the rendered template and committed
extension remain identical after render. Use the project runner's Pi lane rather than invoking an
unversioned host tool.

### Phase close

Run `./x render` and require a no-op render with `./x check` clean; Phase 2 introduces tests, not new
active authoring or generated output. Run the closed-root signature test and inspect its stable exact
two-finding intermediate result alongside representative root and adopter outputs. Confirm both are
the deferred claim locations and old signatures otherwise remain untouched only in excluded
historical roots; do not rewrite history to make an unrestricted grep empty. The deferred terminal
flip removes the two-location allowance and proves the final zero-finding result.

Stage the complete deterministic-coverage transaction explicitly. Verify no ADR status/history,
current-state claim mutation, active guidance source, or render-derived output is staged. Run
`git diff --check`, focused Go tests, the Pi extension lane, `awf check staged`, and `./x gate`; every
command must pass with clean drift and 100% coverage. `git status --short` must list only intended
Phase 2 test changes. Create one commit:

```commit
test(rendering): gate explicit effort slug signatures
```

## Definition of done

- `dod: explicit-creation-contract` Every new effort requires interspersed `--slug`, accepts only canonical 1-through-32-byte new identities, carries a named slug/title input through rollback, and preserves schema-2 and default managed-worktree behavior.
- `dod: resident-compatibility` Existing canonical identities through 63 bytes remain usable across effort commands, memory/activity, managed topology, finish, and Pi attachment without migration.
- `dod: active-signature-sync` Every active authoring/current/rendered surface and adopter uses three-field confirmation and the required explicit signature, while terminal ADRs, historical plans, and changelog history remain unchanged.
- `dod: deterministic-drift-coverage` Closed-root tests, catalog projections, CLI/worktree suites, Pi extension tests, render/check, and the full gate deterministically reject contract drift and pass on the implemented behavior.

## Notes

- ADR application partition: Phase 1 moves the ADR from Proposed to Implementing and applies
  operations 1 through 5 in declaration order with their matching claim mutations. Phase 2 adds the
  exhaustive coverage and remaining implementation but applies no claim operation. After terminal
  implementation review settles, the established deferred flip transaction updates
  `rendering/workflow-skill-templates:unified-effort-workflow-coverage` and provenance. Its claim body
  must require every discovery owner to present `Outcome:`, `Effort title:`, and `Effort slug:`, then
  invoke the explicit `--slug` creation signature only after later confirmation, while preserving
  the existing closed role map, minimal-fix exception, fixed-effort resume, ownership, checkpoint,
  and terminal-flow guarantees. The same transaction appends the final Applied event followed by the
  Implemented content stamp, flips this plan to Implemented, runs `./x render`, stages INDEX and lock
  output, and passes `awf check staged` plus `./x gate` before commit.
- If implementation changes any settled ADR commitment or the six-operation partition, stop for ADR
  amendment and renewed ADR review rather than adapting the plan silently.
- No schema migration, package boundary, new dependency, parser rewrite, slug semantic-similarity
  rule, or historical-record rewrite is part of this plan.
