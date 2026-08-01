---
date: 2026-08-01
adrs: [205]
status: Implemented
---
# Plan: Enforce named proof markers

## Goal

Implement ADR-0205: make every `invariant:` proof marker name the unit that proves it, and make
`awf check` fail when that name no longer occurs on a searched line of the marker's own file, so a
marker can no longer outlive its test. Non-goal: detecting a marker whose named test exists but
does not exercise the claim, which is the separate nominal-proof problem `docs/roadmap.md` tracks.

## Architecture summary

Four phases, each independently green, sequenced so no commit leaves the corpus and the checker
disagreeing (ADR-0205 item 12).

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
  is deliberately never committed (ADR-0205 item 9).
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

  The six entry points do not share one signature, so the test cannot drive them uniformly.
  `MarshalSkeleton(s Skeleton) ([]byte, error)` at `internal/config/edit.go:50` takes no source
  bytes, while `SetArrayMember` (:60), `SetArray` (:101), `SetMappingScalar` (:143),
  `SetMappingInteger` (:175) and `SetMappingString` (:219) all take `src []byte`. Drive the five
  `src []byte` editors over one config source carrying a nested mapping and a nested array, and
  separately marshal a `Skeleton` holding the same nested shapes through `MarshalSkeleton`.

  Assert all six results against the same exact expected byte strings for nested-mapping indent
  and sequence style: a nested mapping child indented by exactly two spaces relative to its
  parent, and a nested array item indented consistently with it. Compare exact bytes rather than a
  regex, so a second hand-rolled encoder with a different indent or a different sequence style
  fails. Keep all six assertions in one test so that adding a seventh editor that bypasses the
  funnel is a visible omission.

  Do not attempt to prove the claim's "no other package hand-rolls config.yaml serialization"
  half by scanning imports: many packages legitimately import `gopkg.in/yaml.v3` for unrelated
  documents, so such a scan would be either false or vacuous. That half stays a review concern;
  the funnel half is what the marker backs.

  Place the proof marker `// invariant: config/configuration:config-serialization-owned`
  immediately above the `func` line. It carries no name yet; Phase 3 adds one.

- [ ] **Task 1.2: Write the backing test for `config/migrations-and-locks:migration-ordering`.**
  Add `TestMigrationOrderingAscendingAndIdempotent` to `internal/migrate/migrate_test.go`, not to
  `internal/config/config_test.go`. The registry lives in `internal/migrate`, which imports
  `internal/config` (`internal/migrate/adrformatv2.go:9`, `anchoredglobs.go:7`), and
  `internal/config/config_test.go` is `package config`, so putting the test there would be an
  import cycle. `internal/migrate/migrate_test.go` is also where the other
  `config/migrations-and-locks` proof markers already live.

  A proof marker has no topic-scope requirement to satisfy: `resolveMarker` checks topic scope
  only in its non-proof branch (`internal/topic/markers.go:206-215`), so the whole scope
  requirement for a proof marker is that its file matches `currentState.testGlobs`, which
  `**/*_test.go` does.

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

- [ ] **Task 1.3: Delete the two stranded markers.** Remove lines 741 to 744 of
  `internal/config/config_test.go`: the trailing blank line and both detached markers for
  `config/configuration:config-serialization-owned` and
  `config/migrations-and-locks:migration-ordering`, which sit at lines 742 and 744 with a blank
  line between them. The file must then end at its final closing brace, so its last non-blank line
  is `}`.

  Verify by terminal state rather than by counting occurrences:

  - `grep -n 'invariant: config/migrations-and-locks:migration-ordering' internal/config/config_test.go`
    returns no output and exits non-zero, since that claim's marker now lives in
    `internal/migrate/migrate_test.go`.
  - `./x check` is clean, which is what actually proves both claims still have backing.

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
  `invariant:` payload (ADR-0205 items 1 and 4):

```go
const claimIDPattern = `[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*:[a-z0-9]+(?:-[a-z0-9]+)*`

var statePayloadRE = regexp.MustCompile(`^state: (` + claimIDPattern + `)$`)
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `)(?: \((.+)\))?$`)
var touchesPayloadRE = regexp.MustCompile(`^touches-state: (` + claimIDPattern + `) - (.+)$`)
```

  The `(.+)` name group is greedy by construction, so a name containing parentheses, such as the
  `it('strips the header')` form ADR-0205 item 2 requires to remain legal, captures through to the
  payload's final closing parenthesis rather than the first.

- [ ] **Task 2.2: Resolve each kind through its own expression.** In `resolveMarker`, replace the
  single `markerPayloadRE` match with an ordered attempt against `statePayloadRE` (kind
  `StateMarker`), then `proofPayloadRE` (kind `ProofMarker`), then `touchesPayloadRE` (kind
  `TouchesMarker`, note from its second group). A payload matching none of the three keeps
  returning the existing `%s:%d: malformed current-state marker %q` error unchanged.

  Do not read the name in this phase and do not assign it anywhere. `proofPayloadRE` simply
  carries a second group that `resolveMarker` never touches, and the existing `s.Kind` and
  `s.ClaimID` assignments stay as they are. Binding the name to a local variable here would not
  compile, because Go rejects a declared-and-unused variable and `.golangci.yml` additionally
  enables `unused`, `ineffassign` and `wastedassign`. Do not add a field to `MarkerSite` either,
  which ADR-0205 rejected for having no consumer. Phase 4 gives the group its first reader.

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
  (ADR-0205 item 9). It may use the Go AST or any other language-specific knowledge, because it
  runs once and is not part of the shipped checker.

  The tracked `*_test.go` set is repo-root-wide and includes `examples/`, whose bundled sundial
  adopter carries its own proof markers.

  For each line in each tracked `*_test.go` file whose trimmed form matches
  `^// invariant: <claim-id>$`, the script derives a name and rewrites the line as
  `// invariant: <claim-id> (<name>)`, preserving the original leading whitespace exactly. Name
  derivation follows ADR-0205 item 10, nearest unique anchor first:

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
  that every consecutive marker in the block above
  `TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion` in
  `internal/contextq/adapter_outputs_test.go` now names that function, and that the in-body
  marker above the table row in `internal/config/edit_test.go` names the row's own string literal
  rather than its enclosing function. Confirm as a third check that the sundial adopter's marker at
  `examples/sundial/internal/almanac/almanac_test.go` carries a name. The root marker walk skips
  that tree because it contains its own `.awf` directory (`internal/topic/markers.go:68`), so
  sundial is checked only by the nested runs `./x` makes with sundial as its own root (`x:90` and
  `x:99`) and by the pre-commit hook's own sliced check. Neither `awf check --staged` nor
  `./x gate` scans it, which makes Task 3.2's `./x check` the only step in this plan that would
  catch a marker missed there. If any of the three spot-checks is wrong, fix the script and re-run
  rather than hand-editing the output, so the corpus stays uniform.

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
var proofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `) \((\S(?:.*\S)?)\)$`)
var unnamedProofPayloadRE = regexp.MustCompile(`^invariant: (` + claimIDPattern + `)(?: \(.*\))?$`)
```

  The fallback deliberately also matches a padded or empty parenthetical, so
  `invariant: <id> ( TestFoo )` and `invariant: <id> ()` reach the named diagnostic below rather
  than falling through to the generic malformed-marker error. Ordering makes this safe:
  `proofPayloadRE` is attempted first, so a well-formed named marker never reaches the fallback.

  The name group stays greedy, so ADR-0205 item 2's `it('strips the header')` case still captures
  through to the payload's final closing parenthesis. Requiring a non-space first and last
  character implements item 2's "no leading or trailing whitespace" requirement here, at parse
  time, rather than letting ` TestFoo ` reach the occurrence check and be reported as a missing
  unit, which would be the most confusing possible diagnostic for the most likely authoring slip.

  In `resolveMarker`, branch on `unnamedProofPayloadRE` after `proofPayloadRE` fails, and use its
  group 1 as the claim id in the new error
  `%s:%d: proof marker for %s does not name a proving unit`, so the diagnostic points at the
  actual defect rather than falling through to the generic malformed-marker error (ADR-0205 item
  6). Both new statements must be exercised by Task 4.3's cases, since the coverage gate admits no
  unexercised branch.

- [ ] **Task 4.2: Add the occurrence check.** First widen the seam that carries the name out of
  parsing. Change `resolveMarker` to return `(MarkerSite, string, error)`, the string being the
  parsed proof name and empty for `StateMarker` and `TouchesMarker`. It has exactly one caller,
  at `internal/topic/markers.go:132`, so the change is local, and both `scanMarkerBytes` entry
  paths (the walker at `markers.go:87` and the snapshot loader at `tree.go:111`) inherit the
  result through that one call site, which is what ADR-0205 item 5 requires.

  In `scanMarkerBytes`, hoist the line slice produced by `strings.Split(string(b), "\n")` into a
  named variable before the loop, and verify each proof site inline, at the point it resolves
  inside the loop. Do not collect sites and verify after the loop: the hoisted slice is already
  complete before the first iteration, and deferring would put every resolve error in the file
  ahead of every occurrence error, so a malformed marker late in the file would be reported before
  a missing-name failure early in it. ADR-0205 item 3 requires the scan to keep reporting the
  first failure in line order. Add an unexported helper:

  `proofNameOccurs(lines []string, name, marker string, markerLine int) bool` returns true when
  `name` occurs verbatim on some line other than `markerLine` whose `strings.TrimSpace` form does
  not begin with `marker`, and where the match is not immediately preceded or followed by a byte
  in `[A-Za-z0-9_]`. The marker token must be threaded in as a parameter, taken from `src.Marker`
  in the `for _, src := range sources` loop the caller is already inside; it cannot be hardcoded,
  because the exclusion is per-family (`//`, `#`, `<!--`). Because the marker token is the family's comment leader by
  construction, this excludes comments, and every marker line is a special case of a comment line
  (ADR-0205 item 3).

  Recognition must be syntactic and line-local: test the line's leading token only, never whether
  it resolves to a valid claim, so the exclusion introduces no error of its own and the scan keeps
  reporting the first failure in line order.

  When the helper returns false, return
  `%s:%d: proof marker for %s names %q, which does not occur in this file; the test was deleted, renamed, or moved`.

  Forbidden: computing the exclusion from the resolved marker index, which would require a
  resolving pre-pass and could surface a later marker's error before an earlier missing-name one;
  introducing any collection or second pass over the file, for the same reason; and adding a
  `Proof` field to `MarkerSite`, which ADR-0205 rejected.

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
  name and a matching declaration line too, and assert on the original error text.

  Then add four new cases, one per load-bearing mechanism, each asserting the exact Task 4.2 error
  text:

  1. **Name absent.** A marker naming a unit that appears nowhere in the file fails.
  2. **Name only in a comment.** The name's sole occurrence is inside a `//` comment; this is the
     regression test for the 28% class the comment exclusion closes.
  3. **Flanking, the rename case.** A marker names `TestFoo` in a file whose only code occurrence
     is `func TestFooBar(t *testing.T) {}`. Without the flanking condition this would pass, and it
     is the precise mechanism that catches a rename, which is the drift this effort exists for.
  4. **Stacked markers do not satisfy each other.** Two consecutive markers naming the *same*
     function, absent from the file; assert the error reports the first marker's line. They must
     name the same function: naming two different absent functions would fail on the `markerLine`
     exclusion alone and merely duplicate case 1, testing nothing about stacking. The second
     marker is unreachable in that run by design, since the scan returns on its first error, so
     the case pins the comment exclusion rather than a pair of failures. This is the twelve-claim
     silent-stranding failure mode ADR-0205 item 3 identifies.
  5. **Bare unnamed marker.** A payload `invariant: alpha/contracts:stable` with no parenthetical
     at all must fail with `does not name a proving unit`. Without this case
     `unnamedProofPayloadRE`'s error statement is never executed once the fixtures above are all
     named, and Phase 4 closes red on the 100% statement-coverage gate. Convert Phase 2's Task 2.3
     case asserting that an unnamed proof marker *resolves* into this one rather than deleting it,
     so the assertion is inverted rather than silently dropped.

  Also add a case pinning the Task 4.1 whitespace rejection: payload
  `invariant: alpha/contracts:stable ( TestStable )` must fail with
  `does not name a proving unit`, which the widened `unnamedProofPayloadRE` fallback is what makes
  true; without that widening it would fall through to the malformed-marker error instead.

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
  that merely compiles, and ADR-0205's history is the argument for running it: both blockers found
  during review were rules that read correctly and did not hold.
- A stacked-marker site is genuinely protected: delete
  `TestGeneratedAdapterRuntimeOwnershipContextAndCoverageExclusion` in a scratch working copy and
  confirm `./x check` fails, reporting `internal/contextq/adapter_outputs_test.go:17`, the first of
  the twelve stacked markers. Reporting that first marker is what proves the neighbours no longer
  satisfy each other. Do not expect twelve reported failures: `scanMarkerBytes` returns on its
  first error, so one failure per run is the designed behaviour. Restore afterwards.
- `./x gate` is clean at every phase boundary, and the implementation series is six commits: the
  four phase commits in order, plus the two repair commits execution surfaced and the user
  authorised before Phase 3 (see Notes). Review settlement and the deferred post-review
  transaction add further commits outside that list, so count the series, not the log.

Not part of this plan, and owned by the deferred post-review transaction after terminal review
settles: authoring the claim `invariants/topics-and-markers:proof-marker-names-its-unit` with its
`Backing: test` prose and its own named proof marker, appending ADR-0205's Applied event for that
single `add`, flipping ADR-0205 to `Implemented`, and freezing this plan at `status: Implemented`
with its implementation findings.

## Notes

- The `internal/config/config_test.go` markers this plan repairs in Phase 1 are the last two
  survivors of commit 4c61356a; ADR-0198 repaired the other two.
- Phase 3's script is deliberately disposable. Do not generalise it into a committed tool: the
  separation between language knowledge used once to migrate and never to enforce is the whole
  adopter-facing argument of ADR-0205, and a committed migration tool would blur it.
- If Phase 4 reveals a fixture that cannot satisfy the name rule without contorting the test it
  belongs to, that is a finding worth recording here rather than working around silently, because
  it would be evidence the rule is harder to satisfy than the corpus census suggested. No such
  fixture appeared: all nine took a name and a matching declaration without changing what they
  assert.

- **Execution deviation, two authorised repair commits.** Phase 3's migration surfaced thirteen
  orphaned proof markers in `internal/project/target_test.go:519-549`, stranded by the same commit
  4c61356a this plan already names, bringing that commit's total to seventeen rather than four.
  The plan anticipated only anchorless orphans, which the script reports and skips; these had a
  following function, so the naming rules would have handed each the plausible name of the helper
  `renderPiExtensionFile` and frozen a known stranding green forever. The user chose deletion in
  its own commit before the sweep (6f69c9f6): every affected claim retains another proof marker,
  so no claim lost backing. A repo-wide sweep for comments naming an absent test found four more
  cases carrying no marker, fixed separately in 14706ee3.

- **Migration deviation found at the third review pass, with its cause.** Task 3.1's rule 2 is
  scoped to a marker "inside a function body" above a composite-literal element with a leading
  string literal. Two shapes fall outside that scope and the script therefore fell through to
  rule 3, naming a function where a finer unique anchor existed: a marker above a row of a
  PACKAGE-LEVEL table (three in `internal/project/spine_test.go`, whose rows are keyed structs, so
  neither the in-body nor the leading-literal condition held), and a marker above a `t.Run` label
  (`internal/project/inplace_test.go:17`, and `:47` in the same function, found by the sweep one
  round later). All five were re-anchored to their own row literal or subtest label and each was
  mutation-confirmed to fail once that row or label is removed. One further marker named the wrong
  declaration outright: `internal/adr/corpus_test.go` carried a doc comment for
  `TestCorpusParsedOnce` above the pure helper `parseDirProblems`, marker included. A seventh,
  `internal/project/docs_sections_test.go`, named its own test correctly but was independently
  satisfied by a TRAILING comment on a code line, which is a boundary of the shipped rule rather
  than a migration slip; ADR-0205 item 3 now records it. The correct census for f1397870 is 427
  above a test function, 91 in a test body, 2 naming a helper, 2 naming their own table row, and
  3 above a package-level table row taking the consuming test's identifier, which is the defect
  this entry records; that partition closes at 525, where the commit's own message says 5
  helper-named and 2 row-named, counting those 3 as helpers.

  The sweep that found the last instance was AST-backed, replicated `proofNameOccurs` exactly, and
  was validated against the pre-settlement tree first, where it reproduced all six known defects
  before its silence on the rest was trusted. It covered doubly-backed claims too, not only the
  singly-backed ones the first correction round prioritised.

- **Wording correction at review.** Tasks 4.4 and 4.5 above prescribe "must occur on a non-comment
  line of the marker's own file". The shipped rule skips lines that open with the family's marker
  token, which equals "non-comment" only for a family whose token is its comment leader. Terminal
  review caught the overstatement and the settlement corrected all six documentation sources, the
  changelog, and ADR-0205 item 3; the task text above is left as the historical instruction.

- **Residual the rule does not close, recorded deliberately.** The check proves a marker names a
  live unit; it cannot judge whether that unit is topical. Six claims now hold exactly one proof
  marker each, all inside broad stacks above `TestCurrentStateOutputPlanMatchesTree` or
  `TestTargetDescriptorValidation` in `internal/project/output_plan_test.go`, whose subjects are
  unrelated to what the claims assert: `pi-child-process-safety`,
  `pi-implementation-state-boundary`, `pi-subagent-progress-context-isolation`,
  `pi-implementation-batch-exclusivity`, `pi-subagent-progress-rendering`, and
  `pi-subagent-progress-bounds`. For each, the substantive proof was among the thirteen deleted
  tests. They are ledger-backed by a structural test and are candidates for the mutation-testing
  rung the roadmap's retained nominal-proof entry describes.
