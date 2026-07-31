---
date: 2026-08-01
adrs: [199]
status: Proposed
---
# Plan: Enforce named proof markers

## Goal

Implement ADR-0199: make every `invariant:` proof marker name the unit that proves it, and make
`awf check` fail when that name no longer occurs on a searched line of the marker's own file, so a
marker can no longer outlive its test. Non-goal: detecting a marker whose named test exists but
does not exercise the claim, which is the separate nominal-proof problem `docs/roadmap.md` tracks.

## Architecture summary

Four phases, each independently green, sequenced so no commit leaves the corpus and the checker
disagreeing (ADR-0199 item 12).

Phase 1 repairs the two claims whose markers already point at nothing, under today's checker.
Phase 2 splits `markerPayloadRE` into per-kind regexes and lets an `invariant:` payload carry an
optional trailing name that nothing yet requires or reads. Phase 3 migrates every proof marker in
the tree to carry a name, which is legal under the permissive schema and inert under the checker.
Phase 4 makes the name required, adds the occurrence check inside `scanMarkerBytes`, repairs the
test fixtures the new rule rejects, and lands the documentation and changelog.

The claim `add`, the ADR flip to `Implemented`, and this plan's freeze are not phases here: they
belong to the deferred post-review transaction (see Verification).

## File structure

- **Created:** none. The migration script of Phase 3 is written to an untracked working path and
  is deliberately never committed (ADR-0199 item 9).
- **Modified:** `internal/topic/markers.go`, `internal/topic/markers_test.go`,
  `internal/config/config_test.go`, every tracked `*_test.go` carrying a proof marker (Phase 3,
  set defined by command), the proof-marker fixtures in the test files Phase 4's build rejects,
  `.awf/agents-doc.yaml`, `.awf/domains/parts/invariants/current-state.md`, `.awf/docs/pitfalls.yaml`,
  `.awf/docs/glossary.yaml`, `.awf/agents/code-reviewer.yaml`, `.awf/docs/parts/roadmap/deferred.md`,
  `templates/skills/retrospective/SKILL.md.tmpl`, `changelog/CHANGELOG.md`, plus every rendered
  output `./x render` regenerates from those sources and `.awf/awf.lock`.
- **Deleted:** none.

## Phase 1: Back the two claims whose markers point at nothing

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. It runs entirely under the current checker: markers carry no name yet.

Context: `internal/config/config_test.go` ends with two proof markers below the file's final
closing brace, with no test beneath them. Commit 4c61356a added them there when it deleted
`TestWorkflowTelemetryConfigContract`. They currently satisfy `Backing: test` for
`config/configuration:config-serialization-owned` and
`config/migrations-and-locks:migration-ordering` while proving nothing.

- [ ] **Task 1.1: Write the backing test for `config/configuration:config-serialization-owned`.**
  In `internal/config/config_test.go`, add `TestConfigSerializationFunnelOwnsEncoding`. The claim
  states that the live `.awf/config.yaml` is constructed and mutated only through `internal/config`
  via `MarshalSkeleton`, `SetArrayMember`, `SetArray`, `SetMappingScalar`, `SetMappingInteger`, and
  `SetMappingString`, which share one encoding funnel at a two-space indent. Prove the operative
  half, the shared funnel:

  Build one config source that has a nested mapping and a nested array, then drive each of the six
  entry points over it in turn. For each result assert that a nested mapping child is indented by
  exactly two spaces relative to its parent and that a nested array item is indented consistently
  with it, comparing against exact expected byte strings rather than a regex, so a second
  hand-rolled encoder with a different indent or a different sequence style fails. Assert on all
  six in one test so that adding a seventh editor that bypasses the funnel is a visible omission.

  Do not attempt to prove the claim's "no other package hand-rolls config.yaml serialization"
  half by scanning imports: many packages legitimately import `gopkg.in/yaml.v3` for unrelated
  documents, so such a scan would be either false or vacuous. That half stays a review concern;
  the funnel half is what the marker backs.

  Place the proof marker `// invariant: config/configuration:config-serialization-owned`
  immediately above the `func` line. It carries no name yet; Phase 3 adds one.

- [ ] **Task 1.2: Write the backing test for `config/migrations-and-locks:migration-ordering`.**
  In `internal/config/config_test.go`, add `TestMigrationOrderingAscendingAndIdempotent`. Note that
  the migration registry lives in `internal/migrate`, so if importing it from `internal/config`
  would create an import cycle or otherwise fail to build, place this test in
  `internal/migrate/migrate_test.go` instead and put the marker there; the claim's topic
  `config/migrations-and-locks` must still match that path under its topic scope, which
  `awf topic config/migrations-and-locks` reports. Resolve that before writing the test, not after.

  The claim states that `awf upgrade` applies exactly the registered migrations whose target
  generation exceeds the project's detected generation, in ascending target order, and that
  re-running it at the current schema applies nothing and exits zero. Assert three things:

  1. Registry ordering: walking `registry` yields strictly ascending `To` values with no duplicate
     and no gap-in-reverse, so a migration appended out of order fails.
  2. Selection and order: from a project at a generation below `Current()`, `Upgrade` reports
     applied migrations whose targets are exactly the registered targets greater than that
     generation, in ascending order, and none at or below it.
  3. Idempotence: running `Upgrade` again on the resulting tree applies nothing, returns an empty
     applied list, and returns no error.

  Use the existing config and migration test helpers in the package rather than hand-building a
  tree. Place the proof marker `// invariant: config/migrations-and-locks:migration-ordering`
  immediately above the `func` line, with no name.

- [ ] **Task 1.3: Delete the two stranded markers.** Remove the final two lines of
  `internal/config/config_test.go`, which are the detached markers for
  `config/configuration:config-serialization-owned` and
  `config/migrations-and-locks:migration-ordering`, along with the blank lines separating them from
  the file's final closing brace. Verify that
  `grep -c "invariant: config/configuration:config-serialization-owned" internal/config/config_test.go`
  reports exactly one occurrence, which is the marker written in Task 1.1, and likewise for the
  migration-ordering claim against whichever file Task 1.2 chose.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the one
  phase-closing commit; it requires `awf check --staged` and `./x gate` to pass,
  enforced by a wired pre-commit hook or run manually first in a clone without one (checkable with `git config core.hooksPath`):

```commit
test(config): back two claims whose markers outlived their test
```

## Phase 2: Accept an optional proof-marker name

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. It changes what the payload parser accepts and nothing else: no marker in the tree
carries a name yet, and no check reads one.

- [ ] **Task 2.1: Split the payload regexes by marker kind.** In `internal/topic/markers.go`,
  replace the shared alternation at the `markerPayloadRE` declaration with a shared claim-id
  fragment and three per-kind expressions, so that a name is grammatically available only to an
  `invariant:` payload (ADR-0199 items 1 and 4):

```go
const claimIDPattern = `[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*`

var statePayloadRE = regexp.MustCompile(`^state: (` + claimIDPattern + `)$`)
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `)(?: \((.+)\))?$`)
var touchesPayloadRE = regexp.MustCompile(`^touches-state: (` + claimIDPattern + `) - (.+)$`)
```

  The `(.+)` name group is greedy by construction, so a name containing parentheses, such as the
  `it('strips the header')` form ADR-0199 item 2 requires to remain legal, captures through to the
  payload's final closing parenthesis rather than the first.

- [ ] **Task 2.2: Resolve each kind through its own expression.** In `resolveMarker`, replace the
  single `markerPayloadRE` match with an ordered attempt against `statePayloadRE` (kind
  `StateMarker`), then `proofPayloadRE` (kind `ProofMarker`), then `touchesPayloadRE` (kind
  `TouchesMarker`, note from its second group). A payload matching none of the three keeps
  returning the existing `%s:%d: malformed current-state marker %q` error unchanged. Capture the
  proof name into a local variable only; do not add a field to `MarkerSite`, which ADR-0199
  rejected for having no consumer. The name is parsed and discarded in this phase; Phase 4 gives
  it its consumer.

  Forbidden: changing the error text, the error's `path:line` prefix, or the relative order in
  which the three kinds are attempted in a way that would let a `touches-state:` payload match as
  a proof.

- [ ] **Task 2.3: Cover the new grammar.** In `internal/topic/markers_test.go`, add cases asserting
  that a proof marker with a name resolves without error and yields kind `ProofMarker` with the
  expected claim id; that a proof marker whose name itself contains parentheses resolves, using
  `invariant: alpha/contracts:stable (it('strips the header'))` as the exact payload; that a
  `state:` marker carrying a name, exact payload `state: alpha/contracts:rule (TestThing)`, fails
  with the existing malformed-marker error rather than any new one; and that an unnamed proof
  marker still resolves, since the name is optional in this phase. Run `go test ./internal/topic/...`
  and expect `ok`.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the one
  phase-closing commit; it requires `awf check --staged` and `./x gate` to pass,
  enforced by a wired pre-commit hook or run manually first in a clone without one (checkable with `git config core.hooksPath`):

```commit
feat(invariants): accept an optional name on a proof marker
```

## Phase 3: Migrate every proof marker to carry a name

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. It is a mechanical corpus sweep: no production behaviour changes, and the permissive
schema from Phase 2 accepts every intermediate state.

- [ ] **Task 3.1: Write the throwaway migration script.** Write it to an untracked working path
  outside the repository tree, or to a path covered by `.gitignore`; it is never committed
  (ADR-0199 item 9). It may use the Go AST or any other language-specific knowledge, because it
  runs once and is not part of the shipped checker.

  For each line in each tracked `*_test.go` file whose trimmed form matches
  `^// invariant: <claim-id>$`, the script derives a name and rewrites the line as
  `// invariant: <claim-id> (<name>)`, preserving the original leading whitespace exactly. Name
  derivation follows ADR-0199 item 10, nearest unique anchor first:

  1. If the marker sits directly above, or within a run of markers and comments directly above, a
     top-level `func (Test|Benchmark|Fuzz|Example)\w*\(` declaration, use that function's
     identifier.
  2. If the marker sits inside a function body directly above a composite-literal element that
     carries a leading string literal, such as a table row `{"remove block-scoped", ...}` or a map
     key, use that literal's contents when it occurs exactly once among the file's non-comment
     lines. Otherwise fall back to the enclosing function's identifier.
  3. Otherwise use the enclosing or following non-test function's identifier.

  Uniqueness in rules 1 to 3 means unique among the lines the Phase 4 check will search, that is,
  lines whose trimmed form does not begin with `//`. A marker with no enclosing and no following
  function has no anchor: the script must report it and change nothing, and no such marker should
  remain after Phase 1.

- [ ] **Task 3.2: Run the migration and confirm the corpus is uniformly named.** Run the script
  over the worktree. Then confirm that no unnamed proof marker survives: the command

```
grep -rEn '^[[:space:]]*// invariant: [a-z0-9/:-]+[[:space:]]*$' --include='*_test.go' .
```

  must produce no output and exit non-zero. Confirm the script itself was not added to the tree:
  `git status --porcelain` must list only modified `*_test.go` files and no untracked script.
  Then run `go test ./...` and expect every package to report `ok` or no test files, and run
  `./x check` and expect it clean; both must pass without any change to production code, which is
  the evidence that Phase 2's schema really was permissive.

- [ ] **Task 3.3: Spot-check the two shapes the script treats differently.** Confirm by reading
  that the twelve stacked markers above
  `TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion` in
  `internal/contextq/adapter_outputs_test.go` each now name that function, and that the in-body
  marker above the table row in `internal/config/edit_test.go` names the row's own string literal
  rather than its enclosing function. If either is wrong, fix the script and re-run rather than
  hand-editing the output, so the corpus stays uniform.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the one
  phase-closing commit; it requires `awf check --staged` and `./x gate` to pass,
  enforced by a wired pre-commit hook or run manually first in a clone without one (checkable with `git config core.hooksPath`):

```commit
refactor(invariants): name the proving unit on every proof marker
```

## Phase 4: Require the name and enforce its occurrence

**Execution mode: inline.** This phase is one independently green coherent implementation
transaction. It makes the name mandatory, adds the check, repairs the fixtures the check rejects,
and lands the documentation and changelog together, because this is the commit in which the
documented grammar actually changes.

- [ ] **Task 4.1: Make the name required.** In `internal/topic/markers.go`, change
  `proofPayloadRE` so the name group is no longer optional:

```go
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `) \((.+)\)$`)
```

  A proof marker with no name now fails the payload match. In `resolveMarker`, when a payload
  begins with `invariant: ` and matches `statePayloadRE`-style shape but not `proofPayloadRE`,
  return the new error `%s:%d: proof marker for %s does not name a proving unit` rather than the
  generic malformed-marker error, so the diagnostic points at the actual defect (ADR-0199 item 6).

- [ ] **Task 4.2: Add the occurrence check.** In `scanMarkerBytes`, hoist the line slice produced by
  `strings.Split(string(b), "\n")` into a named variable, collect each proof site resolved from
  this file together with its parsed name, and after the line loop verify each one. Add an
  unexported helper:

  `proofNameOccurs(lines []string, name string, markerLine int) bool` returns true when `name`
  occurs verbatim on some line other than `markerLine` whose `strings.TrimSpace` form does not
  begin with the source family's marker token, and where the match is not immediately preceded or
  followed by a byte in `[A-Za-z0-9_]`. Because the marker token is the family's comment leader by
  construction, this excludes comments, and every marker line is a special case of a comment line
  (ADR-0199 item 3).

  Recognition must be syntactic and line-local: test the line's leading token only, never whether
  it resolves to a valid claim, so the exclusion introduces no error of its own and the scan keeps
  reporting the first failure in line order.

  When the helper returns false, return
  `%s:%d: proof marker for %s names %q, which does not occur in this file; the test was deleted, renamed, or moved`.

  Forbidden: computing the exclusion from the resolved marker index, which would require a
  resolving pre-pass and could surface a later marker's error before an earlier missing-name one;
  and adding a `Proof` field to `MarkerSite`, which ADR-0199 rejected.

- [ ] **Task 4.3: Repair the proof-marker fixtures the new rule rejects.** Test fixtures that write
  a proof marker into a synthetic file must now carry a name whose text also appears on a
  non-comment line of that same synthetic file. Do not enumerate these by hand: run `go test ./...`
  and repair every failure it surfaces, repeating until the suite is green. The affected set is
  bounded by

```
grep -rln '"[^"]*// invariant:' --include='*.go' .
```

  Exact representative, in `internal/topic/markers_test.go`, inside `TestBuildMarkerIndex`, where
  the fixture is currently a file consisting only of a marker:

```go
testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable\n")
```

  becomes

```go
testsupport.WriteFile(t, filepath.Join(root, "internal/a_test.go"), "// invariant: alpha/contracts:stable (TestStable)\nfunc TestStable() {}\n")
```

  Exact edge, a fixture that must keep failing for its original reason rather than newly failing on
  the name rule: any fixture asserting that a proof marker outside `currentState.testGlobs` is
  rejected must still reach `proof marker is outside currentState.testGlobs`, so give it a valid
  name and a matching declaration line too, and assert on the original error text. Add one new case
  asserting the new failure directly: a fixture whose marker names a unit that appears nowhere in
  the file must fail with the Task 4.2 error, and a companion whose only occurrence of the name is
  inside a `//` comment must fail identically, which is the regression test for the 28% class.

- [ ] **Task 4.4: Update the documentation sources that specify the payload grammar.** Every one of
  these is a `.awf/` or `templates/` source; never hand-edit a rendered output. Change the marker
  grammar shown in each to the named form `invariant: <domain>/<topic>:<slug> (<name>)`, and where
  the surrounding prose explains what backing means, say that the named unit must occur on a
  non-comment line of the marker's own file:

  - `.awf/agents-doc.yaml`, the Backed invariants bullet
  - `.awf/domains/parts/invariants/current-state.md`, the three-marker-forms description
  - `.awf/docs/pitfalls.yaml`, the entry on an `invariant:` marker opening its own line
  - `.awf/docs/glossary.yaml`, the claim and invariant-backing entries
  - `.awf/agents/code-reviewer.yaml`, the proof-marker review guidance
  - `templates/skills/retrospective/SKILL.md.tmpl`, the shipped instruction that teaches adopters
    the grammar

  In `.awf/docs/parts/roadmap/deferred.md`, remove the second subclass of the nominal-proof entry,
  the one describing a proof marker outliving its test, since this plan resolves it; leave the
  nominal-proof subclass and its mutation-testing candidate intact. Then run `./x render` and stage
  every regenerated output together with `.awf/awf.lock`.

- [ ] **Task 4.5: Record the breaking change in the changelog.** In `changelog/CHANGELOG.md`, add
  an entry under the existing `## [Unreleased]` / `### Breaking changes` heading stating that every
  `invariant:` proof marker must now name the unit that proves it, that the named text must occur
  on a non-comment line of the marker's own file, that no schema bump or `awf upgrade` step signals
  the break because marker syntax is not part of the config schema, and that adopters migrate their
  own markers since deriving names requires language knowledge awf does not have. Match the voice
  of the neighbouring entries.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction and create the one
  phase-closing commit; it requires `awf check --staged` and `./x gate` to pass,
  enforced by a wired pre-commit hook or run manually first in a clone without one (checkable with `git config core.hooksPath`):

```commit
feat(invariants): fail a proof marker whose named unit is gone
```

## Verification

Whole-effort acceptance, beyond each phase's gate:

- Every proof marker in the tree carries a name: the Task 3.2 grep still produces no output.
- The check actually fails on the drift it targets. Delete any one named test function in a
  scratch working copy, run `./x check`, and confirm it reports the Task 4.2 error at that
  marker's line. Restore the file afterwards with `git apply -R` or `git restore` from HEAD, not
  by re-typing it. This is the acceptance step that distinguishes a check that works from a check
  that merely compiles, and ADR-0199's history is the argument for running it: both blockers found
  during review were rules that read correctly and did not hold.
- A stacked-marker site is genuinely protected: delete
  `TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion` in a scratch working copy and
  confirm `./x check` reports twelve failures rather than passing, then restore.
- `./x gate` is clean at every phase boundary, and `git log --oneline` shows four commits whose
  order matches the four phases.

Not part of this plan, and owned by the deferred post-review transaction after terminal review
settles: authoring the claim `invariants/topics-and-markers:proof-marker-names-its-unit` with its
`Backing: test` prose and its own named proof marker, appending ADR-0199's Applied event for that
single `add`, flipping ADR-0199 to `Implemented`, and freezing this plan at `status: Implemented`
with its implementation findings.

## Notes

- The `internal/config/config_test.go` markers this plan repairs in Phase 1 are the last two
  survivors of commit 4c61356a; ADR-0198 repaired the other two.
- Phase 3's script is deliberately disposable. Do not generalise it into a committed tool: the
  separation between language knowledge used once to migrate and never to enforce is the whole
  adopter-facing argument of ADR-0199, and a committed migration tool would blur it.
- If Phase 4 reveals a fixture that cannot satisfy the name rule without contorting the test it
  belongs to, that is a finding worth recording here rather than working around silently, because
  it would be evidence the rule is harder to satisfy than the corpus census suggested.
