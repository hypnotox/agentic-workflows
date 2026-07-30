---
date: 2026-07-30
adrs: [0179]
status: Proposed
---
# Plan: Drop configurable severity and unify the finding rank

## Goal

Execute ADR-0179: remove the two `currentState` severity configuration keys and the `off` value, collapse
the surviving severity encodings into one shared two-member rank, replace `topic.CoveragePolicy`'s
per-check severity with explicit check selection, and retire the unranked claim-handshake severity. Design
and rationale live in ADR-0179; this plan is the execution record.

Non-goals: no new outcome-modeling claims, no change to `maxTopicsPerPath` or `maxClaimsPerTopic`, no
promotion of fan-out to error, and no path-owning global topic (recorded as a roadmap idea instead).

## Architecture summary

Four independently green transactions, ordered so that each phase's consumers exist before the phase that
removes what they replace.

Phase 1 creates `internal/severity` holding `Rank` and converts the two audit surfaces, which are the only
producers of the `warning` token. Phase 2 removes the config keys, so the ranks passed into coverage
evaluation become code constants instead of parsed strings, and ships the migration. Phase 3 converts
`internal/topic` to the shared rank and replaces the policy's severity fields with check selection, which
is what finally removes `off`. Phase 4 deletes the claim-handshake severity and freezes both records.

ADR-0179 applies incrementally: Phase 1 applies no operation, Phase 2 flips the ADR to Implementing and
applies the two config operations, Phase 3 applies the two evaluation operations, and Phase 4 applies the
last operation and flips both records to Implemented.

Rank ordering note: the shared `Rank` uses `Error` as its zero value, which inverts `internal/audit`'s
current `Warning = iota`. This is safe because no code compares ranks relationally and every audit
construction site names its constant; verify with the grep in Task 1.3 rather than assuming it.

## File structure

- **Created:**
  - `internal/severity/severity.go`
  - `internal/severity/severity_test.go`
  - `internal/migrate/dropseveritysettings.go`
  - `internal/migrate/dropseveritysettings_test.go`
- **Modified:**
  - `internal/audit/audit.go`, `internal/audit/git.go`
  - `cmd/repoaudit/main.go`, `cmd/repoaudit/main_test.go`
  - `cmd/awf/audit.go`, `cmd/awf/audit_test.go`, `cmd/awf/check_test.go`
  - `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/edit_test.go`
  - `internal/configspec/spec.go`
  - `internal/migrate/migrate.go`
  - `internal/project/project.go`, `internal/project/check.go`, `internal/project/currentstate.go`,
    `internal/project/context.go`
  - `internal/topic/coverage.go`, `internal/topic/coverage_test.go`
  - `internal/currentstate/check.go`, `internal/currentstate/transition.go`,
    `internal/currentstate/check_test.go`
  - `.awf/config.yaml`, `examples/sundial/.awf/config.yaml`
  - `.awf/domains/tooling.yaml`, `.awf/topics/metadata/tooling/audit-and-snapshots.yaml`,
    `.awf/topics/metadata/invariants/topics-and-markers.yaml`
  - `.awf/topics/parts/config/configuration/current-state.md`,
    `.awf/topics/parts/config/migrations-and-locks/current-state.md`,
    `.awf/topics/parts/tooling/audit-commands/current-state.md`,
    `.awf/topics/parts/invariants/topics-and-markers/current-state.md`,
    `.awf/topics/parts/invariants/current-state-authority/current-state.md`
  - `.awf/docs/glossary.yaml`, `.awf/docs/parts/roadmap/ideas.md`, `changelog/CHANGELOG.md`
  - `docs/decisions/0179-drop-configurable-severity-and-unify-the-finding-rank.md`, this plan
  - every file `./x render` regenerates in both trees, committed with its config change
- **Deleted:** none. `internal/audit.Severity`, `topic.CoverageSeverity`, `cmd/repoaudit`'s private
  `severity`, and `internal/currentstate.Severity` are removed from files that survive.

## Phase 1: Introduce the shared rank and convert the audit surfaces

**Execution mode: inline.** One independently green transaction. It applies no State-changes operation, so
ADR-0179 stays `Proposed` through this phase. Start from a clean working tree on
`awf/drop-severity-settings-and-unify-the-rank` with `./x check` and `./x gate` successful.

- [ ] **Task 1.1: Create `internal/severity/severity.go`.** Exact content:

  ```go
  // Package severity holds the one finding rank awf reports. It owns no other
  // concern: internal/audit and internal/topic both consume it and import each
  // other in neither direction, so housing the rank in either would make an
  // unrelated sibling depend on it purely to borrow a type (ADR-0179 item 4).
  package severity

  // Rank is how bad a produced finding is. There are exactly two, and Error is
  // the zero value so an accidentally-defaulted finding is reported rather than
  // silently downgraded. There is deliberately no suppressing value: a caller
  // that does not want a finding class does not request it (ADR-0179 item 2).
  type Rank int

  const (
  	// Error makes a consuming command exit nonzero.
  	Error Rank = iota
  	// Warn reports the finding without changing the exit code.
  	Warn
  )

  // String renders the rank exactly as it is spelled everywhere awf reports it.
  func (r Rank) String() string {
  	if r == Warn {
  		return "warn"
  	}
  	return "error"
  }
  ```

- [ ] **Task 1.2: Create `internal/severity/severity_test.go`.** Cover both branches of `String()` and pin
  the zero value, so the 100% floor is met by this package alone. Exact content:

  ```go
  package severity_test

  import (
  	"testing"

  	"github.com/hypnotox/agentic-workflows/internal/severity"
  )

  func TestRankString(t *testing.T) {
  	for _, tc := range []struct {
  		rank severity.Rank
  		want string
  	}{
  		{severity.Error, "error"},
  		{severity.Warn, "warn"},
  	} {
  		if got := tc.rank.String(); got != tc.want {
  			t.Fatalf("Rank(%d).String() = %q, want %q", tc.rank, got, tc.want)
  		}
  	}
  }

  func TestErrorIsZeroValue(t *testing.T) {
  	var zero severity.Rank
  	if zero != severity.Error {
  		t.Fatalf("zero Rank = %v, want Error", zero)
  	}
  }
  ```

- [ ] **Task 1.3: Prove no rank ordering dependence before converting.** Run, expecting no output from each:

  ```
  grep -rnE "Severity *(<|>|<=|>=)" --include='*.go' internal cmd | grep -v _test.go
  grep -rnE "(Error|Warning|Warn) *(<|>|<=|>=)" --include='*.go' internal/audit cmd/repoaudit | grep -v _test.go
  ```

  If either returns a line, stop: the enum reordering is unsafe and the phase needs a different rank
  ordering. Do not proceed on a non-empty result.

- [ ] **Task 1.4: Convert `internal/audit` to `severity.Rank`.** In `internal/audit/audit.go`, delete the
  `type Severity int` block, its `const` block, and its `String()` method (currently at `:23-35`), and add
  `"github.com/hypnotox/agentic-workflows/internal/severity"` to the import block. Then replace every
  reference within the package:
  - `Severity` as a type (the `Finding.Severity` field and the `scopeSeverity Severity` parameter at
    `:162`) becomes `severity.Rank`.
  - the bare constants `Error` and `Warning` become `severity.Error` and `severity.Warn`.

  Representative, at the `Finding` declaration:

  ```go
  type Finding struct {
  	Severity severity.Rank
  	Rule     string
  	Commit   string // short hash, "" for a branch-level finding
  	Subject  string
  	Detail   string
  }
  ```

  Edge, the keyed literal in `internal/audit/git.go:30` whose constant is package-qualified for the first
  time:

  ```go
  	return []Finding{{
  		Severity: severity.Error,
  		Rule:     "uncommitted-changes",
  ```

  Affected-site set: every match of `grep -rn "Severity\|\bWarning\b\|\bError\b" internal/audit/*.go`
  excluding `_test.go`. Post-check: `grep -rn "type Severity\|Warning" internal/audit/*.go | grep -v
  _test.go` returns no output, and `go build ./...` succeeds.

- [ ] **Task 1.5: Update the audit consumers outside the package.** `internal/project/check.go:628` and
  `:656` compare `f.Severity == audit.Error` and `== audit.Warning`; these become `severity.Error` and
  `severity.Warn`. `internal/project/currentstate.go:404` and `:409` construct `audit.Finding` with
  `Severity: audit.Warning` and `Severity: audit.Error`; these become `severity.Warn` and
  `severity.Error`. Add the `internal/severity` import to each file. `cmd/awf/audit.go` renders findings:
  update any `audit.Severity`, `audit.Error`, or `audit.Warning` reference the same way. Post-check:
  `grep -rn "audit\.Warning\|audit\.Severity\|audit\.Error" --include='*.go' internal cmd` returns no
  output.

- [ ] **Task 1.6: Convert `cmd/repoaudit` to the shared rank.** Delete the private `type severity int`
  block, its constants, and its `label()` method (currently `:24-35`), and delete the stale justification
  comment at `:22-23` that claims repoaudit avoids importing the type because it is standalone tooling: its
  production code already imports `internal/git`. Replace the `sev severity` field on the private `finding`
  struct with `sev severity.Rank`, every `errorSev`/warning constant with `severity.Error`/`severity.Warn`,
  and every `.label()` call with `.String()`. Add the `internal/severity` import. Post-check: `grep -n
  "label()\|errorSev\|type severity" cmd/repoaudit/main.go` returns no output.

- [ ] **Task 1.7: Update the golden output for the token change.** Both audit surfaces now render `warn`
  where they rendered `warning`. Update every affected expectation in `cmd/awf/audit_test.go` and
  `cmd/repoaudit/main_test.go`. Do not weaken an assertion to accommodate the change: if a test asserted
  the literal `warning`, it asserts the literal `warn`. Post-check: `go test ./cmd/... ./internal/audit/...`
  passes, and `grep -rn '"warning"' cmd/awf cmd/repoaudit internal/audit` returns no output.

- [ ] **Task 1.8: Give the new package domain ownership and scoped topic coverage.** Without both, the
  commit introducing the package emits an Uncovered finding, which is at error severity. Add
  `- internal/severity/**` to the `paths:` list in `.awf/domains/tooling.yaml`, keeping the file's existing
  order and inserting it adjacent to the other `internal/` entries. Add the same selector to the `paths:`
  list in `.awf/topics/metadata/tooling/audit-and-snapshots.yaml`:

  ```yaml
  paths:
    - internal/audit/**
    - internal/git/**
    - internal/severity/**
    - internal/snapshot/**
  ```

  That topic already carries claims, so it satisfies coverage; an empty topic shell would not, because
  `internal/topic/coverage.go:180` skips any topic with no claims. Then run `./x render` and stage every
  regenerated file it reports.

- [ ] **Task 1.9: Confirm coverage is clean for the new package.** Run `./awf context internal/severity`
  and confirm the group reports `Classification: covered` with `Domains: tooling` and
  `tooling/audit-and-snapshots` among its topics. Run `./awf check` and confirm it is clean.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
refactor(code-design): unify the audit finding rank
```

## Phase 2: Remove the severity configuration keys

**Execution mode: inline.** Start from a clean working tree with Phase 1 committed and `./x check` and
`./x gate` successful. This is ADR-0179's first incremental application transaction: it applies
`config/configuration:severity-not-configurable` and `config/migrations-and-locks:severity-keys-dropped`
in declaration order. The three remaining operations stay pending, so the Implementing state is legal.

- [ ] **Task 2.1: Remove the keys from the config model.** In `internal/config/config.go`: delete the
  `topicCoverage` and `topicFanout` cases from the `currentState` decode switch (currently `:126-135`),
  delete the `TopicCoverage` and `TopicFanout` fields (`:96-97`), delete the `coverageSet` and `fanoutSet`
  bool fields, and delete from `Validate` both the default assignments and the whole severity validation
  loop (currently `:541-556`). Because the decoder is strict, a surviving key in an adopter tree now
  hard-fails at load, which is what the migration in Task 2.4 exists to prevent. Post-check: `grep -n
  "TopicCoverage\|TopicFanout\|coverageSet\|fanoutSet" internal/config/config.go` returns no output.

- [ ] **Task 2.2: Remove both configspec entries.** In `internal/configspec/spec.go`, delete the
  `currentState.topicCoverage` entry (`:153-157`) and the `currentState.topicFanout` entry (`:158-162`),
  leaving the surrounding entries and their order untouched. Post-check: `grep -n "topicCoverage\|topicFanout"
  internal/configspec/spec.go` returns no output.

- [ ] **Task 2.3: Pass fixed ranks into coverage evaluation.** `internal/project/currentstate.go:449-450`
  currently casts the two config strings into `topic.CoverageSeverity`. Replace both with the constants the
  ADR fixes, leaving `MaxTopicsPerPath` sourced from config:

  ```go
  		Coverage:         topic.CoverageError,
  		Fanout:           topic.CoverageWarn,
  ```

  This deliberately keeps `topic.CoverageSeverity` for now; Phase 3 converts it. Post-check: `grep -n
  "topic.CoverageSeverity(" internal/project/*.go` returns no output.

- [ ] **Task 2.4: Add the migration.** Create `internal/migrate/dropseveritysettings.go`, following the
  `applyDropAuditBase` precedent: the keys are nested, so `config.RemoveMappingKey` is required and
  `config.RemoveKey` cannot be used, and the removal is announced because it deletes a value an adopter
  deliberately set. Exact content:

  ```go
  package migrate

  import (
  	"bytes"
  	"fmt"
  	"io"

  	"github.com/hypnotox/agentic-workflows/internal/config"
  )

  // applyDropSeveritySettings ports schema 23 -> 24: currentState.topicCoverage
  // and currentState.topicFanout are removed (ADR-0179), so topic coverage and
  // fan-out always evaluate at ranks fixed in code. config.yaml is strict-parsed,
  // so a surviving key would hard-fail on the new binary rather than warn. Each
  // removal is announced for the applyDropAuditBase reason: deleting a value an
  // adopter deliberately set must be readable from command output rather than
  // recovered by git archaeology. The edit routes through RemoveMappingKey because
  // both keys are nested under currentState, which RemoveKey cannot reach.
  func applyDropSeveritySettings(root string, w io.Writer) error {
  	return editConfig(root, func(src []byte) ([]byte, error) {
  		out := src
  		for _, key := range []string{"topicCoverage", "topicFanout"} {
  			next, err := config.RemoveMappingKey(out, "currentState", key)
  			if err != nil {
  				return nil, err
  			}
  			if !bytes.Equal(next, out) {
  				fmt.Fprintf(w, "drop-severity-settings: removed currentState.%s\n", key)
  			}
  			out = next
  		}
  		return out, nil
  	})
  }
  ```

  Register it as the new tip in `internal/migrate/migrate.go`, appended after the schema 23 entry:

  ```go
  	{To: 24, Name: "drop-severity-settings", Apply: applyDropSeveritySettings},
  ```

- [ ] **Task 2.5: Add the migration test.** Create `internal/migrate/dropseveritysettings_test.go`
  asserting, against a fixture tree at generation 23: both keys are removed; every other `currentState`
  key (`sources`, `testGlobs`, `maxTopicsPerPath`, `maxClaimsPerTopic`) survives byte-identically; one
  announcement line is written per key actually removed and none for a key already absent; and a tree with
  neither key is a clean no-op. Place the proof marker for the migration claim on the test that asserts
  removal plus intact siblings:

  ```go
  // invariant: config/migrations-and-locks:severity-keys-dropped
  ```

  Follow the existing per-migration `TestXIsCurrent` convention in the package if one applies to the tip
  migration. Post-check: `go test ./internal/migrate/...` passes.

- [ ] **Task 2.6: Set the schema version floor.** Add `24: "0.27.0"` to `minVersionBySchema` in
  `internal/project/project.go:42`. Reusing 0.27.0 is legal only because it is unreleased: the newest
  released section in `changelog/CHANGELOG.md` is `## [0.22.0]`, and 0.27.0 sits under `[Unreleased]`.
  Confirm that is still true before writing the entry; if 0.27.0 has shipped in the meantime, bump `Version`
  to `0.28.0` and map 24 to it instead. Leave `Version` unchanged in the unreleased case.

- [ ] **Task 2.7: Remove the keys from both in-repo trees.** Delete the `topicCoverage` and `topicFanout`
  lines from `.awf/config.yaml` (currently `:53-54`) and from `examples/sundial/.awf/config.yaml`
  (currently `:68-69`). awf is its own first adopter, so omitting the root tree would leave the project
  failing its own strict validation. Then run `./x render`, which re-renders both trees, and stage every
  file it reports including both locks.

- [ ] **Task 2.8: Migrate the affected config tests.** Four sites, each different:
  - `internal/config/config_test.go:477` `TestCurrentStateSeverityValidation` retires entirely.
  - the `topicCoverage`/`topicFanout` sub-cases inside `TestCurrentStateStrictValidation` (`:507`),
    `TestCurrentStateRejectsNonStringScalars` (`:591`), and `TestCurrentStateRejectsWrongValueTypes`
    (`:620`) drop; those tests survive with their `sources`, `testGlobs`, `maxTopicsPerPath`, and
    `maxClaimsPerTopic` cases intact. Do not delete the enclosing functions.
  - `internal/config/edit_test.go:123` is a comment-preservation case that anchors on `topicCoverage`
    purely as a YAML child and contains no severity behaviour. Re-anchor it on a surviving key, preserving
    what it actually tests:

    ```go
    		{"adds child preserving comment", "currentState:\n  maxTopicsPerPath: 8 # keep\n", "currentState:\n  maxTopicsPerPath: 8 # keep\n  maxClaimsPerTopic: 20\n", false},
    ```

  Place the proof marker for the config claim on the test asserting that a tree carrying either key is
  rejected:

  ```go
  // invariant: config/configuration:severity-not-configurable
  ```

- [ ] **Task 2.9: Migrate the check fixtures that used `off`.** `cmd/awf/check_test.go:115` hard-codes
  `topicFanout: off` inside the shared `coverageYAML` helper while parameterizing `topicCoverage`; the
  helper's severity parameter becomes dead, so remove the parameter and both keys from the emitted YAML and
  update its four callers (`:132`, `:149`, `:213`, `:229`) to call it with no argument. `:164` and `:244`
  set `topicCoverage: off` to suppress coverage entirely; each must now either supply real scoped topic
  coverage in its fixture or assert the Uncovered findings it produces. Choose per fixture by what the test
  is about, and do not suppress a finding by weakening an assertion. Post-check: `grep -rn "off" cmd/awf/check_test.go
  | grep -i "topic"` returns no output, and `go test ./cmd/awf/...` passes.

- [ ] **Task 2.10: Update the documentation this falsifies.** Reality and its documentation land in the
  same commit.
  - `.awf/docs/glossary.yaml`, the `topic coverage` entry: it currently says `currentState.topicCoverage`
    (default `error`), `currentState.topicFanout` (default `warn`), and `currentState.maxTopicsPerPath`
    "tune the severities". Rewrite so only `maxTopicsPerPath` remains, described as a budget rather than a
    severity, and state that coverage always evaluates at error and fan-out at warn.
  - `.awf/docs/parts/roadmap/ideas.md`: the idea proposing to promote the topic-claim-budget advisory to a
    configurable severity with an adopter-facing config key is now foreclosed. Withdraw the severity half
    and keep only what survives, namely that the budget threshold itself remains configurable.
  - `changelog/CHANGELOG.md`, under `## [Unreleased]` `### Breaking changes`: add an entry naming both
    removed keys, the fixed ranks, the removal of `off`, and schema generation 24 with `awf upgrade` as the
    migration path. Match the surrounding entries' voice and line width.
  - `internal/project/currentstate.go:26-28`, the doc comment describing coverage findings as each
    carrying "its configured severity, ADR-0134 item 11": correct it to the fixed ranks.
  - the `docs/config-reference.md` and `docs/glossary.md` regenerations in both trees come from `./x render`;
    stage them, never hand-edit them.

- [ ] **Task 2.11: Author the two claims and record the application.** In
  `.awf/topics/parts/config/configuration/current-state.md`, add in the file's existing alphabetical
  position:

  ```markdown
  ### `invariant: severity-not-configurable`

  The currentState configuration exposes no severity setting: topic coverage and topic fan-out always evaluate, coverage reporting at error and fan-out at warn, and a tree carrying a currentState.topicCoverage or currentState.topicFanout key is rejected by strict parsing rather than honoured.
  Origin: ADR-0179
  Backing: test
  ```

  In `.awf/topics/parts/config/migrations-and-locks/current-state.md`, add:

  ```markdown
  ### `invariant: severity-keys-dropped`

  Schema generation 24 removes currentState.topicCoverage and currentState.topicFanout from a config tree, announcing each removal it performs, and leaves every other configured key byte-identical.
  Origin: ADR-0179
  Backing: test
  ```

  In ADR-0179, change `status: Proposed` to `status: Implementing`, append the Implementing status event
  with the frozen content digest, then append one Applied event using the next `state-sequence` value
  reported at execution time and exactly the two operations
  `add config/configuration:severity-not-configurable` and
  `add config/migrations-and-locks:severity-keys-dropped`, in that declaration order. Do not hardcode or
  predict the repository-global sequence. Leave the other three operations unapplied. Run `./x render` and
  stage the regenerated topic documents, `docs/decisions/INDEX.md`, and the locks.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
feat(config)!: drop the currentState severity settings
```

## Phase 3: Convert topic coverage and split the policy

**Execution mode: inline.** Start from a clean working tree with Phase 2 committed and `./x check` and
`./x gate` successful. This is ADR-0179's second incremental application transaction: it applies
`tooling/audit-commands:severity-single-spelling` and
`invariants/topics-and-markers:coverage-evaluation-selects-checks`. This phase is what removes `off`, so
the single-spelling claim only becomes true here, not in Phase 1.

- [ ] **Task 3.1: Replace `CoverageSeverity` with the shared rank.** In `internal/topic/coverage.go`,
  delete the `type CoverageSeverity string` declaration and its `CoverageError`, `CoverageWarn`, and
  `CoverageOff` constants (currently `:56-65`), and add the `internal/severity` import. Change
  `CoverageFinding.Severity` to `severity.Rank`, preserving its existing `json:"severity"` tag so the JSON
  shape is unchanged in value as well as key: `severity.Rank` must marshal as its string spelling, not its
  integer. If `CoverageFinding` is marshalled anywhere, add a `MarshalJSON` on `Rank` in
  `internal/severity` returning the quoted `String()` value, with a test, rather than letting the integer
  leak into output. Determine which applies by running `grep -rn "CoverageFinding" --include='*.go' internal
  cmd | grep -v _test.go` and inspecting each consumer for JSON encoding.

- [ ] **Task 3.2: Replace the policy's severities with check selection.** In `internal/topic/coverage.go`,
  replace the `CoveragePolicy` declaration with:

  ```go
  // CoveragePolicy carries which coverage checks a caller wants evaluated and the
  // per-path fan-out budget. A caller that does not want a finding class does not
  // request it; no value suppresses a requested check (ADR-0179 items 2 and 8).
  type CoveragePolicy struct {
  	Coverage, Fanout bool
  	MaxTopicsPerPath int
  }
  ```

  In `EvaluateCoverage`, replace the two guards `if policy.Coverage != CoverageOff` and
  `if policy.Fanout != CoverageOff` with `if policy.Coverage` and `if policy.Fanout`, and set each
  finding's rank from the fixed constants rather than the policy:

  ```go
  				findings = append(findings, CoverageFinding{Path: path, Domain: d, Kind: Uncovered, Severity: severity.Error})
  ```

  ```go
  			findings = append(findings, CoverageFinding{Path: path, Kind: Fanout, Severity: severity.Warn, Topics: count})
  ```

  Update the `EvaluateCoverage` doc comment: the sentence "a CoverageOff severity suppresses its findings"
  is false once `off` is gone, and is replaced by the selection contract. Keep the existing sentences about
  per-owner gaps, global and claimless topics, and once-per-path fan-out counting, which are unchanged.

- [ ] **Task 3.3: Update both policy construction sites.** `internal/project/context.go:371` currently
  passes `topic.CoveragePolicy{Coverage: topic.CoverageError, Fanout: topic.CoverageOff}`, which used a
  rank as a control-flow switch. It becomes a request for coverage only, preserving the uncovered report's
  current output exactly:

  ```go
  	for _, f := range topic.EvaluateCoverage(corpus, scoped, topic.CoveragePolicy{Coverage: true, Fanout: false}) {
  ```

  `internal/project/currentstate.go:449-450`, set to the fixed constants in Task 2.3, becomes
  `Coverage: true, Fanout: true` with `MaxTopicsPerPath` still from config. Post-check: `grep -rn
  "CoverageOff\|CoverageError\|CoverageWarn" --include='*.go' internal cmd` returns no output.

- [ ] **Task 3.4: Update the remaining doc comments.** `internal/topic/coverage.go:53-55` documents
  `CoverageOff`'s suppression semantics; `internal/project/currentstate.go:51-52` says "Off findings are
  never emitted by the evaluator"; `internal/project/context.go:361-363` explains a forced coverage
  severity "regardless of the project's configured strictness" that no longer varies. Correct each to the
  shipped behaviour. Post-check: `grep -rn "CoverageOff\|configured strictness\|configured severity"
  --include='*.go' internal` returns no output.

- [ ] **Task 3.5: Update the topic tests and place both proof markers.** In
  `internal/topic/coverage_test.go`, convert every `CoverageSeverity` reference and every policy literal to
  the new shape. Add or extend a test asserting the selection contract: a policy requesting coverage only
  produces no Fanout finding even when a path exceeds the budget, and a policy requesting fan-out only
  produces no Uncovered finding. Place:

  ```go
  // invariant: invariants/topics-and-markers:coverage-evaluation-selects-checks
  ```

  For the spelling claim, add a test asserting that every rank any awf surface renders is exactly `error` or
  `warn`. Assert it over the rank type's own values plus the audit and coverage finding renderings, and place
  in the test file whose package owns the assertion:

  ```go
  // invariant: tooling/audit-commands:severity-single-spelling
  ```

- [ ] **Task 3.6: Widen the destination topic's metadata summary.** The
  `invariants/topics-and-markers` summary describes parsing and resolution only and does not cover coverage
  evaluation policy, which the new claim is about. In
  `.awf/topics/metadata/invariants/topics-and-markers.yaml`, extend `summary:` to name coverage evaluation
  policy alongside topic input, claim, and marker parsing and resolution. Then run `./x render` and stage
  the regenerated topic documents.

- [ ] **Task 3.7: Author the two claims and record the application.** In
  `.awf/topics/parts/tooling/audit-commands/current-state.md`, add in the file's existing alphabetical
  position:

  ```markdown
  ### `invariant: severity-single-spelling`

  Every finding rank awf reports renders as exactly error or warn: one shared two-member rank backs the audit findings, the repo-local audit tool, and current-state topic coverage, and no surface renders warning.
  Origin: ADR-0179
  Backing: test
  ```

  In `.awf/topics/parts/invariants/topics-and-markers/current-state.md`, add:

  ```markdown
  ### `invariant: coverage-evaluation-selects-checks`

  A coverage evaluation caller selects which checks run: coverage and fan-out are requested independently, no rank value suppresses a requested finding, and the uncovered report requests coverage only.
  Origin: ADR-0179
  Backing: test
  ```

  In ADR-0179, append one Applied event using the next `state-sequence` reported at execution time and
  exactly `add tooling/audit-commands:severity-single-spelling` and
  `add invariants/topics-and-markers:coverage-evaluation-selects-checks`, in declaration order. Do not
  hardcode or predict the sequence. The ADR stays `Implementing`; one operation remains.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
refactor(code-design): request coverage checks explicitly
```

## Phase 4: Retire the claim-handshake rank and freeze

**Execution mode: inline.** Start from a clean working tree with Phase 3 committed and `./x check` and
`./x gate` successful. This is ADR-0179's final application transaction: it applies
`invariants/current-state-authority:currentstate-handshake-findings-unranked`, then flips ADR-0179 and this
plan to Implemented in the same green commit.

- [ ] **Task 4.1: Remove the claim-handshake severity.** In `internal/currentstate/check.go`, delete the
  `type Severity int` declaration, its `Error`/`Warn` constants, and its `String()` method (currently
  `:13-25`), and delete the `Severity` field from `Finding`, leaving:

  ```go
  type Finding struct {
  	Message string
  }
  ```

- [ ] **Task 4.2: Convert every finding construction site in the package.** Batch task. Every literal
  currently passes `Error` positionally and becomes a single-field literal. Representative, from
  `internal/currentstate/check.go:95`:

  ```go
  			findings = append(findings, Finding{err.Error()})
  ```

  Edge, a returned slice literal with a formatted message, from `:242`:

  ```go
  		return []Finding{{fmt.Sprintf("ADR-%s operation %s targets missing topic %s", a.Number, op.Verb, topicID)}}
  ```

  Affected-site set: every match of `grep -n "Finding{" internal/currentstate/check.go
  internal/currentstate/transition.go`. Both files are in scope; `transition.go` is easy to miss because the
  type is declared in `check.go`. Post-check: `grep -rn "Finding{Error\|{Error, " internal/currentstate/` returns
  no output, and `go build ./...` succeeds.

- [ ] **Task 4.3: Simplify the two consumers.** `internal/project/currentstate.go:409` maps every
  current-state finding to `audit.Finding{Severity: severity.Error, ...}` using `f.Message`; it keeps doing
  exactly that, with the now-absent source field simply not read. `:41` already reads only `f.Message`.
  Confirm neither reads a severity from the current-state finding: `grep -n "\.Severity" internal/project/currentstate.go`
  must show only `audit.Finding` and `topic.CoverageFinding` uses. Update the `CurrentStateReport` doc
  comment at `:26` if it describes a rank the handshake findings no longer carry.

- [ ] **Task 4.4: Retire the dead test and place the proof marker.** `TestSeverityString` at
  `internal/currentstate/check_test.go:480` was the only consumer of `Warn` and retires with the type. Add
  or extend a test asserting that every finding the package produces is reported without a rank and that
  the check path treats each as blocking, and place:

  ```go
  // invariant: invariants/current-state-authority:currentstate-handshake-findings-unranked
  ```

  Post-check: `go test ./internal/currentstate/... ./internal/project/...` passes.

- [ ] **Task 4.5: Author the final claim.** In
  `.awf/topics/parts/invariants/current-state-authority/current-state.md`, add in the file's existing
  alphabetical position:

  ```markdown
  ### `invariant: currentstate-handshake-findings-unranked`

  A current-state claim-handshake finding carries no rank: every provenance and transition finding the current-state checker produces is blocking, and the check path reports each by message with no severity field. The ranked coverage and fan-out findings the project report also carries are a separate concern and keep their ranks.
  Origin: ADR-0179
  Backing: test
  ```

- [ ] **Task 4.6: Flip both records.** In ADR-0179, append the final Applied event using the next
  `state-sequence` reported at execution time and only
  `add invariants/current-state-authority:currentstate-handshake-findings-unranked`, then append the
  Implemented status event with the frozen content digest and change frontmatter `status: Implementing` to
  `status: Implemented`. In this plan, record any execution deviation under Notes and change
  `status: Proposed` to `status: Implemented`; do not edit it after this commit. Run `./x render` and stage
  the regenerated `docs/decisions/INDEX.md`, topic documents, and locks.

- [ ] **Phase-close: stage, check, gate, and commit.** Stage the complete transaction; run
  `awf check --staged` then `./x gate`; create the one phase-closing commit:

```commit
refactor(invariants): retire the claim-handshake finding rank
```

## Verification

Beyond each phase's `awf check --staged` and `./x gate`:

- `grep -rnE "type (Severity|CoverageSeverity) |type severity " --include='*.go' internal cmd | grep -v _test.go`
  returns no output: one rank type remains.
- `grep -rn "warning" --include='*.go' internal cmd | grep -v _test.go` returns no line where the token is a
  finding rank. Unrelated uses in `x` and the Pi extension template are out of scope and stay.
- `grep -rn "topicCoverage\|topicFanout" . --exclude-dir=.git` returns matches only in retained history:
  released changelog sections, `docs/decisions/`, and `docs/plans/`. No match in `.awf/config.yaml`,
  `examples/sundial/.awf/config.yaml`, `internal/`, or `cmd/`.
- `./awf context internal/severity` reports the group as covered, owned by `tooling`, with
  `tooling/audit-and-snapshots` among its topics.
- Applied against a fixture tree at generation 23, `awf upgrade` announces each key it removes, leaves every
  other `currentState` key byte-identical, and lands the tree at generation 24.
- `./awf check` is clean in both the root tree and `examples/sundial`.
- ADR-0179's Status history carries one Implementing event and Applied events covering all five declared
  operations exactly once each, in declaration order.

## Notes

Out of scope, recorded during authoring:

- Letting a global topic carry path selectors, so a shared-pattern holder like `internal/severity` could be
  owned by the global topic describing its pattern rather than by whichever scoped topic is nearest. Phase 1
  Task 1.8 extends `tooling/audit-and-snapshots` precisely because that capability does not exist. Recorded
  as a roadmap idea needing its own ADR.
- The outcome-modeling pattern topic that ADR-0179 unblocks. Its shape is settled but it is a separate
  effort, and two of the claims it would otherwise need retire with this plan.
- `internal/prosegate` and `internal/memorycite` produce findings that carry no rank and are untouched here.
  Their duplicated pinned-exemption comparison is a separate known candidate.
