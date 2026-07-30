---
date: 2026-07-30
adrs: [0179]
status: Implemented
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
  - `internal/audit/audit.go`, `internal/audit/git.go`, `internal/audit/audit_test.go`,
    `internal/audit/git_test.go`
  - `cmd/repoaudit/main.go`, `cmd/repoaudit/main_test.go`
  - `cmd/awf/audit.go`, `cmd/awf/audit_test.go`, `cmd/awf/check_test.go`, `cmd/awf/checkgroup_test.go`
  - `internal/config/config.go`, `internal/config/config_test.go`, `internal/config/edit_test.go`
  - `internal/configspec/spec.go`, `internal/configspec/spec_test.go`
  - `internal/migrate/migrate.go`, `internal/migrate/dropworkflowtelemetry_test.go`,
    `internal/migrate/workflowtelemetry_test.go`
  - `internal/project/project.go`, `internal/project/version_test.go`, `internal/project/check.go`,
    `internal/project/currentstate.go`, `internal/project/currentstate_test.go`,
    `internal/project/context.go`, `internal/project/configreference.go`,
    `internal/project/configreference_test.go`, `internal/project/staged_test.go`
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
  `type Severity int` block, its `const` block, and its `String()` method together with the doc comment
  above the type (currently `:22-35`; starting at `:23` orphans the comment above an unrelated
  declaration), and add
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
  block, its constants, and its `label()` method (currently `:24-36`; the method's closing brace is at
  `:36`, so stopping at `:35` leaves a stray `}`), and delete the stale justification
  comment at `:22-23` that claims repoaudit avoids importing the type because it is standalone tooling: its
  production code already imports `internal/git`. Replace the `sev severity` field on the private `finding`
  struct with `sev severity.Rank`, every `errorSev`/warning constant with `severity.Error`/`severity.Warn`,
  and every `.label()` call with `.String()`. Add the `internal/severity` import. Post-check: `grep -n
  "label()\|errorSev\|type severity" cmd/repoaudit/main.go` returns no output.

- [ ] **Task 1.7: Update the tests the conversion touches.** Both audit surfaces now render `warn` where
  they rendered `warning`, and the in-package tests reference the deleted types directly, so they do not
  compile until converted. Do not weaken an assertion to accommodate the change: if a test asserted the
  literal `warning` as a rank, it asserts the literal `warn`.
  - `internal/audit/audit_test.go`: `TestSeverityString` at `:13-20` retires, superseded by
    `internal/severity/severity_test.go` from Task 1.2. Retype `countRule`'s `sev Severity` parameter at
    `:23` to `severity.Rank`. Convert every bare `Error`/`Warning` constant: use
    `grep -n '\bError\b\|\bWarning\b' internal/audit/audit_test.go` as the affected-site set rather than a
    line list, since the constants appear at more sites than a first read suggests.
  - `internal/audit/git_test.go:327`: convert the bare constant.
  - `cmd/repoaudit/main_test.go`: `TestSeverityLabel` at `:37-40` retires with the `label()` method Task 1.6
    deletes. Update the remaining rank expectations from `warning` to `warn`.
  - `internal/project/staged_test.go`: convert `audit.Error`/`audit.Warning` at `:617`, `:658`, `:685`,
    and `:746`.
  - `cmd/awf/audit_test.go`: update every rendered-rank expectation.

  Post-check: `go test ./cmd/... ./internal/audit/... ./internal/project/... ./internal/severity/...`
  passes, and `grep -rn '"warning"' cmd/awf cmd/repoaudit internal/audit` returns no output.

- [ ] **Task 1.8: Give the new package domain ownership and scoped topic coverage.** Without both, the
  commit introducing the package emits an Uncovered finding, which is at error severity. In
  `.awf/domains/tooling.yaml`, insert `  - internal/severity/**` immediately after `  - internal/audit/**`.
  Rendered selector lists are sorted, so the insertion point affects no rendered byte; the exact anchor is an
  authoring convenience that keeps the source file readable. Add the same selector to the `paths:`
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
  loop (currently `:542-556`). The `if c.CurrentState != nil {` guard at `:541` must SURVIVE: it also guards
  the maxima positivity loop at `:557-566`. Because the decoder is strict, a surviving key in an adopter tree
  now hard-fails at load, which is what the migration in Task 2.4 exists to prevent.

  The generated config reference also reads both fields and will not compile once they are gone. In
  `internal/project/configreference.go`, delete the `case "currentState.topicCoverage":` and
  `case "currentState.topicFanout":` arms (currently `:141-150`), leaving the surrounding arms untouched. In
  `internal/project/configreference_test.go`, delete the two fixture lines at `:200-201` and the two
  expected table rows at `:231-232`.

  Post-check: `grep -rn "TopicCoverage\|TopicFanout\|coverageSet\|fanoutSet" --include='*.go' internal cmd`
  returns no output, and `go build ./...` succeeds.

- [ ] **Task 2.2: Remove both configspec entries.** In `internal/configspec/spec.go`, delete the
  `currentState.topicCoverage` entry (`:153-157`) and the `currentState.topicFanout` entry (`:158-162`),
  leaving the surrounding entries and their order untouched. Also correct the surviving
  `currentState.maxTopicsPerPath` description at `:165`, which says the budget is exceeded "before the
  configured fan-out finding is emitted": drop "configured", since the rank is now fixed. That string renders
  into `docs/config-reference.md` in both trees. Post-check: `grep -n "topicCoverage\|topicFanout\|configured
  fan-out" internal/configspec/spec.go` returns no output.

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

- [ ] **Task 2.6: Set the schema version floor and re-point the generation-pinned assertions.** Bump
  `Version` in `internal/project/project.go:31` to `"0.28.0"` and add `24: "0.28.0"` to
  `minVersionBySchema` at `:42`. Reusing 0.27.0 would be legal, since it is unreleased (the newest released
  section in `changelog/CHANGELOG.md` is `## [0.22.0]`), but generations 19 through 23 each took their own
  unreleased minor, and reuse would leave `internal/project/version_test.go:15` pinned at generation 23,
  where its "highest mapped generation equals `Version`" assertion silently stops meaning anything.

  Four existing assertions pin generation 23 as the tip and fail the moment migration 24 registers. Update
  all four:
  - `internal/migrate/dropworkflowtelemetry_test.go:11`: `if Current() != 23` becomes `24`.
  - `internal/migrate/workflowtelemetry_test.go:64`: append `,drop-severity-settings` to the pinned joined
    applied-migration list, which currently ends `...,implementer-agent-closure`.
  - `internal/project/version_test.go:15`: re-point `minVersionBySchema[23]` to `minVersionBySchema[24]`.
  - `internal/project/version_test.go:29`: the unmapped-generation case asserts
    `ValidateSchemaMinimumVersion(24, Version)` returns a "no minimum" error; re-point it from 24 to 25.

  Post-check: `go test ./internal/migrate/... ./internal/project/...` passes.

- [ ] **Task 2.7: Upgrade both in-repo trees before any render or check.** Registering `{To: 24}` makes
  `migrate.Current()` report 24 while both committed locks still record `"schemaVersion": 23`.
  `gateStateFor` in `internal/migrate/migrate.go:220` classifies a registered `To` landing in
  `(gen, current]` as `gate`, so `cmd/awf/gate.go` refuses EVERY gated command, including `./x render`,
  `./x check`, and `awf check --staged`, until both trees are upgraded. Run the upgrade before any of them,
  mirroring the same step in `docs/plans/2026-07-29-rendered-implementer-role-contract-adr-0177-part-a.md`:

  ```bash
  bindir="$(mktemp -d)"
  go build -o "$bindir/awf" ./cmd/awf
  ./awf upgrade
  (cd examples/sundial && "$bindir/awf" upgrade)
  rm -rf "$bindir"
  ```

  Expected terminal state: each invocation announces `drop-severity-settings: removed currentState.topicCoverage`
  and `... topicFanout`, and both `.awf/awf.lock` and `examples/sundial/.awf/awf.lock` stamp
  `"schemaVersion": 24` AND `"awfVersion": "0.28.0"`. The version restamp follows from Task 2.6 and is
  asserted by `internal/project/drift_test.go:327`, so it is an expected part of the lock diff rather than an
  unexplained change. Each `upgrade` ends in its own render, so the `./x render` below is a confirmation step
  rather than the thing that produces the rendered output. Let the migration delete the keys from
  `.awf/config.yaml` and
  `examples/sundial/.awf/config.yaml` rather than hand-editing them, so this transaction exercises the real
  migration on the trees it ships. awf is its own first adopter, so omitting the root tree would leave the
  project failing its own strict validation. Then run `./x render` and stage every file it reports, both
  locks included.

- [ ] **Task 2.8: Migrate the affected config tests.** Each site differs; work the list to its end rather
  than stopping at the first few:
  - `internal/config/config_test.go:477` `TestCurrentStateSeverityValidation` retires entirely.
  - `internal/config/config_test.go:462`, inside `TestCurrentStateDefaultsAndPresence`: drop the
    `cfg.CurrentState.TopicCoverage != "error" || cfg.CurrentState.TopicFanout != "warn"` conditions from the
    predicate, keeping the maxima assertions and the `config/configuration:topic-claim-budget-configured`
    proof marker at `:442` intact.
  - `internal/configspec/spec_test.go:152-153`, inside `TestCurrentStateKeysPublished`: delete the two
    hard-listed key paths without retiring the test.
  - Add a new strict-rejection case, which is what backs the config claim. In
    `TestCurrentStateStrictValidation`'s strict-nested table (the block at `internal/config/config_test.go:554-560`),
    add one case per removed key so a tree carrying either is rejected by the decoder's unknown-field path at
    `internal/config/config.go:148` (`field %s not found in type config.CurrentStateConfig`):

    ```go
    		{"prefix: x\ncurrentState:\n  topicCoverage: error\n", "topicCoverage"},
    		{"prefix: x\ncurrentState:\n  topicFanout: warn\n", "topicFanout"},
    ```

    Note the duplicate-key case at `:557` that currently names `topicCoverage` is dropped by this task, so
    without these two new cases nothing exercises the rejection and the claim would have no honest proof.
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

- [ ] **Task 2.9: Migrate the check fixtures that used `off`.** Batch task. The shared `coverageYAML` helper
  at `cmd/awf/check_test.go:113` hard-codes `topicFanout: off` while parameterizing `topicCoverage`, so its
  severity parameter becomes dead. Affected-site set: every match of `grep -rn 'coverageYAML(' cmd/awf`,
  which spans TWO files, `cmd/awf/check_test.go` and `cmd/awf/checkgroup_test.go`. Do not work from a
  hardcoded caller list.

  Make the helper parameterless, keeping a NON-EMPTY `currentState` block. Both constraints are load-bearing:
  coverage is only evaluated when the config declares a `currentState` block at all
  (`internal/project/currentstate.go:144` and `:195`), and a bare `currentState:` key with no children is a
  hard parse error (`internal/config/config.go:107`, "currentState must be a mapping"). Exact replacement
  body:

  ```go
  // coverageYAML owns internal/** with the fan-out budget the warn fixtures need.
  func coverageYAML() string {
  	return "prefix: example\nskills: [tdd]\nagents: []\ndomains: [alpha]\n" +
  		"currentState:\n  maxTopicsPerPath: 1\n"
  }
  ```

  A shared `maxTopicsPerPath: 1` is safe for the error callers: their fixtures match no scoped topic, so the
  fan-out count is 0 and `0 > 1` is false, meaning no Fanout finding appears. Update every caller to pass no
  argument.

  Three callers need more than a signature change. `TestRunCheckCurrentStateWarnNote`
  (`check_test.go:149`), `TestRunCheckStagedWarnNote` (`check_test.go:229`), and the
  `"state prints warn notes"` subtest of `TestCheckChildrenErrorPaths` (`checkgroup_test.go:346`) each exist
  to prove a warn-rank finding prints a note WITHOUT failing, and each got that from `coverageYAML("warn")`.
  Once coverage is fixed at error, the same fixture (`coverageFiles()` owns `internal/**` for domain alpha
  and supplies `internal/bar.go` with no scoped topic) produces an Uncovered finding at error, `runCheck`
  returns non-nil, and all three hit their `t.Fatalf` on a warn finding that must not fail. Re-base all three
  on fan-out, the one warn class that survives: give each fixture two path-scoped claim-bearing topics whose
  selectors both match the same file, so the count exceeds the budget of 1 and a Fanout finding is emitted at
  warn while coverage stays satisfied.

  Each re-based fixture needs, explicitly, because no helper supplies these:
  - two topic metadata files under `.awf/topics/metadata/alpha/`, each with a `paths:` selector matching the
    same file;
  - two matching parts under `.awf/topics/parts/alpha/<topic>/current-state.md`, each carrying at least one
    claim, since `internal/topic/coverage.go:180` skips a claimless topic. Use `rule:` claims, NOT
    `invariant:` claims: an invariant would additionally demand a `Backing:` line and a proof marker inside a
    fixture testGlobs file, which the fixture cannot supply, so it would fail to load. Mirror the part string
    already used at `cmd/awf/check_test.go:165`, with `Origin: ADR-0001`;
  - `docs/decisions/0001-one.md`, written by the fixture itself via
    `testsupport.ADR("Implemented", testsupport.WithTitle("0001: One"))`, for claim provenance.
    `syncedGitProjectFiles` (`cmd/awf/gitproject_test.go:36`) and `stagedCheckProject` write only the config
    and the files they are given; neither writes an ADR.

  Keep all three tests asserting the note channel and the verdicts they assert today. Do not suppress a
  finding by weakening an assertion.

  At `check_test.go:164` and `:244`, delete only the two key lines, keeping `maxClaimsPerTopic: 1` so the
  `currentState` block stays non-nil. Both are claim-budget fixtures that already declare a claim-bearing
  path-scoped topic and contain no file matching `internal/**`, so no coverage or fan-out finding can arise
  either way and no further fixture work is needed there.

  Post-check: `grep -rn "topicCoverage\|topicFanout" cmd/awf` returns no output, and `go test ./cmd/awf/...`
  passes.

- [ ] **Task 2.10: Update the documentation this falsifies.** Reality and its documentation land in the
  same commit.
  - `.awf/docs/glossary.yaml`, the `topic coverage` entry: it currently says `currentState.topicCoverage`
    (default `error`), `currentState.topicFanout` (default `warn`), and `currentState.maxTopicsPerPath`
    "tune the severities". Rewrite so only `maxTopicsPerPath` remains, described as a budget rather than a
    severity, and state that coverage always evaluates at error and fan-out at warn.
  - `.awf/docs/parts/roadmap/ideas.md`: the idea proposing to promote the topic-claim-budget advisory to a
    configurable severity with an adopter-facing config key is now foreclosed in part. Withdraw the
    configurable-severity and adopter-facing-config-key half. What survives is promoting the advisory from a
    non-failing note to a FIXED blocking rank, still needing its own small ADR revising
    `tooling/cli:topic-claim-budget-advisory`. Do not rewrite the residue as "the budget threshold remains
    configurable": that is already shipped behaviour under
    `config/configuration:topic-claim-budget-configured`, so recording it as a future idea would be wrong.
  - `changelog/CHANGELOG.md`, under `## [Unreleased]` `### Breaking changes`: add an entry naming both
    removed keys, the fixed ranks, the removal of `off`, and schema generation 24 with `awf upgrade` as the
    migration path. Match the surrounding entries' voice and line width.
  - `internal/project/currentstate.go:26-28`, the doc comment describing coverage findings as each
    carrying "its configured severity, ADR-0134 item 11": correct it to the fixed ranks.
  - `internal/project/currentstate.go:445-446`, the `coveragePolicy` doc comment saying it "reads the
    coverage and fan-out severities and the fan-out budget from a currentState config block": rewrite it to
    say it reads only the fan-out budget from config, with the requested checks and their ranks fixed in
    code. Task 3.4's post-check grep does not match this wording, so it must be corrected here.
  - the `docs/config-reference.md` and `docs/glossary.md` regenerations in both trees come from `./x render`;
    stage them, never hand-edit them.

- [ ] **Task 2.11: Author the two claims and record the application.** Claim order in a part file renders as
  file order, and none of these files is strictly alphabetical, so each insertion point is given as an exact
  preceding heading. In `.awf/topics/parts/config/configuration/current-state.md`, insert immediately after
  the `scope-config-dual-form` claim block:

  ```markdown
  ### `invariant: severity-not-configurable`

  The currentState configuration exposes no severity setting: no configuration value selects, suppresses, or reranks topic coverage and topic fan-out, where a caller requests one it reports at error and the other at warn, and a tree carrying a currentState.topicCoverage or currentState.topicFanout key is rejected by strict parsing rather than honoured.
  Origin: ADR-0179
  Backing: test
  ```

  The sentence is scoped to the configuration surface on purpose: an unqualified "always evaluate" would be
  false on the uncovered-report path, which requests coverage only, and would contradict the sibling claim
  Phase 3 adds.

  In `.awf/topics/parts/config/migrations-and-locks/current-state.md`, insert immediately after the
  `schema-version-lock` claim block:

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
  delete the `type CoverageSeverity string` declaration, its doc comment, and its `CoverageError`,
  `CoverageWarn`, and `CoverageOff` constants (currently `:53-65`; starting at `:56` orphans the doc comment
  above an unrelated declaration, the same trap Tasks 1.4 and 1.6 call out), and add the
  `internal/severity` import. Change
  `CoverageFinding.Severity` to `severity.Rank` and DELETE its `json:"severity"` tag, leaving the tags on the
  other fields untouched.

  Settled here rather than at execution time: no production consumer marshals a `CoverageFinding`. Its only
  uses are the struct field at `internal/project/currentstate.go:33` and `coverageLine` at `:70`, which
  formats Path, Kind, and Topics only; the one JSON encoder on a topic surface, `cmd/awf/topic.go:57`,
  encodes `topic.QueryResult`, whose coverage member carries only topic applicability. So no `MarshalJSON` is
  added to `internal/severity`: a serialization method with no production consumer is exactly what
  `code-design/dependency-composition:concrete-first-consumer` refuses. Dropping the tag alongside it removes
  the reason the method looked necessary, rather than leaving a tagged field whose value would marshal as `0`
  or `1`. If a caller ever needs to serialize a rank, it lands with that caller and its spelling method
  together.

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

  Update the `EvaluateCoverage` doc comment. Three sentences change, not one: "a CoverageOff severity
  suppresses its findings" is false once `off` is gone and is replaced by the selection contract, and the
  phrases "one Uncovered finding at the coverage severity" (`:115`) and "a single Fanout finding at the
  fan-out severity" (`:119`) both imply the policy still supplies the rank, so each names its fixed rank
  instead (error and warn respectively). Keep the existing sentences about per-owner gaps, global and
  claimless topics, and once-per-path fan-out counting, which are unchanged.

- [ ] **Task 3.3: Update both policy construction sites.** `internal/project/context.go:371` currently
  passes `topic.CoveragePolicy{Coverage: topic.CoverageError, Fanout: topic.CoverageOff}`, which used a
  rank as a control-flow switch. It becomes a request for coverage only, preserving the uncovered report's
  current output exactly:

  ```go
  	for _, f := range topic.EvaluateCoverage(corpus, scoped, topic.CoveragePolicy{Coverage: true, Fanout: false}) {
  ```

  `internal/project/currentstate.go:449-450`, set to the fixed constants in Task 2.3, becomes
  `Coverage: true, Fanout: true` with `MaxTopicsPerPath` still from config.

  Two further production consumers of the deleted constants decide the blocking-versus-note routing and must
  be converted with them, not improvised: `internal/project/currentstate.go:45`
  (`if c.Severity == topic.CoverageError` in `CurrentStateReport.Findings()`) becomes `severity.Error`, and
  `:61` (`topic.CoverageWarn` in `Notes()`) becomes `severity.Warn`. The `internal/severity` import is
  already present in that file from Task 1.5. Also rewrite the `coveragePolicy` doc comment at `:445-446`
  again if Task 2.10's rewrite still mentions severities at all.

  Post-check: `grep -rn "CoverageOff\|CoverageError\|CoverageWarn\|CoverageSeverity" --include='*.go' internal cmd | grep -v _test.go`
  returns no output. The `_test.go` exclusion is required at this step: `internal/topic/coverage_test.go` and
  `internal/project/currentstate_test.go` still carry the old constants until Task 3.5 converts them, so the
  whole-tree form belongs there and is stated as Task 3.5's post-check.

- [ ] **Task 3.4: Update the remaining doc comments.** The `CoverageSeverity` doc comment is deleted by Task
  3.1 rather than corrected, since the type it documents is gone. Two remain:
  `internal/project/currentstate.go:52-54` says "Off findings are never emitted by the evaluator" (the
  sentence itself is at `:53`), and `internal/project/context.go:361-363` explains a forced coverage severity
  "regardless of the project's configured strictness" that no longer varies. Correct both to the shipped
  behaviour. Post-check: `grep -rn "CoverageOff\|configured strictness\|configured severity"
  --include='*.go' internal | grep -v _test.go` returns no output.

- [ ] **Task 3.5: Update the topic tests and place both proof markers.** In
  `internal/topic/coverage_test.go`, convert every `CoverageSeverity` reference and every policy literal to
  the new shape. Add or extend a test asserting the selection contract: a policy requesting coverage only
  produces no Fanout finding even when a path exceeds the budget, and a policy requesting fan-out only
  produces no Uncovered finding. Place:

  ```go
  // invariant: invariants/topics-and-markers:coverage-evaluation-selects-checks
  ```

  In `internal/project/currentstate_test.go:93-94`, convert the `topic.CoverageError` and `topic.CoverageWarn`
  literals to `severity.Error` and `severity.Warn`; the file also constructs a `currentstate.Finding` at
  `:91` that Task 4.4 handles.

  For the spelling claim, extend `internal/severity/severity_test.go`, which is already `package
  severity_test` and may therefore import `internal/audit` and `internal/topic` without a cycle. Add a test
  that renders `severity.Error` and `severity.Warn` directly, an `audit.Finding`'s rank, and a
  `topic.CoverageFinding`'s rank, asserting each renders within the set {`error`, `warn`} and that none
  renders `warning`. Place the marker there:

  ```go
  // invariant: tooling/audit-commands:severity-single-spelling
  ```

  Post-check for this task, now that every consumer is converted:
  `grep -rn "CoverageOff\|CoverageError\|CoverageWarn\|CoverageSeverity" --include='*.go' internal cmd`
  returns no output across the whole tree, tests included.

- [ ] **Task 3.6: Widen the destination topic's metadata summary.** The
  `invariants/topics-and-markers` summary describes parsing and resolution only and does not cover coverage
  evaluation policy, which the new claim is about. The summary is machine-consumed: it renders into
  `docs/topics/invariants/topics-and-markers.md` and appears in every `awf context` authority block, so drift
  depends on its exact bytes. In `.awf/topics/metadata/invariants/topics-and-markers.yaml`, replace the
  `summary:` value exactly:

  ```yaml
  summary: How topic inputs, claims, and their relevance and proof markers are parsed and resolved, and how coverage evaluation selects its checks.
  ```

  Then run `./x render` and stage every regenerated file it reports. A topic summary renders beyond its own
  topic document: it also reaches `docs/domains/invariants.md` and `docs/topics/invariants/index.md`, so
  staging only the topic document would leave the phase red at close.

- [ ] **Task 3.7: Author the two claims and record the application.** In
  `.awf/topics/parts/tooling/audit-commands/current-state.md`, insert immediately after the
  `repoaudit-requires-explicit-range` claim block:

  ```markdown
  ### `invariant: severity-single-spelling`

  Every finding rank awf reports renders as exactly error or warn: one shared two-member rank backs the audit findings, the repo-local audit tool, and current-state topic coverage, and no finding rank renders as warning.
  Origin: ADR-0179
  Backing: test
  ```

  The final clause is scoped to the rank deliberately. Both audit verdict summaries still print a
  `%d warning(s)` count (`cmd/awf/audit.go:58` and `:61`, `cmd/repoaudit/main.go:115`), which ADR-0179 item 3
  leaves out of scope; an unqualified "no surface renders warning" would be a false obligation the gate
  cannot catch.

  In `.awf/topics/parts/invariants/topics-and-markers/current-state.md`, insert immediately after the
  `claim-id-qualified` claim block, which keeps the file's alphabetical run intact
  (`coverage-evaluation-selects-checks` sorts after `claim-id-qualified`, not after
  `backed-requires-proof`):

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

  Then run `./x render` and stage every regenerated file it reports. `INDEX.md` is unaffected here, since
  this phase appends an Applied event without a status change, but the two topic documents are not.

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

Commit scope note: the phase removes an exported type and a struct field from `internal/currentstate` and
adjusts an `internal/project` consumer, which
`code-design/dependency-composition:dependency-composition-commit-classification` would place under the
`code-design` scope. The scope is `invariants` because the phase's dominant concern is retiring an
invariants-domain rank and freezing both governance records; the type removal is the mechanism, not the
point. `./x gate` cannot arbitrate this, so the choice is stated rather than left implicit.

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
  type is declared in `check.go`. One cross-package literal is also affected:
  `internal/project/currentstate_test.go:91` constructs `currentstate.Finding{{Severity: currentstate.Error,
  Message: ...}}` and becomes `currentstate.Finding{{Message: "handshake broke"}}`. Post-check:
  `grep -rn "currentstate.Error\|Finding{Error\|{Error, " --include='*.go' internal` returns no output, and
  `go build ./...` succeeds.

- [ ] **Task 4.3: Simplify the two consumers.** `internal/project/currentstate.go:409` maps every
  current-state finding to `audit.Finding{Severity: severity.Error, ...}` using `f.Message`; it keeps doing
  exactly that, with the now-absent source field simply not read. `:41` already reads only `f.Message`.
  Confirm neither reads a severity from the current-state finding: `grep -n "\.Severity" internal/project/currentstate.go`
  must show only `audit.Finding` and `topic.CoverageFinding` uses. Update the `CurrentStateReport` doc
  comment at `:26` if it describes a rank the handshake findings no longer carry.

- [ ] **Task 4.4: Retire the dead test and place the proof marker.** `TestSeverityString` at
  `internal/currentstate/check_test.go:480` was the only consumer of `Warn` and retires with the type.

  The claim describes a structural property, so a behavioural assertion cannot back it: once the field is
  gone, no ordinary test fails if someone adds it back, and a proof marker on such a test would stay green
  through the exact regression it is supposed to catch. Back it by reflection instead, which does fail. Add
  to `internal/currentstate/check_test.go`:

  ```go
  // invariant: invariants/current-state-authority:currentstate-handshake-findings-unranked
  func TestFindingCarriesOnlyMessage(t *testing.T) {
  	typ := reflect.TypeFor[currentstate.Finding]()
  	if typ.NumField() != 1 {
  		t.Fatalf("Finding has %d fields, want exactly 1: a handshake finding carries no rank", typ.NumField())
  	}
  	if name := typ.Field(0).Name; name != "Message" {
  		t.Fatalf("Finding field 0 is %q, want %q", name, "Message")
  	}
  }
  ```

  Add `"reflect"` to that file's imports. Also extend an existing behavioural test, or add one, asserting
  the check path treats every handshake finding as blocking, so the claim's second clause is exercised too;
  that assertion is not the marker site.

  Post-check: `go test ./internal/currentstate/... ./internal/project/...` passes, and re-adding a
  `Severity` field to `Finding` makes `TestFindingCarriesOnlyMessage` fail. Verify that second half by
  actually adding the field, running the test, and reverting: a proof marker that cannot fail is worse than
  none.

- [ ] **Task 4.5: Author the final claim.** In
  `.awf/topics/parts/invariants/current-state-authority/current-state.md`, insert immediately after the
  `current-state-sole-active-authority` claim block:

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

**The ADR number is already taken on main and MUST be pinned as the first step of the integration
transaction.** This branch forked from main at `65c4e967`, where 0178 was the tip. Main has since advanced
past it twice: `0179-rendered-explorer-and-grounding-checker-role-contracts.md` and
`0180-state-ownership-and-derived-state-lifetime.md` both landed on 2026-07-30, the same day this branch was
cut. So the number this ADR and plan use collides, and 0180 is taken as well.

Do not assume any specific successor number. Two ADRs were proposed on main within hours of each other, so
the free number moves faster than this branch's implementation will. The number is therefore DERIVED, not
predicted: as the first step of the integration transaction, after the merge brings main's ADRs in, read the
actual tip of `docs/decisions/` and take the next number. Pin it once, apply it across every site listed
below in the same transaction, and do not begin the phase commits with a number chosen earlier.

The renumber cannot be done on this branch. `awf check` enforces contiguous ADR numbering, and this branch
does not contain main's 0179, so renaming to 0180 here fails with `ADR numbers are not contiguous from 1:
missing [179]`. Verified by attempting it. The renumber therefore belongs to the integration transaction,
once main's ADRs are present, which is the ADR-0151-to-0153 precedent: author on the branch, renumber
mechanically at merge time.

At integration: build the merge with main's side as HEAD (the staged-check transition handshake validates
HEAD-to-index as one transition, so merging main INTO this branch fails structurally), then re-derive the
next free number rather than assuming 0180, since main may have moved again. Rename the ADR file and update
its H1, this plan's `adrs:` frontmatter and every `ADR-NNNN` reference in both files, the `Origin:` line in
each of the five claim blocks, the roadmap idea in `.awf/docs/parts/roadmap/ideas.md`, and the changelog entry
from Task 2.10 if it carries the number.

The renumber must also reach the ADR citations this plan writes into PRODUCTION GO SOURCE, which nothing
validates: `checkDeadRefs` scans rendered files, and the ADR checks in `internal/project/check.go` cover
frontmatter tags and related arrays only, so a stale citation in a Go comment stays green while pointing at
an unrelated ADR. The sites are `internal/severity/severity.go` (two citations, Task 1.1),
`internal/migrate/dropseveritysettings.go` (one, Task 2.4), and `internal/topic/coverage.go` (one, Task 3.2).
Close the integration step with `grep -rn 'ADR-0179' --include='*.go' internal cmd` returning no output,
substituting whatever number this ADR carried.

The `state-sequence` values need no pre-assignment: every phase takes the next value reported at execution
time.

Checked at authoring: main's 0179 declares only `rendering/*` operations and its 0180 declares five
`code-design/state-ownership` operations, so neither collides with any of this plan's five claim operations.

**One live design interaction to re-check before Phase 3.** Main's 0180 creates a third `code-design` topic,
`code-design/state-ownership`, whose claims include `project-derived-state-ownership`. It was `Proposed` when
this plan settled, so it carried no authority over Phase 3 or Phase 4. If it reaches Accepted or Implemented
first, re-read it against the `CurrentStateReport` changes in Tasks 3.1 through 3.3 and Task 4.3, which
reshape exactly the derived-state surface in `internal/project` its name suggests it governs. This is an
authority check, not a known conflict.

**One chain step was deliberately skipped, by user decision.** The resync pass produced two findings that
implicated the ADR rather than the plan, and the ADR was amended for them while still `Proposed`.
`awf-reviewing-plan-resync` prescribes routing such an amendment back through `awf-reviewing-adr` and then a
second resync. That loop was not run: the amendment was authored to the resync reviewer's own written
specification, touches two prose passages, and changes no `State changes` operation, no Decision item, and no
claim destination. Recorded here so a later reader sees the deviation rather than inferring the loop ran.

**Two execution deviations, recorded at freeze.**

First, Phase 2 needed one change this plan did not anticipate. `migrate.ConfigForCurrentSchema` forward-ports
a historical committed config so the current strict parser can read it, and it does so through an explicit
per-migration byte-level branch; only generation 20 had one. Without a generation-24 branch every
`awf check --staged` failed to parse HEAD the moment the two keys left the config model, because HEAD still
carried them. The four drop-a-key precedents never exposed this: none of those keys was actually set in this
repo, which is also why Task 2.4's precedent reading did not surface it. Added with the two nested removals
and a test covering the forward-port, its idempotence, and the skip at generation 24. Anyone removing a
config key hereafter needs that branch, and the phase-close staged check is where its absence surfaces.

Second, Task 2.8's enumeration was incomplete. Beyond the config and configspec sites it lists, fixtures in
`internal/project/currentstate_test.go`, `internal/project/context_test.go`, `internal/upgrade/upgrade_test.go`,
and `internal/migrate/maxclaimspertopic_test.go` also carried the keys, most of them only to make the
`currentState` block non-empty. Each was re-anchored on a surviving key rather than deleted, and the plan's
own Verification greps caught them, which is what that section is for.

Task 4.4's second post-check half was satisfied in two layers rather than one. Re-adding a `Severity` field
breaks compilation outright, because every literal in the package is positional, so the reflection test never
gets to run. The reflection assertion was therefore proven against the shape that would compile - a keyed
two-field `Finding` - confirming it reports "Finding has 2 fields, want exactly 1" rather than passing.

Out of scope, recorded during authoring:

- Letting a global topic carry path selectors, so a shared-pattern holder like `internal/severity` could be
  owned by the global topic describing its pattern rather than by whichever scoped topic is nearest. Phase 1
  Task 1.8 extends `tooling/audit-and-snapshots` precisely because that capability does not exist. Recorded
  as a roadmap idea needing its own ADR.
- The outcome-modeling pattern topic that ADR-0179 unblocks. Its shape is settled but it is a separate
  effort, and two of the claims it would otherwise need retire with this plan.
- `internal/prosegate` and `internal/memorycite` produce findings that carry no rank and are untouched here.
  Their duplicated pinned-exemption comparison is a separate known candidate.
- The `%d warning(s)` verdict summaries at `cmd/awf/audit.go:58`, `:61`, and `cmd/repoaudit/main.go:115`
  survive deliberately, per ADR-0179 item 3. They are verdict prose, not a finding rank, so
  `severity-single-spelling` is scoped to the rank and a later reader should not read the surviving
  summaries as contradicting it.
